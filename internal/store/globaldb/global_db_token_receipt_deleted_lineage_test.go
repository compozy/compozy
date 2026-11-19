package globaldb

import (
	"context"
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
		stamp := store.FormatTimestamp(at)
		for _, entry := range []struct{ id, parent, instance, kind string }{
			{main, "", instance.ID, ""},
			{middle, main, "", "spawn"},
			{leaf, middle, "", "spawn"},
			{"budget-lineage-fork", middle, "", "fork"},
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
		for _, sessionID := range []string{leaf, "budget-lineage-fork"} {
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
		var catalogEntries int
		if err := db.DB().
			QueryRowContext(ctx, `SELECT count(*) FROM sessions WHERE id=?`, middle).
			Scan(&catalogEntries); err != nil ||
			catalogEntries != 0 {
			t.Fatalf("deleted catalog entries=%d err=%v, want the parent still deleted", catalogEntries, err)
		}
	})
}
