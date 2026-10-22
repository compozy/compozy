package workspacedb

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	compozyworkspace "github.com/compozy/compozy/internal/workspace"
)

func TestOpen(t *testing.T) {
	t.Run("Should own its migration stream and preserve data across reopen", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		workspaceRoot := t.TempDir()
		options := Options{HomeDir: t.TempDir(), WorkspaceID: "workspace-a", WorkspaceRoot: workspaceRoot}
		db := openWorkspaceTestDB(ctx, t, options)

		if got, want := db.Path(), filepath.Join(
			options.HomeDir,
			"workspaces",
			options.WorkspaceID,
			store.GlobalDatabaseName,
		); got != want {
			t.Fatalf("Path() = %q, want %q", got, want)
		}
		if !compozyworkspace.IsWorkspaceID(db.WorkspaceID()) {
			t.Fatalf("WorkspaceID() = %q, want canonical ULID", db.WorkspaceID())
		}
		firstStatus, err := store.Status(ctx, db.DB(), MigrationStream())
		if err != nil {
			t.Fatalf("Status(first) error = %v", err)
		}
		if firstStatus.Version != 5 || firstStatus.AppliedCount != 5 {
			t.Fatalf("Status(first) = %#v, want version/applied count 5", firstStatus)
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

		reopened := openWorkspaceTestDB(ctx, t, options)
		secondStatus, err := store.Status(ctx, reopened.DB(), MigrationStream())
		if err != nil {
			t.Fatalf("Status(reopen) error = %v", err)
		}
		if secondStatus != firstStatus {
			t.Fatalf("Status(reopen) = %#v, want unchanged %#v", secondStatus, firstStatus)
		}
		assertTerminalRowCounts(ctx, t, reopened.DB(), 1)
		for _, suffix := range []string{"", "-wal", "-shm"} {
			path := filepath.Join(workspaceRoot, ".compozy", store.GlobalDatabaseName) + suffix
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("workspace SQLite file %q: %v, want not exist", path, err)
			}
		}
		if _, err := reopened.DB().ExecContext(
			ctx,
			`UPDATE terminal_commands SET profile_id = 'profile-2' WHERE id = 'cmd-1'`,
		); err == nil || !strings.Contains(err.Error(), "profile_owner_immutable") {
			t.Fatalf("Update terminal_commands.profile_id error = %v, want profile_owner_immutable", err)
		}
	})

	t.Run("Should leave the retired workspace database untouched", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		options := Options{HomeDir: t.TempDir(), WorkspaceID: "workspace-a", WorkspaceRoot: t.TempDir()}
		identity, err := compozyworkspace.EnsureIdentity(ctx, options.WorkspaceRoot)
		if err != nil {
			t.Fatal(err)
		}
		oldPath := filepath.Join(filepath.Dir(identity.Path), store.GlobalDatabaseName)
		const canary = "retired workspace database must not be opened or modified"
		if err := os.WriteFile(oldPath, []byte(canary), 0o600); err != nil {
			t.Fatal(err)
		}
		db := openWorkspaceTestDB(ctx, t, options)
		assertTerminalRowCounts(ctx, t, db.DB(), 0)
		contents, err := os.ReadFile(oldPath)
		if err != nil || string(contents) != canary {
			t.Fatalf("retired database = %q, %v; want unchanged canary", contents, err)
		}
	})

	t.Run("Should surface the shared engine ahead-schema error", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		workspaceRoot := t.TempDir()
		options := Options{HomeDir: t.TempDir(), WorkspaceID: "workspace-a", WorkspaceRoot: workspaceRoot}
		db := openWorkspaceTestDB(ctx, t, options)
		if _, err := db.DB().ExecContext(
			ctx,
			`INSERT INTO goose_db_version_workspace (version_id, is_applied) VALUES (99, 1)`,
		); err != nil {
			t.Fatalf("Insert future migration error = %v", err)
		}
		if err := db.Close(ctx); err != nil {
			t.Fatalf("Close() error = %v", err)
		}

		_, err := Open(ctx, options)
		if !errors.Is(err, store.ErrSchemaAhead) {
			t.Fatalf("Open(ahead schema) error = %v, want ErrSchemaAhead", err)
		}
		if err == nil || !strings.Contains(err.Error(), "install a newer CompozyOS binary") {
			t.Fatalf("Open(ahead schema) error = %v, want deterministic remediation", err)
		}
	})

	t.Run("Should isolate rows across workspaces and daemon homes", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		homeDir := t.TempDir()
		first := openWorkspaceTestDB(
			ctx,
			t,
			Options{HomeDir: homeDir, WorkspaceID: "workspace-a", WorkspaceRoot: t.TempDir()},
		)
		second := openWorkspaceTestDB(
			ctx,
			t,
			Options{HomeDir: homeDir, WorkspaceID: "workspace-b", WorkspaceRoot: t.TempDir()},
		)
		otherHome := openWorkspaceTestDB(ctx, t, Options{
			HomeDir: t.TempDir(), WorkspaceID: "workspace-a", WorkspaceRoot: first.WorkspaceRoot(),
		})
		for _, db := range []*DB{first, second, otherHome} {
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
		assertWorkspaceRecordCount(ctx, t, otherHome.DB(), "first", 0)
		assertWorkspaceRecordCount(ctx, t, otherHome.DB(), "second", 0)
	})

	t.Run("Should support idempotent close", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db, err := Open(ctx, Options{HomeDir: t.TempDir(), WorkspaceID: "workspace-a", WorkspaceRoot: t.TempDir()})
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		if db.DB() == nil || db.WorkspaceRoot() == "" || db.Path() == "" || db.WorkspaceID() == "" {
			t.Fatalf("Open() returned incomplete database: %#v", db)
		}
		if err := db.Close(ctx); err != nil {
			t.Fatalf("Close(first) error = %v", err)
		}
		if err := db.Close(ctx); err != nil {
			t.Fatalf("Close(second) error = %v", err)
		}
	})

	// Invariant: close tolerates retained snapshots and preserves committed data; owner: workspace lifecycle, TestOpen.
	t.Run("Should close with a retained reader and preserve later writes across reopen", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		workspaceRoot := t.TempDir()
		options := Options{HomeDir: t.TempDir(), WorkspaceID: "workspace-a", WorkspaceRoot: workspaceRoot}
		db := openWorkspaceTestDB(ctx, t, options)
		if _, err := db.DB().ExecContext(ctx, `INSERT INTO terminal_recordings (
			id, terminal_id, profile_id, digest, path, started_at, bytes, expires_at
		) VALUES ('rec-before', 'term-1', 'profile-1', 'digest-before', '/tmp/before', 10, 12, 100)`); err != nil {
			t.Fatal(err)
		}
		readerDB, err := store.OpenSQLiteDatabase(ctx, db.Path(), nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := readerDB.Close(); err != nil {
				t.Error(err)
			}
		})
		if err := store.Checkpoint(ctx, db.DB()); err != nil {
			t.Fatal(err)
		}
		reader, err := readerDB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := reader.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				t.Error(err)
			}
		})
		var count int
		if err := reader.QueryRowContext(ctx, `SELECT COUNT(*) FROM terminal_recordings`).
			Scan(&count); err != nil ||
			count != 1 {
			t.Fatalf("reader recordings = %d, %v; want one", count, err)
		}
		if _, err := db.DB().ExecContext(ctx, `INSERT INTO terminal_recordings (
			id, terminal_id, profile_id, digest, path, started_at, bytes, expires_at
		) VALUES ('rec-after', 'term-1', 'profile-1', 'digest-after', '/tmp/after', 20, 24, 100)`); err != nil {
			t.Fatal(err)
		}
		closeCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		if err := db.Close(closeCtx); err != nil {
			t.Fatalf("Close with retained reader error = %v", err)
		}
		if err := reader.QueryRowContext(ctx, `SELECT COUNT(*) FROM terminal_recordings`).
			Scan(&count); err != nil ||
			count != 1 {
			t.Fatalf("retained snapshot after close = %d, %v; want one", count, err)
		}
		if err := reader.Rollback(); err != nil {
			t.Fatal(err)
		}
		reopened := openWorkspaceTestDB(ctx, t, options)
		var bytes int
		if err := reopened.DB().QueryRowContext(ctx, `SELECT bytes FROM terminal_recordings WHERE id = 'rec-after'`).
			Scan(&bytes); err != nil || bytes != 24 {
			t.Fatalf("recording committed after snapshot = %d bytes, %v; want 24", bytes, err)
		}
	})

	t.Run("Should reject invalid open and close inputs", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		if _, err := Open(ctx, Options{WorkspaceRoot: "   "}); err == nil {
			t.Fatal("Open(blank root) error = nil, want validation error")
		}
		if _, err := NewPool("", func(context.Context, string) (ResolvedRoot, error) {
			return ResolvedRoot{}, nil
		}); err == nil || !strings.Contains(err.Error(), "absolute home directory is required") {
			t.Fatalf("NewPool(blank home) error = %v, want absolute home validation error", err)
		}
		var nilDB *DB
		if nilDB.Path() != "" || nilDB.WorkspaceID() != "" || nilDB.WorkspaceRoot() != "" || nilDB.DB() != nil {
			t.Fatal("nil DB accessors returned non-zero values")
		}
		if err := nilDB.Close(ctx); err != nil {
			t.Fatalf("nil DB Close() error = %v", err)
		}
	})

	for _, workspaceID := range []string{"", ".", "..", "../other", "other/nested", `other\nested`, "C:other"} {
		t.Run("Should reject unsafe workspace storage id "+workspaceID, func(t *testing.T) {
			t.Parallel()
			_, err := Open(t.Context(), Options{
				HomeDir: t.TempDir(), WorkspaceID: workspaceID, WorkspaceRoot: t.TempDir(),
			})
			if err == nil || !strings.Contains(err.Error(), "canonical workspace id is required") {
				t.Fatalf("Open(unsafe workspace id) error = %v, want canonical workspace id validation error", err)
			}
		})
	}

	t.Run("Should reuse and close per-workspace pool handles", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		workspaceRoot := t.TempDir()
		identity, err := compozyworkspace.EnsureIdentity(ctx, workspaceRoot)
		if err != nil {
			t.Fatalf("EnsureIdentity() error = %v", err)
		}
		const registrationID = "ws_0123456789abcdef"
		pool, err := NewPool(t.TempDir(), func(_ context.Context, workspaceID string) (ResolvedRoot, error) {
			if workspaceID != registrationID {
				return ResolvedRoot{}, errors.New("unexpected workspace")
			}
			return ResolvedRoot{RootDir: workspaceRoot, WorkspaceID: identity.WorkspaceID}, nil
		})
		if err != nil {
			t.Fatalf("NewPool() error = %v", err)
		}
		t.Cleanup(func() {
			if err := pool.Close(testutil.Context(t)); err != nil {
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
		pool, err := NewPool(t.TempDir(), func(context.Context, string) (ResolvedRoot, error) {
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
		pool, err := NewPool(t.TempDir(), func(context.Context, string) (ResolvedRoot, error) {
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

	t.Run("Should remove workspace database files after restart without the project root", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		workspaceRoot := t.TempDir()
		identity, err := compozyworkspace.EnsureIdentity(ctx, workspaceRoot)
		if err != nil {
			t.Fatalf("EnsureIdentity() error = %v", err)
		}
		const registrationID = "ws_1123456789abcdef"
		homeDir := t.TempDir()
		pool, err := NewPool(homeDir, func(_ context.Context, workspaceID string) (ResolvedRoot, error) {
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
		if err := pool.Close(ctx); err != nil {
			t.Fatalf("Pool.Close() error = %v", err)
		}
		sibling := openWorkspaceTestDB(ctx, t, Options{
			HomeDir: homeDir, WorkspaceID: "workspace-b", WorkspaceRoot: t.TempDir(),
		})
		if err := os.RemoveAll(workspaceRoot); err != nil {
			t.Fatal(err)
		}
		restarted, err := NewPool(homeDir, func(context.Context, string) (ResolvedRoot, error) {
			return ResolvedRoot{}, errors.New("workspace registration no longer exists")
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := restarted.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		if err := restarted.RemoveWorkspace(ctx, registrationID); err != nil {
			t.Fatalf("Pool.RemoveWorkspace() error = %v", err)
		}
		for _, suffix := range []string{"", "-wal", "-shm"} {
			if _, err := os.Stat(dbPath + suffix); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("Stat(%q) error = %v, want %v", dbPath+suffix, err, os.ErrNotExist)
			}
		}
		if err := sibling.DB().PingContext(ctx); err != nil {
			t.Fatalf("sibling database unavailable after removal: %v", err)
		}
		if _, err := os.Stat(sibling.Path()); err != nil {
			t.Fatalf("sibling database missing after removal: %v", err)
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
		pool, err := NewPool(t.TempDir(), func(_ context.Context, workspaceID string) (ResolvedRoot, error) {
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
			if err := pool.Close(testutil.Context(t)); err != nil {
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

func openWorkspaceTestDB(ctx context.Context, t *testing.T, options Options) *DB {
	t.Helper()
	db, err := Open(ctx, options)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(testutil.Context(t)); err != nil {
			t.Errorf("DB.Close() error = %v", err)
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

func TestSweepExpiredTerminalFiles(t *testing.T) {
	t.Run("Should remove expired recordings while retaining commands and shared artifacts", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db := openWorkspaceTestDB(
			ctx,
			t,
			Options{HomeDir: t.TempDir(), WorkspaceID: "workspace-a", WorkspaceRoot: t.TempDir()},
		)
		if _, err := db.DB().ExecContext(ctx, `INSERT INTO terminal_recordings (
			id, terminal_id, profile_id, digest, path, started_at, bytes, expires_at
		) VALUES ('expired', 'terminal', 'profile', 'digest', '/expired', 1, 10, 100),
			('retained', 'terminal', 'profile', 'digest', '/retained', 2, 10, 200)`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.DB().ExecContext(ctx, `INSERT INTO terminal_commands (
			id, profile_id, actor_kind, actor_id, command, cwd, started_at,
			exit_cause, detected_by, approval, output_bytes, truncated, recording_id
		) VALUES ('command', 'profile', 'human', 'operator', 'pwd', '/tmp', 1,
			'exited', 'exact', 'human', 1, 0, 'expired')`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.DB().ExecContext(ctx, `INSERT INTO terminal_artifacts (
			id, command_id, profile_id, digest, path, bytes, expires_at
		) VALUES ('expired', 'command', 'profile', 'digest', '/shared', 1, 100),
			('retained', 'command', 'profile', 'digest', '/shared', 1, 200)`); err != nil {
			t.Fatal(err)
		}
		var removed []string
		if err := db.SweepExpiredTerminalFiles(ctx, 100, func(path string) error {
			removed = append(removed, path)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if len(removed) != 1 || removed[0] != "/expired" {
			t.Fatalf("removed files = %v, want only /expired", removed)
		}
		assertTerminalRowCounts(ctx, t, db.DB(), 1)
		var recordingID sql.NullString
		if err := db.DB().
			QueryRowContext(ctx, `SELECT recording_id FROM terminal_commands WHERE id = 'command'`).
			Scan(&recordingID); err != nil {
			t.Fatal(err)
		}
		if recordingID.Valid {
			t.Fatalf("expired recording remains linked: %q", recordingID.String)
		}
	})

	t.Run("Should leave the writer available when no terminal files have expired", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db := openWorkspaceTestDB(
			ctx,
			t,
			Options{HomeDir: t.TempDir(), WorkspaceID: "workspace-a", WorkspaceRoot: t.TempDir()},
		)
		writer, err := db.DB().Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := writer.Close(); err != nil {
				t.Error(err)
			}
		})
		if _, err := writer.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
			t.Fatal(err)
		}
		sweepCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()
		sweepErr := db.SweepExpiredTerminalFiles(sweepCtx, 100, func(path string) error {
			t.Errorf("unexpected removal: %s", path)
			return nil
		})
		if _, err := writer.ExecContext(ctx, "ROLLBACK"); err != nil {
			t.Fatal(err)
		}
		if sweepErr != nil {
			t.Fatalf("empty sweep waited for writer: %v", sweepErr)
		}
	})
}

func TestLegacyJournalImport(t *testing.T) {
	t.Run(
		"Should resume after a corrupt blob without duplicating rows or touching existing User data",
		func(t *testing.T) {
			t.Parallel()
			home := t.TempDir()
			ctx := t.Context()
			source := openWorkspaceTestDB(
				ctx,
				t,
				Options{HomeDir: home, WorkspaceID: "legacy", WorkspaceRoot: t.TempDir()},
			)
			root := filepath.Dir(source.Path())
			directory := filepath.Join(root, "terminal-recordings")
			if err := os.MkdirAll(directory, 0o700); err != nil {
				t.Fatal(err)
			}
			contents := []byte("recording bytes")
			digest := fmt.Sprintf("%x", sha256.Sum256(contents))
			for _, profile := range []string{"p1", "p2"} {
				path := filepath.Join(directory, profile)
				if err := os.WriteFile(path, contents, 0o600); err != nil {
					t.Fatal(err)
				}
				if _, err := source.DB().
					ExecContext(ctx, `INSERT INTO terminal_recordings VALUES (?,? ,?, ?, ?, 1,2,?,99)`, profile, "terminal", profile, digest, path, len(contents)); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := source.DB().
				ExecContext(ctx, `INSERT INTO terminal_commands(id,profile_id,actor_kind,actor_id,command,cwd,started_at,exit_cause,detected_by,approval,output_bytes,truncated,recording_id) VALUES ('command','p1','human','operator','pwd','/tmp',1,'exited','exact','human',0,0,'p1')`); err != nil {
				t.Fatal(err)
			}
			artifactDir := filepath.Join(root, "terminal-artifacts")
			if err := os.MkdirAll(artifactDir, 0o700); err != nil {
				t.Fatal(err)
			}
			artifact := filepath.Join(artifactDir, "spill")
			if err := os.WriteFile(artifact, []byte("corrupt"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := source.DB().
				ExecContext(ctx, `INSERT INTO terminal_artifacts(id,command_id,profile_id,digest,path,bytes,expires_at) VALUES ('artifact','command','p1',?,?,?,99)`, digest, artifact, len(contents)); err != nil {
				t.Fatal(err)
			}
			if err := source.Close(ctx); err != nil {
				t.Fatal(err)
			}
			user := openWorkspaceTestDB(ctx, t, Options{HomeDir: home, WorkspaceID: "user-p1", WorkspaceRoot: home})
			if _, err := user.DB().
				ExecContext(ctx, `INSERT INTO terminal_recordings VALUES ('existing','terminal','p1','existing','existing',1,2,0,99)`); err != nil {
				t.Fatal(err)
			}
			if err := user.Close(ctx); err != nil {
				t.Fatal(err)
			}
			if err := ImportLegacyJournal(ctx, home, "legacy", nil); err == nil {
				t.Fatal("accepted corrupt artifact")
			}
			if _, err := os.Stat(root); err != nil {
				t.Fatal("source disappeared before verification", err)
			}
			if err := os.WriteFile(artifact, contents, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := ImportLegacyJournal(ctx, home, "legacy", nil); err != nil {
				t.Fatal(err)
			}
			if err := ImportLegacyJournal(ctx, home, "legacy", nil); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(root + ".migrated"); err != nil {
				t.Fatal(err)
			}
			for _, profile := range []string{"p1", "p2"} {
				target := openWorkspaceTestDB(
					ctx,
					t,
					Options{HomeDir: home, WorkspaceID: "user-" + profile, WorkspaceRoot: home},
				)
				var count int
				if err := target.DB().
					QueryRowContext(ctx, "SELECT count(*) FROM terminal_recordings WHERE id=?", profile).
					Scan(&count); err != nil ||
					count != 1 {
					t.Fatalf("count=%d err=%v", count, err)
				}
				var path string
				if err := target.DB().
					QueryRowContext(ctx, "SELECT path FROM terminal_recordings WHERE id=?", profile).
					Scan(&path); err != nil {
					t.Fatal(err)
				}
				if !strings.HasPrefix(path, filepath.Dir(target.Path())+string(filepath.Separator)) {
					t.Fatal("blob not contained", path)
				}
				data, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(data, contents) {
					t.Fatalf("blob=%q err=%v", data, err)
				}
				if profile == "p1" {
					if err := target.DB().
						QueryRowContext(ctx, "SELECT count(*) FROM terminal_recordings WHERE id='existing' AND digest='existing'").
						Scan(&count); err != nil ||
						count != 1 {
						t.Fatal("existing row changed", err)
					}
					if err := target.DB().
						QueryRowContext(ctx, "SELECT count(*) FROM legacy_import").
						Scan(&count); err != nil ||
						count != 3 {
						t.Fatalf("ledger=%d err=%v", count, err)
					}
				}
			}
		},
	)
}
