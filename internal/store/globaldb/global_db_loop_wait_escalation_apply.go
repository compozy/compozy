package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
)

const (
	expiredWaitAttentionFlag   = "expired_wait"
	expiredWaitAttentionReason = "wait expired without a timeout route"
)

type dueWaitEscalation struct {
	wait     looppkg.NodeWait
	run      looppkg.Run
	node     dsl.Node
	expiry   *dsl.WaitExpiry
	interval time.Duration
	sourceID string
}

func (g *LoopRepo) escalateDueLoopWait(
	ctx context.Context,
	now time.Time,
	cell waitEscalationCell,
) error {
	return g.withTaskImmediateTransaction(ctx, "escalate due Loop wait", func(exec taskSQLExecutor) error {
		due, err := loadDueWaitEscalation(ctx, exec, cell, now)
		if err != nil {
			return err
		}
		cursor := due.wait.EscalationCursor
		if cursor < len(due.expiry.Escalate) {
			return advanceWaitEscalationEffect(ctx, exec, &due, cursor, now)
		}
		if due.expiry.Route != "" {
			return g.resolveExpiredWaitRoute(ctx, exec, &due, now)
		}
		return expireWaitWithAttention(ctx, exec, &due, now)
	})
}

func loadDueWaitEscalation(
	ctx context.Context,
	exec taskSQLExecutor,
	cell waitEscalationCell,
	now time.Time,
) (dueWaitEscalation, error) {
	run, err := getLoopRunByIDWithExecutor(ctx, exec, cell.runID)
	if err != nil {
		return dueWaitEscalation{}, err
	}
	if !run.Status.Live() || run.Status == looppkg.StatusPaused || run.CancelRequested {
		return dueWaitEscalation{}, fmt.Errorf(
			"%w: wait run is not eligible for escalation",
			looppkg.ErrInvalidTransition,
		)
	}
	wait, err := loadWaitForResume(ctx, exec, looppkg.WaitResumeMutation{
		RunID: cell.runID, Generation: cell.generation, NodeID: cell.nodeID, ItemIndex: cell.itemIndex,
	})
	if err != nil {
		return dueWaitEscalation{}, err
	}
	if wait.ClaimState != looppkg.WaitClaimWaiting || wait.NextEscalationAt == nil ||
		wait.NextEscalationAt.After(now.UTC()) || !wait.NextEscalationAt.Equal(cell.nextEscalationAt) {
		return dueWaitEscalation{}, fmt.Errorf("%w: wait escalation schedule changed", looppkg.ErrTransitionConflict)
	}
	if wait.Kind == looppkg.NodeWaitKindApprovalEscalation &&
		(run.Status != looppkg.StatusNeedsApproval || run.ActiveGateID != wait.NodeID) {
		return dueWaitEscalation{}, fmt.Errorf("%w: approval wait is no longer active", looppkg.ErrTransitionConflict)
	}
	if err := validateDueWaitEscalationCell(ctx, exec, cell, wait); err != nil {
		return dueWaitEscalation{}, err
	}
	resolved, err := loadWaitExecutedDefinition(ctx, exec, run)
	if err != nil {
		return dueWaitEscalation{}, err
	}
	node, found := loopWaitNodeByRuntimeID(resolved.Definition.Graph, "", string(cell.nodeID))
	if !found {
		return dueWaitEscalation{}, fmt.Errorf("%w: wait node %q is absent", looppkg.ErrValidation, cell.nodeID)
	}
	expiry, err := waitEscalationExpiry(node, wait.Kind)
	if err != nil {
		return dueWaitEscalation{}, err
	}
	if expiry == nil {
		return dueWaitEscalation{}, fmt.Errorf(
			"%w: wait node %q has no valid expiry",
			looppkg.ErrValidation,
			cell.nodeID,
		)
	}
	lifecycle, err := looppkg.ResolveNodeLifecycleConfig(node, nil, resolved.EffectiveConfig.Lifecycle)
	if err != nil {
		return dueWaitEscalation{}, fmt.Errorf("store: resolve wait escalation lifecycle: %w", err)
	}
	sourceID, err := waitStartedEventID(ctx, exec, cell, wait.IssuedEpoch)
	if err != nil {
		return dueWaitEscalation{}, err
	}
	return dueWaitEscalation{
		wait: wait, run: run, node: node, expiry: expiry,
		interval: lifecycle.WaitAdmissionInterval, sourceID: sourceID,
	}, nil
}

func waitEscalationExpiry(node dsl.Node, kind string) (*dsl.WaitExpiry, error) {
	switch strings.TrimSpace(kind) {
	case looppkg.NodeWaitKindApprovalEscalation:
		if node.Class != dsl.NodeClassControl || dsl.ControlKind(node.Kind) != dsl.ControlGate {
			return nil, fmt.Errorf("%w: approval wait node %q is not a gate", looppkg.ErrValidation, node.ID)
		}
		return node.Expires, nil
	case looppkg.NodeWaitKindTimer, looppkg.NodeWaitKindEvent:
		var params dsl.WaitParams
		if err := node.Params.Decode(&params); err != nil {
			return nil, fmt.Errorf("%w: decode wait node %q expiry: %v", looppkg.ErrValidation, node.ID, err)
		}
		return params.Expires, nil
	default:
		return nil, fmt.Errorf("%w: wait kind %q cannot escalate", looppkg.ErrValidation, kind)
	}
}

func loadWaitExecutedDefinition(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
) (*looppkg.ResolvedDefinition, error) {
	var raw string
	err := exec.QueryRowContext(ctx, `SELECT definition_json FROM loop_definition_snapshots
		WHERE workspace_id = ? AND definition_digest = ?`, run.WorkspaceID, run.DefinitionDigest).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: executed definition snapshot is missing", looppkg.ErrValidation)
	}
	if err != nil {
		return nil, fmt.Errorf("store: load wait escalation definition: %w", err)
	}
	resolved, err := looppkg.LoadExecutedDefinitionSnapshot(json.RawMessage(raw), run.DefinitionDigest)
	if err != nil {
		return nil, fmt.Errorf("store: hydrate wait escalation definition: %w", err)
	}
	return resolved, nil
}

func validateDueWaitEscalationCell(
	ctx context.Context,
	exec taskSQLExecutor,
	cell waitEscalationCell,
	wait looppkg.NodeWait,
) error {
	var status, cancelState string
	var epoch int64
	var paused, quarantined int
	err := exec.QueryRowContext(ctx, `SELECT output.status, output.epoch,
		COALESCE(control.paused, 0), COALESCE(control.quarantined, 0),
		COALESCE(control.cancel_state, '') FROM loop_generation_outputs AS output
		LEFT JOIN loop_node_controls AS control ON control.loop_run_id = output.loop_run_id
		 AND control.node_id = output.node_id WHERE output.loop_run_id = ? AND output.generation = ?
		 AND output.node_id = ? AND output.item_index = ?`, cell.runID, cell.generation,
		cell.nodeID, cell.itemIndex).Scan(&status, &epoch, &paused, &quarantined, &cancelState)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: wait escalation cell no longer exists", looppkg.ErrTransitionConflict)
	}
	if err != nil {
		return fmt.Errorf("store: load due wait escalation cell: %w", err)
	}
	wantStatus := loopNodeOutputWaiting
	if wait.Kind == looppkg.NodeWaitKindApprovalEscalation {
		wantStatus = loopNodeOutputSucceeded
		if cell.nodeID != wait.NodeID {
			return fmt.Errorf("%w: approval wait identity changed", looppkg.ErrTransitionConflict)
		}
	}
	if status != wantStatus || epoch != wait.IssuedEpoch || paused != 0 || quarantined != 0 ||
		strings.TrimSpace(cancelState) != "" {
		return fmt.Errorf("%w: wait escalation cell changed", looppkg.ErrTransitionConflict)
	}
	return nil
}

func waitStartedEventID(
	ctx context.Context,
	exec taskSQLExecutor,
	cell waitEscalationCell,
	issuedEpoch int64,
) (string, error) {
	var eventID string
	err := exec.QueryRowContext(ctx, `SELECT id FROM loop_run_events WHERE loop_run_id = ?
		AND kind = ? AND json_extract(payload_json, '$.generation') = ?
		AND json_extract(payload_json, '$.node_id') = ?
		AND json_extract(payload_json, '$.item_index') = ?
		AND json_extract(payload_json, '$.issued_epoch') = ? ORDER BY seq DESC LIMIT 1`,
		cell.runID, loopRunEventNodeWaitStarted, cell.generation, cell.nodeID,
		cell.itemIndex, issuedEpoch).Scan(&eventID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("%w: wait start event is missing", looppkg.ErrValidation)
	}
	if err != nil {
		return "", fmt.Errorf("store: load wait start event: %w", err)
	}
	return eventID, nil
}

func advanceWaitEscalationEffect(
	ctx context.Context,
	exec taskSQLExecutor,
	due *dueWaitEscalation,
	cursor int,
	now time.Time,
) error {
	intents, err := looppkg.RenderEffectIntents(
		[]dsl.EffectSpec{due.expiry.Escalate[cursor]},
		looppkg.EffectContextRequest{
			Run: &due.run, Trigger: looppkg.EffectTriggerWaitEscalation,
			Generation: due.wait.Generation, NodeID: due.wait.NodeID, ItemIndex: due.wait.ItemIndex,
		},
	)
	if err != nil {
		return err
	}
	intents[0].EntryIndex = cursor
	if err := insertLoopEffectIntentsWithExecutor(ctx, exec, due.run, due.sourceID, intents, now); err != nil {
		return err
	}
	next := now.UTC().Add(due.interval)
	result, err := exec.ExecContext(ctx, `UPDATE loop_node_waits SET escalation_cursor = ?,
		next_escalation_at = ? WHERE loop_run_id = ? AND generation = ? AND node_id = ?
		AND item_index = ? AND claim_state = 'waiting' AND escalation_cursor = ?
		AND issued_epoch = ?`, cursor+1, next, due.wait.LoopRunID, due.wait.Generation,
		due.wait.NodeID, due.wait.ItemIndex, cursor, due.wait.IssuedEpoch)
	if err != nil {
		return fmt.Errorf("store: advance wait escalation ladder: %w", err)
	}
	return requireSingleWaitMutation(result)
}

func expireWaitWithAttention(
	ctx context.Context,
	exec taskSQLExecutor,
	due *dueWaitEscalation,
	now time.Time,
) error {
	result, err := exec.ExecContext(ctx, `UPDATE loop_node_waits SET claim_state = 'intervention_required',
		next_escalation_at = NULL WHERE loop_run_id = ? AND generation = ? AND node_id = ?
		AND item_index = ? AND claim_state = 'waiting' AND escalation_cursor = ? AND issued_epoch = ?`,
		due.wait.LoopRunID, due.wait.Generation, due.wait.NodeID, due.wait.ItemIndex,
		due.wait.EscalationCursor, due.wait.IssuedEpoch)
	if err != nil {
		return fmt.Errorf("store: expire wait with attention: %w", err)
	}
	if err := requireSingleWaitMutation(result); err != nil {
		return err
	}
	status := loopNodeOutputWaiting
	if due.wait.Kind == looppkg.NodeWaitKindApprovalEscalation {
		status = loopNodeOutputSucceeded
	}
	result, err = exec.ExecContext(ctx, `UPDATE loop_generation_outputs SET epoch = epoch + 1
		WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?
		AND status = ? AND epoch = ?`, due.wait.LoopRunID, due.wait.Generation,
		due.wait.NodeID, due.wait.ItemIndex, status, due.wait.IssuedEpoch)
	if err != nil {
		return fmt.Errorf("store: fence expired wait cell: %w", err)
	}
	if err := requireSingleWaitMutation(result); err != nil {
		return err
	}
	if _, err := exec.ExecContext(ctx, `INSERT INTO loop_node_controls (
		loop_run_id, node_id, attention_flag, attention_reason, revision, updated_at
	) VALUES (?, ?, ?, ?, 1, ?)
	ON CONFLICT(loop_run_id, node_id) DO UPDATE SET attention_flag = excluded.attention_flag,
		attention_reason = excluded.attention_reason, revision = revision + 1,
		updated_at = excluded.updated_at`, due.wait.LoopRunID, due.wait.NodeID,
		expiredWaitAttentionFlag, expiredWaitAttentionReason, now.UTC()); err != nil {
		return fmt.Errorf("store: flag expired wait attention: %w", err)
	}
	return appendLoopRunEventWithExecutor(ctx, exec, due.run.ID, due.run.WorkspaceID,
		loopRunEventNodeAttentionFlagged, map[string]any{
			loopRunEventPayloadKeyGeneration: due.wait.Generation,
			loopRunEventPayloadKeyNodeID:     due.wait.NodeID,
			loopRunEventPayloadKeyItemIndex:  due.wait.ItemIndex,
			"attention_flag":                 expiredWaitAttentionFlag,
			loopRunEventPayloadKeyReason:     expiredWaitAttentionReason,
		}, now)
}
