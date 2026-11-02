package monitor

import (
	"context"
	"log/slog"
	"time"
)

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

// WithStore makes monitors durable across daemon restarts.
func WithStore(store Store) Option {
	return func(s *Service) { s.store = store }
}

// WithOwners lets the service capture owner bindings and resume owners during recovery.
func WithOwners(owners Owners) Option {
	return func(s *Service) { s.owners = owners }
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

// WithSessionTargets enables session monitors.
func WithSessionTargets(targets SessionTargets) Option {
	return func(s *Service) { s.targets = targets }
}

// WithSessionWatchGate holds session watches until gate closes. The daemon closes it once restarted
// sessions are resumed and supervised, because a target read before that reports a false state.
func WithSessionWatchGate(gate <-chan struct{}) Option {
	return func(s *Service) { s.watchGate = gate }
}
