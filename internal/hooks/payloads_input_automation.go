package hooks

import "encoding/json"

// InputPreSubmitPayload is delivered before prompt submission.
type InputPreSubmitPayload struct {
	PayloadBase
	SessionContext
	TurnContext
	InputClass    string         `json:"input_class,omitempty"`
	Message       string         `json:"message,omitempty"`
	ContextBlocks []ContextBlock `json:"context_blocks,omitempty"`
}

// InputPreSubmitPatch mutates or denies the submitted input.
type InputPreSubmitPatch struct {
	ControlPatch
	Message       *string        `json:"message,omitempty"`
	ContextBlocks []ContextBlock `json:"context_blocks,omitempty"`
}

// PromptPayload is delivered after prompt assembly.
type PromptPayload struct {
	PayloadBase
	SessionContext
	TurnContext
	InputClass    string         `json:"input_class,omitempty"`
	Prompt        string         `json:"prompt,omitempty"`
	ContextBlocks []ContextBlock `json:"context_blocks,omitempty"`
}

// PromptPatch mutates or denies the assembled prompt.
type PromptPatch struct {
	ControlPatch
	Prompt        *string        `json:"prompt,omitempty"`
	ContextBlocks []ContextBlock `json:"context_blocks,omitempty"`
}

// EventRecordPayload is shared by event pre/post-record hooks.
type EventRecordPayload struct {
	PayloadBase
	SessionContext
	TurnContext
	RecordType string          `json:"record_type,omitempty"`
	Sequence   int64           `json:"sequence,omitempty"`
	Content    json.RawMessage `json:"content,omitempty"`
}

// EventPreRecordPayload is delivered before an event record is written.
type EventPreRecordPayload = EventRecordPayload

// EventPostRecordPayload is delivered after an event record is written.
type EventPostRecordPayload = EventRecordPayload

// EventRecordPatch captures the optional observation patch surface for event hooks.
type EventRecordPatch struct {
	Labels map[string]string `json:"labels,omitempty"`
}

// EventPreRecordPatch is the pre-record patch surface.
type EventPreRecordPatch = EventRecordPatch

// EventPostRecordPatch is the post-record patch surface.
type EventPostRecordPatch = EventRecordPatch

// AutomationSchedulePayload carries the serializable schedule shape exposed to automation hooks.
type AutomationSchedulePayload struct {
	Mode     string `json:"mode,omitempty"`
	Expr     string `json:"expr,omitempty"`
	Interval string `json:"interval,omitempty"`
	Time     string `json:"time,omitempty"`
}

// AutomationJobPreFirePayload is delivered before a job fire dispatches.
type AutomationJobPreFirePayload struct {
	JobID       string                     `json:"job_id"`
	JobName     string                     `json:"job_name,omitempty"`
	AgentName   string                     `json:"agent_name,omitempty"`
	WorkspaceID string                     `json:"workspace_id,omitempty"`
	Prompt      string                     `json:"prompt,omitempty"`
	Schedule    *AutomationSchedulePayload `json:"schedule,omitempty"`
	Payload     map[string]any             `json:"payload,omitempty"`
	Attempt     int                        `json:"attempt,omitempty"`
}

// AutomationJobPostFirePayload is delivered after a job fire hands off to a session.
type AutomationJobPostFirePayload struct {
	JobID       string `json:"job_id"`
	JobName     string `json:"job_name,omitempty"`
	AgentName   string `json:"agent_name,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	RunID       string `json:"run_id,omitempty"`
	SessionID   string `json:"session_id,omitempty"`
}

// AutomationTriggerPreFirePayload is delivered before a trigger fire dispatches.
type AutomationTriggerPreFirePayload struct {
	TriggerID   string         `json:"trigger_id"`
	TriggerName string         `json:"trigger_name,omitempty"`
	Event       string         `json:"event,omitempty"`
	AgentName   string         `json:"agent_name,omitempty"`
	WorkspaceID string         `json:"workspace_id,omitempty"`
	Prompt      string         `json:"prompt,omitempty"`
	Payload     map[string]any `json:"payload,omitempty"`
	Attempt     int            `json:"attempt,omitempty"`
}

// AutomationTriggerPostFirePayload is delivered after a trigger fire hands off to a session.
type AutomationTriggerPostFirePayload struct {
	TriggerID   string `json:"trigger_id"`
	TriggerName string `json:"trigger_name,omitempty"`
	Event       string `json:"event,omitempty"`
	AgentName   string `json:"agent_name,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	RunID       string `json:"run_id,omitempty"`
	SessionID   string `json:"session_id,omitempty"`
}

// AutomationRunCompletedPayload is delivered after an automation run finishes successfully.
type AutomationRunCompletedPayload struct {
	RunID       string `json:"run_id"`
	JobID       string `json:"job_id,omitempty"`
	TriggerID   string `json:"trigger_id,omitempty"`
	AgentName   string `json:"agent_name,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	SessionID   string `json:"session_id,omitempty"`
	Attempt     int    `json:"attempt,omitempty"`
	DurationMS  int64  `json:"duration_ms,omitempty"`
}

// AutomationRunFailedPayload is delivered after an automation run fails.
type AutomationRunFailedPayload struct {
	RunID       string `json:"run_id"`
	JobID       string `json:"job_id,omitempty"`
	TriggerID   string `json:"trigger_id,omitempty"`
	AgentName   string `json:"agent_name,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	SessionID   string `json:"session_id,omitempty"`
	Error       string `json:"error,omitempty"`
	Attempt     int    `json:"attempt,omitempty"`
	WillRetry   bool   `json:"will_retry,omitempty"`
}

// AutomationFirePatch mutates or cancels one automation pre-fire dispatch.
type AutomationFirePatch struct {
	Prompt *string `json:"prompt,omitempty"`
	Cancel bool    `json:"cancel,omitempty"`
}

// AutomationObservationPatch is the no-op patch surface for async automation observation hooks.
type AutomationObservationPatch struct{}
