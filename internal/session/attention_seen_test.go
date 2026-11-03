package session

import (
	"errors"
	"sync"
	"testing"
	"time"

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
		var refusal *AttentionRequiresActionError
		if !errors.Is(err, ErrAttentionRequiresAction) || !errors.As(err, &refusal) ||
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
