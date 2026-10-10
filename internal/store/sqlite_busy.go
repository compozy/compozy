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
	operationFailed := false
	defer func() {
		if operationFailed && ctx.Err() != nil {
			// Driver cancellation can mask a successful BEGIN; closing the connection rolls it back.
			retErr = discardSQLiteConnection(conn, retErr)
		}
	}()
	var timeoutMS int
	if err := conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&timeoutMS); err != nil {
		return err
	}
	if timeoutMS <= 0 {
		err := operation()
		operationFailed = err != nil && !IsSQLiteBusy(err)
		return err
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
		operationFailed = err != nil && !busy
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

func discardSQLiteConnection(conn *sql.Conn, cause error) error {
	err := conn.Raw(func(any) error { return driver.ErrBadConn })
	if err != nil && !errors.Is(err, driver.ErrBadConn) && !errors.Is(err, sql.ErrConnDone) {
		return errors.Join(cause, fmt.Errorf("store: discard sqlite connection: %w", err))
	}
	return cause
}
