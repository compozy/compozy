package store

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/testutil"
	"modernc.org/sqlite"
)

func TestStoreSQLHelpers(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)
	where, args := BuildClauses(
		StringClause("type", " agent_message "),
		StringClause("ignored", "   "),
		TimeClause("timestamp", ">=", now),
		TimeClause("timestamp", ">=", time.Time{}),
		Int64Clause("sequence", ">", 3),
		Int64Clause("sequence", ">", 0),
	)

	if got, want := NormalizeSessionType("   "), defaultSessionType; got != want {
		t.Fatalf("NormalizeSessionType(blank) = %q, want %q", got, want)
	}
	if got := NormalizeSessionType(" dream "); got != "dream" {
		t.Fatalf("NormalizeSessionType(value) = %q, want dream", got)
	}

	if got, want := len(where), 3; got != want {
		t.Fatalf("len(where) = %d, want %d (%v)", got, want, where)
	}
	if got, want := len(args), 3; got != want {
		t.Fatalf("len(args) = %d, want %d (%v)", got, want, args)
	}

	query := AppendWhere("SELECT * FROM events", where)
	if !strings.Contains(query, "WHERE type = ? AND timestamp >= ? AND sequence > ?") {
		t.Fatalf("AppendWhere() = %q", query)
	}

	invalidWhere, invalidArgs := BuildClauses(
		StringClause("bad-name", "value"),
		TimeClause("timestamp", "DROP TABLE", now),
		Int64Clause("sequence", "DROP TABLE", 3),
	)
	if got, want := invalidWhere, []string{"1 = 0", "1 = 0", "1 = 0"}; !testutil.EqualStringSlices(got, want) {
		t.Fatalf("invalid where = %#v, want %#v", got, want)
	}
	if got, want := len(invalidArgs), 0; got != want {
		t.Fatalf("len(invalidArgs) = %d, want %d", got, want)
	}
	t.Run("Should accept qualified column references", func(t *testing.T) {
		t.Parallel()

		qualifiedWhere, qualifiedArgs := BuildClauses(StringClause("jobs.profile_id", "profile-a"))
		if got, want := qualifiedWhere, []string{"jobs.profile_id = ?"}; !testutil.EqualStringSlices(got, want) {
			t.Fatalf("qualified where = %#v, want %#v", got, want)
		}
		if got, want := qualifiedArgs, []any{"profile-a"}; len(got) != 1 || got[0] != want[0] {
			t.Fatalf("qualified args = %#v, want %#v", got, want)
		}
	})
	t.Run("Should reject injected qualified column references without retaining arguments", func(t *testing.T) {
		t.Parallel()

		unsafeWhere, unsafeArgs := BuildClauses(StringClause("jobs.profile_id OR 1=1", "profile-a"))
		if got, want := unsafeWhere, []string{"1 = 0"}; !testutil.EqualStringSlices(got, want) {
			t.Fatalf("unsafe qualified where = %#v, want %#v", got, want)
		}
		if len(unsafeArgs) != 0 {
			t.Fatalf("unsafe qualified args = %#v, want none", unsafeArgs)
		}
	})

	limitedQuery, limitedArgs := AppendLimit(query, args, 5)
	if !strings.HasSuffix(limitedQuery, " LIMIT ?") {
		t.Fatalf("AppendLimit() query = %q", limitedQuery)
	}
	if got, want := limitedArgs[len(limitedArgs)-1], any(5); got != want {
		t.Fatalf("AppendLimit() last arg = %#v, want %#v", got, want)
	}
	if got, want := AppendWhere("SELECT 1", nil), "SELECT 1"; got != want {
		t.Fatalf("AppendWhere(no clauses) = %q, want %q", got, want)
	}
	composed, composedArgs := BuildWhereQuery(
		"SELECT * FROM events",
		" ORDER BY sequence",
		StringClause("type", "agent_message"),
	)
	if composed != "SELECT * FROM events WHERE type = ? ORDER BY sequence" ||
		len(composedArgs) != 1 || composedArgs[0] != "agent_message" {
		t.Fatalf("BuildWhereQuery() = %q, %#v", composed, composedArgs)
	}
	coalescedWhere, coalescedArgs := BuildClauses(ReadScopeCoalescedClause(
		[]string{"thread.profile_id", "session.profile_id"},
		ReadScope{ProfileID: "profile-a"},
	))
	if got, want := coalescedWhere, []string{
		"COALESCE(thread.profile_id, session.profile_id) = ?",
	}; !testutil.EqualStringSlices(got, want) || len(coalescedArgs) != 1 ||
		coalescedArgs[0] != "profile-a" {
		t.Fatalf("ReadScopeCoalescedClause() = %#v, %#v", coalescedWhere, coalescedArgs)
	}
	if gotQuery, gotArgs := AppendLimit("SELECT 1", nil, 0); gotQuery != "SELECT 1" || gotArgs != nil {
		t.Fatalf("AppendLimit(no limit) = (%q, %#v), want (%q, nil)", gotQuery, gotArgs, "SELECT 1")
	}
	if got := SQLNullString("   "); got.Valid {
		t.Fatalf("SQLNullString(blank) = %#v, want invalid", got)
	}
	if got := SQLNullString(" value "); !got.Valid || got.String != "value" {
		t.Fatalf("SQLNullString(value) = %#v, want trimmed valid value", got)
	}
	if got := SQLNullStringPointer(nil); got.Valid {
		t.Fatalf("SQLNullStringPointer(nil) = %#v, want invalid", got)
	}
}

func TestStoreSQLiteHelpers(t *testing.T) {
	t.Parallel()

	// Invariant: checkpointing leaves WAL-header restarts to maintenance, preserving
	// backfill and user state. Owner: SQLite lifecycle; suite: TestStoreSQLiteHelpers.
	t.Run("Should keep WAL restarts off foreground commits", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), "restart.db")
		db, err := OpenSQLiteDatabaseWithPragmas(ctx, path, []string{"wal_autocheckpoint(0)"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(); err != nil {
				t.Error(err)
			}
		})
		if _, err := db.ExecContext(ctx, "CREATE TABLE items (value INTEGER); PRAGMA user_version = 42"); err != nil {
			t.Fatal(err)
		}
		checkpointDB, err := openSQLiteHandle(ctx, path, sqliteCheckpointDSN(path))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := checkpointDB.Close(); err != nil {
				t.Error(err)
			}
		})
		salt := func() string {
			t.Helper()
			data, err := os.ReadFile(path + "-wal")
			if err != nil || len(data) < 32 {
				t.Fatalf("read WAL header: bytes=%d, error=%v", len(data), err)
			}
			return string(data[16:24])
		}
		before := salt()
		if err := CheckpointPassive(ctx, checkpointDB); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, "INSERT INTO items VALUES (0)"); err != nil {
			t.Fatal(err)
		}
		if salt() == before {
			t.Fatal("control: full passive backfill did not restart the next foreground WAL")
		}
		guard, err := openWALRestartGuard(ctx, ctx, checkpointDB, path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := guard.close(); err != nil {
				t.Error(err)
			}
		})
		admission, unregister := registerWriteAdmission(db)
		defer unregister()
		release, err := admission.acquire(ctx, false)
		if err != nil {
			t.Fatal(err)
		}
		releaseOnce := sync.OnceFunc(release)
		defer releaseOnce()
		before = salt()
		copiedBefore := 0
		for range 3 {
			if _, err := db.ExecContext(
				ctx,
				strings.Repeat("INSERT INTO items VALUES (1);", walRestartFrames),
			); err != nil {
				t.Fatal(err)
			}
			if err := guard.checkpoint(ctx, checkpointDB); err != nil {
				t.Fatal(err)
			}
			frames, copied, err := walCheckpointProgress(ctx, checkpointDB)
			if err != nil || copied <= copiedBefore || copied != frames {
				t.Fatalf("contended backfill = %d/%d, previous=%d, error=%v", copied, frames, copiedBefore, err)
			}
			copiedBefore = copied
		}
		if salt() != before {
			t.Fatal("maintenance restarted while another writer owned admission")
		}
		releaseOnce()
		for range 5 {
			if _, err := db.ExecContext(
				ctx,
				strings.Repeat("INSERT INTO items VALUES (1);", walRestartFrames),
			); err != nil {
				t.Fatal(err)
			}
			before = salt()
			if err := guard.checkpoint(ctx, checkpointDB); err != nil {
				t.Fatal(err)
			}
			if salt() == before {
				t.Fatal("maintenance did not restart the WAL; continuous writes would grow it indefinitely")
			}
			before = salt()
			// A full backfill with the guard held must still not let the writer restart.
			if err := CheckpointPassive(ctx, checkpointDB); err != nil {
				t.Fatal(err)
			}
			var busy, frames, copied int
			if err := checkpointDB.QueryRowContext(ctx, "PRAGMA wal_checkpoint(NOOP)").
				Scan(&busy, &frames, &copied); err != nil {
				t.Fatal(err)
			}
			if frames == 0 || copied != frames {
				t.Fatalf("checkpoint copied %d/%d frames, want complete backfill", copied, frames)
			}
			if _, err := db.ExecContext(ctx, "INSERT INTO items VALUES (1)"); err != nil {
				t.Fatal(err)
			}
			if salt() != before {
				t.Fatal("foreground commit restarted the WAL after maintenance")
			}
		}
		before = salt()
		framesBefore, _, err := walCheckpointProgress(ctx, checkpointDB)
		if err != nil {
			t.Fatal(err)
		}
		for range 3 {
			if err := guard.checkpoint(ctx, checkpointDB); err != nil {
				t.Fatal(err)
			}
		}
		framesAfter, _, err := walCheckpointProgress(ctx, checkpointDB)
		if err != nil || framesAfter != framesBefore || salt() != before {
			t.Fatalf("idle maintenance changed WAL: frames %d -> %d, error=%v", framesBefore, framesAfter, err)
		}
		var count, version int
		wantCount := 6 + 8*walRestartFrames
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM items").Scan(&count); err != nil || count != wantCount {
			t.Fatalf("rows = %d, error = %v, want %d", count, err, wantCount)
		}
		if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil || version != 42 {
			t.Fatalf("user_version = %d, error = %v, want 42", version, err)
		}
	})

	t.Run("Should recover WAL maintenance from busy startup and an external writer", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), "restart-retry.db")
		db := openExecuteWriteTestDB(t, path)
		if _, err := db.ExecContext(ctx, "CREATE TABLE items (value INTEGER)"); err != nil {
			t.Fatal(err)
		}
		checkpointDB, err := openSQLiteHandle(ctx, path, sqliteCheckpointDSN(path))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := checkpointDB.Close(); err != nil {
				t.Error(err)
			}
		})
		admission, unregister := registerWriteAdmission(db)
		defer unregister()
		release, err := admission.acquire(ctx, false)
		if err != nil {
			t.Fatal(err)
		}
		release = sync.OnceFunc(release)
		defer release()
		guard, err := openWALRestartGuard(ctx, ctx, checkpointDB, path)
		if err != nil {
			t.Fatalf("open during admission contention = %v", err)
		}
		t.Cleanup(func() {
			if err := guard.close(); err != nil {
				t.Error(err)
			}
		})
		release()
		if err := guard.checkpoint(ctx, checkpointDB); err != nil {
			t.Fatal(err)
		}
		writer, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := writer.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				t.Error(err)
			}
		})
		if _, err := writer.ExecContext(ctx, "INSERT INTO items VALUES (1)"); err != nil {
			t.Fatal(err)
		}
		if err := guard.restart(ctx, checkpointDB); !IsSQLiteBusy(err) {
			t.Fatalf("maintenance during external writer = %v, want SQLite BUSY", err)
		}
		if err := writer.Rollback(); err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadFile(path + "-wal")
		if err != nil {
			t.Fatal(err)
		}
		releaseRetry, err := admission.acquire(ctx, false)
		if err != nil {
			t.Fatal(err)
		}
		releaseRetry = sync.OnceFunc(releaseRetry)
		defer releaseRetry()
		if _, err := db.ExecContext(ctx, "INSERT INTO items VALUES (2)"); err != nil {
			t.Fatal(err)
		}
		after, err := os.ReadFile(path + "-wal")
		if err != nil || len(before) < 32 || len(after) < 32 || !bytes.Equal(before[16:24], after[16:24]) {
			t.Fatalf("foreground restarted the WAL after external contention: %v", err)
		}
		if err := guard.checkpoint(ctx, checkpointDB); err != nil {
			t.Fatalf("backfill after failed restart while admission is occupied = %v", err)
		}
		frames, copied, err := walCheckpointProgress(ctx, checkpointDB)
		if err != nil || frames == 0 || copied != frames {
			t.Fatalf("recovered backfill = %d/%d, error=%v", copied, frames, err)
		}
		releaseRetry()
		if err := guard.checkpoint(ctx, checkpointDB); err != nil {
			t.Fatalf("retry after contention = %v", err)
		}
		var count int
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM items").Scan(&count); err != nil || count != 1 {
			t.Fatalf("rows after retry = %d, error=%v, want 1", count, err)
		}
	})

	t.Run("Should queue foreground writes before BEGIN during maintenance sync", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), "restart-blocked.db")
		seed := openExecuteWriteTestDB(t, path)
		identity := seed.Driver()
		if err := seed.Close(); err != nil {
			t.Fatal(err)
		}
		io := &blockedStoreCheckpointConnector{
			sqliteDriver: identity,
			path:         path,
			entered:      make(chan struct{}),
			released:     make(chan struct{}),
			closed:       make(chan struct{}),
		}
		db := sql.OpenDB(io)
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
		writerDB := openExecuteWriteTestDB(t, path)
		readDB, err := OpenSQLiteReadPool(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		guard := &walRestartGuard{readDB: readDB, lifetime: ctx}
		done := make(chan struct{})
		var checkpointErr error
		var release sync.Once
		t.Cleanup(func() {
			release.Do(func() { close(io.released) })
			<-done
			if checkpointErr != nil {
				t.Error(checkpointErr)
			}
			if err := errors.Join(guard.close(), db.Close()); err != nil {
				t.Error(err)
			}
		})
		go func() {
			checkpointErr = guard.restart(ctx, db)
			close(done)
		}()
		select {
		case <-io.entered:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		writeCtx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
		defer cancel()
		called := false
		err = ExecuteWrite(writeCtx, writerDB, func(context.Context, *WriteTx) error {
			called = true
			return nil
		})
		if !errors.Is(err, context.DeadlineExceeded) || called {
			t.Fatalf("write during maintenance sync = %v, callback called=%t, want deadline before BEGIN", err, called)
		}
		contention, ok := errors.AsType[*WriteContentionError](err)
		if !ok || contention.Phase != "admission" || contention.Attempts != 0 {
			t.Fatalf("write waited outside admission: %#v", err)
		}
		release.Do(func() { close(io.released) })
		select {
		case <-done:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	})

	// Invariant: WAL maintenance progresses without reserving writer capacity or
	// waiting for a live write, and its owner joins it on close. Owner: SQLite
	// connection lifecycle. Canonical suite: the checkpoint helpers in this file.
	t.Run("Should checkpoint committed frames independently of an occupied writer", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), "owned-checkpoint.db")
		db, err := OpenSQLiteDatabaseWithPragmas(ctx, path, []string{"wal_autocheckpoint(0)"},
			func(ctx context.Context, db *sql.DB) error {
				_, err := db.ExecContext(ctx, "CREATE TABLE items (value INTEGER)")
				return err
			})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(); err != nil {
				t.Error(err)
			}
		})
		db.SetMaxOpenConns(1)
		if _, err := db.ExecContext(ctx, "INSERT INTO items VALUES (1)"); err != nil {
			t.Fatal(err)
		}
		probe, err := openSQLiteHandle(ctx, path, sqliteCheckpointDSN(path))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := probe.Close(); err != nil {
				t.Error(err)
			}
		})
		var busy, frames, copied int
		if err := probe.QueryRowContext(ctx, "PRAGMA wal_checkpoint(NOOP)").Scan(&busy, &frames, &copied); err != nil {
			t.Fatal(err)
		}
		if frames <= copied {
			t.Fatalf("WAL frames = %d, copied = %d, want pending committed work", frames, copied)
		}
		pendingFrames := frames
		worker, err := OpenPassiveCheckpointer(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := worker.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		writer, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := writer.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				t.Error(err)
			}
		})
		if _, err := writer.ExecContext(ctx, "INSERT INTO items VALUES (2)"); err != nil {
			t.Fatal(err)
		}
		waitCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for copied < pendingFrames {
			select {
			case <-waitCtx.Done():
				t.Fatalf(
					"maintenance copied %d/%d frames while writer occupied: %v",
					copied,
					pendingFrames,
					waitCtx.Err(),
				)
			case <-ticker.C:
				if err := probe.QueryRowContext(waitCtx, "PRAGMA wal_checkpoint(NOOP)").
					Scan(&busy, &frames, &copied); err != nil {
					t.Fatal(err)
				}
			}
		}
		closed := make(chan error, 1)
		go func() { closed <- worker.Close(ctx) }()
		select {
		case err := <-closed:
			if err != nil {
				t.Fatal(err)
			}
		case <-waitCtx.Done():
			t.Fatal("maintenance close waited for the unrelated writer")
		}
		if err := writer.Commit(); err != nil {
			t.Fatal(err)
		}
		var count int
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM items").Scan(&count); err != nil || count != 2 {
			t.Fatalf("rows after maintenance shutdown = %d, %v, want both commits", count, err)
		}
	})

	// Invariant: shutdown honors deadlines and retains connections until stuck I/O exits.
	// Owner: SQLite lifecycle; canonical suite: TestStoreSQLiteHelpers.
	t.Run("Should bound close while retaining databases until checkpoint I/O exits", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		path := filepath.Join(t.TempDir(), "blocked-checkpoint.db")
		seed := openExecuteWriteTestDB(t, path)
		if err := seed.Close(); err != nil {
			t.Fatal(err)
		}
		owner := &blockedStoreCheckpointConnector{path: path, closed: make(chan struct{})}
		db := sql.OpenDB(owner)
		if _, err := db.ExecContext(ctx, "CREATE TABLE items (value INTEGER)"); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, "INSERT INTO items VALUES (42)"); err != nil {
			t.Fatal(err)
		}
		io := &blockedStoreCheckpointConnector{
			path: path, entered: make(chan struct{}), released: make(chan struct{}), closed: make(chan struct{}),
		}
		checkpointDB := sql.OpenDB(io)
		workerCtx, cancel := context.WithCancel(ctx)
		worker := &PassiveCheckpointer{cancel: cancel, done: make(chan struct{})}
		var release sync.Once
		t.Cleanup(func() {
			release.Do(func() { close(io.released) })
			if err := worker.Close(context.Background()); err != nil {
				t.Error(err)
			}
			if err := db.Close(); err != nil {
				t.Error(err)
			}
		})
		go worker.run(workerCtx, checkpointDB)
		select {
		case <-io.entered:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		closeCtx, closeCancel := context.WithTimeout(ctx, 20*time.Millisecond)
		defer closeCancel()
		if err := worker.Close(closeCtx); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("maintenance close = %v, want deadline", err)
		}
		if err := CloseCheckpointedDatabase(closeCtx, db, worker); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("database close = %v, want deadline", err)
		}
		for _, closed := range []chan struct{}{io.closed, owner.closed} {
			select {
			case <-closed:
				t.Fatal("connection closed before checkpoint I/O exited")
			default:
			}
		}
		var value int
		if err := db.QueryRowContext(ctx, "SELECT value FROM items").Scan(&value); err != nil || value != 42 {
			t.Fatalf("retained database row = %d, %v", value, err)
		}
		release.Do(func() { close(io.released) })
		for _, closed := range []chan struct{}{io.closed, owner.closed} {
			select {
			case <-closed:
			case <-ctx.Done():
				t.Fatal("owned cleanup did not close connection")
			}
		}
		if err := db.PingContext(ctx); err == nil {
			t.Fatal("database remains open after cleanup")
		}
	})

	t.Run("Should report a retained reader preventing WAL truncation", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db := openExecuteWriteTestDB(t, filepath.Join(t.TempDir(), "checkpoint-reader.db"))
		db.SetMaxOpenConns(2)
		if _, err := db.ExecContext(
			ctx,
			`CREATE TABLE items (value INTEGER); INSERT INTO items VALUES (1)`,
		); err != nil {
			t.Fatal(err)
		}
		if err := Checkpoint(ctx, db); err != nil {
			t.Fatal(err)
		}
		reader, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := reader.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				t.Error(err)
			}
		})
		var count int
		if err := reader.QueryRowContext(ctx, `SELECT count(*) FROM items`).Scan(&count); err != nil || count != 1 {
			t.Fatalf("reader rows = %d, error = %v, want one", count, err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO items VALUES (2)`); err != nil {
			t.Fatal(err)
		}
		if err := CheckpointPassive(ctx, db); err != nil {
			t.Fatalf("partial passive checkpoint = %v, want success", err)
		}
		if err := Checkpoint(ctx, db); !IsSQLiteBusy(err) {
			t.Errorf("blocked WAL truncation = %v, want incomplete checkpoint busy error", err)
		}
		if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout = 5000"); err != nil {
			t.Fatal(err)
		}
		checkpointCtx, cancel := context.WithTimeout(ctx, 25*time.Millisecond)
		defer cancel()
		started := time.Now()
		err = Checkpoint(checkpointCtx, db)
		if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) >= time.Second {
			t.Errorf("canceled checkpoint = %v after %s, want prompt deadline", err, time.Since(started))
		}
		var busyTimeout int
		if err := db.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil || busyTimeout != 5000 {
			t.Fatalf("busy timeout after checkpoint cancellation = %d, error = %v, want 5000", busyTimeout, err)
		}
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM items`).Scan(&count); err != nil || count != 2 {
			t.Fatalf("committed rows after checkpoint failure = %d, error = %v, want two", count, err)
		}
		if err := reader.Rollback(); err != nil {
			t.Fatal(err)
		}
		if err := Checkpoint(ctx, db); err != nil {
			t.Fatalf("checkpoint after reader release = %v", err)
		}
	})

	dsn := sqliteDSN("/tmp/example.db")
	parsedDSN, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("url.Parse(sqliteDSN()) error = %v", err)
	}
	if got, want := parsedDSN.Scheme, "file"; got != want {
		t.Fatalf("sqliteDSN() scheme = %q, want %q", got, want)
	}
	if got, want := parsedDSN.Path, "/tmp/example.db"; got != want {
		t.Fatalf("sqliteDSN() path = %q, want %q", got, want)
	}
	pragmas := parsedDSN.Query()["_pragma"]
	for _, want := range []string{
		"busy_timeout(5000)",
		"foreign_keys(ON)",
	} {
		var found bool
		if slices.Contains(pragmas, want) {
			found = true
		}
		if !found {
			t.Fatalf("sqliteDSN() pragmas = %#v, want %q", pragmas, want)
		}
	}
	if got, want := NullableInt64(nil), any(nil); got != want {
		t.Fatalf("NullableInt64(nil) = %#v, want nil", got)
	}
	value := int64(7)
	if got := NullableInt64(&value); got != int64(7) {
		t.Fatalf("NullableInt64(valid) = %#v, want 7", got)
	}
	if got, want := NullableFloat64(nil), any(nil); got != want {
		t.Fatalf("NullableFloat64(nil) = %#v, want nil", got)
	}
	floatValue := 1.5
	if got := NullableFloat64(&floatValue); got != 1.5 {
		t.Fatalf("NullableFloat64(valid) = %#v, want 1.5", got)
	}
	if got := NullString(sql.NullString{String: "   ", Valid: true}); got != nil {
		t.Fatalf("NullString(blank) = %#v, want nil", got)
	}
	if _, err := NormalizeSQLiteIdentifier("bad-name"); err == nil {
		t.Fatal("NormalizeSQLiteIdentifier(invalid) error = nil, want non-nil")
	}
	if got, err := NormalizeSQLiteIdentifier("valid_name_2"); err != nil || got != "valid_name_2" {
		t.Fatalf("NormalizeSQLiteIdentifier(valid) = (%q, %v), want (valid_name_2, nil)", got, err)
	}

	dbPath := filepath.Join(t.TempDir(), "shared.db")
	db, err := openSQLiteDatabaseOnce(testutil.Context(t), dbPath, func(ctx context.Context, db *sql.DB) error {
		return executeTestSchema(ctx, db, []string{
			`CREATE TABLE IF NOT EXISTS sample (id TEXT PRIMARY KEY, value TEXT NOT NULL);`,
			`INSERT INTO sample (id, value) VALUES ('row-1', 'alpha');`,
		})
	})
	if err != nil {
		t.Fatalf("openSQLiteDatabaseOnce() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Errorf("db.Close() error = %v", closeErr)
		}
	})

	if err := configureSQLite(testutil.Context(t), db); err != nil {
		t.Fatalf("configureSQLite() error = %v", err)
	}
	if err := Checkpoint(testutil.Context(t), db); err != nil {
		t.Fatalf("Checkpoint() error = %v", err)
	}

	if mode, err := querySingleString(
		testutil.Context(t),
		db,
		"PRAGMA journal_mode",
	); err != nil ||
		!strings.EqualFold(mode, "wal") {
		t.Fatalf("querySingleString(journal_mode) = (%q, %v), want wal", mode, err)
	}
	verifyConnPragmas := func(t *testing.T, conn *sql.Conn) {
		t.Helper()

		var busyTimeout int
		if err := conn.QueryRowContext(testutil.Context(t), "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
			t.Fatalf("QueryRowContext(PRAGMA busy_timeout) error = %v", err)
		}
		if got, want := busyTimeout, defaultBusyTimeoutMS; got != want {
			t.Fatalf("PRAGMA busy_timeout = %d, want %d", got, want)
		}

		var foreignKeys int
		if err := conn.QueryRowContext(testutil.Context(t), "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
			t.Fatalf("QueryRowContext(PRAGMA foreign_keys) error = %v", err)
		}
		if got, want := foreignKeys, 1; got != want {
			t.Fatalf("PRAGMA foreign_keys = %d, want %d", got, want)
		}

		var tempStore int
		if err := conn.QueryRowContext(testutil.Context(t), "PRAGMA temp_store").Scan(&tempStore); err != nil {
			t.Fatalf("QueryRowContext(PRAGMA temp_store) error = %v", err)
		}
		if got, want := tempStore, 2; got != want {
			t.Fatalf("PRAGMA temp_store = %d, want %d (MEMORY keeps statement journals off disk)", got, want)
		}
	}

	firstConn, err := db.Conn(testutil.Context(t))
	if err != nil {
		t.Fatalf("db.Conn(first) error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := firstConn.Close(); closeErr != nil {
			t.Errorf("firstConn.Close() error = %v", closeErr)
		}
	})
	verifyConnPragmas(t, firstConn)

	secondConn, err := db.Conn(testutil.Context(t))
	if err != nil {
		t.Fatalf("db.Conn(second) error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := secondConn.Close(); closeErr != nil {
			t.Errorf("secondConn.Close() error = %v", closeErr)
		}
	})
	verifyConnPragmas(t, secondConn)

	var count int
	if err := db.QueryRowContext(testutil.Context(t), `SELECT COUNT(*) FROM sample`).Scan(&count); err != nil {
		t.Fatalf("QueryRowContext(count) error = %v", err)
	}
	if count != 1 {
		t.Fatalf("sample row count = %d, want 1", count)
	}
}

func TestSQLiteDSNAppendsExtraPragmas(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve shared pragmas and append caller pragmas", func(t *testing.T) {
		t.Parallel()

		dsn := sqliteDSN("/tmp/example.db", "wal_autocheckpoint(0)", "  ")
		parsedDSN, err := url.Parse(dsn)
		if err != nil {
			t.Fatalf("url.Parse(sqliteDSN()) error = %v", err)
		}
		pragmas := parsedDSN.Query()["_pragma"]
		for _, want := range []string{
			"busy_timeout(5000)",
			"foreign_keys(ON)",
			"wal_autocheckpoint(0)",
		} {
			if !slices.Contains(pragmas, want) {
				t.Fatalf("sqliteDSN() pragmas = %#v, want %q", pragmas, want)
			}
		}
		if slices.Contains(pragmas, "") {
			t.Fatalf("sqliteDSN() pragmas = %#v, want no blank pragma", pragmas)
		}
	})
}

// Block only the driver I/O boundary; database queries still execute on real SQLite.
type blockedStoreCheckpointConnector struct {
	sqliteDriver driver.Driver
	path         string
	entered      chan struct{}
	released     chan struct{}
	closed       chan struct{}
	once         sync.Once
}

func (c *blockedStoreCheckpointConnector) Driver() driver.Driver {
	if c.sqliteDriver != nil {
		return c.sqliteDriver
	}
	return &sqlite.Driver{}
}

func (c *blockedStoreCheckpointConnector) Connect(context.Context) (driver.Conn, error) {
	conn, err := c.Driver().Open(sqliteCheckpointDSN(c.path))
	if err != nil {
		return nil, err
	}
	return &blockedStoreCheckpointConnection{
		Conn: conn, ExecQuerierContext: conn.(sqlite.ExecQuerierContext), owner: c,
	}, nil
}

type blockedStoreCheckpointConnection struct {
	driver.Conn
	sqlite.ExecQuerierContext
	owner *blockedStoreCheckpointConnector
}

func (c *blockedStoreCheckpointConnection) Close() error {
	err := c.Conn.Close()
	close(c.owner.closed)
	return err
}

func (c *blockedStoreCheckpointConnection) ExecContext(
	ctx context.Context, query string, args []driver.NamedValue,
) (driver.Result, error) {
	if query == "PRAGMA wal_checkpoint(PASSIVE)" && c.owner.entered != nil {
		c.owner.once.Do(func() { close(c.owner.entered) })
		<-c.owner.released
	}
	return c.ExecQuerierContext.ExecContext(ctx, query, args)
}
