package globaldb

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/telemetrydb"
	"github.com/compozy/compozy/internal/testutil"
)

func TestInstanceDailyBudgetUsageFromReceipts(t *testing.T) {
	t.Run("Should sum only the instance and local day with exact USD decimals", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
		stamp := store.FormatTimestamp(at)
		for _, instance := range []struct{ id, agent string }{
			{id: "budget-own", agent: "writer"},
			{id: "budget-other", agent: "reviewer"},
		} {
			if _, err := db.db.ExecContext(ctx, `INSERT INTO agent_instances (
				id, profile_id, agent_name, workspace_id, state, created_by_kind, created_by_id, created_at, updated_at
			) VALUES (?, ?, ?, '', 'active', 'human', 'operator', ?, ?)`,
				instance.id, store.DefaultProfileID, instance.agent, stamp, stamp,
			); err != nil {
				t.Fatal(err)
			}
		}
		for _, receipt := range []struct {
			turnID, instanceID, day string
			tokens                  int64
			usd                     any
		}{
			{turnID: "turn-a", instanceID: "budget-own", day: store.LocalDay(at), tokens: 20, usd: "0.10"},
			{turnID: "turn-b", instanceID: "budget-own", day: store.LocalDay(at), tokens: 15, usd: "0.20"},
			{turnID: "turn-c", instanceID: "budget-own", day: store.LocalDay(at), tokens: 10},
			{turnID: "turn-other", instanceID: "budget-other", day: store.LocalDay(at), tokens: 900, usd: "5.00"},
			{
				turnID: "turn-tomorrow", instanceID: "budget-own",
				day: store.LocalDay(at.AddDate(0, 0, 1)), tokens: 700, usd: "9.00",
			},
		} {
			if _, err := db.db.ExecContext(ctx, `INSERT INTO token_usage_receipts (
				profile_id, session_id, turn_id, instance_id, day, total_tokens, usd_amount, cost_status, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, 'estimated', ?)`,
				store.DefaultProfileID, "session-"+receipt.turnID, receipt.turnID,
				receipt.instanceID, receipt.day, receipt.tokens, receipt.usd, stamp,
			); err != nil {
				t.Fatal(err)
			}
		}
		const secondProfile = "00000000000000000000000001"
		if _, err := db.db.ExecContext(ctx, `INSERT INTO profiles (id, name, color, icon, state, created_at)
			VALUES (?, 'different-budget-owner', '#E8572A', 'circle', 'active', ?)`,
			secondProfile, stamp,
		); err != nil {
			t.Fatal(err)
		}
		// A corrupt/misbound receipt from a different profile must not be billed
		// to this owner, even when its instance_id matches by accident.
		if _, err := db.db.ExecContext(ctx, `INSERT INTO token_usage_receipts (
			profile_id, session_id, turn_id, instance_id, day, total_tokens, usd_amount, cost_status, created_at
		) VALUES (?, 'sess-other-profile', 'turn-other-profile', 'budget-own', ?, 900, '9.00', 'estimated', ?)`,
			secondProfile, store.LocalDay(at), stamp,
		); err != nil {
			t.Fatal(err)
		}
		usage, err := db.Instances().ReadDailyBudgetUsage(ctx, "budget-own", at)
		if err != nil {
			t.Fatal(err)
		}
		if usage.Tokens != 45 || !usage.HasUnpricedUsage || usage.USD == nil ||
			usage.USD.Cmp(big.NewRat(3, 10)) != 0 {
			t.Fatalf("instance daily usage = %+v, want 45 tokens, $0.30 and unpriced flag", usage)
		}
	})

	t.Run("Should distinguish included subscription usage from an unknown price", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
		stamp := store.FormatTimestamp(at)
		if _, err := db.db.ExecContext(ctx, `INSERT INTO agent_instances (
			id, profile_id, agent_name, workspace_id, state, created_by_kind, created_by_id, created_at, updated_at
		) VALUES ('budget-included', ?, 'writer', '', 'active', 'human', 'operator', ?, ?)`,
			store.DefaultProfileID, stamp, stamp); err != nil {
			t.Fatal(err)
		}
		if _, err := db.db.ExecContext(ctx, `INSERT INTO token_usage_receipts (
			profile_id, session_id, turn_id, instance_id, day, total_tokens, cost_status, created_at
		) VALUES (?, 'included-session', 'included-turn', 'budget-included', ?, 25, 'included', ?)`,
			store.DefaultProfileID, store.LocalDay(at), stamp); err != nil {
			t.Fatal(err)
		}
		usage, err := db.Instances().ReadDailyBudgetUsage(ctx, "budget-included", at)
		if err != nil || usage.Tokens != 25 || usage.HasUnpricedUsage {
			t.Fatalf("subscription usage=%+v err=%v, want 25 tokens and a known included cost", usage, err)
		}
	})

	t.Run("Should read every page without dropping or repeating a receipt", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
		stamp := store.FormatTimestamp(at)
		if _, err := db.db.ExecContext(ctx, `INSERT INTO agent_instances (
			id, profile_id, agent_name, workspace_id, state, created_by_kind, created_by_id, created_at, updated_at
		) VALUES ('budget-many', ?, 'writer', '', 'active', 'human', 'operator', ?, ?)`,
			store.DefaultProfileID, stamp, stamp,
		); err != nil {
			t.Fatal(err)
		}
		for index := range 258 {
			id := fmt.Sprintf("turn-%04d", index)
			if _, err := db.db.ExecContext(ctx, `INSERT INTO token_usage_receipts (
				profile_id, session_id, turn_id, instance_id, day, total_tokens, usd_amount, cost_status, created_at
			) VALUES (?, ?, ?, 'budget-many', ?, 1, '0.01', 'estimated', ?)`,
				store.DefaultProfileID, "sess-"+id, id, store.LocalDay(at), stamp,
			); err != nil {
				t.Fatal(err)
			}
		}
		usage, err := db.Instances().ReadDailyBudgetUsage(ctx, "budget-many", at)
		if err != nil || usage.Tokens != 258 || usage.HasUnpricedUsage || usage.USD == nil ||
			usage.USD.Cmp(big.NewRat(258, 100)) != 0 {
			t.Fatalf("paged receipts usage=%+v error=%v, want 258 tokens and $2.58", usage, err)
		}
	})

	t.Run("Should reject integer overflow instead of wrapping the daily token total", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
		stamp := store.FormatTimestamp(at)
		if _, err := db.db.ExecContext(ctx, `INSERT INTO agent_instances (
			id, profile_id, agent_name, workspace_id, state, created_by_kind, created_by_id, created_at, updated_at
		) VALUES ('budget-overflow', ?, 'writer', '', 'active', 'human', 'operator', ?, ?)`,
			store.DefaultProfileID, stamp, stamp,
		); err != nil {
			t.Fatal(err)
		}
		for _, receipt := range []struct {
			id     string
			tokens int64
		}{
			{id: "turn-overflow-1", tokens: math.MaxInt64},
			{id: "turn-overflow-2", tokens: 1},
		} {
			if _, err := db.db.ExecContext(ctx, `INSERT INTO token_usage_receipts (
				profile_id, session_id, turn_id, instance_id, day, total_tokens, cost_status, created_at
			) VALUES (?, ?, ?, 'budget-overflow', ?, ?, 'unknown', ?)`,
				store.DefaultProfileID, "sess-"+receipt.id, receipt.id,
				store.LocalDay(at), receipt.tokens, stamp,
			); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := db.Instances().ReadDailyBudgetUsage(ctx, "budget-overflow", at); err == nil {
			t.Fatal("ReadDailyBudgetUsage() accepted overflowing token totals")
		}
	})

	t.Run("Should reject a corrupted USD receipt rather than admit with an understated total", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
		stamp := store.FormatTimestamp(at)
		if _, err := db.db.ExecContext(ctx, `INSERT INTO agent_instances (
			id, profile_id, agent_name, workspace_id, state, created_by_kind, created_by_id, created_at, updated_at
		) VALUES ('budget-corrupt', ?, 'writer', '', 'active', 'human', 'operator', ?, ?)`,
			store.DefaultProfileID, stamp, stamp,
		); err != nil {
			t.Fatal(err)
		}
		if _, err := db.db.ExecContext(ctx, `INSERT INTO token_usage_receipts (
			profile_id, session_id, turn_id, instance_id, day, total_tokens, usd_amount, cost_status, created_at
		) VALUES (?, 'sess-invalid', 'turn-invalid', 'budget-corrupt', ?, 1, 'not-money', 'estimated', ?)`,
			store.DefaultProfileID, store.LocalDay(at), stamp,
		); err != nil {
			t.Fatal(err)
		}
		for _, amount := range []string{"not-money", "1/2", "-0.01"} {
			if _, err := db.db.ExecContext(ctx,
				`UPDATE token_usage_receipts SET usd_amount = ? WHERE turn_id = 'turn-invalid'`, amount,
			); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Instances().ReadDailyBudgetUsage(ctx, "budget-corrupt", at); err == nil {
				t.Fatalf("ReadDailyBudgetUsage() accepted corrupted USD receipt %q", amount)
			}
		}
		// The canonical writer can persist IEEE negative zero as the decimal "-0".
		negativeZero := strconv.FormatFloat(math.Copysign(0, -1), 'f', -1, 64)
		if _, err := db.db.ExecContext(ctx,
			`UPDATE token_usage_receipts SET usd_amount = ? WHERE turn_id = 'turn-invalid'`, negativeZero,
		); err != nil {
			t.Fatal(err)
		}
		usage, err := db.Instances().ReadDailyBudgetUsage(ctx, "budget-corrupt", at)
		if err != nil || usage.USD == nil || usage.USD.Sign() != 0 || usage.HasUnpricedUsage {
			t.Fatalf("canonical negative-zero USD usage=%+v error=%v", usage, err)
		}
	})
}

func TestDailyBudgetProjectionUpgradeSeedsOlderReceiptsExactlyOnce(t *testing.T) {
	t.Run("Should seed prior receipts and preserve the exact projection across reopen", func(t *testing.T) {
		ctx := globalMigrationTestContext(t)
		at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
		stamp := store.FormatTimestamp(at)
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefix, err := openGlobalMigrationPrefixDatabase(t, path, globalMigrationPrefixBefore(t, "00215_schema.sql"))
		if err != nil {
			t.Fatal(err)
		}
		prefixClosed := false
		t.Cleanup(func() {
			if !prefixClosed {
				if err := prefix.Close(); err != nil {
					t.Errorf("close migration prefix: %v", err)
				}
			}
		})
		if _, err := prefix.ExecContext(ctx, `INSERT INTO agent_instances (
			id, profile_id, agent_name, workspace_id, state, created_by_kind, created_by_id, created_at, updated_at
		) VALUES ('budget-projection-upgrade', ?, 'writer', '', 'active', 'human', 'operator', ?, ?)`,
			store.DefaultProfileID, stamp, stamp); err != nil {
			t.Fatal(err)
		}
		if _, err := prefix.ExecContext(ctx, `INSERT INTO sessions (
			id, profile_id, scope, agent_name, workspace_id, session_type, state, instance_id, created_at, updated_at
		) VALUES ('budget-upgrade-session', ?, 'global', 'writer', '', 'user', 'running',
			'budget-projection-upgrade', ?, ?)`, store.DefaultProfileID, stamp, stamp); err != nil {
			t.Fatal(err)
		}
		if _, err := prefix.ExecContext(ctx, `INSERT INTO token_usage_receipts (
			profile_id, session_id, turn_id, instance_id, day, total_tokens, usd_amount, cost_status, created_at
		) VALUES (?, 'budget-upgrade-session', 'old-turn', 'budget-projection-upgrade', ?, 10, '0.1', 'estimated', ?)`,
			store.DefaultProfileID, store.LocalDay(at), stamp); err != nil {
			t.Fatal(err)
		}
		telemetry, err := telemetrydb.Open(ctx, telemetrydb.Path(path))
		if err != nil {
			t.Fatal(err)
		}
		bindErr := bindTelemetryDestination(ctx, prefix, telemetry.DB())
		if err := errors.Join(bindErr, telemetry.Close(ctx)); err != nil {
			t.Fatal(err)
		}
		if err := prefix.Close(); err != nil {
			t.Fatal(err)
		}
		prefixClosed = true
		open := func() (*GlobalDB, func()) {
			t.Helper()
			db, err := openGlobalMigrationUpgrade(t, path)
			if err != nil {
				t.Fatal(err)
			}
			closed := false
			closeDB := func() {
				if !closed {
					closed = true
					if err := db.Close(context.Background()); err != nil {
						t.Errorf("close upgraded budget DB: %v", err)
					}
				}
			}
			t.Cleanup(closeDB)
			return db, closeDB
		}
		db, closeDB := open()
		stats := store.TokenStatsUpdate{
			SessionID: "budget-upgrade-session", TurnID: "new-turn", AgentName: "writer", TotalTokens: new(int64(20)),
			CostStatus: "estimated", CostSource: "builtin", CostAmount: new(0.2), CostCurrency: new("USD"),
			Turns: 1, UpdatedAt: at,
		}
		daily := store.TokenUsageDailyUpdate{
			ProfileID: store.DefaultProfileID, Day: store.LocalDay(at), AgentName: stats.AgentName,
			TotalTokens: stats.TotalTokens, CostStatus: stats.CostStatus, CostSource: stats.CostSource,
			CostAmount: stats.CostAmount, CostCurrency: stats.CostCurrency, Turns: 1, UpdatedAt: at,
		}
		for _, wantAccepted := range []bool{true, false} {
			accepted, err := db.RecordTokenUsageOnce(ctx, stats, daily)
			if err != nil || accepted != wantAccepted {
				t.Fatalf("post-upgrade receipt accepted=%t err=%v, want accepted=%t", accepted, err, wantAccepted)
			}
		}
		closeDB()
		db, closeDB = open()
		defer closeDB()
		usage, err := db.Instances().ReadDailyBudgetUsage(ctx, "budget-projection-upgrade", at)
		if err != nil || usage.Tokens != 30 || usage.USD == nil || usage.USD.Cmp(big.NewRat(3, 10)) != 0 ||
			usage.HasUnpricedUsage {
			t.Fatalf("reopened projected budget usage=%+v err=%v, want 30 tokens and exact USD 0.30", usage, err)
		}
		var tokens int64
		var usd string
		if err := db.db.QueryRowContext(ctx, `SELECT total_tokens, usd_total
			FROM agent_instance_daily_budget_usage WHERE instance_id='budget-projection-upgrade'
			AND profile_id=? AND day=?`, store.DefaultProfileID, store.LocalDay(at)).Scan(&tokens, &usd); err != nil {
			t.Fatal(err)
		}
		if tokens != 30 || usd != "3/10" {
			t.Fatalf("persisted projection tokens=%d USD=%s, want 30 tokens and 3/10 USD", tokens, usd)
		}
		assertGlobalMigrationIntegrity(ctx, t, db.db)
	})
}

func TestDailyBudgetProjectionPreservesReceiptsAfterOverflow(t *testing.T) {
	for _, scenario := range []struct {
		name             string
		legacyCorruption bool
		cachedCorruption bool
	}{
		{name: "Should preserve canonical receipts after token overflow"},
		{name: "Should preserve canonical receipts despite an unreadable historical amount", legacyCorruption: true},
		{name: "Should preserve canonical receipts despite an unreadable stored total", cachedCorruption: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			db := openTestGlobalDB(t)
			at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
			stamp := store.FormatTimestamp(at)
			if _, err := db.db.ExecContext(ctx, `INSERT INTO agent_instances (
			id, profile_id, agent_name, workspace_id, state, created_by_kind, created_by_id, created_at, updated_at
		) VALUES ('budget-total-overflow', ?, 'writer', '', 'active', 'human', 'operator', ?, ?)`,
				store.DefaultProfileID, stamp, stamp); err != nil {
				t.Fatal(err)
			}
			wantReceipts := 2
			if scenario.legacyCorruption {
				if _, err := db.db.ExecContext(ctx, `INSERT INTO token_usage_receipts (
				profile_id, session_id, turn_id, instance_id, day, total_tokens, cost_status, created_at
			) VALUES (?, 'older-session', 'older-turn', 'budget-total-overflow', ?, 'invalid', 'unknown', ?)`,
					store.DefaultProfileID, store.LocalDay(at), stamp); err != nil {
					t.Fatal(err)
				}
				wantReceipts++
			}
			for index, tc := range []struct {
				agent, session string
				tokens         int64
			}{
				{agent: "overflow-writer", session: "overflow-a", tokens: math.MaxInt64},
				{agent: "overflow-reviewer", session: "overflow-b", tokens: 1},
			} {
				if _, err := db.db.ExecContext(ctx, `INSERT INTO sessions (
				id, profile_id, scope, agent_name, workspace_id, session_type, state, instance_id, created_at, updated_at
			) VALUES (?, ?, 'global', ?, '', 'user', 'running', 'budget-total-overflow', ?, ?)`,
					tc.session, store.DefaultProfileID, tc.agent, stamp, stamp); err != nil {
					t.Fatal(err)
				}
				stats := store.TokenStatsUpdate{
					SessionID: tc.session, TurnID: "overflow-turn", AgentName: tc.agent, TotalTokens: &tc.tokens,
					CostStatus: "unknown", CostSource: "none", Turns: 1, UpdatedAt: at,
				}
				daily := store.TokenUsageDailyUpdate{
					ProfileID: store.DefaultProfileID, Day: store.LocalDay(at), AgentName: tc.agent,
					TotalTokens: &tc.tokens, CostStatus: "unknown", CostSource: "none", Turns: 1, UpdatedAt: at,
				}
				if accepted, err := db.RecordTokenUsageOnce(ctx, stats, daily); err != nil || !accepted {
					t.Fatalf("receipt for %s accepted=%t err=%v", tc.session, accepted, err)
				}
				if scenario.cachedCorruption && index == 0 {
					if _, err := db.db.ExecContext(ctx, `UPDATE agent_instance_daily_budget_usage
					SET total_tokens='invalid' WHERE instance_id='budget-total-overflow'`); err != nil {
						t.Fatal(err)
					}
				}
			}
			if _, err := db.Instances().
				ReadDailyBudgetUsage(ctx, "budget-total-overflow", at); !errors.Is(
				err,
				errDailyBudgetTotalsInvalid,
			) {
				t.Fatalf("overflowing budget total returned %v, want invalid-budget error", err)
			}
			var receipts, projections int
			if err := db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM token_usage_receipts
			WHERE instance_id='budget-total-overflow'`).Scan(&receipts); err != nil {
				t.Fatal(err)
			}
			if err := db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM token_stats
			WHERE session_id IN ('overflow-a', 'overflow-b')`).Scan(&projections); err != nil {
				t.Fatal(err)
			}
			var dailyRows int
			if err := db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM token_usage_daily
			WHERE day=? AND agent_name IN ('overflow-writer', 'overflow-reviewer')`, store.LocalDay(at)).
				Scan(&dailyRows); err != nil {
				t.Fatal(err)
			}
			if receipts != wantReceipts || projections != 2 || dailyRows != 2 {
				t.Fatalf("overflow preserved receipts=%d session=%d daily=%d, want %d receipts and 2 each",
					receipts, projections, dailyRows, wantReceipts)
			}
		})
	}
}
