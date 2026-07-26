package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/store"
)

type networkSQLExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

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

func (g *NetworkRepo) withNetworkImmediateTransaction(
	ctx context.Context,
	action string,
	run func(exec networkSQLExecutor) error,
) (err error) {
	return g.withImmediateTransaction(ctx, action, func(exec globalSQLExecutor) error {
		return run(exec)
	})
}

func (g *NetworkRepo) withNetworkReadTransaction(
	ctx context.Context,
	action string,
	run func(exec networkSQLExecutor) error,
) (err error) {
	tx, err := g.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("store: begin %s transaction: %w", action, err)
	}

	finished := false
	defer func() {
		if finished {
			return
		}
		rollbackErr := tx.Rollback()
		if rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			joinCleanupError(&err, fmt.Errorf("store: rollback %s transaction: %w", action, rollbackErr))
		}
	}()

	if err := run(tx); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("store: commit %s transaction: %w", action, err)
	}
	finished = true
	return nil
}

func (r *repoBase) withImmediateTransaction(
	ctx context.Context,
	action string,
	run func(exec globalSQLExecutor) error,
) error {
	if r == nil || r.db == nil {
		return errors.New("store: repository is required")
	}
	if err := store.ExecuteWrite(ctx, r.db, func(_ context.Context, tx *store.WriteTx) error {
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
