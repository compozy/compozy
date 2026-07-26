package automation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"strings"
	"sync"

	"time"

	"github.com/compozy/agh/internal/acp"
	hookspkg "github.com/compozy/agh/internal/hooks"

	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
)

var (
	// ErrConcurrencyLimitReached reports that the shared automation gate rejected a new run.
	ErrConcurrencyLimitReached = errors.New("automation: global concurrency limit reached")
	// ErrFireLimitReached reports that a definition exceeded its rolling fire-limit window.
	ErrFireLimitReached = errors.New("automation: fire limit reached")
	// ErrLoopConcurrencyConflict reports a loop target rejected by its concurrency policy.
	ErrLoopConcurrencyConflict = errors.New("automation: loop concurrency conflict")
)

// FireLimitError carries the next eligible retry instant for fire-limit backoff.
type FireLimitError struct {
	Count   int64
	Limit   int
	Window  time.Duration
	RetryAt time.Time
}

func (e *FireLimitError) Error() string {
	if e == nil {
		return ErrFireLimitReached.Error()
	}
	message := fmt.Sprintf(
		"%s: fires=%d limit=%d window=%s",
		ErrFireLimitReached,
		e.Count,
		e.Limit,
		e.Window,
	)
	if !e.RetryAt.IsZero() {
		message += " retry_at=" + e.RetryAt.UTC().Format(time.RFC3339Nano)
	}
	return message
}

func (e *FireLimitError) Unwrap() error {
	return ErrFireLimitReached
}

// defaultDispatcherSessionStopTimeout must outlive the ACP driver's own
// graceful stop budget, otherwise automation runs can be marked failed while
// the session is still finishing a normal shutdown.
const defaultDispatcherSessionStopTimeout = 10 * time.Second

// DispatchKind identifies which activation path produced a dispatch request.
type DispatchKind string

const (
	// DispatchKindSchedule identifies time-based schedule execution.
	DispatchKindSchedule DispatchKind = "schedule"
	// DispatchKindTrigger identifies event-driven trigger execution.
	DispatchKindTrigger DispatchKind = "trigger"
	// DispatchKindManual identifies explicit user-initiated job execution.
	DispatchKindManual DispatchKind = "manual"
	// DispatchKindExtension identifies extension-fired automation execution.
	DispatchKindExtension DispatchKind = "extension"
)

// Validate ensures the dispatch kind is one of the supported activation paths.
func (k DispatchKind) Validate(path string) error {
	switch k {
	case DispatchKindSchedule, DispatchKindTrigger, DispatchKindManual, DispatchKindExtension:
		return nil
	default:
		return fmt.Errorf(
			"%s must be one of %q, %q, %q, or %q: %q",
			path,
			DispatchKindSchedule,
			DispatchKindTrigger,
			DispatchKindManual,
			DispatchKindExtension,
			k,
		)
	}
}

// DispatchRequest describes one normalized automation execution attempt.
//
// Exactly one of Job or Trigger must be provided. Triggers also require an
// activation envelope so prompt templates can render against the normalized
// trigger payload. Manual job dispatch can carry caller-supplied payload for
// job lifecycle hooks. Prompt allows later callers to inject a pre-render
// override after pre-fire hooks patch the outbound prompt.
type DispatchRequest struct {
	Kind          DispatchKind           `json:"kind"`
	Job           *Job                   `json:"job,omitempty"`
	Trigger       *Trigger               `json:"trigger,omitempty"`
	Envelope      *ActivationEnvelope    `json:"envelope,omitempty"`
	Payload       map[string]any         `json:"payload,omitempty"`
	Prompt        string                 `json:"prompt,omitempty"`
	ReservedRun   *Run                   `json:"-"`
	ScheduledAt   *time.Time             `json:"scheduled_at,omitempty"`
	CatchUp       bool                   `json:"catch_up,omitempty"`
	CatchUpPolicy SchedulerCatchUpPolicy `json:"catch_up_policy,omitempty"`
}

// Validate ensures the request can be executed by the shared dispatcher.
func (r DispatchRequest) Validate(path string) error {
	if err := r.Kind.Validate(nestedPath(path, "kind")); err != nil {
		return err
	}

	hasJob := r.Job != nil
	hasTrigger := r.Trigger != nil
	switch {
	case hasJob && hasTrigger:
		return errors.New(path + " must not define both job and trigger")
	case !hasJob && !hasTrigger:
		return errors.New(path + " must define either job or trigger")
	}

	if hasJob {
		if err := r.Job.Validate(nestedPath(path, "job")); err != nil {
			return err
		}
		return nil
	}

	if err := r.Trigger.Validate(nestedPath(path, "trigger")); err != nil {
		return err
	}
	if r.Envelope == nil {
		return errors.New(nestedPath(path, "envelope") + " is required for trigger dispatch")
	}
	if err := r.Envelope.Validate(nestedPath(path, "envelope")); err != nil {
		return err
	}
	if got, want := strings.TrimSpace(r.Envelope.Kind), strings.TrimSpace(r.Trigger.Event); got != want {
		return fmt.Errorf(
			"%s.kind must match %s.event: %q != %q",
			nestedPath(path, "envelope"),
			nestedPath(path, "trigger"),
			got,
			want,
		)
	}
	if got, want := r.Envelope.Scope, r.Trigger.Scope; got != want {
		return fmt.Errorf(
			"%s.scope must match %s.scope: %q != %q",
			nestedPath(path, "envelope"),
			nestedPath(path, "trigger"),
			got,
			want,
		)
	}
	if got, want := strings.TrimSpace(r.Envelope.WorkspaceID), strings.TrimSpace(r.Trigger.WorkspaceID); got != want {
		return fmt.Errorf(
			"%s.workspace_id must match %s.workspace_id: %q != %q",
			nestedPath(path, "envelope"),
			nestedPath(path, "trigger"),
			got,
			want,
		)
	}

	return nil
}

// SessionCreator is the subset of session.Manager needed by the dispatcher.
type SessionCreator interface {
	Create(ctx context.Context, opts session.CreateOpts) (*session.Session, error)
	Prompt(ctx context.Context, id string, msg string) (<-chan acp.AgentEvent, error)
	StopWithCause(ctx context.Context, id string, cause session.StopCause, detail string) error
}

// RunStore persists automation run state and restart-safe fire-limit inputs.
type RunStore interface {
	CreateRun(ctx context.Context, run Run) (Run, error)
	UpdateRun(ctx context.Context, run Run) (Run, error)
	CountRuns(ctx context.Context, query RunQuery) (int64, error)
	ListRuns(ctx context.Context, query RunQuery) ([]Run, error)
}

// TaskService exposes the minimal task-domain surface used by task-backed
// automation jobs.
type TaskService interface {
	CreateTask(ctx context.Context, spec taskpkg.CreateTask, actor taskpkg.ActorContext) (*taskpkg.Task, error)
	EnqueueRun(ctx context.Context, spec taskpkg.EnqueueRun, actor taskpkg.ActorContext) (*taskpkg.Run, error)
}

// AutomationSessionTaskActorRecorder stores trusted task-domain provenance for
// automation-launched sessions that may later create tasks explicitly.
type SessionTaskActorRecorder interface {
	RecordAutomationSessionTaskActor(sessionID string, actor taskpkg.ActorContext) error
	DeleteAutomationSessionTaskActor(sessionID string)
}

// HookDispatcher emits automation lifecycle hooks around shared dispatch.
type HookDispatcher interface {
	DispatchAutomationJobPreFire(
		ctx context.Context,
		payload hookspkg.AutomationJobPreFirePayload,
	) (hookspkg.AutomationJobPreFirePayload, error)
	DispatchAutomationJobPostFire(
		ctx context.Context,
		payload hookspkg.AutomationJobPostFirePayload,
	) (hookspkg.AutomationJobPostFirePayload, error)
	DispatchAutomationTriggerPreFire(
		ctx context.Context,
		payload hookspkg.AutomationTriggerPreFirePayload,
	) (hookspkg.AutomationTriggerPreFirePayload, error)
	DispatchAutomationTriggerPostFire(
		ctx context.Context,
		payload hookspkg.AutomationTriggerPostFirePayload,
	) (hookspkg.AutomationTriggerPostFirePayload, error)
	DispatchAutomationRunCompleted(
		ctx context.Context,
		payload hookspkg.AutomationRunCompletedPayload,
	) (hookspkg.AutomationRunCompletedPayload, error)
	DispatchAutomationRunFailed(
		ctx context.Context,
		payload hookspkg.AutomationRunFailedPayload,
	) (hookspkg.AutomationRunFailedPayload, error)
}

// SleepFunc waits for retry backoff with context cancellation support.
type SleepFunc func(ctx context.Context, delay time.Duration) error

// DispatcherOption customizes shared automation dispatch behavior.
type DispatcherOption func(*Dispatcher)

// Dispatcher routes every automation activation through one execution path.
type Dispatcher struct {
	sessions    SessionCreator
	runs        RunStore
	tasks       TaskService
	loopStarter LoopStarter

	logger              *slog.Logger
	now                 func() time.Time
	sleep               SleepFunc
	globalWorkspacePath string
	maxConcurrent       int
	sessionStopTimeout  time.Duration
	hooks               HookDispatcher
	taskActors          SessionTaskActorRecorder

	fireLimitMu sync.Mutex
	gate        chan struct{}
}

// NewDispatcher constructs a shared automation dispatcher.
func NewDispatcher(sessions SessionCreator, runs RunStore, opts ...DispatcherOption) (*Dispatcher, error) {
	if sessions == nil {
		return nil, errors.New("automation: session creator is required")
	}
	if runs == nil {
		return nil, errors.New("automation: run store is required")
	}

	dispatcher := &Dispatcher{
		sessions:           sessions,
		runs:               runs,
		logger:             slog.Default(),
		now:                func() time.Time { return time.Now().UTC() },
		sleep:              sleepWithContext,
		maxConcurrent:      DefaultMaxConcurrentJobs,
		sessionStopTimeout: defaultDispatcherSessionStopTimeout,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(dispatcher)
		}
	}

	if dispatcher.logger == nil {
		dispatcher.logger = slog.Default()
	}
	if dispatcher.now == nil {
		dispatcher.now = func() time.Time { return time.Now().UTC() }
	}
	if dispatcher.sleep == nil {
		dispatcher.sleep = sleepWithContext
	}
	if strings.TrimSpace(dispatcher.globalWorkspacePath) == "" {
		return nil, errors.New("automation: global workspace path is required")
	}
	if dispatcher.maxConcurrent <= 0 {
		dispatcher.maxConcurrent = DefaultMaxConcurrentJobs
	}
	if dispatcher.sessionStopTimeout <= 0 {
		dispatcher.sessionStopTimeout = defaultDispatcherSessionStopTimeout
	}
	dispatcher.gate = make(chan struct{}, dispatcher.maxConcurrent)

	return dispatcher, nil
}

// WithDispatcherLogger overrides the dispatcher logger.
func WithDispatcherLogger(logger *slog.Logger) DispatcherOption {
	return func(dispatcher *Dispatcher) {
		dispatcher.logger = logger
	}
}

// WithDispatcherNow overrides the dispatcher clock.
func WithDispatcherNow(now func() time.Time) DispatcherOption {
	return func(dispatcher *Dispatcher) {
		dispatcher.now = now
	}
}

// WithDispatcherSleep overrides retry waiting, mainly for tests.
func WithDispatcherSleep(sleep SleepFunc) DispatcherOption {
	return func(dispatcher *Dispatcher) {
		dispatcher.sleep = sleep
	}
}

// WithDispatcherGlobalWorkspacePath overrides the fallback path used for global automations.
func WithDispatcherGlobalWorkspacePath(path string) DispatcherOption {
	return func(dispatcher *Dispatcher) {
		dispatcher.globalWorkspacePath = strings.TrimSpace(path)
	}
}

// WithDispatcherMaxConcurrent overrides the shared automation concurrency gate.
func WithDispatcherMaxConcurrent(limit int) DispatcherOption {
	return func(dispatcher *Dispatcher) {
		dispatcher.maxConcurrent = limit
	}
}

// WithDispatcherSessionStopTimeout overrides the automation session stop budget.
func WithDispatcherSessionStopTimeout(timeout time.Duration) DispatcherOption {
	return func(dispatcher *Dispatcher) {
		dispatcher.sessionStopTimeout = timeout
	}
}

// WithDispatcherHooks injects the automation lifecycle hook dispatcher.
func WithDispatcherHooks(hooks HookDispatcher) DispatcherOption {
	return func(dispatcher *Dispatcher) {
		dispatcher.hooks = hooks
	}
}

// WithDispatcherTasks injects the task-domain service used for direct
// task-backed automation jobs.
func WithDispatcherTasks(tasks TaskService) DispatcherOption {
	return func(dispatcher *Dispatcher) {
		dispatcher.tasks = tasks
	}
}

// WithDispatcherTaskActorRecorder injects the session provenance recorder used
// to support automation-linked agent task creation.
func WithDispatcherTaskActorRecorder(recorder SessionTaskActorRecorder) DispatcherOption {
	return func(dispatcher *Dispatcher) {
		dispatcher.taskActors = recorder
	}
}

// Dispatch executes one automation request through the shared governance path.
func (d *Dispatcher) Dispatch(ctx context.Context, req DispatchRequest) (*Run, error) {
	if ctx == nil {
		return nil, errors.New("automation: dispatch context is required")
	}
	if err := req.Validate("dispatch"); err != nil {
		return nil, err
	}

	attempt := 1
	reservedDispatch := req.ReservedRun != nil
	var lastRun *Run
	for {
		run, err := d.dispatchAttempt(ctx, req, attempt)
		if run != nil {
			lastRun = cloneRun(run)
			if reservedDispatch {
				req.ReservedRun = cloneRun(run)
			}
		}
		if run != nil && d.hooks != nil {
			willRetry := err != nil && shouldRetry(req.retryConfig(), run, attempt, err)
			d.emitRunLifecycleHooks(ctx, req, *run, err, willRetry)
		}
		if err == nil {
			return lastRun, nil
		}
		if !shouldRetry(req.retryConfig(), run, attempt, err) {
			return lastRun, err
		}

		delay, delayErr := retryDelay(req.retryConfig(), attempt)
		if delayErr != nil {
			return lastRun, errors.Join(err, delayErr)
		}
		nextAttempt := attempt + 1
		d.logger.Info(
			"automation.dispatch.retry_scheduled",
			"run_id", run.ID,
			"job_id", strings.TrimSpace(run.JobID),
			"trigger_id", strings.TrimSpace(run.TriggerID),
			"attempt", nextAttempt,
			"delay", delay.String(),
		)
		if sleepErr := d.sleep(ctx, delay); sleepErr != nil {
			return lastRun, errors.Join(err, sleepErr)
		}
		attempt = nextAttempt
	}
}
