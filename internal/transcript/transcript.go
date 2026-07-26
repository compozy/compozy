package transcript

import (
	"encoding/json"

	"sort"
	"strings"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/store"
)

// CanonicalSchema is the stored envelope schema for transcript-aware session events.
const CanonicalSchema = "agh.session.event.v1"

// Assembler assembles persisted session events into the canonical transcript shape.
type Assembler interface {
	Assemble(events []store.SessionEvent) ([]Message, error)
}

// Role is the renderable chat role emitted by the canonical transcript API.
type Role string

const (
	RoleUser       Role = "user"
	RoleAssistant  Role = "assistant"
	RoleToolCall   Role = "tool_call"
	RoleToolResult Role = "tool_result"
	RoleSystem     Role = "system"
)

// ToolResult is the canonical renderable tool output shape for replay.
type ToolResult struct {
	Stdout          string          `json:"stdout,omitempty"`
	Stderr          string          `json:"stderr,omitempty"`
	FilePath        string          `json:"file_path,omitempty"`
	Content         string          `json:"content,omitempty"`
	StructuredPatch json.RawMessage `json:"structured_patch,omitempty"`
	Error           string          `json:"error,omitempty"`
	RawOutput       json.RawMessage `json:"raw_output,omitempty"`
}

// Message is the canonical replay message returned to transport callers.
type Message struct {
	ID               string          `json:"id"`
	Role             Role            `json:"role"`
	Content          string          `json:"content"`
	Thinking         string          `json:"thinking,omitempty"`
	ThinkingComplete bool            `json:"thinking_complete"`
	ToolName         string          `json:"tool_name,omitempty"`
	ToolInput        json.RawMessage `json:"tool_input,omitempty"`
	ToolResult       *ToolResult     `json:"tool_result,omitempty"`
	ToolError        bool            `json:"tool_error"`
	Timestamp        time.Time       `json:"timestamp"`
}

type event struct {
	ID         string
	TurnID     string
	Type       string
	Text       string
	StopReason string
	Error      string
	Failure    *store.SessionFailure
	Runtime    *acp.RuntimeActivity
	Marker     *Marker
	ToolCallID string
	ToolName   string
	ToolInput  json.RawMessage
	ToolResult *ToolResult
	ToolError  bool
	Timestamp  time.Time
}

type assistantBuffer struct {
	id        string
	turnID    string
	timestamp time.Time
	content   strings.Builder
	thinking  strings.Builder
}

type toolLifecycle struct {
	callIndex   int
	resultIndex int
}

// Assemble returns the canonical replay transcript for the provided persisted events.
func Assemble(events []store.SessionEvent) ([]Message, error) {
	if len(events) == 0 {
		return []Message{}, nil
	}

	sorted := sortedTranscriptEvents(events)

	messages := make([]Message, 0, len(sorted))
	var assistant assistantBuffer
	toolStates := make(map[string]*toolLifecycle)

	for _, sessionEvent := range sorted {
		processTranscriptEvent(&messages, &assistant, toolStates, parseEvent(sessionEvent))
	}

	flushAssistantBuffer(&messages, &assistant)
	return messages, nil
}

func sortedTranscriptEvents(events []store.SessionEvent) []store.SessionEvent {
	sorted := append([]store.SessionEvent(nil), events...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Sequence == sorted[j].Sequence {
			if sorted[i].Timestamp.Equal(sorted[j].Timestamp) {
				return sorted[i].ID < sorted[j].ID
			}
			return sorted[i].Timestamp.Before(sorted[j].Timestamp)
		}
		return sorted[i].Sequence < sorted[j].Sequence
	})
	return sorted
}

func processTranscriptEvent(
	messages *[]Message,
	assistant *assistantBuffer,
	toolStates map[string]*toolLifecycle,
	parsed event,
) {
	flushAssistantOnTurnChange(messages, assistant, parsed)

	switch parsed.Type {
	case acp.EventTypeUserMessage:
		appendInputTranscriptMessage(messages, assistant, parsed, RoleUser)
	case acp.EventTypeSyntheticReentry:
		appendInputTranscriptMessage(messages, assistant, parsed, RoleSystem)
	case acp.EventTypeAgentMessage:
		appendAssistantTranscriptContent(assistant, parsed, false)
	case acp.EventTypeThought:
		appendAssistantTranscriptContent(assistant, parsed, true)
	case acp.EventTypeToolCall:
		flushAssistantBuffer(messages, assistant)
		applyToolCall(messages, toolStates, parsed)
	case acp.EventTypeToolResult:
		flushAssistantBuffer(messages, assistant)
		applyToolResult(messages, toolStates, parsed)
	case events.TranscriptMarkerCreated, events.TranscriptMarkerRedacted:
		appendMarkerTranscriptMessage(messages, assistant, parsed)
	default:
		flushAssistantBuffer(messages, assistant)
	}
}

func flushAssistantOnTurnChange(messages *[]Message, assistant *assistantBuffer, parsed event) {
	if assistant.id == "" || assistant.turnID == "" || parsed.TurnID == "" {
		return
	}
	if assistant.turnID != parsed.TurnID {
		flushAssistantBuffer(messages, assistant)
	}
}

func appendInputTranscriptMessage(messages *[]Message, assistant *assistantBuffer, parsed event, role Role) {
	flushAssistantBuffer(messages, assistant)
	if strings.TrimSpace(parsed.Text) == "" {
		return
	}
	*messages = append(*messages, Message{
		ID:        parsed.ID,
		Role:      role,
		Content:   parsed.Text,
		Timestamp: parsed.Timestamp,
	})
}

func appendMarkerTranscriptMessage(messages *[]Message, assistant *assistantBuffer, parsed event) {
	markerText := transcriptMarkerText(parsed)
	if markerText == "" {
		flushAssistantBuffer(messages, assistant)
		return
	}
	flushAssistantBuffer(messages, assistant)
	*messages = append(*messages, Message{
		ID:        parsed.ID,
		Role:      RoleSystem,
		Content:   markerText,
		Timestamp: parsed.Timestamp,
	})
}

func appendAssistantTranscriptContent(assistant *assistantBuffer, parsed event, thinking bool) {
	if strings.TrimSpace(parsed.Text) == "" && assistant.id == "" {
		return
	}
	ensureAssistantBuffer(assistant, parsed)
	if thinking {
		assistant.thinking.WriteString(parsed.Text)
		return
	}
	assistant.content.WriteString(parsed.Text)
}

func ensureAssistantBuffer(assistant *assistantBuffer, parsed event) {
	if assistant.id != "" {
		return
	}
	assistant.id = parsed.ID
	assistant.turnID = parsed.TurnID
	assistant.timestamp = parsed.Timestamp
}

func flushAssistantBuffer(messages *[]Message, assistant *assistantBuffer) {
	if assistant.id == "" {
		return
	}
	content := assistant.content.String()
	thinking := assistant.thinking.String()
	if strings.TrimSpace(content) == "" && strings.TrimSpace(thinking) == "" {
		*assistant = assistantBuffer{}
		return
	}

	*messages = append(*messages, Message{
		ID:               assistant.id,
		Role:             RoleAssistant,
		Content:          content,
		Thinking:         thinking,
		ThinkingComplete: strings.TrimSpace(thinking) != "",
		Timestamp:        assistant.timestamp,
	})
	*assistant = assistantBuffer{}
}

func applyToolCall(messages *[]Message, toolStates map[string]*toolLifecycle, parsed event) {
	toolKey := toolLifecycleKey(parsed)
	if toolKey == "" {
		return
	}
	messageID := toolMessageID(parsed)

	lifecycle, ok := toolStates[toolKey]
	if !ok {
		lifecycle = &toolLifecycle{callIndex: -1, resultIndex: -1}
		toolStates[toolKey] = lifecycle
	}

	if lifecycle.callIndex >= 0 {
		msg := &(*messages)[lifecycle.callIndex]
		mergeToolCallMessage(msg, parsed)
		return
	}

	*messages = append(*messages, Message{
		ID:        messageID,
		Role:      RoleToolCall,
		Content:   "",
		ToolName:  parsed.ToolName,
		ToolInput: acp.CloneRawMessage(parsed.ToolInput),
		Timestamp: parsed.Timestamp,
	})
	lifecycle.callIndex = len(*messages) - 1
}

func applyToolResult(messages *[]Message, toolStates map[string]*toolLifecycle, parsed event) {
	toolKey := toolLifecycleKey(parsed)
	if toolKey == "" {
		return
	}
	messageID := toolMessageID(parsed)

	lifecycle, ok := toolStates[toolKey]
	if !ok {
		lifecycle = &toolLifecycle{callIndex: -1, resultIndex: -1}
		toolStates[toolKey] = lifecycle
	}

	if lifecycle.callIndex < 0 {
		*messages = append(*messages, Message{
			ID:        messageID,
			Role:      RoleToolCall,
			Content:   "",
			ToolName:  parsed.ToolName,
			ToolInput: acp.CloneRawMessage(parsed.ToolInput),
			Timestamp: parsed.Timestamp,
		})
		lifecycle.callIndex = len(*messages) - 1
	} else {
		mergeToolCallMessage(&(*messages)[lifecycle.callIndex], parsed)
	}

	result := cloneToolResult(parsed.ToolResult)
	if result == nil {
		result = &ToolResult{}
	}
	if lifecycle.resultIndex >= 0 {
		msg := &(*messages)[lifecycle.resultIndex]
		msg.ToolName = firstNonEmpty(msg.ToolName, parsed.ToolName)
		msg.ToolResult = result
		msg.ToolError = msg.ToolError || parsed.ToolError
		return
	}

	*messages = append(*messages, Message{
		ID:         messageID,
		Role:       RoleToolResult,
		Content:    "",
		ToolName:   parsed.ToolName,
		ToolResult: result,
		ToolError:  parsed.ToolError,
		Timestamp:  parsed.Timestamp,
	})
	lifecycle.resultIndex = len(*messages) - 1
}

func toolLifecycleKey(parsed event) string {
	toolID := strings.TrimSpace(parsed.ToolCallID)
	if toolID != "" {
		return toolID
	}

	fallbackID := strings.TrimSpace(parsed.ID)
	if fallbackID == "" {
		return ""
	}

	turnID := strings.TrimSpace(parsed.TurnID)
	if turnID == "" {
		return fallbackID
	}
	return turnID + ":" + fallbackID
}

func toolMessageID(parsed event) string {
	return toolLifecycleKey(parsed)
}

func mergeToolCallMessage(msg *Message, parsed event) {
	if msg == nil {
		return
	}
	msg.ToolName = firstNonEmpty(msg.ToolName, parsed.ToolName)
	if (len(msg.ToolInput) == 0 || rawMessageIsEmptyObject(msg.ToolInput)) && len(parsed.ToolInput) > 0 &&
		!rawMessageIsEmptyObject(parsed.ToolInput) {
		msg.ToolInput = acp.CloneRawMessage(parsed.ToolInput)
	}
}
