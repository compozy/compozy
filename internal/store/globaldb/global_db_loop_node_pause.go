package globaldb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ looppkg.NodePauseStore = (*LoopRepo)(nil)

const (
	loopNextAttemptAtColumn = "next_attempt_at"
	loopPendingSQLLiteral   = "'pending'"
	sqlNullLiteral          = "NULL"
)

// PauseNode parks one authored node and fences every schedule it supersedes.
func (g *LoopRepo) PauseNode(
	ctx context.Context,
	mutation looppkg.NodePauseMutation,
) (looppkg.NodePauseResult, error) {
	if err := g.checkReady(ctx, "pause Loop node"); err != nil {
		return looppkg.NodePauseResult{}, err
	}
	if err := mutation.Validate(); err != nil {
		return looppkg.NodePauseResult{}, err
	}
	var result looppkg.NodePauseResult
	err := g.withTaskImmediateTransaction(ctx, "pause Loop node", func(exec taskSQLExecutor) error {
		return g.pauseNodeWithExecutor(ctx, exec, mutation, &result)
	})
	if err != nil {
		return looppkg.NodePauseResult{}, err
	}
	return result, nil
}

func (g *LoopRepo) pauseNodeWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.NodePauseMutation,
	result *looppkg.NodePauseResult,
) error {
	run, err := nodePauseRun(ctx, exec, mutation.WorkspaceID, mutation.RunID)
	if err != nil {
		return err
	}
	if mutation.ItemIndex != nil {
		return g.pauseNodeLane(ctx, exec, run, mutation, result)
	}
	control, found, err := loadNodePauseControl(ctx, exec, mutation.WorkspaceID, mutation.RunID, mutation.NodeID)
	if err != nil {
		return err
	}
	if mutation.ExpectedRevision != nil && (!found || control.Revision != *mutation.ExpectedRevision) {
		actualState := nodeLifecycleActualState(control, found)
		return loopLifecycleReasonError(
			looppkg.ReasonCodeAlreadyDecided,
			fmt.Errorf("%w: node %q revision changed", looppkg.ErrTransitionConflict, mutation.NodeID),
			actualState,
			nodeLifecycleAllowedTransitions(actualState),
			nodeLifecycleWinner(control),
		)
	}
	if found && control.Paused {
		result.Control = control
		return g.collectPauseCancellationSessions(ctx, exec, mutation, result)
	}
	if err := g.collectPauseCancellationSessions(ctx, exec, mutation, result); err != nil {
		return err
	}
	nextRevision := int64(1)
	if found {
		nextRevision = control.Revision + 1
	}
	if err := writeNodePauseControl(ctx, exec, mutation, found, control.Revision); err != nil {
		return err
	}
	if err := parkNodeSchedules(ctx, exec, mutation); err != nil {
		return err
	}
	if err := appendLoopRunEventWithExecutor(
		ctx,
		exec,
		run.ID,
		run.WorkspaceID,
		loopRunEventNodePaused,
		nodePauseEventPayload(mutation, nextRevision),
		mutation.RequestedAt,
	); err != nil {
		return err
	}
	if err := appendManualNodePauseEffects(ctx, exec, run, mutation, nextRevision); err != nil {
		return err
	}
	result.Control = pausedNodeControl(mutation, nextRevision)
	result.Applied = true
	return nil
}

func (g *LoopRepo) collectPauseCancellationSessions(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.NodePauseMutation,
	result *looppkg.NodePauseResult,
) error {
	if mutation.Mode != looppkg.NodePauseCancel {
		return nil
	}
	sessions, err := listNodeCancellationSessions(ctx, exec, looppkg.CancellationMutation{
		WorkspaceID: mutation.WorkspaceID,
		RunID:       mutation.RunID,
		NodeID:      mutation.NodeID,
		RequestedAt: mutation.RequestedAt,
	})
	if err != nil {
		return err
	}
	result.SessionIDs = sessions
	return nil
}

// ResumeNode releases one paused node and reserves its coordinator wake atomically.
func (g *LoopRepo) ResumeNode(
	ctx context.Context,
	mutation looppkg.NodeResumeMutation,
) (looppkg.NodeResumeResult, error) {
	if err := g.checkReady(ctx, "resume Loop node"); err != nil {
		return looppkg.NodeResumeResult{}, err
	}
	if err := mutation.Validate(); err != nil {
		return looppkg.NodeResumeResult{}, err
	}
	var result looppkg.NodeResumeResult
	err := g.withTaskImmediateTransaction(ctx, "resume Loop node", func(exec taskSQLExecutor) error {
		run, err := nodePauseRun(ctx, exec, mutation.WorkspaceID, mutation.RunID)
		if err != nil {
			return err
		}
		if mutation.ItemIndex != nil {
			return g.resumeNodeLane(ctx, exec, run, mutation, &result)
		}
		control, found, err := loadNodePauseControl(
			ctx, exec, mutation.WorkspaceID, mutation.RunID, mutation.NodeID,
		)
		if err != nil {
			return err
		}
		if mutation.ExpectedRevision != nil && (!found || control.Revision != *mutation.ExpectedRevision) {
			actualState := nodeLifecycleActualState(control, found)
			return loopLifecycleReasonError(
				looppkg.ReasonCodeAlreadyDecided,
				fmt.Errorf("%w: node %q revision changed", looppkg.ErrTransitionConflict, mutation.NodeID),
				actualState,
				nodeLifecycleAllowedTransitions(actualState),
				nodeLifecycleWinner(control),
			)
		}
		if !found || !control.Paused {
			actualState := nodeLifecycleActualState(control, found)
			return loopLifecycleReasonError(
				looppkg.ReasonCodeNodeNotPaused,
				fmt.Errorf("%w: node %q is not paused", looppkg.ErrInvalidTransition, mutation.NodeID),
				actualState,
				nodeLifecycleAllowedTransitions(actualState),
				nodeLifecycleWinner(control),
			)
		}
		if err := releaseNodePause(ctx, exec, mutation, control); err != nil {
			return err
		}
		if err := restorePausedNodeCells(ctx, exec, mutation, control); err != nil {
			return err
		}
		parkedFor := nodePauseDuration(control, mutation.RequestedAt)
		if err := shiftLoopWallClockIfUnparked(ctx, exec, mutation.RunID, parkedFor); err != nil {
			return err
		}
		nextRevision := control.Revision + 1
		if err := appendLoopRunEventWithExecutor(ctx, exec, run.ID, run.WorkspaceID, loopRunEventNodeResumed,
			nodeResumeEventPayload(mutation, nextRevision), mutation.RequestedAt); err != nil {
			return err
		}
		coordinator, _, err := g.reserveLoopCoordinatorRunWithExecutor(
			ctx, exec, run, mutation.Actor.Origin, mutation.RequestedAt, "",
			fmt.Sprintf("loop.node.resume.%s.%s.%d", mutation.RunID, mutation.NodeID, nextRevision),
		)
		if err != nil {
			return err
		}
		control.Paused = false
		control.Revision = nextRevision
		control.UpdatedAt = mutation.RequestedAt.UTC()
		result = looppkg.NodeResumeResult{Control: control, Coordinator: &coordinator, Applied: true}
		return nil
	})
	if err != nil {
		return looppkg.NodeResumeResult{}, err
	}
	return result, nil
}

func nodePauseRun(
	ctx context.Context,
	exec taskSQLExecutor,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
) (looppkg.Run, error) {
	run, err := getLoopRunByIDWithExecutor(ctx, exec, runID)
	if err != nil {
		return looppkg.Run{}, err
	}
	if run.WorkspaceID != workspaceID {
		return looppkg.Run{}, fmt.Errorf("%w: loop run %q not found", looppkg.ErrRunNotFound, runID)
	}
	if run.Status.Terminal() {
		return looppkg.Run{}, loopLifecycleReasonError(
			looppkg.ReasonCodeRunTerminal,
			fmt.Errorf("%w: loop run %q cannot change node pause state", looppkg.ErrInvalidTransition, runID),
			string(run.Status),
			[]string{},
			runLifecycleWinner(run),
		)
	}
	return run, nil
}

func loadNodePauseControl(
	ctx context.Context,
	exec taskSQLExecutor,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
	nodeID looppkg.NodeID,
) (looppkg.NodeControl, bool, error) {
	rows, err := sqlcgen.New(exec).ListLoopNodeControls(ctx, sqlcgen.ListLoopNodeControlsParams{
		WorkspaceID: string(workspaceID),
		LoopRunID:   string(runID),
	})
	if err != nil {
		return looppkg.NodeControl{}, false, fmt.Errorf("store: load Loop node pause: %w", err)
	}
	for _, row := range rows {
		if looppkg.NodeID(row.NodeID) != nodeID {
			continue
		}
		control, mapErr := loopNodeControlFromGenerated(row)
		return control, true, mapErr
	}
	return looppkg.NodeControl{LoopRunID: runID, NodeID: nodeID}, false, nil
}

func writeNodePauseControl(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.NodePauseMutation,
	found bool,
	currentRevision int64,
) error {
	if !found {
		_, err := exec.ExecContext(ctx, `INSERT INTO loop_node_controls (
			loop_run_id, node_id, paused, pause_actor_kind, pause_actor_id, pause_reason,
			pause_rule_id, pause_requested_at, revision, updated_at
		) VALUES (?, ?, 1, ?, ?, ?, ?, ?, 1, ?)`, mutation.RunID, mutation.NodeID,
			mutation.Actor.Actor.Kind, mutation.Actor.Actor.Ref, diagnostics.RedactAndBound(mutation.Reason, 1024),
			nullableNodePauseString(mutation.RuleID), mutation.RequestedAt.UTC(), mutation.RequestedAt.UTC())
		if err != nil {
			return fmt.Errorf("store: insert Loop node pause: %w", err)
		}
		return nil
	}
	result, err := exec.ExecContext(ctx, `UPDATE loop_node_controls SET paused = 1,
		pause_actor_kind = ?, pause_actor_id = ?, pause_reason = ?, pause_rule_id = ?,
		pause_requested_at = ?, revision = revision + 1, updated_at = ?
		WHERE loop_run_id = ? AND node_id = ? AND paused = 0 AND revision = ?`,
		mutation.Actor.Actor.Kind, mutation.Actor.Actor.Ref, diagnostics.RedactAndBound(mutation.Reason, 1024),
		nullableNodePauseString(mutation.RuleID), mutation.RequestedAt.UTC(), mutation.RequestedAt.UTC(),
		mutation.RunID, mutation.NodeID, currentRevision)
	if err != nil {
		return fmt.Errorf("store: update Loop node pause: %w", err)
	}
	return requireSinglePauseMutation(result, mutation.NodeID)
}

func parkNodeSchedules(ctx context.Context, exec taskSQLExecutor, mutation looppkg.NodePauseMutation) error {
	statuses := "'pending','enqueued','retrying'"
	if mutation.Mode == looppkg.NodePauseCancel {
		statuses += ",'running','control_pending','awaiting_goal'"
	}
	_, err := exec.ExecContext(ctx, `UPDATE loop_generation_outputs SET status = 'paused', epoch = epoch + 1
		WHERE loop_run_id = ? AND generation = (SELECT generation FROM loop_runs WHERE id = ?)
		AND node_id = ? AND status IN (`+statuses+`)`, mutation.RunID, mutation.RunID, mutation.NodeID)
	if err != nil {
		return fmt.Errorf("store: park Loop node schedules: %w", err)
	}
	return nil
}

func releaseNodePause(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.NodeResumeMutation,
	control looppkg.NodeControl,
) error {
	result, err := exec.ExecContext(ctx, `UPDATE loop_node_controls SET paused = 0,
		revision = revision + 1, updated_at = ? WHERE loop_run_id = ? AND node_id = ?
		AND paused = 1 AND revision = ?`, mutation.RequestedAt.UTC(), mutation.RunID, mutation.NodeID, control.Revision)
	if err != nil {
		return fmt.Errorf("store: release Loop node pause: %w", err)
	}
	return requireSinglePauseMutation(result, mutation.NodeID)
}

func restorePausedNodeCells(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.NodeResumeMutation,
	control looppkg.NodeControl,
) error {
	pauseDuration := nodePauseDuration(control, mutation.RequestedAt)
	if err := shiftPausedNodeFirstScheduledAnchors(
		ctx, exec, mutation.RunID, mutation.NodeID, pauseDuration,
	); err != nil {
		return err
	}
	statusExpression := "CASE WHEN next_attempt_at IS NOT NULL AND next_attempt_at > ? THEN 'retrying' ELSE 'pending' END"
	attemptExpression := watchEventsPayloadAttemptKey
	nextAttemptExpression := loopNextAttemptAtColumn
	if mutation.Mode == looppkg.NodeResumeImmediate {
		statusExpression = loopPendingSQLLiteral
		nextAttemptExpression = sqlNullLiteral
	}
	if mutation.Mode == looppkg.NodeResumeResetAttempts {
		statusExpression = loopPendingSQLLiteral
		attemptExpression = "1"
		nextAttemptExpression = sqlNullLiteral
	}
	args := make([]any, 0, 5)
	if mutation.Mode == looppkg.NodeResumePlain {
		args = append(args, mutation.RequestedAt.UTC(), mutation.RequestedAt.UTC())
	}
	args = append(args, mutation.RunID, mutation.RunID, mutation.NodeID)
	_, err := exec.ExecContext(ctx, `UPDATE loop_generation_outputs SET status = `+statusExpression+`,
		attempt = `+attemptExpression+`, next_attempt_at = `+nextAttemptExpression+`, task_run_id = NULL,
		output_ref = CASE WHEN `+statusExpression+` = 'pending' THEN NULL ELSE output_ref END,
		epoch = epoch + 1 WHERE loop_run_id = ?
		AND generation = (SELECT generation FROM loop_runs WHERE id = ?)
		AND node_id = ? AND status = 'paused'`, args...)
	if err != nil {
		return fmt.Errorf("store: restore paused Loop node cells: %w", err)
	}
	return nil
}

func nodePauseDuration(control looppkg.NodeControl, resumedAt time.Time) time.Duration {
	if control.PauseProvenance == nil || control.PauseProvenance.RequestedAt.IsZero() {
		return 0
	}
	duration := resumedAt.UTC().Sub(control.PauseProvenance.RequestedAt.UTC())
	if duration < 0 {
		return 0
	}
	return duration
}

func nodePauseEventPayload(m looppkg.NodePauseMutation, revision int64) map[string]any {
	payload := map[string]any{
		loopRunEventPayloadKeyNodeID:    m.NodeID,
		loopRunEventPayloadKeyActorKind: m.Actor.Actor.Kind,
		loopRunEventPayloadKeyActorID:   m.Actor.Actor.Ref,
		loopRunEventPayloadKeyReason:    diagnostics.RedactAndBound(m.Reason, 1024),
		loopRunEventPayloadKeyMode:      m.Mode,
		"rule_id":                       strings.TrimSpace(m.RuleID),
		"revision":                      revision,
	}
	if m.ItemIndex != nil {
		payload[loopRunEventPayloadKeyItemIndex] = *m.ItemIndex
	}
	return payload
}

func nodeResumeEventPayload(m looppkg.NodeResumeMutation, revision int64) map[string]any {
	payload := map[string]any{
		loopRunEventPayloadKeyNodeID: m.NodeID, loopRunEventPayloadKeyActorKind: m.Actor.Actor.Kind,
		loopRunEventPayloadKeyActorID: m.Actor.Actor.Ref, loopRunEventPayloadKeyMode: m.Mode, "revision": revision,
	}
	if m.ItemIndex != nil {
		payload[loopRunEventPayloadKeyItemIndex] = *m.ItemIndex
	}
	return payload
}

func pausedNodeControl(m looppkg.NodePauseMutation, revision int64) looppkg.NodeControl {
	return looppkg.NodeControl{
		LoopRunID: m.RunID, NodeID: m.NodeID, Paused: true, Revision: revision, UpdatedAt: m.RequestedAt.UTC(),
		PauseProvenance: &looppkg.ControlProvenance{
			ActorKind: string(m.Actor.Actor.Kind),
			ActorID:   m.Actor.Actor.Ref,
			Reason:    diagnostics.RedactAndBound(m.Reason, 1024),
			RuleID:    strings.TrimSpace(m.RuleID), RequestedAt: m.RequestedAt.UTC(),
		},
	}
}

func requireSinglePauseMutation(result sql.Result, nodeID looppkg.NodeID) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: inspect Loop node pause mutation: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("%w: node %q pause state changed", looppkg.ErrTransitionConflict, nodeID)
	}
	return nil
}

func nullableNodePauseString(value string) sql.NullString {
	trimmed := strings.TrimSpace(value)
	return sql.NullString{String: trimmed, Valid: trimmed != ""}
}
