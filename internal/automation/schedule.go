package automation

import (
	"context"
	"errors"

	"log/slog"

	"sync"
	"time"

	"github.com/jonboulle/clockwork"
)

var (
	// ErrScheduledJobNotFound reports that a scheduler registration does not exist.
	ErrScheduledJobNotFound = errors.New("automation: scheduled job not found")
	// ErrScheduledJobAlreadyRegistered reports that the job is already registered with the scheduler.
	ErrScheduledJobAlreadyRegistered = errors.New("automation: scheduled job already registered")
	// ErrSchedulerStopped reports that the scheduler has already been stopped and cannot accept new work.
	ErrSchedulerStopped = errors.New("automation: scheduler stopped")
)

const (
	defaultSchedulerStopTimeout = 10 * time.Second
	schedulerCatchUpJitterGrace = time.Second
)

// ScheduleDispatcher is the execution surface used by scheduled jobs.
type ScheduleDispatcher interface {
	Dispatch(ctx context.Context, req DispatchRequest) (*Run, error)
}

// SchedulerStore persists durable scheduler cursor state and run
// reservations before dispatch.
type SchedulerStore interface {
	GetSchedulerState(ctx context.Context, jobID string) (SchedulerState, error)
	SaveSchedulerState(ctx context.Context, state SchedulerState) (SchedulerState, error)
	DeleteSchedulerState(ctx context.Context, jobID string) error
	ClaimScheduledRun(ctx context.Context, claim SchedulerClaim) (SchedulerClaimResult, error)
	RecordRunDeliveryError(ctx context.Context, runID string, runErr error) (Run, error)
}

// SchedulerOption customizes scheduled-job runtime behavior.
type SchedulerOption func(*Scheduler)

// SchedulerCatchUpPolicyResolver classifies a job's default catch-up behavior.
type SchedulerCatchUpPolicyResolver func(context.Context, Job) (SchedulerCatchUpPolicy, error)

// ScheduledJobState exposes runtime schedule metadata for one registered job.
type ScheduledJobState struct {
	JobID               string                 `json:"job_id"`
	Registered          bool                   `json:"registered"`
	NextRun             *time.Time             `json:"next_run,omitempty"`
	LastRun             *time.Time             `json:"last_run,omitempty"`
	LastScheduledAt     *time.Time             `json:"last_scheduled_at,omitempty"`
	LastFireID          string                 `json:"last_fire_id,omitempty"`
	CatchUpPolicy       SchedulerCatchUpPolicy `json:"catch_up_policy,omitempty"`
	MisfireGraceSeconds int                    `json:"misfire_grace_seconds,omitempty"`
	LastMisfireAt       *time.Time             `json:"last_misfire_at,omitempty"`
	MisfireCount        int                    `json:"misfire_count,omitempty"`
	Durable             *SchedulerState        `json:"durable,omitempty"`
}

// Scheduler owns durable cursor-driven scheduled-job dispatch.
type Scheduler struct {
	dispatcher   ScheduleDispatcher
	store        SchedulerStore
	logger       *slog.Logger
	clock        clockwork.Clock
	location     *time.Location
	stopTimeout  time.Duration
	policyForJob SchedulerCatchUpPolicyResolver

	mu            sync.RWMutex
	runtimeCtx    context.Context
	runtimeCancel context.CancelFunc
	wg            sync.WaitGroup
	started       bool
	stopped       bool
	registrations map[string]scheduledRegistration
}

type scheduledRegistration struct {
	definition   Job
	registeredAt time.Time
	state        SchedulerState
	cancel       context.CancelFunc
}

type schedulePlan struct {
	register bool
	nextRun  time.Time
}

// NewScheduler constructs a scheduled-job runtime over gocron.
func NewScheduler(dispatcher ScheduleDispatcher, opts ...SchedulerOption) (*Scheduler, error) {
	if dispatcher == nil {
		return nil, errors.New("automation: scheduler dispatcher is required")
	}

	scheduler := &Scheduler{
		dispatcher:    dispatcher,
		logger:        slog.Default(),
		clock:         clockwork.NewRealClock(),
		location:      time.UTC,
		stopTimeout:   defaultSchedulerStopTimeout,
		registrations: make(map[string]scheduledRegistration),
	}

	for _, opt := range opts {
		if opt != nil {
			opt(scheduler)
		}
	}

	if scheduler.logger == nil {
		scheduler.logger = slog.Default()
	}
	if scheduler.clock == nil {
		scheduler.clock = clockwork.NewRealClock()
	}
	if scheduler.location == nil {
		scheduler.location = time.UTC
	}
	if scheduler.stopTimeout <= 0 {
		scheduler.stopTimeout = defaultSchedulerStopTimeout
	}

	return scheduler, nil
}

// WithSchedulerLogger overrides the scheduler logger.
func WithSchedulerLogger(logger *slog.Logger) SchedulerOption {
	return func(scheduler *Scheduler) {
		scheduler.logger = logger
	}
}

// WithSchedulerStore injects durable scheduler cursor persistence.
func WithSchedulerStore(store SchedulerStore) SchedulerOption {
	return func(scheduler *Scheduler) {
		scheduler.store = store
	}
}

// WithSchedulerCatchUpPolicyResolver injects target-aware catch-up defaults.
func WithSchedulerCatchUpPolicyResolver(resolver SchedulerCatchUpPolicyResolver) SchedulerOption {
	return func(scheduler *Scheduler) {
		scheduler.policyForJob = resolver
	}
}

// WithSchedulerClock overrides the scheduler clock, mainly for tests.
func WithSchedulerClock(clock clockwork.Clock) SchedulerOption {
	return func(scheduler *Scheduler) {
		scheduler.clock = clock
	}
}

// WithSchedulerLocation overrides the timezone used for schedule evaluation.
func WithSchedulerLocation(location *time.Location) SchedulerOption {
	return func(scheduler *Scheduler) {
		scheduler.location = location
	}
}

// WithSchedulerStopTimeout overrides the graceful shutdown timeout used by gocron.
func WithSchedulerStopTimeout(timeout time.Duration) SchedulerOption {
	return func(scheduler *Scheduler) {
		scheduler.stopTimeout = timeout
	}
}
