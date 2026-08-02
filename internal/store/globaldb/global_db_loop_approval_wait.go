package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
)

func claimActiveApprovalWait(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	decision looppkg.GateDecisionRecord,
) error {
	var activeCount int
	if err := exec.QueryRowContext(ctx, `SELECT COUNT(*) FROM loop_node_waits
		WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND kind = 'approval_escalation'
		AND claim_state IN ('waiting','intervention_required')`,
		run.ID, decision.Generation, decision.GateID).Scan(&activeCount); err != nil {
		return fmt.Errorf("store: count active approval waits: %w", err)
	}
	if activeCount == 0 {
		return nil
	}
	if activeCount > 1 {
		return fmt.Errorf(
			"%w: gate %q has %d active approval waits but the decision has no item identity",
			looppkg.ErrTransitionConflict,
			decision.GateID,
			activeCount,
		)
	}
	var wait looppkg.NodeWait
	var itemIndex int64
	err := exec.QueryRowContext(ctx, `SELECT loop_run_id, generation, node_id, item_index,
		issued_epoch, created_at FROM loop_node_waits WHERE loop_run_id = ? AND generation = ?
		AND node_id = ? AND kind = 'approval_escalation'
		AND claim_state IN ('waiting','intervention_required')`,
		run.ID, decision.Generation, decision.GateID).Scan(
		&wait.LoopRunID, &wait.Generation, &wait.NodeID, &itemIndex, &wait.IssuedEpoch, &wait.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: active approval wait disappeared", looppkg.ErrTransitionConflict)
	}
	if err != nil {
		return fmt.Errorf("store: load active approval wait: %w", err)
	}
	wait.ItemIndex = int(itemIndex)
	payload, err := json.Marshal(map[string]any{
		"decision": decision.Decision, "criterion_id": decision.CriterionID,
	})
	if err != nil {
		return fmt.Errorf("store: marshal approval wait decision: %w", err)
	}
	actorKind := strings.TrimSpace(string(decision.Actor.Actor.Kind))
	actorID := strings.TrimSpace(decision.Actor.Actor.Ref)
	result, err := exec.ExecContext(ctx, `UPDATE loop_node_waits SET claim_state = 'resumed',
		claimed_by_kind = ?, claimed_by_id = ?, claimed_at = ?, ahead_payload_json = ?,
		next_escalation_at = NULL WHERE loop_run_id = ? AND generation = ? AND node_id = ?
		AND item_index = ? AND kind = 'approval_escalation'
		AND claim_state IN ('waiting','intervention_required') AND issued_epoch = ?`,
		actorKind, actorID, decision.DecidedAt.UTC(), string(payload), wait.LoopRunID,
		wait.Generation, wait.NodeID, wait.ItemIndex, wait.IssuedEpoch)
	if err != nil {
		return fmt.Errorf("store: claim active approval wait: %w", err)
	}
	if err := requireSingleWaitMutation(result); err != nil {
		return err
	}
	parkedFor := decision.DecidedAt.UTC().Sub(wait.CreatedAt.UTC())
	parkedFor = max(parkedFor, 0)
	if err := shiftLoopWallClockIfUnparked(ctx, exec, run.ID, parkedFor); err != nil {
		return err
	}
	return appendLoopRunEventWithExecutor(ctx, exec, run.ID, run.WorkspaceID,
		loopRunEventNodeWaitResumed, map[string]any{
			loopRunEventPayloadKeyGeneration:  wait.Generation,
			loopRunEventPayloadKeyNodeID:      wait.NodeID,
			loopRunEventPayloadKeyItemIndex:   wait.ItemIndex,
			loopRunEventPayloadKeyIssuedEpoch: wait.IssuedEpoch,
			loopRunEventPayloadKeyActorKind:   actorKind,
			loopRunEventPayloadKeyActorID:     actorID,
			loopRunEventPayloadKeyWaitKind:    looppkg.NodeWaitKindApprovalEscalation,
			"decision":                        decision.Decision,
		}, decision.DecidedAt)
}
