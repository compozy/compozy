package loop

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/deadentity"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/dsl/refs"
	"github.com/compozy/compozy/internal/loop/gate"
	"github.com/compozy/compozy/internal/task"
)

func taskRunWithResult(run task.Run, result json.RawMessage) task.Run {
	run.SetResult(result)
	return run
}

func TestCoordinatorOperatorRerunPlanner(t *testing.T) {
	t.Parallel()

	graph := dsl.Graph{
		Nodes: []dsl.Node{
			{ID: "load", Class: dsl.NodeClassSource, Kind: string(dsl.SourceInput)},
			{ID: "fan", Class: dsl.NodeClassControl, Kind: string(dsl.ControlFanOut)},
			{ID: "shard", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent)},
			{ID: "publish", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent)},
		},
		Edges: []dsl.Edge{{From: "load", To: "fan"}, {From: "fan", To: "shard"}, {From: "shard", To: "publish"}},
	}

	t.Run(
		"Should rerun the selected lane and transitive dependents while carrying siblings UT-070 UT-075",
		func(t *testing.T) {
			t.Parallel()

			lane := 1
			current := []GenerationOutput{
				{Generation: 2, NodeID: "load", Status: generationOutputSucceeded, OutputRef: "load"},
				{Generation: 2, NodeID: "fan", Status: generationOutputSucceeded, OutputRef: "fan"},
				{Generation: 2, NodeID: "shard", ItemIndex: 0, Status: generationOutputSucceeded, OutputRef: "s0"},
				{Generation: 2, NodeID: "shard", ItemIndex: 1, Status: generationOutputFailed, OutputRef: "s1"},
				{Generation: 2, NodeID: "publish", ItemIndex: 0, Status: generationOutputSucceeded, OutputRef: "p0"},
				{Generation: 2, NodeID: "publish", ItemIndex: 1, Status: generationOutputFailed, OutputRef: "p1"},
			}
			next, labels, err := planOperatorRerun(graph, current, "shard", &lane, 3)
			if err != nil {
				t.Fatalf("planOperatorRerun() error = %v", err)
			}
			if want := []string{"publish[1]", "shard[1]"}; !reflect.DeepEqual(labels, want) {
				t.Fatalf("rerun labels = %#v, want %#v", labels, want)
			}
			byCell := make(map[generationOutputKey]GenerationOutput, len(next))
			for _, output := range next {
				byCell[generationOutputKey{nodeID: output.NodeID, itemIndex: output.ItemIndex}] = output
			}
			if got := byCell[generationOutputKey{nodeID: "shard", itemIndex: 1}]; got.Status != generationOutputPending ||
				got.OutputRef != "" {
				t.Fatalf("selected shard lane = %#v, want pending without carried output", got)
			}
			if got := byCell[generationOutputKey{nodeID: "publish", itemIndex: 1}]; got.Status != generationOutputPending ||
				got.OutputRef != "" {
				t.Fatalf("dependent publish lane = %#v, want pending without carried output", got)
			}
			if got := byCell[generationOutputKey{nodeID: "shard", itemIndex: 0}]; got.Status != generationOutputSucceeded ||
				got.OutputRef != "s0" {
				t.Fatalf("sibling shard lane = %#v, want carried", got)
			}
			if got := byCell[generationOutputKey{nodeID: "load", itemIndex: 0}]; got.Generation != 3 ||
				got.OutputRef != "load" {
				t.Fatalf("upstream load = %#v, want generation-3 carry", got)
			}
		},
	)

	t.Run("Should reject parked and unsettled targets UT-073", func(t *testing.T) {
		t.Parallel()

		for _, status := range []string{generationOutputWaiting, generationOutputRunning} {
			_, _, err := planOperatorRerun(graph, []GenerationOutput{{
				Generation: 2, NodeID: "shard", Status: status,
			}}, "shard", nil, 3)
			if !errors.Is(err, ErrRerunNodeUnsettled) {
				t.Fatalf("planOperatorRerun(status=%q) error = %v, want ErrRerunNodeUnsettled", status, err)
			}
		}
	})
}

func TestCoordinatorRunnerShouldMaterializeReadyLayerPlan(t *testing.T) {
	t.Run("Should materialize ready layer plan", func(t *testing.T) {
		t.Parallel()

		loopRun := Run{
			ID:          "looprun-ready-layer",
			WorkspaceID: "ws-1",
			LoopName:    "delivery",
			Status:      StatusRunning,
			Generation:  0,
			Inputs: map[string]any{
				"load": "tasks",
			},
		}
		taskRun := task.Run{
			ID:        "run-coordinator-ready-layer",
			TaskID:    "task-coordinator-ready-layer",
			RunKind:   task.RunKindCoordinator,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusClaimed,
		}
		resolved := resolvedCoordinatorDefinitionForTest(t, dsl.Definition{
			Inputs: map[string]dsl.Input{"load": {Type: dsl.InputTypeString}},
			Graph: dsl.Graph{
				Nodes: []dsl.Node{
					{
						ID:       "load",
						Class:    dsl.NodeClassSource,
						Kind:     string(dsl.SourceInput),
						InputRef: "load",
					},
					{
						ID:    "agent",
						Class: dsl.NodeClassAction,
						Kind:  string(dsl.ActionRunAgent),
						Params: dsl.NodeParams{
							"agent":  "codex",
							"prompt": "Process the loaded input",
						},
					},
				},
				Edges: []dsl.Edge{{From: "load", To: "agent"}},
			},
		})
		loopRun, snapshot := pinCoordinatorResolvedForTest(
			t,
			loopRun,
			resolved,
			snapshotEffectiveConfig(),
		)
		runner, err := NewCoordinatorRunner(
			&coordinatorRunnerTaskRunReader{run: taskRun},
			&coordinatorRunnerLoopStore{run: loopRun, snapshot: snapshot},
			coordinatorRunnerOutputs{},
			slog.New(slog.NewTextHandler(io.Discard, nil)),
		)
		if err != nil {
			t.Fatalf("NewCoordinatorRunner() error = %v", err)
		}

		plan, err := runner.Run(context.Background(), task.RunID(taskRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if got, want := len(plan.NodeTasks), 1; got != want {
			t.Fatalf("node tasks = %d, want %d", got, want)
		}
		if got, want := len(plan.Dependencies), 0; got != want {
			t.Fatalf("dependencies = %d, want %d", got, want)
		}
		if got, want := len(plan.NodeRuns), 1; got != want {
			t.Fatalf("node runs = %d, want %d", got, want)
		}
		if got, want := plan.NodeRuns[0].TaskID, coordinatorNodeTaskID(loopRun.ID, 1, "agent", 0); got != want {
			t.Fatalf("agent node run task_id = %q, want %q", got, want)
		}
		if plan.Terminal != nil {
			t.Fatalf("plan.Terminal = %#v, want nil while root node is ready", plan.Terminal)
		}
		payload, ok := plan.Snapshot.Payload.(GenerationSnapshotPayload)
		if !ok {
			t.Fatalf(
				"snapshot payload type = %T, want GenerationSnapshotPayload",
				plan.Snapshot.Payload,
			)
		}
		if got, want := len(payload.Outputs), 2; got != want {
			t.Fatalf("snapshot outputs = %d, want %d", got, want)
		}
		if payload.Outputs[0].Status != "succeeded" || payload.Outputs[1].Status != "pending" {
			t.Fatalf("snapshot output statuses = %#v, want succeeded then pending", payload.Outputs)
		}
		postReserve := coordinatorPostReservePayloadForTest(t, plan)
		if postReserve.Outputs[0].Status != "succeeded" || postReserve.Outputs[1].Status != "enqueued" {
			t.Fatalf("post-reserve output statuses = %#v, want succeeded then enqueued", postReserve.Outputs)
		}
	})
}

func TestCoordinatorRunnerShouldResolveFanOutFromWorkspaceDefaults(t *testing.T) {
	t.Run("Should use the Run-pinned fan-out width", func(t *testing.T) {
		t.Parallel()

		resolved := resolvedCoordinatorDefinitionForTest(t, dsl.Definition{
			Graph: dsl.Graph{
				Nodes: []dsl.Node{
					{ID: "load", Class: dsl.NodeClassSource, Kind: string(dsl.SourceInput)},
					{ID: "split", Class: dsl.NodeClassControl, Kind: string(dsl.ControlFanOut)},
				},
				Edges: []dsl.Edge{{From: "load", To: "split"}},
			},
		})
		fanOut := 7
		resolved.EffectiveConfig = snapshotEffectiveConfig()
		resolved.EffectiveConfig.FanOutWidth = fanOut
		effective, err := pinnedEffectiveConfig(resolved)
		if err != nil {
			t.Fatalf("pinnedEffectiveConfig() error = %v", err)
		}
		if got, want := coordinatorFanOutWidth(effective), fanOut; got != want {
			t.Fatalf("fan-out width = %d, want %d", got, want)
		}
	})
}

func TestCoordinatorActionExecutionInputShouldCarryPinnedPolicyAndSessionProvenance(t *testing.T) {
	t.Parallel()

	t.Run("Should copy the Run context nudge ratio into every Goal action input", func(t *testing.T) {
		t.Parallel()

		node := dsl.Node{ID: "goal", Class: dsl.NodeClassAction, Kind: string(dsl.ActionGoal)}
		resolved := &ResolvedDefinition{Definition: dsl.Definition{Graph: dsl.Graph{Nodes: []dsl.Node{node}}}}
		run := Run{
			ID:                    "looprun-goal-policy",
			WorkspaceID:           "ws-goal-policy",
			Inputs:                map[string]any{},
			GoalContextNudgeRatio: 0,
			Origin: &RunOrigin{
				Kind: RunOriginSession, SessionID: "session-origin",
				CreationProfileRef: "profile-origin", PolicySpecDigest: "policy-origin",
				CreationDigest: "creation-origin",
			},
		}
		actor, err := task.DeriveDaemonActorContext("loop-goal-policy-test", "")
		if err != nil {
			t.Fatalf("DeriveDaemonActorContext() error = %v", err)
		}
		input, err := actionExecutionInput(
			task.Run{ID: "taskrun-goal-policy"},
			actor,
			run,
			resolved,
			EffectiveConfig{},
			node,
			coordinatorActionRunMetadata{Generation: 1, GoalSegmentEpoch: 1},
			nil,
			GenerationHistory{},
			"session-provenance",
		)
		if err != nil {
			t.Fatalf("actionExecutionInput() error = %v", err)
		}
		if input.GoalContextNudgeRatio == nil || *input.GoalContextNudgeRatio != 0 {
			t.Fatalf("GoalContextNudgeRatio = %#v, want pinned explicit zero", input.GoalContextNudgeRatio)
		}
		if input.OriginSessionID != run.Origin.SessionID ||
			input.OriginCreationProfileRef != run.Origin.CreationProfileRef ||
			input.OriginPolicySpecDigest != run.Origin.PolicySpecDigest ||
			input.OriginCreationDigest != run.Origin.CreationDigest {
			t.Fatalf("action origin identity = %#v, want %#v", input, run.Origin)
		}
		if input.ProvenanceParentSessionID != "session-provenance" {
			t.Fatalf("ProvenanceParentSessionID = %q, want session-provenance", input.ProvenanceParentSessionID)
		}
	})

	t.Run("Should resolve the nearest session before executing a Goal action", func(t *testing.T) {
		t.Parallel()

		node := dsl.Node{ID: "goal", Class: dsl.NodeClassAction, Kind: string(dsl.ActionGoal)}
		origin := Run{
			ID: "looprun-origin", WorkspaceID: "ws-goal-provenance",
			StartedBy: task.ActorIdentity{Kind: task.ActorKindAgentSession, Ref: "session-origin"},
			Origin:    &RunOrigin{Kind: RunOriginCatalog},
		}
		loopRun := Run{
			ID: "looprun-goal-provenance", WorkspaceID: origin.WorkspaceID,
			ParentLoopRunID: origin.ID, Inputs: map[string]any{},
			StartedBy: task.ActorIdentity{Kind: task.ActorKindDaemon, Ref: "loop-runner"},
			Origin:    &RunOrigin{Kind: RunOriginCatalog},
		}
		coordinatorRun := controlCoordinatorRun(loopRun, 1)
		capture := &recordingActionExecutor{}
		actions, err := NewActionRegistry(
			&internalActionRegistryFake{},
			WithActionGoalExecutor(capture),
		)
		if err != nil {
			t.Fatalf("NewActionRegistry() error = %v", err)
		}
		runner := newCoordinatorRunnerForTestWithDefinition(
			t,
			loopRun,
			coordinatorRun,
			nil,
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {{
				Generation: 1, NodeID: string(node.ID), Status: generationOutputRunning, TaskRunID: "run-goal",
			}}}},
			dsl.Definition{Graph: dsl.Graph{Nodes: []dsl.Node{node}}},
			WithCoordinatorActionRegistry(actions),
		)
		setCoordinatorRunnerRunsForTest(t, runner, map[RunID]Run{
			loopRun.ID: loopRun,
			origin.ID:  origin,
		})
		metadata, err := json.Marshal(coordinatorActionRunMetadata{
			Generation: 1, NodeID: string(node.ID), Attempt: 1, GoalSegmentEpoch: 1,
		})
		if err != nil {
			t.Fatalf("json.Marshal(action metadata) error = %v", err)
		}
		workerRun := task.Run{
			ID: "run-goal", RunKind: task.RunKindWorker, LoopRunID: string(loopRun.ID),
			Status: task.TaskRunStatusClaimed, Metadata: metadata,
		}

		if _, err := runner.ExecuteActionRun(t.Context(), workerRun, task.ActorContext{}); err != nil {
			t.Fatalf("ExecuteActionRun() error = %v", err)
		}
		if got := capture.input.ProvenanceParentSessionID; got != "session-origin" {
			t.Fatalf("executed provenance parent = %q, want session-origin", got)
		}
	})

	t.Run("Should resolve the nearest session before executing a run-agent action", func(t *testing.T) {
		t.Parallel()

		node := dsl.Node{ID: "agent", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent)}
		origin := Run{
			ID: "looprun-agent-origin", WorkspaceID: "ws-agent-provenance",
			StartedBy: task.ActorIdentity{Kind: task.ActorKindAgentSession, Ref: "session-agent-origin"},
			Origin:    &RunOrigin{Kind: RunOriginCatalog},
		}
		loopRun := Run{
			ID: "looprun-agent-provenance", WorkspaceID: origin.WorkspaceID,
			ParentLoopRunID: origin.ID, Inputs: map[string]any{},
			StartedBy: task.ActorIdentity{Kind: task.ActorKindDaemon, Ref: "loop-runner"},
			Origin:    &RunOrigin{Kind: RunOriginCatalog},
		}
		coordinatorRun := controlCoordinatorRun(loopRun, 1)
		capture := &recordingActionExecutor{}
		actions, err := NewActionRegistry(
			&internalActionRegistryFake{},
			WithActionRunAgentExecutor(capture),
		)
		if err != nil {
			t.Fatalf("NewActionRegistry() error = %v", err)
		}
		runner := newCoordinatorRunnerForTestWithDefinition(
			t,
			loopRun,
			coordinatorRun,
			nil,
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {{
				Generation: 1, NodeID: string(node.ID), Status: generationOutputRunning, TaskRunID: "run-agent",
			}}}},
			dsl.Definition{Graph: dsl.Graph{Nodes: []dsl.Node{node}}},
			WithCoordinatorActionRegistry(actions),
		)
		setCoordinatorRunnerRunsForTest(t, runner, map[RunID]Run{
			loopRun.ID: loopRun,
			origin.ID:  origin,
		})
		metadata, err := json.Marshal(coordinatorActionRunMetadata{
			Generation: 1, NodeID: string(node.ID), Attempt: 1,
		})
		if err != nil {
			t.Fatalf("json.Marshal(action metadata) error = %v", err)
		}
		workerRun := task.Run{
			ID: "run-agent", RunKind: task.RunKindWorker, LoopRunID: string(loopRun.ID),
			Status: task.TaskRunStatusClaimed, Metadata: metadata,
		}

		if _, err := runner.ExecuteActionRun(t.Context(), workerRun, task.ActorContext{}); err != nil {
			t.Fatalf("ExecuteActionRun() error = %v", err)
		}
		if got := capture.input.ProvenanceParentSessionID; got != "session-agent-origin" {
			t.Fatalf("executed provenance parent = %q, want session-agent-origin", got)
		}
	})

	t.Run("Should stop before Goal execution when provenance lookup fails", func(t *testing.T) {
		t.Parallel()

		node := dsl.Node{ID: "goal", Class: dsl.NodeClassAction, Kind: string(dsl.ActionGoal)}
		loopRun := Run{
			ID: "looprun-goal-provenance-error", WorkspaceID: "ws-goal-provenance-error",
			ParentLoopRunID: "looprun-missing-origin", Inputs: map[string]any{},
			StartedBy: task.ActorIdentity{Kind: task.ActorKindDaemon, Ref: "loop-runner"},
			Origin:    &RunOrigin{Kind: RunOriginCatalog},
		}
		coordinatorRun := controlCoordinatorRun(loopRun, 1)
		capture := &recordingActionExecutor{}
		actions, err := NewActionRegistry(
			&internalActionRegistryFake{},
			WithActionGoalExecutor(capture),
		)
		if err != nil {
			t.Fatalf("NewActionRegistry() error = %v", err)
		}
		runner := newCoordinatorRunnerForTestWithDefinition(
			t,
			loopRun,
			coordinatorRun,
			nil,
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {{
				Generation: 1, NodeID: string(node.ID), Status: generationOutputRunning, TaskRunID: "run-goal-error",
			}}}},
			dsl.Definition{Graph: dsl.Graph{Nodes: []dsl.Node{node}}},
			WithCoordinatorActionRegistry(actions),
		)
		store := runner.store.(*coordinatorRunnerLoopStore)
		current := store.run
		storeErr := errors.New("provenance store unavailable")
		store.getRun = func(runID RunID) (Run, error) {
			if runID == current.ID {
				return current, nil
			}
			return Run{}, storeErr
		}
		metadata, err := json.Marshal(coordinatorActionRunMetadata{
			Generation: 1, NodeID: string(node.ID), Attempt: 1, GoalSegmentEpoch: 1,
		})
		if err != nil {
			t.Fatalf("json.Marshal(action metadata) error = %v", err)
		}
		workerRun := task.Run{
			ID: "run-goal-error", RunKind: task.RunKindWorker, LoopRunID: string(loopRun.ID),
			Status: task.TaskRunStatusClaimed, Metadata: metadata,
		}

		_, err = runner.ExecuteActionRun(t.Context(), workerRun, task.ActorContext{})
		if !errors.Is(err, storeErr) {
			t.Fatalf("ExecuteActionRun() error = %v, want %v", err, storeErr)
		}
		if capture.input.LoopRunID != "" {
			t.Fatalf("action executor input = %#v, want no execution", capture.input)
		}
	})

	t.Run("Should skip provenance ancestry for a non-Goal action", func(t *testing.T) {
		t.Parallel()

		node := dsl.Node{ID: "transform", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform)}
		loopRun := Run{
			ID: "looprun-transform-provenance", WorkspaceID: "ws-transform-provenance",
			ParentLoopRunID: "looprun-unavailable-origin", Inputs: map[string]any{},
			StartedBy: task.ActorIdentity{Kind: task.ActorKindDaemon, Ref: "loop-runner"},
			Origin:    &RunOrigin{Kind: RunOriginCatalog},
		}
		coordinatorRun := controlCoordinatorRun(loopRun, 1)
		capture := &recordingActionExecutor{}
		actions, err := NewActionRegistry(
			&internalActionRegistryFake{},
			WithActionTransformExecutor(capture),
		)
		if err != nil {
			t.Fatalf("NewActionRegistry() error = %v", err)
		}
		runner := newCoordinatorRunnerForTestWithDefinition(
			t,
			loopRun,
			coordinatorRun,
			nil,
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {{
				Generation: 1, NodeID: string(node.ID), Status: generationOutputRunning, TaskRunID: "run-transform",
			}}}},
			dsl.Definition{Graph: dsl.Graph{Nodes: []dsl.Node{node}}},
			WithCoordinatorActionRegistry(actions),
		)
		store := runner.store.(*coordinatorRunnerLoopStore)
		current := store.run
		ancestorLookups := 0
		store.getRun = func(runID RunID) (Run, error) {
			if runID == current.ID {
				return current, nil
			}
			ancestorLookups++
			return Run{}, errors.New("unexpected ancestry lookup")
		}
		metadata, err := json.Marshal(coordinatorActionRunMetadata{
			Generation: 1, NodeID: string(node.ID), Attempt: 1,
		})
		if err != nil {
			t.Fatalf("json.Marshal(action metadata) error = %v", err)
		}
		workerRun := task.Run{
			ID: "run-transform", RunKind: task.RunKindWorker, LoopRunID: string(loopRun.ID),
			Status: task.TaskRunStatusClaimed, Metadata: metadata,
		}

		if _, err := runner.ExecuteActionRun(t.Context(), workerRun, task.ActorContext{}); err != nil {
			t.Fatalf("ExecuteActionRun() error = %v", err)
		}
		if ancestorLookups != 0 {
			t.Fatalf("provenance ancestor lookups = %d, want 0", ancestorLookups)
		}
	})

	t.Run("Should expose a downstream template failure as a routable authoring failure", func(t *testing.T) {
		t.Parallel()

		loopRun := Run{
			ID: "looprun-template-failure", WorkspaceID: "ws-template-failure",
			Status: StatusRunning, Generation: 1, Inputs: map[string]any{},
			Origin: &RunOrigin{Kind: RunOriginSession, SessionID: "session-template-failure"},
		}
		node := dsl.Node{
			ID: "consumer", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent),
			Params: dsl.NodeParams{
				"agent": "codex", "prompt": "Use {{ .nodes.producer.output.summary }}",
			},
		}
		definition := dsl.Definition{Graph: dsl.Graph{
			Nodes: []dsl.Node{
				{
					ID: "producer", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent),
					Params: dsl.NodeParams{"output_schema": map[string]any{
						"type": "object", "properties": map[string]any{
							"summary": map[string]any{"type": "string"},
						},
					}},
				},
				node,
			},
			Edges: []dsl.Edge{{From: "producer", To: "consumer"}},
		}}
		capture := &recordingActionExecutor{execute: func(node dsl.Node, input ActionExecutionInput) error {
			_, err := renderNodeParams(node, input.Namespace)
			return err
		}}
		actions, err := NewActionRegistry(
			&internalActionRegistryFake{},
			WithActionRunAgentExecutor(capture),
		)
		if err != nil {
			t.Fatalf("NewActionRegistry() error = %v", err)
		}
		runner := newCoordinatorRunnerForTestWithDefinition(
			t,
			loopRun,
			controlCoordinatorRun(loopRun, 1),
			nil,
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
				{Generation: 1, NodeID: "producer", Status: generationOutputSucceeded, OutputRef: `{"tipo":"backend"}`},
				{Generation: 1, NodeID: "consumer", Status: generationOutputRunning, TaskRunID: "run-consumer"},
			}}},
			definition,
			WithCoordinatorActionRegistry(actions),
		)
		metadata, err := json.Marshal(coordinatorActionRunMetadata{
			Generation: 1, NodeID: "consumer", Attempt: 1,
		})
		if err != nil {
			t.Fatalf("json.Marshal(action metadata) error = %v", err)
		}
		_, err = runner.ExecuteActionRun(t.Context(), task.Run{
			ID: "run-consumer", RunKind: task.RunKindWorker, LoopRunID: string(loopRun.ID),
			Status: task.TaskRunStatusClaimed, Metadata: metadata,
		}, task.ActorContext{})
		if !errors.Is(err, ErrActionMaterialization) {
			t.Fatalf("ExecuteActionRun() error = %v, want ErrActionMaterialization", err)
		}
		provider, ok := errors.AsType[SafeActionFailureProvider](err)
		if !ok || provider.SafeActionFailure().Code != string(ReasonCodeActionMaterializationFailed) {
			t.Fatalf("ExecuteActionRun() failure = %#v, want routable authoring code", provider)
		}
	})

	t.Run("Should expose externalized outputs to action templates without replacing their refs", func(t *testing.T) {
		t.Parallel()

		largeSummary := strings.Repeat("summary-", LoopOutputInlineLimitBytes/8+1)
		payload, err := json.Marshal(map[string]any{
			"status": "done", "tipo": "backend", "resumo": largeSummary,
		})
		if err != nil {
			t.Fatalf("json.Marshal(output payload) error = %v", err)
		}
		outputRef := OutputRefForPayload(payload)
		loopRun := Run{
			ID: "looprun-action-runtime-payload", WorkspaceID: "ws-action-runtime-payload",
			LoopName: "delivery", Status: StatusRunning, Generation: 1, Inputs: map[string]any{},
			Origin: &RunOrigin{Kind: RunOriginSession, SessionID: "session-action-runtime-payload"},
		}
		coordinatorRun := controlCoordinatorRun(loopRun, 1)
		definition := dsl.Definition{Graph: dsl.Graph{
			Nodes: []dsl.Node{
				{ID: "load", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent),
					Params: dsl.NodeParams{"output_schema": map[string]any{
						"type": "object", "properties": map[string]any{
							"status": map[string]any{"type": "string"},
							"tipo":   map[string]any{"type": "string"},
							"resumo": map[string]any{"type": "string"},
						}, "required": []string{"status", "tipo", "resumo"},
					}},
				},
				{ID: "agent", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent),
					Params: dsl.NodeParams{"agent": "codex", "prompt": "Use {{ .nodes.load.output.resumo }}"}},
			},
			Edges: []dsl.Edge{{From: "load", To: "agent"}},
		}}
		capture := &recordingActionExecutor{}
		actions, err := NewActionRegistry(
			&internalActionRegistryFake{},
			WithActionRunAgentExecutor(capture),
		)
		if err != nil {
			t.Fatalf("NewActionRegistry() error = %v", err)
		}
		payloadKey := GenerationOutputPayloadKey{
			WorkspaceID: loopRun.WorkspaceID,
			RunID:       loopRun.ID,
			Generation:  1,
			NodeID:      "load",
			OutputRef:   outputRef,
		}
		outputStore := coordinatorRunnerOutputs{
			outputs: map[int][]GenerationOutput{1: {
				{Generation: 1, NodeID: "load", Status: generationOutputSucceeded, OutputRef: outputRef},
				{Generation: 1, NodeID: "agent", Status: generationOutputRunning, TaskRunID: "run-agent"},
			}},
			payloads: map[GenerationOutputPayloadKey]json.RawMessage{payloadKey: payload},
		}
		runner := newCoordinatorRunnerForTestWithDefinition(
			t,
			loopRun,
			coordinatorRun,
			nil,
			outputStore,
			definition,
			WithCoordinatorActionRegistry(actions),
		)
		metadata, err := json.Marshal(coordinatorActionRunMetadata{
			Generation: 1,
			NodeID:     "agent",
			Attempt:    1,
		})
		if err != nil {
			t.Fatalf("json.Marshal(action metadata) error = %v", err)
		}
		workerRun := task.Run{
			ID: "run-agent", RunKind: task.RunKindWorker, LoopRunID: string(loopRun.ID),
			Status: task.TaskRunStatusClaimed, Metadata: metadata,
		}

		if _, err := runner.ExecuteActionRun(context.Background(), workerRun, task.ActorContext{}); err != nil {
			t.Fatalf("ExecuteActionRun() error = %v", err)
		}
		nodes, ok := capture.input.Namespace[namespaceNodesKey].(map[string]any)
		if !ok {
			t.Fatalf("namespace nodes = %#v, want map", capture.input.Namespace[namespaceNodesKey])
		}
		load, ok := nodes["load"].(map[string]any)
		if !ok {
			t.Fatalf("namespace load = %#v, want map", nodes["load"])
		}
		loaded, ok := load[namespaceOutputKey].(map[string]any)
		if !ok || loaded["status"] != "done" || loaded["tipo"] != "backend" ||
			loaded["resumo"] != largeSummary {
			t.Fatalf("namespace load output = %#v, want every externalized required field", load[namespaceOutputKey])
		}
		rendered, err := renderNodeParams(definition.Graph.Nodes[1], capture.input.Namespace)
		if err != nil {
			t.Fatalf("renderNodeParams(externalized output) error = %v", err)
		}
		if got := rendered["prompt"]; got != "Use "+largeSummary {
			t.Fatalf("rendered prompt length = %d, want exact externalized summary", len(got.(string)))
		}
		conditionCompiler, err := refs.NewConditionCompiler(refs.Namespace{Nodes: map[string]refs.NodeSchema{
			"load": {
				HasOutput: true,
				Output: refs.Schema{"type": "object", "properties": map[string]any{
					"status": map[string]any{"type": "string"},
					"tipo":   map[string]any{"type": "string"},
					"resumo": map[string]any{"type": "string"},
				}},
			},
		}})
		if err != nil {
			t.Fatalf("NewConditionCompiler() error = %v", err)
		}
		condition, err := conditionCompiler.Compile(
			`nodes.load.output.status == "done" && nodes.load.output.tipo == "backend" && nodes.load.output.resumo.size() > 16000`,
		)
		if err != nil {
			t.Fatalf("Compile(externalized output CEL) error = %v", err)
		}
		evaluation, err := condition.Evaluate(capture.input.Namespace)
		if err != nil || !evaluation.Value {
			t.Fatalf("Evaluate(externalized output CEL) = %#v, error = %v, want true", evaluation, err)
		}
		if got := outputStore.outputs[1][0].OutputRef; got != outputRef {
			t.Fatalf("stored output_ref = %q, want %q", got, outputRef)
		}
	})
}

func TestCoordinatorActionMetadataShouldDistinguishContinuationFailures(t *testing.T) {
	t.Parallel()

	base := `{"generation":1,"node_id":"work","item_index":0,"attempt":1,"epoch":1,`
	_, unknownErr := parseCoordinatorActionRunMetadata(json.RawMessage(
		base + `"continuation_kind":"unexpected","resume_from_task_run_id":"run-1",` +
			`"resume_from_session_id":"session-1"}`,
	))
	if !errors.Is(unknownErr, ErrValidation) || !strings.Contains(unknownErr.Error(), `"unexpected"`) {
		t.Fatalf("unknown continuation error = %v, want distinct kind validation", unknownErr)
	}
	_, incompleteErr := parseCoordinatorActionRunMetadata(json.RawMessage(
		base + `"continuation_kind":"death_resume"}`,
	))
	if !errors.Is(incompleteErr, ErrValidation) ||
		!strings.Contains(incompleteErr.Error(), "provenance is incomplete") {
		t.Fatalf("incomplete continuation error = %v, want provenance validation", incompleteErr)
	}
}

func TestCoordinatorRunnerShouldResolveNoProgressWindowFromWorkspaceDefaults(t *testing.T) {
	t.Run("Should use workspace default no-progress window for stall detection", func(t *testing.T) {
		t.Parallel()

		loopRun := Run{
			ID:          "looprun-default-stall-window",
			WorkspaceID: "ws-defaults",
			LoopName:    "delivery",
			Status:      StatusRunning,
			Generation:  2,
		}
		coordinatorRun := task.Run{
			ID:        "run-coordinator-default-stall-window",
			TaskID:    "task-coordinator-default-stall-window",
			RunKind:   task.RunKindCoordinator,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusClaimed,
		}
		blockerRef := `{"blocking_issues":[{"id":"missing-reviewer"}]}`
		runner := newCoordinatorRunnerForTestWithDefinition(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{coordinatorRun.ID: coordinatorRun},
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{
				1: {{
					Generation: 1,
					NodeID:     "load",
					Status:     generationOutputFailed,
					OutputRef:  blockerRef,
				}},
				2: {{
					Generation: 2,
					NodeID:     "load",
					Status:     generationOutputFailed,
					OutputRef:  blockerRef,
				}},
			}},
			dsl.Definition{
				Graph:    coordinatorTestGraph(),
				Contract: dsl.Contract{NoProgress: dsl.NoProgress{Window: 2}},
			},
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.Terminal == nil {
			t.Fatal("Terminal = nil, want stalled")
		}
		if got, want := plan.Terminal.Status, string(StatusStalled); got != want {
			t.Fatalf("terminal status = %q, want %q", got, want)
		}
		if got, want := plan.Terminal.ReasonCode, blockingIssuesRepeatedCode; got != want {
			t.Fatalf("reason_code = %q, want %q", got, want)
		}
	})
}

func TestCoordinatorRunnerShouldApplyGateRevisionsFromWorkspaceDefaults(t *testing.T) {
	t.Run("Should rewrite gate node max revisions with the Run-pinned value", func(t *testing.T) {
		t.Parallel()

		resolved := resolvedCoordinatorDefinitionForTest(t, dsl.Definition{
			Graph: dsl.Graph{
				Nodes: []dsl.Node{
					{ID: "load", Class: dsl.NodeClassSource, Kind: string(dsl.SourceInput)},
					{
						ID:           "review_gate",
						Class:        dsl.NodeClassControl,
						Kind:         string(dsl.ControlGate),
						MaxRevisions: 1,
					},
				},
				Edges: []dsl.Edge{{From: "load", To: "review_gate"}},
			},
		})
		gateRevisions := 6
		resolved.EffectiveConfig = snapshotEffectiveConfig()
		resolved.EffectiveConfig.GateMaxRevisions = gateRevisions
		effective, err := pinnedEffectiveConfig(resolved)
		if err != nil {
			t.Fatalf("pinnedEffectiveConfig() error = %v", err)
		}
		rewritten := coordinatorResolvedWithEffectiveConfig(resolved, effective)
		node := rewritten.Definition.Graph.Nodes[1]
		if got, want := node.MaxRevisions, gateRevisions; got != want {
			t.Fatalf("gate MaxRevisions = %d, want %d", got, want)
		}
		if got, want := resolved.Definition.Graph.Nodes[1].MaxRevisions, 1; got != want {
			t.Fatalf("original gate MaxRevisions = %d, want unchanged %d", got, want)
		}
	})
}

func TestBranchPredicateEvaluationShouldHonorFailurePolicy(t *testing.T) {
	t.Parallel()

	definition := dsl.Definition{
		Inputs: map[string]dsl.Input{
			"denominator": {Type: dsl.InputTypeNumber},
			"seed":        {Type: dsl.InputTypeString},
		},
		Graph: dsl.Graph{
			Nodes: []dsl.Node{
				{
					ID: "load", Class: dsl.NodeClassSource, Kind: string(dsl.SourceInput),
					InputRef: "seed",
				},
				{
					ID: "route", Class: dsl.NodeClassControl, Kind: string(dsl.ControlBranch),
					Condition: `inputs.denominator / 0 > 1`,
				},
				{
					ID: "fallback", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform),
					Params: dsl.NodeParams{"map": map[string]any{"routed": map[string]any{"value": true}}},
				},
				{
					ID: "success", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform),
					Params: dsl.NodeParams{"map": map[string]any{"matched": map[string]any{"value": true}}},
				},
			},
			Edges: []dsl.Edge{
				{From: "load", To: "route"},
				{From: "route", To: "fallback"},
				{From: "route", To: "success"},
			},
		}}
	resolved := compileCoordinatorControlDefinition(t, definition)
	topology := newControlTopology(resolved.Definition.Graph)
	baseOutputs := []GenerationOutput{
		{Generation: 1, NodeID: "load", Status: generationOutputSucceeded, OutputRef: `{"items":[]}`},
		{Generation: 1, NodeID: "route", Status: generationOutputPending},
		{Generation: 1, NodeID: "fallback", Status: generationOutputPending},
		{Generation: 1, NodeID: "success", Status: generationOutputPending},
	}

	t.Run("Should fail a broken routing predicate closed", func(t *testing.T) {
		t.Parallel()

		outputs := cloneGenerationOutputs(baseOutputs)
		result, terminal, err := evaluateBranchNode(
			Run{ID: "run-branch-fail", WorkspaceID: "ws-1", Inputs: map[string]any{"denominator": 1}},
			1,
			resolved,
			topology,
			GenerationHistory{},
			outputs[1],
			resolved.Definition.Graph.Nodes[1],
			&outputs,
			&gateEvaluationCollector{},
		)
		if err != nil {
			t.Fatalf("evaluateBranchNode() error = %v", err)
		}
		if terminal != nil || result.Status != generationOutputFailed ||
			!strings.Contains(result.OutputRef, "predicate_evaluation_failed") {
			t.Fatalf("branch result = %#v terminal=%#v, want routable authoring failure", result, terminal)
		}
	})

	t.Run("Should route a broken predicate through on-error", func(t *testing.T) {
		t.Parallel()

		outputs := cloneGenerationOutputs(baseOutputs)
		node := resolved.Definition.Graph.Nodes[1]
		node.NodeLifecycleState = &dsl.NodeLifecycleState{
			OnError: &dsl.ErrorPolicy{Route: "fallback"},
		}
		result, terminal, err := evaluateBranchNode(
			Run{ID: "run-branch-route", WorkspaceID: "ws-1", Inputs: map[string]any{"denominator": 1}},
			1,
			resolved,
			topology,
			GenerationHistory{},
			outputs[1],
			node,
			&outputs,
			&gateEvaluationCollector{},
		)
		if err != nil {
			t.Fatalf("evaluateBranchNode() error = %v", err)
		}
		if terminal != nil || result.Status != generationOutputSucceeded ||
			result.OutputRef != errorRoutedOutputRefPrefix+"fallback" ||
			outputs[2].Status != generationOutputPending ||
			outputs[3].OutputRef != branchSkippedOutputRef {
			t.Fatalf("routed branch result = %#v outputs=%#v terminal=%#v", result, outputs, terminal)
		}
	})

	t.Run("Should isolate a broken predicate to its item lane", func(t *testing.T) {
		t.Parallel()

		outputs := make([]GenerationOutput, 0, len(baseOutputs)*2)
		for itemIndex := range 2 {
			for _, output := range baseOutputs {
				output.ItemIndex = itemIndex
				outputs = append(outputs, output)
			}
		}
		outputByKey := generationOutputMap(outputs)
		failedLane := outputByKey[generationOutputKey{nodeID: "route", itemIndex: 1}]
		result, terminal, err := evaluateBranchNode(
			Run{ID: "run-branch-lane", WorkspaceID: "ws-1", Inputs: map[string]any{"denominator": 1}},
			1,
			resolved,
			topology,
			GenerationHistory{},
			failedLane,
			resolved.Definition.Graph.Nodes[1],
			&outputs,
			&gateEvaluationCollector{},
		)
		if err != nil {
			t.Fatalf("evaluateBranchNode() error = %v", err)
		}
		outputByKey = generationOutputMap(outputs)
		if terminal != nil || result.Status != generationOutputFailed ||
			outputByKey[generationOutputKey{nodeID: "route", itemIndex: 0}].Status != generationOutputPending ||
			outputByKey[generationOutputKey{nodeID: "route", itemIndex: 1}].Status != generationOutputFailed {
			t.Fatalf("lane outputs = %#v terminal=%#v, want only item 1 failed", outputs, terminal)
		}
	})

	t.Run("Should exit when the routing override requests it", func(t *testing.T) {
		t.Parallel()

		outputs := cloneGenerationOutputs(baseOutputs)
		node := resolved.Definition.Graph.Nodes[1]
		node.OnEvalError = dsl.EvalErrorExit
		result, terminal, err := evaluateBranchNode(
			Run{ID: "run-branch-exit", WorkspaceID: "ws-1", Inputs: map[string]any{"denominator": 1}},
			1,
			resolved,
			topology,
			GenerationHistory{},
			outputs[1],
			node,
			&outputs,
			&gateEvaluationCollector{},
		)
		if err != nil {
			t.Fatalf("evaluateBranchNode() error = %v", err)
		}
		if terminal == nil || terminal.Status != string(StatusDone) || result.Status != generationOutputSucceeded {
			t.Fatalf("branch exit result = %#v terminal=%#v, want successful policy exit", result, terminal)
		}
	})
}

func TestRoutePlannerShouldSelectExactlyOnePath(t *testing.T) {
	t.Parallel()

	definition := routePlannerDefinitionForTest()
	resolved := compileCoordinatorControlDefinition(t, definition)
	topology := newControlTopology(resolved.Definition.Graph)
	baseOutputs := []GenerationOutput{
		{Generation: 1, NodeID: "load", Status: generationOutputSucceeded, OutputRef: `{"risk":"high"}`},
		{Generation: 1, NodeID: "router", Status: generationOutputPending},
		{Generation: 1, NodeID: "quick", Status: generationOutputPending},
		{Generation: 1, NodeID: "review", Status: generationOutputPending},
		{Generation: 1, NodeID: "fallback", Status: generationOutputPending},
		{Generation: 1, NodeID: "shared", Status: generationOutputPending},
	}

	t.Run("Should select the first matching route and preserve a live join", func(t *testing.T) {
		t.Parallel()

		outputs := cloneGenerationOutputs(baseOutputs)
		collector := &gateEvaluationCollector{}
		result, terminal, err := evaluateRouteNode(
			Run{ID: "run-route", WorkspaceID: "ws-1"},
			1,
			resolved,
			topology,
			GenerationHistory{},
			outputs[1],
			resolved.Definition.Graph.Nodes[1],
			&outputs,
			collector,
		)
		if err != nil {
			t.Fatalf("evaluateRouteNode() error = %v", err)
		}
		mapped := generationOutputMap(outputs)
		if terminal != nil || result.Status != generationOutputSucceeded || result.OutputRef != "route:review" ||
			!isRouteNotTakenOutputRef(mapped[generationOutputKey{nodeID: "quick"}].OutputRef) ||
			!isRouteNotTakenOutputRef(mapped[generationOutputKey{nodeID: "fallback"}].OutputRef) ||
			mapped[generationOutputKey{nodeID: "review"}].Status != generationOutputPending ||
			mapped[generationOutputKey{nodeID: "shared"}].Status != generationOutputPending {
			t.Fatalf("route result = %#v outputs=%#v terminal=%#v", result, outputs, terminal)
		}
		if len(collector.routeDecisions) != 1 || collector.routeDecisions[0].Target != "review" ||
			collector.routeDecisions[0].MatchedWhen != `nodes.load.output.risk == "high"` {
			t.Fatalf("route decisions = %#v, want matched review condition", collector.routeDecisions)
		}
	})

	t.Run("Should use the default only after every condition is false", func(t *testing.T) {
		t.Parallel()

		definition := routePlannerDefinitionForTest()
		definition.Graph.Nodes[1].Routes[1].When = `nodes.load.output.risk == "critical"`
		defaultResolved := compileCoordinatorControlDefinition(t, definition)
		outputs := cloneGenerationOutputs(baseOutputs)
		collector := &gateEvaluationCollector{}
		result, terminal, err := evaluateRouteNode(
			Run{ID: "run-default", WorkspaceID: "ws-1"},
			1,
			defaultResolved,
			topology,
			GenerationHistory{},
			outputs[1],
			defaultResolved.Definition.Graph.Nodes[1],
			&outputs,
			collector,
		)
		if err != nil {
			t.Fatalf("evaluateRouteNode() error = %v", err)
		}
		if terminal != nil || result.OutputRef != "route:fallback" || len(collector.routeDecisions) != 1 ||
			!collector.routeDecisions[0].Default || collector.routeDecisions[0].Cause != "default" {
			t.Fatalf("default result = %#v decisions=%#v terminal=%#v", result, collector.routeDecisions, terminal)
		}
	})

	t.Run("Should prefer declaration order when two routes match", func(t *testing.T) {
		t.Parallel()

		definition := routePlannerDefinitionForTest()
		definition.Graph.Nodes[1].Routes[0].When = "true"
		definition.Graph.Nodes[1].Routes[1].When = "true"
		orderedResolved := compileCoordinatorControlDefinition(t, definition)
		outputs := cloneGenerationOutputs(baseOutputs)
		collector := &gateEvaluationCollector{}
		result, _, err := evaluateRouteNode(
			Run{ID: "run-ordered", WorkspaceID: "ws-1"},
			1,
			orderedResolved,
			topology,
			GenerationHistory{},
			outputs[1],
			orderedResolved.Definition.Graph.Nodes[1],
			&outputs,
			collector,
		)
		if err != nil {
			t.Fatalf("evaluateRouteNode() error = %v", err)
		}
		if result.OutputRef != "route:quick" || collector.routeDecisions[0].Target != "quick" {
			t.Fatalf("ordered result = %#v decisions=%#v, want quick", result, collector.routeDecisions)
		}
	})

	t.Run("Should fail a broken condition without taking the default", func(t *testing.T) {
		t.Parallel()

		definition := routePlannerDefinitionForTest()
		definition.Inputs = map[string]dsl.Input{"denominator": {Type: dsl.InputTypeNumber}}
		definition.Graph.Nodes[1].Routes[0].When = "inputs.denominator / 0 > 1"
		brokenResolved := compileCoordinatorControlDefinition(t, definition)
		outputs := cloneGenerationOutputs(baseOutputs)
		collector := &gateEvaluationCollector{}
		result, terminal, err := evaluateRouteNode(
			Run{ID: "run-broken-route", WorkspaceID: "ws-1", Inputs: map[string]any{"denominator": 1}},
			1,
			brokenResolved,
			topology,
			GenerationHistory{},
			outputs[1],
			brokenResolved.Definition.Graph.Nodes[1],
			&outputs,
			collector,
		)
		if err != nil {
			t.Fatalf("evaluateRouteNode() error = %v", err)
		}
		if terminal != nil || result.Status != generationOutputFailed || len(collector.routeDecisions) != 0 ||
			outputs[4].Status != generationOutputPending ||
			!strings.Contains(result.OutputRef, "predicate_evaluation_failed") {
			t.Fatalf("broken result = %#v outputs=%#v decisions=%#v", result, outputs, collector.routeDecisions)
		}
	})
}

func TestGateRoutePlannerShouldSelectAndRecordTarget(t *testing.T) {
	t.Parallel()

	definition := dsl.Definition{Graph: dsl.Graph{
		Nodes: []dsl.Node{
			{
				ID: "load", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform),
				Params: dsl.NodeParams{"map": map[string]any{"ready": map[string]any{"value": true}}},
			},
			{
				ID: "quality", Class: dsl.NodeClassControl, Kind: string(dsl.ControlGate),
				Criteria: []dsl.GateCriterion{{ID: "check", Type: dsl.CriterionCommand, Check: "true"}},
				OnResult: map[string]any{"fail": map[string]any{"route": "repair"}},
			},
			{
				ID: "publish", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform),
				Params: dsl.NodeParams{"map": map[string]any{"published": map[string]any{"value": true}}},
			},
			{
				ID: "repair", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform),
				Params: dsl.NodeParams{"map": map[string]any{"repaired": map[string]any{"value": true}}},
			},
		},
		Edges: []dsl.Edge{
			{From: "load", To: "quality"}, {From: "quality", To: "publish"}, {From: "quality", To: "repair"},
		},
	}}
	resolved := compileCoordinatorControlDefinition(t, definition)
	topology := newControlTopology(resolved.Definition.Graph)
	outputs := []GenerationOutput{
		{Generation: 1, NodeID: "load", Status: generationOutputSucceeded, OutputRef: `{}`},
		{Generation: 1, NodeID: "quality", Status: generationOutputPending},
		{Generation: 1, NodeID: "publish", Status: generationOutputPending},
		{Generation: 1, NodeID: "repair", Status: generationOutputPending},
	}
	collector := &gateEvaluationCollector{}
	evaluator := gateEvaluatorFunc(func(context.Context, gate.Gate, gate.GateInput) (gate.Verdict, error) {
		verdict := testRouteVerdict(gate.RouteContinue)
		verdict.Route.Target = "repair"
		return verdict, nil
	})
	result, terminal, err := evaluateGateNode(
		context.Background(),
		Run{ID: "run-gate-route", WorkspaceID: "ws-1"},
		1,
		resolved,
		topology,
		EffectiveConfig{},
		evaluator,
		nil,
		nil,
		nil,
		GenerationHistory{},
		outputs[1],
		resolved.Definition.Graph.Nodes[1],
		&outputs,
		collector,
		time.Now(),
	)
	if err != nil {
		t.Fatalf("evaluateGateNode() error = %v", err)
	}
	if terminal != nil || result.Status != generationOutputSucceeded ||
		!isRouteNotTakenOutputRef(outputs[2].OutputRef) || outputs[3].Status != generationOutputPending ||
		len(collector.routeDecisions) != 1 || collector.routeDecisions[0].Target != "repair" {
		t.Fatalf(
			"gate result = %#v outputs=%#v decisions=%#v terminal=%#v",
			result,
			outputs,
			collector.routeDecisions,
			terminal,
		)
	}

	t.Run("Should scope a gate route to one fan-out lane", func(t *testing.T) {
		t.Parallel()

		graph := dsl.Graph{
			Nodes: []dsl.Node{
				{ID: "fan", Class: dsl.NodeClassControl, Kind: string(dsl.ControlFanOut)},
				{ID: "quality", Class: dsl.NodeClassControl, Kind: string(dsl.ControlGate)},
				{ID: "publish", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform)},
				{ID: "repair", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform)},
				{ID: "collect", Class: dsl.NodeClassControl, Kind: string(dsl.ControlCollect)},
			},
			Edges: []dsl.Edge{
				{From: "fan", To: "quality"}, {From: "quality", To: "publish"},
				{From: "quality", To: "repair"}, {From: "publish", To: "collect"},
				{From: "repair", To: "collect"},
			},
		}
		laneTopology := newControlTopology(graph)
		laneOutputs := []GenerationOutput{
			{NodeID: "quality", ItemIndex: 0, Status: generationOutputPending},
			{NodeID: "publish", ItemIndex: 0, Status: generationOutputPending},
			{NodeID: "repair", ItemIndex: 0, Status: generationOutputPending},
			{NodeID: "quality", ItemIndex: 1, Status: generationOutputSucceeded},
			{NodeID: "publish", ItemIndex: 1, Status: generationOutputPending},
			{NodeID: "repair", ItemIndex: 1, Status: generationOutputPending},
		}
		skipUnselectedRoutePaths(
			graph,
			laneTopology,
			"quality",
			"repair",
			laneOutputs[3],
			&laneOutputs,
		)
		mapped := generationOutputMap(laneOutputs)
		if mapped[generationOutputKey{nodeID: "publish", itemIndex: 0}].Status != generationOutputPending ||
			!isRouteNotTakenOutputRef(mapped[generationOutputKey{nodeID: "publish", itemIndex: 1}].OutputRef) ||
			mapped[generationOutputKey{nodeID: "repair", itemIndex: 1}].Status != generationOutputPending {
			t.Fatalf("lane route outputs = %#v", laneOutputs)
		}
	})

	t.Run("Should keep route events independent per generation", func(t *testing.T) {
		t.Parallel()

		for generation, target := range map[int]dsl.NodeID{1: "repair", 2: "publish"} {
			evaluations := &gateEvaluationCollector{}
			evaluations.recordRoute(routeDecision{
				NodeID: "quality", ItemIndex: 0, Target: target, Cause: "gate_verdict:rejected",
			})
			plan := task.CoordinatorCompletionPlan{Snapshot: task.GenerationSnapshot{
				Generation: generation, Payload: GenerationSnapshotPayload{},
			}}
			if err := applyGateEvaluationIntents(&plan, Run{}, generation, evaluations); err != nil {
				t.Fatalf("applyGateEvaluationIntents(generation %d) error = %v", generation, err)
			}
			payload, err := GenerationSnapshotPayloadFrom(plan.Snapshot.Payload)
			if err != nil {
				t.Fatalf("GenerationSnapshotPayloadFrom() error = %v", err)
			}
			if len(payload.Events) != 1 || payload.Events[0].Kind != GenerationLifecycleEventRouteTaken ||
				payload.Events[0].SelectedRoute != string(target) {
				t.Fatalf("generation %d events = %#v, want route %q", generation, payload.Events, target)
			}
		}
	})
}

func TestCoordinatorGateRevisionCountersShouldStayIsolated(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 16, 12, 0, 0, 0, time.UTC)
	rejected := func(action gate.RouteAction) gate.Verdict {
		return gate.Verdict{
			Outcome: gate.VerdictOutcomeRejected,
			Route:   gate.RouteDecision{Placement: gate.PlacementInBody, Action: action},
		}
	}

	t.Run("Should advance only the gate that caused succession", func(t *testing.T) {
		t.Parallel()

		gateA := NodeControl{NodeID: "gate_a", Revision: 7, GateRevisions: map[int]int{0: 2}}
		gateB := NodeControl{NodeID: "gate_b", Revision: 3, GateRevisions: map[int]int{0: 1}}
		for wantRevision := 3; wantRevision <= 4; wantRevision++ {
			collector := &gateEvaluationCollector{}
			collector.recordWithControl(gate.Gate{ID: "gate_a"}, 0, rejected(gate.RouteRevise), gateA, now)
			collector.recordWithControl(
				gate.Gate{ID: "gate_b"}, 0,
				gate.Verdict{
					Outcome: gate.VerdictOutcomeApproved,
					Route:   gate.RouteDecision{Action: gate.RouteContinue},
				},
				gateB,
				now,
			)
			mutations := collector.gateRevisionMutations()
			if len(mutations) != 1 || mutations[0].NodeID != "gate_a" ||
				mutations[0].ExpectedRevision != gateA.Revision ||
				mutations[0].GateRevisions[0] != wantRevision {
				t.Fatalf(
					"gate revision mutations = %#v, want only gate_a item 0 at revision %d",
					mutations,
					wantRevision,
				)
			}
			gateA.Revision++
			gateA.GateRevisions = mutations[0].GateRevisions
		}
		if gateB.Revision != 3 || gateB.GateRevisions[0] != 1 {
			t.Fatalf("gate_b control = %#v, want untouched after two gate_a revisions", gateB)
		}
	})

	t.Run("Should advance fan-out lanes independently", func(t *testing.T) {
		t.Parallel()

		control := NodeControl{
			NodeID: "lane_gate", Revision: 4, GateRevisions: map[int]int{0: 1, 3: 4},
		}
		collector := &gateEvaluationCollector{}
		collector.recordWithControl(gate.Gate{ID: "lane_gate"}, 0, rejected(gate.RouteRevise), control, now)
		collector.recordWithControl(gate.Gate{ID: "lane_gate"}, 3, rejected(gate.RouteRevise), control, now)
		mutations := collector.gateRevisionMutations()
		if len(mutations) != 1 || mutations[0].GateRevisions[0] != 2 ||
			mutations[0].GateRevisions[3] != 5 {
			t.Fatalf("lane gate revisions = %#v, want item 0=2 and item 3=5", mutations)
		}
	})

	t.Run("Should record exhaustion without consuming another revision", func(t *testing.T) {
		t.Parallel()

		collector := &gateEvaluationCollector{}
		collector.recordWithControl(
			gate.Gate{ID: "exhausted_gate"}, 0,
			gate.Verdict{
				Outcome: gate.VerdictOutcomeRejected,
				Route:   gate.RouteDecision{Action: gate.RouteHalt, ReasonCode: "gate_max_revisions_exhausted"},
			},
			NodeControl{NodeID: "exhausted_gate", GateRevisions: map[int]int{0: 2}}, now,
		)
		plan := task.CoordinatorCompletionPlan{Snapshot: task.GenerationSnapshot{
			Payload: GenerationSnapshotPayload{},
		}}
		if err := applyGateEvaluationIntents(&plan, Run{}, 9, collector); err != nil {
			t.Fatalf("applyGateEvaluationIntents() error = %v", err)
		}
		payload, err := GenerationSnapshotPayloadFrom(plan.Snapshot.Payload)
		if err != nil {
			t.Fatalf("GenerationSnapshotPayloadFrom() error = %v", err)
		}
		if len(payload.Controls) != 0 || len(payload.Events) != 1 ||
			payload.Events[0].GateID != "exhausted_gate" ||
			payload.Events[0].Reason != "gate_max_revisions_exhausted" {
			t.Fatalf("exhaustion payload = %#v, want named gate event and no counter mutation", payload)
		}
	})

	t.Run("Should derive revision only from the persisted counter", func(t *testing.T) {
		t.Parallel()

		input, err := runtimeGateInput(
			Run{ID: "run-operator-generation", WorkspaceID: "ws-1", Generation: 99},
			0,
			&ResolvedDefinition{Definition: dsl.Definition{}},
			EffectiveConfig{},
			gate.PlacementInBody,
			nil,
		)
		if err != nil {
			t.Fatalf("runtimeGateInput() error = %v", err)
		}
		if input.Revision != 0 {
			t.Fatalf("GateInput.Revision = %d, want persisted counter 0 despite generation 99", input.Revision)
		}
	})
}

func TestPredicateEvaluationShouldSurfaceCostDiagnostics(t *testing.T) {
	t.Parallel()

	expression := `[1, 2, 3, 4, 5, 6, 7, 8].exists(value, value == 8)`
	compiler, err := refs.NewConditionCompiler(refs.Namespace{}, refs.WithCostLimit(10_000))
	if err != nil {
		t.Fatalf("NewConditionCompiler() error = %v", err)
	}
	condition, err := compiler.Compile(expression)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	baseline, err := condition.Evaluate(nil)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	condition.CostLimit = max(1, baseline.Cost)
	evaluated, err := evaluatePredicate("nodes.route.condition", condition, nil, PredicateRouting, "")
	if err != nil {
		t.Fatalf("evaluatePredicate(warning) error = %v", err)
	}
	if len(evaluated.Diagnostics) != 1 || !evaluated.Diagnostics[0].Warning ||
		evaluated.Diagnostics[0].Code != predicateCostWarningCode ||
		evaluated.Diagnostics[0].Cost != baseline.Cost {
		t.Fatalf("cost warning diagnostics = %#v, want tracked threshold warning", evaluated.Diagnostics)
	}

	limitedCompiler, err := refs.NewConditionCompiler(refs.Namespace{}, refs.WithCostLimit(1))
	if err != nil {
		t.Fatalf("NewConditionCompiler(limited) error = %v", err)
	}
	limited, err := limitedCompiler.Compile(expression)
	if err != nil {
		t.Fatalf("Compile(limited) error = %v", err)
	}
	exhausted, err := evaluatePredicate("nodes.route.condition", limited, nil, PredicateRouting, "")
	if err != nil {
		t.Fatalf("evaluatePredicate(limit) error = %v", err)
	}
	if exhausted.Disposition == nil || exhausted.Disposition.Failure == nil ||
		exhausted.Disposition.Failure.Class != FailureAuthoring ||
		slices.ContainsFunc(exhausted.Diagnostics, func(diagnostic PredicateDiagnostic) bool {
			return diagnostic.Warning
		}) ||
		!slices.ContainsFunc(exhausted.Diagnostics, func(diagnostic PredicateDiagnostic) bool {
			return diagnostic.Code == "predicate_evaluation_failed" && diagnostic.CostLimit == 1
		}) {
		t.Fatalf("cost-limit result = %#v, want authoring failure with cost limit diagnostic", exhausted)
	}
	for _, diagnostic := range exhausted.Diagnostics {
		if err := predicateDiagnosticEvent(diagnostic).validate(); err != nil {
			t.Fatalf("persist cost-limit diagnostic error = %v", err)
		}
	}
}

func TestCoordinatorRunnerShouldReconcileReadyDependentsFromGenerationSnapshot(t *testing.T) {
	t.Run("Should reconcile ready dependents from generation snapshot", func(t *testing.T) {
		t.Parallel()

		loopRun := Run{
			ID:          "looprun-finisher-ready",
			WorkspaceID: "ws-1",
			LoopName:    "delivery",
			Status:      StatusRunning,
			Generation:  1,
		}
		coordinatorRun := task.Run{
			ID:        "run-coordinator-finisher-ready",
			TaskID:    "task-coordinator-finisher-ready",
			RunKind:   task.RunKindCoordinator,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusClaimed,
		}
		rootRun := task.Run{
			ID:        coordinatorNodeRunID(loopRun.ID, 1, "load", 0),
			TaskID:    coordinatorNodeTaskID(loopRun.ID, 1, "load", 0),
			RunKind:   task.RunKindWorker,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusCompleted,
		}
		runner := newCoordinatorRunnerForTest(t, loopRun, coordinatorRun, map[string]task.Run{
			coordinatorRun.ID: coordinatorRun,
			rootRun.ID:        rootRun,
		}, coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
			{
				Generation: 1,
				NodeID:     "load",
				Status:     generationOutputEnqueued,
				TaskRunID:  rootRun.ID,
			},
			{
				Generation: 1,
				NodeID:     "agent",
				Status:     generationOutputPending,
			},
		}}})

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if got, want := len(plan.NodeRuns), 1; got != want {
			t.Fatalf("node runs = %d, want %d", got, want)
		}
		if got, want := plan.NodeRuns[0].TaskID, coordinatorNodeTaskID(loopRun.ID, 1, "agent", 0); got != want {
			t.Fatalf("node run task_id = %q, want %q", got, want)
		}
		if plan.Terminal != nil {
			t.Fatalf("Terminal = %#v, want nil while dependent is ready", plan.Terminal)
		}
		payload := coordinatorSnapshotPayloadForTest(t, plan)
		statuses := map[string]string{}
		for _, output := range payload.Outputs {
			statuses[output.NodeID] = output.Status
		}
		if got, want := statuses["load"], generationOutputSucceeded; got != want {
			t.Fatalf("load status = %q, want %q", got, want)
		}
		if got, want := statuses["agent"], generationOutputPending; got != want {
			t.Fatalf("agent status = %q, want %q before reservation", got, want)
		}
		postReserve := coordinatorPostReservePayloadForTest(t, plan)
		postStatuses := map[string]string{}
		for _, output := range postReserve.Outputs {
			postStatuses[output.NodeID] = output.Status
		}
		if got, want := postStatuses["agent"], generationOutputEnqueued; got != want {
			t.Fatalf("agent status = %q, want %q", got, want)
		}
	})
}

func TestCoordinatorRunnerShouldMarkReadyPlanInFlightWhenSiblingIsLive(t *testing.T) {
	t.Run("Should mark ready plan in flight when sibling is live", func(t *testing.T) {
		t.Parallel()

		loopRun := Run{
			ID:          "looprun-finisher-ready-live",
			WorkspaceID: "ws-1",
			LoopName:    "delivery",
			Status:      StatusRunning,
			Generation:  1,
		}
		coordinatorRun := task.Run{
			ID:        "run-coordinator-finisher-ready-live",
			TaskID:    "task-coordinator-finisher-ready-live",
			RunKind:   task.RunKindCoordinator,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusClaimed,
		}
		completedRun := task.Run{
			ID:        coordinatorNodeRunID(loopRun.ID, 1, "load", 0),
			TaskID:    coordinatorNodeTaskID(loopRun.ID, 1, "load", 0),
			RunKind:   task.RunKindWorker,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusCompleted,
		}
		liveRun := task.Run{
			ID:        coordinatorNodeRunID(loopRun.ID, 1, "slow", 0),
			TaskID:    coordinatorNodeTaskID(loopRun.ID, 1, "slow", 0),
			RunKind:   task.RunKindWorker,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusRunning,
		}
		graph := dsl.Graph{
			Nodes: []dsl.Node{
				{ID: "load", Class: dsl.NodeClassSource, Kind: string(dsl.SourceInput)},
				{ID: "agent", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent)},
				{ID: "slow", Class: dsl.NodeClassSource, Kind: string(dsl.SourceInput)},
			},
			Edges: []dsl.Edge{{From: "load", To: "agent"}},
		}
		runner := newCoordinatorRunnerForTestWithGraph(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{
				coordinatorRun.ID: coordinatorRun,
				completedRun.ID:   completedRun,
				liveRun.ID:        liveRun,
			},
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
				{
					Generation: 1,
					NodeID:     "load",
					Status:     generationOutputEnqueued,
					TaskRunID:  completedRun.ID,
				},
				{
					Generation: 1,
					NodeID:     "agent",
					Status:     generationOutputPending,
				},
				{
					Generation: 1,
					NodeID:     "slow",
					Status:     generationOutputEnqueued,
					TaskRunID:  liveRun.ID,
				},
			}}},
			graph,
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if !plan.GenerationInFlight {
			t.Fatal("GenerationInFlight = false, want true while sibling node is live")
		}
		if got, want := len(plan.NodeRuns), 1; got != want {
			t.Fatalf("node runs = %d, want %d", got, want)
		}
		if got, want := plan.NodeRuns[0].TaskID, coordinatorNodeTaskID(loopRun.ID, 1, "agent", 0); got != want {
			t.Fatalf("node run task_id = %q, want %q", got, want)
		}
		if plan.Yield {
			t.Fatal("Yield = true, want false so ready work can dispatch when not paused/budgeted")
		}
	})
}

func TestCoordinatorRunnerShouldYieldWhenGenerationStillHasLiveNode(t *testing.T) {
	t.Run("Should yield when generation still has live node", func(t *testing.T) {
		t.Parallel()

		loopRun := Run{
			ID:          "looprun-live-yield",
			WorkspaceID: "ws-1",
			LoopName:    "delivery",
			Status:      StatusRunning,
			Generation:  1,
		}
		coordinatorRun := task.Run{
			ID:        "run-coordinator-live-yield",
			TaskID:    "task-coordinator-live-yield",
			RunKind:   task.RunKindCoordinator,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusClaimed,
		}
		rootRun := task.Run{
			ID:        coordinatorNodeRunID(loopRun.ID, 1, "load", 0),
			TaskID:    coordinatorNodeTaskID(loopRun.ID, 1, "load", 0),
			RunKind:   task.RunKindWorker,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusRunning,
		}
		runner := newCoordinatorRunnerForTest(t, loopRun, coordinatorRun, map[string]task.Run{
			coordinatorRun.ID: coordinatorRun,
			rootRun.ID:        rootRun,
		}, coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {{
			Generation: 1,
			NodeID:     "load",
			Status:     generationOutputEnqueued,
			TaskRunID:  rootRun.ID,
		}}}})

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if !plan.Yield {
			t.Fatal("Yield = false, want true while a node is live")
		}
		if len(plan.NodeRuns) != 0 || plan.Terminal != nil {
			t.Fatalf(
				"plan enqueues/terminalizes while yielding: runs=%d terminal=%#v",
				len(plan.NodeRuns),
				plan.Terminal,
			)
		}
	})
}

func TestCoordinatorRunnerShouldKeepHealthyLaneRunningWhenTargetIsUnavailable(t *testing.T) {
	t.Parallel()

	t.Run("Should yield instead of terminalizing while an independent lane is live", func(t *testing.T) {
		t.Parallel()

		loopRun := Run{
			ID: "looprun-target-unavailable-live", WorkspaceID: "ws-1", LoopName: "delivery",
			Status: StatusRunning, Generation: 1,
		}
		coordinatorRun := task.Run{
			ID: "run-coordinator-target-unavailable-live", TaskID: "task-coordinator-target-unavailable-live",
			RunKind: task.RunKindCoordinator, LoopRunID: string(loopRun.ID), Status: task.TaskRunStatusClaimed,
		}
		healthyRun := task.Run{
			ID:      coordinatorNodeRunID(loopRun.ID, 1, "healthy", 0),
			TaskID:  coordinatorNodeTaskID(loopRun.ID, 1, "healthy", 0),
			RunKind: task.RunKindWorker, LoopRunID: string(loopRun.ID), Status: task.TaskRunStatusRunning,
		}
		failureRef, ok := ActionFailureOutputRef(NewActionFailure(
			targetUnavailableReasonCode,
			"target health breaker is open",
			"repair the target and requeue the node",
		))
		if !ok {
			t.Fatal("ActionFailureOutputRef() = false")
		}
		graph := dsl.Graph{Nodes: []dsl.Node{
			{ID: "sick", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform)},
			{ID: "healthy", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform)},
		}}
		runner := newCoordinatorRunnerForTestWithGraph(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{coordinatorRun.ID: coordinatorRun, healthyRun.ID: healthyRun},
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
				{Generation: 1, NodeID: "sick", Status: generationOutputFailed, OutputRef: failureRef},
				{
					Generation: 1, NodeID: "healthy", Status: generationOutputEnqueued,
					TaskRunID: healthyRun.ID,
				},
			}}},
			graph,
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.Terminal != nil || !plan.GenerationInFlight || !plan.Yield {
			t.Fatalf(
				"plan terminal/in-flight/yield = %#v/%t/%t, want nil/true/true",
				plan.Terminal,
				plan.GenerationInFlight,
				plan.Yield,
			)
		}
		if len(plan.NodeRuns) != 0 || plan.NextCoordinator != nil {
			t.Fatalf("plan scheduled work while healthy lane runs: %#v", plan)
		}
	})
}

func TestCoordinatorRunnerShouldYieldWhileAwaitingChildLoop(t *testing.T) {
	t.Run("Should yield while awaiting child loop", func(t *testing.T) {
		t.Parallel()

		loopRun := Run{
			ID:          "looprun-awaiting-child-live",
			WorkspaceID: "ws-1",
			LoopName:    "delivery",
			Status:      StatusRunning,
			Generation:  1,
		}
		childRun := Run{
			ID:              "looprun-awaiting-child-live-child",
			WorkspaceID:     "ws-1",
			LoopName:        "child",
			Status:          StatusRunning,
			Generation:      1,
			ParentLoopRunID: loopRun.ID,
			CreatedAt:       time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC),
		}
		coordinatorRun := task.Run{
			ID:        "run-coordinator-awaiting-child-live",
			TaskID:    "task-coordinator-awaiting-child-live",
			RunKind:   task.RunKindCoordinator,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusClaimed,
		}
		runner := newCoordinatorRunnerForTest(t, loopRun, coordinatorRun, map[string]task.Run{
			coordinatorRun.ID: coordinatorRun,
		}, coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {{
			Generation:     1,
			NodeID:         "load",
			Status:         generationOutputAwaitingChild,
			ChildLoopRunID: string(childRun.ID),
		}}}})
		setCoordinatorRunnerRunsForTest(t, runner, map[RunID]Run{
			loopRun.ID:  loopRun,
			childRun.ID: childRun,
		})

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if !plan.Yield {
			t.Fatal("Yield = false, want true while child loop is live")
		}
		if len(plan.RunStops) != 0 || plan.Terminal != nil {
			t.Fatalf("RunStops/Terminal = %#v/%#v, want none", plan.RunStops, plan.Terminal)
		}
	})
}

func TestCoordinatorRunnerShouldRestoreAwaitedChildFromCompletedActionResult(t *testing.T) {
	t.Parallel()

	t.Run("Should keep dependent blocked while completed action child is live", func(t *testing.T) {
		t.Parallel()

		parent := Run{
			ID:          "looprun-await-result-parent",
			WorkspaceID: "ws-1",
			LoopName:    "parent",
			Status:      StatusRunning,
			Generation:  1,
		}
		child := Run{
			ID:              "looprun-await-result-child",
			WorkspaceID:     parent.WorkspaceID,
			LoopName:        "child",
			Status:          StatusRunning,
			Generation:      1,
			ParentLoopRunID: parent.ID,
		}
		coordinatorRun := task.Run{
			ID:        "run-coordinator-await-result",
			TaskID:    "task-coordinator-await-result",
			RunKind:   task.RunKindCoordinator,
			LoopRunID: string(parent.ID),
			Status:    task.TaskRunStatusClaimed,
		}
		completedRun := taskRunWithResult(task.Run{
			ID:        coordinatorNodeRunID(parent.ID, 1, "first_child", 0),
			TaskID:    coordinatorNodeTaskID(parent.ID, 1, "first_child", 0),
			RunKind:   task.RunKindWorker,
			LoopRunID: string(parent.ID),
			Status:    task.TaskRunStatusCompleted,
		}, json.RawMessage(
			`{"loop_run_id":"looprun-await-result-child","status":"awaiting_child"}`,
		))
		graph := dsl.Graph{
			Nodes: []dsl.Node{
				{
					ID:    "first_child",
					Class: dsl.NodeClassAction,
					Kind:  string(dsl.ActionRunLoop),
					Params: dsl.NodeParams{
						"loop": "child",
						"mode": string(dsl.RunLoopAwait),
					},
				},
				{
					ID:    "second_child",
					Class: dsl.NodeClassAction,
					Kind:  string(dsl.ActionRunLoop),
					Params: dsl.NodeParams{
						"loop": "child",
						"mode": string(dsl.RunLoopAwait),
					},
				},
			},
			Edges: []dsl.Edge{{From: "first_child", To: "second_child"}},
		}
		runner := newCoordinatorRunnerForTestWithGraph(
			t,
			parent,
			coordinatorRun,
			map[string]task.Run{
				coordinatorRun.ID: coordinatorRun,
				completedRun.ID:   completedRun,
			},
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
				{
					Generation: 1,
					NodeID:     "first_child",
					Status:     generationOutputEnqueued,
					TaskRunID:  completedRun.ID,
				},
				{
					Generation: 1,
					NodeID:     "second_child",
					Status:     generationOutputPending,
				},
			}}},
			graph,
		)
		setCoordinatorRunnerRunsForTest(t, runner, map[RunID]Run{
			parent.ID: parent,
			child.ID:  child,
		})

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if !plan.Yield {
			t.Fatal("Yield = false, want true while restored child wait is live")
		}
		if len(plan.NodeRuns) != 0 || plan.Terminal != nil {
			t.Fatalf("NodeRuns/Terminal = %#v/%#v, want no dependent or terminal", plan.NodeRuns, plan.Terminal)
		}
		outputs := outputsByNodeForTest(coordinatorSnapshotPayloadForTest(t, plan).Outputs)
		first := outputs["first_child"]
		if got, want := first.Status, generationOutputAwaitingChild; got != want {
			t.Fatalf("first_child status = %q, want %q", got, want)
		}
		if got, want := first.ChildLoopRunID, string(child.ID); got != want {
			t.Fatalf("first_child child_loop_run_id = %q, want %q", got, want)
		}
		if got, want := outputs["second_child"].Status, generationOutputPending; got != want {
			t.Fatalf("second_child status = %q, want %q", got, want)
		}
	})
}

func TestRefreshCompletedTaskRunOutputShouldValidateRunLoopAwaitResult(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name      string
		graph     dsl.Graph
		result    json.RawMessage
		wantCause string
	}{
		{name: "Should reject an empty result"},
		{name: "Should reject malformed JSON", result: json.RawMessage(`{"loop_run_id":`)},
		{name: "Should reject a missing child id", result: json.RawMessage(`{"status":"awaiting_child"}`)},
		{name: "Should reject a whitespace child id", result: json.RawMessage(`{"loop_run_id":" ","status":"awaiting_child"}`)},
		{name: "Should reject a contradictory status", result: json.RawMessage(`{"loop_run_id":"looprun-child","status":"completed"}`)},
		{
			name:      "Should reject a missing owner node",
			graph:     dsl.Graph{Nodes: []dsl.Node{}},
			result:    json.RawMessage(`{"loop_run_id":"looprun-child","status":"awaiting_child"}`),
			wantCause: "completed action owner node is missing",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			wantCause := testCase.wantCause
			if wantCause == "" {
				wantCause = "completed run-loop await result is invalid"
			}

			graph := testCase.graph
			if graph.Nodes == nil {
				graph = dsl.Graph{Nodes: []dsl.Node{{
					ID:    "child",
					Class: dsl.NodeClassAction,
					Kind:  string(dsl.ActionRunLoop),
					Params: dsl.NodeParams{
						"loop": "child-loop",
						"mode": string(dsl.RunLoopAwait),
					},
				}}}
			}
			output, live, stops, terminal, err := refreshCompletedTaskRunOutput(
				Run{ID: "looprun-parent", WorkspaceID: "ws-1"},
				graph,
				GenerationOutput{NodeID: "child", Status: generationOutputEnqueued},
				taskRunWithResult(task.Run{Status: task.TaskRunStatusCompleted}, testCase.result),
			)
			if err != nil {
				t.Fatalf("refreshCompletedTaskRunOutput() error = %v", err)
			}
			if live || len(stops) != 0 || terminal != nil || output.Status != generationOutputFailed {
				t.Fatalf(
					"completed output = %#v live=%t stops=%#v terminal=%#v",
					output,
					live,
					stops,
					terminal,
				)
			}
			failure := classifyGenerationOutputFailure(output, task.Run{})
			if failure.Class != FailureAuthoring || failure.Code != string(ReasonCodeInvalidOutput) ||
				failure.Cause != wantCause || failure.Target != "child" {
				t.Fatalf("classified invalid result = %#v", failure)
			}
		})
	}
}

func TestRefreshCompletedTaskRunOutputShouldRevalidateRunAgentSchema(t *testing.T) {
	t.Parallel()

	node := dsl.Node{
		ID: "worker", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent),
		Params: dsl.NodeParams{"output_schema": map[string]any{
			"type": "object", "properties": map[string]any{
				"status":  map[string]any{"type": "string"},
				"summary": map[string]any{"type": "string"},
			}, "required": []string{"status", "summary"},
		}},
	}
	graph := dsl.Graph{Nodes: []dsl.Node{node}}

	t.Run("Should fail a completed capture missing a required field", func(t *testing.T) {
		t.Parallel()

		output, live, stops, terminal, err := refreshCompletedTaskRunOutput(
			Run{ID: "looprun-parent", WorkspaceID: "ws-1"},
			graph,
			GenerationOutput{NodeID: "worker", Status: generationOutputEnqueued},
			taskRunWithResult(
				task.Run{Status: task.TaskRunStatusCompleted},
				json.RawMessage(`{"status":"done"}`),
			),
		)
		if err != nil {
			t.Fatalf("refreshCompletedTaskRunOutput() error = %v", err)
		}
		if live || len(stops) != 0 || terminal != nil || output.Status != generationOutputFailed {
			t.Fatalf("completed output = %#v live=%t stops=%#v terminal=%#v", output, live, stops, terminal)
		}
		failure := classifyGenerationOutputFailure(output, task.Run{})
		if failure.Class != FailureAuthoring || failure.Code != string(ReasonCodeInvalidOutput) {
			t.Fatalf("classified invalid output = %#v, want authoring invalid_output", failure)
		}
	})

	t.Run("Should accept the exact resolved externalized payload", func(t *testing.T) {
		t.Parallel()

		payload := json.RawMessage(`{"status":"done","summary":"complete"}`)
		output := GenerationOutput{
			NodeID: "worker", Status: generationOutputEnqueued, OutputRef: OutputRefForPayload(payload),
			runtimePayload: payload,
		}
		refreshed, live, stops, terminal, err := refreshCompletedTaskRunOutput(
			Run{ID: "looprun-parent", WorkspaceID: "ws-1"},
			graph,
			output,
			task.Run{Status: task.TaskRunStatusCompleted},
		)
		if err != nil {
			t.Fatalf("refreshCompletedTaskRunOutput() error = %v", err)
		}
		if live || len(stops) != 0 || terminal != nil || refreshed.Status != generationOutputSucceeded {
			t.Fatalf("completed output = %#v live=%t stops=%#v terminal=%#v", refreshed, live, stops, terminal)
		}
	})
}

func TestRefreshCompletedTaskRunOutputShouldPreserveRunLoopDetachSuccess(t *testing.T) {
	t.Parallel()

	t.Run("Should keep detached child as immediate success", func(t *testing.T) {
		t.Parallel()

		graph := dsl.Graph{Nodes: []dsl.Node{{
			ID:    "child",
			Class: dsl.NodeClassAction,
			Kind:  string(dsl.ActionRunLoop),
			Params: dsl.NodeParams{
				"loop": "child-loop",
				"mode": string(dsl.RunLoopDetach),
			},
		}}}
		output, live, stops, terminal, err := refreshCompletedTaskRunOutput(
			Run{ID: "looprun-parent", WorkspaceID: "ws-1"},
			graph,
			GenerationOutput{NodeID: "child", Status: generationOutputEnqueued},
			taskRunWithResult(task.Run{
				Status: task.TaskRunStatusCompleted,
			}, json.RawMessage(`{"loop_run_id":"looprun-detached-child"}`)),
		)
		if err != nil {
			t.Fatalf("refreshCompletedTaskRunOutput() error = %v", err)
		}
		if live || len(stops) != 0 || terminal != nil {
			t.Fatalf("live/stops/terminal = %t/%#v/%#v, want none", live, stops, terminal)
		}
		if got, want := output.Status, generationOutputSucceeded; got != want {
			t.Fatalf("detached status = %q, want %q", got, want)
		}
		if output.ChildLoopRunID != "" {
			t.Fatalf("detached child_loop_run_id = %q, want no parent wait ownership", output.ChildLoopRunID)
		}
	})
}

func TestCoordinatorRunnerShouldResolveCompletedAwaitedChildTerminal(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name       string
		status     Status
		wantStatus string
	}{
		{name: "Should resolve a done child", status: StatusDone, wantStatus: generationOutputSucceeded},
		{name: "Should resolve a no-op child", status: StatusNoOp, wantStatus: generationOutputSucceeded},
		{name: "Should resolve a failed child", status: StatusFailed, wantStatus: generationOutputFailed},
		{name: "Should resolve a canceled child", status: StatusCanceled, wantStatus: generationOutputFailed},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			parent := Run{
				ID: "looprun-completed-child-parent", WorkspaceID: "ws-1", LoopName: "parent",
				Status: StatusRunning, Generation: 1,
			}
			child := Run{
				ID: "looprun-completed-child", WorkspaceID: parent.WorkspaceID, LoopName: "child",
				Status: testCase.status, Generation: 1, ParentLoopRunID: parent.ID,
			}
			completedRun := taskRunWithResult(task.Run{
				ID: "run-completed-child", RunKind: task.RunKindWorker,
				LoopRunID: string(parent.ID), Status: task.TaskRunStatusCompleted,
			}, json.RawMessage(
				`{"loop_run_id":"looprun-completed-child","status":"awaiting_child"}`,
			))
			graph := dsl.Graph{Nodes: []dsl.Node{{
				ID:    "child",
				Class: dsl.NodeClassAction,
				Kind:  string(dsl.ActionRunLoop),
				Params: dsl.NodeParams{
					"loop": "child-loop",
					"mode": string(dsl.RunLoopAwait),
				},
			}}}
			coordinatorRun := task.Run{
				ID: "run-coordinator-completed-child", TaskID: "task-coordinator-completed-child",
				RunKind: task.RunKindCoordinator, LoopRunID: string(parent.ID), Status: task.TaskRunStatusClaimed,
			}
			runner := newCoordinatorRunnerForTestWithGraph(
				t,
				parent,
				coordinatorRun,
				map[string]task.Run{
					coordinatorRun.ID: coordinatorRun,
					completedRun.ID:   completedRun,
				},
				coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{}},
				graph,
			)
			setCoordinatorRunnerRunsForTest(t, runner, map[RunID]Run{parent.ID: parent, child.ID: child})

			output, live, stops, terminal, err := runner.refreshGenerationOutputFromTaskRun(
				context.Background(),
				parent,
				graph,
				GenerationOutput{
					NodeID: "child", Status: generationOutputEnqueued, TaskRunID: completedRun.ID,
				},
			)
			if err != nil {
				t.Fatalf("refreshGenerationOutputFromTaskRun() error = %v", err)
			}
			if live || len(stops) != 0 || terminal != nil {
				t.Fatalf("live/stops/terminal = %t/%#v/%#v, want none", live, stops, terminal)
			}
			if got := output.Status; got != testCase.wantStatus {
				t.Fatalf("output status = %q, want %q", got, testCase.wantStatus)
			}
			if got, want := output.ChildLoopRunID, string(child.ID); got != want {
				t.Fatalf("child_loop_run_id = %q, want %q", got, want)
			}
		})
	}
}

func TestCoordinatorRunnerShouldResolveAwaitingChildCoordinatorTerminal(t *testing.T) {
	t.Run("Should resolve awaiting child loop terminal", func(t *testing.T) {
		t.Parallel()

		loopRun := Run{
			ID:          "looprun-awaiting-child-terminal",
			WorkspaceID: "ws-1",
			LoopName:    "delivery",
			Status:      StatusRunning,
			Generation:  1,
		}
		childRun := Run{
			ID:              "looprun-awaiting-child-terminal-child",
			WorkspaceID:     "ws-1",
			LoopName:        "child",
			Status:          StatusDone,
			Generation:      1,
			ParentLoopRunID: loopRun.ID,
		}
		coordinatorRun := task.Run{
			ID:        "run-coordinator-awaiting-child-terminal",
			TaskID:    "task-coordinator-awaiting-child-terminal",
			RunKind:   task.RunKindCoordinator,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusClaimed,
		}
		runner := newCoordinatorRunnerForTest(t, loopRun, coordinatorRun, map[string]task.Run{
			coordinatorRun.ID: coordinatorRun,
		}, coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {{
			Generation:     1,
			NodeID:         "load",
			Status:         generationOutputAwaitingChild,
			ChildLoopRunID: string(childRun.ID),
		}, {
			Generation: 1,
			NodeID:     "agent",
			Status:     generationOutputPending,
		}}}})
		setCoordinatorRunnerRunsForTest(t, runner, map[RunID]Run{
			loopRun.ID:  loopRun,
			childRun.ID: childRun,
		})

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if got, want := len(plan.NodeRuns), 1; got != want {
			t.Fatalf("node runs = %d, want %d after child success", got, want)
		}
		payload := coordinatorSnapshotPayloadForTest(t, plan)
		statuses := map[string]string{}
		for _, output := range payload.Outputs {
			statuses[output.NodeID] = output.Status
		}
		if got, want := statuses["load"], generationOutputSucceeded; got != want {
			t.Fatalf("load status = %q, want %q", got, want)
		}
		if got, want := statuses["agent"], generationOutputPending; got != want {
			t.Fatalf("agent status = %q, want %q before reservation", got, want)
		}
		postReserve := coordinatorPostReservePayloadForTest(t, plan)
		postStatuses := map[string]string{}
		for _, output := range postReserve.Outputs {
			postStatuses[output.NodeID] = output.Status
		}
		if got, want := postStatuses["agent"], generationOutputEnqueued; got != want {
			t.Fatalf("agent status = %q, want %q", got, want)
		}
	})
}

// Invariant: an awaited child terminal crosses the composite-node boundary as a bounded
// classified failure with the exact child run reference. A child cancellation never mutates
// the parent Run directly. The awaiting-child coordinator suite owns this boundary.
func TestCoordinatorRunnerShouldClassifyAwaitedChildTerminalFailClosed(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name      string
		status    Status
		wantClass FailureClass
	}{
		{name: "failed child", status: StatusFailed, wantClass: FailureTransport},
		{name: "canceled child", status: StatusCanceled, wantClass: FailureCancellation},
	} {
		t.Run("Should classify "+testCase.name, func(t *testing.T) {
			t.Parallel()

			parent := Run{
				ID: "looprun-parent", WorkspaceID: "ws-1", LoopName: "parent",
				Status: StatusRunning, Generation: 1,
			}
			child := Run{
				ID: "looprun-child", WorkspaceID: parent.WorkspaceID, LoopName: "child",
				Status: testCase.status, Generation: 1, ParentLoopRunID: parent.ID,
			}
			coordinatorRun := task.Run{
				ID: "run-coordinator-child-boundary", TaskID: "task-coordinator-child-boundary",
				RunKind: task.RunKindCoordinator, LoopRunID: string(parent.ID), Status: task.TaskRunStatusClaimed,
			}
			runner := newCoordinatorRunnerForTestWithGraph(
				t, parent, coordinatorRun, map[string]task.Run{coordinatorRun.ID: coordinatorRun},
				coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{}},
				dsl.Graph{Nodes: []dsl.Node{{
					ID: "child", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunLoop),
				}}},
			)
			setCoordinatorRunnerRunsForTest(t, runner, map[RunID]Run{parent.ID: parent, child.ID: child})
			output, live, stops, err := runner.refreshAwaitingChildOutput(
				context.Background(), parent, dsl.Graph{}, GenerationOutput{
					Generation: 1, NodeID: "child", Status: generationOutputAwaitingChild,
					ChildLoopRunID: string(child.ID),
				},
			)
			if err != nil {
				t.Fatalf("refreshAwaitingChildOutput() error = %v", err)
			}
			if live || len(stops) != 0 || output.Status != generationOutputFailed {
				t.Fatalf("awaited child output = %#v live=%t stops=%#v", output, live, stops)
			}
			failure := classifyGenerationOutputFailure(output, task.Run{})
			if failure.Class != testCase.wantClass || failure.Target != string(child.ID) ||
				failure.Code != childLoopStatusRef(testCase.status) {
				t.Fatalf("classified child failure = %#v", failure)
			}
			storedParent, err := runner.store.GetLoopRunByID(context.Background(), parent.ID)
			if err != nil {
				t.Fatalf("GetLoopRunByID(parent) error = %v", err)
			}
			if storedParent.Status != StatusRunning || storedParent.CancelRequested {
				t.Fatalf("child terminal mutated persisted parent = %#v", storedParent)
			}
		})
	}
}

func TestCoordinatorRunnerShouldRejectAwaitedChildOutsideParentBoundary(t *testing.T) {
	t.Parallel()

	parent := Run{
		ID: "looprun-parent-boundary", WorkspaceID: "ws-1", LoopName: "parent",
		Status: StatusRunning, Generation: 1,
	}
	for _, testCase := range []struct {
		name  string
		child Run
	}{
		{
			name: "Should reject a child from another workspace",
			child: Run{
				ID: "looprun-child-other-workspace", WorkspaceID: "ws-2", LoopName: "child",
				Status: StatusRunning, Generation: 1, ParentLoopRunID: parent.ID,
			},
		},
		{
			name: "Should reject a child owned by another parent",
			child: Run{
				ID: "looprun-child-other-parent", WorkspaceID: parent.WorkspaceID, LoopName: "child",
				Status: StatusRunning, Generation: 1, ParentLoopRunID: "looprun-another-parent",
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			coordinatorRun := task.Run{
				ID: "run-coordinator-child-boundary", TaskID: "task-coordinator-child-boundary",
				RunKind: task.RunKindCoordinator, LoopRunID: string(parent.ID), Status: task.TaskRunStatusClaimed,
			}
			runner := newCoordinatorRunnerForTestWithGraph(
				t,
				parent,
				coordinatorRun,
				map[string]task.Run{coordinatorRun.ID: coordinatorRun},
				coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{}},
				dsl.Graph{Nodes: []dsl.Node{{
					ID: "child", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunLoop),
				}}},
			)
			setCoordinatorRunnerRunsForTest(t, runner, map[RunID]Run{
				parent.ID:         parent,
				testCase.child.ID: testCase.child,
			})

			output, live, stops, err := runner.refreshAwaitingChildOutput(
				context.Background(), parent, dsl.Graph{}, GenerationOutput{
					Generation: 1, NodeID: "child", Status: generationOutputAwaitingChild,
					ChildLoopRunID: string(testCase.child.ID),
				},
			)
			if err != nil {
				t.Fatalf("refreshAwaitingChildOutput() error = %v", err)
			}
			if live || len(stops) != 0 || output.Status != generationOutputFailed {
				t.Fatalf("awaited child output = %#v live=%t stops=%#v", output, live, stops)
			}
			if output.ChildLoopRunID != "" {
				t.Fatalf("failed boundary child_loop_run_id = %q, want cleared", output.ChildLoopRunID)
			}
			failure := classifyGenerationOutputFailure(output, task.Run{})
			if failure.Class != FailureAuthoring || failure.Target != string(testCase.child.ID) ||
				failure.Code != string(ReasonCodeInvalidOutput) ||
				failure.Cause != "awaited child Loop is outside the parent boundary" {
				t.Fatalf("classified boundary failure = %#v", failure)
			}
		})
	}
}

func TestCoordinatorRunnerShouldRejectMalformedAwaitedChildIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should reject child id that would require normalization", func(t *testing.T) {
		t.Parallel()

		parent := Run{
			ID: "looprun-parent-identity", WorkspaceID: "ws-1", LoopName: "parent",
			Status: StatusRunning, Generation: 1,
		}
		child := Run{
			ID: "looprun-child-identity", WorkspaceID: parent.WorkspaceID, LoopName: "child",
			Status: StatusRunning, Generation: 1, ParentLoopRunID: parent.ID,
		}
		coordinatorRun := task.Run{
			ID: "run-coordinator-child-identity", TaskID: "task-coordinator-child-identity",
			RunKind: task.RunKindCoordinator, LoopRunID: string(parent.ID), Status: task.TaskRunStatusClaimed,
		}
		runner := newCoordinatorRunnerForTestWithGraph(
			t,
			parent,
			coordinatorRun,
			map[string]task.Run{coordinatorRun.ID: coordinatorRun},
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{}},
			dsl.Graph{Nodes: []dsl.Node{{
				ID: "child", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunLoop),
			}}},
		)
		setCoordinatorRunnerRunsForTest(t, runner, map[RunID]Run{parent.ID: parent, child.ID: child})

		output, live, stops, err := runner.refreshAwaitingChildOutput(
			context.Background(), parent, dsl.Graph{}, GenerationOutput{
				Generation: 1, NodeID: "child", Status: generationOutputAwaitingChild,
				ChildLoopRunID: " " + string(child.ID),
			},
		)
		if err != nil {
			t.Fatalf("refreshAwaitingChildOutput() error = %v", err)
		}
		if live || len(stops) != 0 || output.Status != generationOutputFailed {
			t.Fatalf("awaited child output = %#v live=%t stops=%#v", output, live, stops)
		}
		if output.ChildLoopRunID != "" {
			t.Fatalf("malformed child_loop_run_id = %q, want cleared", output.ChildLoopRunID)
		}
		failure := classifyGenerationOutputFailure(output, task.Run{})
		if failure.Class != FailureAuthoring || failure.Code != string(ReasonCodeInvalidOutput) ||
			failure.Cause != "awaited child Loop identity is invalid" {
			t.Fatalf("classified malformed child identity = %#v", failure)
		}
	})
}

// Invariant: a terminal parent plan carries only owned terminate/cancel children; abandon is
// intentionally absent, and an omitted policy defaults to terminate. The coordinator suite
// owns authored parent-close selection.
func TestAttachCoordinatorParentCloseIntentsShouldHonorAuthoredPolicy(t *testing.T) {
	t.Parallel()

	parent := Run{ID: "looprun-parent-close", WorkspaceID: "ws-1", LoopName: "parent"}
	resolved := &ResolvedDefinition{Definition: dsl.Definition{Graph: dsl.Graph{Nodes: []dsl.Node{
		{
			ID: "default_child", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunLoop),
			NodeLifecycleState: &dsl.NodeLifecycleState{},
		},
		{
			ID: "cancel_child", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunLoop),
			NodeLifecycleState: &dsl.NodeLifecycleState{OnParentClose: dsl.ParentCloseCancel},
		},
		{
			ID: "abandon_child", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunLoop),
			NodeLifecycleState: &dsl.NodeLifecycleState{OnParentClose: dsl.ParentCloseAbandon},
		},
	}}}}
	plan := task.CoordinatorCompletionPlan{
		Snapshot: task.GenerationSnapshot{Payload: GenerationSnapshotPayload{Outputs: []GenerationOutput{
			{NodeID: "default_child", ChildLoopRunID: "child-default"},
			{NodeID: "cancel_child", ChildLoopRunID: "child-cancel"},
			{NodeID: "abandon_child", ChildLoopRunID: "child-abandon"},
		}}},
		Terminal: &task.CoordinatorTerminal{Status: string(StatusFailed)},
	}
	if err := attachCoordinatorParentCloseIntents(parent, resolved, &plan); err != nil {
		t.Fatalf("attachCoordinatorParentCloseIntents() error = %v", err)
	}
	if len(plan.ParentCloses) != 2 {
		t.Fatalf("parent close intents = %#v, want terminate and cancel only", plan.ParentCloses)
	}
	policies := map[string]string{}
	for _, spec := range plan.ParentCloses {
		if spec.ParentLoopRunID != string(parent.ID) || spec.ParentStatus != string(StatusFailed) {
			t.Fatalf("parent close identity = %#v", spec)
		}
		policies[spec.ChildLoopRunID] = spec.Policy
	}
	if policies["child-default"] != string(dsl.ParentCloseTerminate) ||
		policies["child-cancel"] != string(dsl.ParentCloseCancel) {
		t.Fatalf("parent close policies = %#v", policies)
	}
	if _, found := policies["child-abandon"]; found {
		t.Fatalf("abandoned child appeared in parent close intents: %#v", policies)
	}
}

func TestCoordinatorRunnerShouldRetryAwaitingChildLoopOnTimeout(t *testing.T) {
	t.Run("Should retry awaiting child loop on timeout", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC)
		loopRun := Run{
			ID:             "looprun-awaiting-child-timeout",
			WorkspaceID:    "ws-1",
			LoopName:       "delivery",
			Status:         StatusRunning,
			Generation:     1,
			LastProgressAt: now,
		}
		childRun := Run{
			ID:              "looprun-awaiting-child-timeout-child",
			WorkspaceID:     "ws-1",
			LoopName:        "child",
			Status:          StatusRunning,
			Generation:      1,
			ParentLoopRunID: loopRun.ID,
			CreatedAt:       now,
		}
		coordinatorRun := task.Run{
			ID:        "run-coordinator-awaiting-child-timeout",
			TaskID:    "task-coordinator-awaiting-child-timeout",
			RunKind:   task.RunKindCoordinator,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusClaimed,
		}
		runner := newCoordinatorRunnerForTestWithGraph(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{coordinatorRun.ID: coordinatorRun},
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {{
				Generation:     1,
				NodeID:         "load",
				Status:         generationOutputAwaitingChild,
				ChildLoopRunID: string(childRun.ID),
			}}}},
			dsl.Graph{Nodes: []dsl.Node{{
				ID:      "load",
				Class:   dsl.NodeClassAction,
				Kind:    string(dsl.ActionRunLoop),
				Timeout: "1s",
			}}},
		)
		setCoordinatorRunnerRunsForTest(t, runner, map[RunID]Run{
			loopRun.ID:  loopRun,
			childRun.ID: childRun,
		})
		runner.now = func() time.Time { return now.Add(2 * time.Second) }

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.Terminal != nil {
			t.Fatalf("Terminal = %#v, want nil for failed-only retry", plan.Terminal)
		}
		if got, want := len(plan.RunStops), 1; got != want {
			t.Fatalf("loop stops = %d, want %d", got, want)
		}
		if got, want := plan.RunStops[0].LoopRunID, string(childRun.ID); got != want {
			t.Fatalf("RunStops[0].LoopRunID = %q, want %q", got, want)
		}
		payload := coordinatorSnapshotPayloadForTest(t, plan)
		if got, want := payload.Outputs[0].Status, generationOutputFailed; got != want {
			t.Fatalf("output status = %q, want %q", got, want)
		}
		if got, want := payload.Outputs[0].OutputRef, "child_loop_timeout"; got != want {
			t.Fatalf("output ref = %q, want %q", got, want)
		}
		next := coordinatorPostReservePayloadForTest(t, plan)
		if got, want := next.Outputs[0].Status, generationOutputPending; got != want {
			t.Fatalf("next output status = %q, want %q", got, want)
		}
		if got := next.Outputs[0].ChildLoopRunID; got != "" {
			t.Fatalf("next child_loop_run_id = %q, want cleared rerun ownership", got)
		}
		if plan.NextCoordinator == nil {
			t.Fatal("NextCoordinator = nil, want retry coordinator")
		}
	})
}

func TestCoordinatorRunnerShouldTerminalizeDoneWhenGenerationSucceeded(t *testing.T) {
	t.Run("Should terminalize done when generation succeeded", func(t *testing.T) {
		t.Parallel()

		loopRun := Run{
			ID:          "looprun-done",
			WorkspaceID: "ws-1",
			LoopName:    "delivery",
			Status:      StatusRunning,
			Generation:  1,
		}
		coordinatorRun := task.Run{
			ID:        "run-coordinator-done",
			TaskID:    "task-coordinator-done",
			RunKind:   task.RunKindCoordinator,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusClaimed,
		}
		runner := newCoordinatorRunnerForTest(t, loopRun, coordinatorRun, map[string]task.Run{
			coordinatorRun.ID: coordinatorRun,
		}, coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {{
			Generation: 1,
			NodeID:     "load",
			Status:     generationOutputSucceeded,
		}, {
			Generation: 1,
			NodeID:     "agent",
			Status:     generationOutputSucceeded,
		}}}})

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.Terminal == nil {
			t.Fatal("Terminal = nil, want done")
		}
		if got, want := plan.Terminal.Status, string(StatusDone); got != want {
			t.Fatalf("terminal status = %q, want %q", got, want)
		}
	})
}

func TestCoordinatorRunnerShouldRespectContractStopWhen(t *testing.T) {
	t.Run("Should expose a same-transaction metric best update to stop-when", func(t *testing.T) {
		t.Parallel()

		score := 0.9
		conditionRun, conditionHistory, err := pendingBestConditionContext(
			Run{ID: "looprun-pending-best", WorkspaceID: "ws-1"},
			GenerationHistory{},
			[]GenerationOutput{{
				Generation: 1, NodeID: "quality", Status: generationOutputSucceeded,
				OutputRef: `{"outcome":"approved"}`,
			}},
			task.GenerationSnapshot{Generation: 1, Payload: GenerationSnapshotPayload{
				BestUpdate: &gate.BestUpdateIntent{Generation: 1, Score: score},
			}},
		)
		if err != nil {
			t.Fatalf("pendingBestConditionContext() error = %v", err)
		}
		if conditionRun.BestGeneration == nil || *conditionRun.BestGeneration != 1 ||
			conditionRun.BestScore == nil || *conditionRun.BestScore != score {
			t.Fatalf(
				"condition run best = %#v/%#v, want generation 1 score %.1f",
				conditionRun.BestGeneration,
				conditionRun.BestScore,
				score,
			)
		}
		if conditionHistory.Best == nil || conditionHistory.Best.Score != score ||
			conditionHistory.Best.Generation != 1 {
			t.Fatalf("condition history best = %#v, want generation 1 score %.1f", conditionHistory.Best, score)
		}
	})

	t.Run("Should expose a same-transaction metric best update to definition of done", func(t *testing.T) {
		t.Parallel()

		score := 0.9
		graph := dsl.Graph{
			Nodes: []dsl.Node{
				{ID: "draft", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent)},
				testMetricRouteGateNode("quality"),
			},
			Edges: []dsl.Edge{{From: "draft", To: "quality"}},
		}
		run := Run{
			ID: "looprun-dod-pending-best", WorkspaceID: "ws-1", LoopName: "delivery",
			Status: StatusRunning, Generation: 1, IterationCap: 2,
		}
		coordinatorRun := task.Run{
			ID: "run-coordinator-dod-pending-best", TaskID: "task-coordinator-dod-pending-best",
			RunKind: task.RunKindCoordinator, LoopRunID: string(run.ID), Status: task.TaskRunStatusClaimed,
		}
		runner := newCoordinatorRunnerForTestWithDefinition(
			t,
			run,
			coordinatorRun,
			nil,
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{
				1: {
					{Generation: 1, NodeID: "draft", Status: generationOutputSucceeded, OutputRef: `{"draft":"best"}`},
				},
				2: {
					{Generation: 2, NodeID: "draft", Status: generationOutputSucceeded, OutputRef: `{"draft":2}`},
					{Generation: 2, NodeID: "quality", Status: generationOutputPending},
				},
			}},
			dsl.Definition{
				Graph: graph,
				Contract: dsl.Contract{Verification: []dsl.GateCriterion{{
					ID: "check", Type: dsl.CriterionCommand, Check: "best={{ .best.score | shellQuote }}",
				}}},
			},
			WithCoordinatorGateEvaluator(gateEvaluatorFunc(
				func(_ context.Context, runtimeGate gate.Gate, input gate.GateInput) (gate.Verdict, error) {
					if input.Placement == gate.PlacementDefinitionOfDone {
						if input.BestScore == nil || *input.BestScore != score {
							t.Fatalf("definition-of-done best score = %#v, want %.1f", input.BestScore, score)
						}
						if got, want := runtimeGate.Criteria[0].Check, "best='0.9'"; got != want {
							t.Fatalf("definition-of-done rendered check = %q, want %q", got, want)
						}
						return gate.Verdict{
							Outcome: gate.VerdictOutcomeApproved,
							Criteria: []gate.CriterionResult{{
								ID: "check", Type: dsl.CriterionCommand,
								Outcome: gate.VerdictOutcomeApproved, Passed: true,
							}},
							Route: gate.RouteDecision{
								Placement: gate.PlacementDefinitionOfDone,
								Action:    gate.RouteDone,
							},
						}, nil
					}
					return gate.Verdict{
						Outcome: gate.VerdictOutcomeApproved,
						Criteria: []gate.CriterionResult{{
							ID: "check", Type: dsl.CriterionCommand, Score: &score,
							Outcome: gate.VerdictOutcomeApproved, Passed: true,
						}},
						Route: gate.RouteDecision{Placement: gate.PlacementInBody, Action: gate.RouteContinue},
					}, nil
				},
			)),
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.Terminal == nil || plan.Terminal.Status != string(StatusDone) {
			t.Fatalf("terminal = %#v, want done", plan.Terminal)
		}
	})

	t.Run("Should start next generation while stop_when is false", func(t *testing.T) {
		t.Parallel()

		loopRun := Run{
			ID:           "looprun-stop-when-dirty",
			WorkspaceID:  "ws-1",
			LoopName:     "review-and-fix",
			Status:       StatusRunning,
			Generation:   1,
			IterationCap: 0,
		}
		liveSpec := coordinatorLiveParticipationForTest(loopRun)
		loopRun.SetNetworkSpec(liveSpec)
		coordinatorRun := task.Run{
			ID:        "run-coordinator-stop-when-dirty",
			TaskID:    "task-coordinator-stop-when-dirty",
			RunKind:   task.RunKindCoordinator,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusClaimed,
		}
		runner := newCoordinatorRunnerForStopWhenTest(
			t,
			loopRun,
			coordinatorRun,
			`{"issues":[{"id":"R1"}]}`,
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.Terminal != nil {
			t.Fatalf("Terminal = %#v, want nil while stop_when is false", plan.Terminal)
		}
		if plan.NextCoordinator == nil {
			t.Fatal("NextCoordinator = nil, want next generation coordinator")
		}
		if got, want := plan.NextCoordinator.RunID, coordinatorRunID(loopRun.ID, 2); got != want {
			t.Fatalf("NextCoordinator.RunID = %q, want %q", got, want)
		}
		if got := plan.NextCoordinator.ResolvedNetworkParticipation; got == nil || *got != liveSpec {
			t.Fatalf("NextCoordinator participation = %#v, want %#v", got, liveSpec)
		}
		next := outputsByNodeForTest(coordinatorPostReservePayloadForTest(t, plan).Outputs)
		if got, want := next["inspect_issues"].Status, generationOutputPending; got != want {
			t.Fatalf("next inspect_issues status = %q, want %q", got, want)
		}
		if got, want := next["verify_gate"].Status, generationOutputPending; got != want {
			t.Fatalf("next verify_gate status = %q, want %q", got, want)
		}
		gateTaskID := coordinatorNodeTaskID(loopRun.ID, 2, "verify_gate", 0)
		for _, spec := range plan.NodeTasks {
			if spec.TaskID == gateTaskID {
				t.Fatalf("NodeTasks included coordinator-owned gate task %q", gateTaskID)
			}
		}
		if got, want := plan.PostReserveSnapshot.Generation, 2; got != want {
			t.Fatalf("post-reserve generation = %d, want %d", got, want)
		}
	})

	t.Run("Should terminalize done when stop_when is true", func(t *testing.T) {
		t.Parallel()

		loopRun := Run{
			ID:           "looprun-stop-when-clean",
			WorkspaceID:  "ws-1",
			LoopName:     "review-and-fix",
			Status:       StatusRunning,
			Generation:   1,
			IterationCap: 0,
		}
		coordinatorRun := task.Run{
			ID:        "run-coordinator-stop-when-clean",
			TaskID:    "task-coordinator-stop-when-clean",
			RunKind:   task.RunKindCoordinator,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusClaimed,
		}
		runner := newCoordinatorRunnerForStopWhenTest(t, loopRun, coordinatorRun, `{"issues":[]}`)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.Terminal == nil {
			t.Fatal("Terminal = nil, want done")
		}
		if got, want := plan.Terminal.Status, string(StatusDone); got != want {
			t.Fatalf("terminal status = %q, want %q", got, want)
		}
		if plan.NextCoordinator != nil {
			t.Fatalf("NextCoordinator = %#v, want nil when stop_when is true", plan.NextCoordinator)
		}
	})

	t.Run("Should exit with a durable diagnostic when stop_when evaluation fails", func(t *testing.T) {
		t.Parallel()

		loopRun := Run{
			ID: "looprun-stop-when-broken", WorkspaceID: "ws-1", LoopName: "review-and-fix",
			Status: StatusRunning, Generation: 1,
		}
		coordinatorRun := task.Run{
			ID: "run-coordinator-stop-when-broken", TaskID: "task-coordinator-stop-when-broken",
			RunKind: task.RunKindCoordinator, LoopRunID: string(loopRun.ID), Status: task.TaskRunStatusClaimed,
		}
		runner := newCoordinatorRunnerForStopWhenSpecTest(
			t,
			loopRun,
			coordinatorRun,
			`{"issues":[]}`,
			dsl.StopWhenSpec{Expr: `nodes.inspect_issues.output.issues[0] == "done"`},
		)
		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.Terminal == nil || plan.Terminal.Status != string(StatusDone) || plan.NextCoordinator != nil {
			t.Fatalf("broken stop_when plan = %#v, want done without succession", plan)
		}
		payload := coordinatorSnapshotPayloadForTest(t, plan)
		if len(payload.Events) != 1 || payload.Events[0].Kind != GenerationLifecycleEventPredicateDiagnostic ||
			payload.Events[0].DiagnosticCode != "predicate_evaluation_failed" {
			t.Fatalf("predicate events = %#v, want evaluation diagnostic", payload.Events)
		}
	})

	t.Run("Should honor a fail override on broken stop_when", func(t *testing.T) {
		t.Parallel()

		loopRun := Run{
			ID: "looprun-stop-when-fail", WorkspaceID: "ws-1", LoopName: "review-and-fix",
			Status: StatusRunning, Generation: 1,
		}
		coordinatorRun := task.Run{
			ID: "run-coordinator-stop-when-fail", TaskID: "task-coordinator-stop-when-fail",
			RunKind: task.RunKindCoordinator, LoopRunID: string(loopRun.ID), Status: task.TaskRunStatusClaimed,
		}
		runner := newCoordinatorRunnerForStopWhenSpecTest(
			t,
			loopRun,
			coordinatorRun,
			`{"issues":[]}`,
			dsl.StopWhenSpec{
				Expr:        `nodes.inspect_issues.output.issues[0] == "done"`,
				OnEvalError: dsl.EvalErrorFail,
			},
		)
		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.Terminal == nil || plan.Terminal.Status != string(StatusFailed) {
			t.Fatalf("broken overridden stop_when terminal = %#v, want failed", plan.Terminal)
		}
	})
}

func TestCoordinatorRunnerShouldSkipEmptyCommandGate(t *testing.T) {
	t.Run("Should terminalize done when the only command check renders empty", func(t *testing.T) {
		t.Parallel()

		loopRun := Run{
			ID:          "looprun-empty-command-gate",
			WorkspaceID: "ws-1",
			LoopName:    "software-delivery",
			Status:      StatusRunning,
			Generation:  1,
			Inputs:      map[string]any{"verify_command": ""},
		}
		coordinatorRun := task.Run{
			ID:        "run-coordinator-empty-command-gate",
			TaskID:    "task-coordinator-empty-command-gate",
			RunKind:   task.RunKindCoordinator,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusClaimed,
		}
		def := dsl.Definition{
			Graph: dsl.Graph{
				Nodes: []dsl.Node{
					{ID: "slug_input", Class: dsl.NodeClassSource, Kind: string(dsl.SourceInput)},
					{
						ID:    "verify_gate",
						Class: dsl.NodeClassControl,
						Kind:  string(dsl.ControlGate),
						Criteria: []dsl.GateCriterion{{
							ID:     "verify",
							Type:   dsl.CriterionCommand,
							Check:  "",
							Expect: "exit_zero",
						}},
						VerdictPolicy: dsl.VerdictPolicyFixedPasses,
					},
				},
				Edges: []dsl.Edge{{From: "slug_input", To: "verify_gate"}},
			},
		}
		runner := newCoordinatorRunnerForTestWithDefinition(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{coordinatorRun.ID: coordinatorRun},
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
				{
					Generation: 1,
					NodeID:     "slug_input",
					Status:     generationOutputSucceeded,
					OutputRef:  `{"slug":"task-001"}`,
				},
				{Generation: 1, NodeID: "verify_gate", Status: generationOutputPending},
			}}},
			def,
			WithCoordinatorGateEvaluator(gateEvaluatorFunc(
				func(context.Context, gate.Gate, gate.GateInput) (gate.Verdict, error) {
					t.Fatal("gate evaluator was called for an empty command criterion")
					return gate.Verdict{}, nil
				},
			)),
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.Terminal == nil {
			t.Fatal("Terminal = nil, want done after empty gate is skipped")
		}
		if got, want := plan.Terminal.Status, string(StatusDone); got != want {
			t.Fatalf("terminal status = %q, want %q", got, want)
		}
		outputs := outputsByNodeForTest(coordinatorSnapshotPayloadForTest(t, plan).Outputs)
		if got, want := outputs["verify_gate"].Status, generationOutputSucceeded; got != want {
			t.Fatalf("verify_gate status = %q, want %q", got, want)
		}
	})
}

func TestCoordinatorRunnerShouldClassifyExplicitDependencyFailureAsBlocked(t *testing.T) {
	t.Run("Should classify explicit dependency failure as blocked", func(t *testing.T) {
		t.Parallel()

		loopRun := Run{
			ID:          "looprun-blocked",
			WorkspaceID: "ws-1",
			LoopName:    "delivery",
			Status:      StatusRunning,
			Generation:  1,
		}
		coordinatorRun := task.Run{
			ID:        "run-coordinator-blocked",
			TaskID:    "task-coordinator-blocked",
			RunKind:   task.RunKindCoordinator,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusClaimed,
		}
		rootRun := task.Run{
			ID:        coordinatorNodeRunID(loopRun.ID, 1, "load", 0),
			TaskID:    coordinatorNodeTaskID(loopRun.ID, 1, "load", 0),
			RunKind:   task.RunKindWorker,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusFailed,
			Error:     `{"reason_code":"credential_missing"}`,
		}
		runner := newCoordinatorRunnerForTest(t, loopRun, coordinatorRun, map[string]task.Run{
			coordinatorRun.ID: coordinatorRun,
			rootRun.ID:        rootRun,
		}, coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {{
			Generation: 1,
			NodeID:     "load",
			Status:     generationOutputRunning,
			TaskRunID:  rootRun.ID,
		}}}})

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.Terminal == nil {
			t.Fatal("Terminal = nil, want blocked")
		}
		if got, want := plan.Terminal.Status, string(StatusBlocked); got != want {
			t.Fatalf("terminal status = %q, want %q", got, want)
		}
		if got, want := plan.Terminal.ReasonCode, "credential_missing"; got != want {
			t.Fatalf("reason_code = %q, want %q", got, want)
		}
	})
}

func TestCoordinatorRunnerShouldPreferExplicitBlockerWhenMultipleNodesFail(t *testing.T) {
	t.Run("Should prefer explicit blocker when multiple nodes fail", func(t *testing.T) {
		t.Parallel()

		loopRun := Run{
			ID:          "looprun-multi-failure",
			WorkspaceID: "ws-1",
			LoopName:    "delivery",
			Status:      StatusRunning,
			Generation:  1,
		}
		coordinatorRun := task.Run{
			ID:        "run-coordinator-multi-failure",
			TaskID:    "task-coordinator-multi-failure",
			RunKind:   task.RunKindCoordinator,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusClaimed,
		}
		plainRun := task.Run{
			ID:        coordinatorNodeRunID(loopRun.ID, 1, "alpha", 0),
			TaskID:    coordinatorNodeTaskID(loopRun.ID, 1, "alpha", 0),
			RunKind:   task.RunKindWorker,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusFailed,
		}
		blockedRun := task.Run{
			ID:        coordinatorNodeRunID(loopRun.ID, 1, "zulu", 0),
			TaskID:    coordinatorNodeTaskID(loopRun.ID, 1, "zulu", 0),
			RunKind:   task.RunKindWorker,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusFailed,
			Error:     `{"reason_code":"dependency_missing"}`,
		}
		graph := dsl.Graph{
			Nodes: []dsl.Node{
				{ID: "alpha", Class: dsl.NodeClassSource, Kind: string(dsl.SourceInput)},
				{ID: "zulu", Class: dsl.NodeClassSource, Kind: string(dsl.SourceInput)},
			},
		}
		runner := newCoordinatorRunnerForTestWithGraph(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{
				coordinatorRun.ID: coordinatorRun,
				plainRun.ID:       plainRun,
				blockedRun.ID:     blockedRun,
			},
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
				{
					Generation: 1,
					NodeID:     "alpha",
					Status:     generationOutputRunning,
					TaskRunID:  plainRun.ID,
				},
				{
					Generation: 1,
					NodeID:     "zulu",
					Status:     generationOutputRunning,
					TaskRunID:  blockedRun.ID,
				},
			}}},
			graph,
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.Terminal == nil {
			t.Fatal("Terminal = nil, want blocked")
		}
		if got, want := plan.Terminal.Status, string(StatusBlocked); got != want {
			t.Fatalf("terminal status = %q, want %q", got, want)
		}
		if got, want := plan.Terminal.ReasonCode, "dependency_missing"; got != want {
			t.Fatalf("reason_code = %q, want %q", got, want)
		}
	})
}

func TestCoordinatorRunnerShouldRetryUnstructuredNodeFailure(t *testing.T) {
	t.Run("Should schedule failed-only retry for unstructured node failure", func(t *testing.T) {
		t.Parallel()

		loopRun := Run{
			ID:          "looprun-node-failed",
			WorkspaceID: "ws-1",
			LoopName:    "delivery",
			Status:      StatusRunning,
			Generation:  1,
		}
		coordinatorRun := task.Run{
			ID:        "run-coordinator-node-failed",
			TaskID:    "task-coordinator-node-failed",
			RunKind:   task.RunKindCoordinator,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusClaimed,
		}
		rootRun := task.Run{
			ID:        coordinatorNodeRunID(loopRun.ID, 1, "load", 0),
			TaskID:    coordinatorNodeTaskID(loopRun.ID, 1, "load", 0),
			RunKind:   task.RunKindWorker,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusFailed,
			Error:     "worker mentioned dependency_missing in plain text but did not emit a reason code",
		}
		runner := newCoordinatorRunnerForTest(t, loopRun, coordinatorRun, map[string]task.Run{
			coordinatorRun.ID: coordinatorRun,
			rootRun.ID:        rootRun,
		}, coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {{
			Generation: 1,
			NodeID:     "load",
			Status:     generationOutputRunning,
			TaskRunID:  rootRun.ID,
		}}}})

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.Terminal != nil {
			t.Fatalf("Terminal = %#v, want nil for failed-only retry", plan.Terminal)
		}
		if plan.NextCoordinator == nil {
			t.Fatal("NextCoordinator = nil, want retry coordinator")
		}
		if got, want := plan.NextCoordinator.RunID, coordinatorRunID(loopRun.ID, 2); got != want {
			t.Fatalf("next coordinator run_id = %q, want %q", got, want)
		}
		nextOutputs := outputsByNodeForTest(coordinatorPostReservePayloadForTest(t, plan).Outputs)
		if got, want := nextOutputs["load"].Status, generationOutputPending; got != want {
			t.Fatalf("next load status = %q, want %q", got, want)
		}
	})
}

func TestCoordinatorRunnerShouldUnionRouteCausingGateProducers(t *testing.T) {
	t.Run("Should rerun the deterministic producer union and carry unrelated work", func(t *testing.T) {
		t.Parallel()

		graph := dsl.Graph{
			Nodes: []dsl.Node{
				{ID: "a", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunLoop)},
				{ID: "b", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent)},
				{ID: "e", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent)},
				{ID: "d", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunLoop)},
				testRouteGateNode("g1"),
				testRouteGateNode("g2"),
				{ID: "c", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunLoop)},
			},
			Edges: []dsl.Edge{
				{From: "a", To: "g1"},
				{From: "b", To: "g1"},
				{From: "b", To: "g2"},
				{From: "e", To: "g2"},
				{From: "g1", To: "c"},
				{From: "g2", To: "c"},
			},
		}
		run := Run{
			ID:                "looprun-route-union",
			WorkspaceID:       "ws-1",
			LoopName:          "delivery",
			Status:            StatusRunning,
			Generation:        1,
			IterationCap:      3,
			ReattemptStrategy: ReattemptFailedOnly,
		}
		coordinatorRun := task.Run{
			ID:        "run-coordinator-route-union",
			TaskID:    "task-coordinator-route-union",
			RunKind:   task.RunKindCoordinator,
			LoopRunID: string(run.ID),
			Status:    task.TaskRunStatusClaimed,
		}
		current := []GenerationOutput{
			{
				Generation:     1,
				NodeID:         "a",
				Status:         generationOutputSucceeded,
				OutputRef:      `{"value":"a"}`,
				ChildLoopRunID: "child-a",
			},
			{
				Generation: 1,
				NodeID:     "b",
				Status:     generationOutputSucceeded,
				OutputRef:  `{"value":"b"}`,
			},
			{Generation: 1, NodeID: "c", Status: generationOutputPending, ChildLoopRunID: "child-c"},
			{
				Generation:     1,
				NodeID:         "d",
				Status:         generationOutputSucceeded,
				OutputRef:      `{"value":"carry"}`,
				TaskRunID:      "run-d",
				ChildLoopRunID: "child-d",
			},
			{
				Generation: 1,
				NodeID:     "e",
				Status:     generationOutputSucceeded,
				OutputRef:  `{"value":"e"}`,
			},
			{Generation: 1, NodeID: "g1", Status: generationOutputPending},
			{Generation: 1, NodeID: "g2", Status: generationOutputPending},
		}
		runner := newCoordinatorRunnerForTestWithDefinition(
			t,
			run,
			coordinatorRun,
			nil,
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: current}},
			dsl.Definition{Graph: graph},
			WithCoordinatorGateEvaluator(testRouteEvaluator(gate.RouteRevise)),
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.Terminal != nil || plan.NextCoordinator == nil {
			t.Fatalf("terminal/next = %#v/%#v, want successor", plan.Terminal, plan.NextCoordinator)
		}
		currentPayload := coordinatorSnapshotPayloadForTest(t, plan)
		if got, want := len(currentPayload.Verdicts), 2; got != want {
			t.Fatalf("verdict intents = %d, want %d", got, want)
		}
		for rank, verdict := range currentPayload.Verdicts {
			if verdict.RouteCauseRank == nil || *verdict.RouteCauseRank != rank {
				t.Fatalf("verdict %q rank = %#v, want %d", verdict.GateID, verdict.RouteCauseRank, rank)
			}
		}
		nextPayload := coordinatorPostReservePayloadForTest(t, plan)
		next := outputsByNodeForTest(nextPayload.Outputs)
		for _, nodeID := range []string{"a", "b", "e", "g1", "g2", "c"} {
			got := next[nodeID]
			if got.Status != generationOutputPending || got.OutputRef != "" || got.ChildLoopRunID != "" {
				t.Fatalf("rerun output %q = %#v, want blank pending", nodeID, got)
			}
		}
		if got := next["d"]; got.Status != generationOutputSucceeded ||
			got.OutputRef != `{"value":"carry"}` || got.ChildLoopRunID != "child-d" {
			t.Fatalf("carried output d = %#v", got)
		}
		if nextPayload.GenerationProvenance == nil ||
			nextPayload.GenerationProvenance.Origin != OriginGateRevise {
			t.Fatalf("generation provenance = %#v, want gate_revise", nextPayload.GenerationProvenance)
		}
	})

	t.Run("Should retain route-causing fan-out gate instances", func(t *testing.T) {
		t.Parallel()

		runtimeGate := gate.Gate{ID: "quality", Criteria: []dsl.GateCriterion{testRouteCriterion()}}
		collector := &gateEvaluationCollector{}
		collector.record(runtimeGate, 0, testRouteVerdict(gate.RouteRevise))
		collector.record(runtimeGate, 1, gate.Verdict{
			Outcome: gate.VerdictOutcomeApproved,
			Route:   gate.RouteDecision{Placement: gate.PlacementInBody, Action: gate.RouteContinue},
		})
		if got, want := len(collector.ordered()), 2; got != want {
			t.Fatalf("gate evaluations = %d, want %d", got, want)
		}
		causes := collector.routeCauses()
		if len(causes) != 1 || causes[0].itemIndex != 0 {
			t.Fatalf("route causes = %#v, want only item 0", causes)
		}
		intents, _, err := collector.intents(Run{}, 1)
		if err != nil {
			t.Fatalf("intents() error = %v", err)
		}
		if len(intents) != 2 || intents[0].ItemIndex != 0 || intents[1].ItemIndex != 1 {
			t.Fatalf("verdict intents = %#v, want both gate items", intents)
		}
	})

	t.Run("Should rematerialize fan-out body instead of carrying stale item slots", func(t *testing.T) {
		t.Parallel()

		graph := dsl.Graph{
			Nodes: []dsl.Node{
				{ID: "fan", Class: dsl.NodeClassControl, Kind: string(dsl.ControlFanOut)},
				{ID: "work", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent)},
				testRouteGateNode("quality"),
				{ID: "collect", Class: dsl.NodeClassControl, Kind: string(dsl.ControlCollect)},
			},
			Edges: []dsl.Edge{
				{From: "fan", To: "work"},
				{From: "work", To: "quality"},
				{From: "quality", To: "collect"},
			},
		}
		current := []GenerationOutput{
			{Generation: 1, NodeID: "fan", Status: generationOutputSucceeded},
			{Generation: 1, NodeID: "collect", Status: generationOutputPending},
		}
		rerun := map[generationOutputKey]struct{}{
			{nodeID: "fan", itemIndex: 0}:     {},
			{nodeID: "collect", itemIndex: 0}: {},
		}
		for itemIndex := range 3 {
			current = append(
				current,
				GenerationOutput{
					Generation: 1, NodeID: "work", ItemIndex: itemIndex,
					Status: generationOutputSucceeded,
				},
				GenerationOutput{
					Generation: 1, NodeID: "quality", ItemIndex: itemIndex,
					Status: generationOutputFailed,
				},
			)
			rerun[generationOutputKey{nodeID: "work", itemIndex: itemIndex}] = struct{}{}
			rerun[generationOutputKey{nodeID: "quality", itemIndex: itemIndex}] = struct{}{}
		}
		next := successionGenerationOutputs(graph, current, current, rerun, 2)
		for _, output := range next {
			if output.NodeID == "work" || output.NodeID == "quality" {
				t.Fatalf("successor retained stale fan-out body output %#v", output)
			}
		}
	})
}

func TestCoordinatorRunnerShouldStartFreshGenerationForBothNextGenerationSurfaces(t *testing.T) {
	cases := []struct {
		name       string
		fixtureID  string
		definition dsl.Definition
		outputs    []GenerationOutput
		wantOrigin GenerationOrigin
	}{
		{
			name:      "in-body gate",
			fixtureID: "in-body-gate",
			definition: dsl.Definition{Graph: dsl.Graph{
				Nodes: []dsl.Node{
					{ID: "draft", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent)},
					testRouteGateNode("quality"),
				},
				Edges: []dsl.Edge{{From: "draft", To: "quality"}},
			}},
			outputs: []GenerationOutput{
				{Generation: 1, NodeID: "draft", Status: generationOutputSucceeded, OutputRef: `{"draft":1}`},
				{Generation: 1, NodeID: "quality", Status: generationOutputPending},
			},
			wantOrigin: OriginGateNextGeneration,
		},
		{
			name:      "definition of done",
			fixtureID: "definition-of-done",
			definition: dsl.Definition{
				Contract: dsl.Contract{Verification: []dsl.GateCriterion{testRouteCriterion()}},
				Graph: dsl.Graph{Nodes: []dsl.Node{
					{ID: "draft", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent)},
				}},
			},
			outputs: []GenerationOutput{
				{Generation: 1, NodeID: "draft", Status: generationOutputSucceeded, OutputRef: `{"draft":1}`},
			},
			wantOrigin: OriginDoDRetry,
		},
	}
	for _, tc := range cases {
		t.Run("Should schedule a fresh generation for "+tc.name, func(t *testing.T) {
			t.Parallel()

			run := Run{
				ID:           RunID("looprun-next-" + tc.fixtureID),
				WorkspaceID:  "ws-1",
				LoopName:     "delivery",
				Status:       StatusRunning,
				Generation:   1,
				IterationCap: 2,
			}
			coordinatorRun := task.Run{
				ID:        "run-coordinator-next-" + tc.fixtureID,
				TaskID:    "task-coordinator-next-" + tc.fixtureID,
				RunKind:   task.RunKindCoordinator,
				LoopRunID: string(run.ID),
				Status:    task.TaskRunStatusClaimed,
			}
			runner := newCoordinatorRunnerForTestWithDefinition(
				t,
				run,
				coordinatorRun,
				nil,
				coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: tc.outputs}},
				tc.definition,
				WithCoordinatorGateEvaluator(testRouteEvaluator(gate.RouteNextGeneration)),
			)

			plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if plan.Terminal != nil || plan.NextCoordinator == nil {
				t.Fatalf("terminal/next = %#v/%#v, want fresh successor", plan.Terminal, plan.NextCoordinator)
			}
			next := coordinatorPostReservePayloadForTest(t, plan)
			for _, output := range next.Outputs {
				if output.Status != generationOutputPending || output.OutputRef != "" {
					t.Fatalf("fresh output = %#v, want blank pending", output)
				}
			}
			if next.GenerationProvenance == nil || next.GenerationProvenance.Origin != tc.wantOrigin {
				t.Fatalf("provenance = %#v, want %q", next.GenerationProvenance, tc.wantOrigin)
			}
			if got := coordinatorSnapshotPayloadForTest(t, plan).Verdicts; len(got) != 1 {
				t.Fatalf("verdict intents = %#v, want one", got)
			}

			cappedRun := run
			cappedRun.ID = RunID(string(run.ID) + "-capped")
			cappedRun.IterationCap = 1
			cappedCoordinator := coordinatorRun
			cappedCoordinator.ID += "-capped"
			cappedCoordinator.TaskID += "-capped"
			cappedCoordinator.LoopRunID = string(cappedRun.ID)
			cappedRunner := newCoordinatorRunnerForTestWithDefinition(
				t,
				cappedRun,
				cappedCoordinator,
				nil,
				coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: tc.outputs}},
				tc.definition,
				WithCoordinatorGateEvaluator(testRouteEvaluator(gate.RouteNextGeneration)),
			)
			cappedPlan, err := cappedRunner.Run(context.Background(), task.RunID(cappedCoordinator.ID))
			if err != nil {
				t.Fatalf("Run(capped) error = %v", err)
			}
			if cappedPlan.Terminal == nil || cappedPlan.Terminal.Status != string(StatusExhausted) ||
				cappedPlan.NextCoordinator != nil || cappedPlan.PostReserveSnapshot != nil {
				t.Fatalf("capped plan = %#v, want exhausted without successor", cappedPlan)
			}
		})
	}
}

func TestCoordinatorRunnerShouldSeedMetricRevisionFromBest(t *testing.T) {
	bestGeneration := int64(1)
	currentBestGeneration := int64(2)
	bestScore := 0.9
	cases := []struct {
		name           string
		bestGeneration *int64
		bestScore      *float64
		wantRef        string
		wantOrigin     GenerationOrigin
		wantParent     int64
	}{
		{
			name: "with baseline", bestGeneration: &bestGeneration, bestScore: &bestScore,
			wantRef: `{"value":"best"}`, wantOrigin: OriginRatchetRestore, wantParent: 1,
		},
		{
			name: "without baseline", wantRef: `{"value":"latest"}`,
			wantOrigin: OriginGateRevise, wantParent: 2,
		},
		{
			name: "with inconsistent current baseline", bestGeneration: &currentBestGeneration, bestScore: &bestScore,
			wantRef: `{"value":"latest"}`, wantOrigin: OriginGateRevise, wantParent: 2,
		},
	}
	for _, tc := range cases {
		t.Run("Should seed a metric rejection "+tc.name, func(t *testing.T) {
			t.Parallel()

			graph := dsl.Graph{
				Nodes: []dsl.Node{
					{ID: "producer", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent)},
					{ID: "stable", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent)},
					testMetricRouteGateNode("quality"),
				},
				Edges: []dsl.Edge{{From: "producer", To: "quality"}},
			}
			run := Run{
				ID: RunID("looprun-ratchet-" + tc.name), WorkspaceID: "ws-1", LoopName: "delivery",
				Status: StatusRunning, Generation: 2, IterationCap: 3,
				BestGeneration: tc.bestGeneration, BestScore: tc.bestScore,
			}
			coordinatorRun := task.Run{
				ID: "run-coordinator-ratchet-" + tc.name, TaskID: "task-coordinator-ratchet-" + tc.name,
				RunKind: task.RunKindCoordinator, LoopRunID: string(run.ID), Status: task.TaskRunStatusClaimed,
			}
			outputs := coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{
				1: {
					{
						Generation: 1,
						NodeID:     "producer",
						Status:     generationOutputSucceeded,
						OutputRef:  `{"value":"best-producer"}`,
					},
					{
						Generation: 1,
						NodeID:     "stable",
						Status:     generationOutputSucceeded,
						OutputRef:  `{"value":"best"}`,
					},
					{
						Generation: 1,
						NodeID:     "quality",
						Status:     generationOutputSucceeded,
						OutputRef:  `{"outcome":"approved"}`,
					},
				},
				2: {
					{
						Generation: 2,
						NodeID:     "producer",
						Status:     generationOutputSucceeded,
						OutputRef:  `{"value":"regressed"}`,
					},
					{
						Generation: 2,
						NodeID:     "stable",
						Status:     generationOutputSucceeded,
						OutputRef:  `{"value":"latest"}`,
					},
					{Generation: 2, NodeID: "quality", Status: generationOutputPending},
				},
			}}
			runner := newCoordinatorRunnerForTestWithDefinition(
				t,
				run,
				coordinatorRun,
				nil,
				outputs,
				dsl.Definition{Graph: graph},
				WithCoordinatorGateEvaluator(gateEvaluatorFunc(
					func(context.Context, gate.Gate, gate.GateInput) (gate.Verdict, error) {
						verdict := testRouteVerdict(gate.RouteRevise)
						score := 0.7
						verdict.Criteria[0].Score = &score
						return verdict, nil
					},
				)),
			)

			plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			next := coordinatorPostReservePayloadForTest(t, plan)
			if got := outputsByNodeForTest(next.Outputs)["stable"].OutputRef; got != tc.wantRef {
				t.Fatalf("stable output_ref = %q, want %q", got, tc.wantRef)
			}
			if next.GenerationProvenance == nil || next.GenerationProvenance.Origin != tc.wantOrigin ||
				next.GenerationProvenance.ParentGeneration != tc.wantParent {
				t.Fatalf("provenance = %#v, want %q parent %d", next.GenerationProvenance, tc.wantOrigin, tc.wantParent)
			}
		})
	}
}

func TestCoordinatorRunnerShouldValidatePersistedRouteCauseOutputs(t *testing.T) {
	t.Parallel()

	runner := &CoordinatorRunner{verdicts: generationHistoryReaderStub{routeCauses: map[int][]gate.VerdictRecord{
		1: {{GateID: "quality"}},
	}}}
	graph := dsl.Graph{Nodes: []dsl.Node{testRouteGateNode("quality")}}
	t.Run("Should load a rejected gate persisted as failed", func(t *testing.T) {
		t.Parallel()

		ref, err := gateVerdictOutputRef(testRouteVerdict(gate.RouteRevise))
		if err != nil {
			t.Fatalf("gateVerdictOutputRef() error = %v", err)
		}
		collector, err := runner.loadPersistedRouteCauses(
			t.Context(),
			Run{ID: "run-1", WorkspaceID: "ws-1"},
			1,
			graph,
			[]GenerationOutput{{NodeID: "quality", Status: generationOutputFailed, OutputRef: ref}},
		)
		if err != nil {
			t.Fatalf("loadPersistedRouteCauses() error = %v", err)
		}
		if causes := collector.routeCauses(); len(causes) != 1 || causes[0].verdict.Route.Action != gate.RouteRevise {
			t.Fatalf("route causes = %#v, want one revise verdict", causes)
		}
	})
	t.Run("Should hydrate a content-addressed gate verdict before decoding it", func(t *testing.T) {
		t.Parallel()

		ref, err := gateVerdictOutputRef(testRouteVerdict(gate.RouteRevise))
		if err != nil {
			t.Fatalf("gateVerdictOutputRef() error = %v", err)
		}
		payload := json.RawMessage(ref)
		outputRef := OutputRefForPayload(payload)
		key := GenerationOutputPayloadKey{
			WorkspaceID: "ws-1",
			RunID:       "run-1",
			Generation:  1,
			NodeID:      "quality",
			OutputRef:   outputRef,
		}
		contentAddressedRunner := &CoordinatorRunner{
			verdicts: runner.verdicts,
			outputs: coordinatorRunnerOutputs{payloads: map[GenerationOutputPayloadKey]json.RawMessage{
				key: payload,
			}},
		}
		collector, err := contentAddressedRunner.loadPersistedRouteCauses(
			t.Context(),
			Run{ID: "run-1", WorkspaceID: "ws-1"},
			1,
			graph,
			[]GenerationOutput{{
				Generation: 1,
				NodeID:     "quality",
				Status:     generationOutputFailed,
				OutputRef:  outputRef,
			}},
		)
		if err != nil {
			t.Fatalf("loadPersistedRouteCauses() error = %v", err)
		}
		if causes := collector.routeCauses(); len(causes) != 1 || causes[0].verdict.Route.Action != gate.RouteRevise {
			t.Fatalf("route causes = %#v, want one hydrated revise verdict", causes)
		}
	})
	tests := []struct {
		name       string
		output     GenerationOutput
		wantDetail string
	}{
		{
			name: "Should reject a pending persisted route cause",
			output: GenerationOutput{
				NodeID: "quality", Status: generationOutputPending, OutputRef: `{"outcome":"rejected"}`,
			},
			wantDetail: `gate "quality" output status is "pending"`,
		},
		{
			name: "Should reject a successful persisted route cause without output",
			output: GenerationOutput{
				NodeID: "quality", Status: generationOutputSucceeded,
			},
			wantDetail: `gate "quality" finished without an output reference`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := runner.loadPersistedRouteCauses(
				t.Context(),
				Run{ID: "run-1", WorkspaceID: "ws-1"},
				1,
				graph,
				[]GenerationOutput{tt.output},
			)
			if !errors.Is(err, ErrValidation) || !strings.Contains(err.Error(), tt.wantDetail) {
				t.Fatalf("loadPersistedRouteCauses() error = %v, want validation detail %q", err, tt.wantDetail)
			}
		})
	}
}

func TestCoordinatorRunnerVerdictHistory(t *testing.T) {
	t.Parallel()

	t.Run("Should signal an unavailable verdict reader", func(t *testing.T) {
		t.Parallel()

		runner := &CoordinatorRunner{outputs: coordinatorRunnerOutputs{}}
		_, err := runner.readGenerationHistory(
			t.Context(),
			Run{ID: "run-1", WorkspaceID: "ws-1"},
			2,
		)
		if !errors.Is(err, ErrValidation) || !strings.Contains(err.Error(), "verdict history is unavailable") {
			t.Fatalf("readGenerationHistory() error = %v, want explicit unavailable-history validation", err)
		}
	})
}

func TestCoordinatorRunnerShouldExposePreviousHistoryToRerunGate(t *testing.T) {
	t.Run("Should evaluate a rejected gate with the prior durable generation namespace", func(t *testing.T) {
		t.Parallel()

		graph := dsl.Graph{
			Nodes: []dsl.Node{
				{ID: "draft", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent)},
				testRouteGateNode("quality"),
			},
			Edges: []dsl.Edge{{From: "draft", To: "quality"}},
		}
		graph.Nodes[1].Criteria[0].Check = "previous={{ .previous.nodes.draft.output.text | shellQuote }}"
		run := Run{
			ID: "looprun-previous-history", WorkspaceID: "ws-1", LoopName: "delivery",
			Status: StatusRunning, Generation: 2, IterationCap: 3,
		}
		coordinatorRun := task.Run{
			ID: "run-coordinator-previous-history", TaskID: "task-coordinator-previous-history",
			RunKind: task.RunKindCoordinator, LoopRunID: string(run.ID), Status: task.TaskRunStatusClaimed,
		}
		outputs := coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{
			1: {{Generation: 1, NodeID: "draft", Status: generationOutputSucceeded, OutputRef: `{"text":"previous"}`}},
			2: {
				{Generation: 2, NodeID: "draft", Status: generationOutputSucceeded, OutputRef: `{"text":"current"}`},
				{Generation: 2, NodeID: "quality", Status: generationOutputPending},
			},
		}}
		runner := newCoordinatorRunnerForTestWithDefinition(
			t,
			run,
			coordinatorRun,
			nil,
			outputs,
			dsl.Definition{Graph: graph},
			WithCoordinatorGateEvaluator(gateEvaluatorFunc(
				func(_ context.Context, runtimeGate gate.Gate, _ gate.GateInput) (gate.Verdict, error) {
					if got, want := runtimeGate.Criteria[0].Check, "previous='previous'"; got != want {
						t.Fatalf("previous draft check = %q, want %q", got, want)
					}
					return testRouteVerdict(gate.RouteRevise), nil
				},
			)),
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.NextCoordinator == nil {
			t.Fatal("NextCoordinator = nil, want repaired successor")
		}
	})
}

func TestCoordinatorRunnerShouldPersistEveryGateVerdictInSucceededGeneration(t *testing.T) {
	t.Run("Should retain in-body and definition-of-done verdict intents together", func(t *testing.T) {
		t.Parallel()

		graph := dsl.Graph{
			Nodes: []dsl.Node{
				{ID: "draft", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent)},
				testRouteGateNode("quality"),
			},
			Edges: []dsl.Edge{{From: "draft", To: "quality"}},
		}
		run := Run{
			ID: "looprun-all-verdicts", WorkspaceID: "ws-1", LoopName: "delivery",
			Status: StatusRunning, Generation: 1, IterationCap: 2,
		}
		coordinatorRun := task.Run{
			ID: "run-coordinator-all-verdicts", TaskID: "task-coordinator-all-verdicts",
			RunKind: task.RunKindCoordinator, LoopRunID: string(run.ID), Status: task.TaskRunStatusClaimed,
		}
		runner := newCoordinatorRunnerForTestWithDefinition(
			t,
			run,
			coordinatorRun,
			nil,
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
				{Generation: 1, NodeID: "draft", Status: generationOutputSucceeded, OutputRef: `{"draft":1}`},
				{Generation: 1, NodeID: "quality", Status: generationOutputPending},
			}}},
			dsl.Definition{
				Graph:    graph,
				Contract: dsl.Contract{Verification: []dsl.GateCriterion{testRouteCriterion()}},
			},
			WithCoordinatorGateEvaluator(gateEvaluatorFunc(
				func(_ context.Context, _ gate.Gate, input gate.GateInput) (gate.Verdict, error) {
					if input.Placement == gate.PlacementDefinitionOfDone {
						verdict := testRouteVerdict(gate.RouteNextGeneration)
						verdict.Route.Placement = gate.PlacementDefinitionOfDone
						return verdict, nil
					}
					return gate.Verdict{
						Outcome: gate.VerdictOutcomeApproved,
						Criteria: []gate.CriterionResult{{
							ID: "check", Type: dsl.CriterionCommand,
							Outcome: gate.VerdictOutcomeApproved, Passed: true,
						}},
						Route: gate.RouteDecision{
							Placement: gate.PlacementInBody, Action: gate.RouteContinue,
						},
					}, nil
				},
			)),
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		verdicts := coordinatorSnapshotPayloadForTest(t, plan).Verdicts
		if len(verdicts) != 2 || verdicts[0].GateID != "quality" ||
			verdicts[1].GateID != definitionOfDoneGateID {
			t.Fatalf("verdict intents = %#v, want quality and definition of done", verdicts)
		}
	})
}

func testRouteGateNode(id dsl.NodeID) dsl.Node {
	return dsl.Node{
		ID: id, Class: dsl.NodeClassControl, Kind: string(dsl.ControlGate),
		Criteria:      []dsl.GateCriterion{testRouteCriterion()},
		VerdictPolicy: dsl.VerdictPolicyFixedPasses,
	}
}

func testMetricRouteGateNode(id dsl.NodeID) dsl.Node {
	node := testRouteGateNode(id)
	node.Criteria[0].Metric = &dsl.MetricSpec{Direction: dsl.MetricMaximize}
	return node
}

func testRouteCriterion() dsl.GateCriterion {
	return dsl.GateCriterion{ID: "check", Type: dsl.CriterionCommand, Check: "true", Expect: "exit_zero"}
}

func testRouteEvaluator(action gate.RouteAction) gate.GateEvaluator {
	return gateEvaluatorFunc(func(context.Context, gate.Gate, gate.GateInput) (gate.Verdict, error) {
		return testRouteVerdict(action), nil
	})
}

func testRouteVerdict(action gate.RouteAction) gate.Verdict {
	return gate.Verdict{
		Outcome: gate.VerdictOutcomeRejected,
		Criteria: []gate.CriterionResult{{
			ID: "check", Type: dsl.CriterionCommand, Outcome: gate.VerdictOutcomeRejected,
		}},
		BlockingIssues: []gate.BlockingIssue{{ID: "needs_revision", Note: "repair the output"}},
		Route:          gate.RouteDecision{Placement: gate.PlacementInBody, Action: action},
	}
}

func TestCoordinatorRunnerShouldPlanReattemptStrategy(t *testing.T) {
	cases := []struct {
		name             string
		strategy         ReattemptStrategy
		graph            dsl.Graph
		outputs          []GenerationOutput
		wantStatuses     map[string]string
		wantCarriedRefs  map[string]string
		wantClearedRefs  []string
		wantNextRunID    string
		wantNodeTaskSize int
	}{
		{
			name:     "failed-only carries succeeded outputs and reruns failed pending dependents",
			strategy: ReattemptFailedOnly,
			graph: dsl.Graph{
				Nodes: []dsl.Node{
					{ID: "setup", Class: dsl.NodeClassSource, Kind: string(dsl.SourceInput)},
					{ID: "test", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent)},
					{ID: "deploy", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent)},
					{ID: "notify", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent)},
					{ID: "archive", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent)},
					{ID: "doc", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent)},
				},
				Edges: []dsl.Edge{
					{From: "setup", To: "test"},
					{From: "test", To: "deploy"},
					{From: "deploy", To: "notify"},
					{From: "setup", To: "archive"},
				},
			},
			outputs: []GenerationOutput{
				{
					Generation: 2,
					NodeID:     "setup",
					Status:     generationOutputSucceeded,
					OutputRef:  "sha256:setup",
					TaskRunID:  "run-setup-g2",
				},
				{
					Generation: 2,
					NodeID:     "test",
					Status:     generationOutputFailed,
					OutputRef:  "tool_failed",
					TaskRunID:  "run-test-g2",
				},
				{
					Generation: 2,
					NodeID:     "deploy",
					Status:     generationOutputSucceeded,
					OutputRef:  "sha256:deploy-old",
					TaskRunID:  "run-deploy-g2",
				},
				{
					Generation: 2,
					NodeID:     "notify",
					Status:     generationOutputSucceeded,
					OutputRef:  "sha256:notify-old",
					TaskRunID:  "run-notify-g2",
				},
				{
					Generation: 2,
					NodeID:     "archive",
					Status:     generationOutputSucceeded,
					OutputRef:  "sha256:archive",
					TaskRunID:  "run-archive-g2",
				},
				{
					Generation: 2,
					NodeID:     "doc",
					Status:     generationOutputPending,
				},
			},
			wantStatuses: map[string]string{
				"setup":   generationOutputSucceeded,
				"test":    generationOutputPending,
				"deploy":  generationOutputPending,
				"notify":  generationOutputPending,
				"archive": generationOutputSucceeded,
				"doc":     generationOutputPending,
			},
			wantCarriedRefs: map[string]string{
				"setup":   "sha256:setup",
				"archive": "sha256:archive",
			},
			wantClearedRefs:  []string{"test", "deploy", "notify", "doc"},
			wantNodeTaskSize: 5,
		},
		{
			name:     "full-body reruns every node",
			strategy: ReattemptFullBody,
			graph:    coordinatorTestGraph(),
			outputs: []GenerationOutput{
				{
					Generation: 2,
					NodeID:     "load",
					Status:     generationOutputSucceeded,
					OutputRef:  "sha256:load",
					TaskRunID:  "run-load-g2",
				},
				{
					Generation: 2,
					NodeID:     "agent",
					Status:     generationOutputFailed,
					OutputRef:  "tool_failed",
					TaskRunID:  "run-agent-g2",
				},
			},
			wantStatuses: map[string]string{
				"load":  generationOutputPending,
				"agent": generationOutputPending,
			},
			wantClearedRefs:  []string{"load", "agent"},
			wantNodeTaskSize: 1,
		},
	}

	for _, tc := range cases {
		t.Run("Should plan "+tc.name, func(t *testing.T) {
			t.Parallel()

			loopRun := Run{
				ID:                RunID("looprun-reattempt-" + tc.name),
				WorkspaceID:       "ws-1",
				LoopName:          "delivery",
				Status:            StatusRunning,
				Generation:        2,
				ReattemptStrategy: tc.strategy,
			}
			coordinatorRun := task.Run{
				ID:        "run-coordinator-reattempt-" + tc.name,
				TaskID:    "task-coordinator-reattempt-" + tc.name,
				RunKind:   task.RunKindCoordinator,
				LoopRunID: string(loopRun.ID),
				Status:    task.TaskRunStatusClaimed,
			}
			runner := newCoordinatorRunnerForTestWithGraph(
				t,
				loopRun,
				coordinatorRun,
				map[string]task.Run{coordinatorRun.ID: coordinatorRun},
				coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{2: tc.outputs}},
				tc.graph,
			)

			plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if plan.Terminal != nil {
				t.Fatalf("Terminal = %#v, want nil for retry", plan.Terminal)
			}
			if got, want := len(plan.NodeRuns), 0; got != want {
				t.Fatalf("node runs = %d, want %d until next coordinator", got, want)
			}
			if got, want := len(plan.NodeTasks), tc.wantNodeTaskSize; got != want {
				t.Fatalf("node tasks = %d, want %d", got, want)
			}
			if plan.NextCoordinator == nil {
				t.Fatal("NextCoordinator = nil, want retry coordinator")
			}
			if got, want := plan.NextCoordinator.TaskID, coordinatorRun.TaskID; got != want {
				t.Fatalf("NextCoordinator.TaskID = %q, want %q", got, want)
			}
			if got, want := plan.NextCoordinator.RunID, coordinatorRunID(loopRun.ID, 3); got != want {
				t.Fatalf("NextCoordinator.RunID = %q, want %q", got, want)
			}
			current := coordinatorSnapshotPayloadForTest(t, plan)
			if got, want := current.Outputs[0].Generation, 2; got != want {
				t.Fatalf("current snapshot generation = %d, want %d", got, want)
			}
			if plan.PostReserveSnapshot == nil {
				t.Fatal("PostReserveSnapshot = nil, want next-generation carry-forward")
			}
			if got, want := plan.PostReserveSnapshot.Generation, 3; got != want {
				t.Fatalf("post-reserve generation = %d, want %d", got, want)
			}
			nextOutputs := outputsByNodeForTest(coordinatorPostReservePayloadForTest(t, plan).Outputs)
			for nodeID, wantStatus := range tc.wantStatuses {
				if got := nextOutputs[nodeID].Status; got != wantStatus {
					t.Fatalf("%s status = %q, want %q", nodeID, got, wantStatus)
				}
			}
			for nodeID, wantRef := range tc.wantCarriedRefs {
				got := nextOutputs[nodeID]
				if got.OutputRef != wantRef {
					t.Fatalf("%s output_ref = %q, want %q", nodeID, got.OutputRef, wantRef)
				}
				if got.TaskRunID == "" {
					t.Fatalf("%s task_run_id was cleared, want read-only provenance", nodeID)
				}
			}
			for _, nodeID := range tc.wantClearedRefs {
				got := nextOutputs[nodeID]
				if got.OutputRef != "" || got.TaskRunID != "" {
					t.Fatalf("%s output carried output_ref/task_run_id: %#v", nodeID, got)
				}
			}
		})
	}
}

func TestCoordinatorRunnerShouldClearSubLoopChildOnFailedOnlyRetry(t *testing.T) {
	t.Run("Should clear child ownership for a rerun root and its dependents", func(t *testing.T) {
		t.Parallel()

		loopRun := Run{
			ID:                "looprun-sub-loop-retry",
			WorkspaceID:       "ws-1",
			LoopName:          "delivery",
			Status:            StatusRunning,
			Generation:        1,
			ReattemptStrategy: ReattemptFailedOnly,
		}
		coordinatorRun := task.Run{
			ID:        "run-coordinator-sub-loop-retry",
			TaskID:    "task-coordinator-sub-loop-retry",
			RunKind:   task.RunKindCoordinator,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusClaimed,
		}
		graph := dsl.Graph{
			Nodes: []dsl.Node{
				{ID: "child", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunLoop)},
				{ID: "after", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent)},
			},
			Edges: []dsl.Edge{{From: "child", To: "after"}},
		}
		runner := newCoordinatorRunnerForTestWithGraph(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{coordinatorRun.ID: coordinatorRun},
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
				{
					Generation:     1,
					NodeID:         "child",
					Status:         generationOutputFailed,
					OutputRef:      "child_failed",
					ChildLoopRunID: "looprun-child-existing",
				},
				{
					Generation: 1,
					NodeID:     "after",
					Status:     generationOutputPending,
				},
			}}},
			graph,
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		nextOutputs := outputsByNodeForTest(coordinatorPostReservePayloadForTest(t, plan).Outputs)
		child := nextOutputs["child"]
		if got, want := child.Status, generationOutputPending; got != want {
			t.Fatalf("child status = %q, want %q", got, want)
		}
		if got := child.ChildLoopRunID; got != "" {
			t.Fatalf("child_loop_run_id = %q, want cleared for rerun", got)
		}
		if child.TaskRunID != "" || child.OutputRef != "" {
			t.Fatalf("child retry kept task/output refs: %#v", child)
		}
		if got, want := nextOutputs["after"].Status, generationOutputPending; got != want {
			t.Fatalf("after status = %q, want %q", got, want)
		}
	})
}

func TestCoordinatorRunnerShouldExhaustWhenIterationCapHit(t *testing.T) {
	t.Run("Should exhaust instead of scheduling retry past iteration cap", func(t *testing.T) {
		t.Parallel()

		loopRun := Run{
			ID:                "looprun-iteration-cap",
			WorkspaceID:       "ws-1",
			LoopName:          "delivery",
			Status:            StatusRunning,
			Generation:        1,
			ReattemptStrategy: ReattemptFullBody,
			IterationCap:      1,
		}
		coordinatorRun := task.Run{
			ID:        "run-coordinator-iteration-cap",
			TaskID:    "task-coordinator-iteration-cap",
			RunKind:   task.RunKindCoordinator,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusClaimed,
		}
		runner := newCoordinatorRunnerForTestWithDefinition(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{coordinatorRun.ID: coordinatorRun},
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
				{
					Generation: 1,
					NodeID:     "load",
					Status:     generationOutputSucceeded,
					OutputRef:  "sha256:load",
					TaskRunID:  "run-load-g1",
				},
				{
					Generation: 1,
					NodeID:     "agent",
					Status:     generationOutputFailed,
					OutputRef:  "tool_failed",
					TaskRunID:  "run-agent-g1",
				},
			}}},
			dsl.Definition{
				Graph:    coordinatorTestGraph(),
				Contract: dsl.Contract{IterationCap: 50},
			},
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.Terminal == nil {
			t.Fatal("Terminal = nil, want exhausted iteration cap")
		}
		if got, want := plan.Terminal.Status, string(StatusExhausted); got != want {
			t.Fatalf("terminal status = %q, want %q", got, want)
		}
		if got, want := plan.Terminal.Cause, string(TransitionCauseIterationCap); got != want {
			t.Fatalf("terminal cause = %q, want %q", got, want)
		}
		if got, want := plan.Terminal.ReasonCode, "iteration_cap_exceeded"; got != want {
			t.Fatalf("terminal reason_code = %q, want %q", got, want)
		}
		if plan.NextCoordinator != nil || len(plan.NodeRuns) != 0 {
			t.Fatalf(
				"retry work scheduled after cap: next=%#v node_runs=%d",
				plan.NextCoordinator,
				len(plan.NodeRuns),
			)
		}
	})
}

func TestCoordinatorRunnerShouldStallOnRepeatedBlockingIssueSignature(t *testing.T) {
	t.Run("Should stall on repeated blocking issue signature", func(t *testing.T) {
		t.Parallel()

		loopRun := Run{
			ID:          "looprun-blocker-stall",
			WorkspaceID: "ws-1",
			LoopName:    "delivery",
			Status:      StatusRunning,
			Generation:  2,
		}
		coordinatorRun := task.Run{
			ID:        "run-coordinator-blocker-stall",
			TaskID:    "task-coordinator-blocker-stall",
			RunKind:   task.RunKindCoordinator,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusClaimed,
		}
		blockerPayload := json.RawMessage(`{"blocking_issues":[{"id":"missing-reviewer"},{"id":"blocked-api"}]}`)
		blockerRef := OutputRefForPayload(blockerPayload)
		outputStore := coordinatorRunnerOutputs{
			outputs: map[int][]GenerationOutput{
				1: {{Generation: 1, NodeID: "load", Status: generationOutputFailed, OutputRef: blockerRef}},
				2: {{Generation: 2, NodeID: "load", Status: generationOutputFailed, OutputRef: blockerRef}},
			},
			payloads: map[GenerationOutputPayloadKey]json.RawMessage{
				{WorkspaceID: loopRun.WorkspaceID, RunID: loopRun.ID, Generation: 1,
					NodeID: "load", OutputRef: blockerRef}: blockerPayload,
				{WorkspaceID: loopRun.WorkspaceID, RunID: loopRun.ID, Generation: 2,
					NodeID: "load", OutputRef: blockerRef}: blockerPayload,
			},
		}
		runner := newCoordinatorRunnerForTestWithDefinition(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{coordinatorRun.ID: coordinatorRun},
			outputStore,
			dsl.Definition{
				Graph:    coordinatorTestGraph(),
				Contract: dsl.Contract{NoProgress: dsl.NoProgress{Window: 2}},
			},
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.Terminal == nil {
			t.Fatal("Terminal = nil, want stalled")
		}
		if got, want := plan.Terminal.Status, string(StatusStalled); got != want {
			t.Fatalf("terminal status = %q, want %q", got, want)
		}
		if got, want := plan.Terminal.ReasonCode, blockingIssuesRepeatedCode; got != want {
			t.Fatalf("reason_code = %q, want %q", got, want)
		}
	})
}

func TestCoordinatorRunnerShouldResetStallWhenBlockingIssueSignatureChanges(t *testing.T) {
	t.Run("Should reset stall when blocking issue signature changes", func(t *testing.T) {
		t.Parallel()

		loopRun := Run{
			ID:          "looprun-blocker-reset",
			WorkspaceID: "ws-1",
			LoopName:    "delivery",
			Status:      StatusRunning,
			Generation:  2,
		}
		coordinatorRun := task.Run{
			ID:        "run-coordinator-blocker-reset",
			TaskID:    "task-coordinator-blocker-reset",
			RunKind:   task.RunKindCoordinator,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusClaimed,
		}
		runner := newCoordinatorRunnerForTestWithDefinition(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{coordinatorRun.ID: coordinatorRun},
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{
				1: {{
					Generation: 1,
					NodeID:     "agent",
					Status:     generationOutputFailed,
					OutputRef:  `{"blocking_issues":[{"id":"old-blocker"}]}`,
				}},
				2: {{
					Generation: 2,
					NodeID:     "load",
					Status:     generationOutputFailed,
					OutputRef:  `{"blocking_issues":[{"id":"new-blocker"}]}`,
				}},
			}},
			dsl.Definition{
				Graph:    coordinatorTestGraph(),
				Contract: dsl.Contract{NoProgress: dsl.NoProgress{Window: 2}},
			},
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.Terminal != nil {
			t.Fatalf("Terminal = %#v, want nil after changed blocker signature", plan.Terminal)
		}
		if plan.NextCoordinator == nil {
			t.Fatal("NextCoordinator = nil, want failed-only retry after changed blocker signature")
		}
	})
}

func TestCoordinatorRunnerShouldIsolateRepeatedFailures(t *testing.T) {
	t.Parallel()

	t.Run("Should quarantine a repeated failing node while preserving its healthy sibling", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, time.August, 2, 12, 0, 0, 0, time.UTC)
		loopRun := Run{
			ID:          "looprun-circuit-breaker",
			WorkspaceID: "ws-1",
			LoopName:    "delivery",
			Status:      StatusRunning,
			Generation:  2,
		}
		coordinatorRun := task.Run{
			ID:        "run-coordinator-circuit-breaker",
			TaskID:    "task-coordinator-circuit-breaker",
			RunKind:   task.RunKindCoordinator,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusClaimed,
		}
		graph := dsl.Graph{Nodes: []dsl.Node{
			{ID: "a_failing", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform)},
			{ID: "z_healthy", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform)},
		}}
		failureClass := FailureTransport
		outputs := &lifecycleCoordinatorStore{
			coordinatorRunnerOutputs: coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{
				1: {
					{Generation: 1, NodeID: "a_failing", Status: generationOutputFailed},
					{Generation: 1, NodeID: "z_healthy", Status: generationOutputSucceeded},
				},
				2: {
					{Generation: 2, NodeID: "a_failing", Status: generationOutputFailed},
					{Generation: 2, NodeID: "z_healthy", Status: generationOutputSucceeded},
				},
			}},
			attempts: []NodeAttempt{
				{
					LoopRunID: loopRun.ID, Generation: 1, NodeID: "a_failing", ItemIndex: 0,
					Attempt: 1, FailureClass: &failureClass, FailureCode: "provider_unavailable",
					Cause: "provider unavailable", Target: "transform:a_failing",
					Disposition: AttemptEscalated, StartedAt: now.Add(-2 * time.Minute),
				},
				{
					LoopRunID: loopRun.ID, Generation: 2, NodeID: "a_failing", ItemIndex: 0,
					Attempt: 1, FailureClass: &failureClass, FailureCode: "provider_unavailable",
					Cause: "provider unavailable", Target: "transform:a_failing",
					Disposition: AttemptEscalated, StartedAt: now.Add(-time.Minute),
				},
			},
		}
		runner := newCoordinatorRunnerForTestWithDefinition(
			t,
			loopRun,
			coordinatorRun,
			nil,
			outputs,
			dsl.Definition{Graph: graph},
			WithCoordinatorNodeAttemptReader(outputs),
		)
		runner.now = func() time.Time { return now }

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.Terminal != nil || plan.NextCoordinator == nil || plan.PostReserveSnapshot == nil {
			t.Fatalf("plan = %#v, want non-terminal quarantine succession", plan)
		}
		payload := coordinatorSnapshotPayloadForTest(t, plan)
		if got, want := len(payload.Controls), 1; got != want ||
			payload.Controls[0].Kind != NodeControlMutationQuarantine {
			t.Fatalf("control intents = %#v, want one quarantine", payload.Controls)
		}
		if got, want := len(payload.Events), 1; got != want ||
			payload.Events[0].Kind != GenerationLifecycleEventNodeQuarantined {
			t.Fatalf("events = %#v, want node_quarantined", payload.Events)
		}
		next := outputsByNodeAndItemForTest(coordinatorPostReservePayloadForTest(t, plan).Outputs)
		if got := next["a_failing/0"]; got.Status != generationOutputQuarantined {
			t.Fatalf("quarantined output = %#v", got)
		}
		if got := next["z_healthy/0"]; got.Status != generationOutputSucceeded {
			t.Fatalf("healthy output = %#v", got)
		}
	})

	t.Run("Should backstop an unbounded watch after consecutive failed generations", func(t *testing.T) {
		t.Parallel()

		loopRun := Run{
			ID: "looprun-watch-breaker", WorkspaceID: "ws-1", LoopName: "watch",
			Status: StatusRunning, Generation: 2, IterationCap: 0,
		}
		coordinatorRun := task.Run{
			ID: "run-coordinator-watch-breaker", TaskID: "task-coordinator-watch-breaker",
			RunKind: task.RunKindCoordinator, LoopRunID: string(loopRun.ID),
			Status: task.TaskRunStatusClaimed,
		}
		graph := dsl.Graph{Nodes: []dsl.Node{
			{ID: "watch", Class: dsl.NodeClassSource, Kind: string(dsl.SourceWatchSource)},
			{ID: "fail_a", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform)},
			{ID: "fail_b", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform)},
		}}
		runner := newCoordinatorRunnerForTestWithDefinition(
			t,
			loopRun,
			coordinatorRun,
			nil,
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{
				1: {
					{Generation: 1, NodeID: "watch", Status: generationOutputSucceeded},
					{Generation: 1, NodeID: "fail_a", Status: generationOutputFailed},
					{Generation: 1, NodeID: "fail_b", Status: generationOutputSucceeded},
				},
				2: {
					{Generation: 2, NodeID: "watch", Status: generationOutputSucceeded},
					{Generation: 2, NodeID: "fail_a", Status: generationOutputSucceeded},
					{Generation: 2, NodeID: "fail_b", Status: generationOutputFailed},
				},
			}},
			dsl.Definition{Graph: graph},
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		assertCircuitBreakerTerminal(t, plan.Terminal)
	})

	t.Run("Should never trip for healthy generations", func(t *testing.T) {
		t.Parallel()

		loopRun := Run{
			ID: "looprun-healthy-breaker", WorkspaceID: "ws-1", LoopName: "delivery",
			Status: StatusRunning, Generation: 2,
		}
		coordinatorRun := task.Run{
			ID: "run-coordinator-healthy-breaker", TaskID: "task-coordinator-healthy-breaker",
			RunKind: task.RunKindCoordinator, LoopRunID: string(loopRun.ID),
			Status: task.TaskRunStatusClaimed,
		}
		graph := dsl.Graph{Nodes: []dsl.Node{
			{ID: "a", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform)},
			{ID: "b", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform)},
		}}
		healthy := []GenerationOutput{
			{NodeID: "a", Status: generationOutputSucceeded},
			{NodeID: "b", Status: generationOutputSucceeded},
		}
		runner := newCoordinatorRunnerForTestWithDefinition(
			t,
			loopRun,
			coordinatorRun,
			nil,
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: healthy, 2: healthy}},
			dsl.Definition{Graph: graph},
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.Terminal != nil && plan.Terminal.ReasonCode == circuitBreakerReasonCode {
			t.Fatalf("Terminal = %#v, want no circuit breaker", plan.Terminal)
		}
	})
}

func TestCoordinatorRunnerShouldPlanRequeueThroughSuccession(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 2, 12, 30, 0, 0, time.UTC)
	loopRun := Run{
		ID:           "looprun-requeue-succession",
		ProfileID:    "profile-default",
		WorkspaceID:  "ws-1",
		LoopName:     "delivery",
		Status:       StatusRunning,
		Generation:   2,
		IterationCap: 7,
		Origin:       &RunOrigin{Kind: RunOriginCatalog},
	}
	coordinatorRun := controlCoordinatorRun(loopRun, 2)
	entry, err := encodeQuarantineEntry(QuarantineEntry{
		NodeID:   "finish",
		InputRef: "loop-run:looprun-requeue-succession:node:finish:input",
		Episodes: []QuarantineEpisode{{Generation: 2, QuarantinedAt: now}},
		Requeues: []QuarantineProvenance{{
			ActorKind: "human", ActorID: "operator:alice", RequestedAt: now, Generation: 3,
		}},
	})
	if err != nil {
		t.Fatalf("encodeQuarantineEntry() error = %v", err)
	}
	controls := coordinatorNodeControlReaderStub{controls: []NodeControl{{
		LoopRunID: loopRun.ID, NodeID: "finish", Quarantined: false,
		QuarantineEntry: entry, Revision: 5, UpdatedAt: now,
	}}}
	graph := dsl.Graph{Nodes: []dsl.Node{{
		ID: "finish", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent),
		Params: dsl.NodeParams{"agent": "planner", "prompt": "finish the repair"},
	}}}
	runner := newCoordinatorRunnerForTestWithDefinition(
		t,
		loopRun,
		coordinatorRun,
		nil,
		coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{2: {{
			Generation: 2, NodeID: "finish", Status: generationOutputQuarantined, Attempt: 1, Epoch: 10,
		}}}},
		dsl.Definition{Graph: graph},
		WithCoordinatorNodeControlReader(controls),
	)
	runner.now = func() time.Time { return now }

	plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if plan.Terminal != nil || plan.NextCoordinator == nil || plan.PostReserveSnapshot == nil {
		t.Fatalf("plan = %#v, want non-terminal requeue succession", plan)
	}
	payload := coordinatorPostReservePayloadForTest(t, plan)
	if payload.GenerationProvenance == nil || payload.GenerationProvenance.Origin != OriginRequeue ||
		payload.GenerationProvenance.Generation != 3 || payload.GenerationProvenance.ParentGeneration != 2 {
		t.Fatalf("generation provenance = %#v, want requeue 2 -> 3", payload.GenerationProvenance)
	}
	if len(payload.Outputs) != 1 || payload.Outputs[0].Status != generationOutputPending ||
		payload.Outputs[0].Generation != 3 {
		t.Fatalf("requeue outputs = %#v, want pending generation 3", payload.Outputs)
	}

	health := newRecordingTargetHealth()
	health.decision = deadentity.ProbeDecision{
		Allowed: false,
		Dead:    true,
		Reason:  "target remains unavailable after repair",
	}
	actions, err := NewActionRegistry(&internalActionRegistryFake{})
	if err != nil {
		t.Fatalf("NewActionRegistry() error = %v", err)
	}
	runAfterRequeue := loopRun
	runAfterRequeue.Generation = 3
	coordinatorAfterRequeue := controlCoordinatorRun(runAfterRequeue, 3)
	runnerAfterRequeue := newCoordinatorRunnerForTestWithDefinition(
		t,
		runAfterRequeue,
		coordinatorAfterRequeue,
		nil,
		coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{3: payload.Outputs}},
		dsl.Definition{Graph: graph},
		WithCoordinatorNodeControlReader(controls),
		WithCoordinatorActionRegistry(actions),
		WithCoordinatorTargetHealth(health),
	)
	readyPlan, err := runnerAfterRequeue.Run(
		context.Background(), task.RunID(coordinatorAfterRequeue.ID),
	)
	if err != nil {
		t.Fatalf("Run(after requeue) error = %v", err)
	}
	if len(readyPlan.NodeRuns) != 1 {
		t.Fatalf("requeue node runs = %#v, want one admitted attempt", readyPlan.NodeRuns)
	}
	workerSpec := readyPlan.NodeRuns[0]
	workerRun := task.Run{
		ID: workerSpec.RunID, TaskID: workerSpec.TaskID, RunKind: task.RunKindWorker,
		LoopRunID: string(runAfterRequeue.ID), Status: task.TaskRunStatusClaimed, Metadata: workerSpec.Metadata,
	}
	_, err = runnerAfterRequeue.ExecuteActionRun(context.Background(), workerRun, task.ActorContext{})
	var safeFailure SafeActionFailureProvider
	if !errors.As(err, &safeFailure) || safeFailure.SafeActionFailure().Code != targetUnavailableReasonCode {
		t.Fatalf("ExecuteActionRun(requeued) error = %v, want target_unavailable", err)
	}
	if got := health.probedKeys(); len(got) != 1 || got[0].EntityID != "run-agent:planner" {
		t.Fatalf("requeue probe keys = %#v, want run-agent:planner", got)
	}
}

func TestCoordinatorRunnerShouldPlanEveryPendingRequeueInOneGeneration(t *testing.T) {
	t.Parallel()

	t.Run("Should place every pending requeue in one successor generation", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, time.August, 2, 13, 0, 0, 0, time.UTC)
		run := Run{
			ID: "looprun-multi-requeue", WorkspaceID: "ws-1", LoopName: "delivery",
			Status: StatusRunning, Generation: 2, IterationCap: 7,
		}
		entryFor := func(nodeID NodeID) json.RawMessage {
			entry, err := encodeQuarantineEntry(QuarantineEntry{
				NodeID: nodeID, InputRef: "loop-run:looprun-multi-requeue:node:" + string(nodeID) + ":input",
				Episodes: []QuarantineEpisode{{Generation: 2, QuarantinedAt: now}},
				Requeues: []QuarantineProvenance{{
					ActorKind: "human", ActorID: "operator:alice", RequestedAt: now, Generation: 3,
				}},
			})
			if err != nil {
				t.Fatalf("encodeQuarantineEntry(%s) error = %v", nodeID, err)
			}
			return entry
		}
		controls := coordinatorNodeControlReaderStub{controls: []NodeControl{
			{LoopRunID: run.ID, NodeID: "first", QuarantineEntry: entryFor("first")},
			{LoopRunID: run.ID, NodeID: "second", QuarantineEntry: entryFor("second")},
		}}
		graph := dsl.Graph{Nodes: []dsl.Node{
			{ID: "first", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform)},
			{ID: "second", Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform)},
		}}
		current := []GenerationOutput{
			{Generation: 2, NodeID: "first", Status: generationOutputQuarantined, Attempt: 1, Epoch: 2},
			{Generation: 2, NodeID: "second", Status: generationOutputQuarantined, Attempt: 1, Epoch: 3},
		}
		runner := &CoordinatorRunner{controls: controls}
		plan, found, err := runner.buildPendingRequeuePlan(
			t.Context(), controlCoordinatorRun(run, 2), run, graph, current,
		)
		if err != nil {
			t.Fatalf("buildPendingRequeuePlan() error = %v", err)
		}
		if !found || plan.PostReserveSnapshot == nil {
			t.Fatalf("requeue plan = %#v found=%v, want one successor snapshot", plan, found)
		}
		payload := coordinatorPostReservePayloadForTest(t, plan)
		outputs := outputsByNodeAndItemForTest(payload.Outputs)
		for _, nodeID := range []string{"first", "second"} {
			output := outputs[nodeID+"/0"]
			if output.Generation != 3 || output.Status != generationOutputPending {
				t.Fatalf("requeued output %s = %#v, want pending generation 3", nodeID, output)
			}
		}
	})
}

func assertCircuitBreakerTerminal(t *testing.T, terminal *task.CoordinatorTerminal) {
	t.Helper()

	if terminal == nil {
		t.Fatal("Terminal = nil, want stalled")
	}
	if got, want := terminal.Status, string(StatusStalled); got != want {
		t.Fatalf("terminal status = %q, want %q", got, want)
	}
	if got, want := terminal.Cause, string(TransitionCauseNoProgress); got != want {
		t.Fatalf("terminal cause = %q, want %q", got, want)
	}
	if got, want := terminal.ReasonCode, circuitBreakerReasonCode; got != want {
		t.Fatalf("reason_code = %q, want %q", got, want)
	}
}

func TestCoordinatorRunnerShouldTreatZeroTokenBudgetAsUnlimited(t *testing.T) {
	t.Run("Should treat zero token budget as unlimited", func(t *testing.T) {
		t.Parallel()

		loopRun := Run{
			ID:           "looprun-budget-zero",
			WorkspaceID:  "ws-1",
			LoopName:     "delivery",
			Status:       StatusRunning,
			Generation:   0,
			BudgetTokens: 0,
			TokensUsed:   100,
		}
		coordinatorRun := task.Run{
			ID:        "run-coordinator-budget-zero",
			TaskID:    "task-coordinator-budget-zero",
			RunKind:   task.RunKindCoordinator,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusClaimed,
		}
		runner := newCoordinatorRunnerForTest(t, loopRun, coordinatorRun, map[string]task.Run{
			coordinatorRun.ID: coordinatorRun,
		}, coordinatorRunnerOutputs{})

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.Terminal != nil {
			t.Fatalf("Terminal = %#v, want nil for unlimited zero budget", plan.Terminal)
		}
		if got, want := len(plan.NodeRuns), 1; got != want {
			t.Fatalf("node runs = %d, want %d", got, want)
		}
	})
}

func TestCoordinatorRunnerShouldApplyLoopControlHooks(t *testing.T) {
	t.Run("Should fail generation when generation pre denies", func(t *testing.T) {
		t.Parallel()

		loopRun := Run{
			ID:          "looprun-generation-pre-denied",
			WorkspaceID: "ws-1",
			LoopName:    "delivery",
			Status:      StatusRunning,
			Generation:  0,
		}
		coordinatorRun := task.Run{
			ID:        "run-coordinator-generation-pre-denied",
			TaskID:    "task-coordinator-generation-pre-denied",
			RunKind:   task.RunKindCoordinator,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusClaimed,
		}
		hooks := &coordinatorHookDispatcher{
			generationPre: func(
				context.Context,
				hookspkg.LoopGenerationPrePayload,
			) (hookspkg.LoopGenerationPrePayload, error) {
				return hookspkg.LoopGenerationPrePayload{
					Denied:     true,
					DenyReason: "policy_denied",
				}, nil
			},
		}
		runner := newCoordinatorRunnerForTest(t, loopRun, coordinatorRun, nil, coordinatorRunnerOutputs{})
		runner.hooks = hooks

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.Terminal == nil {
			t.Fatal("Terminal = nil, want failed generation")
		}
		if got, want := plan.Terminal.Status, string(StatusFailed); got != want {
			t.Fatalf("terminal status = %q, want %q", got, want)
		}
		if got, want := plan.Terminal.ReasonCode, "policy_denied"; got != want {
			t.Fatalf("reason_code = %q, want %q", got, want)
		}
		if len(plan.NodeRuns) != 0 {
			t.Fatalf("node runs = %d, want 0 after generation pre deny", len(plan.NodeRuns))
		}
		if hooks.generationPostCalls != 0 || hooks.gatePreCalls != 0 || hooks.gatePostCalls != 0 {
			t.Fatalf(
				"unexpected hook calls after generation deny: post=%d gate_pre=%d gate_post=%d",
				hooks.generationPostCalls,
				hooks.gatePreCalls,
				hooks.gatePostCalls,
			)
		}
	})

	t.Run("Should block terminal plan when gate pre denies", func(t *testing.T) {
		t.Parallel()

		loopRun := Run{
			ID:          "looprun-gate-pre-denied",
			WorkspaceID: "ws-1",
			LoopName:    "delivery",
			Status:      StatusRunning,
			Generation:  1,
		}
		coordinatorRun := task.Run{
			ID:        "run-coordinator-gate-pre-denied",
			TaskID:    "task-coordinator-gate-pre-denied",
			RunKind:   task.RunKindCoordinator,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusClaimed,
		}
		hooks := &coordinatorHookDispatcher{}
		preStatuses := make([]string, 0, 2)
		postStatuses := make([]string, 0, 2)
		hooks.gatePre = func(
			_ context.Context,
			payload hookspkg.LoopGatePrePayload,
		) (hookspkg.LoopGatePrePayload, error) {
			preStatuses = append(preStatuses, payload.Status)
			reason := "later_denial"
			if payload.GateID == "quality" {
				reason = "human_gate"
			}
			return hookspkg.LoopGatePrePayload{Denied: true, DenyReason: reason}, nil
		}
		hooks.gatePost = func(
			_ context.Context,
			payload hookspkg.LoopGatePostPayload,
		) (hookspkg.LoopGatePostPayload, error) {
			postStatuses = append(postStatuses, payload.Status)
			return payload, nil
		}
		runner := newCoordinatorRunnerForTest(t, loopRun, coordinatorRun, nil, coordinatorRunnerOutputs{})
		runner.hooks = hooks
		plan := runner.dispatchGateHooks(t.Context(), coordinatorRun, loopRun, task.CoordinatorCompletionPlan{
			Snapshot: task.GenerationSnapshot{
				LoopRunID:  string(loopRun.ID),
				Generation: 1,
				Payload: GenerationSnapshotPayload{Verdicts: []gate.VerdictIntent{
					{
						GateID:         "quality",
						Outcome:        gate.VerdictOutcomeApproved,
						BlockingIssues: []byte("[]"),
						Criteria:       []byte("[]"),
					},
					{
						GateID:         "security",
						Outcome:        gate.VerdictOutcomeApproved,
						BlockingIssues: []byte("[]"),
						Criteria:       []byte("[]"),
					},
				}},
			},
		})
		if plan.Terminal == nil {
			t.Fatal("Terminal = nil, want blocked gate")
		}
		if got, want := plan.Terminal.Status, string(StatusBlocked); got != want {
			t.Fatalf("terminal status = %q, want %q", got, want)
		}
		if got, want := plan.Terminal.ReasonCode, "human_gate"; got != want {
			t.Fatalf("reason_code = %q, want %q", got, want)
		}
		if hooks.gatePreCalls != 2 || hooks.gatePostCalls != 2 {
			t.Fatalf(
				"gate hook calls = pre:%d post:%d, want pre:2 post:2",
				hooks.gatePreCalls,
				hooks.gatePostCalls,
			)
		}
		if got, want := hooks.lastGatePostStatus, string(StatusBlocked); got != want {
			t.Fatalf("gate post status = %q, want %q", got, want)
		}
		if got, want := preStatuses, []string{
			coordinatorGateStatusEvaluated,
			coordinatorGateStatusEvaluated,
		}; !reflect.DeepEqual(got, want) {
			t.Fatalf("gate pre statuses = %#v, want stable %#v", got, want)
		}
		if got, want := postStatuses, []string{
			string(StatusBlocked),
			string(StatusBlocked),
		}; !reflect.DeepEqual(got, want) {
			t.Fatalf("gate post statuses = %#v, want stable %#v", got, want)
		}
	})

	t.Run("Should expose real gate verdict and successor provenance to hooks", func(t *testing.T) {
		t.Parallel()

		bestGeneration := int64(1)
		bestScore := 0.8
		verdictScore := 0.7
		loopRun := Run{
			ID: "looprun-hook-verdict", WorkspaceID: "ws-1", LoopName: "delivery",
			Status: StatusRunning, Generation: 2, IterationCap: 3,
			BestGeneration: &bestGeneration, BestScore: &bestScore,
		}
		coordinatorRun := task.Run{
			ID: "run-coordinator-hook-verdict", TaskID: "task-coordinator-hook-verdict",
			RunKind: task.RunKindCoordinator, LoopRunID: string(loopRun.ID), Status: task.TaskRunStatusClaimed,
		}
		graph := dsl.Graph{
			Nodes: []dsl.Node{
				{ID: "draft", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent)},
				testMetricRouteGateNode("quality"),
			},
			Edges: []dsl.Edge{{From: "draft", To: "quality"}},
		}
		runner := newCoordinatorRunnerForTestWithDefinition(
			t,
			loopRun,
			coordinatorRun,
			nil,
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{
				1: {
					{Generation: 1, NodeID: "draft", Status: generationOutputSucceeded, OutputRef: `{"draft":"best"}`},
				},
				2: {
					{Generation: 2, NodeID: "draft", Status: generationOutputSucceeded, OutputRef: `{"draft":2}`},
					{Generation: 2, NodeID: "quality", Status: generationOutputPending},
				},
			}},
			dsl.Definition{Graph: graph},
			WithCoordinatorGateEvaluator(gateEvaluatorFunc(
				func(context.Context, gate.Gate, gate.GateInput) (gate.Verdict, error) {
					verdict := testRouteVerdict(gate.RouteRevise)
					verdict.Criteria[0].Score = &verdictScore
					return verdict, nil
				},
			)),
		)
		hooks := &coordinatorHookDispatcher{}
		runner.hooks = hooks

		plan, err := runner.Run(t.Context(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.PostReserveSnapshot == nil {
			t.Fatal("PostReserveSnapshot = nil, want ratchet successor")
		}
		if hooks.lastGenerationPre.Origin != hookspkg.LoopGenerationOriginRatchetRestore ||
			hooks.lastGenerationPre.ParentGeneration != bestGeneration {
			t.Fatalf(
				"generation pre payload = %#v, want ratchet restore from %d",
				hooks.lastGenerationPre,
				bestGeneration,
			)
		}
		post := hooks.lastGatePost
		if post.GateID != "quality" || post.Outcome != string(gate.VerdictOutcomeRejected) ||
			post.Score == nil || *post.Score != verdictScore ||
			post.BestGeneration == nil || *post.BestGeneration != bestGeneration {
			t.Fatalf("gate post payload = %#v, want rejected score %.2f best %d", post, verdictScore, bestGeneration)
		}
	})

	t.Run("Should fail open when loop hook errors", func(t *testing.T) {
		t.Parallel()

		loopRun := Run{
			ID:          "looprun-hook-fail-open",
			WorkspaceID: "ws-1",
			LoopName:    "delivery",
			Status:      StatusRunning,
			Generation:  0,
		}
		coordinatorRun := task.Run{
			ID:        "run-coordinator-hook-fail-open",
			TaskID:    "task-coordinator-hook-fail-open",
			RunKind:   task.RunKindCoordinator,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusClaimed,
		}
		hooks := &coordinatorHookDispatcher{
			generationPre: func(
				context.Context,
				hookspkg.LoopGenerationPrePayload,
			) (hookspkg.LoopGenerationPrePayload, error) {
				return hookspkg.LoopGenerationPrePayload{}, errors.New("hook runner unavailable")
			},
			generationPost: func(
				context.Context,
				hookspkg.LoopGenerationPostPayload,
			) (hookspkg.LoopGenerationPostPayload, error) {
				return hookspkg.LoopGenerationPostPayload{}, errors.New("hook runner unavailable")
			},
		}
		runner := newCoordinatorRunnerForTest(t, loopRun, coordinatorRun, nil, coordinatorRunnerOutputs{})
		runner.hooks = hooks

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.Terminal != nil {
			t.Fatalf("Terminal = %#v, want nil after fail-open hook errors", plan.Terminal)
		}
		if got, want := len(plan.NodeRuns), 1; got != want {
			t.Fatalf("node runs = %d, want %d after fail-open hook errors", got, want)
		}
		if hooks.generationPreCalls != 1 || hooks.generationPostCalls != 1 {
			t.Fatalf(
				"generation hook calls = pre:%d post:%d, want pre:1 post:1",
				hooks.generationPreCalls,
				hooks.generationPostCalls,
			)
		}
	})
}

func TestCoordinatorRunnerShouldResolveInputSourceNodes(t *testing.T) {
	t.Run("Should complete input sources without queuing action task runs", func(t *testing.T) {
		t.Parallel()

		loopRun := Run{
			ID:          "looprun-input-source",
			WorkspaceID: "ws-1",
			LoopName:    "software-delivery",
			Status:      StatusRunning,
			Generation:  0,
			Inputs: map[string]any{
				"slug": "loops-refac",
			},
		}
		coordinatorRun := task.Run{
			ID:        "run-coordinator-input-source",
			TaskID:    "task-coordinator-input-source",
			RunKind:   task.RunKindCoordinator,
			LoopRunID: string(loopRun.ID),
			Status:    task.TaskRunStatusClaimed,
		}
		def := dsl.Definition{
			Inputs: map[string]dsl.Input{
				"slug": {Type: dsl.InputTypeString, Required: true},
			},
			Graph: dsl.Graph{
				Nodes: []dsl.Node{{
					ID:       "slug_input",
					Class:    dsl.NodeClassSource,
					Kind:     string(dsl.SourceInput),
					InputRef: "slug",
				}},
			},
		}
		runner := newCoordinatorRunnerForTestWithDefinition(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{coordinatorRun.ID: coordinatorRun},
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{}},
			def,
		)

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if len(plan.NodeRuns) != 0 {
			t.Fatalf("node runs = %#v, want none for input source", plan.NodeRuns)
		}
		outputs := outputsByNodeForTest(coordinatorSnapshotPayloadForTest(t, plan).Outputs)
		slugInput, ok := outputs["slug_input"]
		if !ok {
			t.Fatal("slug_input output missing")
		}
		if got, want := slugInput.Status, generationOutputSucceeded; got != want {
			t.Fatalf("slug_input status = %q, want %q", got, want)
		}
		if got, want := outputValue(slugInput.OutputRef), "loops-refac"; got != want {
			t.Fatalf("slug_input output = %#v, want %#v", got, want)
		}
		if plan.Terminal == nil {
			t.Fatal("Terminal = nil, want done after input source succeeds")
		}
		if got, want := plan.Terminal.Status, string(StatusDone); got != want {
			t.Fatalf("terminal status = %q, want %q", got, want)
		}
	})
}

func newCoordinatorRunnerForTest(
	t *testing.T,
	loopRun Run,
	coordinatorRun task.Run,
	runs map[string]task.Run,
	outputs GenerationOutputReader,
) *CoordinatorRunner {
	t.Helper()
	return newCoordinatorRunnerForTestWithGraph(
		t,
		loopRun,
		coordinatorRun,
		runs,
		outputs,
		coordinatorTestGraph(),
	)
}

func newCoordinatorRunnerForTestWithGraph(
	t *testing.T,
	loopRun Run,
	coordinatorRun task.Run,
	runs map[string]task.Run,
	outputs GenerationOutputReader,
	graph dsl.Graph,
) *CoordinatorRunner {
	t.Helper()
	loopRun = loopRunWithInputSourceDefaults(loopRun, graph)
	return newCoordinatorRunnerForTestWithDefinition(
		t,
		loopRun,
		coordinatorRun,
		runs,
		outputs,
		dsl.Definition{Graph: graph},
	)
}

func newCoordinatorRunnerForTestWithDefinition(
	t *testing.T,
	loopRun Run,
	coordinatorRun task.Run,
	runs map[string]task.Run,
	outputs GenerationOutputReader,
	def dsl.Definition,
	opts ...CoordinatorRunnerOption,
) *CoordinatorRunner {
	t.Helper()
	if runs == nil {
		runs = map[string]task.Run{coordinatorRun.ID: coordinatorRun}
	}
	resolved := resolvedCoordinatorDefinitionForTest(t, def)
	definitionDefaults := LoopDefaults{
		Delivery: definitionConfigLayer(def),
		Watch:    definitionConfigLayer(def),
	}
	effective, err := ResolveEffectiveConfig(resolved, definitionDefaults, nil, LoopConfig{})
	if err != nil {
		t.Fatalf("ResolveEffectiveConfig() error = %v", err)
	}
	loopRun, snapshot := pinCoordinatorResolvedForTest(t, loopRun, resolved, effective)
	options := []CoordinatorRunnerOption{}
	options = append(options, opts...)
	runner, err := NewCoordinatorRunner(
		&coordinatorRunnerTaskRunReader{runs: runs},
		&coordinatorRunnerLoopStore{run: loopRun, snapshot: snapshot},
		outputs,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		options...,
	)
	if err != nil {
		t.Fatalf("NewCoordinatorRunner() error = %v", err)
	}
	return runner
}

func pinCoordinatorResolvedForTest(
	t *testing.T,
	run Run,
	resolved *ResolvedDefinition,
	effective EffectiveConfig,
) (Run, *DefinitionSnapshot) {
	t.Helper()

	snapshotJSON, digest, err := BuildExecutedDefinitionSnapshot(resolved, effective)
	if err != nil {
		t.Fatalf("BuildExecutedDefinitionSnapshot() error = %v", err)
	}
	run.DefinitionVersion = resolved.DefinitionVersion
	run.DefinitionDigest = digest
	return run, &DefinitionSnapshot{
		WorkspaceID: run.WorkspaceID,
		Digest:      digest,
		Version:     resolved.DefinitionVersion,
		Definition:  snapshotJSON,
		ByteSize:    len(snapshotJSON),
	}
}

func setCoordinatorRunnerRunsForTest(
	t *testing.T,
	runner *CoordinatorRunner,
	runs map[RunID]Run,
) {
	t.Helper()

	store, ok := runner.store.(*coordinatorRunnerLoopStore)
	if !ok {
		t.Fatalf("coordinator store type = %T, want *coordinatorRunnerLoopStore", runner.store)
	}
	if _, exists := runs[store.run.ID]; exists {
		runs[store.run.ID] = store.run
	}
	store.runs = runs
	runner.store = store
}

func resolvedCoordinatorDefinitionForTest(t *testing.T, definition dsl.Definition) *ResolvedDefinition {
	t.Helper()

	resolved, err := NewCompiler().Compile(definition)
	if err == nil {
		return resolved
	}
	definition.Normalize()
	toolSchemas := map[string]ToolSchemaSnapshot{}
	openKinds := map[string]struct{}{}
	collectOpenActionKinds(definition.Graph, openKinds)
	for kind := range openKinds {
		toolSchemas[kind] = ToolSchemaSnapshot{
			ToolID:            kind,
			InputSchema:       []byte(`{}`),
			InputSchemaDigest: "test:" + kind,
		}
	}
	return &ResolvedDefinition{
		Definition:        foldDefinitionDefaults(definition),
		DefinitionVersion: definition.Meta.Version,
		Templates:         map[string]*refs.Template{},
		Conditions:        map[string]*refs.Condition{},
		ToolSchemas:       toolSchemas,
		WatchEventsContracts: referencedWatchEventsContracts(
			definition,
			SupportedWatchEvents(),
		),
		Defaults: ResolvedDefaults{
			FanOutBatchSize: 1,
			RunLoopMode:     dsl.RunLoopAwait,
			Concurrency:     definition.Concurrency,
		},
		compiled: true,
	}
}

func routePlannerDefinitionForTest() dsl.Definition {
	transform := func(id dsl.NodeID) dsl.Node {
		return dsl.Node{
			ID: id, Class: dsl.NodeClassAction, Kind: string(dsl.ActionTransform),
			Params: dsl.NodeParams{"map": map[string]any{"value": map[string]any{"value": string(id)}}},
		}
	}
	load := transform("load")
	load.Produces = dsl.Schema{"risk": "string"}
	return dsl.Definition{Graph: dsl.Graph{
		Nodes: []dsl.Node{
			load,
			{
				ID: "router", Class: dsl.NodeClassControl, Kind: string(dsl.ControlRoute),
				Routes: []dsl.RouteSpec{
					{When: `nodes.load.output.risk == "low"`, To: "quick"},
					{When: `nodes.load.output.risk == "high"`, To: "review"},
				},
				Default: "fallback",
			},
			transform("quick"),
			transform("review"),
			transform("fallback"),
			transform("shared"),
		},
		Edges: []dsl.Edge{
			{From: "load", To: "router"},
			{From: "router", To: "quick"},
			{From: "router", To: "review"},
			{From: "router", To: "fallback"},
			{From: "quick", To: "shared"},
			{From: "review", To: "shared"},
		},
	}}
}

func newCoordinatorRunnerForStopWhenTest(
	t *testing.T,
	loopRun Run,
	coordinatorRun task.Run,
	outputRef string,
) *CoordinatorRunner {
	t.Helper()
	return newCoordinatorRunnerForStopWhenSpecTest(
		t,
		loopRun,
		coordinatorRun,
		outputRef,
		dsl.StopWhenSpec{
			Expr: "nodes.inspect_issues.status == 'succeeded' && size(nodes.inspect_issues.output.issues) == 0",
		},
	)
}

func newCoordinatorRunnerForStopWhenSpecTest(
	t *testing.T,
	loopRun Run,
	coordinatorRun task.Run,
	outputRef string,
	stopWhen dsl.StopWhenSpec,
) *CoordinatorRunner {
	t.Helper()
	resolved := compileCoordinatorControlDefinition(t, dsl.Definition{
		Inputs: map[string]dsl.Input{
			"seed": {Type: dsl.InputTypeString},
		},
		Contract: dsl.Contract{StopWhen: stopWhen},
		Graph: dsl.Graph{
			Nodes: []dsl.Node{
				{
					ID:       "inspect_issues",
					Class:    dsl.NodeClassSource,
					Kind:     string(dsl.SourceInput),
					InputRef: "seed",
					Produces: dsl.Schema{"issues": "array"},
				},
				{
					ID:    "verify_gate",
					Class: dsl.NodeClassControl,
					Kind:  string(dsl.ControlGate),
					Criteria: []dsl.GateCriterion{{
						ID:     "human_review",
						Type:   dsl.CriterionHuman,
						Prompt: "Review current generation.",
					}},
					VerdictPolicy: dsl.VerdictPolicyFixedPasses,
				},
			},
			Edges: []dsl.Edge{{From: "inspect_issues", To: "verify_gate"}},
		},
	})
	effective, err := ResolveEffectiveConfig(
		resolved,
		LoopDefaults{
			Delivery: definitionConfigLayer(resolved.Definition),
			Watch:    definitionConfigLayer(resolved.Definition),
		},
		nil,
		LoopConfig{},
	)
	if err != nil {
		t.Fatalf("ResolveEffectiveConfig() error = %v", err)
	}
	loopRun, snapshot := pinCoordinatorResolvedForTest(t, loopRun, resolved, effective)
	runner, err := NewCoordinatorRunner(
		&coordinatorRunnerTaskRunReader{runs: map[string]task.Run{coordinatorRun.ID: coordinatorRun}},
		&coordinatorRunnerLoopStore{run: loopRun, snapshot: snapshot},
		coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
			{
				Generation: 1,
				NodeID:     "inspect_issues",
				Status:     generationOutputSucceeded,
				OutputRef:  outputRef,
			},
			{
				Generation: 1,
				NodeID:     "verify_gate",
				Status:     generationOutputSucceeded,
				OutputRef:  `{"outcome":"approved","route":{"action":"continue"}}`,
			},
		}}},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		WithCoordinatorGateEvaluator(gateEvaluatorFunc(
			func(context.Context, gate.Gate, gate.GateInput) (gate.Verdict, error) {
				t.Fatal("gate evaluator was called while all generation outputs were already terminal")
				return gate.Verdict{}, nil
			},
		)),
	)
	if err != nil {
		t.Fatalf("NewCoordinatorRunner() error = %v", err)
	}
	return runner
}

func coordinatorTestGraph() dsl.Graph {
	return dsl.Graph{
		Nodes: []dsl.Node{
			{
				ID:       "load",
				Class:    dsl.NodeClassSource,
				Kind:     string(dsl.SourceInput),
				InputRef: "load",
			},
			{ID: "agent", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent)},
		},
		Edges: []dsl.Edge{{From: "load", To: "agent"}},
	}
}

func loopRunWithInputSourceDefaults(loopRun Run, graph dsl.Graph) Run {
	inputs := cloneAnyMap(loopRun.Inputs)
	for _, node := range graph.Nodes {
		if !isInputSourceNode(node) {
			continue
		}
		if node.InputRef == "" {
			continue
		}
		if _, ok := inputs[node.InputRef]; !ok {
			inputs[node.InputRef] = string(node.ID)
		}
	}
	loopRun.Inputs = inputs
	return loopRun
}

func coordinatorSnapshotPayloadForTest(
	t *testing.T,
	plan task.CoordinatorCompletionPlan,
) GenerationSnapshotPayload {
	t.Helper()
	payload, ok := plan.Snapshot.Payload.(GenerationSnapshotPayload)
	if !ok {
		t.Fatalf(
			"snapshot payload type = %T, want GenerationSnapshotPayload",
			plan.Snapshot.Payload,
		)
	}
	return payload
}

func coordinatorPostReservePayloadForTest(
	t *testing.T,
	plan task.CoordinatorCompletionPlan,
) GenerationSnapshotPayload {
	t.Helper()

	if plan.PostReserveSnapshot == nil {
		t.Fatal("PostReserveSnapshot = nil, want snapshot for queued node runs")
	}
	payload, ok := plan.PostReserveSnapshot.Payload.(GenerationSnapshotPayload)
	if !ok {
		t.Fatalf(
			"post-reserve payload type = %T, want GenerationSnapshotPayload",
			plan.PostReserveSnapshot.Payload,
		)
	}
	return payload
}

func outputsByNodeForTest(outputs []GenerationOutput) map[string]GenerationOutput {
	mapped := make(map[string]GenerationOutput, len(outputs))
	for _, output := range outputs {
		mapped[output.NodeID] = output
	}
	return mapped
}

type gateEvaluatorFunc func(context.Context, gate.Gate, gate.GateInput) (gate.Verdict, error)

func (f gateEvaluatorFunc) Evaluate(
	ctx context.Context,
	runtimeGate gate.Gate,
	input gate.GateInput,
) (gate.Verdict, error) {
	return f(ctx, runtimeGate, input)
}

type coordinatorHookDispatcher struct {
	generationPre  func(context.Context, hookspkg.LoopGenerationPrePayload) (hookspkg.LoopGenerationPrePayload, error)
	generationPost func(context.Context, hookspkg.LoopGenerationPostPayload) (hookspkg.LoopGenerationPostPayload, error)
	gatePre        func(context.Context, hookspkg.LoopGatePrePayload) (hookspkg.LoopGatePrePayload, error)
	gatePost       func(context.Context, hookspkg.LoopGatePostPayload) (hookspkg.LoopGatePostPayload, error)

	generationPreCalls  int
	generationPostCalls int
	gatePreCalls        int
	gatePostCalls       int
	lastGatePostStatus  string
	lastGenerationPre   hookspkg.LoopGenerationPrePayload
	lastGatePost        hookspkg.LoopGatePostPayload
}

func (d *coordinatorHookDispatcher) DispatchLoopStarted(
	context.Context,
	hookspkg.LoopStartedPayload,
) (hookspkg.LoopStartedPayload, error) {
	return hookspkg.LoopStartedPayload{}, nil
}

func (d *coordinatorHookDispatcher) DispatchLoopGenerationPre(
	ctx context.Context,
	payload hookspkg.LoopGenerationPrePayload,
) (hookspkg.LoopGenerationPrePayload, error) {
	d.generationPreCalls++
	d.lastGenerationPre = payload
	if d.generationPre != nil {
		return d.generationPre(ctx, payload)
	}
	return payload, nil
}

func (d *coordinatorHookDispatcher) DispatchLoopGenerationPost(
	ctx context.Context,
	payload hookspkg.LoopGenerationPostPayload,
) (hookspkg.LoopGenerationPostPayload, error) {
	d.generationPostCalls++
	if d.generationPost != nil {
		return d.generationPost(ctx, payload)
	}
	return payload, nil
}

func (d *coordinatorHookDispatcher) DispatchLoopGatePre(
	ctx context.Context,
	payload hookspkg.LoopGatePrePayload,
) (hookspkg.LoopGatePrePayload, error) {
	d.gatePreCalls++
	if d.gatePre != nil {
		return d.gatePre(ctx, payload)
	}
	return payload, nil
}

func (d *coordinatorHookDispatcher) DispatchLoopGatePost(
	ctx context.Context,
	payload hookspkg.LoopGatePostPayload,
) (hookspkg.LoopGatePostPayload, error) {
	d.gatePostCalls++
	d.lastGatePostStatus = payload.Status
	d.lastGatePost = payload
	if d.gatePost != nil {
		return d.gatePost(ctx, payload)
	}
	return payload, nil
}

func (d *coordinatorHookDispatcher) DispatchLoopNodeTerminal(
	context.Context,
	hookspkg.LoopNodeTerminalPayload,
) (hookspkg.LoopNodeTerminalPayload, error) {
	return hookspkg.LoopNodeTerminalPayload{}, nil
}

func (d *coordinatorHookDispatcher) DispatchLoopTerminal(
	context.Context,
	hookspkg.LoopTerminalPayload,
) (hookspkg.LoopTerminalPayload, error) {
	return hookspkg.LoopTerminalPayload{}, nil
}

type coordinatorRunnerTaskRunReader struct {
	run  task.Run
	runs map[string]task.Run
}

type recordingActionExecutor struct {
	input   ActionExecutionInput
	execute func(dsl.Node, ActionExecutionInput) error
}

func (e *recordingActionExecutor) Execute(
	_ context.Context,
	node dsl.Node,
	input ActionExecutionInput,
) (ActionRawResult, error) {
	e.input = input
	if e.execute != nil {
		if err := e.execute(node, input); err != nil {
			return ActionRawResult{}, err
		}
	}
	return ActionRawResult{Status: "completed"}, nil
}

func (e *recordingActionExecutor) Harvest(
	_ context.Context,
	_ ActionRawResult,
	_ dsl.Node,
) (ActionOutput, error) {
	return ActionOutput{Structured: json.RawMessage(`{"ok":true}`)}, nil
}

func (r *coordinatorRunnerTaskRunReader) GetTaskRun(
	_ context.Context,
	id string,
) (task.Run, error) {
	if r.runs != nil {
		run, ok := r.runs[id]
		if !ok {
			return task.Run{}, task.ErrTaskRunNotFound
		}
		return run, nil
	}
	return r.run, nil
}

type coordinatorRunnerOutputs struct {
	outputs      map[int][]GenerationOutput
	payloads     map[GenerationOutputPayloadKey]json.RawMessage
	payloadCalls map[GenerationOutputPayloadKey]int
}

func (r coordinatorRunnerOutputs) GetGenerationOutputPayload(
	_ context.Context,
	key GenerationOutputPayloadKey,
) (json.RawMessage, error) {
	if r.payloadCalls != nil {
		r.payloadCalls[key]++
	}
	payload, ok := r.payloads[key]
	if !ok {
		return nil, ErrOutputRefNotFound
	}
	return cloneRawMessage(payload), nil
}

type coordinatorNodeControlReaderStub struct {
	controls []NodeControl
}

func (s coordinatorNodeControlReaderStub) ListNodeControls(
	context.Context,
	WorkspaceID,
	RunID,
) ([]NodeControl, error) {
	return append([]NodeControl(nil), s.controls...), nil
}

func (r coordinatorRunnerOutputs) ListGateVerdicts(
	context.Context,
	string,
	string,
	int64,
) ([]gate.VerdictRecord, error) {
	return []gate.VerdictRecord{}, nil
}

func (r coordinatorRunnerOutputs) ListRouteCausingVerdicts(
	context.Context,
	string,
	string,
	int64,
) ([]gate.VerdictRecord, error) {
	return []gate.VerdictRecord{}, nil
}

func (r coordinatorRunnerOutputs) ListGenerationOutputs(
	_ context.Context,
	_ WorkspaceID,
	_ RunID,
	generation int,
) ([]GenerationOutput, error) {
	if r.outputs == nil {
		return nil, nil
	}
	return append([]GenerationOutput(nil), r.outputs[generation]...), nil
}

type coordinatorRunnerLoopStore struct {
	run      Run
	runs     map[RunID]Run
	snapshot *DefinitionSnapshot
	getRun   func(RunID) (Run, error)
}

func (s *coordinatorRunnerLoopStore) CreateLoopRunForStart(
	context.Context,
	Run,
	dsl.ConcurrencyPolicy,
) (Run, error) {
	panic("CreateLoopRunForStart should not be called")
}

func (s *coordinatorRunnerLoopStore) GetLoopRun(context.Context, WorkspaceID, RunID) (Run, error) {
	panic("GetLoopRun should not be called")
}

func (s *coordinatorRunnerLoopStore) GetLoopRunByID(_ context.Context, runID RunID) (Run, error) {
	if s.getRun != nil {
		return s.getRun(runID)
	}
	if s.runs != nil {
		run, ok := s.runs[runID]
		if !ok {
			return Run{}, ErrRunNotFound
		}
		return run, nil
	}
	return s.run, nil
}

func (s *coordinatorRunnerLoopStore) GetLoopDefinitionSnapshot(
	_ context.Context,
	workspaceID WorkspaceID,
	digest string,
) (DefinitionSnapshot, error) {
	if s.snapshot != nil && s.snapshot.WorkspaceID == workspaceID && s.snapshot.Digest == digest {
		return *s.snapshot, nil
	}
	panic("GetLoopDefinitionSnapshot should not be called")
}

func (s *coordinatorRunnerLoopStore) FindActiveLoopRun(
	context.Context,
	WorkspaceID,
	string,
) (*Run, error) {
	panic("FindActiveLoopRun should not be called")
}

func (s *coordinatorRunnerLoopStore) CompareAndSwapLoopRunStatus(
	context.Context,
	RunID,
	Status,
	Status,
	TransitionCause,
	time.Time,
) error {
	panic("CompareAndSwapLoopRunStatus should not be called")
}

func (s *coordinatorRunnerLoopStore) RecordLoopGateDecisions(
	context.Context,
	[]GateDecisionRecord,
) error {
	panic("RecordLoopGateDecisions should not be called")
}

func (s *coordinatorRunnerLoopStore) ListLoopGateDecisions(
	context.Context,
	WorkspaceID,
	RunID,
	int,
	NodeID,
) (map[string]gate.HumanDecision, error) {
	return map[string]gate.HumanDecision{}, nil
}

func (s *coordinatorRunnerLoopStore) SetLoopRunPauseRequested(
	context.Context,
	WorkspaceID,
	RunID,
	bool,
	task.ActorContext,
) error {
	panic("SetLoopRunPauseRequested should not be called")
}

func (s *coordinatorRunnerLoopStore) UpsertLoopConfig(
	context.Context,
	WorkspaceID,
	string,
	LoopConfig,
) error {
	panic("UpsertLoopConfig should not be called")
}

func (s *coordinatorRunnerLoopStore) GetLoopConfig(
	context.Context,
	WorkspaceID,
	string,
) (*LoopConfig, error) {
	return nil, ErrConfigNotFound
}
