package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"time"
)

func executeSQLiteWithBusyWait(ctx context.Context, conn *sql.Conn, operation func() error) (retErr error) {
	if ctx.Done() == nil {
		return operation()
	}
	var timeoutMS int
	if err := conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&timeoutMS); err != nil {
		return err
	}
	if timeoutMS <= 0 {
		return operation()
	}
	defer func() {
		restoreCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultWriteRollbackTimeout)
		defer cancel()
		_, err := conn.ExecContext(restoreCtx, fmt.Sprintf("PRAGMA busy_timeout = %d", timeoutMS))
		if err != nil {
			retErr = discardSQLiteConnection(
				conn,
				errors.Join(retErr, fmt.Errorf("store: restore sqlite busy timeout: %w", err)),
			)
			return
		}
	}()
	if _, err := conn.ExecContext(ctx, "PRAGMA busy_timeout = 0"); err != nil {
		return err
	}

	// SQLite's native busy handler does not observe context cancellation while waiting for a writer.
	deadline := time.Now().Add(time.Duration(timeoutMS) * time.Millisecond)
	delay := time.Millisecond
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := operation()
		busy := IsSQLiteBusy(err)
		if !busy {
			return err
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return err
		}
		if err := waitForWriteRetry(ctx, min(delay, remaining)); err != nil {
			return err
		}
		delay = min(delay*2, defaultWriteMinRetryDelay)
	}
}

// beginSQLiteImmediate discards an ambiguous canceled BEGIN, which may have acquired
// a transaction even if the driver reports cancellation. Checkpoints need no such discard.
func beginSQLiteImmediate(ctx context.Context, conn *sql.Conn) error {
	operationFailed := false
	err := executeSQLiteWithBusyWait(ctx, conn, func() error {
		_, err := conn.ExecContext(ctx, sqliteBeginImmediateStatement)
		operationFailed = err != nil && !IsSQLiteBusy(err)
		return err
	})
	if operationFailed && ctx.Err() != nil {
		return discardSQLiteConnection(conn, err)
	}
	return err
}

func discardSQLiteConnection(conn *sql.Conn, cause error) error {
	err := conn.Raw(func(any) error { return driver.ErrBadConn })
	if err != nil && !errors.Is(err, driver.ErrBadConn) && !errors.Is(err, sql.ErrConnDone) {
		return errors.Join(cause, fmt.Errorf("store: discard sqlite connection: %w", err))
	}
	return cause
}
