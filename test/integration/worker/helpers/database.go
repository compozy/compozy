package helpers

import (
	"fmt"
	"testing"

	helpers "github.com/compozy/compozy/test/helpers"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// DatabaseHelper provides database setup and teardown for integration tests
type DatabaseHelper struct {
	pool    *pgxpool.Pool
	cleanup func()
}

// NewDatabaseHelper creates a new database helper using the shared container pattern.
func NewDatabaseHelper(t *testing.T) *DatabaseHelper {
	pool, cleanup := helpers.GetSharedPostgresDB(t)
	return &DatabaseHelper{
		pool:    pool,
		cleanup: cleanup,
	}
}

// GetPool returns the database connection pool
func (h *DatabaseHelper) GetPool() *pgxpool.Pool {
	return h.pool
}

// Cleanup cleans up database resources
func (h *DatabaseHelper) Cleanup(t *testing.T) {
	h.cleanup()
	t.Logf("Database helper cleanup completed")
}

// TruncateTables truncates all tables for test cleanup
func (h *DatabaseHelper) TruncateTables(t *testing.T, tables ...string) {
	for _, table := range tables {
		query := fmt.Sprintf("TRUNCATE TABLE %s CASCADE", pgx.Identifier{table}.Sanitize())
		_, err := h.pool.Exec(t.Context(), query)
		require.NoError(t, err, "Failed to truncate table %s", table)
	}
}

// BeginTx starts a new transaction for test isolation
func (h *DatabaseHelper) BeginTx(t *testing.T) pgx.Tx {
	tx, err := h.pool.Begin(t.Context())
	require.NoError(t, err, "Failed to begin transaction")
	return tx
}
