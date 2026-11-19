package globaldb

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestRecordTokenUsageReceiptsAreIdempotentAcrossReplays(t *testing.T) {
	t.Run("Should preserve one receipt through replay and deleted sessions", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
		if _, err := db.db.ExecContext(ctx, `INSERT INTO sessions (
		id, profile_id, scope, agent_name, workspace_id, session_type, state, created_at, updated_at
	) VALUES (?, ?, 'global', 'writer', '', 'user', 'stopped', ?, ?)`,
			"sess-receipt", store.DefaultProfileID, store.FormatTimestamp(now), store.FormatTimestamp(now),
		); err != nil {
			t.Fatalf("seed user-scope session: %v", err)
		}
		stats := store.TokenStatsUpdate{
			SessionID: "sess-receipt", TurnID: "turn-once", AgentName: "writer",
			TotalTokens: new(int64(123)), CostStatus: "unknown", CostSource: "none", UpdatedAt: now,
		}
		daily := store.TokenUsageDailyUpdate{
			ProfileID: store.DefaultProfileID, Day: "2026-11-19", AgentName: "writer",
			TotalTokens: new(int64(123)), CostStatus: "unknown", CostSource: "none", UpdatedAt: now,
		}
		for range 3 {
			if _, err := db.RecordTokenUsageOnce(ctx, stats, daily); err != nil {
				t.Fatalf("RecordTokenUsageOnce() error = %v", err)
			}
		}
		got, err := db.ListTokenStats(ctx, store.TokenStatsQuery{SessionID: stats.SessionID, Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].TurnCount != 1 || got[0].TotalTokens == nil || *got[0].TotalTokens != 123 {
			t.Fatalf("duplicate replay changed session aggregate: %+v", got)
		}
		usage, err := db.ListTokenUsageByDay(ctx, store.OverviewDayQuery{
			ReadScope: store.ReadScope{AllProfiles: true}, SinceDay: "2026-11-19",
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(usage) != 1 || usage[0].TotalTokens != 123 {
			t.Fatalf("duplicate replay changed daily rollup: %+v", usage)
		}
		var receipts int
		if err := db.db.QueryRowContext(ctx, `SELECT count(*) FROM token_usage_receipts WHERE session_id = ?`, stats.SessionID).Scan(&receipts); err != nil {
			t.Fatal(err)
		}
		if receipts != 1 {
			t.Fatalf("immutable receipt rows = %d, want 1", receipts)
		}
		// Quiet triage deletes its session. The receipt must still fence replay.
		if _, err := db.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, stats.SessionID); err != nil {
			t.Fatalf("delete completed triage session: %v", err)
		}
		if _, err := db.RecordTokenUsageOnce(ctx, stats, daily); err != nil {
			t.Fatalf("replay after session deletion: %v", err)
		}
		if err := db.db.QueryRowContext(ctx, `SELECT count(*) FROM token_usage_receipts WHERE session_id = ?`, stats.SessionID).Scan(&receipts); err != nil {
			t.Fatal(err)
		}
		if receipts != 1 {
			t.Fatalf("deleted session refunded its immutable receipt: %d", receipts)
		}
		otherProfile := daily
		otherProfile.ProfileID = "different-profile"
		if _, err := db.RecordTokenUsageOnce(ctx, stats, otherProfile); err == nil {
			t.Fatal("replay in another profile was accepted")
		}
	})
}

func TestTokenUsageReceiptSurvivesDatabaseReopen(t *testing.T) {
	t.Run("Should reject a replay after closing and reopening the global database", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		path := t.TempDir() + "/" + GlobalDatabaseName
		copyCurrentSchemaGlobalDBSeed(t, path)
		first, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
		if _, err := first.db.ExecContext(ctx, `INSERT INTO sessions (
			id, profile_id, scope, agent_name, workspace_id, session_type, state, created_at, updated_at
		) VALUES ('sess-reopen', ?, 'global', 'writer', '', 'user', 'stopped', ?, ?)`,
			store.DefaultProfileID, store.FormatTimestamp(now), store.FormatTimestamp(now),
		); err != nil {
			t.Fatal(err)
		}
		stats := store.TokenStatsUpdate{
			SessionID: "sess-reopen", TurnID: "turn-reopen", AgentName: "writer",
			TotalTokens: new(int64(51)), CostStatus: "unknown", CostSource: "none", UpdatedAt: now,
		}
		daily := store.TokenUsageDailyUpdate{
			ProfileID: store.DefaultProfileID, Day: "2026-11-19", AgentName: "writer",
			TotalTokens: stats.TotalTokens, CostStatus: "unknown", CostSource: "none", UpdatedAt: now,
		}
		accepted, err := first.RecordTokenUsageOnce(ctx, stats, daily)
		if err != nil || !accepted {
			t.Fatalf("first acceptance=%v error=%v", accepted, err)
		}
		if err := first.Close(ctx); err != nil {
			t.Fatal(err)
		}
		reopened, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(context.Background()); err != nil {
				t.Errorf("close reopened db: %v", err)
			}
		})
		accepted, err = reopened.RecordTokenUsageOnce(ctx, stats, daily)
		if err != nil || accepted {
			t.Fatalf("replay after reopen acceptance=%v error=%v", accepted, err)
		}
		var count int
		if err := reopened.db.QueryRowContext(ctx,
			`SELECT count(*) FROM token_usage_receipts WHERE session_id = ?`, stats.SessionID,
		).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("persisted receipt rows=%d, want 1", count)
		}
	})
}

func TestTokenUsageReceiptCountsInputAndOutputWithoutReportedTotal(t *testing.T) {
	t.Run("Should retain summed input and output in an unpriced receipt", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
		if _, err := db.db.ExecContext(ctx, `INSERT INTO sessions (
		id, profile_id, scope, agent_name, workspace_id, session_type, state, created_at, updated_at
	) VALUES ('sess-buckets', ?, 'global', 'writer', '', 'user', 'stopped', ?, ?)`,
			store.DefaultProfileID, store.FormatTimestamp(now), store.FormatTimestamp(now),
		); err != nil {
			t.Fatal(err)
		}
		stats := store.TokenStatsUpdate{
			SessionID: "sess-buckets", TurnID: "turn-buckets", AgentName: "writer",
			InputTokens: new(int64(1200)), OutputTokens: new(int64(340)),
			CostStatus: "unknown", CostSource: "none", UpdatedAt: now,
		}
		daily := store.TokenUsageDailyUpdate{
			ProfileID: store.DefaultProfileID, Day: "2026-11-19", AgentName: "writer",
			InputTokens: stats.InputTokens, OutputTokens: stats.OutputTokens,
			CostStatus: "unknown", CostSource: "none", UpdatedAt: now,
		}
		accepted, err := db.RecordTokenUsageOnce(ctx, stats, daily)
		if err != nil || !accepted {
			t.Fatalf("first receipt accepted=%v error=%v", accepted, err)
		}
		accepted, err = db.RecordTokenUsageOnce(ctx, stats, daily)
		if err != nil || accepted {
			t.Fatalf("replayed receipt accepted=%v error=%v", accepted, err)
		}
		var tokens int64
		if err := db.db.QueryRowContext(ctx,
			`SELECT total_tokens FROM token_usage_receipts WHERE session_id = ? AND turn_id = ?`,
			stats.SessionID, stats.TurnID,
		).Scan(&tokens); err != nil {
			t.Fatal(err)
		}
		if tokens != 1540 {
			t.Fatalf("receipt tokens=%d, want input+output=1540", tokens)
		}
	})
}

func TestTokenUsageReceiptRollbackAndRetry(t *testing.T) {
	t.Run("Should atomically roll back receipt and aggregates on a rollup failure", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
		if _, err := db.db.ExecContext(ctx, `INSERT INTO sessions (
			id, profile_id, scope, agent_name, workspace_id, session_type, state, created_at, updated_at
		) VALUES ('sess-rollback', ?, 'global', 'writer', '', 'user', 'stopped', ?, ?)`,
			store.DefaultProfileID, store.FormatTimestamp(now), store.FormatTimestamp(now),
		); err != nil {
			t.Fatal(err)
		}
		stats := store.TokenStatsUpdate{
			SessionID: "sess-rollback", TurnID: "turn-rollback", AgentName: "writer",
			TotalTokens: new(int64(77)), CostStatus: "unknown", CostSource: "none", UpdatedAt: now,
		}
		daily := store.TokenUsageDailyUpdate{
			ProfileID: store.DefaultProfileID, Day: "2026-11-19", AgentName: "writer",
			TotalTokens: stats.TotalTokens, CostStatus: "unknown", CostSource: "none", UpdatedAt: now,
		}
		if _, err := db.db.ExecContext(ctx, `CREATE TRIGGER reject_receipt_rollup
			BEFORE INSERT ON token_usage_daily
			BEGIN SELECT RAISE(ABORT, 'forced rollup failure'); END;`); err != nil {
			t.Fatal(err)
		}
		accepted, err := db.RecordTokenUsageOnce(ctx, stats, daily)
		if err == nil || accepted {
			t.Fatalf("forced failure accepted=%v error=%v", accepted, err)
		}
		for _, table := range []string{"token_usage_receipts", "token_stats", "token_usage_daily"} {
			var count int
			if err := db.db.QueryRowContext(ctx, `SELECT count(*) FROM `+table).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatalf("table %s retained %d rows after rollback", table, count)
			}
		}
		if _, err := db.db.ExecContext(ctx, `DROP TRIGGER reject_receipt_rollup`); err != nil {
			t.Fatal(err)
		}
		accepted, err = db.RecordTokenUsageOnce(ctx, stats, daily)
		if err != nil || !accepted {
			t.Fatalf("retry accepted=%v error=%v", accepted, err)
		}
	})
}

func TestTokenUsageReceiptProfileDeletion(t *testing.T) {
	t.Run("Should delete retained receipt when its profile is deleted", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
		const profileID = "00000000000000000000000001"
		if _, err := db.db.ExecContext(ctx,
			`INSERT INTO profiles (id,name,color,icon,state,created_at)
			 VALUES (?, 'receipt-owner', '#E8572A', 'circle', 'active', ?)`,
			profileID, store.FormatTimestamp(now),
		); err != nil {
			t.Fatal(err)
		}
		if _, err := db.db.ExecContext(ctx, `INSERT INTO sessions (
			id, profile_id, scope, agent_name, workspace_id, session_type, state, created_at, updated_at
		) VALUES ('sess-private', ?, 'global', 'writer', '', 'user', 'stopped', ?, ?)`,
			profileID, store.FormatTimestamp(now), store.FormatTimestamp(now),
		); err != nil {
			t.Fatal(err)
		}
		accepted, err := db.RecordTokenUsageOnce(ctx,
			store.TokenStatsUpdate{
				SessionID: "sess-private", TurnID: "turn-private", AgentName: "writer",
				TotalTokens: new(int64(42)), CostStatus: "unknown", CostSource: "none", UpdatedAt: now,
			},
			store.TokenUsageDailyUpdate{
				ProfileID: profileID, Day: "2026-11-19", AgentName: "writer",
				TotalTokens: new(int64(42)), CostStatus: "unknown", CostSource: "none", UpdatedAt: now,
			},
		)
		if err != nil || !accepted {
			t.Fatalf("insert receipt accepted=%v error=%v", accepted, err)
		}
		if _, err := db.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = 'sess-private'`); err != nil {
			t.Fatal(err)
		}
		// Existing daily rollups independently constrain profile deletion. Simulate
		// their normal retention cleanup before asserting receipt-owned cascade.
		if _, err := db.db.ExecContext(ctx, `DELETE FROM token_usage_daily WHERE profile_id = ?`, profileID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.db.ExecContext(ctx, `DELETE FROM profiles WHERE id = ?`, profileID); err != nil {
			t.Fatal(err)
		}
		var count int
		if err := db.db.QueryRowContext(ctx,
			`SELECT count(*) FROM token_usage_receipts WHERE profile_id = ?`, profileID,
		).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("profile deletion retained %d private receipts", count)
		}
	})
}

func TestTokenUsageReceiptCapturesInstanceAndNormalizedUSD(t *testing.T) {
	t.Run("Should freeze owning instance and normalized dollar amount", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
		if _, err := db.db.ExecContext(ctx, `INSERT INTO agent_instances (
			id, profile_id, agent_name, workspace_id, state, created_by_kind, created_by_id, created_at, updated_at
		) VALUES ('instance-receipt', ?, 'writer', '', 'active', 'human', 'operator', ?, ?)`,
			store.DefaultProfileID, store.FormatTimestamp(now), store.FormatTimestamp(now),
		); err != nil {
			t.Fatal(err)
		}
		if _, err := db.db.ExecContext(ctx, `INSERT INTO sessions (
			id, profile_id, scope, agent_name, workspace_id, session_type, state, instance_id, created_at, updated_at
		) VALUES ('sess-instance', ?, 'global', 'writer', '', 'user', 'stopped', 'instance-receipt', ?, ?)`,
			store.DefaultProfileID, store.FormatTimestamp(now), store.FormatTimestamp(now),
		); err != nil {
			t.Fatal(err)
		}
		cost := 1.25
		currency := " USD "
		stats := store.TokenStatsUpdate{
			SessionID: "sess-instance", TurnID: "turn-instance", AgentName: "writer",
			TotalTokens: new(int64(100)), CostAmount: &cost, CostCurrency: &currency,
			CostStatus: "actual", CostSource: "agent_reported", UpdatedAt: now,
		}
		daily := store.TokenUsageDailyUpdate{
			ProfileID: store.DefaultProfileID, Day: "2026-11-19", AgentName: "writer",
			TotalTokens: stats.TotalTokens, CostAmount: &cost, CostCurrency: &currency,
			CostStatus: "actual", CostSource: "agent_reported", UpdatedAt: now,
		}
		accepted, err := db.RecordTokenUsageOnce(ctx, stats, daily)
		if err != nil || !accepted {
			t.Fatalf("accept instance receipt=%v error=%v", accepted, err)
		}
		var instanceID, usd string
		if err := db.db.QueryRowContext(ctx,
			`SELECT instance_id, usd_amount FROM token_usage_receipts WHERE session_id = ?`, stats.SessionID,
		).Scan(&instanceID, &usd); err != nil {
			t.Fatal(err)
		}
		if instanceID != "instance-receipt" || usd != "1.25" {
			t.Fatalf("receipt instance=%q usd=%q, want instance-receipt and 1.25", instanceID, usd)
		}
	})
}

func TestTokenUsageReceiptRejectsIncoherentTurnUpdates(t *testing.T) {
	stats := store.TokenStatsUpdate{
		SessionID: "sess", TurnID: "turn", AgentName: "writer", TotalTokens: new(int64(42)),
		CostStatus: "unknown", CostSource: "none",
	}
	daily := store.TokenUsageDailyUpdate{
		ProfileID: store.DefaultProfileID, Day: "2026-11-19", AgentName: "writer",
		TotalTokens: new(int64(42)), CostStatus: "unknown", CostSource: "none",
	}
	for _, tc := range []struct {
		name  string
		stats store.TokenStatsUpdate
		daily store.TokenUsageDailyUpdate
	}{
		{name: "Should reject two session turns", stats: func() store.TokenStatsUpdate { s := stats; s.Turns = 2; return s }(), daily: daily},
		{name: "Should reject two daily turns", stats: stats, daily: func() store.TokenUsageDailyUpdate { d := daily; d.Turns = 2; return d }()},
		{name: "Should reject mismatched totals", stats: stats, daily: func() store.TokenUsageDailyUpdate { d := daily; d.TotalTokens = new(int64(43)); return d }()},
		{name: "Should reject mismatched agent", stats: stats, daily: func() store.TokenUsageDailyUpdate { d := daily; d.AgentName = "other"; return d }()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := validateTokenReceiptPair(tc.stats, tc.daily); err == nil {
				t.Fatal("incoherent canonical usage was accepted")
			}
		})
	}
}
