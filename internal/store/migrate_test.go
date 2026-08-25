package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	atlasmigrate "ariga.io/atlas/sql/migrate"
	"github.com/compozy/compozy/internal/testutil"
)

//go:embed testdata/migrations/*
var migrationFixtureFS embed.FS

func TestApplyMigrationStream(t *testing.T) {
	t.Run("Should apply fresh migrations in order and report their state", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openEngineTestDB(t, "fresh.db")
		stream := fixtureMigrationStream("goose_db_version_global", "testdata/migrations/happy")
		if err := Apply(ctx, db, stream); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
		status, err := Status(ctx, db, stream)
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if status.Version != 2 || status.AppliedCount != 2 {
			t.Fatalf("Status() = %#v, want version 2 with 2 applied migrations", status)
		}
		if got := migrationItemValues(t, db); got != "first,second" {
			t.Fatalf("migration item values = %q, want first,second", got)
		}
	})

	t.Run("Should apply one explicit migration step without rebuilding the plan", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openEngineTestDB(t, "stepper.db")
		stream := fixtureMigrationStream("goose_db_version_stepper", "testdata/migrations/happy")
		stepper, err := NewMigrationStepper(ctx, db, stream)
		if err != nil {
			t.Fatalf("NewMigrationStepper() error = %v", err)
		}
		applied, err := stepper.UpTo(ctx, 1)
		if err != nil {
			t.Fatalf("UpTo(1) error = %v", err)
		}
		if applied != 1 || migrationItemValues(t, db) != "first" {
			t.Fatalf("UpTo(1) applied = %d, values = %q, want 1 and first", applied, migrationItemValues(t, db))
		}
		applied, err = stepper.UpTo(ctx, 2)
		if err != nil {
			t.Fatalf("UpTo(2) error = %v", err)
		}
		if applied != 1 || migrationItemValues(t, db) != "first,second" {
			t.Fatalf("UpTo(2) applied = %d, values = %q, want 1 and first,second", applied, migrationItemValues(t, db))
		}
	})

	t.Run("Should create only the configured stream version table", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openEngineTestDB(t, "own-table.db")
		stream := fixtureMigrationStream("goose_db_version_global", "testdata/migrations/happy")
		if err := Apply(ctx, db, stream); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
		for _, table := range []string{
			"goose_db_version_global",
			"goose_db_version_session",
			"goose_db_version_memory",
			"goose_db_version_workspace",
		} {
			got := tableExistsForTest(t, db, table)
			want := table == stream.VersionTable
			if got != want {
				t.Fatalf("table %s existence = %t, want %t", table, got, want)
			}
		}
	})

	t.Run("Should leave an idempotent reopen unchanged", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openEngineTestDB(t, "idempotent.db")
		stream := fixtureMigrationStream("goose_db_version_session", "testdata/migrations/happy")
		if err := Apply(ctx, db, stream); err != nil {
			t.Fatalf("Apply(first) error = %v", err)
		}
		before, err := Status(ctx, db, stream)
		if err != nil {
			t.Fatalf("Status(before) error = %v", err)
		}
		if err := Apply(ctx, db, stream); err != nil {
			t.Fatalf("Apply(second) error = %v", err)
		}
		after, err := Status(ctx, db, stream)
		if err != nil {
			t.Fatalf("Status(after) error = %v", err)
		}
		if before != after {
			t.Fatalf("status after reopen = %#v, want %#v", after, before)
		}
		if got := migrationItemValues(t, db); got != "first,second" {
			t.Fatalf("migration item values = %q, want first,second", got)
		}
	})

	t.Run("Should apply a no-transaction table rebuild and preserve rows", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openEngineTestDB(t, "no-transaction.db")
		stream := fixtureMigrationStream("goose_db_version_workspace", "testdata/migrations/no_transaction")
		if err := Apply(ctx, db, stream); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
		var value string
		var enabled int
		if err := db.QueryRowContext(ctx, `SELECT value, enabled FROM rebuild_items WHERE id = 1`).Scan(
			&value,
			&enabled,
		); err != nil {
			t.Fatalf("query rebuilt row: %v", err)
		}
		if value != "preserved" || enabled != 1 {
			t.Fatalf("rebuilt row = (%q, %d), want (preserved, 1)", value, enabled)
		}
	})

	t.Run("Should honor foreign key disabling in immutable migration history", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openEngineTestDB(t, "historical-foreign-key-rebuild.db")
		files := signedMigrationMapFS(t, map[string][]byte{
			"00001_rebuild.sql": []byte(`-- +goose Up
PRAGMA foreign_keys = OFF;
CREATE TABLE parents (id INTEGER PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE children (
  id INTEGER PRIMARY KEY,
  parent_id INTEGER NOT NULL REFERENCES parents(id) ON DELETE CASCADE
);
INSERT INTO parents (id, value) VALUES (1, 'before');
INSERT INTO children (id, parent_id) VALUES (1, 1);
CREATE TABLE new_parents (id INTEGER PRIMARY KEY, value TEXT NOT NULL, enabled INTEGER NOT NULL DEFAULT 1);
INSERT INTO new_parents (id, value) SELECT id, value FROM parents;
DROP TABLE parents;
ALTER TABLE new_parents RENAME TO parents;
PRAGMA foreign_keys = ON;
`),
		})
		stream := MigrationStream{
			Name:         "historical-foreign-key-rebuild",
			FS:           files,
			Dir:          ".",
			VersionTable: "goose_db_version_global",
		}
		if err := Apply(ctx, db, stream); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
		var parentID int
		if err := db.QueryRowContext(ctx, `SELECT parent_id FROM children WHERE id = 1`).Scan(&parentID); err != nil {
			t.Fatalf("query preserved child: %v", err)
		}
		if parentID != 1 {
			t.Fatalf("preserved child parent_id = %d, want 1", parentID)
		}
	})

	t.Run("Should discard a failed no-transaction migration connection", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), "failed-no-transaction.db")
		db, err := OpenSQLiteDatabase(ctx, path, nil)
		if err != nil {
			t.Fatalf("OpenSQLiteDatabase() error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := db.Close(); closeErr != nil {
				t.Errorf("Close() error = %v", closeErr)
			}
		})
		db.SetMaxOpenConns(2)
		db.SetMaxIdleConns(2)
		files := signedMigrationMapFS(t, map[string][]byte{
			"00001_fail.sql": []byte(`-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys = OFF;
BEGIN IMMEDIATE;
THIS IS NOT VALID SQL;
`),
		})
		stream := MigrationStream{
			Name:         "failed-no-transaction",
			FS:           files,
			Dir:          ".",
			VersionTable: "goose_db_version_failed_no_transaction",
		}
		if err := Apply(ctx, db, stream); err == nil {
			t.Fatal("Apply() error = nil, want failed migration")
		}

		connections := make([]*sql.Conn, 0, 2)
		for range 2 {
			conn, connErr := db.Conn(ctx)
			if connErr != nil {
				t.Fatalf("db.Conn() error = %v", connErr)
			}
			connections = append(connections, conn)
		}
		for index, conn := range connections {
			var foreignKeys int
			if err := conn.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
				t.Fatalf("query foreign_keys on connection %d: %v", index+1, err)
			}
			if foreignKeys != 1 {
				t.Errorf("foreign_keys on connection %d = %d, want 1", index+1, foreignKeys)
			}
			if err := conn.Close(); err != nil {
				t.Errorf("close connection %d: %v", index+1, err)
			}
		}
	})

	t.Run("Should reject a no-transaction migration when the pool has one connection", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openEngineTestDB(t, "no-transaction-one-connection.db")
		db.SetMaxOpenConns(1)
		stream := fixtureMigrationStream("goose_db_version_workspace", "testdata/migrations/no_transaction")
		err := Apply(ctx, db, stream)
		if !errors.Is(err, ErrMigrationConnectionCapacity) {
			t.Fatalf("Apply() error = %v, want ErrMigrationConnectionCapacity", err)
		}
		if tableExistsForTest(t, db, stream.VersionTable) || tableExistsForTest(t, db, "rebuild_items") {
			t.Fatal("non-transaction migration changed the database after connection-capacity refusal")
		}
	})

	t.Run("Should reject a database ahead of the embedded head", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openEngineTestDB(t, "ahead.db")
		stream := fixtureMigrationStream("goose_db_version_global", "testdata/migrations/happy")
		seedGooseVersion(t, db, stream.VersionTable, 99)
		err := Apply(ctx, db, stream)
		if !errors.Is(err, ErrSchemaAhead) {
			t.Fatalf("Apply() error = %v, want ErrSchemaAhead", err)
		}
		if !strings.Contains(err.Error(), "install a newer CompozyOS binary") ||
			!strings.Contains(err.Error(), migrationFamilyRecoveryInstruction) {
			t.Fatalf("Apply() error = %q, want newer-binary or whole-family recovery remediation", err)
		}
		if got := latestSeededVersion(t, db, stream.VersionTable); got != 99 {
			t.Fatalf("version table head = %d, want 99", got)
		}
		if tableExistsForTest(t, db, "migration_items") {
			t.Fatal("migration_items exists after ahead-version refusal")
		}
	})
}

func TestApplyRejectsAtlasSumDrift(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, files fstest.MapFS)
	}{
		{
			name: "Should reject an edited migration",
			mutate: func(t *testing.T, files fstest.MapFS) {
				t.Helper()
				files["00001_create.sql"].Data = []byte("-- +goose Up\nCREATE TABLE edited (id INTEGER);")
			},
		},
		{
			name: "Should reject an added migration",
			mutate: func(t *testing.T, files fstest.MapFS) {
				t.Helper()
				files["00003_added.sql"] = &fstest.MapFile{Data: []byte("-- +goose Up\nSELECT 1;")}
			},
		},
		{
			name: "Should reject a removed migration",
			mutate: func(t *testing.T, files fstest.MapFS) {
				t.Helper()
				delete(files, "00002_index.sql")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			files := copyFixtureDirectory(t, "testdata/migrations/happy")
			tt.mutate(t, files)
			db := openEngineTestDB(t, strings.ReplaceAll(tt.name, " ", "_")+".db")
			stream := MigrationStream{
				Name:         "tampered",
				FS:           files,
				Dir:          ".",
				VersionTable: "goose_db_version_tampered",
			}
			err := Apply(testutil.Context(t), db, stream)
			if !errors.Is(err, atlasmigrate.ErrChecksumMismatch) {
				t.Fatalf("Apply() error = %v, want Atlas checksum mismatch", err)
			}
			if tableExistsForTest(t, db, stream.VersionTable) {
				t.Fatalf("version table %s exists after sum refusal", stream.VersionTable)
			}
		})
	}
}

func TestApplyRejectsLegacyDatabase(t *testing.T) {
	tests := []struct {
		name        string
		legacyTable string
		streamName  string
	}{
		{name: "Should reject the global legacy table", legacyTable: "schema_migrations", streamName: "global"},
		{name: "Should reject the memory legacy table", legacyTable: "memory_schema_migrations", streamName: "memory"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), tt.streamName+".db")
			db := openSQLitePathForTest(t, path)
			if _, err := db.ExecContext(
				testutil.Context(t),
				`CREATE TABLE `+quoteIdentifier(tt.legacyTable)+` (version INTEGER)`,
			); err != nil {
				t.Fatalf("seed legacy table: %v", err)
			}
			stream := fixtureMigrationStream("goose_db_version_"+tt.streamName, "testdata/migrations/happy")
			stream.Name = tt.streamName
			stream.LegacyTables = []string{tt.legacyTable}
			err := Apply(testutil.Context(t), db, stream)
			if !errors.Is(err, ErrLegacyDatabase) {
				t.Fatalf("Apply() error = %v, want ErrLegacyDatabase", err)
			}
			reportedPath, pathErr := filepath.EvalSymlinks(path)
			if pathErr != nil {
				t.Fatalf("resolve database path: %v", pathErr)
			}
			if !strings.Contains(err.Error(), reportedPath) ||
				!strings.Contains(err.Error(), migrationFamilyRecoveryInstruction) {
				t.Fatalf("Apply() error = %q, want path and whole-family recovery remediation", err)
			}
			if tableExistsForTest(t, db, stream.VersionTable) {
				t.Fatalf("version table %s exists after legacy refusal", stream.VersionTable)
			}
		})
	}
}

func TestApplyLegacyRefusalLeavesDatabaseUntouched(t *testing.T) {
	t.Run("Should leave a refused legacy database byte-identical", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "legacy-untouched.db")
		seed := openSQLitePathForTest(t, path)
		if _, err := seed.ExecContext(
			testutil.Context(t),
			`CREATE TABLE schema_migrations (version INTEGER)`,
		); err != nil {
			t.Fatalf("seed legacy table: %v", err)
		}
		if err := seed.Close(); err != nil {
			t.Fatalf("close seeded database: %v", err)
		}
		before := fileDigest(t, path)
		beforeWAL := filePresence(t, path+"-wal")
		beforeSHM := filePresence(t, path+"-shm")

		db := openSQLitePathForTest(t, path)
		stream := fixtureMigrationStream("goose_db_version_global", "testdata/migrations/happy")
		stream.LegacyTables = []string{"schema_migrations"}
		err := Apply(testutil.Context(t), db, stream)
		if !errors.Is(err, ErrLegacyDatabase) {
			t.Fatalf("Apply() error = %v, want ErrLegacyDatabase", err)
		}
		if err := db.Close(); err != nil {
			t.Fatalf("close refused database: %v", err)
		}
		after := fileDigest(t, path)
		if before != after {
			t.Fatalf("database digest after refusal = %x, want %x", after, before)
		}
		if got := filePresence(t, path+"-wal"); got != beforeWAL {
			t.Fatalf("WAL presence after refusal = %t, want %t", got, beforeWAL)
		}
		if got := filePresence(t, path+"-shm"); got != beforeSHM {
			t.Fatalf("SHM presence after refusal = %t, want %t", got, beforeSHM)
		}
	})
}

func TestMigrationStreamValidation(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T, stream *MigrationStream, db **sql.DB, nilContext *bool)
	}{
		{
			name: "Should reject a nil context",
			prepare: func(t *testing.T, _ *MigrationStream, _ **sql.DB, nilContext *bool) {
				t.Helper()
				*nilContext = true
			},
		},
		{
			name: "Should reject a nil database",
			prepare: func(t *testing.T, _ *MigrationStream, db **sql.DB, _ *bool) {
				t.Helper()
				*db = nil
			},
		},
		{
			name: "Should reject an empty stream name",
			prepare: func(t *testing.T, stream *MigrationStream, _ **sql.DB, _ *bool) {
				t.Helper()
				stream.Name = ""
			},
		},
		{
			name: "Should reject a nil filesystem",
			prepare: func(t *testing.T, stream *MigrationStream, _ **sql.DB, _ *bool) {
				t.Helper()
				stream.FS = nil
			},
		},
		{
			name: "Should reject an empty migration directory",
			prepare: func(t *testing.T, stream *MigrationStream, _ **sql.DB, _ *bool) {
				t.Helper()
				stream.Dir = ""
			},
		},
		{
			name: "Should reject an invalid version table",
			prepare: func(t *testing.T, stream *MigrationStream, _ **sql.DB, _ *bool) {
				t.Helper()
				stream.VersionTable = "invalid-table"
			},
		},
		{
			name: "Should reject an invalid legacy table",
			prepare: func(t *testing.T, stream *MigrationStream, _ **sql.DB, _ *bool) {
				t.Helper()
				stream.LegacyTables = []string{"invalid-table"}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stream := fixtureMigrationStream("goose_db_version_validation", "testdata/migrations/happy")
			db := openEngineTestDB(t, strings.ReplaceAll(tt.name, " ", "_")+".db")
			nilContext := false
			tt.prepare(t, &stream, &db, &nilContext)
			ctx := testutil.Context(t)
			if nilContext {
				ctx = nil
			}
			if err := Apply(ctx, db, stream); err == nil {
				t.Fatal("Apply() error = nil, want input validation failure")
			}
		})
	}
}

func TestMigrationDirectoryValidation(t *testing.T) {
	t.Run("Should reject a directory without SQL migrations", func(t *testing.T) {
		t.Parallel()

		stream := MigrationStream{
			Name:         "empty",
			FS:           fstest.MapFS{"atlas.sum": &fstest.MapFile{Data: []byte("h1:invalid\n")}},
			Dir:          ".",
			VersionTable: "goose_db_version_empty",
		}
		if err := Apply(testutil.Context(t), openEngineTestDB(t, "empty-dir.db"), stream); err == nil {
			t.Fatal("Apply() error = nil, want no-SQL validation failure")
		}
	})

	t.Run("Should reject a malformed migration filename", func(t *testing.T) {
		t.Parallel()

		files := signedMigrationMapFS(t, map[string][]byte{
			"invalid.sql": []byte("-- +goose Up\nSELECT 1;"),
		})
		stream := MigrationStream{
			Name:         "bad-filename",
			FS:           files,
			Dir:          ".",
			VersionTable: "goose_db_version_bad_filename",
		}
		if err := Apply(testutil.Context(t), openEngineTestDB(t, "bad-filename.db"), stream); err == nil {
			t.Fatal("Apply() error = nil, want filename validation failure")
		}
	})

	t.Run("Should reject a migration directory without atlas.sum", func(t *testing.T) {
		t.Parallel()

		stream := MigrationStream{
			Name: "missing-sum",
			FS: fstest.MapFS{
				"00001_baseline.sql": &fstest.MapFile{Data: []byte("-- +goose Up\nSELECT 1;")},
			},
			Dir:          ".",
			VersionTable: "goose_db_version_missing_sum",
		}
		err := Apply(testutil.Context(t), openEngineTestDB(t, "missing-sum.db"), stream)
		if !errors.Is(err, atlasmigrate.ErrChecksumNotFound) {
			t.Fatalf("Apply() error = %v, want ErrChecksumNotFound", err)
		}
	})

	t.Run("Should leave no applied version after invalid migration SQL", func(t *testing.T) {
		t.Parallel()

		files := signedMigrationMapFS(t, map[string][]byte{
			"00001_invalid.sql": []byte("-- +goose Up\nCREATE TABLE"),
		})
		stream := MigrationStream{
			Name:         "invalid-sql",
			FS:           files,
			Dir:          ".",
			VersionTable: "goose_db_version_invalid_sql",
		}
		db := openEngineTestDB(t, "invalid-sql.db")
		if err := Apply(testutil.Context(t), db, stream); err == nil {
			t.Fatal("Apply() error = nil, want SQL failure")
		}
		status, err := Status(testutil.Context(t), db, stream)
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if status.Version != 0 || status.AppliedCount != 0 {
			t.Fatalf("Status() = %#v, want no applied migration", status)
		}
	})
}

func TestApplyMigrationBootstrapRollbackErrors(t *testing.T) {
	t.Run("Should roll back declarative schema when instance initialization fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openEngineTestDB(t, "bootstrap-initializer-rollback.db")
		initializerErr := errors.New("instance initializer failed")
		err := applyMigrationBootstrap(
			ctx,
			db,
			"goose_db_version_bootstrap_initializer",
			[]int64{1},
			[]migrationFile{{
				path:     "schema.sql",
				contents: []byte(`CREATE TABLE bootstrap_owner (id TEXT PRIMARY KEY)`),
			}},
			nil,
			func(ctx context.Context, tx *sql.Tx) error {
				if _, err := tx.ExecContext(ctx, `INSERT INTO bootstrap_owner (id) VALUES ('owner')`); err != nil {
					return err
				}
				return initializerErr
			},
		)
		if !errors.Is(err, initializerErr) {
			t.Fatalf("applyMigrationBootstrap() error = %v, want initializer failure", err)
		}
		if tableExistsForTest(t, db, "bootstrap_owner") ||
			tableExistsForTest(t, db, "goose_db_version_bootstrap_initializer") {
			t.Fatal("failed bootstrap initializer committed schema or version state")
		}
	})

	tests := []struct {
		name                    string
		dataSource              string
		wantPrimaryError        error
		wantRollbackError       error
		wantRollbackErrorInText bool
	}{
		{
			name:             "Should suppress an already-finished transaction rollback",
			dataSource:       "finished",
			wantPrimaryError: errMigrationBootstrapCommit,
		},
		{
			name:                    "Should retain and contextualize a real rollback failure",
			dataSource:              "failed",
			wantPrimaryError:        errMigrationBootstrapPrimary,
			wantRollbackError:       errMigrationBootstrapRollback,
			wantRollbackErrorInText: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			db := openMigrationBootstrapRollbackDB(t, test.dataSource)
			err := applyMigrationBootstrap(
				testutil.Context(t),
				db,
				"goose_db_version_bootstrap",
				nil,
				nil,
				nil,
				nil,
			)
			if !errors.Is(err, test.wantPrimaryError) {
				t.Fatalf("applyMigrationBootstrap() error = %v, want primary error", err)
			}
			if errors.Is(err, sql.ErrTxDone) {
				t.Fatalf("applyMigrationBootstrap() error = %v, must suppress sql.ErrTxDone", err)
			}
			if test.wantRollbackError != nil && !errors.Is(err, test.wantRollbackError) {
				t.Fatalf(
					"applyMigrationBootstrap() error = %v, rollback error match = false, want true",
					err,
				)
			}
			if test.wantRollbackErrorInText && !strings.Contains(err.Error(), "rollback migration bootstrap") {
				t.Fatalf(
					"applyMigrationBootstrap() error = %q, want rollback migration bootstrap context",
					err,
				)
			}
		})
	}
}

var (
	errMigrationBootstrapPrimary                 = errors.New("bootstrap SQL execution failed")
	errMigrationBootstrapCommit                  = errors.New("bootstrap commit failed")
	errMigrationBootstrapRollback                = errors.New("bootstrap rollback failed")
	migrationBootstrapRollbackDriverRegistration sync.Once
)

const migrationBootstrapRollbackDriverName = "compozy-migration-bootstrap-rollback"

type migrationBootstrapRollbackDriver struct{}

func (migrationBootstrapRollbackDriver) Open(name string) (driver.Conn, error) {
	switch name {
	case "finished":
		return migrationBootstrapRollbackConn{commitErr: errMigrationBootstrapCommit}, nil
	case "failed":
		return migrationBootstrapRollbackConn{
			execErr:     errMigrationBootstrapPrimary,
			rollbackErr: errMigrationBootstrapRollback,
		}, nil
	default:
		return nil, fmt.Errorf("migration bootstrap rollback driver: unsupported data source %q", name)
	}
}

type migrationBootstrapRollbackConn struct {
	execErr     error
	commitErr   error
	rollbackErr error
}

func (c migrationBootstrapRollbackConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("migration bootstrap rollback driver does not prepare statements")
}

func (migrationBootstrapRollbackConn) Close() error {
	return nil
}

func (c migrationBootstrapRollbackConn) Begin() (driver.Tx, error) {
	return migrationBootstrapRollbackTx{
		commitErr:   c.commitErr,
		rollbackErr: c.rollbackErr,
	}, nil
}

func (c migrationBootstrapRollbackConn) BeginTx(
	ctx context.Context,
	_ driver.TxOptions,
) (driver.Tx, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.Begin()
}

func (c migrationBootstrapRollbackConn) ExecContext(
	ctx context.Context,
	_ string,
	_ []driver.NamedValue,
) (driver.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.execErr != nil {
		return nil, c.execErr
	}
	return driver.RowsAffected(1), nil
}

type migrationBootstrapRollbackTx struct {
	commitErr   error
	rollbackErr error
}

func (tx migrationBootstrapRollbackTx) Commit() error {
	return tx.commitErr
}

func (tx migrationBootstrapRollbackTx) Rollback() error {
	return tx.rollbackErr
}

func openMigrationBootstrapRollbackDB(t *testing.T, dataSource string) *sql.DB {
	t.Helper()

	migrationBootstrapRollbackDriverRegistration.Do(func() {
		sql.Register(migrationBootstrapRollbackDriverName, migrationBootstrapRollbackDriver{})
	})
	db, err := sql.Open(migrationBootstrapRollbackDriverName, dataSource)
	if err != nil {
		t.Fatalf("sql.Open(%s) error = %v", migrationBootstrapRollbackDriverName, err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close migration bootstrap rollback database: %v", err)
		}
	})
	return db
}

func fixtureMigrationStream(versionTable, directory string) MigrationStream {
	return MigrationStream{
		Name:         "fixture",
		FS:           migrationFixtureFS,
		Dir:          directory,
		VersionTable: versionTable,
	}
}

func copyFixtureDirectory(t *testing.T, directory string) fstest.MapFS {
	t.Helper()
	files := fstest.MapFS{}
	err := fs.WalkDir(migrationFixtureFS, directory, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		contents, err := fs.ReadFile(migrationFixtureFS, path)
		if err != nil {
			return err
		}
		files[strings.TrimPrefix(path, directory+"/")] = &fstest.MapFile{Data: contents}
		return nil
	})
	if err != nil {
		t.Fatalf("copy fixture directory: %v", err)
	}
	return files
}

func signedMigrationMapFS(t *testing.T, migrations map[string][]byte) fstest.MapFS {
	t.Helper()
	directory := &atlasmigrate.MemDir{}
	files := fstest.MapFS{}
	for name, contents := range migrations {
		if err := directory.WriteFile(name, contents); err != nil {
			t.Fatalf("write in-memory migration %s: %v", name, err)
		}
		files[name] = &fstest.MapFile{Data: append([]byte(nil), contents...)}
	}
	sum, err := directory.Checksum()
	if err != nil {
		t.Fatalf("checksum in-memory migrations: %v", err)
	}
	sumBytes, err := sum.MarshalText()
	if err != nil {
		t.Fatalf("marshal in-memory atlas.sum: %v", err)
	}
	files[atlasmigrate.HashFileName] = &fstest.MapFile{Data: sumBytes}
	return files
}

func openEngineTestDB(t *testing.T, name string) *sql.DB {
	t.Helper()
	return openSQLitePathForTest(t, filepath.Join(t.TempDir(), name))
}

func openSQLitePathForTest(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		t.Fatalf("sql.Open(%s): %v", path, err)
	}
	if err := db.PingContext(testutil.Context(t)); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			t.Errorf("close failed database: %v", closeErr)
		}
		t.Fatalf("ping %s: %v", path, err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close %s: %v", path, err)
		}
	})
	return db
}

func migrationItemValues(t *testing.T, db *sql.DB) string {
	t.Helper()
	rows, err := db.QueryContext(testutil.Context(t), `SELECT value FROM migration_items ORDER BY id`)
	if err != nil {
		t.Fatalf("query migration items: %v", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			t.Errorf("close migration item rows: %v", err)
		}
	}()
	values := make([]string, 0, 2)
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			t.Fatalf("scan migration item: %v", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate migration items: %v", err)
	}
	return strings.Join(values, ",")
}

func tableExistsForTest(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()
	var exists bool
	if err := db.QueryRowContext(
		testutil.Context(t),
		`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ?)`,
		table,
	).Scan(&exists); err != nil {
		t.Fatalf("query table %s existence: %v", table, err)
	}
	return exists
}

func seedGooseVersion(t *testing.T, db *sql.DB, table string, version int64) {
	t.Helper()
	if _, err := db.ExecContext(testutil.Context(t), `CREATE TABLE `+quoteIdentifier(table)+` (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		version_id INTEGER NOT NULL,
		is_applied INTEGER NOT NULL,
		tstamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatalf("create goose version table: %v", err)
	}
	if _, err := db.ExecContext(
		testutil.Context(t),
		`INSERT INTO `+quoteIdentifier(table)+` (version_id, is_applied) VALUES (?, 1)`,
		version,
	); err != nil {
		t.Fatalf("seed goose version: %v", err)
	}
}

func latestSeededVersion(t *testing.T, db *sql.DB, table string) int64 {
	t.Helper()
	var version int64
	if err := db.QueryRowContext(
		testutil.Context(t),
		`SELECT MAX(version_id) FROM `+quoteIdentifier(table),
	).Scan(&version); err != nil {
		t.Fatalf("query seeded version: %v", err)
	}
	return version
}

func fileDigest(t *testing.T, path string) [sha256.Size]byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return sha256.Sum256(contents)
}

func filePresence(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	t.Fatalf("stat %s: %v", path, err)
	return false
}
