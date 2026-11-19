package globaldb

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestDailyBudgetBackfillDoesNotReprojectExistingLegacyUsage(t *testing.T) {
	t.Run("Should retain legacy totals during receipt backfill", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		// This simulates an older daemon that wrote an additive projection after
		// migration 207 but before the new receipt observer was deployed.
		now := time.Now().UTC().Add(2 * time.Minute)
		db, err := OpenGlobalDB(ctx, filepath.Join(t.TempDir(), "backfill.db"),
			WithClock(func() time.Time { return now }))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Errorf("close backfill DB: %v", err)
			}
		})
		instance, _, err := db.Instances().AdmitCreate(ctx, agentinstance.Instance{
			ProfileID: store.DefaultProfileID, AgentName: "writer", CreatedByKind: "human",
			PermissionCeiling: "{}",
		}, 10)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.DB().ExecContext(ctx, `INSERT INTO sessions
		(id, profile_id, scope, agent_name, workspace_id, instance_id, session_type, state, created_at, updated_at)
		VALUES ('budget-backfill', ?, 'global', 'writer', '', ?, 'user', 'stopped', ?, ?)`,
			store.DefaultProfileID, instance.ID, store.FormatTimestamp(now), store.FormatTimestamp(now)); err != nil {
			t.Fatal(err)
		}
		if err := db.Instances().SetDailyBudgetLimits(ctx, instance.ID, agentinstance.DailyBudgetLimits{
			Tokens: new(int64(100)),
		}); err != nil {
			t.Fatal(err)
		}
		stats := store.TokenStatsUpdate{
			SessionID: "budget-backfill", TurnID: "backfill-turn", AgentName: "writer",
			InputTokens: new(int64(80)), OutputTokens: new(int64(15)),
			CostStatus: "unknown", CostSource: "none", Turns: 1, UpdatedAt: now,
		}
		daily := store.TokenUsageDailyUpdate{
			ProfileID: store.DefaultProfileID, Day: store.LocalDay(now), AgentName: "writer",
			InputTokens: stats.InputTokens, OutputTokens: stats.OutputTokens,
			CostStatus: "unknown", CostSource: "none", Turns: 1, UpdatedAt: now,
		}
		if err := db.RecordTokenUsage(ctx, stats, daily); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if err := db.BackfillDailyBudgetReceipt(ctx, stats, daily, false); err != nil {
				t.Fatal(err)
			}
		}
		var receipts, projectedTurns int64
		if err := db.DB().QueryRowContext(ctx,
			`SELECT count(*) FROM token_usage_receipts WHERE session_id='budget-backfill'`).Scan(&receipts); err != nil {
			t.Fatal(err)
		}
		if err := db.DB().QueryRowContext(ctx,
			`SELECT turn_count FROM token_stats WHERE session_id='budget-backfill'`).Scan(&projectedTurns); err != nil {
			t.Fatal(err)
		}
		usage, err := db.Instances().ReadDailyBudgetUsage(ctx, instance.ID, now)
		if err != nil || receipts != 1 || projectedTurns != 1 || usage.Tokens != 95 {
			t.Fatalf("backfill receipt=%d legacy turns=%d usage=%+v err=%v", receipts, projectedTurns, usage, err)
		}
	})
	t.Run("Should not duplicate legacy projections with out-of-order event timestamps", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		start := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
		db, err := OpenGlobalDB(ctx, filepath.Join(t.TempDir(), "legacy-order.db"),
			WithClock(func() time.Time { return start.Add(10 * time.Minute) }))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Errorf("close legacy order DB: %v", err)
			}
		})
		if _, err := db.DB().ExecContext(ctx, `INSERT INTO sessions
		(id, profile_id, scope, agent_name, workspace_id, session_type, state, created_at, updated_at)
		VALUES ('legacy-order', ?, 'global', 'writer', '', 'user', 'stopped', ?, ?)`,
			store.DefaultProfileID, store.FormatTimestamp(start), store.FormatTimestamp(start)); err != nil {
			t.Fatal(err)
		}
		for index, stamp := range []time.Time{start.Add(5 * time.Minute), start.Add(4 * time.Minute)} {
			tokens := int64(10 + index)
			stats := store.TokenStatsUpdate{SessionID: "legacy-order", TurnID: "legacy", AgentName: "writer",
				TotalTokens: &tokens, CostStatus: "unknown", CostSource: "none", Turns: 1, UpdatedAt: stamp}
			daily := store.TokenUsageDailyUpdate{ProfileID: store.DefaultProfileID, Day: store.LocalDay(stamp),
				AgentName: "writer", TotalTokens: &tokens,
				CostStatus: "unknown", CostSource: "none", Turns: 1, UpdatedAt: stamp}
			if err := db.RecordTokenUsage(ctx, stats, daily); err != nil {
				t.Fatal(err)
			}
		}
		firstTokens := int64(10)
		first := store.TokenStatsUpdate{SessionID: "legacy-order", TurnID: "first-done", AgentName: "writer",
			TotalTokens: &firstTokens, CostStatus: "unknown", CostSource: "none", Turns: 1,
			UpdatedAt: start.Add(5 * time.Minute)}
		daily := store.TokenUsageDailyUpdate{ProfileID: store.DefaultProfileID,
			Day: store.LocalDay(first.UpdatedAt), AgentName: "writer", TotalTokens: &firstTokens,
			CostStatus: "unknown", CostSource: "none", Turns: 1, UpdatedAt: first.UpdatedAt}
		if err := db.BackfillDailyBudgetReceipt(ctx, first, daily, false); err != nil {
			t.Fatal(err)
		}
		var projectedTurns int
		if err := db.DB().QueryRowContext(ctx,
			`SELECT turn_count FROM token_stats WHERE session_id='legacy-order'`).Scan(&projectedTurns); err != nil {
			t.Fatal(err)
		}
		if projectedTurns != 2 {
			t.Fatalf("replaying one of two old turns projected %d times, want two", projectedTurns)
		}
	})
	t.Run("Should preserve the exact legacy projection when a different receipt is missing", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db := openTestGlobalDB(t)
		now := time.Now().UTC()
		stamp := store.FormatTimestamp(now)
		if _, err := db.DB().ExecContext(ctx, `INSERT INTO sessions
		(id, profile_id, scope, agent_name, workspace_id, session_type, state, created_at, updated_at)
		VALUES ('legacy-mixed', ?, 'global', 'writer', '', 'user', 'stopped', ?, ?)`,
			store.DefaultProfileID, stamp, stamp); err != nil {
			t.Fatal(err)
		}
		stats := store.TokenStatsUpdate{SessionID: "legacy-mixed", TurnID: "already-projected", AgentName: "writer",
			TotalTokens: new(int64(100)), CostStatus: "unknown", CostSource: "none", Turns: 1, UpdatedAt: now}
		daily := store.TokenUsageDailyUpdate{ProfileID: store.DefaultProfileID, Day: store.LocalDay(now),
			AgentName: "writer", TotalTokens: stats.TotalTokens,
			CostStatus: "unknown", CostSource: "none", Turns: 1, UpdatedAt: now}
		if err := db.RecordTokenUsage(ctx, stats, daily); err != nil {
			t.Fatal(err)
		}
		missingStats, missingDaily := stats, daily
		missingStats.TurnID = "unprojected"
		missingStats.TotalTokens = new(int64(1))
		missingDaily.TotalTokens = missingStats.TotalTokens
		if err := db.BackfillDailyBudgetReceipt(ctx, missingStats, missingDaily, false); err != nil {
			t.Fatal(err)
		}
		if err := db.BackfillDailyBudgetReceipt(ctx, stats, daily, false); err != nil {
			t.Fatal(err)
		}
		var projection, receipts int64
		if err := db.DB().QueryRowContext(ctx, `SELECT total_tokens FROM token_stats
		WHERE session_id='legacy-mixed'`).Scan(&projection); err != nil {
			t.Fatal(err)
		}
		if err := db.DB().QueryRowContext(ctx, `SELECT sum(total_tokens) FROM token_usage_receipts
		WHERE session_id='legacy-mixed'`).Scan(&receipts); err != nil {
			t.Fatal(err)
		}
		if projection != 100 || receipts != 101 {
			t.Fatalf("legacy projection=%d receipts=%d; want preserved 100 legacy and precise 101 receipts",
				projection, receipts)
		}
	})
}

func TestDailyBudgetBackfillReprojectsMissingModernReceiptsOnce(t *testing.T) {
	t.Run("Should restore missing projections once", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		now := time.Now().UTC().Add(2 * time.Minute)
		db, err := OpenGlobalDB(ctx, filepath.Join(t.TempDir(), "modern-backfill.db"),
			WithClock(func() time.Time { return now }))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Errorf("close backfill DB: %v", err)
			}
		})
		instance, _, err := db.Instances().AdmitCreate(ctx, agentinstance.Instance{
			ProfileID: store.DefaultProfileID, AgentName: "writer", CreatedByKind: "human",
			PermissionCeiling: "{}",
		}, 10)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.DB().ExecContext(ctx, `INSERT INTO sessions
		(id, profile_id, scope, agent_name, workspace_id, instance_id, session_type, state, created_at, updated_at)
		VALUES ('modern-backfill', ?, 'global', 'writer', '', ?, 'user', 'stopped', ?, ?)`,
			store.DefaultProfileID, instance.ID, store.FormatTimestamp(now), store.FormatTimestamp(now)); err != nil {
			t.Fatal(err)
		}
		stats := store.TokenStatsUpdate{
			SessionID: "modern-backfill", TurnID: "modern-missing", AgentName: "writer",
			InputTokens: new(int64(80)), OutputTokens: new(int64(15)),
			CostStatus: "unknown", CostSource: "none", Turns: 1, UpdatedAt: now,
		}
		daily := store.TokenUsageDailyUpdate{
			ProfileID: store.DefaultProfileID, Day: store.LocalDay(now), AgentName: "writer",
			InputTokens: stats.InputTokens, OutputTokens: stats.OutputTokens,
			CostStatus: "unknown", CostSource: "none", Turns: 1, UpdatedAt: now,
		}
		for range 2 {
			if err := db.BackfillDailyBudgetReceipt(ctx, stats, daily, true); err != nil {
				t.Fatal(err)
			}
		}
		var receipts, projectedTurns, dailyTurns int64
		if err := db.DB().QueryRowContext(ctx, `SELECT count(*) FROM token_usage_receipts
		WHERE session_id='modern-backfill'`).Scan(&receipts); err != nil {
			t.Fatal(err)
		}
		if err := db.DB().QueryRowContext(ctx, `SELECT turn_count FROM token_stats
		WHERE session_id='modern-backfill'`).Scan(&projectedTurns); err != nil {
			t.Fatal(err)
		}
		if err := db.DB().QueryRowContext(ctx, `SELECT turn_count FROM token_usage_daily
		WHERE day=? AND agent_name='writer'`, store.LocalDay(now)).Scan(&dailyTurns); err != nil {
			t.Fatal(err)
		}
		if receipts != 1 || projectedTurns != 1 || dailyTurns != 1 {
			t.Fatalf("crash recovery receipts=%d session turns=%d daily turns=%d, want one each",
				receipts, projectedTurns, dailyTurns)
		}
	})
}

func TestDailyBudgetAdoptionDoesNotCaptureEarlierStandaloneUsage(t *testing.T) {
	for _, replay := range []bool{false, true} {
		name := "Should leave earlier standalone Done outside the adopted instance"
		if replay {
			name += " during crash recovery"
		} else {
			name += " during live projection"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.Context(t)
			birth := time.Date(2026, 11, 19, 15, 0, 0, 0, time.UTC)
			db, err := OpenGlobalDB(ctx, filepath.Join(t.TempDir(), "adopt-usage.db"),
				WithClock(func() time.Time { return birth }))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := db.Close(context.Background()); err != nil {
					t.Errorf("close adopted budget DB: %v", err)
				}
			})
			if _, err := db.DB().ExecContext(ctx, `INSERT INTO sessions
			(id, profile_id, scope, agent_name, workspace_id, session_type, state, created_at, updated_at)
			VALUES ('adopt-budget-sess', ?, 'global', 'writer', '', 'user', 'stopped', ?, ?)`,
				store.DefaultProfileID, store.FormatTimestamp(birth.Add(-time.Hour)),
				store.FormatTimestamp(birth.Add(-time.Hour))); err != nil {
				t.Fatal(err)
			}
			instance, err := db.Instances().Adopt(ctx, agentinstance.Instance{
				ProfileID: store.DefaultProfileID, AgentName: "writer", CreatedByKind: "human",
				PermissionCeiling: "{}",
			}, "adopt-budget-sess", 10)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Instances().SetDailyBudgetLimits(ctx, instance.ID,
				agentinstance.DailyBudgetLimits{Tokens: new(int64(100))}); err != nil {
				t.Fatal(err)
			}
			for _, row := range []struct {
				turn   string
				tokens int64
				at     time.Time
			}{
				{turn: "before-adoption", tokens: 85, at: birth.Add(-time.Minute)},
				{turn: "after-adoption", tokens: 10, at: birth.Add(time.Minute)},
			} {
				stats := store.TokenStatsUpdate{SessionID: "adopt-budget-sess", TurnID: row.turn,
					AgentName: "writer", TotalTokens: &row.tokens, CostStatus: "unknown",
					CostSource: "none", Turns: 1, UpdatedAt: row.at}
				daily := store.TokenUsageDailyUpdate{ProfileID: store.DefaultProfileID, Day: store.LocalDay(row.at),
					AgentName: "writer", TotalTokens: &row.tokens, CostStatus: "unknown",
					CostSource: "none", Turns: 1, UpdatedAt: row.at}
				if replay {
					if err := db.BackfillDailyBudgetReceipt(ctx, stats, daily, false); err != nil {
						t.Fatal(err)
					}
				} else if _, err := db.RecordTokenUsageOnce(ctx, stats, daily); err != nil {
					t.Fatal(err)
				}
			}
			var beforeOwner, afterOwner string
			if err := db.DB().QueryRowContext(ctx, `SELECT instance_id FROM token_usage_receipts
				WHERE session_id='adopt-budget-sess' AND turn_id='before-adoption'`).Scan(&beforeOwner); err != nil {
				t.Fatal(err)
			}
			if err := db.DB().QueryRowContext(ctx, `SELECT instance_id FROM token_usage_receipts
				WHERE session_id='adopt-budget-sess' AND turn_id='after-adoption'`).Scan(&afterOwner); err != nil {
				t.Fatal(err)
			}
			usage, err := db.Instances().ReadDailyBudgetUsage(ctx, instance.ID, birth)
			if err != nil || beforeOwner != "" || afterOwner != instance.ID || usage.Tokens != 10 {
				t.Fatalf("adopted usage=%+v err=%v before-owner=%q after-owner=%q; want only 10",
					usage, err, beforeOwner, afterOwner)
			}
		})
	}
}
