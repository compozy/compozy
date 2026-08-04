package presets

import (
	"context"
	"log/slog"

	taskpkg "github.com/compozy/compozy/internal/task"
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
	event := EventFromTaskRecord(ctx, o.tasks, record, o.logger)
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
