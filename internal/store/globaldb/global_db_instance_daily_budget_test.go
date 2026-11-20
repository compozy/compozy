package globaldb

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestInstanceDailyBudgetLimitsPersist(t *testing.T) {
	for _, tc := range []struct {
		name   string
		limits agentinstance.DailyBudgetLimits
	}{
		{name: "Should leave both dimensions unrestricted when omitted"},
		{name: "Should persist an explicit zero token cap", limits: agentinstance.DailyBudgetLimits{Tokens: new(int64(0))}},
		{
			name:   "Should persist the dollar cap independently of tokens",
			limits: agentinstance.DailyBudgetLimits{USD: new(float64(1.25))},
		},
		{
			name:   "Should persist both caps",
			limits: agentinstance.DailyBudgetLimits{Tokens: new(int64(100)), USD: new(float64(3.75))},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := openTestGlobalDB(t)
			ctx := testutil.Context(t)
			now := store.FormatTimestamp(time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC))
			if _, err := db.db.ExecContext(ctx, `INSERT INTO agent_instances (
				id, profile_id, agent_name, workspace_id, state, created_by_kind, created_by_id, created_at, updated_at
			) VALUES ('instance-limit', ?, 'writer', '', 'active', 'human', 'operator', ?, ?)`,
				store.DefaultProfileID, now, now,
			); err != nil {
				t.Fatal(err)
			}
			if err := db.Instances().SetDailyBudgetLimits(ctx, "instance-limit", tc.limits); err != nil {
				t.Fatalf("SetDailyBudgetLimits() error = %v", err)
			}
			got, err := db.Instances().DailyBudgetLimits(ctx, "instance-limit")
			if err != nil {
				t.Fatal(err)
			}
			if !equalOptionalCap(got.Tokens, tc.limits.Tokens) || !equalOptionalUSD(got.USD, tc.limits.USD) {
				t.Fatalf("restored limits = %+v, want %+v", got, tc.limits)
			}
		})
	}
}

func TestInstanceDailyBudgetLimitsProfileIsolationAfterReopen(t *testing.T) {
	t.Run("Should retain separate instance caps across profiles and restart", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		path := t.TempDir() + "/" + GlobalDatabaseName
		copyCurrentSchemaGlobalDBSeed(t, path)
		first, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		now := store.FormatTimestamp(time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC))
		const secondProfile = "00000000000000000000000001"
		if _, err := first.db.ExecContext(ctx,
			`INSERT INTO profiles (id,name,color,icon,state,created_at)
			 VALUES (?, 'other-budget-profile', '#E8572A', 'circle', 'active', ?)`,
			secondProfile, now,
		); err != nil {
			t.Fatal(err)
		}
		for _, row := range []struct{ id, profile string }{
			{id: "first-budget", profile: store.DefaultProfileID},
			{id: "second-budget", profile: secondProfile},
		} {
			if _, err := first.db.ExecContext(ctx, `INSERT INTO agent_instances (
				id, profile_id, agent_name, workspace_id, state, created_by_kind, created_by_id, created_at, updated_at
			) VALUES (?, ?, 'writer', '', 'active', 'human', 'operator', ?, ?)`,
				row.id, row.profile, now, now,
			); err != nil {
				t.Fatal(err)
			}
		}
		if err := first.Instances().SetDailyBudgetLimits(ctx, "first-budget",
			agentinstance.DailyBudgetLimits{Tokens: new(int64(100))}); err != nil {
			t.Fatal(err)
		}
		if err := first.Instances().SetDailyBudgetLimits(ctx, "second-budget",
			agentinstance.DailyBudgetLimits{Tokens: new(int64(200)), USD: new(float64(1.75))}); err != nil {
			t.Fatal(err)
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
		for _, tc := range []struct {
			id    string
			limit int64
			usd   *float64
		}{
			{id: "first-budget", limit: 100},
			{id: "second-budget", limit: 200, usd: new(float64(1.75))},
		} {
			got, err := reopened.Instances().DailyBudgetLimits(ctx, tc.id)
			if err != nil || !equalOptionalCap(got.Tokens, &tc.limit) || !equalOptionalUSD(got.USD, tc.usd) {
				t.Fatalf("instance %s limits=%+v error=%v", tc.id, got, err)
			}
		}
	})
}

func TestInstanceDailyBudgetWarningIsAtomicAndOncePerDay(t *testing.T) {
	t.Run("Should enqueue one profile-scoped bell notice per local day", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
		stamp := store.FormatTimestamp(at)
		if _, err := db.db.ExecContext(ctx, `INSERT INTO agent_instances (
			id, profile_id, agent_name, workspace_id, state, created_by_kind, created_by_id, created_at, updated_at
		) VALUES ('instance-warning', ?, 'writer', '', 'active', 'human', 'operator', ?, ?)`,
			store.DefaultProfileID, stamp, stamp,
		); err != nil {
			t.Fatal(err)
		}
		if _, err := db.db.ExecContext(ctx, `INSERT INTO sessions (
			id, profile_id, scope, agent_name, workspace_id, session_type, state, created_at, updated_at
		) VALUES ('sess-warning', ?, 'global', 'writer', '', 'user', 'active', ?, ?)`,
			store.DefaultProfileID, stamp, stamp,
		); err != nil {
			t.Fatal(err)
		}
		if _, err := db.db.ExecContext(ctx,
			`UPDATE agent_instances SET main_session_id = 'sess-warning' WHERE id = 'instance-warning'`,
		); err != nil {
			t.Fatal(err)
		}
		first, err := db.Instances().ReserveDailyBudgetWarning(ctx, "instance-warning", at)
		if err != nil || !first {
			t.Fatalf("first day warning=%v error=%v", first, err)
		}
		second, err := db.Instances().ReserveDailyBudgetWarning(ctx, "instance-warning", at.Add(time.Hour))
		if err != nil || second {
			t.Fatalf("duplicate warning=%v error=%v", second, err)
		}
		third, err := db.Instances().ReserveDailyBudgetWarning(ctx, "instance-warning", at.AddDate(0, 0, 1))
		if err != nil || !third {
			t.Fatalf("next day warning=%v error=%v", third, err)
		}
		var count int
		if err := db.db.QueryRowContext(ctx,
			`SELECT count(*) FROM operator_notice_outbox WHERE profile_id = ? AND resource = 'agent_instance'`,
			store.DefaultProfileID,
		).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 2 {
			t.Fatalf("durable bell notices=%d, want exactly 2", count)
		}
	})
}

func TestInstanceDailyBudgetLimitsRejectNoncanonicalStoredUSD(t *testing.T) {
	for _, value := range []struct{ name, usd string }{
		{name: "Should reject a decimal rounded up by float parsing", usd: "9007199254740995"},
		{name: "Should reject a hexadecimal float", usd: "0x1p20"},
	} {
		t.Run(value.name, func(t *testing.T) {
			t.Parallel()
			db := openTestGlobalDB(t)
			ctx := testutil.Context(t)
			stamp := store.FormatTimestamp(time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC))
			if _, err := db.db.ExecContext(ctx, `INSERT INTO agent_instances (
				id, profile_id, agent_name, workspace_id, state, created_by_kind, created_by_id, created_at, updated_at
			) VALUES ('instance-noncanonical', ?, 'writer', '', 'active', 'human', 'operator', ?, ?)`,
				store.DefaultProfileID, stamp, stamp,
			); err != nil {
				t.Fatal(err)
			}
			if _, err := db.db.ExecContext(ctx, `INSERT INTO agent_instance_daily_budget_limits (
				instance_id, usd_cap, updated_at
			) VALUES ('instance-noncanonical', ?, ?)`, value.usd, stamp); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Instances().DailyBudgetLimits(ctx, "instance-noncanonical"); err == nil {
				t.Fatal("noncanonical stored dollar cap was accepted")
			}
		})
	}
}

func TestInstanceDailyBudgetWarningRequiresMainSession(t *testing.T) {
	t.Run("Should not consume the warning key without a bound session", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
		stamp := store.FormatTimestamp(at)
		if _, err := db.db.ExecContext(ctx, `INSERT INTO agent_instances (
			id, profile_id, agent_name, workspace_id, state, created_by_kind, created_by_id, created_at, updated_at
		) VALUES ('instance-no-main-warning', ?, 'writer', '', 'active', 'human', 'operator', ?, ?)`,
			store.DefaultProfileID, stamp, stamp,
		); err != nil {
			t.Fatal(err)
		}
		issued, err := db.Instances().ReserveDailyBudgetWarning(ctx, "instance-no-main-warning", at)
		if err == nil || issued {
			t.Fatalf("missing main session warning=%v error=%v", issued, err)
		}
		var count int
		if err := db.db.QueryRowContext(ctx,
			`SELECT count(*) FROM agent_instance_daily_budget_notices WHERE instance_id = 'instance-no-main-warning'`,
		).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("missing main session consumed %d warning keys", count)
		}
	})
}

func TestInstanceDailyBudgetWarningRespectsWorkspaceMute(t *testing.T) {
	t.Run("Should defer a muted warning and issue one notice after unmute", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
		stamp := store.FormatTimestamp(at)
		workspace := insertAttentionTestWorkspace(t, db, "ws_0123456789abcdef", "budget-warning")
		if _, err := db.db.ExecContext(ctx, `INSERT INTO agent_instances (
			id, profile_id, agent_name, workspace_id, state, created_by_kind, created_by_id, created_at, updated_at
		) VALUES ('instance-muted-warning', ?, 'writer', ?, 'active', 'human', 'operator', ?, ?)`,
			store.DefaultProfileID, workspace.ID, stamp, stamp,
		); err != nil {
			t.Fatal(err)
		}
		if _, err := db.db.ExecContext(ctx, `INSERT INTO sessions (
			id, profile_id, scope, agent_name, workspace_id, session_type, state, created_at, updated_at
		) VALUES ('sess-muted-warning', ?, 'workspace', 'writer', ?, 'user', 'active', ?, ?)`,
			store.DefaultProfileID, workspace.ID, stamp, stamp,
		); err != nil {
			t.Fatal(err)
		}
		if _, err := db.db.ExecContext(ctx, `UPDATE agent_instances SET main_session_id = 'sess-muted-warning'
			WHERE id = 'instance-muted-warning'`); err != nil {
			t.Fatal(err)
		}
		if err := db.ReplaceAttentionWorkspaceMutes(ctx, store.DefaultProfileID, []string{workspace.ID}); err != nil {
			t.Fatal(err)
		}
		issued, err := db.Instances().ReserveDailyBudgetWarning(ctx, "instance-muted-warning", at)
		if err != nil || issued {
			t.Fatalf("muted warning=%v error=%v", issued, err)
		}
		if err := db.ReplaceAttentionWorkspaceMutes(ctx, store.DefaultProfileID, []string{}); err != nil {
			t.Fatal(err)
		}
		for index := range 2 {
			issued, err = db.Instances().ReserveDailyBudgetWarning(ctx, "instance-muted-warning", at)
			if err != nil || issued != (index == 0) {
				t.Fatalf("unmuted attempt %d warning=%v error=%v", index, issued, err)
			}
		}
		var count int
		if err := db.db.QueryRowContext(ctx,
			`SELECT count(*) FROM operator_notice_outbox WHERE session_id = 'sess-muted-warning'`,
		).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("unmuted bell notices=%d, want 1", count)
		}
	})
}

func TestInstanceDailyBudgetLimitsRejectInvalidCaps(t *testing.T) {
	for _, tc := range []struct {
		name   string
		limits agentinstance.DailyBudgetLimits
	}{
		{name: "Should reject negative tokens", limits: agentinstance.DailyBudgetLimits{Tokens: new(int64(-1))}},
		{name: "Should reject negative dollars", limits: agentinstance.DailyBudgetLimits{USD: new(float64(-1))}},
		{name: "Should reject NaN dollars", limits: agentinstance.DailyBudgetLimits{USD: new(math.NaN())}},
		{name: "Should reject infinite dollars", limits: agentinstance.DailyBudgetLimits{USD: new(math.Inf(1))}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.limits.Validate(); err == nil {
				t.Fatal("invalid cap was accepted")
			}
		})
	}
}

func equalOptionalCap(a, b *int64) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

func equalOptionalUSD(a, b *float64) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}
