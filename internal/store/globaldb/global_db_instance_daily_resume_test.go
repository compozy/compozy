package globaldb

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestInstanceDailyBudgetResumeEligibility(t *testing.T) {
	t.Run("Should require operator action and a new eligible day before resuming a budget pause", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		current := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
		db.now = func() time.Time { return current }
		stamp := store.FormatTimestamp(current)
		if _, err := db.db.ExecContext(ctx, `INSERT INTO agent_instances (
			id, profile_id, agent_name, workspace_id, state, pause_reason,
			created_by_kind, created_by_id, created_at, updated_at
		) VALUES ('budget-resume', ?, 'writer', '', 'paused', 'daily_budget', 'human', 'operator', ?, ?)`,
			store.DefaultProfileID, stamp, stamp,
		); err != nil {
			t.Fatal(err)
		}
		if err := db.Instances().SetDailyBudgetLimits(ctx, "budget-resume", agentinstance.DailyBudgetLimits{
			Tokens: new(int64(100)),
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.db.ExecContext(ctx, `INSERT INTO token_usage_receipts (
			profile_id, session_id, turn_id, instance_id, day, total_tokens, cost_status, created_at
		) VALUES (?, 'sess-budget-resume', 'turn-budget-resume', 'budget-resume', ?, 100, 'unknown', ?)`,
			store.DefaultProfileID, store.LocalDay(current), stamp,
		); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Instances().Resume(ctx, "budget-resume", 10, false); !errors.Is(err, agentinstance.ErrForbidden) {
			t.Fatalf("nonoperator resume error = %v, want forbidden", err)
		}
		if _, err := db.Instances().Resume(ctx, "budget-resume", 10, true); !errors.Is(err, agentinstance.ErrPaused) {
			t.Fatalf("exhausted budget resume error = %v, want paused", err)
		}
		paused, err := db.Instances().Get(ctx, "budget-resume")
		if err != nil || paused.State != "paused" || paused.PauseReason != "daily_budget" {
			t.Fatalf("persisted pause = %+v, error = %v", paused, err)
		}
		current = current.AddDate(0, 0, 1)
		resumed, err := db.Instances().Resume(ctx, "budget-resume", 10, true)
		if err != nil || resumed.State != "active" || resumed.PauseReason != "" {
			t.Fatalf("eligible next-day resume = %+v, error = %v", resumed, err)
		}
	})

	t.Run("Should allow operator resume after both caps are explicitly removed", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		current := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
		db.now = func() time.Time { return current }
		stamp := store.FormatTimestamp(current)
		if _, err := db.db.ExecContext(ctx, `INSERT INTO agent_instances (
			id, profile_id, agent_name, workspace_id, state, pause_reason,
			created_by_kind, created_by_id, created_at, updated_at
		) VALUES ('budget-cleared', ?, 'writer', '', 'paused', 'daily_budget', 'human', 'operator', ?, ?)`,
			store.DefaultProfileID, stamp, stamp,
		); err != nil {
			t.Fatal(err)
		}
		if err := db.Instances().SetDailyBudgetLimits(ctx, "budget-cleared", agentinstance.DailyBudgetLimits{
			Tokens: new(int64(1)),
		}); err != nil {
			t.Fatal(err)
		}
		if err := db.Instances().SetDailyBudgetLimits(ctx, "budget-cleared", agentinstance.DailyBudgetLimits{}); err != nil {
			t.Fatal(err)
		}
		for _, receipt := range []struct {
			turnID string
			tokens int64
		}{
			{turnID: "turn-cleared-1", tokens: math.MaxInt64},
			{turnID: "turn-cleared-2", tokens: 1},
		} {
			if _, err := db.db.ExecContext(ctx, `INSERT INTO token_usage_receipts (
				profile_id, session_id, turn_id, instance_id, day, total_tokens, cost_status, created_at
			) VALUES (?, ?, ?, 'budget-cleared', ?, ?, 'unknown', ?)`,
				store.DefaultProfileID, "sess-"+receipt.turnID, receipt.turnID,
				store.LocalDay(current), receipt.tokens, stamp,
			); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := db.Instances().Resume(ctx, "budget-cleared", 10, false); !errors.Is(err, agentinstance.ErrForbidden) {
			t.Fatalf("nonoperator resume after removing caps error = %v, want forbidden", err)
		}
		resumed, err := db.Instances().Resume(ctx, "budget-cleared", 10, true)
		if err != nil || resumed.State != "active" {
			t.Fatalf("cleared-cap resume = %+v, error = %v", resumed, err)
		}
	})
}
