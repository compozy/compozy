package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

type globalSQLExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func rollbackTx(tx *sql.Tx, action string) error {
	if tx == nil {
		return nil
	}
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return fmt.Errorf("store: rollback %s transaction: %w", action, err)
	}
	return nil
}

func restoreForeignKeys(ctx context.Context, conn *sql.Conn) error {
	if conn == nil {
		return nil
	}
	// dynamic-sql: SQLite connection configuration is not a schema query owned by sqlc.
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("store: restore sqlite foreign keys: %w", err)
	}
	return nil
}

func joinCleanupError(target *error, cleanupErr error) {
	if cleanupErr == nil || target == nil {
		return
	}
	if *target == nil {
		*target = cleanupErr
		return
	}
	*target = errors.Join(*target, cleanupErr)
}

func (r *repoBase) withImmediateTransaction(
	ctx context.Context,
	action string,
	run func(exec globalSQLExecutor) error,
) error {
	if r == nil || r.db == nil {
		return errors.New("store: repository is required")
	}
	if err := store.ExecuteWriteOperation(ctx, r.db, action, func(_ context.Context, tx *store.WriteTx) error {
		return run(tx)
	}); err != nil {
		return fmt.Errorf("store: %s transaction: %w", action, err)
	}
	return nil
}

func (r *repoBase) withTaskImmediateTransaction(
	ctx context.Context,
	action string,
	run func(exec taskSQLExecutor) error,
) error {
	if r == nil || r.tasks == nil {
		return errors.New("store: task repository is required")
	}
	return r.tasks.withTaskImmediateTransaction(ctx, action, run)
}
