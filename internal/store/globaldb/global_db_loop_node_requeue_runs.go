package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	taskpkg "github.com/compozy/compozy/internal/task"
)

type requeuedNodeContinuationRun struct {
	itemIndex int
	attempt   int
	epoch     int64
	metadata  string
}

func (g *LoopRepo) reserveNodeRequeueCoordinatorWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.NodeRequeueMutation,
	prepared *preparedNodeRequeue,
	idempotencyKey string,
) (taskpkg.Run, bool, error) {
	coordinatorTaskID := loopCoordinatorTaskID(prepared.run.ID)
	openRunID, err := g.tasks.findOpenRunIDForQueuedRunReservation(ctx, exec, coordinatorTaskID, "")
	if err != nil {
		return taskpkg.Run{}, false, err
	}
	continuations, err := listRequeuedNodeContinuationRuns(
		ctx,
		exec,
		mutation.RunID,
		mutation.NodeID,
		prepared.nextGeneration,
	)
	if err != nil {
		return taskpkg.Run{}, false, err
	}
	if len(continuations) == 0 {
		coordinator, err := g.reserveOrReuseOpenLoopCoordinatorRunWithExecutor(
			ctx,
			exec,
			prepared.run,
			mutation.Actor.Origin,
			mutation.RequestedAt,
			idempotencyKey,
		)
		return coordinator, false, err
	}
	if err := ensureRetainedRequeueGenerationWithExecutor(
		ctx,
		exec,
		prepared,
		mutation.RequestedAt,
	); err != nil {
		return taskpkg.Run{}, false, err
	}
	expectedRunID := loopCoordinatorRunID(prepared.run.ID, prepared.nextGeneration)
	if openRunID != "" {
		coordinator, err := g.reserveOrReuseOpenLoopCoordinatorRunWithExecutor(
			ctx,
			exec,
			prepared.run,
			mutation.Actor.Origin,
			mutation.RequestedAt,
			idempotencyKey,
		)
		if err != nil {
			return taskpkg.Run{}, false, err
		}
		if coordinator.ID != expectedRunID {
			return taskpkg.Run{}, false, fmt.Errorf(
				"%w: open Loop coordinator %q is not requeue generation %d",
				looppkg.ErrTransitionConflict,
				coordinator.ID,
				prepared.nextGeneration,
			)
		}
		return coordinator, true, nil
	}
	coordinator, err := g.reserveOrReuseOpenLoopCoordinatorRunWithExecutor(
		ctx,
		exec,
		prepared.run,
		mutation.Actor.Origin,
		mutation.RequestedAt,
		idempotencyKey,
	)
	if err != nil {
		return taskpkg.Run{}, false, err
	}
	return coordinator, true, nil
}

func ensureRetainedRequeueGenerationWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	prepared *preparedNodeRequeue,
	createdAt time.Time,
) error {
	var exists int
	err := exec.QueryRowContext(
		ctx,
		`SELECT 1 FROM loop_generations WHERE loop_run_id = ? AND generation = ?`,
		prepared.run.ID,
		prepared.nextGeneration,
	).Scan(&exists)
	if err == nil {
		_, err = getLoopGenerationIntentWithExecutor(
			ctx,
			exec,
			prepared.run.ID,
			prepared.nextGeneration,
		)
		return err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("store: inspect retained requeue generation: %w", err)
	}
	return insertLoopGenerationWithExecutor(ctx, exec, prepared.run.ID, looppkg.GenerationIntent{
		Generation:       int64(prepared.nextGeneration),
		ParentGeneration: int64(prepared.run.Generation),
		Origin:           looppkg.OriginRequeue,
	}, createdAt)
}

func (g *LoopRepo) reserveRequeuedNodeContinuationRunsWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.NodeRequeueMutation,
	generation int,
	coordinator taskpkg.Run,
) ([]taskpkg.Run, error) {
	continuations, err := listRequeuedNodeContinuationRuns(
		ctx,
		exec,
		mutation.RunID,
		mutation.NodeID,
		generation,
	)
	if err != nil {
		return nil, err
	}
	if len(continuations) == 0 {
		return nil, fmt.Errorf(
			"%w: requeued Loop node %q has no pending continuation in generation %d",
			taskpkg.ErrInvalidStatusTransition,
			mutation.NodeID,
			generation,
		)
	}
	plan, err := requeuedNodeContinuationPlan(mutation, generation, continuations)
	if err != nil {
		return nil, err
	}
	workers, err := g.tasks.reserveCoordinatorPlanRunsWithExecutor(
		ctx,
		exec,
		plan,
		coordinator,
		mutation.Actor.Origin,
		mutation.RequestedAt,
	)
	if err != nil {
		return nil, err
	}
	if err := markRequeuedNodeContinuationsEnqueued(
		ctx, exec, mutation, generation, plan, continuations,
	); err != nil {
		return nil, err
	}
	return workers, nil
}

func requeuedNodeContinuationPlan(
	mutation looppkg.NodeRequeueMutation,
	generation int,
	continuations []requeuedNodeContinuationRun,
) (taskpkg.CoordinatorCompletionPlan, error) {
	plan := taskpkg.CoordinatorCompletionPlan{NodeRuns: make([]taskpkg.EnqueueSpec, 0, len(continuations))}
	for _, continuation := range continuations {
		metadata, err := requeuedNodeRunMetadata(continuation)
		if err != nil {
			return taskpkg.CoordinatorCompletionPlan{}, err
		}
		plan.NodeRuns = append(plan.NodeRuns, requeuedNodeContinuationSpec(
			mutation, generation, continuation, metadata,
		))
	}
	return plan, nil
}

func requeuedNodeContinuationSpec(
	mutation looppkg.NodeRequeueMutation,
	generation int,
	continuation requeuedNodeContinuationRun,
	metadata []byte,
) taskpkg.EnqueueSpec {
	return taskpkg.EnqueueSpec{
		TaskID: looppkg.NodeCellTaskID(mutation.RunID, generation, string(mutation.NodeID), continuation.itemIndex),
		RunID: looppkg.NodeCellAttemptRunID(
			mutation.RunID, generation, string(mutation.NodeID), continuation.itemIndex, continuation.attempt,
		),
		RunKind:   taskpkg.RunKindWorker,
		LoopRunID: string(mutation.RunID),
		IdempotencyKey: looppkg.NodeCellAttemptIdempotencyKey(
			mutation.RunID, generation, string(mutation.NodeID), continuation.itemIndex, continuation.attempt,
		),
		Metadata: metadata,
	}
}

func markRequeuedNodeContinuationsEnqueued(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.NodeRequeueMutation,
	generation int,
	plan taskpkg.CoordinatorCompletionPlan,
	continuations []requeuedNodeContinuationRun,
) error {
	for index, continuation := range continuations {
		changed, err := exec.ExecContext(ctx, `UPDATE loop_generation_outputs
			 SET status = 'enqueued', task_run_id = ?, attempt = ?,
			     first_scheduled_at = COALESCE(first_scheduled_at, ?)
			 WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?
			   AND status = 'pending'`,
			plan.NodeRuns[index].RunID, continuation.attempt, mutation.RequestedAt.UTC(),
			mutation.RunID, generation, mutation.NodeID, continuation.itemIndex,
		)
		if err != nil {
			return fmt.Errorf("store: enqueue requeued Loop node continuation: %w", err)
		}
		affected, err := changed.RowsAffected()
		if err != nil {
			return fmt.Errorf("store: inspect requeued Loop continuation enqueue: %w", err)
		}
		if affected != 1 {
			return fmt.Errorf(
				"%w: requeued Loop node %q item %d enqueue changed %d rows",
				looppkg.ErrTransitionConflict, mutation.NodeID, continuation.itemIndex, affected,
			)
		}
	}
	return nil
}

func listRequeuedNodeContinuationRuns(
	ctx context.Context,
	exec taskSQLExecutor,
	runID looppkg.RunID,
	nodeID looppkg.NodeID,
	generation int,
) (_ []requeuedNodeContinuationRun, err error) {
	rows, err := exec.QueryContext(
		ctx,
		`SELECT outputs.item_index, outputs.attempt, outputs.epoch, tasks.metadata_json
		 FROM loop_generation_outputs AS outputs
		 JOIN tasks ON tasks.id = (
			'loop.' || outputs.loop_run_id || '.g' || outputs.generation ||
			'.node.' || outputs.node_id || '.' || outputs.item_index
		 )
		 WHERE outputs.loop_run_id = ? AND outputs.generation = ? AND outputs.node_id = ?
		   AND outputs.status = 'pending' AND tasks.status = 'ready'
		 ORDER BY outputs.item_index`,
		runID,
		generation,
		nodeID,
	)
	if err != nil {
		return nil, fmt.Errorf("store: list requeued Loop node continuations: %w", err)
	}
	defer func() { err = joinRowsCloseError(rows, err, "requeued Loop continuation rows") }()
	continuations := make([]requeuedNodeContinuationRun, 0, 4)
	for rows.Next() {
		var continuation requeuedNodeContinuationRun
		if err := rows.Scan(
			&continuation.itemIndex,
			&continuation.attempt,
			&continuation.epoch,
			&continuation.metadata,
		); err != nil {
			return nil, fmt.Errorf("store: scan requeued Loop node continuation: %w", err)
		}
		if continuation.attempt < 1 {
			return nil, fmt.Errorf(
				"%w: requeued Loop node attempt must be positive, got %d",
				taskpkg.ErrValidation,
				continuation.attempt,
			)
		}
		if continuation.epoch < 1 {
			return nil, fmt.Errorf(
				"%w: requeued Loop node epoch must be positive, got %d",
				taskpkg.ErrValidation,
				continuation.epoch,
			)
		}
		continuations = append(continuations, continuation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate requeued Loop node continuations: %w", err)
	}
	return continuations, nil
}

func requeuedNodeRunMetadata(continuation requeuedNodeContinuationRun) ([]byte, error) {
	metadata := map[string]any{}
	if err := json.Unmarshal([]byte(continuation.metadata), &metadata); err != nil {
		return nil, fmt.Errorf("store: decode requeued Loop node metadata: %w", err)
	}
	metadata["attempt"] = continuation.attempt
	metadata["epoch"] = continuation.epoch
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("store: encode requeued Loop node metadata: %w", err)
	}
	return encoded, nil
}
