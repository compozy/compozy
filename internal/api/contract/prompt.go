package contract

import (
	"time"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

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
	Runtime        *PromptRuntimeSelectionPayload `json:"runtime,omitzero"`
	Attachments    []PromptAttachmentRef          `json:"attachments,omitempty"`
	// InReplyTo names the message id this prompt answers. It requires a calling agent session.
	InReplyTo string `json:"in_reply_to,omitempty"`
	// Supersede replaces the caller's own older queued prompt to the same session.
	Supersede bool `json:"supersede,omitempty"`
}

// PromptAttachmentRef is one provider-neutral attachment on a prompt request.
type PromptAttachmentRef struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	MIMEType string `json:"mime_type"`
	Bytes    int64  `json:"bytes"`
	SHA256   string `json:"sha256"`
	Kind     string `json:"kind"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
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
	Attachments    []PromptAttachmentRef `json:"attachments,omitempty"`
	Text           string                `json:"text"`
	MessageID      string                `json:"message_id"`
	IdempotencyKey string                `json:"idempotency_key"`
	ExpectedTurnID string                `json:"expected_turn_id,omitempty"`
}

// SendPromptResultPayload reports non-streaming busy-input outcomes.
type ReplyWatchRefPayload struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

type SendPromptResultPayload struct {
	ReplyWatch            *ReplyWatchRefPayload   `json:"reply_watch,omitempty"`
	TargetWorkspaceID     string                  `json:"target_workspace_id,omitempty"`
	Disposition           session.Disposition     `json:"disposition,omitempty"`
	SteerDelivery         store.SteerDeliveryMode `json:"steer_delivery,omitempty"`
	TurnID                string                  `json:"turn_id,omitempty"`
	EntryID               string                  `json:"entry_id,omitempty"`
	Status                string                  `json:"status"`
	Mode                  PromptMode              `json:"mode,omitempty"`
	Delivery              PromptDelivery          `json:"delivery"`
	MessageID             string                  `json:"message_id"`
	IdempotencyKey        string                  `json:"idempotency_key"`
	Replayed              bool                    `json:"replayed"`
	QueueEntryID          string                  `json:"queue_entry_id,omitempty"`
	QueuePosition         int                     `json:"queue_position"`
	QueueGeneration       int64                   `json:"queue_generation,omitzero"`
	EstimatedSendAt       *time.Time              `json:"estimated_send_at,omitempty"`
	PreviousTurnID        string                  `json:"previous_turn_id,omitempty"`
	NewTurnID             string                  `json:"new_turn_id,omitempty"`
	CanceledQueuedEntries int                     `json:"canceled_queued_entries,omitzero"`
	Goal                  *GoalCommandResult      `json:"goal,omitzero"`
}

// SessionInputPayload is one durable operator input waiting for session dispatch.
type SessionInputPayload struct {
	Origin           *PromptOriginMeta               `json:"origin,omitzero"`
	ID               string                          `json:"id"`
	SessionID        string                          `json:"session_id"`
	OwnerKind        string                          `json:"owner_kind,omitempty"`
	OwnerID          string                          `json:"owner_id,omitempty"`
	MessageID        string                          `json:"message_id,omitempty"`
	IdempotencyKey   string                          `json:"idempotency_key,omitempty"`
	TargetTurnID     string                          `json:"target_turn_id,omitempty"`
	Status           SessionInputStatus              `json:"status"`
	Mode             PromptMode                      `json:"mode"`
	Delivery         PromptDelivery                  `json:"delivery"`
	SteerDelivery    store.SteerDeliveryMode         `json:"steer_delivery,omitempty"`
	Text             string                          `json:"text"`
	QueueGeneration  int64                           `json:"queue_generation"`
	EnqueuedAt       time.Time                       `json:"enqueued_at"`
	Runtime          *PromptRuntimeSelectionPayload  `json:"runtime,omitzero"`
	SkillInvocations []SessionSkillInvocationPayload `json:"skill_invocations,omitempty"`
	Mentions         []SessionInputMentionPayload    `json:"mentions,omitempty"`
	Attachments      []PromptAttachmentRef           `json:"attachments,omitempty"`
}

// SessionInputMentionPayload is one verified @-mention on queued input. Target is
// relative to the session working directory (folders end in "/") or a session id;
// offsets are UTF-16 code units in Text, end exclusive.
type SessionInputMentionPayload struct {
	Kind   string `json:"kind"`
	Token  string `json:"token"`
	Target string `json:"target"`
	Label  string `json:"label"`
	Start  int    `json:"start"`
	End    int    `json:"end"`
}

// SessionInputListResponse returns current-generation pending input in dispatch order.
type SessionInputListResponse struct {
	Inputs []SessionInputPayload       `json:"inputs"`
	Queue  *SessionQueueSummaryPayload `json:"queue,omitzero"`
}

// SessionQueueSummaryPayload exposes the daemon's current admission capacity.
type SessionQueueSummaryPayload struct {
	Entries int `json:"entries"`
	Cap     int `json:"cap"`
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
	ExpectedTurnID string `json:"expected_turn_id,omitempty"`
}

// PromptStreamChunk carries the extensible AI SDK UI message stream discriminator.
type PromptStreamChunk struct {
	Type string `json:"type"`
}
