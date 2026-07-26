package daemon

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	watchpkg "github.com/compozy/agh/internal/loop/watch"
)

const loopWatchEventsTestWatchNodeID = "watch"

func TestLoopWatchEventKindContractParity(t *testing.T) {
	t.Run("Should mirror the supported watch-events family registry exactly", func(t *testing.T) {
		t.Parallel()
		want := make([]string, 0)
		for kind := range looppkg.SupportedWatchEvents() {
			want = append(want, string(kind))
		}
		slices.Sort(want)
		got := slices.Clone(contract.LoopWatchEventKindValues())
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Fatalf("contract watch-event kinds = %#v, want %#v", got, want)
		}
	})
}

func watchEventsDefinition(nodeID, kind string) *contract.LoopDefinitionDocument {
	return &contract.LoopDefinitionDocument{
		Graph: contract.LoopGraph{
			Nodes: []contract.LoopGraphNode{
				{ID: nodeID, Class: contract.LoopNodeClassSource, Kind: kind},
			},
		},
	}
}

func TestLoopWatchEventsReadModel(t *testing.T) {
	t.Run("Should project a parked node from the hydrated executed-definition envelope", func(t *testing.T) {
		t.Parallel()

		resolved := schedulerWatchEventsDefinitionForTest(t)
		effective, err := looppkg.ResolveEffectiveConfig(
			resolved,
			looppkg.DefaultLoopDefaults(),
			nil,
			looppkg.LoopConfig{},
		)
		if err != nil {
			t.Fatalf("ResolveEffectiveConfig() error = %v", err)
		}
		snapshot, digest, err := looppkg.BuildExecutedDefinitionSnapshot(resolved, effective)
		if err != nil {
			t.Fatalf("BuildExecutedDefinitionSnapshot() error = %v", err)
		}
		hydrated, err := looppkg.LoadExecutedDefinitionSnapshot(snapshot, digest)
		if err != nil {
			t.Fatalf("LoadExecutedDefinitionSnapshot() error = %v", err)
		}
		hydratedJSON, err := json.Marshal(hydrated.Definition)
		if err != nil {
			t.Fatalf("json.Marshal(hydrated definition) error = %v", err)
		}
		document, err := loopDefinitionDocumentFromJSON(hydratedJSON)
		if err != nil {
			t.Fatalf("loopDefinitionDocumentFromJSON() error = %v", err)
		}
		ref, err := watchpkg.EventsPendingOutputRef(watchpkg.EventsPendingState{
			Subscriptions: []watchpkg.EventSubscriptionRef{{
				Kind:   string(contract.LoopWatchEventTaskBlocked),
				Filter: `event.task_id == inputs.target_task_id`,
			}},
			Cursors: map[string]int64{looppkg.WatchEventsTaskStream: 7},
		})
		if err != nil {
			t.Fatalf("EventsPendingOutputRef() error = %v", err)
		}
		state, err := loopWatchEventsReadModel(
			looppkg.Run{Generation: 1},
			&document,
			[]contract.LoopGenerationPayload{{
				Generation: 1,
				Outputs: []contract.LoopGenerationOutput{{
					NodeID:    "watch_tasks",
					Status:    daemonRuntimeStatusRunning,
					OutputRef: ref,
				}},
			}},
		)
		if err != nil {
			t.Fatalf("loopWatchEventsReadModel() error = %v", err)
		}
		if state == nil || len(state.Subscriptions) != 1 ||
			state.Subscriptions[0].Kind != contract.LoopWatchEventTaskBlocked {
			t.Fatalf("hydrated watch-events state = %#v, want task.blocked subscription", state)
		}
		if got, want := state.Cursors[looppkg.WatchEventsTaskStream], int64(7); got != want {
			t.Fatalf("hydrated task cursor = %d, want %d", got, want)
		}
	})

	t.Run("Should project subscriptions, cursors, and last_wake_at when parked", func(t *testing.T) {
		t.Parallel()
		ref, err := watchpkg.EventsPendingOutputRef(watchpkg.EventsPendingState{
			Subscriptions: []watchpkg.EventSubscriptionRef{
				{
					Kind:   string(contract.LoopWatchEventTaskStatusChanged),
					Filter: "event.payload.to_status == 'completed'",
				},
			},
			Cursors: map[string]int64{looppkg.WatchEventsTaskStream: 42},
		})
		if err != nil {
			t.Fatalf("EventsPendingOutputRef() error = %v", err)
		}
		wokeAt := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
		run := looppkg.Run{Generation: 1, LastProgressAt: wokeAt}
		generations := []contract.LoopGenerationPayload{{
			Generation: 1,
			Outputs: []contract.LoopGenerationOutput{{
				NodeID:    loopWatchEventsTestWatchNodeID,
				Status:    daemonRuntimeStatusRunning,
				OutputRef: ref,
			}},
		}}
		state, err := loopWatchEventsReadModel(
			run,
			watchEventsDefinition(loopWatchEventsTestWatchNodeID, string(dsl.SourceWatchEvents)),
			generations,
		)
		if err != nil {
			t.Fatalf("loopWatchEventsReadModel() error = %v", err)
		}
		if state == nil {
			t.Fatal("loopWatchEventsReadModel() state = nil, want parked state")
		}
		wantSubscriptions := []contract.LoopWatchEventSubscription{
			{Kind: contract.LoopWatchEventTaskStatusChanged, Filter: "event.payload.to_status == 'completed'"},
		}
		if !slices.Equal(state.Subscriptions, wantSubscriptions) {
			t.Fatalf("subscriptions = %#v, want %#v", state.Subscriptions, wantSubscriptions)
		}
		if got, want := state.Cursors[looppkg.WatchEventsTaskStream], int64(42); got != want {
			t.Fatalf("task_events cursor = %d, want %d", got, want)
		}
		if state.LastWakeAt == nil || !state.LastWakeAt.Equal(wokeAt) {
			t.Fatalf("LastWakeAt = %#v, want %s", state.LastWakeAt, wokeAt)
		}
	})
	t.Run("Should return nil when the watch-events node is not parked", func(t *testing.T) {
		t.Parallel()
		ref, err := watchpkg.EventsConfirmedOutputRef(
			[]map[string]any{{watchEventsPayloadKindKey: string(contract.LoopWatchEventTaskStatusChanged)}},
			map[string]int64{looppkg.WatchEventsTaskStream: 43},
		)
		if err != nil {
			t.Fatalf("EventsConfirmedOutputRef() error = %v", err)
		}
		run := looppkg.Run{Generation: 1, LastProgressAt: time.Now()}
		generations := []contract.LoopGenerationPayload{{
			Generation: 1,
			Outputs:    []contract.LoopGenerationOutput{{NodeID: "watch", Status: "succeeded", OutputRef: ref}},
		}}
		state, err := loopWatchEventsReadModel(
			run,
			watchEventsDefinition(loopWatchEventsTestWatchNodeID, string(dsl.SourceWatchEvents)),
			generations,
		)
		if err != nil {
			t.Fatalf("loopWatchEventsReadModel() error = %v", err)
		}
		if state != nil {
			t.Fatalf("loopWatchEventsReadModel() state = %#v, want nil", state)
		}
	})
	t.Run("Should return nil when the definition has no watch-events node", func(t *testing.T) {
		t.Parallel()
		run := looppkg.Run{Generation: 1, LastProgressAt: time.Now()}
		generations := []contract.LoopGenerationPayload{{
			Generation: 1,
			Outputs:    []contract.LoopGenerationOutput{{NodeID: "load", Status: "succeeded"}},
		}}
		state, err := loopWatchEventsReadModel(
			run,
			watchEventsDefinition("load", string(dsl.SourceFileImport)),
			generations,
		)
		if err != nil {
			t.Fatalf("loopWatchEventsReadModel() error = %v", err)
		}
		if state != nil {
			t.Fatalf("loopWatchEventsReadModel() state = %#v, want nil", state)
		}
	})
	t.Run("Should omit last_wake_at when the run never progressed", func(t *testing.T) {
		t.Parallel()
		ref, err := watchpkg.EventsPendingOutputRef(watchpkg.EventsPendingState{
			Subscriptions: []watchpkg.EventSubscriptionRef{{Kind: string(contract.LoopWatchEventLoopTerminal)}},
			Cursors:       map[string]int64{looppkg.WatchEventsLoopStream: 0},
		})
		if err != nil {
			t.Fatalf("EventsPendingOutputRef() error = %v", err)
		}
		run := looppkg.Run{Generation: 1}
		generations := []contract.LoopGenerationPayload{{
			Generation: 1,
			Outputs: []contract.LoopGenerationOutput{{
				NodeID:    loopWatchEventsTestWatchNodeID,
				Status:    daemonRuntimeStatusRunning,
				OutputRef: ref,
			}},
		}}
		state, err := loopWatchEventsReadModel(
			run,
			watchEventsDefinition(loopWatchEventsTestWatchNodeID, string(dsl.SourceWatchEvents)),
			generations,
		)
		if err != nil {
			t.Fatalf("loopWatchEventsReadModel() error = %v", err)
		}
		if state == nil {
			t.Fatal("loopWatchEventsReadModel() state = nil, want parked state")
		}
		if state.LastWakeAt != nil {
			t.Fatalf("LastWakeAt = %#v, want nil", state.LastWakeAt)
		}
	})
	t.Run("Should surface a decode error for a corrupt watch-events park ref", func(t *testing.T) {
		t.Parallel()
		run := looppkg.Run{Generation: 1}
		generations := []contract.LoopGenerationPayload{{
			Generation: 1,
			Outputs: []contract.LoopGenerationOutput{
				{
					NodeID:    loopWatchEventsTestWatchNodeID,
					Status:    daemonRuntimeStatusRunning,
					OutputRef: "{not json",
				},
			},
		}}
		state, err := loopWatchEventsReadModel(
			run,
			watchEventsDefinition(loopWatchEventsTestWatchNodeID, string(dsl.SourceWatchEvents)),
			generations,
		)
		if err == nil {
			t.Fatal("loopWatchEventsReadModel() error = nil, want decode error")
		}
		if !strings.Contains(err.Error(), "decode watch-events park state for node watch") {
			t.Fatalf("loopWatchEventsReadModel() error = %q, want park-state decode context", err)
		}
		if state != nil {
			t.Fatalf("loopWatchEventsReadModel() state = %#v, want nil", state)
		}
	})
}
