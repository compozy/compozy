package helpers

import (
	"context"
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

// NewDatabaseHelper creates a new database helper using a test container.
func NewDatabaseHelper(t *testing.T) *DatabaseHelper {
	pool, cleanup := helpers.CreateTestContainerDatabase(context.Background(), t)
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
	ctx := context.Background()
	for _, table := range tables {
		query := fmt.Sprintf("TRUNCATE TABLE %s CASCADE", pgx.Identifier{table}.Sanitize())
		_, err := h.pool.Exec(ctx, query)
		require.NoError(t, err, "Failed to truncate table %s", table)
	}
}

// BeginTx starts a new transaction for test isolation
func (h *DatabaseHelper) BeginTx(t *testing.T) pgx.Tx {
	ctx := context.Background()
	tx, err := h.pool.Begin(ctx)
	require.NoError(t, err, "Failed to begin transaction")
	return tx
}
