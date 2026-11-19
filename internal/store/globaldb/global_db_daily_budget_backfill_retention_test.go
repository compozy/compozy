package globaldb

import (
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
)

func TestDailyBudgetBackfillDoesNotExpireNewerUsage(t *testing.T) {
	t.Run("Should retain recent session usage after out-of-order recovery and retention", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db := openTestGlobalDB(t)
		recent := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
		older := recent.AddDate(0, -1, 0)
		cutoff := recent.AddDate(0, 0, -7)
		if _, err := db.DB().ExecContext(ctx, `INSERT INTO sessions
			(id, profile_id, scope, agent_name, workspace_id, session_type, state, created_at, updated_at)
			VALUES ('backfill-retention', ?, 'global', 'writer', '', 'user', 'stopped', ?, ?)`,
			store.DefaultProfileID, store.FormatTimestamp(older), store.FormatTimestamp(recent)); err != nil {
			t.Fatal(err)
		}
		stats := store.TokenStatsUpdate{SessionID: "backfill-retention", TurnID: "newer",
			AgentName: "writer", TotalTokens: new(int64(100)),
			CostStatus: "unknown", CostSource: "none", Turns: 1, UpdatedAt: recent}
		daily := store.TokenUsageDailyUpdate{ProfileID: store.DefaultProfileID, Day: store.LocalDay(recent),
			AgentName: stats.AgentName, TotalTokens: stats.TotalTokens, CostStatus: stats.CostStatus,
			CostSource: stats.CostSource, Turns: 1, UpdatedAt: recent}
		if _, err := db.RecordTokenUsageOnce(ctx, stats, daily); err != nil {
			t.Fatal(err)
		}
		for _, recovered := range []struct {
			id    string
			at    time.Time
			count int64
		}{
			{id: "earlier-same-day", at: recent.Add(-time.Second), count: 2},
			{id: "older-day", at: older, count: 1},
		} {
			stats.TurnID, stats.TotalTokens, stats.UpdatedAt = recovered.id, &recovered.count, recovered.at
			daily.Day, daily.TotalTokens, daily.UpdatedAt = store.LocalDay(recovered.at), &recovered.count, recovered.at
			if err := db.BackfillDailyBudgetReceipt(ctx, stats, daily, true); err != nil {
				t.Fatal(err)
			}
		}
		result, err := db.SweepObservability(ctx, cutoff)
		if err != nil || result.DeletedTokenStats != 0 || result.DeletedTokenUsageDaily != 1 {
			t.Fatalf("retention=%+v err=%v; want only the expired daily rollup removed", result, err)
		}
		var total int64
		var updatedAt string
		if err := db.DB().QueryRowContext(ctx, `SELECT total_tokens, updated_at FROM token_stats
			WHERE session_id='backfill-retention'`).Scan(&total, &updatedAt); err != nil ||
			total != 103 || updatedAt != store.FormatTimestamp(recent) {
			t.Fatalf("session aggregate tokens=%d updated=%q error=%v; want 103 at newer Done", total, updatedAt, err)
		}
		if err := db.DB().QueryRowContext(ctx, `SELECT total_tokens, updated_at FROM token_usage_daily
			WHERE day=? AND agent_name='writer'`, store.LocalDay(recent)).Scan(&total, &updatedAt); err != nil ||
			total != 102 || updatedAt != store.FormatTimestamp(recent) {
			t.Fatalf("daily aggregate tokens=%d updated=%q error=%v; want 102 at newer Done", total, updatedAt, err)
		}
		var day string
		if err := db.DB().QueryRowContext(ctx, `SELECT day, created_at FROM token_usage_receipts
			WHERE session_id='backfill-retention' AND turn_id='older-day'`).Scan(&day, &updatedAt); err != nil ||
			day != store.LocalDay(older) || updatedAt != store.FormatTimestamp(older) {
			t.Fatalf("recovered receipt day=%q created=%q error=%v; want the original Done timestamp", day, updatedAt, err)
		}
	})
}
