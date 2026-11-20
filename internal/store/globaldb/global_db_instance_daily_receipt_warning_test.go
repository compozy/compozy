package globaldb

import (
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func setupDailyBudgetBellFixture(t *testing.T, db *GlobalDB, now time.Time) {
	t.Helper()
	ctx := t.Context()
	stamp := store.FormatTimestamp(now)
	if _, err := db.db.ExecContext(ctx, `INSERT INTO agent_instances (
		id, profile_id, agent_name, workspace_id, state, created_by_kind, created_by_id, created_at, updated_at
	) VALUES ('receipt-bell', ?, 'writer', '', 'active', 'human', 'operator', ?, ?)`,
		store.DefaultProfileID, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.ExecContext(ctx, `INSERT INTO sessions (
		id, profile_id, scope, agent_name, workspace_id, session_type, state,
		instance_id, created_at, updated_at
	) VALUES ('receipt-bell-main', ?, 'global', 'writer', '', 'user', 'running',
		'receipt-bell', ?, ?)`, store.DefaultProfileID, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.ExecContext(ctx,
		`UPDATE agent_instances SET main_session_id = 'receipt-bell-main' WHERE id = 'receipt-bell'`); err != nil {
		t.Fatal(err)
	}
	if err := db.Instances().SetDailyBudgetLimits(ctx, "receipt-bell", agentinstance.DailyBudgetLimits{
		Tokens: new(int64(100)),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCanonicalReceiptRaisesDailyBudgetBellAtThreshold(t *testing.T) {
	t.Run("Should warn once and keep counting after archive", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
		setupDailyBudgetBellFixture(t, db, now)
		for i, count := range []int64{79, 1, 15, 1} {
			timeAt := now.Add(time.Duration(i) * time.Minute)
			stats := store.TokenStatsUpdate{
				SessionID: "receipt-bell-main", TurnID: "bell-" + string(rune('a'+i)),
				AgentName: "writer", TotalTokens: &count,
				CostStatus: "unknown", CostSource: "none", Turns: 1, UpdatedAt: timeAt,
			}
			daily := store.TokenUsageDailyUpdate{
				ProfileID: store.DefaultProfileID, Day: store.LocalDay(timeAt),
				AgentName: "writer", TotalTokens: &count,
				CostStatus: "unknown", CostSource: "none", Turns: 1, UpdatedAt: timeAt,
			}
			accepted, err := db.RecordTokenUsageOnce(ctx, stats, daily)
			if err != nil || !accepted {
				t.Fatalf("receipt %d accepted=%v error=%v", i, accepted, err)
			}
			accepted, err = db.RecordTokenUsageOnce(ctx, stats, daily)
			if err != nil || accepted {
				t.Fatalf("replay %d accepted=%v error=%v", i, accepted, err)
			}
			var notices int
			if err := db.db.QueryRowContext(ctx, `SELECT count(*) FROM operator_notice_outbox
				WHERE profile_id = ? AND resource = 'agent_instance'`, store.DefaultProfileID).Scan(&notices); err != nil {
				t.Fatal(err)
			}
			want := 0
			if i >= 1 {
				want = 1
			}
			if notices != want {
				t.Fatalf("after receipt %d, bell notices=%d, want %d", i, notices, want)
			}
		}
		var total int64
		if err := db.db.QueryRowContext(ctx, `SELECT total_tokens FROM token_stats
			WHERE session_id = 'receipt-bell-main'`).Scan(&total); err != nil {
			t.Fatal(err)
		}
		if total != 96 {
			t.Fatalf("replayed token sum = %d, want 96", total)
		}
		if _, err := db.db.ExecContext(
			ctx,
			`UPDATE agent_instances SET archived_at = ? WHERE id = 'receipt-bell'`,
			store.FormatTimestamp(now),
		); err != nil {
			t.Fatal(err)
		}
		lateAt := now.AddDate(0, 0, 1)
		late := int64(90)
		stats := store.TokenStatsUpdate{
			SessionID: "receipt-bell-main", TurnID: "bell-after-archive", AgentName: "writer",
			TotalTokens: &late, CostStatus: "unknown", CostSource: "none", Turns: 1, UpdatedAt: lateAt,
		}
		daily := store.TokenUsageDailyUpdate{
			ProfileID: store.DefaultProfileID, Day: store.LocalDay(lateAt), AgentName: "writer",
			TotalTokens: &late, CostStatus: "unknown", CostSource: "none", Turns: 1, UpdatedAt: lateAt,
		}
		if accepted, err := db.RecordTokenUsageOnce(ctx, stats, daily); err != nil || !accepted {
			t.Fatalf("in-flight archived receipt accepted=%v error=%v", accepted, err)
		}
		zeroTimeReplay := daily
		zeroTimeReplay.UpdatedAt = time.Time{}
		if accepted, err := db.RecordTokenUsageOnce(ctx, stats, zeroTimeReplay); err != nil || accepted {
			t.Fatalf("old committed receipt replay with changed clock accepted=%v error=%v", accepted, err)
		}
		newTurn := stats
		newTurn.TurnID = "bell-invalid-timestamp"
		if accepted, err := db.RecordTokenUsageOnce(ctx, newTurn, zeroTimeReplay); err == nil ||
			!strings.Contains(err.Error(), "canonical token receipt day does not match its timestamp") || accepted {
			t.Fatalf("new receipt with missing event timestamp accepted=%v error=%v", accepted, err)
		}
		var notices int
		if err := db.db.QueryRowContext(
			ctx,
			`SELECT count(*) FROM operator_notice_outbox WHERE resource = 'agent_instance'`,
		).Scan(&notices); err != nil {
			t.Fatal(err)
		}
		if notices != 1 {
			t.Fatalf("late archived receipt created %d notices, want 1", notices)
		}
	})
}

func TestDailyBudgetReceiptSurvivesWarningWriteFailure(t *testing.T) {
	for _, tc := range []struct {
		name     string
		backfill bool
	}{
		{name: "Should retain live usage when the notice outbox rejects a warning"},
		{name: "Should retain recovered usage when the notice outbox rejects a warning", backfill: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			db := openTestGlobalDB(t)
			at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
			db.now = func() time.Time { return at }
			setupDailyBudgetBellFixture(t, db, at)
			if _, err := db.db.ExecContext(ctx, `CREATE TRIGGER reject_budget_notice
				BEFORE INSERT ON operator_notice_outbox
				BEGIN SELECT RAISE(ABORT, 'fixture outbox unavailable'); END`); err != nil {
				t.Fatal(err)
			}
			stats := store.TokenStatsUpdate{
				SessionID: "receipt-bell-main", TurnID: "threshold", AgentName: "writer",
				TotalTokens: new(int64(85)), CostStatus: "unknown", CostSource: "none", Turns: 1, UpdatedAt: at,
			}
			daily := store.TokenUsageDailyUpdate{
				ProfileID: store.DefaultProfileID, Day: store.LocalDay(at), AgentName: "writer",
				TotalTokens: stats.TotalTokens, CostStatus: "unknown", CostSource: "none", Turns: 1, UpdatedAt: at,
			}
			if tc.backfill {
				if err := db.BackfillDailyBudgetReceipt(ctx, stats, daily, true); err != nil {
					t.Fatal(err)
				}
			} else if accepted, err := db.RecordTokenUsageOnce(ctx, stats, daily); err != nil || !accepted {
				t.Fatalf("receipt=%t err=%v; want accepted usage", accepted, err)
			}
			var sessionTokens, dailyTokens, reservations int64
			if err := db.db.QueryRowContext(ctx, `SELECT
				(SELECT total_tokens FROM token_stats WHERE session_id='receipt-bell-main'),
				(SELECT total_tokens FROM token_usage_daily WHERE profile_id=? AND day=?),
				(SELECT count(*) FROM agent_instance_daily_budget_notices WHERE instance_id='receipt-bell')`,
				store.DefaultProfileID, daily.Day).Scan(&sessionTokens, &dailyTokens, &reservations); err != nil ||
				sessionTokens != 85 || dailyTokens != 85 || reservations != 0 {
				t.Fatalf("session=%d daily=%d reservations=%d err=%v", sessionTokens, dailyTokens, reservations, err)
			}
			if _, err := db.db.ExecContext(ctx, "DROP TRIGGER reject_budget_notice"); err != nil {
				t.Fatal(err)
			}
			stats.TurnID = "retry-warning-on-new-turn"
			stats.TotalTokens = new(int64(1))
			daily.TotalTokens = stats.TotalTokens
			if accepted, err := db.RecordTokenUsageOnce(ctx, stats, daily); err != nil || !accepted {
				t.Fatalf("additional receipt=%t err=%v", accepted, err)
			}
			var notices int
			if err := db.db.QueryRowContext(ctx, `SELECT count(*) FROM operator_notice_outbox
				WHERE resource='agent_instance'`).Scan(&notices); err != nil || notices != 1 {
				t.Fatalf("recovered warning count=%d err=%v; want one", notices, err)
			}
		})
	}
}
