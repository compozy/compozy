package daemon

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	looppkg "github.com/compozy/agh/internal/loop"
	watchpkg "github.com/compozy/agh/internal/loop/watch"
)

func TestLoopWatchEventsObserverShouldUseKindIndexBeforeDoorbellCEL(t *testing.T) {
	t.Parallel()

	t.Run("Should skip CEL evaluations for unrelated parked subscriptions", func(t *testing.T) {
		t.Parallel()

		watchStore := newRecordingLoopWatchEventsStore()
		watchStore.setParked(watchEventsDoorbellSubscriptionsForTest(
			512,
			hookspkg.HookTaskStatusChanged,
			looppkg.WatchEventsTaskStream,
			`event.payload.to_status == "blocked"`,
		))
		observer := newLoopWatchEventsObserverForTest(
			t,
			watchStore,
			&recordingWatchEventsBackstop{},
			time.Date(2026, 7, 9, 11, 0, 0, 0, time.UTC),
		)
		matcher := &countingWatchEventsDoorbellMatcher{}
		observer.compiler = matcher

		if err := observer.OnEventPostRecord(t.Context(), watchEventsPostRecordPayloadForDoorbellTest()); err != nil {
			t.Fatalf("OnEventPostRecord(unrelated) error = %v", err)
		}
		if got := matcher.callCount(); got != 0 {
			t.Fatalf("doorbell CEL evaluations = %d, want 0 for unrelated kinds", got)
		}
		if got := len(watchStore.snapshotWakes()); got != 0 {
			t.Fatalf("wake calls = %d, want 0 for unrelated kinds", got)
		}
	})

	t.Run("Should evaluate only subscriptions indexed under the event kind", func(t *testing.T) {
		t.Parallel()

		parked := watchEventsDoorbellSubscriptionsForTest(
			512,
			hookspkg.HookTaskStatusChanged,
			looppkg.WatchEventsTaskStream,
			`event.payload.to_status == "blocked"`,
		)
		parked = append(parked, watchEventsDoorbellSubscriptionsForTest(
			3,
			hookspkg.HookEventPostRecord,
			looppkg.WatchEventsSessionStreamForSession("sess-hot"),
			`event.session_id == "sess-cold"`,
		)...)
		watchStore := newRecordingLoopWatchEventsStore()
		watchStore.setParked(parked)
		observer := newLoopWatchEventsObserverForTest(
			t,
			watchStore,
			&recordingWatchEventsBackstop{},
			time.Date(2026, 7, 9, 11, 15, 0, 0, time.UTC),
		)
		matcher := &countingWatchEventsDoorbellMatcher{}
		observer.compiler = matcher

		if err := observer.OnEventPostRecord(t.Context(), watchEventsPostRecordPayloadForDoorbellTest()); err != nil {
			t.Fatalf("OnEventPostRecord(indexed) error = %v", err)
		}
		if got, want := matcher.callCount(), int64(3); got != want {
			t.Fatalf("doorbell CEL evaluations = %d, want %d same-kind subscriptions only", got, want)
		}
	})
}

func BenchmarkLoopWatchEventsObserverEventPostRecordDoorbell(b *testing.B) {
	b.Run("unrelated_kind_index_4096", func(b *testing.B) {
		watchStore := newRecordingLoopWatchEventsStore()
		watchStore.setParked(watchEventsDoorbellSubscriptionsForTest(
			4096,
			hookspkg.HookTaskStatusChanged,
			looppkg.WatchEventsTaskStream,
			`event.payload.to_status == "blocked"`,
		))
		observer := newLoopWatchEventsObserverForBenchmark(b, watchStore)
		matcher := &countingWatchEventsDoorbellMatcher{}
		observer.compiler = matcher
		payload := watchEventsPostRecordPayloadForDoorbellTest()

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := observer.OnEventPostRecord(context.Background(), payload); err != nil {
				b.Fatalf("OnEventPostRecord(unrelated) error = %v", err)
			}
		}
		b.StopTimer()
		if got := matcher.callCount(); got != 0 {
			b.Fatalf("doorbell CEL evaluations = %d, want 0 for unrelated kinds", got)
		}
		b.ReportMetric(0, "evals/op")
	})

	b.Run("same_kind_cached_cel_16", func(b *testing.B) {
		watchStore := newRecordingLoopWatchEventsStore()
		watchStore.setParked(watchEventsDoorbellSubscriptionsForTest(
			16,
			hookspkg.HookEventPostRecord,
			looppkg.WatchEventsSessionStreamForSession("sess-hot"),
			`event.session_id == "sess-cold" && event.payload.record_type == "agent_message"`,
		))
		observer := newLoopWatchEventsObserverForBenchmark(b, watchStore)
		payload := watchEventsPostRecordPayloadForDoorbellTest()
		if err := observer.OnEventPostRecord(context.Background(), payload); err != nil {
			b.Fatalf("OnEventPostRecord(warm cache) error = %v", err)
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := observer.OnEventPostRecord(context.Background(), payload); err != nil {
				b.Fatalf("OnEventPostRecord(cached CEL) error = %v", err)
			}
		}
		b.StopTimer()
		b.ReportMetric(16, "evals/op")
	})
}

func BenchmarkLoopWatchEventsObserverWorkerTerminal(b *testing.B) {
	watchStore := newRecordingLoopWatchEventsStore()
	watchStore.setParked(watchEventsDoorbellSubscriptionsForTest(
		4096,
		hookspkg.HookTaskStatusChanged,
		looppkg.WatchEventsTaskStream,
		`event.payload.to_status == "blocked"`,
	))
	observer := newLoopWatchEventsObserverForBenchmark(b, watchStore)
	runKind := "worker"
	payload := hookspkg.TaskRunLeasePayload{
		PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookTaskRunCompleted},
		TaskRunContext: hookspkg.TaskRunContext{
			RunID:     "worker-run",
			RunKind:   &runKind,
			LoopRunID: "loop-run-1",
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := observer.OnTaskRunTerminal(context.Background(), payload); err != nil {
			b.Fatalf("OnTaskRunTerminal(worker) error = %v", err)
		}
	}
}

type countingWatchEventsDoorbellMatcher struct {
	calls atomic.Int64
}

func (m *countingWatchEventsDoorbellMatcher) Match(
	string,
	map[string]any,
	looppkg.WatchEvent,
) bool {
	m.calls.Add(1)
	return false
}

func (m *countingWatchEventsDoorbellMatcher) callCount() int64 {
	return m.calls.Load()
}

func newLoopWatchEventsObserverForBenchmark(
	b *testing.B,
	watchStore *recordingLoopWatchEventsStore,
) *loopWatchEventsObserver {
	b.Helper()
	observer, err := newLoopWatchEventsObserver(
		watchStore,
		&recordingWatchEventsBackstop{},
		func() time.Time { return time.Date(2026, 7, 9, 11, 30, 0, 0, time.UTC) },
	)
	if err != nil {
		b.Fatalf("newLoopWatchEventsObserver() error = %v", err)
	}
	if err := observer.Hydrate(context.Background()); err != nil {
		b.Fatalf("Hydrate() error = %v", err)
	}
	return observer
}

func watchEventsDoorbellSubscriptionsForTest(
	count int,
	kind hookspkg.HookEvent,
	stream string,
	filter string,
) []looppkg.ParkedWatchEventSubscription {
	entries := make([]looppkg.ParkedWatchEventSubscription, 0, count)
	for i := range count {
		entries = append(entries, looppkg.ParkedWatchEventSubscription{
			WorkspaceID: "ws-1",
			LoopRunID:   fmt.Sprintf("loop-run-%d", i),
			LoopName:    "delivery",
			NodeID:      fmt.Sprintf("watch_%d", i),
			Generation:  1,
			Inputs:      map[string]any{"target_task_id": "task-target"},
			Subscriptions: []watchpkg.EventSubscriptionRef{{
				Kind:   string(kind),
				Filter: filter,
			}},
			Cursors:   map[string]int64{stream: 0},
			Contracts: watchEventsContractsForKindForTest(kind),
		})
	}
	return entries
}

func watchEventsPostRecordPayloadForDoorbellTest() hookspkg.EventPostRecordPayload {
	return hookspkg.EventPostRecordPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookEventPostRecord,
			Timestamp: time.Date(2026, 7, 9, 11, 45, 0, 0, time.UTC),
		},
		SessionContext: hookspkg.SessionContext{
			SessionID:   "sess-hot",
			AgentName:   "coder",
			WorkspaceID: "ws-1",
		},
		TurnContext: hookspkg.TurnContext{TurnID: "turn-hot"},
		RecordType:  "agent_message",
		Sequence:    100,
	}
}
