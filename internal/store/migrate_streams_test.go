package store_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	atlasmigrate "ariga.io/atlas/sql/migrate"
	atlasschema "ariga.io/atlas/sql/schema"
	atlassqlite "ariga.io/atlas/sql/sqlite"
	"github.com/compozy/compozy/internal/memory"
	memoryschema "github.com/compozy/compozy/internal/memory/schema"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	globalschema "github.com/compozy/compozy/internal/store/globaldb/schema"
	"github.com/compozy/compozy/internal/store/sessiondb"
	sessionschema "github.com/compozy/compozy/internal/store/sessiondb/schema"
	"github.com/compozy/compozy/internal/store/workspacedb"
	workspaceschema "github.com/compozy/compozy/internal/store/workspacedb/schema"
	"github.com/compozy/compozy/internal/testutil"
	_ "modernc.org/sqlite"
)

const migrationTestTimeout = 3 * time.Minute

func migrationTestContext(t *testing.T) context.Context {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), migrationTestTimeout)
	t.Cleanup(cancel)
	return ctx
}

type productionMigrationStream struct {
	name              string
	stream            store.MigrationStream
	schemaFS          fs.FS
	declarativeSource string
}

func productionMigrationStreams() []productionMigrationStream {
	return []productionMigrationStream{
		{
			name:              "global",
			stream:            globaldb.MigrationStream(),
			schemaFS:          globalschema.Files,
			declarativeSource: "definitions",
		},
		{
			name:              "session",
			stream:            sessiondb.MigrationStream(),
			schemaFS:          sessionschema.Files,
			declarativeSource: "schema.sql",
		},
		{
			name:              "memory",
			stream:            memory.MigrationStream(),
			schemaFS:          memoryschema.Files,
			declarativeSource: "schema.sql",
		},
		{
			name:              "workspace",
			stream:            workspacedb.MigrationStream(),
			schemaFS:          workspaceschema.Files,
			declarativeSource: "schema.sql",
		},
	}
}

func TestProductionMigrationStreams(t *testing.T) {
	t.Run("Should embed four distinct sequential baseline streams", func(t *testing.T) {
		t.Parallel()

		seenTables := make(map[string]string)
		for _, item := range productionMigrationStreams() {
			if item.stream.Name != item.name {
				t.Fatalf("stream name = %q, want %q", item.stream.Name, item.name)
			}
			if owner, exists := seenTables[item.stream.VersionTable]; exists {
				t.Fatalf("version table %q shared by %s and %s", item.stream.VersionTable, owner, item.name)
			}
			seenTables[item.stream.VersionTable] = item.name
			versions := embeddedMigrationVersions(t, item.stream)
			foundBaseline := false
			for _, version := range versions {
				if version == 1 {
					foundBaseline = true
				}
			}
			sort.Ints(versions)
			if !foundBaseline || len(versions) == 0 {
				t.Fatalf("%s migrations have no 00001_baseline.sql", item.name)
			}
			for index, version := range versions {
				if want := index + 1; version != want {
					t.Fatalf("%s migration versions = %v, want sequential versions from 1", item.name, versions)
				}
			}
			if _, err := fs.ReadFile(item.stream.FS, item.stream.Dir+"/atlas.sum"); err != nil {
				t.Fatalf("read %s atlas.sum: %v", item.name, err)
			}
		}
	})

	t.Run("Should keep global and memory domain table ownership disjoint", func(t *testing.T) {
		t.Parallel()

		globalTables := schemaOwnedTables(t, globalschema.Files, "definitions")
		memoryTables := schemaOwnedTables(t, memoryschema.Files, "schema.sql")
		for table := range globalTables {
			if memoryTables[table] {
				t.Fatalf("table %q is owned by both global and memory baselines", table)
			}
		}
		if globalTables["memory_events"] {
			t.Fatal("global baseline owns memory_events, want memory stream ownership")
		}
		if !memoryTables["memory_events"] {
			t.Fatal("memory baseline does not own memory_events")
		}
	})

	t.Run("Should apply global and memory baselines to one physical database", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openStreamTestDB(t, "shared-compozy.db")
		globalStream := globaldb.MigrationStream()
		memoryStream := memory.MigrationStream()
		if err := store.Apply(ctx, db, globalStream); err != nil {
			t.Fatalf("Apply(global) error = %v", err)
		}
		if err := store.Apply(ctx, db, memoryStream); err != nil {
			t.Fatalf("Apply(memory) error = %v", err)
		}
		for _, stream := range []store.MigrationStream{globalStream, memoryStream} {
			status, err := store.Status(ctx, db, stream)
			if err != nil {
				t.Fatalf("Status(%s) error = %v", stream.Name, err)
			}
			versions := embeddedMigrationVersions(t, stream)
			wantVersion := int64(versions[len(versions)-1])
			if status.Version != wantVersion || status.AppliedCount != len(versions) {
				t.Fatalf(
					"Status(%s) = %#v, want version %d with %d applied migrations",
					stream.Name,
					status,
					wantVersion,
					len(versions),
				)
			}
		}
		if !sqliteTableExists(t, db, "memory_events") {
			t.Fatal("memory_events missing after shared-file baseline application")
		}
	})
}

func TestProductionMigrationStreamsFreshReopenAndAhead(t *testing.T) {
	for _, item := range productionMigrationStreams() {
		name := "Should fresh-apply, reopen, and reject an ahead " + item.name + " stream"
		if item.name == "global" {
			name += " [UT-160]"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := migrationTestContext(t)
			stream := item.stream
			stream.Bootstrap = nil
			versions := embeddedMigrationVersions(t, stream)
			head := int64(versions[len(versions)-1])
			path := filepath.Join(t.TempDir(), item.name+"-reopen.db")
			first, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatalf("open fresh %s stream: %v", item.name, err)
			}
			firstOpen := true
			t.Cleanup(func() {
				if firstOpen {
					if err := first.Close(); err != nil {
						t.Errorf("close fresh %s stream after failure: %v", item.name, err)
					}
				}
			})
			if err := store.Apply(ctx, first, stream); err != nil {
				t.Fatalf("Apply(%s fresh) error = %v", item.name, err)
			}
			before, err := store.Status(ctx, first, stream)
			if err != nil {
				t.Fatalf("Status(%s fresh) error = %v", item.name, err)
			}
			if before.Version != head || before.AppliedCount != len(versions) {
				t.Fatalf(
					"Status(%s fresh) = %#v, want head %d with %d migrations",
					item.name,
					before,
					head,
					len(versions),
				)
			}
			if err := first.Close(); err != nil {
				t.Fatalf("close fresh %s stream: %v", item.name, err)
			}
			firstOpen = false

			reopened, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatalf("reopen %s stream: %v", item.name, err)
			}
			t.Cleanup(func() {
				if err := reopened.Close(); err != nil {
					t.Errorf("close reopened %s stream: %v", item.name, err)
				}
			})
			if err := store.Apply(ctx, reopened, stream); err != nil {
				t.Fatalf("Apply(%s reopen) error = %v", item.name, err)
			}
			after, err := store.Status(ctx, reopened, stream)
			if err != nil {
				t.Fatalf("Status(%s reopen) error = %v", item.name, err)
			}
			if after != before {
				t.Fatalf("Status(%s reopen) = %#v, want %#v", item.name, after, before)
			}
			assertSQLiteIntegrity(t, item.name, reopened)

			aheadDB := openStreamTestDB(t, item.name+"-ahead.db")
			if _, err := aheadDB.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE %q (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				version_id INTEGER NOT NULL,
				is_applied INTEGER NOT NULL,
				tstamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`, stream.VersionTable)); err != nil {
				t.Fatalf("create %s version table: %v", item.name, err)
			}
			if _, err := aheadDB.ExecContext(
				ctx,
				fmt.Sprintf("INSERT INTO %q (version_id, is_applied) VALUES (?, 1)", stream.VersionTable),
				head+1,
			); err != nil {
				t.Fatalf("seed ahead %s version: %v", item.name, err)
			}
			if err := store.Apply(ctx, aheadDB, stream); !errors.Is(err, store.ErrSchemaAhead) {
				t.Fatalf("Apply(%s ahead) error = %v, want ErrSchemaAhead", item.name, err)
			}
		})
	}
}

// Suite: command-palette migration tail.
// Invariant: 00078 remains the approval boundary, 00079 adds the three
// workspace-scoped personalization tables plus their deletion trigger, and
// 00080 rebuilds the recovery index so resume_fence precedes expires_at.
// Owning layer: global migration stream. Canonical suite: this file.
func TestGlobalCommandPaletteMigrationTail(t *testing.T) {
	t.Parallel()

	t.Run("Should upgrade 00078 to 00079 and cascade workspace personalization [IT-020]", func(t *testing.T) {
		t.Parallel()
		ctx := migrationTestContext(t)
		db := openStreamTestDB(t, "global-command-palette-tail.db")
		stream := globaldb.MigrationStream()
		stream.Bootstrap = nil
		if err := store.Apply(ctx, db, migrationPrefixStream(t, stream, 78)); err != nil {
			t.Fatalf("Apply(global through 00078) error = %v", err)
		}
		if !sqliteTableExists(t, db, "tool_approval_pending") {
			t.Fatal("tool_approval_pending missing at 00078")
		}
		for _, table := range []string{"cmd_palette_usage", "cmd_palette_query_hits", "cmd_palette_pins"} {
			if sqliteTableExists(t, db, table) {
				t.Fatalf("table %q exists before 00079", table)
			}
		}

		if err := store.Apply(ctx, db, migrationPrefixStream(t, stream, 80)); err != nil {
			t.Fatalf("Apply(global through 00080) error = %v", err)
		}
		for _, table := range []string{"cmd_palette_usage", "cmd_palette_query_hits", "cmd_palette_pins"} {
			if !sqliteTableExists(t, db, table) {
				t.Fatalf("table %q missing after 00079", table)
			}
		}
		var triggerCount int
		if err := db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM sqlite_master WHERE type = 'trigger' AND name = ?`,
			"cmd_palette_workspace_delete",
		).Scan(&triggerCount); err != nil {
			t.Fatalf("query command palette delete trigger: %v", err)
		}
		if triggerCount != 1 {
			t.Fatalf("command palette delete trigger count = %d, want 1", triggerCount)
		}

		workspaceID := "workspace-migration-tail"
		if _, err := db.ExecContext(ctx, `INSERT INTO workspaces (
			id, root_dir, add_dirs, name, default_agent, sandbox_ref, created_at, updated_at
		) VALUES (?, ?, '[]', ?, '', '', ?, ?)`,
			workspaceID,
			"/tmp/workspace-migration-tail",
			"migration-tail",
			"2026-08-19T12:00:00Z",
			"2026-08-19T12:00:00Z",
		); err != nil {
			t.Fatalf("insert migration-tail workspace: %v", err)
		}
		if _, err := db.ExecContext(ctx,
			`INSERT INTO cmd_palette_usage (
				workspace_id, command_id, use_count, frecency_weight, last_used_at, updated_at
			) VALUES (?, 'session.new', 1, 1, 1, 1)`, workspaceID,
		); err != nil {
			t.Fatalf("insert command palette usage: %v", err)
		}
		if _, err := db.ExecContext(ctx,
			`INSERT INTO cmd_palette_query_hits (
				workspace_id, query, command_id, weight, last_used_at
			) VALUES (?, 'new', 'session.new', 1, 1)`, workspaceID,
		); err != nil {
			t.Fatalf("insert command palette query hit: %v", err)
		}
		if _, err := db.ExecContext(ctx,
			`INSERT INTO cmd_palette_pins (workspace_id, command_id, pinned_at)
			 VALUES (?, 'session.new', 1)`, workspaceID,
		); err != nil {
			t.Fatalf("insert command palette pin: %v", err)
		}
		if _, err := db.ExecContext(ctx, `DELETE FROM workspaces WHERE id = ?`, workspaceID); err != nil {
			t.Fatalf("delete migration-tail workspace: %v", err)
		}
		var remainingRows int
		if err := db.QueryRowContext(ctx, `SELECT
			(SELECT COUNT(*) FROM cmd_palette_usage WHERE workspace_id = ?) +
			(SELECT COUNT(*) FROM cmd_palette_query_hits WHERE workspace_id = ?) +
			(SELECT COUNT(*) FROM cmd_palette_pins WHERE workspace_id = ?)`,
			workspaceID,
			workspaceID,
			workspaceID,
		).Scan(&remainingRows); err != nil {
			t.Fatalf("query command palette cascade count: %v", err)
		}
		if remainingRows != 0 {
			t.Fatalf("command palette cascade count = %d, want 0", remainingRows)
		}
		if sqlText := sqliteIndexSQL(t, db, "idx_tool_approval_pending_recovery"); !strings.Contains(
			sqlText,
			"resume_fence",
		) || strings.Index(sqlText, "resume_fence") > strings.Index(sqlText, "expires_at") {
			t.Fatalf("recovery index after tail = %q, want resume_fence before expires_at", sqlText)
		}
		assertSQLiteIntegrity(t, "global command palette migration tail", db)
	})

	t.Run("Should rebuild the approval recovery index with resume_fence before expires_at", func(t *testing.T) {
		t.Parallel()
		ctx := migrationTestContext(t)
		db := openStreamTestDB(t, "global-approval-recovery-index.db")
		stream := globaldb.MigrationStream()
		stream.Bootstrap = nil
		if err := store.Apply(ctx, db, migrationPrefixStream(t, stream, 79)); err != nil {
			t.Fatalf("Apply(global through 00079) error = %v", err)
		}
		before := sqliteIndexSQL(t, db, "idx_tool_approval_pending_recovery")
		if !strings.Contains(before, "expires_at") ||
			strings.Index(before, "expires_at") > strings.Index(before, "resume_fence") {
			t.Fatalf("recovery index at 00079 = %q, want expires_at before resume_fence", before)
		}
		if err := store.Apply(ctx, db, migrationPrefixStream(t, stream, 80)); err != nil {
			t.Fatalf("Apply(global through 00080) error = %v", err)
		}
		after := sqliteIndexSQL(t, db, "idx_tool_approval_pending_recovery")
		if !strings.Contains(after, "resume_fence") ||
			strings.Index(after, "resume_fence") > strings.Index(after, "expires_at") {
			t.Fatalf("recovery index at 00080 = %q, want resume_fence before expires_at", after)
		}
		assertSQLiteIntegrity(t, "global approval recovery index", db)
	})
}

// Suite: profiles migration tail.
// Invariant: 00082 seeds the permanent default owner before backfill, preserves
// pre-profile owner and personalization rows, and installs the immutable,
// active-owner contract on every durable work root.
// Owning layer: global migration stream. Canonical suite: this file.
func TestGlobalProfilesMigrationTail(t *testing.T) {
	t.Parallel()

	ctx := migrationTestContext(t)
	db := openStreamTestDB(t, "global-profiles-tail.db")
	stream := globaldb.MigrationStream()
	stream.Bootstrap = nil
	if err := store.Apply(ctx, db, migrationPrefixStream(t, stream, 81)); err != nil {
		t.Fatalf("Apply(global through 00081) error = %v", err)
	}

	const (
		workspaceID = "workspace-profiles-tail"
		createdAt   = "2026-08-20T12:00:00Z"
	)
	if _, err := db.ExecContext(ctx, `INSERT INTO workspaces (
		id, root_dir, add_dirs, name, default_agent, sandbox_ref, created_at, updated_at
	) VALUES (?, ?, '[]', ?, '', '', ?, ?)`,
		workspaceID, "/tmp/workspace-profiles-tail", "profiles-tail", createdAt, createdAt,
	); err != nil {
		t.Fatalf("insert pre-profile workspace: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO sessions (
		id, agent_name, workspace_id, state, created_at, updated_at
	) VALUES ('session-profiles-tail', 'coder', ?, 'active', ?, ?)`, workspaceID, createdAt, createdAt); err != nil {
		t.Fatalf("insert pre-profile session: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO token_usage_daily (
		day, workspace_id, agent_name, input_tokens, output_tokens, total_tokens, updated_at
	) VALUES ('2026-08-20', ?, 'coder', 3, 5, 8, ?)`, workspaceID, createdAt); err != nil {
		t.Fatalf("insert pre-profile token usage: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO notification_cursors (
		scope_kind, workspace_id, consumer_id, stream_name, subject_id, last_sequence, updated_at
	) VALUES ('workspace', ?, 'consumer-tail', 'task_events', 'task-tail', 7, ?)`, workspaceID, createdAt); err != nil {
		t.Fatalf("insert pre-profile notification cursor: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO tool_approval_pending (
		approval_id, workspace_id, invocation_id, target_kind, tool_id, args_json,
		approval_status, requested_at, expires_at
	) VALUES ('apr_profiles_tail', ?, 'invocation-profiles-tail', 'tool', 'compozy__status', '{}',
		'pending', 1, 2)`, workspaceID); err != nil {
		t.Fatalf("insert pre-profile pending approval: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO cmd_palette_usage (
		workspace_id, command_id, use_count, frecency_weight, last_used_at, updated_at
	) VALUES (?, 'session.new', 4, 2.5, 11, 12)`, workspaceID); err != nil {
		t.Fatalf("insert pre-profile palette usage: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO cmd_palette_query_hits (
		workspace_id, query, command_id, weight, last_used_at
	) VALUES (?, 'new', 'session.new', 3.5, 13)`, workspaceID); err != nil {
		t.Fatalf("insert pre-profile palette query hit: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO cmd_palette_pins (
		workspace_id, command_id, pinned_at
	) VALUES (?, 'session.new', 14)`, workspaceID); err != nil {
		t.Fatalf("insert pre-profile palette pin: %v", err)
	}

	if err := store.Apply(ctx, db, stream); err != nil {
		t.Fatalf("Apply(global through 00082) error = %v", err)
	}

	var name, color, icon, state string
	if err := db.QueryRowContext(ctx, `SELECT name, color, icon, state FROM profiles WHERE id = ?`,
		store.DefaultProfileID,
	).Scan(&name, &color, &icon, &state); err != nil {
		t.Fatalf("query permanent default profile: %v", err)
	}
	if name != "default" || color != "#8E8EB5" || icon != "circle" || state != "active" {
		t.Fatalf("default profile = (%q, %q, %q, %q), want exact permanent seed", name, color, icon, state)
	}

	ownerRoots := []string{
		"sessions", "tasks", "loop_runs", "automation_jobs", "automation_triggers",
		"automation_suggestions", "bridge_instances", "worktrees", "network_channels",
		"network_direct_rooms", "network_threads", "network_work", "notification_cursors",
		"tool_approval_grants", "event_summaries", "dead_entities", "token_usage_daily",
		"tool_approval_pending",
	}
	for _, table := range ownerRoots {
		if !sqliteColumns(t, db, table)["profile_id"] {
			t.Fatalf("table %q missing profile_id after 00082", table)
		}
		for _, suffix := range []string{"profile_owner_active", "profile_owner_immutable"} {
			var count int
			if err := db.QueryRowContext(ctx,
				`SELECT COUNT(*) FROM sqlite_master WHERE type = 'trigger' AND name = ?`,
				table+"_"+suffix,
			).Scan(&count); err != nil {
				t.Fatalf("query %s %s trigger: %v", table, suffix, err)
			}
			if count != 1 {
				t.Fatalf("%s %s trigger count = %d, want 1", table, suffix, count)
			}
		}
	}

	for _, assertion := range []struct {
		name  string
		query string
		args  []any
		want  int64
	}{
		{name: "session owner", query: `SELECT COUNT(*) FROM sessions WHERE id = 'session-profiles-tail' AND profile_id = ?`, args: []any{store.DefaultProfileID}, want: 1},
		{name: "token usage owner", query: `SELECT total_tokens FROM token_usage_daily WHERE day = '2026-08-20' AND profile_id = ? AND workspace_id = ? AND agent_name = 'coder'`, args: []any{store.DefaultProfileID, workspaceID}, want: 8},
		{name: "cursor owner", query: `SELECT last_sequence FROM notification_cursors WHERE profile_id = ? AND workspace_id = ? AND consumer_id = 'consumer-tail'`, args: []any{store.DefaultProfileID, workspaceID}, want: 7},
		{name: "pending approval owner", query: `SELECT COUNT(*) FROM tool_approval_pending WHERE approval_id = 'apr_profiles_tail' AND profile_id = ?`, args: []any{store.DefaultProfileID}, want: 1},
		{name: "palette usage lens", query: `SELECT use_count FROM cmd_palette_usage WHERE workspace_id = ? AND profile_lens_id = ? AND command_id = 'session.new'`, args: []any{workspaceID, store.DefaultProfileID}, want: 4},
		{name: "palette query lens", query: `SELECT CAST(weight AS INTEGER) FROM cmd_palette_query_hits WHERE workspace_id = ? AND profile_lens_id = ? AND query = 'new'`, args: []any{workspaceID, store.DefaultProfileID}, want: 3},
		{name: "palette pin lens", query: `SELECT pinned_at FROM cmd_palette_pins WHERE workspace_id = ? AND profile_lens_id = ? AND command_id = 'session.new'`, args: []any{workspaceID, store.DefaultProfileID}, want: 14},
	} {
		var got int64
		if err := db.QueryRowContext(ctx, assertion.query, assertion.args...).Scan(&got); err != nil {
			t.Fatalf("query preserved %s: %v", assertion.name, err)
		}
		if got != assertion.want {
			t.Fatalf("preserved %s = %d, want %d", assertion.name, got, assertion.want)
		}
	}

	const otherProfileID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	if _, err := db.ExecContext(ctx, `INSERT INTO profiles (
		id, name, color, icon, state, created_at
	) VALUES (?, 'other', '#8E8EB5', 'circle', 'active', ?)`, otherProfileID, createdAt); err != nil {
		t.Fatalf("insert second profile: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`UPDATE sessions SET profile_id = ? WHERE id = 'session-profiles-tail'`, otherProfileID,
	); err == nil || !strings.Contains(err.Error(), "profile_owner_immutable") {
		t.Fatalf("update immutable session owner error = %v, want profile_owner_immutable", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		t.Fatalf("enable foreign keys for restriction assertion: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`DELETE FROM profiles WHERE id = ?`, store.DefaultProfileID,
	); err == nil {
		t.Fatal("delete profile with owned work error = nil, want FK restriction")
	}

	const emptyWorkspaceID = "workspace-selection-cascade"
	if _, err := db.ExecContext(ctx, `INSERT INTO workspaces (
		id, root_dir, add_dirs, name, default_agent, sandbox_ref, created_at, updated_at
	) VALUES (?, ?, '[]', ?, '', '', ?, ?)`,
		emptyWorkspaceID, "/tmp/workspace-selection-cascade", "selection-cascade", createdAt, createdAt,
	); err != nil {
		t.Fatalf("insert selection workspace: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO profile_selections (
		lens, workspace_id, profile_id, updated_at
	) VALUES ('workspace', ?, ?, ?)`, emptyWorkspaceID, store.DefaultProfileID, createdAt); err != nil {
		t.Fatalf("insert profile selection: %v", err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM workspaces WHERE id = ?`, emptyWorkspaceID); err != nil {
		t.Fatalf("delete selection workspace: %v", err)
	}
	var remainingSelections int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM profile_selections WHERE workspace_id = ?`, emptyWorkspaceID,
	).Scan(&remainingSelections); err != nil {
		t.Fatalf("query selection cascade: %v", err)
	}
	if remainingSelections != 0 {
		t.Fatalf("selection rows after workspace delete = %d, want 0", remainingSelections)
	}
	assertSQLiteIntegrity(t, "global profiles migration tail", db)
}

func TestGlobalExtensionManifestV2Migration(t *testing.T) {
	t.Run("Should migrate extension manifests to the v2 contract", func(t *testing.T) {
		t.Parallel()

		ctx := migrationTestContext(t)
		db := openStreamTestDB(t, "extension-manifest-v2.db")
		stream := globaldb.MigrationStream()
		if err := store.Apply(ctx, db, migrationPrefixStream(t, stream, 27)); err != nil {
			t.Fatalf("Apply(global prefix) error = %v", err)
		}
		if _, err := db.ExecContext(
			ctx,
			`INSERT INTO extensions (
			name, version, source, manifest_path, installed_at,
			capabilities, actions, checksum, provenance_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"fixture-ext",
			"0.1.0",
			"user",
			"/extensions/fixture-ext/extension.toml",
			"2026-07-28T00:00:00Z",
			`{"provides":["memory.backend","tool.provider"]}`,
			`{"requires":["memory/store","sessions/list"]}`,
			"sha256:fixture",
			`{"source_kind":"local"}`,
		); err != nil {
			t.Fatalf("seed manifest v1 extension: %v", err)
		}

		stream.Bootstrap = nil
		if err := store.Apply(ctx, db, stream); err != nil {
			t.Fatalf("Apply(global) error = %v", err)
		}

		var providesJSON string
		var permissionsJSON string
		if err := db.QueryRowContext(
			ctx,
			`SELECT provides_json, permissions_json FROM extensions WHERE name = ?`,
			"fixture-ext",
		).Scan(&providesJSON, &permissionsJSON); err != nil {
			t.Fatalf("query migrated extension: %v", err)
		}
		if got, want := providesJSON, `["memory.backend","tool.provider"]`; got != want {
			t.Fatalf("provides_json = %q, want %q", got, want)
		}
		if got, want := permissionsJSON, `["memory/store","sessions/list"]`; got != want {
			t.Fatalf("permissions_json = %q, want %q", got, want)
		}
		for _, table := range []string{"extensions", "extension_dev_links"} {
			if !sqliteTableExists(t, db, table) {
				t.Fatalf("table %q missing after manifest v2 migration", table)
			}
		}
		columns := sqliteColumns(t, db, "extensions")
		for _, removed := range []string{"capabilities", "actions"} {
			if columns[removed] {
				t.Fatalf("extensions column %q remains after manifest v2 migration", removed)
			}
		}
		for _, added := range []string{"provides_json", "permissions_json"} {
			if !columns[added] {
				t.Fatalf("extensions column %q missing after manifest v2 migration", added)
			}
		}
	})
}

func migrationPrefixStream(t *testing.T, stream store.MigrationStream, head int) store.MigrationStream {
	t.Helper()

	files := fstest.MapFS{}
	directory := &atlasmigrate.MemDir{}
	entries, err := fs.ReadDir(stream.FS, stream.Dir)
	if err != nil {
		t.Fatalf("read %s migration directory: %v", stream.Name, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		separator := strings.IndexByte(entry.Name(), '_')
		if separator <= 0 {
			t.Fatalf("migration filename %q has no version prefix", entry.Name())
		}
		version, err := strconv.Atoi(entry.Name()[:separator])
		if err != nil {
			t.Fatalf("parse migration version from %q: %v", entry.Name(), err)
		}
		if version > head {
			continue
		}
		contents, err := fs.ReadFile(stream.FS, path.Join(stream.Dir, entry.Name()))
		if err != nil {
			t.Fatalf("read migration %q: %v", entry.Name(), err)
		}
		if err := directory.WriteFile(entry.Name(), contents); err != nil {
			t.Fatalf("write migration prefix %q: %v", entry.Name(), err)
		}
		files[path.Join(stream.Dir, entry.Name())] = &fstest.MapFile{Data: contents}
	}
	checksum, err := directory.Checksum()
	if err != nil {
		t.Fatalf("checksum %s migration prefix: %v", stream.Name, err)
	}
	checksumBytes, err := checksum.MarshalText()
	if err != nil {
		t.Fatalf("marshal %s migration prefix checksum: %v", stream.Name, err)
	}
	files[path.Join(stream.Dir, atlasmigrate.HashFileName)] = &fstest.MapFile{Data: checksumBytes}

	stream.FS = files
	stream.Bootstrap = nil
	return stream
}

func embeddedMigrationVersions(t *testing.T, stream store.MigrationStream) []int {
	t.Helper()

	entries, err := fs.ReadDir(stream.FS, stream.Dir)
	if err != nil {
		t.Fatalf("read %s migration directory: %v", stream.Name, err)
	}
	versions := make([]int, 0)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		separator := strings.IndexByte(entry.Name(), '_')
		if separator <= 0 {
			t.Fatalf("%s migration filename %q has no version prefix", stream.Name, entry.Name())
		}
		version, err := strconv.Atoi(entry.Name()[:separator])
		if err != nil {
			t.Fatalf("parse %s migration version: %v", stream.Name, err)
		}
		if version == 1 && entry.Name() != "00001_baseline.sql" {
			t.Fatalf("%s first migration = %q, want 00001_baseline.sql", stream.Name, entry.Name())
		}
		versions = append(versions, version)
	}
	if len(versions) == 0 {
		t.Fatalf("%s migration directory contains no SQL migrations", stream.Name)
	}
	sort.Ints(versions)
	return versions
}

func TestMigrationSchemaEquivalence(t *testing.T) {
	for _, item := range productionMigrationStreams() {
		name := "Should match the declarative schema for the " + item.name + " stream"
		if item.name == "global" {
			name += " [UT-160]"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := migrationTestContext(t)
			replayStream := item.stream
			replayStream.Bootstrap = nil
			migrationDB := openStreamTestDB(t, item.name+"-migration.db")
			if err := store.Apply(ctx, migrationDB, replayStream); err != nil {
				t.Fatalf("Apply(%s) error = %v", item.name, err)
			}
			if _, err := migrationDB.ExecContext(ctx, "DROP TABLE "+item.stream.VersionTable); err != nil {
				t.Fatalf("drop %s migration version table: %v", item.name, err)
			}
			schemaDB := openStreamTestDB(t, item.name+"-schema.db")
			executeDeclarativeSchema(t, schemaDB, item.schemaFS, item.declarativeSource)
			assertMigrationSchemaEquivalent(t, item.name, migrationDB, schemaDB, item.stream.VersionTable)

			bootstrapDB := openStreamTestDB(t, item.name+"-bootstrap.db")
			if err := store.Apply(ctx, bootstrapDB, item.stream); err != nil {
				t.Fatalf("Apply(%s bootstrap) error = %v", item.name, err)
			}
			assertMigrationSchemaEquivalent(
				t,
				item.name+" bootstrap",
				bootstrapDB,
				migrationDB,
				item.stream.VersionTable,
			)
			if got, want := normalizedSQLiteTableCounts(t, bootstrapDB, item.stream.VersionTable),
				normalizedSQLiteTableCounts(t, migrationDB, item.stream.VersionTable); got != want {
				t.Fatalf("%s bootstrap row counts = %q, want replay counts %q", item.name, got, want)
			}
		})
	}
}

func assertMigrationSchemaEquivalent(
	t *testing.T,
	streamName string,
	migrationDB *sql.DB,
	declarativeDB *sql.DB,
	versionTable string,
) {
	t.Helper()
	ctx := testutil.Context(t)
	migrationDriver, err := atlassqlite.Open(migrationDB)
	if err != nil {
		t.Fatalf("open %s migration schema inspector: %v", streamName, err)
	}
	declarativeDriver, err := atlassqlite.Open(declarativeDB)
	if err != nil {
		t.Fatalf("open %s declarative schema inspector: %v", streamName, err)
	}
	migrationRealm, err := migrationDriver.InspectRealm(ctx, nil)
	if err != nil {
		t.Fatalf("inspect %s migration schema: %v", streamName, err)
	}
	declarativeRealm, err := declarativeDriver.InspectRealm(ctx, nil)
	if err != nil {
		t.Fatalf("inspect %s declarative schema: %v", streamName, err)
	}
	removeSchemaTable(migrationRealm, versionTable)
	normalizeSQLiteAutoIndexes(migrationRealm)
	normalizeSQLiteAutoIndexes(declarativeRealm)
	changes, err := migrationDriver.RealmDiff(
		migrationRealm,
		declarativeRealm,
		atlasschema.DiffNormalized(),
	)
	if err != nil {
		t.Fatalf("diff %s migration and declarative schemas: %v", streamName, err)
	}
	if len(changes) != 0 {
		t.Fatalf(
			"%s migrations schema differs structurally from declarative schema\n--- migrations ---\n%s\n--- declarative ---\n%s",
			streamName,
			normalizedSQLiteSchema(t, migrationDB, versionTable),
			normalizedSQLiteSchema(t, declarativeDB, ""),
		)
	}

	gotNonAtlas := normalizedSQLiteObjects(t, migrationDB, "trigger", "view")
	wantNonAtlas := normalizedSQLiteObjects(t, declarativeDB, "trigger", "view")
	if gotNonAtlas != wantNonAtlas {
		t.Fatalf(
			"%s migration triggers/views differ from declarative schema\n--- migrations ---\n%s\n--- declarative ---\n%s",
			streamName,
			gotNonAtlas,
			wantNonAtlas,
		)
	}
}

func removeSchemaTable(realm *atlasschema.Realm, tableName string) {
	for _, schemaObject := range realm.Schemas {
		tables := schemaObject.Tables[:0]
		for _, table := range schemaObject.Tables {
			if table.Name != tableName {
				tables = append(tables, table)
			}
		}
		schemaObject.Tables = tables
	}
}

func normalizeSQLiteAutoIndexes(realm *atlasschema.Realm) {
	for _, schemaObject := range realm.Schemas {
		for _, table := range schemaObject.Tables {
			for _, index := range table.Indexes {
				if !strings.HasPrefix(index.Name, "sqlite_autoindex_") || sqliteIndexOrigin(index) != "u" {
					continue
				}
				parts := make([]string, 0, len(index.Parts)+1)
				parts = append(parts, table.Name)
				for _, part := range index.Parts {
					if part.C != nil {
						parts = append(parts, part.C.Name)
					}
				}
				if len(parts) > 1 {
					index.Name = strings.Join(parts, "_")
				}
			}
		}
	}
}

func sqliteIndexOrigin(index *atlasschema.Index) string {
	for _, attribute := range index.Attrs {
		if origin, ok := attribute.(*atlassqlite.IndexOrigin); ok {
			return origin.O
		}
	}
	return ""
}

func schemaOwnedTables(t *testing.T, schemaFS fs.FS, source string) map[string]bool {
	t.Helper()
	db := openStreamTestDB(t, "owned-tables.db")
	executeDeclarativeSchema(t, db, schemaFS, source)
	rows, err := db.QueryContext(
		testutil.Context(t),
		`SELECT name FROM pragma_table_list WHERE schema = 'main' AND type IN ('table', 'virtual')`,
	)
	if err != nil {
		t.Fatalf("query owned tables: %v", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			t.Errorf("close owned table rows: %v", err)
		}
	}()
	tables := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan owned table: %v", err)
		}
		if !strings.HasPrefix(name, "sqlite_") {
			tables[name] = true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate owned tables: %v", err)
	}
	return tables
}

func executeDeclarativeSchema(t *testing.T, db *sql.DB, schemaFS fs.FS, source string) {
	t.Helper()
	info, err := fs.Stat(schemaFS, source)
	if err != nil {
		t.Fatalf("stat declarative schema source %q: %v", source, err)
	}
	files := []string{source}
	if info.IsDir() {
		files = files[:0]
		entries, err := fs.ReadDir(schemaFS, source)
		if err != nil {
			t.Fatalf("read declarative schema directory %q: %v", source, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || path.Ext(entry.Name()) != ".sql" {
				continue
			}
			files = append(files, path.Join(source, entry.Name()))
		}
	}
	if len(files) == 0 {
		t.Fatalf("declarative schema source %q contains no SQL files", source)
	}
	for _, name := range files {
		contents, err := fs.ReadFile(schemaFS, name)
		if err != nil {
			t.Fatalf("read declarative schema file %q: %v", name, err)
		}
		if _, err := db.ExecContext(testutil.Context(t), string(contents)); err != nil {
			t.Fatalf("execute declarative schema file %q: %v", name, err)
		}
	}
}

func openStreamTestDB(t *testing.T, name string) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	if err := db.PingContext(testutil.Context(t)); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			t.Errorf("close failed database: %v", closeErr)
		}
		t.Fatalf("ping %s: %v", path, err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil && !errors.Is(err, sql.ErrConnDone) {
			t.Errorf("close %s: %v", path, err)
		}
	})
	return db
}

func normalizedSQLiteSchema(t *testing.T, db *sql.DB, excludedVersionTable string) string {
	t.Helper()
	rows, err := db.QueryContext(testutil.Context(t), `SELECT type, name, sql FROM sqlite_master
		WHERE sql IS NOT NULL AND name NOT LIKE 'sqlite_%' ORDER BY type, name`)
	if err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			t.Errorf("close sqlite_master rows: %v", err)
		}
	}()
	objects := make([]string, 0)
	for rows.Next() {
		var kind string
		var name string
		var statement string
		if err := rows.Scan(&kind, &name, &statement); err != nil {
			t.Fatalf("scan sqlite_master: %v", err)
		}
		if name == excludedVersionTable {
			continue
		}
		objects = append(objects, fmt.Sprintf("%s|%s|%s", kind, name, normalizeSchemaSQL(statement)))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate sqlite_master: %v", err)
	}
	return strings.Join(objects, "\n")
}

func normalizedSQLiteTableCounts(t *testing.T, db *sql.DB, excludedVersionTable string) string {
	t.Helper()
	rows, err := db.QueryContext(testutil.Context(t), `SELECT name FROM sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name <> ? ORDER BY name`, excludedVersionTable)
	if err != nil {
		t.Fatalf("query sqlite table names: %v", err)
	}
	tableNames := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan sqlite table name: %v", err)
		}
		tableNames = append(tableNames, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate sqlite table names: %v", err)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("close sqlite table names: %v", err)
	}
	counts := make([]string, 0, len(tableNames))
	for _, name := range tableNames {
		var count int
		quotedName := `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
		if err := db.QueryRowContext(testutil.Context(t), "SELECT COUNT(*) FROM "+quotedName).Scan(&count); err != nil {
			t.Fatalf("count rows in %s: %v", name, err)
		}
		counts = append(counts, fmt.Sprintf("%s=%d", name, count))
	}
	return strings.Join(counts, ",")
}

func normalizedSQLiteObjects(t *testing.T, db *sql.DB, objectTypes ...string) string {
	t.Helper()
	wanted := make(map[string]bool, len(objectTypes))
	for _, objectType := range objectTypes {
		wanted[objectType] = true
	}
	rows, err := db.QueryContext(testutil.Context(t), `SELECT type, name, sql FROM sqlite_master
		WHERE sql IS NOT NULL AND name NOT LIKE 'sqlite_%' ORDER BY type, name`)
	if err != nil {
		t.Fatalf("query sqlite_master objects: %v", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			t.Errorf("close sqlite_master object rows: %v", err)
		}
	}()
	objects := make([]string, 0)
	for rows.Next() {
		var kind string
		var name string
		var statement string
		if err := rows.Scan(&kind, &name, &statement); err != nil {
			t.Fatalf("scan sqlite_master object: %v", err)
		}
		if wanted[kind] {
			objects = append(objects, fmt.Sprintf("%s|%s|%s", kind, name, normalizeSchemaSQL(statement)))
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate sqlite_master objects: %v", err)
	}
	return strings.Join(objects, "\n")
}

func sqliteIndexSQL(t *testing.T, db *sql.DB, name string) string {
	t.Helper()
	var sqlText string
	if err := db.QueryRowContext(
		testutil.Context(t),
		`SELECT sql FROM sqlite_master WHERE type = 'index' AND name = ?`,
		name,
	).Scan(&sqlText); err != nil {
		t.Fatalf("query index %s sql: %v", name, err)
	}
	return sqlText
}

func sqliteTableExists(t *testing.T, db *sql.DB, table string) bool {
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

func assertSQLiteIntegrity(t *testing.T, streamName string, db *sql.DB) {
	t.Helper()
	ctx := testutil.Context(t)
	var integrity string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil {
		t.Fatalf("run %s integrity_check: %v", streamName, err)
	}
	if integrity != "ok" {
		t.Fatalf("%s integrity_check = %q, want ok", streamName, integrity)
	}
	rows, err := db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		t.Fatalf("run %s foreign_key_check: %v", streamName, err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			t.Errorf("close %s foreign_key_check rows: %v", streamName, err)
		}
	}()
	if rows.Next() {
		var table string
		var rowID sql.NullInt64
		var parent string
		var foreignKeyID int
		if err := rows.Scan(&table, &rowID, &parent, &foreignKeyID); err != nil {
			t.Fatalf("scan %s foreign_key_check result: %v", streamName, err)
		}
		t.Fatalf(
			"%s foreign_key_check violation: table=%q row_id=%v parent=%q foreign_key_id=%d",
			streamName,
			table,
			rowID,
			parent,
			foreignKeyID,
		)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate %s foreign_key_check results: %v", streamName, err)
	}
}

func sqliteColumns(t *testing.T, db *sql.DB, table string) map[string]bool {
	t.Helper()
	rows, err := db.QueryContext(testutil.Context(t), `SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		t.Fatalf("query table %s columns: %v", table, err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			t.Errorf("close table %s column rows: %v", table, err)
		}
	}()

	columns := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table %s column: %v", table, err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table %s columns: %v", table, err)
	}
	return columns
}

func normalizeSchemaSQL(statement string) string {
	normalized := strings.Join(strings.Fields(strings.ReplaceAll(statement, "`", "")), " ")
	normalized = strings.ReplaceAll(normalized, "( ", "(")
	return strings.ReplaceAll(normalized, " )", ")")
}
