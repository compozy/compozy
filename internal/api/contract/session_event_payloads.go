package contract

import (
	"encoding/json"
	"time"

	"github.com/compozy/agh/internal/store"
)

// SessionEventPayload is the shared session event response payload.
type SessionEventPayload struct {
	ID            string `json:"id"`
	SessionID     string `json:"session_id"`
	Sequence      int64  `json:"sequence"`
	TurnID        string `json:"turn_id"`
	Type          string `json:"type"`
	AgentName     string `json:"agent_name"`
	WorkspaceID   string `json:"workspace_id,omitempty"`
	WorkspacePath string `json:"workspace_path,omitempty"`
	store.EventCorrelation
	ParentSessionID string                 `json:"parent_session_id,omitempty"`
	RootSessionID   string                 `json:"root_session_id,omitempty"`
	SpawnDepth      int                    `json:"spawn_depth"`
	Content         json.RawMessage        `json:"content"`
	Goal            *GoalPromptMeta        `json:"goal,omitempty"`
	StopReason      store.StopReason       `json:"stop_reason,omitempty"`
	StopDetail      string                 `json:"stop_detail,omitempty"`
	Failure         *SessionFailurePayload `json:"failure,omitempty"`
	Timestamp       time.Time              `json:"timestamp"`
}

// TurnHistoryPayload is the shared turn history response payload.
type TurnHistoryPayload struct {
	TurnID string                `json:"turn_id"`
	Events []SessionEventPayload `json:"events"`
}

// SessionRepairPayload reports one dry-run or persisted session repair pass.
type SessionRepairPayload struct {
	SessionID string                       `json:"session_id"`
	Issues    []SessionRepairIssuePayload  `json:"issues"`
	Actions   []SessionRepairActionPayload `json:"actions"`
	Persisted bool                         `json:"persisted"`
}

// SessionRepairIssuePayload is one inconsistency found during session repair.
type SessionRepairIssuePayload struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	TurnID   string `json:"turn_id,omitempty"`
	EventID  string `json:"event_id,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

// SessionRepairActionPayload is one append-only repair action.
type SessionRepairActionPayload struct {
	Code       string `json:"code"`
	TurnID     string `json:"turn_id"`
	EventID    string `json:"event_id,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
	ToolName   string `json:"tool_name,omitempty"`
	Persisted  bool   `json:"persisted"`
}
