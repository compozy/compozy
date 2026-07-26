package core

import (
	"encoding/json"
	"errors"

	"strings"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/session"
)

const (
	promptStreamErrorKey = "error"
	promptStreamStopKey  = "stop"
)

func AcceptedPromptStreamTurnID(result session.SendPromptResult) (string, error) {
	turnID := strings.TrimSpace(result.NewTurnID)
	if turnID == "" {
		return "", errors.New("accepted prompt stream missing turn id")
	}
	return turnID, nil
}

type promptAgentEventPayload struct {
	Type       string                           `json:"type"`
	SessionID  string                           `json:"session_id,omitempty"`
	TurnID     string                           `json:"turn_id,omitempty"`
	RequestID  string                           `json:"request_id,omitempty"`
	Timestamp  string                           `json:"timestamp,omitempty"`
	Text       string                           `json:"text,omitempty"`
	Title      string                           `json:"title,omitempty"`
	ToolCallID string                           `json:"tool_call_id,omitempty"`
	StopReason string                           `json:"stop_reason,omitempty"`
	Action     string                           `json:"action,omitempty"`
	Resource   string                           `json:"resource,omitempty"`
	Decision   string                           `json:"decision,omitempty"`
	Error      string                           `json:"error,omitempty"`
	Usage      *promptTokenUsagePayload         `json:"usage,omitempty"`
	Runtime    *contract.RuntimeActivityPayload `json:"runtime,omitempty"`
	Raw        json.RawMessage                  `json:"raw,omitempty"`
}

type promptTokenUsagePayload struct {
	TurnID           string   `json:"turn_id,omitempty"`
	InputTokens      *int64   `json:"input_tokens,omitempty"`
	OutputTokens     *int64   `json:"output_tokens,omitempty"`
	TotalTokens      *int64   `json:"total_tokens,omitempty"`
	ThoughtTokens    *int64   `json:"thought_tokens,omitempty"`
	CacheReadTokens  *int64   `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens *int64   `json:"cache_write_tokens,omitempty"`
	ContextUsed      *int64   `json:"context_used,omitempty"`
	ContextSize      *int64   `json:"context_size,omitempty"`
	CostAmount       *float64 `json:"cost_amount,omitempty"`
	CostCurrency     *string  `json:"cost_currency,omitempty"`
	Timestamp        string   `json:"timestamp,omitempty"`
}

type promptFinishPayload struct {
	Type         string `json:"type"`
	FinishReason string `json:"finishReason,omitempty"`
}

type promptStartPayload struct {
	Type      string `json:"type"`
	MessageID string `json:"messageId"`
}

type promptBlockPayload struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type promptDeltaPayload struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Delta string `json:"delta"`
}

type promptToolInputStartPayload struct {
	Type       string `json:"type"`
	ToolCallID string `json:"toolCallId"`
	ToolName   string `json:"toolName"`
}

type promptToolInputAvailablePayload struct {
	Type       string `json:"type"`
	ToolCallID string `json:"toolCallId"`
	ToolName   string `json:"toolName"`
	Input      any    `json:"input"`
}

type promptDataEventEnvelope struct {
	Type string                  `json:"type"`
	ID   string                  `json:"id,omitempty"`
	Data promptAgentEventPayload `json:"data"`
}

type promptToolOutputAvailablePayload struct {
	Type       string                  `json:"type"`
	ToolCallID string                  `json:"toolCallId"`
	Output     promptAgentEventPayload `json:"output"`
}

type promptErrorPayload struct {
	Type      string `json:"type"`
	ErrorText string `json:"errorText"`
}

// PromptStreamEncoder converts raw ACP agent events into the typed public
// prompt stream envelope used by HTTP, UDS, and CLI streaming surfaces.
type PromptStreamEncoder struct {
	now               func() string
	messageID         string
	textBlockID       string
	reasoningBlockID  string
	textBlockSeq      int
	reasoningBlockSeq int
	messageStarted    bool
	textStarted       bool
	reasoningStarted  bool
	toolStarted       map[string]struct{}
	toolInputsReady   map[string]struct{}
	toolInputPending  map[string]struct{}
	toolNames         map[string]string
	finished          bool
}

// NewPromptStreamEncoder constructs a prompt-stream encoder with deterministic
// timestamps for generated IDs when provided.
func NewPromptStreamEncoder(now func() time.Time) *PromptStreamEncoder {
	clock := func() string {
		return time.Now().UTC().Format(time.RFC3339Nano)
	}
	if now != nil {
		clock = func() string {
			return now().UTC().Format(time.RFC3339Nano)
		}
	}

	return &PromptStreamEncoder{
		now:              clock,
		toolStarted:      make(map[string]struct{}),
		toolInputsReady:  make(map[string]struct{}),
		toolInputPending: make(map[string]struct{}),
		toolNames:        make(map[string]string),
	}
}

// Emit writes one public prompt-stream frame for the supplied raw agent event.
func (e *PromptStreamEncoder) Emit(writer FlushWriter, event acp.AgentEvent) error {
	e.ensureInitialized()
	if err := e.ensureMessageStarted(writer, event); err != nil {
		return err
	}

	switch event.Type {
	case acp.EventTypeAgentMessage:
		return e.emitAgentMessage(writer, event)
	case acp.EventTypeThought:
		return e.emitThought(writer, event)
	case acp.EventTypeToolCall:
		return e.emitToolCall(writer, event)
	case acp.EventTypeToolResult:
		return e.emitToolResult(writer, event)
	case acp.EventTypePermission:
		return e.emitPermission(writer, event)
	case acp.EventTypeError:
		return e.emitError(writer, event)
	case acp.EventTypeDone:
		return e.finish(writer, event)
	default:
		return e.emitGenericEvent(writer, event)
	}
}

// Start emits the public prompt stream start frame as soon as the daemon has
// accepted a durable turn. Later event frames reuse the same message id.
func (e *PromptStreamEncoder) Start(writer FlushWriter, messageID string) error {
	e.ensureInitialized()
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return errors.New("prompt stream start requires message id")
	}
	if e.messageStarted {
		return nil
	}
	e.messageID = messageID
	e.messageStarted = true
	return WriteSSE(writer, SSEMessage{
		Data: promptStartPayload{Type: "start", MessageID: e.messageID},
	})
}

// Finish closes any open blocks and emits the terminal finish frame followed by
// the `[DONE]` sentinel exactly once.
func (e *PromptStreamEncoder) Finish(writer FlushWriter, event acp.AgentEvent) error {
	e.ensureInitialized()
	return e.finish(writer, event)
}

func (e *PromptStreamEncoder) ensureInitialized() {
	if e.now == nil {
		e.now = func() string {
			return time.Now().UTC().Format(time.RFC3339Nano)
		}
	}
	if e.toolStarted == nil {
		e.toolStarted = make(map[string]struct{})
	}
	if e.toolInputsReady == nil {
		e.toolInputsReady = make(map[string]struct{})
	}
	if e.toolInputPending == nil {
		e.toolInputPending = make(map[string]struct{})
	}
	if e.toolNames == nil {
		e.toolNames = make(map[string]string)
	}
}
