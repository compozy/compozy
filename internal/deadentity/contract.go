// Package deadentity owns workspace-scoped failure streaks and durable dead-runtime recovery.
package deadentity

import (
	"errors"
	"log/slog"
	"time"

	"github.com/compozy/agh/internal/store"
)

const (
	DefaultPermanentFailureThreshold = 5
	DefaultRecoveryInterval          = time.Minute
)

// FailureClass describes whether one failed attempt confirms a stable fault.
type FailureClass uint8

const (
	FailureTransient FailureClass = iota
	FailurePermanent
)

var ErrInvalidFailureClass = errors.New("deadentity: invalid failure class")

// ProbeDecision tells a runtime adapter whether it may contact an external entity.
type ProbeDecision struct {
	Allowed  bool
	Recovery bool
	Dead     bool
	Reason   string
	MarkedAt time.Time
}

// Status is the read-only dead state for one runtime entity.
type Status struct {
	Dead     bool
	Reason   string
	MarkedAt time.Time
}

// Option configures a Service.
type Option func(*Service)

// WithClock injects the service clock.
func WithClock(now func() time.Time) Option {
	return func(service *Service) {
		service.now = now
	}
}

// WithLogger injects the diagnostic logger used for fail-open persistence paths.
func WithLogger(logger *slog.Logger) Option {
	return func(service *Service) {
		service.logger = logger
	}
}

// WithEventStore enables public transition summaries.
func WithEventStore(events store.EventSummaryStore) Option {
	return func(service *Service) {
		service.events = events
	}
}

// WithPermanentFailureThreshold overrides the production threshold for tests.
func WithPermanentFailureThreshold(threshold int) Option {
	return func(service *Service) {
		service.permanentFailureThreshold = threshold
	}
}

// WithRecoveryInterval overrides the production recovery interval for tests.
func WithRecoveryInterval(interval time.Duration) Option {
	return func(service *Service) {
		service.recoveryInterval = interval
	}
}
