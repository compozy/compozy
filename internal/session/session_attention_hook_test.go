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
		if err := catalog.RegisterSession(testutil.Context(t), store.SessionInfo{
			ID: "sess-hook", AgentName: "coder", WorkspaceID: "ws-hook",
			SessionType: string(SessionTypeUser), State: string(StateActive),
			CreatedAt: at.Add(-time.Hour), UpdatedAt: at.Add(-time.Minute),
		}); err != nil {
			t.Fatalf("RegisterSession() error = %v", err)
		}
		attentionStore := &committingAttentionStore{
			at:      at,
			listErr: errors.New("post-commit projection read must not run"),
		}
		var captured hookspkg.SessionAttentionChangedPayload
		hook := attentionHookFunc(func(
			_ context.Context,
			payload hookspkg.SessionAttentionChangedPayload,
		) (hookspkg.SessionAttentionChangedPayload, error) {
			if !attentionStore.wasCommitted() {
				t.Fatal("attention hook ran before the canonical store commit")
			}
			captured = payload
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
	})

	t.Run("Should preserve and publish canonical attention when transcript append fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		at := time.Date(2026, 8, 15, 20, 35, 0, 0, time.UTC)
		catalog := newRecordingSessionCatalog()
		if err := catalog.RegisterSession(ctx, store.SessionInfo{
			ID: "sess-transcript-failure", AgentName: "coder", WorkspaceID: "ws-hook",
			SessionType: string(SessionTypeUser), State: string(StateActive),
			CreatedAt: at.Add(-time.Hour), UpdatedAt: at.Add(-time.Minute),
		}); err != nil {
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
	mu        sync.Mutex
	at        time.Time
	committed bool
	listErr   error
}

func (s *committingAttentionStore) GetSessionAttention(
	context.Context,
	string,
) (store.SessionAttention, error) {
	return store.SessionAttention{}, nil
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
	context.Context,
	string,
	[]string,
) ([]store.PendingInteraction, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return []store.PendingInteraction{}, nil
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

func (s *committingAttentionStore) wasCommitted() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.committed
}
