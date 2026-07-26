package store

import (
	"database/sql"
	"errors"
	"testing"

	atlasmigrate "ariga.io/atlas/sql/migrate"

	"github.com/compozy/agh/internal/testutil"
)

func TestMigrationStreamStatus(t *testing.T) {
	t.Run("Should reject a nil context", func(t *testing.T) {
		t.Parallel()

		stream := fixtureMigrationStream("goose_db_version_nil_context", "testdata/migrations/happy")
		//nolint:staticcheck // This case verifies the public nil-context rejection contract.
		status, err := Status(nil, openEngineTestDB(t, "status-nil-context.db"), stream)
		if err == nil {
			t.Fatal("Status() error = nil, want context validation failure")
		}
		if status != (StreamStatus{}) {
			t.Fatalf("Status() = %#v, want zero status on error", status)
		}
	})

	t.Run("Should report a migrated stream without writing", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openEngineTestDB(t, "status-migrated.db")
		stream := fixtureMigrationStream("goose_db_version_global", "testdata/migrations/happy")
		if err := Apply(ctx, db, stream); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
		before := pragmaDataVersion(t, db)
		status, err := Status(ctx, db, stream)
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		after := pragmaDataVersion(t, db)
		if status.Stream != stream.Name || status.Version != 2 || status.AppliedCount != 2 || status.SumDigest == "" {
			t.Fatalf("Status() = %#v, want populated migrated status", status)
		}
		if after != before {
			t.Fatalf("PRAGMA data_version after Status = %d, want %d", after, before)
		}
	})

	t.Run("Should return the legacy sentinel instead of a fabricated status", func(t *testing.T) {
		t.Parallel()

		db := openEngineTestDB(t, "status-legacy.db")
		if _, err := db.ExecContext(
			testutil.Context(t),
			`CREATE TABLE schema_migrations (version INTEGER)`,
		); err != nil {
			t.Fatalf("seed legacy table: %v", err)
		}
		stream := fixtureMigrationStream("goose_db_version_global", "testdata/migrations/happy")
		stream.LegacyTables = []string{"schema_migrations"}
		status, err := Status(testutil.Context(t), db, stream)
		if !errors.Is(err, ErrLegacyDatabase) {
			t.Fatalf("Status() error = %v, want ErrLegacyDatabase", err)
		}
		if status != (StreamStatus{}) {
			t.Fatalf("Status() = %#v, want zero status on error", status)
		}
	})

	t.Run("Should report a fresh stream without bootstrapping its version table", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openEngineTestDB(t, "status-fresh.db")
		stream := fixtureMigrationStream("goose_db_version_memory", "testdata/migrations/happy")
		before := sqliteMasterRows(t, db)
		status, err := Status(ctx, db, stream)
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		after := sqliteMasterRows(t, db)
		if status.Stream != stream.Name || status.Version != 0 || status.AppliedCount != 0 || status.SumDigest == "" {
			t.Fatalf("Status() = %#v, want fresh read-only status", status)
		}
		if before != after {
			t.Fatalf("sqlite_master rows after Status = %d, want %d", after, before)
		}
		if tableExistsForTest(t, db, stream.VersionTable) {
			t.Fatalf("Status() bootstrapped %s", stream.VersionTable)
		}
	})

	t.Run("Should use the latest applied state for each migration version", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openEngineTestDB(t, "status-history.db")
		stream := fixtureMigrationStream("goose_db_version_history", "testdata/migrations/happy")
		seedGooseVersion(t, db, stream.VersionTable, 1)
		if _, err := db.ExecContext(
			ctx,
			`INSERT INTO `+quoteIdentifier(stream.VersionTable)+` (version_id, is_applied) VALUES (1, 0), (2, 1)`,
		); err != nil {
			t.Fatalf("seed migration history: %v", err)
		}
		status, err := Status(ctx, db, stream)
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if status.Version != 2 || status.AppliedCount != 1 {
			t.Fatalf("Status() = %#v, want only version 2 applied", status)
		}
	})

	t.Run("Should reject a stream ahead of the embedded directory without writing", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openEngineTestDB(t, "status-ahead.db")
		stream := fixtureMigrationStream("goose_db_version_status_ahead", "testdata/migrations/happy")
		seedGooseVersion(t, db, stream.VersionTable, 99)
		before := pragmaDataVersion(t, db)

		status, err := Status(ctx, db, stream)
		if !errors.Is(err, ErrSchemaAhead) {
			t.Fatalf("Status() error = %v, want ErrSchemaAhead", err)
		}
		if status != (StreamStatus{}) {
			t.Fatalf("Status() = %#v, want zero status on error", status)
		}
		if after := pragmaDataVersion(t, db); after != before {
			t.Fatalf("PRAGMA data_version after Status = %d, want %d", after, before)
		}
	})

	t.Run("Should report a fresh in-memory database", func(t *testing.T) {
		t.Parallel()

		db, err := sql.Open(sqliteDriverName, ":memory:")
		if err != nil {
			t.Fatalf("open in-memory database: %v", err)
		}
		t.Cleanup(func() {
			if err := db.Close(); err != nil {
				t.Errorf("close in-memory database: %v", err)
			}
		})
		stream := fixtureMigrationStream("goose_db_version_memory_status", "testdata/migrations/happy")
		status, err := Status(testutil.Context(t), db, stream)
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if status.Version != 0 || status.AppliedCount != 0 || status.SumDigest == "" {
			t.Fatalf("Status() = %#v, want fresh in-memory status", status)
		}
	})

	t.Run("Should reject a tampered embedded migration directory", func(t *testing.T) {
		t.Parallel()

		files := copyFixtureDirectory(t, "testdata/migrations/happy")
		files["00001_create.sql"].Data = []byte("-- +goose Up\nSELECT 1;")
		stream := MigrationStream{
			Name:         "tampered-status",
			FS:           files,
			Dir:          ".",
			VersionTable: "goose_db_version_tampered_status",
		}
		status, err := Status(testutil.Context(t), openEngineTestDB(t, "tampered-status.db"), stream)
		if !errors.Is(err, atlasmigrate.ErrChecksumMismatch) {
			t.Fatalf("Status() error = %v, want Atlas checksum mismatch", err)
		}
		if status != (StreamStatus{}) {
			t.Fatalf("Status() = %#v, want zero status on error", status)
		}
	})
}

func TestRequireCurrentMigrationStream(t *testing.T) {
	t.Parallel()

	t.Run("Should accept the embedded head without writing", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openEngineTestDB(t, "require-current.db")
		stream := fixtureMigrationStream("goose_db_version_require_current", "testdata/migrations/happy")
		if err := Apply(ctx, db, stream); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
		before := pragmaDataVersion(t, db)
		if err := RequireCurrent(ctx, db, stream); err != nil {
			t.Fatalf("RequireCurrent() error = %v", err)
		}
		if after := pragmaDataVersion(t, db); after != before {
			t.Fatalf("PRAGMA data_version after RequireCurrent = %d, want %d", after, before)
		}
	})

	t.Run("Should reject a behind stream without writing", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openEngineTestDB(t, "require-current-behind.db")
		stream := fixtureMigrationStream("goose_db_version_require_current_behind", "testdata/migrations/happy")
		seedGooseVersion(t, db, stream.VersionTable, 1)
		before := pragmaDataVersion(t, db)
		err := RequireCurrent(ctx, db, stream)
		if !errors.Is(err, ErrSchemaBehind) {
			t.Fatalf("RequireCurrent(behind) error = %v, want ErrSchemaBehind", err)
		}
		if after := pragmaDataVersion(t, db); after != before {
			t.Fatalf("PRAGMA data_version after RequireCurrent = %d, want %d", after, before)
		}
	})
}

func pragmaDataVersion(t *testing.T, db interface {
	QueryRow(query string, args ...any) *sql.Row
}) int64 {
	t.Helper()
	var version int64
	if err := db.QueryRow(`PRAGMA data_version`).Scan(&version); err != nil {
		t.Fatalf("query PRAGMA data_version: %v", err)
	}
	return version
}

func sqliteMasterRows(t *testing.T, db interface {
	QueryRow(query string, args ...any) *sql.Row
}) int {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master`).Scan(&count); err != nil {
		t.Fatalf("count sqlite_master: %v", err)
	}
	return count
}
