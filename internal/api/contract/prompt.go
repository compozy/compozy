package contract

import "time"

// PromptMode selects how prompt input is handled while the session is busy.
type PromptMode string

const (
	PromptModeQueue     PromptMode = "queue"
	PromptModeInterrupt PromptMode = "interrupt"
	PromptModeSteer     PromptMode = "steer"
)

// PromptDelivery describes when the daemon will submit accepted input.
type PromptDelivery string

const (
	PromptDeliveryNone                PromptDelivery = "none"
	PromptDeliveryDirect              PromptDelivery = "direct"
	PromptDeliveryAfterTurn           PromptDelivery = "after_turn"
	PromptDeliveryInterruptThenPrompt PromptDelivery = "interrupt_then_prompt"
)

// SendPromptRequest captures user-facing prompt input plus optional busy-input mode.
type SendPromptRequest struct {
	Message        string                         `json:"message,omitempty"`
	Messages       []PromptUIMessage              `json:"messages,omitempty"`
	MessageID      string                         `json:"message_id"`
	IdempotencyKey string                         `json:"idempotency_key"`
	Mode           PromptMode                     `json:"mode,omitempty"`
	ExpectedTurnID string                         `json:"expected_turn_id,omitempty"`
	Runtime        *PromptRuntimeSelectionPayload `json:"runtime,omitempty"`
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

// SteerPromptRequest captures replacement guidance for a fenced active session turn.
type SteerPromptRequest struct {
	Text           string `json:"text"`
	MessageID      string `json:"message_id"`
	IdempotencyKey string `json:"idempotency_key"`
	ExpectedTurnID string `json:"expected_turn_id"`
}

// SendPromptResultPayload reports non-streaming busy-input outcomes.
type SendPromptResultPayload struct {
	Status                string             `json:"status"`
	Mode                  PromptMode         `json:"mode,omitempty"`
	Delivery              PromptDelivery     `json:"delivery"`
	MessageID             string             `json:"message_id"`
	IdempotencyKey        string             `json:"idempotency_key"`
	Replayed              bool               `json:"replayed"`
	QueueEntryID          string             `json:"queue_entry_id,omitempty"`
	QueuePosition         int                `json:"queue_position,omitempty"`
	QueueGeneration       int64              `json:"queue_generation,omitempty"`
	EstimatedSendAt       *time.Time         `json:"estimated_send_at,omitempty"`
	PreviousTurnID        string             `json:"previous_turn_id,omitempty"`
	NewTurnID             string             `json:"new_turn_id,omitempty"`
	CanceledQueuedEntries int                `json:"canceled_queued_entries,omitempty"`
	Goal                  *GoalCommandResult `json:"goal,omitempty"`
}

// SessionInputPayload is one durable operator input waiting for session dispatch.
type SessionInputPayload struct {
	ID               string                          `json:"id"`
	SessionID        string                          `json:"session_id"`
	MessageID        string                          `json:"message_id,omitempty"`
	IdempotencyKey   string                          `json:"idempotency_key,omitempty"`
	TargetTurnID     string                          `json:"target_turn_id,omitempty"`
	Status           string                          `json:"status"`
	Mode             PromptMode                      `json:"mode"`
	Delivery         PromptDelivery                  `json:"delivery"`
	Text             string                          `json:"text"`
	QueueGeneration  int64                           `json:"queue_generation"`
	EnqueuedAt       time.Time                       `json:"enqueued_at"`
	Runtime          *PromptRuntimeSelectionPayload  `json:"runtime,omitempty"`
	SkillInvocations []SessionSkillInvocationPayload `json:"skill_invocations,omitempty"`
}

// SessionInputListResponse returns current-generation pending input in dispatch order.
type SessionInputListResponse struct {
	Inputs []SessionInputPayload `json:"inputs"`
}

// SessionInputResponse returns one durable pending input.
type SessionInputResponse struct {
	Input SessionInputPayload `json:"input"`
}

// ReplaceSessionInputRequest atomically replaces one queued input.
type ReplaceSessionInputRequest struct {
	Text           string `json:"text"`
	MessageID      string `json:"message_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

// PromoteSessionInputRequest atomically replaces queued input with steering input.
type PromoteSessionInputRequest struct {
	Text           string `json:"text"`
	MessageID      string `json:"message_id"`
	IdempotencyKey string `json:"idempotency_key"`
	ExpectedTurnID string `json:"expected_turn_id"`
}
