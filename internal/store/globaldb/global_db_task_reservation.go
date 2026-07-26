package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

// ReserveQueuedRun atomically allocates one queued run attempt and optional idempotency binding.
func (g *TaskRepo) ReserveQueuedRun(
	ctx context.Context,
	reservation taskpkg.QueueRunReservation,
) (taskpkg.Task, taskpkg.Run, bool, error) {
	if err := g.checkReady(ctx, "reserve queued task run"); err != nil {
		return taskpkg.Task{}, taskpkg.Run{}, false, err
	}

	input, err := g.normalizeQueuedRunReservationInput(reservation)
	if err != nil {
		return taskpkg.Task{}, taskpkg.Run{}, false, err
	}

	var reservedTask taskpkg.Task
	var reservedRun taskpkg.Run
	var existing bool
	if err := g.withTaskImmediateTransaction(ctx, "reserve queued task run", func(exec taskSQLExecutor) error {
		taskRecord, runRecord, alreadyExists, err := g.reserveQueuedRunWithExecutor(ctx, exec, input)
		if err != nil {
			return err
		}
		reservedTask = taskRecord
		reservedRun = runRecord
		existing = alreadyExists
		return nil
	}); err != nil {
		return taskpkg.Task{}, taskpkg.Run{}, false, err
	}

	return reservedTask, reservedRun, existing, nil
}

func (g *TaskRepo) normalizeQueuedRunReservationInput(
	reservation taskpkg.QueueRunReservation,
) (queuedRunReservationInput, error) {
	normalizedReservation := reservation
	normalizedReservation.TaskID = strings.TrimSpace(normalizedReservation.TaskID)
	normalizedReservation.RunID = strings.TrimSpace(normalizedReservation.RunID)
	normalizedReservation.RunKind = normalizedReservation.RunKind.Normalize()
	if normalizedReservation.RunKind == taskpkg.RunKindUnknown {
		normalizedReservation.RunKind = taskpkg.RunKindWorker
	}
	normalizedReservation.LoopRunID = strings.TrimSpace(normalizedReservation.LoopRunID)
	normalizedOrigin := taskpkg.Origin{
		Kind: normalizedReservation.Origin.Kind.Normalize(),
		Ref:  strings.TrimSpace(normalizedReservation.Origin.Ref),
	}
	normalizedReservation.Origin = normalizedOrigin
	normalizedReservation.IdempotencyKey = strings.TrimSpace(normalizedReservation.IdempotencyKey)
	normalizedReservation.DesignationGroupID = strings.TrimSpace(normalizedReservation.DesignationGroupID)
	normalizedReservation.Metadata = normalizeTaskJSON(normalizedReservation.Metadata)
	if err := normalizedReservation.Validate("queue_run_reservation"); err != nil {
		return queuedRunReservationInput{}, err
	}
	normalizedQueuedAt := normalizedReservation.QueuedAt.UTC()
	if normalizedQueuedAt.IsZero() {
		normalizedQueuedAt = g.now()
	}
	return queuedRunReservationInput{
		taskID:             normalizedReservation.TaskID,
		runID:              normalizedReservation.RunID,
		runKind:            normalizedReservation.RunKind,
		loopRunID:          normalizedReservation.LoopRunID,
		idempotencyKey:     normalizedReservation.IdempotencyKey,
		origin:             normalizedOrigin,
		networkSpec:        normalizedReservation.NetworkSpec,
		designationGroupID: normalizedReservation.DesignationGroupID,
		metadata:           normalizedReservation.Metadata,
		queuedAt:           normalizedQueuedAt,
	}, nil
}

func (g *TaskRepo) reserveQueuedRunWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	input queuedRunReservationInput,
) (taskpkg.Task, taskpkg.Run, bool, error) {
	taskRecord, err := g.getTaskWithExecutor(ctx, exec, input.taskID)
	if err != nil {
		return taskpkg.Task{}, taskpkg.Run{}, false, err
	}
	if err := validateTaskForQueuedRunReservation(taskRecord); err != nil {
		return taskpkg.Task{}, taskpkg.Run{}, false, err
	}

	runRecord, exists, err := g.lookupQueuedRunReservationByIdempotency(
		ctx,
		exec,
		taskRecord,
		input.idempotencyKey,
		input.origin,
	)
	if err != nil {
		return taskpkg.Task{}, taskpkg.Run{}, false, err
	}
	if exists {
		return taskRecord, runRecord, true, nil
	}

	openRunID, err := g.findOpenRunIDForQueuedRunReservation(ctx, exec, taskRecord.ID, input.designationGroupID)
	if err != nil {
		return taskpkg.Task{}, taskpkg.Run{}, false, err
	}
	if openRunID != "" {
		return taskpkg.Task{}, taskpkg.Run{}, false, fmt.Errorf(
			"%w: task %q has open run %q; finish or cancel it before enqueueing another run",
			taskpkg.ErrInvalidStatusTransition,
			taskRecord.ID,
			openRunID,
		)
	}

	runRecord, err = g.createQueuedRunWithExecutor(ctx, exec, taskRecord, input)
	if err != nil {
		return taskpkg.Task{}, taskpkg.Run{}, false, err
	}
	return taskRecord, runRecord, false, nil
}

func (g *TaskRepo) lookupQueuedRunReservationByIdempotency(
	ctx context.Context,
	exec taskSQLExecutor,
	taskRecord taskpkg.Task,
	idempotencyKey string,
	origin taskpkg.Origin,
) (taskpkg.Run, bool, error) {
	if idempotencyKey == "" {
		return taskpkg.Run{}, false, nil
	}

	current, err := getTaskRunIdempotencyRecord(ctx, exec, idempotencyKey, origin)
	switch {
	case errors.Is(err, taskpkg.ErrTaskRunIdempotencyNotFound):
		return taskpkg.Run{}, false, nil
	case err != nil:
		return taskpkg.Run{}, false, err
	}

	run, err := g.getTaskRunWithExecutor(ctx, exec, current.RunID)
	if err != nil {
		return taskpkg.Run{}, false, err
	}
	if strings.TrimSpace(run.TaskID) != taskRecord.ID {
		return taskpkg.Run{}, false, fmt.Errorf(
			"%w: idempotency key %q is already bound to task %q",
			taskpkg.ErrValidation,
			idempotencyKey,
			run.TaskID,
		)
	}
	return run, true, nil
}

func (g *TaskRepo) createQueuedRunWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	taskRecord taskpkg.Task,
	input queuedRunReservationInput,
) (taskpkg.Run, error) {
	nextAttempt, err := nextQueuedRunReservationAttemptWithExecutor(ctx, exec, taskRecord, input.runKind)
	if err != nil {
		return taskpkg.Run{}, err
	}
	runAttempt, err := taskRunAttemptFromInt(nextAttempt)
	if err != nil {
		return taskpkg.Run{}, err
	}

	workspaceID := strings.TrimSpace(taskRecord.WorkspaceID)
	if workspaceID == "" {
		workspaceID = strings.TrimSpace(input.networkSpec.WorkspaceID)
	}
	run := taskpkg.Run{
		ID:                 input.runID,
		TaskID:             taskRecord.ID,
		WorkspaceID:        workspaceID,
		RunKind:            input.runKind,
		LoopRunID:          input.loopRunID,
		Status:             taskpkg.TaskRunStatusQueued,
		Attempt:            runAttempt,
		Origin:             input.origin,
		IdempotencyKey:     input.idempotencyKey,
		DesignationGroupID: input.designationGroupID,
		Metadata:           input.metadata,
		QueuedAt:           input.queuedAt,
	}
	run.SetNetworkState(input.networkSpec, "", "", "")
	normalizedRun, err := g.normalizeTaskRunForCreate(run)
	if err != nil {
		return taskpkg.Run{}, err
	}
	if err := insertQueuedTaskRun(ctx, exec, normalizedRun); err != nil {
		return taskpkg.Run{}, err
	}
	if err := g.saveQueuedRunIdempotencyWithExecutor(ctx, exec, normalizedRun); err != nil {
		return taskpkg.Run{}, err
	}
	return normalizedRun, nil
}

func taskRunAttemptFromInt(value int) (int32, error) {
	const maxTaskRunAttempt = int(^uint32(0) >> 1)
	if value <= 0 || value > maxTaskRunAttempt {
		return 0, fmt.Errorf("%w: task run attempt out of range: %d", taskpkg.ErrValidation, value)
	}
	return int32(value), nil
}

func insertQueuedTaskRun(ctx context.Context, exec taskSQLExecutor, run taskpkg.Run) error {
	if err := insertTaskRunWithExecutor(ctx, exec, run); err != nil {
		return err
	}
	return replaceTaskRunCapabilitiesWithExecutor(ctx, exec, run)
}

func (g *TaskRepo) saveQueuedRunIdempotencyWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
) error {
	if strings.TrimSpace(run.IdempotencyKey) == "" {
		return nil
	}

	idempotency := taskpkg.RunIdempotency{
		IdempotencyKey: run.IdempotencyKey,
		RunID:          run.ID,
		Origin:         run.Origin,
		CreatedAt:      run.QueuedAt,
	}
	normalizedID, err := g.normalizeTaskRunIdempotencyForCreate(idempotency)
	if err != nil {
		return err
	}
	rowsAffected, err := sqlcgen.New(exec).InsertTaskRunIdempotency(
		ctx,
		sqlcgen.InsertTaskRunIdempotencyParams{
			IdempotencyKey: normalizedID.IdempotencyKey,
			OriginKind:     string(normalizedID.Origin.Kind),
			OriginRef:      normalizedID.Origin.Ref,
			RunID:          normalizedID.RunID,
			CreatedAt:      store.FormatTimestamp(normalizedID.CreatedAt),
		},
	)
	if err != nil {
		return fmt.Errorf("store: save task run idempotency %q: %w", normalizedID.IdempotencyKey, err)
	}
	if rowsAffected > 0 {
		return nil
	}

	current, err := getTaskRunIdempotencyRecord(ctx, exec, normalizedID.IdempotencyKey, normalizedID.Origin)
	if err != nil {
		return err
	}
	if current.RunID != normalizedID.RunID {
		return fmt.Errorf(
			"%w: idempotency key %q is already bound to run %q",
			taskpkg.ErrValidation,
			normalizedID.IdempotencyKey,
			current.RunID,
		)
	}
	return nil
}

// GetTaskRunByIdempotencyKey returns the original persisted run bound to one origin-scoped idempotency key.
func (g *TaskRepo) GetTaskRunByIdempotencyKey(
	ctx context.Context,
	key string,
	origin taskpkg.Origin,
) (taskpkg.Run, error) {
	if err := g.checkReady(ctx, "get task run by idempotency key"); err != nil {
		return taskpkg.Run{}, err
	}

	trimmedKey, normalizedOrigin, err := normalizeTaskRunIdempotencyLookup(key, origin)
	if err != nil {
		return taskpkg.Run{}, err
	}

	runID, err := g.queries.GetTaskRunIDByIdempotency(ctx, sqlcgen.GetTaskRunIDByIdempotencyParams{
		IdempotencyKey: trimmedKey,
		OriginKind:     string(normalizedOrigin.Kind),
		OriginRef:      normalizedOrigin.Ref,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return taskpkg.Run{}, taskpkg.ErrTaskRunIdempotencyNotFound
		}
		return taskpkg.Run{}, fmt.Errorf("store: lookup task run idempotency %q: %w", trimmedKey, err)
	}
	return g.getTaskRunWithExecutor(ctx, g.db, runID)
}

// SaveTaskRunIdempotency inserts one origin-scoped idempotency binding for a persisted run.
func (g *TaskRepo) SaveTaskRunIdempotency(ctx context.Context, record taskpkg.RunIdempotency) error {
	if err := g.checkReady(ctx, "save task run idempotency"); err != nil {
		return err
	}

	return g.saveTaskRunIdempotencyWithExecutor(ctx, g.db, record)
}

func (g *TaskRepo) saveTaskRunIdempotencyWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	record taskpkg.RunIdempotency,
) error {
	normalized, err := g.normalizeTaskRunIdempotencyForCreate(record)
	if err != nil {
		return err
	}

	run, err := g.getTaskRunWithExecutor(ctx, exec, normalized.RunID)
	if err != nil {
		return err
	}
	if !taskOriginsEqual(run.Origin, normalized.Origin) {
		return fmt.Errorf(
			"%w: task_run_idempotency origin %q/%q does not match run origin %q/%q",
			taskpkg.ErrValidation,
			normalized.Origin.Kind,
			normalized.Origin.Ref,
			run.Origin.Kind,
			run.Origin.Ref,
		)
	}

	rowsAffected, err := sqlcgen.New(exec).InsertTaskRunIdempotency(ctx, sqlcgen.InsertTaskRunIdempotencyParams{
		IdempotencyKey: normalized.IdempotencyKey,
		OriginKind:     string(normalized.Origin.Kind),
		OriginRef:      normalized.Origin.Ref,
		RunID:          normalized.RunID,
		CreatedAt:      store.FormatTimestamp(normalized.CreatedAt),
	})
	if err != nil {
		return fmt.Errorf("store: save task run idempotency %q: %w", normalized.IdempotencyKey, err)
	}
	if rowsAffected > 0 {
		return nil
	}

	current, err := getTaskRunIdempotencyRecord(ctx, exec, normalized.IdempotencyKey, normalized.Origin)
	if err != nil {
		return err
	}
	if current.RunID != normalized.RunID {
		return fmt.Errorf(
			"%w: idempotency key %q is already bound to run %q",
			taskpkg.ErrValidation,
			normalized.IdempotencyKey,
			current.RunID,
		)
	}

	return nil
}
