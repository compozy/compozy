package globaldb

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestOpenGlobalDBCreatesNetworkAuditLogSchema(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)

	assertTablesPresent(t, globalDB.db, "network_audit_log")
	assertTableColumns(t, globalDB.db, "network_audit_log", []string{
		"id",
		"profile_id",
		"session_id",
		"workspace_id",
		"direction",
		"kind",
		"channel",
		"surface",
		"thread_id",
		"direct_id",
		"work_id",
		"peer_from",
		"peer_to",
		"message_id",
		"reason",
		"size",
		"timestamp",
	})
	hasProfileForeignKey, err := tableHasForeignKey(
		testutil.Context(t), globalDB.db, "network_audit_log", "profiles",
	)
	if err != nil {
		t.Fatalf("tableHasForeignKey(network_audit_log, profiles) error = %v", err)
	}
	if !hasProfileForeignKey {
		t.Fatal("network_audit_log profile foreign key = false, want true")
	}
}

func TestGlobalDBWriteAndListNetworkAudit(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	workspaceID := registerSessionForGlobalTests(t, globalDB, "sess-network-audit")

	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	globalDB.now = func() time.Time { return now }

	if err := globalDB.WriteNetworkAudit(testutil.Context(t), store.NetworkAuditEntry{
		ProfileID:   store.DefaultProfileID,
		SessionID:   "sess-network-audit",
		WorkspaceID: workspaceID,
		Direction:   "sent",
		Kind:        "say",
		Channel:     "builders",
		Surface:     store.NetworkSurfaceDirect,
		DirectID:    "direct_0123456789abcdef0123456789abcdef",
		WorkID:      "work_patch_42",
		PeerFrom:    "coder.sess-network-audit",
		PeerTo:      "reviewer.sess-xyz",
		MessageID:   "msg_direct_01",
		Size:        128,
	}); err != nil {
		t.Fatalf("WriteNetworkAudit(sent) error = %v", err)
	}

	if err := globalDB.WriteNetworkAudit(testutil.Context(t), store.NetworkAuditEntry{
		ProfileID:   store.DefaultProfileID,
		SessionID:   "sess-network-audit",
		WorkspaceID: workspaceID,
		Direction:   "rejected",
		Kind:        "receipt",
		Channel:     "builders",
		PeerFrom:    "reviewer.sess-xyz",
		MessageID:   "msg_receipt_01",
		Reason:      "not_found",
		Size:        64,
		Timestamp:   now.Add(time.Minute),
	}); err != nil {
		t.Fatalf("WriteNetworkAudit(rejected) error = %v", err)
	}

	entries, err := globalDB.ListNetworkAudit(testutil.Context(t), store.NetworkAuditQuery{
		ReadScope:   store.ReadScope{AllProfiles: true},
		WorkspaceID: workspaceID,
		SessionID:   "sess-network-audit",
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("ListNetworkAudit() error = %v", err)
	}
	if got, want := len(entries), 2; got != want {
		t.Fatalf("len(entries) = %d, want %d", got, want)
	}

	if got, want := entries[0].Direction, "sent"; got != want {
		t.Fatalf("entries[0].Direction = %q, want %q", got, want)
	}
	if got, want := entries[0].Timestamp, now; !got.Equal(want) {
		t.Fatalf("entries[0].Timestamp = %s, want %s", got, want)
	}
	if got, want := entries[0].PeerTo, "reviewer.sess-xyz"; got != want {
		t.Fatalf("entries[0].PeerTo = %q, want %q", got, want)
	}
	if got, want := entries[0].Surface, store.NetworkSurfaceDirect; got != want {
		t.Fatalf("entries[0].Surface = %q, want %q", got, want)
	}
	if got, want := entries[0].DirectID, "direct_0123456789abcdef0123456789abcdef"; got != want {
		t.Fatalf("entries[0].DirectID = %q, want %q", got, want)
	}
	if got, want := entries[0].WorkID, "work_patch_42"; got != want {
		t.Fatalf("entries[0].WorkID = %q, want %q", got, want)
	}

	if got, want := entries[1].Direction, "rejected"; got != want {
		t.Fatalf("entries[1].Direction = %q, want %q", got, want)
	}
	if got, want := entries[1].Reason, "not_found"; got != want {
		t.Fatalf("entries[1].Reason = %q, want %q", got, want)
	}
}

func TestGlobalDBWriteNetworkAuditAllowsUnknownSessionID(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	ctx := testutil.Context(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"network-audit-unknown",
		filepath.Join(t.TempDir(), "network-audit-unknown"),
	)
	foreignProfileID := "01ARZ3NDEKTSV4RRFFQ69G5FAX"
	if _, err := globalDB.db.ExecContext(ctx, `
		INSERT INTO profiles (id, name, color, icon, state, created_at)
		VALUES (?, 'network-foreign', '#8E8EB5', 'circle', 'active', ?)`,
		foreignProfileID,
		store.FormatTimestamp(time.Now().UTC()),
	); err != nil {
		t.Fatalf("insert foreign profile: %v", err)
	}

	if err := globalDB.WriteNetworkAudit(ctx, store.NetworkAuditEntry{
		ProfileID:   store.DefaultProfileID,
		SessionID:   "sess-network-unknown",
		WorkspaceID: workspaceID,
		Direction:   "sent",
		Kind:        "greet",
		Channel:     "builders",
		PeerFrom:    "coder.sess-network-unknown",
		MessageID:   "msg_greet_01",
		Size:        32,
	}); err != nil {
		t.Fatalf("WriteNetworkAudit(unknown session) error = %v", err)
	}

	entries, err := globalDB.ListNetworkAudit(ctx, store.NetworkAuditQuery{
		ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
		WorkspaceID: workspaceID,
		SessionID:   "sess-network-unknown",
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("ListNetworkAudit(unknown session) error = %v", err)
	}
	if got, want := len(entries), 1; got != want {
		t.Fatalf("len(entries) = %d, want %d", got, want)
	}
	if got, want := entries[0].MessageID, "msg_greet_01"; got != want {
		t.Fatalf("entries[0].MessageID = %q, want %q", got, want)
	}
	if got, want := entries[0].ProfileID, store.DefaultProfileID; got != want {
		t.Fatalf("entries[0].ProfileID = %q, want %q", got, want)
	}
	foreignEntries, err := globalDB.ListNetworkAudit(ctx, store.NetworkAuditQuery{
		ReadScope:   store.ReadScope{ProfileID: foreignProfileID},
		WorkspaceID: workspaceID,
		SessionID:   "sess-network-unknown",
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("ListNetworkAudit(foreign profile) error = %v", err)
	}
	if len(foreignEntries) != 0 {
		t.Fatalf("ListNetworkAudit(foreign profile) = %#v, want empty", foreignEntries)
	}
}

func TestGlobalDBNetworkAuditProfileOwnershipMigration(t *testing.T) {
	t.Parallel()

	t.Run("Should backfill the channel owner without widening visibility", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefixDB, err := openGlobalMigrationPrefixDatabase(
			t,
			path,
			globalMigrationPrefixBefore(t, "00089_schema.sql"),
		)
		if err != nil {
			t.Fatalf("OpenSQLiteDatabase(v88 prefix) error = %v", err)
		}
		prefixClosed := false
		t.Cleanup(func() {
			if prefixClosed {
				return
			}
			if err := prefixDB.Close(); err != nil {
				t.Errorf("prefixDB.Close(cleanup) error = %v", err)
			}
		})

		ctx := testutil.Context(t)
		profileID := "01ARZ3NDEKTSV4RRFFQ69G5FAX"
		workspaceID := "ws-network-audit-upgrade"
		timestamp := store.FormatTimestamp(time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC))
		if _, err := prefixDB.ExecContext(ctx, `
			INSERT INTO profiles (id, name, color, icon, state, created_at)
			VALUES (?, 'network-owner', '#8E8EB5', 'circle', 'active', ?)`,
			profileID,
			timestamp,
		); err != nil {
			t.Fatalf("insert profile owner error = %v", err)
		}
		if _, err := prefixDB.ExecContext(ctx, `
			INSERT INTO workspaces (id, root_dir, name, created_at, updated_at)
			VALUES (?, ?, 'network-audit-upgrade', ?, ?)`,
			workspaceID,
			filepath.Join(t.TempDir(), "network-audit-upgrade"),
			timestamp,
			timestamp,
		); err != nil {
			t.Fatalf("insert workspace error = %v", err)
		}
		if _, err := prefixDB.ExecContext(ctx, `
			INSERT INTO network_channels (
				profile_id, workspace_id, channel, purpose, created_at, updated_at
			) VALUES (?, ?, 'builders', 'migration ownership', ?, ?)`,
			profileID,
			workspaceID,
			timestamp,
			timestamp,
		); err != nil {
			t.Fatalf("insert network channel error = %v", err)
		}
		if _, err := prefixDB.ExecContext(ctx, `
			INSERT INTO network_audit_log (
				id, session_id, workspace_id, direction, kind, channel, peer_from,
				message_id, size, timestamp
			) VALUES (
				'naud-profile-backfill', 'sess-ownerless', ?, 'sent', 'say', 'builders',
				'coder.sess-ownerless', 'msg-profile-backfill', 42, ?
			)`,
			workspaceID,
			timestamp,
		); err != nil {
			t.Fatalf("insert legacy audit row error = %v", err)
		}
		if err := prefixDB.Close(); err != nil {
			t.Fatalf("prefixDB.Close() error = %v", err)
		}
		prefixClosed = true

		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(v89 upgrade) error = %v", err)
		}
		t.Cleanup(func() {
			if err := upgraded.Close(testutil.Context(t)); err != nil {
				t.Errorf("upgraded.Close(cleanup) error = %v", err)
			}
		})

		entries, err := upgraded.ListNetworkAudit(testutil.Context(t), store.NetworkAuditQuery{
			ReadScope:   store.ReadScope{ProfileID: profileID},
			WorkspaceID: workspaceID,
			Limit:       10,
		})
		if err != nil {
			t.Fatalf("ListNetworkAudit(owner profile) error = %v", err)
		}
		if got, want := len(entries), 1; got != want {
			t.Fatalf("len(owner entries) = %d, want %d", got, want)
		}
		if got, want := entries[0].ProfileID, profileID; got != want {
			t.Fatalf("entries[0].ProfileID = %q, want %q", got, want)
		}

		defaultEntries, err := upgraded.ListNetworkAudit(testutil.Context(t), store.NetworkAuditQuery{
			ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
			WorkspaceID: workspaceID,
			Limit:       10,
		})
		if err != nil {
			t.Fatalf("ListNetworkAudit(default profile) error = %v", err)
		}
		if len(defaultEntries) != 0 {
			t.Fatalf("ListNetworkAudit(default profile) = %#v, want empty", defaultEntries)
		}
	})
}

func TestGlobalDBNetworkAuditGuardClauses(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	if err := globalDB.WriteNetworkAudit(
		nilGlobalContext(),
		store.NetworkAuditEntry{ProfileID: store.DefaultProfileID},
	); err == nil {
		t.Fatal("WriteNetworkAudit(nil ctx) error = nil, want non-nil")
	}
	if _, err := globalDB.ListNetworkAudit(nilGlobalContext(), store.NetworkAuditQuery{
		ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID},
	}); err == nil {
		t.Fatal("ListNetworkAudit(nil ctx) error = nil, want non-nil")
	}
	if err := globalDB.Close(testutil.Context(t)); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := globalDB.WriteNetworkAudit(
		testutil.Context(t),
		store.NetworkAuditEntry{ProfileID: store.DefaultProfileID},
	); !errors.Is(
		err,
		store.ErrClosed,
	) {
		t.Fatalf("WriteNetworkAudit(after close) error = %v, want ErrClosed", err)
	}
}

func TestGlobalDBWriteNetworkAuditRejectsWhitechannelPaddedDirection(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	workspaceID := registerSessionForGlobalTests(t, globalDB, "sess-network-direction")

	err := globalDB.WriteNetworkAudit(testutil.Context(t), store.NetworkAuditEntry{
		ProfileID:   store.DefaultProfileID,
		SessionID:   "sess-network-direction",
		WorkspaceID: workspaceID,
		Direction:   " sent ",
		Kind:        "direct",
		Channel:     "builders",
		PeerFrom:    "coder.sess-network-direction",
		MessageID:   "msg_direction_01",
		Size:        12,
	})
	if err == nil {
		t.Fatal("WriteNetworkAudit(whitespace direction) error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "must not contain surrounding whitespace") {
		t.Fatalf("WriteNetworkAudit(whitespace direction) error = %v, want whitespace validation context", err)
	}
}

func TestGlobalDBListNetworkAuditWrapsTimestampParseFailures(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	workspaceID := registerSessionForGlobalTests(t, globalDB, "sess-network-bad-timestamp")

	if _, err := globalDB.db.ExecContext(
		testutil.Context(t),
		`INSERT INTO network_audit_log (
			id, profile_id, session_id, workspace_id, direction, kind, channel, surface, thread_id, direct_id, work_id,
			peer_from, peer_to, message_id, reason, size, timestamp
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"naud_bad_timestamp",
		store.DefaultProfileID,
		"sess-network-bad-timestamp",
		workspaceID,
		"sent",
		"say",
		"builders",
		store.NetworkSurfaceThread,
		"thread_bad_timestamp",
		nil,
		nil,
		"coder.sess-network-bad-timestamp",
		nil,
		"msg_bad_timestamp_01",
		nil,
		1,
		"not-a-timestamp",
	); err != nil {
		t.Fatalf("ExecContext(insert invalid audit row) error = %v", err)
	}

	_, err := globalDB.ListNetworkAudit(testutil.Context(t), store.NetworkAuditQuery{
		ReadScope:   store.ReadScope{AllProfiles: true},
		WorkspaceID: workspaceID,
		SessionID:   "sess-network-bad-timestamp",
		Limit:       10,
	})
	if err == nil {
		t.Fatal("ListNetworkAudit(invalid timestamp) error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "parse network audit timestamp") {
		t.Fatalf("ListNetworkAudit(invalid timestamp) error = %v, want wrapped timestamp parse context", err)
	}
}
