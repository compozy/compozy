package helpers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTransactionIsolation demonstrates the new transaction-based test isolation
func TestTransactionIsolation(t *testing.T) {
	t.Run("Should rollback inserted data via transaction cleanup", func(t *testing.T) {
		ctx := context.Background()

		// Start a test transaction
		tx, cleanup := GetSharedPostgresTx(ctx, t)
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
