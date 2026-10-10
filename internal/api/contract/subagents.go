package contract

import (
	"strings"
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
	Isolation          string                   `json:"isolation"`
	Worktree           *SubagentWorktreePayload `json:"worktree,omitempty"`
	SubagentID         string                   `json:"subagent_id"`
	WorkspaceID        string                   `json:"workspace_id"`
	ParentSessionID    string                   `json:"parent_session_id"`
	ParentTurnID       string                   `json:"parent_turn_id"`
	ChildSessionID     *string                  `json:"child_session_id"`
	Origin             string                   `json:"origin"`
	ProviderToolCallID *string                  `json:"provider_tool_call_id"`
	Title              string                   `json:"title"`
	Role               string                   `json:"role"`
	Status             string                   `json:"status"`
	WorkState          string                   `json:"work_state"`
	Runtime            SubagentRuntimePayload   `json:"runtime"`
	Depth              int                      `json:"depth"`
	Progress           string                   `json:"progress"`
	Result             *string                  `json:"result"`
	ResultPreview      string                   `json:"result_preview"`
	ResultTruncated    bool                     `json:"result_truncated"`
	Error              *string                  `json:"error"`
	WaitTimedOut       bool                     `json:"wait_timed_out"`
	Hint               string                   `json:"hint,omitempty"`
	Delivery           string                   `json:"delivery"`
	StartedAt          *time.Time               `json:"started_at"`
	SettledAt          *time.Time               `json:"settled_at"`
	CreatedAt          time.Time                `json:"created_at"`
	UpdatedAt          time.Time                `json:"updated_at"`
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

func SubagentFromDomain(row *session.Subagent) SubagentPayload {
	runtimeSpeed, resultPreview, hint := row.RuntimeSpeed, row.ResultPreview, row.Hint
	if row.Origin == store.SubagentOriginDelegated && row.RuntimeSpeed == "" {
		runtimeSpeed = "normal"
	}
	if row.ResultPreview == "" && row.Result != nil {
		line, _, _ := strings.Cut(*row.Result, "\n")
		preview := []rune(line)
		resultPreview = string(preview[:min(len(preview), 280)])
	}
	if row.ResultTruncated && row.Hint == "" {
		hint = "Read the full answer with compozy__session_history on child_session_id."
	}
	var toolCallID *string
	if row.ProviderToolCallID != "" {
		toolCallID = new(row.ProviderToolCallID)
	}
	return SubagentPayload{
		Isolation: subagentIsolation(row.Isolation), Worktree: subagentWorktreePayload(row),
		SubagentID: row.ID, WorkspaceID: row.WorkspaceID,
		ParentSessionID: row.ParentSessionID, ParentTurnID: row.ParentTurnID,
		ChildSessionID: row.ChildSessionID, Origin: row.Origin, ProviderToolCallID: toolCallID,
		Title: row.Title, Role: row.Role, Status: row.Status, WorkState: row.WorkState,
		Runtime: SubagentRuntimePayload{Agent: row.RuntimeAgent, Provider: row.RuntimeProvider,
			Model: row.RuntimeModel, ReasoningEffort: row.RuntimeEffort, Speed: runtimeSpeed},
		Depth: row.Depth, Progress: row.Progress, Result: row.Result, ResultPreview: resultPreview,
		ResultTruncated: row.ResultTruncated, Error: row.Error, WaitTimedOut: row.WaitTimedOut,
		Hint: hint, Delivery: row.Delivery, StartedAt: row.StartedAt,
		SettledAt: row.SettledAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func SubagentSummaryFromStore(summary store.SubagentSummary) *SubagentSummaryPayload {
	if summary.Total == 0 {
		return nil
	}
	return &SubagentSummaryPayload{Live: summary.Live, Total: summary.Total,
		Failed: summary.Failed, Attention: summary.Attention, MostUrgent: summary.MostUrgent}
}

type SubagentPullRequestPayload struct {
	URL    string `json:"url"`
	Number int    `json:"number"`
	State  string `json:"state"`
}
type SubagentWorktreePayload struct {
	ID                string                      `json:"id"`
	Name              string                      `json:"name"`
	Branch            string                      `json:"branch"`
	BaseRef           string                      `json:"base_ref"`
	BaseSHA           string                      `json:"base_sha,omitempty"`
	Path              string                      `json:"path"`
	HeadSHA           string                      `json:"head_sha,omitempty"`
	CommitsAhead      *int                        `json:"commits_ahead,omitempty"`
	DirtyFiles        *int                        `json:"dirty_files,omitempty"`
	ObservedAt        *time.Time                  `json:"observed_at,omitempty"`
	PullRequestStatus string                      `json:"pull_request_status,omitempty"`
	PullRequest       *SubagentPullRequestPayload `json:"pull_request,omitempty"`
}

func subagentIsolation(mode string) string {
	if mode == "" {
		return "shared"
	}
	return mode
}
func subagentWorktreePayload(row *session.Subagent) *SubagentWorktreePayload {
	if row.Isolation != "worktree" || row.WorktreeState().ID == "" {
		return nil
	}
	facts := row.WorktreeState().Facts
	out := &SubagentWorktreePayload{
		ID:                row.WorktreeState().ID,
		Name:              row.WorktreeState().Name,
		Branch:            row.WorktreeState().Branch,
		BaseRef:           row.WorktreeState().BaseRef,
		BaseSHA:           row.WorktreeState().BaseSHA,
		Path:              row.WorktreeState().Path,
		HeadSHA:           facts.HeadSHA,
		CommitsAhead:      facts.CommitsAhead,
		DirtyFiles:        facts.DirtyFiles,
		PullRequestStatus: facts.PRStatus,
	}
	if !facts.ObservedAt.IsZero() {
		out.ObservedAt = &facts.ObservedAt
	}
	if facts.PRNumber != nil && facts.PRURL != "" && (facts.PRStatus == "open" || facts.PRStatus == "draft" ||
		facts.PRStatus == "merged" || facts.PRStatus == "closed") {
		out.PullRequest = &SubagentPullRequestPayload{URL: facts.PRURL, Number: *facts.PRNumber, State: facts.PRStatus}
	}
	return out
}
