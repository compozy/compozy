package store

import (
	"context"
	"database/sql"
	"fmt"
)

// Checkpoint truncates the WAL for an open SQLite database.
func Checkpoint(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return nil
	}
	if _, err := db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("store: checkpoint sqlite wal: %w", err)
	}
	return nil
}

// CheckpointPassive runs a non-truncating SQLite WAL checkpoint.
func CheckpointPassive(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return nil
	}
	if _, err := db.ExecContext(ctx, "PRAGMA wal_checkpoint(PASSIVE)"); err != nil {
		return fmt.Errorf("store: passive checkpoint sqlite wal: %w", err)
	}
	return nil
}
