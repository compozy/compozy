package contract

import (
	"time"

	"github.com/compozy/compozy/internal/agentinstance"

	"github.com/compozy/compozy/internal/heartbeat"
)

type AgentInstanceNextWake struct {
	Kind        string `json:"kind"`
	At          string `json:"at,omitempty"`
	Description string `json:"description"`
}

type AgentInstancePayload struct {
	AuthorityStatus      string                            `json:"authority_status"`
	Sources              []heartbeat.HeartbeatSourceStatus `json:"sources"`
	UnattendedTurns      int64                             `json:"unattended_turns"`
	UnattendedTurnLimit  int64                             `json:"unattended_turn_limit"`
	HeldWakes            int64                             `json:"held_wakes"`
	NextWake             *AgentInstanceNextWake            `json:"next_wake,omitempty"`
	Created              bool                              `json:"created"`
	ChecksEnabled        bool                              `json:"checks_enabled"`
	LastWakeAt           string                            `json:"last_wake_at"`
	NextWakeAt           string                            `json:"next_wake_at"`
	QuietStreak          int64                             `json:"quiet_streak"`
	FailureStreak        int64                             `json:"failure_streak"`
	FatigueLimit         int                               `json:"fatigue_limit"`
	Label                string                            `json:"label"`
	Provider             string                            `json:"provider"`
	ResolvedAppearance   map[string]any                    `json:"resolved_appearance"`
	Scope                string                            `json:"scope"`
	Activity             string                            `json:"activity"`
	Now                  string                            `json:"now"`
	AttentionDismissible bool                              `json:"attention_dismissible"`
	NeedsAttention       bool                              `json:"needs_attention"`
	ID                   string                            `json:"id"`
	ProfileID            string                            `json:"profile_id"`
	ProfileName          string                            `json:"profile_name"`
	AgentName            string                            `json:"agent_name"`
	WorkspaceID          string                            `json:"workspace_id"`
	State                string                            `json:"state"`
	PauseReason          string                            `json:"pause_reason"`
	MainSessionID        string                            `json:"main_session_id"`
	CreatedAt            time.Time                         `json:"created_at"`
	UpdatedAt            time.Time                         `json:"updated_at"`
}
type AgentInstanceResponse struct {
	Instance AgentInstancePayload `json:"instance"`
}
type AgentInstancePeer struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Scope       string `json:"scope"`
	State       string `json:"state"`
	// Accepts lists the input forms the peer's definition declares.
	Accepts []string `json:"accepts,omitempty"`
}

type AgentInstanceListResponse struct {
	Peers              []AgentInstancePeer    `json:"peers,omitempty"`
	MaxActiveInstances int                    `json:"max_active_instances"`
	ActiveCount        int64                  `json:"active_count"`
	Instances          []AgentInstancePayload `json:"instances"`
	NextCursor         string                 `json:"next_cursor"`
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
	Diagnostics   []string                         `json:"diagnostics,omitempty"`
	OccurrenceID  string                           `json:"occurrence_id,omitempty"`
	PicksTracked  bool                             `json:"picks_tracked"`
	Picks         []agentinstance.Pick             `json:"picks"`
	ToolCalls     agentinstance.CheckInToolSummary `json:"tool_calls"`
	Source        *AgentInstanceReportSource       `json:"source,omitempty"`
	Action        string                           `json:"action,omitempty"`
	Resource      string                           `json:"resource,omitempty"`
	SkillReviewID string                           `json:"skill_review_id,omitempty"`
	ID            string                           `json:"id"`
	Kind          string                           `json:"kind"`
	State         string                           `json:"state"`
	Proposals     []AgentInstanceProposal          `json:"proposals,omitempty"`
	Reason        string                           `json:"reason,omitempty"`
	Detail        string                           `json:"detail,omitempty"`
	SessionID     string                           `json:"session_id,omitempty"`
	Tool          string                           `json:"tool,omitempty"`
	ExpiredBy     string                           `json:"expired_by,omitempty"`
	Cause         string                           `json:"cause,omitempty"`
	Since         string                           `json:"since,omitempty"`
	CreatedAt     time.Time                        `json:"created_at"`
}
type AgentInstanceReportsResponse struct {
	Reports    []AgentInstanceReportPayload `json:"reports"`
	NextCursor string                       `json:"next_cursor"`
}

type AgentInstanceProposal struct {
	Action           *AgentInstanceProposalAction  `json:"action,omitempty"`
	AutoAcceptStatus AutoAcceptStatus              `json:"auto_accept_status"`
	Sources          []AgentInstanceProposalSource `json:"sources,omitempty"`
	Title            string                        `json:"title"`
	Body             string                        `json:"body"`
	ItemKeys         []string                      `json:"item_keys"`
}

type AgentInstanceCheckRequest struct {
	DryRun bool `json:"dry_run"`
}
type AgentInstanceCheckResponse struct {
	Sources      []HeartbeatSourceOutcomePayload `json:"sources,omitempty"`
	Gate         string                          `json:"gate,omitempty"`
	OccurrenceID string                          `json:"occurrence_id,omitempty"`
	Digest       []AgentInstanceCheckDigest      `json:"digest"`
}
type AgentInstanceCheckDigest struct {
	Source  *AgentInstanceProposalSource `json:"source,omitempty"`
	Key     string                       `json:"key"`
	Summary string                       `json:"summary"`
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
	Proposals    agentinstance.CheckInProposalSummary `json:"proposals"`
	OccurrenceID string                               `json:"occurrence_id"`
	ToolsStatus  string                               `json:"tools_status"`
	ToolCalls    []agentinstance.CheckInToolCall      `json:"tool_calls"`
	ID           string                               `json:"id"`
	Result       string                               `json:"result"`
	Reason       string                               `json:"reason"`
	Detail       string                               `json:"detail,omitempty"`
	CreatedAt    time.Time                            `json:"created_at"`
}

type AgentInstanceCheckInsResponse struct {
	CheckIns   []AgentInstanceCheckInPayload `json:"check_ins"`
	NextCursor string                        `json:"next_cursor"`
}

// AgentInstanceProposalSource identifies daemon-observed input to a proactive proposal.
type AgentInstanceProposalSource struct {
	SourceID   string `json:"source_id,omitempty"`
	SourceRef  string `json:"source_ref,omitempty"`
	ItemKey    string `json:"item_key,omitempty"`
	URL        string `json:"url,omitempty"`
	Kind       string `json:"kind"`
	EventID    string `json:"event_id"`
	TriggerID  string `json:"trigger_id"`
	RunID      string `json:"run_id"`
	ReceivedAt string `json:"received_at"`
}

// AgentInstanceReportSource is daemon-owned provenance for an event-derived proposal.
type AgentInstanceReportSource struct {
	SourceID    string `json:"source_id,omitempty"`
	SourceRef   string `json:"source_ref,omitempty"`
	ItemKey     string `json:"item_key,omitempty"`
	URL         string `json:"url,omitempty"`
	Kind        string `json:"kind"`
	TriggerName string `json:"trigger_name"`
	Event       string `json:"event"`
	Summary     string `json:"summary,omitempty"`
}

type AgentInstancePickRequest struct {
	Proposal *int `json:"proposal"`
}
type AgentInstancePickResponse struct {
	Pick agentinstance.Pick `json:"pick"`
}

type AgentInstanceProposalAction struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	AutoAccept bool   `json:"auto_accept"`
}

type AutoAcceptStatus string

const (
	AutoAcceptPicked     AutoAcceptStatus = "picked"
	AutoAcceptLimit      AutoAcceptStatus = "limit"
	AutoAcceptQueueFull  AutoAcceptStatus = "queue_full"
	AutoAcceptUnapproved AutoAcceptStatus = "unapproved"
	AutoAcceptNoMain     AutoAcceptStatus = "no_main"
	AutoAcceptInactive   AutoAcceptStatus = "inactive"
	AutoAcceptNone       AutoAcceptStatus = "none"
)
