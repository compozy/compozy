package globaldb

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
		name:    "global_catalog_initial",
		statements: []string{
			`CREATE TABLE IF NOT EXISTS workspaces (
				id         TEXT PRIMARY KEY,
				root_dir   TEXT NOT NULL,
				name       TEXT NOT NULL,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL
			);`,
			`CREATE UNIQUE INDEX IF NOT EXISTS uq_workspaces_root_dir ON workspaces(root_dir);`,
			`CREATE INDEX IF NOT EXISTS idx_workspaces_name ON workspaces(name);`,
			`CREATE TABLE IF NOT EXISTS workflows (
				id             TEXT PRIMARY KEY,
				workspace_id   TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
				slug           TEXT NOT NULL,
				archived_at    TEXT,
				last_synced_at TEXT,
				created_at     TEXT NOT NULL,
				updated_at     TEXT NOT NULL
			);`,
			`CREATE INDEX IF NOT EXISTS idx_workflows_workspace ON workflows(workspace_id);`,
			`CREATE INDEX IF NOT EXISTS idx_workflows_workspace_slug ON workflows(workspace_id, slug);`,
			`CREATE UNIQUE INDEX IF NOT EXISTS uq_workflows_active_slug
				ON workflows(workspace_id, slug)
				WHERE archived_at IS NULL;`,
			fmt.Sprintf(`CREATE TABLE IF NOT EXISTS artifact_snapshots (
				workflow_id       TEXT NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
				artifact_kind     TEXT NOT NULL,
				relative_path     TEXT NOT NULL,
				checksum          TEXT NOT NULL,
				frontmatter_json  TEXT NOT NULL DEFAULT '{}',
				body_text         TEXT,
				body_storage_kind TEXT NOT NULL DEFAULT 'inline',
				source_mtime      TEXT NOT NULL,
				synced_at         TEXT NOT NULL,
				PRIMARY KEY (workflow_id, artifact_kind, relative_path),
				CHECK (body_text IS NULL OR length(body_text) <= %d)
			);`, 256*1024),
			`CREATE INDEX IF NOT EXISTS idx_artifact_snapshots_checksum ON artifact_snapshots(checksum);`,
			`CREATE TABLE IF NOT EXISTS task_items (
				id               TEXT PRIMARY KEY,
				workflow_id       TEXT NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
				task_number       INTEGER NOT NULL,
				task_id           TEXT NOT NULL,
				title             TEXT NOT NULL,
				status            TEXT NOT NULL,
				kind              TEXT NOT NULL,
				depends_on_json   TEXT NOT NULL DEFAULT '[]',
				source_path       TEXT NOT NULL,
				updated_at        TEXT NOT NULL
			);`,
			`CREATE UNIQUE INDEX IF NOT EXISTS uq_task_items_workflow_number
				ON task_items(workflow_id, task_number);`,
			`CREATE UNIQUE INDEX IF NOT EXISTS uq_task_items_workflow_task_id
				ON task_items(workflow_id, task_id);`,
			`CREATE TABLE IF NOT EXISTS review_rounds (
				id               TEXT PRIMARY KEY,
				workflow_id       TEXT NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
				round_number      INTEGER NOT NULL,
				provider          TEXT NOT NULL,
				pr_ref            TEXT NOT NULL DEFAULT '',
				resolved_count    INTEGER NOT NULL DEFAULT 0,
				unresolved_count  INTEGER NOT NULL DEFAULT 0,
				updated_at        TEXT NOT NULL
			);`,
			`CREATE UNIQUE INDEX IF NOT EXISTS uq_review_rounds_workflow_round
				ON review_rounds(workflow_id, round_number);`,
			`CREATE TABLE IF NOT EXISTS review_issues (
				id            TEXT PRIMARY KEY,
				round_id       TEXT NOT NULL REFERENCES review_rounds(id) ON DELETE CASCADE,
				issue_number   INTEGER NOT NULL,
				severity       TEXT NOT NULL,
				status         TEXT NOT NULL,
				source_path    TEXT NOT NULL,
				updated_at     TEXT NOT NULL
			);`,
			`CREATE UNIQUE INDEX IF NOT EXISTS uq_review_issues_round_issue
				ON review_issues(round_id, issue_number);`,
			`CREATE TABLE IF NOT EXISTS runs (
				run_id             TEXT PRIMARY KEY,
				workspace_id        TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
				workflow_id         TEXT REFERENCES workflows(id) ON DELETE SET NULL,
				mode                TEXT NOT NULL,
				status              TEXT NOT NULL,
				presentation_mode   TEXT NOT NULL,
				started_at          TEXT NOT NULL,
				ended_at            TEXT,
				error_text          TEXT NOT NULL DEFAULT '',
				request_id          TEXT NOT NULL DEFAULT ''
			);`,
			`CREATE INDEX IF NOT EXISTS idx_runs_workspace_started
				ON runs(workspace_id, started_at DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_runs_workspace_status
				ON runs(workspace_id, status);`,
			`CREATE TABLE IF NOT EXISTS sync_checkpoints (
				workflow_id       TEXT NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
				scope             TEXT NOT NULL,
				checksum          TEXT NOT NULL DEFAULT '',
				last_scan_at      TEXT,
				last_success_at   TEXT,
				last_error_text   TEXT NOT NULL DEFAULT '',
				PRIMARY KEY (workflow_id, scope)
			);`,
		},
	},
	{
		version: 2,
		name:    "runs_status_normalization",
		statements: []string{
			`UPDATE runs
			 SET status = CASE LOWER(TRIM(status))
				WHEN 'cancelled' THEN 'canceled'
				ELSE LOWER(TRIM(status))
			 END
			 WHERE status <> CASE LOWER(TRIM(status))
				WHEN 'cancelled' THEN 'canceled'
				ELSE LOWER(TRIM(status))
			 END;`,
			`CREATE INDEX IF NOT EXISTS idx_runs_workflow_id
				ON runs(workflow_id)
				WHERE workflow_id IS NOT NULL;`,
		},
	},
	{
		version: 3,
		name:    "runs_status_index_cover_ordering",
		statements: []string{
			`DROP INDEX IF EXISTS idx_runs_workspace_status;`,
			`CREATE INDEX IF NOT EXISTS idx_runs_workspace_status
				ON runs(workspace_id, status, started_at DESC, run_id ASC);`,
		},
	},
	{
		version: 4,
		name:    "workspace_filesystem_state_and_artifact_bodies",
		statements: []string{
			`ALTER TABLE workspaces ADD COLUMN filesystem_state TEXT NOT NULL DEFAULT 'present';`,
			`ALTER TABLE workspaces ADD COLUMN last_checked_at TEXT;`,
			`ALTER TABLE workspaces ADD COLUMN last_sync_at TEXT;`,
			`ALTER TABLE workspaces ADD COLUMN last_sync_error TEXT NOT NULL DEFAULT '';`,
			`CREATE INDEX IF NOT EXISTS idx_workspaces_filesystem_state
				ON workspaces(filesystem_state);`,
			`CREATE TABLE IF NOT EXISTS artifact_bodies (
				checksum   TEXT PRIMARY KEY,
				body_text  TEXT NOT NULL,
				size_bytes INTEGER NOT NULL,
				created_at TEXT NOT NULL
			);`,
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
var ErrSchemaTooNew = errors.New("globaldb: schema too new")

// SchemaTooNewError carries the current database and binary migration versions.
type SchemaTooNewError struct {
	CurrentVersion int
	KnownVersion   int
}

func (e SchemaTooNewError) Error() string {
	return fmt.Sprintf(
		"globaldb: schema too new (db=%d binary=%d)",
		e.CurrentVersion,
		e.KnownVersion,
	)
}

func (e SchemaTooNewError) Is(target error) bool {
	return target == ErrSchemaTooNew
}

func applyMigrations(ctx context.Context, db *sql.DB, now func() time.Time) error {
	if ctx == nil {
		return errors.New("globaldb: migrate context is required")
	}
	if db == nil {
		return errors.New("globaldb: migrate database is required")
	}
	if now == nil {
		now = func() time.Time {
			return time.Now().UTC()
		}
	}

	if err := store.EnsureSchema(ctx, db, migrationTableStatements); err != nil {
		return fmt.Errorf("globaldb: ensure schema migrations table: %w", err)
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
	rows, err := db.QueryContext(
		ctx,
		`SELECT version FROM schema_migrations ORDER BY version ASC`,
	)
	if err != nil {
		return appliedMigrationState{}, fmt.Errorf("globaldb: query schema migrations: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	state := appliedMigrationState{versions: make(map[int]bool)}
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return appliedMigrationState{}, fmt.Errorf("globaldb: scan schema migration: %w", err)
		}
		state.versions[version] = true
		if version > state.highestVersion {
			state.highestVersion = version
		}
	}
	if err := rows.Err(); err != nil {
		return appliedMigrationState{}, fmt.Errorf("globaldb: iterate schema migrations: %w", err)
	}

	return state, nil
}

func applyMigration(ctx context.Context, db *sql.DB, item migration, now func() time.Time) (retErr error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("globaldb: begin migration %d: %w", item.version, err)
	}

	committed := false
	defer func() {
		if !committed {
			if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
				retErr = errors.Join(
					retErr,
					fmt.Errorf("globaldb: rollback migration %d: %w", item.version, rollbackErr),
				)
			}
		}
	}()

	for _, stmt := range item.statements {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf(
				"globaldb: apply migration %d (%s): %w",
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
		return fmt.Errorf("globaldb: record migration %d: %w", item.version, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("globaldb: commit migration %d: %w", item.version, err)
	}
	committed = true
	return nil
}
