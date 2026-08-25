package globaldb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
)

func (g *LoopRepo) resolveExpiredWaitRoute(
	ctx context.Context,
	exec taskSQLExecutor,
	due *dueWaitEscalation,
	now time.Time,
) error {
	route := strings.TrimSpace(string(due.expiry.Route))
	payload, err := json.Marshal(map[string]any{
		loopRunEventPayloadKeyExpired: true,
		loopRunEventPayloadKeyRoute:   route,
	})
	if err != nil {
		return fmt.Errorf("store: marshal expired wait route payload: %w", err)
	}
	result, err := exec.ExecContext(ctx, `UPDATE loop_node_waits SET claim_state = 'resumed',
		claimed_by_kind = 'expiry', claimed_by_id = ?, claimed_at = ?, ahead_payload_json = ?,
		next_escalation_at = NULL WHERE loop_run_id = ? AND generation = ? AND node_id = ?
		AND item_index = ? AND claim_state = 'waiting' AND escalation_cursor = ? AND issued_epoch = ?`,
		route, now.UTC(), string(payload), due.wait.LoopRunID, due.wait.Generation,
		due.wait.NodeID, due.wait.ItemIndex, due.wait.EscalationCursor, due.wait.IssuedEpoch)
	if err != nil {
		return fmt.Errorf("store: claim expired wait route: %w", err)
	}
	if err := requireSingleWaitMutation(result); err != nil {
		return err
	}
	if due.wait.Kind == looppkg.NodeWaitKindRequest {
		if err := expireLoopRequestWithExecutor(ctx, exec, due, now); err != nil {
			return err
		}
	}
	parkedFor := now.UTC().Sub(due.wait.CreatedAt.UTC())
	parkedFor = max(parkedFor, 0)
	if due.wait.Kind == looppkg.NodeWaitKindApprovalEscalation {
		return g.resolveExpiredApprovalRoute(ctx, exec, due, route, payload, parkedFor, now)
	}
	firstScheduledAt, err := shiftedLoopCellFirstScheduledAt(
		ctx, exec, due.wait.LoopRunID, due.wait.Generation, due.wait.NodeID, due.wait.ItemIndex, parkedFor,
	)
	if err != nil {
		return err
	}
	storedResult, err := looppkg.EncodeGenerationResultForRef(looppkg.WaitExpiryRouteOutputRef(route))
	if err != nil {
		return fmt.Errorf("store: encode expired wait route result: %w", err)
	}
	result, err = exec.ExecContext(ctx, `UPDATE loop_generation_outputs SET status = 'succeeded',
		output_ref = ?, task_run_id = NULL, next_attempt_at = NULL,
		first_scheduled_at = ?, epoch = epoch + 1
		WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?
		AND status = 'waiting' AND epoch = ?`, storedResult,
		firstScheduledAt, due.wait.LoopRunID, due.wait.Generation, due.wait.NodeID,
		due.wait.ItemIndex, due.wait.IssuedEpoch)
	if err != nil {
		return fmt.Errorf("store: resolve expired wait route cell: %w", err)
	}
	if err := requireSingleWaitMutation(result); err != nil {
		return err
	}
	if err := shiftLoopWallClockIfUnparked(ctx, exec, due.run.ID, parkedFor); err != nil {
		return err
	}
	if err := appendLoopRunEventWithExecutor(ctx, exec, due.run.ID, due.run.WorkspaceID,
		loopRunEventNodeWaitResumed, expiredWaitResumeEventPayload(due, route, payload), now); err != nil {
		return err
	}
	_, _, err = g.reserveLoopCoordinatorRunWithExecutor(
		ctx, exec, due.run, looppkg.WaitExpiryOrigin(route), now, "",
		fmt.Sprintf("loop.wait.expiry.%s.%d.%s.%d.%d", due.run.ID, due.wait.Generation,
			due.wait.NodeID, due.wait.ItemIndex, due.wait.IssuedEpoch),
	)
	return err
}

func expireLoopRequestWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	due *dueWaitEscalation,
	now time.Time,
) error {
	result, err := exec.ExecContext(ctx, `UPDATE loop_requests SET state = 'expired', resolved_at = ?,
		actor_kind = 'expiry', actor_id = 'wait-expiry' WHERE loop_run_id = ? AND generation = ?
		AND node_id = ? AND item_index = ? AND state = 'pending'`, now.UTC(), due.wait.LoopRunID,
		due.wait.Generation, due.wait.NodeID, due.wait.ItemIndex)
	if err != nil {
		return fmt.Errorf("store: expire Loop request: %w", err)
	}
	if err := requireSingleWaitMutation(result); err != nil {
		return err
	}
	return appendLoopRunEventWithExecutor(ctx, exec, due.run.ID, due.run.WorkspaceID,
		loopRunEventRequestExpired, map[string]any{
			loopRunEventPayloadKeyGeneration:  due.wait.Generation,
			loopRunEventPayloadKeyNodeID:      due.wait.NodeID,
			loopRunEventPayloadKeyItemIndex:   due.wait.ItemIndex,
			loopRunEventPayloadKeyIssuedEpoch: due.wait.IssuedEpoch,
		}, now)
}

func (g *LoopRepo) resolveExpiredApprovalRoute(
	ctx context.Context,
	exec taskSQLExecutor,
	due *dueWaitEscalation,
	route string,
	payload []byte,
	parkedFor time.Duration,
	now time.Time,
) error {
	storedResult, err := looppkg.EncodeGenerationResultForRef(looppkg.WaitExpiryRouteOutputRef(route))
	if err != nil {
		return fmt.Errorf("store: encode expired approval route result: %w", err)
	}
	result, err := exec.ExecContext(ctx, `UPDATE loop_generation_outputs SET output_ref = ?, epoch = epoch + 1
		WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?
		AND status = 'succeeded' AND epoch = ?`, storedResult,
		due.wait.LoopRunID, due.wait.Generation, due.wait.NodeID, due.wait.ItemIndex, due.wait.IssuedEpoch)
	if err != nil {
		return fmt.Errorf("store: resolve expired approval route cell: %w", err)
	}
	if err := requireSingleWaitMutation(result); err != nil {
		return err
	}
	if err := shiftLoopWallClockIfUnparked(ctx, exec, due.run.ID, parkedFor); err != nil {
		return err
	}
	if err := compareAndSwapLoopRunStatusWithExecutor(
		ctx, exec, due.run.ID, looppkg.StatusNeedsApproval, looppkg.StatusRunning,
		looppkg.TransitionCauseWaitExpired, now,
	); err != nil {
		return err
	}
	due.run.Status = looppkg.StatusRunning
	if err := appendLoopRunEventWithExecutor(ctx, exec, due.run.ID, due.run.WorkspaceID,
		loopRunEventNodeWaitResumed, expiredWaitResumeEventPayload(due, route, payload), now); err != nil {
		return err
	}
	_, _, err = g.reserveLoopCoordinatorRunWithExecutor(
		ctx, exec, due.run, looppkg.WaitExpiryOrigin(route), now, "",
		fmt.Sprintf("loop.wait.expiry.%s.%d.%s.%d.%d", due.run.ID, due.wait.Generation,
			due.wait.NodeID, due.wait.ItemIndex, due.wait.IssuedEpoch),
	)
	return err
}

func expiredWaitResumeEventPayload(
	due *dueWaitEscalation,
	route string,
	payload []byte,
) map[string]any {
	return map[string]any{
		loopRunEventPayloadKeyGeneration:  due.wait.Generation,
		loopRunEventPayloadKeyNodeID:      due.wait.NodeID,
		loopRunEventPayloadKeyItemIndex:   due.wait.ItemIndex,
		loopRunEventPayloadKeyIssuedEpoch: due.wait.IssuedEpoch,
		loopRunEventPayloadKeyActorKind:   loopWaitClaimedByExpiry,
		loopRunEventPayloadKeyActorID:     route,
		loopRunEventPayloadKeyWaitKind:    due.wait.Kind,
		loopRunEventPayloadKeyRoute:       route,
		eventSummaryContentPayloadKey:     json.RawMessage(payload),
	}
}
