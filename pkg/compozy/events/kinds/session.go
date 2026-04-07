package kinds

// SessionStatus describes the lifecycle state of a streamed session update.
type SessionStatus string

const (
	// StatusRunning marks an in-flight session update.
	StatusRunning SessionStatus = "running"
	// StatusCompleted marks a completed session.
	StatusCompleted SessionStatus = "completed"
	// StatusFailed marks a failed or canceled session.
	StatusFailed SessionStatus = "failed"
)

// SessionUpdateKind identifies the semantic variant of a session update.
type SessionUpdateKind string

const (
	// UpdateKindUnknown marks an update with no additional semantic classification.
	UpdateKindUnknown SessionUpdateKind = ""
	// UpdateKindUserMessageChunk marks a streamed user message chunk.
	UpdateKindUserMessageChunk SessionUpdateKind = "user_message_chunk"
	// UpdateKindAgentMessageChunk marks a streamed agent message chunk.
	UpdateKindAgentMessageChunk SessionUpdateKind = "agent_message_chunk"
	// UpdateKindAgentThoughtChunk marks a streamed agent thought chunk.
	UpdateKindAgentThoughtChunk SessionUpdateKind = "agent_thought_chunk"
	// UpdateKindToolCallStarted marks the start of a tool call lifecycle.
	UpdateKindToolCallStarted SessionUpdateKind = "tool_call_started"
	// UpdateKindToolCallUpdated marks an update to an existing tool call lifecycle.
	UpdateKindToolCallUpdated SessionUpdateKind = "tool_call_updated"
	// UpdateKindPlanUpdated marks a plan update.
	UpdateKindPlanUpdated SessionUpdateKind = "plan_updated"
	// UpdateKindAvailableCommandsUpdated marks an available commands update.
	UpdateKindAvailableCommandsUpdated SessionUpdateKind = "available_commands_updated"
	// UpdateKindCurrentModeUpdated marks a current mode update.
	UpdateKindCurrentModeUpdated SessionUpdateKind = "current_mode_updated"
)

// ToolCallState describes the lifecycle state of a tool call entry.
type ToolCallState string

const (
	// ToolCallStateUnknown marks a tool call without an explicit lifecycle state.
	ToolCallStateUnknown ToolCallState = ""
	// ToolCallStatePending marks a pending tool call.
	ToolCallStatePending ToolCallState = "pending"
	// ToolCallStateInProgress marks an in-flight tool call.
	ToolCallStateInProgress ToolCallState = "in_progress"
	// ToolCallStateCompleted marks a completed tool call.
	ToolCallStateCompleted ToolCallState = "completed"
	// ToolCallStateFailed marks a failed tool call.
	ToolCallStateFailed ToolCallState = "failed"
	// ToolCallStateWaitingForConfirmation is reserved for future permission-aware UX.
	ToolCallStateWaitingForConfirmation ToolCallState = "waiting_for_confirmation"
)

// SessionPlanEntry describes one plan entry.
type SessionPlanEntry struct {
	Content  string `json:"content"`
	Priority string `json:"priority"`
	Status   string `json:"status"`
}

// SessionAvailableCommand describes one slash-command style action.
type SessionAvailableCommand struct {
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	ArgumentHint string `json:"argument_hint,omitempty"`
}

// SessionUpdate is the public view of one streamed ACP update.
type SessionUpdate struct {
	Kind              SessionUpdateKind         `json:"kind,omitempty"`
	ToolCallID        string                    `json:"tool_call_id,omitempty"`
	ToolCallState     ToolCallState             `json:"tool_call_state,omitempty"`
	Blocks            []ContentBlock            `json:"blocks,omitempty"`
	ThoughtBlocks     []ContentBlock            `json:"thought_blocks,omitempty"`
	PlanEntries       []SessionPlanEntry        `json:"plan_entries,omitempty"`
	AvailableCommands []SessionAvailableCommand `json:"available_commands,omitempty"`
	CurrentModeID     string                    `json:"current_mode_id,omitempty"`
	Usage             Usage                     `json:"usage,omitempty"`
	Status            SessionStatus             `json:"status"`
}

// SessionStartedPayload describes a new attached session.
type SessionStartedPayload struct {
	Index          int    `json:"index"`
	ACPSessionID   string `json:"acp_session_id"`
	AgentSessionID string `json:"agent_session_id,omitempty"`
	Resumed        bool   `json:"resumed,omitempty"`
}

// SessionUpdatePayload carries one streamed session update.
type SessionUpdatePayload struct {
	Index  int           `json:"index"`
	Update SessionUpdate `json:"update"`
}

// SessionCompletedPayload describes a completed session.
type SessionCompletedPayload struct {
	Index int   `json:"index"`
	Usage Usage `json:"usage,omitempty"`
}

// SessionFailedPayload describes a failed session.
type SessionFailedPayload struct {
	Index int    `json:"index"`
	Error string `json:"error,omitempty"`
	Usage Usage  `json:"usage,omitempty"`
}
