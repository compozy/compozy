package globaldb

import (
	"context"
	"fmt"

	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

type taskTransactionExecutor struct {
	taskSQLExecutor
	committedEvents []taskpkg.EventRecord
}

func (e *taskTransactionExecutor) collectTaskEvent(record taskpkg.EventRecord) {
	e.committedEvents = append(e.committedEvents, record)
}

type taskEventCollector interface {
	collectTaskEvent(record taskpkg.EventRecord)
}

func collectTaskEvent(exec taskSQLExecutor, record taskpkg.EventRecord) {
	collector, ok := exec.(taskEventCollector)
	if !ok {
		return
	}
	collector.collectTaskEvent(record)
}

func (g *TaskRepo) withTaskImmediateTransaction(
	ctx context.Context,
	action string,
	run func(exec taskSQLExecutor) error,
) error {
	var committedEvents []taskpkg.EventRecord
	err := store.ExecuteWrite(ctx, g.db, func(_ context.Context, tx *store.WriteTx) error {
		transaction := &taskTransactionExecutor{taskSQLExecutor: tx}
		if err := run(transaction); err != nil {
			return err
		}
		committedEvents = append(committedEvents[:0], transaction.committedEvents...)
		return nil
	})
	if err != nil {
		return fmt.Errorf("store: %s transaction: %w", action, err)
	}
	g.notifyCommittedTaskEvents(context.WithoutCancel(ctx), committedEvents)
	return nil
}
