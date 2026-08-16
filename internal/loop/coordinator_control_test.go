package loop

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/gate"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/task"
)

// Invariant: a wait control is coordinator-owned and its waiting output, durable row intent,
// lifecycle event, and lack of worker work are one completion plan. This canonical control
// suite owns wait-node planning.
func TestCoordinatorRunnerShouldParkWaitControls(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 4, 15, 0, 0, 0, time.UTC)
	definition := dsl.Definition{Graph: dsl.Graph{
		Nodes: []dsl.Node{
			{ID: "wait_for_ack", Class: dsl.NodeClassControl, Kind: string(dsl.ControlWait),
				Params: dsl.NodeParams{"for": "10m", "expect": map[string]any{"approved": "boolean"}}},
			{ID: "publish", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform),
				Params: dsl.NodeParams{"map": map[string]any{"published": map[string]any{"value": true}}}},
		},
		Edges: []dsl.Edge{{From: "wait_for_ack", To: "publish"}},
	}}
	resolved := compileCoordinatorControlDefinition(t, definition)
	loopRun := controlLoopRun("looprun-wait-control", nil)
	coordinatorRun := controlCoordinatorRun(loopRun, 1)
	runner := newCoordinatorRunnerForControlTest(
		t,
		loopRun,
		coordinatorRun,
		nil,
		coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
			{Generation: 1, NodeID: "wait_for_ack", Status: generationOutputPending, Attempt: 1},
			{Generation: 1, NodeID: "publish", Status: generationOutputPending, Attempt: 1},
		}}},
		resolved,
	)
	runner.now = func() time.Time { return now }
	plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if plan.Terminal != nil || len(plan.NodeRuns) != 0 || !plan.Yield || !plan.GenerationInFlight {
		t.Fatalf("wait plan = terminal:%#v runs:%d yield:%v live:%v, want parked in-flight yield",
			plan.Terminal, len(plan.NodeRuns), plan.Yield, plan.GenerationInFlight)
	}
	payload := coordinatorSnapshotPayloadForTest(t, plan)
	outputs := outputsByNodeAndItemForTest(payload.Outputs)
	waitOutput := outputs["wait_for_ack/0"]
	if waitOutput.Status != generationOutputWaiting || waitOutput.Epoch != 1 || waitOutput.TaskRunID != "" {
		t.Fatalf("wait output = %#v, want waiting epoch 1 without worker", waitOutput)
	}
	if len(payload.Waits) != 1 || payload.Waits[0].Kind != "timer" ||
		payload.Waits[0].ResumeAt == nil || !payload.Waits[0].ResumeAt.Equal(now.Add(10*time.Minute)) ||
		payload.Waits[0].IssuedEpoch != 1 {
		t.Fatalf("wait intents = %#v, want one durable ten-minute timer", payload.Waits)
	}
	if len(payload.Events) != 1 || payload.Events[0].Kind != GenerationLifecycleEventNodeWaitStarted {
		t.Fatalf("wait events = %#v, want node_wait_started", payload.Events)
	}

	initialRun := controlLoopRun("looprun-initial-wait-control", nil)
	initialRun.Generation = 0
	initialCoordinator := controlCoordinatorRun(initialRun, 0)
	initialRunner := newCoordinatorRunnerForControlTest(
		t,
		initialRun,
		initialCoordinator,
		nil,
		coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{}},
		resolved,
	)
	initialRunner.now = func() time.Time { return now }
	initialPlan, err := initialRunner.Run(context.Background(), task.RunID(initialCoordinator.ID))
	if err != nil {
		t.Fatalf("Run(initial wait) error = %v", err)
	}
	if initialPlan.Terminal != nil || !initialPlan.Yield || !initialPlan.GenerationInFlight {
		t.Fatalf(
			"initial wait plan = terminal:%#v yield:%v live:%v, want parked in-flight yield",
			initialPlan.Terminal,
			initialPlan.Yield,
			initialPlan.GenerationInFlight,
		)
	}
	initialPayload := coordinatorSnapshotPayloadForTest(t, initialPlan)
	initialOutputs := outputsByNodeAndItemForTest(initialPayload.Outputs)
	if initialOutputs["wait_for_ack/0"].Status != generationOutputWaiting || len(initialPayload.Waits) != 1 {
		t.Fatalf(
			"initial wait outputs = %#v waits = %#v, want one durable waiting node",
			initialOutputs,
			initialPayload.Waits,
		)
	}

	deadline := now.Add(45 * time.Minute)
	templatedDefinition := dsl.Definition{
		Inputs: map[string]dsl.Input{"deadline": {Type: dsl.InputTypeString, Required: true}},
		Graph: dsl.Graph{Nodes: []dsl.Node{{
			ID: "wait_until_deadline", Class: dsl.NodeClassControl, Kind: string(dsl.ControlWait),
			Params: dsl.NodeParams{"until": "{{ .inputs.deadline }}"},
		}}},
	}
	templatedResolved := compileCoordinatorControlDefinition(t, templatedDefinition)
	templatedRun := controlLoopRun("looprun-wait-until-template", map[string]any{
		"deadline": deadline.Format(time.RFC3339Nano),
	})
	templatedCoordinator := controlCoordinatorRun(templatedRun, 1)
	templatedRunner := newCoordinatorRunnerForControlTest(
		t,
		templatedRun,
		templatedCoordinator,
		nil,
		coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {{
			Generation: 1, NodeID: "wait_until_deadline", Status: generationOutputPending, Attempt: 1,
		}}}},
		templatedResolved,
	)
	templatedRunner.now = func() time.Time { return now }
	templatedPlan, err := templatedRunner.Run(context.Background(), task.RunID(templatedCoordinator.ID))
	if err != nil {
		t.Fatalf("Run(templated until) error = %v", err)
	}
	templatedPayload := coordinatorSnapshotPayloadForTest(t, templatedPlan)
	if len(templatedPayload.Waits) != 1 || templatedPayload.Waits[0].ResumeAt == nil ||
		!templatedPayload.Waits[0].ResumeAt.Equal(deadline) {
		t.Fatalf("templated wait intents = %#v, want resolved deadline %s", templatedPayload.Waits, deadline)
	}
}

// Invariant: an ask freezes one redacted request and parks the exact cell without worker work.
// The canonical coordinator-control suite owns ask planning and default-vs-authored expiry precedence.
func TestCoordinatorRunnerShouldParkAskRequests(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 16, 12, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		name          string
		authoredAfter string
		wantAfter     time.Duration
	}{
		{name: "Should seed expiry when the ask omits it", wantAfter: 72 * time.Hour},
		{name: "Should preserve authored expiry over the seed", authoredAfter: "1h", wantAfter: time.Hour},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			params := dsl.NodeParams{
				"prompt": "Choose release target for {{ .inputs.release }}",
				"context": map[string]any{
					"release":   "{{ .inputs.release }}",
					"api_token": "super-secret-token",
					"evidence":  strings.Repeat("x", requestPreviewLimitBytes+256),
				},
				"expect":     map[string]any{"environment": "string"},
				"responders": map[string]any{"agents": "allow"},
			}
			if tt.authoredAfter != "" {
				params["expires"] = map[string]any{"after": tt.authoredAfter}
			}
			definition := dsl.Definition{
				APIVersion: dsl.APIVersion,
				Kind:       dsl.KindLoop,
				Meta:       dsl.Meta{Name: "ask-control"},
				Inputs:     map[string]dsl.Input{"release": {Type: dsl.InputTypeString, Required: true}},
				Contract: dsl.Contract{
					Goal: "Choose a target", DefinitionOfDone: "Target chosen", IterationCap: 2,
				},
				Graph: dsl.Graph{Nodes: []dsl.Node{
					{ID: "select", Class: dsl.NodeClassControl, Kind: string(dsl.ControlAsk), Params: params},
					{ID: "publish", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform),
						Params: dsl.NodeParams{"map": map[string]any{"ok": map[string]any{"value": true}}}},
				}, Edges: []dsl.Edge{{From: "select", To: "publish"}}},
			}
			resolved := compileCoordinatorControlDefinition(t, definition)
			loopRun := controlLoopRun("looprun-ask-"+strings.ReplaceAll(tt.name, " ", "-"), map[string]any{
				"release": "v2.4.1",
			})
			coordinatorRun := controlCoordinatorRun(loopRun, 1)
			defaults := DefaultLoopDefaults()
			seed := "72h"
			defaults.Delivery.RequestExpireAfter = &seed
			runner := newCoordinatorRunnerForControlTestWithDefaults(
				t, loopRun, coordinatorRun, nil,
				coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
					{Generation: 1, NodeID: "select", Status: generationOutputPending, Attempt: 1},
					{Generation: 1, NodeID: "publish", Status: generationOutputPending, Attempt: 1},
				}}},
				resolved,
				defaults,
			)
			runner.now = func() time.Time { return now }

			plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			payload := coordinatorSnapshotPayloadForTest(t, plan)
			outputs := outputsByNodeAndItemForTest(payload.Outputs)
			if plan.Terminal != nil || len(plan.NodeRuns) != 0 || !plan.Yield ||
				outputs["select/0"].Status != generationOutputWaiting {
				t.Fatalf("ask plan = %#v outputs = %#v, want parked without worker", plan, outputs)
			}
			if len(payload.Waits) != 1 || payload.Waits[0].Kind != NodeWaitKindRequest ||
				payload.Waits[0].NextEscalationAt == nil ||
				!payload.Waits[0].NextEscalationAt.Equal(now.Add(tt.wantAfter)) {
				t.Fatalf("ask waits = %#v, want request expiry after %s", payload.Waits, tt.wantAfter)
			}
			if len(payload.Requests) != 1 {
				t.Fatalf("ask requests = %#v, want one", payload.Requests)
			}
			request := payload.Requests[0]
			if request.Prompt != "Choose release target for v2.4.1" ||
				request.Agents != dsl.ResponderAgentsAllow ||
				!strings.Contains(string(request.Context), `"api_token":"[REDACTED]"`) ||
				strings.Contains(string(request.Context), "super-secret-token") ||
				!strings.Contains(string(request.ContextPreview), `"truncated":true`) {
				t.Fatalf("frozen request = %#v", request)
			}
		})
	}
}

func TestCoordinatorRunnerShouldParkGateApprovalWait(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 4, 16, 0, 0, 0, time.UTC)
	gateNode := dsl.Node{
		ID: "approval", Class: dsl.NodeClassControl, Kind: string(dsl.ControlGate),
		Criteria:      []dsl.GateCriterion{testRouteCriterion()},
		VerdictPolicy: dsl.VerdictPolicyFixedPasses,
		Expires:       &dsl.WaitExpiry{After: "15m", Route: "timed_out"},
	}
	gateNode.Normalize()
	gateNode.OnPause = []dsl.EffectSpec{{Emit: &dsl.EmitSpec{Kind: "approval_requested"}}}
	definition := dsl.Definition{Graph: dsl.Graph{
		Nodes: []dsl.Node{
			gateNode,
			lifecycleNode("normal"),
			lifecycleNode("timed_out"),
		},
		Edges: []dsl.Edge{{From: "approval", To: "normal"}, {From: "approval", To: "timed_out"}},
	}}
	loopRun := controlLoopRun("looprun-gate-approval-wait", nil)
	coordinatorRun := controlCoordinatorRun(loopRun, 1)
	runner := newCoordinatorRunnerForTestWithDefinition(
		t,
		loopRun,
		coordinatorRun,
		nil,
		coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
			{Generation: 1, NodeID: "approval", Status: generationOutputPending, Attempt: 1},
			{Generation: 1, NodeID: "normal", Status: generationOutputPending, Attempt: 1},
			{Generation: 1, NodeID: "timed_out", Status: generationOutputPending, Attempt: 1},
		}}},
		definition,
		WithCoordinatorGateEvaluator(gateEvaluatorFunc(
			func(context.Context, gate.Gate, gate.GateInput) (gate.Verdict, error) {
				return gate.Verdict{
					Outcome: gate.VerdictOutcomeAwaitingApproval,
					Criteria: []gate.CriterionResult{{
						ID: "operator", Type: dsl.CriterionHuman,
						Outcome: gate.VerdictOutcomeAwaitingApproval,
					}},
					Route: gate.RouteDecision{
						Placement: gate.PlacementInBody, Action: gate.RouteEscalate,
						TerminalStatus: string(StatusNeedsApproval), ReasonCode: "human_pending",
					},
				}, nil
			},
		)),
	)
	runner.now = func() time.Time { return now }

	plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	payload := coordinatorSnapshotPayloadForTest(t, plan)
	outputs := outputsByNodeAndItemForTest(payload.Outputs)
	if plan.Terminal == nil || plan.Terminal.Status != string(StatusNeedsApproval) ||
		outputs["approval/0"].Status != generationOutputSucceeded || outputs["approval/0"].Epoch != 1 {
		t.Fatalf("approval plan = %#v outputs = %#v", plan, outputs)
	}
	if len(payload.Waits) != 1 || payload.Waits[0].Kind != "approval_escalation" ||
		payload.Waits[0].IssuedEpoch != 1 || payload.Waits[0].NextEscalationAt == nil ||
		!payload.Waits[0].NextEscalationAt.Equal(now.Add(15*time.Minute)) {
		t.Fatalf("approval waits = %#v, want durable expiring approval", payload.Waits)
	}
	var waitEvent *GenerationLifecycleEventIntent
	for index := range payload.Events {
		if payload.Events[index].Kind == GenerationLifecycleEventNodeWaitStarted {
			waitEvent = &payload.Events[index]
			break
		}
	}
	if waitEvent == nil || len(waitEvent.Effects) != 1 || waitEvent.Effects[0].Trigger != EffectTriggerOnPause {
		t.Fatalf("approval wait events = %#v, want one on_pause delivery", payload.Events)
	}
}

func TestCoordinatorWaitExpiryRouteShouldSkipNormalPath(t *testing.T) {
	t.Parallel()

	graph := dsl.Graph{
		Nodes: []dsl.Node{
			{ID: "wait_for_ack", Class: dsl.NodeClassControl, Kind: string(dsl.ControlWait)},
			{ID: "normal", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform)},
			{ID: "timeout", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform)},
		},
		Edges: []dsl.Edge{{From: "wait_for_ack", To: "normal"}, {From: "wait_for_ack", To: "timeout"}},
	}
	outputs := []GenerationOutput{
		{Generation: 1, NodeID: "wait_for_ack", Status: generationOutputSucceeded,
			OutputRef: WaitExpiryRouteOutputRef("timeout"), Epoch: 2},
		{Generation: 1, NodeID: "normal", Status: generationOutputPending},
		{Generation: 1, NodeID: "timeout", Status: generationOutputPending},
	}
	applyWaitExpiryRoutes(graph, newControlTopology(graph), &outputs)
	mapped := outputsByNodeAndItemForTest(outputs)
	if mapped["normal/0"].Status != generationOutputSucceeded ||
		mapped["normal/0"].OutputRef != branchSkippedOutputRef ||
		mapped["timeout/0"].Status != generationOutputPending {
		t.Fatalf("expiry route outputs = %#v, want normal skipped and timeout pending", mapped)
	}
}

func TestCoordinatorRunnerShouldDriveFanOutAndCollectControls(t *testing.T) {
	t.Run("Should materialize chunks and hold collect until all branch items are terminal", func(t *testing.T) {
		t.Parallel()

		resolved := compileCoordinatorControlDefinition(t, fanOutControlDefinition(2, 1, 2))
		loopRun := controlLoopRun("looprun-fanout-barrier", map[string]any{})
		liveSpec := coordinatorLiveParticipationForTest(loopRun)
		loopRun.SetNetworkSpec(liveSpec)
		coordinatorRun := controlCoordinatorRun(loopRun, 1)
		loadRun := controlWorkerRun(loopRun, "load", 0, task.TaskRunStatusCompleted)
		runner := newCoordinatorRunnerForControlTest(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{coordinatorRun.ID: coordinatorRun, loadRun.ID: loadRun},
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
				{
					Generation: 1,
					NodeID:     "load",
					Status:     generationOutputEnqueued,
					OutputRef:  `{"items":[{"id":"A"},{"id":"B"},{"id":"C"}]}`,
					TaskRunID:  loadRun.ID,
				},
				{Generation: 1, NodeID: "fan", Status: generationOutputPending},
				{Generation: 1, NodeID: "collect", Status: generationOutputPending},
			}}},
			resolved,
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.Terminal != nil {
			t.Fatalf("Terminal = %#v, want nil while first chunk is queued", plan.Terminal)
		}
		if got, want := len(plan.NodeRuns), 1; got != want {
			t.Fatalf("node runs = %d, want %d", got, want)
		}
		if got, want := plan.NodeRuns[0].TaskID, coordinatorNodeTaskID(loopRun.ID, 1, "work", 0); got != want {
			t.Fatalf("queued task = %q, want %q", got, want)
		}
		if got := plan.NodeRuns[0].ResolvedNetworkParticipation; got == nil || *got != liveSpec {
			t.Fatalf("queued participation = %#v, want %#v", got, liveSpec)
		}
		var metadata map[string]any
		if err := json.Unmarshal(plan.NodeRuns[0].Metadata, &metadata); err != nil {
			t.Fatalf("unmarshal node metadata: %v", err)
		}
		if got, want := int(metadata["index"].(float64)), 0; got != want {
			t.Fatalf("metadata.index = %d, want %d", got, want)
		}
		item, ok := metadata["item"].([]any)
		if !ok || len(item) != 2 {
			t.Fatalf("metadata.item = %#v, want first two-item chunk", metadata["item"])
		}
		post := outputsByNodeAndItemForTest(coordinatorPostReservePayloadForTest(t, plan).Outputs)
		if got, want := post["fan/0"].Status, generationOutputSucceeded; got != want {
			t.Fatalf("fan status = %q, want %q", got, want)
		}
		if got, want := post["work/0"].Status, generationOutputEnqueued; got != want {
			t.Fatalf("work[0] status = %q, want %q", got, want)
		}
		if got, want := post["work/1"].Status, generationOutputPending; got != want {
			t.Fatalf("work[1] status = %q, want %q", got, want)
		}
		if got, want := post["collect/0"].Status, generationOutputPending; got != want {
			t.Fatalf("collect status = %q, want %q", got, want)
		}

		nextCoordinatorRun := controlCoordinatorRun(loopRun, 1)
		nextCoordinatorRun.ID = "run-coordinator-fanout-barrier-next"
		workZeroRun := controlWorkerRun(loopRun, "work", 0, task.TaskRunStatusCompleted)
		secondRunner := newCoordinatorRunnerForControlTest(
			t,
			loopRun,
			nextCoordinatorRun,
			map[string]task.Run{
				nextCoordinatorRun.ID: nextCoordinatorRun,
				loadRun.ID:            loadRun,
				workZeroRun.ID:        workZeroRun,
			},
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{
				1: postReserveOutputsForTest(post, workZeroRun.ID),
			}},
			resolved,
		)

		secondPlan, err := secondRunner.Run(context.Background(), task.RunID(nextCoordinatorRun.ID))
		if err != nil {
			t.Fatalf("second Run() error = %v", err)
		}
		if got, want := len(secondPlan.NodeRuns), 1; got != want {
			t.Fatalf("second node runs = %d, want %d", got, want)
		}
		if got, want := secondPlan.NodeRuns[0].TaskID, coordinatorNodeTaskID(loopRun.ID, 1, "work", 1); got != want {
			t.Fatalf("second queued task = %q, want %q", got, want)
		}
	})

	t.Run(
		"Should resolve externalized producer and fan-out payloads without replacing stored refs",
		func(t *testing.T) {
			t.Parallel()

			largeBody := strings.Repeat("x", LoopOutputInlineLimitBytes+1)
			producerPayload, err := json.Marshal(map[string]any{
				"items": []any{map[string]any{"id": "A", "body": largeBody}},
			})
			if err != nil {
				t.Fatalf("json.Marshal(producer payload) error = %v", err)
			}
			producerRef := OutputRefForPayload(producerPayload)
			resolved := compileCoordinatorControlDefinition(t, fanOutControlDefinition(1, 1, 2))
			loopRun := controlLoopRun("looprun-externalized-fanout", map[string]any{})
			coordinatorRun := controlCoordinatorRun(loopRun, 1)
			loadRun := controlWorkerRun(loopRun, "load", 0, task.TaskRunStatusCompleted)
			payloadKey := GenerationOutputPayloadKey{
				WorkspaceID: loopRun.WorkspaceID,
				RunID:       loopRun.ID,
				Generation:  1,
				NodeID:      "load",
				OutputRef:   producerRef,
			}
			outputStore := coordinatorRunnerOutputs{
				outputs: map[int][]GenerationOutput{1: {
					{Generation: 1, NodeID: "load", Status: generationOutputEnqueued,
						OutputRef: producerRef, TaskRunID: loadRun.ID},
					{Generation: 1, NodeID: "fan", Status: generationOutputPending},
					{Generation: 1, NodeID: "collect", Status: generationOutputPending},
				}},
				payloads:     map[GenerationOutputPayloadKey]json.RawMessage{payloadKey: producerPayload},
				payloadCalls: map[GenerationOutputPayloadKey]int{},
			}
			runner := newCoordinatorRunnerForControlTest(
				t,
				loopRun,
				coordinatorRun,
				map[string]task.Run{coordinatorRun.ID: coordinatorRun, loadRun.ID: loadRun},
				outputStore,
				resolved,
			)

			plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if got, want := outputStore.payloadCalls[payloadKey], 1; got != want {
				t.Fatalf("payload lookups = %d, want %d", got, want)
			}
			if got, want := len(plan.NodeRuns), 1; got != want {
				t.Fatalf("node runs = %d, want %d", got, want)
			}
			var metadata map[string]any
			if err := json.Unmarshal(plan.NodeRuns[0].Metadata, &metadata); err != nil {
				t.Fatalf("json.Unmarshal(node metadata) error = %v", err)
			}
			item, ok := metadata["item"].(map[string]any)
			if !ok || item["body"] != largeBody {
				t.Fatalf("metadata.item = %#v, want externalized producer item", metadata["item"])
			}
			payload := coordinatorPostReservePayloadForTest(t, plan)
			outputs := outputsByNodeAndItemForTest(payload.Outputs)
			if got := outputs["load/0"].OutputRef; got != producerRef {
				t.Fatalf("load output_ref = %q, want stored ref %q", got, producerRef)
			}
			fanRef := outputs["fan/0"].OutputRef
			if !OutputRefLooksContentAddressed(fanRef) || fanRef == producerRef {
				t.Fatalf("fan output_ref = %q, want distinct content-addressed materialization", fanRef)
			}
			if got, want := len(payload.OutputBlobs), 1; got != want {
				t.Fatalf("output blobs = %d, want %d fan-out materialization", got, want)
			}
			if payload.OutputBlobs[0].OutputRef != fanRef {
				t.Fatalf("materialization blob ref = %q, want %q", payload.OutputBlobs[0].OutputRef, fanRef)
			}
		},
	)

	t.Run("Should fail loudly when an externalized producer payload is missing", func(t *testing.T) {
		t.Parallel()

		producerRef := OutputRefForPayload(json.RawMessage(`{"items":[{"id":"A"}]}`))
		resolved := compileCoordinatorControlDefinition(t, fanOutControlDefinition(1, 1, 2))
		loopRun := controlLoopRun("looprun-missing-fanout-payload", map[string]any{})
		coordinatorRun := controlCoordinatorRun(loopRun, 1)
		loadRun := controlWorkerRun(loopRun, "load", 0, task.TaskRunStatusCompleted)
		runner := newCoordinatorRunnerForControlTest(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{coordinatorRun.ID: coordinatorRun, loadRun.ID: loadRun},
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
				{Generation: 1, NodeID: "load", Status: generationOutputEnqueued,
					OutputRef: producerRef, TaskRunID: loadRun.ID},
				{Generation: 1, NodeID: "fan", Status: generationOutputPending},
				{Generation: 1, NodeID: "collect", Status: generationOutputPending},
			}}},
			resolved,
		)

		_, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if !errors.Is(err, ErrOutputRefNotFound) {
			t.Fatalf("Run() error = %v, want %v", err, ErrOutputRefNotFound)
		}
	})

	t.Run("Should inspect an externalized result contract before declaring success", func(t *testing.T) {
		t.Parallel()

		failurePayload, err := json.Marshal(map[string]any{
			"error": "declared failure",
			"body":  strings.Repeat("x", LoopOutputInlineLimitBytes+1),
		})
		if err != nil {
			t.Fatalf("json.Marshal(failure payload) error = %v", err)
		}
		failureRef := OutputRefForPayload(failurePayload)
		resolved := compileCoordinatorControlDefinition(t, fanOutControlDefinition(1, 1, 2))
		loopRun := controlLoopRun("looprun-externalized-result-contract", map[string]any{})
		coordinatorRun := controlCoordinatorRun(loopRun, 1)
		loadRun := controlWorkerRun(loopRun, "load", 0, task.TaskRunStatusCompleted)
		payloadKey := GenerationOutputPayloadKey{
			WorkspaceID: loopRun.WorkspaceID,
			RunID:       loopRun.ID,
			Generation:  1,
			NodeID:      "load",
			OutputRef:   failureRef,
		}
		runner := newCoordinatorRunnerForControlTest(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{coordinatorRun.ID: coordinatorRun, loadRun.ID: loadRun},
			coordinatorRunnerOutputs{
				outputs: map[int][]GenerationOutput{1: {
					{Generation: 1, NodeID: "load", Status: generationOutputEnqueued,
						OutputRef: failureRef, TaskRunID: loadRun.ID},
					{Generation: 1, NodeID: "fan", Status: generationOutputPending},
					{Generation: 1, NodeID: "collect", Status: generationOutputPending},
				}},
				payloads: map[GenerationOutputPayloadKey]json.RawMessage{payloadKey: failurePayload},
			},
			resolved,
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		outputs := outputsByNodeAndItemForTest(coordinatorSnapshotPayloadForTest(t, plan).Outputs)
		load := outputs["load/0"]
		if load.Status != generationOutputFailed {
			t.Fatalf("load status = %q, want %q", load.Status, generationOutputFailed)
		}
		if OutputRefLooksContentAddressed(load.OutputRef) || !strings.Contains(load.OutputRef, "declared failure") {
			t.Fatalf("load output_ref = %q, want classified inline failure", load.OutputRef)
		}
	})
}

func TestCoordinatorRunnerShouldExhaustFanOutOverflow(t *testing.T) {
	t.Run("Should exhaust when materialized branches exceed max fan-out", func(t *testing.T) {
		t.Parallel()

		resolved := compileCoordinatorControlDefinition(t, fanOutControlDefinition(1, 2, 2))
		loopRun := controlLoopRun("looprun-fanout-overflow", map[string]any{})
		coordinatorRun := controlCoordinatorRun(loopRun, 1)
		loadRun := controlWorkerRun(loopRun, "load", 0, task.TaskRunStatusCompleted)
		runner := newCoordinatorRunnerForControlTest(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{coordinatorRun.ID: coordinatorRun, loadRun.ID: loadRun},
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
				{
					Generation: 1,
					NodeID:     "load",
					Status:     generationOutputEnqueued,
					OutputRef:  `{"items":[{"id":"A"},{"id":"B"},{"id":"C"}]}`,
					TaskRunID:  loadRun.ID,
				},
				{Generation: 1, NodeID: "fan", Status: generationOutputPending},
				{Generation: 1, NodeID: "collect", Status: generationOutputPending},
			}}},
			resolved,
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.Terminal == nil {
			t.Fatal("Terminal = nil, want exhausted terminal")
		}
		if got, want := plan.Terminal.Status, string(StatusExhausted); got != want {
			t.Fatalf("terminal status = %q, want %q", got, want)
		}
		if got, want := plan.Terminal.ReasonCode, "fan_out_width_exceeded"; got != want {
			t.Fatalf("reason = %q, want %q", got, want)
		}
		if got, want := len(plan.NodeRuns), 0; got != want {
			t.Fatalf("node runs = %d, want %d", got, want)
		}
		outputs := outputsByNodeAndItemForTest(coordinatorSnapshotPayloadForTest(t, plan).Outputs)
		if got, want := outputs["fan/0"].Status, generationOutputPending; got != want {
			t.Fatalf("fan output status = %q, want %q", got, want)
		}
	})
}

func TestCoordinatorRunnerShouldRouteBranchCondition(t *testing.T) {
	t.Run("Should route true branch and skip false branch with compiled CEL", func(t *testing.T) {
		t.Parallel()

		resolved := compileCoordinatorControlDefinition(t, branchControlDefinition())
		for _, tc := range []struct {
			name         string
			autoPush     bool
			wantNodeRuns int
			wantTerminal bool
			wantPush     string
		}{
			{name: "true edge", autoPush: true, wantNodeRuns: 1, wantPush: generationOutputEnqueued},
			{name: "false edge", autoPush: false, wantTerminal: true, wantPush: generationOutputSucceeded},
		} {
			t.Run("Should route "+tc.name, func(t *testing.T) {
				t.Parallel()

				loopRun := controlLoopRun("looprun-branch-"+tc.name, map[string]any{"route_enabled": tc.autoPush})
				coordinatorRun := controlCoordinatorRun(loopRun, 1)
				loadRun := controlWorkerRun(loopRun, "load", 0, task.TaskRunStatusCompleted)
				runner := newCoordinatorRunnerForControlTest(
					t,
					loopRun,
					coordinatorRun,
					map[string]task.Run{coordinatorRun.ID: coordinatorRun, loadRun.ID: loadRun},
					coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
						{
							Generation: 1,
							NodeID:     "load",
							Status:     generationOutputEnqueued,
							TaskRunID:  loadRun.ID,
						},
						{Generation: 1, NodeID: "should_route", Status: generationOutputPending},
						{Generation: 1, NodeID: "push", Status: generationOutputPending},
					}}},
					resolved,
				)

				plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
				if err != nil {
					t.Fatalf("Run() error = %v", err)
				}
				if got := len(plan.NodeRuns); got != tc.wantNodeRuns {
					t.Fatalf("node runs = %d, want %d", got, tc.wantNodeRuns)
				}
				if tc.wantTerminal && plan.Terminal == nil {
					t.Fatal("Terminal = nil, want done terminal")
				}
				payload := coordinatorSnapshotPayloadForTest(t, plan)
				if plan.PostReserveSnapshot != nil {
					payload = coordinatorPostReservePayloadForTest(t, plan)
				}
				outputs := outputsByNodeAndItemForTest(payload.Outputs)
				if got := outputs["push/0"].Status; got != tc.wantPush {
					t.Fatalf("push status = %q, want %q", got, tc.wantPush)
				}
			})
		}
	})

	t.Run("Should not skip a branch descendant that still has a live independent dependency", func(t *testing.T) {
		t.Parallel()

		resolved := compileCoordinatorControlDefinition(t, branchJoinControlDefinition())
		loopRun := controlLoopRun("looprun-branch-join", map[string]any{"route_enabled": false})
		coordinatorRun := controlCoordinatorRun(loopRun, 1)
		loadRun := controlWorkerRun(loopRun, "load", 0, task.TaskRunStatusCompleted)
		independentRun := controlWorkerRun(loopRun, "independent", 0, task.TaskRunStatusCompleted)
		runner := newCoordinatorRunnerForControlTest(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{
				coordinatorRun.ID: coordinatorRun,
				loadRun.ID:        loadRun,
				independentRun.ID: independentRun,
			},
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
				{Generation: 1, NodeID: "load", Status: generationOutputEnqueued, TaskRunID: loadRun.ID},
				{Generation: 1, NodeID: "independent", Status: generationOutputEnqueued, TaskRunID: independentRun.ID},
				{Generation: 1, NodeID: "should_route", Status: generationOutputPending},
				{Generation: 1, NodeID: "join", Status: generationOutputPending},
			}}},
			resolved,
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if got, want := len(plan.NodeRuns), 1; got != want {
			t.Fatalf("node runs = %d, want %d", got, want)
		}
		if got, want := plan.NodeRuns[0].TaskID, coordinatorNodeTaskID(loopRun.ID, 1, "join", 0); got != want {
			t.Fatalf("queued task = %q, want %q", got, want)
		}
		post := outputsByNodeAndItemForTest(coordinatorPostReservePayloadForTest(t, plan).Outputs)
		if got, want := post["join/0"].OutputRef, ""; got != want {
			t.Fatalf("join output_ref = %q, want not skipped", got)
		}
	})

	t.Run("Should skip every node in a multi-hop false branch tail", func(t *testing.T) {
		t.Parallel()

		resolved := compileCoordinatorControlDefinition(t, branchTailControlDefinition())
		loopRun := controlLoopRun("looprun-branch-tail", map[string]any{"route_enabled": false})
		coordinatorRun := controlCoordinatorRun(loopRun, 1)
		loadRun := controlWorkerRun(loopRun, "load", 0, task.TaskRunStatusCompleted)
		runner := newCoordinatorRunnerForControlTest(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{coordinatorRun.ID: coordinatorRun, loadRun.ID: loadRun},
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
				{Generation: 1, NodeID: "load", Status: generationOutputEnqueued, TaskRunID: loadRun.ID},
				{Generation: 1, NodeID: "should_route", Status: generationOutputPending},
				{Generation: 1, NodeID: "first", Status: generationOutputPending},
				{Generation: 1, NodeID: "second", Status: generationOutputPending},
			}}},
			resolved,
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if len(plan.NodeRuns) != 0 {
			t.Fatalf("node runs = %d, want no false-tail enqueue", len(plan.NodeRuns))
		}
		if plan.Terminal == nil {
			t.Fatal("Terminal = nil, want done terminal")
		}
		outputs := outputsByNodeAndItemForTest(coordinatorSnapshotPayloadForTest(t, plan).Outputs)
		for _, key := range []string{"first/0", "second/0"} {
			if got, want := outputs[key].OutputRef, branchSkippedOutputRef; got != want {
				t.Fatalf("%s output_ref = %q, want %q", key, got, want)
			}
		}
	})

	t.Run("Should settle a fan-out collect when a false branch bypasses the fan-out", func(t *testing.T) {
		t.Parallel()

		resolved := compileCoordinatorControlDefinition(t, branchFanOutControlDefinition())
		loopRun := controlLoopRun("looprun-branch-fanout", map[string]any{"has_issues": false})
		coordinatorRun := controlCoordinatorRun(loopRun, 1)
		loadRun := controlWorkerRun(loopRun, "load", 0, task.TaskRunStatusCompleted)
		runner := newCoordinatorRunnerForControlTest(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{coordinatorRun.ID: coordinatorRun, loadRun.ID: loadRun},
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
				{Generation: 1, NodeID: "load", Status: generationOutputEnqueued, TaskRunID: loadRun.ID},
				{Generation: 1, NodeID: "should_fix", Status: generationOutputPending},
				{Generation: 1, NodeID: "fan", Status: generationOutputPending},
				{Generation: 1, NodeID: "collect", Status: generationOutputPending},
				{Generation: 1, NodeID: "finalize", Status: generationOutputPending},
			}}},
			resolved,
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if len(plan.NodeRuns) != 0 {
			t.Fatalf("node runs = %d, want no false-branch enqueue", len(plan.NodeRuns))
		}
		payload := coordinatorSnapshotPayloadForTest(t, plan)
		if plan.Terminal == nil || plan.Terminal.Status != string(StatusDone) {
			t.Fatalf("Terminal = %#v, want done; outputs=%#v", plan.Terminal, payload.Outputs)
		}
		outputs := outputsByNodeAndItemForTest(payload.Outputs)
		for _, key := range []string{"fan/0", "collect/0", "finalize/0"} {
			if got, want := outputs[key].OutputRef, branchSkippedOutputRef; got != want {
				t.Fatalf("%s output_ref = %q, want %q", key, got, want)
			}
		}
	})

	t.Run("Should skip an unequal-length false branch diamond to a join", func(t *testing.T) {
		t.Parallel()

		resolved := compileCoordinatorControlDefinition(t, branchUnequalDiamondControlDefinition())
		loopRun := controlLoopRun("looprun-branch-diamond", map[string]any{"route_enabled": false})
		coordinatorRun := controlCoordinatorRun(loopRun, 1)
		loadRun := controlWorkerRun(loopRun, "load", 0, task.TaskRunStatusCompleted)
		runner := newCoordinatorRunnerForControlTest(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{coordinatorRun.ID: coordinatorRun, loadRun.ID: loadRun},
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
				{Generation: 1, NodeID: "load", Status: generationOutputEnqueued, TaskRunID: loadRun.ID},
				{Generation: 1, NodeID: "should_route", Status: generationOutputPending},
				{Generation: 1, NodeID: "short", Status: generationOutputPending},
				{Generation: 1, NodeID: "long_first", Status: generationOutputPending},
				{Generation: 1, NodeID: "long_second", Status: generationOutputPending},
				{Generation: 1, NodeID: "join", Status: generationOutputPending},
			}}},
			resolved,
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if len(plan.NodeRuns) != 0 {
			t.Fatalf("node runs = %d, want no false-diamond enqueue", len(plan.NodeRuns))
		}
		if plan.Terminal == nil {
			t.Fatal("Terminal = nil, want done terminal")
		}
		outputs := outputsByNodeAndItemForTest(coordinatorSnapshotPayloadForTest(t, plan).Outputs)
		for _, key := range []string{"short/0", "long_first/0", "long_second/0", "join/0"} {
			if got, want := outputs[key].OutputRef, branchSkippedOutputRef; got != want {
				t.Fatalf("%s output_ref = %q, want %q", key, got, want)
			}
		}
	})
}

func TestCoordinatorRunnerShouldExecuteSubLoopBody(t *testing.T) {
	t.Run("Should enqueue nested body nodes before downstream work", func(t *testing.T) {
		t.Parallel()

		resolved := compileCoordinatorControlDefinition(t, subLoopControlDefinition())
		loopRun := controlLoopRun("looprun-subloop-body", map[string]any{})
		coordinatorRun := controlCoordinatorRun(loopRun, 1)
		loadRun := controlWorkerRun(loopRun, "load", 0, task.TaskRunStatusCompleted)
		runner := newCoordinatorRunnerForControlTest(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{coordinatorRun.ID: coordinatorRun, loadRun.ID: loadRun},
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
				{Generation: 1, NodeID: "load", Status: generationOutputEnqueued, TaskRunID: loadRun.ID},
				{Generation: 1, NodeID: "nested", Status: generationOutputPending},
				{Generation: 1, NodeID: "nested__draft", Status: generationOutputPending},
				{Generation: 1, NodeID: "nested__review", Status: generationOutputPending},
				{Generation: 1, NodeID: "publish", Status: generationOutputPending},
			}}},
			resolved,
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if got, want := len(plan.NodeRuns), 1; got != want {
			t.Fatalf("node runs = %d, want %d", got, want)
		}
		if got, want := plan.NodeRuns[0].TaskID, coordinatorNodeTaskID(loopRun.ID, 1, "nested__draft", 0); got != want {
			t.Fatalf("queued task = %q, want %q", got, want)
		}
		post := outputsByNodeAndItemForTest(coordinatorPostReservePayloadForTest(t, plan).Outputs)
		if got, want := post["nested/0"].OutputRef, subLoopEnteredOutputRef; got != want {
			t.Fatalf("sub-loop output_ref = %q, want %q", got, want)
		}
		if got, want := post["nested__draft/0"].Status, generationOutputEnqueued; got != want {
			t.Fatalf("draft status = %q, want %q", got, want)
		}
		if got, want := post["publish/0"].Status, generationOutputPending; got != want {
			t.Fatalf("publish status = %q, want %q before nested body terminal", got, want)
		}
	})

	t.Run("Should release downstream work after nested body terminal node succeeds", func(t *testing.T) {
		t.Parallel()

		resolved := compileCoordinatorControlDefinition(t, subLoopControlDefinition())
		loopRun := controlLoopRun("looprun-subloop-release", map[string]any{})
		coordinatorRun := controlCoordinatorRun(loopRun, 1)
		draftRun := controlWorkerRun(loopRun, "nested__draft", 0, task.TaskRunStatusCompleted)
		reviewRun := controlWorkerRun(loopRun, "nested__review", 0, task.TaskRunStatusCompleted)
		runner := newCoordinatorRunnerForControlTest(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{
				coordinatorRun.ID: coordinatorRun,
				draftRun.ID:       draftRun,
				reviewRun.ID:      reviewRun,
			},
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
				{Generation: 1, NodeID: "load", Status: generationOutputSucceeded},
				{
					Generation: 1,
					NodeID:     "nested",
					Status:     generationOutputSucceeded,
					OutputRef:  subLoopEnteredOutputRef,
				},
				{Generation: 1, NodeID: "nested__draft", Status: generationOutputEnqueued, TaskRunID: draftRun.ID},
				{Generation: 1, NodeID: "nested__review", Status: generationOutputEnqueued, TaskRunID: reviewRun.ID},
				{Generation: 1, NodeID: "publish", Status: generationOutputPending},
			}}},
			resolved,
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if got, want := len(plan.NodeRuns), 1; got != want {
			t.Fatalf("node runs = %d, want %d", got, want)
		}
		if got, want := plan.NodeRuns[0].TaskID, coordinatorNodeTaskID(loopRun.ID, 1, "publish", 0); got != want {
			t.Fatalf("queued task = %q, want %q", got, want)
		}
	})

	t.Run("Should evaluate a nested branch against a sibling body output", func(t *testing.T) {
		t.Parallel()

		resolved := compileCoordinatorControlDefinition(t, subLoopBranchControlDefinition())
		loopRun := controlLoopRun("looprun-subloop-branch", map[string]any{})
		coordinatorRun := controlCoordinatorRun(loopRun, 1)
		runner := newCoordinatorRunnerForControlTest(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{coordinatorRun.ID: coordinatorRun},
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
				{Generation: 1, NodeID: "load", Status: generationOutputSucceeded},
				{
					Generation: 1,
					NodeID:     "nested",
					Status:     generationOutputSucceeded,
					OutputRef:  subLoopEnteredOutputRef,
				},
				{
					Generation: 1,
					NodeID:     "nested__draft",
					Status:     generationOutputSucceeded,
					OutputRef:  `{"approved":true}`,
				},
				{Generation: 1, NodeID: "nested__gate", Status: generationOutputPending},
				{Generation: 1, NodeID: "nested__publish_inner", Status: generationOutputPending},
				{Generation: 1, NodeID: "publish", Status: generationOutputPending},
			}}},
			resolved,
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if got, want := len(plan.NodeRuns), 1; got != want {
			t.Fatalf("node runs = %d, want %d", got, want)
		}
		if got, want := plan.NodeRuns[0].TaskID,
			coordinatorNodeTaskID(loopRun.ID, 1, "nested__publish_inner", 0); got != want {
			t.Fatalf("queued task = %q, want %q", got, want)
		}
	})

	t.Run("Should materialize nested fan-out from a sibling body output", func(t *testing.T) {
		t.Parallel()

		resolved := compileCoordinatorControlDefinition(t, subLoopFanOutControlDefinition())
		loopRun := controlLoopRun("looprun-subloop-fanout", map[string]any{})
		coordinatorRun := controlCoordinatorRun(loopRun, 1)
		runner := newCoordinatorRunnerForControlTest(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{coordinatorRun.ID: coordinatorRun},
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
				{Generation: 1, NodeID: "load", Status: generationOutputSucceeded},
				{
					Generation: 1,
					NodeID:     "nested",
					Status:     generationOutputSucceeded,
					OutputRef:  subLoopEnteredOutputRef,
				},
				{
					Generation: 1,
					NodeID:     "nested__draft",
					Status:     generationOutputSucceeded,
					OutputRef:  `{"items":[{"id":"A"},{"id":"B"}]}`,
				},
				{Generation: 1, NodeID: "nested__fan", Status: generationOutputPending},
				{Generation: 1, NodeID: "nested__collect", Status: generationOutputPending},
				{Generation: 1, NodeID: "publish", Status: generationOutputPending},
			}}},
			resolved,
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if got, want := len(plan.NodeRuns), 1; got != want {
			t.Fatalf("node runs = %d, want first nested fan-out branch", got)
		}
		if got, want := plan.NodeRuns[0].TaskID,
			coordinatorNodeTaskID(loopRun.ID, 1, "nested__work", 0); got != want {
			t.Fatalf("queued task = %q, want %q", got, want)
		}
		post := outputsByNodeAndItemForTest(coordinatorPostReservePayloadForTest(t, plan).Outputs)
		if got, want := post["nested__work/0"].Status, generationOutputEnqueued; got != want {
			t.Fatalf("nested work status = %q, want %q", got, want)
		}
		if got, want := post["nested__work/1"].Status, generationOutputPending; got != want {
			t.Fatalf("nested work[1] status = %q, want %q", got, want)
		}
	})
}

func TestCoordinatorRunnerShouldFailWhenSubLoopBodyFails(t *testing.T) {
	t.Run("Should propagate nested body failure to the parent generation", func(t *testing.T) {
		t.Parallel()

		resolved := compileCoordinatorControlDefinition(t, subLoopControlDefinition())
		loopRun := controlLoopRun("looprun-subloop-fail", map[string]any{})
		coordinatorRun := controlCoordinatorRun(loopRun, 1)
		runner := newCoordinatorRunnerForControlTest(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{coordinatorRun.ID: coordinatorRun},
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
				{Generation: 1, NodeID: "load", Status: generationOutputSucceeded},
				{
					Generation: 1,
					NodeID:     "nested",
					Status:     generationOutputSucceeded,
					OutputRef:  subLoopEnteredOutputRef,
				},
				{Generation: 1, NodeID: "nested__draft", Status: generationOutputFailed, OutputRef: "draft_failed"},
				{Generation: 1, NodeID: "nested__review", Status: generationOutputPending},
				{Generation: 1, NodeID: "publish", Status: generationOutputPending},
			}}},
			resolved,
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.NextCoordinator == nil {
			t.Fatal("NextCoordinator = nil, want retry after nested body failure")
		}
		next := outputsByNodeAndItemForTest(coordinatorPostReservePayloadForTest(t, plan).Outputs)
		if got, want := next["nested__draft/0"].Status, generationOutputPending; got != want {
			t.Fatalf("draft status = %q, want rerun %q", got, want)
		}
		if got, want := next["nested__review/0"].Status, generationOutputPending; got != want {
			t.Fatalf("review status = %q, want dependent rerun %q", got, want)
		}
		if got, want := next["publish/0"].Status, generationOutputPending; got != want {
			t.Fatalf("publish status = %q, want dependent rerun %q", got, want)
		}
		if got, want := len(plan.NodeRuns), 0; got != want {
			t.Fatalf("node runs = %d, want no enqueue after nested failure", got)
		}
	})
}

func TestCoordinatorRunnerShouldRerunOnlyFailedFanOutItem(t *testing.T) {
	t.Run("Should rerun only the failed chunk and transitive dependents", func(t *testing.T) {
		t.Parallel()

		resolved := compileCoordinatorControlDefinition(t, fanOutWithDependentDefinition())
		loopRun := controlLoopRun("looprun-failed-item-rerun", map[string]any{})
		loopRun.ReattemptStrategy = ReattemptFailedOnly
		coordinatorRun := controlCoordinatorRun(loopRun, 1)
		runner := newCoordinatorRunnerForControlTest(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{coordinatorRun.ID: coordinatorRun},
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: failedItemOutputsForTest(t)}},
			resolved,
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.NextCoordinator == nil {
			t.Fatal("NextCoordinator = nil, want retry coordinator")
		}
		next := outputsByNodeAndItemForTest(coordinatorPostReservePayloadForTest(t, plan).Outputs)
		if got, want := next["work/0"].Status, generationOutputSucceeded; got != want {
			t.Fatalf("work[0] status = %q, want carried %q", got, want)
		}
		if got, want := next["after/0"].Status, generationOutputSucceeded; got != want {
			t.Fatalf("after[0] status = %q, want carried %q", got, want)
		}
		if got, want := next["work/1"].Status, generationOutputPending; got != want {
			t.Fatalf("work[1] status = %q, want rerun %q", got, want)
		}
		if got, want := next["after/1"].Status, generationOutputPending; got != want {
			t.Fatalf("after[1] status = %q, want rerun %q", got, want)
		}
		if got, want := next["collect/0"].Status, generationOutputPending; got != want {
			t.Fatalf("collect status = %q, want rerun %q", got, want)
		}
	})
}

func fanOutControlDefinition(batchSize int, maxParallel int, maxFanOut int) dsl.Definition {
	return dsl.Definition{
		APIVersion: dsl.APIVersion,
		Kind:       dsl.KindLoop,
		Meta:       dsl.Meta{Name: "delivery"},
		Inputs:     map[string]dsl.Input{},
		Contract:   dsl.Contract{Goal: "test", DefinitionOfDone: "done", IterationCap: 3},
		Graph: dsl.Graph{
			Nodes: []dsl.Node{
				{
					ID:    "load",
					Class: dsl.NodeClassAction,
					Kind:  string(dsl.ActionTransform),
					Params: dsl.NodeParams{
						"map": map[string]any{"items": map[string]any{"value": []any{}}},
					},
					Produces: dsl.Schema{
						"items": []any{map[string]any{"id": "string"}},
					},
				},
				{
					ID:          "fan",
					Class:       dsl.NodeClassControl,
					Kind:        string(dsl.ControlFanOut),
					Collection:  "{{ .nodes.load.output.items }}",
					BatchSize:   batchSize,
					MaxParallel: maxParallel,
					MaxFanOut:   maxFanOut,
				},
				{
					ID:    "work",
					Class: dsl.NodeClassAction,
					Kind:  string(dsl.ActionRunAgent),
					Params: dsl.NodeParams{
						"agent":  "codex",
						"prompt": "Handle {{ .item }}",
					},
				},
				{ID: "collect", Class: dsl.NodeClassControl, Kind: string(dsl.ControlCollect)},
			},
			Edges: []dsl.Edge{
				{From: "load", To: "fan"},
				{From: "fan", To: "work"},
				{From: "work", To: "collect"},
			},
		},
	}
}

func fanOutWithDependentDefinition() dsl.Definition {
	def := fanOutControlDefinition(1, 2, 2)
	def.Graph.Nodes = append(def.Graph.Nodes[:3], append([]dsl.Node{
		{
			ID:    "after",
			Class: dsl.NodeClassAction,
			Kind:  string(dsl.ActionTransform),
			Params: dsl.NodeParams{
				"map": map[string]any{"ok": map[string]any{"value": true}},
			},
		},
	}, def.Graph.Nodes[3:]...)...)
	def.Graph.Edges = []dsl.Edge{
		{From: "load", To: "fan"},
		{From: "fan", To: "work"},
		{From: "work", To: "after"},
		{From: "after", To: "collect"},
	}
	return def
}

func branchControlDefinition() dsl.Definition {
	return dsl.Definition{
		APIVersion: dsl.APIVersion,
		Kind:       dsl.KindLoop,
		Meta:       dsl.Meta{Name: "delivery"},
		Inputs: map[string]dsl.Input{
			"route_enabled": {Type: dsl.InputTypeBoolean},
		},
		Contract: dsl.Contract{Goal: "test", DefinitionOfDone: "done", IterationCap: 3},
		Graph: dsl.Graph{
			Nodes: []dsl.Node{
				{ID: "load", Class: dsl.NodeClassSource, Kind: string(dsl.SourceInput), InputRef: "route_enabled"},
				{
					ID:        "should_route",
					Class:     dsl.NodeClassControl,
					Kind:      string(dsl.ControlBranch),
					Condition: "inputs.route_enabled == true",
				},
				{
					ID:    "push",
					Class: dsl.NodeClassAction,
					Kind:  string(dsl.ActionTransform),
					Params: dsl.NodeParams{
						"map": map[string]any{"pushed": map[string]any{"value": true}},
					},
				},
			},
			Edges: []dsl.Edge{
				{From: "load", To: "should_route"},
				{From: "should_route", To: "push"},
			},
		},
	}
}

func branchFanOutControlDefinition() dsl.Definition {
	return dsl.Definition{
		APIVersion: dsl.APIVersion,
		Kind:       dsl.KindLoop,
		Meta:       dsl.Meta{Name: "review-and-fix"},
		Inputs: map[string]dsl.Input{
			"has_issues": {Type: dsl.InputTypeBoolean},
		},
		Contract: dsl.Contract{Goal: "test", DefinitionOfDone: "done", IterationCap: 3},
		Graph: dsl.Graph{
			Nodes: []dsl.Node{
				{
					ID:    "load",
					Class: dsl.NodeClassAction,
					Kind:  string(dsl.ActionTransform),
					Params: dsl.NodeParams{
						"map": map[string]any{"items": map[string]any{"value": []any{}}},
					},
					Produces: dsl.Schema{"items": []any{map[string]any{"id": "string"}}},
				},
				{
					ID:        "should_fix",
					Class:     dsl.NodeClassControl,
					Kind:      string(dsl.ControlBranch),
					Condition: "inputs.has_issues == true",
				},
				{
					ID:          "fan",
					Class:       dsl.NodeClassControl,
					Kind:        string(dsl.ControlFanOut),
					Collection:  "{{ .nodes.load.output.items }}",
					BatchSize:   1,
					MaxParallel: 1,
					MaxFanOut:   2,
				},
				{
					ID:    "work",
					Class: dsl.NodeClassAction,
					Kind:  string(dsl.ActionTransform),
					Params: dsl.NodeParams{
						"map": map[string]any{"ok": map[string]any{"value": true}},
					},
				},
				{ID: "collect", Class: dsl.NodeClassControl, Kind: string(dsl.ControlCollect)},
				{
					ID:    "finalize",
					Class: dsl.NodeClassAction,
					Kind:  string(dsl.ActionTransform),
					Params: dsl.NodeParams{
						"map": map[string]any{"ok": map[string]any{"value": true}},
					},
				},
			},
			Edges: []dsl.Edge{
				{From: "load", To: "should_fix"},
				{From: "should_fix", To: "fan"},
				{From: "fan", To: "work"},
				{From: "work", To: "collect"},
				{From: "collect", To: "finalize"},
			},
		},
	}
}

func branchJoinControlDefinition() dsl.Definition {
	def := branchControlDefinition()
	def.Graph.Nodes = append(def.Graph.Nodes[:1], append([]dsl.Node{
		{ID: "independent", Class: dsl.NodeClassSource, Kind: string(dsl.SourceInput), InputRef: "route_enabled"},
	}, def.Graph.Nodes[1:]...)...)
	def.Graph.Nodes[3] = dsl.Node{
		ID:    "join",
		Class: dsl.NodeClassAction,
		Kind:  string(dsl.ActionTransform),
		Params: dsl.NodeParams{
			"map": map[string]any{"joined": map[string]any{"value": true}},
		},
	}
	def.Graph.Edges = []dsl.Edge{
		{From: "load", To: "should_route"},
		{From: "should_route", To: "join"},
		{From: "independent", To: "join"},
	}
	return def
}

func branchTailControlDefinition() dsl.Definition {
	def := branchControlDefinition()
	def.Graph.Nodes[2] = dsl.Node{
		ID:    "first",
		Class: dsl.NodeClassAction,
		Kind:  string(dsl.ActionTransform),
		Params: dsl.NodeParams{
			"map": map[string]any{"first": map[string]any{"value": true}},
		},
	}
	def.Graph.Nodes = append(def.Graph.Nodes, dsl.Node{
		ID:    "second",
		Class: dsl.NodeClassAction,
		Kind:  string(dsl.ActionTransform),
		Params: dsl.NodeParams{
			"map": map[string]any{"second": map[string]any{"value": true}},
		},
	})
	def.Graph.Edges = []dsl.Edge{
		{From: "load", To: "should_route"},
		{From: "should_route", To: "first"},
		{From: "first", To: "second"},
	}
	return def
}

func branchUnequalDiamondControlDefinition() dsl.Definition {
	def := branchControlDefinition()
	def.Graph.Nodes[2] = dsl.Node{
		ID:    "short",
		Class: dsl.NodeClassAction,
		Kind:  string(dsl.ActionTransform),
		Params: dsl.NodeParams{
			"map": map[string]any{"short": map[string]any{"value": true}},
		},
	}
	def.Graph.Nodes = append(def.Graph.Nodes,
		dsl.Node{
			ID:    "long_first",
			Class: dsl.NodeClassAction,
			Kind:  string(dsl.ActionTransform),
			Params: dsl.NodeParams{
				"map": map[string]any{"long_first": map[string]any{"value": true}},
			},
		},
		dsl.Node{
			ID:    "long_second",
			Class: dsl.NodeClassAction,
			Kind:  string(dsl.ActionTransform),
			Params: dsl.NodeParams{
				"map": map[string]any{"long_second": map[string]any{"value": true}},
			},
		},
		dsl.Node{
			ID:    "join",
			Class: dsl.NodeClassAction,
			Kind:  string(dsl.ActionTransform),
			Params: dsl.NodeParams{
				"map": map[string]any{"joined": map[string]any{"value": true}},
			},
		},
	)
	def.Graph.Edges = []dsl.Edge{
		{From: "load", To: "should_route"},
		{From: "should_route", To: "short"},
		{From: "short", To: "join"},
		{From: "should_route", To: "long_first"},
		{From: "long_first", To: "long_second"},
		{From: "long_second", To: "join"},
	}
	return def
}

func subLoopControlDefinition() dsl.Definition {
	return dsl.Definition{
		APIVersion: dsl.APIVersion,
		Kind:       dsl.KindLoop,
		Meta:       dsl.Meta{Name: "delivery"},
		Inputs:     map[string]dsl.Input{},
		Contract:   dsl.Contract{Goal: "test", DefinitionOfDone: "done", IterationCap: 3},
		Graph: dsl.Graph{
			Nodes: []dsl.Node{
				{
					ID:    "load",
					Class: dsl.NodeClassAction,
					Kind:  string(dsl.ActionTransform),
					Params: dsl.NodeParams{
						"map": map[string]any{"items": map[string]any{"value": []any{}}},
					},
				},
				{
					ID:    "nested",
					Class: dsl.NodeClassControl,
					Kind:  string(dsl.ControlSubLoop),
					Body: &dsl.Graph{
						Nodes: []dsl.Node{
							{
								ID:    "draft",
								Class: dsl.NodeClassAction,
								Kind:  string(dsl.ActionTransform),
								Params: dsl.NodeParams{
									"map": map[string]any{"drafted": map[string]any{"value": true}},
								},
							},
							{
								ID:    "review",
								Class: dsl.NodeClassAction,
								Kind:  string(dsl.ActionTransform),
								Params: dsl.NodeParams{
									"map": map[string]any{"reviewed": map[string]any{"value": true}},
								},
							},
						},
						Edges: []dsl.Edge{{From: "draft", To: "review"}},
					},
				},
				{
					ID:    "publish",
					Class: dsl.NodeClassAction,
					Kind:  string(dsl.ActionTransform),
					Params: dsl.NodeParams{
						"map": map[string]any{"published": map[string]any{"value": true}},
					},
				},
			},
			Edges: []dsl.Edge{
				{From: "load", To: "nested"},
				{From: "nested", To: "publish"},
			},
		},
	}
}

func subLoopBranchControlDefinition() dsl.Definition {
	def := subLoopControlDefinition()
	nested := requireSubLoopBodyForTest(&def, "nested")
	nested.Nodes = []dsl.Node{
		{
			ID:    "draft",
			Class: dsl.NodeClassAction,
			Kind:  string(dsl.ActionTransform),
			Params: dsl.NodeParams{
				"map": map[string]any{"approved": map[string]any{"value": true}},
			},
			Produces: dsl.Schema{"approved": "boolean"},
		},
		{
			ID:        "gate",
			Class:     dsl.NodeClassControl,
			Kind:      string(dsl.ControlBranch),
			Condition: "nodes.draft.output.approved == true",
		},
		{
			ID:    "publish_inner",
			Class: dsl.NodeClassAction,
			Kind:  string(dsl.ActionTransform),
			Params: dsl.NodeParams{
				"map": map[string]any{"ok": map[string]any{"value": true}},
			},
		},
	}
	nested.Edges = []dsl.Edge{
		{From: "draft", To: "gate"},
		{From: "gate", To: "publish_inner"},
	}
	return def
}

func subLoopFanOutControlDefinition() dsl.Definition {
	def := subLoopControlDefinition()
	nested := requireSubLoopBodyForTest(&def, "nested")
	nested.Nodes = []dsl.Node{
		{
			ID:    "draft",
			Class: dsl.NodeClassAction,
			Kind:  string(dsl.ActionTransform),
			Params: dsl.NodeParams{
				"map": map[string]any{"items": map[string]any{"value": []any{}}},
			},
			Produces: dsl.Schema{"items": []any{map[string]any{"id": "string"}}},
		},
		{
			ID:          "fan",
			Class:       dsl.NodeClassControl,
			Kind:        string(dsl.ControlFanOut),
			Collection:  "{{ .nodes.draft.output.items }}",
			BatchSize:   1,
			MaxParallel: 1,
			MaxFanOut:   2,
		},
		{
			ID:    "work",
			Class: dsl.NodeClassAction,
			Kind:  string(dsl.ActionTransform),
			Params: dsl.NodeParams{
				"map": map[string]any{"ok": map[string]any{"value": true}},
			},
		},
		{ID: "collect", Class: dsl.NodeClassControl, Kind: string(dsl.ControlCollect)},
	}
	nested.Edges = []dsl.Edge{
		{From: "draft", To: "fan"},
		{From: "fan", To: "work"},
		{From: "work", To: "collect"},
	}
	return def
}

func requireSubLoopBodyForTest(def *dsl.Definition, nodeID dsl.NodeID) *dsl.Graph {
	for idx := range def.Graph.Nodes {
		node := &def.Graph.Nodes[idx]
		if node.ID == nodeID {
			return node.Body
		}
	}
	return nil
}

func compileCoordinatorControlDefinition(t *testing.T, def dsl.Definition) *ResolvedDefinition {
	t.Helper()

	resolved, err := NewCompiler().Compile(def)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	return resolved
}

func newCoordinatorRunnerForControlTest(
	t *testing.T,
	loopRun Run,
	coordinatorRun task.Run,
	runs map[string]task.Run,
	outputs GenerationOutputReader,
	resolved *ResolvedDefinition,
) *CoordinatorRunner {
	t.Helper()
	defaults := LoopDefaults{
		Delivery: definitionConfigLayer(resolved.Definition),
		Watch:    definitionConfigLayer(resolved.Definition),
	}
	return newCoordinatorRunnerForControlTestWithDefaults(
		t, loopRun, coordinatorRun, runs, outputs, resolved, defaults,
	)
}

func newCoordinatorRunnerForControlTestWithDefaults(
	t *testing.T,
	loopRun Run,
	coordinatorRun task.Run,
	runs map[string]task.Run,
	outputs GenerationOutputReader,
	resolved *ResolvedDefinition,
	defaults LoopDefaults,
) *CoordinatorRunner {
	t.Helper()
	if runs == nil {
		runs = map[string]task.Run{coordinatorRun.ID: coordinatorRun}
	}
	effective, err := ResolveEffectiveConfig(resolved, defaults, nil, LoopConfig{})
	if err != nil {
		t.Fatalf("ResolveEffectiveConfig() error = %v", err)
	}
	loopRun, snapshot := pinCoordinatorResolvedForTest(t, loopRun, resolved, effective)
	runner, err := NewCoordinatorRunner(
		&coordinatorRunnerTaskRunReader{runs: runs},
		&coordinatorRunnerLoopStore{run: loopRun, snapshot: snapshot},
		outputs,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	if err != nil {
		t.Fatalf("NewCoordinatorRunner() error = %v", err)
	}
	return runner
}

func controlLoopRun(id string, inputs map[string]any) Run {
	return Run{
		ID:                RunID(id),
		WorkspaceID:       "ws-1",
		LoopName:          "delivery",
		Status:            StatusRunning,
		Generation:        1,
		ReattemptStrategy: ReattemptFailedOnly,
		IterationCap:      3,
		Inputs:            inputs,
	}
}

func coordinatorLiveParticipationForTest(run Run) participation.Spec {
	return participation.Spec{
		Version:         participation.SpecVersion,
		Mode:            participation.ModeLive,
		WorkspaceID:     string(run.WorkspaceID),
		ChannelStrategy: participation.StrategyLoopRun,
		ChannelID:       string(run.ID),
		Source:          participation.SourceLoopDefinition,
		Bounds: participation.Bounds{
			MaxWakes: 2, MaxWakeWallTime: "1s", MaxTotalWallTime: "2s",
			MaxInputTokens: 100, MaxOutputTokens: 100, MaxWakeDepth: 2, CoalesceWindow: "100ms",
		},
	}
}

func controlCoordinatorRun(run Run, generation int) task.Run {
	return task.Run{
		ID:        "run-coordinator-" + string(run.ID),
		TaskID:    "task-coordinator-" + string(run.ID),
		RunKind:   task.RunKindCoordinator,
		LoopRunID: string(run.ID),
		Status:    task.TaskRunStatusClaimed,
		Metadata:  json.RawMessage(`{"generation":` + strconv.Itoa(generation) + `}`),
	}
}

func controlWorkerRun(run Run, nodeID dsl.NodeID, itemIndex int, status task.RunStatus) task.Run {
	return task.Run{
		ID:        coordinatorNodeRunID(run.ID, 1, nodeID, itemIndex),
		TaskID:    coordinatorNodeTaskID(run.ID, 1, nodeID, itemIndex),
		RunKind:   task.RunKindWorker,
		LoopRunID: string(run.ID),
		Status:    status,
	}
}

func outputsByNodeAndItemForTest(outputs []GenerationOutput) map[string]GenerationOutput {
	mapped := make(map[string]GenerationOutput, len(outputs))
	for _, output := range outputs {
		mapped[output.NodeID+"/"+strconv.Itoa(output.ItemIndex)] = output
	}
	return mapped
}

func TestCoordinatorControlHelpersShouldFormatMultiDigitIndexes(t *testing.T) {
	t.Parallel()

	t.Run("Should format generation and item indexes as decimal strings", func(t *testing.T) {
		t.Parallel()

		run := controlCoordinatorRun(controlLoopRun("looprun-format", nil), 10)
		if got, want := string(run.Metadata), `{"generation":10}`; got != want {
			t.Fatalf("coordinator metadata = %s, want %s", got, want)
		}
		outputs := outputsByNodeAndItemForTest([]GenerationOutput{{
			NodeID:    "worker",
			ItemIndex: 10,
			Status:    generationOutputSucceeded,
		}})
		if _, ok := outputs["worker/10"]; !ok {
			t.Fatalf("outputs keys = %#v, want worker/10", outputs)
		}
	})
}

func postReserveOutputsForTest(outputs map[string]GenerationOutput, workZeroRunID string) []GenerationOutput {
	next := make([]GenerationOutput, 0, len(outputs))
	for _, output := range outputs {
		if output.NodeID == "work" && output.ItemIndex == 0 {
			output.Status = generationOutputEnqueued
			output.TaskRunID = workZeroRunID
		}
		next = append(next, output)
	}
	return next
}

func failedItemOutputsForTest(t *testing.T) []GenerationOutput {
	t.Helper()

	ref, err := fanOutMaterializationRef(fanOutMaterialization{
		Kind:        fanOutMaterializationKind,
		Branches:    2,
		BatchSize:   1,
		MaxParallel: 2,
		Chunks:      [][]any{{map[string]any{"id": "A"}}, {map[string]any{"id": "B"}}},
	})
	if err != nil {
		t.Fatalf("fanOutMaterializationRef() error = %v", err)
	}
	return []GenerationOutput{
		{
			Generation: 1,
			NodeID:     "load",
			Status:     generationOutputSucceeded,
			OutputRef:  `{"items":[{"id":"A"},{"id":"B"}]}`,
		},
		{Generation: 1, NodeID: "fan", Status: generationOutputSucceeded, OutputRef: ref},
		{
			Generation: 1,
			NodeID:     "work",
			ItemIndex:  0,
			Status:     generationOutputSucceeded,
			OutputRef:  `{"ok":true}`,
			TaskRunID:  "run-work-0",
		},
		{
			Generation: 1,
			NodeID:     "work",
			ItemIndex:  1,
			Status:     generationOutputFailed,
			OutputRef:  "worker_failed",
			TaskRunID:  "run-work-1",
		},
		{
			Generation: 1,
			NodeID:     "after",
			ItemIndex:  0,
			Status:     generationOutputSucceeded,
			OutputRef:  `{"ok":true}`,
			TaskRunID:  "run-after-0",
		},
		{
			Generation: 1,
			NodeID:     "after",
			ItemIndex:  1,
			Status:     generationOutputPending,
		},
		{
			Generation: 1,
			NodeID:     "collect",
			Status:     generationOutputPending,
		},
	}
}
