package loop

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
)

func TestRosterContract(t *testing.T) {
	t.Parallel()
	t.Run("Should satisfy UT-014 and UT-048 with complete current-state rows and links", func(t *testing.T) {
		t.Parallel()
		now := time.Now().UTC()
		source := rosterFixture(now)
		page, err := ProjectRoster(&source, RosterQuery{State: NodeStateFilterAll})
		if err != nil {
			t.Fatalf("ProjectRoster() error = %v", err)
		}
		if len(page.Nodes) != 3 {
			t.Fatalf("nodes = %d, want 3", len(page.Nodes))
		}
		if page.Nodes[0].State != NodeStateSucceeded || page.Nodes[1].State != NodeStateRunning ||
			page.Nodes[2].State != NodeStatePending {
			t.Fatalf("states = %#v", page.Nodes)
		}
		if page.Nodes[1].SessionID != "session-1" || page.Nodes[1].CellTaskID == "" ||
			len(page.Nodes[1].Attempts) != 1 {
			t.Fatalf("running links/history = %#v", page.Nodes[1])
		}
	})
	t.Run("Should project a live task run as running before the output refreshes", func(t *testing.T) {
		t.Parallel()
		source := rosterFixture(time.Now().UTC())
		source.Outputs[1].Status = generationOutputEnqueued
		source.Outputs[1].TaskRunStatus = task.TaskRunStatusRunning
		source.Attempts = nil

		page, err := ProjectRoster(&source, RosterQuery{State: NodeStateFilterAll})
		if err != nil {
			t.Fatalf("ProjectRoster() error = %v", err)
		}
		live := rosterNodesByIdentity(page.Nodes)[rosterKey(1, "live", 0)]
		if live.State != NodeStateRunning {
			t.Fatalf("live-task state = %q, want %q", live.State, NodeStateRunning)
		}
	})
	t.Run("Should keep a dispatched node live until its completed task run reaches the output", func(t *testing.T) {
		t.Parallel()
		source := rosterFixture(time.Now().UTC())
		source.Outputs[1].Status = generationOutputEnqueued
		source.Outputs[1].TaskRunStatus = task.TaskRunStatusCompleted
		source.Attempts = nil

		page, err := ProjectRoster(&source, RosterQuery{State: NodeStateFilterAll})
		if err != nil {
			t.Fatalf("ProjectRoster() error = %v", err)
		}
		dispatched := rosterNodesByIdentity(page.Nodes)[rosterKey(1, "live", 0)]
		if dispatched.State != NodeStateRunning {
			t.Fatalf("dispatched-task state = %q, want %q", dispatched.State, NodeStateRunning)
		}
	})
	t.Run("Should project retained task-run usage on the owning row", func(t *testing.T) {
		t.Parallel()
		source := rosterFixture(time.Now().UTC())
		source.Outputs[0].TaskRunTokensUsed = 22

		page, err := ProjectRoster(&source, RosterQuery{State: NodeStateFilterAll})
		if err != nil {
			t.Fatalf("ProjectRoster() error = %v", err)
		}
		row := rosterNodesByIdentity(page.Nodes)[rosterKey(1, "done", 0)]
		if row.Usage == nil || row.Usage.Tokens != 22 {
			t.Fatalf("row usage = %#v, want 22 retained tokens", row.Usage)
		}
	})
	t.Run("Should satisfy UT-014 with a total persisted-state mapping", func(t *testing.T) {
		t.Parallel()
		cases := map[string]NodeState{
			"pending":         NodeStatePending,
			"enqueued":        NodeStateQueued,
			"running":         NodeStateRunning,
			"retrying":        NodeStateRetrying,
			"waiting":         NodeStateWaiting,
			"paused":          NodeStatePaused,
			"awaiting_child":  NodeStateAwaitingChild,
			"control_pending": NodeStateControlPending,
			"awaiting_goal":   NodeStateAwaitingGoal,
			"succeeded":       NodeStateSucceeded,
			"partial":         NodeStatePartial,
			"failed":          NodeStateFailed,
			"canceled":        NodeStateCanceled,
			"quarantined":     NodeStateQuarantined,
		}
		for source, want := range cases {
			got, err := mapOutputState(source)
			if err != nil || got != want {
				t.Fatalf("mapOutputState(%q) = %q, %v; want %q", source, got, err, want)
			}
		}
		if _, err := mapOutputState("unknown"); err == nil {
			t.Fatal("mapOutputState(unknown) error = nil")
		}
	})
	t.Run("Should satisfy UT-015 by excluding control and not-taken nodes from progress", func(t *testing.T) {
		t.Parallel()
		roster := RosterPage{Nodes: []RosterNode{
			{Generation: 1, Action: true, State: NodeStateSucceeded},
			{Generation: 1, Action: true, State: NodeStatePartial},
			{Generation: 1, Action: true, State: NodeStateFailed},
			{Generation: 1, Action: true, State: NodeStateCanceled},
			{Generation: 1, Action: true, State: NodeStateRunning},
			{Generation: 1, Action: true, State: NodeStateNotTaken},
			{Generation: 1, Action: false, State: NodeStateSucceeded},
		}}
		got := ProgressFromRoster(roster, 1)
		if got.StepsDone != 4 || got.StepsTotal != 5 {
			t.Fatalf("progress = %#v", got)
		}
	})
	t.Run("Should satisfy UT-016 by deriving not_taken only from durable route evidence", func(t *testing.T) {
		t.Parallel()
		source := rosterFixture(time.Now())
		source.Graph.Nodes[0] = dsl.Node{
			ID:     "route",
			Class:  dsl.NodeClassControl,
			Kind:   string(dsl.ControlRoute),
			Routes: []dsl.RouteSpec{{To: "selected"}, {To: "skipped"}},
		}
		source.Graph.Nodes = append(source.Graph.Nodes, dsl.Node{ID: "skipped", Class: dsl.NodeClassAction})
		source.RouteCauses = []RouteCause{
			{Generation: 1, NodeID: "route", Route: "selected", Cause: "predicate"},
		}
		page, err := ProjectRoster(&source, RosterQuery{})
		if err != nil {
			t.Fatalf("ProjectRoster() error = %v", err)
		}
		found := false
		for _, node := range page.Nodes {
			if node.NodeID == "skipped" {
				found = true
				if node.State != NodeStateNotTaken {
					t.Fatalf("skipped state = %q", node.State)
				}
			}
		}
		if !found {
			t.Fatal("skipped node missing")
		}

		fanout := RosterSource{
			Run: Run{ID: "run-fanout", Generation: 1},
			Graph: dsl.Graph{Nodes: []dsl.Node{
				{
					ID: "route", Class: dsl.NodeClassControl, Kind: string(dsl.ControlRoute),
					Routes: []dsl.RouteSpec{{To: "a"}, {To: "b"}},
				},
				{ID: "a", Class: dsl.NodeClassAction},
				{ID: "b", Class: dsl.NodeClassAction},
			}},
			RouteCauses: []RouteCause{
				{Generation: 1, NodeID: "route", ItemIndex: 0, Route: "a", Cause: "predicate"},
				{Generation: 1, NodeID: "route", ItemIndex: 1, Route: "b", Cause: "predicate"},
			},
		}
		for _, nodeID := range []string{"a", "b"} {
			for itemIndex := range 2 {
				fanout.Outputs = append(fanout.Outputs, GenerationOutput{
					Generation: 1, NodeID: nodeID, ItemIndex: itemIndex, Status: generationOutputSucceeded,
				})
			}
		}
		fanoutPage, err := ProjectRoster(&fanout, RosterQuery{})
		if err != nil {
			t.Fatalf("ProjectRoster(fanout) error = %v", err)
		}
		fanoutNodes := rosterNodesByIdentity(fanoutPage.Nodes)
		if fanoutNodes[rosterKey(1, "a", 0)].State != NodeStateSucceeded ||
			fanoutNodes[rosterKey(1, "a", 1)].State != NodeStateNotTaken ||
			fanoutNodes[rosterKey(1, "b", 0)].State != NodeStateNotTaken ||
			fanoutNodes[rosterKey(1, "b", 1)].State != NodeStateSucceeded {
			t.Fatalf("fanout route states = %#v", fanoutNodes)
		}
	})
	t.Run("Should keep output-less nodes pending without durable not-taken evidence", func(t *testing.T) {
		t.Parallel()
		statuses := []Status{
			StatusDone,
			StatusNoOp,
			StatusBlocked,
			StatusFailed,
			StatusExhausted,
			StatusStalled,
			StatusCanceled,
		}
		for _, status := range statuses {
			t.Run("Should preserve pending state for "+string(status), func(t *testing.T) {
				t.Parallel()
				source := RosterSource{
					Run: Run{ID: "run-pending", Status: status, Generation: 1},
					Graph: dsl.Graph{Nodes: []dsl.Node{{
						ID: "unfinished", Class: dsl.NodeClassAction,
					}}},
				}
				page, err := ProjectRoster(&source, RosterQuery{})
				if err != nil {
					t.Fatalf("ProjectRoster() error = %v", err)
				}
				if len(page.Nodes) != 1 || page.Nodes[0].State != NodeStatePending {
					t.Fatalf("nodes = %#v, want one pending node", page.Nodes)
				}
			})
		}
	})
	t.Run("Should project durable branch skips in their generation without inflating progress", func(t *testing.T) {
		t.Parallel()
		source := RosterSource{
			Run: Run{ID: "run-branch", Status: StatusDone, Generation: 5},
			Graph: dsl.Graph{Nodes: []dsl.Node{
				{ID: "review", Class: dsl.NodeClassAction},
				{ID: "fix", Class: dsl.NodeClassAction},
			}},
			Generations: []LoopGeneration{{Generation: 1}, {Generation: 5}},
			Outputs: []GenerationOutput{
				{Generation: 1, NodeID: "review", Status: generationOutputSucceeded},
				{Generation: 1, NodeID: "fix", Status: generationOutputSucceeded},
				{Generation: 5, NodeID: "review", Status: generationOutputSucceeded},
				{
					Generation: 5,
					NodeID:     "fix",
					Status:     generationOutputSucceeded,
					OutputRef:  branchSkippedOutputRef,
				},
			},
		}

		page, err := ProjectRoster(&source, RosterQuery{})
		if err != nil {
			t.Fatalf("ProjectRoster() error = %v", err)
		}
		nodes := rosterNodesByIdentity(page.Nodes)
		if nodes[rosterKey(1, "fix", 0)].State != NodeStateSucceeded ||
			nodes[rosterKey(5, "fix", 0)].State != NodeStateNotTaken {
			t.Fatalf("branch states = %#v", nodes)
		}
		progress := ProgressFromRoster(page, 5)
		if progress.StepsDone != 1 || progress.StepsTotal != 1 {
			t.Fatalf("progress = %#v, want 1 of 1", progress)
		}
	})
	t.Run("Should satisfy UT-017 with fanout rollups and stable pagination", func(t *testing.T) {
		t.Parallel()
		source := RosterSource{
			Run: Run{ID: "r", LoopName: "fan", Generation: 1},
			Graph: dsl.Graph{
				Nodes: []dsl.Node{{ID: "fan", Class: dsl.NodeClassAction}},
			},
		}
		for i := range 100 {
			source.Outputs = append(source.Outputs, GenerationOutput{
				Generation: 1,
				NodeID:     "fan",
				ItemIndex:  i,
				Status:     generationOutputSucceeded,
			})
		}
		page, err := ProjectRoster(&source, RosterQuery{Limit: 10})
		if err != nil {
			t.Fatalf("ProjectRoster() error = %v", err)
		}
		if len(page.Nodes) != 10 || page.NextCursor == "" || len(page.FanoutRollups) != 1 ||
			page.FanoutRollups[0].Total != 100 {
			t.Fatalf("page = %#v", page)
		}
	})
	t.Run("Should preserve only materialized fanout item indexes", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name        string
			outputs     []GenerationOutput
			wantIndexes []int
			wantRollups []FanoutRollup
		}{
			{
				name: "Should preserve a lone nonzero worker index",
				outputs: []GenerationOutput{
					{Generation: 1, NodeID: "worker", ItemIndex: 2, Status: generationOutputSucceeded},
				},
				wantIndexes: []int{2},
				wantRollups: []FanoutRollup{},
			},
			{
				name: "Should sort sparse worker indexes without filling gaps",
				outputs: []GenerationOutput{
					{Generation: 1, NodeID: "worker", ItemIndex: 5, Status: generationOutputSucceeded},
					{Generation: 1, NodeID: "worker", ItemIndex: 2, Status: generationOutputSucceeded},
				},
				wantIndexes: []int{2, 5},
				wantRollups: []FanoutRollup{{Generation: 1, NodeID: "fan", Done: 2, Total: 2}},
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				source := RosterSource{
					Run: Run{ID: "r", LoopName: "fan", Generation: 1},
					Graph: dsl.Graph{
						Nodes: []dsl.Node{
							{ID: "fan", Class: dsl.NodeClassControl, Kind: string(dsl.ControlFanOut)},
							{ID: "worker", Class: dsl.NodeClassAction},
						},
						Edges: []dsl.Edge{{From: "fan", To: "worker"}},
					},
					Outputs: test.outputs,
				}

				page, err := ProjectRoster(&source, RosterQuery{})
				if err != nil {
					t.Fatalf("ProjectRoster() error = %v", err)
				}
				gotIndexes := make([]int, 0, len(test.wantIndexes))
				for _, node := range page.Nodes {
					if node.NodeID == "worker" {
						gotIndexes = append(gotIndexes, node.ItemIndex)
					}
				}
				if !slices.Equal(gotIndexes, test.wantIndexes) {
					t.Fatalf("worker item indexes = %v, want %v", gotIndexes, test.wantIndexes)
				}
				if !slices.Equal(page.FanoutRollups, test.wantRollups) {
					t.Fatalf("fanout rollups = %#v, want %#v", page.FanoutRollups, test.wantRollups)
				}
				progress := ProgressFromRoster(page, 1)
				if progress.StepsDone != len(test.wantIndexes) || progress.StepsTotal != len(test.wantIndexes) {
					t.Fatalf("progress = %#v, want %d/%d", progress, len(test.wantIndexes), len(test.wantIndexes))
				}
			})
		}
	})
	t.Run("Should name a fanout rollup after its authored container", func(t *testing.T) {
		t.Parallel()
		source := RosterSource{
			Run: Run{ID: "r", LoopName: "fan", Generation: 1},
			Graph: dsl.Graph{
				Nodes: []dsl.Node{
					{ID: "source", Class: dsl.NodeClassAction},
					{ID: "revisores", Class: dsl.NodeClassControl, Kind: string(dsl.ControlFanOut)},
					{ID: "revisar", Class: dsl.NodeClassAction},
					{ID: "join", Class: dsl.NodeClassControl},
				},
				Edges: []dsl.Edge{
					{From: "source", To: "revisores"},
					{From: "revisores", To: "revisar"},
					{From: "revisar", To: "join"},
				},
			},
		}
		for itemIndex := range 3 {
			source.Outputs = append(source.Outputs, GenerationOutput{
				Generation: 1, NodeID: "revisar", ItemIndex: itemIndex, Status: generationOutputSucceeded,
			})
		}

		page, err := ProjectRoster(&source, RosterQuery{})
		if err != nil {
			t.Fatalf("ProjectRoster() error = %v", err)
		}
		if len(page.FanoutRollups) != 1 || page.FanoutRollups[0].NodeID != "revisores" {
			t.Fatalf("fanout rollups = %#v, want authored container revisores", page.FanoutRollups)
		}
	})
	t.Run("Should satisfy UT-018 with distinct strategy and operator cancellation", func(t *testing.T) {
		t.Parallel()
		source := RosterSource{
			Run: Run{ID: "r", Generation: 1},
			Graph: dsl.Graph{Nodes: []dsl.Node{
				{ID: "op", Class: dsl.NodeClassAction},
				{ID: "strategy", Class: dsl.NodeClassAction},
				{ID: "never-started", Class: dsl.NodeClassAction},
			}},
			Outputs: []GenerationOutput{
				{Generation: 1, NodeID: "op", Status: generationOutputCanceled, OutputRef: strategyCanceledReasonCode},
				{
					Generation: 1, NodeID: "strategy", Status: generationOutputCanceled,
					OutputRef: strategyCanceledReasonCode,
				},
				{
					Generation: 1, NodeID: "never-started", Status: generationOutputCanceled,
					OutputRef: strategyNeverStartedReasonCode,
				},
			},
			Controls: []NodeControl{{
				NodeID:      "op",
				CancelState: CancelStateCanceled,
				CancelProvenance: &ControlProvenance{
					ActorKind: "human",
					ActorID:   "pedro",
					Reason:    "stop",
				},
			}},
		}
		page, err := ProjectRoster(&source, RosterQuery{})
		if err != nil {
			t.Fatalf("ProjectRoster() error = %v", err)
		}
		if len(page.Nodes) != 3 {
			t.Fatalf("nodes = %d, want 3", len(page.Nodes))
		}
		nodes := rosterNodesByIdentity(page.Nodes)
		operator := nodes[rosterKey(1, "op", 0)].Cancellation
		if operator == nil || operator.Disposition != nodeCancellationOperator ||
			operator.ActorKind != "human" || operator.ActorRef != "pedro" || operator.Cause != "stop" {
			t.Fatalf("operator cancellation = %#v", operator)
		}
		strategy := nodes[rosterKey(1, "strategy", 0)].Cancellation
		if strategy == nil || strategy.Disposition != nodeCancellationStrategy ||
			strategy.Cause != strategyCanceledReasonCode {
			t.Fatalf("strategy cancellation = %#v", strategy)
		}
		neverStarted := nodes[rosterKey(1, "never-started", 0)].Cancellation
		if neverStarted == nil || neverStarted.Disposition != nodeCancellationStrategy ||
			neverStarted.Cause != strategyNeverStartedReasonCode {
			t.Fatalf("never-started cancellation = %#v", neverStarted)
		}
	})
	t.Run("Should preserve item indexes from branch-pruned evidence", func(t *testing.T) {
		t.Parallel()
		payload := json.RawMessage(`{"generation":1,"node_id":"fan","item_indexes":[1,3]}`)
		source := RosterSource{Run: Run{Status: StatusRunning}}
		err := applyRunReadEvidence(&source, []RunEvent{{Kind: string(RunEventBranchPruned), Payload: payload}})
		if err != nil {
			t.Fatalf("applyRunReadEvidence() error = %v", err)
		}
		if source.PrunedNodes[rosterKey(1, "fan", 0)] ||
			!source.PrunedNodes[rosterKey(1, "fan", 1)] ||
			!source.PrunedNodes[rosterKey(1, "fan", 3)] {
			t.Fatalf("pruned nodes = %#v", source.PrunedNodes)
		}
	})
	t.Run("Should reject a negative branch-pruned item identity without mutation", func(t *testing.T) {
		t.Parallel()

		source := RosterSource{}
		err := source.MarkPrunedNodeItem(1, "fan", -1)
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("MarkPrunedNodeItem() error = %v, want ErrValidation", err)
		}
		if len(source.PrunedNodes) != 0 {
			t.Fatalf("pruned nodes = %#v, want no mutation", source.PrunedNodes)
		}
	})
	t.Run("Should satisfy UT-019 with every durable generation and no invented score", func(t *testing.T) {
		t.Parallel()
		source := rosterFixture(time.Now())
		source.Generations = []LoopGeneration{{Generation: 1}, {Generation: 2}}
		page, err := ProjectRoster(&source, RosterQuery{})
		if err != nil {
			t.Fatalf("ProjectRoster() error = %v", err)
		}
		seen := map[int]bool{}
		for _, node := range page.Nodes {
			seen[node.Generation] = true
		}
		if !seen[1] || !seen[2] {
			t.Fatalf("generations = %#v", seen)
		}
	})
	t.Run("Should satisfy UT-020 for a terminal run before round one", func(t *testing.T) {
		t.Parallel()
		page, err := ProjectRoster(&RosterSource{Run: Run{ID: "r", Status: StatusFailed}}, RosterQuery{})
		if err != nil {
			t.Fatalf("ProjectRoster() error = %v", err)
		}
		if len(page.Nodes) != 0 || page.RunStatus != StatusFailed {
			t.Fatalf("page = %#v", page)
		}
	})
	t.Run("Should satisfy UT-050 with the exact public state filter vocabulary", func(t *testing.T) {
		t.Parallel()
		want := "all|running|queued|waiting|retrying|paused|quarantined|succeeded|failed|canceled|not_taken"
		if strings.Join(NodeStateFilterValues(), "|") != want {
			t.Fatalf("filters = %q", strings.Join(NodeStateFilterValues(), "|"))
		}
		_, err := ProjectRoster(&RosterSource{}, RosterQuery{State: "pending"})
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("pending filter error = %v", err)
		}
	})
	t.Run("Should retain branch pruning evidence beyond one SQL event page", func(t *testing.T) {
		t.Parallel()
		events := make([]RunEvent, 0, 501)
		events = append(events, RunEvent{
			LoopRunID: "run-a",
			Seq:       1,
			Kind:      string(RunEventBranchPruned),
		})
		for sequence := int64(2); sequence <= 501; sequence++ {
			events = append(events, RunEvent{
				LoopRunID: "run-a",
				Seq:       sequence,
				Kind:      string(RunEventTokenTick),
			})
		}
		store := &pagedRouteEvidenceStore{events: events}
		service := &computedRunReadService{store: store}

		got, err := service.loadRouteEvidence(context.Background(), "ws", "run-a")
		if err != nil {
			t.Fatalf("loadRouteEvidence() error = %v", err)
		}
		if len(got) != 501 || got[len(got)-1].Kind != string(RunEventBranchPruned) {
			t.Fatalf("route evidence = %#v", got)
		}
		wantQueries := []RunEventBackwardQuery{
			{WorkspaceID: "ws", RunID: "run-a", FixedHeadSeq: 501, BeforeSeq: 502, Limit: 500},
			{WorkspaceID: "ws", RunID: "run-a", FixedHeadSeq: 501, BeforeSeq: 2, Limit: 500},
		}
		if !slices.Equal(store.queries, wantQueries) {
			t.Fatalf("backward queries = %#v, want %#v", store.queries, wantQueries)
		}
	})
}

func rosterNodesByIdentity(nodes []RosterNode) map[string]RosterNode {
	indexed := make(map[string]RosterNode, len(nodes))
	for _, node := range nodes {
		indexed[rosterKey(node.Generation, node.NodeID, node.ItemIndex)] = node
	}
	return indexed
}

type pagedRouteEvidenceStore struct {
	RunReadStore
	events  []RunEvent
	queries []RunEventBackwardQuery
}

func (s *pagedRouteEvidenceStore) GetLoopRun(
	_ context.Context,
	workspaceID WorkspaceID,
	runID RunID,
) (Run, error) {
	return Run{ID: runID, WorkspaceID: workspaceID}, nil
}

func (s *pagedRouteEvidenceStore) GetLoopRunEventHead(
	context.Context,
	WorkspaceID,
	RunID,
) (int64, error) {
	return int64(len(s.events)), nil
}

func (s *pagedRouteEvidenceStore) ListLoopRunEventsBackward(
	_ context.Context,
	query RunEventBackwardQuery,
) ([]RunEvent, error) {
	s.queries = append(s.queries, query)
	page := make([]RunEvent, 0, query.Limit)
	for _, event := range slices.Backward(s.events) {
		if event.Seq > query.FixedHeadSeq || event.Seq >= query.BeforeSeq {
			continue
		}
		page = append(page, event)
		if len(page) == query.Limit {
			break
		}
	}
	return page, nil
}

func rosterFixture(now time.Time) RosterSource {
	return RosterSource{
		Run: Run{
			ID:         "run-a",
			LoopName:   "demo",
			Status:     StatusRunning,
			Generation: 1,
			StartedBy:  task.ActorIdentity{},
		},
		Graph: dsl.Graph{Nodes: []dsl.Node{
			{ID: "done", Class: dsl.NodeClassAction},
			{ID: "live", Class: dsl.NodeClassAction},
			{ID: "later", Class: dsl.NodeClassAction},
		}},
		Outputs: []GenerationOutput{
			{Generation: 1, NodeID: "done", Status: "succeeded", Attempt: 1},
			{
				Generation: 1,
				NodeID:     "live",
				Status:     "running",
				Attempt:    1,
				SessionID:  "session-1",
				TaskRunID:  "task-run-1",
			},
		},
		Attempts: []NodeAttempt{{
			LoopRunID:   "run-a",
			Generation:  1,
			NodeID:      "live",
			Attempt:     1,
			Disposition: AttemptResumed,
			StartedAt:   now,
		}},
	}
}
