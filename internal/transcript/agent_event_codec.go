package transcript

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/store"
)

// MarshalAgentEvent converts a runtime ACP event into the canonical stored payload.
func MarshalAgentEvent(event acp.AgentEvent) (string, error) {
	return marshalAgentEvent(event, "")
}

// MarshalPromptInputEvent preserves the exact authored text alongside the
// effective input sent through prompt hooks.
func MarshalPromptInputEvent(event acp.AgentEvent, authoredText string) (string, error) {
	return marshalAgentEvent(event, authoredText)
}

func marshalAgentEvent(event acp.AgentEvent, authoredText string) (string, error) {
	typedToolPayload := event.HasToolPayload()
	payload := canonicalEventPayload{
		Schema:            CanonicalSchema,
		Type:              event.Type,
		SessionID:         event.SessionID,
		TurnID:            event.TurnID,
		ClientMessageID:   event.ClientMessageIDValue(),
		RequestID:         event.RequestID,
		EventCorrelation:  event.Normalize(),
		Timestamp:         event.Timestamp,
		Text:              event.Text,
		AuthoredText:      authoredText,
		Title:             event.Title,
		ToolName:          event.ToolName(),
		ToolKind:          strings.TrimSpace(event.ToolKind()),
		ToolCallID:        event.ToolCallID,
		ToolInput:         event.ToolInput(),
		ToolError:         event.ToolError(),
		StopReason:        event.StopReason,
		PromptStopReason:  event.PromptStopReason,
		Action:            event.Action,
		Resource:          event.Resource,
		Decision:          event.Decision,
		Error:             event.Error,
		Failure:           store.CloneSessionFailure(event.Failure),
		Synthetic:         clonePromptSyntheticMeta(event.Synthetic),
		Goal:              acp.CloneGoalPromptMeta(event.Goal),
		AvailableCommands: event.AvailableCommands.Values(),
		Usage:             event.Usage,
		Runtime:           cloneRuntimeActivity(event.Runtime),
	}

	if len(event.Raw) > 0 {
		var rawPayload map[string]any
		if err := json.Unmarshal(event.Raw, &rawPayload); err == nil {
			if event.Type == acp.EventTypePermission ||
				event.Type == acp.EventTypeClarify ||
				event.Type == events.SessionCompactionFired ||
				event.Type == events.TranscriptMarkerCreated ||
				event.Type == events.TranscriptMarkerRedacted {
				payload.Raw = acp.CloneRawMessage(event.Raw)
			}
			payload.ToolName = firstNonEmpty(payload.ToolName, legacyToolName(rawPayload))
			payload.ToolKind = firstNonEmpty(payload.ToolKind, nestedString(rawPayload, "kind"))
			payload.ToolInput = acp.CloneRawMessage(firstNonEmptyRaw(
				payload.ToolInput,
				rawMessageFromValue(rawPayload["rawInput"]),
			))
			if event.Type == acp.EventTypeToolResult {
				rawToolError := strings.EqualFold(nestedString(rawPayload, "status"), "failed")
				effectiveToolError := rawToolError
				if typedToolPayload {
					effectiveToolError = payload.ToolError
				}
				payload.ToolResult = buildToolResult(
					payload.ToolName,
					effectiveToolError,
					extractLegacyContentText(rawPayload["content"]),
					rawPayload["rawOutput"],
				)
				payload.ToolError = effectiveToolError
			}
		}
	}
	if detail := strings.TrimSpace(event.ToolErrorDetail()); detail != "" {
		if payload.ToolResult == nil {
			payload.ToolResult = &ToolResult{}
		}
		payload.ToolResult.Error = detail
	}

	payload.ToolName = firstNonEmpty(payload.ToolName, event.Title)
	redactCanonicalPayload(&payload)

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("transcript: marshal agent event: %w", err)
	}
	return string(data), nil
}

// UnmarshalAgentEvent converts a canonical stored payload back into an ACP event.
func UnmarshalAgentEvent(payload string) (acp.AgentEvent, error) {
	trimmed := strings.TrimSpace(payload)
	if trimmed == "" {
		return acp.AgentEvent{}, nil
	}

	var decoded canonicalEventPayload
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return acp.AgentEvent{}, fmt.Errorf("transcript: unmarshal agent event: %w", err)
	}

	event := acp.AgentEvent{
		Type:             strings.TrimSpace(decoded.Type),
		SessionID:        strings.TrimSpace(decoded.SessionID),
		TurnID:           strings.TrimSpace(decoded.TurnID),
		RequestID:        strings.TrimSpace(decoded.RequestID),
		EventCorrelation: decoded.Normalize(),
		Timestamp:        decoded.Timestamp,
		Text:             decoded.Text,
		Title:            firstNonEmpty(decoded.Title, decoded.ToolName),
		ToolCallID:       strings.TrimSpace(decoded.ToolCallID),
		StopReason:       strings.TrimSpace(decoded.StopReason),
		PromptStopReason: acp.PromptStopReason(strings.TrimSpace(string(decoded.PromptStopReason))),
		Action:           strings.TrimSpace(decoded.Action),
		Resource:         strings.TrimSpace(decoded.Resource),
		Decision:         strings.TrimSpace(decoded.Decision),
		Error:            strings.TrimSpace(decoded.Error),
		Failure:          store.CloneSessionFailure(decoded.Failure),
		Synthetic:        clonePromptSyntheticMeta(decoded.Synthetic),
		Goal:             acp.CloneGoalPromptMeta(decoded.Goal),
		Usage:            decoded.Usage,
		Runtime:          cloneRuntimeActivity(decoded.Runtime),
		Raw:              acp.CloneRawMessage(decoded.Raw),
	}
	if clientMessageID := strings.TrimSpace(decoded.ClientMessageID); clientMessageID != "" {
		event = event.WithClientMessageID(clientMessageID)
	}
	toolErrorDetail := ""
	if event.Type == acp.EventTypeToolResult && decoded.ToolResult != nil {
		toolErrorDetail = strings.TrimSpace(decoded.ToolResult.Error)
	}
	event = event.WithToolDetail(
		strings.TrimSpace(decoded.ToolName),
		decoded.ToolInput,
		decoded.ToolError,
		toolErrorDetail,
	).WithToolKind(decoded.ToolKind)
	if decoded.AvailableCommands != nil || event.Type == acp.EventTypeAvailableCommands {
		event.AvailableCommands = acp.NewAvailableCommandSet(decoded.AvailableCommands)
	}
	return RedactAgentEvent(event), nil
}

func cloneRuntimeActivity(activity *acp.RuntimeActivity) *acp.RuntimeActivity {
	if activity == nil {
		return nil
	}
	cloned := *activity
	if activity.TurnStartedAt != nil {
		turnStartedAt := activity.TurnStartedAt.UTC()
		cloned.TurnStartedAt = &turnStartedAt
	}
	if activity.DeadlineAt != nil {
		deadlineAt := activity.DeadlineAt.UTC()
		cloned.DeadlineAt = &deadlineAt
	}
	if activity.LastActivityAt != nil {
		lastActivityAt := activity.LastActivityAt.UTC()
		cloned.LastActivityAt = &lastActivityAt
	}
	if activity.LastProgressAt != nil {
		lastProgressAt := activity.LastProgressAt.UTC()
		cloned.LastProgressAt = &lastProgressAt
	}
	return &cloned
}

func clonePromptSyntheticMeta(meta *acp.PromptSyntheticMeta) *acp.PromptSyntheticMeta {
	if meta == nil {
		return nil
	}

	cloned := meta.Normalize()
	if cloned.IsZero() {
		return nil
	}
	return &cloned
}
