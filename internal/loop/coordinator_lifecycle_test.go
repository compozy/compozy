package loop

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
)

func TestCoordinatorRunnerShouldApplyNodeFailurePrecedence(t *testing.T) {
	t.Parallel()

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
