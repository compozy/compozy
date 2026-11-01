package monitor

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"
)

// TerminalWatch blocks until the watched terminal condition resolves or ctx ends.
type TerminalWatch func(ctx context.Context) (Outcome, error)

// TerminalWatcher resolves a terminal at creation time and returns the blocking watch.
type TerminalWatcher interface {
	WatchTerminal(ctx context.Context, spec Spec) (TerminalWatch, error)
}

// Waker admits one fired monitor to its owning session. It returns as soon as
// the wake is queued; ErrSessionGone marks a permanent refusal, and any other
// error is retried as transient.
type Waker interface {
	WakeSession(ctx context.Context, notification Notification) error
}

// Option configures a Service.
type Option func(*Service)

// WithLogger sets the service logger.
func WithLogger(logger *slog.Logger) Option {
	return func(s *Service) {
		if logger != nil {
			s.logger = logger
		}
	}
}

// WithClock overrides the wall clock used for timestamps and retention.
func WithClock(now func() time.Time) Option {
	return func(s *Service) {
		if now != nil {
			s.now = now
		}
	}
}

// WithObserver registers a callback for every change to a session's monitors:
// one starts, fires, is canceled, delivered, or abandoned. Calls run one at a
// time on a service-owned goroutine with a bounded context, never on the
// caller's path, and stop when the service shuts down. Changes that arrive
// while a session's refresh is still pending coalesce into that one call.
func WithObserver(observe func(ctx context.Context, sessionID string)) Option {
	return func(s *Service) {
		s.observer = observe
	}
}

// Service owns every live monitor and the goroutines that watch them. State is
// indexed by owning session; active monitors are capped per session, per
// terminal, and per daemon, and finished ones expire after finishedRetention.
type Service struct {
	terminals TerminalWatcher
	waker     Waker
	logger    *slog.Logger
	now       func() time.Time
	observer  func(ctx context.Context, sessionID string)
	notices   *noticeQueue

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu          sync.Mutex
	stopped     bool
	retired     map[string]struct{}
	sessions    map[string]map[string]*entry
	occupied    int
	perTerminal map[string]int
	lastSweep   time.Time
}

type entry struct {
	state       Monitor
	terminalKey string
	cancel      context.CancelFunc
}

// occupies reports whether the monitor still holds a slot in the active caps.
func (e *entry) occupies() bool {
	return e.state.Status == StatusActive || e.state.Status == StatusDelivering
}

// NewService builds a monitor service bound to a daemon lifetime context.
func NewService(ctx context.Context, terminals TerminalWatcher, waker Waker, opts ...Option) (*Service, error) {
	if ctx == nil {
		return nil, errors.New("monitor: service context is required")
	}
	if waker == nil {
		return nil, errors.New("monitor: waker is required")
	}
	serviceCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	service := &Service{
		terminals:   terminals,
		waker:       waker,
		logger:      slog.Default(),
		now:         func() time.Time { return time.Now().UTC() },
		ctx:         serviceCtx,
		cancel:      cancel,
		sessions:    make(map[string]map[string]*entry),
		perTerminal: make(map[string]int),
	}
	for _, opt := range opts {
		opt(service)
	}
	if service.observer != nil {
		service.notices = newNoticeQueue()
		service.wg.Go(service.dispatchNotices)
	}
	return service, nil
}

// Create validates spec, starts watching, and returns the active monitor.
func (s *Service) Create(ctx context.Context, spec Spec) (Monitor, error) {
	normalized, err := normalizeSpec(spec)
	if err != nil {
		return Monitor{}, err
	}
	var watch TerminalWatch
	if normalized.Kind == KindTerminal {
		if s.terminals == nil {
			return Monitor{}, errors.New("monitor: terminal service is unavailable")
		}
		watch, err = s.terminals.WatchTerminal(ctx, normalized)
		if err != nil {
			return Monitor{}, err
		}
	}
	id, err := newID()
	if err != nil {
		return Monitor{}, err
	}
	now := s.now()
	window := normalized.Timeout
	if normalized.Kind == KindTimer {
		window = normalized.Delay
	}
	item := &entry{
		state: Monitor{
			ID: id, SessionID: normalized.SessionID, Kind: normalized.Kind, Description: normalized.Description,
			TerminalID: normalized.TerminalID, Until: normalized.Until, Pattern: normalized.Pattern,
			Status: StatusActive, CreatedAt: now, DeadlineAt: now.Add(window),
		},
		terminalKey: terminalKey(normalized),
	}

	s.mu.Lock()
	created, err := s.startLocked(item, normalized, watch, now)
	s.mu.Unlock()
	if err != nil {
		return Monitor{}, err
	}
	s.notify(created.SessionID)
	return created, nil
}

func (s *Service) startLocked(item *entry, spec Spec, watch TerminalWatch, now time.Time) (Monitor, error) {
	if s.stopped {
		return Monitor{}, ErrStopped
	}
	if _, retired := s.retired[item.state.SessionID]; retired {
		return Monitor{}, ErrSessionGone
	}
	s.sweepLocked(now)
	if err := s.admitLocked(item); err != nil {
		return Monitor{}, err
	}
	runCtx, cancel := context.WithCancel(s.ctx)
	item.cancel = cancel
	s.insertLocked(item)
	created := item.state
	s.wg.Go(func() {
		defer cancel()
		s.run(runCtx, created, spec, watch)
	})
	return created, nil
}

// List returns the session's monitors, active first, newest first within each group.
func (s *Service) List(sessionID string) []Monitor {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneSessionLocked(sessionID, s.now())
	items := make([]Monitor, 0, len(s.sessions[sessionID]))
	for _, item := range s.sessions[sessionID] {
		items = append(items, item.state)
	}
	slices.SortFunc(items, compareMonitors)
	return items
}

// ActiveCounts reports, per requested session, how many monitors still hold a
// slot (active or delivering). Sessions without one are omitted.
func (s *Service) ActiveCounts(sessionIDs []string) map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	counts := make(map[string]int)
	for _, sessionID := range sessionIDs {
		for _, item := range s.sessions[sessionID] {
			if item.occupies() {
				counts[sessionID]++
			}
		}
	}
	return counts
}

// Cancel stops one active or delivering monitor owned by sessionID without waking the session.
func (s *Service) Cancel(sessionID, id string) (Monitor, error) {
	s.mu.Lock()
	item, ok := s.sessions[sessionID][id]
	if !ok {
		s.mu.Unlock()
		return Monitor{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	if !item.occupies() {
		state := item.state
		s.mu.Unlock()
		return state, nil
	}
	s.finishLocked(item, StatusCanceled, s.now())
	item.cancel()
	s.pruneSessionLocked(sessionID, s.now())
	state := item.state
	s.mu.Unlock()
	s.notify(sessionID)
	return state, nil
}

// CancelSession stops every monitor of a stopped or removed session and drops
// its history; nothing it registered may wake a later incarnation.
func (s *Service) CancelSession(sessionID string) int {
	canceled := s.cancelSession(sessionID)
	if canceled > 0 {
		s.notify(sessionID)
	}
	return canceled
}

func (s *Service) cancelSession(sessionID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	canceled := 0
	now := s.now()
	for _, item := range s.sessions[sessionID] {
		if item.occupies() {
			s.finishLocked(item, StatusCanceled, now)
			item.cancel()
			canceled++
		}
	}
	delete(s.sessions, sessionID)
	return canceled
}

// Shutdown cancels every monitor and waits for watcher goroutines.
func (s *Service) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.stopped = true
	s.mu.Unlock()
	s.cancel()
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("monitor: shutdown: %w", ctx.Err())
	}
}

func (s *Service) admitLocked(item *entry) error {
	if s.occupied >= MaxActiveTotal {
		return fmt.Errorf("%w: at most %d active monitors per daemon", ErrLimit, MaxActiveTotal)
	}
	active := 0
	for _, existing := range s.sessions[item.state.SessionID] {
		if existing.occupies() {
			active++
		}
	}
	if active >= MaxActivePerSession {
		return fmt.Errorf("%w: at most %d per session", ErrLimit, MaxActivePerSession)
	}
	if item.terminalKey != "" && s.perTerminal[item.terminalKey] >= MaxActivePerTerminal {
		return fmt.Errorf("%w: at most %d per terminal", ErrLimit, MaxActivePerTerminal)
	}
	return nil
}

func (s *Service) insertLocked(item *entry) {
	bucket := s.sessions[item.state.SessionID]
	if bucket == nil {
		bucket = make(map[string]*entry)
		s.sessions[item.state.SessionID] = bucket
	}
	bucket[item.state.ID] = item
	s.occupied++
	if item.terminalKey != "" {
		s.perTerminal[item.terminalKey]++
	}
}

// finishLocked moves an occupying monitor to a terminal status and releases its cap slots.
func (s *Service) finishLocked(item *entry, status Status, at time.Time) {
	if item.occupies() {
		s.occupied--
		if item.terminalKey != "" {
			if s.perTerminal[item.terminalKey]--; s.perTerminal[item.terminalKey] <= 0 {
				delete(s.perTerminal, item.terminalKey)
			}
		}
	}
	item.state.Status = status
	if item.state.FiredAt.IsZero() {
		item.state.FiredAt = at
	}
}

// sweepLocked expires finished history across all sessions at most once per sweepInterval.
func (s *Service) sweepLocked(now time.Time) {
	if now.Sub(s.lastSweep) < sweepInterval {
		return
	}
	s.lastSweep = now
	for sessionID := range s.sessions {
		s.pruneSessionLocked(sessionID, now)
	}
}

// pruneSessionLocked drops expired finished monitors and keeps only the newest
// retainedFinishedPerSession; an empty session bucket is removed.
func (s *Service) pruneSessionLocked(sessionID string, now time.Time) {
	bucket := s.sessions[sessionID]
	finished := make([]Monitor, 0)
	for id, item := range bucket {
		if item.occupies() {
			continue
		}
		if now.Sub(item.state.FiredAt) > finishedRetention {
			delete(bucket, id)
			continue
		}
		finished = append(finished, item.state)
	}
	if len(finished) > retainedFinishedPerSession {
		// Keep the most recent outcomes, not the most recently created monitors.
		slices.SortFunc(finished, func(a, b Monitor) int {
			if c := b.FiredAt.Compare(a.FiredAt); c != 0 {
				return c
			}
			return compareMonitors(a, b)
		})
		for _, stale := range finished[retainedFinishedPerSession:] {
			delete(bucket, stale.ID)
		}
	}
	if len(bucket) == 0 {
		delete(s.sessions, sessionID)
	}
}

func terminalKey(spec Spec) string {
	if spec.Kind != KindTerminal {
		return ""
	}
	return spec.WorkspaceID + "\x00" + spec.ProfileID + "\x00" + spec.TerminalID
}

func compareMonitors(a, b Monitor) int {
	aActive, bActive := a.Status == StatusActive, b.Status == StatusActive
	if aActive != bActive {
		if aActive {
			return -1
		}
		return 1
	}
	if c := b.CreatedAt.Compare(a.CreatedAt); c != 0 {
		return c
	}
	if a.ID < b.ID {
		return -1
	}
	if a.ID > b.ID {
		return 1
	}
	return 0
}

func newID() (string, error) {
	var raw [6]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("monitor: generate id: %w", err)
	}
	return "mon-" + hex.EncodeToString(raw[:]), nil
}
