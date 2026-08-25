package contract

import (
	"encoding/json"
	"time"
)

// CallTargetRequest selects exactly one new agent definition or existing child session.
type CallTargetRequest struct {
	Agent     string `json:"agent,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

// CallRuntimeRequest optionally narrows the runtime used by a new child.
type CallRuntimeRequest struct {
	Provider        string `json:"provider,omitempty"`
	Model           string `json:"model,omitempty"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	Speed           string `json:"speed,omitempty"`
}

// CallPermissionNarrowingRequest carries subset-only permission atoms.
type CallPermissionNarrowingRequest struct {
	Tools           []string `json:"tools,omitempty"`
	Skills          []string `json:"skills,omitempty"`
	MCPServers      []string `json:"mcp_servers,omitempty"`
	WorkspacePaths  []string `json:"workspace_paths,omitempty"`
	NetworkChannels []string `json:"network_channels,omitempty"`
	SandboxProfiles []string `json:"sandbox_profiles,omitempty"`
}

// CreateCallItemRequest contains one independently admitted call.
type CreateCallItemRequest struct {
	Target          CallTargetRequest              `json:"target"`
	Prompt          string                         `json:"prompt"`
	Expect          json.RawMessage                `json:"expect,omitempty"`
	IdleTTLSeconds  *int64                         `json:"idle_ttl_seconds,omitempty"`
	DeadlineSeconds *int64                         `json:"deadline_seconds,omitempty"`
	Strict          bool                           `json:"strict,omitempty"`
	ResultBudget    string                         `json:"result_budget,omitempty"`
	ResultOverflow  string                         `json:"result_overflow,omitempty"`
	IdempotencyKey  string                         `json:"idempotency_key,omitempty"`
	Runtime         *CallRuntimeRequest            `json:"runtime,omitempty"`
	Narrow          CallPermissionNarrowingRequest `json:"narrow,omitempty"`
}

// CreateCallRequest accepts either one item inline or a bounded batch.
type CreateCallRequest struct {
	CreateCallItemRequest
	Tasks       []CreateCallItemRequest `json:"tasks,omitempty"`
	Scope       string                  `json:"scope,omitempty"`
	WorkspaceID string                  `json:"workspace_id,omitempty"`
}

// CallOwnerPayload identifies one durable owner or actor.
type CallOwnerPayload struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// CallProvenancePayload states who produced a result and how it was admitted.
type CallProvenancePayload struct {
	ProducedBy string `json:"produced_by,omitempty"`
	SessionID  string `json:"session_id,omitempty"`
	Admitted   string `json:"admitted,omitempty"`
}

// CallPayload is the shared read projection for one durable call.
type CallPayload struct {
	CallID            string                 `json:"call_id"`
	ProfileID         string                 `json:"profile_id"`
	ProfileName       string                 `json:"profile_name"`
	Scope             string                 `json:"scope"`
	WorkspaceID       string                 `json:"workspace_id,omitempty"`
	Caller            CallOwnerPayload       `json:"caller"`
	Actor             CallOwnerPayload       `json:"actor"`
	Agent             string                 `json:"agent,omitempty"`
	ChildSessionID    string                 `json:"child_session_id,omitempty"`
	ParentSessionID   string                 `json:"parent_session_id,omitempty"`
	RootSessionID     string                 `json:"root_session_id"`
	Depth             int                    `json:"depth"`
	State             string                 `json:"state"`
	Verdict           string                 `json:"verdict,omitempty"`
	ExpectDigest      string                 `json:"expect_digest,omitempty"`
	PromptPreview     string                 `json:"prompt_preview,omitempty"`
	PromptBytes       int                    `json:"prompt_bytes"`
	ResultPreview     json.RawMessage        `json:"result_preview,omitempty"`
	ResultBytes       int                    `json:"result_bytes,omitempty"`
	ResultBudget      int                    `json:"result_budget_bytes"`
	ResultOverflow    string                 `json:"result_overflow"`
	Strict            bool                   `json:"strict"`
	IdleTTLSeconds    int64                  `json:"idle_ttl_seconds"`
	IdleExpiresAt     *time.Time             `json:"idle_expires_at"`
	FailureCode       string                 `json:"failure_code,omitempty"`
	FailureDetail     string                 `json:"failure_detail,omitempty"`
	FirstIssueText    string                 `json:"first_issue_text,omitempty"`
	SecondIssueText   string                 `json:"second_issue_text,omitempty"`
	FinalProsePreview string                 `json:"final_prose_preview,omitempty"`
	SupersededPreview json.RawMessage        `json:"superseded_preview,omitempty"`
	SupersededBytes   int                    `json:"superseded_bytes"`
	RepairAttempts    int                    `json:"repair_attempts"`
	Replayed          bool                   `json:"replayed,omitempty"`
	Provenance        *CallProvenancePayload `json:"provenance,omitempty"`
	DeadlineAt        *time.Time             `json:"deadline_at,omitempty"`
	CreatedAt         time.Time              `json:"created_at"`
	StartedAt         *time.Time             `json:"started_at,omitempty"`
	SettledAt         *time.Time             `json:"settled_at,omitempty"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

// CallCreatePayload is the stable asynchronous acceptance shape.
type CallCreatePayload struct {
	CallID         string     `json:"call_id"`
	ChildSessionID string     `json:"child_session_id,omitempty"`
	State          string     `json:"state"`
	Replayed       bool       `json:"replayed"`
	IdleExpiresAt  *time.Time `json:"idle_expires_at"`
}

// CallBatchItemPayload carries one independently accepted or rejected item.
type CallBatchItemPayload struct {
	Call  *CallCreatePayload `json:"call,omitempty"`
	Error *CallErrorResponse `json:"error,omitempty"`
}

// CallsResponse is a counted cursor page.
type CallsResponse struct {
	Items      []CallPayload `json:"items"`
	NextCursor string        `json:"next_cursor,omitempty"`
	Total      int           `json:"total"`
}

// AwaitCallsRequest waits on one or more call identities.
type AwaitCallsRequest struct {
	CallIDs   []string `json:"call_ids,omitempty"`
	TimeoutMS int64    `json:"timeout_ms,omitempty"`
	Resume    string   `json:"resume,omitempty"`
}

// AwaitCallsResponse reports settled and still-pending calls.
type AwaitCallsResponse struct {
	Settled          []CallPayload `json:"settled"`
	Pending          []string      `json:"pending"`
	Outcome          string        `json:"outcome"`
	Resume           string        `json:"resume,omitempty"`
	ClampedTimeoutMS int64         `json:"clamped_timeout_ms"`
}

// CancelCallRequest records the operator's cancellation reason.
type CancelCallRequest struct {
	Reason string `json:"reason,omitempty"`
}

// CancelCallResponse is intentionally small and idempotent.
type CancelCallResponse struct {
	State string `json:"state"`
}

// StopSessionRequest optionally drains the governed call subtree before stopping the root.
type StopSessionRequest struct {
	Subtree bool   `json:"subtree,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// StopSessionSubtreeResponse summarizes an idempotent subtree drain.
type StopSessionSubtreeResponse struct {
	StoppedChildren  int `json:"stopped_children"`
	ClosedCalls      int `json:"closed_calls"`
	PreservedResults int `json:"preserved_results"`
}

// SendCallMessageRequest sends inert text to one lineage session.
type SendCallMessageRequest struct {
	To          CallTargetRequest `json:"to"`
	Text        string            `json:"text"`
	CallID      string            `json:"call_id,omitempty"`
	Scope       string            `json:"scope,omitempty"`
	WorkspaceID string            `json:"workspace_id,omitempty"`
}

// CallMessagePayload is the closed receipt/read projection.
type CallMessagePayload struct {
	MessageID     string           `json:"message_id"`
	ProfileID     string           `json:"profile_id"`
	ProfileName   string           `json:"profile_name"`
	Scope         string           `json:"scope"`
	WorkspaceID   string           `json:"workspace_id,omitempty"`
	From          CallOwnerPayload `json:"from"`
	FromAgentName string           `json:"from_agent_name,omitempty"`
	ToSessionID   string           `json:"to_session_id"`
	CallID        string           `json:"call_id,omitempty"`
	Text          string           `json:"text"`
	Delivery      string           `json:"delivery"`
	Reason        string           `json:"reason,omitempty"`
	Attempts      int              `json:"attempts"`
	CreatedAt     time.Time        `json:"created_at"`
	DeliveredAt   *time.Time       `json:"delivered_at,omitempty"`
}

// SendCallMessageResponse is the asynchronous acceptance receipt.
type SendCallMessageResponse struct {
	MessageID string `json:"message_id"`
	Delivery  string `json:"delivery"`
}

// CallMessagesResponse is an uncounted cursor page.
type CallMessagesResponse struct {
	Items      []CallMessagePayload `json:"items"`
	NextCursor string               `json:"next_cursor,omitempty"`
}

// CallResultResponse returns the exact stored JSON bytes under a stable key.
type CallResultResponse struct {
	CallID string          `json:"call_id"`
	Result json.RawMessage `json:"result"`
}

// CallPromptResponse returns the exact authored prompt without a storage reference.
type CallPromptResponse struct {
	CallID string `json:"call_id"`
	Prompt string `json:"prompt"`
}

// CallSupersededResponse returns preserved late-result evidence without a storage reference.
type CallSupersededResponse struct {
	CallID string          `json:"call_id"`
	Result json.RawMessage `json:"result"`
}

// PublishCallRequest targets a channel-thread Network conversation.
type PublishCallRequest struct {
	Channel  string `json:"channel"`
	ThreadID string `json:"thread_id,omitempty"`
}

// PublishCallResponse records the one-way Network evidence message.
type PublishCallResponse struct {
	NetworkMessageID string `json:"network_message_id"`
	Published        bool   `json:"published"`
}

// CallErrorResponse preserves the shared error shape and adds bounded calls diagnostics.
type CallErrorResponse struct {
	Error      string            `json:"error"`
	Code       string            `json:"code"`
	Details    map[string]string `json:"details,omitempty"`
	Available  []CallAgentOption `json:"available,omitempty"`
	Widening   []string          `json:"widening,omitempty"`
	OriginalID string            `json:"original_id,omitempty"`
}

// CallAgentOption is one safe roster row returned with an unknown-agent error.
type CallAgentOption struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}
