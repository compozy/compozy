package globaldb

import (
	"database/sql"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	atlasmigrate "ariga.io/atlas/sql/migrate"
	"github.com/compozy/agh/internal/store"
	globalschema "github.com/compozy/agh/internal/store/globaldb/schema"
	"github.com/compozy/agh/internal/testutil"
)

func TestNetworkParticipationHardeningMigration(t *testing.T) {
	t.Run("Should backfill workspace-qualified evidence from the frozen v9 authorities", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		seedNetworkParticipationV9Fixture(t, path, false)

		globalDB, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(v9 fixture) error = %v", err)
		}
		closed := false
		t.Cleanup(func() {
			if closed {
				return
			}
			if closeErr := globalDB.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("Close(upgraded fixture cleanup) error = %v", closeErr)
			}
		})

		assertNetworkHardeningWorkspace(t, globalDB.db, "task_runs", "id", "run-v5-task")
		assertNetworkHardeningWorkspace(t, globalDB.db, "task_runs", "id", "run-v5-wake")
		assertNetworkHardeningWorkspace(
			t,
			globalDB.db,
			"network_coordination_invitations",
			"scope_id",
			"task-v5",
		)
		assertNetworkHardeningWorkspace(
			t,
			globalDB.db,
			"network_message_dispositions",
			"message_id",
			"msg-v5",
		)
		assertNetworkHardeningWorkspace(t, globalDB.db, "network_live_wakes", "wake_id", "wake-v5")
		assertNetworkHardeningWorkspace(t, globalDB.db, "network_wake_sources", "wake_id", "wake-v5")
		assertNetworkHardeningWorkspace(
			t,
			globalDB.db,
			"network_participation_budgets",
			"owner_key",
			"session:sess-v5-target",
		)
		assertNoNetworkHardeningForeignKeyViolations(t, globalDB.db)
		status, err := store.Status(testutil.Context(t), globalDB.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(upgraded fixture) error = %v", err)
		}
		assertCompleteMigrationStream(t, status, MigrationStream())

		if err := globalDB.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(upgraded fixture) error = %v", err)
		}
		closed = true
		reopened, err := OpenGlobalDB(testutil.Context(t), path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen upgraded fixture) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := reopened.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("Close(reopened fixture) error = %v", closeErr)
			}
		})
		assertNetworkHardeningWorkspace(t, reopened.db, "task_runs", "id", "run-v5-wake")
		assertNoNetworkHardeningForeignKeyViolations(t, reopened.db)
	})

	t.Run("Should reject unresolved v9 evidence without leaving a partial migration", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		seedNetworkParticipationV9Fixture(t, path, true)

		if globalDB, err := openGlobalMigrationUpgrade(t, path); err == nil {
			if closeErr := globalDB.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("Close(unexpected upgraded fixture) error = %v", closeErr)
			}
			t.Fatal("OpenGlobalDB(unresolved v9 fixture) error = nil, want migration refusal")
		} else if !strings.Contains(err.Error(), "version:10") ||
			!strings.Contains(err.Error(), "CHECK constraint failed") {
			t.Fatalf("OpenGlobalDB(unresolved v9 fixture) error = %q, want v10 assertion failure", err)
		}

		db, err := sql.Open(sqliteDriverName, path)
		if err != nil {
			t.Fatalf("sql.Open(rejected v9 fixture) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := db.Close(); closeErr != nil {
				t.Errorf("Close(rejected v9 fixture) error = %v", closeErr)
			}
		})
		status, err := store.Status(testutil.Context(t), db, networkParticipationV9MigrationStream(t))
		if err != nil {
			t.Fatalf("Status(rejected v9 fixture) error = %v", err)
		}
		if status.Version != 9 || status.AppliedCount != 9 {
			t.Fatalf("Status(rejected v9 fixture) = %#v, want frozen v9 prefix", status)
		}
		assertTableExcludesColumns(t, db, "task_runs", []string{"workspace_id"})
		assertTableExcludesColumns(t, db, "network_participation_budgets", []string{"workspace_id"})
	})
}

func TestNetworkSubscriptionHardCutMigration(t *testing.T) {
	t.Run("Should preserve supported subscriptions while removing legacy digest state", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		db, err := sql.Open(sqliteDriverName, path)
		if err != nil {
			t.Fatalf("sql.Open(v12 subscription fixture) error = %v", err)
		}
		if err := applyGlobalMigrationPrefix(t, db, networkSubscriptionV12MigrationStream(t)); err != nil {
			t.Fatalf("Apply(frozen v12 prefix) error = %v", err)
		}
		statements := []string{
			`INSERT INTO workspaces (id, root_dir, name, created_at, updated_at)
		 VALUES ('ws-v8', '/tmp/agh-network-v8', 'network-v8',
		 '2026-07-17T10:00:00Z', '2026-07-17T10:00:00Z')`,
			`INSERT INTO sessions (
		 id, agent_name, provider, workspace_id, state, created_at, updated_at
		 ) VALUES (
		 'sess-v8', 'reviewer', 'claude', 'ws-v8', 'active',
		 '2026-07-17T10:01:00Z', '2026-07-17T10:01:00Z'
		 )`,
			`INSERT INTO network_channels (
		 workspace_id, channel, purpose, created_by, created_at, updated_at
		 ) VALUES (
		 'ws-v8', 'builders', 'Coordinate review.', 'operator',
		 '2026-07-17T10:02:00Z', '2026-07-17T10:02:00Z'
		 )`,
			`INSERT INTO network_subscriptions (
		 workspace_id, channel, thread_id, session_id, mode, keyword_filters_json,
		 created_at, updated_at
		 ) VALUES
		 ('ws-v8', 'builders', '', 'sess-v8', 'full', '["release"]',
		  '2026-07-17T10:03:00Z', '2026-07-17T10:03:00Z'),
		 ('ws-v8', 'builders', 'thread-muted', 'sess-v8', 'mute', '["quiet"]',
		  '2026-07-17T10:04:00Z', '2026-07-17T10:04:00Z'),
		 ('ws-v8', 'builders', 'thread-digest', 'sess-v8', 'digest', '["summary"]',
		  '2026-07-17T10:05:00Z', '2026-07-17T10:05:00Z')`,
		}
		for index, statement := range statements {
			if _, err := db.ExecContext(testutil.Context(t), statement); err != nil {
				t.Fatalf("seed v8 subscription statement %d error = %v", index+1, err)
			}
		}
		if err := db.Close(); err != nil {
			t.Fatalf("Close(v8 subscription fixture) error = %v", err)
		}

		globalDB, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(v8 subscription fixture) error = %v", err)
		}
		closed := false
		t.Cleanup(func() {
			if closed {
				return
			}
			if closeErr := globalDB.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("Close(upgraded subscription fixture cleanup) error = %v", closeErr)
			}
		})

		entries, err := globalDB.ListNetworkSubscriptions(
			testutil.Context(t),
			store.NetworkSubscriptionQuery{WorkspaceID: "ws-v8", Channel: "builders", Limit: 10},
		)
		if err != nil {
			t.Fatalf("ListNetworkSubscriptions(upgraded fixture) error = %v", err)
		}
		if len(entries) != 2 || entries[0].Mode != store.NetworkSubscriptionModeMute ||
			entries[1].Mode != store.NetworkSubscriptionModeFull {
			t.Fatalf("upgraded subscriptions = %#v, want preserved mute/full rows only", entries)
		}
		assertTableExcludesColumns(t, globalDB.db, "network_subscriptions", []string{"keyword_filters_json"})
		if _, err := globalDB.db.ExecContext(testutil.Context(t), `INSERT INTO network_subscriptions (
		workspace_id, channel, thread_id, session_id, mode, created_at, updated_at
		) VALUES (
		'ws-v8', 'builders', 'thread-invalid', 'sess-v8', 'digest',
		'2026-07-17T10:06:00Z', '2026-07-17T10:06:00Z'
		)`); err == nil {
			t.Fatal("insert digest subscription error = nil, want v9 CHECK rejection")
		}
		status, err := store.Status(testutil.Context(t), globalDB.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(upgraded subscription fixture) error = %v", err)
		}
		assertCompleteMigrationStream(t, status, MigrationStream())

		if err := globalDB.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(upgraded subscription fixture) error = %v", err)
		}
		closed = true
		reopened, err := OpenGlobalDB(testutil.Context(t), path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen upgraded subscription fixture) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := reopened.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("Close(reopened subscription fixture) error = %v", closeErr)
			}
		})
		assertTableExcludesColumns(t, reopened.db, "network_subscriptions", []string{"keyword_filters_json"})
	})
}

func seedNetworkParticipationV9Fixture(t *testing.T, path string, unresolved bool) {
	t.Helper()
	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		t.Fatalf("sql.Open(v5 fixture) error = %v", err)
	}
	closed := false
	t.Cleanup(func() {
		if closed {
			return
		}
		if closeErr := db.Close(); closeErr != nil {
			t.Errorf("Close(v5 fixture cleanup) error = %v", closeErr)
		}
	})
	if err := applyGlobalMigrationPrefix(t, db, networkParticipationV9MigrationStream(t)); err != nil {
		t.Fatalf("Apply(frozen v9 prefix) error = %v", err)
	}

	statements := []string{
		`INSERT INTO workspaces (id, root_dir, name, created_at, updated_at)
		 VALUES ('ws-v5', '/tmp/agh-network-v5', 'network-v5',
		 '2026-07-16T10:00:00Z', '2026-07-16T10:00:00Z')`,
		`INSERT INTO sessions (
		 id, agent_name, provider, workspace_id, state, created_at, updated_at
		 ) VALUES
		 ('sess-v5-source', 'coder', 'claude', 'ws-v5', 'active',
		  '2026-07-16T10:01:00Z', '2026-07-16T10:01:00Z'),
		 ('sess-v5-target', 'reviewer', 'claude', 'ws-v5', 'active',
		  '2026-07-16T10:01:00Z', '2026-07-16T10:01:00Z')`,
		`INSERT INTO tasks (
		 id, scope, workspace_id, title, status, created_by_kind, created_by_ref,
		 origin_kind, origin_ref, created_at, updated_at
		 ) VALUES (
		 'task-v5', 'workspace', 'ws-v5', 'V5 task', 'ready', 'daemon', 'scheduler',
		 'daemon', 'scheduler', '2026-07-16T10:02:00Z', '2026-07-16T10:02:00Z'
		 )`,
		`INSERT INTO task_runs (
		 id, task_id, status, attempt, origin_kind, origin_ref, queued_at
		 ) VALUES (
		 'run-v5-task', 'task-v5', 'queued', 1, 'daemon', 'scheduler', '2026-07-16T10:03:00Z'
		 )`,
		`INSERT INTO network_timeline_log (
		 message_id, session_id, workspace_id, channel, surface, direction, peer_from,
		 peer_to, kind, preview_text, body_json, timestamp
		 ) VALUES (
		 'msg-v5', 'sess-v5-source', 'ws-v5', 'builders', NULL, 'sent',
		 'sess-v5-source', NULL, 'greet', 'hello', '{"text":"hello"}', '2026-07-16T10:04:00Z'
		 )`,
		`INSERT INTO network_coordination_invitations (
		 scope_kind, scope_id, dismissed_at, dismissed_by
		 ) VALUES ('task', 'task-v5', '2026-07-16T10:05:00Z', 'operator')`,
		`INSERT INTO network_message_dispositions (
		 message_id, recipient_session_id, decision, decided_at, acceptance_seq
		 ) VALUES ('msg-v5', 'sess-v5-target', 'deliver', '2026-07-16T10:06:00Z', 1)`,
		`INSERT INTO task_runs (
		 id, task_id, status, attempt, origin_kind, origin_ref,
		 network_spec_json, network_mode, network_channel, network_source, queued_at,
		 run_kind, network_wake_id, network_target_session_id, network_owner_key
		 ) VALUES (
		 'run-v5-wake', NULL, 'queued', 1, 'network', 'network.accept:msg-v5',
		 '{"version":"network-participation/v1","mode":"live","workspace_id":"ws-v5","channel_strategy":"named","channel_id":"builders","source":"explicit_request"}',
		 'live', 'builders', 'explicit_request', '2026-07-16T10:07:00Z',
		 'network_wake', 'wake-v5', 'sess-v5-target', 'session:sess-v5-target'
		 )`,
		`INSERT INTO network_live_wakes (
		 wake_id, task_run_id, owner_key, workspace_id, channel, root_id, depth, state,
		 coalesce_until, reserved_wall_ms, reserved_at
		 ) VALUES (
		 'wake-v5', 'run-v5-wake', 'session:sess-v5-target', 'ws-v5', 'builders',
		 'msg-v5', 0, 'open', '2026-07-16T10:08:00Z', 500, '2026-07-16T10:07:00Z'
		 )`,
		`INSERT INTO network_wake_sources (owner_key, envelope_id, wake_id)
		 VALUES ('session:sess-v5-target', 'msg-v5', 'wake-v5')`,
		`INSERT INTO network_participation_budgets (
		 owner_key, wakes_used, wall_ms_used, input_tokens_used, output_tokens_used,
		 exhausted_reason, updated_at
		 ) VALUES (
		 'session:sess-v5-target', 1, 0, 0, 0, '', '2026-07-16T10:07:00Z'
		 )`,
	}
	if unresolved {
		statements = append(statements, `INSERT INTO network_participation_budgets (
		 owner_key, wakes_used, wall_ms_used, input_tokens_used, output_tokens_used,
		 exhausted_reason, updated_at
		 ) VALUES ('session:orphan', 1, 0, 0, 0, '', '2026-07-16T10:09:00Z')`)
	}
	for index, statement := range statements {
		if _, err := db.ExecContext(testutil.Context(t), statement); err != nil {
			t.Fatalf("seed v5 statement %d error = %v", index+1, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close(v5 fixture) error = %v", err)
	}
	closed = true
}

func networkParticipationV9MigrationStream(t *testing.T) store.MigrationStream {
	t.Helper()
	return networkHardeningMigrationPrefix(t, "v9", []string{
		"00001_baseline.sql",
		"00002_schema.sql",
		"00003_schema.sql",
		"00004_schema.sql",
		"00005_schema.sql",
		"00006_schema.sql",
		"00007_schema.sql",
		"00008_network_participation.sql",
		"00009_automation_network_participation.sql",
	})
}

func networkSubscriptionV12MigrationStream(t *testing.T) store.MigrationStream {
	t.Helper()
	return networkHardeningMigrationPrefix(t, "v12", []string{
		"00001_baseline.sql",
		"00002_schema.sql",
		"00003_schema.sql",
		"00004_schema.sql",
		"00005_schema.sql",
		"00006_schema.sql",
		"00007_schema.sql",
		"00008_network_participation.sql",
		"00009_automation_network_participation.sql",
		"00010_network_participation_hardening.sql",
		"00011_schema.sql",
		"00012_schema.sql",
	})
}

func networkHardeningMigrationPrefix(t *testing.T, label string, names []string) store.MigrationStream {
	t.Helper()
	directory := &atlasmigrate.MemDir{}
	files := fstest.MapFS{}
	for _, name := range names {
		data, err := fs.ReadFile(globalschema.Files, "migrations/"+name)
		if err != nil {
			t.Fatalf("read frozen %s migration %q: %v", label, name, err)
		}
		if err := directory.WriteFile(name, data); err != nil {
			t.Fatalf("write frozen %s migration %q: %v", label, name, err)
		}
		files[name] = &fstest.MapFile{Data: data}
	}
	sum, err := directory.Checksum()
	if err != nil {
		t.Fatalf("checksum frozen %s migrations: %v", label, err)
	}
	sumBytes, err := sum.MarshalText()
	if err != nil {
		t.Fatalf("marshal frozen %s atlas.sum: %v", label, err)
	}
	files[atlasmigrate.HashFileName] = &fstest.MapFile{Data: sumBytes}
	stream := MigrationStream()
	stream.FS = files
	stream.Dir = "."
	return stream
}

func assertNetworkHardeningWorkspace(
	t *testing.T,
	db *sql.DB,
	table string,
	keyColumn string,
	key string,
) {
	t.Helper()
	queries := map[string]string{
		"task_runs:id": `SELECT workspace_id FROM task_runs WHERE id = ?`,
		"network_coordination_invitations:scope_id": `SELECT workspace_id
			FROM network_coordination_invitations WHERE scope_id = ?`,
		"network_message_dispositions:message_id": `SELECT workspace_id
			FROM network_message_dispositions WHERE message_id = ?`,
		"network_live_wakes:wake_id":   `SELECT workspace_id FROM network_live_wakes WHERE wake_id = ?`,
		"network_wake_sources:wake_id": `SELECT workspace_id FROM network_wake_sources WHERE wake_id = ?`,
		"network_participation_budgets:owner_key": `SELECT workspace_id
			FROM network_participation_budgets WHERE owner_key = ?`,
	}
	query, ok := queries[table+":"+keyColumn]
	if !ok {
		t.Fatalf("no workspace assertion query for %s.%s", table, keyColumn)
	}
	var workspaceID string
	if err := db.QueryRowContext(testutil.Context(t), query, key).Scan(&workspaceID); err != nil {
		t.Fatalf("query %s workspace for %q error = %v", table, key, err)
	}
	if workspaceID != "ws-v5" {
		t.Fatalf("%s workspace for %q = %q, want ws-v5", table, key, workspaceID)
	}
}

func assertNoNetworkHardeningForeignKeyViolations(t *testing.T, db *sql.DB) {
	t.Helper()
	rows, err := db.QueryContext(testutil.Context(t), `PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatalf("PRAGMA foreign_key_check error = %v", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			t.Errorf("Close(foreign_key_check rows) error = %v", closeErr)
		}
	}()
	if rows.Next() {
		var table string
		var rowID sql.NullInt64
		var parent string
		var foreignKeyID int
		if err := rows.Scan(&table, &rowID, &parent, &foreignKeyID); err != nil {
			t.Fatalf("scan foreign_key_check violation error = %v", err)
		}
		t.Fatalf(
			"foreign_key_check violation table=%q row_id=%v parent=%q foreign_key_id=%d",
			table,
			rowID,
			parent,
			foreignKeyID,
		)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate foreign_key_check rows error = %v", err)
	}
}
