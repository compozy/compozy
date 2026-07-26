package contract

import "time"

// PromptMode selects how prompt input is handled while the session is busy.
type PromptMode string

const (
	PromptModeQueue     PromptMode = "queue"
	PromptModeInterrupt PromptMode = "interrupt"
	PromptModeSteer     PromptMode = "steer"
)

// SendPromptRequest captures user-facing prompt input plus optional busy-input mode.
type SendPromptRequest struct {
	Message   string            `json:"message,omitempty"`
	Messages  []PromptUIMessage `json:"messages,omitempty"`
	MessageID string            `json:"messageId,omitempty"`
	Mode      PromptMode        `json:"mode,omitempty"`
}

// PromptUIMessage carries Vercel AI SDK compatible message input.
type PromptUIMessage struct {
	ID      string             `json:"id,omitempty"`
	Role    string             `json:"role"`
	Content string             `json:"content,omitempty"`
	Parts   []PromptUITextPart `json:"parts,omitempty"`
}

// PromptUITextPart carries one text part from a Vercel AI SDK message.
type PromptUITextPart struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

// SteerPromptRequest captures staged steering guidance for an active session.
type SteerPromptRequest struct {
	Text string `json:"text"`
}

// SendPromptResultPayload reports non-streaming busy-input outcomes.
type SendPromptResultPayload struct {
	Status                     string             `json:"status"`
	Mode                       PromptMode         `json:"mode,omitempty"`
	Queued                     bool               `json:"queued,omitempty"`
	Staged                     bool               `json:"staged,omitempty"`
	Interrupted                bool               `json:"interrupted,omitempty"`
	QueueEntryID               string             `json:"queue_entry_id,omitempty"`
	QueuePosition              int                `json:"queue_position,omitempty"`
	QueueGeneration            int64              `json:"queue_generation,omitempty"`
	EstimatedSendAt            *time.Time         `json:"estimated_send_at,omitempty"`
	PreviousTurnID             string             `json:"previous_turn_id,omitempty"`
	NewTurnID                  string             `json:"new_turn_id,omitempty"`
	CanceledQueuedEntries      int                `json:"canceled_queued_entries,omitempty"`
	FallbackModeIfNoToolResult PromptMode         `json:"fallback_mode_if_no_tool_result,omitempty"`
	Goal                       *GoalCommandResult `json:"goal,omitempty"`
}
