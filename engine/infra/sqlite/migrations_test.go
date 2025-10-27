package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrations(t *testing.T) {
	t.Run("Should apply all migrations successfully", func(t *testing.T) {
		ctx := t.Context()
		dbPath := filepath.Join(t.TempDir(), "apply.db")
		require.NoError(t, ApplyMigrations(ctx, dbPath))

		store, err := NewStore(ctx, &Config{Path: dbPath})
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, store.Close(ctx))
		})
	})

	t.Run("Should create all required tables", func(t *testing.T) {
		ctx := t.Context()
		dbPath := filepath.Join(t.TempDir(), "tables.db")
		require.NoError(t, ApplyMigrations(ctx, dbPath))

		db := openTestSQLite(ctx, t, dbPath)
		defer db.Close()

		expected := map[string]bool{
			"workflow_states": true,
			"task_states":     true,
			"users":           true,
			"api_keys":        true,
		}
		rows, err := db.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type = 'table'")
		require.NoError(t, err)
		defer rows.Close()

		for rows.Next() {
			var name string
			require.NoError(t, rows.Scan(&name))
			delete(expected, name)
		}
		require.NoError(t, rows.Err())
		assert.Empty(t, expected)
	})

	t.Run("Should create all indexes", func(t *testing.T) {
		ctx := t.Context()
		dbPath := filepath.Join(t.TempDir(), "indexes.db")
		require.NoError(t, ApplyMigrations(ctx, dbPath))

		db := openTestSQLite(ctx, t, dbPath)
		defer db.Close()

		indexes := make(map[string]bool)
		rows, err := db.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type = 'index'")
		require.NoError(t, err)
		defer rows.Close()
		for rows.Next() {
			var name string
			require.NoError(t, rows.Scan(&name))
			indexes[name] = true
		}
		require.NoError(t, rows.Err())

		expectedIndexes := []string{
			"idx_workflow_states_status",
			"idx_workflow_states_workflow_id",
			"idx_workflow_states_workflow_status",
			"idx_workflow_states_created_at",
			"idx_workflow_states_updated_at",
			"idx_task_states_status",
			"idx_task_states_workflow_id",
			"idx_task_states_workflow_exec_id",
			"idx_task_states_task_id",
			"idx_task_states_parent_state_id",
			"idx_users_email_ci",
			"idx_users_role",
			"idx_api_keys_hash",
			"idx_api_keys_user_id",
			"idx_api_keys_created_at",
		}
		for _, idx := range expectedIndexes {
			assert.Truef(t, indexes[idx], "expected index %s to exist", idx)
		}
	})

	t.Run("Should enforce foreign keys", func(t *testing.T) {
		ctx := t.Context()
		dbPath := filepath.Join(t.TempDir(), "fk.db")
		require.NoError(t, ApplyMigrations(ctx, dbPath))

		db := openTestSQLite(ctx, t, dbPath)
		defer db.Close()

		_, err := db.ExecContext(
			ctx,
			`INSERT INTO task_states (component, status, task_exec_id, task_id, workflow_exec_id, workflow_id, execution_type)
			 VALUES ('component', 'pending', 'task-exec', 'task', 'missing-workflow', 'workflow', 'router')`,
		)
		require.Error(t, err)
	})

	t.Run("Should enforce check constraints", func(t *testing.T) {
		ctx := t.Context()
		dbPath := filepath.Join(t.TempDir(), "constraints.db")
		require.NoError(t, ApplyMigrations(ctx, dbPath))

		db := openTestSQLite(ctx, t, dbPath)
		defer db.Close()

		_, err := db.ExecContext(
			ctx,
			"INSERT INTO users (id, email, role) VALUES ('user-1', 'user@example.com', 'guest')",
		)
		require.Error(t, err)
	})

	t.Run("Should rollback migrations", func(t *testing.T) {
		ctx := t.Context()
		dbPath := filepath.Join(t.TempDir(), "rollback.db")
		require.NoError(t, ApplyMigrations(ctx, dbPath))

		db := openTestSQLite(ctx, t, dbPath)
		defer db.Close()

		goose.SetBaseFS(migrationsFS)
		require.NoError(t, goose.SetDialect("sqlite3"))
		require.NoError(t, goose.DownToContext(ctx, db, "migrations", 0))

		var count int
		err := db.QueryRowContext(
			ctx,
			"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name IN ('workflow_states', 'task_states', 'users', 'api_keys')",
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("Should be idempotent", func(t *testing.T) {
		ctx := t.Context()
		dbPath := filepath.Join(t.TempDir(), "idempotent.db")
		require.NoError(t, ApplyMigrations(ctx, dbPath))
		require.NoError(t, ApplyMigrations(ctx, dbPath))
	})
}

func openTestSQLite(ctx context.Context, t *testing.T, dbPath string) *sql.DB {
	t.Helper()
	dsn, _, err := buildDSN(&Config{Path: dbPath})
	require.NoError(t, err)
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	require.NoError(t, db.PingContext(ctx))
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})
	return db
}
