package core

import (
	"strings"

	"github.com/compozy/agh/internal/acp"
)

func (e *PromptStreamEncoder) emitAgentMessage(writer FlushWriter, event acp.AgentEvent) error {
	if err := e.ensureTextStarted(writer); err != nil {
		return err
	}
	return WriteSSE(writer, SSEMessage{
		Data: promptDeltaPayload{Type: "text-delta", ID: e.textBlockID, Delta: event.Text},
	})
}

func (e *PromptStreamEncoder) emitThought(writer FlushWriter, event acp.AgentEvent) error {
	if err := e.ensureReasoningStarted(writer); err != nil {
		return err
	}
	return WriteSSE(writer, SSEMessage{
		Data: promptDeltaPayload{Type: "reasoning-delta", ID: e.reasoningBlockID, Delta: event.Text},
	})
}

func (e *PromptStreamEncoder) emitToolCall(writer FlushWriter, event acp.AgentEvent) error {
	if err := e.closeOpenBlocks(writer); err != nil {
		return err
	}
	toolCallID := e.toolCallID(event)
	if err := e.ensureToolCallStarted(writer, toolCallID, event); err != nil {
		return err
	}
	if err := e.ensureToolInputAvailable(writer, toolCallID, event, false); err != nil {
		return err
	}
	return WriteSSE(writer, SSEMessage{
		Data: promptDataEventEnvelope{Type: "data-agh-event", Data: promptAgentEventPayloadFromEvent(event)},
	})
}

func (e *PromptStreamEncoder) emitToolResult(writer FlushWriter, event acp.AgentEvent) error {
	if err := e.closeOpenBlocks(writer); err != nil {
		return err
	}
	toolCallID := e.toolCallID(event)
	if err := e.ensureToolCallStarted(writer, toolCallID, event); err != nil {
		return err
	}
	if err := e.ensureToolInputAvailable(writer, toolCallID, event, true); err != nil {
		return err
	}
	return WriteSSE(writer, SSEMessage{
		Data: promptToolOutputAvailablePayload{
			Type:       "tool-output-available",
			ToolCallID: toolCallID,
			Output:     promptAgentEventPayloadFromEvent(event),
		},
	})
}

func (e *PromptStreamEncoder) emitPermission(writer FlushWriter, event acp.AgentEvent) error {
	if err := e.closeOpenBlocks(writer); err != nil {
		return err
	}
	return WriteSSE(writer, SSEMessage{
		Data: promptDataEventEnvelope{
			Type: "data-agh-permission",
			ID:   promptPermissionDataPartID(event),
			Data: promptAgentEventPayloadFromEvent(event),
		},
	})
}

func (e *PromptStreamEncoder) emitError(writer FlushWriter, event acp.AgentEvent) error {
	if err := e.closeOpenBlocks(writer); err != nil {
		return err
	}
	if err := WriteSSE(writer, SSEMessage{
		Data: promptErrorPayload{Type: promptStreamErrorKey, ErrorText: e.errorText(event)},
	}); err != nil {
		return err
	}
	return e.finish(writer, event)
}

func (e *PromptStreamEncoder) emitGenericEvent(writer FlushWriter, event acp.AgentEvent) error {
	if err := e.closeOpenBlocks(writer); err != nil {
		return err
	}
	return WriteSSE(writer, SSEMessage{
		Data: promptDataEventEnvelope{Type: "data-agh-event", Data: promptAgentEventPayloadFromEvent(event)},
	})
}

func (e *PromptStreamEncoder) toolCallID(event acp.AgentEvent) string {
	toolCallID := strings.TrimSpace(event.ToolCallID)
	if toolCallID == "" {
		return e.messageID + "-tool"
	}
	return toolCallID
}

func (e *PromptStreamEncoder) ensureToolCallStarted(
	writer FlushWriter,
	toolCallID string,
	event acp.AgentEvent,
) error {
	if _, ok := e.toolStarted[toolCallID]; ok {
		if toolName := e.toolName(event); toolName != "" {
			e.toolNames[toolCallID] = toolName
		}
		return nil
	}

	e.toolStarted[toolCallID] = struct{}{}
	if toolName := e.toolName(event); toolName != "" {
		e.toolNames[toolCallID] = toolName
	}

	return WriteSSE(writer, SSEMessage{
		Data: promptToolInputStartPayload{
			Type:       "tool-input-start",
			ToolCallID: toolCallID,
			ToolName:   e.toolNameByID(toolCallID),
		},
	})
}

func (e *PromptStreamEncoder) ensureToolInputAvailable(
	writer FlushWriter,
	toolCallID string,
	event acp.AgentEvent,
	force bool,
) error {
	if _, ok := e.toolInputsReady[toolCallID]; ok {
		return nil
	}

	input, ok := promptNormalizedToolInput(event)
	if !ok || !promptToolInputReady(input) {
		if !force {
			return nil
		}
		if _, ok := e.toolInputPending[toolCallID]; ok {
			return nil
		}
		e.toolInputPending[toolCallID] = struct{}{}
		input = map[string]any{}
	} else {
		delete(e.toolInputPending, toolCallID)
		e.toolInputsReady[toolCallID] = struct{}{}
	}

	return WriteSSE(writer, SSEMessage{
		Data: promptToolInputAvailablePayload{
			Type:       "tool-input-available",
			ToolCallID: toolCallID,
			ToolName:   e.toolNameByID(toolCallID),
			Input:      input,
		},
	})
}

func (e *PromptStreamEncoder) errorText(event acp.AgentEvent) string {
	errorText := strings.TrimSpace(event.Error)
	if errorText == "" {
		errorText = strings.TrimSpace(event.Text)
	}
	return errorText
}
