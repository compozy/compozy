package loop

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
)

func TestCoordinatorRunnerShouldApplyNodeFailurePrecedence(t *testing.T) {
	t.Parallel()

	t.Run("Should apply only the first matching autopause rule with durable provenance", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, time.August, 2, 18, 30, 0, 0, time.UTC)
		firstRule := "node_id == 'work' && class == 'transport' && attempt == 1"
		secondRule := "class == 'transport'"
		lifecycle := DefaultLifecycleConfig()
		lifecycle.Autopause = []LifecycleAutopauseRule{
			{Match: firstRule, Action: "pause"},
			{Match: secondRule, Action: "pause"},
		}
		plan := runLifecycleFailurePlanWithConfig(
			t,
			"autopause",
			now,
			lifecycleDefinition(lifecycleNode("work")),
			lifecycle,
		)
		payload := coordinatorSnapshotPayloadForTest(t, plan)

		if got, want := len(payload.Controls), 1; got != want {
			t.Fatalf("control mutations = %d, want %d", got, want)
		}
		control := payload.Controls[0]
		wantRuleID := autopauseRuleID(0, firstRule)
		if control.Kind != NodeControlMutationPause || control.NodeID != "work" ||
			control.PauseActorKind != autopauseActorKind || control.PauseActorID != autopauseActorID ||
			control.PauseRuleID != wantRuleID || !strings.Contains(control.PauseReason, "transport failure") {
			t.Fatalf("autopause control = %#v, want first-rule provenance", control)
		}
		if got, want := len(payload.Events), 1; got != want {
			t.Fatalf("lifecycle events = %d, want %d", got, want)
		}
		event := payload.Events[0]
		if event.Kind != GenerationLifecycleEventNodePaused || event.RuleID != wantRuleID ||
			event.ActorKind != autopauseActorKind || event.ActorID != autopauseActorID || event.Attempt != 1 {
			t.Fatalf("autopause event = %#v, want matching rule provenance", event)
		}
		output := payload.Outputs[0]
		if output.Status != generationOutputPaused || output.Attempt != 2 || output.Epoch != 1 ||
			!plan.GenerationInFlight || !plan.Yield || len(plan.NodeRuns) != 0 {
			t.Fatalf("autopause output = %#v plan = %#v", output, plan)
		}
	})

	t.Run("Should not refire autopause while the node remains paused", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, time.August, 2, 18, 45, 0, 0, time.UTC)
		lifecycle := DefaultLifecycleConfig()
		lifecycle.Autopause = []LifecycleAutopauseRule{{Match: "class == 'transport'", Action: "pause"}}
		control := NodeControl{
			LoopRunID: "looprun-lifecycle-autopause-held", NodeID: "work", Paused: true,
			PauseProvenance: &ControlProvenance{
				ActorKind: autopauseActorKind, ActorID: autopauseActorID,
				RuleID: autopauseRuleID(0, lifecycle.Autopause[0].Match), RequestedAt: now.Add(-time.Minute),
			},
			Revision: 1,
		}
		plan := runLifecycleFailurePlanWithConfigAndControls(
			t,
			"autopause-held",
			now,
			lifecycleDefinition(lifecycleNode("work")),
			lifecycle,
			[]NodeControl{control},
		)
		payload := coordinatorSnapshotPayloadForTest(t, plan)

		if len(payload.Controls) != 0 {
			t.Fatalf("control mutations = %#v, want no repeated autopause", payload.Controls)
		}
		for _, event := range payload.Events {
			if event.Kind == GenerationLifecycleEventNodePaused {
				t.Fatalf("events = %#v, want no repeated node_paused event", payload.Events)
			}
		}
	})

	t.Run("Should schedule and release a durable retry", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, time.August, 2, 19, 0, 0, 0, time.UTC)
		loopRun := controlLoopRun("looprun-lifecycle-retry", nil)
		coordinatorRun := controlCoordinatorRun(loopRun, 1)
		workerRun := lifecycleWorkerRun(loopRun, "work", task.TaskRunStatusFailed, now)
		failureRef := lifecycleFailureRef(t, string(toolsUnavailableCodeForTest), "provider unavailable")
		firstScheduledAt := now.Add(-time.Second)
		outputs := &lifecycleCoordinatorStore{coordinatorRunnerOutputs: coordinatorRunnerOutputs{
			outputs: map[int][]GenerationOutput{1: {{
				Generation:       1,
				NodeID:           "work",
				Status:           generationOutputFailed,
				OutputRef:        failureRef,
				TaskRunID:        workerRun.ID,
				Attempt:          1,
				FirstScheduledAt: &firstScheduledAt,
				Epoch:            3,
			}},
			}}}
		runner := newCoordinatorRunnerForTestWithDefinition(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{coordinatorRun.ID: coordinatorRun, workerRun.ID: workerRun},
			outputs,
			lifecycleDefinition(lifecycleNode("work")),
			WithCoordinatorNodeAttemptReader(outputs),
			WithCoordinatorRetryRand(func() float64 { return 0 }),
		)
		runner.now = func() time.Time { return now }

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		payload := coordinatorSnapshotPayloadForTest(t, plan)
		if got, want := len(payload.Attempts), 1; got != want {
			t.Fatalf("attempt intents = %d, want %d", got, want)
		}
		attempt := payload.Attempts[0]
		if attempt.Disposition != AttemptRetried || attempt.FailureClass == nil ||
			*attempt.FailureClass != FailureTransport || attempt.NextAttemptAt == nil {
			t.Fatalf("retry attempt = %#v, want classified retried disposition", attempt)
		}
		output := payload.Outputs[0]
		if output.Status != generationOutputRetrying || output.NextAttemptAt == nil || output.Epoch != 4 {
			t.Fatalf("retry output = %#v, want retrying epoch 4", output)
		}
		if !plan.GenerationInFlight || !plan.Yield || len(plan.NodeRuns) != 0 {
			t.Fatalf("retry plan = %#v, want yielded in-flight generation", plan)
		}
		if len(plan.PostCommitTimers) != 1 || plan.PostCommitTimers[0].IssuedEpoch != 4 ||
			plan.PostCommitTimers[0].IdempotencyKey != RetryWakeIdempotencyKey(RetryDueCell{
				LoopRunID: loopRun.ID, Generation: 1, NodeID: "work", Attempt: 1,
				Epoch: 4, NextAttemptAt: *output.NextAttemptAt,
			}) {
			t.Fatalf("retry timers = %#v, want shared epoch-fenced identity", plan.PostCommitTimers)
		}
		if len(payload.Events) != 1 || payload.Events[0].Kind != GenerationLifecycleEventNodeRetryScheduled ||
			payload.Events[0].Attempt != 2 || payload.Events[0].IssuedEpoch != 4 {
			t.Fatalf("retry events = %#v, want node_retry_scheduled", payload.Events)
		}

		dueOutput := output
		dueOutput.ExpectedEpoch = nil
		outputs.outputs[1] = []GenerationOutput{dueOutput}
		outputs.attempts = []NodeAttempt{attempt}
		dueCoordinatorRun := coordinatorRun
		dueCoordinatorRun.ID += "-due"
		dueRunner := newCoordinatorRunnerForTestWithDefinition(
			t,
			loopRun,
			dueCoordinatorRun,
			map[string]task.Run{dueCoordinatorRun.ID: dueCoordinatorRun, workerRun.ID: workerRun},
			outputs,
			lifecycleDefinition(lifecycleNode("work")),
			WithCoordinatorNodeAttemptReader(outputs),
		)
		dueRunner.now = func() time.Time { return output.NextAttemptAt.Add(time.Millisecond) }

		duePlan, err := dueRunner.Run(context.Background(), task.RunID(dueCoordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run(due) error = %v", err)
		}
		if got, want := len(duePlan.NodeRuns), 1; got != want {
			t.Fatalf("due node runs = %d, want %d", got, want)
		}
		wantRetryRunID := coordinatorNodeAttemptRunID(loopRun.ID, 1, "work", 0, 2)
		if got, want := duePlan.NodeRuns[0].RunID, wantRetryRunID; got != want {
			t.Fatalf("retry run id = %q, want %q", got, want)
		}
		var metadata coordinatorActionRunMetadata
		if err := json.Unmarshal(duePlan.NodeRuns[0].Metadata, &metadata); err != nil {
			t.Fatalf("decode retry metadata: %v", err)
		}
		if metadata.Attempt != 2 || metadata.Epoch != 4 {
			t.Fatalf("retry metadata = %#v, want attempt 2 epoch 4", metadata)
		}
	})

	t.Run("Should route failure and skip the success path", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, time.August, 2, 20, 0, 0, 0, time.UTC)
		node := lifecycleNode("work")
		node.Retry = &dsl.RetrySpec{MaxAttempts: 0}
		node.NodeLifecycleState = &dsl.NodeLifecycleState{
			OnError: &dsl.ErrorPolicy{Route: "fallback"},
		}
		def := lifecycleDefinition(
			node,
			lifecycleNode("fallback"),
			lifecycleNode("success"),
		)
		def.Graph.Edges = []dsl.Edge{{From: "work", To: "fallback"}, {From: "work", To: "success"}}
		topology := newControlTopology(def.Graph)
		initialOutputs := initialGenerationOutputs(def.Graph, topology, 1)
		initialPlan, err := newInitialControlCoordinatorPlan(
			Run{ID: "looprun-lifecycle-route", LoopName: "route"},
			1,
			def.Graph,
			topology,
			false,
			initialOutputs,
		)
		if err != nil {
			t.Fatalf("newInitialControlCoordinatorPlan() error = %v", err)
		}
		if got, want := len(initialPlan.Dependencies), 1; got != want {
			t.Fatalf(
				"initial route dependencies = %#v, want only the success-path dependency",
				initialPlan.Dependencies,
			)
		}
		dependency := initialPlan.Dependencies[0]
		wantTaskID := coordinatorNodeTaskID("looprun-lifecycle-route", 1, "success", 0)
		if got, want := dependency.TaskID, wantTaskID; got != want {
			t.Fatalf("initial route dependency task_id = %q, want %q", got, want)
		}
		wantDependsOnTaskID := coordinatorNodeTaskID("looprun-lifecycle-route", 1, "work", 0)
		if got, want := dependency.DependsOnTaskID, wantDependsOnTaskID; got != want {
			t.Fatalf("initial route dependency prerequisite = %q, want %q", got, want)
		}
		plan := runLifecycleFailurePlan(t, "route", now, def)
		payload := coordinatorSnapshotPayloadForTest(t, plan)
		outputs := outputsByNodeAndItemForTest(payload.Outputs)
		if got := outputs["work/0"]; got.Status != generationOutputSucceeded ||
			got.OutputRef != errorRoutedOutputRefPrefix+"fallback" {
			t.Fatalf("routed source = %#v", got)
		}
		if got := outputs["success/0"]; got.OutputRef != branchSkippedOutputRef {
			t.Fatalf("success path = %#v, want skipped", got)
		}
		if got, want := len(plan.NodeRuns), 1; got != want || plan.NodeRuns[0].TaskID !=
			coordinatorNodeTaskID("looprun-lifecycle-route", 1, "fallback", 0) {
			t.Fatalf("route node runs = %#v, want fallback only", plan.NodeRuns)
		}
		if payload.Attempts[0].Disposition != AttemptRouted {
			t.Fatalf("route attempt = %#v, want routed", payload.Attempts[0])
		}

		preterminalPlan := runLifecyclePayloadFailurePlan(t, now, def)
		preterminalPayload := coordinatorSnapshotPayloadForTest(t, preterminalPlan)
		preterminalOutputs := outputsByNodeAndItemForTest(preterminalPayload.Outputs)
		if got := preterminalOutputs["work/0"]; got.Status != generationOutputSucceeded ||
			got.OutputRef != errorRoutedOutputRefPrefix+"fallback" {
			t.Fatalf("preterminal routed source = %#v", got)
		}
		if got := preterminalOutputs["success/0"]; got.OutputRef != branchSkippedOutputRef {
			t.Fatalf("preterminal success path = %#v, want skipped", got)
		}
		if got, want := len(preterminalPlan.NodeRuns), 1; got != want ||
			preterminalPlan.NodeRuns[0].TaskID != coordinatorNodeTaskID(
				"looprun-lifecycle-route-preterminal",
				1,
				"fallback",
				0,
			) {
			t.Fatalf("preterminal route node runs = %#v, want fallback only", preterminalPlan.NodeRuns)
		}
	})

	t.Run("Should absorb only an explicitly allowed failure", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, time.August, 2, 21, 0, 0, 0, time.UTC)
		node := lifecycleNode("work")
		node.Retry = &dsl.RetrySpec{MaxAttempts: 0}
		node.NodeLifecycleState = &dsl.NodeLifecycleState{
			OnError: &dsl.ErrorPolicy{AllowFail: true},
		}
		def := lifecycleDefinition(node, lifecycleNode("next"))
		def.Graph.Edges = []dsl.Edge{{From: "work", To: "next"}}
		plan := runLifecycleFailurePlan(t, "absorb", now, def)
		payload := coordinatorSnapshotPayloadForTest(t, plan)
		outputs := outputsByNodeAndItemForTest(payload.Outputs)
		if got := outputs["work/0"]; got.Status != generationOutputSucceeded ||
			got.OutputRef != failureAbsorbedOutputRef || outputValue(got.OutputRef) != nil {
			t.Fatalf("absorbed source = %#v, want successful absent value", got)
		}
		if payload.Attempts[0].Disposition != AttemptAbsorbed {
			t.Fatalf("absorbed attempt = %#v", payload.Attempts[0])
		}
		if got, want := len(plan.NodeRuns), 1; got != want {
			t.Fatalf("node runs = %d, want downstream continuation", got)
		}
	})

	t.Run("Should escalate an unhandled failure to generation succession", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, time.August, 2, 22, 0, 0, 0, time.UTC)
		node := lifecycleNode("work")
		node.Retry = &dsl.RetrySpec{MaxAttempts: 0}
		plan := runLifecycleFailurePlan(t, "escalate", now, lifecycleDefinition(node))
		payload := coordinatorSnapshotPayloadForTest(t, plan)
		if payload.Attempts[0].Disposition != AttemptEscalated ||
			payload.Attempts[0].FailureClass == nil {
			t.Fatalf("escalated attempt = %#v", payload.Attempts[0])
		}
		if plan.NextCoordinator == nil || plan.PostReserveSnapshot == nil ||
			plan.PostReserveSnapshot.Generation != 2 {
			t.Fatalf("succession plan = %#v, want generation 2", plan)
		}
	})

	t.Run("Should keep cooperative cancellation draining while the task run is live", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, time.August, 2, 23, 0, 0, 0, time.UTC)
		loopRun := controlLoopRun("looprun-lifecycle-cancel-live", nil)
		loopRun.CancelRequested = true
		loopRun.CancelKind = RunCancelCancel
		coordinatorRun := controlCoordinatorRun(loopRun, 1)
		workerRun := lifecycleWorkerRun(loopRun, "work", task.TaskRunStatusRunning, now)
		outputs := &lifecycleCoordinatorStore{coordinatorRunnerOutputs: coordinatorRunnerOutputs{
			outputs: map[int][]GenerationOutput{1: {{
				Generation: 1, NodeID: "work", Status: generationOutputRunning,
				TaskRunID: workerRun.ID, Attempt: 1, Epoch: 4,
			}},
			},
		}}
		control := NodeControl{
			LoopRunID: loopRun.ID, NodeID: "work", CancelState: CancelStateDraining,
			CancelProvenance: &ControlProvenance{
				ActorKind: "human", ActorID: "operator-1", Reason: "operator request", RequestedAt: now,
			},
			Revision: 3,
		}
		runner := newCoordinatorRunnerForTestWithDefinition(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{coordinatorRun.ID: coordinatorRun, workerRun.ID: workerRun},
			outputs,
			lifecycleDefinition(lifecycleNode("work")),
			WithCoordinatorNodeAttemptReader(outputs),
			WithCoordinatorNodeControlReader(coordinatorNodeControlReaderStub{controls: []NodeControl{control}}),
		)
		runner.now = func() time.Time { return now.Add(time.Minute) }

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		payload := coordinatorSnapshotPayloadForTest(t, plan)
		if !plan.CancellationDrain || !plan.Yield || !plan.GenerationInFlight || plan.Terminal != nil {
			t.Fatalf("draining plan = %#v, want a visible non-terminal drain", plan)
		}
		if len(payload.Attempts) != 0 || len(payload.Controls) != 0 ||
			payload.Outputs[0].Status != generationOutputRunning {
			t.Fatalf("draining payload = %#v, want live work untouched", payload)
		}
	})

	t.Run("Should close a drained run cancellation with node and contract effects", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, time.August, 2, 23, 30, 0, 0, time.UTC)
		loopRun := controlLoopRun("looprun-lifecycle-cancel-closed", nil)
		loopRun.CancelRequested = true
		loopRun.CancelKind = RunCancelCancel
		coordinatorRun := controlCoordinatorRun(loopRun, 1)
		workerRun := lifecycleWorkerRun(loopRun, "work", task.TaskRunStatusCanceled, now)
		node := lifecycleNode("work")
		node.Normalize()
		node.OnCancel = []dsl.EffectSpec{{Emit: &dsl.EmitSpec{Kind: "work_canceled"}}}
		definition := lifecycleDefinition(node)
		definition.Contract.Normalize()
		definition.Contract.OnCanceled = []dsl.EffectSpec{{Emit: &dsl.EmitSpec{Kind: "run_canceled"}}}
		outputs := &lifecycleCoordinatorStore{coordinatorRunnerOutputs: coordinatorRunnerOutputs{
			outputs: map[int][]GenerationOutput{1: {{
				Generation: 1, NodeID: "work", Status: generationOutputRunning,
				TaskRunID: workerRun.ID, Attempt: 1, Epoch: 4,
			}},
			},
		}}
		control := NodeControl{
			LoopRunID: loopRun.ID, NodeID: "work", CancelState: CancelStateDraining,
			CancelProvenance: &ControlProvenance{
				ActorKind: "human", ActorID: "operator-1", Reason: "operator request", RequestedAt: now,
			},
			Revision: 3,
		}
		runner := newCoordinatorRunnerForTestWithDefinition(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{coordinatorRun.ID: coordinatorRun, workerRun.ID: workerRun},
			outputs,
			definition,
			WithCoordinatorNodeAttemptReader(outputs),
			WithCoordinatorNodeControlReader(coordinatorNodeControlReaderStub{controls: []NodeControl{control}}),
		)
		runner.now = func() time.Time { return now.Add(time.Minute) }

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		payload := coordinatorSnapshotPayloadForTest(t, plan)
		if plan.Terminal == nil || plan.Terminal.Status != string(StatusCanceled) ||
			plan.Terminal.Cause != string(TransitionCauseOperatorCancel) || !plan.CancellationDrain {
			t.Fatalf("terminal cancellation plan = %#v", plan)
		}
		if len(payload.Attempts) != 1 || payload.Attempts[0].Disposition != AttemptCanceled ||
			payload.Outputs[0].Status != generationOutputCanceled {
			t.Fatalf("canceled payload = %#v", payload)
		}
		if len(payload.Controls) != 1 || payload.Controls[0].Kind != NodeControlMutationCancel ||
			payload.Controls[0].CancelState != CancelStateCanceled {
			t.Fatalf("cancel controls = %#v", payload.Controls)
		}
		if len(payload.Events) != 1 || payload.Events[0].Kind != GenerationLifecycleEventNodeCanceled ||
			len(payload.Events[0].Effects) != 1 {
			t.Fatalf("cancel events = %#v, want one on_cancel delivery", payload.Events)
		}
		if got := payload.BoundaryEffects[StatusCanceled]; len(got) != 1 {
			t.Fatalf("on_canceled boundary effects = %#v, want one", got)
		}
	})

	t.Run("Should close only the targeted node after its task run drains", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, time.August, 3, 0, 0, 0, 0, time.UTC)
		loopRun := controlLoopRun("looprun-lifecycle-node-cancel", nil)
		coordinatorRun := controlCoordinatorRun(loopRun, 1)
		workerRun := lifecycleWorkerRun(loopRun, "work", task.TaskRunStatusCanceled, now)
		node := lifecycleNode("work")
		node.Normalize()
		node.OnCancel = []dsl.EffectSpec{{Emit: &dsl.EmitSpec{Kind: "work_canceled"}}}
		outputs := &lifecycleCoordinatorStore{coordinatorRunnerOutputs: coordinatorRunnerOutputs{
			outputs: map[int][]GenerationOutput{1: {{
				Generation: 1, NodeID: "work", Status: generationOutputRunning,
				TaskRunID: workerRun.ID, Attempt: 1, Epoch: 8,
			}},
			},
		}}
		control := NodeControl{
			LoopRunID: loopRun.ID, NodeID: "work", CancelState: CancelStateDraining,
			CancelProvenance: &ControlProvenance{
				ActorKind: "human", ActorID: "operator-1", Reason: "cancel only work", RequestedAt: now,
			},
			Revision: 5,
		}
		runner := newCoordinatorRunnerForTestWithDefinition(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{coordinatorRun.ID: coordinatorRun, workerRun.ID: workerRun},
			outputs,
			lifecycleDefinition(node),
			WithCoordinatorNodeAttemptReader(outputs),
			WithCoordinatorNodeControlReader(coordinatorNodeControlReaderStub{controls: []NodeControl{control}}),
		)
		runner.now = func() time.Time { return now.Add(time.Minute) }

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		payload := coordinatorSnapshotPayloadForTest(t, plan)
		if plan.Terminal == nil || plan.Terminal.Status == string(StatusCanceled) {
			t.Fatalf("node-only cancellation changed Run outcome: %#v", plan.Terminal)
		}
		if len(payload.Controls) != 1 || payload.Controls[0].CancelState != CancelStateCanceled ||
			len(payload.Events) != 1 || len(payload.Events[0].Effects) != 1 ||
			payload.Outputs[0].Status != generationOutputCanceled {
			t.Fatalf("node-only cancellation payload = %#v", payload)
		}
	})

	t.Run("Should close canceled waiting and paused nodes without a task run", func(t *testing.T) {
		t.Parallel()

		for _, status := range []string{generationOutputWaiting, generationOutputPaused} {
			t.Run("Should close "+status+" node", func(t *testing.T) {
				t.Parallel()

				now := time.Date(2026, time.August, 3, 0, 15, 0, 0, time.UTC)
				loopRun := controlLoopRun("looprun-lifecycle-node-cancel-"+status, nil)
				coordinatorRun := controlCoordinatorRun(loopRun, 1)
				outputs := &lifecycleCoordinatorStore{coordinatorRunnerOutputs: coordinatorRunnerOutputs{
					outputs: map[int][]GenerationOutput{1: {{
						Generation: 1, NodeID: "work", Status: status, Attempt: 1, Epoch: 2,
					}}},
				}}
				control := NodeControl{
					LoopRunID: loopRun.ID, NodeID: "work", CancelState: CancelStateDraining,
					CancelProvenance: &ControlProvenance{
						ActorKind: "human", ActorID: "operator-1", Reason: "cancel parked work", RequestedAt: now,
					},
					Revision: 3,
				}
				runner := newCoordinatorRunnerForTestWithDefinition(
					t,
					loopRun,
					coordinatorRun,
					nil,
					outputs,
					lifecycleDefinition(lifecycleNode("work")),
					WithCoordinatorNodeAttemptReader(outputs),
					WithCoordinatorNodeControlReader(
						coordinatorNodeControlReaderStub{controls: []NodeControl{control}},
					),
				)
				runner.now = func() time.Time { return now.Add(time.Minute) }

				plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
				if err != nil {
					t.Fatalf("Run(%s) error = %v", status, err)
				}
				payload := coordinatorSnapshotPayloadForTest(t, plan)
				if len(payload.Controls) != 1 || payload.Controls[0].CancelState != CancelStateCanceled ||
					len(payload.Events) != 1 || payload.Outputs[0].Status != generationOutputCanceled {
					t.Fatalf("%s cancellation payload = %#v", status, payload)
				}
			})
		}
	})

	t.Run("Should cancel a backoff without scheduling its next attempt", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, time.August, 3, 0, 30, 0, 0, time.UTC)
		loopRun := controlLoopRun("looprun-lifecycle-backoff-cancel", nil)
		coordinatorRun := controlCoordinatorRun(loopRun, 1)
		nextAttemptAt := now.Add(time.Hour)
		failureClass := FailureTransport
		outputs := &lifecycleCoordinatorStore{
			coordinatorRunnerOutputs: coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {{
				Generation: 1, NodeID: "work", Status: generationOutputRetrying,
				Attempt: 1, NextAttemptAt: &nextAttemptAt, Epoch: 10,
			}}}},
			attempts: []NodeAttempt{{
				LoopRunID: loopRun.ID, Generation: 1, NodeID: "work", Attempt: 1,
				FailureClass: &failureClass, Disposition: AttemptRetried,
				StartedAt: now.Add(-time.Minute), EndedAt: new(now), NextAttemptAt: &nextAttemptAt,
			}},
		}
		control := NodeControl{
			LoopRunID: loopRun.ID, NodeID: "work", CancelState: CancelStateDraining,
			CancelProvenance: &ControlProvenance{
				ActorKind: "human", ActorID: "operator-1", Reason: "cancel backoff", RequestedAt: now,
			},
			Revision: 3,
		}
		runner := newCoordinatorRunnerForTestWithDefinition(
			t,
			loopRun,
			coordinatorRun,
			nil,
			outputs,
			lifecycleDefinition(lifecycleNode("work")),
			WithCoordinatorNodeAttemptReader(outputs),
			WithCoordinatorNodeControlReader(coordinatorNodeControlReaderStub{controls: []NodeControl{control}}),
		)
		runner.now = func() time.Time { return now.Add(time.Minute) }

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		payload := coordinatorSnapshotPayloadForTest(t, plan)
		if len(plan.PostCommitTimers) != 0 || len(plan.NodeRuns) != 0 ||
			len(payload.Attempts) != 1 || payload.Attempts[0].Attempt != 2 ||
			payload.Attempts[0].Disposition != AttemptCanceled ||
			payload.Outputs[0].Status != generationOutputCanceled || payload.Outputs[0].NextAttemptAt != nil {
			t.Fatalf("backoff cancellation plan = %#v payload = %#v", plan, payload)
		}
	})
}

const toolsUnavailableCodeForTest = "tool_unavailable"

type lifecycleCoordinatorStore struct {
	coordinatorRunnerOutputs
	attempts []NodeAttempt
}

func (s *lifecycleCoordinatorStore) ListNodeAttempts(
	context.Context,
	WorkspaceID,
	RunID,
) ([]NodeAttempt, error) {
	return append([]NodeAttempt(nil), s.attempts...), nil
}

func lifecycleNode(id dsl.NodeID) dsl.Node {
	return dsl.Node{
		ID:    id,
		Class: dsl.NodeClassAction,
		Kind:  string(dsl.ActionTransform),
		Params: dsl.NodeParams{"map": map[string]any{
			"value": map[string]any{"value": string(id)},
		}},
	}
}

func lifecycleDefinition(nodes ...dsl.Node) dsl.Definition {
	return dsl.Definition{Graph: dsl.Graph{Nodes: nodes, Edges: []dsl.Edge{}}}
}

func lifecycleWorkerRun(loopRun Run, nodeID dsl.NodeID, status task.RunStatus, now time.Time) task.Run {
	run := controlWorkerRun(loopRun, nodeID, 0, status)
	run.QueuedAt = now.Add(-2 * time.Second)
	run.StartedAt = now.Add(-time.Second)
	run.EndedAt = now
	run.Error = "provider unavailable"
	return run
}

func lifecycleFailureRef(t *testing.T, code string, cause string) string {
	t.Helper()
	ref, ok := ActionFailureOutputRef(NewActionFailure(code, cause, "retry later"))
	if !ok {
		t.Fatal("ActionFailureOutputRef() = false")
	}
	return ref
}

func runLifecycleFailurePlan(
	t *testing.T,
	slug string,
	now time.Time,
	def dsl.Definition,
) task.CoordinatorCompletionPlan {
	t.Helper()
	return runLifecycleFailurePlanWithConfigAndControls(t, slug, now, def, LifecycleConfig{}, nil)
}

func runLifecycleFailurePlanWithConfig(
	t *testing.T,
	slug string,
	now time.Time,
	def dsl.Definition,
	lifecycle LifecycleConfig,
) task.CoordinatorCompletionPlan {
	t.Helper()
	return runLifecycleFailurePlanWithConfigAndControls(t, slug, now, def, lifecycle, nil)
}

func runLifecycleFailurePlanWithConfigAndControls(
	t *testing.T,
	slug string,
	now time.Time,
	def dsl.Definition,
	lifecycle LifecycleConfig,
	controls []NodeControl,
) task.CoordinatorCompletionPlan {
	t.Helper()
	loopRun := controlLoopRun("looprun-lifecycle-"+slug, nil)
	coordinatorRun := controlCoordinatorRun(loopRun, 1)
	workerRun := lifecycleWorkerRun(loopRun, "work", task.TaskRunStatusFailed, now)
	firstScheduledAt := now.Add(-time.Second)
	outputRows := make([]GenerationOutput, 0, len(def.Graph.Nodes))
	for _, node := range def.Graph.Nodes {
		output := GenerationOutput{
			Generation: 1,
			NodeID:     string(node.ID),
			Status:     generationOutputPending,
			Attempt:    1,
		}
		if node.ID == "work" {
			output.Status = generationOutputFailed
			output.OutputRef = lifecycleFailureRef(t, string(toolsUnavailableCodeForTest), "provider unavailable")
			output.TaskRunID = workerRun.ID
			output.FirstScheduledAt = &firstScheduledAt
		}
		outputRows = append(outputRows, output)
	}
	outputs := &lifecycleCoordinatorStore{coordinatorRunnerOutputs: coordinatorRunnerOutputs{
		outputs: map[int][]GenerationOutput{1: outputRows},
	}}
	options := []CoordinatorRunnerOption{WithCoordinatorNodeAttemptReader(outputs)}
	if controls != nil {
		options = append(
			options,
			WithCoordinatorNodeControlReader(coordinatorNodeControlReaderStub{controls: controls}),
		)
	}
	runner := newCoordinatorRunnerForLifecycleTest(
		t, loopRun, coordinatorRun, workerRun, outputs, def, lifecycle, options...,
	)
	runner.now = func() time.Time { return now }
	plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	return plan
}

func newCoordinatorRunnerForLifecycleTest(
	t *testing.T,
	loopRun Run,
	coordinatorRun task.Run,
	workerRun task.Run,
	outputs GenerationOutputReader,
	def dsl.Definition,
	lifecycle LifecycleConfig,
	opts ...CoordinatorRunnerOption,
) *CoordinatorRunner {
	t.Helper()
	resolved := resolvedCoordinatorDefinitionForTest(t, def)
	defaults := LoopDefaults{
		Delivery: definitionConfigLayer(def),
		Watch:    definitionConfigLayer(def),
	}
	if lifecycle.Autopause != nil {
		defaults.Delivery.Lifecycle = &lifecycle
		defaults.Watch.Lifecycle = &lifecycle
	}
	effective, err := ResolveEffectiveConfig(resolved, defaults, nil, LoopConfig{})
	if err != nil {
		t.Fatalf("ResolveEffectiveConfig() error = %v", err)
	}
	loopRun, snapshot := pinCoordinatorResolvedForTest(t, loopRun, resolved, effective)
	runner, err := NewCoordinatorRunner(
		&coordinatorRunnerTaskRunReader{runs: map[string]task.Run{
			coordinatorRun.ID: coordinatorRun,
			workerRun.ID:      workerRun,
		}},
		&coordinatorRunnerLoopStore{run: loopRun, snapshot: snapshot},
		outputs,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		opts...,
	)
	if err != nil {
		t.Fatalf("NewCoordinatorRunner() error = %v", err)
	}
	return runner
}

func runLifecyclePayloadFailurePlan(
	t *testing.T,
	now time.Time,
	def dsl.Definition,
) task.CoordinatorCompletionPlan {
	t.Helper()
	loopRun := controlLoopRun("looprun-lifecycle-route-preterminal", nil)
	coordinatorRun := controlCoordinatorRun(loopRun, 1)
	workerRun := lifecycleWorkerRun(loopRun, "work", task.TaskRunStatusCompleted, now)
	workerRun.Result = json.RawMessage(`{"summary":"failed","error":"provider unavailable"}`)
	firstScheduledAt := now.Add(-time.Second)
	outputRows := make([]GenerationOutput, 0, len(def.Graph.Nodes))
	for _, node := range def.Graph.Nodes {
		output := GenerationOutput{
			Generation: 1,
			NodeID:     string(node.ID),
			Status:     generationOutputPending,
			Attempt:    1,
		}
		if node.ID == "work" {
			output.Status = generationOutputSucceeded
			output.OutputRef = string(workerRun.Result)
			output.TaskRunID = workerRun.ID
			output.FirstScheduledAt = &firstScheduledAt
		}
		outputRows = append(outputRows, output)
	}
	outputs := &lifecycleCoordinatorStore{coordinatorRunnerOutputs: coordinatorRunnerOutputs{
		outputs: map[int][]GenerationOutput{1: outputRows},
	}}
	runner := newCoordinatorRunnerForTestWithDefinition(
		t,
		loopRun,
		coordinatorRun,
		map[string]task.Run{coordinatorRun.ID: coordinatorRun, workerRun.ID: workerRun},
		outputs,
		def,
		WithCoordinatorNodeAttemptReader(outputs),
	)
	runner.now = func() time.Time { return now }
	plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	return plan
}

func TestCoordinatorQuarantineShouldPreserveDiagnosableSanitizedHistory(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 2, 23, 0, 0, 0, time.UTC)
	failureClass := FailureTransport
	oldEntry, err := encodeQuarantineEntry(QuarantineEntry{
		NodeID:   "fetch",
		InputRef: "loop-run:looprun-quarantine-history:node:fetch:input",
		Target:   "compozy__fetch",
		Episodes: []QuarantineEpisode{{
			Generation: 2, QuarantinedAt: now.Add(-time.Hour),
			Attempts: []NodeAttempt{{
				LoopRunID: "looprun-quarantine-history", Generation: 2, NodeID: "fetch",
				Attempt: 1, FailureClass: &failureClass, FailureCode: "backend_failed",
				Cause: "old failure", Disposition: AttemptQuarantined, StartedAt: now.Add(-2 * time.Hour),
			}},
		}},
		Requeues: []QuarantineProvenance{{
			ActorKind: "human", ActorID: "operator:alice", Reason: "target repaired",
			RequestedAt: now.Add(-30 * time.Minute), Generation: 3,
		}},
	})
	if err != nil {
		t.Fatalf("encodeQuarantineEntry(old) error = %v", err)
	}
	endedAt := now.Add(-time.Second)
	attempts := []NodeAttempt{
		{
			LoopRunID: "looprun-quarantine-history", Generation: 3, NodeID: "fetch",
			Attempt: 1, FailureClass: &failureClass, FailureCode: "backend_failed",
			Cause: "api_key=planted-secret", Hint: "rotate token=planted-secret",
			Target: "compozy__fetch", Disposition: AttemptEscalated,
			StartedAt: now.Add(-2 * time.Minute), EndedAt: &endedAt,
		},
		{
			LoopRunID: "looprun-quarantine-history", Generation: 4, NodeID: "fetch",
			Attempt: 2, FailureClass: &failureClass, FailureCode: "backend_failed",
			Cause: "api_key=planted-secret", Hint: "check the upstream",
			Target: "compozy__fetch", Disposition: AttemptQuarantined,
			StartedAt: now.Add(-time.Minute), EndedAt: &endedAt,
		},
	}
	runner := &CoordinatorRunner{now: func() time.Time { return now }}
	raw, mutation, event, err := runner.buildNodeQuarantine(
		Run{ID: "looprun-quarantine-history", WorkspaceID: "ws-1"},
		4,
		dsl.Graph{Nodes: []dsl.Node{{ID: "fetch", Class: dsl.NodeClassAction, Kind: "compozy__fetch"}}},
		"fetch",
		attempts,
		NodeControl{LoopRunID: "looprun-quarantine-history", NodeID: "fetch", Revision: 7,
			QuarantineEntry: oldEntry},
	)
	if err != nil {
		t.Fatalf("buildNodeQuarantine() error = %v", err)
	}
	if strings.Contains(string(raw), "planted-secret") || !strings.Contains(string(raw), "[REDACTED]") {
		t.Fatalf("quarantine entry = %s, want planted secret redacted", raw)
	}
	for _, key := range []string{
		`"loop_run_id"`, `"node_id"`, `"item_index"`, `"failure_class"`,
		`"failure_code"`, `"started_at"`, `"ended_at"`,
	} {
		if !strings.Contains(string(raw), key) {
			t.Fatalf("quarantine entry = %s, want canonical key %s", raw, key)
		}
	}
	for _, legacyKey := range []string{`"LoopRunID"`, `"FailureClass"`, `"StartedAt"`} {
		if strings.Contains(string(raw), legacyKey) {
			t.Fatalf("quarantine entry = %s, want no Go field key %s", raw, legacyKey)
		}
	}
	var entry QuarantineEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatalf("json.Unmarshal(quarantine entry) error = %v", err)
	}
	if entry.NodeID != "fetch" || entry.Target != "compozy__fetch" ||
		entry.InputRef != "loop-run:looprun-quarantine-history:node:fetch:input" ||
		len(entry.Episodes) != 2 || len(entry.Episodes[1].Attempts) != 2 || len(entry.Requeues) != 1 {
		t.Fatalf("quarantine entry = %#v, want complete appended history", entry)
	}
	latest := entry.Episodes[1].Attempts[1]
	if latest.StartedAt.IsZero() || latest.EndedAt == nil || latest.Hint != "check the upstream" {
		t.Fatalf("latest attempt = %#v, want timestamps and remediation hint", latest)
	}
	if mutation.Kind != NodeControlMutationQuarantine || mutation.ExpectedRevision != 7 ||
		event.Kind != GenerationLifecycleEventNodeQuarantined || event.Disposition != AttemptQuarantined {
		t.Fatalf("mutation/event = %#v / %#v, want atomic quarantine intents", mutation, event)
	}

	oversized := QuarantineEntry{NodeID: "fetch", InputRef: quarantineInputRef(
		Run{ID: "looprun-quarantine-history"}, "fetch",
	)}
	for generation := 1; generation <= maxQuarantineEpisodes+8; generation++ {
		episode := QuarantineEpisode{Generation: generation, QuarantinedAt: now}
		for attempt := 1; attempt <= 4; attempt++ {
			episode.Attempts = append(episode.Attempts, NodeAttempt{
				LoopRunID: "looprun-quarantine-history", Generation: generation, NodeID: "fetch",
				Attempt: attempt, FailureClass: &failureClass, FailureCode: "backend_failed",
				Cause:       strings.Repeat("upstream transport failure ", 160),
				Hint:        strings.Repeat("repair upstream then requeue ", 160),
				Disposition: AttemptQuarantined, StartedAt: now,
			})
		}
		oversized.Episodes = append(oversized.Episodes, episode)
		oversized.Requeues = append(oversized.Requeues, QuarantineProvenance{
			ActorKind: "human", ActorID: "operator:alice", Reason: strings.Repeat("repair note ", 160),
			RequestedAt: now, Generation: generation,
		})
	}
	bounded, err := encodeQuarantineEntry(oversized)
	if err != nil {
		t.Fatalf("encodeQuarantineEntry(oversized) error = %v", err)
	}
	if len(bounded) > maxQuarantineEntryBytes {
		t.Fatalf("bounded quarantine bytes = %d, want <= %d", len(bounded), maxQuarantineEntryBytes)
	}
	var boundedEntry QuarantineEntry
	if err := json.Unmarshal(bounded, &boundedEntry); err != nil {
		t.Fatalf("json.Unmarshal(bounded quarantine) error = %v", err)
	}
	if !boundedEntry.Truncated || len(boundedEntry.Episodes) == 0 ||
		boundedEntry.Episodes[len(boundedEntry.Episodes)-1].Generation != maxQuarantineEpisodes+8 ||
		len(boundedEntry.Requeues) == 0 ||
		boundedEntry.Requeues[len(boundedEntry.Requeues)-1].Generation != maxQuarantineEpisodes+8 {
		t.Fatalf("bounded quarantine entry = %#v, want latest evidence with truncation marker", boundedEntry)
	}
}

func TestCoordinatorParkedDependencyShouldSurfaceNeedsAttention(t *testing.T) {
	t.Parallel()

	graph := dsl.Graph{
		Nodes: []dsl.Node{
			{ID: "producer", Class: dsl.NodeClassAction, Kind: "compozy__fetch"},
			{ID: "consumer", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform)},
		},
		Edges: []dsl.Edge{{From: "producer", To: "consumer"}},
	}
	for _, parkedStatus := range []string{generationOutputQuarantined, generationOutputPaused} {
		t.Run("Should name a "+parkedStatus+" required producer", func(t *testing.T) {
			t.Parallel()

			now := time.Date(2026, time.August, 2, 23, 30, 0, 0, time.UTC)
			runner := &CoordinatorRunner{now: func() time.Time { return now }}
			outputs := []GenerationOutput{
				{Generation: 2, NodeID: "producer", Status: parkedStatus},
				{Generation: 2, NodeID: "consumer", Status: generationOutputPending},
				{Generation: 2, NodeID: "consumer", ItemIndex: 1, Status: generationOutputPending},
			}
			plan := coordinatorFinisherPlan(
				Run{ID: "looprun-parked-dependency", WorkspaceID: "ws-1"}, 2, outputs, nil,
			)
			updated, blocked, err := runner.applyParkedDependencyAttention(
				context.Background(),
				Run{ID: "looprun-parked-dependency", WorkspaceID: "ws-1"},
				graph,
				newControlTopology(graph),
				outputs,
				plan,
			)
			if err != nil {
				t.Fatalf("applyParkedDependencyAttention() error = %v", err)
			}
			payload := coordinatorSnapshotPayloadForTest(t, updated)
			if !blocked || !updated.Yield || len(payload.Controls) != 1 ||
				payload.Controls[0].AttentionFlag != "dependency_quarantined" ||
				!strings.Contains(payload.Controls[0].AttentionReason, "producer") {
				t.Fatalf("plan = %#v payload = %#v, want named dependency attention", updated, payload)
			}
			materialized := coordinatorFinisherPlan(
				Run{ID: "looprun-parked-dependency", WorkspaceID: "ws-1"}, 2, outputs, nil,
			)
			if err := appendCoordinatorTasksForOutputs(
				&materialized,
				Run{ID: "looprun-parked-dependency", WorkspaceID: "ws-1", LoopName: "delivery"},
				2,
				graph,
				newControlTopology(graph),
				false,
				outputs,
			); err != nil {
				t.Fatalf("appendCoordinatorTasksForOutputs() error = %v", err)
			}
			if len(materialized.NodeTasks) != 0 {
				t.Fatalf("node tasks = %#v, want parked producer and dependents excluded", materialized.NodeTasks)
			}
		})
	}

	t.Run("Should keep an all-paused generation live without stall work", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, time.August, 3, 0, 15, 0, 0, time.UTC)
		runner := &CoordinatorRunner{now: func() time.Time { return now }}
		outputs := []GenerationOutput{
			{Generation: 2, NodeID: "producer", Status: generationOutputPaused},
			{Generation: 2, NodeID: "consumer", Status: generationOutputPaused},
		}
		plan := coordinatorFinisherPlan(
			Run{ID: "looprun-all-paused", WorkspaceID: "ws-1"}, 2, outputs, nil,
		)
		updated, parked, err := runner.applyParkedDependencyAttention(
			context.Background(),
			Run{ID: "looprun-all-paused", WorkspaceID: "ws-1"},
			graph,
			newControlTopology(graph),
			outputs,
			plan,
		)
		if err != nil {
			t.Fatalf("applyParkedDependencyAttention() error = %v", err)
		}
		if !parked || !updated.Yield || !updated.GenerationInFlight || updated.Terminal != nil ||
			len(updated.NodeRuns) != 0 {
			t.Fatalf("all-paused plan = %#v, want live paused-dominant yield without work", updated)
		}
	})
}
