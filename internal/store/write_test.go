package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/testutil"
	"modernc.org/sqlite"
)

func TestExecuteWrite(t *testing.T) {
	t.Run("Should cancel an external lock without waiting for the native busy timeout", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		path := filepath.Join(t.TempDir(), "cancel-external-lock.db")
		db, err := OpenSQLiteDatabase(ctx, path, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(); err != nil {
				t.Error(err)
			}
		})
		db.SetMaxOpenConns(1)
		if _, err := db.ExecContext(ctx, `CREATE TABLE items (id TEXT PRIMARY KEY)`); err != nil {
			t.Fatal(err)
		}
		locker := openExecuteWriteTestDB(t, path)
		conn, err := locker.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := conn.Close(); err != nil {
				t.Error(err)
			}
		})
		if _, err := conn.ExecContext(ctx, sqliteBeginImmediateStatement); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if _, err := conn.ExecContext(testutil.Context(t), sqliteRollbackStatement); err != nil {
				t.Error(err)
			}
		}()
		waitCtx, cancel := context.WithTimeout(ctx, 25*time.Millisecond)
		defer cancel()
		started := time.Now()
		err = ExecuteWrite(waitCtx, db, func(context.Context, *WriteTx) error {
			t.Error("blocked write callback must not execute")
			return nil
		})
		elapsed := time.Since(started)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("write error = %v, want deadline exceeded", err)
		}
		if elapsed >= time.Second {
			t.Errorf("canceled writer waited %s for a native busy timeout", elapsed)
		}
		var busyTimeout, count int
		if err := db.QueryRowContext(ctx, `PRAGMA busy_timeout`).Scan(&busyTimeout); err != nil {
			t.Fatal(err)
		}
		if busyTimeout != DefaultSQLiteBusyTimeoutMS {
			t.Errorf("busy timeout after cancellation = %d, want %d", busyTimeout, DefaultSQLiteBusyTimeoutMS)
		}
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM items`).Scan(&count); err != nil || count != 0 {
			t.Fatalf("rows after cancellation = %d, error = %v, want zero", count, err)
		}
	})

	t.Run("Should discard a transaction connection after cleanup failure", func(t *testing.T) {
		for _, tc := range []struct {
			name          string
			statement     string
			callbackError bool
		}{
			{name: "Should discard a failed timeout restoration", statement: "PRAGMA busy_timeout = 5000"},
			{name: "Should discard a failed rollback", statement: sqliteRollbackStatement, callbackError: true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				ctx := t.Context()
				injected := errors.New("injected connection cleanup failure")
				db := sql.OpenDB(&writeCleanupFailureConnector{
					dsn:       sqliteDSN(filepath.Join(t.TempDir(), "cleanup.db")),
					statement: tc.statement,
					failure:   injected,
				})
				db.SetMaxOpenConns(1)
				t.Cleanup(func() {
					if err := db.Close(); err != nil {
						t.Error(err)
					}
				})
				if _, err := db.ExecContext(ctx, `CREATE TABLE items (id TEXT PRIMARY KEY)`); err != nil {
					t.Fatal(err)
				}
				err := ExecuteWrite(ctx, db, func(ctx context.Context, tx *WriteTx) error {
					if _, err := tx.ExecContext(ctx, `INSERT INTO items VALUES ('partial')`); err != nil {
						return err
					}
					if tc.callbackError {
						return errors.New("reject partial write")
					}
					return nil
				})
				if !errors.Is(err, injected) {
					t.Fatalf("write error = %v, want cleanup failure", err)
				}
				var count int
				if err := db.QueryRowContext(ctx, `SELECT count(*) FROM items`).Scan(&count); err != nil || count != 0 {
					t.Fatalf("rows after cleanup failure = %d, error = %v, want zero", count, err)
				}
				if err := ExecuteWrite(ctx, db, func(ctx context.Context, tx *WriteTx) error {
					_, err := tx.ExecContext(ctx, `INSERT INTO items VALUES ('recovered')`)
					return err
				}); err != nil {
					t.Fatalf("write after failed cleanup = %v", err)
				}
			})
		}
	})

	t.Run("Should preserve committed success when the caller cancels during commit", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		db := sql.OpenDB(&commitCancellationConnector{
			sqliteDriver: &sqlite.Driver{},
			dsn:          sqliteDSN(filepath.Join(t.TempDir(), "commit-cancellation.db")),
			cancel:       cancel,
		})
		t.Cleanup(func() {
			if err := db.Close(); err != nil {
				t.Error(err)
			}
		})
		if _, err := db.ExecContext(ctx, `CREATE TABLE items (id TEXT PRIMARY KEY)`); err != nil {
			t.Fatal(err)
		}
		err := ExecuteWrite(ctx, db, func(ctx context.Context, tx *WriteTx) error {
			_, err := tx.ExecContext(ctx, `INSERT INTO items VALUES ('committed')`)
			return err
		})
		if err != nil {
			t.Fatalf("committed write returned a retryable failure: %v", err)
		}
		if !errors.Is(ctx.Err(), context.Canceled) {
			t.Fatal("caller was not canceled during commit")
		}
		var count int
		if err := db.QueryRowContext(t.Context(), `SELECT count(*) FROM items`).Scan(&count); err != nil || count != 1 {
			t.Fatalf("committed rows = %d, error = %v, want one", count, err)
		}
	})

	t.Run("Should roll back cancellation observed at the commit fence", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		db := openExecuteWriteTestDB(t, filepath.Join(t.TempDir(), "cancel-before-commit.db"))
		if _, err := db.ExecContext(ctx, `CREATE TABLE items (id TEXT PRIMARY KEY)`); err != nil {
			t.Fatal(err)
		}
		ctx = ContextWithMutationCommitFence(ctx, func(context.Context) error {
			cancel()
			return nil
		})
		err := ExecuteWrite(ctx, db, func(ctx context.Context, tx *WriteTx) error {
			_, err := tx.ExecContext(ctx, `INSERT INTO items VALUES ('rolled-back')`)
			return err
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("write error = %v, want cancellation", err)
		}
		var count int
		if err := db.QueryRowContext(t.Context(), `SELECT count(*) FROM items`).Scan(&count); err != nil || count != 0 {
			t.Fatalf("rows after cancellation = %d, error = %v, want zero", count, err)
		}
	})

	t.Run("Should reject same database reentry and roll back the outer write", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		db := openExecuteWriteTestDB(t, filepath.Join(t.TempDir(), "reentry.db"))
		if _, err := db.ExecContext(ctx, `CREATE TABLE items (id TEXT PRIMARY KEY)`); err != nil {
			t.Fatal(err)
		}
		err := ExecuteWrite(ctx, db, func(ctx context.Context, tx *WriteTx) error {
			if _, err := tx.ExecContext(ctx, `INSERT INTO items VALUES ('outer')`); err != nil {
				return err
			}
			return ExecuteWrite(ctx, db, func(context.Context, *WriteTx) error {
				t.Error("reentrant callback must not execute")
				return nil
			})
		})
		if !errors.Is(err, errWriteReentry) || errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("same database reentry error = %v", err)
		}
		var count int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM items`).Scan(&count); err != nil || count != 0 {
			t.Fatalf("rows after rejected nested write = %d, error = %v", count, err)
		}
		if err := ExecuteWrite(ctx, db, func(ctx context.Context, tx *WriteTx) error {
			_, err := tx.ExecContext(ctx, `INSERT INTO items VALUES ('recovered')`)
			return err
		}); err != nil {
			t.Fatalf("write after rejected reentry: %v", err)
		}
	})

	t.Run("Should permit nested writes on another database and reuse completed callback contexts", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		db := openExecuteWriteTestDB(t, filepath.Join(t.TempDir(), "outer.db"))
		otherDB := openExecuteWriteTestDB(t, filepath.Join(t.TempDir(), "inner.db"))
		if _, err := otherDB.ExecContext(ctx, `CREATE TABLE items (id TEXT PRIMARY KEY)`); err != nil {
			t.Fatal(err)
		}
		var completedCtx context.Context
		err := ExecuteWrite(ctx, db, func(ctx context.Context, _ *WriteTx) error {
			completedCtx = ctx
			return ExecuteWrite(ctx, otherDB, func(ctx context.Context, tx *WriteTx) error {
				if _, err := tx.ExecContext(ctx, `INSERT INTO items VALUES ('inner')`); err != nil {
					return err
				}
				err := ExecuteWrite(ctx, db, func(context.Context, *WriteTx) error {
					t.Error("ancestor database reentrant callback must not execute")
					return nil
				})
				if !errors.Is(err, errWriteReentry) {
					return errors.New("ancestor database reentry was not rejected")
				}
				return nil
			})
		})
		if err != nil {
			t.Fatal(err)
		}
		var count int
		if err := otherDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM items`).Scan(&count); err != nil || count != 1 {
			t.Fatalf("committed nested rows = %d, error = %v", count, err)
		}
		if err := ExecuteWrite(completedCtx, db, func(context.Context, *WriteTx) error { return nil }); err != nil {
			t.Fatalf("write using completed callback context: %v", err)
		}
	})

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
			done <- ExecuteWriteOperation(ctx, db, "session reconciliation", func(ctx context.Context, _ *WriteTx) error {
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
		for {
			writeAdmissions.Lock()
			users := writeAdmissions.byDB[db].users
			writeAdmissions.Unlock()
			if users == 2 {
				break
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
				if _, err := conn.ExecContext(testutil.Context(t), sqliteRollbackStatement); err != nil {
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
			releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 45*time.Second)
			defer cancel()
			_, commitErr := lockConn.ExecContext(releaseCtx, sqliteCommitStatement)
			releaseDone <- commitErr
		})
		t.Cleanup(func() {
			if timer.Stop() {
				_, err := lockConn.ExecContext(testutil.Context(t), sqliteCommitStatement)
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

		ctx, cancel := context.WithCancel(t.Context())
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

	dsn, err := url.Parse(sqliteDSN(path))
	if err != nil {
		t.Fatal(err)
	}
	query := dsn.Query()
	for index, pragma := range query["_pragma"] {
		if strings.HasPrefix(pragma, "busy_timeout(") {
			query["_pragma"][index] = "busy_timeout(1)"
		}
	}
	dsn.RawQuery = query.Encode()
	db, err := sql.Open(sqliteDriverName, dsn.String())
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

// The driver boundary cancels after SQLite commits, reproducing an ambiguous ExecContext result.
type commitCancellationConnector struct {
	sqliteDriver *sqlite.Driver
	dsn          string
	cancel       context.CancelFunc
}

func (c *commitCancellationConnector) Connect(context.Context) (driver.Conn, error) {
	conn, err := c.sqliteDriver.Open(c.dsn)
	if err != nil {
		return nil, err
	}
	return &commitCancellationConn{Conn: conn, cancel: c.cancel}, nil
}

func (c *commitCancellationConnector) Driver() driver.Driver { return c.sqliteDriver }

type commitCancellationConn struct {
	driver.Conn
	cancel context.CancelFunc
}

func (c *commitCancellationConn) ExecContext(
	ctx context.Context,
	query string,
	args []driver.NamedValue,
) (driver.Result, error) {
	result, err := c.Conn.(driver.ExecerContext).ExecContext(ctx, query, args)
	if err == nil && query == sqliteCommitStatement {
		c.cancel()
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	return result, err
}

func BenchmarkExecuteWrite(b *testing.B) {
	ctx := b.Context()
	db, err := OpenSQLiteDatabase(ctx, filepath.Join(b.TempDir(), "write.db"), nil)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := db.Close(); err != nil {
			b.Error(err)
		}
	})
	if _, err := db.ExecContext(ctx, `CREATE TABLE items (value INTEGER); INSERT INTO items VALUES (0)`); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if err := ExecuteWrite(ctx, db, func(ctx context.Context, tx *WriteTx) error {
			_, err := tx.ExecContext(ctx, `UPDATE items SET value = value + 1`)
			return err
		}); err != nil {
			b.Fatal(err)
		}
	}
}

type writeCleanupFailureConnector struct {
	dsn       string
	statement string
	failure   error
	failed    atomic.Bool
}

func (c *writeCleanupFailureConnector) Connect(context.Context) (driver.Conn, error) {
	conn, err := (&sqlite.Driver{}).Open(c.dsn)
	if err != nil {
		return nil, err
	}
	return &writeCleanupFailureConn{Conn: conn, connector: c}, nil
}

func (*writeCleanupFailureConnector) Driver() driver.Driver { return &sqlite.Driver{} }

type writeCleanupFailureConn struct {
	driver.Conn
	connector *writeCleanupFailureConnector
}

func (c *writeCleanupFailureConn) ExecContext(
	ctx context.Context,
	query string,
	args []driver.NamedValue,
) (driver.Result, error) {
	if query == c.connector.statement && c.connector.failed.CompareAndSwap(false, true) {
		return nil, c.connector.failure
	}
	return c.Conn.(driver.ExecerContext).ExecContext(ctx, query, args)
}
