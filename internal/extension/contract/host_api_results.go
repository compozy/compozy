package contract

import (
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"

	observepkg "github.com/compozy/agh/internal/observe"

	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
)

// SessionSummary is the lightweight host-visible session listing shape.
type SessionSummary struct {
	ID        string        `json:"id"`
	Name      string        `json:"name,omitempty"`
	Agent     string        `json:"agent"`
	Provider  string        `json:"provider"`
	Workspace string        `json:"workspace,omitempty"`
	State     session.State `json:"state"`
	CreatedAt time.Time     `json:"created_at"`
}

// SessionStatus is the detailed host-visible session status shape.
type SessionStatus struct {
	SessionID    string           `json:"session_id"`
	Name         string           `json:"name,omitempty"`
	Agent        string           `json:"agent"`
	Provider     string           `json:"provider"`
	WorkspaceID  string           `json:"workspace_id,omitempty"`
	Workspace    string           `json:"workspace,omitempty"`
	State        session.State    `json:"state"`
	StopReason   store.StopReason `json:"stop_reason,omitempty"`
	StopDetail   string           `json:"stop_detail,omitempty"`
	ACPSessionID string           `json:"acp_session_id,omitempty"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
}

// SessionEvent is the host-visible session or observe event record.
type SessionEvent struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data,omitempty"`
}

// SessionCreateResult returns the created session identifier.
type SessionCreateResult struct {
	SessionID string `json:"session_id"`
	Provider  string `json:"provider"`
}

// SessionPromptResult returns the created turn identifier.
type SessionPromptResult struct {
	TurnID string `json:"turn_id"`
}

// SandboxSummary is one active sandbox in the host-visible list response.
type SandboxSummary struct {
	SessionID  string `json:"session_id"`
	SandboxID  string `json:"sandbox_id"`
	Backend    string `json:"backend"`
	Profile    string `json:"profile,omitempty"`
	InstanceID string `json:"instance_id,omitempty"`
	State      string `json:"state"`
	SyncState  string `json:"sync_state,omitempty"`
}

// SandboxListResult returns active sandbox instances.
type SandboxListResult struct {
	Sandboxes []SandboxSummary `json:"sandboxes"`
}

// SandboxInfoResult returns detailed sandbox state for a session.
type SandboxInfoResult struct {
	SandboxID     string    `json:"sandbox_id"`
	Backend       string    `json:"backend"`
	Profile       string    `json:"profile"`
	InstanceID    string    `json:"instance_id"`
	RuntimeRoot   string    `json:"runtime_root"`
	SyncState     string    `json:"sync_state"`
	CreatedAt     time.Time `json:"created_at"`
	LastSyncError string    `json:"last_sync_error"`
}

// SandboxExecResult returns command execution output.
type SandboxExecResult struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
}

// MemoryRecallEntry is one scored memory lookup hit.
type MemoryRecallEntry struct {
	Key     string  `json:"key"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

// ObserveHealth is the host-visible daemon health payload.
type ObserveHealth = observepkg.Health

// BridgesMessagesIngestResult reports the resolved session association for one inbound message.
type BridgesMessagesIngestResult = bridgepkg.BridgesMessagesIngestResult
