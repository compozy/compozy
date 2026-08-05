package task

import (
	"context"

	"fmt"

	"time"
)

func (m *Service) recoverRunByRequeue(
	ctx context.Context,
	_ Task,
	run Run,
	recovery RunBootRecovery,
	actor ActorContext,
	previousStatus RunStatus,
	previousSessionID string,
) (*Run, error) {
	if previousStatus != TaskRunStatusClaimed {
		return nil, invalidRunRecoveryTransition(run, previousStatus, recovery.Action)
	}
	if err := m.preflightRecoveredRunEvent(
		run,
		recovery,
		actor,
		previousStatus,
		previousSessionID,
		TaskRunStatusQueued,
	); err != nil {
		return nil, err
	}

	commandAt := m.now().UTC()
	settlement, err := m.commitNominalRunSettlement(
		ctx,
		"requeue task run during boot recovery",
		taskEventRunRecovered,
		actor,
		commandAt,
		func(store runMutationStore) (NominalRunMutationResult, error) {
			return store.RecoverTaskRunOnBoot(
				ctx,
				NewRunBootRecoveryMutation(run, RunBootRecoveryRequeue, time.Time{}),
			)
		},
		recoveredRunEventPayload(recovery, previousStatus, previousSessionID),
	)
	if err != nil {
		return nil, err
	}
	m.dispatchTaskRunLeaseRecovered(
		ctx,
		settlement.mutation.Run,
		settlement.task,
		actor,
		previousStatus,
		previousSessionID,
		recovery,
	)
	return &settlement.mutation.Run, nil
}

func (m *Service) recoverRunByMarkRunning(
	ctx context.Context,
	_ Task,
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
	if err := m.preflightRecoveredRunEvent(
		run,
		recovery,
		actor,
		previousStatus,
		previousSessionID,
		TaskRunStatusRunning,
	); err != nil {
		return nil, err
	}

	commandAt := m.now().UTC()
	startedAt := run.StartedAt
	if startedAt.IsZero() {
		startedAt = commandAt
	}
	settlement, err := m.commitNominalRunSettlement(
		ctx,
		"mark task run running during boot recovery",
		taskEventRunRecovered,
		actor,
		commandAt,
		func(store runMutationStore) (NominalRunMutationResult, error) {
			return store.RecoverTaskRunOnBoot(
				ctx,
				NewRunBootRecoveryMutation(run, RunBootRecoveryMarkRunning, startedAt),
			)
		},
		recoveredRunEventPayload(recovery, previousStatus, previousSessionID),
	)
	if err != nil {
		return nil, err
	}
	m.dispatchTaskRunLeaseRecovered(
		ctx,
		settlement.mutation.Run,
		settlement.task,
		actor,
		previousStatus,
		previousSessionID,
		recovery,
	)
	return &settlement.mutation.Run, nil
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
	switch previousStatus {
	case TaskRunStatusStarting, TaskRunStatusRunning:
	default:
		return nil, requireRunTransition(run, TaskRunStatusFailed)
	}
	if recovery.StopRequired && previousSessionID == "" {
		return nil, fmt.Errorf(
			"%w: task run %q cannot stop a recovered process without a session binding",
			ErrInvalidStatusTransition,
			run.ID,
		)
	}
	if err := m.preflightRecoveredRunEvent(
		run,
		recovery,
		actor,
		previousStatus,
		previousSessionID,
		TaskRunStatusFailed,
	); err != nil {
		return nil, err
	}
	metadata, err := runBootRecoveryMetadata(run, recovery)
	if err != nil {
		return nil, err
	}

	failedRun, err := m.failRunRecordWithOptions(ctx, taskRecord, run, RunFailure{
		Error:    runBootRecoveryError(run, recovery),
		Metadata: metadata,
	}, actor, failRunRecordOptions{
		stopTerminalSession: recovery.StopRequired,
		recoveryAudit: &bootRecoveryAudit{
			recovery:          recovery,
			previousStatus:    previousStatus,
			previousSessionID: previousSessionID,
		},
	})
	if err != nil {
		return nil, err
	}
	reconciledTask, err := m.store.GetTask(ctx, taskRecord.ID)
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
