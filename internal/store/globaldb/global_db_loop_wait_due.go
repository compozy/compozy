package globaldb

import (
	"context"
	"errors"
	"fmt"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	taskpkg "github.com/compozy/compozy/internal/task"
)

type waitDueCell struct {
	workspaceID looppkg.WorkspaceID
	runID       looppkg.RunID
	generation  int
	nodeID      looppkg.NodeID
	itemIndex   int
	resumeAt    time.Time
}

// ResumeDueLoopWaitsPage resolves one stable page of due timer waits.
func (g *LoopRepo) ResumeDueLoopWaitsPage(
	ctx context.Context,
	now time.Time,
	cursor looppkg.WaitDueCursor,
	limit int,
) ([]taskpkg.Run, looppkg.WaitDueCursor, error) {
	cells, next, err := g.listDueLoopWaits(ctx, now, cursor, limit)
	if err != nil {
		return nil, looppkg.WaitDueCursor{}, err
	}
	runs := make([]taskpkg.Run, 0, len(cells))
	definitions := make(map[looppkg.RunID]*looppkg.ResolvedDefinition)
	completed := cursor
	for _, cell := range cells {
		admissionAttempts, configErr := g.waitDueAdmissionAttempts(ctx, cell, definitions)
		if configErr != nil {
			return runs, completed, configErr
		}
		result, resumeErr := g.ResumeWait(ctx, looppkg.WaitResumeMutation{
			WorkspaceID: cell.workspaceID, RunID: cell.runID, Generation: cell.generation,
			NodeID: cell.nodeID, ItemIndex: cell.itemIndex, Payload: []byte(`{}`),
			ClaimedByKind: loopWaitClaimedByTimer, ClaimedByID: loopWaitClaimedByScheduler,
			AdmissionAttempts: admissionAttempts, RequestedAt: now.UTC(),
		})
		if errors.Is(resumeErr, looppkg.ErrTransitionConflict) || errors.Is(resumeErr, looppkg.ErrInvalidTransition) {
			completed = waitDueCursorForCell(cell)
			continue
		}
		if resumeErr != nil {
			return runs, completed, resumeErr
		}
		if result.Won && result.Coordinator != nil {
			runs = append(runs, *result.Coordinator)
		}
		completed = waitDueCursorForCell(cell)
	}
	return runs, next, nil
}

func (g *LoopRepo) waitDueAdmissionAttempts(
	ctx context.Context,
	cell waitDueCell,
	definitions map[looppkg.RunID]*looppkg.ResolvedDefinition,
) (int, error) {
	resolved := definitions[cell.runID]
	if resolved == nil {
		run, err := getLoopRunByIDWithExecutor(ctx, g.db, cell.runID)
		if err != nil {
			return 0, err
		}
		resolved, err = loadWaitExecutedDefinition(ctx, g.db, run)
		if err != nil {
			return 0, err
		}
		definitions[cell.runID] = resolved
	}
	node, found := loopWaitNodeByRuntimeID(resolved.Definition.Graph, "", string(cell.nodeID))
	if !found {
		return 0, fmt.Errorf(
			"%w: due Loop wait node %q is absent from the executed definition",
			looppkg.ErrValidation,
			cell.nodeID,
		)
	}
	lifecycle, err := looppkg.ResolveNodeLifecycleConfig(node, nil, resolved.EffectiveConfig.Lifecycle)
	if err != nil {
		return 0, fmt.Errorf("store: resolve due Loop wait lifecycle: %w", err)
	}
	return lifecycle.WaitAdmissionAttempts, nil
}

func waitDueCursorForCell(cell waitDueCell) looppkg.WaitDueCursor {
	return looppkg.WaitDueCursor{
		ResumeAt: cell.resumeAt, LoopRunID: cell.runID, Generation: cell.generation,
		NodeID: cell.nodeID, ItemIndex: cell.itemIndex,
	}
}

func (g *LoopRepo) listDueLoopWaits(
	ctx context.Context,
	now time.Time,
	cursor looppkg.WaitDueCursor,
	limit int,
) (returnCells []waitDueCell, returnCursor looppkg.WaitDueCursor, returnErr error) {
	if err := g.checkReady(ctx, "list due Loop waits"); err != nil {
		return nil, looppkg.WaitDueCursor{}, err
	}
	if limit <= 0 || limit > 1000 {
		return nil, looppkg.WaitDueCursor{}, fmt.Errorf(
			"%w: wait due page limit must be between 1 and 1000", looppkg.ErrValidation,
		)
	}
	rows, err := g.db.QueryContext(ctx, `SELECT run.workspace_id, wait.loop_run_id, wait.generation,
		wait.node_id, wait.item_index, wait.resume_at FROM loop_node_waits AS wait
		JOIN loop_runs AS run ON run.id = wait.loop_run_id
		JOIN loop_generation_outputs AS output ON output.loop_run_id = wait.loop_run_id
		 AND output.generation = wait.generation AND output.node_id = wait.node_id
		 AND output.item_index = wait.item_index
		LEFT JOIN loop_node_controls AS control ON control.loop_run_id = wait.loop_run_id
		 AND control.node_id = wait.node_id
		WHERE wait.kind = 'timer' AND wait.claim_state = 'waiting' AND wait.resume_at IS NOT NULL
		 AND wait.resume_at <= ? AND output.status = 'waiting' AND output.epoch = wait.issued_epoch
		 AND run.status NOT IN ('paused','done','no-op','blocked','failed','exhausted','stalled','canceled')
		 AND COALESCE(control.paused, 0) = 0 AND COALESCE(control.quarantined, 0) = 0
		 AND COALESCE(control.cancel_state, '') = ''
		 AND (? IS NULL OR wait.resume_at > ? OR (wait.resume_at = ? AND
		   (wait.loop_run_id, wait.generation, wait.node_id, wait.item_index) > (?, ?, ?, ?)))
		ORDER BY wait.resume_at, wait.loop_run_id, wait.generation, wait.node_id, wait.item_index LIMIT ?`,
		now.UTC(), waitCursorTime(cursor), waitCursorTime(cursor), waitCursorTime(cursor),
		cursor.LoopRunID, cursor.Generation, cursor.NodeID, cursor.ItemIndex, limit)
	if err != nil {
		return nil, looppkg.WaitDueCursor{}, fmt.Errorf("store: query due Loop waits: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("store: close due Loop wait rows: %w", closeErr))
		}
	}()
	cells := make([]waitDueCell, 0, limit)
	for rows.Next() {
		var cell waitDueCell
		if err := rows.Scan(&cell.workspaceID, &cell.runID, &cell.generation, &cell.nodeID,
			&cell.itemIndex, &cell.resumeAt); err != nil {
			return nil, looppkg.WaitDueCursor{}, fmt.Errorf("store: scan due Loop wait: %w", err)
		}
		cell.resumeAt = cell.resumeAt.UTC()
		cells = append(cells, cell)
	}
	if err := rows.Err(); err != nil {
		return nil, looppkg.WaitDueCursor{}, fmt.Errorf("store: iterate due Loop waits: %w", err)
	}
	if len(cells) < limit {
		return cells, looppkg.WaitDueCursor{}, nil
	}
	last := cells[len(cells)-1]
	return cells, looppkg.WaitDueCursor{
		ResumeAt: last.resumeAt, LoopRunID: last.runID, Generation: last.generation,
		NodeID: last.nodeID, ItemIndex: last.itemIndex,
	}, nil
}

func waitCursorTime(cursor looppkg.WaitDueCursor) any {
	if cursor.Empty() {
		return nil
	}
	return cursor.ResumeAt.UTC()
}
