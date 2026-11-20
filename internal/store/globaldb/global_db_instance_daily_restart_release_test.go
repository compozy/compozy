package globaldb

import (
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
)

func TestDailyBudgetRestartReleaseSelectsOperatorQueues(t *testing.T) {
	for _, tc := range []struct {
		name    string
		latched bool
		nested  bool
	}{
		{name: "Should release only operator queues from a budget pause"},
		{name: "Should release only operator queues from an active budget latch", latched: true},
		{name: "Should release a nested budget-held main beneath a manually paused root", nested: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			db := openTestGlobalDB(t)
			at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
			main := budgetBoundSession(t, db, at)
			const instance = "daily-dispatch-instance"
			if tc.nested {
				bindBudgetReleaseTestOuterRoot(t, db, instance, main, at)
			}
			if tc.latched {
				if _, err := db.DB().ExecContext(ctx, `INSERT INTO agent_instance_daily_budget_recovery
				(instance_id, generation, pending, latched) VALUES (?, 1, 0, 1)`, instance); err != nil {
					t.Fatal(err)
				}
			} else if _, err := db.Instances().Pause(ctx, instance, "daily_budget"); err != nil {
				t.Fatal(err)
			}
			if _, _, err := db.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
				ID: "restart-app", SessionID: main, Text: "app input", AppIngress: true, HumanIngress: true,
				QueueCap: 2, Now: at,
			}); err != nil {
				t.Fatal(err)
			}
			if ids, err := db.Instances().QueuedSessions(ctx, instance, ""); err != nil || len(ids) != 0 {
				t.Fatalf("release sessions with app input = %v, %v; want none", ids, err)
			}
			if _, _, err := db.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
				ID: "restart-operator", SessionID: main, Text: "operator input", HumanIngress: true,
				QueueCap: 2, Now: at.Add(time.Second),
			}); err != nil {
				t.Fatal(err)
			}
			if ids, err := db.Instances().
				QueuedSessions(ctx, instance, ""); err != nil || len(ids) != 1 ||
				ids[0] != main {
				t.Fatalf("release sessions with operator input = %v, %v; want %s", ids, err, main)
			}
		})
	}
}

func bindBudgetReleaseTestOuterRoot(t *testing.T, db *GlobalDB, instance, main string, at time.Time) {
	t.Helper()
	ctx := t.Context()
	stamp := store.FormatTimestamp(at)
	if _, err := db.DB().ExecContext(ctx, `INSERT INTO agent_instances
		(id, profile_id, agent_name, workspace_id, state, pause_reason,
		 created_by_kind, created_by_id, created_at, updated_at)
		VALUES ('outer', ?, 'outer', '', 'paused', 'manual', 'human', 'operator', ?, ?)`,
		store.DefaultProfileID, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().ExecContext(ctx, `INSERT INTO sessions
		(id, profile_id, scope, agent_name, workspace_id, session_type, state, instance_id, created_at, updated_at)
		SELECT 'outer-session', profile_id, scope, 'outer', workspace_id, session_type, 'stopped', 'outer', ?, ?
		FROM sessions WHERE id = ?`, stamp, stamp, main); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().ExecContext(ctx, `UPDATE sessions SET
		state='stopped', parent_session_id='outer-session', root_session_id='outer-session', lineage_kind='spawn'
		WHERE id=?`, main); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().
		ExecContext(ctx, "UPDATE agent_instances SET main_session_id=? WHERE id=?", main, instance); err != nil {
		t.Fatal(err)
	}
}
