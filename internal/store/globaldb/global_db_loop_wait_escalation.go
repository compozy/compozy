package globaldb

import (
	"context"
	"errors"
	"fmt"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
)

var _ looppkg.WaitEscalationStore = (*LoopRepo)(nil)

type waitEscalationCell struct {
	nextEscalationAt time.Time
	runID            looppkg.RunID
	generation       int
	nodeID           looppkg.NodeID
	itemIndex        int
}

// EscalateDueLoopWaitsPage advances one stable page of authored wait ladders.
func (g *LoopRepo) EscalateDueLoopWaitsPage(
	ctx context.Context,
	now time.Time,
	cursor looppkg.WaitEscalationCursor,
	limit int,
) (looppkg.WaitEscalationCursor, error) {
	cells, next, err := g.listDueLoopWaitEscalations(ctx, now, cursor, limit)
	if err != nil {
		return looppkg.WaitEscalationCursor{}, err
	}
	completed := cursor
	for _, cell := range cells {
		if err := g.escalateDueLoopWait(ctx, now.UTC(), cell); err != nil {
			if errors.Is(err, looppkg.ErrTransitionConflict) || errors.Is(err, looppkg.ErrInvalidTransition) {
				completed = waitEscalationCursorForCell(cell)
				continue
			}
			return completed, err
		}
		completed = waitEscalationCursorForCell(cell)
	}
	return next, nil
}

func waitEscalationCursorForCell(cell waitEscalationCell) looppkg.WaitEscalationCursor {
	return looppkg.WaitEscalationCursor{
		NextEscalationAt: cell.nextEscalationAt, LoopRunID: cell.runID,
		Generation: cell.generation, NodeID: cell.nodeID, ItemIndex: cell.itemIndex,
	}
}

func (g *LoopRepo) listDueLoopWaitEscalations(
	ctx context.Context,
	now time.Time,
	cursor looppkg.WaitEscalationCursor,
	limit int,
) (returnCells []waitEscalationCell, returnCursor looppkg.WaitEscalationCursor, returnErr error) {
	if err := g.checkReady(ctx, "list due Loop wait escalations"); err != nil {
		return nil, looppkg.WaitEscalationCursor{}, err
	}
	if limit <= 0 || limit > 1000 {
		return nil, looppkg.WaitEscalationCursor{}, fmt.Errorf(
			"%w: wait escalation page limit must be between 1 and 1000", looppkg.ErrValidation,
		)
	}
	rows, err := g.db.QueryContext(ctx, `SELECT wait.next_escalation_at, wait.loop_run_id,
		wait.generation, wait.node_id, wait.item_index
		FROM loop_node_waits AS wait
		JOIN loop_runs AS run ON run.id = wait.loop_run_id
		JOIN loop_generation_outputs AS output ON output.loop_run_id = wait.loop_run_id
		 AND output.generation = wait.generation AND output.node_id = wait.node_id
		 AND output.item_index = wait.item_index
		LEFT JOIN loop_node_controls AS control ON control.loop_run_id = wait.loop_run_id
		 AND control.node_id = wait.node_id
		WHERE wait.claim_state = 'waiting' AND wait.next_escalation_at IS NOT NULL
		 AND wait.next_escalation_at <= ?
		 AND ((wait.kind IN ('timer','event','request') AND output.status = 'waiting')
		   OR (wait.kind = 'approval_escalation' AND output.status = 'succeeded'
		     AND run.status = 'needs-approval' AND run.active_gate_id = wait.node_id))
		 AND output.epoch = wait.issued_epoch
		 AND run.status NOT IN ('paused','done','no-op','blocked','failed','exhausted','stalled','canceled')
		 AND COALESCE(control.paused, 0) = 0 AND COALESCE(control.quarantined, 0) = 0
		 AND COALESCE(control.cancel_state, '') = ''
		 AND (? IS NULL OR wait.next_escalation_at > ? OR (wait.next_escalation_at = ? AND
		   (wait.loop_run_id, wait.generation, wait.node_id, wait.item_index) > (?, ?, ?, ?)))
		ORDER BY wait.next_escalation_at, wait.loop_run_id, wait.generation, wait.node_id,
		 wait.item_index LIMIT ?`, now.UTC(), waitEscalationCursorTime(cursor),
		waitEscalationCursorTime(cursor), waitEscalationCursorTime(cursor), cursor.LoopRunID,
		cursor.Generation, cursor.NodeID, cursor.ItemIndex, limit)
	if err != nil {
		return nil, looppkg.WaitEscalationCursor{}, fmt.Errorf("store: query due Loop wait escalations: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("store: close due Loop wait escalations: %w", closeErr))
		}
	}()
	cells := make([]waitEscalationCell, 0, limit)
	for rows.Next() {
		var cell waitEscalationCell
		if err := rows.Scan(&cell.nextEscalationAt, &cell.runID, &cell.generation, &cell.nodeID,
			&cell.itemIndex); err != nil {
			return nil, looppkg.WaitEscalationCursor{}, fmt.Errorf("store: scan due Loop wait escalation: %w", err)
		}
		cell.nextEscalationAt = cell.nextEscalationAt.UTC()
		cells = append(cells, cell)
	}
	if err := rows.Err(); err != nil {
		return nil, looppkg.WaitEscalationCursor{}, fmt.Errorf("store: iterate due Loop wait escalations: %w", err)
	}
	if len(cells) < limit {
		return cells, looppkg.WaitEscalationCursor{}, nil
	}
	last := cells[len(cells)-1]
	return cells, looppkg.WaitEscalationCursor{
		NextEscalationAt: last.nextEscalationAt, LoopRunID: last.runID,
		Generation: last.generation, NodeID: last.nodeID, ItemIndex: last.itemIndex,
	}, nil
}

func waitEscalationCursorTime(cursor looppkg.WaitEscalationCursor) any {
	if cursor.Empty() {
		return nil
	}
	return cursor.NextEscalationAt.UTC()
}
