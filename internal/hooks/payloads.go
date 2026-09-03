package hooks

import (
	"encoding/json"
	"errors"
	"strings"
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
	ProfileID   string `json:"profile_id,omitempty"`
	SessionID   string `json:"session_id,omitempty"`
	SessionName string `json:"session_name,omitempty"`
	SessionType string `json:"session_type,omitempty"`
	AgentName   string `json:"agent_name,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	Workspace   string `json:"workspace,omitempty"`
	*SessionRuntimeContext
	State string `json:"state,omitempty"`
	*SessionSoulContext
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// HookProfileID returns the durable owner used to isolate profile-scoped declarations.
func (c SessionContext) HookProfileID() string { return strings.TrimSpace(c.ProfileID) }

// SessionRuntimeContext carries optional runtime bindings on session-scoped hooks.
// Embedding preserves the flat hook payload JSON contract.
type SessionRuntimeContext struct {
	WorktreeID   string `json:"worktree_id,omitempty"`
	ACPSessionID string `json:"acp_session_id,omitempty"`
}

// WorktreeIDValue returns the optional worktree binding.
func (c SessionContext) WorktreeIDValue() string {
	if c.SessionRuntimeContext == nil {
		return ""
	}
	return c.WorktreeID
}

// ACPSessionIDValue returns the optional ACP session binding.
func (c SessionContext) ACPSessionIDValue() string {
	if c.SessionRuntimeContext == nil {
		return ""
	}
	return c.ACPSessionID
}

// NewSessionRuntimeContext returns compact runtime bindings when either value is present.
func NewSessionRuntimeContext(worktreeID string, acpSessionID string) *SessionRuntimeContext {
	worktreeID = strings.TrimSpace(worktreeID)
	acpSessionID = strings.TrimSpace(acpSessionID)
	if worktreeID == "" && acpSessionID == "" {
		return nil
	}
	return &SessionRuntimeContext{WorktreeID: worktreeID, ACPSessionID: acpSessionID}
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

// SessionRuntimeRecoveryPayload reports one automatic runtime recovery lifecycle event.
type SessionRuntimeRecoveryPayload struct {
	PayloadBase
	SessionContext
	TurnContext
	RunID         string `json:"run_id"`
	Attempt       int    `json:"attempt"`
	MaxAttempts   int    `json:"max_attempts"`
	Generation    int64  `json:"generation"`
	FailureKind   string `json:"failure_kind,omitempty"`
	FailureDetail string `json:"failure_detail,omitempty"`
}

// SessionRuntimeRecoveryStartedPayload is emitted before a bounded recovery attempt.
type SessionRuntimeRecoveryStartedPayload = SessionRuntimeRecoveryPayload

// SessionRuntimeRecoverySucceededPayload is emitted after the replacement runtime accepts the turn.
type SessionRuntimeRecoverySucceededPayload = SessionRuntimeRecoveryPayload

// SessionRuntimeRecoveryExhaustedPayload is emitted after all recovery attempts fail.
type SessionRuntimeRecoveryExhaustedPayload = SessionRuntimeRecoveryPayload

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
