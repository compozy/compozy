package session

import (
	"encoding/json"

	"strings"

	"time"

	"github.com/compozy/agh/internal/acp"

	"github.com/compozy/agh/internal/store"
)

func deadlinePointer(value time.Time, ok bool) *time.Time {
	if !ok || value.IsZero() {
		return nil
	}
	deadline := value.UTC()
	return &deadline
}

func jsonMarshalDeadlineWarning(activity acp.RuntimeActivity) (json.RawMessage, error) {
	payload := map[string]any{
		"elapsed_ms": activity.ElapsedMS,
	}
	if activity.DeadlineAt != nil && !activity.DeadlineAt.IsZero() {
		payload["deadline_at"] = activity.DeadlineAt.UTC().Format(time.RFC3339Nano)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func activityFromEvent(
	event acp.AgentEvent,
) (kind string, detail string, currentTool string, toolCallID string, clearTool bool) {
	kind = strings.TrimSpace(event.Type)
	switch kind {
	case "":
		return "", "", "", "", false
	case acp.EventTypeRuntimeProgress, acp.EventTypeRuntimeWarning:
		return "", "", "", "", false
	case acp.EventTypeToolCall:
		detail = firstNonEmpty(event.Title, event.Text, event.ToolCallID)
		currentTool = firstNonEmpty(event.Title, event.Resource)
		toolCallID = event.ToolCallID
	case acp.EventTypeToolResult:
		detail = firstNonEmpty(event.Title, event.Text, event.ToolCallID)
		clearTool = true
	default:
		detail = firstNonEmpty(event.Title, event.Text, event.Error, event.Action, event.Resource)
	}
	return kind, detail, currentTool, toolCallID, clearTool
}

func storeActivityFromRuntime(activity acp.RuntimeActivity) *store.SessionActivityMeta {
	meta := &store.SessionActivityMeta{
		TurnID:             strings.TrimSpace(activity.TurnID),
		TurnSource:         strings.TrimSpace(activity.TurnSource),
		TurnStartedAt:      cloneTimePointer(activity.TurnStartedAt),
		LastActivityAt:     cloneTimePointer(activity.LastActivityAt),
		LastActivityKind:   strings.TrimSpace(activity.LastActivityKind),
		LastActivityDetail: strings.TrimSpace(activity.LastActivityDetail),
		CurrentTool:        strings.TrimSpace(activity.CurrentTool),
		ToolCallID:         strings.TrimSpace(activity.ToolCallID),
		LastProgressAt:     cloneTimePointer(activity.LastProgressAt),
		IterationCurrent:   activity.IterationCurrent,
		IterationMax:       activity.IterationMax,
		IdleSeconds:        activity.IdleSeconds,
	}
	if meta.TurnID == "" && meta.LastActivityAt == nil && meta.LastProgressAt == nil {
		return nil
	}
	return meta
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copyValue := value.UTC()
	return &copyValue
}
