package loop

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/dsl/refs"
	"github.com/compozy/agh/internal/loop/gate"
	"github.com/compozy/agh/internal/task"
)

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

func TestCoordinatorActionExecutionInputShouldCarryPinnedGoalPolicy(t *testing.T) {
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
	})
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
			ID:          "looprun-awaiting-child-live-child",
			WorkspaceID: "ws-1",
			LoopName:    "child",
			Status:      StatusRunning,
			Generation:  1,
			CreatedAt:   time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC),
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
			ID:          "looprun-awaiting-child-terminal-child",
			WorkspaceID: "ws-1",
			LoopName:    "child",
			Status:      StatusDone,
			Generation:  1,
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
			ID:          "looprun-awaiting-child-timeout-child",
			WorkspaceID: "ws-1",
			LoopName:    "child",
			Status:      StatusRunning,
			Generation:  1,
			CreatedAt:   now,
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
		if got, want := next.Outputs[0].Status, generationOutputAwaitingChild; got != want {
			t.Fatalf("next output status = %q, want %q", got, want)
		}
		if got, want := next.Outputs[0].ChildLoopRunID, string(childRun.ID); got != want {
			t.Fatalf("next child_loop_run_id = %q, want %q", got, want)
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
	t.Run("Should start next generation while stop_when is false", func(t *testing.T) {
		t.Parallel()

		loopRun := Run{
			ID:           "looprun-stop-when-dirty",
			WorkspaceID:  "ws-1",
			LoopName:     "reviews-watch",
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
		if got, want := next["fetch_issues"].Status, generationOutputPending; got != want {
			t.Fatalf("next fetch_issues status = %q, want %q", got, want)
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
			LoopName:     "reviews-watch",
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
			Inputs: map[string]any{
				"verify_command": "",
			},
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
							Check:  "{{ .inputs.verify_command }}",
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

func TestCoordinatorRunnerShouldResumeSubLoopChildOnFailedOnlyRetry(t *testing.T) {
	t.Run("Should keep the child loop run for failed-only sub-loop retry", func(t *testing.T) {
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
		if got, want := child.Status, generationOutputAwaitingChild; got != want {
			t.Fatalf("child status = %q, want %q", got, want)
		}
		if got, want := child.ChildLoopRunID, "looprun-child-existing"; got != want {
			t.Fatalf("child_loop_run_id = %q, want %q", got, want)
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
		blockerRef := `{"blocking_issues":[{"id":"missing-reviewer"},{"id":"blocked-api"}]}`
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

func TestCoordinatorRunnerShouldTripCircuitBreakerAsStalled(t *testing.T) {
	t.Run("Should preserve a failing node streak across sibling success", func(t *testing.T) {
		t.Parallel()

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
		runner := newCoordinatorRunnerForTestWithDefinition(
			t,
			loopRun,
			coordinatorRun,
			nil,
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{
				1: {
					{Generation: 1, NodeID: "a_failing", Status: generationOutputFailed},
					{Generation: 1, NodeID: "z_healthy", Status: generationOutputSucceeded},
				},
				2: {
					{Generation: 2, NodeID: "a_failing", Status: generationOutputFailed},
					{Generation: 2, NodeID: "z_healthy", Status: generationOutputSucceeded},
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
		hooks := &coordinatorHookDispatcher{
			gatePre: func(
				context.Context,
				hookspkg.LoopGatePrePayload,
			) (hookspkg.LoopGatePrePayload, error) {
				return hookspkg.LoopGatePrePayload{
					Denied:     true,
					DenyReason: "human_gate",
				}, nil
			},
		}
		runner := newCoordinatorRunnerForTest(t, loopRun, coordinatorRun, nil, coordinatorRunnerOutputs{
			outputs: map[int][]GenerationOutput{1: {{
				Generation: 1,
				NodeID:     "load",
				Status:     generationOutputSucceeded,
			}, {
				Generation: 1,
				NodeID:     "agent",
				Status:     generationOutputSucceeded,
			}}},
		})
		runner.hooks = hooks

		plan, err := runner.Run(context.Background(), task.RunID(coordinatorRun.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if plan.Terminal == nil {
			t.Fatal("Terminal = nil, want blocked gate")
		}
		if got, want := plan.Terminal.Status, string(StatusBlocked); got != want {
			t.Fatalf("terminal status = %q, want %q", got, want)
		}
		if got, want := plan.Terminal.ReasonCode, "human_gate"; got != want {
			t.Fatalf("reason_code = %q, want %q", got, want)
		}
		if hooks.gatePreCalls != 1 || hooks.gatePostCalls != 1 {
			t.Fatalf(
				"gate hook calls = pre:%d post:%d, want pre:1 post:1",
				hooks.gatePreCalls,
				hooks.gatePostCalls,
			)
		}
		if got, want := hooks.lastGatePostStatus, string(StatusBlocked); got != want {
			t.Fatalf("gate post status = %q, want %q", got, want)
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

func newCoordinatorRunnerForStopWhenTest(
	t *testing.T,
	loopRun Run,
	coordinatorRun task.Run,
	outputRef string,
) *CoordinatorRunner {
	t.Helper()
	resolved := compileCoordinatorControlDefinition(t, dsl.Definition{
		Inputs: map[string]dsl.Input{
			"seed": {Type: dsl.InputTypeString},
		},
		Contract: dsl.Contract{
			StopWhen: "nodes.fetch_issues.status == 'succeeded' && size(nodes.fetch_issues.output.issues) == 0",
		},
		Graph: dsl.Graph{
			Nodes: []dsl.Node{
				{
					ID:       "fetch_issues",
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
			Edges: []dsl.Edge{{From: "fetch_issues", To: "verify_gate"}},
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
				NodeID:     "fetch_issues",
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
	outputs map[int][]GenerationOutput
}

func (r coordinatorRunnerOutputs) ListGenerationOutputs(
	_ context.Context,
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
