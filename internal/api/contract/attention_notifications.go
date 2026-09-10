package contract

import "time"

// AttentionNotificationPayload describes one occurrence without changing its source's attention state.
type AttentionNotificationPayload struct {
	ID             string    `json:"id"`
	Kind           string    `json:"kind"`
	SourceID       string    `json:"source_id"`
	WorkspaceID    string    `json:"workspace_id"`
	WorkspaceLabel string    `json:"workspace_label"`
	Title          string    `json:"title"`
	Detail         string    `json:"detail"`
	OccurredAt     time.Time `json:"occurred_at"`
	Finished       bool      `json:"finished"`
	Badge          string    `json:"badge,omitempty"`
	AgentName      string    `json:"agent_name,omitempty"`
	RunID          string    `json:"run_id,omitempty"`
	NodeID         string    `json:"node_id,omitempty"`
	ItemIndex      int       `json:"item_index"`
	Generation     int       `json:"generation"`
	LoopName       string    `json:"loop_name,omitempty"`
	RequestKind    string    `json:"request_kind,omitempty"`
	TerminalID     string    `json:"terminal_id,omitempty"`
	Redacted       bool      `json:"redacted"`
}

// AttentionNotificationsResponse carries exact unread counts and a snapshot covering every page.
type AttentionNotificationsResponse struct {
	Snapshot string                         `json:"snapshot"`
	Total    int                            `json:"total"`
	NeedsYou int                            `json:"needs_you"`
	Finished int                            `json:"finished"`
	Items    []AttentionNotificationPayload `json:"items"`
}

// AcknowledgeAttentionRequest acknowledges one snapshot member, or every member when id is omitted.
type AcknowledgeAttentionRequest struct {
	Snapshot string `json:"snapshot"     binding:"required"`
	ID       string `json:"id,omitempty"`
}
