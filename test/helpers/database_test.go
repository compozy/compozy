package helpers

import (
	"context"
	"testing"
)

// TestTransactionIsolation demonstrates the new transaction-based test isolation
func TestTransactionIsolation(t *testing.T) {
	ctx := context.Background()

	// Start a test transaction
	tx, cleanup := GetSharedPostgresTx(ctx, t)
	defer cleanup() // Optional - t.Cleanup already registered

	// Insert a test record (only visible within this transaction)
	_, err := tx.Exec(ctx, "INSERT INTO users (id, email, role) VALUES ($1, $2, $3)",
		"test-id", "test@example.com", "user")
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	// Verify the record exists within the transaction
	var count int
	err = tx.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE id = $1", "test-id").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query test data: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 record, got %d", count)
	}

	// The transaction will be rolled back automatically by t.Cleanup,
	// so this data won't affect other tests
}
