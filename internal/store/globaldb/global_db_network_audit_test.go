package globaldb

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

func TestOpenGlobalDBCreatesNetworkAuditLogSchema(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)

	assertTablesPresent(t, globalDB.db, "network_audit_log")
	assertTableColumns(t, globalDB.db, "network_audit_log", []string{
		"id",
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
	assertTableHasNoForeignKeys(t, globalDB.db, "network_audit_log")
}

func TestGlobalDBWriteAndListNetworkAudit(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	workspaceID := registerSessionForGlobalTests(t, globalDB, "sess-network-audit")

	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	globalDB.now = func() time.Time { return now }

	if err := globalDB.WriteNetworkAudit(testutil.Context(t), store.NetworkAuditEntry{
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
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"network-audit-unknown",
		filepath.Join(t.TempDir(), "network-audit-unknown"),
	)

	if err := globalDB.WriteNetworkAudit(testutil.Context(t), store.NetworkAuditEntry{
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

	entries, err := globalDB.ListNetworkAudit(testutil.Context(t), store.NetworkAuditQuery{
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
}

func TestGlobalDBNetworkAuditGuardClauses(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	if err := globalDB.WriteNetworkAudit(nilGlobalContext(), store.NetworkAuditEntry{}); err == nil {
		t.Fatal("WriteNetworkAudit(nil ctx) error = nil, want non-nil")
	}
	if _, err := globalDB.ListNetworkAudit(nilGlobalContext(), store.NetworkAuditQuery{}); err == nil {
		t.Fatal("ListNetworkAudit(nil ctx) error = nil, want non-nil")
	}
	if err := globalDB.Close(testutil.Context(t)); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := globalDB.WriteNetworkAudit(
		testutil.Context(t),
		store.NetworkAuditEntry{},
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
			id, session_id, workspace_id, direction, kind, channel, surface, thread_id, direct_id, work_id,
			peer_from, peer_to, message_id, reason, size, timestamp
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"naud_bad_timestamp",
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
