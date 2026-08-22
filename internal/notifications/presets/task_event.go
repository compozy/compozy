package presets

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/notifications"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// EventFromTaskRecord builds one dispatchable preset event from a durable task event record.
func EventFromTaskRecord(
	ctx context.Context,
	tasks TaskReader,
	record taskpkg.EventRecord,
	logger *slog.Logger,
) Event {
	event := Event{
		ID:        record.Event.ID,
		ProfileID: store.DefaultProfileID,
		Type:      record.Event.EventType,
		Scope:     globalTaskEventScope(),
		AgentName: strings.TrimSpace(record.Event.Actor.Ref),
		TaskID:    record.Event.TaskID,
		RunID:     record.Event.RunID,
		Outcome:   eventspkg.OutcomeFor(record.Event.EventType),
		Sequence:  record.Sequence,
		Payload:   append(json.RawMessage(nil), record.Event.Payload...),
		Timestamp: record.Event.Timestamp,
	}
	if tasks == nil || event.TaskID == "" {
		return event
	}
	taskRecord, err := tasks.GetTask(ctx, event.TaskID)
	if err != nil {
		if logger != nil {
			logger.Debug(
				"notifications: preset task event ownership unavailable",
				"task_id", event.TaskID,
				"event_id", event.ID,
				"error", err,
			)
		}
		return event
	}
	event.Scope = taskEventScope(taskRecord)
	if profileID := strings.TrimSpace(taskRecord.ProfileID); profileID != "" {
		event.ProfileID = profileID
	}
	event.Summary = strings.TrimSpace(taskRecord.Title)
	return event
}

func taskEventScope(record taskpkg.Task) notifications.ScopeRef {
	if record.Scope.Normalize() == taskpkg.ScopeWorkspace && record.WorkspaceID != "" {
		return notifications.ScopeRef{
			Kind:        notifications.ScopeKindWorkspace,
			WorkspaceID: record.WorkspaceID,
		}
	}
	return globalTaskEventScope()
}

// Unreadable task ownership degrades to global scope instead of claiming workspace membership.
func globalTaskEventScope() notifications.ScopeRef {
	return notifications.ScopeRef{Kind: notifications.ScopeKindGlobal}
}
