package loop

import (
	"context"
	"encoding/json"
	"strings"
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

	t.Run("Should classify a raw output reference with a stable payload-declared code", func(t *testing.T) {
		t.Parallel()

		failure := classifyGenerationOutputFailure(
			GenerationOutput{Status: generationOutputFailed, OutputRef: strings.Repeat("raw-reference-", 300)},
			task.Run{Error: "worker failed"},
		)
		if failure.Class != FailurePayloadDeclared || failure.Code != string(FailurePayloadDeclared) ||
			!strings.Contains(failure.Cause, "raw-reference-") || len(failure.Cause) > actionFailureTextLimit {
			t.Fatalf("classified raw output = %#v, want bounded payload-declared failure", failure)
		}
	})

	t.Run("Should not skip a fan-out collect before all route dependencies are skipped", func(t *testing.T) {
		t.Parallel()

		graph := dsl.Graph{
			Nodes: []dsl.Node{
				{ID: "fan", Class: dsl.NodeClassControl, Kind: string(dsl.ControlFanOut)},
				{ID: "work", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform)},
				{ID: "independent", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform)},
				{ID: "collect", Class: dsl.NodeClassControl, Kind: string(dsl.ControlCollect)},
			},
			Edges: []dsl.Edge{
				{From: "fan", To: "work"},
				{From: "work", To: "collect"},
				{From: "independent", To: "collect"},
			},
		}
		outputs := []GenerationOutput{
			{NodeID: "fan", Status: generationOutputPending},
			{NodeID: "work", Status: generationOutputPending},
			{NodeID: "independent", Status: generationOutputPending},
			{NodeID: "collect", Status: generationOutputPending},
		}
		skipRouteNodes(
			graph,
			newControlTopology(graph),
			GenerationOutput{NodeID: "source"},
			[]dsl.NodeID{"fan"},
			&outputs,
		)
		byNode := outputsByNodeAndItemForTest(outputs)
		if byNode["fan/0"].OutputRef != branchSkippedOutputRef ||
			byNode["work/0"].OutputRef != branchSkippedOutputRef ||
			byNode["collect/0"].Status != generationOutputPending {
			t.Fatalf("route outputs = %#v, want collect pending behind live dependency", byNode)
		}
	})

	t.Run("Should discard a target probe when terminal task lookup fails", func(t *testing.T) {
		t.Parallel()

		const taskRunID = "taskrun-terminal-read-failure"
		firstScheduledAt := time.Now().UTC()
		runner := &CoordinatorRunner{
			taskRuns:     &coordinatorRunnerTaskRunReader{runs: map[string]task.Run{}},
			targetHealth: newRecordingTargetHealth(),
		}
		runner.targetProbes.Store(taskRunID, targetProbeObservation{})
		graph := dsl.Graph{Nodes: []dsl.Node{{
			ID: "work", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform),
		}}}
		_, err := runner.applyTerminalNodeLifecycle(
			t.Context(),
			Run{ID: "run-terminal-read-failure", WorkspaceID: "ws-1"},
			1,
			EffectiveConfig{},
			graph,
			newControlTopology(graph),
			[]GenerationOutput{{
				Generation: 1, NodeID: "work", Status: generationOutputFailed,
				TaskRunID: taskRunID, Attempt: 1, FirstScheduledAt: &firstScheduledAt,
			}},
			0,
			nil,
			GenerationHistory{},
		)
		if err == nil {
			t.Fatal("applyTerminalNodeLifecycle() error = nil, want task lookup failure")
		}
		if _, found := runner.takeTargetProbe(taskRunID); found {
			t.Fatal("terminal task lookup failure retained target probe")
		}
	})

	t.Run("Should not require generation history when target health is disabled", func(t *testing.T) {
		t.Parallel()

		const taskRunID = "taskrun-no-target-health"
		firstScheduledAt := time.Date(2026, time.August, 3, 20, 30, 0, 0, time.UTC)
		runner := &CoordinatorRunner{taskRuns: &coordinatorRunnerTaskRunReader{runs: map[string]task.Run{
			taskRunID: {ID: taskRunID, Status: task.TaskRunStatusCompleted},
		}}}
		runner.now = func() time.Time { return firstScheduledAt }
		graph := dsl.Graph{Nodes: []dsl.Node{{
			ID: "work", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform),
		}}}
		result, err := runner.applyTerminalNodeLifecycle(
			t.Context(),
			Run{ID: "run-no-target-health", WorkspaceID: "ws-1"},
			2,
			EffectiveConfig{},
			graph,
			newControlTopology(graph),
			[]GenerationOutput{{
				Generation: 2, NodeID: "work", Status: generationOutputSucceeded,
				TaskRunID: taskRunID, Attempt: 1, FirstScheduledAt: &firstScheduledAt,
			}},
			0,
			nil,
			GenerationHistory{},
		)
		if err != nil {
			t.Fatalf("applyTerminalNodeLifecycle() error = %v", err)
		}
		if !result.handled || result.attempt.Disposition != AttemptSucceeded {
			t.Fatalf("lifecycle result = %#v, want handled success without history", result)
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
}
