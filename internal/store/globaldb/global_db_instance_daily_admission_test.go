package globaldb

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
)

func TestInstanceDailyBudgetTurnAdmission(t *testing.T) {
	ctx := context.Background()
	db := openTestGlobalDB(t)
	at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
	db.now = func() time.Time { return at }
	stamp := store.FormatTimestamp(at)
	if _, err := db.db.ExecContext(ctx, `INSERT INTO agent_instances (
		id, profile_id, agent_name, workspace_id, state, created_by_kind, created_by_id, created_at, updated_at
	) VALUES ('budget-admit', ?, 'writer', '', 'active', 'human', 'operator', ?, ?)`,
		store.DefaultProfileID, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.ExecContext(ctx, `INSERT INTO sessions (
		id, profile_id, scope, agent_name, workspace_id, session_type, state, instance_id, created_at, updated_at
	) VALUES ('budget-session', ?, 'global', 'writer', '', 'user', 'running', 'budget-admit', ?, ?)`,
		store.DefaultProfileID, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if err := db.Instances().SetDailyBudgetLimits(ctx, "budget-admit", agentinstance.DailyBudgetLimits{
		Tokens: new(int64(100)),
	}); err != nil {
		t.Fatal(err)
	}
	allowed, trip, err := db.Instances().AdmitDailyBudgetTurn(ctx, "budget-session")
	if err != nil || !allowed || trip != nil {
		t.Fatalf("empty budget: allowed=%v trip=%+v err=%v", allowed, trip, err)
	}
	if _, err := db.db.ExecContext(ctx, `INSERT INTO token_usage_receipts (
		profile_id, session_id, turn_id, instance_id, day, total_tokens, cost_status, created_at
	) VALUES (?, 'budget-session', 'budget-turn-1', 'budget-admit', ?, 100, 'unknown', ?)`,
		store.DefaultProfileID, store.LocalDay(at), stamp); err != nil {
		t.Fatal(err)
	}
	allowed, trip, err = db.Instances().AdmitDailyBudgetTurn(ctx, "budget-session")
	if err != nil || allowed || trip == nil || trip.Reason != "daily_budget" {
		t.Fatalf("exhausted budget: allowed=%v trip=%+v err=%v", allowed, trip, err)
	}
	paused, err := db.Instances().Get(ctx, "budget-admit")
	if err != nil || paused.State != "paused" || paused.PauseReason != "daily_budget" {
		t.Fatalf("durable pause: instance=%+v err=%v", paused, err)
	}
	allowed, trip, err = db.Instances().AdmitDailyBudgetTurn(ctx, "budget-session")
	if err != nil || allowed || trip != nil {
		t.Fatalf("repeated admission: allowed=%v trip=%+v err=%v", allowed, trip, err)
	}
	at = at.AddDate(0, 0, 1)
	allowed, trip, err = db.Instances().AdmitDailyBudgetTurn(ctx, "budget-session")
	if err != nil || allowed || trip != nil {
		t.Fatalf("rollover must not auto-resume: allowed=%v trip=%+v err=%v", allowed, trip, err)
	}
}

func TestInstanceDailyBudgetAdmissionScopeAndUnpricedUsage(t *testing.T) {
	ctx := context.Background()
	db := openTestGlobalDB(t)
	at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
	db.now = func() time.Time { return at }
	stamp := store.FormatTimestamp(at)
	for _, instance := range []struct{ id, agent string }{
		{id: "scope-one", agent: "writer"}, {id: "scope-two", agent: "editor"},
	} {
		if _, err := db.db.ExecContext(ctx, `INSERT INTO agent_instances (
			id, profile_id, agent_name, workspace_id, state, created_by_kind, created_by_id, created_at, updated_at
		) VALUES (?, ?, ?, '', 'active', 'human', 'operator', ?, ?)`,
			instance.id, store.DefaultProfileID, instance.agent, stamp, stamp); err != nil {
			t.Fatal(err)
		}
		if err := db.Instances().SetDailyBudgetLimits(ctx, instance.id,
			agentinstance.DailyBudgetLimits{Tokens: new(int64(100)), USD: new(float64(0.5))}); err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []struct{ session, instance string }{
		{session: "scope-session-one", instance: "scope-one"},
		{session: "scope-session-two", instance: "scope-two"},
		{session: "scope-session-free", instance: ""},
	} {
		if _, err := db.db.ExecContext(ctx, `INSERT INTO sessions (
			id, profile_id, scope, agent_name, workspace_id, session_type, state, instance_id, created_at, updated_at
		) VALUES (?, ?, 'global', 'writer', '', 'user', 'running', ?, ?, ?)`,
			row.session, store.DefaultProfileID,
			sql.NullString{String: row.instance, Valid: row.instance != ""}, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.db.ExecContext(ctx, `INSERT INTO token_usage_receipts (
		profile_id, session_id, turn_id, instance_id, day, total_tokens, cost_status, created_at
	) VALUES (?, 'scope-session-one', 'scope-unpriced', 'scope-one', ?, 25, 'unknown', ?)`,
		store.DefaultProfileID, store.LocalDay(at), stamp); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		session string
		allowed bool
		tripped bool
	}{
		{session: "scope-session-one", tripped: true},
		{session: "scope-session-two", allowed: true},
		{session: "scope-session-free", allowed: true},
	} {
		allowed, trip, err := db.Instances().AdmitDailyBudgetTurn(ctx, tc.session)
		if err != nil || allowed != tc.allowed || (trip != nil) != tc.tripped {
			t.Fatalf("session %s: allowed=%v trip=%+v err=%v", tc.session, allowed, trip, err)
		}
	}
}

func TestInstanceDailyBudgetAdmissionPreservesPauseOwnership(t *testing.T) {
	ctx := context.Background()
	db := openTestGlobalDB(t)
	at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
	db.now = func() time.Time { return at }
	stamp := store.FormatTimestamp(at)
	for _, tc := range []struct {
		name, reason string
		allowed      bool
	}{
		{name: "budget-paused-cleared", reason: "daily_budget"},
		{name: "manually-paused-unlimited", reason: "manual", allowed: true},
	} {
		if _, err := db.db.ExecContext(ctx, `INSERT INTO agent_instances (
			id, profile_id, agent_name, workspace_id, state, pause_reason,
			created_by_kind, created_by_id, created_at, updated_at
		) VALUES (?, ?, ?, '', 'paused', ?, 'human', 'operator', ?, ?)`,
			tc.name, store.DefaultProfileID, tc.name, tc.reason, stamp, stamp); err != nil {
			t.Fatal(err)
		}
		sessionID := "session-" + tc.name
		if _, err := db.db.ExecContext(ctx, `INSERT INTO sessions (
			id, profile_id, scope, agent_name, workspace_id, session_type, state, instance_id,
			created_at, updated_at
		) VALUES (?, ?, 'global', ?, '', 'user', 'running', ?, ?, ?)`,
			sessionID, store.DefaultProfileID, tc.name, tc.name, stamp, stamp); err != nil {
			t.Fatal(err)
		}
		allowed, trip, err := db.Instances().AdmitDailyBudgetTurn(ctx, sessionID)
		if err != nil || allowed != tc.allowed || trip != nil {
			t.Fatalf("%s: allowed=%v trip=%+v err=%v", tc.name, allowed, trip, err)
		}
		stored, err := db.Instances().Get(ctx, tc.name)
		if err != nil || stored.State != "paused" || stored.PauseReason != tc.reason {
			t.Fatalf("%s: state=%+v err=%v", tc.name, stored, err)
		}
	}
}
