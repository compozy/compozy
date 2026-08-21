package loop

import (
	"context"
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
		operator := page.Nodes[0].Cancellation
		if operator == nil || operator.Disposition != nodeCancellationOperator ||
			operator.ActorKind != "human" || operator.ActorRef != "pedro" || operator.Cause != "stop" {
			t.Fatalf("operator cancellation = %#v", operator)
		}
		strategy := page.Nodes[1].Cancellation
		if strategy == nil || strategy.Disposition != nodeCancellationStrategy ||
			strategy.Cause != strategyCanceledReasonCode {
			t.Fatalf("strategy cancellation = %#v", strategy)
		}
		neverStarted := page.Nodes[2].Cancellation
		if neverStarted == nil || neverStarted.Disposition != nodeCancellationStrategy ||
			neverStarted.Cause != strategyNeverStartedReasonCode {
			t.Fatalf("never-started cancellation = %#v", neverStarted)
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
	})
}

type pagedRouteEvidenceStore struct {
	RunReadStore
	events []RunEvent
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
	_ WorkspaceID,
	_ RunID,
	head int64,
	before int64,
	limit int,
) ([]RunEvent, error) {
	page := make([]RunEvent, 0, limit)
	for _, event := range slices.Backward(s.events) {
		if event.Seq > head || event.Seq >= before {
			continue
		}
		page = append(page, event)
		if len(page) == limit {
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
