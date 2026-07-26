package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/agh/internal/testutil"
)

func TestExecuteWrite(t *testing.T) {
	t.Run("Should retry busy begin immediate writes until the lock is released", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), "busy.db")
		locker := openExecuteWriteTestDB(t, path)
		contender := openExecuteWriteTestDB(t, path)

		if _, err := locker.ExecContext(ctx, `CREATE TABLE items (id TEXT PRIMARY KEY)`); err != nil {
			t.Fatalf("Create table error = %v", err)
		}
		lockConn, err := locker.Conn(ctx)
		if err != nil {
			t.Fatalf("locker.Conn() error = %v", err)
		}
		t.Cleanup(func() {
			if err := lockConn.Close(); err != nil {
				t.Fatalf("lockConn.Close() error = %v", err)
			}
		})
		if _, err := lockConn.ExecContext(ctx, sqliteBeginImmediateStatement); err != nil {
			t.Fatalf("BEGIN IMMEDIATE locker error = %v", err)
		}

		releaseDone := make(chan error, 1)
		timer := time.AfterFunc(10*time.Millisecond, func() {
			_, commitErr := lockConn.ExecContext(ctx, sqliteCommitStatement)
			releaseDone <- commitErr
		})
		t.Cleanup(func() {
			if timer.Stop() {
				_, err := lockConn.ExecContext(ctx, sqliteCommitStatement)
				if err != nil {
					t.Fatalf("manual lock release error = %v", err)
				}
				return
			}
			if err := <-releaseDone; err != nil {
				t.Fatalf("timed lock release error = %v", err)
			}
		})

		cfg := defaultExecuteWriteConfig()
		cfg.maxAttempts = 80
		cfg.minRetryDelay = time.Millisecond
		cfg.maxRetryDelay = time.Millisecond
		err = executeWrite(ctx, contender, cfg, func(ctx context.Context, tx *WriteTx) error {
			_, execErr := tx.ExecContext(ctx, `INSERT INTO items (id) VALUES ('ok')`)
			return execErr
		})
		if err != nil {
			t.Fatalf("executeWrite() error = %v", err)
		}

		var count int
		if err := contender.QueryRowContext(ctx, `SELECT COUNT(*) FROM items WHERE id = 'ok'`).
			Scan(&count); err != nil {
			t.Fatalf("QueryRowContext(count) error = %v", err)
		}
		if count != 1 {
			t.Fatalf("items count = %d, want 1", count)
		}
	})

	t.Run("Should roll back callback errors without committing partial writes", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openExecuteWriteTestDB(t, filepath.Join(t.TempDir(), "rollback.db"))
		if _, err := db.ExecContext(ctx, `CREATE TABLE items (id TEXT PRIMARY KEY)`); err != nil {
			t.Fatalf("Create table error = %v", err)
		}
		sentinel := errors.New("sentinel")

		err := ExecuteWrite(ctx, db, func(ctx context.Context, tx *WriteTx) error {
			if _, execErr := tx.ExecContext(ctx, `INSERT INTO items (id) VALUES ('rolled-back')`); execErr != nil {
				return execErr
			}
			return sentinel
		})
		if !errors.Is(err, sentinel) {
			t.Fatalf("ExecuteWrite() error = %v, want sentinel", err)
		}

		var count int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM items`).Scan(&count); err != nil {
			t.Fatalf("QueryRowContext(count) error = %v", err)
		}
		if count != 0 {
			t.Fatalf("items count = %d, want rollback to 0", count)
		}
	})

	t.Run("Should expose query helpers inside the active transaction", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openExecuteWriteTestDB(t, filepath.Join(t.TempDir(), "query-helpers.db"))
		if _, err := db.ExecContext(ctx, `CREATE TABLE items (id TEXT PRIMARY KEY)`); err != nil {
			t.Fatalf("Create table error = %v", err)
		}

		var capturedTx *WriteTx
		err := ExecuteWrite(ctx, db, func(ctx context.Context, tx *WriteTx) error {
			capturedTx = tx
			if _, execErr := tx.ExecContext(ctx, `INSERT INTO items (id) VALUES ('queryable')`); execErr != nil {
				return execErr
			}
			var count int
			if scanErr := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM items`).Scan(&count); scanErr != nil {
				return scanErr
			}
			if count != 1 {
				return errors.New("unexpected transaction row count")
			}
			rows, queryErr := tx.QueryContext(ctx, `SELECT id FROM items WHERE id = 'queryable'`)
			if queryErr != nil {
				return queryErr
			}
			defer func() {
				if closeErr := rows.Close(); closeErr != nil {
					t.Fatalf("rows.Close() error = %v", closeErr)
				}
			}()
			if !rows.Next() {
				return errors.New("transaction query did not return inserted row")
			}
			var id string
			if scanErr := rows.Scan(&id); scanErr != nil {
				return scanErr
			}
			if id != "queryable" {
				return errors.New("transaction query returned unexpected id")
			}
			return rows.Err()
		})
		if err != nil {
			t.Fatalf("ExecuteWrite() error = %v", err)
		}
		if _, err := capturedTx.ExecContext(ctx, `INSERT INTO items (id) VALUES ('closed')`); err == nil {
			t.Fatal("capturedTx.ExecContext() error = nil, want closed transaction error")
		}
		rows, err := capturedTx.QueryContext(ctx, `SELECT id FROM items`)
		if err == nil {
			if closeErr := rows.Close(); closeErr != nil {
				t.Fatalf("closed transaction rows.Close() error = %v", closeErr)
			}
			t.Fatal("capturedTx.QueryContext() error = nil, want closed transaction error")
		}
		var closedCount int
		if err := capturedTx.QueryRowContext(ctx, `SELECT COUNT(*) FROM items`).Scan(&closedCount); err == nil {
			t.Fatal("capturedTx.QueryRowContext().Scan() error = nil, want closed transaction error")
		}
	})

	t.Run("Should reject invalid execute write inputs", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openExecuteWriteTestDB(t, filepath.Join(t.TempDir(), "invalid-inputs.db"))
		cases := []struct {
			name string
			ctx  context.Context
			db   *sql.DB
			fn   func(context.Context, *WriteTx) error
		}{
			{
				name: "Should reject nil context",
				ctx:  nil,
				db:   db,
				fn:   func(context.Context, *WriteTx) error { return nil },
			},
			{
				name: "Should reject nil database",
				ctx:  ctx,
				db:   nil,
				fn:   func(context.Context, *WriteTx) error { return nil },
			},
			{name: "Should reject nil callback", ctx: ctx, db: db, fn: nil},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				if err := ExecuteWrite(tc.ctx, tc.db, tc.fn); err == nil {
					t.Fatal("ExecuteWrite() error = nil, want validation error")
				}
			})
		}
	})

	t.Run("Should honor canceled retry waits", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := waitForWriteRetry(ctx, time.Hour)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("waitForWriteRetry() error = %v, want context.Canceled", err)
		}
	})

	t.Run("Should bound random retry delay within configured limits", func(t *testing.T) {
		t.Parallel()

		minDelay := 20 * time.Millisecond
		maxDelay := 150 * time.Millisecond
		for range 64 {
			got := randomWriteRetryDelay(minDelay, maxDelay)
			if got < minDelay || got > maxDelay {
				t.Fatalf("randomWriteRetryDelay() = %s, want between %s and %s", got, minDelay, maxDelay)
			}
		}
		if got := randomWriteRetryDelay(maxDelay, minDelay); got != maxDelay {
			t.Fatalf("randomWriteRetryDelay(inverted) = %s, want %s", got, maxDelay)
		}
	})
}

func openExecuteWriteTestDB(t *testing.T, path string) *sql.DB {
	t.Helper()

	db, err := sql.Open(sqliteDriverName, sqliteDSN(path, "busy_timeout(1)"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	db.SetMaxOpenConns(defaultMaxOpenConns)
	db.SetMaxIdleConns(defaultMaxIdleConns)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("db.Close() error = %v", err)
		}
	})
	return db
}
