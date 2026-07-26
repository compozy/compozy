package heartbeat

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/diagnostics"
)

func TestManagedWakeServiceDecision(t *testing.T) {
	t.Parallel()

	t.Run("Should use the latest valid policy snapshot at decision time", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		cfg := aghconfig.DefaultHeartbeatConfig()
		store := newFakeWakeStore(t)
		older := wakeSnapshot(t, cfg, "hb-older", "ws-1", "coder", base.Add(-time.Hour), "Older policy")
		invalid := older
		invalid.ID = "hb-invalid-newer"
		invalid.Digest = "sha256:invalid"
		invalid.ResolvedJSON = []byte(
			`{"schema_version":1,"present":true,"active":false,"valid":false,"config_provenance":{"digest":"sha256:config"}}`,
		)
		invalid.CreatedAt = base.Add(-time.Minute)
		latest := wakeSnapshot(t, cfg, "hb-latest", "ws-1", "coder", base, "Latest policy")
		store.snapshots = []Snapshot{older, invalid, latest}
		health := newFakeWakeHealth()
		health.rows["sess-1"] = eligibleWakeHealth("sess-1", "ws-1", "coder", base)
		prompter := &fakeWakePrompter{}
		service := newTestWakeService(t, store, health, prompter, cfg, base)

		decision, err := service.Wake(context.Background(), WakeRequest{
			WorkspaceID: "ws-1",
			AgentName:   "coder",
			SessionID:   "sess-1",
			Source:      WakeSourceScheduler,
		})
		if err != nil {
			t.Fatalf("Wake() error = %v", err)
		}
		if decision.Result != WakeResultSent || decision.PolicySnapshotID != "hb-latest" {
			t.Fatalf("Wake() = %#v, want sent decision for latest valid snapshot", decision)
		}
		requests := prompter.requestsSnapshot()
		if got, want := len(requests), 1; got != want {
			t.Fatalf("prompt requests = %d, want %d", got, want)
		}
		if !strings.Contains(requests[0].Message, "Latest policy") {
			t.Fatalf("wake prompt = %q, want latest policy summary", requests[0].Message)
		}
		assertWakePromptHasNoOwnershipCredentials(t, requests[0].Message)
		assertLastWakeEvent(t, store, WakeResultSent, WakeReasonSent, "hb-latest")
	})

	t.Run("Should evaluate dry run without prompt or persisted wake identifiers", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		cfg := aghconfig.DefaultHeartbeatConfig()
		store := newFakeWakeStore(t)
		store.snapshots = []Snapshot{wakeSnapshot(t, cfg, "hb-policy", "ws-1", "coder", base, "Policy")}
		health := newFakeWakeHealth()
		health.rows["sess-1"] = eligibleWakeHealth("sess-1", "ws-1", "coder", base)
		prompter := &fakeWakePrompter{}
		service := newTestWakeService(t, store, health, prompter, cfg, base)

		decision, err := service.Wake(context.Background(), WakeRequest{
			WorkspaceID: "ws-1",
			AgentName:   "coder",
			SessionID:   "sess-1",
			Source:      WakeSourceManual,
			DryRun:      true,
		})
		if err != nil {
			t.Fatalf("Wake(dry run) error = %v", err)
		}
		if decision.Result != WakeResultSent || decision.Reason != WakeReasonSent {
			t.Fatalf("Wake(dry run) = %#v, want would-send decision", decision)
		}
		if strings.TrimSpace(decision.WakeEventID) != "" || strings.TrimSpace(decision.SyntheticPromptID) != "" {
			t.Fatalf("Wake(dry run) = %#v, want no non-persisted identifiers", decision)
		}
		if got := len(prompter.requestsSnapshot()); got != 0 {
			t.Fatalf("prompt requests = %d, want 0", got)
		}
		if got := len(store.eventsSnapshot()); got != 0 {
			t.Fatalf("wake events = %d, want 0", got)
		}
		if state := store.stateSnapshot("ws-1/coder/sess-1"); state.WorkspaceID != "" {
			t.Fatalf("wake state = %#v, want no persisted dry-run state", state)
		}
	})

	t.Run("Should preserve optional synthetic correlation on sent wake prompts", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		cfg := aghconfig.DefaultHeartbeatConfig()
		store := newFakeWakeStore(t)
		store.snapshots = []Snapshot{wakeSnapshot(t, cfg, "hb-policy", "ws-1", "coder", base, "Policy")}
		health := newFakeWakeHealth()
		health.rows["sess-1"] = eligibleWakeHealth("sess-1", "ws-1", "coder", base)
		prompter := &fakeWakePrompter{}
		service := newTestWakeService(t, store, health, prompter, cfg, base)

		decision, err := service.Wake(context.Background(), WakeRequest{
			WorkspaceID: "ws-1",
			AgentName:   "coder",
			SessionID:   "sess-1",
			Source:      WakeSourceHarnessReentry,
			SyntheticCorrelation: WakeSyntheticCorrelation{
				TaskID:               "task-1",
				TaskRunID:            "run-1",
				WorkflowID:           "wf-1",
				ClaimTokenHash:       "sha256:claim-1",
				CoordinatorSessionID: "sess-coordinator-1",
			},
		})
		if err != nil {
			t.Fatalf("Wake() error = %v", err)
		}
		if decision.Result != WakeResultSent {
			t.Fatalf("Wake() result = %q, want sent", decision.Result)
		}

		requests := prompter.requestsSnapshot()
		if got, want := len(requests), 1; got != want {
			t.Fatalf("prompt requests = %d, want %d", got, want)
		}
		if got, want := requests[0].SyntheticCorrelation.TaskID, "task-1"; got != want {
			t.Fatalf("prompt synthetic task id = %q, want %q", got, want)
		}
		if got, want := requests[0].SyntheticCorrelation.TaskRunID, "run-1"; got != want {
			t.Fatalf("prompt synthetic task run id = %q, want %q", got, want)
		}
		if got, want := requests[0].SyntheticCorrelation.WorkflowID, "wf-1"; got != want {
			t.Fatalf("prompt synthetic workflow id = %q, want %q", got, want)
		}
		if got, want := requests[0].SyntheticCorrelation.ClaimTokenHash, "sha256:claim-1"; got != want {
			t.Fatalf("prompt synthetic claim token hash = %q, want %q", got, want)
		}
		if got, want := requests[0].SyntheticCorrelation.CoordinatorSessionID, "sess-coordinator-1"; got != want {
			t.Fatalf("prompt synthetic coordinator session id = %q, want %q", got, want)
		}
	})

	t.Run("Should skip ineligible session health with a closed audit reason", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		cfg := aghconfig.DefaultHeartbeatConfig()
		store := newFakeWakeStore(t)
		store.snapshots = []Snapshot{wakeSnapshot(t, cfg, "hb-policy", "ws-1", "coder", base, "Policy")}
		health := newFakeWakeHealth()
		row := eligibleWakeHealth("sess-1", "ws-1", "coder", base)
		row.EligibleForWake = false
		row.ActivePrompt = true
		row.State = SessionHealthStatePrompting
		row.IneligibilityReason = string(SessionHealthReasonPromptActive)
		health.rows["sess-1"] = row
		prompter := &fakeWakePrompter{}
		service := newTestWakeService(t, store, health, prompter, cfg, base)

		decision, err := service.Wake(context.Background(), WakeRequest{
			WorkspaceID: "ws-1",
			AgentName:   "coder",
			SessionID:   "sess-1",
			Source:      WakeSourceScheduler,
		})
		if err != nil {
			t.Fatalf("Wake() error = %v", err)
		}
		if decision.Result != WakeResultSkipped || decision.Reason != WakeReasonSessionPromptActive {
			t.Fatalf("Wake() = %#v, want skipped prompt-active decision", decision)
		}
		if got := len(prompter.requestsSnapshot()); got != 0 {
			t.Fatalf("prompt requests = %d, want 0", got)
		}
		assertLastWakeEvent(t, store, WakeResultSkipped, WakeReasonSessionPromptActive, "hb-policy")
	})

	t.Run("Should coalesce scheduler wakes during cooldown and rate limit manual wakes", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		cfg := aghconfig.DefaultHeartbeatConfig()
		store := newFakeWakeStore(t)
		store.snapshots = []Snapshot{wakeSnapshot(t, cfg, "hb-policy", "ws-1", "coder", base, "Policy")}
		store.states["ws-1/coder/sess-scheduler"] = WakeState{
			WorkspaceID:      "ws-1",
			AgentName:        "coder",
			SessionID:        "sess-scheduler",
			PolicySnapshotID: "hb-policy",
			LastWakeAt:       base.Add(-30 * time.Second),
			NextAllowedAt:    base.Add(30 * time.Second),
			CoalescedCount:   1,
			LastResult:       WakeResultSent,
			LastReason:       WakeReasonSent,
			UpdatedAt:        base.Add(-30 * time.Second),
		}
		store.states["ws-1/coder/sess-manual"] = WakeState{
			WorkspaceID:      "ws-1",
			AgentName:        "coder",
			SessionID:        "sess-manual",
			PolicySnapshotID: "hb-policy",
			LastWakeAt:       base.Add(-30 * time.Second),
			NextAllowedAt:    base.Add(30 * time.Second),
			LastResult:       WakeResultSent,
			LastReason:       WakeReasonSent,
			UpdatedAt:        base.Add(-30 * time.Second),
		}
		health := newFakeWakeHealth()
		health.rows["sess-scheduler"] = eligibleWakeHealth("sess-scheduler", "ws-1", "coder", base)
		health.rows["sess-manual"] = eligibleWakeHealth("sess-manual", "ws-1", "coder", base)
		service := newTestWakeService(t, store, health, &fakeWakePrompter{}, cfg, base)

		schedulerDecision, err := service.Wake(context.Background(), WakeRequest{
			WorkspaceID: "ws-1",
			AgentName:   "coder",
			SessionID:   "sess-scheduler",
			Source:      WakeSourceScheduler,
		})
		if err != nil {
			t.Fatalf("Wake(scheduler) error = %v", err)
		}
		manualDecision, err := service.Wake(context.Background(), WakeRequest{
			WorkspaceID: "ws-1",
			AgentName:   "coder",
			SessionID:   "sess-manual",
			Source:      WakeSourceManual,
		})
		if err != nil {
			t.Fatalf("Wake(manual) error = %v", err)
		}
		if schedulerDecision.Result != WakeResultCoalesced || schedulerDecision.Reason != WakeReasonCoalesced {
			t.Fatalf("Wake(scheduler) = %#v, want coalesced", schedulerDecision)
		}
		if manualDecision.Result != WakeResultRateLimited || manualDecision.Reason != WakeReasonCooldownActive {
			t.Fatalf("Wake(manual) = %#v, want cooldown rate limit", manualDecision)
		}
		state := store.stateSnapshot("ws-1/coder/sess-scheduler")
		if got, want := state.CoalescedCount, 2; got != want {
			t.Fatalf("coalesced count = %d, want %d", got, want)
		}
	})

	t.Run("Should honor active hours and quiet window boundaries", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		cfg := aghconfig.DefaultHeartbeatConfig()
		store := newFakeWakeStore(t)
		store.snapshots = []Snapshot{
			wakeSnapshotWithWindows(t, cfg, "hb-policy", "ws-1", "coder", base, "09:00", "10:00"),
		}
		health := newFakeWakeHealth()
		health.rows["sess-1"] = eligibleWakeHealth("sess-1", "ws-1", "coder", base)
		service := newTestWakeService(t, store, health, &fakeWakePrompter{}, cfg, base)

		decision, err := service.Wake(context.Background(), WakeRequest{
			WorkspaceID: "ws-1",
			AgentName:   "coder",
			SessionID:   "sess-1",
			Source:      WakeSourceScheduler,
		})
		if err != nil {
			t.Fatalf("Wake() error = %v", err)
		}
		if decision.Result != WakeResultSkipped || decision.Reason != WakeReasonQuietWindow {
			t.Fatalf("Wake() = %#v, want quiet-window skip", decision)
		}
		assertLastWakeEvent(t, store, WakeResultSkipped, WakeReasonQuietWindow, "hb-policy")
	})

	t.Run("Should record prompt gate races without duplicate wakes", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		cfg := aghconfig.DefaultHeartbeatConfig()
		store := newFakeWakeStore(t)
		store.snapshots = []Snapshot{wakeSnapshot(t, cfg, "hb-policy", "ws-1", "coder", base, "Policy")}
		health := newFakeWakeHealth()
		health.rows["sess-1"] = eligibleWakeHealth("sess-1", "ws-1", "coder", base)
		prompter := &fakeWakePrompter{err: ErrSyntheticPromptBusy}
		service := newTestWakeService(t, store, health, prompter, cfg, base)

		decision, err := service.Wake(context.Background(), WakeRequest{
			WorkspaceID: "ws-1",
			AgentName:   "coder",
			SessionID:   "sess-1",
			Source:      WakeSourceScheduler,
		})
		if err != nil {
			t.Fatalf("Wake() error = %v", err)
		}
		if decision.Result != WakeResultSkipped || decision.Reason != WakeReasonSessionPromptRace {
			t.Fatalf("Wake() = %#v, want prompt race skip", decision)
		}
		if got, want := len(prompter.requestsSnapshot()), 1; got != want {
			t.Fatalf("prompt requests = %d, want %d", got, want)
		}
		assertLastWakeEvent(t, store, WakeResultSkipped, WakeReasonSessionPromptRace, "hb-policy")
	})

	t.Run("Should write failed audit events when synthetic prompt dispatch fails", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		cfg := aghconfig.DefaultHeartbeatConfig()
		store := newFakeWakeStore(t)
		store.snapshots = []Snapshot{wakeSnapshot(t, cfg, "hb-policy", "ws-1", "coder", base, "Policy")}
		health := newFakeWakeHealth()
		health.rows["sess-1"] = eligibleWakeHealth("sess-1", "ws-1", "coder", base)
		service := newTestWakeService(
			t,
			store,
			health,
			&fakeWakePrompter{err: errors.New("driver refused prompt")},
			cfg,
			base,
		)

		decision, err := service.Wake(context.Background(), WakeRequest{
			WorkspaceID: "ws-1",
			AgentName:   "coder",
			SessionID:   "sess-1",
			Source:      WakeSourceScheduler,
		})
		if err != nil {
			t.Fatalf("Wake() error = %v", err)
		}
		if decision.Result != WakeResultFailed || decision.Reason != WakeReasonSyntheticPromptFailed {
			t.Fatalf("Wake() = %#v, want failed synthetic prompt decision", decision)
		}
		assertLastWakeEvent(t, store, WakeResultFailed, WakeReasonSyntheticPromptFailed, "hb-policy")
	})

	t.Run("Should redact synthetic prompt errors in wake diagnostics", func(t *testing.T) {
		t.Parallel()

		secret := "wake-diagnostic-secret-123456"
		cleanup := diagnostics.RegisterDynamicSecret(secret)
		t.Cleanup(cleanup)

		base := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		cfg := aghconfig.DefaultHeartbeatConfig()
		store := newFakeWakeStore(t)
		store.snapshots = []Snapshot{wakeSnapshot(t, cfg, "hb-policy", "ws-1", "coder", base, "Policy")}
		health := newFakeWakeHealth()
		health.rows["sess-1"] = eligibleWakeHealth("sess-1", "ws-1", "coder", base)
		service := newTestWakeService(
			t,
			store,
			health,
			&fakeWakePrompter{err: fmt.Errorf("driver refused prompt with token %s", secret)},
			cfg,
			base,
		)

		decision, err := service.Wake(context.Background(), WakeRequest{
			WorkspaceID: "ws-1",
			AgentName:   "coder",
			SessionID:   "sess-1",
			Source:      WakeSourceScheduler,
		})
		if err != nil {
			t.Fatalf("Wake() error = %v", err)
		}
		if decision.Result != WakeResultFailed || decision.Reason != WakeReasonSyntheticPromptFailed {
			t.Fatalf("Wake() = %#v, want failed synthetic prompt decision", decision)
		}
		if diagnosticsContain(decision.Diagnostics, secret) {
			t.Fatalf("Wake() diagnostics leaked registered secret: %#v", decision.Diagnostics)
		}
		if len(decision.Diagnostics) == 0 ||
			!strings.Contains(decision.Diagnostics[0].Message, "[REDACTED]") {
			t.Fatalf("Wake() diagnostics = %#v, want redacted marker", decision.Diagnostics)
		}
		assertLastWakeEvent(t, store, WakeResultFailed, WakeReasonSyntheticPromptFailed, "hb-policy")
	})

	t.Run("Should persist wake audit and cooldown before prompting", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name      string
			appendErr error
			upsertErr error
		}{
			{
				name:      "Should not prompt when wake audit append fails",
				appendErr: errors.New("append failed"),
			},
			{
				name:      "Should not prompt when wake cooldown state upsert fails",
				upsertErr: errors.New("upsert failed"),
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				base := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
				cfg := aghconfig.DefaultHeartbeatConfig()
				store := newFakeWakeStore(t)
				store.appendErr = tc.appendErr
				store.upsertErr = tc.upsertErr
				store.snapshots = []Snapshot{wakeSnapshot(t, cfg, "hb-policy", "ws-1", "coder", base, "Policy")}
				health := newFakeWakeHealth()
				health.rows["sess-1"] = eligibleWakeHealth("sess-1", "ws-1", "coder", base)
				prompter := &fakeWakePrompter{}
				service := newTestWakeService(t, store, health, prompter, cfg, base)

				_, err := service.Wake(context.Background(), WakeRequest{
					WorkspaceID: "ws-1",
					AgentName:   "coder",
					SessionID:   "sess-1",
					Source:      WakeSourceScheduler,
				})
				if err == nil {
					t.Fatal("Wake() error = nil, want persistence failure")
				}
				if got := len(prompter.requestsSnapshot()); got != 0 {
					t.Fatalf("prompt requests = %d, want 0 before persistence succeeds", got)
				}
			})
		}
	})

	t.Run("Should enforce configured max wakes per cycle", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		cfg := aghconfig.DefaultHeartbeatConfig()
		cfg.MaxWakesPerCycle = 1
		store := newFakeWakeStore(t)
		store.snapshots = []Snapshot{wakeSnapshot(t, cfg, "hb-policy", "ws-1", "coder", base, "Policy")}
		health := newFakeWakeHealth()
		health.rows["sess-1"] = eligibleWakeHealth("sess-1", "ws-1", "coder", base)
		health.rows["sess-2"] = eligibleWakeHealth("sess-2", "ws-1", "coder", base)
		prompter := &fakeWakePrompter{}
		service := newTestWakeService(t, store, health, prompter, cfg, base)

		decisions, err := service.WakeMany(context.Background(), []WakeRequest{
			{WorkspaceID: "ws-1", AgentName: "coder", SessionID: "sess-1", Source: WakeSourceScheduler},
			{WorkspaceID: "ws-1", AgentName: "coder", SessionID: "sess-2", Source: WakeSourceScheduler},
		})
		if err != nil {
			t.Fatalf("WakeMany() error = %v", err)
		}
		if got, want := len(decisions), 2; got != want {
			t.Fatalf("len(decisions) = %d, want %d", got, want)
		}
		if decisions[0].Result != WakeResultSent ||
			decisions[1].Result != WakeResultRateLimited ||
			decisions[1].Reason != WakeReasonHeartbeatRateLimited {
			t.Fatalf("WakeMany() = %#v, want sent then rate limited", decisions)
		}
		if got, want := len(prompter.requestsSnapshot()), 1; got != want {
			t.Fatalf("prompt requests = %d, want %d", got, want)
		}
		if got, want := len(store.eventsSnapshot()), 2; got != want {
			t.Fatalf("wake events = %d, want %d", got, want)
		}
	})

	t.Run("Should preserve WakeMany decision order when one request errors", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		cfg := aghconfig.DefaultHeartbeatConfig()
		store := newFakeWakeStore(t)
		store.snapshots = []Snapshot{wakeSnapshot(t, cfg, "hb-policy", "ws-1", "coder", base, "Policy")}
		health := newFakeWakeHealth()
		health.rows["sess-1"] = eligibleWakeHealth("sess-1", "ws-1", "coder", base)
		service := newTestWakeService(t, store, health, &fakeWakePrompter{}, cfg, base)

		decisions, err := service.WakeMany(context.Background(), []WakeRequest{
			{WorkspaceID: "ws-1", AgentName: "coder", SessionID: "sess-1", Source: WakeSourceScheduler},
			{AgentName: "coder", SessionID: "sess-2", Source: WakeSourceScheduler},
		})
		if err == nil {
			t.Fatal("WakeMany(partial invalid request) error = nil, want aggregate error")
		}
		if got, want := len(decisions), 2; got != want {
			t.Fatalf("len(decisions) = %d, want %d", got, want)
		}
		if decisions[0].Result != WakeResultSent {
			t.Fatalf("first decision = %#v, want sent", decisions[0])
		}
		if decisions[1].Result != WakeResultFailed || len(decisions[1].Diagnostics) == 0 {
			t.Fatalf("second decision = %#v, want failed diagnostic placeholder", decisions[1])
		}
		if got, want := len(store.eventsSnapshot()), 1; got != want {
			t.Fatalf("wake events = %d, want %d", got, want)
		}
	})

	t.Run("Should redact WakeMany failure diagnostics", func(t *testing.T) {
		t.Parallel()

		secret := "wake-many-diagnostic-secret-123456"
		cleanup := diagnostics.RegisterDynamicSecret(secret)
		t.Cleanup(cleanup)

		base := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		cfg := aghconfig.DefaultHeartbeatConfig()
		store := newFakeWakeStore(t)
		store.appendErr = fmt.Errorf("append failed with token %s", secret)
		store.snapshots = []Snapshot{wakeSnapshot(t, cfg, "hb-policy", "ws-1", "coder", base, "Policy")}
		health := newFakeWakeHealth()
		health.rows["sess-1"] = eligibleWakeHealth("sess-1", "ws-1", "coder", base)
		service := newTestWakeService(t, store, health, &fakeWakePrompter{}, cfg, base)

		decisions, err := service.WakeMany(context.Background(), []WakeRequest{
			{WorkspaceID: "ws-1", AgentName: "coder", SessionID: "sess-1", Source: WakeSourceScheduler},
		})
		if err == nil {
			t.Fatal("WakeMany(store failure) error = nil, want aggregate error")
		}
		if got, want := len(decisions), 1; got != want {
			t.Fatalf("len(decisions) = %d, want %d", got, want)
		}
		if decisions[0].Result != WakeResultFailed || len(decisions[0].Diagnostics) == 0 {
			t.Fatalf("WakeMany() = %#v, want failed diagnostic placeholder", decisions)
		}
		if diagnosticsContain(decisions[0].Diagnostics, secret) {
			t.Fatalf("WakeMany() diagnostics leaked registered secret: %#v", decisions[0].Diagnostics)
		}
		if !strings.Contains(decisions[0].Diagnostics[0].Message, "[REDACTED]") {
			t.Fatalf("WakeMany() diagnostics = %#v, want redacted marker", decisions[0].Diagnostics)
		}
	})
}

func TestManagedWakeServiceClosedSkipsAndValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should validate constructor and wake request dependencies", func(t *testing.T) {
		t.Parallel()

		cfg := aghconfig.DefaultHeartbeatConfig()
		store := newFakeWakeStore(t)
		health := newFakeWakeHealth()
		prompter := &fakeWakePrompter{}
		if _, err := NewManagedWakeService(nil, health, prompter, cfg); err == nil {
			t.Fatal("NewManagedWakeService(nil store) error = nil, want validation error")
		}
		if _, err := NewManagedWakeService(store, nil, prompter, cfg); err == nil {
			t.Fatal("NewManagedWakeService(nil health) error = nil, want validation error")
		}
		if _, err := NewManagedWakeService(store, health, nil, cfg); err == nil {
			t.Fatal("NewManagedWakeService(nil prompter) error = nil, want validation error")
		}
		invalidConfig := cfg
		invalidConfig.WakeCooldown = 0
		if _, err := NewManagedWakeService(store, health, prompter, invalidConfig); err == nil {
			t.Fatal("NewManagedWakeService(invalid config) error = nil, want validation error")
		}
		service := newTestWakeService(t, store, health, prompter, cfg, time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC))
		if _, err := service.Wake(context.Background(), WakeRequest{}); err == nil {
			t.Fatal("Wake(empty request) error = nil, want validation error")
		}
	})

	t.Run("Should record disabled config decisions without loading policy", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		cfg := aghconfig.DefaultHeartbeatConfig()
		cfg.Enabled = false
		store := newFakeWakeStore(t)
		health := newFakeWakeHealth()
		service := newTestWakeService(t, store, health, &fakeWakePrompter{}, cfg, base)

		decision, err := service.Wake(context.Background(), WakeRequest{
			WorkspaceID: "ws-1",
			AgentName:   "coder",
			SessionID:   "sess-1",
			Source:      WakeSourceManual,
		})
		if err != nil {
			t.Fatalf("Wake(disabled) error = %v", err)
		}
		if decision.Result != WakeResultSkipped || decision.Reason != WakeReasonHeartbeatDisabled {
			t.Fatalf("Wake(disabled) = %#v, want disabled skip", decision)
		}
		assertLastWakeEvent(t, store, WakeResultSkipped, WakeReasonHeartbeatDisabled, "")
	})

	t.Run("Should record no policy decisions and preserve generated ids", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		cfg := aghconfig.DefaultHeartbeatConfig()
		store := newFakeWakeStore(t)
		service, err := NewManagedWakeService(
			store,
			newFakeWakeHealth(),
			&fakeWakePrompter{},
			cfg,
			WithWakeClock(func() time.Time { return base }),
		)
		if err != nil {
			t.Fatalf("NewManagedWakeService() error = %v", err)
		}

		decision, err := service.Wake(context.Background(), WakeRequest{
			WorkspaceID: "ws-1",
			AgentName:   "coder",
			SessionID:   "sess-1",
			Source:      WakeSourceScheduler,
		})
		if err != nil {
			t.Fatalf("Wake(no policy) error = %v", err)
		}
		if decision.Result != WakeResultSkipped ||
			decision.Reason != WakeReasonHeartbeatNoPolicy ||
			strings.TrimSpace(decision.WakeEventID) == "" {
			t.Fatalf("Wake(no policy) = %#v, want no-policy skip with generated id", decision)
		}
		assertLastWakeEvent(t, store, WakeResultSkipped, WakeReasonHeartbeatNoPolicy, "")
	})

	t.Run("Should fail closed for stale config digests and disabled policy snapshots", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		snapshotConfig := aghconfig.DefaultHeartbeatConfig()
		serviceConfig := snapshotConfig
		serviceConfig.WakeCooldown = 2 * time.Minute
		store := newFakeWakeStore(t)
		store.snapshots = []Snapshot{
			wakeSnapshot(t, snapshotConfig, "hb-stale-config", "ws-1", "coder", base, "Policy"),
		}
		health := newFakeWakeHealth()
		health.rows["sess-1"] = eligibleWakeHealth("sess-1", "ws-1", "coder", base)
		service := newTestWakeService(t, store, health, &fakeWakePrompter{}, serviceConfig, base)

		decision, err := service.Wake(context.Background(), WakeRequest{
			WorkspaceID: "ws-1",
			AgentName:   "coder",
			SessionID:   "sess-1",
			Source:      WakeSourceScheduler,
		})
		if err != nil {
			t.Fatalf("Wake(stale config) error = %v", err)
		}
		if decision.Result != WakeResultSkipped ||
			decision.Reason != WakeReasonHeartbeatInvalid ||
			len(decision.Diagnostics) == 0 {
			t.Fatalf("Wake(stale config) = %#v, want invalid skip with diagnostics", decision)
		}

		disabledStore := newFakeWakeStore(t)
		disabledStore.snapshots = []Snapshot{
			wakeDisabledSnapshot(t, snapshotConfig, "hb-disabled", "ws-1", "coder", base),
		}
		disabledService := newTestWakeService(t, disabledStore, health, &fakeWakePrompter{}, snapshotConfig, base)
		decision, err = disabledService.Wake(context.Background(), WakeRequest{
			WorkspaceID: "ws-1",
			AgentName:   "coder",
			SessionID:   "sess-1",
			Source:      WakeSourceScheduler,
		})
		if err != nil {
			t.Fatalf("Wake(disabled policy) error = %v", err)
		}
		if decision.Result != WakeResultSkipped || decision.Reason != WakeReasonHeartbeatDisabled {
			t.Fatalf("Wake(disabled policy) = %#v, want disabled skip", decision)
		}
	})

	t.Run("Should skip missing mismatched and non-attachable session health", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		cfg := aghconfig.DefaultHeartbeatConfig()
		store := newFakeWakeStore(t)
		store.snapshots = []Snapshot{wakeSnapshot(t, cfg, "hb-policy", "ws-1", "coder", base, "Policy")}
		health := newFakeWakeHealth()
		health.rows["sess-mismatch"] = eligibleWakeHealth("sess-mismatch", "ws-other", "coder", base)
		notAttachable := eligibleWakeHealth("sess-detached", "ws-1", "coder", base)
		notAttachable.EligibleForWake = false
		notAttachable.Attachable = false
		notAttachable.State = SessionHealthStateDetached
		notAttachable.IneligibilityReason = string(SessionHealthReasonNotAttachable)
		health.rows["sess-detached"] = notAttachable
		service := newTestWakeService(t, store, health, &fakeWakePrompter{}, cfg, base)

		missing, err := service.Wake(context.Background(), WakeRequest{
			WorkspaceID: "ws-1",
			AgentName:   "coder",
			SessionID:   "sess-missing",
			Source:      WakeSourceScheduler,
		})
		if err != nil {
			t.Fatalf("Wake(missing health) error = %v", err)
		}
		mismatch, err := service.Wake(context.Background(), WakeRequest{
			WorkspaceID: "ws-1",
			AgentName:   "coder",
			SessionID:   "sess-mismatch",
			Source:      WakeSourceScheduler,
		})
		if err != nil {
			t.Fatalf("Wake(mismatched health) error = %v", err)
		}
		detached, err := service.Wake(context.Background(), WakeRequest{
			WorkspaceID: "ws-1",
			AgentName:   "coder",
			SessionID:   "sess-detached",
			Source:      WakeSourceScheduler,
		})
		if err != nil {
			t.Fatalf("Wake(detached health) error = %v", err)
		}
		if missing.Reason != WakeReasonSessionNotFound ||
			mismatch.Reason != WakeReasonHeartbeatNoEligible ||
			detached.Reason != WakeReasonSessionNotAttachable {
			t.Fatalf(
				"health decisions = %#v/%#v/%#v, want missing/no-eligible/not-attachable",
				missing,
				mismatch,
				detached,
			)
		}
	})
}

func newTestWakeService(
	t *testing.T,
	store *fakeWakeStore,
	health *fakeWakeHealth,
	prompter *fakeWakePrompter,
	cfg aghconfig.HeartbeatConfig,
	now time.Time,
) *ManagedWakeService {
	t.Helper()

	service, err := NewManagedWakeService(
		store,
		health,
		prompter,
		cfg,
		WithWakeClock(func() time.Time { return now }),
		WithWakeIDGenerator(sequentialWakeIDGenerator()),
	)
	if err != nil {
		t.Fatalf("NewManagedWakeService() error = %v", err)
	}
	return service
}

func wakeSnapshot(
	t *testing.T,
	cfg aghconfig.HeartbeatConfig,
	id string,
	workspaceID string,
	agentName string,
	createdAt time.Time,
	summary string,
) Snapshot {
	t.Helper()

	return wakeSnapshotFromContent(t, cfg, id, workspaceID, agentName, createdAt, fmt.Sprintf(`---
version: 1
enabled: true
summary: %q
preferences:
  min_interval: "30m"
---
Inspect context before waking.
`, summary))
}

func wakeSnapshotWithWindows(
	t *testing.T,
	cfg aghconfig.HeartbeatConfig,
	id string,
	workspaceID string,
	agentName string,
	createdAt time.Time,
	start string,
	end string,
) Snapshot {
	t.Helper()

	return wakeSnapshotFromContent(t, cfg, id, workspaceID, agentName, createdAt, fmt.Sprintf(`---
version: 1
enabled: true
summary: "Windowed policy"
preferences:
  min_interval: "30m"
  active_hours:
    - timezone: "UTC"
      start: %q
      end: %q
---
Wake only inside configured active hours.
`, start, end))
}

func wakeDisabledSnapshot(
	t *testing.T,
	cfg aghconfig.HeartbeatConfig,
	id string,
	workspaceID string,
	agentName string,
	createdAt time.Time,
) Snapshot {
	t.Helper()

	return wakeSnapshotFromContent(t, cfg, id, workspaceID, agentName, createdAt, `---
version: 1
enabled: false
summary: "Disabled policy"
preferences:
  min_interval: "30m"
---
Disabled policies should not wake sessions.
`)
}

func wakeSnapshotFromContent(
	t *testing.T,
	cfg aghconfig.HeartbeatConfig,
	id string,
	workspaceID string,
	agentName string,
	createdAt time.Time,
	content string,
) Snapshot {
	t.Helper()

	root := t.TempDir()
	sourcePath := root + "/agents/" + agentName + "/" + FileName
	resolved, err := Parse(context.Background(), ParseRequest{
		SourcePath:    sourcePath,
		WorkspaceRoot: root,
		Content:       []byte(content),
		Config:        cfg,
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	snapshot, err := SnapshotFromResolved(id, workspaceID, agentName, &resolved, createdAt)
	if err != nil {
		t.Fatalf("SnapshotFromResolved() error = %v", err)
	}
	return snapshot
}

func eligibleWakeHealth(sessionID string, workspaceID string, agentName string, at time.Time) SessionHealth {
	return SessionHealth{
		SessionID:       sessionID,
		WorkspaceID:     workspaceID,
		AgentName:       agentName,
		State:           SessionHealthStateIdle,
		Health:          SessionHealthHealthy,
		Attachable:      true,
		EligibleForWake: true,
		LastActivityAt:  at.Add(-2 * time.Minute),
		LastPresenceAt:  at.Add(-time.Minute),
		UpdatedAt:       at,
	}
}

func assertWakePromptHasNoOwnershipCredentials(t *testing.T, prompt string) {
	t.Helper()

	for _, forbidden := range []string{"claim_token", "claim token", "lease token", "task lease", "queue"} {
		if strings.Contains(strings.ToLower(prompt), forbidden) {
			t.Fatalf("wake prompt %q contains forbidden ownership term %q", prompt, forbidden)
		}
	}
	if !strings.Contains(prompt, "/agent/context") {
		t.Fatalf("wake prompt %q does not reference /agent/context", prompt)
	}
}

func assertLastWakeEvent(
	t *testing.T,
	store *fakeWakeStore,
	result WakeResult,
	reason WakeReason,
	snapshotID string,
) {
	t.Helper()

	events := store.eventsSnapshot()
	if len(events) == 0 {
		t.Fatal("wake events = 0, want at least one event")
	}
	last := events[len(events)-1]
	if last.Result != result || last.Reason != reason || last.PolicySnapshotID != snapshotID {
		t.Fatalf("last wake event = %#v, want result=%s reason=%s snapshot=%s", last, result, reason, snapshotID)
	}
}

func sequentialWakeIDGenerator() func(prefix string) string {
	counts := make(map[string]int)
	return func(prefix string) string {
		counts[prefix]++
		return fmt.Sprintf("%s-%d", prefix, counts[prefix])
	}
}

type fakeWakeStore struct {
	mu        sync.Mutex
	snapshots []Snapshot
	states    map[string]WakeState
	events    []WakeEvent
	appendErr error
	upsertErr error
}

func newFakeWakeStore(t *testing.T) *fakeWakeStore {
	t.Helper()
	return &fakeWakeStore{
		states: make(map[string]WakeState),
		events: make([]WakeEvent, 0),
	}
}

func (s *fakeWakeStore) GetLatestValidHeartbeatSnapshot(
	_ context.Context,
	workspaceID string,
	agentName string,
) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	candidates := append([]Snapshot(nil), s.snapshots...)
	sort.SliceStable(candidates, func(i int, j int) bool {
		if !candidates[i].CreatedAt.Equal(candidates[j].CreatedAt) {
			return candidates[i].CreatedAt.After(candidates[j].CreatedAt)
		}
		return candidates[i].ID > candidates[j].ID
	})
	for _, snapshot := range candidates {
		if snapshot.WorkspaceID != workspaceID || snapshot.AgentName != agentName {
			continue
		}
		envelope, err := snapshot.ResolvedEnvelope()
		if err != nil {
			return Snapshot{}, err
		}
		if envelope.Valid {
			return snapshot, nil
		}
	}
	return Snapshot{}, fmt.Errorf("fake: latest heartbeat snapshot: %w", ErrSnapshotNotFound)
}

func (s *fakeWakeStore) GetHeartbeatWakeState(
	_ context.Context,
	workspaceID string,
	agentName string,
	sessionID string,
) (WakeState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := wakeStateTestKey(workspaceID, agentName, sessionID)
	state, ok := s.states[key]
	if !ok {
		return WakeState{}, fmt.Errorf("fake: wake state: %w", ErrWakeStateNotFound)
	}
	return state, nil
}

func (s *fakeWakeStore) UpsertHeartbeatWakeState(_ context.Context, state WakeState) (WakeState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.upsertErr != nil {
		return WakeState{}, s.upsertErr
	}
	normalized := state.Normalize()
	if err := normalized.Validate(); err != nil {
		return WakeState{}, err
	}
	s.states[wakeStateTestKey(normalized.WorkspaceID, normalized.AgentName, normalized.SessionID)] = normalized
	return normalized, nil
}

func (s *fakeWakeStore) AppendHeartbeatWakeEvent(_ context.Context, event WakeEvent) (WakeEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.appendErr != nil {
		return WakeEvent{}, s.appendErr
	}
	normalized := event.Normalize()
	if err := normalized.Validate(); err != nil {
		return WakeEvent{}, err
	}
	s.events = append(s.events, normalized)
	return normalized, nil
}

func (s *fakeWakeStore) RecordHeartbeatWakeDecision(
	_ context.Context,
	event WakeEvent,
	state WakeState,
) (WakeEvent, WakeState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.appendErr != nil {
		return WakeEvent{}, WakeState{}, s.appendErr
	}
	normalizedEvent := event.Normalize()
	if err := normalizedEvent.Validate(); err != nil {
		return WakeEvent{}, WakeState{}, err
	}
	if s.upsertErr != nil {
		return WakeEvent{}, WakeState{}, s.upsertErr
	}
	normalizedState := state.Normalize()
	if err := normalizedState.Validate(); err != nil {
		return WakeEvent{}, WakeState{}, err
	}
	s.events = append(s.events, normalizedEvent)
	s.states[wakeStateTestKey(
		normalizedState.WorkspaceID,
		normalizedState.AgentName,
		normalizedState.SessionID,
	)] = normalizedState
	return normalizedEvent, normalizedState, nil
}

func (s *fakeWakeStore) eventsSnapshot() []WakeEvent {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]WakeEvent(nil), s.events...)
}

func (s *fakeWakeStore) stateSnapshot(key string) WakeState {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.states[key]
}

func wakeStateTestKey(workspaceID string, agentName string, sessionID string) string {
	return strings.Join([]string{workspaceID, agentName, sessionID}, "/")
}

type fakeWakeHealth struct {
	mu   sync.Mutex
	rows map[string]SessionHealth
}

func newFakeWakeHealth() *fakeWakeHealth {
	return &fakeWakeHealth{rows: make(map[string]SessionHealth)}
}

func (h *fakeWakeHealth) GetSessionHealth(_ context.Context, sessionID string) (SessionHealth, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	health, ok := h.rows[strings.TrimSpace(sessionID)]
	if !ok {
		return SessionHealth{}, fmt.Errorf("fake: session health: %w", ErrSessionHealthNotFound)
	}
	return health, nil
}

type fakeWakePrompter struct {
	mu       sync.Mutex
	requests []SyntheticWakePromptRequest
	err      error
}

func (p *fakeWakePrompter) PromptHeartbeatWake(
	_ context.Context,
	req SyntheticWakePromptRequest,
) (SyntheticWakePromptResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.requests = append(p.requests, req)
	if p.err != nil {
		return SyntheticWakePromptResult{}, p.err
	}
	return SyntheticWakePromptResult{SyntheticPromptID: req.TurnID}, nil
}

func (p *fakeWakePrompter) requestsSnapshot() []SyntheticWakePromptRequest {
	p.mu.Lock()
	defer p.mu.Unlock()

	return append([]SyntheticWakePromptRequest(nil), p.requests...)
}
