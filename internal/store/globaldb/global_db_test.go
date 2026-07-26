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

	looppkg "github.com/compozy/agh/internal/loop"
	mcpauth "github.com/compozy/agh/internal/mcp/auth"
	memorypkg "github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
	globalschema "github.com/compozy/agh/internal/store/globaldb/schema"
	"github.com/compozy/agh/internal/store/sessiondb"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	aghworkspace "github.com/compozy/agh/internal/workspace"
	"github.com/pressly/goose/v3"
)

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
	dir, err := os.MkdirTemp("", "agh-globaldb-current-schema-*")
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
	path := filepath.Join(dir, GlobalDatabaseName)
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
	return m.Run()
}

func reportTestMainError(format string, args ...any) {
	if _, err := fmt.Fprintf(os.Stderr, format+"\n", args...); err != nil {
		panic(err)
	}
}

func TestOpenGlobalDBAppliesGlobalMigrationsAndEnablesWAL(t *testing.T) {
	t.Run("Should apply the complete global migration stream before repository use", func(t *testing.T) {
		t.Parallel()

		globalDB := openFreshTestGlobalDB(t)

		assertTablesPresent(
			t,
			globalDB.db,
			globalMigrationVersionTable,
			"workspaces",
			"sessions",
			"event_summaries",
			"token_stats",
			"permission_log",
			"extensions",
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
		)
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

		globalDB := openFreshTestGlobalDB(t)
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
		workspace := aghworkspace.Workspace{
			ID:        "ws-reopen",
			RootDir:   root,
			Name:      "reopen",
			CreatedAt: time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC),
		}
		if err := first.InsertWorkspace(ctx, workspace); err != nil {
			t.Fatalf("InsertWorkspace() error = %v", err)
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
	})

	t.Run("Should apply the destructive participation cut once and retain canonical Local history", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		createPreCutNetworkParticipationFixture(ctx, t, path)

		first, err := OpenGlobalDB(ctx, path)
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
		assertPostCutNetworkParticipationFixture(t, first)
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
		assertPostCutNetworkParticipationFixture(t, second)
		secondStatus, err := store.Status(ctx, second.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(second post-cut) error = %v", err)
		}
		if secondStatus != firstStatus {
			t.Fatalf("Status(second post-cut) = %#v, want unchanged %#v", secondStatus, firstStatus)
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

func createPreCutNetworkParticipationFixture(ctx context.Context, t *testing.T, path string) {
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
				'ws-precut', '/tmp/agh-precut', 'precut',
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
				network_channel, coordination_channel_id, queued_at
			) VALUES (
				'run-precut', 'task-precut', 'completed', 1, 'daemon', 'scheduler',
				'coord-run-precut', 'coord-run-precut', '2026-07-13T10:05:00Z'
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

func assertPostCutNetworkParticipationFixture(t *testing.T, globalDB *GlobalDB) {
	t.Helper()

	ctx := testutil.Context(t)
	for _, table := range []string{"sessions", "task_runs", "loop_runs"} {
		assertOwnerTableCanonicalLocal(t, globalDB.db, table)
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

	sessions, err := globalDB.ListSessions(ctx, store.SessionListQuery{ID: "sess-precut", Limit: 10})
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
	loopRun, err := globalDB.GetLoopRun(ctx, looppkg.WorkspaceID("ws-precut"), looppkg.RunID("loop-precut"))
	if err != nil {
		t.Fatalf("GetLoopRun(pre-cut history) error = %v", err)
	}
	if !reflect.DeepEqual(loopRun.NetworkSpec, participation.LocalSpec()) {
		t.Fatalf("GetLoopRun(pre-cut history).NetworkSpec = %#v, want canonical Local", loopRun.NetworkSpec)
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
			!strings.Contains(openErr.Error(), "complete AGH_HOME or workspace .agh directory") {
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
		{ID: "sum-old", SessionID: "sess-retention", Type: "agent_message", AgentName: "coder", Timestamp: old},
		{ID: "sum-boundary", SessionID: "sess-retention", Type: "agent_message", AgentName: "coder", Timestamp: boundary},
		{ID: "sum-fresh", SessionID: "sess-retention", Type: "agent_message", AgentName: "coder", Timestamp: fresh},
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
	t.Parallel()

	globalDB := openFreshTestGlobalDB(t)

	assertTableColumns(t, globalDB.db, "extensions", []string{
		"name",
		"version",
		"source",
		"enabled",
		"manifest_path",
		"installed_at",
		"capabilities",
		"actions",
		"checksum",
		"registry_slug",
		"registry_name",
		"remote_version",
		globalDBExtensionProvenanceJSONKey,
	})
}

func TestOpenGlobalDBExtensionsSchemaIsIdempotent(t *testing.T) {
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
		"enabled",
		"manifest_path",
		"installed_at",
		"capabilities",
		"actions",
		"checksum",
		"registry_slug",
		"registry_name",
		"remote_version",
		globalDBExtensionProvenanceJSONKey,
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
		ID:          "sess-global",
		Name:        "Alpha",
		AgentName:   "coder",
		Provider:    "claude",
		WorkspaceID: workspaceID,
		SessionType: "dream",
		State:       "active",
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
	}

	if err := globalDB.RegisterSession(testutil.Context(t), session); err != nil {
		t.Fatalf("RegisterSession() error = %v", err)
	}

	acpSessionID := "acp-123"
	if err := globalDB.UpdateSessionState(testutil.Context(t), SessionStateUpdate{
		ID:           session.ID,
		State:        "stopped",
		ACPSessionID: &acpSessionID,
		UpdatedAt:    createdAt.Add(2 * time.Minute),
	}); err != nil {
		t.Fatalf("UpdateSessionState() error = %v", err)
	}

	sessions, err := globalDB.ListSessions(testutil.Context(t), SessionListQuery{State: "stopped"})
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
	if sessions[0].Provider != "claude" {
		t.Fatalf("sessions[0].Provider = %q, want claude", sessions[0].Provider)
	}
	if sessions[0].ACPSessionID == nil || *sessions[0].ACPSessionID != "acp-123" {
		t.Fatalf("sessions[0].ACPSessionID = %#v, want acp-123", sessions[0].ACPSessionID)
	}
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
		ID:          "sess-provider-upsert",
		AgentName:   "coder",
		Provider:    "claude",
		WorkspaceID: workspaceID,
		State:       "active",
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
	}
	if err := globalDB.RegisterSession(testutil.Context(t), session); err != nil {
		t.Fatalf("RegisterSession(initial) error = %v", err)
	}

	session.Provider = "codex"
	session.UpdatedAt = createdAt.Add(time.Minute)
	if err := globalDB.RegisterSession(testutil.Context(t), session); err != nil {
		t.Fatalf("RegisterSession(update) error = %v", err)
	}

	sessions, err := globalDB.ListSessions(testutil.Context(t), SessionListQuery{})
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
				ID:          "sess-" + strings.ReplaceAll(tc.name, " ", "-"),
				AgentName:   "coder",
				WorkspaceID: workspaceID,
				State:       "stopped",
				StopReason:  tc.stopReason,
				StopDetail:  tc.stopDetail,
				CreatedAt:   time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
				UpdatedAt:   time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
			}

			if err := globalDB.RegisterSession(testutil.Context(t), session); err != nil {
				t.Fatalf("RegisterSession() error = %v", err)
			}

			gotStopReason, gotStopDetail := queryStoredSessionStopFields(t, globalDB.db, session.ID)
			assertOptionalStringEqual(t, gotStopReason, tc.wantStopReason, "stop_reason")
			assertOptionalStringEqual(t, gotStopDetail, tc.wantStopDetail, "stop_detail")

			sessions, err := globalDB.ListSessions(testutil.Context(t), SessionListQuery{})
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
		ID:          "sess-default-type",
		AgentName:   "coder",
		WorkspaceID: workspaceID,
		State:       "active",
		CreatedAt:   time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
	}

	if err := globalDB.RegisterSession(testutil.Context(t), session); err != nil {
		t.Fatalf("RegisterSession() error = %v", err)
	}

	sessions, err := globalDB.ListSessions(testutil.Context(t), SessionListQuery{})
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
	origin := taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "agh task test"}

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
	ws := aghworkspace.Workspace{
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
	assertWorkspaceEqual(t, byID, aghworkspace.Workspace{
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
		ref := aghworkspace.CoordinationRef{
			WorkspaceID: updated.ID,
			ScopeKind:   aghworkspace.InvitationScopeWorkspace,
		}
		commands := aghworkspace.NewCoordinationService(globalDB)
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
		firstView, setErr := commands.Set(ctx, aghworkspace.SetCoordination{
			Ref: ref, Enabled: true, ExpectedRevision: 0,
		}, operatorActorContextForTest("operator:first"))
		if setErr != nil {
			t.Fatalf("Set(first coordination) error = %v", setErr)
		}
		first := firstView.Setting
		if !first.Enabled || first.Revision != 1 || first.UpdatedBy != "operator:first" {
			t.Fatalf("Set(first coordination) = %#v, want enabled revision one", first)
		}

		secondView, setErr := commands.Set(ctx, aghworkspace.SetCoordination{
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
		if _, setErr = commands.Set(ctx, aghworkspace.SetCoordination{
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
		thirdView, setErr := commands.Set(ctx, aghworkspace.SetCoordination{
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
			view aghworkspace.CoordinationView
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
			view, callErr := commands.Set(ctx, aghworkspace.SetCoordination{
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
			view, callErr := commands.Set(ctx, aghworkspace.SetCoordination{
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
		if !errors.Is(secondConcurrent.err, aghworkspace.ErrCoordinationConflict) {
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
		summaries, summaryErr := globalDB.ListEventSummaries(ctx, EventSummaryQuery{
			WorkspaceID: updated.ID,
			Type:        "network.coordination.setting_changed",
		})
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
		aghworkspace.ErrWorkspaceNotFound,
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
		ID:          "sess-delete-guard",
		AgentName:   "coder",
		WorkspaceID: workspaceID,
		State:       "stopped",
		CreatedAt:   time.Date(2026, 4, 3, 14, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 3, 14, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("RegisterSession() error = %v", err)
	}

	if err := globalDB.DeleteWorkspace(testutil.Context(t), workspaceID); err != nil {
		t.Fatalf("DeleteWorkspace() error = %v, want nil", err)
	}

	sessions, err := globalDB.ListSessions(testutil.Context(t), SessionListQuery{
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
			aghworkspace.ErrWorkspaceNotFound,
		) {
			t.Fatalf("GetWorkspace(deleted) error = %v, want ErrWorkspaceNotFound", err)
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
			globalMCPAuthTarget("linear"),
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
		for _, target := range targets[1:] {
			if _, err := globalDB.GetMCPAuthToken(ctx, target); err != nil {
				t.Fatalf("GetMCPAuthToken(preserved %#v) error = %v", target, err)
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
			`CREATE TRIGGER fail_workspace_mcp_secret_cleanup
			 BEFORE DELETE ON vault_secrets
			 WHEN OLD.kind = 'mcp_oauth_access_token'
			 BEGIN
			   SELECT RAISE(ABORT, 'forced MCP secret cleanup failure');
			 END`,
		); err != nil {
			t.Fatalf("create cleanup failure trigger error = %v", err)
		}

		err := globalDB.DeleteWorkspace(ctx, workspaceID)
		if err == nil || !strings.Contains(err.Error(), "forced MCP secret cleanup failure") {
			t.Fatalf("DeleteWorkspace() error = %v, want forced cleanup failure", err)
		}
		if _, err := globalDB.GetWorkspace(ctx, workspaceID); err != nil {
			t.Fatalf("GetWorkspace(after rollback) error = %v", err)
		}
		if _, err := globalDB.GetMCPAuthToken(ctx, target); err != nil {
			t.Fatalf("GetMCPAuthToken(after rollback) error = %v", err)
		}
		for _, ref := range []string{accessRef, refreshRef} {
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
		ID:          "sess-active-guard",
		AgentName:   "coder",
		WorkspaceID: workspaceID,
		State:       "active",
		CreatedAt:   time.Date(2026, 4, 3, 14, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 3, 14, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("RegisterSession() error = %v", err)
	}

	err := globalDB.DeleteWorkspace(testutil.Context(t), workspaceID)
	if !errors.Is(err, aghworkspace.ErrWorkspaceHasActiveSessions) {
		t.Fatalf("DeleteWorkspace() error = %v, want ErrWorkspaceHasActiveSessions", err)
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

	base := aghworkspace.Workspace{
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
		ws   aghworkspace.Workspace
		want error
	}{
		{
			name: "Should reject a duplicate root directory",
			ws: aghworkspace.Workspace{
				ID:        "ws-duplicate-root",
				RootDir:   rootA,
				Name:      "beta",
				CreatedAt: base.CreatedAt,
				UpdatedAt: base.UpdatedAt,
			},
			want: aghworkspace.ErrWorkspacePathTaken,
		},
		{
			name: "Should reject a duplicate name",
			ws: aghworkspace.Workspace{
				ID:        "ws-duplicate-name",
				RootDir:   rootB,
				Name:      "alpha",
				CreatedAt: base.CreatedAt,
				UpdatedAt: base.UpdatedAt,
			},
			want: aghworkspace.ErrWorkspaceNameTaken,
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

	t.Run("Should map a real delete foreign key constraint", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"workspace-delete-constraint",
			filepath.Join(t.TempDir(), "workspace-delete-constraint"),
		)
		if err := globalDB.RegisterSession(ctx, SessionInfo{
			ID:          "sess-delete-constraint",
			AgentName:   "coder",
			WorkspaceID: workspaceID,
			State:       "stopped",
			CreatedAt:   time.Date(2026, 4, 3, 14, 0, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2026, 4, 3, 14, 0, 0, 0, time.UTC),
		}); err != nil {
			t.Fatalf("RegisterSession() error = %v", err)
		}

		_, err := globalDB.db.ExecContext(ctx, `DELETE FROM workspaces WHERE id = ?`, workspaceID)
		if err == nil {
			t.Fatal("raw workspace delete error = nil, want foreign key constraint")
		}
		if mapped := mapWorkspaceDeleteConstraintError(err); !errors.Is(
			mapped,
			aghworkspace.ErrWorkspaceHasSessions,
		) {
			t.Fatalf("mapWorkspaceDeleteConstraintError() error = %v, want ErrWorkspaceHasSessions", mapped)
		}
	})
}

func TestGlobalDBWorkspaceNotFoundErrors(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)

	if _, err := globalDB.GetWorkspace(
		testutil.Context(t),
		"ws-missing",
	); !errors.Is(
		err,
		aghworkspace.ErrWorkspaceNotFound,
	) {
		t.Fatalf("GetWorkspace(missing) error = %v, want ErrWorkspaceNotFound", err)
	}
	if _, err := globalDB.GetWorkspaceByPath(
		testutil.Context(t),
		filepath.Join(t.TempDir(), "missing"),
	); !errors.Is(
		err,
		aghworkspace.ErrWorkspaceNotFound,
	) {
		t.Fatalf("GetWorkspaceByPath(missing) error = %v, want ErrWorkspaceNotFound", err)
	}
	if _, err := globalDB.GetWorkspaceByName(
		testutil.Context(t),
		"missing",
	); !errors.Is(
		err,
		aghworkspace.ErrWorkspaceNotFound,
	) {
		t.Fatalf("GetWorkspaceByName(missing) error = %v, want ErrWorkspaceNotFound", err)
	}
	if err := globalDB.UpdateWorkspace(testutil.Context(t), aghworkspace.Workspace{
		ID:        "ws-missing",
		RootDir:   filepath.Join(t.TempDir(), "missing"),
		Name:      "missing",
		UpdatedAt: time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
	}); !errors.Is(err, aghworkspace.ErrWorkspaceNotFound) {
		t.Fatalf("UpdateWorkspace(missing) error = %v, want ErrWorkspaceNotFound", err)
	}
	if err := globalDB.DeleteWorkspace(
		testutil.Context(t),
		"ws-missing",
	); !errors.Is(
		err,
		aghworkspace.ErrWorkspaceNotFound,
	) {
		t.Fatalf("DeleteWorkspace(missing) error = %v, want ErrWorkspaceNotFound", err)
	}
}

func TestGlobalDBWorkspaceValidationAndDefaulting(t *testing.T) {
	t.Parallel()

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

	if err := globalDB.InsertWorkspace(testutil.Context(t), aghworkspace.Workspace{
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
	if !aghworkspace.IsWorkspaceID(workspaces[0].ID) {
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
				return globalDB.InsertWorkspace(testutil.Context(t), aghworkspace.Workspace{Name: "missing-root"})
			},
		},
		{
			name: "insert missing name",
			run: func() error {
				return globalDB.InsertWorkspace(testutil.Context(t), aghworkspace.Workspace{RootDir: rootDir})
			},
		},
		{
			name: "update missing id",
			run: func() error {
				return globalDB.UpdateWorkspace(
					testutil.Context(t),
					aghworkspace.Workspace{RootDir: rootDir, Name: "missing-id"},
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
	first := insertWorkspaceForGlobalTests(t, globalDB, aghworkspace.Workspace{
		ID:        "ws-zeta",
		RootDir:   filepath.Join(t.TempDir(), "workspace-zeta"),
		Name:      "zeta",
		CreatedAt: time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC),
	})
	second := insertWorkspaceForGlobalTests(t, globalDB, aghworkspace.Workspace{
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
		ID:                  "sess-workspace-id",
		AgentName:           "coder",
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

	sessions, err := globalDB.ListSessions(testutil.Context(t), SessionListQuery{})
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

	assertTableColumns(
		t,
		globalDB.db,
		"sessions",
		[]string{
			"id",
			"name",
			"agent_name",
			"provider",
			"workspace_id",
			"session_type",
			"state",
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
		ID:          "sess-invalid-stall",
		AgentName:   "coder",
		WorkspaceID: workspaceID,
		State:       "active",
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
			ID:          "sess-invalid-activity",
			AgentName:   "coder",
			WorkspaceID: workspaceID,
			State:       "active",
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

		sessions, err := globalDB.ListSessions(testutil.Context(t), SessionListQuery{})
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
		SessionID:     "sess-summary",
		Type:          "agent_message",
		AgentName:     "coder",
		RootSessionID: "sess-summary",
		Summary:       "assistant replied",
		Timestamp:     time.Date(2026, 4, 3, 14, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("WriteEventSummary() error = %v", err)
	}

	summaries, err := globalDB.ListEventSummaries(testutil.Context(t), EventSummaryQuery{SessionID: "sess-summary"})
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

	providerFiltered, err := globalDB.ListEventSummaries(testutil.Context(t), EventSummaryQuery{
		Provider:  "claude",
		Component: "session",
	})
	if err != nil {
		t.Fatalf("ListEventSummaries(provider component) error = %v", err)
	}
	if got, want := len(providerFiltered), 1; got != want {
		t.Fatalf("len(providerFiltered) = %d, want %d", got, want)
	}

	if err := globalDB.WriteEventSummary(testutil.Context(t), EventSummary{
		SessionID: "sess-summary",
		Type:      "task.run_failed",
		AgentName: "coder",
		Summary:   "task run failed",
		Timestamp: time.Date(2026, 4, 3, 14, 1, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("WriteEventSummary(failed task run) error = %v", err)
	}
	errorOnly, err := globalDB.ListEventSummaries(testutil.Context(t), EventSummaryQuery{
		Provider:  "claude",
		Component: "task",
		ErrorOnly: true,
	})
	if err != nil {
		t.Fatalf("ListEventSummaries(error only) error = %v", err)
	}
	if got, want := len(errorOnly), 1; got != want {
		t.Fatalf("len(errorOnly) = %d, want %d", got, want)
	}
	if got, want := errorOnly[0].Outcome, "failure"; got != want {
		t.Fatalf("errorOnly[0].Outcome = %q, want %q", got, want)
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
			summary: EventSummary{
				Type:      "settings.changed",
				Content:   []byte(`{"section":"skills","source":"http","operation":"patch"}`),
				Summary:   "skills settings changed",
				Timestamp: time.Date(2026, 5, 4, 14, 5, 0, 0, time.UTC),
			},
			wantType: "settings.changed",
			wantBody: `{"section":"skills","source":"http","operation":"patch"}`,
		},
		{
			name: "Should persist skill shadowed events without a session",
			summary: EventSummary{
				Type:      "skill.shadowed",
				AgentName: "reviewer",
				Content:   []byte(skillShadowedBody),
				Summary:   "skill review shadowed workspace with agent-local",
				Timestamp: time.Date(2026, 5, 4, 14, 6, 0, 0, time.UTC),
			},
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

			summaries, err := globalDB.ListEventSummaries(testutil.Context(t), EventSummaryQuery{})
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
				summaries[0].Content,
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

	sessions, err := globalDB.ListSessions(testutil.Context(t), SessionListQuery{})
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

	_, err := globalDB.ListSessions(testutil.Context(t), SessionListQuery{})
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
			ID:          "sess-update-stop",
			AgentName:   "coder",
			WorkspaceID: workspaceID,
			State:       "active",
			CreatedAt:   time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
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
			ID:          "sess-preserve-stop",
			AgentName:   "coder",
			WorkspaceID: workspaceID,
			State:       "stopped",
			StopReason:  store.StopTimeout,
			StopDetail:  "deadline exceeded",
			CreatedAt:   time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
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
			ID:          "sess-clear-stop",
			AgentName:   "coder",
			WorkspaceID: workspaceID,
			State:       "stopped",
			StopReason:  store.StopTimeout,
			StopDetail:  "deadline exceeded",
			CreatedAt:   time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
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
	registerSessionForGlobalTests(t, globalDB, "sess-keep")
	registerSessionForGlobalTests(t, globalDB, "sess-orphan")

	onDisk := []SessionInfo{
		{
			ID:        "sess-keep",
			AgentName: "coder",
			Provider:  "claude",
			WorkspaceID: registerWorkspaceForGlobalTests(
				t,
				globalDB,
				"sess-keep-reconciled-workspace",
				filepath.Join(t.TempDir(), "sess-keep"),
			),
			State:      "stopped",
			StopReason: store.StopCompleted,
			CreatedAt:  time.Date(2026, 4, 3, 16, 0, 0, 0, time.UTC),
			UpdatedAt:  time.Date(2026, 4, 3, 16, 0, 0, 0, time.UTC),
		},
		{
			ID:        "sess-new",
			AgentName: "reviewer",
			Provider:  "codex",
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

	sessions, err := globalDB.ListSessions(testutil.Context(t), SessionListQuery{})
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
			ID:          "sess-duplicate",
			AgentName:   "coder",
			Provider:    "claude",
			WorkspaceID: workspaceID,
			State:       "stopped",
		},
		{
			ID:          "sess-duplicate",
			AgentName:   "coder",
			Provider:    "codex",
			WorkspaceID: workspaceID,
			State:       "orphaned",
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

	sessions, err := globalDB.ListSessions(testutil.Context(t), SessionListQuery{})
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
		ID:          sessionID,
		AgentName:   "coder",
		Provider:    "claude",
		WorkspaceID: workspaceID,
		State:       "active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		t.Fatalf("RegisterSession(%q) error = %v", sessionID, err)
	}
	return workspaceID
}

func insertWorkspaceForGlobalTests(t *testing.T, globalDB *GlobalDB, ws aghworkspace.Workspace) aghworkspace.Workspace {
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

	workspace := insertWorkspaceForGlobalTests(t, globalDB, aghworkspace.Workspace{
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

	events, err := globalDB.ListEventSummaries(testutil.Context(t), EventSummaryQuery{})
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

func assertWorkspaceEqual(t *testing.T, got aghworkspace.Workspace, want aghworkspace.Workspace) {
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
