package session

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store"
)

// Invariant: marking Needs you as seen clears only the operator-dismissible sources
// (a turn that ended early, a return from Later, a stopped session's failure), the
// next occurrence raises it again, and a session still waiting on the operator is
// refused with the blocker instead of being hidden.
// Owner: session attention (manager).
func TestManagerMarkAttentionSeen(t *testing.T) {
	t.Parallel()

	t.Run("Should clear an early stop until the next turn ends early", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		h, _ := newMentionHarness(t)
		sess := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, sess.ID) })
		h.manager.setEndedEarly(ctx, sess, "turn-early")
		info, err := h.manager.Status(ctx, sess.ID)
		if err != nil || BadgeForInfo(info) != BadgeNeedsAttention || !AttentionDismissible(info) {
			t.Fatalf("Status(ended early) = %#v, %v, want a dismissible needs-attention", info, err)
		}

		seen, err := h.manager.MarkAttentionSeen(ctx, sess.ID)
		if err != nil {
			t.Fatalf("MarkAttentionSeen() error = %v", err)
		}
		if got := BadgeForInfo(seen); ClassForBadge(got) != AttentionNone || seen.EndedEarlyTurnID != "" {
			t.Fatalf("MarkAttentionSeen() badge %q ended_early %q, want out of Needs you", got, seen.EndedEarlyTurnID)
		}
		again, err := h.manager.MarkAttentionSeen(ctx, sess.ID)
		if err != nil || again.AttentionRevision != seen.AttentionRevision {
			t.Fatalf("MarkAttentionSeen(repeat) = %#v, %v, want an unchanged no-op", again, err)
		}

		h.manager.setEndedEarly(ctx, sess, "turn-later")
		info, err = h.manager.Status(ctx, sess.ID)
		if err != nil || BadgeForInfo(info) != BadgeNeedsAttention {
			t.Fatalf("Status(next early stop) = %#v, %v, want Needs you raised again", info, err)
		}
	})

	t.Run("Should refuse a session that still waits on an answer", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		h, database := newMentionHarness(t)
		sess := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, sess.ID) })
		h.manager.setEndedEarly(ctx, sess, "turn-early")
		commit, err := database.CreatePendingInteraction(ctx, store.PendingInteractionCreate{
			InteractionID: "clarify-1", SessionID: sess.ID, Kind: store.PendingInteractionKindClarify,
			ProviderRequestID: "request-1", TurnID: "turn-early", Title: "Which branch?",
			CreatedAt: time.Now().UTC(),
		})
		if err != nil {
			t.Fatalf("CreatePendingInteraction() error = %v", err)
		}
		h.manager.applyAttentionProjection(sess.ID, commit.After)

		_, err = h.manager.MarkAttentionSeen(ctx, sess.ID)
		refusal, refused := errors.AsType[*AttentionRequiresActionError](err)
		if !errors.Is(err, ErrAttentionRequiresAction) || !refused ||
			refusal.Blocker != AttentionBlockerPendingClarify {
			t.Fatalf("MarkAttentionSeen(pending clarify) error = %v, want a pending_clarify refusal", err)
		}
		info, err := h.manager.Status(ctx, sess.ID)
		if err != nil || BadgeForInfo(info) != BadgeWaitingForInput || info.EndedEarlyTurnID != "turn-early" {
			t.Fatalf("Status(after refusal) = %#v, %v, want the question and the early stop kept", info, err)
		}
	})

	t.Run("Should clear a return from Later", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		var clockMu sync.Mutex
		now := time.Now().UTC()
		clock := func() time.Time { clockMu.Lock(); defer clockMu.Unlock(); return now }
		h, _ := newMentionHarness(t, WithNow(clock))
		sess := createSession(t, h)
		returnAt := now.Add(time.Hour)
		if _, err := h.manager.MoveToLater(ctx, h.workspaceID, sess.ID, LaterRequest{ReturnAt: &returnAt}); err != nil {
			t.Fatalf("MoveToLater() error = %v", err)
		}
		clockMu.Lock()
		now = returnAt
		clockMu.Unlock()
		if returned, err := h.manager.ReturnDueLaterSessions(ctx); err != nil || returned != 1 {
			t.Fatalf("ReturnDueLaterSessions() = %d, %v, want the session back", returned, err)
		}

		seen, err := h.manager.MarkAttentionSeen(ctx, sess.ID)
		if err != nil || seen.LaterReturnedAt != nil || BadgeForInfo(seen) != BadgeStopped {
			t.Fatalf("MarkAttentionSeen(back from Later) = %#v, %v, want a plain stopped session", seen, err)
		}
	})
}

// Invariant: the dismissibility verdict follows CanonicalBadge precedence, so only
// a Needs you caused by a dismissible source can be marked as seen.
// Owner: session attention (badge contract).
func TestAttentionBlockerForInfo(t *testing.T) {
	t.Parallel()

	failure := &store.SessionFailure{Kind: store.FailureProcess}
	authFailure := &store.SessionFailure{Kind: store.FailureProviderAuth}
	tests := []struct {
		name        string
		info        Info
		want        AttentionBlocker
		dismissible bool
	}{
		{
			name:        "Should dismiss a turn that ended early",
			info:        Info{State: StateActive, EndedEarlyTurnID: "turn-1"},
			dismissible: true,
		},
		{
			name:        "Should dismiss a stopped failure, including a sign-in failure",
			info:        Info{State: StateStopped, Failure: authFailure},
			dismissible: true,
		},
		{
			name:        "Should dismiss a return from Later",
			info:        Info{State: StateStopped, LaterReturnedAt: new(time.Now())},
			dismissible: true,
		},
		{
			name: "Should refuse a pending permission",
			info: Info{State: StateActive, PendingPermissionCount: 1, EndedEarlyTurnID: "turn-1"},
			want: AttentionBlockerPendingPermission,
		},
		{
			name: "Should refuse a stopped failure with a pending permission",
			info: Info{State: StateStopped, Failure: failure, PendingPermissionCount: 1},
			want: AttentionBlockerPendingPermission,
		},
		{
			name: "Should refuse a stopped failure with a pending question",
			info: Info{State: StateStopped, Failure: authFailure, PendingClarifyCount: 1},
			want: AttentionBlockerPendingClarify,
		},
		{
			name: "Should refuse a live sign-in failure",
			info: Info{State: StateActive, Failure: authFailure},
			want: AttentionBlockerProviderAuth,
		},
		{
			name: "Should refuse an unverified stop",
			info: Info{State: StateActive, StopVerificationFailed: true, Failure: failure},
			want: AttentionBlockerStopUnverified,
		},
		{
			name: "Should refuse supervision evidence that needs attention",
			info: Info{State: StateActive, Supervision: &SupervisionState{
				Sources: []SignalSourceState{{Kind: WorkSignalActiveMonitor, State: signalSourceUnknown}},
			}},
			want: AttentionBlockerSupervision,
		},
		{
			name: "Should report nothing to mark for an idle session",
			info: Info{State: StateActive},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			info := test.info
			if got := AttentionBlockerForInfo(&info); got != test.want {
				t.Fatalf("AttentionBlockerForInfo() = %q, want %q", got, test.want)
			}
			if got := AttentionDismissible(&info); got != test.dismissible {
				t.Fatalf("AttentionDismissible() = %v, want %v", got, test.dismissible)
			}
		})
	}
}

// Invariant: resident applies and published attention edges preserve canonical session ordering.
// Owner: session manager; canonical suite: TestManagerAttentionCommitOrdering.
func TestManagerAttentionCommitOrdering(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"early", store.PendingInteractionKindPermission, store.PendingInteractionKindClarify} {
		t.Run("Should preserve newer "+kind+" attention after dismissal", func(t *testing.T) {
			t.Parallel()
			h, _ := newMentionHarness(t)
			sess := createSession(t, h)
			t.Cleanup(func() { reportSessionStop(t, h, sess.ID) })
			h.manager.setEndedEarly(t.Context(), sess, "old-turn")
			wrapped := &pausedDismissStore{
				SessionAttentionStore: h.manager.attentionStore,
				committed:             make(chan struct{}),
				release:               make(chan struct{}),
				entered:               make(chan struct{}, 1),
			}
			h.manager.attentionStore = wrapped
			dismissed := make(chan error, 1)
			go func() { _, err := h.manager.MarkAttentionSeen(t.Context(), sess.ID); dismissed <- err }()
			<-wrapped.committed
			newer := make(chan error, 1)
			go func() {
				if kind == "early" {
					h.manager.setEndedEarly(t.Context(), sess, "new-turn")
					newer <- nil
					return
				}
				_, err := h.manager.createPendingInteraction(t.Context(), store.PendingInteractionCreate{
					SessionID: sess.ID, InteractionID: "new-interaction", Kind: kind, ProviderRequestID: "request",
				})
				newer <- err
			}()
			select {
			case <-wrapped.entered:
				close(wrapped.release)
				<-dismissed
				<-newer
				t.Fatal("new attention write overtook the dismissal apply")
			case <-time.After(100 * time.Millisecond):
			}
			close(wrapped.release)
			if err := <-dismissed; err != nil {
				t.Fatal(err)
			}
			if err := <-newer; err != nil {
				t.Fatal(err)
			}
			info, err := h.manager.Status(t.Context(), sess.ID)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "early" && info.EndedEarlyTurnID != "new-turn" ||
				kind == store.PendingInteractionKindPermission && info.PendingPermissionCount != 1 ||
				kind == store.PendingInteractionKindClarify && info.PendingClarifyCount != 1 {
				t.Fatalf("newer attention was lost: %#v", info)
			}
		})
	}
	t.Run("Should preserve newer presence after a committed permission transition", func(t *testing.T) {
		t.Parallel()
		h, _ := newMentionHarness(t)
		sess := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, sess.ID) })
		original := h.manager.attentionStore
		created, err := h.manager.createPendingInteraction(t.Context(), store.PendingInteractionCreate{
			SessionID: sess.ID, InteractionID: "resolve-presence", Kind: store.PendingInteractionKindPermission,
			ProviderRequestID: "resolve-presence",
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := original.MarkSessionSettled(t.Context(), sess.ID, false, h.manager.now()); err != nil {
			t.Fatal(err)
		}
		wrapped := &pausedTransitionStore{
			SessionAttentionStore: original,
			committed:             make(chan struct{}),
			release:               make(chan struct{}),
			entered:               make(chan struct{}, 1),
		}
		h.manager.attentionStore = wrapped
		t.Cleanup(func() { h.manager.attentionStore = original })
		transitioned := make(chan error, 1)
		go func() {
			_, err := h.manager.commitTransitionPendingInteraction(
				t.Context(),
				sess.ID,
				store.PendingInteractionTransition{
					InteractionID: created.Interaction.InteractionID,
					Status:        store.PendingInteractionStatusResolved,
					Resolution:    "allow",
					ResolvedBy:    "operator",
				},
			)
			transitioned <- err
		}()
		<-wrapped.committed
		seen := make(chan error, 1)
		go func() { _, err := h.manager.SessionPresence(t.Context(), sess.ID, "", true); seen <- err }()
		select {
		case <-wrapped.entered:
			close(wrapped.release)
			<-transitioned
			<-seen
			t.Fatal("presence overtook transition resident apply")
		case <-time.After(100 * time.Millisecond):
		}
		close(wrapped.release)
		if err := <-transitioned; err != nil {
			t.Fatal(err)
		}
		if err := <-seen; err != nil {
			t.Fatal(err)
		}
		durable, err := original.GetSessionAttention(t.Context(), sess.ID)
		if err != nil {
			t.Fatal(err)
		}
		info := sess.Info()
		if durable.LastSeenRevision == 0 || info.AttentionRevision != durable.AttentionRevision ||
			info.LastSeenRevision != durable.LastSeenRevision {
			t.Fatalf("resident=%+v durable=%+v", info, durable)
		}
	})
	t.Run("Should suppress an older ordinary publication after acknowledgement cleanup", func(t *testing.T) {
		t.Parallel()
		h, db := newMentionHarness(t)
		if err := db.RegisterSession(t.Context(), &store.SessionInfo{
			ID: "ordered", ProfileID: store.DefaultProfileID, WorkspaceID: h.workspaceID,
			AgentName: "coder", SessionType: string(SessionTypeUser), State: string(StateStopped),
			RuntimeStatus: store.SessionRuntimeReady,
			CreatedAt:     h.manager.now(), UpdatedAt: h.manager.now(),
		}); err != nil {
			t.Fatal(err)
		}
		ctx := t.Context()
		permission, err := h.manager.createPendingInteraction(ctx, store.PendingInteractionCreate{
			SessionID: "ordered", InteractionID: "permission", ProviderRequestID: "permission",
			Kind: store.PendingInteractionKindPermission,
		})
		if err != nil {
			t.Fatal(err)
		}
		clarify, err := h.manager.createPendingInteraction(ctx, store.PendingInteractionCreate{
			SessionID: "ordered", InteractionID: "clarify", ProviderRequestID: "clarify",
			Kind: store.PendingInteractionKindClarify,
		})
		if err != nil {
			t.Fatal(err)
		}
		// Pause at the existing store boundary after the ordinary commit, before publication.
		older, err := h.manager.commitTransitionPendingInteraction(ctx, "ordered", store.PendingInteractionTransition{
			InteractionID: permission.Interaction.InteractionID,
			Status:        store.PendingInteractionStatusCanceled,
			At:            h.manager.now(),
		})
		if err != nil {
			t.Fatal(err)
		}
		cleared, err := db.TransitionPendingInteraction(ctx, store.PendingInteractionTransition{
			InteractionID: clarify.Interaction.InteractionID,
			Status:        store.PendingInteractionStatusCanceled,
			At:            h.manager.now(),
		})
		if err != nil {
			t.Fatal(err)
		}
		var edges []hookspkg.SessionAttentionChangedPayload
		h.manager.hooks.Attention = attentionHookFunc(
			func(_ context.Context, p hookspkg.SessionAttentionChangedPayload) (hookspkg.SessionAttentionChangedPayload, error) {
				edges = append(edges, p)
				return p, nil
			},
		)
		publishAttentionCommitsAndWait(t, h.manager, []store.SessionAttentionCommit{cleared})
		h.manager.publishAttentionCommit(ctx, "ordered", older)
		if len(edges) != 1 || edges[0].From != string(BadgeWaitingForAuth) || edges[0].To != string(BadgeStopped) {
			t.Fatalf("late ordinary publication regressed attention: %+v", edges)
		}
	})
	t.Run(
		"Should avoid duplicate hooks when a new permission publishes before reconciliation locks",
		func(t *testing.T) {
			t.Parallel()
			h, db := newMentionHarness(t)
			registerMentionCatalogSession(t, db, "newer-first", h.workspaceID)
			ctx := t.Context()
			old, err := h.manager.createPendingInteraction(ctx, store.PendingInteractionCreate{
				SessionID:         "newer-first",
				InteractionID:     "old",
				ProviderRequestID: "old",
				Kind:              store.PendingInteractionKindClarify,
			})
			if err != nil {
				t.Fatal(err)
			}
			cleared, err := db.TransitionPendingInteraction(ctx, store.PendingInteractionTransition{
				InteractionID: old.Interaction.InteractionID,
				Status:        store.PendingInteractionStatusCanceled,
				At:            h.manager.now(),
			})
			if err != nil {
				t.Fatal(err)
			}
			var edges []hookspkg.SessionAttentionChangedPayload
			h.manager.hooks.Attention = attentionHookFunc(
				func(_ context.Context, p hookspkg.SessionAttentionChangedPayload) (hookspkg.SessionAttentionChangedPayload, error) {
					edges = append(edges, p)
					return p, nil
				},
			)
			_, err = h.manager.createPendingInteraction(ctx, store.PendingInteractionCreate{
				SessionID:         "newer-first",
				InteractionID:     "new",
				ProviderRequestID: "new",
				Kind:              store.PendingInteractionKindPermission,
			})
			if err != nil {
				t.Fatal(err)
			}
			publishAttentionCommitsAndWait(t, h.manager, []store.SessionAttentionCommit{cleared})
			if len(edges) != 1 || edges[0].From != string(BadgeWaitingForInput) ||
				edges[0].To != string(BadgeWaitingForAuth) {
				t.Fatalf("reconciliation duplicated or invented transition: %+v", edges)
			}
		},
	)

	t.Run("Should cancel a same-session lock waiter with Manager shutdown", func(t *testing.T) {
		t.Parallel()
		manager, err := NewManager(WithHomePaths(testHomePaths(t)))
		if err != nil {
			t.Fatal(err)
		}
		cleanupTestManager(t, manager)
		unlock, err := manager.lockAttentionCommit(t.Context(), "locked")
		if err != nil {
			t.Fatal(err)
		}
		defer unlock()
		waiting := make(chan error, 1)
		go func() {
			release, err := manager.lockAttentionCommit(manager.attentionReconciliation.ctx, "locked")
			if err == nil {
				release()
			}
			waiting <- err
		}()
		if err := manager.Shutdown(t.Context()); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-waiting:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled lock wait=%v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("shutdown did not release lock waiter")
		}
	})
}

type pausedDismissStore struct {
	store.SessionAttentionStore
	committed, release chan struct{}
	entered            chan struct{}
}

func (s *pausedDismissStore) DismissSessionAttention(
	ctx context.Context,
	id string,
	fence store.SessionAttentionFence,
	at time.Time,
) (store.SessionAttentionCommit, error) {
	commit, err := s.SessionAttentionStore.DismissSessionAttention(ctx, id, fence, at)
	close(s.committed)
	<-s.release
	return commit, err
}
func (s *pausedDismissStore) MarkSessionEndedEarly(
	ctx context.Context,
	id, turn string,
) (store.SessionAttentionCommit, error) {
	s.entered <- struct{}{}
	return s.SessionAttentionStore.MarkSessionEndedEarly(ctx, id, turn)
}
func (s *pausedDismissStore) CreatePendingInteraction(
	ctx context.Context,
	req store.PendingInteractionCreate,
) (store.SessionAttentionCommit, error) {
	s.entered <- struct{}{}
	return s.SessionAttentionStore.CreatePendingInteraction(ctx, req)
}

// pausedTransitionStore holds the I/O completion before the Manager applies its resident projection.
type pausedTransitionStore struct {
	store.SessionAttentionStore
	committed, release, entered chan struct{}
}

func (s *pausedTransitionStore) TransitionPendingInteraction(
	ctx context.Context,
	req store.PendingInteractionTransition,
) (store.SessionAttentionCommit, error) {
	commit, err := s.SessionAttentionStore.TransitionPendingInteraction(ctx, req)
	close(s.committed)
	<-s.release
	return commit, err
}

func (s *pausedTransitionStore) MarkSessionSeen(
	ctx context.Context,
	id string,
	at time.Time,
) (store.SessionAttentionCommit, error) {
	s.entered <- struct{}{}
	return s.SessionAttentionStore.MarkSessionSeen(ctx, id, at)
}
