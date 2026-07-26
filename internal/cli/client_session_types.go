package cli

import (
	"time"

	"github.com/compozy/agh/internal/api/contract"
)

// CreateSessionRequest captures the shared daemon session creation payload.
type CreateSessionRequest = contract.CreateSessionRequest

// SessionRecord is the shared daemon session payload.
type SessionRecord = contract.SessionPayload

// SessionRecapRecord is the shared deterministic session recap payload.
type SessionRecapRecord = contract.RecapPayload

// SessionSoulRefreshRequest captures the managed session Soul refresh payload.
type SessionSoulRefreshRequest = contract.SessionSoulRefreshRequest

// SessionHealthRecord is the shared session-health payload.
type SessionHealthRecord = contract.SessionHealthPayload

// SessionStatusRecord is the shared compact session status payload.
type SessionStatusRecord = contract.SessionStatusResponse

// SessionInspectRecord is the shared detailed session inspect payload.
type SessionInspectRecord = contract.SessionInspectResponse

// SessionInspectQuery captures optional session inspect expansion fields.
type SessionInspectQuery struct {
	IncludeRecentWakeEvents bool
}

// ACPCapsRecord captures optional runtime capabilities exposed by the daemon API.
type ACPCapsRecord = contract.ACPCapsPayload

// SessionRepairRecord reports one session repair pass returned by the daemon API.
type SessionRepairRecord = contract.SessionRepairPayload

// SessionRepairIssueRecord is one inconsistency reported by session repair.
type SessionRepairIssueRecord = contract.SessionRepairIssuePayload

// SessionRepairActionRecord is one planned or persisted repair action.
type SessionRepairActionRecord = contract.SessionRepairActionPayload

// SessionRepairQuery captures CLI repair modifiers.
type SessionRepairQuery struct {
	DryRun bool
	Force  bool
}

// SessionApprovalRequest captures an interactive permission decision.
type SessionApprovalRequest = contract.ApproveSessionRequest

// SessionApprovalRecord is the shared session approval response payload.
type SessionApprovalRecord = contract.SessionApprovalResponse

// SessionPromptRequest captures a session prompt plus explicit busy-input mode.
type SessionPromptRequest = contract.SendPromptRequest

// SessionPromptResultRecord is the shared non-streaming prompt outcome payload.
type SessionPromptResultRecord = contract.SendPromptResultPayload

// SessionPromptRecord wraps prompt outcomes that may either stream events or return a busy-input decision.
type SessionPromptRecord struct {
	Prompt SessionPromptResultRecord   `json:"prompt"`
	Goal   *contract.GoalCommandResult `json:"goal,omitempty"`
	Events []AgentEventRecord          `json:"events,omitempty"`
}

// SessionEventRecord is one persisted session event row returned by the daemon API.
type SessionEventRecord = contract.SessionEventPayload

// TurnHistoryRecord groups session events by turn.
type TurnHistoryRecord = contract.TurnHistoryPayload

// SessionEventQuery captures the CLI filters for session event/history queries.
type SessionEventQuery struct {
	Type          string
	AgentName     string
	TurnID        string
	Since         time.Time
	Last          int
	AfterSequence int64
}
