package contract

import (
	"time"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

const (
	SessionStreamEventSubagentsSnapshot = "subagents_snapshot"
	SessionStreamEventSubagentUpdated   = "subagent_updated"
)

type SubagentRuntimePayload struct {
	Agent           string `json:"agent"`
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort"`
	Speed           string `json:"speed"`
}

type SubagentPayload struct {
	SubagentID         string                 `json:"subagent_id"`
	WorkspaceID        string                 `json:"workspace_id"`
	ParentSessionID    string                 `json:"parent_session_id"`
	ParentTurnID       string                 `json:"parent_turn_id"`
	ChildSessionID     *string                `json:"child_session_id"`
	Origin             string                 `json:"origin"`
	ProviderToolCallID *string                `json:"provider_tool_call_id"`
	Title              string                 `json:"title"`
	Role               string                 `json:"role"`
	Status             string                 `json:"status"`
	WorkState          string                 `json:"work_state"`
	Runtime            SubagentRuntimePayload `json:"runtime"`
	Depth              int                    `json:"depth"`
	Progress           string                 `json:"progress"`
	Result             *string                `json:"result"`
	ResultPreview      string                 `json:"result_preview"`
	ResultTruncated    bool                   `json:"result_truncated"`
	Error              *string                `json:"error"`
	WaitTimedOut       bool                   `json:"wait_timed_out"`
	Hint               string                 `json:"hint,omitempty"`
	Delivery           string                 `json:"delivery"`
	StartedAt          *time.Time             `json:"started_at"`
	SettledAt          *time.Time             `json:"settled_at"`
	UpdatedAt          time.Time              `json:"updated_at"`
}

type SubagentListPayload struct {
	Subagents  []SubagentPayload `json:"subagents"`
	NextCursor *string           `json:"next_cursor"`
}

type SubagentCancelRequest struct {
	Reason string `json:"reason"`
}

type SubagentCancelPayload struct {
	SubagentID string `json:"subagent_id"`
	Status     string `json:"status"`
}

type SubagentSummaryPayload struct {
	Live       int    `json:"live"`
	Total      int    `json:"total"`
	Failed     int    `json:"failed"`
	Attention  int    `json:"attention"`
	MostUrgent string `json:"most_urgent"`
}

type SubagentsSnapshotEvent struct {
	SessionID string            `json:"session_id"`
	Subagents []SubagentPayload `json:"subagents"`
}

type SubagentUpdatedEvent struct {
	SessionID string          `json:"session_id"`
	Subagent  SubagentPayload `json:"subagent"`
}

func SubagentFromDomain(row session.Subagent) SubagentPayload {
	var toolCallID *string
	if row.ProviderToolCallID != "" {
		toolCallID = &row.ProviderToolCallID
	}
	return SubagentPayload{
		SubagentID: row.ID, WorkspaceID: row.WorkspaceID,
		ParentSessionID: row.ParentSessionID, ParentTurnID: row.ParentTurnID,
		ChildSessionID: row.ChildSessionID, Origin: row.Origin, ProviderToolCallID: toolCallID,
		Title: row.Title, Role: row.Role, Status: row.Status, WorkState: row.WorkState,
		Runtime: SubagentRuntimePayload{Agent: row.RuntimeAgent, Provider: row.RuntimeProvider,
			Model: row.RuntimeModel, ReasoningEffort: row.RuntimeEffort, Speed: row.RuntimeSpeed},
		Depth: row.Depth, Progress: row.Progress, Result: row.Result, ResultPreview: row.ResultPreview,
		ResultTruncated: row.ResultTruncated, Error: row.Error, WaitTimedOut: row.WaitTimedOut,
		Hint: row.Hint, Delivery: row.Delivery, StartedAt: row.StartedAt,
		SettledAt: row.SettledAt, UpdatedAt: row.UpdatedAt,
	}
}

func SubagentSummaryFromStore(summary store.SubagentSummary) *SubagentSummaryPayload {
	if summary.Total == 0 {
		return nil
	}
	return &SubagentSummaryPayload{Live: summary.Live, Total: summary.Total,
		Failed: summary.Failed, Attention: summary.Attention, MostUrgent: summary.MostUrgent}
}
