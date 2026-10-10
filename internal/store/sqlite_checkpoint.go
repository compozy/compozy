package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var errSQLiteCheckpointBusy = errors.New("store: sqlite wal checkpoint is busy")

// Checkpoint truncates the WAL for an open SQLite database.
func Checkpoint(ctx context.Context, db *sql.DB) (retErr error) {
	if db == nil {
		return nil
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("store: acquire sqlite checkpoint connection: %w", err)
	}
	defer func() {
		if err := conn.Close(); err != nil && !errors.Is(err, sql.ErrConnDone) {
			retErr = errors.Join(retErr, fmt.Errorf("store: close sqlite checkpoint connection: %w", err))
		}
	}()
	if err := executeSQLiteWithBusyWait(ctx, conn, func() error {
		var busy, frames, checkpointed int
		if err := conn.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").
			Scan(&busy, &frames, &checkpointed); err != nil {
			return err
		}
		if busy != 0 {
			return fmt.Errorf("%w: checkpointed %d of %d frames", errSQLiteCheckpointBusy, checkpointed, frames)
		}
		return nil
	}); err != nil {
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
