package core

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

func (e *PromptStreamEncoder) emitAgentMessage(writer FlushWriter, event acp.AgentEvent) error {
	if err := e.ensureTextStarted(writer, event.ParentToolCallID()); err != nil {
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
	if err := WriteSSE(writer, SSEMessage{
		Data: promptDataEventEnvelope{Type: "data-compozy-event", Data: promptAgentEventPayloadFromEvent(event)},
	}); err != nil {
		return err
	}
	return e.emitSubagentPart(writer, event)
}

// emitSubagentPart writes the card the transcript projection derives from this
// event (a provider-native Agent/Task call, a successful delegate result).
func (e *PromptStreamEncoder) emitSubagentPart(writer FlushWriter, event acp.AgentEvent) error {
	payload, ok := e.subagentCard(event)
	if !ok {
		return nil
	}
	payload.Title = promptRedactString(payload.Title)
	return WriteSSE(writer, SSEMessage{
		Data: promptSubagentDataPayload{Type: "data-compozy-subagent", ID: payload.SubagentID, Data: payload},
	})
}

// subagentCard prefers the card the session derived from the persisted event
// (the decoded event no longer holds the tool result) and derives one from the
// event itself only when it arrived without that annotation.
func (e *PromptStreamEncoder) subagentCard(event acp.AgentEvent) (transcript.UISubagentPayload, bool) {
	if card := event.SubagentCard(); len(card) > 0 {
		var payload transcript.UISubagentPayload
		if json.Unmarshal(card, &payload) == nil && payload.SubagentID != "" {
			return payload, true
		}
	}
	content, err := transcript.MarshalAgentEvent(event)
	if err != nil {
		return transcript.UISubagentPayload{}, false
	}
	return transcript.SubagentPartForStoredEvent(store.SessionEvent{
		SessionID: e.sessionID,
		TurnID:    event.TurnID,
		Type:      event.Type,
		Content:   content,
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
	if err := WriteSSE(writer, SSEMessage{
		Data: promptToolOutputAvailablePayload{
			Type:       "tool-output-available",
			ToolCallID: toolCallID,
			Output:     promptAgentEventPayloadFromEvent(event),
		},
	}); err != nil {
		return err
	}
	e.toolCompleted[toolCallID] = struct{}{}
	return e.emitSubagentPart(writer, event)
}

func (e *PromptStreamEncoder) emitUnresolvedToolResults(writer FlushWriter, event acp.AgentEvent) error {
	toolCallIDs := make([]string, 0, len(e.toolStarted))
	for toolCallID := range e.toolStarted {
		if _, completed := e.toolCompleted[toolCallID]; !completed {
			toolCallIDs = append(toolCallIDs, toolCallID)
		}
	}
	slices.Sort(toolCallIDs)
	for _, toolCallID := range toolCallIDs {
		errorText := "Tool call ended before returning a result."
		raw, err := json.Marshal(map[string]string{"error": errorText})
		if err != nil {
			return err
		}
		synthetic := acp.AgentEvent{
			Type:       acp.EventTypeToolResult,
			TurnID:     event.TurnID,
			Timestamp:  event.Timestamp,
			ToolCallID: toolCallID,
			Title:      e.toolNameByID(toolCallID),
			Error:      errorText,
			Raw:        raw,
		}
		if err := e.ensureToolInputAvailable(writer, toolCallID, synthetic, true); err != nil {
			return err
		}
		if err := e.emitToolResult(writer, synthetic); err != nil {
			return err
		}
	}
	return nil
}

func (e *PromptStreamEncoder) emitPermission(writer FlushWriter, event acp.AgentEvent) error {
	if err := e.closeOpenBlocks(writer); err != nil {
		return err
	}
	return WriteSSE(writer, SSEMessage{
		Data: promptDataEventEnvelope{
			Type: "data-compozy-permission",
			ID:   promptPermissionDataPartID(event),
			Data: promptAgentEventPayloadFromEvent(event),
		},
	})
}

func (e *PromptStreamEncoder) emitError(writer FlushWriter, event acp.AgentEvent) error {
	if event.ProviderError != nil {
		if err := e.emitGenericEvent(writer, event); err != nil {
			return err
		}
	}
	if err := e.closeOpenBlocks(writer); err != nil {
		return err
	}
	if err := WriteSSE(writer, SSEMessage{
		Data: promptErrorPayload{
			Type:      promptStreamErrorKey,
			ErrorText: e.errorText(event),
			Failure:   SessionFailurePayloadFromStore(event.Failure),
		},
	}); err != nil {
		return err
	}
	return e.finish(writer, event)
}

func (e *PromptStreamEncoder) emitGenericEvent(writer FlushWriter, event acp.AgentEvent) error {
	if err := e.closeOpenBlocks(writer); err != nil {
		return err
	}
	return WriteSSE(writer, SSEMessage{Data: promptDataEventEnvelope{
		Type: "data-compozy-event", Data: promptAgentEventPayloadFromEvent(event),
	}})
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
			Type:             "tool-input-start",
			ToolCallID:       toolCallID,
			ToolName:         e.toolNameByID(toolCallID),
			ProviderMetadata: promptAttribution(event.ParentToolCallID()),
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
