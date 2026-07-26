package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

func (g *TaskRepo) normalizeTaskDependencyForCreate(record taskpkg.Dependency) (taskpkg.Dependency, error) {
	normalized := normalizeTaskDependencyRecord(record)
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = g.now()
	}
	if err := normalized.Validate(); err != nil {
		return taskpkg.Dependency{}, err
	}
	return normalized, nil
}

func (g *TaskRepo) normalizeTaskTriageStateForUpsert(
	record taskpkg.TriageState,
) (taskpkg.TriageState, error) {
	normalized := normalizeTaskTriageStateRecord(record)
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = g.now()
	}
	if err := normalized.Validate(); err != nil {
		return taskpkg.TriageState{}, err
	}
	return normalized, nil
}

func (g *TaskRepo) normalizeTaskEventForCreate(record taskpkg.Event) (taskpkg.Event, error) {
	normalized := normalizeTaskEventRecord(record)
	if normalized.Timestamp.IsZero() {
		normalized.Timestamp = g.now()
	}
	if err := normalized.Validate(); err != nil {
		return taskpkg.Event{}, err
	}
	return normalized, nil
}

func (g *TaskRepo) normalizeTaskRunIdempotencyForCreate(
	record taskpkg.RunIdempotency,
) (taskpkg.RunIdempotency, error) {
	normalized := normalizeTaskRunIdempotencyRecord(record)
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = g.now()
	}
	if err := normalized.Validate(); err != nil {
		return taskpkg.RunIdempotency{}, err
	}
	return normalized, nil
}

func (g *TaskRepo) ensureTaskExistsWithExecutor(ctx context.Context, exec taskSQLExecutor, taskID string) error {
	trimmedID, err := requireTaskValue(taskID, "task id")
	if err != nil {
		return err
	}

	if _, err := sqlcgen.New(exec).GetTaskID(ctx, trimmedID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return taskpkg.ErrTaskNotFound
		}
		return fmt.Errorf("store: lookup task %q: %w", trimmedID, err)
	}

	return nil
}

func (g *TaskRepo) getTaskRunWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	runID string,
) (taskpkg.Run, error) {
	trimmedRunID, err := requireTaskValue(runID, "task run id")
	if err != nil {
		return taskpkg.Run{}, err
	}

	row, err := sqlcgen.New(exec).GetTaskRun(ctx, trimmedRunID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return taskpkg.Run{}, taskpkg.ErrTaskRunNotFound
		}
		return taskpkg.Run{}, err
	}
	run, err := taskRunFromGenerated(&row)
	if err != nil {
		return taskpkg.Run{}, err
	}
	return g.loadTaskRunCapabilities(ctx, exec, run)
}

func (g *TaskRepo) getTaskWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
) (taskpkg.Task, error) {
	trimmedTaskID, err := requireTaskValue(taskID, "task id")
	if err != nil {
		return taskpkg.Task{}, err
	}

	row, err := sqlcgen.New(exec).GetTask(ctx, trimmedTaskID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return taskpkg.Task{}, taskpkg.ErrTaskNotFound
		}
		return taskpkg.Task{}, err
	}
	return taskFromGenerated(&row)
}

func nextTaskEventSequenceWithExecutor(ctx context.Context, exec taskSQLExecutor) (int64, error) {
	current, err := sqlcgen.New(exec).MaxTaskEventSequence(ctx)
	if err != nil {
		return 0, fmt.Errorf("store: query next task event sequence: %w", err)
	}
	return current + 1, nil
}

func nextTaskRunAttemptWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	taskRecord taskpkg.Task,
) (int, error) {
	nextAttempt, err := nextTaskRunAttemptNumberWithExecutor(ctx, exec, taskRecord)
	if err != nil {
		return 0, err
	}
	if err := validateNextTaskRunAttempt(taskRecord, nextAttempt); err != nil {
		return 0, err
	}
	return nextAttempt, nil
}

func nextQueuedRunReservationAttemptWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	taskRecord taskpkg.Task,
	runKind taskpkg.RunKind,
) (int, error) {
	nextAttempt, err := nextTaskRunAttemptNumberWithExecutor(ctx, exec, taskRecord)
	if err != nil {
		return 0, err
	}
	if runKind.Normalize() == taskpkg.RunKindCoordinator {
		return nextAttempt, nil
	}
	if err := validateNextTaskRunAttempt(taskRecord, nextAttempt); err != nil {
		return 0, err
	}
	return nextAttempt, nil
}

func nextTaskRunAttemptNumberWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	taskRecord taskpkg.Task,
) (int, error) {
	current, err := sqlcgen.New(exec).MaxTaskRunAttempt(ctx, nullableTaskString(taskRecord.ID))
	if err != nil {
		return 0, fmt.Errorf("store: query next task run attempt for %q: %w", taskRecord.ID, err)
	}
	return int(current) + 1, nil
}

func validateNextTaskRunAttempt(taskRecord taskpkg.Task, nextAttempt int) error {
	maxAttempts := normalizeStoredTaskMaxAttempts(taskRecord.MaxAttempts)
	if nextAttempt > maxAttempts {
		return fmt.Errorf(
			"%w: task %q exhausted max_attempts=%d",
			taskpkg.ErrInvalidStatusTransition,
			taskRecord.ID,
			maxAttempts,
		)
	}
	return nil
}

func (g *TaskRepo) countDependenciesWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
) (int, error) {
	count, err := sqlcgen.New(exec).CountTaskDependencies(ctx, taskID)
	if err != nil {
		return 0, fmt.Errorf("store: count task dependencies for %q: %w", taskID, err)
	}
	return int(count), nil
}

func (g *TaskRepo) taskDependencyExists(
	ctx context.Context,
	exec taskSQLExecutor,
	dependency taskpkg.Dependency,
) (bool, error) {
	exists, err := sqlcgen.New(exec).TaskDependencyExists(ctx, sqlcgen.TaskDependencyExistsParams{
		TaskID: dependency.TaskID, DependsOnTaskID: dependency.DependsOnTaskID, Kind: string(dependency.Kind),
	})
	if err != nil {
		return false, fmt.Errorf(
			"store: lookup task dependency %q -> %q: %w",
			dependency.TaskID,
			dependency.DependsOnTaskID,
			err,
		)
	}
	return exists, nil
}

func (g *TaskRepo) hasDependencyPathWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	fromTaskID string,
	toTaskID string,
) (bool, error) {
	// dynamic-sql: sqlc's SQLite analyzer cannot resolve the recursive CTE column scope in this fixed graph query.
	var exists int
	if err := exec.QueryRowContext(
		ctx,
		`WITH RECURSIVE dependency_path(node_id) AS (
			SELECT depends_on_task_id
			  FROM task_dependencies
			 WHERE task_id = ?
			UNION
			SELECT td.depends_on_task_id
			  FROM task_dependencies td
			  JOIN dependency_path path ON td.task_id = path.node_id
		)
		SELECT EXISTS(SELECT 1 FROM dependency_path WHERE node_id = ?)`,
		fromTaskID,
		toTaskID,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("store: query task dependency path %q -> %q: %w", fromTaskID, toTaskID, err)
	}
	return exists == 1, nil
}

func getTaskRunIdempotencyRecord(
	ctx context.Context,
	exec taskSQLExecutor,
	key string,
	origin taskpkg.Origin,
) (taskpkg.RunIdempotency, error) {
	row, err := sqlcgen.New(exec).GetTaskRunIdempotency(ctx, sqlcgen.GetTaskRunIdempotencyParams{
		IdempotencyKey: key,
		OriginKind:     string(origin.Kind),
		OriginRef:      origin.Ref,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return taskpkg.RunIdempotency{}, taskpkg.ErrTaskRunIdempotencyNotFound
		}
		return taskpkg.RunIdempotency{}, fmt.Errorf("store: get task run idempotency %q: %w", key, err)
	}
	return taskRunIdempotencyFromGenerated(row)
}

func joinRowsCloseError(rows *sql.Rows, base error, context string) error {
	closeErr := rows.Close()
	switch {
	case base != nil && closeErr != nil:
		return errors.Join(base, fmt.Errorf("store: close %s rows: %w", context, closeErr))
	case base != nil:
		return base
	case closeErr != nil:
		return fmt.Errorf("store: close %s rows: %w", context, closeErr)
	default:
		return nil
	}
}

func validateTaskForQueuedRunReservation(taskRecord taskpkg.Task) error {
	switch taskRecord.Status.Normalize() {
	case taskpkg.TaskStatusDraft:
		return fmt.Errorf("%w: task %q is draft", taskpkg.ErrInvalidStatusTransition, taskRecord.ID)
	case taskpkg.TaskStatusCanceled:
		return fmt.Errorf("%w: task %q is canceled", taskpkg.ErrInvalidStatusTransition, taskRecord.ID)
	default:
		return nil
	}
}

func (g *TaskRepo) findOpenRunIDForQueuedRunReservation(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
	designationGroupID string,
) (string, error) {
	normalizedDesignationGroupID := strings.TrimSpace(designationGroupID)
	runID, err := sqlcgen.New(exec).GetOpenTaskRunID(ctx, sqlcgen.GetOpenTaskRunIDParams{
		TaskID:             nullableTaskString(taskID),
		CompletedStatus:    taskpkg.TaskRunStatusCompleted.String(),
		FailedStatus:       taskpkg.TaskRunStatusFailed.String(),
		CanceledStatus:     taskpkg.TaskRunStatusCanceled.String(),
		DesignationGroupID: normalizedDesignationGroupID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("store: lookup open queued run reservation for task %q: %w", taskID, err)
	}
	return runID, nil
}

func normalizeStoredTaskMaxAttempts(maxAttempts int) int {
	if maxAttempts <= 0 {
		return taskpkg.DefaultTaskMaxAttempts
	}
	return maxAttempts
}
