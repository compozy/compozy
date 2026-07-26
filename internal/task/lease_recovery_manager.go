package task

import (
	"context"
)

// RecoverExpiredRunLeases requeues stale task-run leases and emits lease-expiration hooks once.
func (m *Service) RecoverExpiredRunLeases(
	ctx context.Context,
	recovery ExpiredLeaseRecovery,
	actor ActorContext,
) ([]ExpiredLeaseRecoveryResult, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return nil, err
	}
	normalized, err := recovery.Normalize(m.now().UTC())
	if err != nil {
		return nil, err
	}
	normalized.Actor = actor
	results, err := m.store.RecoverExpiredRunLeases(ctx, normalized)
	if err != nil {
		return nil, err
	}
	for idx := range results {
		if err := m.handleExpiredRunLeaseRecovery(ctx, &results[idx], actor); err != nil {
			return nil, err
		}
	}
	return results, nil
}

func (m *Service) handleExpiredRunLeaseRecovery(
	ctx context.Context,
	result *ExpiredLeaseRecoveryResult,
	actor ActorContext,
) error {
	if result.Run.IsNetworkWake() {
		m.dispatchTaskRunLeaseExpired(ctx, result.Run, Task{}, actor, result)
		m.dispatchTaskRunLeaseRecoveredFromExpiration(ctx, result.Run, Task{}, actor, result)
		return nil
	}
	reconciledTask, err := m.reconcileTaskCascade(ctx, result.Run.TaskID, actor)
	if err != nil {
		return err
	}
	m.dispatchTaskRunLeaseExpired(ctx, result.Run, reconciledTask, actor, result)
	if result.Exhausted {
		return m.handleExhaustedRunLeaseRecovery(ctx, result, reconciledTask, actor)
	}
	m.dispatchTaskRunLeaseRecoveredFromExpiration(ctx, result.Run, reconciledTask, actor, result)
	return nil
}

func (m *Service) handleExhaustedRunLeaseRecovery(
	ctx context.Context,
	result *ExpiredLeaseRecoveryResult,
	reconciledTask Task,
	actor ActorContext,
) error {
	m.dispatchTaskNeedsAttention(
		ctx,
		reconciledTask,
		actor,
		result.Run.Error,
		result.Run.EndedAt,
		nil,
	)
	return nil
}
