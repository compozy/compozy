package session

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestManagerAttentionHookRunsAfterCanonicalCommitAndFailsOpen(t *testing.T) {
	t.Parallel()

	for _, badgeChanges := range []bool{false, true} {
		name := "Should ignore a late same-badge supervision attention snapshot"
		if badgeChanges {
			name = "Should ignore a late badge-changing supervision attention snapshot"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h, db := newMentionHarness(t)
			sess := createSession(t, h)
			t.Cleanup(func() { reportSessionStop(t, h, sess.ID) })
			ctx := t.Context()
			created, err := h.manager.createPendingInteraction(ctx, store.PendingInteractionCreate{
				SessionID: sess.ID, InteractionID: "late-permission", ProviderRequestID: "late-permission",
				Kind: store.PendingInteractionKindPermission,
			})
			if err != nil {
				t.Fatal(err)
			}
			// Supervision captured these values before the operator answered.
			before, after := sess.Info(), sess.Info()
			if badgeChanges {
				before.PendingPermissionCount = 0
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
			if _, err := h.manager.transitionPendingInteraction(ctx, sess.ID, store.PendingInteractionTransition{
				InteractionID: created.Interaction.InteractionID, Status: store.PendingInteractionStatusResolved,
				Resolution: "allow", ResolvedBy: "operator",
			}); err != nil {
				t.Fatal(err)
			}
			h.manager.publishLifecycleAttentionTransitionWake(ctx, before, after, true, "")
			if _, err := h.manager.SessionPresence(ctx, sess.ID, "", true); err != nil {
				t.Fatal(err)
			}
			if len(edges) != 1 || edges[0].From != string(BadgeWaitingForAuth) || edges[0].To != string(BadgeIdle) {
				t.Fatalf("late lifecycle repeated clearing hook: %+v", edges)
			}
			canonical, err := db.GetSessionAttention(ctx, sess.ID)
			if err != nil {
				t.Fatal(err)
			}
			clears := 0
			for len(events) > 0 {
				event := <-events
				if event.Attention != nil {
					clears++
					if event.Attention.From != BadgeWaitingForAuth || event.Attention.To != BadgeIdle {
						t.Fatalf("stale catalog edge: %+v", event)
					}
				}
			}
			if clears != 1 || canonical.PendingPermissionCount != 0 || sess.Info().PendingPermissionCount != 0 {
				t.Fatalf("catalog/storage mismatch: clears=%d canonical=%+v", clears, canonical)
			}
		})
	}

	t.Run("Should invalidate the catalog after refreshing an already published acknowledgement", func(t *testing.T) {
		t.Parallel()
		h, db := newMentionHarness(t)
		sess := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, sess.ID) })
		old, err := h.manager.createPendingInteraction(t.Context(), store.PendingInteractionCreate{
			SessionID:         sess.ID,
			InteractionID:     "old-question",
			ProviderRequestID: "old-question",
			Kind:              store.PendingInteractionKindClarify,
		})
		if err != nil {
			t.Fatal(err)
		}
		cleared, err := db.TransitionPendingInteraction(t.Context(), store.PendingInteractionTransition{
			InteractionID: old.Interaction.InteractionID,
			Status:        store.PendingInteractionStatusCanceled,
			At:            h.manager.now(),
		})
		if err != nil {
			t.Fatal(err)
		}
		h.manager.publishAttentionCommit(t.Context(), sess.ID, cleared)
		if len(sess.Info().PendingInteractions) != 1 {
			t.Fatal("schedule did not retain the pre-refresh interaction list")
		}
		events, cancel, err := h.manager.SubscribeSessionCatalogEvents(t.Context(), CatalogScope{
			ReadScope: store.ReadScope{AllProfiles: true}, AllWorkspaces: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		defer cancel()
		publishAttentionCommitsAndWait(t, h.manager, []store.SessionAttentionCommit{cleared})
		assertCatalogEventName(t, events, CatalogEventNameChanged)
		if len(sess.Info().PendingInteractions) != 0 {
			t.Fatal("catalog invalidation preceded the canonical list refresh")
		}
		select {
		case extra := <-events:
			t.Fatalf("unchanged projection emitted extra edge: %+v", extra)
		default:
		}
	})

	t.Run("Should clear an unseen completed turn after an earlier permission publication", func(t *testing.T) {
		t.Parallel()
		h, _ := newMentionHarness(t)
		sess := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, sess.ID) })
		ctx := t.Context()
		created, err := h.manager.createPendingInteraction(ctx, store.PendingInteractionCreate{
			SessionID: sess.ID, InteractionID: "first-permission", ProviderRequestID: "first-permission",
			Kind: store.PendingInteractionKindPermission,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := h.manager.transitionPendingInteraction(ctx, sess.ID, store.PendingInteractionTransition{
			InteractionID: created.Interaction.InteractionID, Status: store.PendingInteractionStatusResolved,
			Resolution: "allow", ResolvedBy: "operator",
		}); err != nil {
			t.Fatal(err)
		}
		before := sess.Info()
		if err := h.manager.settleSessionAttention(ctx, sess.ID, h.manager.now()); err != nil {
			t.Fatal(err)
		}
		h.manager.publishLifecycleAttentionTransitionWake(ctx, before, sess.Info(), true, "first-turn")
		var edges []hookspkg.SessionAttentionChangedPayload
		h.manager.hooks.Attention = attentionHookFunc(
			func(_ context.Context, p hookspkg.SessionAttentionChangedPayload) (hookspkg.SessionAttentionChangedPayload, error) {
				edges = append(edges, p)
				return p, nil
			},
		)
		if _, err := h.manager.SessionPresence(ctx, sess.ID, "", true); err != nil {
			t.Fatal(err)
		}
		if len(edges) != 1 || edges[0].From != string(BadgeDone) || edges[0].To != string(BadgeIdle) {
			t.Fatalf("lost completion clearing edge: %+v", edges)
		}
	})
	t.Run("Should retry ordinary publication read failure without losing the creator wake", func(t *testing.T) {
		t.Parallel()
		h, db := newMentionHarness(t)
		registerMentionCatalogSession(t, db, "retry-child", h.workspaceID)
		h.manager.sessions["retry-child"] = &Session{ID: "retry-child", ProfileID: store.DefaultProfileID,
			State: StateActive, Lineage: &store.SessionLineage{ParentSessionID: "creator", NotifyCreator: true}}
		wrapped := &failingPublicationReadStore{SessionAttentionStore: h.manager.attentionStore}
		h.manager.attentionStore = wrapped
		wakes := make(chan SpawnWakeEvent, 2)
		h.manager.spawnWakeNotifier = attentionWakeFunc(func(_ context.Context, _ string, event SpawnWakeEvent) error {
			wakes <- event
			return nil
		})
		hooks := make(chan hookspkg.SessionAttentionChangedPayload, 2)
		h.manager.hooks.Attention = attentionHookFunc(
			func(_ context.Context, p hookspkg.SessionAttentionChangedPayload) (hookspkg.SessionAttentionChangedPayload, error) {
				hooks <- p
				return p, nil
			},
		)
		wrapped.fail.Store(true)
		commit, err := h.manager.createPendingInteraction(t.Context(), store.PendingInteractionCreate{
			SessionID: "retry-child", InteractionID: "retry-permission", ProviderRequestID: "retry-permission",
			Kind: store.PendingInteractionKindPermission,
		})
		if err != nil || commit.Interaction == nil {
			t.Fatalf("committed creation: %+v, %v", commit, err)
		}
		select {
		case event := <-hooks:
			if event.From != string(BadgeIdle) || event.To != string(BadgeWaitingForAuth) {
				t.Fatalf("retry hook: %+v", event)
			}
		case <-time.After(time.Second):
			t.Fatal("committed permission lost its hook")
		}
		select {
		case event := <-wakes:
			if event.Badge != BadgeWaitingForAuth {
				t.Fatalf("retry wake: %+v", event)
			}
		case <-time.After(time.Second):
			t.Fatal("committed permission lost its wake")
		}
		h.manager.stopAttentionReconciliation()
		select {
		case event := <-hooks:
			t.Fatalf("duplicate retry hook: %+v", event)
		default:
		}
		select {
		case event := <-wakes:
			t.Fatalf("duplicate retry wake: %+v", event)
		default:
		}
	})

	for _, publication := range []string{"reconciliation", "ordinary", "lifecycle"} {
		name := "Should let a synchronous creator wake call child presence after " + publication + " publication"
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h, db := newMentionHarness(t)
			registerMentionCatalogSession(t, db, "wake-child", h.workspaceID)
			ctx := t.Context()
			old, err := h.manager.createPendingInteraction(ctx, store.PendingInteractionCreate{
				SessionID:         "wake-child",
				InteractionID:     "old",
				ProviderRequestID: "old",
				Kind:              store.PendingInteractionKindClarify,
			})
			if err != nil {
				t.Fatal(err)
			}

			// A stopped child emits a creator wake when its last question clears.
			h.manager.sessions["wake-child"] = &Session{ID: "wake-child", State: StateStopped,
				Lineage: &store.SessionLineage{ParentSessionID: "creator", NotifyCreator: true}}
			called := make(chan error, 1)
			h.manager.spawnWakeNotifier = attentionWakeFunc(
				func(ctx context.Context, _ string, _ SpawnWakeEvent) error {
					bounded, cancel := context.WithTimeout(ctx, time.Second)
					defer cancel()
					_, err := h.manager.SessionPresence(bounded, "wake-child", "", true)
					called <- err
					return err
				},
			)
			transition := store.PendingInteractionTransition{
				InteractionID: old.Interaction.InteractionID,
				Status:        store.PendingInteractionStatusCanceled,
				At:            h.manager.now(),
			}
			switch publication {
			case "lifecycle":
				before := h.manager.sessions["wake-child"].Info()
				commit, err := h.manager.commitAttention(ctx, "wake-child",
					func() (store.SessionAttentionCommit, error) {
						return db.TransitionPendingInteraction(ctx, transition)
					})
				if err != nil {
					t.Fatal(err)
				}
				after := h.manager.sessions["wake-child"].Info()
				applyAttentionToInfo(after, commit.After)
				h.manager.publishLifecycleAttentionTransitionWake(ctx, before, after, true, "")
			case "ordinary":
				if _, err := h.manager.transitionPendingInteraction(ctx, "wake-child", transition); err != nil {
					t.Fatal(err)
				}
			default:
				cleared, err := db.TransitionPendingInteraction(ctx, transition)
				if err != nil {
					t.Fatal(err)
				}
				publishAttentionCommitsAndWait(t, h.manager, []store.SessionAttentionCommit{cleared})
			}
			select {
			case err := <-called:
				if err != nil {
					t.Fatalf("creator wake child presence: %v", err)
				}
			default:
				t.Fatal("creator wake was suppressed")
			}
		})
	}

	t.Run("Should publish nonresident synchronous presence after worker shutdown", func(t *testing.T) {
		t.Parallel()
		h, db := newMentionHarness(t)
		registerMentionCatalogSession(t, db, "sync-presence", h.workspaceID)
		ctx := t.Context()
		if _, err := db.MarkSessionSettled(ctx, "sync-presence", false, h.manager.now()); err != nil {
			t.Fatal(err)
		}
		h.manager.stopAttentionReconciliation()
		var edges []hookspkg.SessionAttentionChangedPayload
		h.manager.hooks.Attention = attentionHookFunc(
			func(_ context.Context, p hookspkg.SessionAttentionChangedPayload) (hookspkg.SessionAttentionChangedPayload, error) {
				edges = append(edges, p)
				return p, nil
			},
		)
		if _, err := h.manager.SessionPresence(ctx, "sync-presence", "", true); err != nil {
			t.Fatal(err)
		}
		if len(edges) != 1 || edges[0].From != string(BadgeDone) || edges[0].To != string(BadgeIdle) {
			t.Fatalf("synchronous publication lost after worker shutdown: %+v", edges)
		}
	})

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
		attentionStore.mu.Lock()
		delete(attentionStore.createdSnapshots, "sess-hook")
		attentionStore.mu.Unlock()
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
		if manager.PublishPendingInteractionCommits(commits, func() { completed <- struct{}{} }) {
			t.Fatal("shutdown admitted another worker")
		}
		select {
		case <-completed:
			t.Fatal("shutdown allowed acknowledgement completion")
		default:
		}
	})

	t.Run("Should complete the batch when an acknowledged descendant was deleted", func(t *testing.T) {
		t.Parallel()
		manager, err := NewManager(WithHomePaths(testHomePaths(t)))
		if err != nil {
			t.Fatal(err)
		}
		cleanupTestManager(t, manager)
		manager.attentionStore = &committingAttentionStore{missingSessionID: "deleted"}
		manager.sessions["kept"] = &Session{ID: "kept", State: StateStopped, PendingClarifyCount: 1}
		publishAttentionCommitsAndWait(t, manager, []store.SessionAttentionCommit{
			{
				Before:      store.SessionAttention{PendingClarifyCount: 1},
				Interaction: &store.PendingInteraction{SessionID: "deleted"},
			},
			{
				Before:      store.SessionAttention{PendingClarifyCount: 1},
				Interaction: &store.PendingInteraction{SessionID: "kept"},
			},
		})
		if info := manager.sessions["kept"].Info(); info.PendingClarifyCount != 0 {
			t.Fatalf("remaining session not refreshed: %+v", info)
		}
	})
	t.Run("Should cancel nonresident publication reads before joining workers", func(t *testing.T) {
		t.Parallel()
		manager, err := NewManager(WithHomePaths(testHomePaths(t)))
		if err != nil {
			t.Fatal(err)
		}
		cleanupTestManager(t, manager)
		entered := make(chan struct{}, 1)
		manager.attentionStore = &committingAttentionStore{snapshot: store.SessionAttention{AttentionRevision: 1}}
		manager.sessionCatalog = &attentionReadCatalog{
			recordingSessionCatalog: newRecordingSessionCatalog(),
			entered:                 entered,
		}
		manager.PublishPendingInteractionCommits(
			[]store.SessionAttentionCommit{{Interaction: &store.PendingInteraction{SessionID: "nonresident"}}},
			nil,
		)
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("bounded publication read did not start")
		}
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		stopped := make(chan error, 1)
		go func() { stopped <- manager.Shutdown(ctx) }()
		select {
		case err := <-stopped:
			if err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal("publication read prevented worker shutdown")
		}
	})
	t.Run("Should publish reconciliation before a resumed session's new permission", func(t *testing.T) {
		t.Parallel()
		manager, err := NewManager(WithHomePaths(testHomePaths(t)))
		if err != nil {
			t.Fatal(err)
		}
		cleanupTestManager(t, manager)
		entered, created := make(chan struct{}, 1), make(chan struct{}, 1)
		release := make(chan struct{})
		catalog := &attentionReadCatalog{
			recordingSessionCatalog: newRecordingSessionCatalog(),
			entered:                 entered,
			release:                 release,
		}
		if err := catalog.RegisterSession(
			t.Context(),
			&store.SessionInfo{ID: "resumed", ProfileID: store.DefaultProfileID, State: string(StateStopped)},
		); err != nil {
			t.Fatal(err)
		}
		manager.sessionCatalog = catalog
		manager.attentionStore = &committingAttentionStore{createEntered: created}
		events, cancel, err := manager.SubscribeSessionCatalogEvents(
			t.Context(),
			CatalogScope{ReadScope: store.ReadScope{AllProfiles: true}, AllWorkspaces: true},
		)
		if err != nil {
			t.Fatal(err)
		}
		defer cancel()
		done := make(chan struct{})
		manager.PublishPendingInteractionCommits(
			[]store.SessionAttentionCommit{
				{
					Before:      store.SessionAttention{PendingClarifyCount: 1},
					Interaction: &store.PendingInteraction{SessionID: "resumed"},
				},
			},
			func() { close(done) },
		)
		select {
		case <-entered:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("publication did not start")
		}
		newer := make(chan error, 1)
		go func() {
			_, err := manager.createPendingInteraction(
				t.Context(),
				store.PendingInteractionCreate{
					SessionID:         "resumed",
					InteractionID:     "new-permission",
					ProviderRequestID: "new-permission",
					Kind:              store.PendingInteractionKindPermission,
				},
			)
			newer <- err
		}()
		select {
		case <-created:
			close(release)
			<-newer
			t.Fatal("new permission overtook reconciliation publication")
		case <-time.After(100 * time.Millisecond):
		}
		close(release)
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("reconciliation did not complete")
		}
		if err := <-newer; err != nil {
			t.Fatal(err)
		}
		assertCatalogEventName(t, events, CatalogEventNameChanged)
		first := assertCatalogEventName(t, events, CatalogEventNameAttention).Attention
		assertCatalogEventName(t, events, CatalogEventNameChanged)
		second := assertCatalogEventName(t, events, CatalogEventNameAttention).Attention
		if first == nil || second == nil || first.To != BadgeStopped || second.To != BadgeWaitingForAuth {
			t.Fatalf("publication order: first=%+v second=%+v", first, second)
		}
	})

	for _, deleted := range []bool{false, true} {
		name := "Should retry nonresident publication timeouts before instance completion"
		if deleted {
			name = "Should finish confirmed deletion after a publication timeout"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			manager, err := NewManager(WithHomePaths(testHomePaths(t)))
			if err != nil {
				t.Fatal(err)
			}
			cleanupTestManager(t, manager)
			release := make(chan struct{})
			close(release)
			catalog := &attentionReadCatalog{
				recordingSessionCatalog: newRecordingSessionCatalog(),
				entered:                 make(chan struct{}, 2),
				release:                 release,
				readFailures:            1,
				emptyRead:               make(chan struct{}),
			}
			if !deleted {
				if err := catalog.RegisterSession(
					t.Context(),
					&store.SessionInfo{
						ID:        "publication-retry",
						ProfileID: store.DefaultProfileID,
						State:     string(StateStopped),
					},
				); err != nil {
					t.Fatal(err)
				}
			}
			manager.sessionCatalog = catalog
			manager.attentionStore = &committingAttentionStore{}
			events, cancel, err := manager.SubscribeSessionCatalogEvents(
				t.Context(),
				CatalogScope{ReadScope: store.ReadScope{AllProfiles: true}, AllWorkspaces: true},
			)
			if err != nil {
				t.Fatal(err)
			}
			defer cancel()
			published := make(chan *AttentionEvent, 1)
			manager.PublishPendingInteractionCommits(
				[]store.SessionAttentionCommit{
					{
						Before:      store.SessionAttention{PendingClarifyCount: 1},
						Interaction: &store.PendingInteraction{SessionID: "publication-retry"},
					},
				},
				func() {
					var edge *AttentionEvent
					for range 2 {
						select {
						case event := <-events:
							if event.Attention != nil {
								edge = event.Attention
							}
						default:
						}
					}
					published <- edge
				},
			)
			select {
			case edge := <-published:
				if deleted {
					select {
					case <-catalog.emptyRead:
					default:
						t.Fatal("completion preceded successful empty catalog read")
					}
				}
				if deleted && edge != nil {
					t.Fatalf("deleted session published attention=%+v", edge)
				}
				if !deleted &&
					(edge == nil || edge.SessionID != "publication-retry" || edge.From != BadgeWaitingForInput || edge.To != BadgeStopped) {
					t.Fatalf("lost recovered publication=%+v", edge)
				}
			case <-time.After(2 * defaultLifecycleTimeout):
				t.Fatal("publication recovery prevented completion")
			}
		})
	}

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
	createdSnapshots   map[string]store.SessionAttention
	pending            []store.PendingInteraction
	blockedSessionID   string
	blockedReads       int
	readStarted        chan struct{}
	missingSessionID   string
	createEntered      chan struct{}
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
	if id != "" && id == s.missingSessionID {
		return store.SessionAttention{}, errors.Join(store.ErrSessionNotFound, errors.New("deleted session"))
	}
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
	s.mu.Lock()
	snapshot, ok := s.createdSnapshots[id]
	s.mu.Unlock()
	if ok {
		return snapshot, ctx.Err()
	}
	return s.snapshot, ctx.Err()
}

func (s *committingAttentionStore) CreatePendingInteraction(
	_ context.Context,
	req store.PendingInteractionCreate,
) (store.SessionAttentionCommit, error) {
	if s.createEntered != nil {
		s.createEntered <- struct{}{}
	}
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
	s.mu.Lock()
	if s.createdSnapshots == nil {
		s.createdSnapshots = make(map[string]store.SessionAttention)
	}
	s.createdSnapshots[req.SessionID] = after
	s.mu.Unlock()
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

// attentionReadCatalog models a catalog read that must retain cancellation and a deadline.
type attentionReadCatalog struct {
	*recordingSessionCatalog
	entered      chan struct{}
	release      <-chan struct{}
	readFailures int
	emptyRead    chan struct{}
}

func (c *attentionReadCatalog) ListSessions(
	ctx context.Context,
	query store.SessionListQuery,
) ([]store.SessionInfo, error) {
	if _, bounded := ctx.Deadline(); !bounded {
		return nil, errors.New("publication read has no deadline")
	}
	select {
	case c.entered <- struct{}{}:
	default:
	}
	if c.readFailures > 0 {
		c.readFailures--
		<-ctx.Done()
		return nil, ctx.Err()
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.release:
		rows, err := c.recordingSessionCatalog.ListSessions(ctx, query)
		if err == nil && len(rows) == 0 && c.emptyRead != nil {
			close(c.emptyRead)
		}
		return rows, err
	}
}

// attentionWakeFunc models the synchronous daemon bridge at its I/O boundary.
type attentionWakeFunc func(context.Context, string, SpawnWakeEvent) error

func (f attentionWakeFunc) WakeSpawnCreator(ctx context.Context, creator string, event SpawnWakeEvent) error {
	return f(ctx, creator, event)
}

type failingPublicationReadStore struct {
	store.SessionAttentionStore
	fail atomic.Bool
}

func (s *failingPublicationReadStore) GetSessionAttention(
	ctx context.Context,
	id string,
) (store.SessionAttention, error) {
	if s.fail.Swap(false) {
		return store.SessionAttention{}, errors.New("transient publication read failure")
	}
	return s.SessionAttentionStore.GetSessionAttention(ctx, id)
}
