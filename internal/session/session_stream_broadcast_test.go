package session

import (
	"context"
	"errors"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/sessiondb"
	"github.com/compozy/agh/internal/testutil"
)

func TestSessionEventBroadcaster(t *testing.T) {
	t.Parallel()

	t.Run("Should deliver persisted events after cursor and ignore older sequences", func(t *testing.T) {
		t.Parallel()

		manager, err := NewManager(WithHomePaths(testHomePaths(t)))
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		events, cancel, err := manager.SubscribeSessionEvents(testutil.Context(t), "sess-stream", 1)
		if err != nil {
			t.Fatalf("SubscribeSessionEvents() error = %v", err)
		}
		defer cancel()

		manager.publishSessionEvent(testutil.Context(t), &Session{ID: "sess-stream"}, store.SessionEvent{
			SessionID: "sess-stream",
			Sequence:  1,
			TurnID:    "turn-1",
			Type:      "agent_message",
		})
		manager.publishSessionEvent(testutil.Context(t), &Session{ID: "sess-stream"}, store.SessionEvent{
			SessionID: "sess-stream",
			Sequence:  2,
			TurnID:    "turn-1",
			Type:      "agent_message",
		})

		select {
		case event := <-events:
			if event.Sequence != 2 {
				t.Fatalf("event.Sequence = %d, want 2", event.Sequence)
			}
		case <-testutil.Context(t).Done():
			t.Fatal("timed out waiting for stream event")
		}
	})

	t.Run("Should deliver wake events across a sequence reset", func(t *testing.T) {
		t.Parallel()

		manager, err := NewManager(WithHomePaths(testHomePaths(t)))
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		events, cancel, err := manager.SubscribeSessionEventWakes(testutil.Context(t), "sess-wake")
		if err != nil {
			t.Fatalf("SubscribeSessionEventWakes() error = %v", err)
		}
		defer cancel()

		for _, sequence := range []int64{100, 1} {
			manager.publishSessionEvent(testutil.Context(t), &Session{ID: "sess-wake"}, store.SessionEvent{
				SessionID: "sess-wake",
				Sequence:  sequence,
				TurnID:    "turn-wake",
				Type:      "agent_message",
			})
		}

		for _, wantSequence := range []int64{100, 1} {
			select {
			case event := <-events:
				if event.Sequence != wantSequence {
					t.Fatalf("event.Sequence = %d, want %d", event.Sequence, wantSequence)
				}
			case <-testutil.Context(t).Done():
				t.Fatalf("timed out waiting for wake sequence %d", wantSequence)
			}
		}
	})

	t.Run("Should close slow subscriber on overflow", func(t *testing.T) {
		t.Parallel()

		manager, err := NewManager(WithHomePaths(testHomePaths(t)))
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		events, cancel, err := manager.SubscribeSessionEvents(testutil.Context(t), "sess-overflow", 0)
		if err != nil {
			t.Fatalf("SubscribeSessionEvents() error = %v", err)
		}
		defer cancel()

		for sequence := int64(1); sequence <= sessionEventSubscriberBuffer+1; sequence++ {
			manager.publishSessionEvent(context.Background(), &Session{ID: "sess-overflow"}, store.SessionEvent{
				SessionID: "sess-overflow",
				Sequence:  sequence,
				TurnID:    "turn-overflow",
				Type:      "agent_message",
			})
		}

		for range sessionEventSubscriberBuffer {
			<-events
		}
		if _, ok := <-events; ok {
			t.Fatal("events channel remains open after overflow")
		}
	})
}

func TestSessionCatalogBroadcaster(t *testing.T) {
	t.Parallel()

	t.Run("Should broadcast workspace-identified events to one global subscriber", func(t *testing.T) {
		t.Parallel()

		manager, err := NewManager(WithHomePaths(testHomePaths(t)))
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		events, cancel, err := manager.SubscribeSessionCatalogEvents(testutil.Context(t))
		if err != nil {
			t.Fatalf("SubscribeSessionCatalogEvents() error = %v", err)
		}

		manager.publishSessionCatalogEvent(CatalogEvent{
			Kind:        CatalogEventUpserted,
			WorkspaceID: "ws-beta",
			SessionID:   "sess-beta",
		})
		manager.publishSessionCatalogEvent(CatalogEvent{
			Kind:        CatalogEventUpserted,
			WorkspaceID: "ws-alpha",
			SessionID:   "sess-alpha",
		})

		for _, want := range []CatalogEvent{
			{Kind: CatalogEventUpserted, WorkspaceID: "ws-beta", SessionID: "sess-beta"},
			{Kind: CatalogEventUpserted, WorkspaceID: "ws-alpha", SessionID: "sess-alpha"},
		} {
			select {
			case event := <-events:
				if event != want {
					t.Fatalf("catalog event = %#v, want %#v", event, want)
				}
			default:
				t.Fatalf("catalog event %#v was not delivered", want)
			}
		}

		cancel()
		if _, ok := <-events; ok {
			t.Fatal("catalog event channel remains open after cancel")
		}
	})
}

func TestManagerAppendSessionEventIfAbsent(t *testing.T) {
	t.Parallel()

	t.Run("Should persist publish dedupe and reopen Goal snapshot events", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		sess := createSession(t, h)
		events, cancel, err := h.manager.SubscribeSessionEvents(testutil.Context(t), sess.ID, 0)
		if err != nil {
			t.Fatalf("SubscribeSessionEvents() error = %v", err)
		}
		defer cancel()
		projection := GoalEvent{
			EventID: "goal-snapshot:event-1", SessionID: sess.ID,
			SyntheticTurnID: "goal-snapshot:" + sess.ID, AgentName: "system",
			Type:      EventTypeGoalSnapshotChanged,
			Content:   []byte(`{"session_id":"` + sess.ID + `","run_id":"run-1","revision":1,"cause":"start"}`),
			CreatedAt: time.Date(2026, 7, 10, 19, 0, 0, 0, time.UTC),
		}
		if err := h.manager.AppendSessionEventIfAbsent(testutil.Context(t), projection); err != nil {
			t.Fatalf("AppendSessionEventIfAbsent(first) error = %v", err)
		}
		select {
		case event := <-events:
			if event.ID != projection.EventID || event.Type != EventTypeGoalSnapshotChanged || event.Sequence < 1 {
				t.Fatalf("published event = %#v", event)
			}
		case <-testutil.Context(t).Done():
			t.Fatal("timed out waiting for Goal snapshot event")
		}
		if err := h.manager.AppendSessionEventIfAbsent(testutil.Context(t), projection); err != nil {
			t.Fatalf("AppendSessionEventIfAbsent(identical) error = %v", err)
		}
		select {
		case duplicate := <-events:
			t.Fatalf("duplicate event was republished: %#v", duplicate)
		default:
		}

		collision := projection
		collision.Content = []byte(`{"session_id":"` + sess.ID + `","run_id":"run-2","revision":1,"cause":"start"}`)
		if err := h.manager.AppendSessionEventIfAbsent(
			testutil.Context(t),
			collision,
		); !errors.Is(
			err,
			sessiondb.ErrEventIdentityCollision,
		) {
			t.Fatalf("AppendSessionEventIfAbsent(collision) error = %v", err)
		}
		if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		stoppedProjection := projection
		stoppedProjection.EventID = "goal-snapshot:event-2"
		stoppedProjection.Content = []byte(
			`{"session_id":"` + sess.ID + `","run_id":null,"revision":2,"cause":"clear"}`,
		)
		stoppedProjection.CreatedAt = projection.CreatedAt.Add(time.Second)
		if err := h.manager.AppendSessionEventIfAbsent(testutil.Context(t), stoppedProjection); err != nil {
			t.Fatalf("AppendSessionEventIfAbsent(stopped) error = %v", err)
		}
		stored, err := h.manager.Events(
			testutil.Context(t),
			sess.ID,
			store.EventQuery{Type: EventTypeGoalSnapshotChanged},
		)
		if err != nil {
			t.Fatalf("Events() error = %v", err)
		}
		if len(stored) != 2 || stored[0].ID != projection.EventID || stored[1].ID != stoppedProjection.EventID {
			t.Fatalf("stored Goal snapshot events = %#v", stored)
		}
	})

	t.Run("Should retry through active session finalization after the recorder closes", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		sess := createSession(t, h)
		ctx := testutil.Context(t)
		original := sess.recorderHandle()
		if original == nil {
			t.Fatal("recorderHandle() = nil, want active recorder")
		}
		if err := original.Close(ctx); err != nil {
			t.Fatalf("Close(active recorder) error = %v", err)
		}
		done := make(chan struct{})
		h.manager.mu.Lock()
		h.manager.finalizing[sess.ID] = &sessionFinalization{done: done}
		h.manager.mu.Unlock()
		stub := &queryRecorderStub{appendErrs: []error{store.ErrClosed}}
		stub.onAppend = func() {
			stub.onAppend = nil
			h.manager.removeActive(sess.ID)
			h.manager.finishFinalization(sess.ID, nil)
		}
		sess.setRecorder(stub)
		projection := GoalEvent{
			EventID: "goal-snapshot:finalizing", SessionID: sess.ID,
			SyntheticTurnID: "goal-snapshot:" + sess.ID, AgentName: "system",
			Type: EventTypeGoalSnapshotChanged,
			Content: []byte(
				`{"session_id":"` + sess.ID + `","run_id":"run-1","revision":1,"cause":"status"}`,
			),
			CreatedAt: time.Date(2026, 7, 10, 19, 1, 0, 0, time.UTC),
		}

		if err := h.manager.AppendSessionEventIfAbsent(ctx, projection); err != nil {
			t.Fatalf("AppendSessionEventIfAbsent(finalizing) error = %v", err)
		}
		stored, err := h.manager.Events(ctx, sess.ID, store.EventQuery{Type: EventTypeGoalSnapshotChanged})
		if err != nil {
			t.Fatalf("Events() error = %v", err)
		}
		if len(stored) != 1 || stored[0].ID != projection.EventID {
			t.Fatalf("stored Goal snapshot events = %#v", stored)
		}
	})
}

func testHomePaths(t *testing.T) aghconfig.HomePaths {
	t.Helper()
	homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	return homePaths
}
