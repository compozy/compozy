package task

import (
	"fmt"

	"sync"
	"time"

	"github.com/compozy/agh/internal/admission"
	configdefaults "github.com/compozy/agh/internal/config/defaults"
	eventspkg "github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/network/participation"

	"github.com/compozy/agh/internal/store"
)

const (
	managerActiveKey         = "active"
	managerOrphanedOnBootKey = "orphaned_on_boot"
	runBootRecoveryReasonKey = "reason"
)

const (
	taskEventCreated                          = eventspkg.TaskCreated
	taskEventUpdated                          = eventspkg.TaskUpdated
	taskEventPublished                        = eventspkg.TaskPublished
	taskEventApproved                         = eventspkg.TaskApproved
	taskEventRejected                         = eventspkg.TaskRejected
	taskEventCanceled                         = eventspkg.TaskCanceled
	taskEventChildCreated                     = eventspkg.TaskChildCreated
	taskEventDependencyAdded                  = eventspkg.TaskDependencyAdded
	taskEventDependencyRemoved                = eventspkg.TaskDependencyRemoved
	taskEventPaused                           = eventspkg.TaskPaused
	taskEventResumed                          = eventspkg.TaskResumed
	taskEventBlockCreated                     = eventspkg.TaskBlockCreated
	taskEventBlockCleared                     = eventspkg.TaskBlockCleared
	taskEventBlockExpired                     = eventspkg.TaskBlockExpired
	taskEventRunEnqueued                      = eventspkg.TaskRunEnqueued
	taskEventRunClaimed                       = eventspkg.TaskRunClaimed
	taskEventRunStarting                      = eventspkg.TaskRunStarting
	taskEventRunSessionBound                  = eventspkg.TaskRunSessionBound
	taskEventRunStarted                       = eventspkg.TaskRunStarted
	taskEventRunCompleted                     = eventspkg.TaskRunCompleted
	taskEventRunFailed                        = eventspkg.TaskRunFailed
	taskEventRunCanceled                      = eventspkg.TaskRunCanceled
	taskEventRunForceStopped                  = eventspkg.TaskRunForceStopped
	taskEventRunRecovered                     = eventspkg.TaskRunRecovered
	taskEventRunRejected                      = eventspkg.TaskRunRejected
	taskEventRunLeaseExtended                 = eventspkg.TaskRunLeaseExtended
	taskEventRunLeaseExpired                  = eventspkg.TaskRunLeaseExpired
	taskEventRunReleased                      = eventspkg.TaskRunReleased
	taskEventRunOperatorForcedFail            = eventspkg.TaskRunOperatorForcedFail
	taskEventRunOperatorRetry                 = eventspkg.TaskRunOperatorRetry
	taskEventRunRecoveredFromAttention        = eventspkg.TaskRunRecoveredFromAttention
	taskEventRunStarved                       = eventspkg.TaskRunStarved
	taskEventRunNeedsAttention                = eventspkg.TaskRunNeedsAttention
	taskEventProfileUpdated                   = eventspkg.TaskExecutionProfileUpdated
	taskEventProfileDeleted                   = eventspkg.TaskExecutionProfileDeleted
	taskEventRunReviewRequested               = eventspkg.TaskRunReviewRequested
	taskEventRunReviewBound                   = eventspkg.TaskRunReviewBound
	taskEventRunReviewRecorded                = eventspkg.TaskRunReviewRecorded
	taskEventRunReviewApproved                = eventspkg.TaskRunReviewApproved
	taskEventRunReviewRejected                = eventspkg.TaskRunReviewRejected
	taskEventRunReviewBlocked                 = eventspkg.TaskRunReviewBlocked
	taskEventRunReviewError                   = eventspkg.TaskRunReviewError
	taskEventRunReviewTimeout                 = eventspkg.TaskRunReviewTimeout
	taskEventRunReviewInvalid                 = eventspkg.TaskRunReviewInvalidOutput
	taskEventRunReviewRetry                   = eventspkg.TaskRunReviewRetryEnqueued
	taskEventAutoEnqueueTriggered             = eventspkg.TaskAutoEnqueueTriggered
	taskEventCompletionHallucinationBlocked   = eventspkg.TaskCompletionHallucinationBlocked
	taskEventCompletionHallucinationSuspected = eventspkg.TaskCompletionHallucinationSuspected
	taskEventWakeDelivered                    = eventspkg.TaskWakeDelivered
	taskEventWakeSuppressed                   = eventspkg.TaskWakeSuppressed
)

// Option customizes Service construction.
type Option func(*managerOptions)

type managerOptions struct {
	store                 Store
	sessions              SessionExecutor
	runtimeViews          RuntimeViewReader
	inspectReader         InspectStateReader
	eventObserver         EventObserver
	reviewObserver        RunReviewRequestedObserver
	taskHooks             RunHookDispatcher
	coordinatorRunner     CoordinatorRunner
	generationFinalizer   GenerationStateFinalizer
	wakeNotifier          WakeNotifier
	participationResolver participation.Resolver
	coordinatorStatusOK   func(string) bool
	coordinatorHookOK     func(string) bool
	profileValidation     ExecutionProfileValidationOptions
	forceRecovery         ForceRecoveryOptions
	now                   func() time.Time
	newID                 func(prefix string) string
	cancelGracePeriod     time.Duration
	starvationAge         time.Duration
	blockRecurrenceLimit  int
	workspaceActiveRunCap int
	workAdmission         admission.Checker
}

// Service centralizes canonical task-domain creation, mutation, read, and
// graph-management rules above the persistence layer.
type Service struct {
	store                 Store
	sessions              SessionExecutor
	runtimeViews          RuntimeViewReader
	inspectReader         InspectStateReader
	eventObserver         EventObserver
	reviewObserver        RunReviewRequestedObserver
	taskHooks             RunHookDispatcher
	coordinatorRunner     CoordinatorRunner
	generationFinalizer   GenerationStateFinalizer
	wakeNotifier          WakeNotifier
	participationResolver participation.Resolver
	taskAuthorizer        ResourceAuthorizer
	runReadAuthorizer     RunReadAuthorizer
	coordinatorStatusOK   func(string) bool
	coordinatorHookOK     func(string) bool
	profileValidation     ExecutionProfileValidationOptions
	forceRecovery         ForceRecoveryOptions
	now                   func() time.Time
	newID                 func(prefix string) string
	cancelGracePeriod     time.Duration
	starvationAge         time.Duration
	blockRecurrenceLimit  int
	workspaceActiveRunCap int
	workAdmission         admission.Checker
	forceRateLimiter      *forceRunRateLimiter
	wakeMu                sync.Mutex
	wakeEventIDs          map[string]struct{}
	wakeEventOrder        []string
	liveMu                sync.Mutex
	liveSubscribers       map[uint64]*taskStreamSubscriber
	nextSubscriberID      uint64
}

var _ Manager = (*Service)(nil)

// WithStore injects the durable task-domain store consumed by the manager.
func WithStore(store Store) Option {
	return func(opts *managerOptions) {
		opts.store = store
	}
}

// WithSessionExecutor injects the runtime session bridge used by later
// task-run lifecycle operations.
func WithSessionExecutor(sessions SessionExecutor) Option {
	return func(opts *managerOptions) {
		opts.sessions = sessions
	}
}

// WithRuntimeViewReader injects optional session telemetry enrichment for task live reads.
func WithRuntimeViewReader(reader RuntimeViewReader) Option {
	return func(opts *managerOptions) {
		opts.runtimeViews = reader
	}
}

// WithInspectStateReader injects read-only runtime state used by task inspect.
func WithInspectStateReader(reader InspectStateReader) Option {
	return func(opts *managerOptions) {
		opts.inspectReader = reader
	}
}

// WithEventObserver injects a best-effort observer for immutable task events.
func WithEventObserver(observer EventObserver) Option {
	return func(opts *managerOptions) {
		opts.eventObserver = observer
	}
}

// WithRunReviewRequestedObserver injects a best-effort observer for newly
// persisted run review requests.
func WithRunReviewRequestedObserver(observer RunReviewRequestedObserver) Option {
	return func(opts *managerOptions) {
		opts.reviewObserver = observer
	}
}

// WithTaskRunHooks injects the task-run hook bridge used at authoritative run transitions.
func WithTaskRunHooks(hooks RunHookDispatcher) Option {
	return func(opts *managerOptions) {
		opts.taskHooks = hooks
	}
}

// WithCoordinatorRunner injects the in-daemon generation coordinator runner.
func WithCoordinatorRunner(runner CoordinatorRunner) Option {
	return func(opts *managerOptions) {
		opts.coordinatorRunner = runner
	}
}

// WithGenerationStateFinalizer injects the generation-state writer used inside coordinator finalization.
func WithGenerationStateFinalizer(finalizer GenerationStateFinalizer) Option {
	return func(opts *managerOptions) {
		opts.generationFinalizer = finalizer
	}
}

// WithWakeNotifier injects the creator-session wake bridge used at task transitions.
func WithWakeNotifier(notifier WakeNotifier) Option {
	return func(opts *managerOptions) {
		opts.wakeNotifier = notifier
	}
}

// WithParticipationResolver injects the single network participation resolver
// used before task-run reservation.
func WithParticipationResolver(resolver participation.Resolver) Option {
	return func(opts *managerOptions) {
		opts.participationResolver = resolver
	}
}

// WithExecutionProfileValidationOptions injects config-backed profile gates.
func WithExecutionProfileValidationOptions(options ExecutionProfileValidationOptions) Option {
	return func(opts *managerOptions) {
		opts.profileValidation = options
	}
}

// WithForceRecoveryOptions injects config-backed force-operation policy.
func WithForceRecoveryOptions(options ForceRecoveryOptions) Option {
	return func(opts *managerOptions) {
		opts.forceRecovery = options
	}
}

// WithManagerNow overrides the manager clock for deterministic tests.
func WithManagerNow(now func() time.Time) Option {
	return func(opts *managerOptions) {
		opts.now = now
	}
}

// WithIDGenerator overrides identifier generation for deterministic tests.
func WithIDGenerator(newID func(prefix string) string) Option {
	return func(opts *managerOptions) {
		opts.newID = newID
	}
}

// WithCancelGracePeriod overrides the cooperative-stop grace period used before
// requesting forced session termination during task-driven cancellation.
func WithCancelGracePeriod(timeout time.Duration) Option {
	return func(opts *managerOptions) {
		opts.cancelGracePeriod = timeout
	}
}

// WithBlockRecurrenceLimit overrides the same-kind re-block count before task escalation.
func WithBlockRecurrenceLimit(limit int) Option {
	return func(opts *managerOptions) {
		opts.blockRecurrenceLimit = limit
	}
}

// NewManager constructs one task-domain manager with the supplied dependencies.
func NewManager(opts ...Option) (*Service, error) {
	options := managerOptions{
		profileValidation: DefaultExecutionProfileValidationOptions(),
		forceRecovery: ForceRecoveryOptions{
			AllowAgentForce:    true,
			RateLimitPerMinute: DefaultForceRunRateLimitPerMinute,
		},
		now: func() time.Time {
			return time.Now().UTC()
		},
		newID:                store.NewID,
		starvationAge:        DefaultTaskStarvationAge,
		blockRecurrenceLimit: configdefaults.BlockRecurrenceLimit,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	if options.starvationAge <= 0 {
		options.starvationAge = DefaultTaskStarvationAge
	}
	if options.store == nil {
		return nil, fmt.Errorf("task: manager store is required")
	}
	if options.now == nil {
		return nil, fmt.Errorf("task: manager clock is required")
	}
	if options.newID == nil {
		return nil, fmt.Errorf("task: manager id generator is required")
	}
	if options.cancelGracePeriod < 0 {
		return nil, fmt.Errorf("task: manager cancel grace period must be zero or positive")
	}
	if options.blockRecurrenceLimit < 0 {
		return nil, fmt.Errorf("task: block recurrence limit must be zero or positive")
	}
	if options.workspaceActiveRunCap < 0 {
		return nil, fmt.Errorf("task: workspace active run cap must be zero or positive")
	}

	return newService(options), nil
}
