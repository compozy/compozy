package contract

import (
	"time"

	apicontract "github.com/compozy/compozy/internal/api/contract"

	extensionprotocol "github.com/compozy/compozy/internal/extensionprotocol"
	memcontract "github.com/compozy/compozy/internal/memory/contract"
	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/network/participation"
	speedpkg "github.com/compozy/compozy/internal/speed"
)

// HostAPIMethod identifies one extension -> Compozy Host API request.
type HostAPIMethod = extensionprotocol.HostAPIMethod

// NamedType links a generated TypeScript export name to a Go type.
type NamedType struct {
	Name  string
	Value any
}

// HostAPIMethodSpec describes one Host API request/response contract.
type HostAPIMethodSpec struct {
	Method         HostAPIMethod
	Params         NamedType
	Result         NamedType
	OptionalParams bool
}

// EmptyResult is the empty JSON-RPC result for methods without payloads.
type EmptyResult struct{}

// SessionsListParams filters visible sessions.
type SessionsListParams struct {
	Workspace string `json:"workspace,omitempty"`
	Archive   string `json:"archive,omitempty"`
}

// SessionsCreateParams starts a new session.
type SessionsCreateParams struct {
	Agent                string                 `json:"agent"`
	Workspace            string                 `json:"workspace,omitempty"`
	NetworkParticipation *participation.Request `json:"network_participation,omitempty"`
}

// SessionsPromptParams submits one prompt to an existing session.
type SessionsPromptParams struct {
	WorkspaceID    string                                     `json:"workspace_id"`
	SessionID      string                                     `json:"session_id"`
	Message        string                                     `json:"message"`
	MessageID      string                                     `json:"message_id"`
	IdempotencyKey string                                     `json:"idempotency_key"`
	Mode           apicontract.PromptMode                     `json:"mode,omitempty"`
	ExpectedTurnID string                                     `json:"expected_turn_id,omitempty"`
	Runtime        *apicontract.PromptRuntimeSelectionPayload `json:"runtime,omitempty"`
}

// SessionRuntimeSetParams durably selects the default runtime for future prompts.
type SessionRuntimeSetParams struct {
	WorkspaceID      string                         `json:"workspace_id"`
	SessionID        string                         `json:"session_id"`
	Runtime          SessionRuntimeSelectionPayload `json:"runtime"`
	ExpectedRevision *int64                         `json:"expected_revision"`
}

// SessionRuntimeSelectionPayload is the extension-owned wire shape for a durable runtime choice.
type SessionRuntimeSelectionPayload struct {
	Provider        string                       `json:"provider"`
	Model           string                       `json:"model,omitempty"`
	ReasoningEffort modelcatalog.ReasoningEffort `json:"reasoning_effort,omitempty"`
	Speed           speedpkg.Speed               `json:"speed,omitempty"`
}

// SessionRuntimeClearParams clears the durable runtime selection.
type SessionRuntimeClearParams struct {
	WorkspaceID      string `json:"workspace_id"`
	SessionID        string `json:"session_id"`
	ExpectedRevision *int64 `json:"expected_revision"`
}

// SessionInputsListParams identifies the pending inputs for one session.
type SessionInputsListParams = SessionTargetParams

// SessionInputTargetParams identifies one durable pending input.
type SessionInputTargetParams struct {
	WorkspaceID  string `json:"workspace_id"`
	SessionID    string `json:"session_id"`
	QueueEntryID string `json:"queue_entry_id"`
}

// SessionInputReplaceParams atomically replaces one queued input and its durable identity.
type SessionInputReplaceParams struct {
	WorkspaceID    string `json:"workspace_id"`
	SessionID      string `json:"session_id"`
	QueueEntryID   string `json:"queue_entry_id"`
	Text           string `json:"text"`
	MessageID      string `json:"message_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

// SessionInputPromoteParams atomically promotes one queued input to fenced steering.
type SessionInputPromoteParams struct {
	WorkspaceID    string `json:"workspace_id"`
	SessionID      string `json:"session_id"`
	QueueEntryID   string `json:"queue_entry_id"`
	Text           string `json:"text"`
	MessageID      string `json:"message_id"`
	IdempotencyKey string `json:"idempotency_key"`
	ExpectedTurnID string `json:"expected_turn_id"`
}

// SessionTargetParams identifies an existing session.
type SessionTargetParams struct {
	WorkspaceID string `json:"workspace_id"`
	SessionID   string `json:"session_id"`
}

// SessionEventsParams filters persisted session events.
type SessionEventsParams struct {
	WorkspaceID string    `json:"workspace_id"`
	SessionID   string    `json:"session_id"`
	Type        string    `json:"type,omitempty"`
	AgentName   string    `json:"agent_name,omitempty"`
	TurnID      string    `json:"turn_id,omitempty"`
	Limit       int       `json:"limit,omitempty"`
	Offset      int64     `json:"offset,omitempty"`
	Since       time.Time `json:"since,omitzero"`
}

// SessionSoulRefreshParams refreshes one session's Soul snapshot through managed CAS.
type SessionSoulRefreshParams struct {
	WorkspaceID string `json:"workspace_id"`
	SessionID   string `json:"session_id"`
	apicontract.SessionSoulRefreshRequest
}

// SessionHealthGetParams identifies one session health row.
type SessionHealthGetParams = SessionTargetParams

// SessionStatusGetParams identifies one authored-context session status row.
type SessionStatusGetParams = SessionTargetParams

// SandboxListParams filters active sandboxes.
type SandboxListParams struct {
	Workspace string `json:"workspace,omitempty"`
}

// SandboxInfoParams identifies one session sandbox.
type SandboxInfoParams struct {
	WorkspaceID string `json:"workspace_id"`
	SessionID   string `json:"session_id"`
}

// SandboxExecParams executes one command inside a session sandbox.
type SandboxExecParams struct {
	WorkspaceID string `json:"workspace_id"`
	SessionID   string `json:"session_id"`
	Command     string `json:"command"`
	Timeout     int    `json:"timeout,omitempty"`
}

// MemoryStoreParams persists one memory document.
type MemoryStoreParams struct {
	Key       string            `json:"key"`
	Content   string            `json:"content"`
	Scope     memcontract.Scope `json:"scope,omitempty"`
	Workspace string            `json:"workspace,omitempty"`
	Tags      []string          `json:"tags,omitempty"`
}

// MemoryRecallParams queries stored memory documents.
type MemoryRecallParams struct {
	Query     string            `json:"query"`
	Limit     int               `json:"limit,omitempty"`
	Scope     memcontract.Scope `json:"scope,omitempty"`
	Workspace string            `json:"workspace,omitempty"`
}

// MemoryForgetParams removes one stored memory document.
type MemoryForgetParams struct {
	Key       string            `json:"key"`
	Scope     memcontract.Scope `json:"scope,omitempty"`
	Workspace string            `json:"workspace,omitempty"`
}

// ListLogsParams filters workspace runtime logs.
type ListLogsParams struct {
	WorkspaceID   string    `json:"workspace_id"`
	SessionID     string    `json:"session_id,omitempty"`
	AgentName     string    `json:"agent_name,omitempty"`
	Type          string    `json:"type,omitempty"`
	RunID         string    `json:"run,omitempty"`
	ActorKind     string    `json:"actor_kind,omitempty"`
	ActorID       string    `json:"actor_id,omitempty"`
	Provider      string    `json:"provider,omitempty"`
	Outcome       string    `json:"outcome,omitempty"`
	Component     string    `json:"component,omitempty"`
	ErrorOnly     bool      `json:"error_only,omitempty"`
	AfterSequence int64     `json:"after_seq,omitempty"`
	Since         time.Time `json:"since,omitzero"`
	Limit         int       `json:"limit,omitempty"`
}

// SkillsListParams filters skills by workspace scope.
type SkillsListParams struct {
	Workspace string `json:"workspace,omitempty"`
	ForAgent  string `json:"for_agent,omitempty"`
}

// ModelsListParams filters daemon-owned model catalog projections.
type ModelsListParams struct {
	ProviderID   string `json:"provider_id,omitempty"`
	SourceID     string `json:"source_id,omitempty"`
	Refresh      bool   `json:"refresh,omitempty"`
	IncludeStale bool   `json:"include_stale,omitempty"`
}

// ModelsRefreshParams requests a daemon-owned model catalog refresh.
type ModelsRefreshParams struct {
	ProviderID string `json:"provider_id,omitempty"`
	SourceID   string `json:"source_id,omitempty"`
	Force      bool   `json:"force,omitempty"`
	RequestID  string `json:"request_id,omitempty"`
}

// ModelsStatusParams filters daemon-owned model catalog source status rows.
type ModelsStatusParams struct {
	ProviderID string `json:"provider_id,omitempty"`
}
