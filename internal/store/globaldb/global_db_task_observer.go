package globaldb

import (
	"context"
	"log/slog"

	taskpkg "github.com/compozy/agh/internal/task"
)

var _ taskpkg.EventCommitObserverStore = (*TaskRepo)(nil)

// SetTaskEventCommitObserver installs the single task service that owns live
// fanout for this database.
func (g *TaskRepo) SetTaskEventCommitObserver(observer taskpkg.EventObserver) {
	g.taskEventObserverMu.Lock()
	defer g.taskEventObserverMu.Unlock()
	g.taskEventObserver = observer
}

func (g *TaskRepo) notifyCommittedTaskEvents(ctx context.Context, records []taskpkg.EventRecord) {
	if len(records) == 0 {
		return
	}
	g.taskEventObserverMu.RLock()
	observer := g.taskEventObserver
	g.taskEventObserverMu.RUnlock()
	if observer == nil {
		return
	}
	for _, record := range records {
		notifyCommittedTaskEvent(ctx, observer, record)
	}
}

func notifyCommittedTaskEvent(ctx context.Context, observer taskpkg.EventObserver, record taskpkg.EventRecord) {
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.Error(
				"store: task event commit observer panicked",
				"panic", recovered,
				"event_id", record.Event.ID,
				"task_id", record.Event.TaskID,
				"event_type", record.Event.EventType,
			)
		}
	}()
	observer.OnTaskEvent(ctx, record)
}
