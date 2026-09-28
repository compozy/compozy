package daemon

import (
	"context"
	"log/slog"

	taskpkg "github.com/compozy/compozy/internal/task"
)

type taskEventObserverFanout struct {
	observers []taskpkg.EventObserver
	logger    *slog.Logger
}

func newTaskEventObserverFanout(
	logger *slog.Logger,
	observers ...taskpkg.EventObserver,
) taskpkg.EventObserver {
	filtered := make([]taskpkg.EventObserver, 0, len(observers))
	for _, observer := range observers {
		if observer != nil {
			filtered = append(filtered, observer)
		}
	}
	switch len(filtered) {
	case 0:
		return nil
	case 1:
		return filtered[0]
	default:
		return &taskEventObserverFanout{observers: filtered, logger: logger}
	}
}

func (f *taskEventObserverFanout) OnTaskEvent(ctx context.Context, record taskpkg.EventRecord) {
	if f == nil {
		return
	}
	for _, observer := range f.observers {
		if observer == nil {
			continue
		}
		func(target taskpkg.EventObserver) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger := f.logger
					if logger == nil {
						logger = slog.Default()
					}
					logger.Error(
						"daemon: task event observer panicked during fanout",
						"panic", recovered,
						"event_id", record.Event.ID,
						"task_id", record.Event.TaskID,
						"run_id", record.Event.RunID,
						"event_type", record.Event.EventType,
					)
				}
			}()
			target.OnTaskEvent(ctx, record)
		}(observer)
	}
}
