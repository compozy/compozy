package transcript

import "github.com/compozy/compozy/internal/acp"

// nativeTaskPayload is the stored form of an acp.NativeTaskObservation. The
// originating Agent/Task call is the event's tool_call_id.
type nativeTaskPayload struct {
	Source       string                  `json:"source,omitempty"`
	Kind         acp.NativeTaskKind      `json:"kind"`
	TaskID       string                  `json:"task_id"`
	RunID        string                  `json:"run_id,omitempty"`
	Status       string                  `json:"status,omitempty"`
	Summary      *string                 `json:"summary,omitempty"`
	Error        string                  `json:"error,omitempty"`
	Description  string                  `json:"description,omitempty"`
	LastToolName string                  `json:"last_tool_name,omitempty"`
	Usage        *nativeTaskUsagePayload `json:"usage,omitempty"`
	Background   *bool                   `json:"background,omitempty"`
	EventUUID    string                  `json:"event_uuid,omitempty"`
}

type nativeTaskUsagePayload struct {
	TotalTokens int64 `json:"total_tokens"`
	ToolUses    int64 `json:"tool_uses"`
	DurationMs  int64 `json:"duration_ms"`
}

func nativeTaskPayloadFromObservation(observation *acp.NativeTaskObservation) *nativeTaskPayload {
	if observation == nil {
		return nil
	}
	cloned := observation.Clone()
	payload := &nativeTaskPayload{
		Source: cloned.Source, Kind: cloned.Kind, TaskID: cloned.TaskID, RunID: cloned.RunID,
		Status: cloned.Status, Summary: cloned.Summary, Error: cloned.Error, Description: cloned.Description,
		LastToolName: cloned.LastToolName, Background: cloned.Background, EventUUID: cloned.EventUUID,
	}
	if cloned.Usage != nil {
		payload.Usage = &nativeTaskUsagePayload{
			TotalTokens: cloned.Usage.TotalTokens, ToolUses: cloned.Usage.ToolUses, DurationMs: cloned.Usage.DurationMs,
		}
	}
	return payload
}

func nativeTaskObservationFromPayload(payload *nativeTaskPayload, toolCallID string) *acp.NativeTaskObservation {
	if payload == nil {
		return nil
	}
	observation := &acp.NativeTaskObservation{
		Source: payload.Source, Kind: payload.Kind, TaskID: payload.TaskID, RunID: payload.RunID,
		ToolCallID: toolCallID, Status: payload.Status, Summary: payload.Summary, Error: payload.Error,
		Description: payload.Description, LastToolName: payload.LastToolName, Background: payload.Background,
		EventUUID: payload.EventUUID,
	}
	if payload.Usage != nil {
		observation.Usage = &acp.NativeTaskUsage{
			TotalTokens: payload.Usage.TotalTokens,
			ToolUses:    payload.Usage.ToolUses,
			DurationMs:  payload.Usage.DurationMs,
		}
	}
	return observation.Clone()
}

// redactNativeTaskObservation redacts identifiers structurally and provider
// prose for display, bounding the summary like other stored summaries.
func redactNativeTaskObservation(observation *acp.NativeTaskObservation) *acp.NativeTaskObservation {
	redacted := observation.Clone()
	if redacted == nil {
		return nil
	}
	redacted.Source = redactStructuralString(redacted.Source)
	redacted.Kind = acp.NativeTaskKind(redactStructuralString(string(redacted.Kind)))
	redacted.TaskID = redactStructuralString(redacted.TaskID)
	redacted.RunID = redactStructuralString(redacted.RunID)
	redacted.ToolCallID = redactStructuralString(redacted.ToolCallID)
	redacted.Status = redactStructuralString(redacted.Status)
	if redacted.Summary != nil {
		redacted.Summary = new(boundedCompactionSummary(redactDisplayString(*redacted.Summary)))
	}
	redacted.Error = boundedCompactionSummary(redactDisplayString(redacted.Error))
	redacted.Description = boundedCompactionSummary(redactDisplayString(redacted.Description))
	redacted.LastToolName = redactStructuralString(redacted.LastToolName)
	redacted.EventUUID = redactStructuralString(redacted.EventUUID)
	return redacted
}
