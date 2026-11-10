package session

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
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

		var edges []hookspkg.SessionAttentionChangedPayload
		h.manager.hooks.Attention = attentionHookFunc(
			func(_ context.Context, p hookspkg.SessionAttentionChangedPayload) (hookspkg.SessionAttentionChangedPayload, error) {
				edges = append(edges, p)
				return p, nil
			},
		)
		events, cancel, err := h.manager.SubscribeSessionCatalogEvents(ctx, CatalogScope{
			ReadScope: store.ReadScope{AllProfiles: true}, AllWorkspaces: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		defer cancel()
		seen, err := h.manager.MarkAttentionSeen(ctx, sess.ID)
		if err != nil || seen.LaterReturnedAt != nil || BadgeForInfo(seen) != BadgeStopped {
			t.Fatalf("MarkAttentionSeen(back from Later) = %#v, %v, want a plain stopped session", seen, err)
		}
		if len(edges) != 1 || edges[0].From != string(BadgeNeedsAttention) || edges[0].To != string(BadgeStopped) {
			t.Fatalf("Later dismissal hooks: %+v", edges)
		}
		count := 0
		for len(events) > 0 {
			if event := <-events; event.Attention != nil {
				count++
				if event.Attention.From != BadgeNeedsAttention || event.Attention.To != BadgeStopped {
					t.Fatalf("Later dismissal edge: %+v", event)
				}
			}
		}
		if count != 1 {
			t.Fatalf("Later dismissal events: %d", count)
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
	for _, source := range []string{"early stop", "Later return"} {
		t.Run(
			"Should publish a committed "+source+" dismissal even when the final Status read fails",
			func(t *testing.T) {
				t.Parallel()
				var clockMu sync.Mutex
				now := time.Date(2026, time.November, 1, 12, 0, 0, 0, time.UTC)
				h, db := newMentionHarness(t, WithNow(func() time.Time {
					clockMu.Lock()
					defer clockMu.Unlock()
					return now
				}))
				sess := createSession(t, h)
				wantBadge := BadgeIdle
				if source == "Later return" {
					returnAt := now.Add(time.Hour)
					if _, err := h.manager.MoveToLater(
						t.Context(),
						h.workspaceID,
						sess.ID,
						LaterRequest{ReturnAt: &returnAt},
					); err != nil {
						t.Fatal(err)
					}
					clockMu.Lock()
					now = returnAt
					clockMu.Unlock()
					if returned, err := h.manager.ReturnDueLaterSessions(t.Context()); err != nil || returned != 1 {
						t.Fatalf("return from Later = %d, %v", returned, err)
					}
					wantBadge = BadgeStopped
				} else {
					h.manager.setEndedEarly(t.Context(), sess, "dismiss-status-failure")
				}
				h.manager.removeActive(sess.ID)
				readMeta := h.manager.readSessionMeta
				h.manager.readSessionMeta = func(path string) (store.SessionMeta, error) {
					reader := readMeta
					if reader == nil {
						reader = store.ReadSessionMeta
					}
					meta, err := reader(path)
					meta.Lineage = &store.SessionLineage{
						ParentSessionID: "creator", RootSessionID: "creator", SpawnDepth: 1, NotifyCreator: true,
					}
					return meta, err
				}
				original := h.manager.attentionStore
				wrapped := &dismissalStatusFailureStore{SessionAttentionStore: original}
				h.manager.attentionStore = wrapped
				t.Cleanup(func() {
					h.manager.attentionStore = original
					h.manager.mu.Lock()
					h.manager.sessions[sess.ID] = sess
					h.manager.mu.Unlock()
					reportSessionStop(t, h, sess.ID)
				})
				var edges []hookspkg.SessionAttentionChangedPayload
				h.manager.hooks.Attention = attentionHookFunc(
					func(_ context.Context, p hookspkg.SessionAttentionChangedPayload) (hookspkg.SessionAttentionChangedPayload, error) {
						edges = append(edges, p)
						return p, nil
					},
				)
				var wakes atomic.Int32
				h.manager.spawnWakeNotifier = attentionWakeFunc(
					func(context.Context, string, SpawnWakeEvent) error { wakes.Add(1); return nil },
				)
				events, cancel, err := h.manager.SubscribeSessionCatalogEvents(t.Context(), CatalogScope{
					ReadScope: store.ReadScope{AllProfiles: true}, AllWorkspaces: true,
				})
				if err != nil {
					t.Fatal(err)
				}
				defer cancel()
				if _, err := h.manager.MarkAttentionSeen(
					t.Context(),
					sess.ID,
				); !errors.Is(
					err,
					errDismissalStatusRead,
				) {
					t.Fatalf("dismissal error = %v, want post-commit read failure", err)
				}
				canonical, err := db.GetSessionAttention(t.Context(), sess.ID)
				if err != nil {
					t.Fatal(err)
				}
				if canonical.EndedEarlyTurnID != "" || canonical.LaterReturnedAt != nil {
					t.Fatalf("dismissal did not commit: %+v", canonical)
				}
				if len(edges) != 1 {
					t.Fatalf("committed dismissal hooks before retry: %+v", edges)
				}
				seen, err := h.manager.MarkAttentionSeen(t.Context(), sess.ID)
				if err != nil || seen.EndedEarlyTurnID != "" || seen.LaterReturnedAt != nil {
					t.Fatalf("retry = %+v, %v", seen, err)
				}
				count := 0
				for len(events) > 0 {
					if event := <-events; event.Attention != nil {
						count++
						if event.Attention.From != BadgeNeedsAttention || event.Attention.To != wantBadge {
							t.Fatalf("dismissal event: %+v", event)
						}
					}
				}
				if count != 1 || len(edges) != 1 || edges[0].From != string(BadgeNeedsAttention) ||
					edges[0].To != string(wantBadge) || wakes.Load() != 0 {
					t.Fatalf("dismissal events=%d hooks=%+v wakes=%d", count, edges, wakes.Load())
				}
			},
		)
	}

	t.Run("Should publish a nonresident dismissal without another catalog snapshot", func(t *testing.T) {
		t.Parallel()
		h, db := newMentionHarness(t)
		sess := createSession(t, h)
		h.manager.setEndedEarly(t.Context(), sess, "dismiss-nonresident")
		h.manager.removeActive(sess.ID)
		t.Cleanup(func() {
			h.manager.sessionCatalog = db
			h.manager.mu.Lock()
			h.manager.sessions[sess.ID] = sess
			h.manager.mu.Unlock()
			reportSessionStop(t, h, sess.ID)
		})
		h.manager.sessionCatalog = &dismissalSnapshotFailureCatalog{SessionCatalog: db}
		var edges []hookspkg.SessionAttentionChangedPayload
		h.manager.hooks.Attention = attentionHookFunc(
			func(_ context.Context, p hookspkg.SessionAttentionChangedPayload) (hookspkg.SessionAttentionChangedPayload, error) {
				edges = append(edges, p)
				return p, nil
			},
		)
		events, cancel, err := h.manager.SubscribeSessionCatalogEvents(t.Context(), CatalogScope{
			ReadScope: store.ReadScope{AllProfiles: true}, AllWorkspaces: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		defer cancel()
		for range 2 {
			seen, err := h.manager.MarkAttentionSeen(t.Context(), sess.ID)
			if err != nil || seen.EndedEarlyTurnID != "" {
				t.Fatalf("dismissal: %+v, %v", seen, err)
			}
			h.manager.sessionCatalog = db
		}
		if len(edges) != 1 || edges[0].From != string(BadgeNeedsAttention) || edges[0].To != string(BadgeIdle) {
			t.Fatalf("nonresident dismissal hooks: %+v", edges)
		}
		count := 0
		for len(events) > 0 {
			if event := <-events; event.Attention != nil {
				count++
			}
		}
		canonical, err := db.GetSessionAttention(t.Context(), sess.ID)
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 || canonical.EndedEarlyTurnID != "" {
			t.Fatalf("dismissal state: count=%d canonical=%+v", count, canonical)
		}
	})
	for _, publishedQuestion := range []bool{false, true} {
		name := "Should retain the first dismissal baseline before delayed presence publication"
		if publishedQuestion {
			name = "Should dismiss from the published question instead of the unpublished early stop"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h, _ := newMentionHarness(t)
			sess := createSession(t, h)
			t.Cleanup(func() { reportSessionStop(t, h, sess.ID) })
			ctx := t.Context()
			if publishedQuestion {
				question, err := h.manager.createPendingInteraction(ctx, store.PendingInteractionCreate{
					SessionID: sess.ID, InteractionID: "dismiss-question", ProviderRequestID: "dismiss-question",
					Kind: store.PendingInteractionKindClarify,
				})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := h.manager.commitTransitionPendingInteraction(
					ctx,
					sess.ID,
					store.PendingInteractionTransition{
						InteractionID: question.Interaction.InteractionID,
						Status:        store.PendingInteractionStatusCanceled,
						At:            h.manager.now(),
					},
				); err != nil {
					t.Fatal(err)
				}
			}
			h.manager.setEndedEarly(ctx, sess, "unpublished-turn")
			// Commit presence without publishing, then restore a dismissible occurrence.
			delayed, err := h.manager.commitAttention(ctx, sess.ID, func() (store.SessionAttentionCommit, error) {
				return h.manager.attentionStore.MarkSessionSeen(ctx, sess.ID, h.manager.now())
			})
			if err != nil {
				t.Fatal(err)
			}
			h.manager.setEndedEarly(ctx, sess, "dismiss-turn")
			var edges []hookspkg.SessionAttentionChangedPayload
			h.manager.hooks.Attention = attentionHookFunc(
				func(_ context.Context, p hookspkg.SessionAttentionChangedPayload) (hookspkg.SessionAttentionChangedPayload, error) {
					edges = append(edges, p)
					return p, nil
				},
			)
			events, cancel, err := h.manager.SubscribeSessionCatalogEvents(ctx, CatalogScope{
				ReadScope: store.ReadScope{AllProfiles: true}, AllWorkspaces: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			defer cancel()
			sess.mu.Lock()
			originalLineage := sess.Lineage
			sess.Lineage = &store.SessionLineage{
				ParentSessionID: "creator",
				RootSessionID:   "creator",
				SpawnDepth:      1,
				NotifyCreator:   true,
			}
			sess.mu.Unlock()
			t.Cleanup(func() { sess.mu.Lock(); sess.Lineage = originalLineage; sess.mu.Unlock() })
			wakes := 0
			h.manager.spawnWakeNotifier = attentionWakeFunc(
				func(context.Context, string, SpawnWakeEvent) error { wakes++; return nil },
			)
			if _, err := h.manager.MarkAttentionSeen(ctx, sess.ID); err != nil {
				t.Fatal(err)
			}
			h.manager.publishAttentionCommit(ctx, sess.ID, delayed)
			from := BadgeNeedsAttention
			if publishedQuestion {
				from = BadgeWaitingForInput
			}
			if len(edges) != 1 || edges[0].From != string(from) || edges[0].To != string(BadgeIdle) {
				t.Fatalf("dismissal edges: %+v", edges)
			}
			count := 0
			for len(events) > 0 {
				if event := <-events; event.Attention != nil {
					count++
					if event.Attention.From != from || event.Attention.To != BadgeIdle {
						t.Fatalf("catalog edge: %+v", event)
					}
				}
			}
			canonical, err := h.manager.attentionStore.GetSessionAttention(ctx, sess.ID)
			if err != nil {
				t.Fatal(err)
			}
			if count != 1 || wakes != 0 || canonical.EndedEarlyTurnID != "" || BadgeForInfo(sess.Info()) != BadgeIdle {
				t.Fatalf("dismissal final state: events=%d wakes=%d canonical=%+v", count, wakes, canonical)
			}
		})
	}
	t.Run("Should canonicalize padded reconciliation keys and forget them on removal", func(t *testing.T) {
		t.Parallel()
		h, _ := newMentionHarness(t)
		sess := createSession(t, h)
		ctx := t.Context()
		commit, err := h.manager.commitCreatePendingInteraction(ctx, store.PendingInteractionCreate{
			SessionID:         sess.ID,
			InteractionID:     "padded",
			ProviderRequestID: "padded",
			Kind:              store.PendingInteractionKindClarify,
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
		padded := commit
		padded.Interaction = new(*commit.Interaction)
		padded.Interaction.SessionID = "  " + sess.ID + "  "
		publishAttentionCommitsAndWait(t, h.manager, []store.SessionAttentionCommit{padded, commit})
		publishAttentionCommitsAndWait(t, h.manager, []store.SessionAttentionCommit{padded})
		if len(edges) != 1 || edges[0].From != string(BadgeIdle) || edges[0].To != string(BadgeWaitingForInput) {
			t.Fatalf("padded reconciliation edges: %+v", edges)
		}
		canonical, err := h.manager.attentionStore.GetSessionAttention(ctx, sess.ID)
		if err != nil {
			t.Fatal(err)
		}
		if canonical.PendingClarifyCount != 1 || len(sess.Info().PendingInteractions) != 1 {
			t.Fatalf("projection mismatch: %+v", canonical)
		}
		reportSessionStop(t, h, sess.ID)
		h.manager.remove(sess.ID)
		h.manager.attentionCommitMu.Lock()
		defer h.manager.attentionCommitMu.Unlock()
		if len(h.manager.attentionPublished) != 0 {
			t.Fatalf("retained baselines: %+v", h.manager.attentionPublished)
		}
	})
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

	t.Run("Should not recreate a removed publication baseline from captured lifecycle info", func(t *testing.T) {
		t.Parallel()
		h, _ := newMentionHarness(t)
		sess := createSession(t, h)
		if _, err := h.manager.createPendingInteraction(t.Context(), store.PendingInteractionCreate{
			SessionID:         sess.ID,
			InteractionID:     "removal",
			ProviderRequestID: "removal",
			Kind:              store.PendingInteractionKindClarify,
		}); err != nil {
			t.Fatal(err)
		}
		reportSessionStop(t, h, sess.ID)
		captured := sess.Info()
		h.manager.remove(sess.ID)
		h.manager.publishLifecycleAttentionTransition(t.Context(), captured, captured)
		h.manager.attentionCommitMu.Lock()
		_, retained := h.manager.attentionPublished[sess.ID]
		h.manager.attentionCommitMu.Unlock()
		if retained {
			t.Fatal("publication recreated removed baseline")
		}
	})
	t.Run("Should finish dismissal publication before a newer permission publishes", func(t *testing.T) {
		t.Parallel()
		h, _ := newMentionHarness(t)
		sess := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, sess.ID) })
		h.manager.setEndedEarly(t.Context(), sess, "dismissed-turn")
		entered, release := make(chan struct{}), make(chan struct{})
		var once sync.Once
		t.Cleanup(func() { once.Do(func() { close(release) }) })
		h.manager.hooks.Attention = attentionHookFunc(
			func(_ context.Context, p hookspkg.SessionAttentionChangedPayload) (hookspkg.SessionAttentionChangedPayload, error) {
				if p.To == string(BadgeIdle) {
					close(entered)
					<-release
				}
				return p, nil
			},
		)
		wrapped := &pausedDismissStore{SessionAttentionStore: h.manager.attentionStore,
			committed: make(chan struct{}), release: make(chan struct{}), entered: make(chan struct{}, 1)}
		close(wrapped.release)
		h.manager.attentionStore = wrapped
		dismissed := make(chan error, 1)
		go func() { _, err := h.manager.MarkAttentionSeen(t.Context(), sess.ID); dismissed <- err }()
		<-entered
		created := make(chan error, 1)
		go func() {
			_, err := h.manager.createPendingInteraction(t.Context(), store.PendingInteractionCreate{
				SessionID:         sess.ID,
				InteractionID:     "after-dismiss",
				ProviderRequestID: "after-dismiss",
				Kind:              store.PendingInteractionKindPermission,
			})
			created <- err
		}()
		select {
		case <-wrapped.entered:
			once.Do(func() { close(release) })
			<-dismissed
			<-created
			t.Fatal("permission overtook dismissal publication")
		case <-time.After(100 * time.Millisecond):
		}
		once.Do(func() { close(release) })
		if err := <-dismissed; err != nil {
			t.Fatal(err)
		}
		if err := <-created; err != nil {
			t.Fatal(err)
		}
		if got := BadgeForInfo(sess.Info()); got != BadgeWaitingForAuth {
			t.Fatalf("new permission badge = %s", got)
		}
	})

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

// dismissalSnapshotFailureCatalog rejects a redundant publication read after Status has succeeded.
type dismissalSnapshotFailureCatalog struct {
	store.SessionCatalog
}

func (c *dismissalSnapshotFailureCatalog) ListSessions(
	ctx context.Context,
	query store.SessionListQuery,
) ([]store.SessionInfo, error) {
	if query.Limit == 1 {
		return nil, errors.New("injected post-dismissal snapshot failure")
	}
	return c.SessionCatalog.ListSessions(ctx, query)
}

var errDismissalStatusRead = errors.New("injected post-commit Status read failure")

type dismissalStatusFailureStore struct {
	store.SessionAttentionStore
	fail atomic.Bool
}

func (s *dismissalStatusFailureStore) DismissSessionAttention(
	ctx context.Context, id string, fence store.SessionAttentionFence, at time.Time,
) (store.SessionAttentionCommit, error) {
	commit, err := s.SessionAttentionStore.DismissSessionAttention(ctx, id, fence, at)
	if err == nil {
		s.fail.Store(true)
	}
	return commit, err
}

func (s *dismissalStatusFailureStore) GetSessionAttention(
	ctx context.Context,
	id string,
) (store.SessionAttention, error) {
	if s.fail.Swap(false) {
		return store.SessionAttention{}, errDismissalStatusRead
	}
	return s.SessionAttentionStore.GetSessionAttention(ctx, id)
}
