package globaldb

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/extensionenv"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	memorypkg "github.com/compozy/compozy/internal/memory"
	"github.com/compozy/compozy/internal/network/participation"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	globalschema "github.com/compozy/compozy/internal/store/globaldb/schema"
	"github.com/compozy/compozy/internal/store/sessiondb"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/vault"
	compozyworkspace "github.com/compozy/compozy/internal/workspace"
	worktreepkg "github.com/compozy/compozy/internal/worktree"
	"github.com/pressly/goose/v3"
)

func eventSummaryWithContent(summary EventSummary, content json.RawMessage) EventSummary {
	summary.SetContent(content)
	return summary
}

type SessionInfo = store.SessionInfo
type SessionStateUpdate = store.SessionStateUpdate
type SessionListQuery = store.SessionListQuery
type EventSummary = store.EventSummary
type EventSummaryQuery = store.EventSummaryQuery
type TokenStats = store.TokenStats
type TokenStatsUpdate = store.TokenStatsUpdate
type TokenStatsQuery = store.TokenStatsQuery
type PermissionLogEntry = store.PermissionLogEntry
type PermissionLogQuery = store.PermissionLogQuery

const GlobalDatabaseName = store.GlobalDatabaseName
const defaultSessionType = "user"

const globalDBExtensionProvenanceJSONKey = "provenance_json"
const sqliteDriverName = "sqlite"

var testGlobalDBCurrentSchemaSeedPath string
var testGlobalDBMigrationTemplatePaths map[string]string

func formatTimestamp(value time.Time) string {
	return store.FormatTimestamp(value)
}

func sqliteDSN(path string) string {
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
}

func SessionMetaFile(sessionDir string) string {
	return store.SessionMetaFile(sessionDir)
}

func ReadSessionMeta(path string) (store.SessionMeta, error) {
	return store.ReadSessionMeta(path)
}

func TestMain(m *testing.M) {
	code := runGlobalDBTests(m)
	os.Exit(code)
}

func runGlobalDBTests(m *testing.M) (code int) {
	dir, err := os.MkdirTemp("", "compozy-globaldb-current-schema-*")
	if err != nil {
		reportTestMainError("MkdirTemp(globaldb seed) error = %v", err)
		return 1
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			reportTestMainError("RemoveAll(globaldb seed) error = %v", err)
			if code == 0 {
				code = 1
			}
		}
	}()

	ctx := context.Background()
	path, migrationTemplates, err := buildGlobalMigrationTestSeeds(ctx, dir)
	if err != nil {
		reportTestMainError("build global migration seeds error = %v", err)
		return 1
	}
	globalDB, err := OpenGlobalDB(ctx, path)
	if err != nil {
		reportTestMainError("OpenGlobalDB(globaldb seed) error = %v", err)
		return 1
	}
	memoryStore := memorypkg.NewStore(
		filepath.Join(dir, "memory"),
		memorypkg.WithCatalogDatabasePath(path),
	)
	if err := memoryStore.OpenCatalog(ctx); err != nil {
		reportTestMainError("OpenCatalog(globaldb seed) error = %v", err)
		return 1
	}
	if err := memoryStore.CloseCatalog(ctx); err != nil {
		reportTestMainError("CloseCatalog(globaldb seed) error = %v", err)
		return 1
	}
	if err := globalDB.Close(ctx); err != nil {
		reportTestMainError("Close(globaldb seed) error = %v", err)
		return 1
	}

	testGlobalDBCurrentSchemaSeedPath = path
	testGlobalDBMigrationTemplatePaths = migrationTemplates
	return m.Run()
}

func reportTestMainError(format string, args ...any) {
	if _, err := fmt.Fprintf(os.Stderr, format+"\n", args...); err != nil {
		panic(err)
	}
}

func TestOpenGlobalDBAppliesGlobalMigrationsAndEnablesWAL(t *testing.T) {
	t.Parallel()
	t.Run("Should verify the exact permanent default profile without repairing it", func(t *testing.T) {
		t.Parallel()

		globalDB := openFreshTestGlobalDB(t)
		ctx := testutil.Context(t)
		if err := globalDB.VerifyDefaultProfile(ctx); err != nil {
			t.Fatalf("VerifyDefaultProfile() error = %v", err)
		}
		var handoffTables int
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM sqlite_master
			 WHERE type = 'table' AND name = 'phase0_operator_home_context'`,
		).Scan(&handoffTables); err != nil {
			t.Fatalf("inspect operator-home handoff table error = %v", err)
		}
		if handoffTables != 0 {
			t.Fatalf("operator-home handoff table count = %d, want migration-only state", handoffTables)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE profiles SET color = '#000000' WHERE id = ?`,
			store.DefaultProfileID,
		); err != nil {
			t.Fatalf("corrupt default profile fixture error = %v", err)
		}
		if err := globalDB.VerifyDefaultProfile(ctx); err == nil || !strings.Contains(
			err.Error(),
			"does not match the permanent identity",
		) {
			t.Fatalf("VerifyDefaultProfile(corrupt) error = %v, want permanent identity mismatch", err)
		}
		var color string
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT color FROM profiles WHERE id = ?`,
			store.DefaultProfileID,
		).Scan(&color); err != nil {
			t.Fatalf("read corrupt default profile error = %v", err)
		}
		if color != "#000000" {
			t.Fatalf("VerifyDefaultProfile() repaired color = %q, want verify-only state", color)
		}
	})

	t.Run("Should apply the complete global migration stream before repository use", func(t *testing.T) {
		t.Parallel()

		globalDB := openFreshTestGlobalDB(t)

		assertTablesPresent(
			t,
			globalDB.db,
			globalMigrationVersionTable,
			"workspaces",
			"sessions",
			"session_pending_interactions",
			"event_summaries",
			"token_stats",
			"permission_log",
			"extensions",
			"extension_dev_links",
			"config_apply_records",
			"network_channel_stats",
			"network_channel_participants",
			"network_channel_kind_counts",
			"workspace_network_coordination",
			"network_coordination_invitations",
			"network_availability",
			"network_message_dispositions",
			"network_live_wakes",
			"network_wake_sources",
			"network_participation_budgets",
			"network_task_status_projections",
			"scheduler_pause",
			"task_run_terminal_commands",
			"worktrees",
			"worktree_status",
			"worktree_forge_status",
			"worktree_exit_ops",
		)
		assertTableHasColumns(t, globalDB.db, "sessions", []string{
			"worktree_id",
			"notify_creator",
			"pending_permission_count",
			"pending_clarify_count",
			"attention_revision",
			"last_settled_revision",
			"last_seen_revision",
			"last_seen_at",
			"attention_changed_at",
		})
		assertTableHasColumns(t, globalDB.db, "session_pending_interactions", []string{
			"interaction_id",
			"session_id",
			"kind",
			"provider_request_id",
			"payload_json",
			"status",
			"resolution",
		})
		assertIndexesPresent(
			t,
			globalDB.db,
			"session_pending_interactions",
			"idx_session_pending_interactions_session_status",
			"uq_session_pending_interactions_active_provider_request",
		)
		assertTableHasColumns(t, globalDB.db, "event_summaries", []string{"worktree_id"})
		assertTableHasColumns(t, globalDB.db, "extensions", []string{"format", "ingest_diagnostics_json"})
		assertTableHasColumns(
			t,
			globalDB.db,
			"extension_dev_links",
			[]string{"format", "ingest_diagnostics_json"},
		)
		assertTableHasColumns(t, globalDB.db, "extension_env_bindings", []string{"mcp_server", "header_name"})
		for _, table := range []string{"sessions", "task_runs", "loop_runs"} {
			assertTableHasColumns(t, globalDB.db, table, []string{
				"network_spec_json",
				"network_mode",
				"network_channel",
				"network_source",
			})
		}
		assertTableExcludesColumns(t, globalDB.db, "sessions", []string{"channel"})
		assertTableExcludesColumns(t, globalDB.db, "tasks", []string{"network_channel"})
		assertTableExcludesColumns(t, globalDB.db, "task_runs", []string{"coordination_channel_id"})
		assertTableHasColumns(t, globalDB.db, "network_channel_participants", []string{"session_id"})
		assertTableHasColumns(t, globalDB.db, "network_direct_rooms", []string{"session_a", "session_b"})
		assertTableHasColumns(t, globalDB.db, "network_subscriptions", []string{"session_id"})
		assertTableHasColumns(t, globalDB.db, "network_thread_participants", []string{"session_id"})
		assertTableExcludesColumns(t, globalDB.db, "network_channel_participants", []string{"peer_id"})
		assertTableExcludesColumns(t, globalDB.db, "network_direct_rooms", []string{"peer_a", "peer_b"})
		assertTableExcludesColumns(t, globalDB.db, "network_subscriptions", []string{"peer_id"})
		assertTableExcludesColumns(t, globalDB.db, "network_thread_participants", []string{"peer_id"})
		assertTableHasColumns(t, globalDB.db, "event_summaries", []string{
			"provider",
			"outcome",
		})
		assertTableHasColumns(t, globalDB.db, "model_catalog_rows", []string{
			"cost_cache_read_per_million",
			"cost_cache_write_per_million",
			"cost_reasoning_per_million",
		})
		for _, absent := range []string{"schema_migrations", "memory_events", "goose_db_version_memory"} {
			exists, err := tableExists(testutil.Context(t), globalDB.db, absent)
			if err != nil {
				t.Fatalf("tableExists(%q) error = %v", absent, err)
			}
			if exists {
				t.Fatalf("global stream unexpectedly owns table %q", absent)
			}
		}
		assertTableColumns(t, globalDB.db, "config_apply_records", []string{
			"id",
			"desired_config_hash",
			"active_config_hash",
			"generation",
			"actor",
			"write_target",
			"write_path",
			"diff_class",
			"status",
			"diagnostic_json",
			"created_at",
			"applied_at",
			"updated_at",
		})
		assertIndexesPresent(
			t,
			globalDB.db,
			"config_apply_records",
			"idx_config_apply_records_desired",
			"idx_config_apply_records_active",
			"idx_config_apply_records_generation",
			"idx_config_apply_records_actor",
			"idx_config_apply_records_status",
		)
		assertIndexesPresent(
			t,
			globalDB.db,
			"network_channel_stats",
			"idx_network_channel_stats_activity",
		)
		assertIndexesPresent(
			t,
			globalDB.db,
			"network_threads",
			"idx_network_threads_created",
			"idx_network_threads_title",
			"idx_network_threads_open_work",
		)
		assertTableColumns(t, globalDB.db, "task_run_terminal_commands", []string{
			"command_id",
			"run_id",
			"task_id",
			"workspace_id",
			"kind",
			"phase",
			"source_status",
			"source_session_id",
			"source_claim_token_hash",
			"source_lease_until",
			"intent_json",
			"actor_json",
			"command_at",
			"admitted_at",
			"updated_at",
		})
		assertIndexesPresent(
			t,
			globalDB.db,
			"task_run_terminal_commands",
			"idx_task_run_terminal_commands_phase",
		)
		var terminalGuardCount int
		if err := globalDB.db.QueryRowContext(
			testutil.Context(t),
			`SELECT COUNT(*) FROM sqlite_master
			 WHERE type = 'trigger'
			   AND name IN (
				   'trg_task_runs_terminal_command_guard',
				   'trg_task_runs_terminal_command_delete_guard',
				   'trg_tasks_terminal_command_delete_guard'
			   )`,
		).Scan(&terminalGuardCount); err != nil {
			t.Fatalf("query terminal command guard trigger error = %v", err)
		}
		if terminalGuardCount != 3 {
			t.Fatalf("terminal command guard trigger count = %d, want 3", terminalGuardCount)
		}
		assertJournalModeWAL(t, globalDB.db)
		assertSynchronousNormal(t, globalDB.db)
		status, err := store.Status(testutil.Context(t), globalDB.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(global) error = %v", err)
		}
		assertCompleteMigrationStream(t, status, MigrationStream())
		workspaces, err := globalDB.ListWorkspaces(testutil.Context(t))
		if err != nil {
			t.Fatalf("ListWorkspaces() error = %v", err)
		}
		if len(workspaces) != 0 {
			t.Fatalf("ListWorkspaces() = %#v, want empty fresh repository", workspaces)
		}
	})
}

func TestGlobalDBRepositoryComposition(t *testing.T) {
	t.Run("Should expose unique repository methods without lifecycle ownership", func(t *testing.T) {
		t.Parallel()

		facadeType := reflect.TypeFor[GlobalDB]()
		methodOwners := make(map[string]string)
		repositoryCount := 0
		for field := range facadeType.Fields() {
			if !isRepositoryField(field) {
				continue
			}
			repositoryCount++
			repositoryType := field.Type
			for method := range repositoryType.Methods() {
				if method.Name == "Close" || method.Name == "Path" || method.Name == "Checkpoint" {
					t.Fatalf("repository %s owns facade lifecycle method %s", field.Name, method.Name)
				}
				if owner, exists := methodOwners[method.Name]; exists {
					t.Fatalf("repository method %s is ambiguous between %s and %s", method.Name, owner, field.Name)
				}
				methodOwners[method.Name] = field.Name
			}
		}
		if repositoryCount == 0 {
			t.Fatal("repository reflection found no embedded repositories")
		}
	})

	t.Run("Should initialize every embedded repository after open", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		facadeType := reflect.TypeFor[GlobalDB]()
		facadeValue := reflect.ValueOf(globalDB).Elem()
		for field := range facadeType.Fields() {
			if !isRepositoryField(field) {
				continue
			}
			if facadeValue.FieldByIndex(field.Index).IsNil() {
				t.Fatalf("embedded repository %s is nil after OpenGlobalDB", field.Name)
			}
		}
	})
}

func isRepositoryField(field reflect.StructField) bool {
	return field.Anonymous &&
		field.Type.Kind() == reflect.Pointer &&
		strings.HasSuffix(field.Type.Elem().Name(), "Repo")
}

func TestOpenGlobalDBReopenPreservesRowsAndStatus(t *testing.T) {
	t.Run("Should preserve loop config rows while adding environment storage", func(t *testing.T) {
		t.Parallel()

		// Invariant: migration 00068 preserves prior loop config rows, initializes the
		// new environment column empty, and persists it across reopen.
		// Owning layer: GlobalDB migration stream. Canonical suite: global_db_test.go.
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefixDB, err := openGlobalMigrationPrefixDatabase(
			t,
			path,
			globalMigrationPrefixBefore(t, "00068_schema.sql"),
		)
		if err != nil {
			t.Fatalf("open prior global migration prefix error = %v", err)
		}
		prefixClosed := false
		t.Cleanup(func() {
			if !prefixClosed {
				if closeErr := prefixDB.Close(); closeErr != nil {
					t.Errorf("Close(prior prefix cleanup) error = %v", closeErr)
				}
			}
		})
		ctx := globalMigrationTestContext(t)
		if _, err := prefixDB.ExecContext(ctx, `INSERT INTO loop_config (
			workspace_id, loop_name, human_gate_enabled, enabled_checks_json, iteration_cap
		) VALUES ('ws-loop-environment', 'delivery', 0, '{}', 7)`); err != nil {
			t.Fatalf("seed prior loop config error = %v", err)
		}
		if err := prefixDB.Close(); err != nil {
			t.Fatalf("Close(prior prefix) error = %v", err)
		}
		prefixClosed = true

		upgraded, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(upgrade) error = %v", err)
		}
		upgradedClosed := false
		t.Cleanup(func() {
			if !upgradedClosed {
				if closeErr := upgraded.Close(testutil.Context(t)); closeErr != nil {
					t.Errorf("Close(upgraded cleanup) error = %v", closeErr)
				}
			}
		})
		preserved, err := upgraded.GetLoopConfig(ctx, "ws-loop-environment", "delivery")
		if err != nil {
			t.Fatalf("GetLoopConfig(preserved) error = %v", err)
		}
		if preserved.IterationCap == nil || *preserved.IterationCap != 7 || preserved.Environment != nil {
			t.Fatalf("preserved loop config = %#v, want iteration cap 7 and no environment", preserved)
		}
		environment := dsl.EnvironmentSpec{Mode: dsl.EnvironmentPerRun}
		if err := upgraded.UpsertLoopConfig(ctx, "ws-loop-environment", "delivery", looppkg.LoopConfig{
			Environment: &environment,
		}); err != nil {
			t.Fatalf("UpsertLoopConfig(environment) error = %v", err)
		}
		if err := upgraded.Close(ctx); err != nil {
			t.Fatalf("Close(upgraded) error = %v", err)
		}
		upgradedClosed = true

		reopened, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopened) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := reopened.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("Close(reopened cleanup) error = %v", closeErr)
			}
		})
		stored, err := reopened.GetLoopConfig(ctx, "ws-loop-environment", "delivery")
		if err != nil {
			t.Fatalf("GetLoopConfig(reopened) error = %v", err)
		}
		if stored.Environment == nil || *stored.Environment != environment {
			t.Fatalf("Environment(after reopen) = %#v, want %#v", stored.Environment, environment)
		}
		status, err := store.Status(ctx, reopened.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(reopened) error = %v", err)
		}
		assertCompleteMigrationStream(t, status, MigrationStream())
	})

	t.Run("Should preserve rows and migration status across reopen", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		first, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(first) error = %v", err)
		}
		firstStatus, err := store.Status(ctx, first.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(first) error = %v", err)
		}
		root := t.TempDir()
		workspace := compozyworkspace.Workspace{
			ID:        "ws-reopen",
			RootDir:   root,
			Name:      "reopen",
			CreatedAt: time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC),
		}
		if err := first.InsertWorkspace(ctx, workspace); err != nil {
			t.Fatalf("InsertWorkspace() error = %v", err)
		}
		worktree := worktreepkg.Worktree{
			ID: "wt-reopen", WorkspaceID: workspace.ID, Name: "reopen-worktree",
			Branch: "feature/reopen", Path: filepath.Join(t.TempDir(), "reopen-worktree"),
			State: worktreepkg.StateReady, Origin: worktreepkg.OriginManual, SetupState: worktreepkg.SetupNone,
			ProfileID: store.DefaultProfileID,
			CreatedAt: workspace.CreatedAt, UpdatedAt: workspace.UpdatedAt,
		}
		if err := first.Worktrees.Insert(ctx, worktree); err != nil {
			t.Fatalf("Worktrees.Insert() error = %v", err)
		}
		if err := first.Close(ctx); err != nil {
			t.Fatalf("Close(first) error = %v", err)
		}

		second, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(second) error = %v", err)
		}
		t.Cleanup(func() {
			if err := second.Close(testutil.Context(t)); err != nil {
				t.Fatalf("Close(second) error = %v", err)
			}
		})
		secondStatus, err := store.Status(ctx, second.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(second) error = %v", err)
		}
		if secondStatus != firstStatus {
			t.Fatalf("Status(second) = %#v, want unchanged %#v", secondStatus, firstStatus)
		}
		loaded, err := second.GetWorkspace(ctx, workspace.ID)
		if err != nil {
			t.Fatalf("GetWorkspace(after reopen) error = %v", err)
		}
		if loaded.ID != workspace.ID || loaded.RootDir != workspace.RootDir || loaded.Name != workspace.Name {
			t.Fatalf("GetWorkspace(after reopen) = %#v, want %#v", loaded, workspace)
		}
		loadedWorktree, err := second.Worktrees.Get(ctx, workspace.ID, worktree.ID)
		if err != nil {
			t.Fatalf("Worktrees.Get(after reopen) error = %v", err)
		}
		if loadedWorktree.ID != worktree.ID || loadedWorktree.Path != worktree.Path ||
			loadedWorktree.State != worktreepkg.StateReady {
			t.Fatalf("Worktrees.Get(after reopen) = %#v, want %#v", loadedWorktree, worktree)
		}
	})

	t.Run("Should apply destructive cuts once and remove pre-lineage loop history", func(t *testing.T) {
		t.Parallel()

		ctx := globalMigrationTestContext(t)
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		createHistoricalGlobalSchemaFixture(ctx, t, path)

		first, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(first post-cut) error = %v", err)
		}
		firstClosed := false
		t.Cleanup(func() {
			if firstClosed {
				return
			}
			if err := first.Close(testutil.Context(t)); err != nil {
				t.Fatalf("Close(first post-cut cleanup) error = %v", err)
			}
		})
		assertPostCutHistoricalGlobalSchemaFixture(t, first)
		firstStatus, err := store.Status(ctx, first.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(first post-cut) error = %v", err)
		}
		assertCompleteMigrationStream(t, firstStatus, MigrationStream())
		if err := first.Close(ctx); err != nil {
			t.Fatalf("Close(first post-cut) error = %v", err)
		}
		firstClosed = true

		second, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(second post-cut) error = %v", err)
		}
		t.Cleanup(func() {
			if err := second.Close(testutil.Context(t)); err != nil {
				t.Fatalf("Close(second post-cut) error = %v", err)
			}
		})
		assertPostCutHistoricalGlobalSchemaFixture(t, second)
		secondStatus, err := store.Status(ctx, second.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(second post-cut) error = %v", err)
		}
		if secondStatus != firstStatus {
			t.Fatalf("Status(second post-cut) = %#v, want unchanged %#v", secondStatus, firstStatus)
		}
	})
}

func TestGlobalDBPhase0HomeWorkspaceMigration(t *testing.T) {
	t.Parallel()
	t.Run("Should re-home synthetic workspace data without cascade loss", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		homeDir := t.TempDir()
		prefixDB, err := openGlobalMigrationPrefixDatabase(
			t,
			path,
			globalMigrationPrefixBefore(t, "00085_schema.sql"),
		)
		if err != nil {
			t.Fatalf("open migration 00084 fixture error = %v", err)
		}
		prefixClosed := false
		t.Cleanup(func() {
			if prefixClosed {
				return
			}
			if closeErr := prefixDB.Close(); closeErr != nil {
				t.Errorf("Close(phase0 fixture cleanup) error = %v", closeErr)
			}
		})
		ctx := globalMigrationTestContext(t)
		seedPhase0HomeWorkspaceFixture(ctx, t, prefixDB, homeDir)
		if err := prepareOperatorHomeMigrationContext(ctx, prefixDB, homeDir); err != nil {
			t.Fatalf("prepare phase0 migration context error = %v", err)
		}
		if err := applyGlobalMigrationPrefix(
			t,
			prefixDB,
			globalMigrationPrefixBefore(t, "00086_schema.sql"),
		); err != nil {
			t.Fatalf("apply migration through 00085 error = %v", err)
		}
		assertPhase0HomeWorkspaceDisposition(ctx, t, prefixDB)
		assertPhase0RebuiltGuards(ctx, t, prefixDB)
		if err := prefixDB.Close(); err != nil {
			t.Fatalf("Close(phase0 fixture) error = %v", err)
		}
		prefixClosed = true

		reopened, err := sql.Open(sqliteDriverName, path)
		if err != nil {
			t.Fatalf("sql.Open(phase0 reopen) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := reopened.Close(); closeErr != nil {
				t.Errorf("Close(phase0 reopen) error = %v", closeErr)
			}
		})
		assertPhase0HomeWorkspaceDisposition(ctx, t, reopened)
	})

	t.Run("Should abort atomically when the home workspace owns a worktree", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		homeDir := t.TempDir()
		prefixDB, err := openGlobalMigrationPrefixDatabase(
			t,
			path,
			globalMigrationPrefixBefore(t, "00085_schema.sql"),
		)
		if err != nil {
			t.Fatalf("open migration 00084 worktree fixture error = %v", err)
		}
		ctx := globalMigrationTestContext(t)
		if _, err := prefixDB.ExecContext(ctx, `INSERT INTO workspaces (
			id, root_dir, add_dirs, name, created_at, updated_at
		) VALUES ('home-workspace', ?, '[]', 'home', ?, ?)`, homeDir, phase0FixtureTime, phase0FixtureTime); err != nil {
			t.Fatalf("seed home workspace error = %v", err)
		}
		if _, err := prefixDB.ExecContext(ctx, `INSERT INTO worktrees (
			id, workspace_id, name, branch, path, state, origin, setup_state, created_at, updated_at
		) VALUES (
			'home-worktree', 'home-workspace', 'home', 'main', ?, 'ready', 'manual', 'none', ?, ?
		)`, filepath.Join(homeDir, "worktree"), phase0FixtureTime, phase0FixtureTime); err != nil {
			t.Fatalf("seed home worktree error = %v", err)
		}
		if err := prefixDB.Close(); err != nil {
			t.Fatalf("Close(worktree fixture) error = %v", err)
		}

		_, err = openGlobalMigrationUpgradeWithOptions(t, path, WithOperatorHomeDir(homeDir))
		if err == nil || !strings.Contains(err.Error(), "worktrees") {
			t.Fatalf("OpenGlobalDB(worktree fixture) error = %v, want worktrees assertion", err)
		}

		inspection, err := sql.Open(sqliteDriverName, path)
		if err != nil {
			t.Fatalf("sql.Open(worktree inspection) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := inspection.Close(); closeErr != nil {
				t.Errorf("Close(worktree inspection) error = %v", closeErr)
			}
		})
		assertSQLInt64(ctx, t, inspection, `SELECT COUNT(*) FROM workspaces WHERE id = 'home-workspace'`, 1)
		assertSQLInt64(ctx, t, inspection, `SELECT COUNT(*) FROM worktrees WHERE id = 'home-worktree'`, 1)
		status, err := store.Status(ctx, inspection, MigrationStream())
		if err != nil {
			t.Fatalf("Status(worktree assertion) error = %v", err)
		}
		if status.Version != 84 {
			t.Fatalf("Status(worktree assertion).Version = %d, want 84", status.Version)
		}
	})
}

const phase0FixtureTime = "2026-08-21T12:00:00Z"

func seedPhase0HomeWorkspaceFixture(ctx context.Context, t *testing.T, db *sql.DB, homeDir string) {
	t.Helper()

	statements := []struct {
		name  string
		query string
		args  []any
	}{
		{name: "workspace", query: `INSERT INTO workspaces (
			id, root_dir, add_dirs, name, created_at, updated_at
		) VALUES ('home-workspace', ?, '[]', 'home', ?, ?)`, args: []any{homeDir, phase0FixtureTime, phase0FixtureTime}},
		{name: "session", query: `INSERT INTO sessions (
			id, agent_name, workspace_id, state, created_at, updated_at
		) VALUES ('home-session', 'default', 'home-workspace', 'idle', ?, ?)`, args: []any{phase0FixtureTime, phase0FixtureTime}},
		{name: "session health", query: `INSERT INTO session_health (
			session_id, workspace_id, agent_name, state, health, active_prompt,
			attachable, eligible_for_wake, updated_at
		) VALUES ('home-session', 'home-workspace', 'default', 'idle', 'healthy', 0, 1, 1, ?)`, args: []any{phase0FixtureTime}},
		{name: "session prompt admission", query: `INSERT INTO session_prompt_admissions (
			id, workspace_id, session_id, message_id, idempotency_key, operation,
			fingerprint_version, request_fingerprint, state, turn_id, event_id, created_at, updated_at
		) VALUES (
			'home-admission', 'home-workspace', 'home-session', 'message-1', 'key-1', 'prompt',
			'v1', 'digest-1', 'reserved', 'turn-1', 'event-1', ?, ?
		)`, args: []any{phase0FixtureTime, phase0FixtureTime}},
		{name: "task", query: `INSERT INTO tasks (
			id, scope, workspace_id, title, status, created_by_kind, created_by_ref,
			origin_kind, origin_ref, created_at, updated_at
		) VALUES (
			'home-task', 'workspace', 'home-workspace', 'Home task', 'ready',
			'daemon', 'phase0', 'daemon', 'phase0', ?, ?
		)`, args: []any{phase0FixtureTime, phase0FixtureTime}},
		{name: "task run", query: `INSERT INTO task_runs (
			id, task_id, workspace_id, status, attempt, origin_kind, origin_ref, queued_at
		) VALUES ('home-run', 'home-task', 'home-workspace', 'queued', 1, 'daemon', 'phase0', ?)`, args: []any{phase0FixtureTime}},
		{name: "task block", query: `INSERT INTO task_blocks (
			id, workspace_id, task_id, kind, reason, created_by_kind, created_by_ref, created_at
		) VALUES ('home-block', 'home-workspace', 'home-task', 'needs_input', 'input', 'daemon', 'phase0', ?)`, args: []any{phase0FixtureTime}},
		{name: "automation job", query: `INSERT INTO automation_jobs (
			id, scope, name, agent_name, workspace_id, prompt, enabled, retry, fire_limit, created_at, updated_at
		) VALUES ('home-job', 'workspace', 'home-job', 'default', 'home-workspace', 'run', 1, '{}', '{}', ?, ?)`, args: []any{phase0FixtureTime, phase0FixtureTime}},
		{name: "automation trigger", query: `INSERT INTO automation_triggers (
			id, scope, name, agent_name, workspace_id, prompt, event, enabled, retry, fire_limit, created_at, updated_at
		) VALUES ('home-trigger', 'workspace', 'home-trigger', 'default', 'home-workspace', 'run', 'push', 1, '{}', '{}', ?, ?)`, args: []any{phase0FixtureTime, phase0FixtureTime}},
		{name: "automation suggestion", query: `INSERT INTO automation_suggestions (
			id, workspace_id, source, dedup_key, status, payload, created_at
		) VALUES ('home-suggestion', 'home-workspace', 'catalog', 'daily', 'pending', '{}', ?)`, args: []any{phase0FixtureTime}},
		{
			name: "automation run",
			query: `INSERT INTO automation_runs (id, job_id, session_id, task_id, task_run_id, status, attempt)
		VALUES ('home-automation-run', 'home-job', 'home-session', 'home-task', 'home-run', 'completed', 1)`,
		},
		{name: "bridge instance", query: `INSERT INTO bridge_instances (
			id, scope, workspace_id, platform, extension_name, display_name, status,
			routing_policy, created_at, updated_at
		) VALUES (
			'home-bridge', 'workspace', 'home-workspace', 'test', 'test.extension',
			'Home bridge', 'active', '{}', ?, ?
		)`, args: []any{phase0FixtureTime, phase0FixtureTime}},
		{name: "bridge route", query: `INSERT INTO bridge_routes (
			routing_key_hash, scope, workspace_id, bridge_instance_id, session_id,
			agent_name, last_activity_at, created_at, updated_at
		) VALUES (
			'home-route', 'workspace', 'home-workspace', 'home-bridge', 'home-session',
			'default', ?, ?, ?
		)`, args: []any{phase0FixtureTime, phase0FixtureTime, phase0FixtureTime}},
		{name: "loop config", query: `INSERT INTO loop_config (
			workspace_id, loop_name, human_gate_enabled, enabled_checks_json, iteration_cap
		) VALUES ('home-workspace', 'home-loop', 0, '{}', 3)`},
		{name: "agent soul snapshot", query: `INSERT INTO agent_soul_snapshots (
			id, workspace_id, agent_name, source_path, digest, created_at
		) VALUES ('home-soul-snapshot', 'home-workspace', 'default', 'SOUL.md', 'soul-digest', ?)`, args: []any{phase0FixtureTime}},
		{name: "agent soul revision", query: `INSERT INTO agent_soul_revisions (
			id, workspace_id, agent_name, source_path, action, created_at
		) VALUES ('home-soul-revision', 'home-workspace', 'default', 'SOUL.md', 'put', ?)`, args: []any{phase0FixtureTime}},
		{name: "agent heartbeat snapshot", query: `INSERT INTO agent_heartbeat_snapshots (
			id, workspace_id, agent_name, source_path, digest, config_digest, body,
			frontmatter_json, resolved_json, diagnostics_json, created_at
		) VALUES (
			'home-heartbeat-snapshot', 'home-workspace', 'default', 'HEARTBEAT.md',
			'heartbeat-digest', 'config-digest', '', '{}', '{}', '[]', ?
		)`, args: []any{phase0FixtureTime}},
		{name: "agent heartbeat revision", query: `INSERT INTO agent_heartbeat_revisions (
			id, workspace_id, agent_name, source_path, operation, new_snapshot_id,
			actor_kind, actor_id, created_at
		) VALUES (
			'home-heartbeat-revision', 'home-workspace', 'default', 'HEARTBEAT.md',
			'write', 'home-heartbeat-snapshot', 'system', 'phase0', ?
		)`, args: []any{phase0FixtureTime}},
		{name: "agent heartbeat wake event", query: `INSERT INTO agent_heartbeat_wake_events (
			id, workspace_id, agent_name, session_id, policy_snapshot_id, source,
			result, reason, created_at, expires_at
		) VALUES (
			'home-heartbeat-event', 'home-workspace', 'default', 'home-session',
			'home-heartbeat-snapshot', 'manual', 'sent', 'wake_sent', ?, ?
		)`, args: []any{phase0FixtureTime, "2026-08-22T12:00:00Z"}},
		{name: "agent heartbeat wake state", query: `INSERT INTO agent_heartbeat_wake_state (
			workspace_id, agent_name, session_id, policy_snapshot_id, last_result, updated_at
		) VALUES (
			'home-workspace', 'default', 'home-session', 'home-heartbeat-snapshot', 'sent', ?
		)`, args: []any{phase0FixtureTime}},
		{name: "tool approval grant", query: `INSERT INTO tool_approval_grants (
			id, workspace_id, agent_name, tool_id, decision, created_at, last_used_at
		) VALUES ('home-grant', 'home-workspace', 'default', 'test_tool', 'allow', ?, ?)`, args: []any{phase0FixtureTime, phase0FixtureTime}},
		{name: "dead entity", query: `INSERT INTO dead_entities (workspace_id, kind, entity_id, reason, marked_at)
		VALUES ('home-workspace', 'extension', 'dead-extension', 'removed', ?)`, args: []any{phase0FixtureTime}},
		{name: "pending tool approval", query: `INSERT INTO tool_approval_pending (
			approval_id, workspace_id, invocation_id, target_kind, tool_id, args_json,
			approval_status, requested_at, expires_at
		) VALUES (
			'apr_home', 'home-workspace', 'invocation-home', 'tool', 'test_tool', '{}',
			'pending', 1, 2
		)`},
		{name: "pre-existing aggregate command palette usage", query: `INSERT INTO cmd_palette_usage (
			workspace_id, command_id, use_count, frecency_weight, last_used_at, updated_at
		) VALUES ('', 'home.command', 5, 1.5, 20, 9)`},
		{name: "command palette usage", query: `INSERT INTO cmd_palette_usage (
			workspace_id, command_id, use_count, frecency_weight, last_used_at, updated_at
		) VALUES ('home-workspace', 'home.command', 3, 2.5, 10, 10)`},
		{name: "pre-existing aggregate command palette query hit", query: `INSERT INTO cmd_palette_query_hits (
			workspace_id, query, command_id, weight, last_used_at
		) VALUES ('', 'home', 'home.command', 1.5, 20)`},
		{name: "command palette query hit", query: `INSERT INTO cmd_palette_query_hits (
			workspace_id, query, command_id, weight, last_used_at
		) VALUES ('home-workspace', 'home', 'home.command', 4.5, 10)`},
		{name: "pre-existing aggregate command palette pin", query: `INSERT INTO cmd_palette_pins (
			workspace_id, command_id, pinned_at
		) VALUES ('', 'home.command', 5)`},
		{name: "command palette pin", query: `INSERT INTO cmd_palette_pins (workspace_id, command_id, pinned_at)
		VALUES ('home-workspace', 'home.command', 10)`},
		{name: "event summary", query: `INSERT INTO event_summaries (id, workspace_id, type, timestamp)
		VALUES ('home-summary', 'home-workspace', 'phase0', ?)`, args: []any{phase0FixtureTime}},
		{name: "pre-existing aggregate daily token usage", query: `INSERT INTO token_usage_daily (
			day, workspace_id, agent_name, input_tokens, output_tokens, total_tokens,
			total_cost, cost_currency, cost_status, cost_source, turn_count, updated_at
		) VALUES (
			'2026-08-21', '', 'default', 7, 11, 18,
			1.25, 'USD', 'estimated', 'catalog_config', 2, '2026-08-20T12:00:00Z'
		)`},
		{name: "daily token usage", query: `INSERT INTO token_usage_daily (
			day, workspace_id, agent_name, input_tokens, output_tokens, total_tokens, turn_count, updated_at
		) VALUES ('2026-08-21', 'home-workspace', 'default', 2, 3, 5, 1, ?)`, args: []any{phase0FixtureTime}},
		{name: "network channel", query: `INSERT INTO network_channels (
			workspace_id, channel, purpose, created_at, updated_at
		) VALUES ('home-workspace', 'general', 'coordination', ?, ?)`, args: []any{phase0FixtureTime, phase0FixtureTime}},
		{name: "network channel stats", query: `INSERT INTO network_channel_stats (workspace_id, channel)
		VALUES ('home-workspace', 'general')`},
		{
			name: "network channel kind count",
			query: `INSERT INTO network_channel_kind_counts (workspace_id, channel, kind, message_count)
		VALUES ('home-workspace', 'general', 'say', 2)`,
		},
		{name: "network audit log", query: `INSERT INTO network_audit_log (
			id, session_id, workspace_id, direction, kind, channel, peer_from,
			message_id, size, timestamp
		) VALUES (
			'home-audit', 'home-session', 'home-workspace', 'outbound', 'say', 'general',
			'peer-a', 'message-home', 5, ?
		)`, args: []any{phase0FixtureTime}},
		{name: "workspace network coordination", query: `INSERT INTO workspace_network_coordination (
			workspace_id, enabled, revision, updated_at, updated_by
		) VALUES ('home-workspace', 1, 1, ?, 'phase0')`, args: []any{phase0FixtureTime}},
		{name: "task network coordination", query: `INSERT INTO task_network_coordination (
			task_id, workspace_id, enabled, revision, updated_at, updated_by
		) VALUES ('home-task', 'home-workspace', 1, 1, ?, 'phase0')`, args: []any{phase0FixtureTime}},
		{name: "pre-existing aggregate notification cursor", query: `INSERT INTO notification_cursors (
			scope_kind, workspace_id, consumer_id, stream_name, subject_id,
			last_sequence, last_delivery_id, last_delivered_at, last_error, updated_at
		) VALUES (
			'global', '', 'consumer', 'events', '', 11, 'aggregate-delivery',
			'2026-08-22T12:00:00Z', 'aggregate-error', '2026-08-22T12:00:00Z'
		)`},
		{name: "notification cursor", query: `INSERT INTO notification_cursors (
			scope_kind, workspace_id, consumer_id, stream_name, last_sequence, updated_at
		) VALUES ('workspace', 'home-workspace', 'consumer', 'events', 7, ?)`, args: []any{phase0FixtureTime}},
		{name: "extension environment binding", query: `INSERT INTO extension_env_bindings (
			extension_name, workspace_id, env_name, secret_ref, kind, created_at, updated_at
		) VALUES ('test.extension', 'home-workspace', 'TOKEN', 'vault:extensions/test/token', 'extension_env', ?, ?)`, args: []any{phase0FixtureTime, phase0FixtureTime}},
		{name: "extension development link", query: `INSERT INTO extension_dev_links (
			extension_name, workspace_id, origin_path, bundle_generation, linked_at
		) VALUES ('test.extension', 'home-workspace', '/tmp/test-extension', '1', ?)`, args: []any{phase0FixtureTime}},
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement.query, statement.args...); err != nil {
			t.Fatalf("seed phase0 fixture %s error = %v", statement.name, err)
		}
	}
}

func assertPhase0HomeWorkspaceDisposition(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()

	stringCases := []struct {
		name  string
		query string
		want  string
	}{
		{
			name:  "session scope",
			query: `SELECT scope || ':' || workspace_id FROM sessions WHERE id = 'home-session'`,
			want:  "global:",
		},
		{
			name:  "prompt admission",
			query: `SELECT workspace_id FROM session_prompt_admissions WHERE id = 'home-admission'`,
			want:  "",
		},
		{
			name:  "session health",
			query: `SELECT workspace_id FROM session_health WHERE session_id = 'home-session'`,
			want:  "",
		},
		{
			name:  "task scope",
			query: `SELECT scope || ':' || coalesce(workspace_id, '') FROM tasks WHERE id = 'home-task'`,
			want:  "global:",
		},
		{name: "task run", query: `SELECT coalesce(workspace_id, '') FROM task_runs WHERE id = 'home-run'`, want: ""},
		{
			name:  "automation job",
			query: `SELECT scope || ':' || coalesce(workspace_id, '') FROM automation_jobs WHERE id = 'home-job'`,
			want:  "global:",
		},
		{
			name:  "automation trigger",
			query: `SELECT scope || ':' || coalesce(workspace_id, '') FROM automation_triggers WHERE id = 'home-trigger'`,
			want:  "global:",
		},
		{
			name:  "automation suggestion",
			query: `SELECT coalesce(workspace_id, '') FROM automation_suggestions WHERE id = 'home-suggestion'`,
			want:  "",
		},
		{
			name:  "bridge instance",
			query: `SELECT scope || ':' || coalesce(workspace_id, '') FROM bridge_instances WHERE id = 'home-bridge'`,
			want:  "global:",
		},
		{name: "loop config", query: `SELECT workspace_id FROM loop_config WHERE loop_name = 'home-loop'`, want: ""},
		{
			name:  "soul snapshot",
			query: `SELECT workspace_id FROM agent_soul_snapshots WHERE id = 'home-soul-snapshot'`,
			want:  "",
		},
		{
			name:  "heartbeat snapshot",
			query: `SELECT workspace_id FROM agent_heartbeat_snapshots WHERE id = 'home-heartbeat-snapshot'`,
			want:  "",
		},
		{name: "tool grant", query: `SELECT workspace_id FROM tool_approval_grants WHERE id = 'home-grant'`, want: ""},
		{
			name:  "dead entity",
			query: `SELECT workspace_id FROM dead_entities WHERE entity_id = 'dead-extension'`,
			want:  "",
		},
		{
			name:  "pending approval",
			query: `SELECT coalesce(workspace_id, '') FROM tool_approval_pending WHERE approval_id = 'apr_home'`,
			want:  "",
		},
		{
			name:  "palette usage",
			query: `SELECT workspace_id FROM cmd_palette_usage WHERE command_id = 'home.command'`,
			want:  "",
		},
		{name: "event summary", query: `SELECT workspace_id FROM event_summaries WHERE id = 'home-summary'`, want: ""},
		{name: "token usage", query: `SELECT workspace_id FROM token_usage_daily WHERE day = '2026-08-21'`, want: ""},
		{
			name:  "network channel",
			query: `SELECT workspace_id FROM network_channels WHERE channel = 'general'`,
			want:  "",
		},
		{name: "network audit", query: `SELECT workspace_id FROM network_audit_log WHERE id = 'home-audit'`, want: ""},
		{
			name:  "task coordination",
			query: `SELECT workspace_id FROM task_network_coordination WHERE task_id = 'home-task'`,
			want:  "",
		},
		{
			name:  "notification cursor",
			query: `SELECT scope_kind || ':' || workspace_id FROM notification_cursors WHERE consumer_id = 'consumer'`,
			want:  "global:",
		},
	}
	for _, testCase := range stringCases {
		var got string
		if err := db.QueryRowContext(ctx, testCase.query).Scan(&got); err != nil {
			t.Fatalf("query %s error = %v", testCase.name, err)
		}
		if got != testCase.want {
			t.Fatalf("%s = %q, want %q", testCase.name, got, testCase.want)
		}
	}

	var usageCount, usageLastUsedAt, usageUpdatedAt int64
	var usageWeight float64
	if err := db.QueryRowContext(ctx, `SELECT use_count, frecency_weight, last_used_at, updated_at
		FROM cmd_palette_usage WHERE workspace_id = '' AND command_id = 'home.command'`).Scan(
		&usageCount, &usageWeight, &usageLastUsedAt, &usageUpdatedAt,
	); err != nil {
		t.Fatalf("query merged command palette usage error = %v", err)
	}
	if usageCount != 8 || usageWeight != 4 || usageLastUsedAt != 20 || usageUpdatedAt != 10 {
		t.Fatalf("merged command palette usage = (%d, %v, %d, %d), want (8, 4, 20, 10)",
			usageCount, usageWeight, usageLastUsedAt, usageUpdatedAt)
	}

	var queryWeight float64
	var queryLastUsedAt int64
	if err := db.QueryRowContext(ctx, `SELECT weight, last_used_at FROM cmd_palette_query_hits
		WHERE workspace_id = '' AND query = 'home' AND command_id = 'home.command'`).Scan(
		&queryWeight, &queryLastUsedAt,
	); err != nil {
		t.Fatalf("query merged command palette hit error = %v", err)
	}
	if queryWeight != 6 || queryLastUsedAt != 20 {
		t.Fatalf("merged command palette hit = (%v, %d), want (6, 20)", queryWeight, queryLastUsedAt)
	}
	assertSQLInt64(ctx, t, db, `SELECT pinned_at FROM cmd_palette_pins
		WHERE workspace_id = '' AND command_id = 'home.command'`, 5)

	var inputTokens, outputTokens, totalTokens, turnCount int64
	var totalCost float64
	var currency, costStatus, costSource, usageTimestamp string
	if err := db.QueryRowContext(ctx, `SELECT input_tokens, output_tokens, total_tokens, total_cost,
		cost_currency, cost_status, cost_source, turn_count, updated_at
		FROM token_usage_daily WHERE day = '2026-08-21' AND workspace_id = '' AND agent_name = 'default'`).Scan(
		&inputTokens, &outputTokens, &totalTokens, &totalCost, &currency, &costStatus, &costSource,
		&turnCount, &usageTimestamp,
	); err != nil {
		t.Fatalf("query merged token usage error = %v", err)
	}
	if inputTokens != 9 || outputTokens != 14 || totalTokens != 23 || totalCost != 1.25 ||
		currency != "USD" || costStatus != "unknown" || costSource != "none" || turnCount != 3 ||
		usageTimestamp != phase0FixtureTime {
		t.Fatalf(
			"merged token usage = (%d, %d, %d, %v, %q, %q, %q, %d, %q), want preserved aggregate",
			inputTokens,
			outputTokens,
			totalTokens,
			totalCost,
			currency,
			costStatus,
			costSource,
			turnCount,
			usageTimestamp,
		)
	}

	var cursorSequence int64
	var deliveryID, deliveredAt, lastError, cursorUpdatedAt string
	if err := db.QueryRowContext(ctx, `SELECT last_sequence, last_delivery_id, last_delivered_at, last_error, updated_at
		FROM notification_cursors
		WHERE scope_kind = 'global' AND workspace_id = '' AND consumer_id = 'consumer'
		AND stream_name = 'events' AND subject_id = ''`).Scan(
		&cursorSequence, &deliveryID, &deliveredAt, &lastError, &cursorUpdatedAt,
	); err != nil {
		t.Fatalf("query merged notification cursor error = %v", err)
	}
	if cursorSequence != 11 || deliveryID != "aggregate-delivery" || deliveredAt != "2026-08-22T12:00:00Z" ||
		lastError != "aggregate-error" || cursorUpdatedAt != "2026-08-22T12:00:00Z" {
		t.Fatalf("merged notification cursor = (%d, %q, %q, %q, %q), want latest aggregate cursor",
			cursorSequence, deliveryID, deliveredAt, lastError, cursorUpdatedAt)
	}

	assertSQLInt64(ctx, t, db, `SELECT COUNT(*) FROM workspaces WHERE id = 'home-workspace'`, 0)
	assertSQLInt64(
		ctx,
		t,
		db,
		`SELECT COUNT(*) FROM workspace_network_coordination WHERE workspace_id = 'home-workspace'`,
		0,
	)
	assertSQLInt64(ctx, t, db, `SELECT COUNT(*) FROM extension_env_bindings WHERE workspace_id = 'home-workspace'`, 0)
	assertSQLInt64(ctx, t, db, `SELECT COUNT(*) FROM extension_dev_links WHERE workspace_id = 'home-workspace'`, 0)
	assertNoForeignKeyViolations(ctx, t, db)
}

func assertPhase0RebuiltGuards(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()

	if _, err := db.ExecContext(ctx, `INSERT INTO sessions (
		id, agent_name, scope, workspace_id, state, created_at, updated_at
	) VALUES ('phase0-empty-workspace-session', 'default', 'global', '', 'idle', ?, ?)`,
		phase0FixtureTime, phase0FixtureTime,
	); err != nil {
		t.Fatalf("insert phase0 empty-workspace sentinel session error = %v", err)
	}

	for _, testCase := range []struct {
		name  string
		query string
		args  []any
		want  string
	}{
		{
			name: "session insert workspace guard",
			query: `INSERT INTO sessions (
				id, agent_name, scope, workspace_id, state, created_at, updated_at
			) VALUES ('phase0-missing-workspace-session', 'default', 'workspace', 'missing-workspace', 'idle', ?, ?)`,
			args: []any{phase0FixtureTime, phase0FixtureTime}, want: "workspace not found",
		},
		{
			name: "session update workspace guard",
			query: `UPDATE sessions SET workspace_id = 'missing-workspace'
				WHERE id = 'phase0-empty-workspace-session'`,
			want: "workspace not found",
		},
		{
			name: "task coordination insert workspace guard",
			query: `INSERT INTO task_network_coordination (
				task_id, workspace_id, enabled, revision, updated_at, updated_by
			) VALUES ('phase0-missing-task', 'missing-workspace', 1, 1, ?, 'phase0')`,
			args: []any{phase0FixtureTime}, want: "workspace not found",
		},
		{
			name:  "task coordination update workspace guard",
			query: `UPDATE task_network_coordination SET workspace_id = 'missing-workspace' WHERE task_id = 'home-task'`,
			want:  "workspace not found",
		},
		{
			name: "approval grant insert workspace guard",
			query: `INSERT INTO tool_approval_grants (
				id, workspace_id, agent_name, tool_id, decision, created_at, last_used_at
			) VALUES ('phase0-missing-grant', 'missing-workspace', 'default', 'test_tool', 'allow', ?, ?)`,
			args: []any{phase0FixtureTime, phase0FixtureTime}, want: "workspace not found",
		},
		{
			name:  "approval grant update workspace guard",
			query: `UPDATE tool_approval_grants SET workspace_id = 'missing-workspace' WHERE id = 'home-grant'`,
			want:  "workspace not found",
		},
		{
			name: "session archive insert guard",
			query: `INSERT INTO sessions (
				id, agent_name, scope, workspace_id, state, archived_at, created_at, updated_at
			) VALUES ('phase0-archived-session', 'default', 'global', '', 'idle', ?, ?, ?)`,
			args: []any{phase0FixtureTime, phase0FixtureTime, phase0FixtureTime}, want: "session is archived",
		},
		{
			name:  "session archive update guard",
			query: `UPDATE sessions SET archived_at = ? WHERE id = 'phase0-empty-workspace-session'`,
			args:  []any{phase0FixtureTime}, want: "session is archived",
		},
	} {
		if _, err := db.ExecContext(ctx, testCase.query, testCase.args...); err == nil ||
			!strings.Contains(err.Error(), testCase.want) {
			t.Fatalf("%s error = %v, want %q", testCase.name, err, testCase.want)
		}
	}

	if _, err := db.ExecContext(ctx, `INSERT INTO task_run_terminal_commands (
		command_id, run_id, task_id, workspace_id, kind, phase, source_status,
		source_session_id, source_claim_token_hash, intent_json, actor_json,
		command_at, admitted_at, updated_at
	) VALUES (
		'phase0-terminal-command', 'home-run', 'home-task', '', 'needs_attention', 'admitted', 'queued',
		'', '', '{}', '{}', ?, ?, ?
	)`, phase0FixtureTime, phase0FixtureTime, phase0FixtureTime); err != nil {
		t.Fatalf("insert phase0 terminal command error = %v", err)
	}
	for _, testCase := range []struct {
		name  string
		query string
	}{
		{name: "terminal command update guard", query: `UPDATE task_runs SET status = 'claimed' WHERE id = 'home-run'`},
		{name: "terminal command delete guard", query: `DELETE FROM task_runs WHERE id = 'home-run'`},
	} {
		if _, err := db.ExecContext(ctx, testCase.query); err == nil ||
			!strings.Contains(err.Error(), terminalRunCommandGuardMessage) {
			t.Fatalf("%s error = %v, want %q", testCase.name, err, terminalRunCommandGuardMessage)
		}
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM task_run_terminal_commands
		WHERE command_id = 'phase0-terminal-command'`); err != nil {
		t.Fatalf("delete phase0 terminal command fixture error = %v", err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM sessions
		WHERE id = 'phase0-empty-workspace-session'`); err != nil {
		t.Fatalf("delete phase0 empty-workspace sentinel fixture error = %v", err)
	}
}

func assertSQLInt64(ctx context.Context, t *testing.T, db *sql.DB, query string, want int64) {
	t.Helper()

	var got int64
	if err := db.QueryRowContext(ctx, query).Scan(&got); err != nil {
		t.Fatalf("query %q error = %v", query, err)
	}
	if got != want {
		t.Fatalf("query %q = %d, want %d", query, got, want)
	}
}

func assertNoForeignKeyViolations(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()

	rows, err := db.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatalf("query foreign_key_check error = %v", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			t.Errorf("close foreign_key_check rows error = %v", closeErr)
		}
	}()
	if !rows.Next() {
		return
	}
	var table, parent string
	var rowID sql.NullInt64
	var foreignKeyID int
	if err := rows.Scan(&table, &rowID, &parent, &foreignKeyID); err != nil {
		t.Fatalf("scan foreign_key_check result error = %v", err)
	}
	t.Fatalf("foreign_key_check violation: table=%q row_id=%v parent=%q foreign_key_id=%d",
		table, rowID, parent, foreignKeyID)
}

func TestGlobalDBTerminalRunCommandMigrationPreservesRowsAcrossReopen(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve task runs and terminal-command guards across migration reopen", func(t *testing.T) {
		t.Parallel()

		// Invariant: migration 00042 is append-only over the prior head, preserves
		// task-run rows, and keeps the terminal-command guard effective after reopen.
		// Owning layer: GlobalDB migration stream. Canonical suite: global_db_test.go.
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefixDB, err := openGlobalMigrationPrefixDatabase(
			t,
			path,
			globalMigrationPrefixBefore(t, "00042_schema.sql"),
		)
		if err != nil {
			t.Fatalf("open prior global migration prefix error = %v", err)
		}
		ctx := globalMigrationTestContext(t)
		statements := []string{
			`INSERT INTO workspaces (
			id, root_dir, add_dirs, name, created_at, updated_at
		) VALUES (
			'workspace-terminal-migration', '/tmp/workspace-terminal-migration', '[]',
			'workspace-terminal-migration', '2026-08-02T11:59:00Z', '2026-08-02T11:59:00Z'
		)`,
			`INSERT INTO tasks (
			id, scope, workspace_id, title, status, created_by_kind, created_by_ref,
			origin_kind, origin_ref, created_at, updated_at
		) VALUES (
			'task-terminal-migration', 'workspace', 'workspace-terminal-migration',
			'Terminal migration', 'ready',
			'daemon', 'migration', 'daemon', 'migration',
			'2026-08-02T12:00:00Z', '2026-08-02T12:00:00Z'
		)`,
			`INSERT INTO task_runs (
			id, task_id, workspace_id, status, attempt, origin_kind, origin_ref, queued_at
		) VALUES (
			'run-terminal-migration', 'task-terminal-migration', 'workspace-terminal-migration',
			'queued', 1,
			'daemon', 'migration', '2026-08-02T12:01:00Z'
		)`,
		}
		for _, statement := range statements {
			if _, err := prefixDB.ExecContext(ctx, statement); err != nil {
				t.Fatalf("seed prior terminal migration fixture error = %v", err)
			}
		}
		if err := prefixDB.Close(); err != nil {
			t.Fatalf("Close(prior prefix) error = %v", err)
		}

		first, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(first migration 00042) error = %v", err)
		}
		ctx = globalMigrationTestContext(t)
		commandAt := "2026-08-02T12:02:00Z"
		if _, err := first.db.ExecContext(ctx, `INSERT INTO task_run_terminal_commands (
		command_id, run_id, task_id, workspace_id, kind, phase,
		source_status, source_session_id, source_claim_token_hash,
		intent_json, actor_json, command_at, admitted_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"terminal-command-migration",
			"run-terminal-migration",
			"task-terminal-migration",
			"workspace-terminal-migration",
			"needs_attention",
			"admitted",
			"queued",
			"",
			"",
			`{"version":2,"event_ids":["evt-terminal-migration"],`+
				`"diagnostic":"migration fixture","stop_required":false}`,
			`{"actor":{"kind":"daemon","ref":"migration"},"origin":{"kind":"daemon","ref":"migration"}}`,
			commandAt,
			commandAt,
			commandAt,
		); err != nil {
			t.Fatalf("insert migrated terminal command error = %v", err)
		}
		if err := first.Close(ctx); err != nil {
			t.Fatalf("Close(first migration 00042) error = %v", err)
		}

		second, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen migration 00042) error = %v", err)
		}
		ctx = globalMigrationTestContext(t)
		t.Cleanup(func() {
			if err := second.Close(testutil.Context(t)); err != nil {
				t.Errorf("Close(reopened migration 00042) error = %v", err)
			}
		})
		run, err := second.GetTaskRun(ctx, "run-terminal-migration")
		if err != nil {
			t.Fatalf("GetTaskRun(after migration reopen) error = %v", err)
		}
		if run.Status != taskpkg.TaskRunStatusQueued {
			t.Fatalf("migrated run status = %q, want queued", run.Status)
		}
		if run.WorkspaceID != "workspace-terminal-migration" {
			t.Fatalf("migrated run workspace = %q, want workspace-terminal-migration", run.WorkspaceID)
		}
		var commandCount int
		if err := second.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM task_run_terminal_commands WHERE run_id = ?`,
			run.ID,
		).Scan(&commandCount); err != nil {
			t.Fatalf("count terminal commands after reopen error = %v", err)
		}
		if commandCount != 1 {
			t.Fatalf("terminal command count after reopen = %d, want 1", commandCount)
		}
		if _, err := second.db.ExecContext(
			ctx,
			`UPDATE task_runs SET status = 'claimed' WHERE id = ?`,
			run.ID,
		); err == nil || !strings.Contains(err.Error(), terminalRunCommandGuardMessage) {
			t.Fatalf("guarded task-run update error = %v, want terminal command guard", err)
		}
		if _, err := second.db.ExecContext(
			ctx,
			`DELETE FROM task_runs WHERE id = ?`,
			run.ID,
		); err == nil || !strings.Contains(err.Error(), terminalRunCommandGuardMessage) {
			t.Fatalf("guarded task-run delete error = %v, want terminal command guard", err)
		}
		if _, err := second.db.ExecContext(
			ctx,
			`DELETE FROM tasks WHERE id = ?`,
			"task-terminal-migration",
		); err == nil || !strings.Contains(err.Error(), terminalRunCommandGuardMessage) {
			t.Fatalf("guarded task delete error = %v, want terminal command guard", err)
		}
		if err := second.DeleteWorkspace(ctx, "workspace-terminal-migration"); !errors.Is(
			err,
			taskpkg.ErrTerminalRunCommandInProgress,
		) {
			t.Fatalf(
				"DeleteWorkspace(guarded) error = %v, want %v",
				err,
				taskpkg.ErrTerminalRunCommandInProgress,
			)
		}
	})
}

func assertCompleteMigrationStream(t *testing.T, status store.StreamStatus, stream store.MigrationStream) {
	t.Helper()

	entries, err := fs.ReadDir(stream.FS, stream.Dir)
	if err != nil {
		t.Fatalf("read %s migration directory: %v", stream.Name, err)
	}
	wantVersion := int64(0)
	wantAppliedCount := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		separator := strings.IndexByte(entry.Name(), '_')
		if separator <= 0 {
			t.Fatalf("%s migration filename %q has no version prefix", stream.Name, entry.Name())
		}
		version, err := strconv.ParseInt(entry.Name()[:separator], 10, 64)
		if err != nil {
			t.Fatalf("parse %s migration version: %v", stream.Name, err)
		}
		wantVersion = max(wantVersion, version)
		wantAppliedCount++
	}
	if status.Version != wantVersion || status.AppliedCount != wantAppliedCount {
		t.Fatalf(
			"Status(%s) = %#v, want version %d with %d applied migrations",
			stream.Name,
			status,
			wantVersion,
			wantAppliedCount,
		)
	}
}

func createHistoricalGlobalSchemaFixture(ctx context.Context, t *testing.T, path string) {
	t.Helper()

	legacyDB, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		t.Fatalf("sql.Open(pre-cut fixture) error = %v", err)
	}
	legacyClosed := false
	t.Cleanup(func() {
		if legacyClosed {
			return
		}
		if err := legacyDB.Close(); err != nil {
			t.Fatalf("Close(pre-cut fixture cleanup) error = %v", err)
		}
	})
	migrationFS, err := fs.Sub(globalschema.Files, "migrations")
	if err != nil {
		t.Fatalf("fs.Sub(global migrations) error = %v", err)
	}
	provider, err := goose.NewProvider(
		goose.DialectSQLite3,
		legacyDB,
		migrationFS,
		goose.WithTableName(globalMigrationVersionTable),
		goose.WithDisableGlobalRegistry(true),
	)
	if err != nil {
		t.Fatalf("goose.NewProvider(pre-cut fixture) error = %v", err)
	}
	if _, err := provider.UpTo(ctx, 1); err != nil {
		t.Fatalf("provider.UpTo(pre-cut fixture, 1) error = %v", err)
	}

	statements := []struct {
		name string
		sql  string
	}{
		{
			name: "workspace",
			sql: `INSERT INTO workspaces (
				id, root_dir, name, created_at, updated_at
			) VALUES (
				'ws-precut', '/tmp/compozy-precut', 'precut',
				'2026-07-13T10:00:00Z', '2026-07-13T10:00:00Z'
			)`,
		},
		{
			name: "session",
			sql: `INSERT INTO sessions (
				id, agent_name, workspace_id, channel, state, created_at, updated_at
			) VALUES (
				'sess-precut', 'coder', 'ws-precut', 'coord-run-precut', 'stopped',
				'2026-07-13T10:01:00Z', '2026-07-13T10:02:00Z'
			)`,
		},
		{
			name: "task",
			sql: `INSERT INTO tasks (
				id, scope, workspace_id, network_channel, title, status,
				created_by_kind, created_by_ref, origin_kind, origin_ref, created_at, updated_at
			) VALUES (
				'task-precut', 'workspace', 'ws-precut', 'coord-run-precut', 'Pre-cut task', 'completed',
				'daemon', 'scheduler', 'daemon', 'scheduler',
				'2026-07-13T10:03:00Z', '2026-07-13T10:04:00Z'
			)`,
		},
		{
			name: "task run",
			sql: `INSERT INTO task_runs (
				id, task_id, status, attempt, origin_kind, origin_ref,
				network_channel, coordination_channel_id, loop_run_id, queued_at
			) VALUES (
				'run-precut', 'task-precut', 'completed', 1, 'daemon', 'scheduler',
				'coord-run-precut', 'coord-run-precut', 'loop-precut', '2026-07-13T10:05:00Z'
			)`,
		},
		{
			name: "queued loop worker",
			sql: `INSERT INTO task_runs (
				id, task_id, status, attempt, origin_kind, origin_ref, run_kind,
				loop_run_id, metadata_json, queued_at
			) VALUES (
				'run-precut-loop-worker', 'task-precut', 'queued', 1, 'daemon', 'loop', 'worker',
				'loop-precut', '{"generation":1,"node_id":"work","item_index":0}',
				'2026-07-13T10:05:00Z'
			)`,
		},
		{
			name: "queued loop coordinator",
			sql: `INSERT INTO task_runs (
				id, task_id, status, attempt, origin_kind, origin_ref, run_kind,
				loop_run_id, queued_at
			) VALUES (
				'run-precut-loop-coordinator', 'task-precut', 'queued', 1, 'daemon', 'loop', 'coordinator',
				'loop-precut', '2026-07-13T10:05:00Z'
			)`,
		},
		{
			name: "delegated loop automation run",
			sql: `INSERT INTO automation_runs (
				id, status, attempt, loop_run_id, metadata_json
			) VALUES (
				'automation-precut-loop', 'delegated', 1, 'loop-precut', '{}'
			)`,
		},
		{
			name: "loop run",
			sql: `INSERT INTO loop_runs (
				id, workspace_id, loop_name, status, last_progress_at, inputs_json
			) VALUES (
				'loop-precut', 'ws-precut', 'precut-loop', 'done',
				'2026-07-13T10:06:00Z', '{}'
			)`,
		},
		{
			name: "loop definition snapshot",
			sql: `INSERT INTO loop_definition_snapshots (
				workspace_id, definition_digest, definition_version, definition_json, byte_size, created_at, last_used_at
			) VALUES (
				'ws-precut', 'digest-precut', 1, '{"kind":"loop"}', 15,
				'2026-07-13T10:06:00Z', '2026-07-13T10:06:00Z'
			)`,
		},
		{
			name: "loop configuration",
			sql: `INSERT INTO loop_config (workspace_id, loop_name, enabled_checks_json)
				VALUES ('ws-precut', 'precut-loop', '{"input-default":true}')`,
		},
		{
			name: "loop output blob",
			sql: `INSERT INTO loop_output_blobs (output_ref, payload_json, byte_size, created_at, last_used_at)
				VALUES ('blob-precut', '{"result":"old"}', 16, '2026-07-13T10:06:00Z', '2026-07-13T10:06:00Z')`,
		},
		{
			name: "loop generation output",
			sql: `INSERT INTO loop_generation_outputs (
				loop_run_id, generation, node_id, item_index, status, output_ref
			) VALUES ('loop-precut', 1, 'node-precut', 0, 'succeeded', 'blob-precut')`,
		},
		{
			name: "loop gate decision",
			sql: `INSERT INTO loop_gate_decisions (
				workspace_id, loop_run_id, generation, gate_id, criterion_id, decision,
				actor_kind, actor_ref, origin_kind, origin_ref, decided_at
			) VALUES (
				'ws-precut', 'loop-precut', 1, 'gate-precut', 'criterion-precut', 'approve',
				'human', 'operator', 'cli', 'loop approve', '2026-07-13T10:06:00Z'
			)`,
		},
		{
			name: "loop run event",
			sql: `INSERT INTO loop_run_events (id, loop_run_id, workspace_id, seq, kind, payload_json, at)
				VALUES ('event-precut', 'loop-precut', 'ws-precut', 1, 'status_changed', '{}', '2026-07-13T10:06:00Z')`,
		},
		{
			name: "loop goal checkpoint",
			sql: `INSERT INTO loop_goal_checkpoints (
				loop_run_id, generation, node_id, phase, goal_status, turn_limit, context_nudge_ratio, updated_at
			) VALUES ('loop-precut', 1, 'node-precut', 'idle', 'active', 1, 0.8, '2026-07-13T10:06:00Z')`,
		},
		{
			name: "loop goal turn",
			sql: `INSERT INTO loop_goal_turns (
				loop_run_id, seq, generation, node_id, item_index, turn, session_id, binding_handle,
				binding_epoch, prompt_id, actor_kind, actor_id, started_at
			) VALUES (
				'loop-precut', 1, 1, 'node-precut', 0, 1, 'sess-precut', 'handle-precut',
				1, 'prompt-precut', 'daemon', 'loop', '2026-07-13T10:06:00Z'
			)`,
		},
		{
			name: "loop session binding",
			sql: `INSERT INTO loop_session_bindings (
				loop_run_id, handle, binding_epoch, binding_attempt_id, session_id, workspace_id,
				creation_profile_ref, policy_spec_digest, creation_digest, ownership, state, created_at, activated_at
			) VALUES (
				'loop-precut', 'handle-precut', 1, 'binding-attempt-precut', 'sess-precut', 'ws-precut',
				'profile-precut', 'policy-precut', 'creation-precut', 'run-owned', 'active',
				'2026-07-13T10:06:00Z', '2026-07-13T10:06:00Z'
			)`,
		},
		{
			name: "loop session cleanup",
			sql: `INSERT INTO loop_goal_session_cleanup (
				cleanup_id, workspace_id, loop_run_id, handle, binding_epoch, session_id, cause, created_at
			) VALUES (
				'cleanup-precut', 'ws-precut', 'loop-precut', 'handle-precut', 1, 'sess-precut', 'terminal',
				'2026-07-13T10:06:00Z'
			)`,
		},
		{
			name: "loop goal outbox",
			sql: `INSERT INTO loop_goal_session_outbox (
				event_id, workspace_id, origin_session_id, loop_run_id, cause, created_at
			) VALUES ('outbox-precut', 'ws-precut', 'sess-precut', 'loop-precut', 'start', '2026-07-13T10:06:00Z')`,
		},
		{
			name: "loop goal judge attempt",
			sql: `INSERT INTO loop_goal_judge_attempts (
				attempt_id, loop_run_id, generation, node_id, item_index, turn, judge_digest, status, started_at
			) VALUES (
				'judge-precut', 'loop-precut', 1, 'node-precut', 0, 1, 'judge-digest-precut', 'running',
				'2026-07-13T10:06:00Z'
			)`,
		},
		{
			name: "loop binding retry witness",
			sql: `INSERT INTO loop_goal_binding_retry_witnesses (
				loop_run_id, handle, failed_binding_epoch, request_digest, created_at
			) VALUES (
				'loop-precut', 'handle-precut', 1,
				'0000000000000000000000000000000000000000000000000000000000000000', '2026-07-13T10:06:00Z'
			)`,
		},
		{
			name: "loop-owned input queue entry",
			sql: `INSERT INTO session_input_queue (
				id, session_id, status, mode, text, enqueued_at, updated_at, loop_run_id
			) VALUES (
				'input-precut', 'sess-precut', 'queued', 'queue', 'obsolete loop input',
				'2026-07-13T10:06:00Z', '2026-07-13T10:06:00Z', 'loop-precut'
			)`,
		},
		{
			name: "auto-created coordination channel",
			sql: `INSERT INTO network_channels (
				workspace_id, channel, purpose, created_by, created_at, updated_at
			) VALUES (
				'ws-precut', 'coord-run-precut', 'task_run_coordination', 'scheduler',
				'2026-07-13T10:07:00Z', '2026-07-13T10:07:00Z'
			)`,
		},
		{
			name: "peer channel participant",
			sql: `INSERT INTO network_channel_participants (
				workspace_id, channel, peer_id
			) VALUES ('ws-precut', 'coord-run-precut', 'peer-a')`,
		},
		{
			name: "peer direct room",
			sql: `INSERT INTO network_direct_rooms (
				workspace_id, channel, direct_id, peer_a, peer_b, opened_at, last_activity_at
			) VALUES (
				'ws-precut', 'coord-run-precut', 'direct-precut', 'peer-a', 'peer-b',
				'2026-07-13T10:08:00Z', '2026-07-13T10:08:00Z'
			)`,
		},
		{
			name: "thread",
			sql: `INSERT INTO network_threads (
				workspace_id, channel, thread_id, root_message_id, opened_at, last_activity_at
			) VALUES (
				'ws-precut', 'coord-run-precut', 'thread-precut', 'msg-root',
				'2026-07-13T10:09:00Z', '2026-07-13T10:09:00Z'
			)`,
		},
		{
			name: "peer subscription",
			sql: `INSERT INTO network_subscriptions (
				workspace_id, channel, thread_id, peer_id, mode, created_at, updated_at
			) VALUES (
				'ws-precut', 'coord-run-precut', 'thread-precut', 'peer-a', 'full',
				'2026-07-13T10:10:00Z', '2026-07-13T10:10:00Z'
			)`,
		},
		{
			name: "peer thread participant",
			sql: `INSERT INTO network_thread_participants (
				workspace_id, channel, thread_id, peer_id, first_message_id, first_seen_at, last_seen_at
			) VALUES (
				'ws-precut', 'coord-run-precut', 'thread-precut', 'peer-a', 'msg-root',
				'2026-07-13T10:11:00Z', '2026-07-13T10:11:00Z'
			)`,
		},
		{
			name: "delivery guidance state",
			sql: `INSERT INTO network_delivery_guidance_state (
				session_id, reply_guidance_delivered, protocol_guidance_delivered, created_at, updated_at
			) VALUES (
				'sess-precut', 1, 1, '2026-07-13T10:12:00Z', '2026-07-13T10:12:00Z'
			)`,
		},
	}
	for _, statement := range statements {
		if _, err := legacyDB.ExecContext(ctx, statement.sql); err != nil {
			t.Fatalf("seed pre-cut %s error = %v", statement.name, err)
		}
	}
	if err := legacyDB.Close(); err != nil {
		t.Fatalf("Close(pre-cut fixture) error = %v", err)
	}
	legacyClosed = true
}

func assertPostCutHistoricalGlobalSchemaFixture(t *testing.T, globalDB *GlobalDB) {
	t.Helper()

	ctx := testutil.Context(t)
	for _, table := range []string{"sessions", "task_runs"} {
		assertOwnerTableCanonicalLocal(t, globalDB.db, table)
	}
	var loopRunCount int
	if err := globalDB.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM loop_runs`).Scan(&loopRunCount); err != nil {
		t.Fatalf("count loop_runs after lineage cut error = %v", err)
	}
	if loopRunCount != 0 {
		t.Fatalf("loop_runs row count after lineage cut = %d, want 0", loopRunCount)
	}
	for _, table := range []string{
		"loop_generation_outputs",
		"loop_gate_decisions",
		"loop_run_events",
		"loop_goal_session_outbox",
		"loop_session_cleanup",
		"loop_goal_judge_attempts",
		"loop_goal_turns",
		"loop_goal_checkpoints",
		"loop_goal_binding_retry_witnesses",
		"loop_session_bindings",
		"loop_output_blobs",
		"session_input_queue",
	} {
		var count int
		if err := globalDB.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table).Scan(&count); err != nil {
			t.Fatalf("count %s after lineage cut error = %v", table, err)
		}
		if count != 0 {
			t.Fatalf("%s row count after lineage cut = %d, want 0", table, count)
		}
	}
	var loopRunID sql.NullString
	if err := globalDB.db.QueryRowContext(
		ctx,
		`SELECT loop_run_id FROM task_runs WHERE id = 'run-precut'`,
	).Scan(&loopRunID); err != nil {
		t.Fatalf("read task run loop link after lineage cut error = %v", err)
	}
	if loopRunID.Valid {
		t.Fatalf("task run loop_run_id after lineage cut = %q, want NULL", loopRunID.String)
	}
	for _, orphanID := range []string{"run-precut-loop-worker", "run-precut-loop-coordinator"} {
		var count int
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM task_runs WHERE id = ?`,
			orphanID,
		).Scan(&count); err != nil {
			t.Fatalf("count obsolete loop task run %q error = %v", orphanID, err)
		}
		if count != 0 {
			t.Fatalf("obsolete loop task run %q count = %d, want 0", orphanID, count)
		}
	}
	var delegatedOrphanCount int
	if err := globalDB.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM automation_runs WHERE id = 'automation-precut-loop'`,
	).Scan(&delegatedOrphanCount); err != nil {
		t.Fatalf("count obsolete delegated loop automation run error = %v", err)
	}
	if delegatedOrphanCount != 0 {
		t.Fatalf("obsolete delegated loop automation run count = %d, want 0", delegatedOrphanCount)
	}
	var definitionJSON string
	if err := globalDB.db.QueryRowContext(
		ctx,
		`SELECT definition_json FROM loop_definition_snapshots
		 WHERE workspace_id = 'ws-precut' AND definition_digest = 'digest-precut'`,
	).Scan(&definitionJSON); err != nil {
		t.Fatalf("read preserved loop definition snapshot error = %v", err)
	}
	if definitionJSON != `{"kind":"loop"}` {
		t.Fatalf("preserved definition snapshot = %q, want loop definition", definitionJSON)
	}
	var loopConfig string
	if err := globalDB.db.QueryRowContext(
		ctx,
		`SELECT enabled_checks_json FROM loop_config
		 WHERE workspace_id = 'ws-precut' AND loop_name = 'precut-loop'`,
	).Scan(&loopConfig); err != nil {
		t.Fatalf("read preserved loop configuration error = %v", err)
	}
	if loopConfig != `{"input-default":true}` {
		t.Fatalf("preserved loop configuration = %q, want configured checks", loopConfig)
	}
	assertTableExcludesColumns(t, globalDB.db, "sessions", []string{"channel"})
	assertTableExcludesColumns(t, globalDB.db, "tasks", []string{"network_channel"})
	assertTableExcludesColumns(t, globalDB.db, "task_runs", []string{"coordination_channel_id"})
	assertTableExcludesColumns(t, globalDB.db, "network_channel_participants", []string{"peer_id"})
	assertTableExcludesColumns(t, globalDB.db, "network_direct_rooms", []string{"peer_a", "peer_b"})
	assertTableExcludesColumns(t, globalDB.db, "network_subscriptions", []string{"peer_id"})
	assertTableExcludesColumns(t, globalDB.db, "network_thread_participants", []string{"peer_id"})

	guidanceExists, err := tableExists(ctx, globalDB.db, "network_delivery_guidance_state")
	if err != nil {
		t.Fatalf("tableExists(network_delivery_guidance_state) error = %v", err)
	}
	if guidanceExists {
		t.Fatal("network_delivery_guidance_state still exists after destructive cut")
	}
	for _, relation := range []struct {
		name  string
		query string
	}{
		{name: "network_channel_participants", query: `SELECT COUNT(*) FROM network_channel_participants`},
		{name: "network_direct_rooms", query: `SELECT COUNT(*) FROM network_direct_rooms`},
		{name: "network_subscriptions", query: `SELECT COUNT(*) FROM network_subscriptions`},
		{name: "network_thread_participants", query: `SELECT COUNT(*) FROM network_thread_participants`},
	} {
		var count int
		if err := globalDB.db.QueryRowContext(ctx, relation.query).Scan(&count); err != nil {
			t.Fatalf("count %s error = %v", relation.name, err)
		}
		if count != 0 {
			t.Fatalf("%s row count after destructive cut = %d, want 0", relation.name, count)
		}
	}
	var autoChannelCount int
	if err := globalDB.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM network_channels WHERE purpose = 'task_run_coordination'`,
	).Scan(&autoChannelCount); err != nil {
		t.Fatalf("count task_run_coordination channels error = %v", err)
	}
	if autoChannelCount != 0 {
		t.Fatalf("task_run_coordination channel count = %d, want 0", autoChannelCount)
	}

	sessions, err := globalDB.ListSessions(ctx, store.SessionListQuery{
		ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID},
		ID:        "sess-precut",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("ListSessions(pre-cut history) error = %v", err)
	}
	if len(sessions) != 1 || !reflect.DeepEqual(sessions[0].NetworkSpec, participation.LocalSpec()) {
		t.Fatalf("ListSessions(pre-cut history) = %#v, want one canonical Local session", sessions)
	}
	taskRun, err := globalDB.GetTaskRun(ctx, "run-precut")
	if err != nil {
		t.Fatalf("GetTaskRun(pre-cut history) error = %v", err)
	}
	if !reflect.DeepEqual(taskRun.NetworkSpec, participation.LocalSpec()) {
		t.Fatalf("GetTaskRun(pre-cut history).NetworkSpec = %#v, want canonical Local", taskRun.NetworkSpec)
	}
	if _, err := globalDB.GetLoopRun(
		ctx,
		looppkg.WorkspaceID("ws-precut"),
		looppkg.RunID("loop-precut"),
	); !errors.Is(
		err,
		looppkg.ErrRunNotFound,
	) {
		t.Fatalf("GetLoopRun(pre-lineage history) error = %v, want ErrRunNotFound", err)
	}
}

func assertOwnerTableCanonicalLocal(t *testing.T, db *sql.DB, table string) {
	t.Helper()

	queries := map[string]string{
		"sessions":  `SELECT network_spec_json, network_mode, network_channel, network_source FROM sessions`,
		"task_runs": `SELECT network_spec_json, network_mode, network_channel, network_source FROM task_runs`,
		"loop_runs": `SELECT network_spec_json, network_mode, network_channel, network_source FROM loop_runs`,
	}
	query, ok := queries[table]
	if !ok {
		t.Fatalf("assertOwnerTableCanonicalLocal(%q) has no canonical query", table)
	}
	rows, err := db.QueryContext(testutil.Context(t), query)
	if err != nil {
		t.Fatalf("query %s network snapshots error = %v", table, err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			t.Fatalf("close %s network snapshot rows error = %v", table, err)
		}
	}()
	expectedJSON, err := json.Marshal(participation.LocalSpec())
	if err != nil {
		t.Fatalf("json.Marshal(canonical Local) error = %v", err)
	}
	count := 0
	for rows.Next() {
		var (
			rawSnapshot string
			mode        string
			channel     sql.NullString
			source      string
		)
		if err := rows.Scan(&rawSnapshot, &mode, &channel, &source); err != nil {
			t.Fatalf("scan %s network snapshot error = %v", table, err)
		}
		count++
		if rawSnapshot != string(expectedJSON) || mode != "local" || channel.Valid || source != "built_in_local" {
			t.Fatalf(
				"%s network snapshot = (%q, %q, %#v, %q), want canonical Local projections",
				table,
				rawSnapshot,
				mode,
				channel,
				source,
			)
		}
		decoded, err := decodeParticipationSnapshot("", rawSnapshot, mode, channel, source)
		if err != nil {
			t.Fatalf("decode %s network snapshot error = %v", table, err)
		}
		if !reflect.DeepEqual(decoded, participation.LocalSpec()) {
			t.Fatalf("decoded %s NetworkSpec = %#v, want canonical Local", table, decoded)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate %s network snapshots error = %v", table, err)
	}
	if count != 1 {
		t.Fatalf("%s retained owner count = %d, want 1", table, count)
	}
}

func TestOpenGlobalDBRefusesLegacyDatabaseWithoutMutation(t *testing.T) {
	t.Run("Should return ErrLegacyDatabase without changing the database file", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		legacy, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatalf("sql.Open(legacy) error = %v", err)
		}
		if _, err := legacy.ExecContext(ctx, `CREATE TABLE schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		checksum TEXT NOT NULL,
		applied_at TEXT NOT NULL
	)`); err != nil {
			t.Fatalf("create legacy schema_migrations error = %v", err)
		}
		if _, err := legacy.ExecContext(ctx, `WITH RECURSIVE versions(version) AS (
		SELECT 1 UNION ALL SELECT version + 1 FROM versions WHERE version < 61
	) INSERT INTO schema_migrations (version, name, checksum, applied_at)
	SELECT version,
		CASE WHEN version = 1 THEN 'create_global_schema' ELSE printf('legacy_%02d', version) END,
		printf('checksum-%02d', version),
		'2026-07-10T00:00:00Z'
	FROM versions`); err != nil {
			t.Fatalf("seed legacy schema_migrations error = %v", err)
		}
		if err := legacy.Close(); err != nil {
			t.Fatalf("Close(legacy) error = %v", err)
		}
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(before) error = %v", err)
		}

		_, openErr := OpenGlobalDB(ctx, path)
		if !errors.Is(openErr, store.ErrLegacyDatabase) {
			t.Fatalf("OpenGlobalDB(legacy) error = %v, want ErrLegacyDatabase", openErr)
		}
		canonicalPath, err := filepath.EvalSymlinks(path)
		if err != nil {
			t.Fatalf("EvalSymlinks(legacy path) error = %v", err)
		}
		if !strings.Contains(openErr.Error(), canonicalPath) ||
			!strings.Contains(openErr.Error(), "complete COMPOZY_HOME or workspace .compozy directory") {
			t.Fatalf("OpenGlobalDB(legacy) error = %q, want path and whole-family remediation", openErr)
		}
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(after) error = %v", err)
		}
		if !bytes.Equal(after, before) {
			t.Fatal("legacy database bytes changed after refused open")
		}
	})
}

func TestSweepObservabilityDeletesOnlyRowsOlderThanCutoff(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	globalDB := openTestGlobalDB(t)
	registerSessionForGlobalTests(t, globalDB, "sess-retention")

	cutoff := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	old := cutoff.Add(-time.Nanosecond)
	boundary := cutoff
	fresh := cutoff.Add(time.Nanosecond)

	for _, event := range []EventSummary{
		{ProfileID: store.DefaultProfileID, ID: "sum-old", SessionID: "sess-retention", Type: "agent_message", AgentName: "coder", Timestamp: old},
		{ProfileID: store.DefaultProfileID, ID: "sum-boundary", SessionID: "sess-retention", Type: "agent_message", AgentName: "coder", Timestamp: boundary},
		{ProfileID: store.DefaultProfileID, ID: "sum-fresh", SessionID: "sess-retention", Type: "agent_message", AgentName: "coder", Timestamp: fresh},
	} {
		if err := globalDB.WriteEventSummary(ctx, event); err != nil {
			t.Fatalf("WriteEventSummary(%q) error = %v", event.ID, err)
		}
	}

	for _, update := range []TokenStatsUpdate{
		{SessionID: "sess-retention", AgentName: "coder-old", CostStatus: "unknown", CostSource: "none", Turns: 1, UpdatedAt: old},
		{SessionID: "sess-retention", AgentName: "coder-boundary", CostStatus: "unknown", CostSource: "none", Turns: 1, UpdatedAt: boundary},
		{SessionID: "sess-retention", AgentName: "coder-fresh", CostStatus: "unknown", CostSource: "none", Turns: 1, UpdatedAt: fresh},
	} {
		if err := globalDB.UpdateTokenStats(ctx, update); err != nil {
			t.Fatalf("UpdateTokenStats(%q) error = %v", update.AgentName, err)
		}
	}

	for _, entry := range []PermissionLogEntry{
		{
			ID: "perm-old", SessionID: "sess-retention", AgentName: "coder", Action: "fs/read",
			Resource: "old.md", Decision: "allow", PolicyUsed: "approve-all", Timestamp: old,
		},
		{
			ID: "perm-boundary", SessionID: "sess-retention", AgentName: "coder", Action: "fs/read",
			Resource: "boundary.md", Decision: "allow", PolicyUsed: "approve-all", Timestamp: boundary,
		},
		{
			ID: "perm-fresh", SessionID: "sess-retention", AgentName: "coder", Action: "fs/read",
			Resource: "fresh.md", Decision: "allow", PolicyUsed: "approve-all", Timestamp: fresh,
		},
	} {
		if err := globalDB.WritePermissionLog(ctx, entry); err != nil {
			t.Fatalf("WritePermissionLog(%q) error = %v", entry.ID, err)
		}
	}

	result, err := globalDB.SweepObservability(ctx, cutoff)
	if err != nil {
		t.Fatalf("SweepObservability() error = %v", err)
	}
	if result.DeletedEventSummaries != 1 || result.DeletedTokenStats != 1 || result.DeletedPermissionLogs != 1 {
		t.Fatalf("SweepObservability() = %#v, want one deleted row per observe table", result)
	}
	if !result.CutoffAt.Equal(cutoff) {
		t.Fatalf("SweepObservability().CutoffAt = %s, want %s", result.CutoffAt, cutoff)
	}

	assertEventSummaryIDs(t, globalDB, []string{"sum-boundary", "sum-fresh"})
	assertTokenStatAgents(t, globalDB, []string{"coder-boundary", "coder-fresh"})
	assertPermissionLogIDs(t, globalDB, []string{"perm-boundary", "perm-fresh"})
}

func TestOpenGlobalDBCreatesExtensionsTableWithExpectedColumns(t *testing.T) {
	t.Run("Should create every extension table with the expected columns", func(t *testing.T) {
		t.Parallel()

		globalDB := openFreshTestGlobalDB(t)

		assertTableColumns(t, globalDB.db, "extensions", []string{
			"name",
			"version",
			"source",
			"manifest_path",
			"format",
			"ingest_diagnostics_json",
			"installed_at",
			"provides_json",
			"permissions_json",
			"checksum",
			"lifecycle_token",
			"registry_slug",
			"registry_name",
			"remote_version",
			globalDBExtensionProvenanceJSONKey,
			"network_requirement_digest",
			"network_confirmed_by",
			"network_confirmed_at",
		})
		assertTableColumns(t, globalDB.db, "extension_profile_enablement", []string{
			"extension_name",
			"profile_id",
			"enabled",
		})
		assertTableColumns(t, globalDB.db, "extension_dev_links", []string{
			"extension_name",
			"workspace_id",
			"origin_path",
			"bundle_generation",
			"linked_at",
			"format",
			"ingest_diagnostics_json",
			"network_requirement_digest",
			"network_confirmed_by",
			"network_confirmed_at",
		})
		assertTableColumns(t, globalDB.db, "extension_env_bindings", []string{
			"extension_name",
			"profile_id",
			"workspace_id",
			"env_name",
			"secret_ref",
			"mcp_server",
			"header_name",
			"kind",
			"created_at",
			"updated_at",
		})
	})
}

func TestOpenGlobalDBExtensionsSchemaIsIdempotent(t *testing.T) {
	t.Run("Should preserve the extension schema when the database reopens", func(t *testing.T) {
		t.Parallel()

		dbPath := filepath.Join(t.TempDir(), GlobalDatabaseName)
		first, err := OpenGlobalDB(testutil.Context(t), dbPath)
		if err != nil {
			t.Fatalf("OpenGlobalDB(first) error = %v", err)
		}
		if err := first.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(first) error = %v", err)
		}

		second, err := OpenGlobalDB(testutil.Context(t), dbPath)
		if err != nil {
			t.Fatalf("OpenGlobalDB(second) error = %v", err)
		}
		t.Cleanup(func() {
			if err := second.Close(testutil.Context(t)); err != nil {
				t.Fatalf("Close(second) error = %v", err)
			}
		})

		assertTableColumns(t, second.db, "extensions", []string{
			"name",
			"version",
			"source",
			"manifest_path",
			"format",
			"ingest_diagnostics_json",
			"installed_at",
			"provides_json",
			"permissions_json",
			"checksum",
			"lifecycle_token",
			"registry_slug",
			"registry_name",
			"remote_version",
			globalDBExtensionProvenanceJSONKey,
			"network_requirement_digest",
			"network_confirmed_by",
			"network_confirmed_at",
		})
		assertTableColumns(t, second.db, "extension_profile_enablement", []string{
			"extension_name",
			"profile_id",
			"enabled",
		})
		assertTableColumns(t, second.db, "extension_dev_links", []string{
			"extension_name",
			"workspace_id",
			"origin_path",
			"bundle_generation",
			"linked_at",
			"format",
			"ingest_diagnostics_json",
			"network_requirement_digest",
			"network_confirmed_by",
			"network_confirmed_at",
		})
		assertTableColumns(t, second.db, "extension_env_bindings", []string{
			"extension_name",
			"profile_id",
			"workspace_id",
			"env_name",
			"secret_ref",
			"mcp_server",
			"header_name",
			"kind",
			"created_at",
			"updated_at",
		})
	})
}

func TestRepositoryCheckReady(t *testing.T) {
	t.Run("Should accept a valid repository and reject a nil context", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		nilContext := func() context.Context { return nil }
		if err := globalDB.SessionRepo.checkReady(nilContext(), "list sessions"); err == nil {
			t.Fatal("checkReady(nil context) error = nil, want non-nil")
		}
		if err := globalDB.SessionRepo.checkReady(testutil.Context(t), "list sessions"); err != nil {
			t.Fatalf("checkReady(valid) error = %v", err)
		}
	})

	t.Run("Should reject a repository after its database closes", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		if err := globalDB.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		if err := globalDB.SessionRepo.checkReady(
			testutil.Context(t),
			"list sessions",
		); !errors.Is(err, store.ErrClosed) {
			t.Fatalf("checkReady(after close) error = %v, want ErrClosed", err)
		}
	})
}

func TestGlobalDBRegisterUpdateAndListSessions(t *testing.T) {
	t.Run("Should persist recovery state and preserve workspace-scoped listing", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		createdAt := time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"sess-global-workspace",
			filepath.Join(t.TempDir(), "workspace-global"),
		)
		session := SessionInfo{
			ProfileID:         store.DefaultProfileID,
			ID:                "sess-global",
			Name:              "Alpha",
			AgentName:         "coder",
			Provider:          "claude",
			Model:             "claude-opus-4",
			ReasoningEffort:   "high",
			Speed:             speedpkg.SpeedFast,
			SpeedResolution:   &speedpkg.Resolution{Requested: speedpkg.SpeedFast, Status: speedpkg.ResolutionApplied},
			RuntimeStatus:     store.SessionRuntimeRecovering,
			RuntimeTransition: store.SessionRuntimeTransitionAutomaticRecovery,
			RuntimeGeneration: 3,
			SessionRuntimeDetails: &store.SessionRuntimeDetails{
				RuntimeRecovery: &store.SessionRuntimeRecovery{
					Attempt:       1,
					MaxAttempts:   3,
					Generation:    4,
					StartedAt:     createdAt,
					LastAttemptAt: createdAt.Add(time.Second),
				},
			},
			SelectedRuntime: &store.SessionRuntimeSelection{
				Provider:        "claude",
				Model:           "claude-fable-5",
				ReasoningEffort: "max",
				Speed:           speedpkg.SpeedFast,
			},
			RuntimeSelectionRevision: 4,
			WorkspaceID:              workspaceID,
			SessionType:              "dream",
			State:                    "active",
			CreatedAt:                createdAt,
			UpdatedAt:                createdAt,
		}

		if err := globalDB.RegisterSession(testutil.Context(t), session); err != nil {
			t.Fatalf("RegisterSession() error = %v", err)
		}
		databasePath := globalDB.Path()
		if err := globalDB.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(before recovery reopen) error = %v", err)
		}
		globalDB = openGlobalDBForTest(t, databasePath)

		registered, err := globalDB.ListSessions(testutil.Context(t), SessionListQuery{
			ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID},
			ID:        session.ID,
		})
		if err != nil {
			t.Fatalf("ListSessions(registered recovery) error = %v", err)
		}
		if len(registered) != 1 {
			t.Fatalf("ListSessions(registered recovery) = %#v, want one session", registered)
		}
		registeredRecovery := registered[0].RuntimeRecoveryValue()
		if registered[0].RuntimeGeneration != 3 ||
			registeredRecovery == nil || registeredRecovery.Generation != 4 {
			t.Fatalf("ListSessions(registered recovery) = %#v, want durable generation 3 recovering into 4", registered)
		}

		acpSessionID := "acp-123"
		if err := globalDB.UpdateSessionState(testutil.Context(t), SessionStateUpdate{
			ID:                session.ID,
			State:             "stopped",
			ACPSessionID:      &acpSessionID,
			RuntimeSet:        true,
			Provider:          "codex",
			Model:             "gpt-5.6",
			ReasoningEffort:   "medium",
			Speed:             speedpkg.SpeedNormal,
			RuntimeStatus:     store.SessionRuntimeReady,
			RuntimeTransition: store.SessionRuntimeTransitionProcessReplacement,
			RuntimeGeneration: 4,
			UpdatedAt:         createdAt.Add(2 * time.Minute),
		}); err != nil {
			t.Fatalf("UpdateSessionState() error = %v", err)
		}

		foreignWorkspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"sess-global-foreign-workspace",
			filepath.Join(t.TempDir(), "workspace-global-foreign"),
		)
		if err := globalDB.RegisterSession(testutil.Context(t), SessionInfo{
			ProfileID:         store.DefaultProfileID,
			ID:                "sess-global-foreign",
			AgentName:         "coder",
			Provider:          "foreign-provider",
			Model:             "foreign-model",
			RuntimeStatus:     store.SessionRuntimeReady,
			RuntimeTransition: store.SessionRuntimeTransitionInitialBind,
			WorkspaceID:       foreignWorkspaceID,
			State:             "stopped",
			CreatedAt:         createdAt,
			UpdatedAt:         createdAt,
		}); err != nil {
			t.Fatalf("RegisterSession(foreign) error = %v", err)
		}

		sessions, err := globalDB.ListSessions(testutil.Context(t), SessionListQuery{
			ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
			State:       "stopped",
			WorkspaceID: workspaceID,
		})
		if err != nil {
			t.Fatalf("ListSessions() error = %v", err)
		}
		if got, want := len(sessions), 1; got != want {
			t.Fatalf("len(sessions) = %d, want %d", got, want)
		}
		if sessions[0].State != "stopped" {
			t.Fatalf("sessions[0].State = %q, want stopped", sessions[0].State)
		}
		if sessions[0].SessionType != "dream" {
			t.Fatalf("sessions[0].SessionType = %q, want dream", sessions[0].SessionType)
		}
		if sessions[0].WorkspaceID != workspaceID {
			t.Fatalf("sessions[0].WorkspaceID = %q, want %q", sessions[0].WorkspaceID, workspaceID)
		}
		if sessions[0].Provider != "codex" {
			t.Fatalf("sessions[0].Provider = %q, want codex", sessions[0].Provider)
		}
		sessionRecovery := sessions[0].RuntimeRecoveryValue()
		if sessions[0].RuntimeGeneration != 4 || sessionRecovery != nil {
			t.Fatalf(
				"session runtime after recovery = generation %d, recovery %#v; want generation 4 without recovery",
				sessions[0].RuntimeGeneration,
				sessionRecovery,
			)
		}
		if sessions[0].Model != "gpt-5.6" || sessions[0].ReasoningEffort != "medium" ||
			sessions[0].Speed != speedpkg.SpeedNormal || sessions[0].SpeedResolution != nil ||
			sessions[0].RuntimeStatus != store.SessionRuntimeReady ||
			sessions[0].RuntimeTransition != store.SessionRuntimeTransitionProcessReplacement {
			t.Fatalf("sessions[0] runtime projection = %#v", sessions[0])
		}
		if sessions[0].SelectedRuntime == nil || sessions[0].SelectedRuntime.Provider != "claude" ||
			sessions[0].SelectedRuntime.Model != "claude-fable-5" ||
			sessions[0].SelectedRuntime.ReasoningEffort != "max" ||
			sessions[0].SelectedRuntime.Speed != speedpkg.SpeedFast ||
			sessions[0].RuntimeSelectionRevision != 4 {
			t.Fatalf(
				"sessions[0] selected runtime = %#v revision %d, want durable Claude selection at revision 4",
				sessions[0].SelectedRuntime,
				sessions[0].RuntimeSelectionRevision,
			)
		}
		if sessions[0].ACPSessionID == nil || *sessions[0].ACPSessionID != "acp-123" {
			t.Fatalf("sessions[0].ACPSessionID = %#v, want acp-123", sessions[0].ACPSessionID)
		}
	})
}

func TestGlobalDBRegisterSessionUpsertsProvider(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"provider-upsert-workspace",
		filepath.Join(t.TempDir(), "provider-upsert"),
	)
	createdAt := time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)

	session := SessionInfo{
		ProfileID:     store.DefaultProfileID,
		ID:            "sess-provider-upsert",
		AgentName:     "coder",
		Provider:      "claude",
		RuntimeStatus: store.SessionRuntimeUnbound,
		WorkspaceID:   workspaceID,
		State:         "active",
		CreatedAt:     createdAt,
		UpdatedAt:     createdAt,
	}
	if err := globalDB.RegisterSession(testutil.Context(t), session); err != nil {
		t.Fatalf("RegisterSession(initial) error = %v", err)
	}

	session.Provider = "codex"
	session.UpdatedAt = createdAt.Add(time.Minute)
	if err := globalDB.RegisterSession(testutil.Context(t), session); err != nil {
		t.Fatalf("RegisterSession(update) error = %v", err)
	}

	sessions, err := globalDB.ListSessions(testutil.Context(t), SessionListQuery{
		ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID},
	})
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if got, want := len(sessions), 1; got != want {
		t.Fatalf("len(sessions) = %d, want %d", got, want)
	}
	if got, want := sessions[0].Provider, "codex"; got != want {
		t.Fatalf("sessions[0].Provider = %q, want %q", got, want)
	}

	var provider string
	if err := globalDB.db.QueryRowContext(
		testutil.Context(t),
		`SELECT provider FROM sessions WHERE id = ?`,
		session.ID,
	).Scan(&provider); err != nil {
		t.Fatalf("QueryRowContext(provider) error = %v", err)
	}
	if got, want := provider, "codex"; got != want {
		t.Fatalf("stored provider = %q, want %q", got, want)
	}
}

func TestGlobalDBRegisterSessionPersistsStopFields(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name           string
		stopReason     store.StopReason
		stopDetail     string
		wantStopReason *string
		wantStopDetail *string
	}{
		{
			name: "empty stop reason stores nulls",
		},
		{
			name:           "valid stop reason stores values",
			stopReason:     store.StopTimeout,
			stopDetail:     "deadline exceeded",
			wantStopReason: stringPointerForTest(string(store.StopTimeout)),
			wantStopDetail: stringPointerForTest("deadline exceeded"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			globalDB := openTestGlobalDB(t)
			workspaceID := registerWorkspaceForGlobalTests(
				t,
				globalDB,
				"persist-stop-workspace-"+strings.ReplaceAll(tc.name, " ", "-"),
				filepath.Join(t.TempDir(), "workspace"),
			)
			session := SessionInfo{
				ProfileID:     store.DefaultProfileID,
				ID:            "sess-" + strings.ReplaceAll(tc.name, " ", "-"),
				AgentName:     "coder",
				RuntimeStatus: store.SessionRuntimeUnbound,
				WorkspaceID:   workspaceID,
				State:         "stopped",
				StopReason:    tc.stopReason,
				StopDetail:    tc.stopDetail,
				CreatedAt:     time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
				UpdatedAt:     time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
			}

			if err := globalDB.RegisterSession(testutil.Context(t), session); err != nil {
				t.Fatalf("RegisterSession() error = %v", err)
			}

			gotStopReason, gotStopDetail := queryStoredSessionStopFields(t, globalDB.db, session.ID)
			assertOptionalStringEqual(t, gotStopReason, tc.wantStopReason, "stop_reason")
			assertOptionalStringEqual(t, gotStopDetail, tc.wantStopDetail, "stop_detail")

			sessions, err := globalDB.ListSessions(testutil.Context(t), SessionListQuery{
				ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID},
			})
			if err != nil {
				t.Fatalf("ListSessions() error = %v", err)
			}
			if got, want := len(sessions), 1; got != want {
				t.Fatalf("len(sessions) = %d, want %d", got, want)
			}
			if got, want := sessions[0].StopReason, tc.stopReason; got != want {
				t.Fatalf("sessions[0].StopReason = %q, want %q", got, want)
			}
			if got, want := sessions[0].StopDetail, tc.stopDetail; got != want {
				t.Fatalf("sessions[0].StopDetail = %q, want %q", got, want)
			}
		})
	}
}

func TestGlobalDBRegisterSessionDefaultsTypeToUser(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"sess-default-type-workspace",
		filepath.Join(t.TempDir(), "workspace-default-type"),
	)
	session := SessionInfo{
		ProfileID:     store.DefaultProfileID,
		ID:            "sess-default-type",
		AgentName:     "coder",
		RuntimeStatus: store.SessionRuntimeUnbound,
		WorkspaceID:   workspaceID,
		State:         "active",
		CreatedAt:     time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
	}

	if err := globalDB.RegisterSession(testutil.Context(t), session); err != nil {
		t.Fatalf("RegisterSession() error = %v", err)
	}

	sessions, err := globalDB.ListSessions(testutil.Context(t), SessionListQuery{
		ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID},
	})
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if got, want := len(sessions), 1; got != want {
		t.Fatalf("len(sessions) = %d, want %d", got, want)
	}
	if got, want := sessions[0].SessionType, defaultSessionType; got != want {
		t.Fatalf("sessions[0].SessionType = %q, want %q", got, want)
	}
}

func TestGlobalDBTaskEventSequenceReads(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	ctx := testutil.Context(t)
	createdAt := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	actor := taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "user-1"}
	origin := taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "compozy task test"}

	if err := globalDB.CreateTask(ctx, taskpkg.Task{
		ID:             "task-seq",
		Scope:          taskpkg.ScopeGlobal,
		Title:          "Sequence task",
		Priority:       taskpkg.DefaultPriority,
		MaxAttempts:    taskpkg.DefaultTaskMaxAttempts,
		Status:         taskpkg.TaskStatusReady,
		ApprovalPolicy: taskpkg.ApprovalPolicyNone,
		ApprovalState:  taskpkg.ApprovalStateNotRequired,
		CreatedBy:      actor,
		Origin:         origin,
		CreatedAt:      createdAt,
		ProfileID:      store.DefaultProfileID,
		UpdatedAt:      createdAt,
	}); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if err := globalDB.CreateTaskRun(ctx, taskpkg.Run{
		ID:        "run-seq",
		TaskID:    "task-seq",
		Status:    taskpkg.TaskRunStatusRunning,
		Attempt:   1,
		Origin:    origin,
		QueuedAt:  createdAt,
		ProfileID: store.DefaultProfileID,
		StartedAt: createdAt.Add(time.Minute),
	}); err != nil {
		t.Fatalf("CreateTaskRun() error = %v", err)
	}
	emptyTask, err := globalDB.GetTask(ctx, "task-seq")
	if err != nil {
		t.Fatalf("GetTask(empty) error = %v", err)
	}
	if emptyTask.LatestEventSeq != 0 {
		t.Fatalf("LatestEventSeq before events = %d, want 0", emptyTask.LatestEventSeq)
	}

	for _, event := range []taskpkg.Event{
		{
			ID:        "evt-1",
			TaskID:    "task-seq",
			EventType: "task.created",
			Actor:     actor,
			Origin:    origin,
			Timestamp: createdAt,
		},
		{
			ID:        "evt-2",
			TaskID:    "task-seq",
			RunID:     "run-seq",
			EventType: "task.run_started",
			Actor:     actor,
			Origin:    origin,
			Timestamp: createdAt,
		},
		{
			ID:        "evt-3",
			TaskID:    "task-seq",
			EventType: "task.updated",
			Actor:     actor,
			Origin:    origin,
			Timestamp: createdAt,
		},
	} {
		if err := globalDB.CreateTaskEvent(ctx, event); err != nil {
			t.Fatalf("CreateTaskEvent(%q) error = %v", event.ID, err)
		}
	}

	record, err := globalDB.GetTaskEventRecord(ctx, "evt-2")
	if err != nil {
		t.Fatalf("GetTaskEventRecord() error = %v", err)
	}
	if got, want := record.Sequence, int64(2); got != want {
		t.Fatalf("record.Sequence = %d, want %d", got, want)
	}
	if got, want := record.Event.RunID, "run-seq"; got != want {
		t.Fatalf("record.Event.RunID = %q, want %q", got, want)
	}

	records, err := globalDB.ListTaskEventRecords(ctx, taskpkg.EventRecordQuery{
		TaskID:        "task-seq",
		AfterSequence: 1,
		Limit:         2,
	})
	if err != nil {
		t.Fatalf("ListTaskEventRecords() error = %v", err)
	}
	if got, want := len(records), 2; got != want {
		t.Fatalf("len(records) = %d, want %d", got, want)
	}
	if got, want := []string{
		records[0].Event.ID,
		records[1].Event.ID,
	}, []string{
		"evt-2",
		"evt-3",
	}; !testutil.EqualStringSlices(
		got,
		want,
	) {
		t.Fatalf("record ids = %#v, want %#v", got, want)
	}
	if got, want := []int64{
		records[0].Sequence,
		records[1].Sequence,
	}, []int64{
		2,
		3,
	}; got[0] != want[0] ||
		got[1] != want[1] {
		t.Fatalf("record sequences = %#v, want %#v", got, want)
	}

	taskRecord, err := globalDB.GetTask(ctx, "task-seq")
	if err != nil {
		t.Fatalf("GetTask(after events) error = %v", err)
	}
	if taskRecord.LatestEventSeq != 3 {
		t.Fatalf("LatestEventSeq after events = %d, want 3", taskRecord.LatestEventSeq)
	}
	summaries, err := globalDB.ListTasks(ctx, taskpkg.Query{})
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if len(summaries) != 1 || summaries[0].LatestEventSeq != 3 {
		t.Fatalf("ListTasks() = %#v, want latest_event_seq=3", summaries)
	}
	descRecords, err := globalDB.ListTaskEventRecords(ctx, taskpkg.EventRecordQuery{
		TaskID:     "task-seq",
		Limit:      2,
		Descending: true,
	})
	if err != nil {
		t.Fatalf("ListTaskEventRecords(descending) error = %v", err)
	}
	if len(descRecords) != 2 {
		t.Fatalf("len(descRecords) = %d, want 2", len(descRecords))
	}
	if got, want := []string{
		descRecords[0].Event.ID,
		descRecords[1].Event.ID,
	}, []string{
		"evt-3",
		"evt-2",
	}; !testutil.EqualStringSlices(
		got,
		want,
	) {
		t.Fatalf("descending record ids = %#v, want %#v", got, want)
	}
}

func TestGlobalDBWorkspaceCRUDAndLookups(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	rootParent := t.TempDir()
	rootDir := filepath.Join(rootParent, "workspace-root")
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(rootDir) error = %v", err)
	}
	symlinkPath := filepath.Join(t.TempDir(), "workspace-link")
	if err := os.Symlink(rootDir, symlinkPath); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	canonicalRoot, err := filepath.EvalSymlinks(symlinkPath)
	if err != nil {
		t.Fatalf("EvalSymlinks() error = %v", err)
	}

	createdAt := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	ws := compozyworkspace.Workspace{
		ID:             "ws-primary",
		RootDir:        canonicalRoot,
		AdditionalDirs: []string{filepath.Join(rootDir, "a"), "", filepath.Join(rootDir, "b")},
		Name:           "alpha",
		DefaultAgent:   "coder",
		SandboxRef:     "daytona-dev",
		CreatedAt:      createdAt,
		UpdatedAt:      createdAt,
	}
	if err := globalDB.InsertWorkspace(testutil.Context(t), ws); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}

	byID, err := globalDB.GetWorkspace(testutil.Context(t), ws.ID)
	if err != nil {
		t.Fatalf("GetWorkspace() error = %v", err)
	}
	assertWorkspaceEqual(t, byID, compozyworkspace.Workspace{
		ID:             ws.ID,
		RootDir:        canonicalRoot,
		AdditionalDirs: []string{filepath.Join(rootDir, "a"), filepath.Join(rootDir, "b")},
		Name:           "alpha",
		DefaultAgent:   "coder",
		SandboxRef:     "daytona-dev",
		CreatedAt:      createdAt,
		UpdatedAt:      createdAt,
	})

	byPath, err := globalDB.GetWorkspaceByPath(testutil.Context(t), canonicalRoot)
	if err != nil {
		t.Fatalf("GetWorkspaceByPath() error = %v", err)
	}
	assertWorkspaceEqual(t, byPath, byID)

	byName, err := globalDB.GetWorkspaceByName(testutil.Context(t), "alpha")
	if err != nil {
		t.Fatalf("GetWorkspaceByName() error = %v", err)
	}
	assertWorkspaceEqual(t, byName, byID)

	updated := byID
	updated.Name = "beta"
	updated.DefaultAgent = "reviewer"
	updated.SandboxRef = "local-dev"
	updated.AdditionalDirs = []string{filepath.Join(rootDir, "tools")}
	updated.UpdatedAt = createdAt.Add(5 * time.Minute)
	if err := globalDB.UpdateWorkspace(testutil.Context(t), updated); err != nil {
		t.Fatalf("UpdateWorkspace() error = %v", err)
	}

	gotUpdated, err := globalDB.GetWorkspace(testutil.Context(t), updated.ID)
	if err != nil {
		t.Fatalf("GetWorkspace(updated) error = %v", err)
	}
	assertWorkspaceEqual(t, gotUpdated, updated)

	t.Run("Should persist revisioned network coordination with availability fencing", func(t *testing.T) {
		ctx := testutil.Context(t)
		ref := compozyworkspace.CoordinationRef{
			WorkspaceID: updated.ID,
			ScopeKind:   compozyworkspace.InvitationScopeWorkspace,
		}
		commands := compozyworkspace.NewCoordinationService(globalDB, nil)
		initialView, getErr := commands.Get(ctx, ref, operatorActorContextForTest("operator:reader"))
		if getErr != nil {
			t.Fatalf("Get(initial coordination) error = %v", getErr)
		}
		initial := initialView.Setting
		if initial.Enabled || initial.Revision != 0 || initial.WorkspaceID != updated.ID {
			t.Fatalf("Get(initial coordination) = %#v, want disabled revision zero", initial)
		}
		if !initial.UpdatedAt.IsZero() || initial.UpdatedBy != "" {
			t.Fatalf("Get(initial coordination) = %#v, absent row must not invent provenance", initial)
		}

		firstTime := time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
		globalDB.now = func() time.Time { return firstTime }
		firstView, setErr := commands.Set(ctx, compozyworkspace.SetCoordination{
			Ref: ref, Enabled: true, ExpectedRevision: 0,
		}, operatorActorContextForTest("operator:first"))
		if setErr != nil {
			t.Fatalf("Set(first coordination) error = %v", setErr)
		}
		first := firstView.Setting
		if !first.Enabled || first.Revision != 1 || first.UpdatedBy != "operator:first" {
			t.Fatalf("Set(first coordination) = %#v, want enabled revision one", first)
		}

		secondView, setErr := commands.Set(ctx, compozyworkspace.SetCoordination{
			Ref: ref, Enabled: false, ExpectedRevision: 1,
		}, operatorActorContextForTest("operator:second"))
		if setErr != nil {
			t.Fatalf("Set(second coordination) error = %v", setErr)
		}
		second := secondView.Setting
		if second.Enabled || second.Revision != 2 || second.UpdatedBy != "operator:second" {
			t.Fatalf("Set(second coordination) = %#v, want disabled revision two", second)
		}
		if !second.UpdatedAt.After(first.UpdatedAt) {
			t.Fatalf("second.UpdatedAt = %s, want after %s", second.UpdatedAt, first.UpdatedAt)
		}

		if _, disableErr := globalDB.SetNetworkAvailability(ctx, false, "operator:disable"); disableErr != nil {
			t.Fatalf("SetNetworkAvailability(false) error = %v", disableErr)
		}
		if _, setErr = commands.Set(ctx, compozyworkspace.SetCoordination{
			Ref: ref, Enabled: true, ExpectedRevision: 2,
		}, operatorActorContextForTest("operator:blocked")); !errors.Is(
			setErr,
			participation.ErrUnavailable,
		) {
			t.Fatalf("Set(while unavailable) error = %v, want %v", setErr, participation.ErrUnavailable)
		}
		unchangedView, getErr := commands.Get(ctx, ref, operatorActorContextForTest("operator:reader"))
		if getErr != nil {
			t.Fatalf("Get(after blocked Set) error = %v", getErr)
		}
		unchanged := unchangedView.Setting
		if unchanged.Revision != second.Revision || unchanged.UpdatedBy != second.UpdatedBy {
			t.Fatalf("Get(after blocked Set) = %#v, want unchanged %#v", unchanged, second)
		}
		if _, enableErr := globalDB.SetNetworkAvailability(ctx, true, "operator:enable"); enableErr != nil {
			t.Fatalf("SetNetworkAvailability(true) error = %v", enableErr)
		}
		thirdView, setErr := commands.Set(ctx, compozyworkspace.SetCoordination{
			Ref: ref, Enabled: true, ExpectedRevision: 2,
		}, operatorActorContextForTest("operator:third"))
		if setErr != nil || thirdView.Setting.Revision != 3 {
			t.Fatalf("Set(third coordination) = %#v, error = %v, want revision three", thirdView, setErr)
		}

		var clockCalls atomic.Int64
		firstHasWriteLock := make(chan struct{})
		releaseFirst := make(chan struct{})
		globalDB.now = func() time.Time {
			call := clockCalls.Add(1)
			if call == 1 {
				close(firstHasWriteLock)
				<-releaseFirst
			}
			return firstTime.Add(time.Duration(call) * time.Minute)
		}
		type setResult struct {
			view compozyworkspace.CoordinationView
			err  error
		}
		firstResult := make(chan setResult, 1)
		secondResult := make(chan setResult, 1)
		waitForSetResult := func(name string, results <-chan setResult) setResult {
			t.Helper()
			select {
			case result := <-results:
				return result
			case <-time.After(5 * time.Second):
				t.Fatalf("timed out waiting for %s coordination result", name)
				return setResult{}
			}
		}
		go func() {
			view, callErr := commands.Set(ctx, compozyworkspace.SetCoordination{
				Ref: ref, Enabled: true, ExpectedRevision: 3,
			}, operatorActorContextForTest("operator:concurrent-first"))
			firstResult <- setResult{view: view, err: callErr}
		}()
		select {
		case <-firstHasWriteLock:
		case result := <-firstResult:
			t.Fatalf("first coordination writer returned before holding the write lock: %v", result.err)
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for first coordination writer to hold the write lock")
		}
		go func() {
			view, callErr := commands.Set(ctx, compozyworkspace.SetCoordination{
				Ref: ref, Enabled: false, ExpectedRevision: 3,
			}, operatorActorContextForTest("operator:concurrent-second"))
			secondResult <- setResult{view: view, err: callErr}
		}()
		close(releaseFirst)
		firstConcurrent := waitForSetResult("first", firstResult)
		if firstConcurrent.err != nil {
			t.Fatalf("Set(concurrent first) error = %v", firstConcurrent.err)
		}
		secondConcurrent := waitForSetResult("second", secondResult)
		if !errors.Is(secondConcurrent.err, compozyworkspace.ErrCoordinationConflict) {
			t.Fatalf("Set(concurrent second) error = %v, want revision conflict", secondConcurrent.err)
		}
		winnerView, getErr := commands.Get(ctx, ref, operatorActorContextForTest("operator:reader"))
		if getErr != nil {
			t.Fatalf("Get(concurrent winner) error = %v", getErr)
		}
		winner := winnerView.Setting
		if !winner.Enabled || winner.UpdatedBy != "operator:concurrent-first" || winner.Revision != 4 {
			t.Fatalf("Get(concurrent winner) = %#v, want sole first writer at revision four", winner)
		}
		summaries, summaryErr := globalDB.ListEventSummaries(
			ctx,
			EventSummaryQuery{ReadScope: store.ReadScope{AllProfiles: true},
				WorkspaceID: updated.ID,
				Type:        "network.coordination.setting_changed",
			},
		)
		if summaryErr != nil {
			t.Fatalf("ListEventSummaries(coordination) error = %v", summaryErr)
		}
		if len(summaries) != 4 {
			t.Fatalf("len(coordination summaries) = %d, want four committed winners", len(summaries))
		}
		latestSummary := summaries[len(summaries)-1]
		if latestSummary.ActorID != "operator:concurrent-first" {
			t.Fatalf("latest coordination actor = %q, want committed CAS winner", latestSummary.ActorID)
		}
	})

	if err := globalDB.DeleteWorkspace(testutil.Context(t), updated.ID); err != nil {
		t.Fatalf("DeleteWorkspace() error = %v", err)
	}
	if _, err := globalDB.GetWorkspace(
		testutil.Context(t),
		updated.ID,
	); !errors.Is(
		err,
		compozyworkspace.ErrWorkspaceNotFound,
	) {
		t.Fatalf("GetWorkspace(deleted) error = %v, want ErrWorkspaceNotFound", err)
	}
}

func TestGlobalDBDeleteWorkspaceCascadeDeletesStoppedSessions(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"workspace-delete-guard",
		filepath.Join(t.TempDir(), "workspace-delete-guard"),
	)
	if err := globalDB.RegisterSession(testutil.Context(t), SessionInfo{
		ProfileID:     store.DefaultProfileID,
		ID:            "sess-delete-guard",
		AgentName:     "coder",
		RuntimeStatus: store.SessionRuntimeUnbound,
		WorkspaceID:   workspaceID,
		State:         "stopped",
		CreatedAt:     time.Date(2026, 4, 3, 14, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, 4, 3, 14, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("RegisterSession() error = %v", err)
	}

	if err := globalDB.DeleteWorkspace(testutil.Context(t), workspaceID); err != nil {
		t.Fatalf("DeleteWorkspace() error = %v, want nil", err)
	}

	sessions, err := globalDB.ListSessions(testutil.Context(t), SessionListQuery{
		ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
		WorkspaceID: workspaceID,
	})
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("ListSessions() = %d sessions, want 0 (cascade delete)", len(sessions))
	}
}

func TestGlobalDBDeleteWorkspaceWithoutSessions(t *testing.T) {
	t.Parallel()

	t.Run("Should retain the complete deletion intent across reopen until finalization", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		copyCurrentSchemaGlobalDBSeed(t, path)
		globalDB := openGlobalDBForTest(t, path)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"ws-durable-deletion-intent",
			filepath.Join(t.TempDir(), "ws-durable-deletion-intent"),
		)
		workspace, err := globalDB.GetWorkspace(ctx, workspaceID)
		if err != nil {
			t.Fatalf("GetWorkspace() error = %v", err)
		}
		if err := globalDB.StageWorkspaceDeletion(ctx, workspaceID); err != nil {
			t.Fatalf("StageWorkspaceDeletion() error = %v", err)
		}
		if _, err := globalDB.GetWorkspace(ctx, workspaceID); !errors.Is(
			err, compozyworkspace.ErrWorkspaceNotFound,
		) {
			t.Fatalf("GetWorkspace(staged) error = %v, want ErrWorkspaceNotFound", err)
		}
		if err := globalDB.Close(ctx); err != nil {
			t.Fatalf("Close() error = %v", err)
		}

		reopened := openGlobalDBForTest(t, path)
		intent, err := reopened.GetWorkspaceDeletionIntent(ctx, workspaceID)
		if err != nil {
			t.Fatalf("GetWorkspaceDeletionIntent() error = %v", err)
		}
		if intent.Workspace.ID != workspace.ID || intent.Workspace.RootDir != workspace.RootDir ||
			intent.Workspace.Name != workspace.Name || intent.RequestedAt.IsZero() {
			t.Fatalf("deletion intent = %#v, want complete snapshot %#v", intent, workspace)
		}
		if err := reopened.InsertWorkspace(ctx, workspace); !errors.Is(
			err, compozyworkspace.ErrWorkspaceDeletionPending,
		) {
			t.Fatalf("InsertWorkspace(pending deletion) error = %v, want ErrWorkspaceDeletionPending", err)
		}
		if err := reopened.CompleteWorkspaceDeletion(ctx, workspaceID); err != nil {
			t.Fatalf("CompleteWorkspaceDeletion() error = %v", err)
		}
		if _, err := reopened.GetWorkspaceDeletionIntent(
			ctx, workspaceID,
		); !errors.Is(err, compozyworkspace.ErrWorkspaceDeletionIntentNotFound) {
			t.Fatalf("GetWorkspaceDeletionIntent(completed) error = %v, want not found", err)
		}
	})

	t.Run("Should delete a workspace without sessions", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"ws-no-sessions",
			filepath.Join(t.TempDir(), "ws-no-sessions"),
		)

		if err := globalDB.DeleteWorkspace(testutil.Context(t), workspaceID); err != nil {
			t.Fatalf("DeleteWorkspace() error = %v, want nil", err)
		}
		if _, err := globalDB.GetWorkspace(testutil.Context(t), workspaceID); !errors.Is(
			err,
			compozyworkspace.ErrWorkspaceNotFound,
		) {
			t.Fatalf("GetWorkspace(deleted) error = %v, want ErrWorkspaceNotFound", err)
		}
	})

	t.Run("Should delete scoped extension bindings and prevent same ID recovery", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"ws-extension-secret-delete",
			filepath.Join(t.TempDir(), "ws-extension-secret-delete"),
		)
		siblingID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"ws-extension-secret-sibling",
			filepath.Join(t.TempDir(), "ws-extension-secret-sibling"),
		)
		deletedWorkspace, err := globalDB.GetWorkspace(ctx, workspaceID)
		if err != nil {
			t.Fatalf("GetWorkspace(before extension binding delete) error = %v", err)
		}
		bindings := []extensionenv.Binding{
			{
				ExtensionName: "kit", WorkspaceID: workspaceID, EnvName: "API_KEY",
				SecretRef: vault.ExtensionSecretRef("kit", workspaceID, "API_KEY"), Kind: extensionenv.BindingKind,
			},
			{
				ExtensionName: "kit", WorkspaceID: siblingID, EnvName: "API_KEY",
				SecretRef: vault.ExtensionSecretRef("kit", siblingID, "API_KEY"), Kind: extensionenv.BindingKind,
			},
			{
				ExtensionName: "kit", EnvName: "API_KEY",
				SecretRef: vault.ExtensionSecretRef("kit", "", "API_KEY"), Kind: extensionenv.BindingKind,
			},
		}
		for _, binding := range bindings {
			if err := globalDB.PutEnvBinding(ctx, binding); err != nil {
				t.Fatalf("PutEnvBinding(%#v) error = %v", binding, err)
			}
		}

		if err := globalDB.DeleteWorkspace(ctx, workspaceID); err != nil {
			t.Fatalf("DeleteWorkspace() error = %v", err)
		}
		deleted, err := globalDB.ListEnvBindings(ctx, "kit", "", workspaceID)
		if err != nil || len(deleted) != 0 {
			t.Fatalf("deleted workspace bindings = %#v, %v; want empty", deleted, err)
		}
		for _, preservedWorkspaceID := range []string{"", siblingID} {
			preserved, listErr := globalDB.ListEnvBindings(ctx, "kit", "", preservedWorkspaceID)
			if listErr != nil || len(preserved) != 1 {
				t.Fatalf("preserved bindings for %q = %#v, %v; want one", preservedWorkspaceID, preserved, listErr)
			}
		}
		if err := globalDB.InsertWorkspace(ctx, deletedWorkspace); err != nil {
			t.Fatalf("InsertWorkspace(same ID) error = %v", err)
		}
		reused, err := globalDB.ListEnvBindings(ctx, "kit", "", workspaceID)
		if err != nil || len(reused) != 0 {
			t.Fatalf("reused workspace bindings = %#v, %v; want no recovered secrets", reused, err)
		}
		if err := globalDB.PutEnvBinding(ctx, bindings[0]); err != nil {
			t.Fatalf("PutEnvBinding(raw cascade fixture) error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(ctx, `DELETE FROM workspaces WHERE id = ?`, workspaceID); err != nil {
			t.Fatalf("raw workspace delete error = %v", err)
		}
		cascaded, err := globalDB.ListEnvBindings(ctx, "kit", "", workspaceID)
		if err != nil || len(cascaded) != 0 {
			t.Fatalf("raw-delete cascaded bindings = %#v, %v; want empty", cascaded, err)
		}
	})

	t.Run("Should delete only scoped MCP credentials and prevent same ID recovery", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"ws-oauth-delete",
			filepath.Join(t.TempDir(), "ws-oauth-delete"),
		)
		siblingID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"ws-oauth-sibling",
			filepath.Join(t.TempDir(), "ws-oauth-sibling"),
		)
		deletedWorkspace, err := globalDB.GetWorkspace(ctx, workspaceID)
		if err != nil {
			t.Fatalf("GetWorkspace(before delete) error = %v", err)
		}
		targets := []mcpauth.Target{
			{Scope: mcpauth.ScopeWorkspace, WorkspaceID: workspaceID, ServerName: "linear"},
			{Scope: mcpauth.ScopeWorkspace, WorkspaceID: siblingID, ServerName: "linear"},
			userMCPAuthTarget("linear"),
		}
		issuedAt := time.Date(2026, 7, 16, 18, 0, 0, 0, time.UTC)
		for index, target := range targets {
			if err := globalDB.SaveMCPAuthToken(ctx, mcpauth.TokenRecord{
				Target:                target,
				DefinitionFingerprint: testMCPDefinitionFingerprint,
				ClientID:              "client-id",
				AccessToken:           fmt.Sprintf("access-%d", index),
				RefreshToken:          fmt.Sprintf("refresh-%d", index),
				ObtainedAt:            issuedAt,
				UpdatedAt:             issuedAt,
			}); err != nil {
				t.Fatalf("SaveMCPAuthToken(%#v) error = %v", target, err)
			}
		}
		registrations := make([]mcpauth.ClientRegistration, 0, len(targets))
		for _, target := range targets {
			registration := mcpOAuthRegistrationRecord(t, target, issuedAt)
			saved, err := globalDB.SaveMCPAuthRegistration(ctx, registration, mcpOAuthRegistrationSecrets())
			if err != nil {
				t.Fatalf("SaveMCPAuthRegistration(%#v) error = %v", target, err)
			}
			registrations = append(registrations, saved)
		}
		var accessRef, refreshRef string
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT access_token_ref, refresh_token_ref FROM mcp_auth_tokens
			 WHERE scope = ? AND workspace_id = ? AND server_name = ?`,
			string(mcpauth.ScopeWorkspace),
			workspaceID,
			"linear",
		).Scan(&accessRef, &refreshRef); err != nil {
			t.Fatalf("query deleted workspace token refs error = %v", err)
		}

		if err := globalDB.DeleteWorkspace(ctx, workspaceID); err != nil {
			t.Fatalf("DeleteWorkspace() error = %v", err)
		}
		if _, err := globalDB.GetMCPAuthToken(ctx, targets[0]); !errors.Is(err, mcpauth.ErrTokenNotFound) {
			t.Fatalf("GetMCPAuthToken(deleted workspace) error = %v, want ErrTokenNotFound", err)
		}
		for _, ref := range []string{accessRef, refreshRef} {
			assertVaultRefPresence(ctx, t, globalDB.db, ref, false)
		}
		if _, err := globalDB.GetMCPAuthRegistration(ctx, targets[0]); !errors.Is(
			err,
			mcpauth.ErrRegistrationNotFound,
		) {
			t.Fatalf("GetMCPAuthRegistration(deleted workspace) error = %v, want ErrRegistrationNotFound", err)
		}
		for _, ref := range []string{
			registrations[0].ClientSecretRef,
			registrations[0].RegistrationAccessTokenRef,
		} {
			assertVaultRefPresence(ctx, t, globalDB.db, ref, false)
		}
		for _, target := range targets[1:] {
			if _, err := globalDB.GetMCPAuthToken(ctx, target); err != nil {
				t.Fatalf("GetMCPAuthToken(preserved %#v) error = %v", target, err)
			}
		}
		for _, target := range targets[1:] {
			if _, err := globalDB.GetMCPAuthRegistration(ctx, target); err != nil {
				t.Fatalf("GetMCPAuthRegistration(preserved %#v) error = %v", target, err)
			}
		}

		if err := globalDB.InsertWorkspace(ctx, deletedWorkspace); err != nil {
			t.Fatalf("InsertWorkspace(same ID) error = %v", err)
		}
		if _, err := globalDB.GetMCPAuthToken(ctx, targets[0]); !errors.Is(err, mcpauth.ErrTokenNotFound) {
			t.Fatalf("GetMCPAuthToken(re-registered workspace) error = %v, want ErrTokenNotFound", err)
		}
	})

	t.Run("Should roll back workspace deletion when MCP secret cleanup fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"ws-oauth-rollback",
			filepath.Join(t.TempDir(), "ws-oauth-rollback"),
		)
		target := mcpauth.Target{
			Scope: mcpauth.ScopeWorkspace, WorkspaceID: workspaceID, ServerName: "linear",
		}
		issuedAt := time.Date(2026, 7, 16, 18, 30, 0, 0, time.UTC)
		if err := globalDB.SaveMCPAuthToken(ctx, mcpauth.TokenRecord{
			Target:                target,
			DefinitionFingerprint: testMCPDefinitionFingerprint,
			ClientID:              "client-id",
			AccessToken:           "access-token",
			RefreshToken:          "refresh-token",
			ObtainedAt:            issuedAt,
			UpdatedAt:             issuedAt,
		}); err != nil {
			t.Fatalf("SaveMCPAuthToken() error = %v", err)
		}
		registration, err := globalDB.SaveMCPAuthRegistration(
			ctx,
			mcpOAuthRegistrationRecord(t, target, issuedAt),
			mcpOAuthRegistrationSecrets(),
		)
		if err != nil {
			t.Fatalf("SaveMCPAuthRegistration() error = %v", err)
		}
		var accessRef, refreshRef string
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT access_token_ref, refresh_token_ref FROM mcp_auth_tokens
			 WHERE scope = ? AND workspace_id = ? AND server_name = ?`,
			string(mcpauth.ScopeWorkspace),
			workspaceID,
			"linear",
		).Scan(&accessRef, &refreshRef); err != nil {
			t.Fatalf("query rollback token refs error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`CREATE TRIGGER fail_workspace_mcp_dcr_secret_cleanup
			 BEFORE DELETE ON vault_secrets
			 WHEN OLD.kind = 'mcp_oauth_dcr_client_secret'
			 BEGIN
			   SELECT RAISE(ABORT, 'forced MCP DCR secret cleanup failure');
			 END`,
		); err != nil {
			t.Fatalf("create cleanup failure trigger error = %v", err)
		}

		err = globalDB.DeleteWorkspace(ctx, workspaceID)
		if err == nil || !strings.Contains(err.Error(), "forced MCP DCR secret cleanup failure") {
			t.Fatalf("DeleteWorkspace() error = %v, want forced cleanup failure", err)
		}
		if _, err := globalDB.GetWorkspace(ctx, workspaceID); err != nil {
			t.Fatalf("GetWorkspace(after rollback) error = %v", err)
		}
		if _, err := globalDB.GetMCPAuthToken(ctx, target); err != nil {
			t.Fatalf("GetMCPAuthToken(after rollback) error = %v", err)
		}
		if _, err := globalDB.GetMCPAuthRegistration(ctx, target); err != nil {
			t.Fatalf("GetMCPAuthRegistration(after rollback) error = %v", err)
		}
		for _, ref := range []string{accessRef, refreshRef} {
			assertVaultRefPresence(ctx, t, globalDB.db, ref, true)
		}
		for _, ref := range []string{registration.ClientSecretRef, registration.RegistrationAccessTokenRef} {
			assertVaultRefPresence(ctx, t, globalDB.db, ref, true)
		}
	})
}

func TestGlobalDBDeleteWorkspaceRejectsActiveSessions(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"ws-active-sessions",
		filepath.Join(t.TempDir(), "ws-active-sessions"),
	)
	if err := globalDB.RegisterSession(testutil.Context(t), SessionInfo{
		ProfileID:     store.DefaultProfileID,
		ID:            "sess-active-guard",
		AgentName:     "coder",
		RuntimeStatus: store.SessionRuntimeUnbound,
		WorkspaceID:   workspaceID,
		State:         "active",
		CreatedAt:     time.Date(2026, 4, 3, 14, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, 4, 3, 14, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("RegisterSession() error = %v", err)
	}

	err := globalDB.StageWorkspaceDeletion(testutil.Context(t), workspaceID)
	if !errors.Is(err, compozyworkspace.ErrWorkspaceHasActiveSessions) {
		t.Fatalf("StageWorkspaceDeletion() error = %v, want ErrWorkspaceHasActiveSessions", err)
	}
	if _, err := globalDB.GetWorkspace(testutil.Context(t), workspaceID); err != nil {
		t.Fatalf("GetWorkspace(after rejected stage) error = %v", err)
	}
	if _, err := globalDB.GetWorkspaceDeletionIntent(
		testutil.Context(t), workspaceID,
	); !errors.Is(err, compozyworkspace.ErrWorkspaceDeletionIntentNotFound) {
		t.Fatalf("GetWorkspaceDeletionIntent(after rejected stage) error = %v, want not found", err)
	}
}

func TestGlobalDBWorkspaceConstraintViolations(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	rootA := filepath.Join(t.TempDir(), "root-a")
	rootB := filepath.Join(t.TempDir(), "root-b")
	if err := os.MkdirAll(rootA, 0o755); err != nil {
		t.Fatalf("MkdirAll(rootA) error = %v", err)
	}
	if err := os.MkdirAll(rootB, 0o755); err != nil {
		t.Fatalf("MkdirAll(rootB) error = %v", err)
	}

	base := compozyworkspace.Workspace{
		ID:        "ws-base",
		RootDir:   rootA,
		Name:      "alpha",
		CreatedAt: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
	}
	if err := globalDB.InsertWorkspace(testutil.Context(t), base); err != nil {
		t.Fatalf("InsertWorkspace(base) error = %v", err)
	}

	tests := []struct {
		name string
		ws   compozyworkspace.Workspace
		want error
	}{
		{
			name: "Should reject a duplicate root directory",
			ws: compozyworkspace.Workspace{
				ID:        "ws-duplicate-root",
				RootDir:   rootA,
				Name:      "beta",
				CreatedAt: base.CreatedAt,
				UpdatedAt: base.UpdatedAt,
			},
			want: compozyworkspace.ErrWorkspacePathTaken,
		},
		{
			name: "Should reject a duplicate name",
			ws: compozyworkspace.Workspace{
				ID:        "ws-duplicate-name",
				RootDir:   rootB,
				Name:      "alpha",
				CreatedAt: base.CreatedAt,
				UpdatedAt: base.UpdatedAt,
			},
			want: compozyworkspace.ErrWorkspaceNameTaken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := globalDB.InsertWorkspace(testutil.Context(t), tt.ws)
			if !errors.Is(err, tt.want) {
				t.Fatalf("InsertWorkspace() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestGlobalDBWorkspaceNotFoundErrors(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)

	if _, err := globalDB.GetWorkspace(
		testutil.Context(t),
		"ws-missing",
	); !errors.Is(
		err,
		compozyworkspace.ErrWorkspaceNotFound,
	) {
		t.Fatalf("GetWorkspace(missing) error = %v, want ErrWorkspaceNotFound", err)
	}
	if _, err := globalDB.GetWorkspaceByPath(
		testutil.Context(t),
		filepath.Join(t.TempDir(), "missing"),
	); !errors.Is(
		err,
		compozyworkspace.ErrWorkspaceNotFound,
	) {
		t.Fatalf("GetWorkspaceByPath(missing) error = %v, want ErrWorkspaceNotFound", err)
	}
	if _, err := globalDB.GetWorkspaceByName(
		testutil.Context(t),
		"missing",
	); !errors.Is(
		err,
		compozyworkspace.ErrWorkspaceNotFound,
	) {
		t.Fatalf("GetWorkspaceByName(missing) error = %v, want ErrWorkspaceNotFound", err)
	}
	if err := globalDB.UpdateWorkspace(testutil.Context(t), compozyworkspace.Workspace{
		ID:        "ws-missing",
		RootDir:   filepath.Join(t.TempDir(), "missing"),
		Name:      "missing",
		UpdatedAt: time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
	}); !errors.Is(err, compozyworkspace.ErrWorkspaceNotFound) {
		t.Fatalf("UpdateWorkspace(missing) error = %v, want ErrWorkspaceNotFound", err)
	}
	if err := globalDB.DeleteWorkspace(
		testutil.Context(t),
		"ws-missing",
	); !errors.Is(
		err,
		compozyworkspace.ErrWorkspaceNotFound,
	) {
		t.Fatalf("DeleteWorkspace(missing) error = %v, want ErrWorkspaceNotFound", err)
	}
}

func TestGlobalDBWorkspaceValidationAndDefaulting(t *testing.T) {
	t.Parallel()

	t.Run("Should not persist a workspace when generated ID entropy fails", func(t *testing.T) {
		t.Parallel()

		entropyErr := errors.New("entropy unavailable")
		globalDB := openTestGlobalDB(t)
		globalDB.generateID = func() (string, error) {
			return "", entropyErr
		}
		rootDir := filepath.Join(t.TempDir(), "workspace-entropy-failure")
		if err := os.MkdirAll(rootDir, 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}

		err := globalDB.InsertWorkspace(testutil.Context(t), compozyworkspace.Workspace{
			RootDir: rootDir,
			Name:    "entropy-failure",
		})
		if !errors.Is(err, entropyErr) {
			t.Fatalf("InsertWorkspace() error = %v, want %v", err, entropyErr)
		}
		workspaces, listErr := globalDB.ListWorkspaces(testutil.Context(t))
		if listErr != nil {
			t.Fatalf("ListWorkspaces() error = %v", listErr)
		}
		if len(workspaces) != 0 {
			t.Fatalf("len(workspaces) = %d, want 0", len(workspaces))
		}
	})

	var nilCtx context.Context
	if _, err := OpenGlobalDB(nilCtx, filepath.Join(t.TempDir(), GlobalDatabaseName)); err == nil {
		t.Fatal("OpenGlobalDB(nil) error = nil, want non-nil")
	}

	var nilGlobalDB *GlobalDB
	if got := nilGlobalDB.Path(); got != "" {
		t.Fatalf("(*GlobalDB)(nil).Path() = %q, want empty", got)
	}

	globalDB := openTestGlobalDB(t)
	rootDir := filepath.Join(t.TempDir(), "workspace-defaulted")
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	if err := globalDB.InsertWorkspace(testutil.Context(t), compozyworkspace.Workspace{
		RootDir: rootDir,
		Name:    "defaulted",
	}); err != nil {
		t.Fatalf("InsertWorkspace(defaulted) error = %v", err)
	}

	workspaces, err := globalDB.ListWorkspaces(testutil.Context(t))
	if err != nil {
		t.Fatalf("ListWorkspaces() error = %v", err)
	}
	if got, want := len(workspaces), 1; got != want {
		t.Fatalf("len(workspaces) = %d, want %d", got, want)
	}
	if !compozyworkspace.IsWorkspaceID(workspaces[0].ID) {
		t.Fatalf("workspaces[0].ID = %q, want workspace_id ULID", workspaces[0].ID)
	}
	if workspaces[0].CreatedAt.IsZero() || workspaces[0].UpdatedAt.IsZero() {
		t.Fatalf("workspace timestamps = %#v, want non-zero", workspaces[0])
	}

	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "insert missing root",
			run: func() error {
				return globalDB.InsertWorkspace(testutil.Context(t), compozyworkspace.Workspace{Name: "missing-root"})
			},
		},
		{
			name: "insert missing name",
			run: func() error {
				return globalDB.InsertWorkspace(testutil.Context(t), compozyworkspace.Workspace{RootDir: rootDir})
			},
		},
		{
			name: "update missing id",
			run: func() error {
				return globalDB.UpdateWorkspace(
					testutil.Context(t),
					compozyworkspace.Workspace{RootDir: rootDir, Name: "missing-id"},
				)
			},
		},
		{
			name: "delete missing id",
			run: func() error {
				return globalDB.DeleteWorkspace(testutil.Context(t), "")
			},
		},
		{
			name: "get missing id",
			run: func() error {
				_, err := globalDB.GetWorkspace(testutil.Context(t), "")
				return err
			},
		},
		{
			name: "get by missing path",
			run: func() error {
				_, err := globalDB.GetWorkspaceByPath(testutil.Context(t), "")
				return err
			},
		},
		{
			name: "get by missing name",
			run: func() error {
				_, err := globalDB.GetWorkspaceByName(testutil.Context(t), "")
				return err
			},
		},
		{
			name: "list nil context",
			run: func() error {
				var nilCtx context.Context
				_, err := globalDB.ListWorkspaces(nilCtx)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.run(); err == nil {
				t.Fatal("error = nil, want non-nil")
			}
		})
	}
}

func TestGlobalDBListWorkspacesStableOrder(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	first := insertWorkspaceForGlobalTests(t, globalDB, compozyworkspace.Workspace{
		ID:        "ws-zeta",
		RootDir:   filepath.Join(t.TempDir(), "workspace-zeta"),
		Name:      "zeta",
		CreatedAt: time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC),
	})
	second := insertWorkspaceForGlobalTests(t, globalDB, compozyworkspace.Workspace{
		ID:        "ws-alpha",
		RootDir:   filepath.Join(t.TempDir(), "workspace-alpha"),
		Name:      "alpha",
		CreatedAt: time.Date(2026, 4, 3, 10, 1, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 3, 10, 1, 0, 0, time.UTC),
	})

	workspaces, err := globalDB.ListWorkspaces(testutil.Context(t))
	if err != nil {
		t.Fatalf("ListWorkspaces() error = %v", err)
	}

	if got, want := len(workspaces), 2; got != want {
		t.Fatalf("len(workspaces) = %d, want %d", got, want)
	}
	assertWorkspaceEqual(t, workspaces[0], second)
	assertWorkspaceEqual(t, workspaces[1], first)
}

func TestGlobalDBRegisterAndListSessionsUseWorkspaceID(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"session-workspace",
		filepath.Join(t.TempDir(), "session-workspace"),
	)

	session := SessionInfo{
		ProfileID:           store.DefaultProfileID,
		ID:                  "sess-workspace-id",
		AgentName:           "coder",
		RuntimeStatus:       store.SessionRuntimeUnbound,
		WorkspaceID:         workspaceID,
		SessionNetworkState: &store.SessionNetworkState{NetworkSpec: participation.LocalSpec()},
		State:               "active",
		Liveness: &store.SessionLivenessMeta{
			SubprocessPID: 77,
			LastUpdateAt:  ptrTime(time.Date(2026, 4, 3, 13, 1, 0, 0, time.UTC)),
			StallState:    store.SessionStallStateDetected,
			StallReason:   store.SessionStallReasonActivityTimeout,
		},
		Sandbox: &store.SessionSandboxMeta{
			SandboxID:     "env-workspace-id",
			Backend:       "local",
			Profile:       "local",
			State:         "prepared",
			InstanceID:    "instance-workspace-id",
			ProviderState: []byte(`{"provider":true}`),
			LastSyncError: "last sync failed",
		},
		CreatedAt: time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
	}
	if err := globalDB.RegisterSession(testutil.Context(t), session); err != nil {
		t.Fatalf("RegisterSession() error = %v", err)
	}

	sessions, err := globalDB.ListSessions(testutil.Context(t), SessionListQuery{
		ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID},
	})
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if got, want := len(sessions), 1; got != want {
		t.Fatalf("len(sessions) = %d, want %d", got, want)
	}
	if got, want := sessions[0].WorkspaceID, workspaceID; got != want {
		t.Fatalf("sessions[0].WorkspaceID = %q, want %q", got, want)
	}
	if got, want := sessions[0].NetworkSpecSnapshot().ChannelID, ""; got != want {
		t.Fatalf("sessions[0] Network channel = %q, want %q", got, want)
	}
	if got, want := sessions[0].NetworkSpec, participation.LocalSpec(); got != want {
		t.Fatalf("sessions[0].NetworkSpec = %#v, want %#v", got, want)
	}
	if sessions[0].Sandbox == nil {
		t.Fatal("sessions[0].Sandbox = nil, want sandbox metadata")
	}
	if got, want := sessions[0].Sandbox.SandboxID, "env-workspace-id"; got != want {
		t.Fatalf("sessions[0].Sandbox.SandboxID = %q, want %q", got, want)
	}
	if got, want := sessions[0].Sandbox.InstanceID, "instance-workspace-id"; got != want {
		t.Fatalf("sessions[0].Sandbox.InstanceID = %q, want %q", got, want)
	}
	if got, want := sessions[0].Sandbox.LastSyncError, "last sync failed"; got != want {
		t.Fatalf("sessions[0].Sandbox.LastSyncError = %q, want %q", got, want)
	}
	if sessions[0].Liveness == nil {
		t.Fatal("sessions[0].Liveness = nil, want liveness metadata")
	}
	if got, want := sessions[0].Liveness.SubprocessPID, 77; got != want {
		t.Fatalf("sessions[0].Liveness.SubprocessPID = %d, want %d", got, want)
	}
	if sessions[0].Liveness.LastUpdateAt == nil ||
		!sessions[0].Liveness.LastUpdateAt.Equal(*session.Liveness.LastUpdateAt) {
		t.Fatalf(
			"sessions[0].Liveness.LastUpdateAt = %#v, want %s",
			sessions[0].Liveness.LastUpdateAt,
			session.Liveness.LastUpdateAt,
		)
	}
	if got, want := sessions[0].Liveness.StallState, store.SessionStallStateDetected; got != want {
		t.Fatalf("sessions[0].Liveness.StallState = %q, want %q", got, want)
	}
	if got, want := sessions[0].Liveness.StallReason, store.SessionStallReasonActivityTimeout; got != want {
		t.Fatalf("sessions[0].Liveness.StallReason = %q, want %q", got, want)
	}

	t.Run("Should expose the complete sessions schema", func(t *testing.T) {
		t.Parallel()

		assertTableColumns(
			t,
			globalDB.db,
			"sessions",
			[]string{
				"id",
				"profile_id",
				"name",
				"agent_name",
				"provider",
				"model",
				"reasoning_effort",
				"speed",
				"acp_options_json",
				"speed_resolution_json",
				"runtime_status",
				"runtime_transition",
				"runtime_failure",
				"runtime_generation",
				"runtime_recovery_json",
				"selected_provider",
				"selected_model",
				"selected_reasoning_effort",
				"selected_speed",
				"selected_acp_options_json",
				"runtime_selection_revision",
				"workspace_id",
				"scope",
				"worktree_id",
				"session_type",
				"state",
				"archived_at",
				"acp_session_id",
				"stop_reason",
				"stop_detail",
				"subprocess_pid",
				"subprocess_started_at",
				"last_update_at",
				"stall_state",
				"stall_reason",
				"activity_json",
				"attached_to",
				"attach_expires_at",
				"transcript_epoch",
				"pending_permission_count",
				"pending_clarify_count",
				"attention_revision",
				"last_settled_revision",
				"last_seen_revision",
				"last_seen_at",
				"attention_changed_at",
				"sandbox_id",
				"sandbox_backend",
				"sandbox_profile",
				"sandbox_instance_id",
				"sandbox_state",
				"sandbox_provider_state_json",
				"sandbox_last_sync_at",
				"sandbox_last_sync_error",
				"created_at",
				"updated_at",
				"failure_kind",
				"failure_summary",
				"crash_bundle_path",
				"parent_session_id",
				"root_session_id",
				"spawn_depth",
				"spawn_role",
				"ttl_expires_at",
				"auto_stop_on_parent",
				"notify_creator",
				"spawn_budget_json",
				"permission_policy_json",
				"soul_snapshot_id",
				"soul_digest",
				"parent_soul_digest",
				sessionInputGenerationColumn,
				"creation_digest",
				"policy_spec_digest",
				"creation_profile_ref",
				"network_spec_json",
				"network_mode",
				"network_channel",
				"network_source",
			},
		)
	})
}

func TestGlobalDBRegisterSessionRejectsStallStateWithoutReason(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"invalid-stall-session",
		filepath.Join(t.TempDir(), "invalid-stall-session"),
	)

	err := globalDB.RegisterSession(testutil.Context(t), SessionInfo{
		ProfileID:     store.DefaultProfileID,
		ID:            "sess-invalid-stall",
		AgentName:     "coder",
		RuntimeStatus: store.SessionRuntimeUnbound,
		WorkspaceID:   workspaceID,
		State:         "active",
		Liveness: &store.SessionLivenessMeta{
			SubprocessPID: 77,
			StallState:    store.SessionStallStateDetected,
		},
		CreatedAt: time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("RegisterSession() error = nil, want invalid stall reason failure")
	}
	if got, want := err.Error(), "store: session stall reason required when stall state is set"; !strings.Contains(
		got,
		want,
	) {
		t.Fatalf("RegisterSession() error = %v, want substring %q", err, want)
	}
}

func TestGlobalDBRegisterSessionRejectsUnmarshalableActivity(t *testing.T) {
	t.Run("Should reject unmarshalable session activity without writing a row", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"invalid-activity-session",
			filepath.Join(t.TempDir(), "invalid-activity-session"),
		)
		unmarshalableTime := time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)

		err := globalDB.RegisterSession(testutil.Context(t), SessionInfo{
			ProfileID:     store.DefaultProfileID,
			ID:            "sess-invalid-activity",
			AgentName:     "coder",
			RuntimeStatus: store.SessionRuntimeUnbound,
			WorkspaceID:   workspaceID,
			State:         "active",
			Liveness: &store.SessionLivenessMeta{
				Activity: &store.SessionActivityMeta{
					TurnStartedAt: &unmarshalableTime,
				},
			},
			CreatedAt: time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
		})
		if err == nil {
			t.Fatal("RegisterSession(unmarshalable activity) error = nil, want marshal failure")
		}
		if !strings.Contains(err.Error(), "store: session liveness activity marshal") {
			t.Fatalf("RegisterSession(unmarshalable activity) error = %v, want activity marshal context", err)
		}

		sessions, err := globalDB.ListSessions(testutil.Context(t), SessionListQuery{
			ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID},
		})
		if err != nil {
			t.Fatalf("ListSessions() error = %v", err)
		}
		if len(sessions) != 0 {
			t.Fatalf("len(sessions) = %d, want failed register to skip write", len(sessions))
		}
	})
}

func TestGlobalDBWriteEventSummary(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	registerSessionForGlobalTests(t, globalDB, "sess-summary")

	if err := globalDB.WriteEventSummary(testutil.Context(t), EventSummary{
		ProfileID:     store.DefaultProfileID,
		SessionID:     "sess-summary",
		Type:          "agent_message",
		AgentName:     "coder",
		RootSessionID: "sess-summary",
		Summary:       "assistant replied",
		Timestamp:     time.Date(2026, 4, 3, 14, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("WriteEventSummary() error = %v", err)
	}

	summaries, err := globalDB.ListEventSummaries(
		testutil.Context(t),
		EventSummaryQuery{ReadScope: store.ReadScope{AllProfiles: true}, SessionID: "sess-summary"},
	)
	if err != nil {
		t.Fatalf("ListEventSummaries() error = %v", err)
	}
	if got, want := len(summaries), 1; got != want {
		t.Fatalf("len(summaries) = %d, want %d", got, want)
	}
	if summaries[0].Summary != "assistant replied" {
		t.Fatalf("summaries[0].Summary = %q, want %q", summaries[0].Summary, "assistant replied")
	}
	if got, want := summaries[0].RootSessionID, "sess-summary"; got != want {
		t.Fatalf("summaries[0].RootSessionID = %q, want %q", got, want)
	}
	if got, want := summaries[0].Provider, "claude"; got != want {
		t.Fatalf("summaries[0].Provider = %q, want %q", got, want)
	}
	if got, want := summaries[0].Outcome, "info"; got != want {
		t.Fatalf("summaries[0].Outcome = %q, want %q", got, want)
	}

	providerFiltered, err := globalDB.ListEventSummaries(
		testutil.Context(t),
		EventSummaryQuery{ReadScope: store.ReadScope{AllProfiles: true},
			Provider:  "claude",
			Component: "session",
		},
	)
	if err != nil {
		t.Fatalf("ListEventSummaries(provider component) error = %v", err)
	}
	if got, want := len(providerFiltered), 1; got != want {
		t.Fatalf("len(providerFiltered) = %d, want %d", got, want)
	}

	if err := globalDB.WriteEventSummary(testutil.Context(t), EventSummary{
		ProfileID: store.DefaultProfileID,
		SessionID: "sess-summary",
		Type:      "task.run_failed",
		AgentName: "coder",
		Summary:   "task run failed",
		Timestamp: time.Date(2026, 4, 3, 14, 1, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("WriteEventSummary(failed task run) error = %v", err)
	}
	errorOnly, err := globalDB.ListEventSummaries(
		testutil.Context(t),
		EventSummaryQuery{ReadScope: store.ReadScope{AllProfiles: true},
			Provider:  "claude",
			Component: "task",
			ErrorOnly: true,
		},
	)
	if err != nil {
		t.Fatalf("ListEventSummaries(error only) error = %v", err)
	}
	if got, want := len(errorOnly), 1; got != want {
		t.Fatalf("len(errorOnly) = %d, want %d", got, want)
	}
	if got, want := errorOnly[0].Outcome, "failure"; got != want {
		t.Fatalf("errorOnly[0].Outcome = %q, want %q", got, want)
	}

	for _, testCase := range []struct {
		name    string
		summary EventSummary
	}{
		{
			name: "workspace",
			summary: EventSummary{
				ProfileID: store.DefaultProfileID,
				SessionID: "sess-summary", WorkspaceID: "different-workspace",
				Type: "agent_message", AgentName: "coder", Summary: "invalid workspace",
			},
		},
		{
			name: "provider",
			summary: EventSummary{
				ProfileID: store.DefaultProfileID,
				SessionID: "sess-summary", Provider: "different-provider",
				Type: "agent_message", AgentName: "coder", Summary: "invalid provider",
			},
		},
		{
			name: "worktree",
			summary: EventSummary{
				ProfileID: store.DefaultProfileID,
				SessionID: "sess-summary", EventCorrelation: store.EventCorrelation{WorktreeID: "different-worktree"},
				Type: "agent_message", AgentName: "coder", Summary: "invalid worktree",
			},
		},
	} {
		t.Run("Should reject conflicting session "+testCase.name+" projections", func(t *testing.T) {
			err := globalDB.WriteEventSummary(testutil.Context(t), testCase.summary)
			if err == nil || !strings.Contains(err.Error(), testCase.name) {
				t.Fatalf("WriteEventSummary(conflicting %s) error = %v", testCase.name, err)
			}
		})
	}
}

func TestGlobalDBWriteEventSummariesAtomic(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	timestamp := time.Date(2026, 8, 2, 18, 0, 0, 0, time.UTC)
	err := globalDB.WriteEventSummaries(testutil.Context(t), []EventSummary{
		{
			ProfileID: store.DefaultProfileID,
			ID:        "duplicate-summary",
			Type:      "settings.changed",
			Summary:   "first",
			Timestamp: timestamp,
		},
		{
			ProfileID: store.DefaultProfileID,
			ID:        "duplicate-summary",
			Type:      "settings.changed",
			Summary:   "second",
			Timestamp: timestamp,
		},
	})
	if !isSQLitePrimaryKeyConstraint(err) {
		t.Fatalf("WriteEventSummaries(duplicate id) error = %v, want primary-key constraint", err)
	}
	summaries, listErr := globalDB.ListEventSummaries(
		testutil.Context(t),
		EventSummaryQuery{ReadScope: store.ReadScope{AllProfiles: true}},
	)
	if listErr != nil {
		t.Fatalf("ListEventSummaries() error = %v", listErr)
	}
	if len(summaries) != 0 {
		t.Fatalf("ListEventSummaries() = %#v, want no partial batch writes", summaries)
	}
}

func TestGlobalDBWriteEventSummaryAllowsGlobalEvents(t *testing.T) {
	t.Parallel()

	const skillShadowedBody = "" +
		`{"skill_name":"review","old_source":"workspace","new_source":"agent-local",` +
		`"layer_pair":"agent-local>workspace","shadow_kind":"logical_path",` +
		`"resolution_scope":"agent","agent_name":"reviewer"}`

	tests := []struct {
		name      string
		summary   EventSummary
		wantType  string
		wantAgent string
		wantBody  string
	}{
		{
			name: "Should persist settings changed events with content",
			summary: eventSummaryWithContent(EventSummary{
				ProfileID: store.DefaultProfileID,
				Type:      "settings.changed",
				Summary:   "skills settings changed",
				Timestamp: time.Date(2026, 5, 4, 14, 5, 0, 0, time.UTC),
			}, json.RawMessage(`{"section":"skills","source":"http","operation":"patch"}`)),
			wantType: "settings.changed",
			wantBody: `{"section":"skills","source":"http","operation":"patch"}`,
		},
		{
			name: "Should persist skill shadowed events without a session",
			summary: eventSummaryWithContent(EventSummary{
				ProfileID: store.DefaultProfileID,
				Type:      "skill.shadowed",
				AgentName: "reviewer",
				Summary:   "skill review shadowed workspace with agent-local",
				Timestamp: time.Date(2026, 5, 4, 14, 6, 0, 0, time.UTC),
			}, json.RawMessage(skillShadowedBody)),
			wantType:  "skill.shadowed",
			wantAgent: "reviewer",
			wantBody:  skillShadowedBody,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			globalDB := openTestGlobalDB(t)

			if err := globalDB.WriteEventSummary(testutil.Context(t), tt.summary); err != nil {
				t.Fatalf("WriteEventSummary(global event) error = %v", err)
			}

			summaries, err := globalDB.ListEventSummaries(
				testutil.Context(t),
				EventSummaryQuery{ReadScope: store.ReadScope{AllProfiles: true}},
			)
			if err != nil {
				t.Fatalf("ListEventSummaries() error = %v", err)
			}
			if got, want := len(summaries), 1; got != want {
				t.Fatalf("len(summaries) = %d, want %d", got, want)
			}
			if got := summaries[0].SessionID; got != "" {
				t.Fatalf("summaries[0].SessionID = %q, want empty for global event", got)
			}
			if got, want := summaries[0].Type, tt.wantType; got != want {
				t.Fatalf("summaries[0].Type = %q, want %q", got, want)
			}
			if got, want := summaries[0].AgentName, tt.wantAgent; got != want {
				t.Fatalf("summaries[0].AgentName = %q, want %q", got, want)
			}
			if got, want := string(
				summaries[0].ContentValue(),
			), tt.wantBody; got != want {
				t.Fatalf("summaries[0].Content = %q, want %q", got, want)
			}
		})
	}
}

func TestGlobalDBUpdateTokenStatsAggregation(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	registerSessionForGlobalTests(t, globalDB, "sess-stats")

	currency := "USD"
	inputA := int64(10)
	outputA := int64(20)
	totalA := int64(30)
	costA := 1.25
	if err := globalDB.UpdateTokenStats(testutil.Context(t), TokenStatsUpdate{
		SessionID:    "sess-stats",
		AgentName:    "coder",
		InputTokens:  &inputA,
		OutputTokens: &outputA,
		TotalTokens:  &totalA,
		CostAmount:   &costA,
		CostCurrency: &currency,
		CostStatus:   "actual",
		CostSource:   "agent_reported",
		Turns:        1,
	}); err != nil {
		t.Fatalf("UpdateTokenStats() error = %v", err)
	}

	outputB := int64(5)
	totalB := int64(5)
	costB := 0.75
	if err := globalDB.UpdateTokenStats(testutil.Context(t), TokenStatsUpdate{
		SessionID:    "sess-stats",
		AgentName:    "coder",
		OutputTokens: &outputB,
		TotalTokens:  &totalB,
		CostAmount:   &costB,
		CostCurrency: &currency,
		CostStatus:   "actual",
		CostSource:   "agent_reported",
		Turns:        1,
	}); err != nil {
		t.Fatalf("UpdateTokenStats() error = %v", err)
	}

	stats, err := globalDB.ListTokenStats(testutil.Context(t), TokenStatsQuery{SessionID: "sess-stats"})
	if err != nil {
		t.Fatalf("ListTokenStats() error = %v", err)
	}
	if got, want := len(stats), 1; got != want {
		t.Fatalf("len(stats) = %d, want %d", got, want)
	}
	if stats[0].InputTokens == nil || *stats[0].InputTokens != 10 {
		t.Fatalf("InputTokens = %#v, want 10", stats[0].InputTokens)
	}
	if stats[0].OutputTokens == nil || *stats[0].OutputTokens != 25 {
		t.Fatalf("OutputTokens = %#v, want 25", stats[0].OutputTokens)
	}
	if stats[0].TotalTokens == nil || *stats[0].TotalTokens != 35 {
		t.Fatalf("TotalTokens = %#v, want 35", stats[0].TotalTokens)
	}
	if stats[0].TotalCost == nil || *stats[0].TotalCost != 2.0 {
		t.Fatalf("TotalCost = %#v, want 2.0", stats[0].TotalCost)
	}
	if stats[0].CostCurrency == nil || *stats[0].CostCurrency != "USD" {
		t.Fatalf("CostCurrency = %#v, want USD", stats[0].CostCurrency)
	}
	if stats[0].CostStatus != "actual" || stats[0].CostSource != "agent_reported" {
		t.Fatalf("cost provenance = %q/%q, want actual/agent_reported", stats[0].CostStatus, stats[0].CostSource)
	}
	if stats[0].TurnCount != 2 {
		t.Fatalf("TurnCount = %d, want 2", stats[0].TurnCount)
	}

	outputC := int64(1)
	totalC := int64(1)
	costC := 0.05
	if err := globalDB.UpdateTokenStats(testutil.Context(t), TokenStatsUpdate{
		SessionID:    "sess-stats",
		AgentName:    "coder",
		OutputTokens: &outputC,
		TotalTokens:  &totalC,
		CostAmount:   &costC,
		CostCurrency: &currency,
		CostStatus:   "estimated",
		CostSource:   "catalog_config",
		Turns:        1,
	}); err != nil {
		t.Fatalf("UpdateTokenStats(conflicting provenance) error = %v", err)
	}
	stats, err = globalDB.ListTokenStats(testutil.Context(t), TokenStatsQuery{SessionID: "sess-stats"})
	if err != nil {
		t.Fatalf("ListTokenStats(after provenance conflict) error = %v", err)
	}
	if stats[0].TotalTokens == nil || *stats[0].TotalTokens != 36 || stats[0].TurnCount != 3 {
		t.Fatalf("token aggregate after provenance conflict = %#v, want 36 tokens over 3 turns", stats[0])
	}
	if stats[0].TotalCost != nil || stats[0].CostCurrency != nil ||
		stats[0].CostStatus != "unknown" || stats[0].CostSource != "none" {
		t.Fatalf("cost aggregate after provenance conflict = %#v, want fail-closed unknown/none", stats[0])
	}

	registerSessionForGlobalTests(t, globalDB, "sess-stats-overflow")
	oneToken := int64(1)
	maxCost := math.MaxFloat64
	for range 2 {
		if err := globalDB.UpdateTokenStats(testutil.Context(t), TokenStatsUpdate{
			SessionID:    "sess-stats-overflow",
			AgentName:    "coder",
			TotalTokens:  &oneToken,
			CostAmount:   &maxCost,
			CostCurrency: &currency,
			CostStatus:   "actual",
			CostSource:   "agent_reported",
			Turns:        1,
		}); err != nil {
			t.Fatalf("UpdateTokenStats(overflow) error = %v", err)
		}
	}
	overflowStats, err := globalDB.ListTokenStats(
		testutil.Context(t),
		TokenStatsQuery{SessionID: "sess-stats-overflow"},
	)
	if err != nil {
		t.Fatalf("ListTokenStats(after overflow) error = %v", err)
	}
	if len(overflowStats) != 1 || overflowStats[0].TotalTokens == nil ||
		*overflowStats[0].TotalTokens != 2 || overflowStats[0].TurnCount != 2 {
		t.Fatalf("token aggregate after overflow = %#v, want 2 tokens over 2 turns", overflowStats)
	}
	if overflowStats[0].TotalCost != nil || overflowStats[0].CostCurrency != nil ||
		overflowStats[0].CostStatus != "unknown" || overflowStats[0].CostSource != "none" {
		t.Fatalf("cost aggregate after overflow = %#v, want fail-closed unknown/none", overflowStats[0])
	}
}

func TestGlobalDBUpdateTokenStatsKeepsPerAgentRows(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	registerSessionForGlobalTests(t, globalDB, "sess-multi-agent")

	input := int64(10)
	if err := globalDB.UpdateTokenStats(testutil.Context(t), TokenStatsUpdate{
		SessionID:   "sess-multi-agent",
		AgentName:   "coder",
		InputTokens: &input,
		CostStatus:  "unknown",
		CostSource:  "none",
	}); err != nil {
		t.Fatalf("UpdateTokenStats(coder) error = %v", err)
	}
	if err := globalDB.UpdateTokenStats(testutil.Context(t), TokenStatsUpdate{
		SessionID:   "sess-multi-agent",
		AgentName:   "reviewer",
		InputTokens: &input,
		CostStatus:  "unknown",
		CostSource:  "none",
	}); err != nil {
		t.Fatalf("UpdateTokenStats(reviewer) error = %v", err)
	}

	stats, err := globalDB.ListTokenStats(testutil.Context(t), TokenStatsQuery{SessionID: "sess-multi-agent"})
	if err != nil {
		t.Fatalf("ListTokenStats() error = %v", err)
	}
	if got := len(stats); got != 2 {
		t.Fatalf("len(stats) = %d, want 2", got)
	}

	byAgent := make(map[string]TokenStats, len(stats))
	for _, stat := range stats {
		byAgent[stat.AgentName] = stat
	}
	if _, ok := byAgent["coder"]; !ok {
		t.Fatalf("missing coder stats: %#v", stats)
	}
	if _, ok := byAgent["reviewer"]; !ok {
		t.Fatalf("missing reviewer stats: %#v", stats)
	}
}

func TestGlobalDBUpdateSessionStateReturnsNotFoundForMissingSession(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)

	err := globalDB.UpdateSessionState(testutil.Context(t), SessionStateUpdate{
		ID:    "missing",
		State: "stopped",
	})
	if err == nil || !strings.Contains(err.Error(), `session "missing" not found`) {
		t.Fatalf("UpdateSessionState(missing) error = %v, want missing session error", err)
	}
}

func TestGlobalDBUpdateSessionStateRejectsUnmarshalableActivity(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	registerSessionForGlobalTests(t, globalDB, "sess-update-invalid-activity")
	unmarshalableTime := time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)

	err := globalDB.UpdateSessionState(testutil.Context(t), SessionStateUpdate{
		ID:    "sess-update-invalid-activity",
		State: "active",
		Liveness: &store.SessionLivenessMeta{
			Activity: &store.SessionActivityMeta{
				TurnStartedAt: &unmarshalableTime,
			},
		},
	})
	if err == nil {
		t.Fatal("UpdateSessionState(unmarshalable activity) error = nil, want marshal failure")
	}
	if !strings.Contains(err.Error(), "store: build update session state") ||
		!strings.Contains(err.Error(), "store: session liveness activity marshal") {
		t.Fatalf("UpdateSessionState(unmarshalable activity) error = %v, want activity marshal context", err)
	}

	sessions, err := globalDB.ListSessions(testutil.Context(t), SessionListQuery{
		ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID},
	})
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if got, want := len(sessions), 1; got != want {
		t.Fatalf("len(sessions) = %d, want %d", got, want)
	}
	if sessions[0].Liveness != nil && sessions[0].Liveness.Activity != nil {
		t.Fatalf(
			"sessions[0].Liveness.Activity = %#v, want failed update to skip activity write",
			sessions[0].Liveness.Activity,
		)
	}
}

func TestGlobalDBListSessionsWrapsInvalidActivityJSONValidation(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	registerSessionForGlobalTests(t, globalDB, "sess-invalid-activity-json")
	if _, err := globalDB.DB().ExecContext(
		testutil.Context(t),
		`UPDATE sessions SET activity_json = ? WHERE id = ?`,
		`{"idle_seconds":-1}`,
		"sess-invalid-activity-json",
	); err != nil {
		t.Fatalf("update invalid activity_json error = %v", err)
	}

	_, err := globalDB.ListSessions(testutil.Context(t), SessionListQuery{
		ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID},
	})
	if err == nil {
		t.Fatal("ListSessions(invalid activity_json) error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "store: validate session activity json") ||
		!strings.Contains(err.Error(), "store: session activity idle seconds must be zero or positive") {
		t.Fatalf("ListSessions(invalid activity_json) error = %v, want validation context", err)
	}
}

func TestGlobalDBUpdateSessionStateHandlesStopFields(t *testing.T) {
	t.Parallel()

	t.Run("Should update columns when stop reason is set", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"update-stop-reason",
			filepath.Join(t.TempDir(), "workspace"),
		)
		if err := globalDB.RegisterSession(testutil.Context(t), SessionInfo{
			ProfileID:     store.DefaultProfileID,
			ID:            "sess-update-stop",
			AgentName:     "coder",
			RuntimeStatus: store.SessionRuntimeUnbound,
			WorkspaceID:   workspaceID,
			State:         "active",
			CreatedAt:     time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
			UpdatedAt:     time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
		}); err != nil {
			t.Fatalf("RegisterSession() error = %v", err)
		}

		stopReason := string(store.StopUserCanceled)
		if err := globalDB.UpdateSessionState(testutil.Context(t), SessionStateUpdate{
			ID:            "sess-update-stop",
			State:         "stopped",
			StopReasonSet: true,
			StopReason:    &stopReason,
			StopDetail:    "requested by user",
			UpdatedAt:     time.Date(2026, 4, 3, 13, 2, 0, 0, time.UTC),
		}); err != nil {
			t.Fatalf("UpdateSessionState() error = %v", err)
		}

		gotStopReason, gotStopDetail := queryStoredSessionStopFields(t, globalDB.db, "sess-update-stop")
		assertOptionalStringEqual(t, gotStopReason, stringPointerForTest(string(store.StopUserCanceled)), "stop_reason")
		assertOptionalStringEqual(t, gotStopDetail, stringPointerForTest("requested by user"), "stop_detail")
	})

	t.Run("Should leave existing columns unchanged when stop reason is missing", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"preserve-stop-reason",
			filepath.Join(t.TempDir(), "workspace"),
		)
		if err := globalDB.RegisterSession(testutil.Context(t), SessionInfo{
			ProfileID:     store.DefaultProfileID,
			ID:            "sess-preserve-stop",
			AgentName:     "coder",
			RuntimeStatus: store.SessionRuntimeUnbound,
			WorkspaceID:   workspaceID,
			State:         "stopped",
			StopReason:    store.StopTimeout,
			StopDetail:    "deadline exceeded",
			CreatedAt:     time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
			UpdatedAt:     time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
		}); err != nil {
			t.Fatalf("RegisterSession() error = %v", err)
		}

		if err := globalDB.UpdateSessionState(testutil.Context(t), SessionStateUpdate{
			ID:        "sess-preserve-stop",
			State:     "orphaned",
			UpdatedAt: time.Date(2026, 4, 3, 13, 5, 0, 0, time.UTC),
		}); err != nil {
			t.Fatalf("UpdateSessionState() error = %v", err)
		}

		gotStopReason, gotStopDetail := queryStoredSessionStopFields(t, globalDB.db, "sess-preserve-stop")
		assertOptionalStringEqual(t, gotStopReason, stringPointerForTest(string(store.StopTimeout)), "stop_reason")
		assertOptionalStringEqual(t, gotStopDetail, stringPointerForTest("deadline exceeded"), "stop_detail")
	})

	t.Run("Should clear existing columns when stop reason is explicitly nil", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"clear-stop-reason",
			filepath.Join(t.TempDir(), "workspace"),
		)
		if err := globalDB.RegisterSession(testutil.Context(t), SessionInfo{
			ProfileID:     store.DefaultProfileID,
			ID:            "sess-clear-stop",
			AgentName:     "coder",
			RuntimeStatus: store.SessionRuntimeUnbound,
			WorkspaceID:   workspaceID,
			State:         "stopped",
			StopReason:    store.StopTimeout,
			StopDetail:    "deadline exceeded",
			CreatedAt:     time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
			UpdatedAt:     time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
		}); err != nil {
			t.Fatalf("RegisterSession() error = %v", err)
		}

		if err := globalDB.UpdateSessionState(testutil.Context(t), SessionStateUpdate{
			ID:            "sess-clear-stop",
			State:         "active",
			StopReasonSet: true,
			UpdatedAt:     time.Date(2026, 4, 3, 13, 5, 0, 0, time.UTC),
		}); err != nil {
			t.Fatalf("UpdateSessionState() error = %v", err)
		}

		gotStopReason, gotStopDetail := queryStoredSessionStopFields(t, globalDB.db, "sess-clear-stop")
		assertOptionalStringEqual(t, gotStopReason, nil, "stop_reason")
		assertOptionalStringEqual(t, gotStopDetail, nil, "stop_detail")
	})
}

func TestGlobalDBWritePermissionLogEntry(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	registerSessionForGlobalTests(t, globalDB, "sess-perm")

	if err := globalDB.WritePermissionLog(testutil.Context(t), PermissionLogEntry{
		SessionID:  "sess-perm",
		AgentName:  "coder",
		Action:     "bash",
		Resource:   "/tmp/project",
		Decision:   "allow",
		PolicyUsed: "approve-reads",
		Timestamp:  time.Date(2026, 4, 3, 15, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("WritePermissionLog() error = %v", err)
	}

	entries, err := globalDB.ListPermissionLog(testutil.Context(t), PermissionLogQuery{SessionID: "sess-perm"})
	if err != nil {
		t.Fatalf("ListPermissionLog() error = %v", err)
	}
	if got, want := len(entries), 1; got != want {
		t.Fatalf("len(entries) = %d, want %d", got, want)
	}
	if entries[0].Decision != "allow" || entries[0].PolicyUsed != "approve-reads" {
		t.Fatalf("entry = %#v, want allow/approve-reads", entries[0])
	}
}

func TestGlobalDBReconcileSessions(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	keepWorkspaceID := registerSessionForGlobalTests(t, globalDB, "sess-keep")
	registerSessionForGlobalTests(t, globalDB, "sess-orphan")

	onDisk := []SessionInfo{
		{
			ID:            "sess-keep",
			AgentName:     "coder",
			Provider:      "claude",
			RuntimeStatus: store.SessionRuntimeUnbound,
			WorkspaceID:   keepWorkspaceID,
			State:         "stopped",
			StopReason:    store.StopCompleted,
			CreatedAt:     time.Date(2026, 4, 3, 16, 0, 0, 0, time.UTC),
			ProfileID:     store.DefaultProfileID,
			UpdatedAt:     time.Date(2026, 4, 3, 16, 0, 0, 0, time.UTC),
		},
		{
			ID:            "sess-new",
			AgentName:     "reviewer",
			Provider:      "codex",
			RuntimeStatus: store.SessionRuntimeUnbound,
			WorkspaceID: registerWorkspaceForGlobalTests(
				t,
				globalDB,
				"sess-new-reconciled-workspace",
				filepath.Join(t.TempDir(), "sess-new"),
			),
			State:      "stopped",
			StopReason: store.StopUserCanceled,
			StopDetail: "requested by API",
			CreatedAt:  time.Date(2026, 4, 3, 16, 0, 0, 0, time.UTC),
			ProfileID:  store.DefaultProfileID,
			UpdatedAt:  time.Date(2026, 4, 3, 16, 0, 0, 0, time.UTC),
		},
	}

	result, err := globalDB.ReconcileSessions(testutil.Context(t), onDisk)
	if err != nil {
		t.Fatalf("ReconcileSessions() error = %v", err)
	}
	sort.Strings(result.Indexed)
	sort.Strings(result.Orphaned)
	if !testutil.EqualStringSlices(result.Indexed, []string{"sess-new"}) {
		t.Fatalf("Indexed = %#v, want %#v", result.Indexed, []string{"sess-new"})
	}
	if !testutil.EqualStringSlices(result.Orphaned, []string{"sess-orphan"}) {
		t.Fatalf("Orphaned = %#v, want %#v", result.Orphaned, []string{"sess-orphan"})
	}

	sessions, err := globalDB.ListSessions(testutil.Context(t), SessionListQuery{
		ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID},
	})
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	stateByID := make(map[string]string, len(sessions))
	stopReasonByID := make(map[string]store.StopReason, len(sessions))
	providerByID := make(map[string]string, len(sessions))
	for _, session := range sessions {
		stateByID[session.ID] = session.State
		stopReasonByID[session.ID] = session.StopReason
		providerByID[session.ID] = session.Provider
	}
	if stateByID["sess-new"] != "stopped" {
		t.Fatalf("stateByID[sess-new] = %q, want stopped", stateByID["sess-new"])
	}
	if stopReasonByID["sess-new"] != store.StopUserCanceled {
		t.Fatalf("stopReasonByID[sess-new] = %q, want %q", stopReasonByID["sess-new"], store.StopUserCanceled)
	}
	if providerByID["sess-new"] != "codex" {
		t.Fatalf("providerByID[sess-new] = %q, want codex", providerByID["sess-new"])
	}
	if stateByID["sess-orphan"] != "orphaned" {
		t.Fatalf("stateByID[sess-orphan] = %q, want orphaned", stateByID["sess-orphan"])
	}
}

func TestGlobalDBReconcileSessionsSkipsDuplicateIDsAndDefaultsTimestamps(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	reconciledAt := time.Date(2026, 4, 3, 16, 30, 0, 0, time.UTC)
	globalDB.now = func() time.Time {
		return reconciledAt
	}

	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"sess-duplicate-reconciled-workspace",
		filepath.Join(t.TempDir(), "sess-duplicate"),
	)
	onDisk := []SessionInfo{
		{
			ID:            "sess-duplicate",
			AgentName:     "coder",
			Provider:      "claude",
			RuntimeStatus: store.SessionRuntimeUnbound,
			WorkspaceID:   workspaceID,
			ProfileID:     store.DefaultProfileID,
			State:         "stopped",
		},
		{
			ID:            "sess-duplicate",
			AgentName:     "coder",
			Provider:      "codex",
			RuntimeStatus: store.SessionRuntimeUnbound,
			WorkspaceID:   workspaceID,
			ProfileID:     store.DefaultProfileID,
			State:         "orphaned",
		},
	}

	result, err := globalDB.ReconcileSessions(testutil.Context(t), onDisk)
	if err != nil {
		t.Fatalf("ReconcileSessions() error = %v", err)
	}
	if !testutil.EqualStringSlices(result.Indexed, []string{"sess-duplicate"}) {
		t.Fatalf("Indexed = %#v, want %#v", result.Indexed, []string{"sess-duplicate"})
	}
	if len(result.Orphaned) != 0 {
		t.Fatalf("Orphaned = %#v, want empty", result.Orphaned)
	}

	sessions, err := globalDB.ListSessions(testutil.Context(t), SessionListQuery{
		ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID},
	})
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if got, want := len(sessions), 1; got != want {
		t.Fatalf("len(sessions) = %d, want %d", got, want)
	}

	got := sessions[0]
	if got.Provider != "claude" {
		t.Fatalf("sessions[0].Provider = %q, want claude from first reconcile entry", got.Provider)
	}
	if got.State != "stopped" {
		t.Fatalf("sessions[0].State = %q, want stopped from first reconcile entry", got.State)
	}
	if !got.CreatedAt.Equal(reconciledAt) {
		t.Fatalf("sessions[0].CreatedAt = %v, want %v", got.CreatedAt, reconciledAt)
	}
	if !got.UpdatedAt.Equal(reconciledAt) {
		t.Fatalf("sessions[0].UpdatedAt = %v, want %v", got.UpdatedAt, reconciledAt)
	}
}

func openTestGlobalDB(t *testing.T) *GlobalDB {
	t.Helper()

	path := filepath.Join(t.TempDir(), GlobalDatabaseName)
	copyCurrentSchemaGlobalDBSeed(t, path)
	return openGlobalDBForTest(t, path)
}

func openFreshTestGlobalDB(t *testing.T) *GlobalDB {
	t.Helper()

	return openGlobalDBForTest(t, filepath.Join(t.TempDir(), GlobalDatabaseName))
}

func openGlobalDBForTest(t *testing.T, path string) *GlobalDB {
	t.Helper()

	sessionsDir := filepath.Join(filepath.Dir(path), "sessions")
	globalDB, err := OpenGlobalDB(
		testutil.Context(t),
		path,
		WithSessionEventMetadataOpener(sessiondb.NewEventMetadataOpener(sessionsDir)),
	)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := globalDB.Close(testutil.Context(t)); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	return globalDB
}

func copyCurrentSchemaGlobalDBSeed(t *testing.T, targetPath string) {
	t.Helper()

	if testGlobalDBCurrentSchemaSeedPath == "" {
		t.Fatal("globaldb current schema seed path is empty")
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		sourcePath := testGlobalDBCurrentSchemaSeedPath + suffix
		data, err := os.ReadFile(sourcePath)
		if err != nil {
			if suffix != "" && errors.Is(err, os.ErrNotExist) {
				continue
			}
			t.Fatalf("ReadFile(%q) error = %v", sourcePath, err)
		}
		if err := os.WriteFile(targetPath+suffix, data, 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", targetPath+suffix, err)
		}
	}
}

func registerSessionForGlobalTests(t *testing.T, globalDB *GlobalDB, sessionID string) string {
	t.Helper()

	now := time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		sessionID+"-workspace",
		filepath.Join(t.TempDir(), sessionID),
	)
	if err := globalDB.RegisterSession(testutil.Context(t), SessionInfo{
		ID:            sessionID,
		ProfileID:     store.DefaultProfileID,
		AgentName:     "coder",
		Provider:      "claude",
		RuntimeStatus: store.SessionRuntimeUnbound,
		WorkspaceID:   workspaceID,
		State:         "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("RegisterSession(%q) error = %v", sessionID, err)
	}
	return workspaceID
}

func insertWorkspaceForGlobalTests(
	t *testing.T,
	globalDB *GlobalDB,
	ws compozyworkspace.Workspace,
) compozyworkspace.Workspace {
	t.Helper()

	if strings.TrimSpace(ws.RootDir) == "" {
		t.Fatal("insertWorkspaceForGlobalTests() requires RootDir")
	}
	if err := os.MkdirAll(ws.RootDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", ws.RootDir, err)
	}
	if ws.CreatedAt.IsZero() {
		ws.CreatedAt = time.Date(2026, 4, 3, 9, 0, 0, 0, time.UTC)
	}
	if ws.UpdatedAt.IsZero() {
		ws.UpdatedAt = ws.CreatedAt
	}
	if err := globalDB.InsertWorkspace(testutil.Context(t), ws); err != nil {
		t.Fatalf("InsertWorkspace(%q) error = %v", ws.ID, err)
	}
	return ws
}

func registerWorkspaceForGlobalTests(t *testing.T, globalDB *GlobalDB, name string, rootDir string) string {
	t.Helper()

	workspace := insertWorkspaceForGlobalTests(t, globalDB, compozyworkspace.Workspace{
		ID:        "ws-" + strings.ReplaceAll(name, " ", "-"),
		RootDir:   rootDir,
		Name:      name,
		CreatedAt: time.Date(2026, 4, 3, 9, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 3, 9, 0, 0, 0, time.UTC),
	})
	return workspace.ID
}

func assertEventSummaryIDs(t *testing.T, globalDB *GlobalDB, want []string) {
	t.Helper()

	events, err := globalDB.ListEventSummaries(
		testutil.Context(t),
		EventSummaryQuery{ReadScope: store.ReadScope{AllProfiles: true}},
	)
	if err != nil {
		t.Fatalf("ListEventSummaries() error = %v", err)
	}
	got := make([]string, 0, len(events))
	for _, event := range events {
		got = append(got, event.ID)
	}
	sort.Strings(got)
	sort.Strings(want)
	if !slices.Equal(got, want) {
		t.Fatalf("event summary ids = %#v, want %#v", got, want)
	}
}

func assertTokenStatAgents(t *testing.T, globalDB *GlobalDB, want []string) {
	t.Helper()

	stats, err := globalDB.ListTokenStats(testutil.Context(t), TokenStatsQuery{})
	if err != nil {
		t.Fatalf("ListTokenStats() error = %v", err)
	}
	got := make([]string, 0, len(stats))
	for _, stat := range stats {
		got = append(got, stat.AgentName)
	}
	sort.Strings(got)
	sort.Strings(want)
	if !slices.Equal(got, want) {
		t.Fatalf("token stat agents = %#v, want %#v", got, want)
	}
}

func assertPermissionLogIDs(t *testing.T, globalDB *GlobalDB, want []string) {
	t.Helper()

	entries, err := globalDB.ListPermissionLog(testutil.Context(t), PermissionLogQuery{})
	if err != nil {
		t.Fatalf("ListPermissionLog() error = %v", err)
	}
	got := make([]string, 0, len(entries))
	for _, entry := range entries {
		got = append(got, entry.ID)
	}
	sort.Strings(got)
	sort.Strings(want)
	if !slices.Equal(got, want) {
		t.Fatalf("permission log ids = %#v, want %#v", got, want)
	}
}

func assertWorkspaceEqual(t *testing.T, got compozyworkspace.Workspace, want compozyworkspace.Workspace) {
	t.Helper()

	if got.ID != want.ID ||
		got.RootDir != want.RootDir ||
		got.Name != want.Name ||
		got.DefaultAgent != want.DefaultAgent ||
		got.SandboxRef != want.SandboxRef ||
		!got.CreatedAt.Equal(want.CreatedAt) ||
		!got.UpdatedAt.Equal(want.UpdatedAt) ||
		!testutil.EqualStringSlices(got.AdditionalDirs, want.AdditionalDirs) {
		t.Fatalf("workspace = %#v, want %#v", got, want)
	}
}

func assertTableColumns(t *testing.T, db *sql.DB, table string, want []string) {
	t.Helper()

	rows, err := db.QueryContext(testutil.Context(t), "PRAGMA table_info("+table+")")
	if err != nil {
		t.Fatalf("QueryContext(table_info %q) error = %v", table, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			t.Fatalf("rows.Close(table_info %q) error = %v", table, closeErr)
		}
	}()

	got := make([]string, 0)
	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal sql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &primaryKey); err != nil {
			t.Fatalf("Scan(table_info %q) error = %v", table, err)
		}
		got = append(got, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err(table_info %q) error = %v", table, err)
	}

	if !testutil.EqualStringSlices(got, want) {
		t.Fatalf("columns(%s) = %#v, want %#v", table, got, want)
	}
}

func queryStoredSessionStopFields(t *testing.T, db *sql.DB, sessionID string) (*string, *string) {
	t.Helper()

	var stopReason sql.NullString
	var stopDetail sql.NullString
	if err := db.QueryRowContext(testutil.Context(t), `SELECT stop_reason, stop_detail FROM sessions WHERE id = ?`, sessionID).
		Scan(&stopReason, &stopDetail); err != nil {
		t.Fatalf("QueryRowContext(stop fields %q) error = %v", sessionID, err)
	}
	return store.NullString(stopReason), store.NullString(stopDetail)
}

func assertOptionalStringEqual(t *testing.T, got *string, want *string, field string) {
	t.Helper()

	switch {
	case got == nil && want == nil:
		return
	case got == nil || want == nil:
		t.Fatalf("%s = %#v, want %#v", field, got, want)
	case *got != *want:
		t.Fatalf("%s = %q, want %q", field, *got, *want)
	}
}

func stringPointerForTest(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	copyValue := value
	return &copyValue
}

func ptrTime(value time.Time) *time.Time {
	copyValue := value.UTC()
	return &copyValue
}

func assertTablesPresent(t *testing.T, db *sql.DB, want ...string) {
	t.Helper()

	rows, err := db.QueryContext(testutil.Context(t), `SELECT name FROM sqlite_master WHERE type = 'table'`)
	if err != nil {
		t.Fatalf("QueryContext(sqlite_master) error = %v", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			t.Errorf("rows.Close() error = %v", closeErr)
		}
	}()

	got := make(map[string]struct{})
	for rows.Next() {
		var name string
		if scanErr := rows.Scan(&name); scanErr != nil {
			t.Fatalf("rows.Scan() error = %v", scanErr)
		}
		got[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err() = %v", err)
	}

	for _, table := range want {
		if _, ok := got[table]; !ok {
			t.Fatalf("table %q missing from sqlite_master: %#v", table, got)
		}
	}
}

func assertJournalModeWAL(t *testing.T, db *sql.DB) {
	t.Helper()

	var journalMode string
	if err := db.QueryRowContext(testutil.Context(t), `PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		t.Fatalf("QueryRowContext(PRAGMA journal_mode) error = %v", err)
	}
	if !strings.EqualFold(journalMode, "wal") {
		t.Fatalf("PRAGMA journal_mode = %q, want wal", journalMode)
	}
}

func assertSynchronousNormal(t *testing.T, db *sql.DB) {
	t.Helper()

	var synchronous int
	if err := db.QueryRowContext(testutil.Context(t), `PRAGMA synchronous`).Scan(&synchronous); err != nil {
		t.Fatalf("QueryRowContext(PRAGMA synchronous) error = %v", err)
	}
	if synchronous != 1 {
		t.Fatalf("PRAGMA synchronous = %d, want 1 (NORMAL)", synchronous)
	}
}
