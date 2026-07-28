package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// CompleteRun marks one running task run as completed and reconciles task state.
func (m *Service) CompleteRun(
	ctx context.Context,
	runID string,
	result RunResult,
	actor ActorContext,
) (*Run, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return nil, err
	}

	normalizedResult, err := normalizeRunResult(result)
	if err != nil {
		return nil, err
	}

	run, _, err := m.loadAuthorizedRunWithTask(ctx, runID, actor)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(run.ClaimTokenHash) != "" {
		return nil, fmt.Errorf(
			"%w: task run %q requires token-fenced completion",
			ErrInvalidClaimToken,
			run.ID,
		)
	}
	if err := requireRunTransition(run, TaskRunStatusCompleted); err != nil {
		return nil, err
	}
	storedResult, err := normalizedResult.StoredValue()
	if err != nil {
		return nil, err
	}

	run.Status = TaskRunStatusCompleted
	run.Result = storedResult
	run.Error = ""
	run.LeaseUntil = time.Time{}
	run.HeartbeatAt = time.Time{}
	run.EndedAt = m.now().UTC()
	if err := m.stopTerminalRunSession(ctx, run, StopReasonCompleted); err != nil {
		return nil, err
	}
	settlement, err := m.store.CompleteRunSettlement(ctx, run, actor)
	if err != nil {
		return nil, err
	}

	publicationCtx, publicationCancel := completedSettlementPublicationContext(ctx)
	defer publicationCancel()
	reconciledTask, publicationErr := m.publishCompletedRunSettlement(publicationCtx, &settlement, actor)
	eventErr := m.recordTaskEvent(publicationCtx, run.TaskID, run.ID, taskEventRunCompleted, actor, completedRunPayload{
		Status:     run.Status,
		TaskStatus: reconciledTask.Status,
		Result:     cloneRawJSON(run.Result),
	})
	m.dispatchTerminalWake(publicationCtx, reconciledTask, run, actor)

	return &run, errors.Join(publicationErr, eventErr)
}

// FailRun marks one starting or running task run as failed and reconciles task state.
func (m *Service) FailRun(
	ctx context.Context,
	runID string,
	failure RunFailure,
	actor ActorContext,
) (*Run, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return nil, err
	}

	normalizedFailure, err := normalizeRunFailure(failure)
	if err != nil {
		return nil, err
	}

	run, taskRecord, err := m.loadAuthorizedRunWithTask(ctx, runID, actor)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(run.ClaimTokenHash) != "" {
		return nil, fmt.Errorf(
			"%w: task run %q requires token-fenced failure",
			ErrInvalidClaimToken,
			run.ID,
		)
	}
	return m.failRunRecord(ctx, taskRecord, run, normalizedFailure, actor)
}

// CancelRun cancels one non-terminal task run under manager authority.
func (m *Service) CancelRun(
	ctx context.Context,
	runID string,
	req CancelRun,
	actor ActorContext,
) (*Run, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return nil, err
	}

	normalizedReq, err := normalizeCancelRun(req)
	if err != nil {
		return nil, err
	}

	run, taskRecord, err := m.loadAuthorizedRunWithTask(ctx, runID, actor)
	if err != nil {
		return nil, err
	}
	return m.cancelRunRecord(ctx, taskRecord, run, normalizedReq, actor, cancelRunOptions{
		reconcileTask: true,
	})
}

// RecoverRunOnBoot applies one daemon-owned recovery decision to a non-terminal
// run discovered during startup reconciliation.
func (m *Service) RecoverRunOnBoot(
	ctx context.Context,
	runID string,
	recovery RunBootRecovery,
	actor ActorContext,
) (*Run, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return nil, err
	}

	normalizedRecovery, err := normalizeRunBootRecovery(recovery)
	if err != nil {
		return nil, err
	}

	run, err := m.loadRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	if run.IsNetworkWake() {
		return m.recoverNetworkWakeOnBoot(ctx, run, normalizedRecovery, actor)
	}
	taskRecord, err := m.store.GetTask(ctx, run.TaskID)
	if err != nil {
		return nil, err
	}

	previousStatus := run.Status.Normalize()
	previousSessionID := strings.TrimSpace(run.SessionID)
	switch normalizedRecovery.Action.Normalize() {
	case RunBootRecoveryRequeue:
		return m.recoverRunByRequeue(
			ctx,
			taskRecord,
			run,
			normalizedRecovery,
			actor,
			previousStatus,
			previousSessionID,
		)
	case RunBootRecoveryMarkRunning:
		return m.recoverRunByMarkRunning(
			ctx,
			taskRecord,
			run,
			normalizedRecovery,
			actor,
			previousStatus,
			previousSessionID,
		)
	case RunBootRecoveryFail:
		return m.recoverRunByFailure(
			ctx,
			taskRecord,
			run,
			normalizedRecovery,
			actor,
			previousStatus,
			previousSessionID,
		)
	default:
		return nil, fmt.Errorf(
			"%w: run boot recovery action %q is not supported",
			ErrValidation,
			normalizedRecovery.Action,
		)
	}
}
