package contract

import (
	"time"

	apicontract "github.com/compozy/compozy/internal/api/contract"
	bridgepkg "github.com/compozy/compozy/internal/bridges/contract"

	observepkg "github.com/compozy/compozy/internal/observe"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

// SessionSummary is the lightweight host-visible session listing shape.
type SessionSummary struct {
	ID        string                            `json:"id"`
	Name      string                            `json:"name,omitempty"`
	Agent     string                            `json:"agent"`
	Runtime   apicontract.SessionRuntimePayload `json:"runtime"`
	Workspace string                            `json:"workspace,omitempty"`
	State     session.State                     `json:"state"`
	CreatedAt time.Time                         `json:"created_at"`
}

// SessionStatus is the detailed host-visible session status shape.
type SessionStatus struct {
	SessionID   string                            `json:"session_id"`
	Name        string                            `json:"name,omitempty"`
	Agent       string                            `json:"agent"`
	Runtime     apicontract.SessionRuntimePayload `json:"runtime"`
	WorkspaceID string                            `json:"workspace_id,omitempty"`
	Workspace   string                            `json:"workspace,omitempty"`
	State       session.State                     `json:"state"`
	StopReason  store.StopReason                  `json:"stop_reason,omitempty"`
	StopDetail  string                            `json:"stop_detail,omitempty"`
	CreatedAt   time.Time                         `json:"created_at"`
	UpdatedAt   time.Time                         `json:"updated_at"`
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
}

// SessionPromptResult reports an immediate or deferred prompt admission.
type SessionPromptResult struct {
	Status                string                `json:"status"`
	Mode                  session.BusyInputMode `json:"mode,omitempty"`
	Delivery              string                `json:"delivery"`
	MessageID             string                `json:"message_id"`
	IdempotencyKey        string                `json:"idempotency_key"`
	Replayed              bool                  `json:"replayed"`
	TurnID                string                `json:"turn_id,omitempty"`
	QueueEntryID          string                `json:"queue_entry_id,omitempty"`
	QueuePosition         int                   `json:"queue_position,omitempty"`
	QueueGeneration       int64                 `json:"queue_generation,omitempty"`
	EstimatedSendAt       *time.Time            `json:"estimated_send_at,omitempty"`
	PreviousTurnID        string                `json:"previous_turn_id,omitempty"`
	CanceledQueuedEntries int                   `json:"canceled_queued_entries,omitempty"`
}

// SessionInput is one durable operator input waiting for session dispatch.
type SessionInput struct {
	ID              string                                     `json:"id"`
	SessionID       string                                     `json:"session_id"`
	MessageID       string                                     `json:"message_id,omitempty"`
	IdempotencyKey  string                                     `json:"idempotency_key,omitempty"`
	TargetTurnID    string                                     `json:"target_turn_id,omitempty"`
	Status          string                                     `json:"status"`
	Mode            session.BusyInputMode                      `json:"mode"`
	Delivery        string                                     `json:"delivery"`
	Text            string                                     `json:"text"`
	QueueGeneration int64                                      `json:"queue_generation"`
	EnqueuedAt      time.Time                                  `json:"enqueued_at"`
	Runtime         *apicontract.PromptRuntimeSelectionPayload `json:"runtime,omitempty"`
}

// SessionInputListResult returns pending input in daemon dispatch order.
type SessionInputListResult struct {
	Inputs []SessionInput `json:"inputs"`
}

// SessionInputResult returns one durable pending input.
type SessionInputResult struct {
	Input SessionInput `json:"input"`
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
