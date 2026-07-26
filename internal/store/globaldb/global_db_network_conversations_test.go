package globaldb

import (
	"database/sql"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

const (
	networkConversationMigrationVersion = 21
	networkConversationTestWorkspaceID  = "ws-network-conversation"
)

func TestOpenGlobalDBCreatesNetworkConversationSchema(t *testing.T) {
	t.Parallel()

	t.Run("Should create final conversation tables and indexes on fresh DB", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)

		assertTablesPresent(
			t,
			globalDB.db,
			"network_timeline_log",
			"network_audit_log",
			"network_threads",
			"network_thread_participants",
			"network_direct_rooms",
			"network_work",
		)
		assertIndexesPresent(
			t,
			globalDB.db,
			"network_timeline_log",
			"idx_net_timeline_thread_sequence",
			"idx_net_timeline_direct_sequence",
			"idx_net_timeline_work_sequence",
			"idx_net_timeline_presence_sequence",
			"idx_net_timeline_kind_sequence",
		)
		assertIndexesPresent(
			t,
			globalDB.db,
			"network_audit_log",
			"idx_net_audit_conversation",
			"idx_net_audit_work",
		)
		assertIndexesPresent(
			t,
			globalDB.db,
			"network_direct_rooms",
			"idx_network_direct_rooms_activity",
			"idx_network_direct_rooms_session_a",
			"idx_network_direct_rooms_session_b",
		)
		assertIndexesPresent(
			t,
			globalDB.db,
			"network_work",
			"idx_network_work_conversation",
			"idx_network_work_state",
		)
		assertUniqueIndexColumns(
			t,
			globalDB.db,
			"network_direct_rooms",
			[]string{"workspace_id", "channel", "session_a", "session_b"},
		)
		assertForeignKeysEnabled(t, globalDB.db)
	})
}

func TestNetworkConversationConstraints(t *testing.T) {
	t.Parallel()

	t.Run("Should enforce direct room uniqueness and ordered peers", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		assertForeignKeysEnabled(t, globalDB.db)
		registerNetworkConversationSessions(t, globalDB, "coder.sess-abc", "reviewer.sess-xyz")

		insertDirectRoom(
			t,
			globalDB.db,
			networkConversationTestWorkspaceID,
			"builders",
			"direct_0123456789abcdef0123456789abcdef",
			"coder.sess-abc",
			"reviewer.sess-xyz",
		)
		_, err := globalDB.db.ExecContext(
			ctx,
			`INSERT INTO network_direct_rooms (
				workspace_id, channel, direct_id, session_a, session_b, opened_at, last_activity_at
			) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			networkConversationTestWorkspaceID,
			"builders",
			"direct_fedcba9876543210fedcba9876543210",
			"coder.sess-abc",
			"reviewer.sess-xyz",
			"2026-05-05T12:00:00Z",
			"2026-05-05T12:00:00Z",
		)
		requireSQLiteConstraintError(t, err)

		_, err = globalDB.db.ExecContext(
			ctx,
			`INSERT INTO network_direct_rooms (
				workspace_id, channel, direct_id, session_a, session_b, opened_at, last_activity_at
			) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			networkConversationTestWorkspaceID,
			"builders",
			"direct_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"reviewer.sess-xyz",
			"coder.sess-abc",
			"2026-05-05T12:00:00Z",
			"2026-05-05T12:00:00Z",
		)
		requireSQLiteConstraintError(t, err)
	})

	t.Run("Should reject missing work containers and restrict referenced deletes", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		assertForeignKeysEnabled(t, globalDB.db)
		registerNetworkConversationSessions(t, globalDB, "coder.sess-abc", "reviewer.sess-xyz")

		_, err := globalDB.db.ExecContext(
			ctx,
			`INSERT INTO network_work (
				work_id, workspace_id, channel, surface, thread_id, opened_by_session_id, state, opened_at, last_activity_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"work_missing_thread",
			networkConversationTestWorkspaceID,
			"builders",
			store.NetworkSurfaceThread,
			"thread_missing",
			"coder.sess-abc",
			store.NetworkWorkStateSubmitted,
			"2026-05-05T12:00:00Z",
			"2026-05-05T12:00:00Z",
		)
		requireSQLiteConstraintError(t, err)

		insertThread(t, globalDB.db, "builders", "thread_restrict", "msg_root_restrict")
		insertWorkForThread(t, globalDB.db, "work_thread_restrict", "builders", "thread_restrict")
		_, err = globalDB.db.ExecContext(
			ctx,
			`DELETE FROM network_threads WHERE workspace_id = ? AND channel = ? AND thread_id = ?`,
			networkConversationTestWorkspaceID,
			"builders",
			"thread_restrict",
		)
		requireSQLiteConstraintError(t, err)

		insertDirectRoom(
			t,
			globalDB.db,
			networkConversationTestWorkspaceID,
			"builders",
			"direct_0123456789abcdef0123456789abcdef",
			"coder.sess-abc",
			"reviewer.sess-xyz",
		)
		insertWorkForDirect(
			t,
			globalDB.db,
			"work_direct_restrict",
			"builders",
			"direct_0123456789abcdef0123456789abcdef",
		)
		_, err = globalDB.db.ExecContext(
			ctx,
			`DELETE FROM network_direct_rooms WHERE workspace_id = ? AND channel = ? AND direct_id = ?`,
			networkConversationTestWorkspaceID,
			"builders",
			"direct_0123456789abcdef0123456789abcdef",
		)
		requireSQLiteConstraintError(t, err)
	})

	t.Run("Should cascade thread participants when an unreferenced thread is deleted", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		assertForeignKeysEnabled(t, globalDB.db)
		registerNetworkConversationSessions(t, globalDB, "coder.sess-abc")

		insertThread(t, globalDB.db, "builders", "thread_cascade", "msg_root_cascade")
		if _, err := globalDB.db.ExecContext(
			ctx,
			`INSERT INTO network_thread_participants (
				workspace_id, channel, thread_id, session_id, first_message_id, first_seen_at, last_seen_at
			) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			networkConversationTestWorkspaceID,
			"builders",
			"thread_cascade",
			"coder.sess-abc",
			"msg_root_cascade",
			"2026-05-05T12:00:00Z",
			"2026-05-05T12:00:00Z",
		); err != nil {
			t.Fatalf("insert thread participant error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`DELETE FROM network_threads WHERE workspace_id = ? AND channel = ? AND thread_id = ?`,
			networkConversationTestWorkspaceID,
			"builders",
			"thread_cascade",
		); err != nil {
			t.Fatalf("delete unreferenced thread error = %v", err)
		}

		var count int
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM network_thread_participants WHERE workspace_id = ? AND channel = ? AND thread_id = ?`,
			networkConversationTestWorkspaceID,
			"builders",
			"thread_cascade",
		).Scan(&count); err != nil {
			t.Fatalf("count thread participants error = %v", err)
		}
		if count != 0 {
			t.Fatalf("participant count after cascade = %d, want 0", count)
		}
	})
}

func insertThread(t *testing.T, db *sql.DB, channel string, threadID string, rootMessageID string) {
	t.Helper()

	if _, err := db.ExecContext(
		testutil.Context(t),
		`INSERT INTO network_threads (
			workspace_id, channel, thread_id, root_message_id, opened_by_peer_id, opened_at, last_activity_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		networkConversationTestWorkspaceID,
		channel,
		threadID,
		rootMessageID,
		"coder.sess-abc",
		"2026-05-05T12:00:00Z",
		"2026-05-05T12:00:00Z",
	); err != nil {
		t.Fatalf("insert network thread %q error = %v", threadID, err)
	}
}

func insertDirectRoom(
	t *testing.T,
	db *sql.DB,
	workspaceID string,
	channel string,
	directID string,
	sessionA string,
	sessionB string,
) {
	t.Helper()

	if _, err := db.ExecContext(
		testutil.Context(t),
		`INSERT INTO network_direct_rooms (
			workspace_id, channel, direct_id, session_a, session_b, opened_at, last_activity_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		workspaceID,
		channel,
		directID,
		sessionA,
		sessionB,
		"2026-05-05T12:00:00Z",
		"2026-05-05T12:00:00Z",
	); err != nil {
		t.Fatalf("insert network direct room %q error = %v", directID, err)
	}
}

func insertWorkForThread(t *testing.T, db *sql.DB, workID string, channel string, threadID string) {
	t.Helper()

	if _, err := db.ExecContext(
		testutil.Context(t),
		`INSERT INTO network_work (
			work_id, workspace_id, channel, surface, thread_id, opened_by_session_id, state, opened_at, last_activity_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		workID,
		networkConversationTestWorkspaceID,
		channel,
		store.NetworkSurfaceThread,
		threadID,
		"coder.sess-abc",
		store.NetworkWorkStateSubmitted,
		"2026-05-05T12:00:00Z",
		"2026-05-05T12:00:00Z",
	); err != nil {
		t.Fatalf("insert network thread work %q error = %v", workID, err)
	}
}

func insertWorkForDirect(t *testing.T, db *sql.DB, workID string, channel string, directID string) {
	t.Helper()

	if _, err := db.ExecContext(
		testutil.Context(t),
		`INSERT INTO network_work (
			work_id, workspace_id, channel, surface, direct_id, opened_by_session_id, state, opened_at, last_activity_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		workID,
		networkConversationTestWorkspaceID,
		channel,
		store.NetworkSurfaceDirect,
		directID,
		"coder.sess-abc",
		store.NetworkWorkStateSubmitted,
		"2026-05-05T12:00:00Z",
		"2026-05-05T12:00:00Z",
	); err != nil {
		t.Fatalf("insert network direct work %q error = %v", workID, err)
	}
}

func registerNetworkConversationSessions(t *testing.T, globalDB *GlobalDB, sessionIDs ...string) {
	t.Helper()

	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"network-conversation",
		filepath.Join(t.TempDir(), "workspace"),
	)
	now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	for _, sessionID := range sessionIDs {
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
	}
}

func assertForeignKeysEnabled(t *testing.T, db *sql.DB) {
	t.Helper()

	var enabled int
	if err := db.QueryRowContext(testutil.Context(t), `PRAGMA foreign_keys`).Scan(&enabled); err != nil {
		t.Fatalf("PRAGMA foreign_keys error = %v", err)
	}
	if enabled != 1 {
		t.Fatalf("PRAGMA foreign_keys = %d, want 1", enabled)
	}
}

func assertUniqueIndexColumns(t *testing.T, db *sql.DB, table string, want []string) {
	t.Helper()

	rows, err := db.QueryContext(testutil.Context(t), "PRAGMA index_list("+table+")")
	if err != nil {
		t.Fatalf("PRAGMA index_list(%s) error = %v", table, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			t.Fatalf("rows.Close(index_list %s) error = %v", table, closeErr)
		}
	}()

	for rows.Next() {
		var (
			seq     int
			name    string
			unique  int
			origin  string
			partial int
		)
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			t.Fatalf("scan index_list(%s) error = %v", table, err)
		}
		if unique != 1 {
			continue
		}
		if slices.Equal(indexColumns(t, db, name), want) {
			return
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err(index_list %s) error = %v", table, err)
	}
	t.Fatalf("unique index on %s columns %#v not found", table, want)
}

func indexColumns(t *testing.T, db *sql.DB, indexName string) []string {
	t.Helper()

	rows, err := db.QueryContext(testutil.Context(t), "PRAGMA index_info("+indexName+")")
	if err != nil {
		t.Fatalf("PRAGMA index_info(%s) error = %v", indexName, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			t.Fatalf("rows.Close(index_info %s) error = %v", indexName, closeErr)
		}
	}()

	columns := make([]string, 0)
	for rows.Next() {
		var (
			seqno int
			cid   int
			name  string
		)
		if err := rows.Scan(&seqno, &cid, &name); err != nil {
			t.Fatalf("scan index_info(%s) error = %v", indexName, err)
		}
		columns = append(columns, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err(index_info %s) error = %v", indexName, err)
	}
	return columns
}

func requireSQLiteConstraintError(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal("error = nil, want sqlite constraint error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "constraint") {
		t.Fatalf("error = %v, want sqlite constraint failure", err)
	}
}
