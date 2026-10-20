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

// Waker delivers one fired monitor to its owning session.
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

// WithClock overrides the wall clock used for timestamps.
func WithClock(now func() time.Time) Option {
	return func(s *Service) {
		if now != nil {
			s.now = now
		}
	}
}

// Service owns every live monitor and the goroutines that watch them.
type Service struct {
	terminals TerminalWatcher
	waker     Waker
	logger    *slog.Logger
	now       func() time.Time

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu       sync.Mutex
	stopped  bool
	monitors map[string]*entry
}

type entry struct {
	state  Monitor
	cancel context.CancelFunc
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
		terminals: terminals,
		waker:     waker,
		logger:    slog.Default(),
		now:       func() time.Time { return time.Now().UTC() },
		ctx:       serviceCtx,
		cancel:    cancel,
		monitors:  make(map[string]*entry),
	}
	for _, opt := range opts {
		opt(service)
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
	state := Monitor{
		ID: id, SessionID: normalized.SessionID, Kind: normalized.Kind, Description: normalized.Description,
		TerminalID: normalized.TerminalID, Until: normalized.Until, Pattern: normalized.Pattern,
		Status: StatusActive, CreatedAt: now, DeadlineAt: now.Add(window),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return Monitor{}, ErrStopped
	}
	if s.activeCountLocked(normalized.SessionID) >= MaxActivePerSession {
		return Monitor{}, fmt.Errorf("%w: at most %d per session", ErrLimit, MaxActivePerSession)
	}
	runCtx, cancel := context.WithCancel(s.ctx)
	s.monitors[id] = &entry{state: state, cancel: cancel}
	s.wg.Go(func() {
		defer cancel()
		s.run(runCtx, id, normalized, watch)
	})
	return state, nil
}

// List returns the session's monitors, active first, newest first within each group.
func (s *Service) List(sessionID string) []Monitor {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]Monitor, 0)
	for _, item := range s.monitors {
		if item.state.SessionID == sessionID {
			items = append(items, item.state)
		}
	}
	slices.SortFunc(items, compareMonitors)
	return items
}

// Cancel stops one active monitor owned by sessionID without waking the session.
func (s *Service) Cancel(sessionID, id string) (Monitor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.monitors[id]
	if !ok || item.state.SessionID != sessionID {
		return Monitor{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	if item.state.Status != StatusActive {
		return item.state, nil
	}
	item.state.Status = StatusCanceled
	item.state.FiredAt = s.now()
	item.cancel()
	s.pruneLocked(sessionID)
	return item.state, nil
}

// CancelSession stops every active monitor owned by sessionID.
func (s *Service) CancelSession(sessionID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	canceled := 0
	for _, item := range s.monitors {
		if item.state.SessionID == sessionID && item.state.Status == StatusActive {
			item.state.Status = StatusCanceled
			item.state.FiredAt = s.now()
			item.cancel()
			canceled++
		}
	}
	s.pruneLocked(sessionID)
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

func (s *Service) run(ctx context.Context, id string, spec Spec, watch TerminalWatch) {
	outcome, ok := s.observe(ctx, spec, watch)
	if !ok {
		return
	}
	state, fire := s.markFired(id, outcome)
	if !fire {
		return
	}
	err := s.waker.WakeSession(s.ctx, Notification{Monitor: state, Outcome: outcome})
	if err == nil {
		return
	}
	s.logger.Warn("monitor: session wake failed",
		"monitor_id", id, "session_id", spec.SessionID, "reason", outcome.Reason, "error", err)
	s.mu.Lock()
	if item, exists := s.monitors[id]; exists {
		item.state.Status = StatusUndelivered
	}
	s.mu.Unlock()
}

func (s *Service) observe(ctx context.Context, spec Spec, watch TerminalWatch) (Outcome, bool) {
	if spec.Kind == KindTimer {
		timer := time.NewTimer(spec.Delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return Outcome{}, false
		case <-timer.C:
			return Outcome{Reason: ReasonTimerElapsed}, true
		}
	}
	outcome, err := watch(ctx)
	if ctx.Err() != nil {
		return Outcome{}, false
	}
	if err != nil {
		s.logger.Info("monitor: terminal watch ended without a result",
			"session_id", spec.SessionID, "terminal_id", spec.TerminalID, "error", err)
		return Outcome{Reason: ReasonTerminalFailed}, true
	}
	return outcome, true
}

func (s *Service) markFired(id string, outcome Outcome) (Monitor, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.monitors[id]
	if !ok || item.state.Status != StatusActive {
		return Monitor{}, false
	}
	item.state.Status = StatusFired
	item.state.FiredAt = s.now()
	item.state.Reason = outcome.Reason
	item.state.ExitCode = outcome.ExitCode
	s.pruneLocked(item.state.SessionID)
	return item.state, true
}

func (s *Service) activeCountLocked(sessionID string) int {
	count := 0
	for _, item := range s.monitors {
		if item.state.SessionID == sessionID && item.state.Status == StatusActive {
			count++
		}
	}
	return count
}

// pruneLocked keeps only the newest finished monitors per session for inspection.
func (s *Service) pruneLocked(sessionID string) {
	finished := make([]Monitor, 0)
	for _, item := range s.monitors {
		if item.state.SessionID == sessionID && item.state.Status != StatusActive {
			finished = append(finished, item.state)
		}
	}
	if len(finished) <= retainedFinishedPerSess {
		return
	}
	slices.SortFunc(finished, compareMonitors)
	for _, stale := range finished[retainedFinishedPerSess:] {
		delete(s.monitors, stale.ID)
	}
}

func compareMonitors(a, b Monitor) int {
	if (a.Status == StatusActive) != (b.Status == StatusActive) {
		if a.Status == StatusActive {
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
