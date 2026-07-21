package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
)

func TestApplyMigrationsIsIdempotent(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
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
		"artifact_bodies",
		"artifact_snapshots",
		"review_issues",
		"review_rounds",
		"runs",
		"schema_migrations",
		"sync_checkpoints",
		"task_items",
		"workflows",
		"workspaces",
	}
	for _, tableName := range requiredTables {
		if _, ok := beforeSchema["table:"+tableName]; !ok {
			t.Fatalf("missing required table %q in schema snapshot", tableName)
		}
	}
}

func TestApplyMigrationsUpgradesExistingCatalogWithWorkflowHierarchy(t *testing.T) {
	// Suite boundary
	// IN: a real v5 SQLite catalog upgraded through the registered migration chain
	// OUT: workflow reconciliation behavior, covered by sync tests
	// Invariant: hierarchy metadata is additive and active sibling task group identity is unique.
	t.Parallel()

	sqlDB, err := store.OpenSQLiteDatabase(
		context.Background(),
		filepath.Join(t.TempDir(), "v5-catalog.db"),
		func(ctx context.Context, db *sql.DB) error {
			if err := store.EnsureSchema(ctx, db, migrationTableStatements); err != nil {
				return err
			}
			for _, item := range migrations[:len(migrations)-1] {
				if err := applyMigration(ctx, db, item, time.Now); err != nil {
					return err
				}
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("OpenSQLiteDatabase(v5): %v", err)
	}
	defer func() {
		_ = sqlDB.Close()
	}()

	if err := applyMigrations(context.Background(), sqlDB, time.Now); err != nil {
		t.Fatalf("applyMigrations(v5): %v", err)
	}
	columns := make(map[string]bool)
	rows, err := sqlDB.QueryContext(context.Background(), `PRAGMA table_info(workflows)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info(workflows): %v", err)
	}
	defer func() {
		_ = rows.Close()
	}()
	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal any
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &primaryKey); err != nil {
			t.Fatalf("scan workflows column: %v", err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate workflows columns: %v", err)
	}
	for _, name := range []string{
		"kind",
		"parent_workflow_id",
		"task_group_id",
		"display_title",
		"outcome",
		"lifecycle_completed",
		"dependencies_json",
	} {
		if !columns[name] {
			t.Fatalf("workflow hierarchy column %q is missing", name)
		}
	}

	var indexSQL string
	if err := sqlDB.QueryRowContext(
		context.Background(),
		`SELECT sql FROM sqlite_master WHERE type = 'index' AND name = ?`,
		"uq_workflows_active_child_task_group",
	).Scan(&indexSQL); err != nil {
		t.Fatalf("query active child uniqueness index: %v", err)
	}
	if !strings.Contains(indexSQL, "parent_workflow_id, task_group_id") ||
		!strings.Contains(indexSQL, "archived_at IS NULL") {
		t.Fatalf("active child uniqueness index = %q", indexSQL)
	}
}

func TestApplyMigrationsUpgradesAppliedWorkPackageHierarchyToTaskGroups(t *testing.T) {
	// Suite boundary
	// IN: a real SQLite catalog where the historical Work Package migration is already applied
	// OUT: workflow artifact reconciliation, covered by sync tests
	// Invariant: upgrading preserves workflow-linked data while translating every persisted child identity.
	t.Parallel()

	sqlDB := openAppliedWorkPackageCatalog(t)
	defer func() {
		_ = sqlDB.Close()
	}()
	sqlDB.SetMaxOpenConns(1)

	fixedNow := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	if err := applyMigrations(context.Background(), sqlDB, func() time.Time { return fixedNow }); err != nil {
		t.Fatalf("applyMigrations(applied Work Package catalog): %v", err)
	}

	wantWorkflows := []workflowMigrationSnapshot{
		{
			ID:                 "wf-child",
			WorkspaceID:        "ws-test",
			Slug:               "foods-new-form-consistency/TG-004",
			LastSyncedAt:       "2026-07-21T00:30:00Z",
			CreatedAt:          "2026-07-21T00:00:00Z",
			UpdatedAt:          "2026-07-21T00:00:00Z",
			Kind:               "task_group",
			ParentWorkflowID:   "wf-parent",
			TaskGroupID:        "TG-004",
			DisplayTitle:       "Dirty Exit",
			Outcome:            "Protect exits",
			LifecycleCompleted: 1,
			DependenciesJSON:   `[{"task_group_id":"TG-003","rationale":"Preserve WP-prefixed user text."}]`,
			Missing:            1,
		},
		{
			ID:               "wf-archived",
			WorkspaceID:      "ws-test",
			Slug:             "foods-new-form-consistency/TG-004",
			ArchivedAt:       "2026-07-20T00:00:00Z",
			CreatedAt:        "2026-07-19T00:00:00Z",
			UpdatedAt:        "2026-07-20T00:00:00Z",
			Kind:             "task_group",
			ParentWorkflowID: "wf-parent",
			TaskGroupID:      "TG-004",
			DisplayTitle:     "Archived",
			Outcome:          "Historical workflow",
			DependenciesJSON: "[]",
		},
		{
			ID:               "wf-parent",
			WorkspaceID:      "ws-test",
			Slug:             "foods-new-form-consistency",
			CreatedAt:        "2026-07-21T00:00:00Z",
			UpdatedAt:        "2026-07-21T00:00:00Z",
			Kind:             "initiative",
			DisplayTitle:     "Foods",
			Outcome:          "Parent workflow",
			DependenciesJSON: "[]",
		},
	}
	for _, want := range wantWorkflows {
		if got := loadMigratedWorkflow(t, sqlDB, want.ID); !reflect.DeepEqual(got, want) {
			t.Fatalf("migrated workflow %q = %#v, want %#v", want.ID, got, want)
		}
	}

	workflowReferences := []struct {
		name  string
		query string
		key   string
	}{
		{"artifact snapshot", `SELECT workflow_id FROM artifact_snapshots WHERE artifact_kind = ?`, "prd"},
		{"review round", `SELECT workflow_id FROM review_rounds WHERE id = ?`, "round-child"},
		{"run", `SELECT workflow_id FROM runs WHERE run_id = ?`, "run-child"},
		{"sync checkpoint", `SELECT workflow_id FROM sync_checkpoints WHERE scope = ?`, "workflow"},
		{"task item", `SELECT workflow_id FROM task_items WHERE id = ?`, "task-child"},
	}
	for _, reference := range workflowReferences {
		var workflowID string
		if err := sqlDB.QueryRowContext(context.Background(), reference.query, reference.key).
			Scan(&workflowID); err != nil {
			t.Fatalf("query preserved %s: %v", reference.name, err)
		}
		if workflowID != "wf-child" {
			t.Fatalf("%s workflow ID = %q, want %q", reference.name, workflowID, "wf-child")
		}
	}

	columns := workflowColumnNames(t, sqlDB)
	if columns["package_id"] {
		t.Fatal("legacy package_id column still exists after migration")
	}
	if !columns["task_group_id"] {
		t.Fatal("task_group_id column is missing after migration")
	}

	migrationRows := loadMigrationRows(t, sqlDB)
	lastMigration := migrationRows[len(migrationRows)-1]
	if got, want := lastMigration, (migrationRow{
		Version:   9,
		Name:      "work_package_to_task_group",
		AppliedAt: store.FormatTimestamp(fixedNow),
	}); got != want {
		t.Fatalf("last migration row = %#v, want %#v", got, want)
	}

	assertForeignKeysEnabled(t, sqlDB)
	assertNoForeignKeyViolations(t, sqlDB)
}

func TestApplyMigrationsRejectsInvalidLegacyWorkPackageCatalogs(t *testing.T) {
	// Suite boundary
	// IN: real SQLite catalogs with invalid persisted Work Package hierarchy data
	// OUT: artifact parsing, covered by task group plan tests
	// Invariant: invalid legacy catalogs fail atomically and leave foreign key enforcement enabled.
	t.Parallel()

	tests := []struct {
		name      string
		wantError error
		mutate    func(*testing.T, *sql.DB)
	}{
		{
			name:      "malformed package ID",
			wantError: errInvalidLegacyWorkPackageIdentity,
			mutate: func(t *testing.T, db *sql.DB) {
				execMigrationTestSQL(
					t,
					db,
					`UPDATE workflows SET slug = ?, package_id = ? WHERE id = ?`,
					"foods-new-form-consistency/legacy-004",
					"legacy-004",
					"wf-child",
				)
			},
		},
		{
			name:      "package ID and slug disagree",
			wantError: errInvalidLegacyWorkPackageIdentity,
			mutate: func(t *testing.T, db *sql.DB) {
				execMigrationTestSQL(
					t,
					db,
					`UPDATE workflows SET slug = ? WHERE id = ?`,
					"foods-new-form-consistency/WP-005",
					"wf-child",
				)
			},
		},
		{
			name:      "non-package workflow carries package ID",
			wantError: errInvalidLegacyWorkPackageIdentity,
			mutate: func(t *testing.T, db *sql.DB) {
				execMigrationTestSQL(
					t,
					db,
					`UPDATE workflows SET package_id = ? WHERE id = ?`,
					"WP-001",
					"wf-parent",
				)
			},
		},
		{
			name:      "malformed dependency JSON",
			wantError: errInvalidLegacyWorkflowDependencies,
			mutate: func(t *testing.T, db *sql.DB) {
				execMigrationTestSQL(
					t,
					db,
					`UPDATE workflows SET dependencies_json = ? WHERE id = ?`,
					`{"package_id":`,
					"wf-child",
				)
			},
		},
		{
			name:      "null dependency collection",
			wantError: errInvalidLegacyWorkflowDependencies,
			mutate: func(t *testing.T, db *sql.DB) {
				execMigrationTestSQL(
					t,
					db,
					`UPDATE workflows SET dependencies_json = ? WHERE id = ?`,
					"null",
					"wf-child",
				)
			},
		},
		{
			name:      "invalid dependency package ID",
			wantError: errInvalidLegacyWorkPackageDependency,
			mutate: func(t *testing.T, db *sql.DB) {
				execMigrationTestSQL(
					t,
					db,
					`UPDATE workflows SET dependencies_json = ? WHERE id = ?`,
					`[{"package_id":"TG-003","rationale":"Wrong namespace."}]`,
					"wf-child",
				)
			},
		},
		{
			name:      "both hierarchy ID columns exist",
			wantError: errAmbiguousWorkflowHierarchySchema,
			mutate: func(t *testing.T, db *sql.DB) {
				execMigrationTestSQL(
					t,
					db,
					`ALTER TABLE workflows ADD COLUMN task_group_id TEXT NOT NULL DEFAULT ''`,
				)
			},
		},
		{
			name:      "preexisting foreign key violation",
			wantError: errWorkflowForeignKeyViolation,
			mutate: func(t *testing.T, db *sql.DB) {
				setInvalidLegacyParent(t, db)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sqlDB := openAppliedWorkPackageCatalog(t)
			defer func() {
				_ = sqlDB.Close()
			}()
			sqlDB.SetMaxOpenConns(1)
			tt.mutate(t, sqlDB)

			beforeSchema := loadSchemaSnapshot(t, sqlDB)
			beforeWorkflows := loadLegacyWorkflowRows(t, sqlDB)
			err := applyMigrations(context.Background(), sqlDB, time.Now)
			if !errors.Is(err, tt.wantError) {
				t.Fatalf("applyMigrations(invalid legacy catalog) error = %v, want %v", err, tt.wantError)
			}
			if afterSchema := loadSchemaSnapshot(t, sqlDB); !reflect.DeepEqual(afterSchema, beforeSchema) {
				t.Fatalf("failed migration changed schema\nbefore: %#v\nafter:  %#v", beforeSchema, afterSchema)
			}
			if afterWorkflows := loadLegacyWorkflowRows(t, sqlDB); !reflect.DeepEqual(afterWorkflows, beforeWorkflows) {
				t.Fatalf(
					"failed migration changed workflow data\nbefore: %#v\nafter:  %#v",
					beforeWorkflows,
					afterWorkflows,
				)
			}
			if got, want := len(loadMigrationRows(t, sqlDB)), 8; got != want {
				t.Fatalf("migration history row count = %d, want %d", got, want)
			}
			assertForeignKeysEnabled(t, sqlDB)
		})
	}
}

func TestApplyMigrationsRejectsSchemaTooNew(t *testing.T) {
	t.Parallel()

	fixedNow := time.Date(2026, 4, 17, 19, 0, 0, 0, time.UTC)
	sqlDB, err := store.OpenSQLiteDatabase(
		context.Background(),
		filepath.Join(t.TempDir(), "future.db"),
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
	if got := schemaErr.Error(); got == "" {
		t.Fatal("SchemaTooNewError.Error() returned an empty message")
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

var appliedWorkPackageHierarchyMigration = migration{
	version: 6,
	name:    "workflow_hierarchy",
	statements: []string{
		`ALTER TABLE workflows ADD COLUMN kind TEXT NOT NULL DEFAULT 'ordinary'
			CHECK (kind IN ('ordinary', 'initiative', 'work_package'));`,
		`ALTER TABLE workflows ADD COLUMN parent_workflow_id TEXT REFERENCES workflows(id);`,
		`ALTER TABLE workflows ADD COLUMN package_id TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE workflows ADD COLUMN display_title TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE workflows ADD COLUMN outcome TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE workflows ADD COLUMN lifecycle_completed INTEGER NOT NULL DEFAULT 0
			CHECK (lifecycle_completed IN (0, 1));`,
		`ALTER TABLE workflows ADD COLUMN dependencies_json TEXT NOT NULL DEFAULT '[]';`,
		`CREATE INDEX IF NOT EXISTS idx_workflows_parent_workflow_id
			ON workflows(parent_workflow_id)
			WHERE parent_workflow_id IS NOT NULL;`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_workflows_active_child_package
			ON workflows(parent_workflow_id, package_id)
			WHERE archived_at IS NULL
			  AND parent_workflow_id IS NOT NULL
			  AND package_id <> '';`,
	},
}

func openAppliedWorkPackageCatalog(t *testing.T) *sql.DB {
	t.Helper()

	sqlDB, err := store.OpenSQLiteDatabase(
		context.Background(),
		filepath.Join(t.TempDir(), "applied-work-package.db"),
		func(ctx context.Context, db *sql.DB) error {
			if err := store.EnsureSchema(ctx, db, migrationTableStatements); err != nil {
				return err
			}
			for _, item := range migrations {
				switch {
				case item.version < appliedWorkPackageHierarchyMigration.version:
					if err := applyMigration(ctx, db, item, time.Now); err != nil {
						return err
					}
				case item.version == appliedWorkPackageHierarchyMigration.version:
					if err := applyMigration(ctx, db, appliedWorkPackageHierarchyMigration, time.Now); err != nil {
						return err
					}
				case item.version <= 8:
					if err := applyMigration(ctx, db, item, time.Now); err != nil {
						return err
					}
				}
			}

			seedStatements := []struct {
				name  string
				query string
				args  []any
			}{
				{
					name: "workspace",
					query: `INSERT INTO workspaces (id, root_dir, name, created_at, updated_at)
						VALUES (?, ?, ?, ?, ?)`,
					args: []any{
						"ws-test",
						"/tmp/workspace",
						"workspace",
						"2026-07-21T00:00:00Z",
						"2026-07-21T00:00:00Z",
					},
				},
				{
					name: "initiative workflow",
					query: `INSERT INTO workflows (
						id, workspace_id, slug, created_at, updated_at, kind, display_title, outcome
					) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
					args: []any{
						"wf-parent", "ws-test", "foods-new-form-consistency",
						"2026-07-21T00:00:00Z", "2026-07-21T00:00:00Z",
						"initiative", "Foods", "Parent workflow",
					},
				},
				{
					name: "child workflow",
					query: `INSERT INTO workflows (
						id, workspace_id, slug, last_synced_at, created_at, updated_at, kind,
						parent_workflow_id, package_id, display_title, outcome, lifecycle_completed,
						dependencies_json, missing
					) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					args: []any{
						"wf-child", "ws-test", "foods-new-form-consistency/WP-004",
						"2026-07-21T00:30:00Z", "2026-07-21T00:00:00Z", "2026-07-21T00:00:00Z",
						"work_package", "wf-parent", "WP-004", "Dirty Exit", "Protect exits", 1,
						`[{"package_id":"WP-003","rationale":"Preserve WP-prefixed user text."}]`, 1,
					},
				},
				{
					name: "archived child workflow",
					query: `INSERT INTO workflows (
						id, workspace_id, slug, archived_at, created_at, updated_at, kind,
						parent_workflow_id, package_id, display_title, outcome, dependencies_json
					) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					args: []any{
						"wf-archived", "ws-test", "foods-new-form-consistency/WP-004",
						"2026-07-20T00:00:00Z", "2026-07-19T00:00:00Z", "2026-07-20T00:00:00Z",
						"work_package", "wf-parent", "WP-004", "Archived", "Historical workflow", "[]",
					},
				},
				{
					name: "artifact snapshot",
					query: `INSERT INTO artifact_snapshots (
						workflow_id, artifact_kind, relative_path, checksum, source_mtime, synced_at
					) VALUES (?, ?, ?, ?, ?, ?)`,
					args: []any{
						"wf-child", "prd", "_prd.md", "checksum-prd",
						"2026-07-21T00:00:00Z", "2026-07-21T00:00:00Z",
					},
				},
				{
					name: "task item",
					query: `INSERT INTO task_items (
						id, workflow_id, task_number, task_id, title, status, kind,
						depends_on_json, source_path, updated_at
					) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					args: []any{
						"task-child", "wf-child", 1, "task_01", "Protect exits", "pending", "frontend",
						"[]", "task_01.md", "2026-07-21T00:00:00Z",
					},
				},
				{
					name: "review round",
					query: `INSERT INTO review_rounds (id, workflow_id, round_number, provider, updated_at)
						VALUES (?, ?, ?, ?, ?)`,
					args: []any{
						"round-child", "wf-child", 1, "coderabbit", "2026-07-21T00:00:00Z",
					},
				},
				{
					name: "run",
					query: `INSERT INTO runs (
						run_id, workspace_id, workflow_id, mode, status, presentation_mode, started_at
					) VALUES (?, ?, ?, ?, ?, ?, ?)`,
					args: []any{
						"run-child", "ws-test", "wf-child", "tasks", "completed", "stream",
						"2026-07-21T00:00:00Z",
					},
				},
				{
					name: "sync checkpoint",
					query: `INSERT INTO sync_checkpoints (workflow_id, scope, checksum)
						VALUES (?, ?, ?)`,
					args: []any{"wf-child", "workflow", "checksum-sync"},
				},
			}
			for _, statement := range seedStatements {
				if _, err := db.ExecContext(ctx, statement.query, statement.args...); err != nil {
					return fmt.Errorf("seed %s: %w", statement.name, err)
				}
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("open applied Work Package catalog: %v", err)
	}
	return sqlDB
}

func workflowColumnNames(t *testing.T, sqlDB *sql.DB) map[string]bool {
	t.Helper()

	rows, err := sqlDB.QueryContext(context.Background(), `PRAGMA table_info(workflows)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info(workflows): %v", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	columns := make(map[string]bool)
	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal any
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &primaryKey); err != nil {
			t.Fatalf("scan workflows column: %v", err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate workflows columns: %v", err)
	}
	return columns
}

type workflowMigrationSnapshot struct {
	ID                 string
	WorkspaceID        string
	Slug               string
	ArchivedAt         string
	LastSyncedAt       string
	CreatedAt          string
	UpdatedAt          string
	Kind               string
	ParentWorkflowID   string
	TaskGroupID        string
	DisplayTitle       string
	Outcome            string
	LifecycleCompleted int
	DependenciesJSON   string
	Missing            int
}

func loadMigratedWorkflow(t *testing.T, sqlDB *sql.DB, workflowID string) workflowMigrationSnapshot {
	t.Helper()

	var workflow workflowMigrationSnapshot
	if err := sqlDB.QueryRowContext(
		context.Background(),
		`SELECT
			id,
			workspace_id,
			slug,
			COALESCE(archived_at, ''),
			COALESCE(last_synced_at, ''),
			created_at,
			updated_at,
			kind,
			COALESCE(parent_workflow_id, ''),
			task_group_id,
			display_title,
			outcome,
			lifecycle_completed,
			dependencies_json,
			missing
		 FROM workflows
		 WHERE id = ?`,
		workflowID,
	).Scan(
		&workflow.ID,
		&workflow.WorkspaceID,
		&workflow.Slug,
		&workflow.ArchivedAt,
		&workflow.LastSyncedAt,
		&workflow.CreatedAt,
		&workflow.UpdatedAt,
		&workflow.Kind,
		&workflow.ParentWorkflowID,
		&workflow.TaskGroupID,
		&workflow.DisplayTitle,
		&workflow.Outcome,
		&workflow.LifecycleCompleted,
		&workflow.DependenciesJSON,
		&workflow.Missing,
	); err != nil {
		t.Fatalf("query migrated workflow %q: %v", workflowID, err)
	}
	return workflow
}

type legacyWorkflowRow struct {
	ID               string
	Kind             string
	Slug             string
	ParentWorkflowID string
	PackageID        string
	DependenciesJSON string
}

func loadLegacyWorkflowRows(t *testing.T, sqlDB *sql.DB) []legacyWorkflowRow {
	t.Helper()

	rows, err := sqlDB.QueryContext(
		context.Background(),
		`SELECT id, kind, slug, COALESCE(parent_workflow_id, ''), package_id, dependencies_json
		 FROM workflows
		 ORDER BY id ASC`,
	)
	if err != nil {
		t.Fatalf("query legacy workflows: %v", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	workflows := make([]legacyWorkflowRow, 0)
	for rows.Next() {
		var workflow legacyWorkflowRow
		if err := rows.Scan(
			&workflow.ID,
			&workflow.Kind,
			&workflow.Slug,
			&workflow.ParentWorkflowID,
			&workflow.PackageID,
			&workflow.DependenciesJSON,
		); err != nil {
			t.Fatalf("scan legacy workflow: %v", err)
		}
		workflows = append(workflows, workflow)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate legacy workflows: %v", err)
	}
	return workflows
}

func execMigrationTestSQL(t *testing.T, sqlDB *sql.DB, query string, args ...any) {
	t.Helper()

	if _, err := sqlDB.ExecContext(context.Background(), query, args...); err != nil {
		t.Fatalf("execute migration fixture mutation: %v", err)
	}
}

func setInvalidLegacyParent(t *testing.T, sqlDB *sql.DB) {
	t.Helper()

	conn, err := sqlDB.Conn(context.Background())
	if err != nil {
		t.Fatalf("acquire migration fixture connection: %v", err)
	}
	if _, err := conn.ExecContext(context.Background(), `PRAGMA foreign_keys = OFF`); err != nil {
		_ = conn.Close()
		t.Fatalf("disable migration fixture foreign keys: %v", err)
	}
	if _, err := conn.ExecContext(
		context.Background(),
		`UPDATE workflows SET parent_workflow_id = ? WHERE id = ?`,
		"wf-missing",
		"wf-child",
	); err != nil {
		_ = conn.Close()
		t.Fatalf("seed invalid legacy parent: %v", err)
	}
	if _, err := conn.ExecContext(context.Background(), `PRAGMA foreign_keys = ON`); err != nil {
		_ = conn.Close()
		t.Fatalf("restore migration fixture foreign keys: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("close migration fixture connection: %v", err)
	}
}

func assertForeignKeysEnabled(t *testing.T, sqlDB *sql.DB) {
	t.Helper()

	var enabled int
	if err := sqlDB.QueryRowContext(context.Background(), `PRAGMA foreign_keys`).Scan(&enabled); err != nil {
		t.Fatalf("query foreign key state: %v", err)
	}
	if enabled != 1 {
		t.Fatalf("foreign key state = %d, want 1", enabled)
	}
}

func assertNoForeignKeyViolations(t *testing.T, sqlDB *sql.DB) {
	t.Helper()

	rows, err := sqlDB.QueryContext(context.Background(), `PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatalf("PRAGMA foreign_key_check: %v", err)
	}
	defer func() {
		_ = rows.Close()
	}()
	if rows.Next() {
		t.Fatal("foreign key check returned a violation")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate foreign key violations: %v", err)
	}
}

func TestOpenUsesExportedConstructor(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "opened.db")
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
		filepath.Join(t.TempDir(), "broken.db"),
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

func openTestGlobalDB(t *testing.T) *GlobalDB {
	t.Helper()

	var counter atomic.Int64
	fixedNow := time.Date(2026, 4, 17, 18, 0, 0, 0, time.UTC)

	db, err := openWithOptions(
		context.Background(),
		filepath.Join(t.TempDir(), "global.db"),
		openOptions{
			now: func() time.Time {
				return fixedNow
			},
			newID: func(prefix string) string {
				seq := counter.Add(1)
				return prefix + "-" + time.Date(2026, 4, 17, 18, 0, 0, int(seq), time.UTC).Format("150405.000000000")
			},
		},
	)
	if err != nil {
		t.Fatalf("open test global db: %v", err)
	}
	return db
}
