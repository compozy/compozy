package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func TestSessionListReadStatementReuse(t *testing.T) {
	t.Parallel()

	t.Run("Should reuse one statement per query shape and still read fresh rows", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := t.Context()
		insertStatementCacheSessions(t, globalDB, "sess-stmt-a")
		query := store.SessionListQuery{ReadScope: store.ReadScope{AllProfiles: true}, ID: "sess-stmt-a", Limit: 2}
		first, err := globalDB.ListSessions(ctx, query)
		if err != nil || len(first) != 1 {
			t.Fatalf("ListSessions(first) rows=%d error=%v", len(first), err)
		}
		insertStatementCacheSessions(t, globalDB, "sess-stmt-b")
		query.ID = "sess-stmt-b"
		second, err := globalDB.ListSessions(ctx, query)
		if err != nil || len(second) != 1 || second[0].ID != "sess-stmt-b" {
			t.Fatalf("ListSessions(second) rows=%+v error=%v", second, err)
		}
		if got := globalDB.readStatements.len(); got != 1 {
			t.Fatalf("cached statements = %d, want 1 for one query shape", got)
		}
		if _, err := globalDB.ListSessions(ctx, store.SessionListQuery{
			ReadScope: store.ReadScope{AllProfiles: true}, State: "stopped",
		}); err != nil {
			t.Fatalf("ListSessions(state) error = %v", err)
		}
		if got := globalDB.readStatements.len(); got != 2 {
			t.Fatalf("cached statements = %d, want 2 for two query shapes", got)
		}
	})

	t.Run("Should serve concurrent readers of one shape", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		insertStatementCacheSessions(t, globalDB, "sess-stmt-c")
		var wg sync.WaitGroup
		errs := make(chan error, 16)
		for range 16 {
			wg.Go(func() {
				rows, err := globalDB.ListSessions(t.Context(), store.SessionListQuery{
					ReadScope: store.ReadScope{AllProfiles: true}, ID: "sess-stmt-c", Limit: 2,
				})
				if err == nil && len(rows) != 1 {
					err = fmt.Errorf("rows = %d, want 1", len(rows))
				}
				errs <- err
			})
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("concurrent ListSessions error = %v", err)
			}
		}
		if got := globalDB.readStatements.len(); got != 1 {
			t.Fatalf("cached statements = %d, want 1", got)
		}
	})

	t.Run("Should stop caching beyond the shape bound", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		cache := newReadStatementCache(globalDB.readDB, 1)
		t.Cleanup(func() {
			if err := cache.close(); err != nil {
				t.Errorf("close cache: %v", err)
			}
		})
		for _, query := range []string{"SELECT 1", "SELECT 2"} {
			rows, err := cache.query(t.Context(), query)
			if err != nil {
				t.Fatalf("query %q error = %v", query, err)
			}
			if err := rows.Close(); err != nil {
				t.Fatalf("close rows: %v", err)
			}
		}
		if got := cache.len(); got != 1 {
			t.Fatalf("cached statements = %d, want bound 1", got)
		}
	})

	t.Run("Should close retained statements and fall back while the pool stays open", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		cache := newReadStatementCache(globalDB.readDB, 4)
		rows, err := cache.query(t.Context(), "SELECT 1")
		if err != nil {
			t.Fatalf("query error = %v", err)
		}
		if err := rows.Close(); err != nil {
			t.Fatalf("close rows: %v", err)
		}
		retained, err := cache.statement(t.Context(), "SELECT 1")
		if err != nil || retained == nil {
			t.Fatalf("retained statement = %v, %v", retained, err)
		}
		if err := cache.close(); err != nil {
			t.Fatalf("close cache: %v", err)
		}
		closedRows, err := retained.QueryContext(t.Context())
		if closedRows != nil {
			if closeErr := closedRows.Close(); closeErr != nil {
				t.Fatalf("close rows of closed statement: %v", closeErr)
			}
		}
		if err == nil || !strings.Contains(err.Error(), "sql: statement is closed") {
			t.Fatalf("retained statement after cache close error = %v, want statement is closed", err)
		}
		rows, err = cache.query(t.Context(), "SELECT 1")
		if err != nil {
			t.Fatalf("fallback query after close error = %v", err)
		}
		if err := rows.Close(); err != nil {
			t.Fatalf("close fallback rows: %v", err)
		}
		if got := cache.len(); got != 0 {
			t.Fatalf("cached statements after close = %d, want 0", got)
		}
	})

	t.Run("Should close the pool without waiting for a read queued on a cached statement", func(t *testing.T) {
		t.Parallel()

		db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "queued-read.db"))
		if err != nil {
			t.Fatalf("open pool: %v", err)
		}
		db.SetMaxOpenConns(1)
		cache := newReadStatementCache(db, 4)
		// Registered first so it runs last: an early failure still closes the pool and cache.
		t.Cleanup(func() { _ = closeReadPool(context.WithoutCancel(t.Context()), db, cache) })
		rows, err := cache.query(t.Context(), "SELECT 1")
		if err != nil {
			t.Fatalf("prime cached statement: %v", err)
		}
		if err := rows.Close(); err != nil {
			t.Fatalf("close primed rows: %v", err)
		}
		held, err := db.Conn(t.Context())
		if err != nil {
			t.Fatalf("hold the only connection: %v", err)
		}
		t.Cleanup(func() { _ = held.Close() })
		queued := make(chan error, 1)
		go func() {
			rows, err := cache.query(context.WithoutCancel(t.Context()), "SELECT 1")
			if rows != nil {
				_ = rows.Close()
			}
			queued <- err
		}()
		queueDeadline := time.Now().Add(5 * time.Second)
		for db.Stats().WaitCount == 0 {
			select {
			case err := <-queued:
				t.Fatalf("cached read finished before queueing behind the held connection: %v", err)
			default:
			}
			if time.Now().After(queueDeadline) {
				t.Fatal("cached read never queued behind the held connection")
			}
			time.Sleep(time.Millisecond)
		}
		closed := make(chan error, 1)
		go func() { closed <- closeReadPool(context.WithoutCancel(t.Context()), db, cache) }()
		select {
		case err := <-closed:
			if err != nil {
				t.Fatalf("closeReadPool error = %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("closeReadPool waited for the held connection")
		}
		select {
		case err := <-queued:
			if err == nil || !strings.Contains(err.Error(), "sql: database is closed") {
				t.Fatalf("queued cached read error = %v, want database is closed", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("queued cached read did not finish after the pool closed")
		}
	})

	t.Run("Should read without caching when the open option disables reuse", func(t *testing.T) {
		t.Parallel()

		globalDB, err := OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), GlobalDatabaseName),
			WithReadStatementCacheLimit(0))
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := globalDB.Close(context.WithoutCancel(t.Context())); err != nil {
				t.Errorf("close global db: %v", err)
			}
		})
		insertStatementCacheSessions(t, globalDB, "sess-stmt-off")
		rows, err := globalDB.ListSessions(t.Context(), store.SessionListQuery{
			ReadScope: store.ReadScope{AllProfiles: true}, ID: "sess-stmt-off", Limit: 2,
		})
		if err != nil || len(rows) != 1 {
			t.Fatalf("ListSessions rows=%d error=%v", len(rows), err)
		}
		if got := globalDB.readStatements.len(); got != 0 {
			t.Fatalf("cached statements = %d, want 0 with reuse disabled", got)
		}
	})

	t.Run("Should release statements when the database closes", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		insertStatementCacheSessions(t, globalDB, "sess-stmt-d")
		if _, err := globalDB.ListSessions(t.Context(), store.SessionListQuery{
			ReadScope: store.ReadScope{AllProfiles: true}, ID: "sess-stmt-d", Limit: 2,
		}); err != nil {
			t.Fatalf("ListSessions error = %v", err)
		}
		if err := globalDB.Close(context.WithoutCancel(t.Context())); err != nil {
			t.Fatalf("Close error = %v", err)
		}
		if got := globalDB.readStatements.len(); got != 0 {
			t.Fatalf("cached statements after close = %d, want 0", got)
		}
		_, err := globalDB.ListSessions(t.Context(), store.SessionListQuery{
			ReadScope: store.ReadScope{AllProfiles: true}, ID: "sess-stmt-d", Limit: 2,
		})
		if !errors.Is(err, store.ErrClosed) {
			t.Fatalf("ListSessions after close error = %v, want ErrClosed", err)
		}
	})
}

func insertStatementCacheSessions(t *testing.T, globalDB *GlobalDB, ids ...string) {
	t.Helper()

	for _, id := range ids {
		if _, err := globalDB.db.ExecContext(t.Context(), `INSERT INTO sessions(
 id,profile_id,workspace_id,scope,agent_name,provider,state,created_at,updated_at)
 VALUES (?, ?, '', 'global', 'coder', 'claude', 'stopped', '2026-11-01T00:00:00Z', '2026-11-01T00:00:00Z')`,
			id, store.DefaultProfileID); err != nil {
			t.Fatalf("insert session %q: %v", id, err)
		}
	}
}

// Invariant: sqlc's static reads on the read pool reuse one prepared statement per query text.
// Owning layer/canonical suite: globaldb read statement reuse, next to the dynamic-shape cases above.
func TestStaticReadStatementReuse(t *testing.T) {
	t.Parallel()

	t.Run("Should prepare a static read once and still read fresh rows", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := t.Context()
		insertStatementCacheSessions(t, globalDB, "sess-static-a")
		archivedAt, err := globalDB.SessionArchivedAt(ctx, "", "sess-static-a")
		if err != nil || archivedAt != nil {
			t.Fatalf("SessionArchivedAt(before) = %v, %v; want nil, nil", archivedAt, err)
		}
		if _, err := globalDB.db.ExecContext(ctx, `UPDATE sessions SET archived_at = ? WHERE id = ?`,
			"2026-11-02T00:00:00Z", "sess-static-a"); err != nil {
			t.Fatalf("archive session: %v", err)
		}
		archivedAt, err = globalDB.SessionArchivedAt(ctx, "", "sess-static-a")
		want := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
		if err != nil || archivedAt == nil || !archivedAt.Equal(want) {
			t.Fatalf("SessionArchivedAt(after) = %v, %v; want %s", archivedAt, err, want)
		}
		// Other static reads run in the background, so count only this query's statements.
		archiveRead := "SELECT archived_at FROM sessions"
		if got := cachedStatementsContaining(globalDB.staticReadStatements, archiveRead); got != 1 {
			t.Fatalf("cached archive-state statements = %d, want 1 across two reads", got)
		}
		if got := cachedStatementsContaining(globalDB.readStatements, "archived_at"); got != 0 {
			t.Fatalf("dynamic cache holds %d archive-state statements, want 0 for a static read", got)
		}
	})

	t.Run("Should reuse one statement per task-run filter shape", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		for _, sessionID := range []string{"sess-lease-a", "sess-lease-b"} {
			runs, err := globalDB.ListTaskRuns(t.Context(), taskpkg.RunQuery{
				ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID}, SessionID: sessionID,
			})
			if err != nil || len(runs) != 0 {
				t.Fatalf("ListTaskRuns(%s) = %d runs, %v; want none", sessionID, len(runs), err)
			}
		}
		if got := cachedStatementsContaining(globalDB.readStatements, "FROM task_runs"); got != 1 {
			t.Fatalf("cached task-run statements = %d, want 1 for one filter shape", got)
		}
	})

	t.Run("Should give repositories built on demand the shared statement caches", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		for name, base := range map[string]*repoBase{
			"instances":       globalDB.Instances().repoBase,
			"instance events": globalDB.InstanceEvents(time.Second).repoBase,
			"instance report": globalDB.InstanceReports().repoBase,
			"skill reviews":   globalDB.SkillReviews().repoBase,
		} {
			// newCachedRepoBase wires both caches together, so the shared dynamic cache marks it.
			if base.readStatements == nil || base.readStatements != globalDB.readStatements {
				t.Fatalf("%s repository reads without the shared statement caches", name)
			}
		}
	})

	t.Run("Should serve single-row and multi-row reads through one statement per text", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		cache := newReadStatementCache(globalDB.readDB, 4)
		t.Cleanup(func() {
			if err := cache.close(); err != nil {
				t.Errorf("close cache: %v", err)
			}
		})
		reader := statementReader{db: globalDB.readDB, cache: cache}
		for _, want := range []int{1, 2} {
			var got int
			if err := reader.QueryRowContext(t.Context(), "SELECT ?", want).Scan(&got); err != nil || got != want {
				t.Fatalf("QueryRowContext(%d) = %d, %v", want, got, err)
			}
		}
		rows, err := reader.QueryContext(t.Context(), "SELECT ? UNION ALL SELECT ?", 3, 4)
		if err != nil {
			t.Fatalf("QueryContext error = %v", err)
		}
		var values []int
		for rows.Next() {
			var value int
			if err := rows.Scan(&value); err != nil {
				t.Fatalf("scan row: %v", err)
			}
			values = append(values, value)
		}
		if err := errors.Join(rows.Err(), rows.Close()); err != nil {
			t.Fatalf("read rows: %v", err)
		}
		if len(values) != 2 || values[0] != 3 || values[1] != 4 {
			t.Fatalf("QueryContext rows = %v, want [3 4]", values)
		}
		if got := cache.len(); got != 2 {
			t.Fatalf("cached statements = %d, want 2 for two query texts", got)
		}
	})

	t.Run("Should report no rows through a cached single-row read", func(t *testing.T) {
		t.Parallel()

		reader, _ := openStatementReader(t)
		var got int
		err := reader.QueryRowContext(t.Context(), "SELECT 1 WHERE 0").Scan(&got)
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("QueryRowContext(no rows) error = %v, want sql.ErrNoRows", err)
		}
	})

	t.Run("Should leave expanded slice placeholders unprepared", func(t *testing.T) {
		t.Parallel()

		reader, cache := openStatementReader(t)
		// sqlc expands slices without spaces; sqlInPlaceholders joins them with ", ".
		for _, query := range []string{"SELECT 1 WHERE ? IN (?,?)", "SELECT 1 WHERE ? IN (?, ?)"} {
			rows, err := reader.QueryContext(t.Context(), query, 2, 1, 2)
			if err != nil {
				t.Fatalf("QueryContext(%q) error = %v", query, err)
			}
			found := rows.Next()
			if err := errors.Join(rows.Err(), rows.Close()); err != nil || !found {
				t.Fatalf("QueryContext(%q) found=%t error=%v, want one row", query, found, err)
			}
			dynamic, err := cache.query(t.Context(), query, 2, 1, 2)
			if err != nil {
				t.Fatalf("dynamic query(%q) error = %v", query, err)
			}
			if err := dynamic.Close(); err != nil {
				t.Fatalf("close dynamic rows: %v", err)
			}
		}
		if got := cache.len(); got != 0 {
			t.Fatalf("cached statements = %d, want 0 for expanded placeholder lists", got)
		}
	})

	t.Run("Should never start a read on a statement released by close", func(t *testing.T) {
		t.Parallel()

		reader, cache := openStatementReader(t)
		errs := make(chan error, 64)
		var wg sync.WaitGroup
		for i := range 64 {
			wg.Go(func() {
				var got int
				errs <- reader.QueryRowContext(t.Context(), "SELECT ?", i).Scan(&got)
			})
			if i == 32 {
				if err := cache.close(); err != nil {
					t.Errorf("close cache: %v", err)
				}
			}
		}
		waitStatementReads(t, &wg)
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("read during close error = %v, want every read served", err)
			}
		}
	})

	t.Run("Should bound the close by its context while a read runs", func(t *testing.T) {
		t.Parallel()

		db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "running-read.db"))
		if err != nil {
			t.Fatalf("open pool: %v", err)
		}
		cache := newReadStatementCache(db, 4)
		started, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
		go func() {
			_, runErr := cache.run(t.Context(), "SELECT 1", func(*sql.Stmt) {
				close(started)
				<-release
			})
			done <- runErr
		}()
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("read did not start")
		}
		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()
		closeErr := closeReadPool(ctx, db, cache)
		close(release)
		if !errors.Is(closeErr, context.DeadlineExceeded) {
			t.Fatalf("closeReadPool error = %v, want the context deadline", closeErr)
		}
		select {
		case runErr := <-done:
			if runErr != nil {
				t.Fatalf("running read error = %v", runErr)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("running read did not finish")
		}
	})

	t.Run("Should keep the static bound above the generated read queries", func(t *testing.T) {
		t.Parallel()

		files, err := filepath.Glob(filepath.Join("queries", "*.sql"))
		if err != nil || len(files) == 0 {
			t.Fatalf("glob generated queries: %v (files=%d)", err, len(files))
		}
		reads := 0
		for _, file := range files {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("read %s: %v", file, err)
			}
			for line := range strings.SplitSeq(string(data), "\n") {
				if strings.HasPrefix(line, "-- name: ") &&
					(strings.HasSuffix(line, " :one") || strings.HasSuffix(line, " :many")) {
					reads++
				}
			}
		}
		if reads == 0 || reads > defaultStaticReadStatementCacheLimit {
			t.Fatalf("generated read queries = %d, static bound = %d; raise the bound",
				reads, defaultStaticReadStatementCacheLimit)
		}
	})

	t.Run("Should fall back to unprepared single-row reads after the cache closes", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		cache := newReadStatementCache(globalDB.readDB, 4)
		reader := statementReader{db: globalDB.readDB, cache: cache}
		if err := cache.close(); err != nil {
			t.Fatalf("close cache: %v", err)
		}
		var got int
		if err := reader.QueryRowContext(t.Context(), "SELECT 7").Scan(&got); err != nil || got != 7 {
			t.Fatalf("QueryRowContext after close = %d, %v; want 7", got, err)
		}
		if got := cache.len(); got != 0 {
			t.Fatalf("cached statements after close = %d, want 0", got)
		}
	})

	t.Run("Should surface a failed preparation as the read error", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		cache := newReadStatementCache(globalDB.readDB, 4)
		t.Cleanup(func() {
			if err := cache.close(); err != nil {
				t.Errorf("close cache: %v", err)
			}
		})
		reader := statementReader{db: globalDB.readDB, cache: cache}
		var got int
		err := reader.QueryRowContext(t.Context(), "SELECT missing_column FROM sessions").Scan(&got)
		if err == nil || !strings.Contains(err.Error(), "missing_column") {
			t.Fatalf("QueryRowContext(invalid) error = %v, want the missing column", err)
		}
		if got := cache.len(); got != 0 {
			t.Fatalf("cached statements after a failed prepare = %d, want 0", got)
		}
	})

	t.Run("Should read static queries unprepared when the open option disables reuse", func(t *testing.T) {
		t.Parallel()

		globalDB, err := OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), GlobalDatabaseName),
			WithReadStatementCacheLimit(0))
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := globalDB.Close(context.WithoutCancel(t.Context())); err != nil {
				t.Errorf("close global db: %v", err)
			}
		})
		insertStatementCacheSessions(t, globalDB, "sess-static-off")
		if _, err := globalDB.SessionArchivedAt(t.Context(), "", "sess-static-off"); err != nil {
			t.Fatalf("SessionArchivedAt error = %v", err)
		}
		if got := globalDB.staticReadStatements.len(); got != 0 {
			t.Fatalf("cached static statements = %d, want 0 with reuse disabled", got)
		}
	})

	t.Run("Should take the static bound from its open option", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			options []OpenOption
			want    int
		}{
			{"Should default the static bound above the generated reads", nil, defaultStaticReadStatementCacheLimit},
			{"Should apply a smaller static bound", []OpenOption{WithStaticReadStatementCacheLimit(8)}, 8},
			{"Should disable static reuse alone", []OpenOption{WithStaticReadStatementCacheLimit(0)}, 0},
			{"Should disable static reuse with dynamic reuse", []OpenOption{
				WithStaticReadStatementCacheLimit(8), WithReadStatementCacheLimit(0),
			}, 0},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				if got := newOpenConfig(tc.options).staticReadStatementCacheLimit(); got != tc.want {
					t.Fatalf("static bound = %d, want %d", got, tc.want)
				}
			})
		}
	})

	t.Run("Should release static statements when the database closes", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		insertStatementCacheSessions(t, globalDB, "sess-static-close")
		if _, err := globalDB.SessionArchivedAt(t.Context(), "", "sess-static-close"); err != nil {
			t.Fatalf("SessionArchivedAt error = %v", err)
		}
		if err := globalDB.Close(context.WithoutCancel(t.Context())); err != nil {
			t.Fatalf("Close error = %v", err)
		}
		if got := globalDB.staticReadStatements.len(); got != 0 {
			t.Fatalf("cached static statements after close = %d, want 0", got)
		}
	})
}

func cachedStatementsContaining(cache *readStatementCache, fragment string) int {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	n := 0
	for query := range cache.stmts {
		if strings.Contains(query, fragment) {
			n++
		}
	}
	return n
}

func openStatementReader(t *testing.T) (statementReader, *readStatementCache) {
	t.Helper()
	globalDB := openTestGlobalDB(t)
	cache := newReadStatementCache(globalDB.readDB, 4)
	t.Cleanup(func() {
		if err := cache.close(); err != nil {
			t.Errorf("close cache: %v", err)
		}
	})
	return statementReader{db: globalDB.readDB, cache: cache}, cache
}

func waitStatementReads(t *testing.T, wg *sync.WaitGroup) {
	t.Helper()
	finished := make(chan struct{})
	go func() {
		wg.Wait()
		close(finished)
	}()
	select {
	case <-finished:
	case <-time.After(10 * time.Second):
		t.Fatal("reads did not finish")
	}
}
