package recovery

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/workspace"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestRecoveryFSMModelsRequiredTransitions(t *testing.T) {
	t.Parallel()

	machine := newRecoveryFSM()
	if got := machine.Current(); got != string(recoveryStateExecuting) {
		t.Fatalf("initial state = %q, want %q", got, recoveryStateExecuting)
	}

	transitions := []struct {
		event string
		want  recoveryState
	}{
		{event: recoveryEventFail, want: recoveryStateFailed},
		{event: recoveryEventTriage, want: recoveryStateTriaging},
		{event: recoveryEventRemediate, want: recoveryStateRemediating},
		{event: recoveryEventRestart, want: recoveryStateRestarting},
		{event: recoveryEventFail, want: recoveryStateFailed},
		{event: recoveryEventTriage, want: recoveryStateTriaging},
		{event: recoveryEventRemediate, want: recoveryStateRemediating},
		{event: recoveryEventRestart, want: recoveryStateRestarting},
		{event: recoveryEventRecover, want: recoveryStateRecovered},
	}
	for _, transition := range transitions {
		if err := machine.Event(context.Background(), transition.event); err != nil {
			t.Fatalf("Event(%q) error = %v", transition.event, err)
		}
		if got := machine.Current(); got != string(transition.want) {
			t.Fatalf("after %q state = %q, want %q", transition.event, got, transition.want)
		}
	}
}

func TestRunRecoveryOrchestratorFixedVerdictRestartsFailedJobsAndRecovers(t *testing.T) {
	t.Parallel()

	originalErr := errors.New("unit tests failed")
	prepared := &fakePreparedRun{
		executeOutcome: failedRunOutcome("failed-run", "unit-tests", "assertion failed"),
		executeErr:     originalErr,
		restartOutcomes: []RunOutcome{
			succeededRunOutcome("restart-run"),
		},
	}
	strategy := &fakeRemediationStrategy{
		verdicts: []TriageVerdict{{Decision: VerdictFixed, Reason: "fixed assertion"}},
	}
	sink := &recordingEventSink{}

	got, err := newTestOrchestrator(strategy, enabledRecoveryConfig(1), &model.RuntimeConfig{}, sink).
		Run(context.Background(), prepared)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got.Status != StatusSucceeded {
		t.Fatalf("status = %q, want %q", got.Status, StatusSucceeded)
	}
	if !reflect.DeepEqual(prepared.restartCalls, [][]string{{"unit-tests"}}) {
		t.Fatalf("RestartFailed calls = %#v", prepared.restartCalls)
	}
	if len(strategy.inputs) != 1 {
		t.Fatalf("strategy calls = %d, want 1", len(strategy.inputs))
	}
	assertEventKinds(t, sink, []eventspkg.EventKind{
		eventspkg.EventKindRunRecoveryStarted,
		eventspkg.EventKindRunRecoveryRestarting,
		eventspkg.EventKindRunRecovered,
	})
	var started kinds.RunRecoveryStartedPayload
	decodeEventPayload(t, sink.events[0], &started)
	if started.Attempt != 1 || started.Strategy != fakeRemediationStrategyName {
		t.Fatalf("started payload = %#v", started)
	}
	var restarting kinds.RunRecoveryRestartingPayload
	decodeEventPayload(t, sink.events[1], &restarting)
	if !reflect.DeepEqual(restarting.FailedJobIDs, []string{"unit-tests"}) {
		t.Fatalf("restarting failed jobs = %#v", restarting.FailedJobIDs)
	}
	var recovered kinds.RunRecoveredPayload
	decodeEventPayload(t, sink.events[2], &recovered)
	if recovered.Attempts != 1 {
		t.Fatalf("recovered attempts = %d, want 1", recovered.Attempts)
	}
}

func TestRunRecoveryOrchestratorRejectVerdictExhaustsWithOriginalCause(t *testing.T) {
	t.Parallel()

	originalErr := errors.New("original failure")
	prepared := &fakePreparedRun{
		executeOutcome: failedRunOutcome("failed-run", "build", "compiler failed"),
		executeErr:     originalErr,
	}
	strategy := &fakeRemediationStrategy{
		verdicts: []TriageVerdict{{Decision: VerdictReject, Reason: "missing credentials"}},
	}
	sink := &recordingEventSink{}

	got, err := newTestOrchestrator(strategy, enabledRecoveryConfig(1), &model.RuntimeConfig{}, sink).
		Run(context.Background(), prepared)
	if !errors.Is(err, originalErr) {
		t.Fatalf("Run() error = %v, want original failure", err)
	}
	if got.Status != StatusFailed {
		t.Fatalf("status = %q, want %q", got.Status, StatusFailed)
	}
	if len(prepared.restartCalls) != 0 {
		t.Fatalf("RestartFailed calls = %#v, want none", prepared.restartCalls)
	}
	assertEventKinds(t, sink, []eventspkg.EventKind{
		eventspkg.EventKindRunRecoveryStarted,
		eventspkg.EventKindRunRecoveryExhausted,
	})
	var exhausted kinds.RunRecoveryExhaustedPayload
	decodeEventPayload(t, sink.events[1], &exhausted)
	if exhausted.Error != originalErr.Error() {
		t.Fatalf("exhausted error = %q, want %q", exhausted.Error, originalErr.Error())
	}
}

func TestRunRecoveryOrchestratorSkipsCanceledOutcome(t *testing.T) {
	t.Parallel()

	prepared := &fakePreparedRun{
		executeOutcome: RunOutcome{
			RunID:  "canceled-run",
			Status: StatusCanceled,
			Jobs: []JobOutcome{
				{SafeName: "unit-tests", Status: StatusSucceeded, ExitCode: 0},
			},
		},
	}
	strategy := &fakeRemediationStrategy{
		verdicts: []TriageVerdict{{Decision: VerdictFixed}},
	}
	sink := &recordingEventSink{}

	got, err := newTestOrchestrator(strategy, enabledRecoveryConfig(1), &model.RuntimeConfig{}, sink).
		Run(context.Background(), prepared)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got.Status != StatusCanceled {
		t.Fatalf("status = %q, want %q", got.Status, StatusCanceled)
	}
	if len(strategy.inputs) != 0 {
		t.Fatalf("strategy calls = %d, want 0", len(strategy.inputs))
	}
	if len(sink.events) != 0 {
		t.Fatalf("events = %#v, want none", sink.events)
	}
}

func TestRunRecoveryOrchestratorStopsBeforeRestartWhenContextCancelsDuringRemediation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	originalErr := errors.New("initial failure")
	prepared := &fakePreparedRun{
		executeOutcome: failedRunOutcome("failed-run", "unit-tests", "assertion failed"),
		executeErr:     originalErr,
		restartOutcomes: []RunOutcome{
			succeededRunOutcome("restart-run"),
		},
	}
	strategy := &cancelingRemediationStrategy{cancel: cancel}
	sink := &recordingEventSink{}

	got, err := newTestOrchestrator(strategy, enabledRecoveryConfig(1), &model.RuntimeConfig{}, sink).
		Run(ctx, prepared)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context canceled", err)
	}
	if strings.Contains(err.Error(), "fsm transition") {
		t.Fatalf("Run() error = %v, want cancellation without fsm transition failure", err)
	}
	if got.Status != StatusFailed {
		t.Fatalf("status = %q, want original failed outcome", got.Status)
	}
	if len(prepared.restartCalls) != 0 {
		t.Fatalf("RestartFailed calls = %#v, want none after cancellation", prepared.restartCalls)
	}
	assertEventKinds(t, sink, []eventspkg.EventKind{
		eventspkg.EventKindRunRecoveryStarted,
	})
}

func TestRunRecoveryOrchestratorRecordsCanceledRestartWithoutFSMError(t *testing.T) {
	t.Parallel()

	prepared := &fakePreparedRun{
		executeOutcome: failedRunOutcome("failed-run", "unit-tests", "assertion failed"),
		executeErr:     errors.New("initial failure"),
		restartOutcomes: []RunOutcome{{
			RunID:  "restart-run",
			Status: StatusCanceled,
			Jobs: []JobOutcome{
				{SafeName: "unit-tests", Status: StatusCanceled, ExitCode: 1, Error: context.Canceled.Error()},
			},
		}},
		restartErrs: []error{context.Canceled},
	}
	strategy := &fakeRemediationStrategy{
		verdicts: []TriageVerdict{{Decision: VerdictFixed, Reason: "patched"}},
	}
	sink := &recordingEventSink{}

	got, err := newTestOrchestrator(strategy, enabledRecoveryConfig(1), &model.RuntimeConfig{}, sink).
		Run(context.Background(), prepared)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context canceled", err)
	}
	if strings.Contains(err.Error(), "fsm transition") {
		t.Fatalf("Run() error = %v, want cancellation without fsm transition failure", err)
	}
	if got.Status != StatusCanceled {
		t.Fatalf("status = %q, want canceled", got.Status)
	}
	if !reflect.DeepEqual(prepared.restartCalls, [][]string{{"unit-tests"}}) {
		t.Fatalf("RestartFailed calls = %#v", prepared.restartCalls)
	}
	assertEventKinds(t, sink, []eventspkg.EventKind{
		eventspkg.EventKindRunRecoveryStarted,
		eventspkg.EventKindRunRecoveryRestarting,
	})
}

func TestRunRecoveryOrchestratorEnforcesRecoveryAttemptLoopGuard(t *testing.T) {
	t.Parallel()

	originalErr := errors.New("nested recovery failed")
	prepared := &fakePreparedRun{
		executeOutcome: failedRunOutcome("recovery-run", "unit-tests", "still failing"),
		executeErr:     originalErr,
	}
	strategy := &fakeRemediationStrategy{
		verdicts: []TriageVerdict{{Decision: VerdictFixed}},
	}
	sink := &recordingEventSink{}

	got, err := newTestOrchestrator(
		strategy,
		enabledRecoveryConfig(1),
		&model.RuntimeConfig{RecoveryAttempt: 1},
		sink,
	).Run(context.Background(), prepared)
	if !errors.Is(err, originalErr) {
		t.Fatalf("Run() error = %v, want nested recovery failure", err)
	}
	if got.Status != StatusFailed {
		t.Fatalf("status = %q, want %q", got.Status, StatusFailed)
	}
	if len(strategy.inputs) != 0 {
		t.Fatalf("strategy calls = %d, want 0", len(strategy.inputs))
	}
	if len(prepared.restartCalls) != 0 {
		t.Fatalf("RestartFailed calls = %#v, want none", prepared.restartCalls)
	}
	if len(sink.events) != 0 {
		t.Fatalf("events = %#v, want none", sink.events)
	}
}

func TestRunRecoveryOrchestratorSkipsDisabledRecovery(t *testing.T) {
	t.Parallel()

	prepared := &fakePreparedRun{
		executeOutcome: failedRunOutcome("failed-run", "unit-tests", "assertion failed"),
	}
	strategy := &fakeRemediationStrategy{
		verdicts: []TriageVerdict{{Decision: VerdictFixed}},
	}
	sink := &recordingEventSink{}

	got, err := newTestOrchestrator(strategy, workspace.AgentRecoveryConfig{}, &model.RuntimeConfig{}, sink).
		Run(context.Background(), prepared)
	if err == nil || err.Error() != "assertion failed" {
		t.Fatalf("Run() error = %v, want synthesized assertion failure", err)
	}
	if got.Status != StatusFailed {
		t.Fatalf("status = %q, want %q", got.Status, StatusFailed)
	}
	if len(strategy.inputs) != 0 {
		t.Fatalf("strategy calls = %d, want 0", len(strategy.inputs))
	}
	if len(sink.events) != 0 {
		t.Fatalf("events = %#v, want none", sink.events)
	}
}

func TestRunRecoveryOrchestratorRemediationErrorExhausts(t *testing.T) {
	t.Parallel()

	remediateErr := errors.New("recovery agent crashed")
	prepared := &fakePreparedRun{
		executeOutcome: failedRunOutcome("failed-run", "unit-tests", "assertion failed"),
	}
	strategy := &fakeRemediationStrategy{
		errs: []error{remediateErr},
	}
	sink := &recordingEventSink{}

	got, err := newTestOrchestrator(strategy, enabledRecoveryConfig(1), &model.RuntimeConfig{}, sink).
		Run(context.Background(), prepared)
	if !errors.Is(err, remediateErr) {
		t.Fatalf("Run() error = %v, want remediation error", err)
	}
	if got.Status != StatusFailed {
		t.Fatalf("status = %q, want %q", got.Status, StatusFailed)
	}
	if len(prepared.restartCalls) != 0 {
		t.Fatalf("RestartFailed calls = %#v, want none", prepared.restartCalls)
	}
	assertEventKinds(t, sink, []eventspkg.EventKind{
		eventspkg.EventKindRunRecoveryStarted,
		eventspkg.EventKindRunRecoveryExhausted,
	})
	var exhausted kinds.RunRecoveryExhaustedPayload
	decodeEventPayload(t, sink.events[1], &exhausted)
	if exhausted.Error == "" {
		t.Fatal("expected exhausted event to carry an error")
	}
}

func TestRunRecoveryOrchestratorUsesEventSinkFuncAndLoggerOption(t *testing.T) {
	t.Parallel()

	prepared := &fakePreparedRun{
		executeOutcome: failedRunOutcome("failed-run", "unit-tests", "assertion failed"),
		executeErr:     errors.New("initial failure"),
		restartOutcomes: []RunOutcome{
			succeededRunOutcome("failed-run"),
		},
	}
	strategy := &fakeRemediationStrategy{
		verdicts: []TriageVerdict{{Decision: VerdictFixed, Reason: "patched"}},
	}
	var gotEvents []eventspkg.Event
	sink := EventSinkFunc(func(_ context.Context, event eventspkg.Event) error {
		gotEvents = append(gotEvents, event)
		return nil
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	got, err := NewRunRecoveryOrchestrator(
		strategy,
		enabledRecoveryConfig(1),
		WithFailedRunConfig(&model.RuntimeConfig{}),
		WithRecoveryEventSink(sink),
		WithRecoveryLogger(logger),
	).Run(context.Background(), prepared)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got.Status != StatusSucceeded {
		t.Fatalf("status = %q, want %q", got.Status, StatusSucceeded)
	}
	if len(gotEvents) != 3 {
		t.Fatalf("events = %d, want 3", len(gotEvents))
	}
}

func TestRunRecoveryOrchestratorBoundsAttempts(t *testing.T) {
	t.Parallel()

	t.Run("MaxAttempts=1 exhausts after one failed restart", func(t *testing.T) {
		t.Parallel()

		restartErr := errors.New("restart still failed")
		prepared := &fakePreparedRun{
			executeOutcome: failedRunOutcome("failed-run", "unit-tests", "assertion failed"),
			executeErr:     errors.New("initial failure"),
			restartOutcomes: []RunOutcome{
				failedRunOutcome("failed-run", "unit-tests", "assertion still failed"),
			},
			restartErrs: []error{restartErr},
		}
		strategy := &fakeRemediationStrategy{
			verdicts: []TriageVerdict{{Decision: VerdictFixed, Reason: "patched"}},
		}
		sink := &recordingEventSink{}

		got, err := newTestOrchestrator(strategy, enabledRecoveryConfig(1), &model.RuntimeConfig{}, sink).
			Run(context.Background(), prepared)
		if !errors.Is(err, restartErr) {
			t.Fatalf("Run() error = %v, want restart failure", err)
		}
		if got.Status != StatusFailed {
			t.Fatalf("status = %q, want %q", got.Status, StatusFailed)
		}
		if len(strategy.inputs) != 1 || len(prepared.restartCalls) != 1 {
			t.Fatalf(
				"strategy calls = %d restarts = %d, want one each",
				len(strategy.inputs),
				len(prepared.restartCalls),
			)
		}
		assertEventKinds(t, sink, []eventspkg.EventKind{
			eventspkg.EventKindRunRecoveryStarted,
			eventspkg.EventKindRunRecoveryRestarting,
			eventspkg.EventKindRunRecoveryExhausted,
		})
	})

	t.Run("MaxAttempts=2 permits at most two cycles", func(t *testing.T) {
		t.Parallel()

		prepared := &fakePreparedRun{
			executeOutcome: failedRunOutcome("failed-run", "unit-tests", "assertion failed"),
			executeErr:     errors.New("initial failure"),
			restartOutcomes: []RunOutcome{
				failedRunOutcome("failed-run", "unit-tests", "assertion still failed"),
				succeededRunOutcome("failed-run"),
			},
			restartErrs: []error{errors.New("first restart failed"), nil},
		}
		strategy := &fakeRemediationStrategy{
			verdicts: []TriageVerdict{
				{Decision: VerdictFixed, Reason: "first patch"},
				{Decision: VerdictFixed, Reason: "second patch"},
			},
		}
		sink := &recordingEventSink{}

		got, err := newTestOrchestrator(strategy, enabledRecoveryConfig(2), &model.RuntimeConfig{}, sink).
			Run(context.Background(), prepared)
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if got.Status != StatusSucceeded {
			t.Fatalf("status = %q, want %q", got.Status, StatusSucceeded)
		}
		if len(strategy.inputs) != 2 || len(prepared.restartCalls) != 2 {
			t.Fatalf(
				"strategy calls = %d restarts = %d, want two each",
				len(strategy.inputs),
				len(prepared.restartCalls),
			)
		}
		assertEventKinds(t, sink, []eventspkg.EventKind{
			eventspkg.EventKindRunRecoveryStarted,
			eventspkg.EventKindRunRecoveryRestarting,
			eventspkg.EventKindRunRecoveryStarted,
			eventspkg.EventKindRunRecoveryRestarting,
			eventspkg.EventKindRunRecovered,
		})
	})
}

func TestRunRecoveryOrchestratorIntegrationDrivesPreparedRunAndStrategy(t *testing.T) {
	t.Parallel()

	fixed := false
	prepared := &statefulPreparedRun{fixed: &fixed}
	strategy := &statefulRemediationStrategy{fixed: &fixed}
	sink := &recordingEventSink{}

	got, err := newTestOrchestrator(strategy, enabledRecoveryConfig(1), &model.RuntimeConfig{}, sink).
		Run(context.Background(), prepared)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got.Status != StatusSucceeded {
		t.Fatalf("status = %q, want %q", got.Status, StatusSucceeded)
	}
	if !fixed {
		t.Fatal("strategy did not apply the fake fix")
	}
	if !reflect.DeepEqual(prepared.restartCalls, [][]string{{"unit-tests"}}) {
		t.Fatalf("RestartFailed calls = %#v", prepared.restartCalls)
	}
	assertEventKinds(t, sink, []eventspkg.EventKind{
		eventspkg.EventKindRunRecoveryStarted,
		eventspkg.EventKindRunRecoveryRestarting,
		eventspkg.EventKindRunRecovered,
	})
}

const fakeRemediationStrategyName = "fake-remediation"

type fakePreparedRun struct {
	executeOutcome  RunOutcome
	executeErr      error
	executeCalls    int
	restartOutcomes []RunOutcome
	restartErrs     []error
	restartCalls    [][]string
}

func (f *fakePreparedRun) Execute(context.Context) (RunOutcome, error) {
	f.executeCalls++
	return f.executeOutcome, f.executeErr
}

func (f *fakePreparedRun) RestartFailed(_ context.Context, failedJobIDs []string) (RunOutcome, error) {
	f.restartCalls = append(f.restartCalls, append([]string(nil), failedJobIDs...))
	idx := len(f.restartCalls) - 1
	if idx >= len(f.restartOutcomes) {
		return RunOutcome{}, errors.New("unexpected RestartFailed call")
	}
	var err error
	if idx < len(f.restartErrs) {
		err = f.restartErrs[idx]
	}
	return f.restartOutcomes[idx], err
}

type fakeRemediationStrategy struct {
	verdicts []TriageVerdict
	errs     []error
	inputs   []RemediationInput
}

type cancelingRemediationStrategy struct {
	cancel func()
}

func (*cancelingRemediationStrategy) Name() string {
	return "canceling-remediation"
}

func (s *cancelingRemediationStrategy) Remediate(context.Context, RemediationInput) (TriageVerdict, error) {
	if s.cancel != nil {
		s.cancel()
	}
	return TriageVerdict{Decision: VerdictFixed, Reason: "patched before cancellation"}, nil
}

func (*fakeRemediationStrategy) Name() string {
	return fakeRemediationStrategyName
}

func (f *fakeRemediationStrategy) Remediate(_ context.Context, in RemediationInput) (TriageVerdict, error) {
	input := in
	input.Outcome.Jobs = append([]JobOutcome(nil), in.Outcome.Jobs...)
	f.inputs = append(f.inputs, input)
	idx := len(f.inputs) - 1
	if idx < len(f.errs) && f.errs[idx] != nil {
		return TriageVerdict{}, f.errs[idx]
	}
	if idx < len(f.verdicts) {
		return f.verdicts[idx], nil
	}
	return TriageVerdict{Decision: VerdictReject, Reason: "unexpected remediation call"}, nil
}

type statefulPreparedRun struct {
	fixed        *bool
	restartCalls [][]string
}

func (*statefulPreparedRun) Execute(context.Context) (RunOutcome, error) {
	return failedRunOutcome("stateful-run", "unit-tests", "test failed"), errors.New("test failed")
}

func (s *statefulPreparedRun) RestartFailed(_ context.Context, failedJobIDs []string) (RunOutcome, error) {
	s.restartCalls = append(s.restartCalls, append([]string(nil), failedJobIDs...))
	if s.fixed != nil && *s.fixed {
		return succeededRunOutcome("stateful-run"), nil
	}
	return failedRunOutcome("stateful-run", "unit-tests", "test still failed"), errors.New("test still failed")
}

type statefulRemediationStrategy struct {
	fixed *bool
}

func (*statefulRemediationStrategy) Name() string {
	return "stateful-fake"
}

func (s *statefulRemediationStrategy) Remediate(context.Context, RemediationInput) (TriageVerdict, error) {
	if s.fixed != nil {
		*s.fixed = true
	}
	return TriageVerdict{Decision: VerdictFixed, Reason: "fake production fix"}, nil
}

type recordingEventSink struct {
	events []eventspkg.Event
}

func (s *recordingEventSink) Submit(_ context.Context, event eventspkg.Event) error {
	s.events = append(s.events, event)
	return nil
}

func newTestOrchestrator(
	strategy RemediationStrategy,
	cfg workspace.AgentRecoveryConfig,
	failedConfig *model.RuntimeConfig,
	sink EventSink,
) *RunRecoveryOrchestrator {
	return NewRunRecoveryOrchestrator(
		strategy,
		cfg,
		WithFailedRunConfig(failedConfig),
		WithRecoveryEventSink(sink),
	)
}

func enabledRecoveryConfig(maxAttempts int) workspace.AgentRecoveryConfig {
	return workspace.AgentRecoveryConfig{
		Enabled:     boolPtr(true),
		MaxAttempts: intPtr(maxAttempts),
	}
}

func failedRunOutcome(runID, failedJobID, jobError string) RunOutcome {
	return RunOutcome{
		RunID:      runID,
		Status:     StatusFailed,
		ResultPath: runID + "/result.json",
		Jobs: []JobOutcome{
			{SafeName: "already-green", Status: StatusSucceeded, ExitCode: 0},
			{SafeName: failedJobID, Status: StatusFailed, ExitCode: 1, Error: jobError},
		},
	}
}

func succeededRunOutcome(runID string) RunOutcome {
	return RunOutcome{
		RunID:  runID,
		Status: StatusSucceeded,
		Jobs: []JobOutcome{
			{SafeName: "unit-tests", Status: StatusSucceeded, ExitCode: 0},
		},
	}
}

func assertEventKinds(t *testing.T, sink *recordingEventSink, want []eventspkg.EventKind) {
	t.Helper()

	got := make([]eventspkg.EventKind, 0, len(sink.events))
	for _, event := range sink.events {
		got = append(got, event.Kind)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("event kinds = %#v, want %#v", got, want)
	}
}

func decodeEventPayload(t *testing.T, event eventspkg.Event, dst any) {
	t.Helper()

	if err := json.Unmarshal(event.Payload, dst); err != nil {
		t.Fatalf("decode %s payload: %v", event.Kind, err)
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func intPtr(value int) *int {
	return &value
}
