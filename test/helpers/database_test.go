package helpers

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/pkg/config"
)

// TestTransactionIsolation demonstrates the new transaction-based test isolation
func TestTransactionIsolation(t *testing.T) {
	t.Run("Should rollback inserted data via transaction cleanup", func(t *testing.T) {
		ctx := t.Context()

		// Start a test transaction
		tx, cleanup := GetSharedPostgresTx(t)
		t.Cleanup(cleanup)

		// Insert a test record (only visible within this transaction)
		_, err := tx.Exec(ctx, "INSERT INTO users (id, email, role) VALUES ($1, $2, $3)",
			"test-id", "test@example.com", "user")
		require.NoError(t, err, "failed to insert test data")

		// Verify the record exists within the transaction
		var count int64
		err = tx.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE id = $1", "test-id").Scan(&count)
		require.NoError(t, err, "failed to query test data")

		assert.Equal(t, int64(1), count, "expected exactly one record")
		// The transaction will be rolled back automatically by t.Cleanup,
		// so this data won't affect other tests
	})
}

func TestSetupDatabaseWithModeMemory(t *testing.T) {
	db, cleanup := SetupDatabaseWithMode(t, config.ModeMemory)
	defer cleanup()

	assert.Equal(t, "sqlite", db.DriverName())

	var mainFile string
	require.NoError(
		t,
		db.QueryRowxContext(t.Context(), "SELECT file FROM pragma_database_list WHERE name = 'main'").Scan(&mainFile),
	)
	assert.True(
		t,
		mainFile == "" || strings.Contains(mainFile, "mode=memory"),
		"expected in-memory SQLite database, got %q",
		mainFile,
	)

	var result int
	require.NoError(t, db.QueryRowxContext(t.Context(), "SELECT 1").Scan(&result))
	assert.Equal(t, 1, result)
}

func TestSetupDatabaseWithModePersistent(t *testing.T) {
	db, cleanup := SetupDatabaseWithMode(t, config.ModePersistent)

	assert.Equal(t, "sqlite", db.DriverName())

	var mainFile string
	require.NoError(
		t,
		db.QueryRowxContext(t.Context(), "SELECT file FROM pragma_database_list WHERE name = 'main'").Scan(&mainFile),
	)
	require.NotEmpty(t, mainFile)
	t.Logf("persistent database path: %s", mainFile)
	info, err := os.Stat(mainFile)
	require.NoError(t, err)
	assert.False(t, info.IsDir())

	cleanup()
	_, err = os.Stat(mainFile)
	assert.True(t, os.IsNotExist(err), "expected persistent database file to be removed after cleanup")
}

func TestSetupDatabaseWithModeDistributed(t *testing.T) {
	db, cleanup := SetupDatabaseWithMode(t, config.ModeDistributed)
	defer cleanup()

	assert.Equal(t, "pgx", db.DriverName())

	var result int
	require.NoError(t, db.QueryRowxContext(t.Context(), "SELECT 1").Scan(&result))
	assert.Equal(t, 1, result)
}

func TestSetupDatabaseWithModeSwitching(t *testing.T) {
	modes := []string{config.ModeMemory, config.ModePersistent, config.ModeDistributed}

	for _, m := range modes {
		m := m
		t.Run(m, func(t *testing.T) {
			db, cleanup := SetupDatabaseWithMode(t, m)
			defer cleanup()

			var out int
			require.NoError(t, db.QueryRowxContext(t.Context(), "SELECT 1").Scan(&out))
			assert.Equal(t, 1, out)
		})
	}
}
