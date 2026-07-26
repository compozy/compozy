package globaldb

import (
	"context"

	taskpkg "github.com/compozy/agh/internal/task"
)

type taskExecutionTxStore struct {
	*deleteTaskTxStore
}

var _ taskpkg.ExecutionMutationStore = (*taskExecutionTxStore)(nil)
var _ taskpkg.ExecutionTransactionStore = (*TaskRepo)(nil)

// WithTaskExecutionTransaction runs one Task execution command under the
// repository's immediate writer transaction.
func (g *TaskRepo) WithTaskExecutionTransaction(
	ctx context.Context,
	fn func(taskpkg.ExecutionMutationStore) error,
) error {
	if err := g.checkReady(ctx, "execute task command"); err != nil {
		return err
	}
	return g.withTaskImmediateTransaction(ctx, "execute task command", func(exec taskSQLExecutor) error {
		return fn(&taskExecutionTxStore{deleteTaskTxStore: &deleteTaskTxStore{tasks: g, exec: exec}})
	})
}

func (s *taskExecutionTxStore) GetExecutionProfile(
	ctx context.Context,
	taskID string,
) (taskpkg.ExecutionProfile, error) {
	profile, found, err := loadExecutionProfile(ctx, s.exec, taskID)
	if err != nil {
		return taskpkg.ExecutionProfile{}, err
	}
	if !found {
		return taskpkg.ExecutionProfile{}, taskpkg.ErrExecutionProfileNotFound
	}
	return profile, nil
}

func (s *taskExecutionTxStore) UpsertExecutionProfile(
	ctx context.Context,
	profile *taskpkg.ExecutionProfile,
) (taskpkg.ExecutionProfile, error) {
	return s.tasks.upsertExecutionProfileWithExecutor(ctx, s.exec, profile)
}

func (s *taskExecutionTxStore) DeleteExecutionProfile(ctx context.Context, taskID string) error {
	return deleteExecutionProfileWithExecutor(ctx, s.exec, taskID)
}

func (s *taskExecutionTxStore) GetTaskRunByIdempotencyKey(
	ctx context.Context,
	key string,
	origin taskpkg.Origin,
) (taskpkg.Run, error) {
	trimmedKey, normalizedOrigin, err := normalizeTaskRunIdempotencyLookup(key, origin)
	if err != nil {
		return taskpkg.Run{}, err
	}
	record, err := getTaskRunIdempotencyRecord(ctx, s.exec, trimmedKey, normalizedOrigin)
	if err != nil {
		return taskpkg.Run{}, err
	}
	return s.tasks.getTaskRunWithExecutor(ctx, s.exec, record.RunID)
}

func (s *taskExecutionTxStore) SaveTaskRunIdempotency(
	ctx context.Context,
	record taskpkg.RunIdempotency,
) error {
	return s.tasks.saveTaskRunIdempotencyWithExecutor(ctx, s.exec, record)
}

func (s *taskExecutionTxStore) CreateTaskEvent(ctx context.Context, event taskpkg.Event) error {
	normalized, err := s.tasks.normalizeTaskEventForCreate(event)
	if err != nil {
		return err
	}
	return appendTaskEventWithExecutor(ctx, s.exec, EventRecordInsert{
		ID:        normalized.ID,
		TaskID:    normalized.TaskID,
		RunID:     normalized.RunID,
		EventType: normalized.EventType,
		Actor:     normalized.Actor,
		Origin:    normalized.Origin,
		Payload:   normalized.Payload,
		Timestamp: normalized.Timestamp,
	})
}

func (s *taskExecutionTxStore) ReserveQueuedRun(
	ctx context.Context,
	reservation taskpkg.QueueRunReservation,
) (taskpkg.Task, taskpkg.Run, bool, error) {
	input, err := s.tasks.normalizeQueuedRunReservationInput(reservation)
	if err != nil {
		return taskpkg.Task{}, taskpkg.Run{}, false, err
	}
	return s.tasks.reserveQueuedRunWithExecutor(ctx, s.exec, input)
}
