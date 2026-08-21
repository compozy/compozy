package globaldb

import (
	"context"
	"fmt"

	looppkg "github.com/compozy/compozy/internal/loop"
	taskpkg "github.com/compozy/compozy/internal/task"
)

type requeuedNodeContinuationRun struct {
	itemIndex int
	attempt   int
	metadata  string
}

func (g *LoopRepo) reserveRequeuedNodeContinuationRunsWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.NodeRequeueMutation,
	generation int,
	coordinator taskpkg.Run,
) error {
	continuations, err := listRequeuedNodeContinuationRuns(
		ctx,
		exec,
		mutation.RunID,
		mutation.NodeID,
		generation,
	)
	if err != nil {
		return err
	}
	if len(continuations) == 0 {
		return fmt.Errorf(
			"%w: requeued Loop node %q has no pending continuation in generation %d",
			taskpkg.ErrInvalidStatusTransition,
			mutation.NodeID,
			generation,
		)
	}
	plan := taskpkg.CoordinatorCompletionPlan{
		NodeRuns: make([]taskpkg.EnqueueSpec, 0, len(continuations)),
	}
	for _, continuation := range continuations {
		runID := looppkg.NodeCellAttemptRunID(
			mutation.RunID,
			generation,
			string(mutation.NodeID),
			continuation.itemIndex,
			continuation.attempt,
		)
		plan.NodeRuns = append(plan.NodeRuns, taskpkg.EnqueueSpec{
			TaskID: looppkg.NodeCellTaskID(
				mutation.RunID,
				generation,
				string(mutation.NodeID),
				continuation.itemIndex,
			),
			RunID:     runID,
			RunKind:   taskpkg.RunKindWorker,
			LoopRunID: string(mutation.RunID),
			IdempotencyKey: looppkg.NodeCellAttemptIdempotencyKey(
				mutation.RunID,
				generation,
				string(mutation.NodeID),
				continuation.itemIndex,
				continuation.attempt,
			),
			Metadata: []byte(continuation.metadata),
		})
	}
	if _, err := g.tasks.reserveCoordinatorPlanRunsWithExecutor(
		ctx,
		exec,
		plan,
		coordinator,
		mutation.Actor.Origin,
		mutation.RequestedAt,
	); err != nil {
		return err
	}
	for index, continuation := range continuations {
		changed, err := exec.ExecContext(
			ctx,
			`UPDATE loop_generation_outputs
			 SET status = 'enqueued', task_run_id = ?,
			     first_scheduled_at = COALESCE(first_scheduled_at, ?)
			 WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?
			   AND status = 'pending'`,
			plan.NodeRuns[index].RunID,
			mutation.RequestedAt.UTC(),
			mutation.RunID,
			generation,
			mutation.NodeID,
			continuation.itemIndex,
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
				looppkg.ErrTransitionConflict,
				mutation.NodeID,
				continuation.itemIndex,
				affected,
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
		`SELECT outputs.item_index, outputs.attempt, tasks.metadata_json
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
		if err := rows.Scan(&continuation.itemIndex, &continuation.attempt, &continuation.metadata); err != nil {
			return nil, fmt.Errorf("store: scan requeued Loop node continuation: %w", err)
		}
		continuations = append(continuations, continuation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate requeued Loop node continuations: %w", err)
	}
	return continuations, nil
}
