package contract

import "time"

type AgentInstancePayload struct {
	Label              string         `json:"label"`
	Provider           string         `json:"provider"`
	ResolvedAppearance map[string]any `json:"resolved_appearance"`
	Scope              string         `json:"scope"`
	Activity           string         `json:"activity"`
	Now                string         `json:"now"`
	NeedsAttention     bool           `json:"needs_attention"`
	ID                 string         `json:"id"`
	ProfileID          string         `json:"profile_id"`
	AgentName          string         `json:"agent_name"`
	WorkspaceID        string         `json:"workspace_id"`
	State              string         `json:"state"`
	PauseReason        string         `json:"pause_reason"`
	MainSessionID      string         `json:"main_session_id"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}
type AgentInstanceResponse struct {
	Instance AgentInstancePayload `json:"instance"`
}
type AgentInstanceListResponse struct {
	Instances  []AgentInstancePayload `json:"instances"`
	NextCursor string                 `json:"next_cursor"`
}
type AgentInstanceCreateRequest struct {
	Agent     string `json:"agent"`
	Workspace string `json:"workspace"`
}
type AgentInstanceAdoptRequest struct {
	Agent     string `json:"agent"`
	SessionID string `json:"session_id"`
}
type AgentInstanceReportPayload struct {
	ID        string                  `json:"id"`
	Kind      string                  `json:"kind"`
	State     string                  `json:"state"`
	Proposals []AgentInstanceProposal `json:"proposals,omitempty"`
	Reason    string                  `json:"reason,omitempty"`
	Detail    string                  `json:"detail,omitempty"`
	SessionID string                  `json:"session_id,omitempty"`
	Tool      string                  `json:"tool,omitempty"`
	ExpiredBy string                  `json:"expired_by,omitempty"`
	Cause     string                  `json:"cause,omitempty"`
	Since     string                  `json:"since,omitempty"`
	CreatedAt time.Time               `json:"created_at"`
}
type AgentInstanceReportsResponse struct {
	Reports    []AgentInstanceReportPayload `json:"reports"`
	NextCursor string                       `json:"next_cursor"`
}

type AgentInstanceProposal struct {
	Title    string   `json:"title"`
	Body     string   `json:"body"`
	ItemKeys []string `json:"item_keys"`
}

type AgentInstanceCheckRequest struct {
	DryRun bool `json:"dry_run"`
}
type AgentInstanceCheckResponse struct {
	Gate         string                    `json:"gate,omitempty"`
	OccurrenceID string                    `json:"occurrence_id,omitempty"`
	Digest       *AgentInstanceCheckDigest `json:"digest,omitempty"`
}
type AgentInstanceCheckDigest struct {
	ItemKeys    []string `json:"item_keys"`
	Fingerprint string   `json:"fingerprint"`
	Diagnostics []string `json:"diagnostics,omitempty"`
}

type AgentInstanceUpdateRequest struct {
	ChecksEnabled *bool `json:"checks_enabled"`
}

type AgentInstanceOccurrenceResponse struct {
	State    string `json:"state"`
	Result   string `json:"result,omitempty"`
	Reason   string `json:"reason,omitempty"`
	ReportID string `json:"report_id,omitempty"`
}

type AgentInstanceAwayResponse struct {
	LastSeenAt    *time.Time                   `json:"last_seen_at"`
	QuietCheckIns int                          `json:"quiet_check_ins"`
	Reports       []AgentInstanceReportPayload `json:"reports"`
}

type AgentInstanceSeenResponse struct {
	LastSeenAt time.Time `json:"last_seen_at"`
}

type AgentInstanceCheckInPayload struct {
	ID        string    `json:"id"`
	Result    string    `json:"result"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

type AgentInstanceCheckInsResponse struct {
	CheckIns   []AgentInstanceCheckInPayload `json:"check_ins"`
	NextCursor string                        `json:"next_cursor"`
}
