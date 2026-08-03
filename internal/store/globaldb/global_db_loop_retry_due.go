package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// ListDueLoopRetries returns one stable page of live, unparked retry schedules.
func (g *LoopRepo) ListDueLoopRetries(
	ctx context.Context,
	now time.Time,
	cursor looppkg.RetryDueCursor,
	limit int,
) (returnCells []looppkg.RetryDueCell, returnCursor looppkg.RetryDueCursor, returnErr error) {
	rows, err := g.queryDueLoopRetryRows(ctx, now, cursor, limit)
	if err != nil {
		return nil, looppkg.RetryDueCursor{}, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("store: close due loop retry rows: %w", closeErr))
		}
	}()
	cells, err := scanDueLoopRetryRows(rows, limit)
	if err != nil {
		return nil, looppkg.RetryDueCursor{}, err
	}
	if len(cells) < limit {
		return cells, looppkg.RetryDueCursor{}, nil
	}
	last := cells[len(cells)-1]
	return cells, looppkg.RetryDueCursor{
		NextAttemptAt: last.NextAttemptAt,
		LoopRunID:     last.LoopRunID,
		Generation:    last.Generation,
		NodeID:        last.NodeID,
		ItemIndex:     last.ItemIndex,
	}, nil
}

func (g *LoopRepo) queryDueLoopRetryRows(
	ctx context.Context,
	now time.Time,
	cursor looppkg.RetryDueCursor,
	limit int,
) (*sql.Rows, error) {
	if err := g.checkReady(ctx, "list due loop retries"); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 1000 {
		return nil, fmt.Errorf(
			"%w: retry due page limit must be between 1 and 1000",
			looppkg.ErrValidation,
		)
	}
	rows, err := g.db.QueryContext(
		ctx,
		`SELECT run.workspace_id, output.loop_run_id, output.generation, output.node_id,
		        output.item_index, output.attempt, output.epoch, output.next_attempt_at
		 FROM loop_generation_outputs AS output
		 JOIN loop_runs AS run ON run.id = output.loop_run_id
		 LEFT JOIN loop_node_controls AS control
		   ON control.loop_run_id = output.loop_run_id AND control.node_id = output.node_id
		 WHERE output.status = 'retrying'
		   AND output.next_attempt_at IS NOT NULL
		   AND output.next_attempt_at <= ?
		   AND run.status NOT IN ('paused','done','no-op','blocked','failed','exhausted','stalled','canceled')
		   AND COALESCE(control.paused, 0) = 0
		   AND COALESCE(control.quarantined, 0) = 0
		   AND COALESCE(control.cancel_state, '') = ''
		   AND (
		     ? IS NULL OR output.next_attempt_at > ? OR
		     (output.next_attempt_at = ? AND
		       (output.loop_run_id, output.generation, output.node_id, output.item_index) > (?, ?, ?, ?))
		   )
		 ORDER BY output.next_attempt_at ASC, output.loop_run_id ASC, output.generation ASC,
		          output.node_id ASC, output.item_index ASC
		 LIMIT ?`,
		now.UTC(), cursorTime(cursor), cursorTime(cursor), cursorTime(cursor),
		string(cursor.LoopRunID), cursor.Generation, string(cursor.NodeID), cursor.ItemIndex, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("store: query due loop retries: %w", err)
	}
	return rows, nil
}

func scanDueLoopRetryRows(rows *sql.Rows, limit int) ([]looppkg.RetryDueCell, error) {
	cells := make([]looppkg.RetryDueCell, 0, limit)
	for rows.Next() {
		var cell looppkg.RetryDueCell
		if err := rows.Scan(
			&cell.WorkspaceID, &cell.LoopRunID, &cell.Generation, &cell.NodeID,
			&cell.ItemIndex, &cell.Attempt, &cell.Epoch, &cell.NextAttemptAt,
		); err != nil {
			return nil, fmt.Errorf("store: scan due loop retry: %w", err)
		}
		cell.NextAttemptAt = cell.NextAttemptAt.UTC()
		cells = append(cells, cell)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate due loop retries: %w", err)
	}
	return cells, nil
}

// EnqueueDueLoopRetryWakesPage reserves coordinator wakes for one due-time page.
func (g *LoopRepo) EnqueueDueLoopRetryWakesPage(
	ctx context.Context,
	origin taskpkg.Origin,
	now time.Time,
	cursor looppkg.RetryDueCursor,
	limit int,
) ([]taskpkg.Run, looppkg.RetryDueCursor, error) {
	cells, next, err := g.ListDueLoopRetries(ctx, now, cursor, limit)
	if err != nil {
		return nil, looppkg.RetryDueCursor{}, err
	}
	runs := make([]taskpkg.Run, 0, len(cells))
	for _, cell := range cells {
		run, added, _, enqueueErr := g.EnqueueLoopRetryWakeIfCurrent(ctx, origin, cell, now)
		if enqueueErr != nil {
			return runs, next, enqueueErr
		}
		if added {
			runs = append(runs, run)
		}
	}
	return runs, next, nil
}

// EnqueueLoopRetryWakeIfCurrent fences a timer or due-scan fire against the
// exact retry cell and reserves its coordinator wake in the same transaction.
func (g *LoopRepo) EnqueueLoopRetryWakeIfCurrent(
	ctx context.Context,
	origin taskpkg.Origin,
	cell looppkg.RetryDueCell,
	firedAt time.Time,
) (returnRun taskpkg.Run, returnAdded bool, returnCurrent bool, returnErr error) {
	if err := g.checkReady(ctx, "enqueue loop retry wake"); err != nil {
		return taskpkg.Run{}, false, false, err
	}
	if err := validateRetryDueCell(cell); err != nil {
		return taskpkg.Run{}, false, false, err
	}
	normalizedOrigin, err := normalizeLoopCoordinatorReconcileOrigin(origin)
	if err != nil {
		return taskpkg.Run{}, false, false, err
	}
	firedAt = g.normalizeLoopCoordinatorReconcileTime(firedAt)
	err = g.withTaskImmediateTransaction(ctx, "enqueue loop retry wake", func(exec taskSQLExecutor) error {
		state, stateErr := readLoopRetryScheduleState(ctx, exec, cell)
		if stateErr != nil {
			return stateErr
		}
		if !state.current(cell, firedAt) {
			if err := appendLoopRunEventWithExecutor(
				ctx,
				exec,
				cell.LoopRunID,
				state.workspaceID,
				loopRunEventStaleScheduleDropped,
				staleRetrySchedulePayload(cell, state),
				firedAt,
			); err != nil {
				return err
			}
			return nil
		}
		current, err := getLoopRunByIDWithExecutor(ctx, exec, cell.LoopRunID)
		if err != nil {
			return err
		}
		taskID, err := lastCoordinatorTaskIDForLoopRun(ctx, exec, string(cell.LoopRunID))
		if errors.Is(err, sql.ErrNoRows) {
			taskID, err = g.ensureLoopCoordinatorTaskWithExecutor(ctx, exec, current, firedAt)
		}
		if err != nil {
			return err
		}
		returnRun, returnAdded, err = g.reserveCoordinatorRun(
			ctx,
			exec,
			taskID,
			string(cell.LoopRunID),
			"",
			looppkg.RetryWakeIdempotencyKey(cell),
			normalizedOrigin,
			firedAt,
		)
		if errors.Is(err, taskpkg.ErrInvalidStatusTransition) {
			returnCurrent = true
			return nil
		}
		if err == nil {
			returnCurrent = true
		}
		return err
	})
	return returnRun, returnAdded, returnCurrent, err
}

type loopRetryScheduleState struct {
	workspaceID  looppkg.WorkspaceID
	runStatus    looppkg.Status
	outputStatus sql.NullString
	attempt      sql.NullInt64
	epoch        sql.NullInt64
	nextAt       sql.NullTime
	paused       bool
	quarantined  bool
	cancelState  string
}

func readLoopRetryScheduleState(
	ctx context.Context,
	exec taskSQLExecutor,
	cell looppkg.RetryDueCell,
) (loopRetryScheduleState, error) {
	var state loopRetryScheduleState
	var paused int64
	var quarantined int64
	err := exec.QueryRowContext(ctx, `
		SELECT run.workspace_id, run.status, output.status, output.attempt, output.epoch,
		       output.next_attempt_at, COALESCE(control.paused, 0),
		       COALESCE(control.quarantined, 0), COALESCE(control.cancel_state, '')
		FROM loop_runs AS run
		LEFT JOIN loop_generation_outputs AS output
		  ON output.loop_run_id = run.id AND output.generation = ? AND output.node_id = ?
		 AND output.item_index = ?
		LEFT JOIN loop_node_controls AS control
		  ON control.loop_run_id = run.id AND control.node_id = ?
		WHERE run.id = ?`,
		cell.Generation, cell.NodeID, cell.ItemIndex, cell.NodeID, cell.LoopRunID,
	).Scan(
		&state.workspaceID, &state.runStatus, &state.outputStatus, &state.attempt, &state.epoch,
		&state.nextAt, &paused, &quarantined, &state.cancelState,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return loopRetryScheduleState{}, fmt.Errorf("%w: loop run %q", looppkg.ErrRunNotFound, cell.LoopRunID)
	}
	if err != nil {
		return loopRetryScheduleState{}, fmt.Errorf("store: read loop retry schedule state: %w", err)
	}
	state.paused = paused != 0
	state.quarantined = quarantined != 0
	state.cancelState = strings.TrimSpace(state.cancelState)
	return state, nil
}

func (s loopRetryScheduleState) current(cell looppkg.RetryDueCell, firedAt time.Time) bool {
	return s.runStatus.Live() && s.runStatus != looppkg.StatusPaused &&
		s.outputStatus.Valid && s.outputStatus.String == loopGenerationOutputRetrying &&
		s.attempt.Valid && int(s.attempt.Int64) == cell.Attempt &&
		s.epoch.Valid && s.epoch.Int64 == cell.Epoch && s.nextAt.Valid &&
		!s.nextAt.Time.UTC().After(firedAt.UTC()) && !s.paused && !s.quarantined && s.cancelState == ""
}

func validateRetryDueCell(cell looppkg.RetryDueCell) error {
	if cell.LoopRunID == "" || cell.Generation < 1 || cell.NodeID == "" || cell.ItemIndex < 0 ||
		cell.Attempt < 1 || cell.Epoch < 1 || cell.NextAttemptAt.IsZero() {
		return fmt.Errorf("%w: retry due cell has incomplete identity", looppkg.ErrValidation)
	}
	return nil
}

func staleRetrySchedulePayload(
	cell looppkg.RetryDueCell,
	state loopRetryScheduleState,
) map[string]any {
	var currentEpoch any
	if state.epoch.Valid {
		currentEpoch = state.epoch.Int64
	}
	return map[string]any{
		loopRunEventPayloadKeyGeneration:   cell.Generation,
		loopRunEventPayloadKeyNodeID:       cell.NodeID,
		loopRunEventPayloadKeyItemIndex:    cell.ItemIndex,
		watchEventsPayloadAttemptKey:       cell.Attempt,
		loopRunEventPayloadKeyIssuedEpoch:  cell.Epoch,
		loopRunEventPayloadKeyCurrentEpoch: currentEpoch,
		loopRunEventPayloadKeyScheduleKind: loopRetryScheduleKind,
	}
}

func cursorTime(cursor looppkg.RetryDueCursor) any {
	if cursor.Empty() {
		return nil
	}
	return cursor.NextAttemptAt.UTC()
}
