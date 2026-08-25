package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	storedResult, err := normalizedResult.StoredValue()
	if err != nil {
		return nil, err
	}
	run, taskRecord, err := m.loadAuthorizedRunWithTask(ctx, runID, actor)
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
	storedResult, err = m.prepareResultContract(ctx, run, storedResult)
	if validationErr, invalid := asResultContractValidationError(err); invalid {
		first, admissionErr := m.admitResultContractRepair(ctx, run, "", actor, validationErr)
		if admissionErr != nil {
			return nil, admissionErr
		}
		if first {
			return nil, validationErr
		}
		failure, failureErr := invalidTaskResultFailure(validationErr)
		if failureErr != nil {
			return nil, failureErr
		}
		failed, failureErr := m.failRunRecord(ctx, taskRecord, run, failure, actor)
		if failureErr != nil {
			return nil, errors.Join(ErrResultInvalid, validationErr, failureErr)
		}
		return failed, errors.Join(ErrResultInvalid, validationErr)
	}
	if err != nil {
		return nil, err
	}
	if err := m.preflightTaskEvent(
		run.TaskID,
		run.ID,
		taskEventRunCompleted,
		actor,
		completedRunPayload{
			Status: TaskRunStatusCompleted, TaskStatus: taskRecord.Status, Result: completionEventResult(storedResult),
		},
	); err != nil {
		return nil, err
	}
	commandID, err := m.newID("terminal-command")
	if err != nil {
		return nil, fmt.Errorf("task: generate terminal command id: %w", err)
	}
	eventIDs, err := m.reserveTaskEventIDs(1)
	if err != nil {
		return nil, err
	}
	command, err := newCompletionTerminalRunCommand(
		commandID,
		eventIDs,
		run,
		storedResult,
		actor,
		m.now().UTC(),
	)
	if err != nil {
		return nil, err
	}
	settlement, err := m.executeTerminalRunCommand(ctx, command)
	if err != nil {
		return nil, err
	}
	publicationErr := m.publishTerminalRunCommandSettlement(ctx, &settlement)
	return &settlement.run, publicationErr
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
	return m.failRunRecordWithOptions(
		ctx,
		taskRecord,
		run,
		normalizedFailure,
		actor,
		failRunRecordOptions{stopTerminalSession: true},
	)
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
	return m.cancelRunRecord(
		ctx,
		taskRecord,
		run,
		normalizedReq,
		actor,
		cancelRunOptions{reconcileTask: true},
	)
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
	if actor.Actor.Kind.Normalize() != ActorKindDaemon {
		return nil, ErrPermissionDenied
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
	if err := m.authorizeTaskResource(ctx, actor, taskRecord); err != nil {
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
