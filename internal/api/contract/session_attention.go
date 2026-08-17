package contract

import "time"

// SessionPresenceRequest acquires, renews, or releases one operator presence lease.
type SessionPresenceRequest struct {
	Visible bool   `json:"visible"`
	LeaseID string `json:"lease_id,omitempty"`
}

// SessionPresenceResponse identifies a newly acquired presence lease.
type SessionPresenceResponse struct {
	LeaseID string `json:"lease_id"`
}

// SessionWaitRequest starts or resumes one bounded session badge wait.
type SessionWaitRequest struct {
	Until     []string `json:"until,omitempty"`
	TimeoutMS int64    `json:"timeout_ms"`
	Epoch     int64    `json:"epoch,omitempty"`
	ResumeID  string   `json:"resume_id,omitempty"`
}

// SessionWaitResponse reports one bounded wait outcome and its catalog fence.
type SessionWaitResponse struct {
	SessionID string   `json:"session_id"`
	Outcome   string   `json:"outcome"`
	State     string   `json:"state,omitempty"`
	WaitedMS  int64    `json:"waited_ms"`
	Until     []string `json:"until,omitempty"`
	Revision  int64    `json:"revision"`
	ResumeID  string   `json:"resume_id,omitempty"`
}

// PendingInteractionPayload is the sanitized public interaction projection.
type PendingInteractionPayload struct {
	InteractionID     string     `json:"interaction_id"`
	Kind              string     `json:"kind"`
	ProviderRequestID string     `json:"provider_request_id"`
	TurnID            string     `json:"turn_id,omitempty"`
	Title             string     `json:"title,omitempty"`
	Choices           []string   `json:"choices,omitempty"`
	Decisions         []string   `json:"decisions,omitempty"`
	Status            string     `json:"status"`
	CreatedAt         time.Time  `json:"created_at"`
	ResolvedAt        *time.Time `json:"resolved_at,omitempty"`
	Resolution        string     `json:"resolution,omitempty"`
	ResolvedBy        string     `json:"resolved_by,omitempty"`
}

// SessionInteractionsResponse lists restart-durable actionable interactions.
type SessionInteractionsResponse struct {
	Interactions []PendingInteractionPayload `json:"interactions"`
}

// SessionAttentionSummaryPayload carries exact operator-wide attention counts.
type SessionAttentionSummaryPayload struct {
	NeedsYou    int                                `json:"needs_you"`
	Finished    int                                `json:"finished"`
	ByWorkspace []WorkspaceAttentionSummaryPayload `json:"by_workspace"`
}

// WorkspaceAttentionSummaryPayload carries exact counts for one workspace.
type WorkspaceAttentionSummaryPayload struct {
	WorkspaceID string `json:"workspace_id"`
	NeedsYou    int    `json:"needs_you"`
	Finished    int    `json:"finished"`
}
