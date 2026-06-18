package recovery

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/looplab/fsm"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/internal/runtimeevents"
	"github.com/compozy/compozy/internal/core/workspace"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

var errNoFailedJobsToRestart = errors.New("recovery fixed verdict did not have failed jobs to restart")

// EventSink persists or publishes recovery lifecycle events.
type EventSink interface {
	Submit(ctx context.Context, event events.Event) error
}

// EventSinkFunc adapts a function to EventSink.
type EventSinkFunc func(context.Context, events.Event) error

// Submit sends one event through f.
func (f EventSinkFunc) Submit(ctx context.Context, event events.Event) error {
	if f == nil {
		return nil
	}
	return f(ctx, event)
}

// RunRecoveryOrchestrator drives the recovery FSM around one prepared run.
type RunRecoveryOrchestrator struct {
	strategy     RemediationStrategy
	recovery     workspace.AgentRecoveryConfig
	failedConfig *model.RuntimeConfig
	eventSink    EventSink
	log          *slog.Logger
}

// RunRecoveryOrchestratorOption configures RunRecoveryOrchestrator.
type RunRecoveryOrchestratorOption func(*RunRecoveryOrchestrator)

// WithFailedRunConfig supplies the runtime config of the original failed run.
func WithFailedRunConfig(cfg *model.RuntimeConfig) RunRecoveryOrchestratorOption {
	return func(o *RunRecoveryOrchestrator) {
		o.failedConfig = cfg
	}
}

// WithRecoveryEventSink supplies the destination for recovery lifecycle events.
func WithRecoveryEventSink(sink EventSink) RunRecoveryOrchestratorOption {
	return func(o *RunRecoveryOrchestrator) {
		o.eventSink = sink
	}
}

// WithRecoveryLogger supplies the logger used for FSM transition logs.
func WithRecoveryLogger(log *slog.Logger) RunRecoveryOrchestratorOption {
	return func(o *RunRecoveryOrchestrator) {
		if log != nil {
			o.log = log
		}
	}
}

// NewRunRecoveryOrchestrator constructs an FSM-backed run recovery orchestrator.
func NewRunRecoveryOrchestrator(
	strategy RemediationStrategy,
	recovery workspace.AgentRecoveryConfig,
	opts ...RunRecoveryOrchestratorOption,
) *RunRecoveryOrchestrator {
	if strategy == nil {
		strategy = NewAgenticRemediation()
	}
	o := &RunRecoveryOrchestrator{
		strategy: strategy,
		recovery: recovery.ApplyDefaults(),
		log:      slog.Default(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}
	return o
}

// Run executes the original run and, when gated in, attempts bounded recovery.
func (o *RunRecoveryOrchestrator) Run(ctx context.Context, run PreparedRun) (RunOutcome, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if run == nil {
		return RunOutcome{}, errors.New("run recovery: missing prepared run")
	}
	if o == nil {
		return RunOutcome{}, errors.New("run recovery: missing orchestrator")
	}

	machine := newRecoveryFSM()
	outcome, runErr := run.Execute(ctx)
	if err := o.classifyInitialOutcome(ctx, machine, outcome); err != nil {
		return outcome, errors.Join(runErr, err)
	}
	if outcome.Status != StatusFailed || outcome.Canceled() {
		return outcome, runErr
	}

	return o.recoverFailedRun(ctx, machine, run, outcome, ensureFailureError(outcome, runErr))
}

type recoveryLoopResult struct {
	outcome  RunOutcome
	attempts int
	lastErr  error
	done     bool
	err      error
}

func (o *RunRecoveryOrchestrator) recoverFailedRun(
	ctx context.Context,
	machine *fsm.FSM,
	run PreparedRun,
	originalOutcome RunOutcome,
	originalErr error,
) (RunOutcome, error) {
	outcome := originalOutcome
	attempts := 0
	lastErr := originalErr

	for o.shouldRecover(outcome, attempts) {
		result := o.runRecoveryAttempt(
			ctx,
			machine,
			run,
			outcome,
			attempts,
			lastErr,
			originalOutcome,
			originalErr,
		)
		if result.done {
			return result.outcome, result.err
		}
		outcome = result.outcome
		attempts = result.attempts
		lastErr = result.lastErr
	}

	return outcome, originalErr
}

func (o *RunRecoveryOrchestrator) runRecoveryAttempt(
	ctx context.Context,
	machine *fsm.FSM,
	run PreparedRun,
	outcome RunOutcome,
	attempts int,
	lastErr error,
	originalOutcome RunOutcome,
	originalErr error,
) recoveryLoopResult {
	if o.strategy == nil {
		cause := errors.Join(originalErr, errors.New("run recovery: missing remediation strategy"))
		return terminalRecoveryResult(
			originalOutcome,
			o.exhaustRecovery(ctx, machine, attempts, originalOutcome, cause, TriageVerdict{}),
		)
	}
	if err := ctx.Err(); err != nil {
		cancelErr := o.transition(ctx, machine, recoveryEventCancel, attempts, outcome, TriageVerdict{})
		return terminalRecoveryResult(outcome, errors.Join(lastErr, cancelErr, err))
	}

	attempt := attempts + 1
	if err := o.startRecoveryAttempt(ctx, machine, outcome, attempt); err != nil {
		return terminalRecoveryResult(originalOutcome, errors.Join(originalErr, err))
	}
	verdict, err := o.remediateRecoveryAttempt(ctx, machine, outcome, attempt, originalOutcome, originalErr)
	if err != nil {
		return terminalRecoveryResult(originalOutcome, err)
	}
	if verdict.Decision != VerdictFixed {
		return terminalRecoveryResult(
			originalOutcome,
			o.exhaustRecovery(ctx, machine, attempt, originalOutcome, originalErr, verdict),
		)
	}

	restartOutcome, restartErr, err := o.restartAfterFixedVerdict(ctx, machine, run, outcome, attempt, verdict)
	if err != nil {
		if errors.Is(err, errNoFailedJobsToRestart) {
			cause := errors.Join(originalErr, err)
			return terminalRecoveryResult(
				originalOutcome,
				o.exhaustRecovery(ctx, machine, attempt, originalOutcome, cause, verdict),
			)
		}
		return terminalRecoveryResult(originalOutcome, errors.Join(originalErr, err))
	}
	return o.handleRestartOutcome(ctx, machine, restartOutcome, restartErr, attempt, verdict)
}

func (o *RunRecoveryOrchestrator) startRecoveryAttempt(
	ctx context.Context,
	machine *fsm.FSM,
	outcome RunOutcome,
	attempt int,
) error {
	if err := o.emitRecoveryEvent(
		ctx,
		outcome,
		events.EventKindRunRecoveryStarted,
		kinds.RunRecoveryStartedPayload{Attempt: attempt, Strategy: o.strategy.Name()},
	); err != nil {
		return err
	}
	if err := o.transition(ctx, machine, recoveryEventTriage, attempt, outcome, TriageVerdict{}); err != nil {
		return err
	}
	return o.transition(ctx, machine, recoveryEventRemediate, attempt, outcome, TriageVerdict{})
}

func (o *RunRecoveryOrchestrator) remediateRecoveryAttempt(
	ctx context.Context,
	machine *fsm.FSM,
	outcome RunOutcome,
	attempt int,
	originalOutcome RunOutcome,
	originalErr error,
) (TriageVerdict, error) {
	verdict, remediateErr := o.strategy.Remediate(ctx, RemediationInput{
		Outcome:      outcome,
		FailedConfig: o.failedConfig,
		Recovery:     o.recovery,
	})
	if remediateErr == nil {
		return verdict, nil
	}
	cause := errors.Join(originalErr, fmt.Errorf("remediate recovery: %w", remediateErr))
	return verdict, o.exhaustRecovery(ctx, machine, attempt, originalOutcome, cause, verdict)
}

func (o *RunRecoveryOrchestrator) restartAfterFixedVerdict(
	ctx context.Context,
	machine *fsm.FSM,
	run PreparedRun,
	outcome RunOutcome,
	attempt int,
	verdict TriageVerdict,
) (RunOutcome, error, error) {
	failedJobIDs := outcome.FailedJobIDs()
	if len(failedJobIDs) == 0 {
		return RunOutcome{}, nil, errNoFailedJobsToRestart
	}
	if err := o.emitRecoveryEvent(
		ctx,
		outcome,
		events.EventKindRunRecoveryRestarting,
		kinds.RunRecoveryRestartingPayload{FailedJobIDs: failedJobIDs},
	); err != nil {
		return RunOutcome{}, nil, err
	}
	if err := o.transition(ctx, machine, recoveryEventRestart, attempt, outcome, verdict); err != nil {
		return RunOutcome{}, nil, err
	}
	restartOutcome, restartErr := run.RestartFailed(ctx, failedJobIDs)
	return restartOutcome, restartErr, nil
}

func (o *RunRecoveryOrchestrator) handleRestartOutcome(
	ctx context.Context,
	machine *fsm.FSM,
	outcome RunOutcome,
	restartErr error,
	attempts int,
	verdict TriageVerdict,
) recoveryLoopResult {
	lastErr := ensureFailureError(outcome, restartErr)
	switch {
	case outcome.Canceled():
		err := o.transition(ctx, machine, recoveryEventCancel, attempts, outcome, verdict)
		return terminalRecoveryResult(outcome, errors.Join(restartErr, err))
	case restartErr != nil && outcome.Status != StatusFailed:
		return terminalRecoveryResult(outcome, o.exhaustRecovery(ctx, machine, attempts, outcome, restartErr, verdict))
	case outcome.Status == StatusSucceeded:
		return o.recoveredResult(ctx, machine, outcome, attempts, verdict)
	case outcome.Status == StatusFailed:
		return o.failedRestartResult(ctx, machine, outcome, attempts, lastErr, verdict)
	default:
		statusErr := fmt.Errorf("recovery restart returned non-terminal status %q", outcome.Status)
		cause := errors.Join(lastErr, statusErr)
		return terminalRecoveryResult(outcome, o.exhaustRecovery(ctx, machine, attempts, outcome, cause, verdict))
	}
}

func (o *RunRecoveryOrchestrator) recoveredResult(
	ctx context.Context,
	machine *fsm.FSM,
	outcome RunOutcome,
	attempts int,
	verdict TriageVerdict,
) recoveryLoopResult {
	if err := o.transition(ctx, machine, recoveryEventRecover, attempts, outcome, verdict); err != nil {
		return terminalRecoveryResult(outcome, err)
	}
	if err := o.emitRecoveryEvent(
		ctx,
		outcome,
		events.EventKindRunRecovered,
		kinds.RunRecoveredPayload{Attempts: attempts},
	); err != nil {
		return terminalRecoveryResult(outcome, err)
	}
	return terminalRecoveryResult(outcome, nil)
}

func (o *RunRecoveryOrchestrator) failedRestartResult(
	ctx context.Context,
	machine *fsm.FSM,
	outcome RunOutcome,
	attempts int,
	lastErr error,
	verdict TriageVerdict,
) recoveryLoopResult {
	if err := o.transition(ctx, machine, recoveryEventFail, attempts, outcome, verdict); err != nil {
		return terminalRecoveryResult(outcome, errors.Join(lastErr, err))
	}
	if !o.shouldRecover(outcome, attempts) {
		return terminalRecoveryResult(outcome, o.exhaustRecovery(ctx, machine, attempts, outcome, lastErr, verdict))
	}
	return continueRecoveryResult(outcome, attempts, lastErr)
}

func terminalRecoveryResult(outcome RunOutcome, err error) recoveryLoopResult {
	return recoveryLoopResult{outcome: outcome, done: true, err: err}
}

func continueRecoveryResult(outcome RunOutcome, attempts int, lastErr error) recoveryLoopResult {
	return recoveryLoopResult{outcome: outcome, attempts: attempts, lastErr: lastErr}
}

func (o *RunRecoveryOrchestrator) classifyInitialOutcome(
	ctx context.Context,
	machine *fsm.FSM,
	outcome RunOutcome,
) error {
	switch {
	case outcome.Canceled():
		return o.transition(ctx, machine, recoveryEventCancel, 0, outcome, TriageVerdict{})
	case outcome.Status == StatusFailed:
		return o.transition(ctx, machine, recoveryEventFail, 0, outcome, TriageVerdict{})
	default:
		return o.transition(ctx, machine, recoveryEventRecover, 0, outcome, TriageVerdict{})
	}
}

func (o *RunRecoveryOrchestrator) shouldRecover(outcome RunOutcome, attempt int) bool {
	cfg := o.recovery.ApplyDefaults()
	if cfg.Enabled == nil || !*cfg.Enabled {
		return false
	}
	if recoveryAttempt(o.failedConfig) != 0 {
		return false
	}
	if outcome.Canceled() || outcome.Status != StatusFailed {
		return false
	}
	return attempt < maxRecoveryAttempts(cfg)
}

func recoveryAttempt(cfg *model.RuntimeConfig) int {
	if cfg == nil {
		return 0
	}
	return cfg.RecoveryAttempt
}

func maxRecoveryAttempts(cfg workspace.AgentRecoveryConfig) int {
	if cfg.MaxAttempts == nil {
		return workspace.DefaultRecoveryMaxAttempts
	}
	return *cfg.MaxAttempts
}

func (o *RunRecoveryOrchestrator) exhaustRecovery(
	ctx context.Context,
	machine *fsm.FSM,
	attempt int,
	outcome RunOutcome,
	cause error,
	verdict TriageVerdict,
) error {
	if err := o.transition(ctx, machine, recoveryEventExhaust, attempt, outcome, verdict); err != nil {
		cause = errors.Join(cause, err)
	}
	if err := o.emitRecoveryEvent(
		ctx,
		outcome,
		events.EventKindRunRecoveryExhausted,
		kinds.RunRecoveryExhaustedPayload{
			Error:      outcomeFailureMessage(outcome, cause),
			ResultPath: outcome.ResultPath,
		},
	); err != nil {
		cause = errors.Join(cause, err)
	}
	return cause
}

func (o *RunRecoveryOrchestrator) emitRecoveryEvent(
	ctx context.Context,
	outcome RunOutcome,
	kind events.EventKind,
	payload any,
) error {
	if o == nil || o.eventSink == nil {
		return nil
	}
	event, err := runtimeevents.NewRuntimeEvent(strings.TrimSpace(outcome.RunID), kind, payload)
	if err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return o.eventSink.Submit(ctx, event)
}

func (o *RunRecoveryOrchestrator) transition(
	ctx context.Context,
	machine *fsm.FSM,
	event string,
	attempt int,
	outcome RunOutcome,
	verdict TriageVerdict,
) error {
	if machine == nil {
		return errors.New("run recovery: missing fsm")
	}
	from := machine.Current()
	if err := machine.Event(ctx, event); err != nil {
		return fmt.Errorf("run recovery fsm transition %q from %q: %w", event, from, err)
	}
	to := machine.Current()
	if o != nil && o.log != nil {
		attrs := []any{
			"component", "run_recovery",
			"run_id", strings.TrimSpace(outcome.RunID),
			"from", from,
			"to", to,
			"event", event,
			"attempt", attempt,
		}
		if verdict.Decision != "" {
			attrs = append(attrs, "decision", string(verdict.Decision), "reason", verdict.Reason)
		}
		o.log.Debug("run recovery fsm transition", attrs...)
	}
	return nil
}

func ensureFailureError(outcome RunOutcome, err error) error {
	if err != nil {
		return err
	}
	if outcome.Status != StatusFailed {
		return nil
	}
	return errors.New(outcomeFailureMessage(outcome, nil))
}

func outcomeFailureMessage(outcome RunOutcome, err error) string {
	if err != nil {
		return strings.TrimSpace(err.Error())
	}
	for _, job := range outcome.Jobs {
		if job.Status != StatusFailed {
			continue
		}
		if msg := strings.TrimSpace(job.Error); msg != "" {
			return msg
		}
	}
	if resultPath := strings.TrimSpace(outcome.ResultPath); resultPath != "" {
		return "run failed; see result " + resultPath
	}
	if runID := strings.TrimSpace(outcome.RunID); runID != "" {
		return "run " + runID + " failed"
	}
	return "run failed"
}
