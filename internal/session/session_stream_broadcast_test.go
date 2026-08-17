package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/testutil"
)

func TestSessionEventBroadcaster(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve runtime identity in stream subscription diagnostics", func(t *testing.T) {
		t.Parallel()

		notifier := &streamIdentityNotifier{}
		manager, err := NewManager(WithHomePaths(testHomePaths(t)), WithNotifier(notifier))
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		cleanupTestManager(t, manager)
		sess := &Session{
			ID: "sess-stream-child", AgentName: "worker", Provider: "claude", Model: "claude-sonnet",
			WorkspaceID: "ws-1", State: StateActive,
			Lineage: &store.SessionLineage{
				ParentSessionID: "sess-parent", RootSessionID: "sess-root", SpawnDepth: 2,
			},
		}
		manager.mu.Lock()
		manager.sessions[sess.ID] = sess
		manager.mu.Unlock()

		_, cancel, err := manager.SubscribeSessionEvents(testutil.Context(t), sess.ID, 0)
		if err != nil {
			t.Fatalf("SubscribeSessionEvents() error = %v", err)
		}
		defer cancel()
		got := notifier.last
		if got == nil || got.Provider != sess.Provider || got.Model != sess.Model || got.Lineage == nil ||
			got.Lineage.ParentSessionID != "sess-parent" || got.Lineage.RootSessionID != "sess-root" ||
			got.Lineage.SpawnDepth != 2 {
			t.Fatalf("stream diagnostic session identity = %#v, want full spawned runtime identity", got)
		}
	})

	t.Run("Should deliver persisted events after cursor and ignore older sequences", func(t *testing.T) {
		t.Parallel()

		manager, err := NewManager(WithHomePaths(testHomePaths(t)))
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		cleanupTestManager(t, manager)
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
		cleanupTestManager(t, manager)
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
		cleanupTestManager(t, manager)
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

type streamIdentityNotifier struct {
	last *Info
}

func (n *streamIdentityNotifier) OnSessionCreated(context.Context, *Session) {}

func (n *streamIdentityNotifier) OnSessionStopped(context.Context, *Session) {}

func (n *streamIdentityNotifier) OnAgentEvent(context.Context, string, any) {}

func (n *streamIdentityNotifier) OnAgentEventForSession(_ context.Context, sess *Session, _ any) {
	n.last = sess.Info()
}

func TestSessionCatalogBroadcaster(t *testing.T) {
	t.Parallel()

	t.Run("Should keep daemon-owned sessions out of the public catalog stream", func(t *testing.T) {
		t.Parallel()

		manager, err := NewManager(WithHomePaths(testHomePaths(t)))
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		cleanupTestManager(t, manager)
		events, cancel, err := manager.SubscribeSessionCatalogEvents(testutil.Context(t))
		if err != nil {
			t.Fatalf("SubscribeSessionCatalogEvents() error = %v", err)
		}
		defer cancel()

		internalSessions := []*Session{
			{
				ID: "sess-memory-extractor", WorkspaceID: "ws-internal",
				Lineage: &store.SessionLineage{SpawnRole: SpawnRoleMemoryExtractor},
			},
			{
				ID: "sess-auto-title", WorkspaceID: "ws-internal",
				Lineage: &store.SessionLineage{SpawnRole: SpawnRoleAutoTitle},
			},
			{ID: "sess-dream", WorkspaceID: "ws-internal", Type: SessionTypeDream},
		}
		for _, sess := range internalSessions {
			manager.publishSessionCatalogWakeForEvent(sess, acp.AgentEvent{Type: acp.EventTypePermission})
		}

		select {
		case event := <-events:
			t.Fatalf("daemon-owned session woke the public catalog: %#v", event)
		default:
		}
	})

	t.Run("Should wake the owning workspace catalog for every attention event type", func(t *testing.T) {
		t.Parallel()

		manager, err := NewManager(WithHomePaths(testHomePaths(t)))
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		cleanupTestManager(t, manager)
		events, cancel, err := manager.SubscribeSessionCatalogEvents(testutil.Context(t))
		if err != nil {
			t.Fatalf("SubscribeSessionCatalogEvents() error = %v", err)
		}
		defer cancel()

		session := &Session{ID: "sess-attention", WorkspaceID: "ws-attention"}
		for _, eventType := range []string{
			acp.EventTypePermission,
			acp.EventTypeClarify,
			acp.EventTypeDone,
			acp.EventTypeError,
		} {
			manager.publishSessionCatalogWakeForEvent(session, acp.AgentEvent{Type: eventType})
			select {
			case event := <-events:
				want := CatalogEvent{
					Name:        CatalogEventNameChanged,
					Kind:        CatalogEventUpserted,
					WorkspaceID: "ws-attention",
					SessionID:   "sess-attention",
				}
				if event != want {
					t.Fatalf("catalog event for %q = %#v, want %#v", eventType, event, want)
				}
			default:
				t.Fatalf("attention event %q did not wake the session catalog", eventType)
			}
		}

		manager.publishSessionCatalogWakeForEvent(session, acp.AgentEvent{Type: acp.EventTypeAgentMessage})
		select {
		case event := <-events:
			t.Fatalf("non-attention event woke the catalog: %#v", event)
		default:
		}
	})

	t.Run("Should broadcast workspace-identified events to one global subscriber", func(t *testing.T) {
		t.Parallel()

		manager, err := NewManager(WithHomePaths(testHomePaths(t)))
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		cleanupTestManager(t, manager)
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

	t.Run("Should publish one complete attention edge only when the badge changes", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		at := time.Date(2026, 8, 15, 18, 2, 41, 0, time.UTC)
		catalog := newRecordingSessionCatalog()
		if err := catalog.RegisterSession(ctx, store.SessionInfo{
			ID: "sess-attention", AgentName: "coder", WorkspaceID: "ws-attention",
			SessionType: string(SessionTypeUser), State: string(StateActive),
			CreatedAt: at.Add(-time.Hour), UpdatedAt: at.Add(-time.Minute),
		}); err != nil {
			t.Fatalf("RegisterSession() error = %v", err)
		}
		manager, err := NewManager(
			WithHomePaths(testHomePaths(t)),
			WithSessionCatalog(catalog),
			WithNow(func() time.Time { return at }),
		)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		cleanupTestManager(t, manager)
		events, cancel, err := manager.SubscribeSessionCatalogEvents(ctx)
		if err != nil {
			t.Fatalf("SubscribeSessionCatalogEvents() error = %v", err)
		}
		defer cancel()

		changedAt := at
		manager.publishAttentionCommit(ctx, "sess-attention", store.SessionAttentionCommit{
			Before: store.SessionAttention{AttentionRevision: 1},
			After: store.SessionAttention{
				PendingClarifyCount: 1,
				AttentionRevision:   2,
				AttentionChangedAt:  &changedAt,
			},
		})
		assertCatalogEventName(t, events, CatalogEventNameChanged)
		attention := assertCatalogEventName(t, events, CatalogEventNameAttention)
		if attention.Attention == nil || attention.Attention.SessionID != "sess-attention" ||
			attention.Attention.WorkspaceID != "ws-attention" || attention.Attention.From != BadgeIdle ||
			attention.Attention.To != BadgeWaitingForInput || attention.Attention.Class != AttentionNeedsYou ||
			!attention.Attention.At.Equal(at) {
			t.Fatalf("attention event = %#v, want canonical edge payload", attention.Attention)
		}

		manager.publishAttentionCommit(ctx, "sess-attention", store.SessionAttentionCommit{
			Before: store.SessionAttention{PendingClarifyCount: 1, AttentionRevision: 2},
			After:  store.SessionAttention{PendingClarifyCount: 1, AttentionRevision: 3},
		})
		assertCatalogEventName(t, events, CatalogEventNameChanged)
		select {
		case unexpected := <-events:
			t.Fatalf("same-badge commit published %#v, want no attention edge", unexpected)
		default:
		}

		unchanged := store.SessionAttention{PendingClarifyCount: 1, AttentionRevision: 3}
		manager.publishAttentionCommit(ctx, "sess-attention", store.SessionAttentionCommit{
			Before: unchanged,
			After:  unchanged,
		})
		select {
		case unexpected := <-events:
			t.Fatalf("unchanged attention commit published %#v, want no catalog invalidation", unexpected)
		default:
		}
	})
}

func assertCatalogEventName(
	t *testing.T,
	events <-chan CatalogEvent,
	want CatalogEventName,
) CatalogEvent {
	t.Helper()
	select {
	case event := <-events:
		if event.Name != want {
			t.Fatalf("catalog event name = %q, want %q", event.Name, want)
		}
		return event
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for catalog event %q", want)
		return CatalogEvent{}
	}
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

func testHomePaths(t *testing.T) compozyconfig.HomePaths {
	t.Helper()
	homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	return homePaths
}
