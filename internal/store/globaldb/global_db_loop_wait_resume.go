package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	contractspkg "github.com/compozy/compozy/internal/contracts"
	looppkg "github.com/compozy/compozy/internal/loop"
	storepkg "github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

var _ looppkg.WaitAdmission = (*LoopRepo)(nil)

// ResumeWait atomically admits a payload, resolves the waiting cell, and reserves its coordinator.
func (g *LoopRepo) ResumeWait(
	ctx context.Context,
	mutation looppkg.WaitResumeMutation,
) (looppkg.WaitResumeResult, error) {
	if err := g.checkReady(ctx, "resume Loop wait"); err != nil {
		return looppkg.WaitResumeResult{}, err
	}
	if err := mutation.Validate(); err != nil {
		return looppkg.WaitResumeResult{}, err
	}
	var result looppkg.WaitResumeResult
	var admissionErr error
	err := g.withTaskImmediateTransaction(ctx, "resume Loop wait", func(exec taskSQLExecutor) error {
		run, err := nodePauseRun(ctx, exec, mutation.WorkspaceID, mutation.RunID)
		if err != nil {
			return err
		}
		wait, err := loadWaitForResume(ctx, exec, mutation)
		if err != nil {
			return err
		}
		result.Wait = wait
		if wait.ClaimState == looppkg.WaitClaimResumed {
			return nil
		}
		if wait.ClaimState == looppkg.WaitClaimInterventionRequired {
			return fmt.Errorf("%w: wait requires intervention", looppkg.ErrInvalidTransition)
		}
		if err := validateContractWaitPayload(wait.Expect, mutation.Payload); err != nil {
			updated, updateErr := recordWaitAdmissionFailure(ctx, exec, run, mutation, wait)
			if updateErr != nil {
				return updateErr
			}
			result.Wait = updated
			admissionErr = err
			return nil
		}
		if err := validateWaitResumeCell(ctx, exec, mutation, wait); err != nil {
			return err
		}
		outputRef := contractspkg.OutputRefForPayload(mutation.Payload)
		if err := storepkg.UpsertLoopOutputBlob(
			ctx, exec, outputRef, mutation.Payload, mutation.RequestedAt,
		); err != nil {
			return err
		}
		if err := claimAndResolveWait(ctx, exec, mutation, wait, outputRef); err != nil {
			return err
		}
		parkedFor := max(mutation.RequestedAt.UTC().Sub(wait.CreatedAt.UTC()), 0)
		if err := shiftLoopWallClockIfUnparked(ctx, exec, mutation.RunID, parkedFor); err != nil {
			return err
		}
		if err := appendLoopRunEventWithExecutor(ctx, exec, run.ID, run.WorkspaceID,
			loopRunEventNodeWaitResumed, waitResumeEventPayload(mutation, wait), mutation.RequestedAt); err != nil {
			return err
		}
		coordinator, err := g.reserveOrReuseOpenLoopCoordinatorRunWithExecutor(
			ctx, exec, run, waitResumeOrigin(mutation), mutation.RequestedAt,
			waitResumeIdempotencyKey(mutation, wait.IssuedEpoch),
		)
		if err != nil {
			return err
		}
		claimedAt := mutation.RequestedAt.UTC()
		wait.ClaimState = looppkg.WaitClaimResumed
		wait.ClaimedByKind = strings.TrimSpace(mutation.ClaimedByKind)
		wait.ClaimedByID = strings.TrimSpace(mutation.ClaimedByID)
		wait.ClaimedAt = &claimedAt
		wait.AheadPayload = append([]byte(nil), mutation.Payload...)
		result = looppkg.WaitResumeResult{Wait: wait, Coordinator: &coordinator, Won: true}
		return nil
	})
	if err != nil {
		return looppkg.WaitResumeResult{}, err
	}
	return result, admissionErr
}

func loadWaitForResume(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.WaitResumeMutation,
) (looppkg.NodeWait, error) {
	var wait looppkg.NodeWait
	var resumeAt, nextEscalationAt, claimedAt sql.NullTime
	var claimedByKind, claimedByID, expect, ahead sql.NullString
	var generation int64
	var itemIndex, cursor, failures int64
	var state string
	err := exec.QueryRowContext(ctx, `SELECT loop_run_id, generation, node_id, item_index, kind,
		resume_at, next_escalation_at, escalation_cursor, claim_state, claimed_by_kind,
		claimed_by_id, claimed_at, admission_failures, expect_json, ahead_payload_json,
		issued_epoch, created_at FROM loop_node_waits WHERE loop_run_id = ? AND generation = ?
		AND node_id = ? AND item_index = ?`, mutation.RunID, mutation.Generation, mutation.NodeID,
		mutation.ItemIndex).Scan(&wait.LoopRunID, &generation, &wait.NodeID, &itemIndex, &wait.Kind,
		&resumeAt, &nextEscalationAt, &cursor, &state, &claimedByKind, &claimedByID, &claimedAt,
		&failures, &expect, &ahead, &wait.IssuedEpoch, &wait.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return looppkg.NodeWait{}, fmt.Errorf("%w: wait is not active", looppkg.ErrInvalidTransition)
	}
	if err != nil {
		return looppkg.NodeWait{}, fmt.Errorf("store: load Loop wait for resume: %w", err)
	}
	wait.Generation = int(generation)
	wait.ItemIndex = int(itemIndex)
	wait.EscalationCursor = int(cursor)
	wait.ClaimState = looppkg.WaitClaimState(state)
	if !wait.ClaimState.Valid() {
		return looppkg.NodeWait{}, fmt.Errorf("%w: invalid wait claim state %q", looppkg.ErrValidation, state)
	}
	wait.ResumeAt = loopTimePointer(resumeAt)
	wait.NextEscalationAt = loopTimePointer(nextEscalationAt)
	wait.ClaimedByKind = claimedByKind.String
	wait.ClaimedByID = claimedByID.String
	wait.ClaimedAt = loopTimePointer(claimedAt)
	wait.AdmissionFailures = int(failures)
	if expect.Valid {
		wait.Expect = []byte(expect.String)
	}
	if ahead.Valid {
		wait.AheadPayload = []byte(ahead.String)
	}
	wait.CreatedAt = wait.CreatedAt.UTC()
	return wait, nil
}

func recordWaitAdmissionFailure(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	mutation looppkg.WaitResumeMutation,
	wait looppkg.NodeWait,
) (looppkg.NodeWait, error) {
	nextFailures := wait.AdmissionFailures + 1
	nextState := looppkg.WaitClaimWaiting
	if nextFailures >= mutation.AdmissionAttempts {
		nextState = looppkg.WaitClaimInterventionRequired
	}
	result, err := exec.ExecContext(ctx, `UPDATE loop_node_waits SET admission_failures = ?, claim_state = ?
		WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?
		AND claim_state = 'waiting' AND issued_epoch = ?`, nextFailures, nextState, mutation.RunID,
		mutation.Generation, mutation.NodeID, mutation.ItemIndex, wait.IssuedEpoch)
	if err != nil {
		return looppkg.NodeWait{}, fmt.Errorf("store: record Loop wait admission failure: %w", err)
	}
	if err := requireSingleWaitMutation(result); err != nil {
		return looppkg.NodeWait{}, err
	}
	if nextState == looppkg.WaitClaimInterventionRequired {
		if _, err := exec.ExecContext(ctx, `INSERT INTO loop_node_controls (
			loop_run_id, node_id, attention_flag, attention_reason, revision, updated_at
		) VALUES (?, ?, 'wait_intervention', 'wait payload admission failed repeatedly', 1, ?)
		ON CONFLICT(loop_run_id, node_id) DO UPDATE SET attention_flag = 'wait_intervention',
			attention_reason = 'wait payload admission failed repeatedly', revision = revision + 1,
			updated_at = excluded.updated_at`, mutation.RunID, mutation.NodeID, mutation.RequestedAt.UTC()); err != nil {
			return looppkg.NodeWait{}, fmt.Errorf("store: flag Loop wait intervention: %w", err)
		}
		if err := appendLoopRunEventWithExecutor(ctx, exec, run.ID, run.WorkspaceID,
			loopRunEventNodeAttentionFlagged, map[string]any{
				loopRunEventPayloadKeyGeneration:    mutation.Generation,
				loopRunEventPayloadKeyNodeID:        mutation.NodeID,
				loopRunEventPayloadKeyItemIndex:     mutation.ItemIndex,
				loopRunEventPayloadKeyAttentionFlag: "wait_intervention",
				loopRunEventPayloadKeyReason:        "wait payload admission failed repeatedly",
				"admission_failures":                nextFailures,
			}, mutation.RequestedAt); err != nil {
			return looppkg.NodeWait{}, err
		}
	}
	wait.AdmissionFailures = nextFailures
	wait.ClaimState = nextState
	return wait, nil
}

func validateWaitResumeCell(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.WaitResumeMutation,
	wait looppkg.NodeWait,
) error {
	var status string
	var epoch int64
	var paused, quarantined int64
	var cancelState string
	err := exec.QueryRowContext(ctx, `SELECT output.status, output.epoch,
		COALESCE(control.paused, 0), COALESCE(control.quarantined, 0), COALESCE(control.cancel_state, '')
		FROM loop_generation_outputs AS output LEFT JOIN loop_node_controls AS control
		ON control.loop_run_id = output.loop_run_id AND control.node_id = output.node_id
		WHERE output.loop_run_id = ? AND output.generation = ? AND output.node_id = ?
		AND output.item_index = ?`, mutation.RunID, mutation.Generation, mutation.NodeID,
		mutation.ItemIndex).Scan(&status, &epoch, &paused, &quarantined, &cancelState)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: wait cell no longer exists", looppkg.ErrTransitionConflict)
	}
	if err != nil {
		return fmt.Errorf("store: load Loop wait cell: %w", err)
	}
	if status != loopNodeOutputWaiting || epoch != wait.IssuedEpoch || paused != 0 || quarantined != 0 ||
		strings.TrimSpace(cancelState) != "" {
		return fmt.Errorf("%w: wait cell changed or is parked by another control", looppkg.ErrTransitionConflict)
	}
	return nil
}

func claimAndResolveWait(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.WaitResumeMutation,
	wait looppkg.NodeWait,
	outputRef string,
) error {
	storedResult, err := looppkg.EncodeGenerationResultForRef(outputRef)
	if err != nil {
		return fmt.Errorf("store: encode resumed Loop wait result: %w", err)
	}
	result, err := exec.ExecContext(ctx, `UPDATE loop_node_waits SET claim_state = 'resumed',
		claimed_by_kind = ?, claimed_by_id = ?, claimed_at = ?, ahead_payload_json = ?
		WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?
		AND claim_state = 'waiting' AND issued_epoch = ?`, strings.TrimSpace(mutation.ClaimedByKind),
		strings.TrimSpace(mutation.ClaimedByID), mutation.RequestedAt.UTC(), string(mutation.Payload),
		mutation.RunID, mutation.Generation, mutation.NodeID, mutation.ItemIndex, wait.IssuedEpoch)
	if err != nil {
		return fmt.Errorf("store: claim Loop wait: %w", err)
	}
	if err := requireSingleWaitMutation(result); err != nil {
		return err
	}
	parkedFor := mutation.RequestedAt.UTC().Sub(wait.CreatedAt.UTC())
	parkedFor = max(parkedFor, 0)
	firstScheduledAt, err := shiftedLoopCellFirstScheduledAt(
		ctx, exec, mutation.RunID, mutation.Generation, mutation.NodeID, mutation.ItemIndex, parkedFor,
	)
	if err != nil {
		return err
	}
	result, err = exec.ExecContext(ctx, `UPDATE loop_generation_outputs SET status = 'succeeded',
		output_ref = ?, task_run_id = NULL, next_attempt_at = NULL,
		first_scheduled_at = ?, epoch = epoch + 1
		WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?
		AND status = 'waiting' AND epoch = ?`, storedResult, firstScheduledAt, mutation.RunID, mutation.Generation,
		mutation.NodeID, mutation.ItemIndex, wait.IssuedEpoch)
	if err != nil {
		return fmt.Errorf("store: resolve Loop wait cell: %w", err)
	}
	return requireSingleWaitMutation(result)
}

func waitResumeEventPayload(m looppkg.WaitResumeMutation, wait looppkg.NodeWait) map[string]any {
	return map[string]any{
		loopRunEventPayloadKeyGeneration: m.Generation, loopRunEventPayloadKeyNodeID: m.NodeID,
		loopRunEventPayloadKeyItemIndex: m.ItemIndex, loopRunEventPayloadKeyIssuedEpoch: wait.IssuedEpoch,
		loopRunEventPayloadKeyActorKind: strings.TrimSpace(m.ClaimedByKind),
		loopRunEventPayloadKeyActorID:   strings.TrimSpace(m.ClaimedByID),
		loopRunEventPayloadKeyWaitKind:  wait.Kind,
	}
}

func waitResumeOrigin(m looppkg.WaitResumeMutation) taskpkg.Origin {
	return taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "loop.wait-resume:" + strings.TrimSpace(m.ClaimedByID)}
}

func waitResumeIdempotencyKey(m looppkg.WaitResumeMutation, epoch int64) string {
	return fmt.Sprintf("loop.wait.resume.%s.%d.%s.%d.%d", m.RunID, m.Generation, m.NodeID, m.ItemIndex, epoch)
}

func requireSingleWaitMutation(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: inspect Loop wait mutation: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("%w: Loop wait changed", looppkg.ErrTransitionConflict)
	}
	return nil
}
