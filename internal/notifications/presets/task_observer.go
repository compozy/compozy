package presets

import (
	"context"
	"log/slog"
	"strings"

	taskpkg "github.com/compozy/agh/internal/task"
)

// TaskReader loads task context needed to enrich task-event notifications.
type TaskReader interface {
	GetTask(ctx context.Context, id string) (taskpkg.Task, error)
}

type taskEventObserver struct {
	service *Service
	tasks   TaskReader
	logger  *slog.Logger
}

var _ taskpkg.EventObserver = (*taskEventObserver)(nil)

func NewTaskEventObserver(service *Service, tasks TaskReader, logger *slog.Logger) taskpkg.EventObserver {
	if service == nil {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &taskEventObserver{service: service, tasks: tasks, logger: logger}
}

func (o *taskEventObserver) OnTaskEvent(ctx context.Context, record taskpkg.EventRecord) {
	if o == nil || o.service == nil {
		return
	}
	event := Event{
		ID:        strings.TrimSpace(record.Event.ID),
		Type:      strings.TrimSpace(record.Event.EventType),
		TaskID:    strings.TrimSpace(record.Event.TaskID),
		RunID:     strings.TrimSpace(record.Event.RunID),
		AgentName: strings.TrimSpace(record.Event.Actor.Ref),
		Sequence:  record.Sequence,
		Payload:   record.Event.Payload,
		Timestamp: record.Event.Timestamp,
	}
	if o.tasks != nil && event.TaskID != "" {
		taskRecord, err := o.tasks.GetTask(ctx, event.TaskID)
		if err == nil {
			event.WorkspaceID = strings.TrimSpace(taskRecord.WorkspaceID)
			event.Summary = strings.TrimSpace(taskRecord.Title)
		} else if o.logger != nil {
			o.logger.Debug(
				"notifications: task preset observer could not enrich task event",
				"task_id",
				event.TaskID,
				"event_id",
				event.ID,
				"error",
				err,
			)
		}
	}
	if _, err := o.service.Dispatch(context.WithoutCancel(ctx), event); err != nil && o.logger != nil {
		o.logger.Warn(
			"notifications: preset task event dispatch failed",
			"event_id",
			event.ID,
			"event_type",
			event.Type,
			"error",
			err,
		)
	}
}
