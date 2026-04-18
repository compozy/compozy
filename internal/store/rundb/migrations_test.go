package rundb

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
)

func TestApplyMigrationsIsIdempotent(t *testing.T) {
	t.Parallel()

	db := openTestRunDB(t, "run-migrations")
	defer func() {
		_ = db.Close()
	}()

	beforeSchema := loadSchemaSnapshot(t, db.db)
	beforeMigrations := loadMigrationRows(t, db.db)
	if got, want := len(beforeMigrations), len(migrations); got != want {
		t.Fatalf("schema_migrations row count = %d, want %d", got, want)
	}

	if err := applyMigrations(context.Background(), db.db, db.now); err != nil {
		t.Fatalf("applyMigrations(second pass): %v", err)
	}

	afterSchema := loadSchemaSnapshot(t, db.db)
	afterMigrations := loadMigrationRows(t, db.db)
	if !reflect.DeepEqual(afterSchema, beforeSchema) {
		t.Fatalf("sqlite schema changed on second migration pass\nbefore: %#v\nafter:  %#v", beforeSchema, afterSchema)
	}
	if !reflect.DeepEqual(afterMigrations, beforeMigrations) {
		t.Fatalf(
			"migration history changed on second migration pass\nbefore: %#v\nafter:  %#v",
			beforeMigrations,
			afterMigrations,
		)
	}

	requiredTables := []string{
		"artifact_sync_log",
		"events",
		"hook_runs",
		"job_state",
		"schema_migrations",
		"token_usage",
		"transcript_messages",
	}
	for _, tableName := range requiredTables {
		if _, ok := beforeSchema["table:"+tableName]; !ok {
			t.Fatalf("missing required table %q in schema snapshot", tableName)
		}
	}
}

func TestApplyMigrationsDropsDeadSecondaryIndexesOnly(t *testing.T) {
	t.Parallel()

	db := openTestRunDB(t, "run-migration-indexes")
	defer func() {
		_ = db.Close()
	}()

	schema := loadSchemaSnapshot(t, db.db)
	for _, dropped := range []string{
		"index:idx_events_kind",
		"index:idx_events_timestamp",
		"index:idx_events_job_id",
		"index:idx_job_state_status",
		"index:idx_transcript_messages_timestamp",
		"index:idx_artifact_sync_log_path",
	} {
		if _, ok := schema[dropped]; ok {
			t.Fatalf("expected dead index %q to be absent after migration", dropped)
		}
	}

	for _, retained := range []string{
		"index:idx_hook_runs_recorded_at",
		"index:idx_token_usage_timestamp",
	} {
		if _, ok := schema[retained]; !ok {
			t.Fatalf("expected live index %q to remain after migration", retained)
		}
	}
}

func TestApplyMigrationsRejectsSchemaTooNew(t *testing.T) {
	t.Parallel()

	fixedNow := time.Date(2026, 4, 17, 19, 0, 0, 0, time.UTC)
	sqlDB, err := store.OpenSQLiteDatabase(
		context.Background(),
		filepath.Join(t.TempDir(), "future-run", "run.db"),
		func(ctx context.Context, db *sql.DB) error {
			if err := store.EnsureSchema(ctx, db, migrationTableStatements); err != nil {
				return err
			}
			_, err := db.ExecContext(
				ctx,
				`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
				999,
				"future_migration",
				store.FormatTimestamp(fixedNow),
			)
			return err
		},
	)
	if err != nil {
		t.Fatalf("OpenSQLiteDatabase(): %v", err)
	}
	defer func() {
		_ = sqlDB.Close()
	}()

	err = applyMigrations(context.Background(), sqlDB, func() time.Time { return fixedNow })
	if !errors.Is(err, ErrSchemaTooNew) {
		t.Fatalf("applyMigrations() error = %v, want ErrSchemaTooNew", err)
	}

	var schemaErr SchemaTooNewError
	if !errors.As(err, &schemaErr) {
		t.Fatalf("applyMigrations() error = %v, want SchemaTooNewError details", err)
	}
	if schemaErr.CurrentVersion != 999 {
		t.Fatalf("SchemaTooNewError.CurrentVersion = %d, want 999", schemaErr.CurrentVersion)
	}
	if schemaErr.KnownVersion != migrations[len(migrations)-1].version {
		t.Fatalf(
			"SchemaTooNewError.KnownVersion = %d, want %d",
			schemaErr.KnownVersion,
			migrations[len(migrations)-1].version,
		)
	}
}

func TestOpenUsesExportedConstructor(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "run-open", "run.db")
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	if got := db.Path(); got != path {
		t.Fatalf("Path() = %q, want %q", got, path)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}
}

func TestApplyMigrationsRejectsNilInputs(t *testing.T) {
	t.Parallel()

	var nilCtx context.Context
	if err := applyMigrations(nilCtx, nil, nil); err == nil {
		t.Fatal("applyMigrations(nil, nil, nil) error = nil, want non-nil")
	}
	if err := applyMigrations(context.Background(), nil, nil); err == nil {
		t.Fatal("applyMigrations(ctx, nil, nil) error = nil, want non-nil")
	}
}

func TestApplyMigrationReturnsStatementErrors(t *testing.T) {
	t.Parallel()

	sqlDB, err := store.OpenSQLiteDatabase(
		context.Background(),
		filepath.Join(t.TempDir(), "broken-run", "run.db"),
		func(ctx context.Context, db *sql.DB) error {
			return store.EnsureSchema(ctx, db, migrationTableStatements)
		},
	)
	if err != nil {
		t.Fatalf("OpenSQLiteDatabase(): %v", err)
	}
	defer func() {
		_ = sqlDB.Close()
	}()

	err = applyMigration(context.Background(), sqlDB, migration{
		version:    2,
		name:       "broken",
		statements: []string{"CREATE TABL definitely_invalid ("},
	}, func() time.Time {
		return time.Date(2026, 4, 17, 19, 15, 0, 0, time.UTC)
	})
	if err == nil {
		t.Fatal("applyMigration(broken) error = nil, want non-nil")
	}
}

type migrationRow struct {
	Version   int
	Name      string
	AppliedAt string
}

func loadMigrationRows(t *testing.T, sqlDB *sql.DB) []migrationRow {
	t.Helper()

	rows, err := sqlDB.QueryContext(
		context.Background(),
		`SELECT version, name, applied_at FROM schema_migrations ORDER BY version ASC`,
	)
	if err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	out := make([]migrationRow, 0)
	for rows.Next() {
		var row migrationRow
		if err := rows.Scan(&row.Version, &row.Name, &row.AppliedAt); err != nil {
			t.Fatalf("scan schema_migrations row: %v", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate schema_migrations: %v", err)
	}

	return out
}

func loadSchemaSnapshot(t *testing.T, sqlDB *sql.DB) map[string]string {
	t.Helper()

	rows, err := sqlDB.QueryContext(
		context.Background(),
		`SELECT type, name, sql
		 FROM sqlite_master
		 WHERE type IN ('table', 'index')
		   AND name NOT LIKE 'sqlite_%'
		 ORDER BY type ASC, name ASC`,
	)
	if err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	snapshot := make(map[string]string)
	for rows.Next() {
		var (
			objectType string
			name       string
			sqlText    sql.NullString
		)
		if err := rows.Scan(&objectType, &name, &sqlText); err != nil {
			t.Fatalf("scan sqlite_master row: %v", err)
		}
		snapshot[objectType+":"+name] = sqlText.String
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate sqlite_master: %v", err)
	}

	return snapshot
}
