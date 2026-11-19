package globaldb

import (
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestCanonicalReceiptRaisesDailyBudgetBellAtThreshold(t *testing.T) {
	db := openTestGlobalDB(t)
	ctx := testutil.Context(t)
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
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
	if _, err := db.db.ExecContext(ctx,
		`UPDATE agent_instances SET archived_at = ? WHERE id = 'receipt-bell'`, stamp); err != nil {
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
	if accepted, err := db.RecordTokenUsageOnce(ctx, newTurn, zeroTimeReplay); err == nil || accepted {
		t.Fatalf("new receipt with missing event timestamp accepted=%v error=%v", accepted, err)
	}
	var notices int
	if err := db.db.QueryRowContext(ctx,
		`SELECT count(*) FROM operator_notice_outbox WHERE resource = 'agent_instance'`).Scan(&notices); err != nil {
		t.Fatal(err)
	}
	if notices != 1 {
		t.Fatalf("late archived receipt created %d notices, want 1", notices)
	}
}
