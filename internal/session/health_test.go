package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/heartbeat"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

func TestManagerSessionHealthTransitions(t *testing.T) {
	t.Run("Should persist idle active and idle health without task lease side effects", func(t *testing.T) {
		ctx := testutil.Context(t)
		baseAt := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		clock := newSessionHealthTestClock(baseAt)
		healthStore := newFakeSessionHealthStore()
		h := newHarness(t, WithNow(clock.Now), WithSessionHealthStore(healthStore))
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Fatalf("Stop() cleanup error = %v", err)
			}
		})

		initial := healthStore.mustGet(t, session.ID)
		assertSessionHealthReadModel(t, initial, session.ID, h.workspaceID, "coder")
		assertSessionHealthState(t, initial, heartbeat.SessionHealthStateIdle, heartbeat.SessionHealthHealthy)
		if !initial.EligibleForWake || initial.IneligibilityReason != "" {
			t.Fatalf("initial health = %#v, want eligible idle row", initial)
		}
		if !initial.LastPresenceAt.Equal(baseAt) {
			t.Fatalf("initial LastPresenceAt = %s, want %s", initial.LastPresenceAt, baseAt)
		}
		if !initial.LastActivityAt.IsZero() {
			t.Fatalf("initial LastActivityAt = %s, want zero until prompt activity", initial.LastActivityAt)
		}

		activeAt := baseAt.Add(10 * time.Second)
		clock.Set(activeAt)
		source := make(chan acp.AgentEvent, 2)
		h.driver.promptHook = func(_ *fakeProcess, _ acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			return source, nil
		}
		turnEnded := make(chan string, 1)
		h.manager.SetTurnEndNotifier(func(sessionID string) {
			turnEnded <- sessionID
		})

		events, err := h.manager.Prompt(ctx, session.ID, "hello")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		if !h.manager.IsPrompting(session.ID) {
			t.Fatal("IsPrompting() = false, want true while prompt health is active")
		}
		if meta := session.CurrentPromptMeta(); meta.TurnSource != string(TurnSourceUser) {
			t.Fatalf("CurrentPromptMeta().TurnSource = %q, want %q", meta.TurnSource, TurnSourceUser)
		}

		active := healthStore.mustGet(t, session.ID)
		assertSessionHealthReadModel(t, active, session.ID, h.workspaceID, "coder")
		assertSessionHealthState(t, active, heartbeat.SessionHealthStatePrompting, heartbeat.SessionHealthHealthy)
		if !active.ActivePrompt || !active.Attachable || active.EligibleForWake ||
			active.IneligibilityReason != string(heartbeat.SessionHealthReasonPromptActive) {
			t.Fatalf("active health = %#v, want prompting, attachable, and wake-ineligible", active)
		}
		if !active.LastActivityAt.Equal(activeAt) {
			t.Fatalf("active LastActivityAt = %s, want %s", active.LastActivityAt, activeAt)
		}
		if !active.LastPresenceAt.Equal(baseAt) {
			t.Fatalf("active LastPresenceAt = %s, want idle presence preserved at %s", active.LastPresenceAt, baseAt)
		}
		if got, want := len(h.driver.promptCalls), 1; got != want {
			t.Fatalf("driver prompt calls = %d, want %d", got, want)
		}

		staleCheckAt := activeAt.Add(3 * time.Minute)
		clock.Set(staleCheckAt)
		ineligible := false
		rows, err := h.manager.ListSessionHealth(ctx, heartbeat.SessionHealthListQuery{EligibleForWake: &ineligible})
		if err != nil {
			t.Fatalf("ListSessionHealth(active prompt) error = %v", err)
		}
		if gotIDs, wantIDs := sessionHealthIDs(rows), []string{session.ID}; !slices.Equal(gotIDs, wantIDs) {
			t.Fatalf("active prompt health ids = %#v, want %#v", gotIDs, wantIDs)
		}
		stillActive := rows[0]
		assertSessionHealthState(t, stillActive, heartbeat.SessionHealthStatePrompting, heartbeat.SessionHealthHealthy)
		if !stillActive.ActivePrompt ||
			stillActive.IneligibilityReason != string(heartbeat.SessionHealthReasonPromptActive) {
			t.Fatalf("active prompt health after stale cutoff = %#v, want prompt-active reason", stillActive)
		}

		doneAt := staleCheckAt.Add(5 * time.Second)
		clock.Set(doneAt)
		source <- acp.AgentEvent{
			Type:      acp.EventTypeDone,
			Timestamp: doneAt,
		}
		close(source)
		promptEvents := collectEvents(t, events)
		if got, want := len(promptEvents), 1; got != want {
			t.Fatalf("prompt events = %d, want %d", got, want)
		}
		if got := promptEvents[0].Type; got != acp.EventTypeDone {
			t.Fatalf("promptEvents[0].Type = %q, want %q", got, acp.EventTypeDone)
		}
		if h.manager.IsPrompting(session.ID) {
			t.Fatal("IsPrompting() = true, want false after prompt completion")
		}
		select {
		case got := <-turnEnded:
			if got != session.ID {
				t.Fatalf("turn end notifier session = %q, want %q", got, session.ID)
			}
		default:
			t.Fatal("turn end notifier did not run")
		}

		idleAgain := healthStore.mustGet(t, session.ID)
		assertSessionHealthReadModel(t, idleAgain, session.ID, h.workspaceID, "coder")
		assertSessionHealthState(t, idleAgain, heartbeat.SessionHealthStateIdle, heartbeat.SessionHealthHealthy)
		if idleAgain.ActivePrompt || !idleAgain.Attachable || !idleAgain.EligibleForWake ||
			idleAgain.IneligibilityReason != "" {
			t.Fatalf("idleAgain health = %#v, want idle eligible row after prompt completion", idleAgain)
		}
		if !idleAgain.LastActivityAt.Equal(doneAt) {
			t.Fatalf("idleAgain LastActivityAt = %s, want %s", idleAgain.LastActivityAt, doneAt)
		}
		if !idleAgain.LastPresenceAt.Equal(doneAt) {
			t.Fatalf("idleAgain LastPresenceAt = %s, want %s", idleAgain.LastPresenceAt, doneAt)
		}
		if got, want := len(h.driver.promptCalls), 1; got != want {
			t.Fatalf("driver prompt calls after health updates = %d, want %d", got, want)
		}
	})

	t.Run("Should touch idle presence without prompt ACP network or notifier side effects", func(t *testing.T) {
		ctx := testutil.Context(t)
		baseAt := time.Date(2026, 5, 2, 13, 0, 0, 0, time.UTC)
		clock := newSessionHealthTestClock(baseAt)
		healthStore := newFakeSessionHealthStore()
		h := newHarness(t, WithNow(clock.Now), WithSessionHealthStore(healthStore))
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Fatalf("Stop() cleanup error = %v", err)
			}
		})
		lifecycle := newFakeNetworkPeerLifecycle()
		h.manager.SetNetworkPeerLifecycle(lifecycle)
		promptCallsBefore := len(h.driver.promptCalls)
		notifierEventsBefore := h.notifier.eventCount(session.ID)
		createdBefore := h.notifier.createdCount()
		stoppedBefore := h.notifier.stoppedCount()
		joinsBefore := lifecycle.joinCount()
		leavesBefore := lifecycle.leaveCount()
		upsertsBefore := healthStore.upsertCount()

		touchAt := baseAt.Add(30 * time.Second)
		clock.Set(touchAt)
		health, err := h.manager.TouchSessionPresence(ctx, session.ID)
		if err != nil {
			t.Fatalf("TouchSessionPresence() error = %v", err)
		}

		assertSessionHealthReadModel(t, health, session.ID, h.workspaceID, "coder")
		assertSessionHealthState(t, health, heartbeat.SessionHealthStateIdle, heartbeat.SessionHealthHealthy)
		if !health.EligibleForWake || health.ActivePrompt || !health.Attachable {
			t.Fatalf("TouchSessionPresence() = %#v, want idle eligible attachable row", health)
		}
		if !health.LastPresenceAt.Equal(touchAt) {
			t.Fatalf("TouchSessionPresence().LastPresenceAt = %s, want %s", health.LastPresenceAt, touchAt)
		}
		if got := len(h.driver.promptCalls); got != promptCallsBefore {
			t.Fatalf("driver prompt calls after presence touch = %d, want %d", got, promptCallsBefore)
		}
		if got := h.notifier.eventCount(session.ID); got != notifierEventsBefore {
			t.Fatalf("notifier event count after presence touch = %d, want %d", got, notifierEventsBefore)
		}
		if got := h.notifier.createdCount(); got != createdBefore {
			t.Fatalf("notifier created count after presence touch = %d, want %d", got, createdBefore)
		}
		if got := h.notifier.stoppedCount(); got != stoppedBefore {
			t.Fatalf("notifier stopped count after presence touch = %d, want %d", got, stoppedBefore)
		}
		if got := lifecycle.joinCount(); got != joinsBefore {
			t.Fatalf("network join count after presence touch = %d, want %d", got, joinsBefore)
		}
		if got := lifecycle.leaveCount(); got != leavesBefore {
			t.Fatalf("network leave count after presence touch = %d, want %d", got, leavesBefore)
		}
		if got, want := healthStore.upsertCount(), upsertsBefore+1; got != want {
			t.Fatalf("health upsert count after presence touch = %d, want %d", got, want)
		}
	})
}

func TestManagerSessionHealthHooks(t *testing.T) {
	t.Run("Should dispatch bounded health update hooks on eligible transitions", func(t *testing.T) {
		ctx := testutil.Context(t)
		baseAt := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		clock := newSessionHealthTestClock(baseAt)
		healthStore := newFakeSessionHealthStore()
		hooks := &recordingSessionHealthHooks{}
		cfg := aghconfig.DefaultHeartbeatConfig()
		cfg.SessionHealthHookMinInterval = time.Second
		h := newHarness(
			t,
			WithNow(clock.Now),
			WithSessionHealthStore(healthStore),
			WithSessionHealthConfig(cfg),
			WithHookSet(HookSet{AuthoredContext: hooks}),
		)

		idle := heartbeat.SessionHealth{
			SessionID:       "sess-health-hook",
			WorkspaceID:     h.workspaceID,
			AgentName:       "coder",
			State:           heartbeat.SessionHealthStateIdle,
			Health:          heartbeat.SessionHealthHealthy,
			Attachable:      true,
			EligibleForWake: true,
			UpdatedAt:       baseAt,
		}
		if _, err := h.manager.storeSessionHealth(ctx, idle); err != nil {
			t.Fatalf("storeSessionHealth(idle) error = %v", err)
		}

		promptingAt := baseAt.Add(500 * time.Millisecond)
		clock.Set(promptingAt)
		prompting := idle
		prompting.State = heartbeat.SessionHealthStatePrompting
		prompting.ActivePrompt = true
		prompting.EligibleForWake = false
		prompting.IneligibilityReason = string(heartbeat.SessionHealthReasonPromptActive)
		prompting.UpdatedAt = promptingAt
		if _, err := h.manager.storeSessionHealth(ctx, prompting); err != nil {
			t.Fatalf("storeSessionHealth(prompting) error = %v", err)
		}

		idleAgainAt := baseAt.Add(2 * time.Second)
		clock.Set(idleAgainAt)
		idleAgain := idle
		idleAgain.UpdatedAt = idleAgainAt
		if _, err := h.manager.storeSessionHealth(ctx, idleAgain); err != nil {
			t.Fatalf("storeSessionHealth(idle again) error = %v", err)
		}

		payloads := hooks.snapshot()
		if got, want := len(payloads), 2; got != want {
			t.Fatalf("session health hook payloads = %d, want %d: %#v", got, want, payloads)
		}
		if payloads[0].SessionID != idle.SessionID ||
			payloads[0].WorkspaceID != h.workspaceID ||
			payloads[0].Health != string(heartbeat.SessionHealthHealthy) ||
			!payloads[0].EligibleForWake {
			t.Fatalf("initial hook payload = %#v, want redacted eligible health context", payloads[0])
		}
		if payloads[1].State != string(heartbeat.SessionHealthStateIdle) ||
			!payloads[1].EligibleForWake ||
			payloads[1].IneligibilityReason != "" {
			t.Fatalf("idle-again hook payload = %#v, want eligible idle transition", payloads[1])
		}
	})
}

func TestManagerSessionHealthEligibility(t *testing.T) {
	t.Run("Should derive closed deterministic wake ineligibility reasons", func(t *testing.T) {
		t.Parallel()

		baseAt := time.Date(2026, 5, 2, 14, 0, 0, 0, time.UTC)
		manager := &Manager{}
		testCases := []struct {
			name       string
			info       *Info
			input      sessionHealthInput
			wantState  heartbeat.SessionHealthState
			wantHealth heartbeat.SessionHealthStatus
			wantReason heartbeat.SessionHealthIneligibilityReason
		}{
			{
				name: "Should reject active prompt sessions",
				info: &Info{
					ID:          "sess-prompting",
					WorkspaceID: "ws-health",
					AgentName:   "coder",
					State:       StateActive,
				},
				input: sessionHealthInput{
					activePrompt: true,
					attachable:   true,
				},
				wantState:  heartbeat.SessionHealthStatePrompting,
				wantHealth: heartbeat.SessionHealthHealthy,
				wantReason: heartbeat.SessionHealthReasonPromptActive,
			},
			{
				name: "Should reject detached active sessions",
				info: &Info{
					ID:          "sess-detached",
					WorkspaceID: "ws-health",
					AgentName:   "coder",
					State:       StateActive,
				},
				input: sessionHealthInput{
					attachable: false,
				},
				wantState:  heartbeat.SessionHealthStateDetached,
				wantHealth: heartbeat.SessionHealthHealthy,
				wantReason: heartbeat.SessionHealthReasonNotAttachable,
			},
			{
				name: "Should reject hung sessions",
				info: &Info{
					ID:          "sess-hung",
					WorkspaceID: "ws-health",
					AgentName:   "coder",
					State:       StateActive,
					Liveness: &store.SessionLivenessMeta{
						StallState: store.SessionStallStateDetected,
					},
				},
				input: sessionHealthInput{
					attachable: true,
				},
				wantState:  heartbeat.SessionHealthStateIdle,
				wantHealth: heartbeat.SessionHealthDegraded,
				wantReason: heartbeat.SessionHealthReasonHung,
			},
			{
				name: "Should reject dead stopped sessions",
				info: &Info{
					ID:          "sess-dead",
					WorkspaceID: "ws-health",
					AgentName:   "coder",
					State:       StateStopped,
				},
				wantState:  heartbeat.SessionHealthStateStopped,
				wantHealth: heartbeat.SessionHealthDead,
				wantReason: heartbeat.SessionHealthReasonDead,
			},
			{
				name: "Should reject unknown non-active sessions",
				info: &Info{
					ID:          "sess-unknown",
					WorkspaceID: "ws-health",
					AgentName:   "coder",
					State:       StateStarting,
				},
				wantState:  heartbeat.SessionHealthStateDetached,
				wantHealth: heartbeat.SessionHealthUnknown,
				wantReason: heartbeat.SessionHealthReasonUnknown,
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				got := manager.sessionHealthFromInfo(
					testCase.info,
					heartbeat.SessionHealth{},
					baseAt,
					testCase.input,
				)
				assertSessionHealthState(t, got, testCase.wantState, testCase.wantHealth)
				if got.EligibleForWake {
					t.Fatalf("health = %#v, want wake-ineligible", got)
				}
				if got.IneligibilityReason != string(testCase.wantReason) {
					t.Fatalf("IneligibilityReason = %q, want %q", got.IneligibilityReason, testCase.wantReason)
				}
				if !heartbeat.ValidSessionHealthIneligibilityReason(got.IneligibilityReason) {
					t.Fatalf("IneligibilityReason = %q, want closed reason", got.IneligibilityReason)
				}
			})
		}

		stale := heartbeat.SessionHealth{
			SessionID:       "sess-stale",
			WorkspaceID:     "ws-health",
			AgentName:       "coder",
			State:           heartbeat.SessionHealthStateIdle,
			Health:          heartbeat.SessionHealthStale,
			Attachable:      true,
			LastPresenceAt:  baseAt.Add(-time.Hour),
			UpdatedAt:       baseAt,
			EligibleForWake: true,
		}
		applySessionHealthEligibility(&stale)
		if stale.EligibleForWake || stale.IneligibilityReason != string(heartbeat.SessionHealthReasonStale) {
			t.Fatalf("stale health = %#v, want stale wake-ineligible reason", stale)
		}

		invalid := stale
		invalid.Health = heartbeat.SessionHealthHealthy
		invalid.IneligibilityReason = "task_lease_renewed"
		if err := invalid.Validate(); !errors.Is(err, heartbeat.ErrInvalidSessionHealth) {
			t.Fatalf("SessionHealth.Validate(invalid reason) error = %v, want ErrInvalidSessionHealth", err)
		}
	})
}

func TestManagerSessionHealthQueries(t *testing.T) {
	t.Run("Should derive one catalog page with one durable health batch", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		baseAt := time.Date(2026, 5, 2, 14, 15, 0, 0, time.UTC)
		clock := newSessionHealthTestClock(baseAt)
		healthStore := newFakeSessionHealthStore()
		h := newHarness(t, WithNow(clock.Now), WithSessionHealthStore(healthStore))
		active := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), active.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Errorf("Stop(active cleanup) error = %v", err)
			}
		})

		getsBefore, batchesBefore := healthStore.readCounts()
		healthByID, err := h.manager.SessionHealthForPage(ctx, []*Info{active.Info()})
		if err != nil {
			t.Fatalf("SessionHealthForPage() error = %v", err)
		}
		getsAfter, batchesAfter := healthStore.readCounts()
		if getsAfter != getsBefore || batchesAfter-batchesBefore != 1 {
			t.Fatalf(
				"health reads = (single %d->%d, batch %d->%d), want exactly one batch",
				getsBefore,
				getsAfter,
				batchesBefore,
				batchesAfter,
			)
		}
		health, ok := healthByID[active.ID]
		if !ok {
			t.Fatalf("SessionHealthForPage() = %#v, want active session", healthByID)
		}
		assertSessionHealthState(t, health, heartbeat.SessionHealthStateIdle, heartbeat.SessionHealthHealthy)
	})

	t.Run("Should preserve durable recovery truth and synthesize missing health conservatively", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		baseAt := time.Date(2026, 5, 2, 14, 20, 0, 0, time.UTC)
		clock := newSessionHealthTestClock(baseAt.Add(time.Hour))
		healthStore := newFakeSessionHealthStore()
		h := newHarness(t, WithNow(clock.Now), WithSessionHealthStore(healthStore))
		storedInfo := &Info{
			ID:          "sess-durable-stale",
			WorkspaceID: h.workspaceID,
			AgentName:   "coder",
			State:       StateActive,
		}
		stored := heartbeat.SessionHealth{
			SessionID:           storedInfo.ID,
			WorkspaceID:         h.workspaceID,
			AgentName:           "coder",
			State:               heartbeat.SessionHealthStateDetached,
			Health:              heartbeat.SessionHealthStale,
			IneligibilityReason: string(heartbeat.SessionHealthReasonStale),
			LastPresenceAt:      baseAt,
			UpdatedAt:           baseAt,
		}
		if _, err := healthStore.UpsertSessionHealth(ctx, stored); err != nil {
			t.Fatalf("UpsertSessionHealth(stale) error = %v", err)
		}
		stoppedInfo := &Info{
			ID:          "sess-durable-stopped",
			WorkspaceID: h.workspaceID,
			AgentName:   "coder",
			State:       StateStopped,
		}
		missingActiveInfo := &Info{
			ID:          "sess-durable-active-missing",
			WorkspaceID: h.workspaceID,
			AgentName:   "coder",
			State:       StateActive,
		}

		getsBefore, batchesBefore := healthStore.readCounts()
		healthByID, err := h.manager.SessionHealthForPage(ctx, []*Info{
			nil,
			storedInfo,
			storedInfo,
			stoppedInfo,
			missingActiveInfo,
		})
		if err != nil {
			t.Fatalf("SessionHealthForPage() error = %v", err)
		}
		getsAfter, batchesAfter := healthStore.readCounts()
		if getsAfter != getsBefore || batchesAfter-batchesBefore != 1 {
			t.Fatalf(
				"health reads = (single %d->%d, batch %d->%d), want exactly one batch",
				getsBefore,
				getsAfter,
				batchesBefore,
				batchesAfter,
			)
		}
		if len(healthByID) != 3 {
			t.Fatalf("SessionHealthForPage() = %#v, want three deduplicated non-nil rows", healthByID)
		}
		if got := healthByID[storedInfo.ID]; got != stored {
			t.Fatalf("stored durable health = %#v, want preserved %#v", got, stored)
		}
		assertSessionHealthState(
			t,
			healthByID[stoppedInfo.ID],
			heartbeat.SessionHealthStateStopped,
			heartbeat.SessionHealthDead,
		)
		missing := healthByID[missingActiveInfo.ID]
		assertSessionHealthState(
			t,
			missing,
			heartbeat.SessionHealthStateDetached,
			heartbeat.SessionHealthStale,
		)
		if missing.EligibleForWake ||
			missing.IneligibilityReason != string(heartbeat.SessionHealthReasonStale) {
			t.Fatalf("missing durable health = %#v, want stale and wake-ineligible", missing)
		}
	})

	t.Run("Should refresh active rows and query persisted health read models", func(t *testing.T) {
		ctx := testutil.Context(t)
		baseAt := time.Date(2026, 5, 2, 14, 30, 0, 0, time.UTC)
		clock := newSessionHealthTestClock(baseAt)
		healthStore := newFakeSessionHealthStore()
		h := newHarness(t, WithNow(clock.Now), WithSessionHealthStore(healthStore))
		session := createSession(t, h)

		refreshedAt := baseAt.Add(time.Minute)
		clock.Set(refreshedAt)
		active, err := h.manager.GetSessionHealth(ctx, session.ID)
		if err != nil {
			t.Fatalf("GetSessionHealth(active) error = %v", err)
		}
		assertSessionHealthReadModel(t, active, session.ID, h.workspaceID, "coder")
		assertSessionHealthState(t, active, heartbeat.SessionHealthStateIdle, heartbeat.SessionHealthHealthy)
		if !active.EligibleForWake || !active.LastPresenceAt.Equal(refreshedAt) {
			t.Fatalf("GetSessionHealth(active) = %#v, want refreshed eligible active row", active)
		}

		ineligible := false
		eligible := true
		rows, err := h.manager.ListSessionHealth(ctx, heartbeat.SessionHealthListQuery{
			WorkspaceID:     h.workspaceID,
			AgentName:       "coder",
			State:           heartbeat.SessionHealthStateIdle,
			Health:          heartbeat.SessionHealthHealthy,
			EligibleForWake: &eligible,
		})
		if err != nil {
			t.Fatalf("ListSessionHealth(eligible) error = %v", err)
		}
		if gotIDs, wantIDs := sessionHealthIDs(rows), []string{session.ID}; !slices.Equal(gotIDs, wantIDs) {
			t.Fatalf("eligible health ids = %#v, want %#v", gotIDs, wantIDs)
		}

		empty, err := h.manager.ListSessionHealth(ctx, heartbeat.SessionHealthListQuery{EligibleForWake: &ineligible})
		if err != nil {
			t.Fatalf("ListSessionHealth(ineligible) error = %v", err)
		}
		if len(empty) != 0 {
			t.Fatalf("ineligible health rows = %#v, want none while session is idle eligible", empty)
		}

		recovery, err := h.manager.RecoverSessionHealth(ctx)
		if err != nil {
			t.Fatalf("RecoverSessionHealth(active) error = %v", err)
		}
		if recovery.RefreshedActive != 1 || recovery.Recomputed != 0 || recovery.MarkedStale != 0 {
			t.Fatalf("RecoverSessionHealth(active) = %#v, want one active refresh only", recovery)
		}

		stoppedAt := refreshedAt.Add(time.Minute)
		clock.Set(stoppedAt)
		if err := h.manager.Stop(ctx, session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		stopped, err := h.manager.GetSessionHealth(ctx, session.ID)
		if err != nil {
			t.Fatalf("GetSessionHealth(stopped) error = %v", err)
		}
		assertSessionHealthReadModel(t, stopped, session.ID, h.workspaceID, "coder")
		assertSessionHealthState(t, stopped, heartbeat.SessionHealthStateStopped, heartbeat.SessionHealthDead)
		if stopped.EligibleForWake || stopped.IneligibilityReason != string(heartbeat.SessionHealthReasonDead) {
			t.Fatalf("GetSessionHealth(stopped) = %#v, want dead wake-ineligible row", stopped)
		}

		if _, err := h.manager.GetSessionHealth(ctx, "sess-missing"); !errors.Is(err, ErrSessionNotFound) {
			t.Fatalf("GetSessionHealth(missing) error = %v, want ErrSessionNotFound", err)
		}
	})

	t.Run("Should query active in-memory health when no durable store is configured", func(t *testing.T) {
		ctx := testutil.Context(t)
		baseAt := time.Date(2026, 5, 2, 14, 45, 0, 0, time.UTC)
		clock := newSessionHealthTestClock(baseAt)
		h := newHarness(t, WithNow(clock.Now))
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Fatalf("Stop() cleanup error = %v", err)
			}
		})

		health, err := h.manager.GetSessionHealth(ctx, session.ID)
		if err != nil {
			t.Fatalf("GetSessionHealth(no store active) error = %v", err)
		}
		assertSessionHealthReadModel(t, health, session.ID, h.workspaceID, "coder")
		assertSessionHealthState(t, health, heartbeat.SessionHealthStateIdle, heartbeat.SessionHealthHealthy)

		eligible := true
		rows, err := h.manager.ListSessionHealth(ctx, heartbeat.SessionHealthListQuery{
			WorkspaceID:     h.workspaceID,
			AgentName:       "coder",
			SessionID:       session.ID,
			State:           heartbeat.SessionHealthStateIdle,
			Health:          heartbeat.SessionHealthHealthy,
			EligibleForWake: &eligible,
		})
		if err != nil {
			t.Fatalf("ListSessionHealth(no store) error = %v", err)
		}
		if gotIDs, wantIDs := sessionHealthIDs(rows), []string{session.ID}; !slices.Equal(gotIDs, wantIDs) {
			t.Fatalf("in-memory health ids = %#v, want %#v", gotIDs, wantIDs)
		}
		none, err := h.manager.ListSessionHealth(ctx, heartbeat.SessionHealthListQuery{WorkspaceID: "ws-other"})
		if err != nil {
			t.Fatalf("ListSessionHealth(no store mismatch) error = %v", err)
		}
		if len(none) != 0 {
			t.Fatalf("mismatched in-memory rows = %#v, want none", none)
		}
	})

	t.Run("Should apply every health query filter deterministically", func(t *testing.T) {
		t.Parallel()

		eligible := true
		row := heartbeat.SessionHealth{
			SessionID:       "sess-query",
			WorkspaceID:     "ws-query",
			AgentName:       "coder",
			State:           heartbeat.SessionHealthStateIdle,
			Health:          heartbeat.SessionHealthHealthy,
			Attachable:      true,
			EligibleForWake: true,
		}
		matching := heartbeat.SessionHealthListQuery{
			WorkspaceID:     "ws-query",
			AgentName:       "coder",
			SessionID:       "sess-query",
			State:           heartbeat.SessionHealthStateIdle,
			Health:          heartbeat.SessionHealthHealthy,
			EligibleForWake: &eligible,
		}
		if !sessionHealthMatchesQuery(row, matching) {
			t.Fatalf("sessionHealthMatchesQuery(%#v, %#v) = false, want true", row, matching)
		}
		for _, query := range []heartbeat.SessionHealthListQuery{
			{WorkspaceID: "ws-other"},
			{AgentName: "qa"},
			{SessionID: "sess-other"},
			{State: heartbeat.SessionHealthStatePrompting},
			{Health: heartbeat.SessionHealthDead},
			{EligibleForWake: new(false)},
		} {
			if sessionHealthMatchesQuery(row, query) {
				t.Fatalf("sessionHealthMatchesQuery(%#v, %#v) = true, want false", row, query)
			}
		}
	})
}

func TestManagerSessionHealthErrorPaths(t *testing.T) {
	t.Run("Should reject invalid manager context and identifier inputs", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		var nilManager *Manager
		if _, err := nilManager.TouchSessionPresence(ctx, "sess-1"); err == nil {
			t.Fatal("TouchSessionPresence(nil manager) error = nil, want non-nil")
		}
		if _, err := nilManager.GetSessionHealth(ctx, "sess-1"); err == nil {
			t.Fatal("GetSessionHealth(nil manager) error = nil, want non-nil")
		}
		if _, err := nilManager.ListSessionHealth(ctx, heartbeat.SessionHealthListQuery{}); err == nil {
			t.Fatal("ListSessionHealth(nil manager) error = nil, want non-nil")
		}
		if _, err := nilManager.RecoverSessionHealth(ctx); err == nil {
			t.Fatal("RecoverSessionHealth(nil manager) error = nil, want non-nil")
		}

		manager := newHarness(t).manager
		if _, err := manager.TouchSessionPresence(nilContextForGuardTest(), "sess-1"); err == nil {
			t.Fatal("TouchSessionPresence(nil context) error = nil, want non-nil")
		}
		if _, err := manager.GetSessionHealth(nilContextForGuardTest(), "sess-1"); err == nil {
			t.Fatal("GetSessionHealth(nil context) error = nil, want non-nil")
		}
		if _, err := manager.GetSessionHealth(ctx, " "); err == nil {
			t.Fatal("GetSessionHealth(blank id) error = nil, want non-nil")
		}
		if _, err := manager.ListSessionHealth(
			nilContextForGuardTest(),
			heartbeat.SessionHealthListQuery{},
		); err == nil {
			t.Fatal("ListSessionHealth(nil context) error = nil, want non-nil")
		}
		if _, err := manager.RecoverSessionHealth(nilContextForGuardTest()); err == nil {
			t.Fatal("RecoverSessionHealth(nil context) error = nil, want non-nil")
		}
	})

	t.Run("Should reject malformed health rows before storage", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		manager := newHarness(t).manager
		if _, err := manager.storeSessionHealth(ctx, heartbeat.SessionHealth{}); !errors.Is(
			err,
			heartbeat.ErrInvalidSessionHealth,
		) {
			t.Fatalf("storeSessionHealth(invalid) error = %v, want ErrInvalidSessionHealth", err)
		}
		if _, err := manager.persistSessionHealthForSession(ctx, nil, time.Time{}, sessionHealthInput{}); err == nil {
			t.Fatal("persistSessionHealthForSession(nil session) error = nil, want non-nil")
		}
		if _, err := manager.persistSessionHealthForSession(
			nilContextForGuardTest(),
			&Session{},
			time.Time{},
			sessionHealthInput{},
		); err == nil {
			t.Fatal("persistSessionHealthForSession(nil context) error = nil, want non-nil")
		}
		manager.lifecycleCtx = nil
		healthCtx, cancel := manager.detachedSessionHealthContext(nilContextForGuardTest())
		defer cancel()
		select {
		case <-healthCtx.Done():
			t.Fatal("detachedSessionHealthContext(nil) is already done, want usable background context")
		default:
		}
	})

	t.Run("Should rebuild missing persisted health from recovered metadata", func(t *testing.T) {
		ctx := testutil.Context(t)
		baseAt := time.Date(2026, 5, 2, 14, 55, 0, 0, time.UTC)
		clock := newSessionHealthTestClock(baseAt)
		h := newHarness(t, WithNow(clock.Now))
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Fatalf("Stop() cleanup error = %v", err)
			}
		})

		healthStore := newFakeSessionHealthStore()
		restarted := newManagerWithHarness(t, h, WithNow(clock.Now), WithSessionHealthStore(healthStore))
		health, err := restarted.GetSessionHealth(ctx, session.ID)
		if err != nil {
			t.Fatalf("GetSessionHealth(recovered missing row) error = %v", err)
		}
		assertSessionHealthState(t, health, heartbeat.SessionHealthStateStopped, heartbeat.SessionHealthDead)
		if health.EligibleForWake || health.IneligibilityReason != string(heartbeat.SessionHealthReasonDead) {
			t.Fatalf("recovered missing health = %#v, want dead wake-ineligible row", health)
		}

		withoutStore := newManagerWithHarness(t, h, WithNow(clock.Now))
		recovery, err := withoutStore.RecoverSessionHealth(ctx)
		if err != nil {
			t.Fatalf("RecoverSessionHealth(no store) error = %v", err)
		}
		if recovery != (HealthRecoveryResult{}) {
			t.Fatalf("RecoverSessionHealth(no store) = %#v, want zero result for no active sessions", recovery)
		}
	})
}

func TestManagerSessionHealthRecovery(t *testing.T) {
	t.Run("Should recompute restart rows and mark stale rows before wake eligibility", func(t *testing.T) {
		ctx := testutil.Context(t)
		baseAt := time.Date(2026, 5, 2, 15, 0, 0, 0, time.UTC)
		clock := newSessionHealthTestClock(baseAt)
		healthStore := newFakeSessionHealthStore()
		h := newHarness(
			t,
			WithNow(clock.Now),
			WithSessionHealthStore(healthStore),
			WithSessionHealthConfig(aghconfig.HeartbeatConfig{SessionHealthStaleAfter: time.Minute}),
		)
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Fatalf("Stop() cleanup error = %v", err)
			}
		})

		orphan := heartbeat.SessionHealth{
			SessionID:       "sess-stale-orphan",
			WorkspaceID:     h.workspaceID,
			AgentName:       "coder",
			State:           heartbeat.SessionHealthStateIdle,
			Health:          heartbeat.SessionHealthHealthy,
			Attachable:      true,
			EligibleForWake: true,
			LastPresenceAt:  baseAt.Add(90 * time.Second),
			UpdatedAt:       baseAt.Add(90 * time.Second),
		}
		healthStore.seed(t, orphan)

		clock.Set(baseAt.Add(2 * time.Minute))
		restarted := newManagerWithHarness(
			t,
			h,
			WithNow(clock.Now),
			WithSessionHealthStore(healthStore),
			WithSessionHealthConfig(aghconfig.HeartbeatConfig{SessionHealthStaleAfter: time.Minute}),
		)

		result, err := restarted.RecoverSessionHealth(ctx)
		if err != nil {
			t.Fatalf("RecoverSessionHealth() error = %v", err)
		}
		if result.RefreshedActive != 0 {
			t.Fatalf("RecoverSessionHealth().RefreshedActive = %d, want 0 after restart", result.RefreshedActive)
		}
		if result.Recomputed != 2 {
			t.Fatalf("RecoverSessionHealth().Recomputed = %d, want 2 repaired/stale session rows", result.Recomputed)
		}
		if result.MarkedStale != 0 {
			t.Fatalf("RecoverSessionHealth().MarkedStale = %d, want 0 after direct orphan recovery", result.MarkedStale)
		}

		recovered := healthStore.mustGet(t, session.ID)
		assertSessionHealthState(t, recovered, heartbeat.SessionHealthStateStopped, heartbeat.SessionHealthDead)
		if recovered.EligibleForWake ||
			recovered.IneligibilityReason != string(heartbeat.SessionHealthReasonDead) ||
			recovered.ActivePrompt ||
			recovered.Attachable {
			t.Fatalf("recovered health = %#v, want dead wake-ineligible stopped row", recovered)
		}

		stale := healthStore.mustGet(t, orphan.SessionID)
		assertSessionHealthState(t, stale, heartbeat.SessionHealthStateDetached, heartbeat.SessionHealthStale)
		if stale.EligibleForWake || stale.IneligibilityReason != string(heartbeat.SessionHealthReasonStale) {
			t.Fatalf("orphan stale health = %#v, want stale wake-ineligible row", stale)
		}

		ineligible := false
		rows, err := restarted.ListSessionHealth(ctx, heartbeat.SessionHealthListQuery{EligibleForWake: &ineligible})
		if err != nil {
			t.Fatalf("ListSessionHealth(ineligible) error = %v", err)
		}
		gotIDs := sessionHealthIDs(rows)
		wantIDs := []string{orphan.SessionID, session.ID}
		slices.Sort(gotIDs)
		slices.Sort(wantIDs)
		if !slices.Equal(gotIDs, wantIDs) {
			t.Fatalf("ineligible session ids = %#v, want %#v", gotIDs, wantIDs)
		}
	})
}

func TestManagerSessionHealthTaskLeaseIsolation(t *testing.T) {
	t.Run("Should not couple session health code to task lease packages", func(t *testing.T) {
		t.Parallel()

		for _, fileName := range []string{
			"health.go",
			"prompt_activity.go",
			"manager_prompt.go",
			"manager_helpers.go",
			"manager_lifecycle.go",
		} {
			path := filepath.Join(".", fileName)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile(%q) error = %v", path, err)
			}
			text := string(content)
			for _, forbidden := range []string{
				"github.com/compozy/agh/internal/task",
				"LeaseHeartbeat",
				"HeartbeatRunLease",
				"RenewRunLease",
			} {
				if strings.Contains(text, forbidden) {
					t.Fatalf(
						"%s contains %q, want session health isolated from task lease mutation",
						fileName,
						forbidden,
					)
				}
			}
		}
	})
}

func TestManagerSessionHealthPromptPermissionBoundary(t *testing.T) {
	t.Run("Should keep prompt permission routing independent from health presence updates", func(t *testing.T) {
		ctx := testutil.Context(t)
		h := newHarness(t)
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Fatalf("Stop() cleanup error = %v", err)
			}
		})

		proc := session.processHandle()
		if proc == nil {
			t.Fatal("session process = nil, want active process")
		}
		permissionCalls := 0
		proc.requestPermissionFn = func(
			reqCtx context.Context,
			_ acp.RequestPermissionRequest,
		) (acp.RequestPermissionResponse, error) {
			if err := reqCtx.Err(); err != nil {
				return acp.RequestPermissionResponse{}, err
			}
			permissionCalls++
			return acp.RequestPermissionResponse{}, nil
		}

		if _, err := h.manager.RequestPermission(ctx, session.ID, acp.RequestPermissionRequest{}); err != nil {
			t.Fatalf("RequestPermission(active) error = %v", err)
		}
		if permissionCalls != 1 {
			t.Fatalf("permission calls = %d, want 1", permissionCalls)
		}
		if _, err := h.manager.RequestPermission(
			nilContextForGuardTest(),
			session.ID,
			acp.RequestPermissionRequest{},
		); err == nil {
			t.Fatal("Manager.RequestPermission(nil context) error = nil, want non-nil")
		}
		if _, err := h.manager.RequestPermission(ctx, " ", acp.RequestPermissionRequest{}); err == nil {
			t.Fatal("Manager.RequestPermission(blank id) error = nil, want non-nil")
		}
		if _, err := h.manager.RequestPermission(ctx, "sess-missing", acp.RequestPermissionRequest{}); !errors.Is(
			err,
			ErrSessionNotFound,
		) {
			t.Fatalf("Manager.RequestPermission(missing) error = %v, want ErrSessionNotFound", err)
		}

		var nilSession *Session
		if _, err := nilSession.RequestPermission(ctx, acp.RequestPermissionRequest{}); err == nil {
			t.Fatal("Session.RequestPermission(nil session) error = nil, want non-nil")
		}
		if _, err := session.RequestPermission(nilContextForGuardTest(), acp.RequestPermissionRequest{}); err == nil {
			t.Fatal("Session.RequestPermission(nil context) error = nil, want non-nil")
		}
		stopped := &Session{ID: "sess-stopped", State: StateStopped}
		if _, err := stopped.RequestPermission(
			ctx,
			acp.RequestPermissionRequest{},
		); !errors.Is(err, ErrSessionNotActive) {
			t.Fatalf("Session.RequestPermission(stopped) error = %v, want ErrSessionNotActive", err)
		}
		noProcess := &Session{ID: "sess-no-process", State: StateActive}
		if _, err := noProcess.RequestPermission(ctx, acp.RequestPermissionRequest{}); err == nil {
			t.Fatal("Session.RequestPermission(no process) error = nil, want non-nil")
		}
		unsupported := &Session{
			ID:      "sess-unsupported",
			State:   StateActive,
			process: NewAgentProcess(AgentProcessOptions{SessionID: "acp-unsupported"}),
		}
		if _, err := unsupported.RequestPermission(ctx, acp.RequestPermissionRequest{}); err == nil {
			t.Fatal("Session.RequestPermission(unsupported process) error = nil, want non-nil")
		}
		if err := h.manager.Stop(ctx, session.ID); err != nil {
			t.Fatalf("Stop() before inactive permission request error = %v", err)
		}
		if _, err := h.manager.RequestPermission(ctx, session.ID, acp.RequestPermissionRequest{}); !errors.Is(
			err,
			ErrSessionNotActive,
		) {
			t.Fatalf("Manager.RequestPermission(stopped) error = %v, want ErrSessionNotActive", err)
		}
	})
}

type sessionHealthTestClock struct {
	mu  sync.Mutex
	now time.Time
}

func newSessionHealthTestClock(now time.Time) *sessionHealthTestClock {
	return &sessionHealthTestClock{now: now}
}

func (c *sessionHealthTestClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *sessionHealthTestClock) Set(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = now
}

type fakeSessionHealthStore struct {
	mu         sync.Mutex
	rows       map[string]heartbeat.SessionHealth
	upserts    int
	gets       int
	batchLists int
}

func newFakeSessionHealthStore() *fakeSessionHealthStore {
	return &fakeSessionHealthStore{
		rows: make(map[string]heartbeat.SessionHealth),
	}
}

func (s *fakeSessionHealthStore) UpsertSessionHealth(
	ctx context.Context,
	health heartbeat.SessionHealth,
) (heartbeat.SessionHealth, error) {
	if err := fakeSessionHealthContextErr(ctx); err != nil {
		return heartbeat.SessionHealth{}, err
	}
	normalized := health.Normalize()
	if err := normalized.Validate(); err != nil {
		return heartbeat.SessionHealth{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows[normalized.SessionID] = normalized
	s.upserts++
	return normalized, nil
}

func (s *fakeSessionHealthStore) GetSessionHealth(
	ctx context.Context,
	sessionID string,
) (heartbeat.SessionHealth, error) {
	if err := fakeSessionHealthContextErr(ctx); err != nil {
		return heartbeat.SessionHealth{}, err
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return heartbeat.SessionHealth{}, fmt.Errorf("%w: session id is required", heartbeat.ErrInvalidSessionHealth)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.gets++
	row, ok := s.rows[target]
	if !ok {
		return heartbeat.SessionHealth{}, fmt.Errorf(
			"test: session health %q: %w",
			target,
			heartbeat.ErrSessionHealthNotFound,
		)
	}
	return row, nil
}

func (s *fakeSessionHealthStore) ListSessionHealthByIDs(
	ctx context.Context,
	sessionIDs []string,
) ([]heartbeat.SessionHealth, error) {
	if err := fakeSessionHealthContextErr(ctx); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.batchLists++
	rows := make([]heartbeat.SessionHealth, 0, len(sessionIDs))
	seen := make(map[string]struct{}, len(sessionIDs))
	for _, raw := range sessionIDs {
		id := strings.TrimSpace(raw)
		if id == "" {
			return nil, fmt.Errorf("%w: session id is required", heartbeat.ErrInvalidSessionHealth)
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		if row, exists := s.rows[id]; exists {
			rows = append(rows, row)
		}
	}
	sortFakeSessionHealthRows(rows)
	return rows, nil
}

func (s *fakeSessionHealthStore) readCounts() (gets int, batchLists int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gets, s.batchLists
}

func (s *fakeSessionHealthStore) ListSessionHealth(
	ctx context.Context,
	query heartbeat.SessionHealthListQuery,
) ([]heartbeat.SessionHealth, error) {
	if err := fakeSessionHealthContextErr(ctx); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	rows := make([]heartbeat.SessionHealth, 0, len(s.rows))
	for _, row := range s.rows {
		if !sessionHealthMatchesQuery(row, query) {
			continue
		}
		rows = append(rows, row)
	}
	sortFakeSessionHealthRows(rows)
	if query.Limit > 0 && len(rows) > query.Limit {
		rows = rows[:query.Limit]
	}
	return rows, nil
}

func (s *fakeSessionHealthStore) ListSessionHealthRecoveryInputs(
	ctx context.Context,
	limit int,
) ([]heartbeat.SessionHealth, error) {
	if limit < 0 {
		return nil, fmt.Errorf("%w: invalid recovery limit %d", heartbeat.ErrInvalidSessionHealth, limit)
	}
	return s.ListSessionHealth(ctx, heartbeat.SessionHealthListQuery{Limit: limit})
}

func (s *fakeSessionHealthStore) MarkSessionHealthStale(
	ctx context.Context,
	cutoff time.Time,
	updatedAt time.Time,
) (int64, error) {
	if err := fakeSessionHealthContextErr(ctx); err != nil {
		return 0, err
	}
	if cutoff.IsZero() {
		return 0, fmt.Errorf("%w: stale cutoff is required", heartbeat.ErrInvalidSessionHealth)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	var marked int64
	for id, row := range s.rows {
		if row.Health == heartbeat.SessionHealthStale || row.Health == heartbeat.SessionHealthDead {
			continue
		}
		if row.State != heartbeat.SessionHealthStateIdle || row.ActivePrompt {
			continue
		}
		if !row.LastPresenceAt.IsZero() && !row.LastPresenceAt.Before(cutoff) {
			continue
		}
		row.Health = heartbeat.SessionHealthStale
		row.EligibleForWake = false
		row.IneligibilityReason = string(heartbeat.SessionHealthReasonStale)
		row.UpdatedAt = updatedAt.UTC()
		s.rows[id] = row.Normalize()
		marked++
	}
	return marked, nil
}

func (s *fakeSessionHealthStore) seed(t *testing.T, rows ...heartbeat.SessionHealth) {
	t.Helper()

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, row := range rows {
		normalized := row.Normalize()
		if err := normalized.Validate(); err != nil {
			t.Fatalf("seed session health %q error = %v", normalized.SessionID, err)
		}
		s.rows[normalized.SessionID] = normalized
	}
}

func (s *fakeSessionHealthStore) mustGet(t *testing.T, sessionID string) heartbeat.SessionHealth {
	t.Helper()

	row, err := s.GetSessionHealth(testutil.Context(t), sessionID)
	if err != nil {
		t.Fatalf("GetSessionHealth(%q) error = %v", sessionID, err)
	}
	return row
}

func (s *fakeSessionHealthStore) upsertCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.upserts
}

type recordingSessionHealthHooks struct {
	mu       sync.Mutex
	payloads []hookspkg.SessionHealthUpdateAfterPayload
}

func (h *recordingSessionHealthHooks) DispatchSessionHealthUpdateAfter(
	_ context.Context,
	payload hookspkg.SessionHealthUpdateAfterPayload,
) (hookspkg.SessionHealthUpdateAfterPayload, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.payloads = append(h.payloads, payload)
	return payload, nil
}

func (h *recordingSessionHealthHooks) snapshot() []hookspkg.SessionHealthUpdateAfterPayload {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]hookspkg.SessionHealthUpdateAfterPayload(nil), h.payloads...)
}

func fakeSessionHealthContextErr(ctx context.Context) error {
	if ctx == nil {
		return errors.New("test: context is required")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("test: context canceled: %w", err)
	}
	return nil
}

func sortFakeSessionHealthRows(rows []heartbeat.SessionHealth) {
	slices.SortFunc(rows, func(left heartbeat.SessionHealth, right heartbeat.SessionHealth) int {
		if !left.UpdatedAt.Equal(right.UpdatedAt) {
			if left.UpdatedAt.After(right.UpdatedAt) {
				return -1
			}
			return 1
		}
		switch {
		case left.SessionID > right.SessionID:
			return -1
		case left.SessionID < right.SessionID:
			return 1
		default:
			return 0
		}
	})
}

func assertSessionHealthReadModel(
	t *testing.T,
	health heartbeat.SessionHealth,
	sessionID string,
	workspaceID string,
	agentName string,
) {
	t.Helper()

	if health.SessionID != sessionID {
		t.Fatalf("SessionID = %q, want %q", health.SessionID, sessionID)
	}
	if health.WorkspaceID != workspaceID {
		t.Fatalf("WorkspaceID = %q, want %q", health.WorkspaceID, workspaceID)
	}
	if health.AgentName != agentName {
		t.Fatalf("AgentName = %q, want %q", health.AgentName, agentName)
	}
	if health.UpdatedAt.IsZero() {
		t.Fatal("UpdatedAt is zero, want route-ready timestamp")
	}
	if err := health.Validate(); err != nil {
		t.Fatalf("SessionHealth.Validate() error = %v", err)
	}
}

func assertSessionHealthState(
	t *testing.T,
	health heartbeat.SessionHealth,
	state heartbeat.SessionHealthState,
	status heartbeat.SessionHealthStatus,
) {
	t.Helper()

	if health.State != state {
		t.Fatalf("State = %q, want %q in %#v", health.State, state, health)
	}
	if health.Health != status {
		t.Fatalf("Health = %q, want %q in %#v", health.Health, status, health)
	}
}

func sessionHealthIDs(rows []heartbeat.SessionHealth) []string {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.SessionID)
	}
	return ids
}

func nilContextForGuardTest() context.Context {
	return nil
}
