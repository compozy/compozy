package globaldb

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

func TestOpenGlobalDBCreatesNetworkTimelineLogSchema(t *testing.T) {
	t.Parallel()

	t.Run("Should create timeline schema with extension metadata column", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)

		assertTablesPresent(t, globalDB.db, "network_timeline_log")
		assertTableColumns(t, globalDB.db, "network_timeline_log", []string{
			"sequence",
			"message_id",
			"session_id",
			"workspace_id",
			"channel",
			"surface",
			"thread_id",
			"direct_id",
			"direction",
			"peer_from",
			"peer_to",
			"kind",
			"work_id",
			"reply_to",
			"trace_id",
			"causation_id",
			"intent",
			"text",
			"preview_text",
			"body_json",
			"timestamp",
			"ext_json",
			"mentions_json",
			"work_opened",
			"work_transitioned",
			"work_state",
		})
		assertTableHasNoForeignKeys(t, globalDB.db, "network_timeline_log")
	})
}

func TestGlobalDBWriteAndListNetworkMessages(t *testing.T) {
	t.Parallel()

	t.Run("Should write and list timeline messages with extension metadata", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "network-messages", t.TempDir())
		recordedAt := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)
		globalDB.now = func() time.Time { return recordedAt }
		if err := globalDB.RegisterSession(testutil.Context(t), SessionInfo{
			ID:          "sess-audit",
			AgentName:   "coder",
			Provider:    "claude",
			WorkspaceID: workspaceID,
			State:       "active",
			CreatedAt:   recordedAt,
			UpdatedAt:   recordedAt,
		}); err != nil {
			t.Fatalf("RegisterSession() error = %v", err)
		}

		if err := globalDB.WriteNetworkMessage(testutil.Context(t), store.NetworkMessageEntry{
			MessageID:   "msg_say_01",
			SessionID:   "sess-audit",
			WorkspaceID: workspaceID,
			Channel:     "builders",
			Surface:     store.NetworkSurfaceThread,
			ThreadID:    "thread_patch_42",
			Direction:   "sent",
			PeerFrom:    "coder.sess-audit",
			Kind:        "say",
			Intent:      "announce",
			Text:        "hello builders",
			PreviewText: "hello builders",
			ExtJSON:     []byte(`{"coordination":{"task_id":"task-1"}}`),
			Body:        []byte(`{"text":"hello builders","intent":"announce"}`),
		}); err != nil {
			t.Fatalf("WriteNetworkMessage(first) error = %v", err)
		}
		if err := globalDB.WriteNetworkMessage(testutil.Context(t), store.NetworkMessageEntry{
			MessageID:   "msg_say_01",
			WorkspaceID: workspaceID,
			Channel:     "builders",
			Surface:     store.NetworkSurfaceThread,
			ThreadID:    "thread_patch_42",
			Direction:   "sent",
			PeerFrom:    "coder.sess-audit",
			Kind:        "say",
			Intent:      "announce",
			Text:        "hello builders",
			PreviewText: "hello builders",
			Body:        []byte(`{"text":"hello builders","intent":"announce"}`),
			Timestamp:   recordedAt.Add(time.Minute),
		}); err != nil {
			t.Fatalf("WriteNetworkMessage(duplicate) error = %v", err)
		}
		if err := globalDB.WriteNetworkMessage(testutil.Context(t), store.NetworkMessageEntry{
			MessageID:   "msg_say_02",
			WorkspaceID: workspaceID,
			Channel:     "builders",
			Surface:     store.NetworkSurfaceDirect,
			DirectID:    "direct_0123456789abcdef0123456789abcdef",
			Direction:   "received",
			PeerFrom:    "reviewer.sess-remote",
			PeerTo:      "coder.sess-audit",
			Kind:        "say",
			WorkID:      "work_patch_42",
			Text:        "review in progress",
			PreviewText: "review in progress",
			Body:        []byte(`{"text":"review in progress"}`),
			Timestamp:   recordedAt.Add(time.Minute),
		}); err != nil {
			t.Fatalf("WriteNetworkMessage(second) error = %v", err)
		}

		entries, err := globalDB.ListNetworkMessages(testutil.Context(t), store.NetworkMessageQuery{
			WorkspaceID: workspaceID,
			Channel:     "builders",
			Limit:       10,
		})
		if err != nil {
			t.Fatalf("ListNetworkMessages() error = %v", err)
		}
		if got, want := len(entries), 2; got != want {
			t.Fatalf("len(entries) = %d, want %d", got, want)
		}
		if got, want := entries[0].MessageID, "msg_say_01"; got != want {
			t.Fatalf("entries[0].MessageID = %q, want %q", got, want)
		}
		if got, want := entries[0].Intent, "announce"; got != want {
			t.Fatalf("entries[0].Intent = %q, want %q", got, want)
		}
		if got, want := entries[0].Direction, "sent"; got != want {
			t.Fatalf("entries[0].Direction = %q, want %q", got, want)
		}
		if got, want := string(entries[0].ExtJSON), `{"coordination":{"task_id":"task-1"}}`; got != want {
			t.Fatalf("entries[0].ExtJSON = %q, want %q", got, want)
		}
		if got, want := entries[0].Timestamp, recordedAt; !got.Equal(want) {
			t.Fatalf("entries[0].Timestamp = %s, want %s", got, want)
		}
		if got, want := entries[1].PeerFrom, "reviewer.sess-remote"; got != want {
			t.Fatalf("entries[1].PeerFrom = %q, want %q", got, want)
		}
		if got, want := entries[1].PeerTo, "coder.sess-audit"; got != want {
			t.Fatalf("entries[1].PeerTo = %q, want %q", got, want)
		}
		if got, want := entries[1].Surface, store.NetworkSurfaceDirect; got != want {
			t.Fatalf("entries[1].Surface = %q, want %q", got, want)
		}
		if got, want := entries[1].DirectID, "direct_0123456789abcdef0123456789abcdef"; got != want {
			t.Fatalf("entries[1].DirectID = %q, want %q", got, want)
		}
		if got, want := entries[1].WorkID, "work_patch_42"; got != want {
			t.Fatalf("entries[1].WorkID = %q, want %q", got, want)
		}
		if got, want := string(entries[1].Body), `{"text":"review in progress"}`; got != want {
			t.Fatalf("entries[1].Body = %q, want %q", got, want)
		}
	})
}

func TestGlobalDBListNetworkMessagesSupportsMessageIDCursors(t *testing.T) {
	t.Parallel()

	t.Run("Should support message id cursors within query scope", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "network-cursors", t.TempDir())
		recordedAt := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)

		entries := []store.NetworkMessageEntry{
			{
				MessageID:   "msg-1",
				Channel:     "builders",
				Surface:     store.NetworkSurfaceThread,
				ThreadID:    "thread_cursor",
				Direction:   "sent",
				PeerFrom:    "peer-a",
				Kind:        "say",
				PreviewText: "one",
				Body:        []byte(`{"text":"one"}`),
				Timestamp:   recordedAt,
			},
			{
				MessageID:   "msg-2z",
				Channel:     "builders",
				Surface:     store.NetworkSurfaceThread,
				ThreadID:    "thread_cursor",
				Direction:   "sent",
				PeerFrom:    "peer-a",
				Kind:        "say",
				PreviewText: "two z",
				Body:        []byte(`{"text":"two z"}`),
				Timestamp:   recordedAt.Add(time.Minute),
			},
			{
				MessageID:   "msg-2a",
				Channel:     "builders",
				Surface:     store.NetworkSurfaceThread,
				ThreadID:    "thread_cursor",
				Direction:   "sent",
				PeerFrom:    "peer-a",
				Kind:        "say",
				PreviewText: "two a",
				Body:        []byte(`{"text":"two a"}`),
				Timestamp:   recordedAt.Add(time.Minute),
			},
			{
				MessageID:   "msg-3",
				Channel:     "builders",
				Surface:     store.NetworkSurfaceDirect,
				DirectID:    "direct_0123456789abcdef0123456789abcdef",
				Direction:   "received",
				PeerFrom:    "peer-b",
				PeerTo:      "peer-a",
				Kind:        "say",
				PreviewText: "three",
				Body:        []byte(`{"text":"three"}`),
				Timestamp:   recordedAt.Add(2 * time.Minute),
			},
			{
				MessageID:   "msg-4",
				Channel:     "retro",
				Surface:     store.NetworkSurfaceDirect,
				DirectID:    "direct_fedcba9876543210fedcba9876543210",
				Direction:   "received",
				PeerFrom:    "peer-c",
				PeerTo:      "peer-d",
				Kind:        "say",
				PreviewText: "four",
				Body:        []byte(`{"text":"four"}`),
				Timestamp:   recordedAt.Add(3 * time.Minute),
			},
		}
		for index := range entries {
			entries[index].WorkspaceID = workspaceID
		}
		for _, entry := range entries {
			if err := globalDB.WriteNetworkMessage(testutil.Context(t), entry); err != nil {
				t.Fatalf("WriteNetworkMessage(%q) error = %v", entry.MessageID, err)
			}
		}

		latest, err := globalDB.ListNetworkMessages(testutil.Context(t), store.NetworkMessageQuery{
			WorkspaceID: workspaceID,
			Channel:     "builders",
			Limit:       2,
		})
		if err != nil {
			t.Fatalf("ListNetworkMessages(latest) error = %v", err)
		}
		if got, want := networkMessageIDs(latest), []string{"msg-2a", "msg-3"}; !sameStrings(got, want) {
			t.Fatalf("latest message IDs = %v, want %v", got, want)
		}

		latestPublic, err := globalDB.ListNetworkMessages(testutil.Context(t), store.NetworkMessageQuery{
			WorkspaceID: workspaceID,
			Channel:     "builders",
			PublicOnly:  true,
			Limit:       2,
		})
		if err != nil {
			t.Fatalf("ListNetworkMessages(latest public) error = %v", err)
		}
		if got, want := networkMessageIDs(latestPublic), []string{"msg-2z", "msg-2a"}; !sameStrings(got, want) {
			t.Fatalf("latest public message IDs = %v, want %v", got, want)
		}

		before, err := globalDB.ListNetworkMessages(testutil.Context(t), store.NetworkMessageQuery{
			WorkspaceID:     workspaceID,
			Channel:         "builders",
			BeforeMessageID: "msg-2a",
			Limit:           10,
		})
		if err != nil {
			t.Fatalf("ListNetworkMessages(before) error = %v", err)
		}
		if got, want := len(before), 2; got != want {
			t.Fatalf("len(before) = %d, want %d", got, want)
		}
		if got, want := before[0].MessageID, "msg-1"; got != want {
			t.Fatalf("before[0].MessageID = %q, want %q", got, want)
		}
		if got, want := before[1].MessageID, "msg-2z"; got != want {
			t.Fatalf("before[1].MessageID = %q, want %q", got, want)
		}

		after, err := globalDB.ListNetworkMessages(testutil.Context(t), store.NetworkMessageQuery{
			WorkspaceID:    workspaceID,
			Channel:        "builders",
			AfterMessageID: "msg-2z",
			Limit:          10,
		})
		if err != nil {
			t.Fatalf("ListNetworkMessages(after) error = %v", err)
		}
		if got, want := len(after), 2; got != want {
			t.Fatalf("len(after) = %d, want %d", got, want)
		}
		if got, want := after[0].MessageID, "msg-2a"; got != want {
			t.Fatalf("after[0].MessageID = %q, want %q", got, want)
		}
		if got, want := after[1].MessageID, "msg-3"; got != want {
			t.Fatalf("after[1].MessageID = %q, want %q", got, want)
		}

		_, err = globalDB.ListNetworkMessages(testutil.Context(t), store.NetworkMessageQuery{
			WorkspaceID:     workspaceID,
			Channel:         "builders",
			BeforeMessageID: "msg-4",
			Limit:           10,
		})
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("ListNetworkMessages(cross-channel cursor) error = %v, want sql.ErrNoRows", err)
		}

		_, err = globalDB.ListNetworkMessages(testutil.Context(t), store.NetworkMessageQuery{
			WorkspaceID:     workspaceID,
			PeerID:          "peer-a",
			DirectedOnly:    true,
			BeforeMessageID: "msg-4",
			Limit:           10,
		})
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("ListNetworkMessages(cross-peer cursor) error = %v, want sql.ErrNoRows", err)
		}
	})
}

func networkMessageIDs(messages []store.NetworkMessageEntry) []string {
	ids := make([]string, 0, len(messages))
	for _, message := range messages {
		ids = append(ids, message.MessageID)
	}
	return ids
}

func TestGlobalDBNetworkMessageGuardClauses(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	if err := globalDB.Close(testutil.Context(t)); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	tests := []struct {
		name string
		run  func() error
		want error
	}{
		{
			name: "Should reject writes with a nil context",
			run: func() error {
				freshDB := openTestGlobalDB(t)
				defer func() {
					if closeErr := freshDB.Close(testutil.Context(t)); closeErr != nil {
						t.Errorf("Close() error = %v", closeErr)
					}
				}()
				return freshDB.WriteNetworkMessage(nilGlobalContext(), store.NetworkMessageEntry{})
			},
		},
		{
			name: "Should reject reads with a nil context",
			run: func() error {
				freshDB := openTestGlobalDB(t)
				defer func() {
					if closeErr := freshDB.Close(testutil.Context(t)); closeErr != nil {
						t.Errorf("Close() error = %v", closeErr)
					}
				}()
				_, err := freshDB.ListNetworkMessages(nilGlobalContext(), store.NetworkMessageQuery{})
				return err
			},
		},
		{
			name: "Should reject writes after the store is closed",
			run: func() error {
				return globalDB.WriteNetworkMessage(testutil.Context(t), store.NetworkMessageEntry{})
			},
			want: store.ErrClosed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			if tt.want != nil {
				if !errors.Is(err, tt.want) {
					t.Fatalf("error = %v, want %v", err, tt.want)
				}
				return
			}
			if err == nil {
				t.Fatal("error = nil, want non-nil")
			}
		})
	}
}

func TestGlobalDBListNetworkMessagesWrapsTimestampParseFailures(t *testing.T) {
	t.Parallel()

	t.Run("Should wrap timestamp parse failures", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "network-bad-timestamp", t.TempDir())
		if _, err := globalDB.db.ExecContext(
			testutil.Context(t),
			`INSERT INTO network_timeline_log (
			message_id,
			session_id,
			workspace_id,
			channel,
			surface,
			thread_id,
			direct_id,
			direction,
			peer_from,
			peer_to,
			kind,
			work_id,
			reply_to,
			trace_id,
			causation_id,
			intent,
			text,
			preview_text,
			body_json,
			timestamp
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"msg_bad_timestamp",
			nil,
			workspaceID,
			"builders",
			store.NetworkSurfaceThread,
			"thread_bad_timestamp",
			nil,
			"sent",
			"coder.sess-audit",
			nil,
			"say",
			nil,
			nil,
			nil,
			nil,
			nil,
			"hello",
			"hello",
			`{"text":"hello"}`,
			"not-a-timestamp",
		); err != nil {
			t.Fatalf("ExecContext(insert invalid network message) error = %v", err)
		}

		_, err := globalDB.ListNetworkMessages(testutil.Context(t), store.NetworkMessageQuery{
			WorkspaceID: workspaceID,
			Channel:     "builders",
		})
		if err == nil {
			t.Fatal("ListNetworkMessages(invalid timestamp) error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "parse network message timestamp") {
			t.Fatalf("ListNetworkMessages(invalid timestamp) error = %v, want wrapped timestamp parse context", err)
		}
	})
}

func TestGlobalDBWriteNetworkMessageRejectsNonCanonicalDirection(t *testing.T) {
	t.Parallel()

	t.Run("Should reject non-canonical directions", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "network-bad-direction", t.TempDir())
		err := globalDB.WriteNetworkMessage(testutil.Context(t), store.NetworkMessageEntry{
			MessageID:   "msg_bad_direction",
			WorkspaceID: workspaceID,
			Channel:     "builders",
			Surface:     store.NetworkSurfaceThread,
			ThreadID:    "thread_bad_direction",
			Direction:   " sent ",
			PeerFrom:    "coder.sess-audit",
			Kind:        "say",
			PreviewText: "hello",
			Body:        []byte(`{"text":"hello"}`),
		})
		if err == nil {
			t.Fatal("WriteNetworkMessage() error = nil, want non-canonical direction rejection")
		}
		if !strings.Contains(err.Error(), `unsupported network message direction " sent "`) {
			t.Fatalf("WriteNetworkMessage() error = %v, want direction validation error", err)
		}
	})
}
