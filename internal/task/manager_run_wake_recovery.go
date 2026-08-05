package task

import (
	"context"
	"strings"
)

func (m *Service) recoverNetworkWakeOnBoot(
	ctx context.Context,
	run Run,
	recovery RunBootRecovery,
	actor ActorContext,
) (*Run, error) {
	previousStatus := run.Status.Normalize()
	previousSessionID := strings.TrimSpace(run.SessionID)
	if err := validateNetworkWakeBootRecovery(run, recovery); err != nil {
		return nil, err
	}
	if recovery.Action.Normalize() == RunBootRecoveryMarkRunning &&
		previousStatus == TaskRunStatusRunning {
		return &run, nil
	}

	commandAt := m.now().UTC()
	var mutation NominalRunMutationResult
	err := m.store.WithTaskMutationTransaction(
		ctx,
		"recover network wake on boot",
		func(store runMutationStore) error {
			var mutationErr error
			mutation, mutationErr = store.RecoverNetworkWakeOnBoot(
				ctx,
				NewNetworkWakeBootRecoveryMutation(run, recovery, actor, commandAt),
			)
			return mutationErr
		},
	)
	if err != nil {
		return nil, err
	}
	m.dispatchTaskRunLeaseRecovered(
		ctx,
		mutation.Run,
		Task{},
		actor,
		previousStatus,
		previousSessionID,
		recovery,
	)
	return &mutation.Run, nil
}

func validateNetworkWakeBootRecovery(run Run, recovery RunBootRecovery) error {
	status := run.Status.Normalize()
	switch recovery.Action.Normalize() {
	case RunBootRecoveryRequeue:
		if status != TaskRunStatusClaimed && status != TaskRunStatusStarting && status != TaskRunStatusRunning {
			return invalidRunRecoveryTransition(run, status, recovery.Action)
		}
	case RunBootRecoveryMarkRunning:
		if status != TaskRunStatusClaimed && status != TaskRunStatusStarting && status != TaskRunStatusRunning {
			return invalidRunRecoveryTransition(run, status, recovery.Action)
		}
		if strings.TrimSpace(run.SessionID) == "" {
			return invalidRunRecoveryTransition(run, status, recovery.Action)
		}
	case RunBootRecoveryFail:
		if status != TaskRunStatusStarting && status != TaskRunStatusRunning {
			return invalidRunRecoveryTransition(run, status, recovery.Action)
		}
	default:
		return invalidRunRecoveryTransition(run, status, recovery.Action)
	}
	return nil
}
