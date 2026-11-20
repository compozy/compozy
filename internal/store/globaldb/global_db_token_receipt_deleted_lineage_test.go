package globaldb

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
)

func TestTokenUsageReceiptPreservesOwnedDescendantAfterPlainDelete(t *testing.T) {
	t.Run("Should count a surviving descendant's missing receipt after reopen", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		path := filepath.Join(t.TempDir(), "deleted-lineage.db")
		at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
		db, err := OpenGlobalDB(ctx, path, WithClock(func() time.Time { return at }))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Errorf("close lineage registry: %v", err)
			}
		})
		instance, _, err := db.Instances().AdmitCreate(ctx, agentinstance.Instance{
			ProfileID: store.DefaultProfileID, AgentName: "writer", CreatedByKind: "human", PermissionCeiling: "{}",
		}, 10)
		if err != nil {
			t.Fatal(err)
		}
		const main = "budget-lineage-main"
		const middle = "budget-lineage-middle"
		const leaf = "budget-lineage-leaf"
		const side = "budget-lineage-side"
		stamp := store.FormatTimestamp(at)
		for _, entry := range []struct{ id, parent, instance, kind string }{
			{main, "", instance.ID, ""},
			{middle, main, "", "spawn"},
			{leaf, middle, "", "spawn"},
			{"budget-lineage-fork", middle, "", "fork"},
			{side, main, "", "side"},
		} {
			if _, err := db.DB().ExecContext(ctx, `INSERT INTO sessions
				(id, profile_id, scope, agent_name, workspace_id, session_type, state,
				 parent_session_id, root_session_id, instance_id, lineage_kind, created_at, updated_at)
				VALUES (?, ?, 'global', 'writer', '', 'user', 'stopped', NULLIF(?,''), ?,
				 NULLIF(?,''), ?, ?, ?)`,
				entry.id, store.DefaultProfileID, entry.parent, main, entry.instance, entry.kind, stamp, stamp); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := db.Instances().BindMain(ctx, instance.ID, main); err != nil {
			t.Fatal(err)
		}
		if err := db.DeleteSession(ctx, middle); err != nil {
			t.Fatal(err)
		}
		if err := db.Close(ctx); err != nil {
			t.Fatal(err)
		}
		db, err = OpenGlobalDB(ctx, path, WithClock(func() time.Time { return at }))
		if err != nil {
			t.Fatal(err)
		}
		candidates, err := db.ListDailyBudgetReconcileSessions(ctx, instance.ID)
		if err != nil || len(candidates) != 2 || candidates[0].ID != leaf || candidates[1].ID != main {
			t.Fatalf("restart recovery candidates = %+v, err=%v; want the owned leaf and main", candidates, err)
		}
		for _, sessionID := range []string{leaf, "budget-lineage-fork", side} {
			stats := store.TokenStatsUpdate{SessionID: sessionID, TurnID: "pending-completion",
				AgentName: "writer", TotalTokens: new(int64(7)), CostStatus: "unknown",
				CostSource: "none", Turns: 1, UpdatedAt: at}
			daily := store.TokenUsageDailyUpdate{ProfileID: store.DefaultProfileID, Day: store.LocalDay(at),
				AgentName: stats.AgentName, TotalTokens: stats.TotalTokens, CostStatus: stats.CostStatus,
				CostSource: stats.CostSource, Turns: 1, UpdatedAt: at}
			for range 2 {
				if err := db.BackfillDailyBudgetReceipt(ctx, stats, daily, true); err != nil {
					t.Fatal(err)
				}
			}
		}
		usage, err := db.Instances().ReadDailyBudgetUsage(ctx, instance.ID, at)
		if err != nil || usage.Tokens != 7 {
			t.Fatalf("instance usage = %+v, err=%v, want only the owned leaf's 7 tokens", usage, err)
		}
		if err := db.Instances().SetDailyBudgetLimits(ctx, instance.ID, agentinstance.DailyBudgetLimits{
			Tokens: new(int64(7)),
		}); err != nil {
			t.Fatal(err)
		}
		if allowed, trip, err := db.Instances().AdmitDailyBudgetTurn(ctx, main); err != nil || allowed || trip == nil {
			t.Fatalf("main at cap: allowed=%v trip=%+v err=%v", allowed, trip, err)
		}
		if allowed, trip, err := db.Instances().AdmitDailyBudgetTurn(ctx, side); err != nil || !allowed || trip != nil {
			t.Fatalf("side chat with paused parent: allowed=%v trip=%+v err=%v", allowed, trip, err)
		}
		assertBudgetSideQueueIsolation(t, db, instance.ID, leaf, side, at)
		var catalogEntries int
		if err := db.DB().
			QueryRowContext(ctx, `SELECT count(*) FROM sessions WHERE id=?`, middle).
			Scan(&catalogEntries); err != nil ||
			catalogEntries != 0 {
			t.Fatalf("deleted catalog entries=%d err=%v, want the parent still deleted", catalogEntries, err)
		}
	})
}

func assertBudgetSideQueueIsolation(t *testing.T, db *GlobalDB, instanceID, leaf, side string, at time.Time) {
	t.Helper()
	ctx := t.Context()
	for _, id := range []string{leaf, side} {
		if _, err := db.DB().ExecContext(ctx, `UPDATE sessions SET state='active' WHERE id=?`, id); err != nil {
			t.Fatal(err)
		}
		if _, _, err := db.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID: id + "-input", SessionID: id, Text: "agent input",
			Origin: json.RawMessage(`{"kind":"session","session_id":"peer","hop":1}`), QueueCap: 2, Now: at,
		}); err != nil {
			t.Fatal(err)
		}
	}
	for _, input := range []struct {
		id  string
		app bool
	}{{id: "operator"}, {id: "app", app: true}} {
		if _, _, err := db.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID: input.id, SessionID: leaf, Text: "interactive input", HumanIngress: true, AppIngress: input.app,
			QueueCap: 3, Now: at,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if held, err := db.Instances().HeldWakes(ctx, instanceID); err != nil || held != 2 {
		t.Fatalf("held wakes = %d, %v; want the owned leaf's agent and app inputs", held, err)
	}
	if metrics, err := db.Instances().
		Metrics(ctx, store.DefaultProfileID, ""); err != nil ||
		metrics.HeldInputCount != 2 {
		t.Fatalf("held input metrics = %+v, %v; want the owned leaf's agent and app inputs", metrics, err)
	}
	entry, claimed, err := db.ClaimSelectedSessionInput(ctx, leaf, "operator", at)
	if err != nil || !claimed || entry.ID != "operator" {
		t.Fatalf("operator dispatch during budget hold = %s, %t, %v", entry.ID, claimed, err)
	}
	if entry, found, err := db.PeekNextSessionInput(ctx, leaf); err != nil || found {
		t.Fatalf("next dispatch with only held app and agent inputs = %s, %t, %v", entry.ID, found, err)
	}
	if err := db.ReleaseSessionInput(ctx, leaf, "operator", at); err != nil {
		t.Fatal(err)
	}
	if _, promoted, err := db.PromoteQueuedAgentSessionInputToSteer(
		ctx,
		side,
		side+"-input",
		"live-turn",
		at,
	); err != nil ||
		!promoted {
		t.Fatalf("side promotion = %t, %v; want allowed", promoted, err)
	}
	if _, reserved, err := db.ReserveSessionSteer(ctx, side, side+"-input", at); err != nil || !reserved {
		t.Fatalf("side reservation = %t, %v; want allowed", reserved, err)
	}
	if err := db.Instances().
		SetDailyBudgetLimits(ctx, instanceID, agentinstance.DailyBudgetLimits{Tokens: new(int64(8))}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Instances().Resume(ctx, instanceID, 10, true); err != nil {
		t.Fatal(err)
	}
	if sessions, err := db.Instances().
		QueuedSessions(ctx, instanceID, ""); err != nil || len(sessions) != 1 ||
		sessions[0] != leaf {
		t.Fatalf("post-resume queues = %v, %v; want the owned leaf behind the deleted parent", sessions, err)
	}
	assertBudgetRecoveryPreservesManualSideHold(t, db, instanceID, leaf, side, at)
}

func assertBudgetRecoveryPreservesManualSideHold(
	t *testing.T,
	db *GlobalDB,
	instanceID, leaf, side string,
	at time.Time,
) {
	t.Helper()
	ctx := t.Context()
	if _, err := db.Instances().Pause(ctx, instanceID, "manual"); err != nil {
		t.Fatal(err)
	}
	if quarantined, err := db.QuarantineDailyBudgetInstance(ctx, instanceID); err != nil || !quarantined {
		t.Fatalf("quarantine = %t, error = %v", quarantined, err)
	}
	if allowed, _, err := db.Instances().AdmitDailyBudgetTurn(ctx, leaf); err != nil || allowed {
		t.Fatalf("owned admission with pending recovery = %t, %v; want blocked", allowed, err)
	}
	if _, _, err := db.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
		ID: "side-manual-hold", SessionID: side, Text: "agent input",
		Origin: json.RawMessage(`{"kind":"session","session_id":"another-peer","hop":1}`), QueueCap: 2, Now: at,
	}); err != nil {
		t.Fatal(err)
	}
	if hold, err := db.SessionInputHold(ctx, side); err != nil || hold == nil || hold.Reason != "manual" {
		t.Fatalf("side manual hold = %+v, %v; want the preexisting pause", hold, err)
	}
	if held, err := db.Instances().HeldWakes(ctx, instanceID); err != nil || held != 3 {
		t.Fatalf(
			"held wakes with manual and recovery fences = %d, %v; want the two leaf inputs and the side input",
			held,
			err,
		)
	}
}
