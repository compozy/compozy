package workspacedb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	compozyworkspace "github.com/compozy/compozy/internal/workspace"
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
		if got, want := db.Path(), filepath.Join(realWorkspaceRoot, ".compozy", store.GlobalDatabaseName); got != want {
			t.Fatalf("Path() = %q, want %q", got, want)
		}
		if !compozyworkspace.IsWorkspaceID(db.WorkspaceID()) {
			t.Fatalf("WorkspaceID() = %q, want canonical ULID", db.WorkspaceID())
		}
		firstStatus, err := store.Status(ctx, db.DB(), MigrationStream())
		if err != nil {
			t.Fatalf("Status(first) error = %v", err)
		}
		if firstStatus.Version != 4 || firstStatus.AppliedCount != 4 {
			t.Fatalf("Status(first) = %#v, want version/applied count 4", firstStatus)
		}
		assertTerminalSchema(ctx, t, db.DB())
		if _, err := db.DB().ExecContext(ctx, `INSERT INTO terminal_recordings (
			id, terminal_id, profile_id, digest, path, started_at, bytes, expires_at
		) VALUES ('rec-1', 'term-1', 'profile-1', 'digest-rec', '/tmp/rec', 10, 12, 100)`); err != nil {
			t.Fatalf("Insert recording error = %v", err)
		}
		if _, err := db.DB().ExecContext(ctx, `INSERT INTO terminal_commands (
			id, terminal_id, profile_id, actor_kind, actor_id, command, cwd, started_at,
			exit_cause, detected_by, approval, output_bytes, truncated, recording_id
		) VALUES ('cmd-1', 'term-1', 'profile-1', 'human', 'operator', 'pwd', '/tmp', 10,
			'exited', 'exact', 'human', 4, 0, 'rec-1')`); err != nil {
			t.Fatalf("Insert command error = %v", err)
		}
		if _, err := db.DB().ExecContext(ctx, `INSERT INTO terminal_artifacts (
			id, terminal_id, command_id, profile_id, digest, path, bytes, expires_at
		) VALUES ('art-1', 'term-1', 'cmd-1', 'profile-1', 'digest-art', '/tmp/art', 4, 100)`); err != nil {
			t.Fatalf("Insert artifact error = %v", err)
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
		assertTerminalRowCounts(ctx, t, reopened.DB(), 1)
		if _, err := reopened.DB().ExecContext(
			ctx,
			`UPDATE terminal_commands SET profile_id = 'profile-2' WHERE id = 'cmd-1'`,
		); err == nil || !strings.Contains(err.Error(), "profile_owner_immutable") {
			t.Fatalf("Update terminal_commands.profile_id error = %v, want profile_owner_immutable", err)
		}
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
		if err == nil || !strings.Contains(err.Error(), "install a newer CompozyOS binary") {
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

	t.Run("Should reuse and close per-workspace pool handles", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		workspaceRoot := t.TempDir()
		identity, err := compozyworkspace.EnsureIdentity(ctx, workspaceRoot)
		if err != nil {
			t.Fatalf("EnsureIdentity() error = %v", err)
		}
		const registrationID = "ws_0123456789abcdef"
		pool, err := NewPool(func(_ context.Context, workspaceID string) (ResolvedRoot, error) {
			if workspaceID != registrationID {
				return ResolvedRoot{}, errors.New("unexpected workspace")
			}
			return ResolvedRoot{RootDir: workspaceRoot, WorkspaceID: identity.WorkspaceID}, nil
		})
		if err != nil {
			t.Fatalf("NewPool() error = %v", err)
		}
		t.Cleanup(func() {
			if err := pool.Close(ctx); err != nil {
				t.Errorf("Pool.Close() error = %v", err)
			}
		})
		first, err := pool.Open(ctx, registrationID)
		if err != nil {
			t.Fatalf("Pool.Open(first) error = %v", err)
		}
		second, err := pool.Open(ctx, registrationID)
		if err != nil {
			t.Fatalf("Pool.Open(second) error = %v", err)
		}
		if first != second {
			t.Fatal("Pool.Open() did not reuse the workspace handle")
		}
		if err := pool.CloseWorkspace(ctx, registrationID); err != nil {
			t.Fatalf("Pool.CloseWorkspace() error = %v", err)
		}
		reopened, err := pool.Open(ctx, registrationID)
		if err != nil {
			t.Fatalf("Pool.Open(reopen) error = %v", err)
		}
		if reopened == first {
			t.Fatal("Pool.Open(reopen) reused a closed handle")
		}
	})

	t.Run("Should retain workspace removal ownership until a failed commit retry succeeds", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		workspaceRoot := t.TempDir()
		identity, err := compozyworkspace.EnsureIdentity(ctx, workspaceRoot)
		if err != nil {
			t.Fatalf("EnsureIdentity() error = %v", err)
		}
		pool, err := NewPool(func(context.Context, string) (ResolvedRoot, error) {
			return ResolvedRoot{RootDir: workspaceRoot, WorkspaceID: identity.WorkspaceID}, nil
		})
		if err != nil {
			t.Fatalf("NewPool() error = %v", err)
		}
		database, err := pool.Open(ctx, "workspace-a")
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		preparation, err := pool.PrepareWorkspaceRemoval(ctx, "workspace-a")
		if err != nil {
			t.Fatalf("PrepareWorkspaceRemoval() error = %v", err)
		}
		if err := preparation.BeforeDelete(ctx); err != nil {
			t.Fatalf("BeforeDelete() error = %v", err)
		}
		stagedDatabase, err := pool.Open(ctx, "workspace-a")
		if err != nil {
			t.Fatalf("Open(staged existing) error = %v", err)
		}
		if stagedDatabase != database {
			t.Fatal("Open(staged existing) did not reuse the admitted workspace handle")
		}
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		if err := preparation.Commit(canceled); !errors.Is(err, context.Canceled) {
			t.Fatalf("Commit(canceled) error = %v, want context canceled", err)
		}
		if _, err := pool.Open(ctx, "workspace-a"); !errors.Is(err, errWorkspaceRemovalPending) {
			t.Fatalf("Open(during retry) error = %v, want removal pending", err)
		}
		if err := preparation.Commit(ctx); err != nil {
			t.Fatalf("Commit(retry) error = %v", err)
		}
		if err := preparation.Commit(ctx); err != nil {
			t.Fatalf("Commit(idempotent) error = %v", err)
		}
	})

	t.Run("Should close a detached removal handle during pool shutdown", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		workspaceRoot := t.TempDir()
		identity, err := compozyworkspace.EnsureIdentity(ctx, workspaceRoot)
		if err != nil {
			t.Fatalf("EnsureIdentity() error = %v", err)
		}
		pool, err := NewPool(func(context.Context, string) (ResolvedRoot, error) {
			return ResolvedRoot{RootDir: workspaceRoot, WorkspaceID: identity.WorkspaceID}, nil
		})
		if err != nil {
			t.Fatalf("NewPool() error = %v", err)
		}
		database, err := pool.Open(ctx, "workspace-a")
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		preparation, err := pool.PrepareWorkspaceRemovalAt(ctx, "workspace-a", workspaceRoot)
		if err != nil {
			t.Fatalf("PrepareWorkspaceRemovalAt() error = %v", err)
		}
		if err := preparation.BeforeDelete(ctx); err != nil {
			t.Fatalf("BeforeDelete() error = %v", err)
		}
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		if err := preparation.Commit(canceled); !errors.Is(err, context.Canceled) {
			t.Fatalf("Commit(canceled) error = %v, want context canceled", err)
		}
		if err := pool.Close(ctx); err != nil {
			t.Fatalf("Pool.Close() error = %v", err)
		}
		if err := database.DB().PingContext(ctx); err == nil {
			t.Fatal("detached database remained open after Pool.Close()")
		}
	})

	t.Run("Should remove workspace database files after its handle was closed", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		workspaceRoot := t.TempDir()
		identity, err := compozyworkspace.EnsureIdentity(ctx, workspaceRoot)
		if err != nil {
			t.Fatalf("EnsureIdentity() error = %v", err)
		}
		const registrationID = "ws_1123456789abcdef"
		pool, err := NewPool(func(_ context.Context, workspaceID string) (ResolvedRoot, error) {
			if workspaceID != registrationID {
				return ResolvedRoot{}, errors.New("unexpected workspace")
			}
			return ResolvedRoot{RootDir: workspaceRoot, WorkspaceID: identity.WorkspaceID}, nil
		})
		if err != nil {
			t.Fatalf("NewPool() error = %v", err)
		}
		db, err := pool.Open(ctx, registrationID)
		if err != nil {
			t.Fatalf("Pool.Open() error = %v", err)
		}
		dbPath := db.Path()
		if err := pool.CloseWorkspace(ctx, registrationID); err != nil {
			t.Fatalf("Pool.CloseWorkspace() error = %v", err)
		}
		if err := pool.RemoveWorkspace(ctx, registrationID); err != nil {
			t.Fatalf("Pool.RemoveWorkspace() error = %v", err)
		}
		if _, err := os.Stat(dbPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat(%q) error = %v, want %v", dbPath, err, os.ErrNotExist)
		}
	})

	t.Run("Should open independent workspace roots concurrently", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		firstRoot := t.TempDir()
		firstIdentity, err := compozyworkspace.EnsureIdentity(ctx, firstRoot)
		if err != nil {
			t.Fatalf("EnsureIdentity(first) error = %v", err)
		}
		secondRoot := t.TempDir()
		secondIdentity, err := compozyworkspace.EnsureIdentity(ctx, secondRoot)
		if err != nil {
			t.Fatalf("EnsureIdentity(second) error = %v", err)
		}
		firstEntered := make(chan struct{})
		releaseFirst := make(chan struct{})
		const (
			firstRegistrationID  = "ws_2123456789abcdef"
			secondRegistrationID = "ws_3123456789abcdef"
		)
		pool, err := NewPool(func(_ context.Context, workspaceID string) (ResolvedRoot, error) {
			switch workspaceID {
			case firstRegistrationID:
				close(firstEntered)
				<-releaseFirst
				return ResolvedRoot{RootDir: firstRoot, WorkspaceID: firstIdentity.WorkspaceID}, nil
			case secondRegistrationID:
				return ResolvedRoot{RootDir: secondRoot, WorkspaceID: secondIdentity.WorkspaceID}, nil
			default:
				return ResolvedRoot{}, fmt.Errorf("unexpected workspace %q", workspaceID)
			}
		})
		if err != nil {
			t.Fatalf("NewPool() error = %v", err)
		}
		t.Cleanup(func() {
			if err := pool.Close(ctx); err != nil {
				t.Errorf("Pool.Close() error = %v", err)
			}
		})

		firstResult := make(chan error, 1)
		go func() {
			_, openErr := pool.Open(ctx, firstRegistrationID)
			firstResult <- openErr
		}()
		<-firstEntered

		secondResult := make(chan error, 1)
		go func() {
			_, openErr := pool.Open(ctx, secondRegistrationID)
			secondResult <- openErr
		}()
		var secondErr error
		select {
		case secondErr = <-secondResult:
		case <-ctx.Done():
			secondErr = errors.New("Pool.Open(second) waited for an unrelated workspace")
		}
		close(releaseFirst)
		if secondErr != nil {
			t.Fatal(secondErr)
		}
		if openErr := <-firstResult; openErr != nil {
			t.Fatalf("Pool.Open(first) error = %v", openErr)
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

func assertTerminalSchema(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()
	for _, table := range []string{"terminal_commands", "terminal_artifacts", "terminal_recordings"} {
		var count int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info(?)
			WHERE name = 'profile_id' AND type = 'TEXT' AND "notnull" = 1`, table).Scan(&count); err != nil {
			t.Fatalf("Inspect %s.profile_id error = %v", table, err)
		}
		if count != 1 {
			t.Fatalf("%s.profile_id schema count = %d, want 1", table, count)
		}
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master
			WHERE type = 'trigger' AND tbl_name = ? AND name = ?`,
			table, table+"_profile_owner_immutable").Scan(&count); err != nil {
			t.Fatalf("Inspect %s profile trigger error = %v", table, err)
		}
		if count != 1 {
			t.Fatalf("%s profile trigger count = %d, want 1", table, count)
		}
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('terminal_commands')
		WHERE name = 'id' AND type = 'TEXT' AND "notnull" = 1 AND pk = 1`).Scan(&count); err != nil {
		t.Fatalf("Inspect terminal_commands.id error = %v", err)
	}
	if count != 1 {
		t.Fatalf("terminal_commands.id schema count = %d, want NOT NULL primary key", count)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('terminal_commands')
		WHERE name = 'output_tail_json'`).Scan(&count); err != nil {
		t.Fatalf("Inspect terminal_commands.output_tail_json error = %v", err)
	}
	if count != 0 {
		t.Fatalf("terminal_commands.output_tail_json schema count = %d, want no durable output tail", count)
	}
}

func assertTerminalRowCounts(ctx context.Context, t *testing.T, db *sql.DB, want int) {
	t.Helper()
	for _, table := range []string{"terminal_commands", "terminal_artifacts", "terminal_recordings"} {
		var count int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
			t.Fatalf("Count %s error = %v", table, err)
		}
		if count != want {
			t.Fatalf("%s count = %d, want %d", table, count, want)
		}
	}
}
