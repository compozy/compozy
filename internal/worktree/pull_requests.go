package worktree

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"time"
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
	done  chan struct{}
	entry PullRequestEntry
	err   error
}
type repoKeyCache struct {
	key   string
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
	slots   chan struct{}
	wg      sync.WaitGroup
	closed  bool
}

func NewPullRequestService(ctx context.Context, store PullRequestStore, forge ForgeProvider, runner GitRunner, changed func(string)) *PullRequestService {
	ctx, cancel := context.WithCancel(ctx)
	return &PullRequestService{store: store, forge: forge, runner: runner, ctx: ctx, cancel: cancel, now: time.Now, changed: changed, flights: make(map[PullRequestKey]*pullRequestFlight), repos: make(map[string]repoKeyCache), slots: make(chan struct{}, 4)}
}

func (s *PullRequestService) Close() {
	s.mu.Lock()
	s.closed = true
	s.cancel()
	s.mu.Unlock()
	s.wg.Wait()
}

func (s *PullRequestService) Read(ctx context.Context, keys []PullRequestKey) map[PullRequestKey]PullRequestEntry {
	entries, err := s.store.ReadPullRequests(ctx, keys)
	if err != nil {
		return nil
	}
	for _, key := range keys {
		entry, ok := entries[key]
		if !ok {
			entry = PullRequestEntry{Key: key, LookupState: "pending"}
		}
		entry.Stale = pullRequestStale(entry, s.now())
		entries[key] = entry
		if entry.Stale {
			s.RequestRefresh(key)
		}
	}
	return entries
}

func pullRequestStale(entry PullRequestEntry, now time.Time) bool {
	ttl := time.Minute
	if entry.Facts != nil && entry.Facts.PRState != nil && (*entry.Facts.PRState == "merged" || *entry.Facts.PRState == "closed") {
		ttl = 10 * time.Minute
	}
	return entry.Facts == nil || entry.Facts.IsStale(now, ttl)
}

func (s *PullRequestService) RequestRefresh(key PullRequestKey) { s.start(key, false) }

func (s *PullRequestService) Refresh(ctx context.Context, key PullRequestKey) (PullRequestEntry, error) {
	flight := s.start(key, true)
	if flight == nil {
		return PullRequestEntry{}, context.Canceled
	}
	select {
	case <-ctx.Done():
		return PullRequestEntry{}, ctx.Err()
	case <-flight.done:
		return flight.entry, flight.err
	}
}

func (s *PullRequestService) start(key PullRequestKey, explicit bool) *pullRequestFlight {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || key.WorkspaceID == "" || key.RepoKey == "" || key.Branch == "" {
		return nil
	}
	if flight := s.flights[key]; flight != nil {
		return flight
	}
	flight := &pullRequestFlight{done: make(chan struct{})}
	s.flights[key] = flight
	s.wg.Go(func() {
		defer func() { s.mu.Lock(); delete(s.flights, key); close(flight.done); s.mu.Unlock() }()
		flight.entry, flight.err = s.refresh(key, explicit)
	})
	return flight
}

func (s *PullRequestService) refresh(key PullRequestKey, explicit bool) (PullRequestEntry, error) {
	// Admission is bounded before the provider is invoked, including explicit calls.
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-s.ctx.Done():
		return PullRequestEntry{}, s.ctx.Err()
	}
	entries, err := s.store.ReadPullRequests(s.ctx, []PullRequestKey{key})
	if err != nil {
		return PullRequestEntry{}, err
	}
	before, ok := entries[key]
	if !ok {
		before = PullRequestEntry{Key: key, LookupState: "pending"}
	}
	now := s.now().UTC()
	if !explicit && before.RetryAfter != nil && now.Before(*before.RetryAfter) {
		return before, nil
	}
	ctx, cancel := context.WithTimeout(s.ctx, 8*time.Second)
	defer cancel()
	var facts *ForgeStatus
	if s.forge == nil {
		err = NewForgeFailure(ErrForgeUnavailable, "provider_disabled", nil)
	} else {
		facts, err = s.forge.Status(ctx, ForgeStatusRequest{WorkspaceID: key.WorkspaceID, RemoteURLs: []string{"https://" + key.RepoKey + ".git"}, Branch: key.Branch})
	}
	if err == nil && facts == nil {
		err = NewForgeFailure(ErrForge, "provider_failed", nil)
	}
	if err == nil && facts.PRURL != "" {
		facts.PRURL, err = sanitizeForgeWebURL(facts.PRURL)
	}
	after := before
	after.AttemptAt = &now
	if err != nil {
		after.LookupState = "error"
		after.LookupCause = ForgeFailureCause(err)
		if after.LookupCause == "" {
			after.LookupCause = "provider_failed"
		}
		after.FailureCount++
		delays := []time.Duration{time.Minute, 2 * time.Minute, 5 * time.Minute, 15 * time.Minute}
		retry := now.Add(delays[min(after.FailureCount-1, len(delays)-1)])
		if failure, ok := errors.AsType[*ForgeFailure](err); ok && failure.RetryAfter != nil && failure.RetryAfter.After(now) {
			retry = *failure.RetryAfter
		}
		after.RetryAfter = &retry
	} else {
		after.LookupState = "ok"
		after.LookupCause = ""
		after.FailureCount = 0
		after.RetryAfter = nil
		if facts.Selection == "incomplete" {
			after.LookupState = "incomplete"
			// Incomplete selection is distinct from a confirmed absence.
			after.Facts = &ForgeStatus{FetchedAt: &now, Selection: "incomplete"}
		} else {
			facts.FetchedAt = &now
			after.Facts = facts
		}
	}
	after.Stale = pullRequestStale(after, now)
	if saveErr := s.store.SavePullRequest(s.ctx, after); saveErr != nil {
		return before, saveErr
	}
	if s.changed != nil && (before.LookupState != after.LookupState || before.LookupCause != after.LookupCause || !reflect.DeepEqual(before.Facts, after.Facts)) {
		s.changed(key.WorkspaceID)
	}
	return after, err
}
