package globaldb

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

// TestGlobalDBAlwaysOnMigration owns IT-001: migration 00186 renames the v0.5.0 stays_on
// vocabulary in persisted rows. It rebuilds agent_heartbeat_wake_events with the
// definition_not_always_on reason, maps pause reasons, and rewrites agent resource kinds,
// without losing a row or the cross-table workspace cleanup trigger.
func TestGlobalDBAlwaysOnMigration(t *testing.T) {
	t.Parallel()

	t.Run("Should rename stays_on rows and keep every row across reopen", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefixDB, err := openGlobalMigrationPrefixDatabase(t, path, globalMigrationPrefixBefore(t, "00186_schema.sql"))
		if err != nil {
			t.Fatalf("open prefix before 00186 error = %v", err)
		}
		ctx := globalMigrationTestContext(t)
		seedAlwaysOnMigrationRows(ctx, t, prefixDB)
		if err := prefixDB.Close(); err != nil {
			t.Fatalf("close prefix before 00186 error = %v", err)
		}

		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("upgrade through 00186 error = %v", err)
		}
		assertAlwaysOnMigrationRows(ctx, t, upgraded.db)
		assertGlobalMigrationIntegrity(ctx, t, upgraded.db)
		if _, err := upgraded.db.ExecContext(ctx, `INSERT INTO agent_heartbeat_wake_events (
			id, workspace_id, agent_name, source, result, reason, created_at, expires_at
		) VALUES ('wake-old', 'ws-always-on', 'triage', 'scheduler', 'skipped', 'definition_not_stays_on', ?, ?)`,
			alwaysOnMigrationNow, alwaysOnMigrationNow); err == nil {
			t.Fatal("insert definition_not_stays_on after 00186 succeeded, want CHECK failure")
		}
		if err := upgraded.Close(ctx); err != nil {
			t.Fatalf("close upgraded database error = %v", err)
		}

		reopened, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("reopen migrated database error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := reopened.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("close reopened database error = %v", closeErr)
			}
		})
		assertAlwaysOnMigrationRows(ctx, t, reopened.db)
		status, err := store.Status(ctx, reopened.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(after reopen) error = %v", err)
		}
		assertCompleteMigrationStream(t, status, MigrationStream())

		var triggers int
		if err := reopened.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master
			WHERE type = 'trigger' AND name = 'workspace_scope_cleanup_after_delete'`).Scan(&triggers); err != nil {
			t.Fatalf("count cleanup trigger error = %v", err)
		}
		if triggers != 1 {
			t.Fatalf("workspace_scope_cleanup_after_delete triggers = %d, want 1", triggers)
		}
		if _, err := reopened.db.ExecContext(ctx, `DELETE FROM agent_instances`); err != nil {
			t.Fatalf("delete instances before workspace delete error = %v", err)
		}
		if _, err := reopened.db.ExecContext(ctx, `DELETE FROM workspaces WHERE id = 'ws-always-on'`); err != nil {
			t.Fatalf("delete workspace error = %v", err)
		}
		if got := countRows(ctx, t, reopened.db, `SELECT COUNT(*) FROM agent_heartbeat_wake_events`); got != 0 {
			t.Fatalf("wake events after workspace delete = %d, want 0 (cleanup trigger fired)", got)
		}
	})

	t.Run("Should migrate an empty database", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		db, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("open empty database error = %v", err)
		}
		ctx := globalMigrationTestContext(t)
		if err := db.Close(ctx); err != nil {
			t.Fatalf("close empty database error = %v", err)
		}
		reopened, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("reopen empty database error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := reopened.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("close reopened empty database error = %v", closeErr)
			}
		})
		if got := countRows(ctx, t, reopened.db, `SELECT COUNT(*) FROM agent_heartbeat_wake_events`); got != 0 {
			t.Fatalf("wake events in empty database = %d, want 0", got)
		}
	})
}

var alwaysOnMigrationNow = store.FormatTimestamp(time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC))

func seedAlwaysOnMigrationRows(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()
	now := alwaysOnMigrationNow
	statements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO workspaces (id, root_dir, add_dirs, name, created_at, updated_at)
			VALUES ('ws-always-on', ?, '[]', 'always-on', ?, ?)`, []any{t.TempDir(), now, now}},
		{`INSERT INTO sessions (id, profile_id, agent_name, workspace_id, session_type, state,
			root_session_id, spawn_depth, created_at, updated_at)
			VALUES ('sess-main', ?, 'scout', 'ws-always-on', 'user', 'stopped', 'sess-main', 0, ?, ?)`,
			[]any{store.DefaultProfileID, now, now}},
		{`INSERT INTO agent_heartbeat_wake_events (id, workspace_id, agent_name, source, result, reason,
			created_at, expires_at) VALUES
			('wake-renamed', 'ws-always-on', 'triage', 'instance_check', 'skipped', 'definition_not_stays_on', ?, ?),
			('wake-sent', 'ws-always-on', 'triage', 'scheduler', 'sent', 'wake_sent', ?, ?),
			('wake-missing', 'ws-always-on', 'triage', 'scheduler', 'skipped', 'definition_missing', ?, ?)`,
			[]any{now, now, now, now, now, now}},
		{`INSERT INTO agent_instances (id, profile_id, agent_name, workspace_id, state, pause_reason,
			main_session_id, created_by_kind, created_by_id, created_at, updated_at) VALUES
			('inst-paused', ?, 'triage', 'ws-always-on', 'paused', 'definition_not_stays_on', NULL, 'operator', 'op', ?, ?),
			('inst-active', ?, 'scout', 'ws-always-on', 'active', '', 'sess-main', 'operator', 'op', ?, ?)`,
			[]any{store.DefaultProfileID, now, now, store.DefaultProfileID, now, now}},
		{`INSERT INTO resource_records (kind, id, version, scope_kind, scope_id, owner_kind, owner_id,
			source_kind, source_id, spec_json, created_at, updated_at) VALUES
			('agent', 'triage', 1, 'user', NULL, 'user', 'u', 'file', 'f', '{"name":"triage","kind":"stays_on","provider":"claude"}', ?, ?),
			('agent', 'helper', 1, 'user', NULL, 'user', 'u', 'file', 'f', '{"name":"helper","kind":"on_demand"}', ?, ?),
			('soul', 'triage', 1, 'user', NULL, 'user', 'u', 'file', 'f', '{"kind":"stays_on"}', ?, ?)`,
			[]any{now, now, now, now, now, now}},
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement.query, statement.args...); err != nil {
			t.Fatalf("seed before 00186 error = %v\n%s", err, statement.query)
		}
	}
}

func assertAlwaysOnMigrationRows(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()
	for query, want := range map[string]int{
		`SELECT COUNT(*) FROM agent_heartbeat_wake_events`: 3,
		`SELECT COUNT(*) FROM agent_instances`:             2,
		`SELECT COUNT(*) FROM resource_records`:            3,
	} {
		if got := countRows(ctx, t, db, query); got != want {
			t.Fatalf("%s = %d, want %d", query, got, want)
		}
	}
	wantReasons := map[string]string{
		"wake-renamed": "definition_not_always_on",
		"wake-sent":    "wake_sent",
		"wake-missing": "definition_missing",
	}
	for id, want := range wantReasons {
		if got := queryString(
			ctx,
			t,
			db,
			`SELECT reason FROM agent_heartbeat_wake_events WHERE id = ?`,
			id,
		); got != want {
			t.Fatalf("wake %s reason = %q, want %q", id, got, want)
		}
	}
	if got := queryString(ctx, t, db, `SELECT pause_reason FROM agent_instances WHERE id = 'inst-paused'`); got !=
		"definition_not_always_on" {
		t.Fatalf("paused instance reason = %q, want definition_not_always_on", got)
	}
	if got := queryString(ctx, t, db, `SELECT state || '|' || pause_reason || '|' || main_session_id
		FROM agent_instances WHERE id = 'inst-active'`); got != "active||sess-main" {
		t.Fatalf("active instance = %q, want active||sess-main", got)
	}
	wantSpecs := map[string]string{
		`SELECT spec_json FROM resource_records WHERE kind = 'agent' AND id = 'triage'`: `{"name":"triage","kind":"always_on","provider":"claude"}`,
		`SELECT spec_json FROM resource_records WHERE kind = 'agent' AND id = 'helper'`: `{"name":"helper","kind":"on_demand"}`,
		`SELECT spec_json FROM resource_records WHERE kind = 'soul' AND id = 'triage'`:  `{"kind":"stays_on"}`,
	}
	for query, want := range wantSpecs {
		if got := queryString(ctx, t, db, query); got != want {
			t.Fatalf("%s = %s, want %s", query, got, want)
		}
	}
}

func countRows(ctx context.Context, t *testing.T, db *sql.DB, query string) int {
	t.Helper()
	var count int
	if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		t.Fatalf("%s error = %v", query, err)
	}
	return count
}

func queryString(ctx context.Context, t *testing.T, db *sql.DB, query string, args ...any) string {
	t.Helper()
	var value string
	if err := db.QueryRowContext(ctx, query, args...).Scan(&value); err != nil {
		t.Fatalf("%s error = %v", query, err)
	}
	return value
}
