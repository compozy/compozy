package loop

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/deadentity"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
)

func TestTargetHealthShouldFailFastThroughTheNormalFailureChain(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	health := newRecordingTargetHealth()
	health.decision = deadentity.ProbeDecision{
		Allowed: false,
		Dead:    true,
		Reason:  "five consecutive transport failures",
	}
	runner := &CoordinatorRunner{targetHealth: health}
	node := dsl.Node{ID: "search", Class: dsl.NodeClassAction, Kind: "compozy__search"}

	err := runner.admitActionTarget(ctx, "taskrun-open-target", node, ActionExecutionInput{
		WorkspaceID: "ws-target",
	})
	if err == nil {
		t.Fatal("admitActionTarget() error = nil, want fail-fast denial")
	}
	var safeFailure SafeActionFailureProvider
	if !errors.As(err, &safeFailure) {
		t.Fatalf("admitActionTarget() error = %T, want SafeActionFailureProvider", err)
	}
	failure := safeFailure.SafeActionFailure()
	if failure.Code != targetUnavailableReasonCode {
		t.Fatalf("failure code = %q, want %q", failure.Code, targetUnavailableReasonCode)
	}
	ref, ok := ActionFailureOutputRef(failure)
	if !ok {
		t.Fatalf("ActionFailureOutputRef(%#v) = false", failure)
	}
	classified := classifyGenerationOutputFailure(
		GenerationOutput{Status: generationOutputFailed, OutputRef: ref},
		task.Run{},
	)
	if classified.Class != FailureTargetUnavailable || classified.RetryEligible {
		t.Fatalf("classified failure = %#v, want non-retryable target_unavailable", classified)
	}
	terminal := failedOutputTerminal(GenerationOutput{Status: generationOutputFailed, OutputRef: ref})
	if terminal.Status != string(StatusFailed) || terminal.ReasonCode != targetUnavailableReasonCode ||
		terminal.ReasonCode == circuitBreakerReasonCode {
		t.Fatalf("target-unavailable terminal = %#v, want normal failed precedence", terminal)
	}
	if got, want := health.probedKeys(), []store.DeadEntityKey{{
		ProfileID:   store.DefaultProfileID,
		WorkspaceID: "ws-target",
		Kind:        store.DeadEntityKindLoopTarget,
		EntityID:    "toolcall:compozy__search",
	}}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("probe keys = %#v, want %#v", got, want)
	}
}

func TestTargetHealthShouldCountOnlyUnhandledTransportFailures(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	node := dsl.Node{ID: "search", Class: dsl.NodeClassAction, Kind: "compozy__search"}
	run := Run{ID: "looprun-target-accounting", WorkspaceID: "ws-target"}

	t.Run("Should record transport escalation as failure", func(t *testing.T) {
		t.Parallel()

		health := newRecordingTargetHealth()
		runner := &CoordinatorRunner{targetHealth: health}
		failure := &ClassifiedFailure{Class: FailureTransport, Cause: "backend unavailable"}
		events, err := runner.accountActionTarget(
			ctx, run, node, ActionExecutionInput{WorkspaceID: run.WorkspaceID},
			task.Run{ID: "taskrun-transport"}, failure, AttemptEscalated,
		)
		if err != nil {
			t.Fatalf("accountActionTarget() error = %v", err)
		}
		if got := health.failureCount(); got != 1 {
			t.Fatalf("failure calls = %d, want 1", got)
		}
		if got := health.successCount(); got != 0 {
			t.Fatalf("success calls = %d, want 0", got)
		}
		if len(events) != 1 || events[0].BreakerState != "open" {
			t.Fatalf("events = %#v, want open transition", events)
		}
	})

	for _, tc := range []struct {
		name        string
		failure     *ClassifiedFailure
		disposition AttemptDisposition
	}{
		{
			name:        "Should record semantic failure as success",
			failure:     &ClassifiedFailure{Class: FailurePayloadDeclared},
			disposition: AttemptEscalated,
		},
		{
			name:        "Should record routed transport failure as success",
			failure:     &ClassifiedFailure{Class: FailureTransport},
			disposition: AttemptRouted,
		},
		{
			name:        "Should record absorbed transport failure as success",
			failure:     &ClassifiedFailure{Class: FailureTransport},
			disposition: AttemptAbsorbed,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			health := newRecordingTargetHealth()
			runner := &CoordinatorRunner{targetHealth: health}
			if _, err := runner.accountActionTarget(
				ctx, run, node, ActionExecutionInput{WorkspaceID: run.WorkspaceID},
				task.Run{ID: tc.name}, tc.failure, tc.disposition,
			); err != nil {
				t.Fatalf("accountActionTarget() error = %v", err)
			}
			if got := health.failureCount(); got != 0 {
				t.Fatalf("failure calls = %d, want 0", got)
			}
			if got := health.successCount(); got != 1 {
				t.Fatalf("success calls = %d, want 1", got)
			}
		})
	}
}

func TestTargetHealthShouldRetainOnlyAllowedBoundProbes(t *testing.T) {
	t.Parallel()

	t.Run("Should not retain a denied target probe", func(t *testing.T) {
		t.Parallel()

		health := newRecordingTargetHealth()
		health.decision = deadentity.ProbeDecision{Allowed: false, Dead: true}
		runner := &CoordinatorRunner{targetHealth: health}
		node := dsl.Node{ID: "search", Class: dsl.NodeClassAction, Kind: "compozy__search"}
		if err := runner.admitActionTarget(t.Context(), "taskrun-denied", node, ActionExecutionInput{
			WorkspaceID: "ws-target",
		}); err == nil {
			t.Fatal("admitActionTarget() error = nil, want denied target")
		}
		if _, found := runner.takeTargetProbe("taskrun-denied"); found {
			t.Fatal("denied target probe was retained")
		}
	})

	t.Run("Should reject a blank task run id before probing", func(t *testing.T) {
		t.Parallel()

		health := newRecordingTargetHealth()
		runner := &CoordinatorRunner{targetHealth: health}
		node := dsl.Node{ID: "search", Class: dsl.NodeClassAction, Kind: "compozy__search"}
		if err := runner.admitActionTarget(t.Context(), " ", node, ActionExecutionInput{
			WorkspaceID: "ws-target",
		}); !errors.Is(err, ErrValidation) {
			t.Fatalf("admitActionTarget(blank id) error = %v, want ErrValidation", err)
		}
		if got := health.probedKeys(); len(got) != 0 {
			t.Fatalf("probe keys = %#v, want no target probe", got)
		}
	})
}

func TestTargetHealthShouldReuseRenderedIdentityWithoutProbeMemory(t *testing.T) {
	t.Parallel()

	t.Run("Should reconstruct the rendered target after probe memory is cleared", func(t *testing.T) {
		t.Parallel()

		health := newRecordingTargetHealth()
		runner := &CoordinatorRunner{targetHealth: health}
		run := Run{ID: "looprun-rendered-target", WorkspaceID: "ws-target"}
		node := dsl.Node{
			ID: "agent", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent),
			Params: dsl.NodeParams{"agent": "{{ .inputs.agent }}", "prompt": "work"},
		}
		input := ActionExecutionInput{
			WorkspaceID: run.WorkspaceID,
			Namespace:   map[string]any{"inputs": map[string]any{"agent": "planner"}},
		}
		const taskRunID = "taskrun-rendered-target"
		if err := runner.admitActionTarget(t.Context(), taskRunID, node, input); err != nil {
			t.Fatalf("admitActionTarget() error = %v", err)
		}
		runner.discardTargetProbe(taskRunID)
		target, err := runner.actionTargetFor(taskRunID, run, node, input)
		if err != nil {
			t.Fatalf("actionTargetFor() error = %v", err)
		}
		if target != "planner" {
			t.Fatalf("action target = %q, want planner", target)
		}
		failure := &ClassifiedFailure{Class: FailureTransport, Cause: "transport unavailable"}
		if _, err := runner.accountActionTarget(
			t.Context(), run, node, input, task.Run{ID: taskRunID}, failure, AttemptEscalated,
		); err != nil {
			t.Fatalf("accountActionTarget() error = %v", err)
		}
		probes := health.probedKeys()
		failures := health.failureKeys()
		if len(probes) != 1 || len(failures) != 1 || probes[0] != failures[0] ||
			failures[0].EntityID != "run-agent:planner" {
			t.Fatalf("probe/failure keys = %#v/%#v, want identical rendered identity", probes, failures)
		}
	})
}

func TestTargetHealthShouldEmitHalfOpenOutcomeTransitions(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	node := dsl.Node{ID: "search", Class: dsl.NodeClassAction, Kind: "compozy__search"}
	run := Run{ID: "looprun-target-probe", WorkspaceID: "ws-target"}

	for _, tc := range []struct {
		name        string
		failure     *ClassifiedFailure
		disposition AttemptDisposition
		wantStates  []string
	}{
		{
			name:        "Should reopen after a failed recovery probe",
			failure:     &ClassifiedFailure{Class: FailureTransport},
			disposition: AttemptEscalated,
			wantStates:  []string{"half_open", "open"},
		},
		{
			name:        "Should close after a successful recovery probe",
			failure:     nil,
			disposition: AttemptSucceeded,
			wantStates:  []string{"half_open", "closed"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			health := newRecordingTargetHealth()
			health.dead = true
			health.decision = deadentity.ProbeDecision{Allowed: true, Recovery: true, Dead: true}
			runner := &CoordinatorRunner{targetHealth: health}
			taskRunID := "taskrun-" + tc.name
			if err := runner.admitActionTarget(ctx, taskRunID, node, ActionExecutionInput{
				WorkspaceID: run.WorkspaceID,
			}); err != nil {
				t.Fatalf("admitActionTarget() error = %v", err)
			}
			events, err := runner.accountActionTarget(
				ctx, run, node, ActionExecutionInput{WorkspaceID: run.WorkspaceID},
				task.Run{ID: taskRunID}, tc.failure, tc.disposition,
			)
			if err != nil {
				t.Fatalf("accountActionTarget() error = %v", err)
			}
			if got := breakerStates(events); !equalStrings(got, tc.wantStates) {
				t.Fatalf("breaker states = %#v, want %#v", got, tc.wantStates)
			}
		})
	}
}

type recordingTargetHealth struct {
	mu        sync.Mutex
	decision  deadentity.ProbeDecision
	dead      bool
	probes    []store.DeadEntityKey
	failures  []store.DeadEntityKey
	successes []store.DeadEntityKey
}

func newRecordingTargetHealth() *recordingTargetHealth {
	return &recordingTargetHealth{decision: deadentity.ProbeDecision{Allowed: true}}
}

func (h *recordingTargetHealth) BeforeProbe(
	_ context.Context,
	key store.DeadEntityKey,
) (deadentity.ProbeDecision, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.probes = append(h.probes, key)
	return h.decision, nil
}

func (h *recordingTargetHealth) RecordFailure(
	_ context.Context,
	key store.DeadEntityKey,
	_ deadentity.FailureClass,
	_ string,
) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.failures = append(h.failures, key)
	h.dead = true
	return nil
}

func (h *recordingTargetHealth) RecordSuccess(_ context.Context, key store.DeadEntityKey) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.successes = append(h.successes, key)
	h.dead = false
	return nil
}

func (h *recordingTargetHealth) failureKeys() []store.DeadEntityKey {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]store.DeadEntityKey(nil), h.failures...)
}

func (h *recordingTargetHealth) Status(context.Context, store.DeadEntityKey) (deadentity.Status, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return deadentity.Status{Dead: h.dead, MarkedAt: time.Now().UTC()}, nil
}

func (h *recordingTargetHealth) probedKeys() []store.DeadEntityKey {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]store.DeadEntityKey(nil), h.probes...)
}

func (h *recordingTargetHealth) failureCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.failures)
}

func (h *recordingTargetHealth) successCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.successes)
}

func breakerStates(events []GenerationLifecycleEventIntent) []string {
	states := make([]string, 0, len(events))
	for _, event := range events {
		states = append(states, event.BreakerState)
	}
	return states
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
