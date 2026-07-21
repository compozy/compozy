package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

type migration struct {
	version        int
	name           string
	statements     []string
	foreignKeysOff bool
	apply          func(context.Context, *sql.Tx) error
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
				size_bytes INTEGER NOT NULL CHECK (size_bytes >= 0),
				created_at TEXT NOT NULL
			);`,
		},
	},
	{
		version: 5,
		name:    "runs_parent_run_id",
		statements: []string{
			`ALTER TABLE runs ADD COLUMN parent_run_id TEXT NOT NULL DEFAULT '';`,
			`CREATE INDEX IF NOT EXISTS idx_runs_parent_run_id
				ON runs(parent_run_id)
				WHERE parent_run_id <> '';`,
		},
	},
	{
		version: 6,
		name:    "workflow_hierarchy",
		statements: []string{
			`ALTER TABLE workflows ADD COLUMN kind TEXT NOT NULL DEFAULT 'ordinary'
				CHECK (kind IN ('ordinary', 'initiative', 'task_group'));`,
			`ALTER TABLE workflows ADD COLUMN parent_workflow_id TEXT REFERENCES workflows(id);`,
			`ALTER TABLE workflows ADD COLUMN task_group_id TEXT NOT NULL DEFAULT '';`,
			`ALTER TABLE workflows ADD COLUMN display_title TEXT NOT NULL DEFAULT '';`,
			`ALTER TABLE workflows ADD COLUMN outcome TEXT NOT NULL DEFAULT '';`,
			`ALTER TABLE workflows ADD COLUMN lifecycle_completed INTEGER NOT NULL DEFAULT 0
				CHECK (lifecycle_completed IN (0, 1));`,
			`ALTER TABLE workflows ADD COLUMN dependencies_json TEXT NOT NULL DEFAULT '[]';`,
			`CREATE INDEX IF NOT EXISTS idx_workflows_parent_workflow_id
				ON workflows(parent_workflow_id)
				WHERE parent_workflow_id IS NOT NULL;`,
			`CREATE UNIQUE INDEX IF NOT EXISTS uq_workflows_active_child_task_group
				ON workflows(parent_workflow_id, task_group_id)
				WHERE archived_at IS NULL
				  AND parent_workflow_id IS NOT NULL
				  AND task_group_id <> '';`,
		},
	},
	{
		version: 7,
		name:    "runs_out_of_order_metadata",
		statements: []string{
			`ALTER TABLE runs ADD COLUMN out_of_order_requested INTEGER NOT NULL DEFAULT 0
				CHECK (out_of_order_requested IN (0, 1));`,
			`ALTER TABLE runs ADD COLUMN out_of_order_needed INTEGER NOT NULL DEFAULT 0
				CHECK (out_of_order_needed IN (0, 1));`,
		},
	},
	{
		version: 8,
		name:    "workflow_missing_placeholder_state",
		statements: []string{
			// Existing rows default to present (0); only placeholder rows for absent
			// task group directories set it, letting the read model block their start.
			`ALTER TABLE workflows ADD COLUMN missing INTEGER NOT NULL DEFAULT 0
				CHECK (missing IN (0, 1));`,
		},
	},
	{
		version:        9,
		name:           "work_package_to_task_group",
		foreignKeysOff: true,
		apply:          migrateWorkPackagesToTaskGroups,
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

func applyMigration(ctx context.Context, db *sql.DB, item migration, now func() time.Time) error {
	if item.foreignKeysOff {
		return applyMigrationWithForeignKeysOff(ctx, db, item, now)
	}
	return applyMigrationTransaction(ctx, db, item, now)
}

type transactionBeginner interface {
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
}

func applyMigrationWithForeignKeysOff(
	ctx context.Context,
	db *sql.DB,
	item migration,
	now func() time.Time,
) (retErr error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("globaldb: acquire connection for migration %d: %w", item.version, err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			retErr = errors.Join(
				retErr,
				fmt.Errorf("globaldb: close connection for migration %d: %w", item.version, closeErr),
			)
		}
	}()

	if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return fmt.Errorf("globaldb: disable foreign keys for migration %d: %w", item.version, err)
	}
	defer func() {
		restoreCtx, cancel := context.WithTimeout(context.Background(), store.DefaultDrainTimeout)
		defer cancel()
		if _, err := conn.ExecContext(restoreCtx, `PRAGMA foreign_keys = ON`); err != nil {
			retErr = errors.Join(
				retErr,
				fmt.Errorf("globaldb: restore foreign keys after migration %d: %w", item.version, err),
			)
			return
		}
		if err := requireForeignKeysState(restoreCtx, conn, true); err != nil {
			retErr = errors.Join(
				retErr,
				fmt.Errorf("globaldb: restore foreign keys after migration %d: %w", item.version, err),
			)
		}
	}()
	if err := requireForeignKeysState(ctx, conn, false); err != nil {
		return fmt.Errorf("globaldb: disable foreign keys for migration %d: %w", item.version, err)
	}

	return applyMigrationTransaction(ctx, conn, item, now)
}

func requireForeignKeysState(ctx context.Context, conn *sql.Conn, wantEnabled bool) error {
	var enabled int
	if err := conn.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&enabled); err != nil {
		return fmt.Errorf("query foreign key state: %w", err)
	}
	want := 0
	if wantEnabled {
		want = 1
	}
	if enabled != want {
		return fmt.Errorf("foreign key state = %d, want %d", enabled, want)
	}
	return nil
}

func applyMigrationTransaction(
	ctx context.Context,
	beginner transactionBeginner,
	item migration,
	now func() time.Time,
) (retErr error) {
	tx, err := beginner.BeginTx(ctx, nil)
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
	if item.apply != nil {
		if err := item.apply(ctx, tx); err != nil {
			return fmt.Errorf(
				"globaldb: apply migration %d (%s): %w",
				item.version,
				strings.TrimSpace(item.name),
				err,
			)
		}
	}
	if item.foreignKeysOff {
		if err := ensureNoForeignKeyViolations(ctx, tx); err != nil {
			return fmt.Errorf("globaldb: validate migration %d foreign keys: %w", item.version, err)
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

type legacyWorkflowDependency struct {
	PackageID string `json:"package_id"`
	Rationale string `json:"rationale"`
}

type migratedWorkflowDependency struct {
	TaskGroupID string `json:"task_group_id"`
	Rationale   string `json:"rationale"`
}

var (
	errAmbiguousWorkflowHierarchySchema   = errors.New("ambiguous workflow hierarchy schema")
	errInvalidLegacyWorkPackageIdentity   = errors.New("invalid legacy work package identity")
	errInvalidLegacyWorkflowDependencies  = errors.New("invalid legacy workflow dependencies")
	errInvalidLegacyWorkPackageDependency = errors.New("invalid legacy work package dependency")
	errWorkflowForeignKeyViolation        = errors.New("workflow foreign key violation")
)

func migrateWorkPackagesToTaskGroups(ctx context.Context, tx *sql.Tx) error {
	columns, err := workflowColumns(ctx, tx)
	if err != nil {
		return err
	}

	hasPackageID := columns["package_id"]
	hasTaskGroupID := columns["task_group_id"]
	switch {
	case hasTaskGroupID && !hasPackageID:
		return nil
	case hasPackageID && !hasTaskGroupID:
	default:
		return fmt.Errorf(
			"%w: package_id=%t task_group_id=%t",
			errAmbiguousWorkflowHierarchySchema,
			hasPackageID,
			hasTaskGroupID,
		)
	}

	if err := migrateLegacyWorkPackageRows(ctx, tx); err != nil {
		return err
	}
	return rebuildWorkflowsForTaskGroups(ctx, tx)
}

var workPackageToTaskGroupStatements = []string{
	`CREATE TABLE workflows_task_group_v9 (
			id                  TEXT PRIMARY KEY,
			workspace_id        TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
			slug                TEXT NOT NULL,
			archived_at         TEXT,
			last_synced_at      TEXT,
			created_at          TEXT NOT NULL,
			updated_at          TEXT NOT NULL,
			kind                TEXT NOT NULL DEFAULT 'ordinary'
				CHECK (kind IN ('ordinary', 'initiative', 'task_group')),
			parent_workflow_id  TEXT REFERENCES workflows(id),
			task_group_id       TEXT NOT NULL DEFAULT '',
			display_title       TEXT NOT NULL DEFAULT '',
			outcome             TEXT NOT NULL DEFAULT '',
			lifecycle_completed INTEGER NOT NULL DEFAULT 0
				CHECK (lifecycle_completed IN (0, 1)),
			dependencies_json   TEXT NOT NULL DEFAULT '[]',
			missing             INTEGER NOT NULL DEFAULT 0
				CHECK (missing IN (0, 1))
		);`,
	`INSERT INTO workflows_task_group_v9 (
			id, workspace_id, slug, archived_at, last_synced_at, created_at, updated_at,
			kind, parent_workflow_id, task_group_id, display_title, outcome,
			lifecycle_completed, dependencies_json, missing
		)
		SELECT
			id,
			workspace_id,
			CASE
				WHEN kind = 'work_package'
					THEN substr(slug, 1, length(slug) - length(package_id))
						|| 'TG-' || substr(package_id, 4)
				ELSE slug
			END,
			archived_at,
			last_synced_at,
			created_at,
			updated_at,
			CASE WHEN kind = 'work_package' THEN 'task_group' ELSE kind END,
			parent_workflow_id,
			CASE
				WHEN kind = 'work_package' THEN 'TG-' || substr(package_id, 4)
				ELSE ''
			END,
			display_title,
			outcome,
			lifecycle_completed,
			dependencies_json,
			missing
		FROM workflows;`,
	`DROP TABLE workflows;`,
	`ALTER TABLE workflows_task_group_v9 RENAME TO workflows;`,
	`CREATE INDEX idx_workflows_workspace ON workflows(workspace_id);`,
	`CREATE INDEX idx_workflows_workspace_slug ON workflows(workspace_id, slug);`,
	`CREATE UNIQUE INDEX uq_workflows_active_slug
			ON workflows(workspace_id, slug)
			WHERE archived_at IS NULL;`,
	`CREATE INDEX idx_workflows_parent_workflow_id
			ON workflows(parent_workflow_id)
			WHERE parent_workflow_id IS NOT NULL;`,
	`CREATE UNIQUE INDEX uq_workflows_active_child_task_group
			ON workflows(parent_workflow_id, task_group_id)
			WHERE archived_at IS NULL
			  AND parent_workflow_id IS NOT NULL
			  AND task_group_id <> '';`,
}

func rebuildWorkflowsForTaskGroups(ctx context.Context, tx *sql.Tx) error {
	for _, stmt := range workPackageToTaskGroupStatements {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("rebuild workflows table: %w", err)
		}
	}
	return nil
}

func workflowColumns(ctx context.Context, tx *sql.Tx) (columns map[string]bool, retErr error) {
	rows, err := tx.QueryContext(ctx, `PRAGMA table_info(workflows)`)
	if err != nil {
		return nil, fmt.Errorf("query workflow columns: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("close workflow columns: %w", closeErr))
		}
	}()

	columns = make(map[string]bool)
	for rows.Next() {
		var (
			columnID     int
			name         string
			columnType   string
			notNull      int
			defaultValue sql.NullString
			primaryKey   int
		)
		if err := rows.Scan(
			&columnID,
			&name,
			&columnType,
			&notNull,
			&defaultValue,
			&primaryKey,
		); err != nil {
			return nil, fmt.Errorf("scan workflow column: %w", err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflow columns: %w", err)
	}
	return columns, nil
}

type legacyWorkflowDependencyUpdate struct {
	workflowID       string
	dependenciesJSON string
}

func migrateLegacyWorkPackageRows(ctx context.Context, tx *sql.Tx) (retErr error) {
	rows, err := tx.QueryContext(
		ctx,
		`SELECT id, kind, slug, package_id, dependencies_json FROM workflows`,
	)
	if err != nil {
		return fmt.Errorf("query legacy workflow hierarchy: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("close legacy workflow hierarchy: %w", closeErr))
		}
	}()

	updates := make([]legacyWorkflowDependencyUpdate, 0)
	for rows.Next() {
		var id, kind, slug, packageID, dependenciesJSON string
		if err := rows.Scan(&id, &kind, &slug, &packageID, &dependenciesJSON); err != nil {
			return fmt.Errorf("scan legacy workflow hierarchy: %w", err)
		}

		if err := validateLegacyWorkflowIdentity(id, kind, slug, packageID); err != nil {
			return err
		}
		migratedJSON, err := migrateLegacyWorkflowDependencies(id, dependenciesJSON)
		if err != nil {
			return err
		}
		updates = append(updates, legacyWorkflowDependencyUpdate{
			workflowID:       id,
			dependenciesJSON: migratedJSON,
		})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate legacy workflow hierarchy: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close legacy workflow hierarchy: %w", err)
	}

	for _, update := range updates {
		if _, err := tx.ExecContext(
			ctx,
			`UPDATE workflows SET dependencies_json = ? WHERE id = ?`,
			update.dependenciesJSON,
			update.workflowID,
		); err != nil {
			return fmt.Errorf("update migrated workflow dependencies for workflow %q: %w", update.workflowID, err)
		}
	}
	return nil
}

func validateLegacyWorkflowIdentity(id, kind, slug, packageID string) error {
	if kind == "work_package" {
		if isLegacyWorkPackageID(packageID) && strings.HasSuffix(slug, "/"+packageID) {
			return nil
		}
		return fmt.Errorf(
			"%w for workflow %q: slug=%q package_id=%q",
			errInvalidLegacyWorkPackageIdentity,
			id,
			slug,
			packageID,
		)
	}
	if packageID != "" {
		return fmt.Errorf(
			"%w for workflow %q: kind=%q package_id=%q",
			errInvalidLegacyWorkPackageIdentity,
			id,
			kind,
			packageID,
		)
	}
	return nil
}

func migrateLegacyWorkflowDependencies(id, dependenciesJSON string) (string, error) {
	var dependencies []legacyWorkflowDependency
	if err := json.Unmarshal([]byte(dependenciesJSON), &dependencies); err != nil {
		return "", errors.Join(
			fmt.Errorf("%w for workflow %q", errInvalidLegacyWorkflowDependencies, id),
			fmt.Errorf("decode legacy workflow dependencies: %w", err),
		)
	}
	if dependencies == nil {
		return "", fmt.Errorf(
			"%w for workflow %q: expected a JSON array",
			errInvalidLegacyWorkflowDependencies,
			id,
		)
	}

	migratedDependencies := make([]migratedWorkflowDependency, 0, len(dependencies))
	for _, dependency := range dependencies {
		if !isLegacyWorkPackageID(dependency.PackageID) {
			return "", fmt.Errorf(
				"%w for workflow %q: package_id=%q",
				errInvalidLegacyWorkPackageDependency,
				id,
				dependency.PackageID,
			)
		}
		migratedDependencies = append(migratedDependencies, migratedWorkflowDependency{
			TaskGroupID: "TG-" + strings.TrimPrefix(dependency.PackageID, "WP-"),
			Rationale:   dependency.Rationale,
		})
	}
	migratedJSON, err := json.Marshal(migratedDependencies)
	if err != nil {
		return "", fmt.Errorf("encode migrated workflow dependencies for workflow %q: %w", id, err)
	}
	return string(migratedJSON), nil
}

func isLegacyWorkPackageID(value string) bool {
	if len(value) != len("WP-000") || !strings.HasPrefix(value, "WP-") {
		return false
	}
	for _, digit := range value[len("WP-"):] {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

func ensureNoForeignKeyViolations(ctx context.Context, tx *sql.Tx) (retErr error) {
	rows, err := tx.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return fmt.Errorf("run foreign key check: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("close foreign key check: %w", closeErr))
		}
	}()

	if rows.Next() {
		var (
			tableName   string
			rowID       sql.NullInt64
			parentTable string
			foreignKey  int
		)
		if err := rows.Scan(&tableName, &rowID, &parentTable, &foreignKey); err != nil {
			return fmt.Errorf("scan foreign key violation: %w", err)
		}
		return fmt.Errorf(
			"%w: table=%q row_id=%v parent=%q foreign_key=%d",
			errWorkflowForeignKeyViolation,
			tableName,
			rowID,
			parentTable,
			foreignKey,
		)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate foreign key check: %w", err)
	}
	return nil
}
