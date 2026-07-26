package hooks

import (
	"encoding/json"
	"errors"

	"time"
)

var (
	// ErrAutomationFireCancelled reports that a sync automation pre-fire hook canceled the dispatch.
	ErrAutomationFireCancelled = errors.New("hooks: automation fire canceled")
)

// PayloadBase carries the common identifiers attached to every hook payload.
type PayloadBase struct {
	Event     HookEvent `json:"event"`
	Timestamp time.Time `json:"timestamp"`
}

// SessionContext carries the common session-scoped hook attributes.
type SessionContext struct {
	SessionID    string `json:"session_id,omitempty"`
	SessionName  string `json:"session_name,omitempty"`
	SessionType  string `json:"session_type,omitempty"`
	AgentName    string `json:"agent_name,omitempty"`
	WorkspaceID  string `json:"workspace_id,omitempty"`
	Workspace    string `json:"workspace,omitempty"`
	ACPSessionID string `json:"acp_session_id,omitempty"`
	State        string `json:"state,omitempty"`
	*SessionSoulContext
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SessionSoulContext carries optional authored Soul provenance on session-scoped hooks.
type SessionSoulContext struct {
	SoulSnapshotID string `json:"soul_snapshot_id,omitempty"`
	SoulDigest     string `json:"soul_digest,omitempty"`
}

// TurnContext carries the current turn identifier.
type TurnContext struct {
	TurnID string `json:"turn_id,omitempty"`
}

// ContextBlock is a typed free-form context fragment attached to inputs or prompts.
type ContextBlock struct {
	Kind     string            `json:"kind,omitempty"`
	Text     string            `json:"text,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ToolCallRef identifies a tool invocation in hook payloads.
type ToolCallRef struct {
	ToolCallID string `json:"tool_call_id,omitempty"`
	ToolID     string `json:"tool_id,omitempty"`
	ReadOnly   bool   `json:"read_only,omitempty"`
}

// ToolLocation captures one path-scoped tool location.
type ToolLocation struct {
	Path      string `json:"path,omitempty"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
}

// PermissionOption carries one interactive permission option.
type PermissionOption struct {
	Decision string `json:"decision,omitempty"`
	OptionID string `json:"option_id,omitempty"`
	Kind     string `json:"kind,omitempty"`
	Label    string `json:"label,omitempty"`
}

// PermissionToolCall carries the tool details attached to a permission request.
type PermissionToolCall struct {
	ID        string         `json:"id,omitempty"`
	Kind      string         `json:"kind,omitempty"`
	Title     string         `json:"title,omitempty"`
	Status    string         `json:"status,omitempty"`
	Locations []ToolLocation `json:"locations,omitempty"`
}

// ControlPatch carries the common deny surface shared by mutable hook families.
type ControlPatch struct {
	Deny       bool   `json:"deny,omitempty"`
	DenyReason string `json:"deny_reason,omitempty"`
}

// SessionPreCreatePayload is delivered before a session is created.
type SessionPreCreatePayload struct {
	PayloadBase
	SessionContext
}

// SessionLifecyclePayload is shared by post-create, resume, and stop events.
type SessionLifecyclePayload struct {
	PayloadBase
	SessionContext
}

// SessionPostCreatePayload is delivered after a session is created.
type SessionPostCreatePayload = SessionLifecyclePayload

// SessionPreResumePayload is delivered before a session resumes.
type SessionPreResumePayload = SessionLifecyclePayload

// SessionPostResumePayload is delivered after a session resumes.
type SessionPostResumePayload = SessionLifecyclePayload

// SessionPreStopPayload is delivered before a session stops.
type SessionPreStopPayload = SessionLifecyclePayload

// SessionPostStopPayload is delivered after a session stops.
type SessionPostStopPayload = SessionLifecyclePayload

// SessionMessagePersistedPayload is delivered after an assistant message is durably persisted.
type SessionMessagePersistedPayload struct {
	PayloadBase
	SessionContext
	TurnContext
	MessageID       string          `json:"message_id,omitempty"`
	MessageSeq      int64           `json:"message_seq,omitempty"`
	Role            string          `json:"role,omitempty"`
	Text            string          `json:"text,omitempty"`
	Raw             json.RawMessage `json:"raw,omitempty"`
	Persisted       json.RawMessage `json:"persisted,omitempty"`
	RootSessionID   string          `json:"root_session_id,omitempty"`
	ParentSessionID string          `json:"parent_session_id,omitempty"`
	ActorKind       string          `json:"actor_kind,omitempty"`
	ActorID         string          `json:"actor_id,omitempty"`
}

// SessionCreatePatch mutates or denies session lifecycle operations.
type SessionCreatePatch struct {
	ControlPatch
	SessionName *string `json:"session_name,omitempty"`
	SessionType *string `json:"session_type,omitempty"`
	AgentName   *string `json:"agent_name,omitempty"`
	WorkspaceID *string `json:"workspace_id,omitempty"`
	Workspace   *string `json:"workspace,omitempty"`
}

// SessionPostCreatePatch is the post-create patch surface.
type SessionPostCreatePatch = SessionCreatePatch

// SessionPreResumePatch is the pre-resume patch surface.
type SessionPreResumePatch = SessionCreatePatch

// SessionPostResumePatch is the post-resume patch surface.
type SessionPostResumePatch = SessionCreatePatch

// SessionPreStopPatch is the pre-stop patch surface.
type SessionPreStopPatch = SessionCreatePatch

// SessionPostStopPatch is the post-stop patch surface.
type SessionPostStopPatch = SessionCreatePatch
