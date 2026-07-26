package transcript

import (
	"encoding/json"

	"strings"

	"github.com/compozy/agh/internal/acp"
)

func (b *uiMessageBuilder) applyToolCall(decoded *decodedStoredEvent) {
	b.closeActiveStreamPart()
	part, _ := b.ensureToolPart(decoded)
	if input, ready := toolInputFromDecoded(decoded); ready {
		part.State = uiToolStateAvailable
		part.Input = input
		b.rememberToolInput(decoded, input)
		return
	}
	if part.State == "" {
		part.State = uiToolStateStreaming
	}
}

func (b *uiMessageBuilder) applyToolResult(decoded *decodedStoredEvent) {
	b.closeActiveStreamPart()
	part, existed := b.ensureToolPart(decoded)
	input := acp.CloneRawMessage(part.Input)
	if len(input) == 0 {
		if next, ok := b.rememberedToolInput(decoded); ok {
			input = next
		} else if next, ready := toolInputFromDecoded(decoded); ready {
			input = next
			b.rememberToolInput(decoded, next)
		} else {
			input = acp.CloneRawMessage(emptyJSONObject)
		}
	}

	if decoded.parsed.ToolError {
		part.State = uiToolStateError
		part.ErrorText = firstNonEmpty(
			toolResultErrorText(decoded.parsed.ToolResult),
			decoded.agent.Error,
			decoded.parsed.Text,
		)
	} else {
		part.State = uiToolStateOutput
		part.ErrorText = ""
	}
	part.Input = input
	part.Output = decoded.toolResultDataPayload()

	if !existed {
		part.RawInput = nil
	}
}

func (b *uiMessageBuilder) rememberToolInput(decoded *decodedStoredEvent, input json.RawMessage) {
	if b == nil || len(input) == 0 || rawMessageIsEmptyObject(input) {
		return
	}
	key := toolLifecycleKey(decoded.parsed)
	if key == "" {
		return
	}
	if b.toolLifecycles == nil {
		b.toolLifecycles = make(map[string]*uiToolLifecycle)
	}
	lifecycle := b.toolLifecycles[key]
	if lifecycle == nil {
		lifecycle = &uiToolLifecycle{}
		b.toolLifecycles[key] = lifecycle
	}
	lifecycle.input = acp.CloneRawMessage(input)
}

func (b *uiMessageBuilder) rememberedToolInput(decoded *decodedStoredEvent) (json.RawMessage, bool) {
	if b == nil || len(b.toolLifecycles) == 0 {
		return nil, false
	}
	key := toolLifecycleKey(decoded.parsed)
	if key == "" {
		return nil, false
	}
	lifecycle := b.toolLifecycles[key]
	if lifecycle == nil || len(lifecycle.input) == 0 {
		return nil, false
	}
	return acp.CloneRawMessage(lifecycle.input), true
}

func toolResultErrorText(result *ToolResult) string {
	if result == nil {
		return ""
	}
	return firstNonEmpty(result.Error, result.Stderr, result.Content, result.Stdout)
}

func (b *uiMessageBuilder) ensureToolPart(decoded *decodedStoredEvent) (*UIMessagePart, bool) {
	key := toolLifecycleKey(decoded.parsed)
	if key == "" {
		key = fallbackMessageID(decoded.stored.ID, assistantMessageID(decoded), "tool")
	}
	if index, ok := b.toolIndices[key]; ok {
		part := &b.parts[index]
		if part.Type == uiPartDynamicTool && strings.TrimSpace(decoded.parsed.ToolName) != "" {
			part.Type = toolPartType(decoded.parsed.ToolName)
			part.ToolName = ""
		}
		if part.Title == "" {
			part.Title = strings.TrimSpace(decoded.agent.Title)
		}
		return part, true
	}

	part := UIMessagePart{
		Type:       toolPartKind(decoded.parsed.ToolName),
		ToolCallID: key,
		Title:      strings.TrimSpace(decoded.agent.Title),
	}
	if part.Type == uiPartDynamicTool {
		part.ToolName = strings.TrimSpace(decoded.parsed.ToolName)
	}
	b.parts = append(b.parts, part)
	index := len(b.parts) - 1
	b.toolIndices[key] = index
	return &b.parts[index], false
}

func toolPartKind(toolName string) string {
	if strings.TrimSpace(toolName) == "" {
		return uiPartDynamicTool
	}
	return toolPartType(toolName)
}

func toolPartType(toolName string) string {
	return "tool-" + strings.TrimSpace(toolName)
}
