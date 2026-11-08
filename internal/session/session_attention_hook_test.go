package session

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestManagerAttentionHookRunsAfterCanonicalCommitAndFailsOpen(t *testing.T) {
	t.Parallel()

	t.Run("Should dispatch the committed edge without failing the owning mutation", func(t *testing.T) {
		t.Parallel()

		at := time.Date(2026, 8, 15, 20, 30, 0, 0, time.UTC)
		catalog := newRecordingSessionCatalog()
		if err := catalog.RegisterSession(testutil.Context(t), new(store.SessionInfo{
			ID: "sess-hook", AgentName: "coder", WorkspaceID: "ws-hook",
			SessionType: string(SessionTypeUser), State: string(StateActive),
			CreatedAt: at.Add(-time.Hour), UpdatedAt: at.Add(-time.Minute),
		})); err != nil {
			t.Fatalf("RegisterSession() error = %v", err)
		}
		attentionStore := &committingAttentionStore{
			at:      at,
			listErr: errors.New("post-commit projection read must not run"),
		}
		var captured hookspkg.SessionAttentionChangedPayload
		edges := 0
		hook := attentionHookFunc(func(
			ctx context.Context,
			payload hookspkg.SessionAttentionChangedPayload,
		) (hookspkg.SessionAttentionChangedPayload, error) {
			if !attentionStore.wasCommitted() {
				t.Fatal("attention hook ran before the canonical store commit")
			}
			if edges > 0 {
				if ctx.Err() != nil {
					t.Fatal("committed acknowledgement inherited request cancellation")
				}
			}
			captured = payload
			edges++
			return payload, errors.New("injected observer failure")
		})
		manager, err := NewManager(
			WithHomePaths(testHomePaths(t)),
			WithSessionCatalog(catalog),
			WithHookSet(HookSet{Attention: hook}),
			WithNow(func() time.Time { return at }),
		)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		cleanupTestManager(t, manager)
		manager.attentionStore = attentionStore
		manager.mu.Lock()
		manager.sessions["sess-hook"] = &Session{
			ID: "sess-hook", AgentName: "coder", WorkspaceID: "ws-hook", State: StateActive,
			PendingInteractions: []store.PendingInteraction{},
		}
		manager.mu.Unlock()

		commit, err := manager.createPendingInteraction(testutil.Context(t), store.PendingInteractionCreate{
			InteractionID: "interaction-hook", SessionID: "sess-hook",
			Kind: store.PendingInteractionKindClarify, ProviderRequestID: "request-hook",
			Title: "Choose a path", CreatedAt: at,
		})
		if err != nil {
			t.Fatalf("createPendingInteraction() error = %v, want fail-open success", err)
		}
		if commit.Interaction == nil || commit.Interaction.InteractionID != "interaction-hook" {
			t.Fatalf("createPendingInteraction() commit = %#v, want committed interaction", commit)
		}
		if captured.Event != hookspkg.HookSessionAttentionChanged || captured.SessionID != "sess-hook" ||
			captured.WorkspaceID != "ws-hook" || captured.From != string(BadgeIdle) ||
			captured.To != string(BadgeWaitingForInput) || captured.Class != string(AttentionNeedsYou) ||
			!captured.At.Equal(at) {
			t.Fatalf("attention hook payload = %#v, want complete committed edge", captured)
		}
		commit.Before = commit.After
		commit.Interaction.Status = store.PendingInteractionStatusCanceled
		attentionStore.listErr = nil
		attentionStore.requireBoundedRead = true
		attentionStore.readFailures = 1
		next := commit
		next.Interaction = new(*commit.Interaction)
		next.Interaction.InteractionID = "interaction-hook-other"
		publishAttentionCommitsAndWait(t, manager, []store.SessionAttentionCommit{commit, next})
		if edges != 2 {
			t.Fatalf("attention edges=%d, want one creation and one acknowledgement", edges)
		}
		if captured.From != string(BadgeWaitingForInput) || captured.To != string(BadgeIdle) {
			t.Fatalf("external attention hook payload = %#v, want cleared request", captured)
		}
		target, ok := manager.Get("sess-hook")
		if !ok || target.Info().PendingClarifyCount != 0 || len(target.Info().PendingInteractions) != 0 {
			t.Fatal("external cancellation left resident pending attention")
		}
		attentionStore.snapshot = store.SessionAttention{PendingClarifyCount: 1, AttentionRevision: 3}
		attentionStore.pending = []store.PendingInteraction{
			{
				InteractionID: "newer",
				SessionID:     "sess-hook",
				Kind:          store.PendingInteractionKindClarify,
				Status:        store.PendingInteractionStatusPending,
			},
		}
		publishAttentionCommitsAndWait(t, manager, []store.SessionAttentionCommit{commit, next})
		current := target.Info()
		if BadgeForInfo(current) != BadgeWaitingForInput || len(current.PendingInteractions) != 1 ||
			current.PendingInteractions[0].InteractionID != "newer" {
			t.Fatalf("newer interaction lost: %+v", current)
		}
	})

	t.Run("Should recover timed-out reads without suppressing later sessions", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		manager, err := NewManager(WithHomePaths(testHomePaths(t)))
		if err != nil {
			t.Fatal(err)
		}
		cleanupTestManager(t, manager)
		readStarted := make(chan struct{}, 1)
		manager.attentionStore = &committingAttentionStore{
			readStarted:        readStarted,
			blockedSessionID:   "slow",
			blockedReads:       1,
			requireBoundedRead: true,
		}
		manager.mu.Lock()
		manager.sessions["slow"] = &Session{
			ID:                  "slow",
			ProfileID:           store.DefaultProfileID,
			State:               StateStopped,
			PendingClarifyCount: 1,
		}
		manager.sessions["next"] = &Session{
			ID:                  "next",
			ProfileID:           store.DefaultProfileID,
			State:               StateStopped,
			PendingClarifyCount: 1,
		}
		manager.sessions["live"] = &Session{ID: "live", ProfileID: store.DefaultProfileID, State: StateActive}
		manager.mu.Unlock()
		events, cancel, err := manager.SubscribeSessionCatalogEvents(
			ctx,
			CatalogScope{ReadScope: store.ReadScope{AllProfiles: true}, AllWorkspaces: true},
		)
		if err != nil {
			t.Fatal(err)
		}
		defer cancel()
		done := make(chan struct{})
		returned := make(chan struct{})
		go func() {
			manager.PublishPendingInteractionCommits([]store.SessionAttentionCommit{
				{
					Before: store.SessionAttention{PendingClarifyCount: 1},
					Interaction: &store.PendingInteraction{
						InteractionID: "old-slow",
						SessionID:     "slow",
						Status:        store.PendingInteractionStatusCanceled,
					},
				},
				{
					Before: store.SessionAttention{PendingClarifyCount: 1},
					Interaction: &store.PendingInteraction{
						InteractionID: "old-next",
						SessionID:     "next",
						Status:        store.PendingInteractionStatusCanceled,
					},
				},
			}, func() { close(done) })
			close(returned)
		}()
		select {
		case <-returned:
		case <-time.After(time.Second):
			t.Fatal("acknowledgement waited for reconciliation")
		}
		select {
		case <-readStarted:
		case <-time.After(time.Second):
			t.Fatal("refresh did not start")
		}
		live := make(chan error, 1)
		go func() {
			_, err := manager.createPendingInteraction(ctx, store.PendingInteractionCreate{
				SessionID:         "live",
				InteractionID:     "live-permission",
				Kind:              store.PendingInteractionKindPermission,
				ProviderRequestID: "live-permission",
			})
			live <- err
		}()
		select {
		case err := <-live:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("blocked read delayed unrelated permission")
		}
		select {
		case <-done:
		case <-time.After(2 * defaultLifecycleTimeout):
			t.Fatal("refresh did not recover")
		}
		slow, ok := manager.Get("slow")
		if !ok || slow.Info().PendingClarifyCount != 0 {
			t.Fatal("timeout prevented retrying the committed session refresh")
		}
		next, ok := manager.Get("next")
		if !ok || next.Info().PendingClarifyCount != 0 {
			t.Fatal("earlier timeout suppressed the next session refresh")
		}
		slowEdges := 0
		for range 3 {
			assertCatalogEventName(t, events, CatalogEventNameChanged)
			edge := assertCatalogEventName(t, events, CatalogEventNameAttention).Attention
			if edge != nil && edge.SessionID == "slow" {
				slowEdges++
				if edge.From != BadgeWaitingForInput || edge.To != BadgeStopped {
					t.Fatalf("recovered slow edge=%+v", edge)
				}
			}
		}
		if slowEdges != 1 {
			t.Fatalf("recovered slow edges=%d", slowEdges)
		}
	})

	t.Run("Should reconcile after exhausting immediate reads and publish before completion", func(t *testing.T) {
		t.Parallel()
		manager, err := NewManager(WithHomePaths(testHomePaths(t)))
		if err != nil {
			t.Fatal(err)
		}
		cleanupTestManager(t, manager)
		manager.attentionStore = &committingAttentionStore{readFailures: 3, requireBoundedRead: true}
		manager.sessions["retry"] = &Session{
			ID:                  "retry",
			ProfileID:           store.DefaultProfileID,
			State:               StateStopped,
			PendingClarifyCount: 1,
			PendingInteractions: []store.PendingInteraction{
				{InteractionID: "old", Status: store.PendingInteractionStatusPending},
			},
		}
		events, cancel, err := manager.SubscribeSessionCatalogEvents(
			t.Context(),
			CatalogScope{ReadScope: store.ReadScope{AllProfiles: true}, AllWorkspaces: true},
		)
		if err != nil {
			t.Fatal(err)
		}
		defer cancel()
		published := make(chan []CatalogEvent, 1)
		manager.PublishPendingInteractionCommits([]store.SessionAttentionCommit{
			{
				Before: store.SessionAttention{PendingClarifyCount: 1},
				Interaction: &store.PendingInteraction{
					SessionID:     "retry",
					InteractionID: "old",
					Status:        store.PendingInteractionStatusCanceled,
				},
			},
		}, func() {
			var emitted []CatalogEvent
			for range 2 {
				select {
				case event := <-events:
					emitted = append(emitted, event)
				default:
				}
			}
			published <- emitted
		})
		var emitted []CatalogEvent
		select {
		case emitted = <-published:
		case <-time.After(2 * defaultLifecycleTimeout):
			t.Fatal("reconciliation did not complete")
		}
		if len(emitted) != 2 || emitted[0].Name != CatalogEventNameChanged ||
			emitted[1].Name != CatalogEventNameAttention {
			t.Fatalf("events before completion=%+v", emitted)
		}
		info := manager.sessions["retry"].Info()
		if info.PendingClarifyCount != 0 || len(info.PendingInteractions) != 0 {
			t.Fatalf("stale resident state=%+v", info)
		}
		edge := emitted[1].Attention
		if edge == nil || edge.SessionID != "retry" || edge.From != BadgeWaitingForInput || edge.To != BadgeStopped {
			t.Fatalf("reconciled edge=%+v", edge)
		}
		select {
		case extra := <-events:
			t.Fatalf("duplicate reconciliation event=%+v", extra)
		default:
		}
	})

	t.Run("Should cancel and join acknowledgement reads on shutdown", func(t *testing.T) {
		t.Parallel()
		manager, err := NewManager(WithHomePaths(testHomePaths(t)))
		if err != nil {
			t.Fatal(err)
		}
		cleanupTestManager(t, manager)
		entered := make(chan struct{}, 1)
		manager.attentionStore = &committingAttentionStore{
			blockedSessionID: "stop",
			blockedReads:     10,
			readStarted:      entered,
		}
		completed := make(chan struct{}, 1)
		commits := []store.SessionAttentionCommit{{Interaction: &store.PendingInteraction{SessionID: "stop"}}}
		manager.PublishPendingInteractionCommits(commits, func() { completed <- struct{}{} })
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("refresh did not start")
		}
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		if err := manager.Shutdown(ctx); err != nil {
			t.Fatal(err)
		}
		manager.PublishPendingInteractionCommits(commits, func() { completed <- struct{}{} })
		select {
		case <-completed:
			t.Fatal("shutdown allowed acknowledgement completion")
		default:
		}
	})

	t.Run("Should preserve and publish canonical attention when transcript append fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		at := time.Date(2026, 8, 15, 20, 35, 0, 0, time.UTC)
		catalog := newRecordingSessionCatalog()
		if err := catalog.RegisterSession(ctx, new(store.SessionInfo{
			ID: "sess-transcript-failure", AgentName: "coder", WorkspaceID: "ws-hook",
			SessionType: string(SessionTypeUser), State: string(StateActive),
			CreatedAt: at.Add(-time.Hour), UpdatedAt: at.Add(-time.Minute),
		})); err != nil {
			t.Fatalf("RegisterSession() error = %v", err)
		}
		attentionStore := &committingAttentionStore{
			at:      at,
			listErr: errors.New("post-commit projection read must not run"),
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
		manager.attentionStore = attentionStore
		target := &Session{
			ID: "sess-transcript-failure", ProfileID: store.DefaultProfileID,
			AgentName: "coder", WorkspaceID: "ws-hook", State: StateActive,
			PendingInteractions: []store.PendingInteraction{},
		}
		target.setRecorder(&attentionTranscriptFailingRecorder{
			EventRecorder: &fakeEventRecorder{},
			err:           errors.New("injected transcript append failure"),
		})
		manager.mu.Lock()
		manager.sessions[target.ID] = target
		manager.mu.Unlock()
		events, cancel, err := manager.SubscribeSessionCatalogEvents(
			ctx,
			CatalogScope{ReadScope: store.ReadScope{AllProfiles: true}, AllWorkspaces: true},
		)
		if err != nil {
			t.Fatalf("SubscribeSessionCatalogEvents() error = %v", err)
		}
		defer cancel()

		event := acp.AgentEvent{
			Type: acp.EventTypePermission, TurnID: "turn-permission", Title: "Approve release", Timestamp: at,
		}.WithRequestID("permission-transcript-failure")
		if err := manager.recordEvent(ctx, target, event); err != nil {
			t.Fatalf("recordEvent(transcript failure) error = %v, want committed success", err)
		}
		if got := BadgeForInfo(target.Info()); got != BadgeWaitingForAuth {
			t.Fatalf("BadgeForInfo() = %q, want %q", got, BadgeWaitingForAuth)
		}
		assertCatalogEventName(t, events, CatalogEventNameChanged)
		attention := assertCatalogEventName(t, events, CatalogEventNameAttention)
		if attention.Attention == nil || attention.Attention.To != BadgeWaitingForAuth ||
			!attention.Attention.At.Equal(at) {
			t.Fatalf("attention event = %#v, want committed permission edge", attention.Attention)
		}
	})
}

type attentionTranscriptFailingRecorder struct {
	EventRecorder
	err error
}

func (r *attentionTranscriptFailingRecorder) Record(context.Context, store.SessionEvent) error {
	return r.err
}

type attentionHookFunc func(
	context.Context,
	hookspkg.SessionAttentionChangedPayload,
) (hookspkg.SessionAttentionChangedPayload, error)

func (f attentionHookFunc) DispatchSessionAttentionChanged(
	ctx context.Context,
	payload hookspkg.SessionAttentionChangedPayload,
) (hookspkg.SessionAttentionChangedPayload, error) {
	return f(ctx, payload)
}

type committingAttentionStore struct {
	snapshot           store.SessionAttention
	pending            []store.PendingInteraction
	blockedSessionID   string
	blockedReads       int
	readStarted        chan struct{}
	requireBoundedRead bool
	readFailures       int
	mu                 sync.Mutex
	at                 time.Time
	committed          bool
	listErr            error
}

func (s *committingAttentionStore) GetSessionAttention(
	ctx context.Context,
	id string,
) (store.SessionAttention, error) {
	if id == s.blockedSessionID && s.blockedReads > 0 {
		s.blockedReads--
		if s.readStarted != nil {
			select {
			case s.readStarted <- struct{}{}:
			default:
			}
		}
		<-ctx.Done()
		return store.SessionAttention{}, ctx.Err()
	}
	if s.requireBoundedRead {
		if _, bounded := ctx.Deadline(); !bounded {
			return store.SessionAttention{}, errors.New("post-commit read has no deadline")
		}
	}
	if s.readFailures > 0 {
		s.readFailures--
		return store.SessionAttention{}, errors.New("transient attention read failure")
	}
	return s.snapshot, ctx.Err()
}

func (s *committingAttentionStore) CreatePendingInteraction(
	_ context.Context,
	req store.PendingInteractionCreate,
) (store.SessionAttentionCommit, error) {
	s.mu.Lock()
	s.committed = true
	s.mu.Unlock()
	changedAt := s.at
	interaction := store.PendingInteraction{
		InteractionID: req.InteractionID, SessionID: req.SessionID,
		Kind: req.Kind, ProviderRequestID: req.ProviderRequestID,
		Title: req.Title, Status: store.PendingInteractionStatusPending, CreatedAt: req.CreatedAt,
	}
	after := store.SessionAttention{AttentionRevision: 1, AttentionChangedAt: &changedAt}
	if req.Kind == store.PendingInteractionKindPermission {
		after.PendingPermissionCount = 1
	} else {
		after.PendingClarifyCount = 1
	}
	return store.SessionAttentionCommit{
		Before:      store.SessionAttention{},
		After:       after,
		Interaction: &interaction,
		Outcome:     store.PendingInteractionStatusPending,
	}, nil
}

func (s *committingAttentionStore) TransitionPendingInteraction(
	context.Context,
	store.PendingInteractionTransition,
) (store.SessionAttentionCommit, error) {
	return store.SessionAttentionCommit{}, errors.New("unexpected TransitionPendingInteraction call")
}

func (s *committingAttentionStore) ListPendingInteractions(
	ctx context.Context,
	_ string,
	_ []string,
) ([]store.PendingInteraction, error) {
	if s.requireBoundedRead {
		if _, bounded := ctx.Deadline(); !bounded {
			return nil, errors.New("post-commit read has no deadline")
		}
	}
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.pending, nil
}

func (s *committingAttentionStore) HasPermissionOccurrence(
	context.Context,
	string,
	string,
	string,
	time.Time,
) (bool, error) {
	return false, nil
}

func (s *committingAttentionStore) MarkSessionSettled(
	context.Context,
	string,
	bool,
	time.Time,
) (store.SessionAttentionCommit, error) {
	return store.SessionAttentionCommit{}, errors.New("unexpected MarkSessionSettled call")
}

func (s *committingAttentionStore) MarkSessionSeen(
	context.Context,
	string,
	time.Time,
) (store.SessionAttentionCommit, error) {
	return store.SessionAttentionCommit{}, errors.New("unexpected MarkSessionSeen call")
}

func (s *committingAttentionStore) MarkSessionEndedEarly(
	context.Context,
	string,
	string,
) (store.SessionAttentionCommit, error) {
	return store.SessionAttentionCommit{}, errors.New("unexpected MarkSessionEndedEarly call")
}

func (s *committingAttentionStore) DismissSessionAttention(
	context.Context,
	string,
	store.SessionAttentionFence,
	time.Time,
) (store.SessionAttentionCommit, error) {
	return store.SessionAttentionCommit{}, errors.New("unexpected DismissSessionAttention call")
}

func (s *committingAttentionStore) wasCommitted() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.committed
}

// publishAttentionCommitsAndWait observes completion after the manager publishes the session edges.
func publishAttentionCommitsAndWait(t *testing.T, manager *Manager, commits []store.SessionAttentionCommit) {
	t.Helper()
	done := make(chan struct{})
	manager.PublishPendingInteractionCommits(commits, func() { close(done) })
	select {
	case <-done:
	case <-time.After(2 * defaultLifecycleTimeout):
		t.Fatal("acknowledgement reconciliation did not finish")
	}
}
