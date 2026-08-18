package loop

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
)

// Invariant: a fan-out window materializes the lowest pending indexes exactly once and never exceeds max_parallel.
func TestNextFanOutWindowIndexesShouldAdvanceDeterministically(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name  string
		state fanOutWindowState
		want  []int
	}{
		{
			name:  "Should materialize the first bounded window",
			state: fanOutWindowState{Total: 5, MaxParallel: 2},
			want:  []int{0, 1},
		},
		{
			name: "Should advance one lane after one settles",
			state: fanOutWindowState{Total: 5, MaxParallel: 2,
				Materialized: map[int]bool{0: true, 1: true}, Settled: map[int]bool{0: true}},
			want: []int{2},
		},
		{
			name:  "Should materialize all lanes when the window covers the collection",
			state: fanOutWindowState{Total: 3, MaxParallel: 8},
			want:  []int{0, 1, 2},
		},
		{
			name: "Should not recreate materialized lanes after restart",
			state: fanOutWindowState{Total: 3, MaxParallel: 2,
				Materialized: map[int]bool{0: true, 1: true}},
			want: []int{},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := nextFanOutWindowIndexes(tt.state); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("nextFanOutWindowIndexes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMaterializeFanOutWindowShouldReformFromDurableOutputs(t *testing.T) {
	t.Parallel()

	graph := fanOutWindowGraph()
	topology := newControlTopology(graph)
	outputs := []GenerationOutput{{Generation: 1, NodeID: "work", ItemIndex: 0, Status: generationOutputSucceeded}}
	materialization := fanOutMaterialization{Branches: 4, MaxParallel: 2}
	if changed := materializeFanOutWindow(graph, topology, 1, "fan", materialization, &outputs); !changed {
		t.Fatal("materializeFanOutWindow() changed = false, want true")
	}
	if got := outputItemIndexesForNode(outputs, "work"); !reflect.DeepEqual(got, []int{0, 1, 2}) {
		t.Fatalf("materialized work indexes = %v, want [0 1 2]", got)
	}
	if changed := materializeFanOutWindow(graph, topology, 1, "fan", materialization, &outputs); changed {
		t.Fatal("second materializeFanOutWindow() changed = true, want restart-safe no-op")
	}
}

func TestApplyStrategyLaneCancellationsShouldRecordNeverStartedLanes(t *testing.T) {
	t.Parallel()

	graph := fanOutWindowGraph()
	resolved := &ResolvedDefinition{Definition: dsl.Definition{Graph: graph}}
	eval := &controlEvalContext{
		run: Run{ID: "run-wide"}, generation: 1, resolved: resolved,
		topology: newControlTopology(graph), now: time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC),
	}
	outputs := []GenerationOutput{{
		Generation: 1, NodeID: "work", ItemIndex: 0, Status: generationOutputRunning,
		SessionID: "session-0", Epoch: 3,
	}}
	plan := task.CoordinatorCompletionPlan{Snapshot: task.GenerationSnapshot{Payload: GenerationSnapshotPayload{}}}
	if err := applyStrategyLaneCancellations(eval, &plan, "fan", []int{0, 1, 2}, &outputs); err != nil {
		t.Fatalf("applyStrategyLaneCancellations() error = %v", err)
	}
	byItem := generationOutputMap(outputs)
	if got := byItem[generationOutputKey{nodeID: "work", itemIndex: 0}]; got.Status != generationOutputCanceled ||
		got.OutputRef != strategyCanceledReasonCode {
		t.Fatalf("materialized lane = %#v, want strategy cancellation", got)
	}
	for _, itemIndex := range []int{1, 2} {
		got := byItem[generationOutputKey{nodeID: "work", itemIndex: itemIndex}]
		if got.Status != generationOutputCanceled || got.OutputRef != strategyNeverStartedReasonCode ||
			got.TaskRunID != "" {
			t.Fatalf("never-started lane %d = %#v", itemIndex, got)
		}
	}
	if got, want := len(plan.LaneCancels), 1; got != want {
		t.Fatalf("post-commit lane cancels = %d, want %d", got, want)
	}
	payload, err := GenerationSnapshotPayloadFrom(plan.Snapshot.Payload)
	if err != nil {
		t.Fatalf("GenerationSnapshotPayloadFrom() error = %v", err)
	}
	if got, want := len(payload.Events), 2; got != want {
		t.Fatalf("branch-pruned events = %d, want %d causes", got, want)
	}
}

func TestBranchPrunedEventIntentsShouldStayBounded(t *testing.T) {
	t.Parallel()

	indexes := make([]int, 20_000)
	for index := range indexes {
		indexes[index] = index
	}
	intents := branchPrunedEventIntents("fan", indexes, strategyNeverStartedReasonCode)
	if len(intents) < 2 {
		t.Fatalf("branchPrunedEventIntents() chunks = %d, want multiple bounded chunks", len(intents))
	}
	seen := 0
	for _, intent := range intents {
		encoded, err := json.Marshal(intent)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		if len(encoded) > branchPrunedIntentPayloadLimit {
			t.Fatalf("encoded branch-pruned intent = %d bytes, limit %d", len(encoded), branchPrunedIntentPayloadLimit)
		}
		seen += len(intent.ItemIndexes)
	}
	if seen != len(indexes) {
		t.Fatalf("bounded intents cover %d indexes, want %d", seen, len(indexes))
	}
}

func fanOutWindowGraph() dsl.Graph {
	return dsl.Graph{
		Nodes: []dsl.Node{
			{ID: "fan", Class: dsl.NodeClassControl, Kind: string(dsl.ControlFanOut)},
			{ID: "work", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent)},
			{ID: "collect", Class: dsl.NodeClassControl, Kind: string(dsl.ControlCollect)},
		},
		Edges: []dsl.Edge{{From: "fan", To: "work"}, {From: "work", To: "collect"}},
	}
}

func outputItemIndexesForNode(outputs []GenerationOutput, nodeID string) []int {
	items := make([]int, 0)
	for _, output := range sortedGenerationOutputs(outputs) {
		if output.NodeID == nodeID {
			items = append(items, output.ItemIndex)
		}
	}
	return items
}
