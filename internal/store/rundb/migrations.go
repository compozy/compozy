package rundb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

type migration struct {
	version    int
	name       string
	statements []string
}

var migrations = []migration{
	{
		version: 1,
		name:    "run_store_initial",
		statements: []string{
			`CREATE TABLE IF NOT EXISTS events (
				sequence     INTEGER PRIMARY KEY,
				event_kind   TEXT NOT NULL,
				payload_json TEXT NOT NULL,
				timestamp    TEXT NOT NULL,
				job_id       TEXT NOT NULL DEFAULT '',
				step_key     TEXT NOT NULL DEFAULT ''
			);`,
			`CREATE INDEX IF NOT EXISTS idx_events_kind ON events(event_kind);`,
			`CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);`,
			`CREATE INDEX IF NOT EXISTS idx_events_job_id ON events(job_id);`,
			`CREATE TABLE IF NOT EXISTS job_state (
				job_id       TEXT PRIMARY KEY,
				task_id      TEXT NOT NULL DEFAULT '',
				status       TEXT NOT NULL,
				agent_name   TEXT NOT NULL DEFAULT '',
				summary_json TEXT NOT NULL,
				updated_at   TEXT NOT NULL
			);`,
			`CREATE INDEX IF NOT EXISTS idx_job_state_status ON job_state(status);`,
			`CREATE TABLE IF NOT EXISTS transcript_messages (
				sequence      INTEGER PRIMARY KEY,
				stream        TEXT NOT NULL,
				role          TEXT NOT NULL,
				content       TEXT NOT NULL,
				metadata_json TEXT NOT NULL,
				timestamp     TEXT NOT NULL
			);`,
			`CREATE INDEX IF NOT EXISTS idx_transcript_messages_timestamp
				ON transcript_messages(timestamp);`,
			`CREATE TABLE IF NOT EXISTS hook_runs (
				id          TEXT PRIMARY KEY,
				hook_name   TEXT NOT NULL,
				source      TEXT NOT NULL,
				outcome     TEXT NOT NULL,
				duration_ns INTEGER NOT NULL,
				payload_json TEXT NOT NULL,
				recorded_at TEXT NOT NULL
			);`,
			`CREATE INDEX IF NOT EXISTS idx_hook_runs_recorded_at ON hook_runs(recorded_at);`,
			`CREATE TABLE IF NOT EXISTS token_usage (
				turn_id        TEXT PRIMARY KEY,
				input_tokens   INTEGER NOT NULL DEFAULT 0,
				output_tokens  INTEGER NOT NULL DEFAULT 0,
				total_tokens   INTEGER NOT NULL DEFAULT 0,
				cost_amount    REAL,
				timestamp      TEXT NOT NULL
			);`,
			`CREATE INDEX IF NOT EXISTS idx_token_usage_timestamp ON token_usage(timestamp);`,
			`CREATE TABLE IF NOT EXISTS artifact_sync_log (
				sequence      INTEGER PRIMARY KEY,
				relative_path TEXT NOT NULL,
				change_kind   TEXT NOT NULL,
				checksum      TEXT NOT NULL DEFAULT '',
				synced_at     TEXT NOT NULL
			);`,
			`CREATE INDEX IF NOT EXISTS idx_artifact_sync_log_path ON artifact_sync_log(relative_path);`,
		},
	},
}

var migrationTableStatements = []string{
	`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		name       TEXT NOT NULL,
		applied_at TEXT NOT NULL
	);`,
}

// ErrSchemaTooNew reports that a database carries a migration newer than this binary understands.
var ErrSchemaTooNew = errors.New("rundb: schema too new")

// SchemaTooNewError carries the current database and binary migration versions.
type SchemaTooNewError struct {
	CurrentVersion int
	KnownVersion   int
}

func (e SchemaTooNewError) Error() string {
	return fmt.Sprintf("rundb: schema too new (db=%d binary=%d)", e.CurrentVersion, e.KnownVersion)
}

func (e SchemaTooNewError) Is(target error) bool {
	return target == ErrSchemaTooNew
}

func applyMigrations(ctx context.Context, db *sql.DB, now func() time.Time) error {
	if ctx == nil {
		return errors.New("rundb: migrate context is required")
	}
	if db == nil {
		return errors.New("rundb: migrate database is required")
	}
	if now == nil {
		now = func() time.Time {
			return time.Now().UTC()
		}
	}

	if err := store.EnsureSchema(ctx, db, migrationTableStatements); err != nil {
		return fmt.Errorf("rundb: ensure schema migrations table: %w", err)
	}

	applied, err := loadAppliedMigrations(ctx, db)
	if err != nil {
		return err
	}

	latestKnown := migrations[len(migrations)-1].version
	if applied.highestVersion > latestKnown {
		return SchemaTooNewError{
			CurrentVersion: applied.highestVersion,
			KnownVersion:   latestKnown,
		}
	}

	for _, item := range migrations {
		if applied.versions[item.version] {
			continue
		}
		if err := applyMigration(ctx, db, item, now); err != nil {
			return err
		}
	}

	return nil
}

type appliedMigrationState struct {
	versions       map[int]bool
	highestVersion int
}

func loadAppliedMigrations(ctx context.Context, db *sql.DB) (appliedMigrationState, error) {
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations ORDER BY version ASC`)
	if err != nil {
		return appliedMigrationState{}, fmt.Errorf("rundb: query schema migrations: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	state := appliedMigrationState{versions: make(map[int]bool)}
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return appliedMigrationState{}, fmt.Errorf("rundb: scan schema migration: %w", err)
		}
		state.versions[version] = true
		if version > state.highestVersion {
			state.highestVersion = version
		}
	}
	if err := rows.Err(); err != nil {
		return appliedMigrationState{}, fmt.Errorf("rundb: iterate schema migrations: %w", err)
	}

	return state, nil
}

func applyMigration(ctx context.Context, db *sql.DB, item migration, now func() time.Time) (retErr error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("rundb: begin migration %d: %w", item.version, err)
	}

	committed := false
	defer func() {
		if !committed {
			if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
				retErr = errors.Join(
					retErr,
					fmt.Errorf("rundb: rollback migration %d: %w", item.version, rollbackErr),
				)
			}
		}
	}()

	for _, stmt := range item.statements {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf(
				"rundb: apply migration %d (%s): %w",
				item.version,
				strings.TrimSpace(item.name),
				err,
			)
		}
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
		item.version,
		strings.TrimSpace(item.name),
		store.FormatTimestamp(now()),
	); err != nil {
		return fmt.Errorf("rundb: record migration %d: %w", item.version, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("rundb: commit migration %d: %w", item.version, err)
	}
	committed = true
	return nil
}
