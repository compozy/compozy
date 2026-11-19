package globaldb

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
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
