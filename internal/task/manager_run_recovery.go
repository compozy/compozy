package task

import (
	"context"

	"fmt"

	"time"
)

func (m *Service) recoverRunByRequeue(
	ctx context.Context,
	taskRecord Task,
	run Run,
	recovery RunBootRecovery,
	actor ActorContext,
	previousStatus RunStatus,
	previousSessionID string,
) (*Run, error) {
	if previousStatus != TaskRunStatusClaimed {
		return nil, invalidRunRecoveryTransition(run, previousStatus, recovery.Action)
	}

	run.Status = TaskRunStatusQueued
	run.ClaimedBy = nil
	run.ClaimedAt = time.Time{}
	run.SessionID = ""
	run.ClaimTokenHash = ""
	run.LeaseUntil = time.Time{}
	run.HeartbeatAt = time.Time{}
	run.StartedAt = time.Time{}
	run.EndedAt = time.Time{}
	run.Error = ""
	run.Result = nil
	if err := m.store.UpdateTaskRun(ctx, run); err != nil {
		return nil, err
	}
	reconciledTask, err := m.recordRecoveredRun(
		ctx,
		taskRecord.ID,
		run,
		recovery,
		actor,
		previousStatus,
		previousSessionID,
	)
	if err != nil {
		return nil, err
	}
	m.dispatchTaskRunLeaseRecovered(
		ctx,
		run,
		reconciledTask,
		actor,
		previousStatus,
		previousSessionID,
		recovery,
	)
	return &run, nil
}

func (m *Service) recoverRunByMarkRunning(
	ctx context.Context,
	taskRecord Task,
	run Run,
	recovery RunBootRecovery,
	actor ActorContext,
	previousStatus RunStatus,
	previousSessionID string,
) (*Run, error) {
	switch previousStatus {
	case TaskRunStatusClaimed, TaskRunStatusStarting:
	case TaskRunStatusRunning:
		return &run, nil
	default:
		return nil, invalidRunRecoveryTransition(run, previousStatus, recovery.Action)
	}
	if previousSessionID == "" {
		return nil, fmt.Errorf(
			"%w: task run %q cannot recover to running without a session binding",
			ErrInvalidStatusTransition,
			run.ID,
		)
	}

	run.Status = TaskRunStatusRunning
	if run.StartedAt.IsZero() {
		run.StartedAt = m.now().UTC()
	}
	if err := m.store.UpdateTaskRun(ctx, run); err != nil {
		return nil, err
	}
	reconciledTask, err := m.recordRecoveredRun(
		ctx,
		taskRecord.ID,
		run,
		recovery,
		actor,
		previousStatus,
		previousSessionID,
	)
	if err != nil {
		return nil, err
	}
	m.dispatchTaskRunLeaseRecovered(
		ctx,
		run,
		reconciledTask,
		actor,
		previousStatus,
		previousSessionID,
		recovery,
	)
	return &run, nil
}

func (m *Service) recoverRunByFailure(
	ctx context.Context,
	taskRecord Task,
	run Run,
	recovery RunBootRecovery,
	actor ActorContext,
	previousStatus RunStatus,
	previousSessionID string,
) (*Run, error) {
	failedRun, err := m.failRunRecordWithOptions(ctx, taskRecord, run, RunFailure{
		Error:    runBootRecoveryError(run, recovery),
		Metadata: runBootRecoveryMetadata(run, recovery),
	}, actor, failRunRecordOptions{stopTerminalSession: false})
	if err != nil {
		return nil, err
	}
	reconciledTask, err := m.recordRecoveredRun(
		ctx,
		taskRecord.ID,
		*failedRun,
		recovery,
		actor,
		previousStatus,
		previousSessionID,
	)
	if err != nil {
		return nil, err
	}
	m.dispatchTaskRunLeaseRecovered(
		ctx,
		*failedRun,
		reconciledTask,
		actor,
		previousStatus,
		previousSessionID,
		recovery,
	)
	if err := m.stopRecoveredRunSession(ctx, *failedRun, recovery); err != nil {
		return nil, err
	}
	return failedRun, nil
}

func invalidRunRecoveryTransition(
	run Run,
	previousStatus RunStatus,
	action RunBootRecoveryAction,
) error {
	return fmt.Errorf(
		"%w: task run %q cannot recover from %q via %q",
		ErrInvalidStatusTransition,
		run.ID,
		previousStatus,
		action,
	)
}

func (m *Service) recordRecoveredRun(
	ctx context.Context,
	taskID string,
	run Run,
	recovery RunBootRecovery,
	actor ActorContext,
	previousStatus RunStatus,
	previousSessionID string,
) (Task, error) {
	reconciledTask, err := m.reconcileTaskCascade(ctx, taskID, actor)
	if err != nil {
		return Task{}, err
	}
	if err := m.recordTaskEvent(ctx, run.TaskID, run.ID, taskEventRunRecovered, actor, recoveredRunPayload{
		Action:         recovery.Action,
		PreviousStatus: previousStatus,
		Status:         run.Status,
		TaskStatus:     reconciledTask.Status,
		Reason:         recovery.Reason,
		SessionID:      previousSessionID,
		SessionState:   recovery.SessionState,
		Classification: recovery.Classification,
		Detail:         recovery.Detail,
	}); err != nil {
		return Task{}, err
	}
	return reconciledTask, nil
}
