package providerusage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"maps"
	"slices"
	"sync"
	"time"

	"log/slog"

	"github.com/jonboulle/clockwork"
)

type Manager struct {
	lifecycle sync.Mutex
	closeOnce sync.Once
	closeErr  error
	mu        sync.Mutex
	options   Options
	profiles  map[string]*profileState
	root      context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	bootID    string
	enabled   bool
	stopped   bool
	started   bool
}

type profileState struct {
	revision    uint64
	targets     map[string]*targetState
	order       []string
	subscribers map[chan Revision]struct{}
}

type targetState struct {
	epoch       uint64
	target      Target
	snapshot    Snapshot
	identity    CredentialIdentity
	hasIdentity bool
	lastSuccess time.Time
	flight      *flight
	cooldown    *cooldown
}

type cooldown struct {
	identity CredentialIdentity
	until    time.Time
	bypassed bool
}
type flight struct {
	done     chan struct{}
	intent   Intent
	explicit bool
	maxAge   time.Duration
}

var _ Service = (*Manager)(nil)

func NewService(options Options) *Manager {
	if options.Clock == nil {
		options.Clock = clockwork.NewRealClock()
	}
	if options.RefreshInterval == 0 {
		options.RefreshInterval = 5 * time.Minute
	}
	if options.Logger == nil {
		options.Logger = slog.Default()
	}
	options.ProviderOrder = slices.Clone(options.ProviderOrder)
	options.Collectors = maps.Clone(options.Collectors)
	root, cancel := context.WithCancel(context.Background())
	var id [16]byte
	rand.Read(id[:])
	return &Manager{
		options:  options,
		profiles: make(map[string]*profileState),
		root:     root,
		cancel:   cancel,
		bootID:   "b_" + hex.EncodeToString(id[:]),
		enabled:  !options.Disabled,
	}
}

func (s *Manager) BootID() string { return s.bootID }

func (s *Manager) profileLocked(id string) *profileState {
	p := s.profiles[id]
	if p == nil {
		p = &profileState{targets: make(map[string]*targetState), subscribers: make(map[chan Revision]struct{})}
		s.profiles[id] = p
	}
	return p
}

func (s *Manager) resolve(ctx context.Context, profile string) error {
	if s.options.Resolver == nil {
		return nil
	}
	targets, err := s.options.Resolver.Targets(ctx, profile)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.enabled || s.stopped {
		return nil
	}
	p := s.profileLocked(profile)
	seen := make(map[string]bool, len(targets))
	order := make([]string, 0, len(targets))
	changed := false
	for _, target := range targets {
		seen[target.ProviderID] = true
		order = append(order, target.ProviderID)
		state := p.targets[target.ProviderID]
		if state == nil {
			state = &targetState{}
			p.targets[target.ProviderID] = state
		}
		if state.target.RouteFingerprint != target.RouteFingerprint || state.target.Status != target.Status ||
			state.target.ProviderID == "" {
			state.epoch++
			state.snapshot = initialSnapshot(target, s.options.Collectors[target.ProviderID])
			state.hasIdentity = false
			state.lastSuccess = time.Time{}
			state.cooldown = nil
			changed = true
		}
		state.target = target
	}
	for id := range p.targets {
		if !seen[id] {
			delete(p.targets, id)
			changed = true
		}
	}
	rank := func(id string) int {
		i := slices.Index(s.options.ProviderOrder, id)
		if i < 0 {
			return len(s.options.ProviderOrder)
		}
		return i
	}
	slices.SortStableFunc(order, func(a, b string) int { return rank(a) - rank(b) })
	if !slices.Equal(p.order, order) {
		changed = true
	}
	p.order = order
	if changed {
		s.changedLocked(p)
	}
	return nil
}

func initialSnapshot(t Target, collector Collector) Snapshot {
	result := Snapshot{
		ProviderID:   t.ProviderID,
		ProviderName: t.ProviderName,
		Status:       t.Status,
		Error:        t.Error,
		Windows:      []Window{},
		Balances:     []Balance{},
		SpendCaps:    []SpendCap{},
	}
	if result.Status == "" {
		result.Status = StatusRefreshing
		if collector == nil {
			result.Status = StatusUnsupported
			result.Error = NewUsageError(ErrorCodeNoCollector, t.ProviderName)
		}
	}
	return result
}

func (s *Manager) Envelope(ctx context.Context, profile string) (Envelope, error) {
	if err := s.resolve(ctx, profile); err != nil {
		return Envelope{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.envelopeLocked(profile), nil
}

func (s *Manager) envelopeLocked(profile string) Envelope {
	p := s.profileLocked(profile)
	now := s.options.Clock.Now()
	result := Envelope{
		ProfileID:   profile,
		Revision:    p.revision,
		GeneratedAt: now,
		Enabled:     s.enabled,
		Targets:     []Snapshot{},
	}
	if !s.enabled {
		return result
	}
	for _, id := range p.order {
		row := p.targets[id]
		if row.snapshot.Status == StatusOK && row.snapshot.MeasuredAt != nil &&
			now.Sub(*row.snapshot.MeasuredAt) > 3*s.options.RefreshInterval {
			row.snapshot.Status = StatusStale
			s.changedLocked(p)
		}
		snapshot := cloneSnapshot(row.snapshot)
		snapshot.Refreshing = row.flight != nil
		result.Targets = append(result.Targets, snapshot)
	}
	result.Revision = p.revision
	result.Tray = SelectTray(result.Targets)
	return result
}

func (s *Manager) changedLocked(p *profileState) {
	p.revision++
	for ch := range p.subscribers {
		select {
		case <-ch:
		default:
		}
		ch <- Revision{Revision: p.revision, BootID: s.bootID}
	}
}

func (s *Manager) Subscribe(profile string) (<-chan Revision, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.profileLocked(profile)
	ch := make(chan Revision, 1)
	p.subscribers[ch] = struct{}{}
	var once sync.Once
	return ch, func() { once.Do(func() { s.mu.Lock(); defer s.mu.Unlock(); delete(p.subscribers, ch); close(ch) }) }
}

func (s *Manager) Refresh(ctx context.Context, req RefreshRequest) (Envelope, error) {
	if req.Wait < 0 || req.Wait > 30*time.Second || req.MaxAge < 0 ||
		(req.Intent != "" && req.Intent != IntentBackground && req.Intent != IntentExplicit) {
		return Envelope{}, NewUsageError(ErrorCodeInvalidRequest, "")
	}
	if err := s.resolve(ctx, req.ProfileID); err != nil {
		return Envelope{}, err
	}
	flights, err := s.startRefresh(req)
	if err != nil {
		return Envelope{}, err
	}

	wait := req.Wait
	if wait == 0 {
		wait = 15 * time.Second
	}
	timer := s.options.Clock.NewTimer(wait)
	defer timer.Stop()
	for _, f := range flights {
		select {
		case <-f.done:
		case <-ctx.Done():
			return s.cached(req.ProfileID), ctx.Err()
		case <-timer.Chan():
			return s.cached(req.ProfileID), nil
		}
	}
	return s.cached(req.ProfileID), nil
}

func (s *Manager) cached(profile string) Envelope {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.envelopeLocked(profile)
}

func (s *Manager) admitFlightsLocked(p *profileState, ids []string, req RefreshRequest) []*flight {
	flights := make([]*flight, 0, len(ids))
	for _, id := range ids {
		state := p.targets[id]
		if state.target.Status != "" || s.options.Collectors[id] == nil {
			continue
		}
		f := state.flight
		if f == nil {
			intent := req.Intent
			if intent == "" {
				intent = IntentBackground
			}
			f = &flight{done: make(chan struct{}), intent: intent, maxAge: max(req.MaxAge, 30*time.Second)}
			state.flight = f
			s.wg.Add(1)
			go s.runFlight(s.root, req.ProfileID, id, state, f)
			s.changedLocked(p)
		} else if req.Intent == IntentExplicit && f.intent == IntentBackground {
			f.explicit = true
		}
		flights = append(flights, f)
	}
	return flights
}

func (s *Manager) startRefresh(req RefreshRequest) ([]*flight, error) {
	s.mu.Lock()
	if !s.enabled || s.stopped {
		s.mu.Unlock()
		return nil, NewUsageError(ErrorCodeProviderUsageDisabled, "")
	}
	p := s.profileLocked(req.ProfileID)
	ids := req.ProviderIDs
	if len(ids) == 0 {
		ids = p.order
	}
	for _, id := range ids {
		if p.targets[id] == nil {
			s.mu.Unlock()
			return nil, NewUsageError(ErrorCodeProviderNotFound, id)
		}
	}
	flights := s.admitFlightsLocked(p, ids, req)
	s.mu.Unlock()
	return flights, nil
}
