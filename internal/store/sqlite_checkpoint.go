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
	return passiveCheckpoint(ctx, db)
}

// CheckpointPassiveConnection runs a checkpoint on an already acquired connection.
// Owners can coordinate physical file mutation without opening a connection under their lease.
func CheckpointPassiveConnection(ctx context.Context, connection *sql.Conn) error {
	if connection == nil {
		return nil
	}
	return passiveCheckpoint(ctx, connection)
}

func passiveCheckpoint(ctx context.Context, executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}) error {
	if _, err := executor.ExecContext(ctx, "PRAGMA wal_checkpoint(PASSIVE)"); err != nil {
		return fmt.Errorf("store: passive checkpoint sqlite wal: %w", err)
	}
	return nil
}
