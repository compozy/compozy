package workspacedb

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
	aghworkspace "github.com/compozy/agh/internal/workspace"
)

func TestOpen(t *testing.T) {
	t.Run("Should own its migration stream and preserve data across reopen", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		workspaceRoot := t.TempDir()
		db := openWorkspaceTestDB(ctx, t, workspaceRoot)

		realWorkspaceRoot, err := filepath.EvalSymlinks(workspaceRoot)
		if err != nil {
			t.Fatalf("EvalSymlinks(workspaceRoot) error = %v", err)
		}
		if got, want := db.Path(), filepath.Join(realWorkspaceRoot, ".agh", store.GlobalDatabaseName); got != want {
			t.Fatalf("Path() = %q, want %q", got, want)
		}
		if !aghworkspace.IsWorkspaceID(db.WorkspaceID()) {
			t.Fatalf("WorkspaceID() = %q, want canonical ULID", db.WorkspaceID())
		}
		firstStatus, err := store.Status(ctx, db.DB(), MigrationStream())
		if err != nil {
			t.Fatalf("Status(first) error = %v", err)
		}
		if firstStatus.Version != 1 || firstStatus.AppliedCount != 1 {
			t.Fatalf("Status(first) = %#v, want version/applied count 1", firstStatus)
		}
		if _, err := db.DB().ExecContext(ctx, `CREATE TABLE records (id TEXT PRIMARY KEY)`); err != nil {
			t.Fatalf("Create records table error = %v", err)
		}
		if _, err := db.DB().ExecContext(ctx, `INSERT INTO records (id) VALUES ('first')`); err != nil {
			t.Fatalf("Insert first record error = %v", err)
		}
		if err := db.Close(ctx); err != nil {
			t.Fatalf("Close(first) error = %v", err)
		}

		reopened := openWorkspaceTestDB(ctx, t, workspaceRoot)
		secondStatus, err := store.Status(ctx, reopened.DB(), MigrationStream())
		if err != nil {
			t.Fatalf("Status(reopen) error = %v", err)
		}
		if secondStatus != firstStatus {
			t.Fatalf("Status(reopen) = %#v, want unchanged %#v", secondStatus, firstStatus)
		}
		assertWorkspaceRecordCount(ctx, t, reopened.DB(), "first", 1)
	})

	t.Run("Should surface the shared engine ahead-schema error", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		workspaceRoot := t.TempDir()
		db := openWorkspaceTestDB(ctx, t, workspaceRoot)
		if _, err := db.DB().ExecContext(
			ctx,
			`INSERT INTO goose_db_version_workspace (version_id, is_applied) VALUES (99, 1)`,
		); err != nil {
			t.Fatalf("Insert future migration error = %v", err)
		}
		if err := db.Close(ctx); err != nil {
			t.Fatalf("Close() error = %v", err)
		}

		_, err := Open(ctx, Options{WorkspaceRoot: workspaceRoot})
		if !errors.Is(err, store.ErrSchemaAhead) {
			t.Fatalf("Open(ahead schema) error = %v, want ErrSchemaAhead", err)
		}
		if err == nil || !strings.Contains(err.Error(), "install a newer AGH binary") {
			t.Fatalf("Open(ahead schema) error = %v, want deterministic remediation", err)
		}
	})

	t.Run("Should isolate rows across workspace database files", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		first := openWorkspaceTestDB(ctx, t, t.TempDir())
		second := openWorkspaceTestDB(ctx, t, t.TempDir())
		for _, db := range []*DB{first, second} {
			if _, err := db.DB().ExecContext(ctx, `CREATE TABLE records (id TEXT PRIMARY KEY)`); err != nil {
				t.Fatalf("Create records table error = %v", err)
			}
		}
		if _, err := first.DB().ExecContext(ctx, `INSERT INTO records (id) VALUES ('first')`); err != nil {
			t.Fatalf("Insert first workspace record error = %v", err)
		}
		if _, err := second.DB().ExecContext(ctx, `INSERT INTO records (id) VALUES ('second')`); err != nil {
			t.Fatalf("Insert second workspace record error = %v", err)
		}

		assertWorkspaceRecordCount(ctx, t, first.DB(), "first", 1)
		assertWorkspaceRecordCount(ctx, t, first.DB(), "second", 0)
		assertWorkspaceRecordCount(ctx, t, second.DB(), "second", 1)
		assertWorkspaceRecordCount(ctx, t, second.DB(), "first", 0)
	})

	t.Run("Should support the package-owned helper and idempotent close", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db, err := OpenWorkspace(ctx, t.TempDir())
		if err != nil {
			t.Fatalf("OpenWorkspace() error = %v", err)
		}
		if db.DB() == nil || db.WorkspaceRoot() == "" || db.Path() == "" || db.WorkspaceID() == "" {
			t.Fatalf("OpenWorkspace() returned incomplete database: %#v", db)
		}
		if err := db.Close(ctx); err != nil {
			t.Fatalf("Close(first) error = %v", err)
		}
		if err := db.Close(ctx); err != nil {
			t.Fatalf("Close(second) error = %v", err)
		}
	})

	t.Run("Should reject invalid open and close inputs", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		if _, err := Open(ctx, Options{WorkspaceRoot: "   "}); err == nil {
			t.Fatal("Open(blank root) error = nil, want validation error")
		}
		var nilDB *DB
		if nilDB.Path() != "" || nilDB.WorkspaceID() != "" || nilDB.WorkspaceRoot() != "" || nilDB.DB() != nil {
			t.Fatal("nil DB accessors returned non-zero values")
		}
		if err := nilDB.Close(ctx); err != nil {
			t.Fatalf("nil DB Close() error = %v", err)
		}
	})
}

func openWorkspaceTestDB(ctx context.Context, t *testing.T, workspaceRoot string) *DB {
	t.Helper()
	db, err := Open(ctx, Options{WorkspaceRoot: workspaceRoot})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(ctx); err != nil {
			t.Fatalf("DB.Close() error = %v", err)
		}
	})
	return db
}

func assertWorkspaceRecordCount(ctx context.Context, t *testing.T, db *sql.DB, id string, want int) {
	t.Helper()
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM records WHERE id = ?`, id).Scan(&count); err != nil {
		t.Fatalf("Query record count for %q error = %v", id, err)
	}
	if count != want {
		t.Fatalf("record count for %q = %d, want %d", id, count, want)
	}
}
