package globaldb

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/compozy/compozy/internal/store"
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
		if _, err := retained.QueryContext(t.Context()); err == nil {
			t.Fatal("retained statement still executes after cache close")
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
