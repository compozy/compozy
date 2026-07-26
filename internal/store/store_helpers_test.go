package store

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/testutil"
)

func TestValidationHelpersAndPathUtilities(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 3, 20, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		validate  func() error
		wantError bool
	}{
		{
			name: "session event valid",
			validate: func() error {
				return (SessionEvent{TurnID: "turn-1", Type: "agent_message", AgentName: "coder"}).Validate()
			},
		},
		{
			name: "session event invalid",
			validate: func() error {
				return (SessionEvent{}).Validate()
			},
			wantError: true,
		},
		{
			name: "session event missing type",
			validate: func() error {
				return (SessionEvent{TurnID: "turn-1"}).Validate()
			},
			wantError: true,
		},
		{
			name: "session event missing agent",
			validate: func() error {
				return (SessionEvent{TurnID: "turn-1", Type: "agent_message"}).Validate()
			},
			wantError: true,
		},
		{
			name: "event query valid",
			validate: func() error {
				return (EventQuery{Limit: 1, AfterSequence: 1}).Validate()
			},
		},
		{
			name: "event query invalid",
			validate: func() error {
				return (EventQuery{Limit: -1}).Validate()
			},
			wantError: true,
		},
		{
			name: "token usage valid",
			validate: func() error {
				return (TokenUsage{TurnID: "turn-1"}).Validate()
			},
		},
		{
			name: "token usage invalid",
			validate: func() error {
				return (TokenUsage{}).Validate()
			},
			wantError: true,
		},
		{
			name: "session info valid",
			validate: func() error {
				return (SessionInfo{ID: "sess-1", AgentName: "coder", WorkspaceID: "ws-1", State: "active"}).Validate()
			},
		},
		{
			name: "session info invalid",
			validate: func() error {
				return (SessionInfo{}).Validate()
			},
			wantError: true,
		},
		{
			name: "session info missing agent",
			validate: func() error {
				return (SessionInfo{ID: "sess-1"}).Validate()
			},
			wantError: true,
		},
		{
			name: "session info missing workspace",
			validate: func() error {
				return (SessionInfo{ID: "sess-1", AgentName: "coder"}).Validate()
			},
			wantError: true,
		},
		{
			name: "session info missing state",
			validate: func() error {
				return (SessionInfo{ID: "sess-1", AgentName: "coder", WorkspaceID: "ws-1"}).Validate()
			},
			wantError: true,
		},
		{
			name: "session list query invalid",
			validate: func() error {
				return (SessionListQuery{Limit: -1}).Validate()
			},
			wantError: true,
		},
		{
			name: "session state update valid",
			validate: func() error {
				return (SessionStateUpdate{ID: "sess-1", State: "stopped"}).Validate()
			},
		},
		{
			name: "session state update invalid",
			validate: func() error {
				return (SessionStateUpdate{}).Validate()
			},
			wantError: true,
		},
		{
			name: "session state update missing state",
			validate: func() error {
				return (SessionStateUpdate{ID: "sess-1"}).Validate()
			},
			wantError: true,
		},
		{
			name: "event summary valid",
			validate: func() error {
				return (EventSummary{
					SessionID:   "sess-1",
					WorkspaceID: "ws-store-helpers",
					Type:        "agent_message",
					AgentName:   "coder",
				}).Validate()
			},
		},
		{
			name: "event summary invalid",
			validate: func() error {
				return (EventSummary{}).Validate()
			},
			wantError: true,
		},
		{
			name: "event summary missing type",
			validate: func() error {
				return (EventSummary{SessionID: "sess-1", WorkspaceID: "ws-store-helpers"}).Validate()
			},
			wantError: true,
		},
		{
			name: "event summary missing session",
			validate: func() error {
				return (EventSummary{Type: "agent_message", AgentName: "coder"}).Validate()
			},
			wantError: true,
		},
		{
			name: "event summary missing agent",
			validate: func() error {
				return (EventSummary{
					SessionID:   "sess-1",
					WorkspaceID: "ws-store-helpers",
					Type:        "agent_message",
				}).Validate()
			},
			wantError: true,
		},
		{
			name: "global event summary settings changed",
			validate: func() error {
				return (EventSummary{Type: "settings.changed"}).Validate()
			},
		},
		{
			name: "global event summary skill shadowed",
			validate: func() error {
				return (EventSummary{Type: "skill.shadowed"}).Validate()
			},
		},
		{
			name: "event summary rejects task run dot family",
			validate: func() error {
				return (EventSummary{
					SessionID:   "sess-1",
					WorkspaceID: "ws-store-helpers",
					Type:        "task_run.completed",
					AgentName:   "coder",
				}).Validate()
			},
			wantError: true,
		},
		{
			name: "global event summary skills load failed",
			validate: func() error {
				return (EventSummary{Type: "skills.load_failed"}).Validate()
			},
		},
		{
			name: "global event summary hook dispatch start",
			validate: func() error {
				return (EventSummary{Type: "hook.dispatch.start"}).Validate()
			},
		},
		{
			name: "global event summary hook dispatch complete",
			validate: func() error {
				return (EventSummary{Type: "hook.dispatch.complete"}).Validate()
			},
		},
		{
			name: "global event summary memory provider collision",
			validate: func() error {
				return (EventSummary{Type: "memory.provider.collision"}).Validate()
			},
		},
		{
			name: "event summary query invalid",
			validate: func() error {
				return (EventSummaryQuery{Limit: -1}).Validate()
			},
			wantError: true,
		},
		{
			name: "token stats update valid",
			validate: func() error {
				return (TokenStatsUpdate{
					SessionID:  "sess-1",
					AgentName:  "coder",
					CostStatus: "unknown",
					CostSource: "none",
				}).Validate()
			},
		},
		{
			name: "token stats update invalid",
			validate: func() error {
				return (TokenStatsUpdate{}).Validate()
			},
			wantError: true,
		},
		{
			name: "token stats update missing agent",
			validate: func() error {
				return (TokenStatsUpdate{SessionID: "sess-1"}).Validate()
			},
			wantError: true,
		},
		{
			name: "token stats query invalid",
			validate: func() error {
				return (TokenStatsQuery{Limit: -1}).Validate()
			},
			wantError: true,
		},
		{
			name: "permission log entry valid",
			validate: func() error {
				return (PermissionLogEntry{
					SessionID:  "sess-1",
					AgentName:  "coder",
					Action:     "bash",
					Resource:   "/tmp",
					Decision:   "allow",
					PolicyUsed: "approve-reads",
				}).Validate()
			},
		},
		{
			name: "permission log entry invalid",
			validate: func() error {
				return (PermissionLogEntry{}).Validate()
			},
			wantError: true,
		},
		{
			name: "permission log entry missing agent",
			validate: func() error {
				return (PermissionLogEntry{SessionID: "sess-1"}).Validate()
			},
			wantError: true,
		},
		{
			name: "permission log entry missing action",
			validate: func() error {
				return (PermissionLogEntry{SessionID: "sess-1", AgentName: "coder"}).Validate()
			},
			wantError: true,
		},
		{
			name: "permission log entry missing resource",
			validate: func() error {
				return (PermissionLogEntry{SessionID: "sess-1", AgentName: "coder", Action: "bash"}).Validate()
			},
			wantError: true,
		},
		{
			name: "permission log entry missing decision",
			validate: func() error {
				return (PermissionLogEntry{SessionID: "sess-1", AgentName: "coder", Action: "bash", Resource: "/tmp"}).Validate()
			},
			wantError: true,
		},
		{
			name: "permission log entry missing policy",
			validate: func() error {
				return (PermissionLogEntry{SessionID: "sess-1", AgentName: "coder", Action: "bash", Resource: "/tmp", Decision: "allow"}).Validate()
			},
			wantError: true,
		},
		{
			name: "permission log query invalid",
			validate: func() error {
				return (PermissionLogQuery{Limit: -1}).Validate()
			},
			wantError: true,
		},
		{
			name: "token usage valid",
			validate: func() error {
				return (TokenUsage{TurnID: "turn-1"}).Validate()
			},
		},
		{
			name: "token usage invalid",
			validate: func() error {
				return (TokenUsage{}).Validate()
			},
			wantError: true,
		},
		{
			name: "network audit entry valid",
			validate: func() error {
				return (NetworkAuditEntry{
					SessionID:   "sess-1",
					WorkspaceID: "ws-store-helpers",
					Direction:   "rejected",
					Kind:        "message",
					Channel:     "builders",
					PeerFrom:    "peer-a",
					MessageID:   "msg-1",
					Reason:      "policy",
					Size:        0,
				}).Validate()
			},
		},
		{
			name: "network audit entry invalid direction",
			validate: func() error {
				return (NetworkAuditEntry{
					SessionID:   "sess-1",
					WorkspaceID: "ws-store-helpers",
					Direction:   "replayed",
					Kind:        "message",
					Channel:     "builders",
					PeerFrom:    "peer-a",
					MessageID:   "msg-1",
				}).Validate()
			},
			wantError: true,
		},
		{
			name: "network audit entry rejected requires reason",
			validate: func() error {
				return (NetworkAuditEntry{
					SessionID:   "sess-1",
					WorkspaceID: "ws-store-helpers",
					Direction:   "rejected",
					Kind:        "message",
					Channel:     "builders",
					PeerFrom:    "peer-a",
					MessageID:   "msg-1",
				}).Validate()
			},
			wantError: true,
		},
		{
			name: "network audit query invalid",
			validate: func() error {
				return (NetworkAuditQuery{Limit: -1}).Validate()
			},
			wantError: true,
		},
		{
			name: "network message entry valid",
			validate: func() error {
				return (NetworkMessageEntry{
					WorkspaceID: "ws-store-helpers",
					MessageID:   "msg-1",
					Channel:     "builders",
					Surface:     NetworkSurfaceThread,
					ThreadID:    "thread_helpers",
					Direction:   "sent",
					PeerFrom:    "peer-a",
					Kind:        "say",
					PreviewText: "hello",
					Body:        json.RawMessage(`{"text":"hello"}`),
				}).Validate()
			},
		},
		{
			name: "network message entry invalid",
			validate: func() error {
				return (NetworkMessageEntry{MessageID: "msg-1"}).Validate()
			},
			wantError: true,
		},
		{
			name: "network message query invalid",
			validate: func() error {
				return (NetworkMessageQuery{Limit: -1}).Validate()
			},
			wantError: true,
		},
		{
			name: "session meta valid",
			validate: func() error {
				return (SessionMeta{
					ID:                   "sess-meta",
					AgentName:            "coder",
					WorkspaceID:          "ws-meta",
					NetworkParticipation: participation.CloneSpec(participation.LocalSpec()),
					State:                "active",
					CreatedAt:            now,
					UpdatedAt:            now,
				}).Validate()
			},
		},
		{
			name: "session meta invalid",
			validate: func() error {
				return (SessionMeta{}).Validate()
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.validate()
			if tt.wantError && err == nil {
				t.Fatal("validate() error = nil, want non-nil")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("validate() error = %v", err)
			}
		})
	}

	sessionRoot := filepath.Join(string(filepath.Separator), "tmp", "session-a")
	if got, want := SessionDBFile(sessionRoot), filepath.Join(sessionRoot, SessionDatabaseName); got != want {
		t.Fatalf("SessionDBFile() = %q, want %q", got, want)
	}
	if got, want := SessionMetaFile(sessionRoot), filepath.Join(sessionRoot, SessionMetaName); got != want {
		t.Fatalf("SessionMetaFile() = %q, want %q", got, want)
	}
}

func TestValidationPrimitives(t *testing.T) {
	t.Parallel()

	if err := requireField("value", "session id"); err != nil {
		t.Fatalf("requireField(valid) error = %v", err)
	}
	if err := requireField("   ", "session id"); err == nil {
		t.Fatal("requireField(whitespace) error = nil, want non-nil")
	}

	if err := requirePositiveLimit(0, "event limit"); err != nil {
		t.Fatalf("requirePositiveLimit(0) error = %v", err)
	}
	if err := requirePositiveLimit(3, "event limit"); err != nil {
		t.Fatalf("requirePositiveLimit(3) error = %v", err)
	}
	if err := requirePositiveLimit(-1, "event limit"); err == nil {
		t.Fatal("requirePositiveLimit(-1) error = nil, want non-nil")
	}
}

func TestStoreHelpersAndErrorPaths(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 3, 20, 15, 0, 0, time.UTC)
	if got := normalizeTime(now.In(time.FixedZone("test", -3*60*60))); got.Location() != time.UTC {
		t.Fatalf("normalizeTime() location = %v, want UTC", got.Location())
	}
	if got := normalizeTime(time.Time{}); !got.IsZero() {
		t.Fatalf("normalizeTime(zero) = %v, want zero", got)
	}

	formatted := FormatTimestamp(now)
	parsed, err := ParseTimestamp(formatted)
	if err != nil {
		t.Fatalf("ParseTimestamp() error = %v", err)
	}
	if !parsed.Equal(now.UTC()) {
		t.Fatalf("ParseTimestamp() = %v, want %v", parsed, now.UTC())
	}
	if _, err := ParseTimestamp("bad-timestamp"); err == nil {
		t.Fatal("ParseTimestamp() error = nil, want non-nil")
	}

	if got := NullableString(""); got != nil {
		t.Fatalf("NullableString(\"\") = %#v, want nil", got)
	}
	if got := NullableString("value"); got != "value" {
		t.Fatalf("NullableString(value) = %#v, want value", got)
	}

	var nilString *string
	if got := NullableStringPointer(nilString); got != nil {
		t.Fatalf("NullableStringPointer(nil) = %#v, want nil", got)
	}
	value := "abc"
	if got := NullableStringPointer(&value); got != "abc" {
		t.Fatalf("NullableStringPointer(&value) = %#v, want abc", got)
	}
	if got := NullString(sql.NullString{}); got != nil {
		t.Fatalf("NullString(invalid) = %#v, want nil", got)
	}
	if got := NullString(sql.NullString{String: "value", Valid: true}); got == nil || *got != "value" {
		t.Fatalf("NullString(valid) = %#v, want value", got)
	}
	if got := NullInt64(sql.NullInt64{}); got != nil {
		t.Fatalf("NullInt64(invalid) = %#v, want nil", got)
	}
	if got := NullInt64(sql.NullInt64{Int64: 7, Valid: true}); got == nil || *got != 7 {
		t.Fatalf("NullInt64(valid) = %#v, want 7", got)
	}
	if got := NullFloat64(sql.NullFloat64{}); got != nil {
		t.Fatalf("NullFloat64(invalid) = %#v, want nil", got)
	}
	if got := NullFloat64(sql.NullFloat64{Float64: 1.25, Valid: true}); got == nil || *got != 1.25 {
		t.Fatalf("NullFloat64(valid) = %#v, want 1.25", got)
	}

	if got := NewID("prefix"); got == "" || filepath.Base(got) != got {
		t.Fatalf("NewID(prefix) = %q, want non-empty plain value", got)
	}
	if got := NewID(""); got == "" {
		t.Fatal("NewID(\"\") = empty, want non-empty")
	}

	if err := Checkpoint(testutil.Context(t), nil); err != nil {
		t.Fatalf("Checkpoint(nil) error = %v", err)
	}
	if _, err := OpenSQLiteDatabase(testutil.Context(t), "", nil); err == nil {
		t.Fatal("OpenSQLiteDatabase(\"\") error = nil, want non-nil")
	}
}

func TestMetaReadWriteErrors(t *testing.T) {
	t.Parallel()

	if _, err := ReadSessionMeta(""); err == nil {
		t.Fatal("ReadSessionMeta(\"\") error = nil, want non-nil")
	}
	invalidPath := filepath.Join(t.TempDir(), SessionMetaName)
	if err := os.WriteFile(invalidPath, []byte("{"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := ReadSessionMeta(invalidPath); err == nil {
		t.Fatal("ReadSessionMeta(invalid JSON) error = nil, want non-nil")
	}

	if err := WriteSessionMeta("", SessionMeta{}); err == nil {
		t.Fatal("WriteSessionMeta(\"\") error = nil, want non-nil")
	}
	if err := WriteSessionMeta(filepath.Join(t.TempDir(), SessionMetaName), SessionMeta{}); err == nil {
		t.Fatal("WriteSessionMeta(invalid meta) error = nil, want non-nil")
	}
}
