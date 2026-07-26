package transcript

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
)

const (
	UIRoleSystem    = "system"
	UIRoleUser      = "user"
	UIRoleAssistant = "assistant"

	uiPartText           = "text"
	uiPartReasoning      = "reasoning"
	uiPartDynamicTool    = "dynamic-tool"
	uiPartDataEvent      = "data-agh-event"
	uiPartDataPermission = "data-agh-permission"
	uiPartStateStreaming = "streaming"
	uiPartStateDone      = "done"
	uiToolStateStreaming = "input-streaming"
	uiToolStateAvailable = "input-available"
	uiToolStateOutput    = "output-available"
	uiToolStateError     = "output-error"
)

var emptyJSONObject = json.RawMessage(`{}`)

// UIMessage mirrors the AI SDK UIMessage wire shape used by the web client.
type UIMessage struct {
	ID       string          `json:"id"`
	Role     string          `json:"role"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
	Parts    []UIMessagePart `json:"parts"`
}

// UIMessagePart mirrors the AI SDK UIMessage part wire shape used by the web client.
type UIMessagePart struct {
	Type        string          `json:"type"`
	ID          string          `json:"id,omitempty"`
	Text        string          `json:"text,omitempty"`
	State       string          `json:"state,omitempty"`
	ToolName    string          `json:"toolName,omitempty"`
	ToolCallID  string          `json:"toolCallId,omitempty"`
	Title       string          `json:"title,omitempty"`
	Input       json.RawMessage `json:"input,omitempty"`
	RawInput    json.RawMessage `json:"rawInput,omitempty"`
	Output      json.RawMessage `json:"output,omitempty"`
	ErrorText   string          `json:"errorText,omitempty"`
	Data        json.RawMessage `json:"data,omitempty"`
	Preliminary bool            `json:"preliminary,omitempty"`
}

// UITokenUsagePayload mirrors the prompt-stream token usage payload.
type UITokenUsagePayload struct {
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

type decodedStoredEvent struct {
	stored store.SessionEvent
	parsed event
	agent  acp.AgentEvent
}

type uiMessageBuilder struct {
	logicalID        string
	id               string
	role             string
	finished         bool
	parts            []UIMessagePart
	activePartType   string
	activePartIndex  int
	textPartSeq      int
	reasoningPartSeq int
	toolIndices      map[string]int
	toolLifecycles   map[string]*uiToolLifecycle
}

type uiToolLifecycle struct {
	input json.RawMessage
}

func uiTokenUsagePayloadFromUsage(usage *acp.TokenUsage) *UITokenUsagePayload {
	if usage == nil {
		return nil
	}

	payload := &UITokenUsagePayload{
		TurnID:           usage.TurnID,
		InputTokens:      usage.InputTokens,
		OutputTokens:     usage.OutputTokens,
		TotalTokens:      usage.TotalTokens,
		ThoughtTokens:    usage.ThoughtTokens,
		CacheReadTokens:  usage.CacheReadTokens,
		CacheWriteTokens: usage.CacheWriteTokens,
		ContextUsed:      usage.ContextUsed,
		ContextSize:      usage.ContextSize,
		CostAmount:       usage.CostAmount,
		CostCurrency:     usage.CostCurrency,
	}
	if !usage.Timestamp.IsZero() {
		payload.Timestamp = usage.Timestamp.UTC().Format(time.RFC3339Nano)
	}
	return payload
}

func payloadJSONBytes(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	if json.Valid(raw) {
		return acp.CloneRawMessage(raw)
	}
	return rawMessageFromValue(string(raw))
}

// ToUIMessages projects persisted session events into AI SDK UIMessage objects.
func ToUIMessages(events []store.SessionEvent) ([]UIMessage, error) {
	entries, err := ToUIEntries(events)
	if err != nil {
		return nil, err
	}
	return MessagesFromEntries(entries), nil
}

func newUIMessageBuilder(
	id string,
	logicalID string,
	role string,
	toolLifecycles map[string]*uiToolLifecycle,
) *uiMessageBuilder {
	return &uiMessageBuilder{
		logicalID:       logicalID,
		id:              id,
		role:            role,
		activePartIndex: -1,
		toolIndices:     make(map[string]int),
		toolLifecycles:  toolLifecycles,
	}
}

func (b *uiMessageBuilder) build(complete bool) *UIMessage {
	if b == nil || len(b.parts) == 0 {
		return nil
	}

	parts := cloneUIMessageParts(b.parts)
	for index := range parts {
		switch parts[index].Type {
		case uiPartText, uiPartReasoning:
			if complete {
				parts[index].State = uiPartStateDone
			} else if parts[index].State == "" {
				parts[index].State = uiPartStateStreaming
			}
		}
	}

	return &UIMessage{
		ID:    b.id,
		Role:  b.role,
		Parts: parts,
	}
}

func cloneUIMessageParts(parts []UIMessagePart) []UIMessagePart {
	cloned := make([]UIMessagePart, 0, len(parts))
	for _, part := range parts {
		next := part
		next.Input = acp.CloneRawMessage(part.Input)
		next.RawInput = acp.CloneRawMessage(part.RawInput)
		next.Output = acp.CloneRawMessage(part.Output)
		next.Data = acp.CloneRawMessage(part.Data)
		cloned = append(cloned, next)
	}
	return cloned
}

func applyDecodedEvent(builder *uiMessageBuilder, decoded *decodedStoredEvent) {
	switch decoded.parsed.Type {
	case acp.EventTypeAgentMessage:
		builder.appendText(decoded.parsed.Text)
	case acp.EventTypeThought:
		builder.appendReasoning(decoded.parsed.Text)
	case acp.EventTypeToolCall:
		builder.applyToolCall(decoded)
		builder.appendDataPart(uiPartDataEvent, "", decoded.dataPayload())
	case acp.EventTypeToolResult:
		builder.applyToolResult(decoded)
	case acp.EventTypePermission:
		builder.appendDataPart(uiPartDataPermission, uiPermissionDataPartID(decoded.agent), decoded.dataPayload())
	case acp.EventTypeError:
		builder.appendDataPart(uiPartDataEvent, "", decoded.dataPayload())
		builder.finished = true
	case acp.EventTypeDone:
		builder.finished = true
	default:
		builder.appendDataPart(uiPartDataEvent, "", decoded.dataPayload())
	}
}

func (b *uiMessageBuilder) appendText(text string) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" && b.activePartType != uiPartText {
		return
	}
	index := b.ensureTextPart()
	b.parts[index].Text += text
	if b.parts[index].State == "" {
		b.parts[index].State = uiPartStateStreaming
	}
}

func (b *uiMessageBuilder) appendReasoning(text string) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" && b.activePartType != uiPartReasoning {
		return
	}
	index := b.ensureReasoningPart()
	b.parts[index].Text += text
	if b.parts[index].State == "" {
		b.parts[index].State = uiPartStateStreaming
	}
}

func (b *uiMessageBuilder) ensureTextPart() int {
	if b.activePartType == uiPartText && b.activePartIndex >= 0 {
		return b.activePartIndex
	}
	b.closeActiveStreamPart()
	b.textPartSeq++
	b.parts = append(b.parts, UIMessagePart{
		Type:  uiPartText,
		ID:    fmt.Sprintf("%s-text-%d", b.id, b.textPartSeq),
		State: uiPartStateStreaming,
	})
	b.activePartType = uiPartText
	b.activePartIndex = len(b.parts) - 1
	return b.activePartIndex
}

func (b *uiMessageBuilder) ensureReasoningPart() int {
	if b.activePartType == uiPartReasoning && b.activePartIndex >= 0 {
		return b.activePartIndex
	}
	b.closeActiveStreamPart()
	b.reasoningPartSeq++
	b.parts = append(b.parts, UIMessagePart{
		Type:  uiPartReasoning,
		ID:    fmt.Sprintf("%s-reasoning-%d", b.id, b.reasoningPartSeq),
		State: uiPartStateStreaming,
	})
	b.activePartType = uiPartReasoning
	b.activePartIndex = len(b.parts) - 1
	return b.activePartIndex
}

func (b *uiMessageBuilder) closeActiveStreamPart() {
	if b.activePartIndex < 0 {
		return
	}
	switch b.activePartType {
	case uiPartText, uiPartReasoning:
		b.parts[b.activePartIndex].State = uiPartStateDone
	}
	b.activePartType = ""
	b.activePartIndex = -1
}

func (b *uiMessageBuilder) appendDataPart(partType string, partID string, payload json.RawMessage) {
	if len(payload) == 0 {
		return
	}
	b.closeActiveStreamPart()
	if partID != "" {
		for index := range b.parts {
			if b.parts[index].Type != partType || b.parts[index].ID != partID {
				continue
			}
			b.parts[index].Data = acp.CloneRawMessage(payload)
			return
		}
	}
	b.parts = append(b.parts, UIMessagePart{
		Type: partType,
		ID:   partID,
		Data: acp.CloneRawMessage(payload),
	})
}

func uiPermissionDataPartID(event acp.AgentEvent) string {
	if requestID := strings.TrimSpace(event.RequestID); requestID != "" {
		return requestID
	}
	if turnID := strings.TrimSpace(event.TurnID); turnID != "" {
		if toolCallID := strings.TrimSpace(event.ToolCallID); toolCallID != "" {
			return turnID + ":" + toolCallID
		}
	}
	if toolCallID := strings.TrimSpace(event.ToolCallID); toolCallID != "" {
		return toolCallID
	}
	return ""
}
