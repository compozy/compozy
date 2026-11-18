package worktree

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

const (
	pullRequestIncomplete  = "incomplete"
	pullRequestLookupOK    = "ok"
	pullRequestLookupError = "error"
)

// Display freshness windows. An open pull request changes often and the UI shows its checks; a
// branch without one, or with a terminal one, rarely changes, and explicit refreshes (the turn-end
// check, a refresh click) bypass these windows when a fresh answer matters.
const (
	pullRequestOpenTTL     = time.Minute
	pullRequestTerminalTTL = 30 * time.Minute
	pullRequestAbsentTTL   = 10 * time.Minute
	pullRequestAbsentMax   = time.Hour
)

type PullRequestKey struct{ WorkspaceID, RepoKey, Branch string }

type PullRequestEntry struct {
	Key                      PullRequestKey
	Facts                    *ForgeStatus
	LookupState, LookupCause string
	AttemptAt, RetryAfter    *time.Time
	FailureCount             int
	Stale                    bool
}

type PullRequestStore interface {
	ReadPullRequests(context.Context, []PullRequestKey) (map[PullRequestKey]PullRequestEntry, error)
	SavePullRequest(context.Context, PullRequestEntry) error
}

type pullRequestFlight struct {
	explicit atomic.Bool
	skipped  bool
	started  bool
	done     chan struct{}
	entry    PullRequestEntry
	err      error
}
type repoKeyCache struct {
	key   string
	err   error
	until time.Time
}

// PullRequestService is the sole writer of canonical display observations. Exact
// delivery evidence never enters this cache.
type PullRequestService struct {
	store   PullRequestStore
	forge   ForgeProvider
	runner  GitRunner
	ctx     context.Context
	cancel  context.CancelFunc
	now     func() time.Time
	changed func(string)
	mu      sync.Mutex
	flights map[PullRequestKey]*pullRequestFlight
	repos   map[string]repoKeyCache
	// absent counts consecutive no-PR answers per branch, backing the window off toward an hour.
	// It is memory only: a restart starts each branch back at the base window.
	absent         map[PullRequestKey]int
	defaults       map[string]repoKeyCache
	running        int
	explicitQueue  []PullRequestKey
	automaticQueue []PullRequestKey
	wg             sync.WaitGroup
	closed         bool
}

func NewPullRequestService(
	ctx context.Context,
	store PullRequestStore,
	forge ForgeProvider,
	runner GitRunner,
	changed func(string),
) *PullRequestService {
	ctx, cancel := context.WithCancel(ctx)
	return &PullRequestService{
		store:    store,
		forge:    forge,
		runner:   runner,
		ctx:      ctx,
		cancel:   cancel,
		now:      time.Now,
		changed:  changed,
		flights:  make(map[PullRequestKey]*pullRequestFlight),
		repos:    make(map[string]repoKeyCache),
		absent:   make(map[PullRequestKey]int),
		defaults: make(map[string]repoKeyCache),
	}
}

func (s *PullRequestService) Close() {
	s.mu.Lock()
	s.closed = true
	s.cancel()
	for key, flight := range s.flights {
		if !flight.started {
			flight.err = s.ctx.Err()
			close(flight.done)
			delete(s.flights, key)
		}
	}
	s.explicitQueue, s.automaticQueue = nil, nil
	s.mu.Unlock()
	s.wg.Wait()
}

// Read returns cached observations and schedules a background refresh for each stale one.
func (s *PullRequestService) Read(ctx context.Context, keys []PullRequestKey) map[PullRequestKey]PullRequestEntry {
	return s.read(ctx, keys, true)
}

// Peek returns cached observations without contacting the forge, for rows whose pull request no
// longer changes what the user can do, such as removed worktrees.
func (s *PullRequestService) Peek(ctx context.Context, keys []PullRequestKey) map[PullRequestKey]PullRequestEntry {
	return s.read(ctx, keys, false)
}

func (s *PullRequestService) read(
	ctx context.Context,
	keys []PullRequestKey,
	refresh bool,
) map[PullRequestKey]PullRequestEntry {
	entries, err := s.store.ReadPullRequests(ctx, keys)
	if err != nil {
		return nil
	}
	for _, key := range keys {
		entry, ok := entries[key]
		if !ok {
			entry = PullRequestEntry{Key: key, LookupState: "pending"}
		}
		entry.Stale = s.stale(key, entry, s.now())
		entries[key] = entry
		if refresh && entry.Stale {
			s.RequestRefresh(key)
		}
	}
	return entries
}

func (s *PullRequestService) stale(key PullRequestKey, entry PullRequestEntry, now time.Time) bool {
	s.mu.Lock()
	absent := s.absent[key]
	s.mu.Unlock()
	return pullRequestStale(entry, now, absent)
}

// pullRequestStale applies the display window for the observed facts; absent is the number of
// consecutive no-PR answers for the branch.
func pullRequestStale(entry PullRequestEntry, now time.Time, absent int) bool {
	if entry.Facts == nil {
		return true
	}
	if entry.LookupState == pullRequestLookupError && entry.RetryAfter != nil && !now.Before(*entry.RetryAfter) {
		return true // A failed lookup is retried once its retry time passes, whatever window its facts have.
	}
	ttl := pullRequestOpenTTL
	switch {
	case entry.Facts.PRState == nil && entry.LookupState != pullRequestLookupOK:
		// An incomplete match, or a failed lookup over a no-PR answer, is not confirmed absence.
	case entry.Facts.PRState == nil:
		ttl = pullRequestAbsentTTL
		for range max(absent-1, 0) {
			if ttl >= pullRequestAbsentMax {
				break
			}
			ttl *= 2
		}
		ttl = min(ttl, pullRequestAbsentMax)
	case *entry.Facts.PRState == "merged" || *entry.Facts.PRState == "closed":
		ttl = pullRequestTerminalTTL
	}
	return entry.Facts.IsStale(now, ttl)
}

func (s *PullRequestService) RequestRefresh(key PullRequestKey) { s.start(key, false) }

func (s *PullRequestService) Refresh(ctx context.Context, key PullRequestKey) (PullRequestEntry, error) {
	entries, err := s.store.ReadPullRequests(ctx, []PullRequestKey{key})
	if err != nil {
		return PullRequestEntry{}, err
	}
	flight := s.start(key, true)
	if flight == nil {
		return PullRequestEntry{}, context.Canceled
	}
	select {
	case <-ctx.Done():
		entry := entries[key]
		entry.Stale = s.stale(key, entry, s.now())
		return entry, ctx.Err()
	case <-flight.done:
		return flight.entry, flight.err
	}
}

// RefreshDue is Refresh for a background watcher: while a recorded failure's
// RetryAfter still holds it returns that cached entry without contacting the forge.
func (s *PullRequestService) RefreshDue(ctx context.Context, key PullRequestKey) (PullRequestEntry, error) {
	entries, err := s.store.ReadPullRequests(ctx, []PullRequestKey{key})
	if err != nil {
		return PullRequestEntry{}, err
	}
	if entry, ok := entries[key]; ok && entry.RetryAfter != nil && s.now().Before(*entry.RetryAfter) {
		entry.Stale = s.stale(key, entry, s.now())
		return entry, nil
	}
	return s.Refresh(ctx, key)
}

func (s *PullRequestService) start(key PullRequestKey, explicit bool) *pullRequestFlight {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || key.WorkspaceID == "" || key.RepoKey == "" || key.Branch == "" {
		return nil
	}
	if flight := s.flights[key]; flight != nil {
		if explicit && !flight.explicit.Swap(true) && !flight.started {
			s.explicitQueue = append(s.explicitQueue, key)
		}
		return flight
	}
	flight := &pullRequestFlight{done: make(chan struct{})}
	flight.explicit.Store(explicit)
	s.flights[key] = flight
	if explicit {
		s.explicitQueue = append(s.explicitQueue, key)
	} else {
		s.automaticQueue = append(s.automaticQueue, key)
	}
	s.admitLocked()
	return flight
}

func (s *PullRequestService) admitLocked() {
	for !s.closed && s.running < 4 {
		queue := &s.explicitQueue
		if len(*queue) == 0 {
			queue = &s.automaticQueue
		}
		if len(*queue) == 0 {
			return
		}
		key := (*queue)[0]
		(*queue)[0] = PullRequestKey{}
		*queue = (*queue)[1:]
		flight := s.flights[key]
		if flight == nil || flight.started {
			continue
		}
		flight.started = true
		s.running++
		s.wg.Go(func() { s.runRefresh(key, flight) })
	}
}

func (s *PullRequestService) runRefresh(key PullRequestKey, flight *pullRequestFlight) {
	for {
		entry, err := s.refresh(key, flight)
		s.mu.Lock()
		if flight.skipped && flight.explicit.Load() {
			s.mu.Unlock()
			continue
		}
		flight.entry, flight.err = entry, err
		delete(s.flights, key)
		close(flight.done)
		s.running--
		s.admitLocked()
		s.mu.Unlock()
		return
	}
}

func (s *PullRequestService) refresh(key PullRequestKey, flight *pullRequestFlight) (PullRequestEntry, error) {
	flight.skipped = false
	entries, err := s.store.ReadPullRequests(s.ctx, []PullRequestKey{key})
	if err != nil {
		return PullRequestEntry{}, err
	}
	before, ok := entries[key]
	if !ok {
		before = PullRequestEntry{Key: key, LookupState: "pending"}
	}
	now := s.now().UTC()
	if !flight.explicit.Load() && before.RetryAfter != nil && now.Before(*before.RetryAfter) {
		flight.skipped = true
		return before, nil
	}
	ctx, cancel := context.WithTimeout(s.ctx, 8*time.Second)
	defer cancel()
	var facts *ForgeStatus
	if s.forge == nil {
		err = NewForgeFailure(ErrForgeUnavailable, "provider_disabled", nil)
	} else {
		facts, err = s.forge.Status(
			ctx,
			ForgeStatusRequest{
				WorkspaceID: key.WorkspaceID,
				RemoteURLs:  []string{"https://" + key.RepoKey + ".git"},
				Branch:      key.Branch,
			},
		)
	}
	if err == nil && facts == nil {
		err = NewForgeFailure(ErrForge, "provider_failed", nil)
	}
	if err == nil && facts.PRURL != "" {
		facts.PRURL, err = sanitizeForgeWebURL(facts.PRURL)
	}
	after := pullRequestAttemptResult(before, facts, err, now)
	if saveErr := s.store.SavePullRequest(s.ctx, after); saveErr != nil {
		return before, saveErr
	}
	s.recordAbsence(key, after, err)
	after.Stale = s.stale(key, after, now)
	if s.changed != nil &&
		(before.LookupState != after.LookupState || before.LookupCause != after.LookupCause ||
			!pullRequestFactsEqual(before.Facts, s.storedFacts(key, after.Facts))) {
		s.changed(key.WorkspaceID)
	}
	return after, err
}

// storedFacts returns the facts as the cache keeps them. The store holds only display fields, so
// comparing a live answer with a cached one would always differ; a failed reread falls back to the
// live answer, which at worst publishes one extra change.
func (s *PullRequestService) storedFacts(key PullRequestKey, live *ForgeStatus) *ForgeStatus {
	entries, err := s.store.ReadPullRequests(s.ctx, []PullRequestKey{key})
	if err != nil {
		return live
	}
	if entry, ok := entries[key]; ok {
		return entry.Facts
	}
	return live
}

func (s *PullRequestService) recordAbsence(key PullRequestKey, after PullRequestEntry, err error) {
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if after.LookupState == pullRequestLookupOK && after.Facts != nil && after.Facts.PRState == nil {
		s.absent[key]++
		return
	}
	delete(s.absent, key)
}

// pullRequestFactsEqual compares what a reader can see, ignoring when it was fetched: a refresh
// that only moves the timestamp must not publish a catalog change, which would make clients re-read
// the list and schedule more refreshes.
func pullRequestFactsEqual(before, after *ForgeStatus) bool {
	if before == nil || after == nil {
		return before == after
	}
	left, right := *before, *after
	left.FetchedAt, right.FetchedAt = nil, nil
	return reflect.DeepEqual(left, right)
}

func pullRequestAttemptResult(before PullRequestEntry, facts *ForgeStatus, err error, now time.Time) PullRequestEntry {
	after := before
	after.AttemptAt = &now
	if err != nil {
		after.LookupState = pullRequestLookupError
		after.LookupCause = ForgeFailureCause(err)
		if after.LookupCause == "" {
			after.LookupCause = "provider_failed"
		}
		after.FailureCount++
		delays := []time.Duration{time.Minute, 2 * time.Minute, 5 * time.Minute, 15 * time.Minute}
		retry := now.Add(delays[min(after.FailureCount-1, len(delays)-1)])
		if failure, ok := errors.AsType[*ForgeFailure](
			err,
		); ok && failure.RetryAfter != nil &&
			failure.RetryAfter.After(now) {
			retry = *failure.RetryAfter
		}
		after.RetryAfter = &retry
	} else {
		after.LookupState = pullRequestLookupOK
		after.LookupCause = ""
		after.FailureCount = 0
		after.RetryAfter = nil
		if facts.Selection == pullRequestIncomplete {
			after.LookupState = pullRequestIncomplete
			// Incomplete selection is distinct from a confirmed absence.
			after.Facts = &ForgeStatus{FetchedAt: &now, Selection: pullRequestIncomplete}
		} else {
			facts.FetchedAt = &now
			after.Facts = facts
		}
	}
	return after
}

// ForgetAutomatic drops a removed worktree's absence streak and queued display refresh.
// Explicit readers may still need delivery evidence for the same branch.
func (s *PullRequestService) ForgetAutomatic(key PullRequestKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.absent, key)
	if flight := s.flights[key]; flight != nil && !flight.started && !flight.explicit.Load() {
		flight.err = context.Canceled
		close(flight.done)
		delete(s.flights, key)
	}
}
