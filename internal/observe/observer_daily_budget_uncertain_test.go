package observe

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/agentinstance"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/transcript"
	compozyworkspace "github.com/compozy/compozy/internal/workspace"
)

func registerBudgetRecoveryTestMain(t *testing.T, h *harness, id string, at time.Time) agentinstance.Instance {
	t.Helper()
	ctx := t.Context()
	instance, _, err := h.registry.Instances().AdmitCreate(ctx, agentinstance.Instance{
		ProfileID: store.DefaultProfileID, WorkspaceID: h.workspaceID,
		AgentName: "coder", CreatedByKind: "human", PermissionCeiling: "{}",
	}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.registry.RegisterInstanceSession(ctx, &store.SessionInfo{
		ID: id, ProfileID: store.DefaultProfileID, WorkspaceID: h.workspaceID,
		AgentName: "coder", Provider: "claude", RuntimeStatus: store.SessionRuntimeUnbound,
		State: "stopped", CreatedAt: at, UpdatedAt: at,
	}, instance.ID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := h.registry.Instances().BindMain(ctx, instance.ID, id); err != nil {
		t.Fatal(err)
	}
	return instance
}

func writeBudgetRecoveryTestLedger(t *testing.T, h *harness, id, workspaceID string, event *acp.AgentEvent) {
	t.Helper()
	ctx := t.Context()
	ledger, err := sessiondb.OpenSessionDB(ctx,
		store.SessionDBOwner{SessionID: id, WorkspaceID: workspaceID},
		store.SessionDBFile(filepath.Join(h.home.SessionsDir, id)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ledger.Close(ctx); err != nil {
			t.Errorf("close budget recovery ledger: %v", err)
		}
	}()
	if event == nil {
		return
	}
	payload, err := transcript.MarshalAgentEvent(*event)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ledger.AppendEventIfAbsent(ctx, store.SessionEvent{
		ID: "budget-test-terminal-" + event.TurnID, SessionID: id, TurnID: event.TurnID, Type: event.Type,
		AgentName: "coder", Timestamp: event.Timestamp, Content: payload,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestObserverDailyBudgetRefusesUnresolvedLegacySpend(t *testing.T) {
	for _, tc := range []struct {
		name                                                                        string
		offset                                                                      time.Duration
		missingRoot, recordedRoot, crossWorkspace, recovery, uncertain, liveReceipt bool
		previousReceiptDay                                                          bool
	}{
		{name: "Should refuse current-day usage with a missing parent", uncertain: true},
		{name: "Should refuse current-day usage with a missing superseded main", missingRoot: true, uncertain: true},
		{name: "Should recover the owner hint from a deleted root", missingRoot: true, recordedRoot: true, uncertain: true},
		{name: "Should cover owned spawn across workspaces", missingRoot: true, crossWorkspace: true, uncertain: true},
		{name: "Should cover incomplete recovery lineage", recovery: true, uncertain: true},
		{name: "Should keep ordinary totals and retain uncertain spend through deletion", uncertain: true, liveReceipt: true},
		{name: "Should keep an ambiguous receipt on its original local day", previousReceiptDay: true},
		{name: "Should allow a cap when unresolved usage predated adoption", offset: -2 * time.Hour},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
			h := newHarness(t, globaldb.WithClock(func() time.Time { return at }))
			h.observer.now = func() time.Time { return at }
			const main, parent, child = "budget-legacy-main", "budget-legacy-parent", "budget-legacy-leaf"
			instance := registerBudgetRecoveryTestMain(t, h, main, at)
			writeBudgetRecoveryTestLedger(t, h, main, h.workspaceID, nil)
			stamp := store.FormatTimestamp(at)
			childWorkspace := h.workspaceID
			if tc.crossWorkspace {
				const crossID = "ws_budget_cross_scope"
				if err := h.registry.InsertWorkspace(ctx, compozyworkspace.Workspace{
					ID: crossID, Name: "budget-cross-workspace", RootDir: t.TempDir(), CreatedAt: at, UpdatedAt: at,
				}); err != nil {
					t.Fatal(err)
				}
				childWorkspace = crossID
			}
			childKind := "spawn"
			if tc.recovery {
				childKind = "recovery"
			}
			for _, row := range []struct {
				id, parent, workspace, kind string
				depth                       int
			}{
				{parent, main, h.workspaceID, "spawn", 1}, {child, parent, childWorkspace, childKind, 2},
			} {
				if err := h.registry.RegisterSession(ctx, &store.SessionInfo{
					ID: row.id, ProfileID: store.DefaultProfileID, WorkspaceID: row.workspace,
					AgentName: "coder", Provider: "claude", SessionType: "user",
					RuntimeStatus: store.SessionRuntimeUnbound, State: "stopped", CreatedAt: at, UpdatedAt: at,
					Lineage: &store.SessionLineage{ParentSessionID: row.parent, RootSessionID: main,
						Kind: store.LineageKind(row.kind), SpawnDepth: row.depth},
				}); err != nil {
					t.Fatal(err)
				}
			}
			var triggerSQL string
			if err := h.registry.DB().QueryRowContext(ctx,
				"SELECT sql FROM sqlite_schema WHERE name='preserve_deleted_session_budget_lineage'").
				Scan(&triggerSQL); err != nil {
				t.Fatal(err)
			}
			if _, err := h.registry.DB().
				ExecContext(ctx, "DROP TRIGGER preserve_deleted_session_budget_lineage"); err != nil {
				t.Fatal(err)
			}
			if tc.recordedRoot {
				if err := h.registry.DeleteSession(ctx, parent); err != nil {
					t.Fatal(err)
				}
				const sibling = "budget-legacy-root-sibling"
				if _, err := h.registry.DB().ExecContext(ctx, `INSERT INTO sessions
					(id, profile_id, scope, agent_name, workspace_id, session_type, state,
					 parent_session_id, root_session_id, lineage_kind, spawn_depth, created_at, updated_at)
					VALUES (?, ?, 'workspace', 'coder', ?, 'user', 'stopped', ?, ?, 'spawn', 1, ?, ?)`,
					sibling, store.DefaultProfileID, h.workspaceID, main, main, stamp, stamp); err != nil {
					t.Fatal(err)
				}
				writeBudgetRecoveryTestLedger(t, h, sibling, h.workspaceID, nil)
				if _, err := h.registry.DB().ExecContext(ctx, triggerSQL); err != nil {
					t.Fatal(err)
				}
			}
			deleted := parent
			if tc.missingRoot {
				const replacement = "budget-legacy-new-main"
				if _, err := h.registry.DB().ExecContext(ctx, `INSERT INTO sessions
					(id, profile_id, scope, agent_name, workspace_id, session_type, state, instance_id, created_at, updated_at)
					VALUES (?, ?, 'workspace', 'coder', ?, 'user', 'stopped', ?, ?, ?)`,
					replacement, store.DefaultProfileID, h.workspaceID, instance.ID, stamp, stamp); err != nil {
					t.Fatal(err)
				}
				if _, err := h.registry.DB().ExecContext(ctx, "UPDATE agent_instances SET main_session_id=? WHERE id=?",
					replacement, instance.ID); err != nil {
					t.Fatal(err)
				}
				writeBudgetRecoveryTestLedger(t, h, replacement, h.workspaceID, nil)
				writeBudgetRecoveryTestLedger(t, h, parent, h.workspaceID, nil)
				deleted = main
			}
			if err := h.registry.DeleteSession(ctx, deleted); err != nil {
				t.Fatal(err)
			}
			usageAt := at.Add(tc.offset)
			event := acp.AgentEvent{
				Type: acp.EventTypeDone, TurnID: "legacy-unaccounted", Timestamp: usageAt,
				Usage: &acp.TokenUsage{TurnID: "legacy-unaccounted", TotalTokens: new(int64(120)), Timestamp: usageAt},
			}
			writeBudgetRecoveryTestLedger(t, h, child, childWorkspace, &event)
			if tc.liveReceipt {
				assertUnresolvedBudgetReceiptSurvivesDeletion(t, h, child, instance.ID, event)
			}
			if tc.previousReceiptDay {
				h.observer.OnAgentEvent(ctx, child, event)
				// Simulate a receipt observed before the daemon moved its local
				// date boundary, without changing process-wide time.Local.
				previous := store.LocalDay(at.AddDate(0, 0, -1))
				result, err := h.registry.DB().ExecContext(ctx,
					"UPDATE token_usage_receipts SET day=? WHERE session_id=?", previous, child)
				if err != nil {
					t.Fatal(err)
				}
				if affected, err := result.RowsAffected(); err != nil || affected != 1 {
					t.Fatalf("updated %d stored receipts, err=%v, want one", affected, err)
				}
				if _, err := h.registry.DB().ExecContext(ctx,
					"UPDATE agent_instance_daily_budget_uncertain_days SET day=? WHERE instance_id=?",
					previous, instance.ID); err != nil {
					t.Fatal(err)
				}
			}
			committed := false
			update := func() error {
				return h.observer.WithDailyBudgetUpdate(ctx, instance.ID, func(ctx context.Context) error {
					err := h.registry.Instances().SetDailyBudgetLimits(ctx, instance.ID,
						agentinstance.DailyBudgetLimits{Tokens: new(int64(100))})
					committed = err == nil
					return err
				})
			}
			err := update()
			if !tc.uncertain {
				if err != nil || !committed {
					t.Fatalf("pre-adoption activation committed=%t err=%v", committed, err)
				}
				return
			}
			if !errors.Is(err, agentinstance.ErrBudgetHistoryUncertain) || committed {
				t.Fatalf("ambiguous current-day activation committed=%t err=%v", committed, err)
			}
			at = at.AddDate(0, 0, 1)
			if err := update(); err != nil || !committed {
				t.Fatalf("eligible next-day activation committed=%t err=%v", committed, err)
			}
		})
	}
}

func assertUnresolvedBudgetReceiptSurvivesDeletion(
	t *testing.T, h *harness, child, instanceID string, event acp.AgentEvent,
) {
	t.Helper()
	ctx := t.Context()
	metrics := &recordingEventMetrics{}
	h.observer.eventMetrics = metrics
	for range 2 {
		h.observer.OnAgentEvent(ctx, child, event)
	}
	if count := len(metrics.usageSnapshot()); count != 1 {
		t.Fatalf("accepted usage metrics = %d, want one", count)
	}
	var sessionTotal, dailyTotal int64
	if err := h.registry.DB().QueryRowContext(ctx, `SELECT
		(SELECT total_tokens FROM token_stats WHERE session_id = ?),
		(SELECT SUM(total_tokens) FROM token_usage_daily WHERE profile_id = ? AND day = ?)`,
		child, store.DefaultProfileID, store.LocalDay(event.Timestamp)).Scan(&sessionTotal, &dailyTotal); err != nil ||
		sessionTotal != 120 || dailyTotal != 120 {
		t.Fatalf("ordinary usage session=%d daily=%d err=%v, want 120 once", sessionTotal, dailyTotal, err)
	}
	var assigned int
	if err := h.registry.DB().QueryRowContext(ctx,
		"SELECT count(*) FROM token_usage_receipts WHERE session_id = ? AND instance_id = ?", child, instanceID).
		Scan(&assigned); err != nil || assigned != 0 {
		t.Fatalf("attributed receipts = %d, err = %v", assigned, err)
	}
	if _, err := h.registry.Instances().
		ReadDailyBudgetUsage(ctx, instanceID, event.Timestamp); !errors.Is(
		err,
		agentinstance.ErrDailyBudgetUsageUnavailable,
	) {
		t.Fatalf("unsettled usage status err = %v", err)
	}
	state, err := h.registry.DailyBudgetReceiptState(ctx, child, event.TurnID, store.DefaultProfileID)
	if err != nil || !state.Present || !state.OwnershipUnresolved {
		t.Fatalf("receipt status = %+v err=%v", state, err)
	}
	if allowed, _, err := h.registry.AdmitDailyBudgetTurn(ctx, child); err != nil || !allowed {
		t.Fatalf("opt-out admission allowed=%t err=%v", allowed, err)
	}
	second := event
	second.TurnID = "legacy-durable-without-receipt"
	second = second.WithDailyBudgetUsageEvidence(true, false)
	second.Usage = &acp.TokenUsage{TurnID: second.TurnID, TotalTokens: new(int64(13)), Timestamp: event.Timestamp}
	writeBudgetRecoveryTestLedger(t, h, child, h.workspaceID, &second)
	if _, err := h.registry.DB().ExecContext(ctx, `CREATE TRIGGER fail_uncertain_test_projection
		BEFORE INSERT ON token_stats BEGIN SELECT RAISE(ABORT, 'fixture projection failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := h.observer.ReconcileBudgetSessionBeforeDeletion(
		ctx,
		child,
	); err == nil ||
		!strings.Contains(err.Error(), "fixture projection failure") {
		t.Fatalf("a global projection error must preserve the readable ledger, got %v", err)
	}
	if _, err := h.registry.DB().ExecContext(ctx, "DROP TRIGGER fail_uncertain_test_projection"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := h.observer.ReconcileBudgetSessionBeforeDeletion(ctx, child); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.registry.DB().QueryRowContext(ctx, `SELECT
		(SELECT total_tokens FROM token_stats WHERE session_id = ?),
		(SELECT SUM(total_tokens) FROM token_usage_daily WHERE profile_id = ? AND day = ?)`,
		child, store.DefaultProfileID, store.LocalDay(event.Timestamp)).Scan(&sessionTotal, &dailyTotal); err != nil ||
		sessionTotal != 133 || dailyTotal != 133 {
		t.Fatalf("recovered usage session=%d daily=%d err=%v, want 120 plus 13", sessionTotal, dailyTotal, err)
	}
	if err := h.registry.DeleteSession(ctx, child); err != nil {
		t.Fatal(err)
	}
}

func TestObserverDailyBudgetReplaysProviderCompletionWithoutUsage(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mode    compozyconfig.ProviderAuthMode
		unknown bool
	}{
		{name: "Should hold after a metered completion with no telemetry", mode: compozyconfig.ProviderAuthModeBoundSecret, unknown: true},
		{name: "Should retain included subscription usage", mode: compozyconfig.ProviderAuthModeNativeCLI},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
			h := newHarness(t, globaldb.WithClock(func() time.Time { return at }))
			h.observer.now = func() time.Time { return at }
			const id = "budget-provider-usage-missing"
			instance := registerBudgetRecoveryTestMain(t, h, id, at)
			if err := h.registry.Instances().SetDailyBudgetLimits(ctx, instance.ID,
				agentinstance.DailyBudgetLimits{USD: new(float64(1))}); err != nil {
				t.Fatal(err)
			}
			event := (acp.AgentEvent{Type: acp.EventTypeDone, TurnID: "provider-done", Timestamp: at}).
				WithDailyBudgetUsageEvidence(false, true).WithPromptRuntime(&acp.PromptRuntime{
				Provider: "claude", Model: "coder", MeteringAuthMode: string(tc.mode),
			})
			writeBudgetRecoveryTestLedger(t, h, id, h.workspaceID, &event)
			if err := h.observer.ReconcileDailyBudgetReceipts(ctx); err != nil {
				t.Fatal(err)
			}
			h.observer.OnAgentEvent(ctx, id, event)
			usage, err := h.registry.Instances().ReadDailyBudgetUsage(ctx, instance.ID, at)
			if err != nil || usage.Tokens != 0 || usage.HasUnpricedUsage != tc.unknown {
				t.Fatalf("reported usage=%+v err=%v want unknown=%t", usage, err, tc.unknown)
			}
			var stats, receipts int
			if err := h.registry.DB().QueryRowContext(ctx, `SELECT
				(SELECT count(*) FROM token_stats WHERE session_id=?),
				(SELECT count(*) FROM token_usage_receipts WHERE session_id=?)`, id, id).Scan(&stats, &receipts); err != nil ||
				stats != 0 || receipts != 1 {
				t.Fatalf(
					"legacy aggregates=%d receipts=%d err=%v; want no fabricated aggregates and one receipt",
					stats,
					receipts,
					err,
				)
			}
			allowed, _, err := h.registry.Instances().AdmitDailyBudgetTurn(ctx, id)
			if err != nil || allowed == tc.unknown {
				t.Fatalf("next turn allowed=%t err=%v, unpriced usage=%t", allowed, err, tc.unknown)
			}
		})
	}
}
