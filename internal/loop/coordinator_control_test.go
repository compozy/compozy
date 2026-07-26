package loop

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strconv"
	"testing"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/task"
)

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

				loopRun := controlLoopRun("looprun-branch-"+tc.name, map[string]any{"auto_push": tc.autoPush})
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
						{Generation: 1, NodeID: "should_push", Status: generationOutputPending},
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
		loopRun := controlLoopRun("looprun-branch-join", map[string]any{"auto_push": false})
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
				{Generation: 1, NodeID: "should_push", Status: generationOutputPending},
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
		loopRun := controlLoopRun("looprun-branch-tail", map[string]any{"auto_push": false})
		coordinatorRun := controlCoordinatorRun(loopRun, 1)
		loadRun := controlWorkerRun(loopRun, "load", 0, task.TaskRunStatusCompleted)
		runner := newCoordinatorRunnerForControlTest(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{coordinatorRun.ID: coordinatorRun, loadRun.ID: loadRun},
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
				{Generation: 1, NodeID: "load", Status: generationOutputEnqueued, TaskRunID: loadRun.ID},
				{Generation: 1, NodeID: "should_push", Status: generationOutputPending},
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

	t.Run("Should skip an unequal-length false branch diamond to a join", func(t *testing.T) {
		t.Parallel()

		resolved := compileCoordinatorControlDefinition(t, branchUnequalDiamondControlDefinition())
		loopRun := controlLoopRun("looprun-branch-diamond", map[string]any{"auto_push": false})
		coordinatorRun := controlCoordinatorRun(loopRun, 1)
		loadRun := controlWorkerRun(loopRun, "load", 0, task.TaskRunStatusCompleted)
		runner := newCoordinatorRunnerForControlTest(
			t,
			loopRun,
			coordinatorRun,
			map[string]task.Run{coordinatorRun.ID: coordinatorRun, loadRun.ID: loadRun},
			coordinatorRunnerOutputs{outputs: map[int][]GenerationOutput{1: {
				{Generation: 1, NodeID: "load", Status: generationOutputEnqueued, TaskRunID: loadRun.ID},
				{Generation: 1, NodeID: "should_push", Status: generationOutputPending},
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
			"auto_push": {Type: dsl.InputTypeBoolean},
		},
		Contract: dsl.Contract{Goal: "test", DefinitionOfDone: "done", IterationCap: 3},
		Graph: dsl.Graph{
			Nodes: []dsl.Node{
				{ID: "load", Class: dsl.NodeClassSource, Kind: string(dsl.SourceInput), InputRef: "auto_push"},
				{
					ID:        "should_push",
					Class:     dsl.NodeClassControl,
					Kind:      string(dsl.ControlBranch),
					Condition: "inputs.auto_push == true",
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
				{From: "load", To: "should_push"},
				{From: "should_push", To: "push"},
			},
		},
	}
}

func branchJoinControlDefinition() dsl.Definition {
	def := branchControlDefinition()
	def.Graph.Nodes = append(def.Graph.Nodes[:1], append([]dsl.Node{
		{ID: "independent", Class: dsl.NodeClassSource, Kind: string(dsl.SourceInput), InputRef: "auto_push"},
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
		{From: "load", To: "should_push"},
		{From: "should_push", To: "join"},
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
		{From: "load", To: "should_push"},
		{From: "should_push", To: "first"},
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
		{From: "load", To: "should_push"},
		{From: "should_push", To: "short"},
		{From: "short", To: "join"},
		{From: "should_push", To: "long_first"},
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
	if runs == nil {
		runs = map[string]task.Run{coordinatorRun.ID: coordinatorRun}
	}
	defaults := LoopDefaults{
		Delivery: definitionConfigLayer(resolved.Definition),
		Watch:    definitionConfigLayer(resolved.Definition),
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
