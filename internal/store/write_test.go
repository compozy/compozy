package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/testutil"
)

func TestExecuteWrite(t *testing.T) {
	t.Run("Should cancel local writer admission without spending SQLite attempts", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db := openExecuteWriteTestDB(t, filepath.Join(t.TempDir(), "owner-diagnostic.db"))
		started, release := make(chan struct{}), make(chan struct{})
		done := make(chan error, 1)
		go func() {
			done <- ExecuteWriteOperation(ctx, db, "session supervision", func(context.Context, *WriteTx) error {
				close(started)
				<-release
				return nil
			})
		}()
		<-started
		finished := false
		defer func() {
			if finished {
				return
			}
			close(release)
			if err := <-done; err != nil {
				t.Error(err)
			}
		}()
		cfg := defaultExecuteWriteConfig()
		cfg.operation = "skill event summary"
		cfg.maxAttempts = 3
		cfg.jitter = func(time.Duration, time.Duration) time.Duration { return time.Millisecond }
		waitCtx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
		defer cancel()
		err := executeWrite(waitCtx, db, cfg, func(context.Context, *WriteTx) error {
			t.Error("blocked write callback must not execute")
			return nil
		})
		contention, ok := errors.AsType[*WriteContentionError](err)
		if !ok || contention.Operation != cfg.operation || contention.Owner != "session supervision" ||
			contention.TransactionID == 0 || contention.Attempts != 0 || !errors.Is(err, context.DeadlineExceeded) ||
			contention.Wait <= 0 || contention.OwnerElapsed <= 0 {
			t.Fatalf("contention = %#v, error = %v", contention, err)
		}
		if inUse := db.Stats().InUse; inUse != 1 {
			t.Fatalf("pooled connections while canceled waiter leaves owner active = %d, want 1", inUse)
		}
		writeAdmissions.Lock()
		remainingUsers := writeAdmissions.byDB[db].users
		writeAdmissions.Unlock()
		if remainingUsers != 1 {
			t.Fatalf("remaining admission users = %d, want only owner", remainingUsers)
		}
		close(release)
		ownerErr := <-done
		finished = true
		if ownerErr != nil {
			t.Fatal(ownerErr)
		}
		writeAdmissions.Lock()
		_, admissionRemains := writeAdmissions.byDB[db]
		writeAdmissions.Unlock()
		writeOwners.Lock()
		_, ownerRemains := writeOwners.active[db]
		writeOwners.Unlock()
		if admissionRemains || ownerRemains {
			t.Fatalf("registry after all writes finish = admission %t, owner %t", admissionRemains, ownerRemains)
		}
	})

	t.Run("Should persist after a local holder outlives the SQLite retry budget", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db := openExecuteWriteTestDB(t, filepath.Join(t.TempDir(), "local-wait.db"))
		db.SetMaxOpenConns(2)
		if _, err := db.ExecContext(ctx, `CREATE TABLE items (id TEXT PRIMARY KEY)`); err != nil {
			t.Fatal(err)
		}
		started, release := make(chan struct{}), make(chan struct{})
		done := make(chan error, 1)
		go func() {
			done <- ExecuteWriteOperation(ctx, db, "session reconciliation", func(ctx context.Context, tx *WriteTx) error {
				close(started)
				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			})
		}()
		<-started
		// One SQLite attempt permits no retry waits. The local holder lasts
		// beyond that budget but must not cause the durable receipt write to fail.
		cfg := defaultExecuteWriteConfig()
		cfg.maxAttempts = 1
		cfg.operation = "finish worktree exit operation with event"
		writeDone := make(chan error, 1)
		go func() {
			writeDone <- executeWrite(ctx, db, cfg, func(ctx context.Context, tx *WriteTx) error {
				_, err := tx.ExecContext(ctx, `INSERT INTO items VALUES ('terminal-receipt')`)
				return err
			})
		}()
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
	waitQueued:
		for {
			writeAdmissions.Lock()
			users := writeAdmissions.byDB[db].users
			writeAdmissions.Unlock()
			if users == 2 {
				break waitQueued
			}
			select {
			case err := <-writeDone:
				close(release)
				ownerErr := <-done
				t.Fatalf("writer finished before local admission wait: %v, owner %v", err, ownerErr)
			case <-ctx.Done():
				close(release)
				writerErr, ownerErr := <-writeDone, <-done
				t.Fatalf("waiting for queued writer: %v, writer %v, owner %v", ctx.Err(), writerErr, ownerErr)
			case <-ticker.C:
			}
		}
		connections := db.Stats().InUse
		// Registration is observed before the holder's expiry starts, so the
		// test exercises an actually queued writer rather than scheduling luck.
		timer := time.NewTimer(20 * time.Millisecond)
		select {
		case <-timer.C:
		case <-ctx.Done():
		}
		timer.Stop()
		close(release)
		err, ownerErr := <-writeDone, <-done
		if err != nil || ownerErr != nil || connections != 1 {
			t.Fatalf("receipt write = %v, reconciliation = %v, queued connections = %d, want 1",
				err, ownerErr, connections)
		}
		var count int
		err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM items WHERE id = 'terminal-receipt'`).Scan(&count)
		if err != nil || count != 1 {
			t.Fatalf("durable terminal receipts = %d, error = %v, want 1", count, err)
		}
	})

	t.Run("Should bound an external busy lock and recover on the same handle", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		path := filepath.Join(t.TempDir(), "external-lock.db")
		locker, db := openExecuteWriteTestDB(t, path), openExecuteWriteTestDB(t, path)
		conn, err := locker.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		active := false
		t.Cleanup(func() {
			if active {
				if _, err := conn.ExecContext(ctx, sqliteRollbackStatement); err != nil {
					t.Error(err)
				}
			}
			if err := conn.Close(); err != nil {
				t.Error(err)
			}
		})
		if _, err := conn.ExecContext(ctx, sqliteBeginImmediateStatement); err != nil {
			t.Fatal(err)
		}
		active = true
		cfg := defaultExecuteWriteConfig()
		cfg.operation = "skill event summary"
		cfg.maxAttempts = 3
		cfg.jitter = func(time.Duration, time.Duration) time.Duration { return time.Millisecond }
		err = executeWrite(ctx, db, cfg, func(context.Context, *WriteTx) error { return nil })
		contention, ok := errors.AsType[*WriteContentionError](err)
		if !ok || !IsSQLiteBusy(err) || contention.Attempts != 3 || contention.TransactionID != 0 ||
			contention.Owner != "" {
			t.Fatalf("external contention = %#v, error = %v", contention, err)
		}
		if _, err := conn.ExecContext(ctx, sqliteCommitStatement); err != nil {
			t.Fatal(err)
		}
		active = false
		if err := ExecuteWrite(ctx, db, func(context.Context, *WriteTx) error { return nil }); err != nil {
			t.Fatalf("recovery after lock release = %v", err)
		}
	})

	t.Run("Should reserve pool capacity for the active writer commit fence", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db, err := sql.Open(
			sqliteDriverName,
			sqliteDSN(filepath.Join(t.TempDir(), "writer-pool.db"), "busy_timeout(5000)"),
		)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(); err != nil {
				t.Error(err)
			}
		})
		db.SetMaxOpenConns(2)
		if _, err := db.ExecContext(ctx, `CREATE TABLE items (id TEXT PRIMARY KEY)`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `PRAGMA busy_timeout = 5000`); err != nil {
			t.Fatal(err)
		}
		ownerStarted := make(chan struct{})
		readReady := make(chan struct{}, 1)
		ownerDone := make(chan error, 1)
		fencedCtx := ContextWithMutationCommitFence(ctx, func(ctx context.Context) error {
			readCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
			defer cancel()
			var count int
			return db.QueryRowContext(readCtx, `SELECT COUNT(*) FROM items`).Scan(&count)
		})
		go func() {
			ownerDone <- ExecuteWrite(fencedCtx, db, func(ctx context.Context, tx *WriteTx) error {
				close(ownerStarted)
				select {
				case <-readReady:
				case <-ctx.Done():
					return ctx.Err()
				}
				_, err := tx.ExecContext(ctx, `INSERT INTO items VALUES ('owner')`)
				return err
			})
		}()
		<-ownerStarted
		contenderDone := make(chan error, 1)
		cfg := defaultExecuteWriteConfig()

		go func() {
			contenderDone <- executeWrite(ctx, db, cfg, func(ctx context.Context, tx *WriteTx) error {
				_, err := tx.ExecContext(ctx, `INSERT INTO items VALUES ('contender')`)
				return err
			})
		}()
		// A waiting BEGIN occupying the final connection reproduces the old
		// starvation. Local admission instead waits without a connection.
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		var ownerErr error
		readSignaled := false
	waitOwner:
		for {
			select {
			case ownerErr = <-ownerDone:
				break waitOwner
			case <-ticker.C:
				writeAdmissions.Lock()
				users := 0
				if admission := writeAdmissions.byDB[db]; admission != nil {
					users = admission.users
				}
				writeAdmissions.Unlock()
				if users == 2 && !readSignaled {
					if inUse := db.Stats().InUse; inUse != 1 {
						t.Errorf("queued writer pooled connections = %d, want only the owner connection", inUse)
					}
					select {
					case readReady <- struct{}{}:
						readSignaled = true
					default:
					}
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		}
		contenderErr := <-contenderDone
		if ownerErr != nil || contenderErr != nil {
			t.Fatalf("concurrent writes = owner %v, contender %v", ownerErr, contenderErr)
		}
		var count int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM items`).Scan(&count); err != nil || count != 2 {
			t.Fatalf("committed count = %d, error = %v, want 2", count, err)
		}
	})

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

	t.Run("Should run the mutation fence before commit and roll back its rejection", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openExecuteWriteTestDB(t, filepath.Join(t.TempDir(), "mutation-fence.db"))
		if _, err := db.ExecContext(ctx, `CREATE TABLE items (id TEXT PRIMARY KEY)`); err != nil {
			t.Fatalf("Create table error = %v", err)
		}
		sentinel := errors.New("mutation fence rejected commit")
		fenceCalls := 0
		ctx = ContextWithMutationCommitFence(ctx, func(context.Context) error {
			fenceCalls++
			return sentinel
		})

		err := ExecuteWrite(ctx, db, func(ctx context.Context, tx *WriteTx) error {
			_, execErr := tx.ExecContext(ctx, `INSERT INTO items (id) VALUES ('rolled-back')`)
			return execErr
		})
		if !errors.Is(err, sentinel) {
			t.Fatalf("ExecuteWrite() error = %v, want mutation fence rejection", err)
		}
		if fenceCalls != 1 {
			t.Fatalf("mutation fence calls = %d, want 1", fenceCalls)
		}
		var count int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM items`).Scan(&count); err != nil {
			t.Fatalf("QueryRowContext(count) error = %v", err)
		}
		if count != 0 {
			t.Fatalf("items count = %d, want fenced rollback to 0", count)
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
