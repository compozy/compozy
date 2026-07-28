package task

import (
	"context"
	"time"
)

func (m *Service) recoverNetworkWakeOnBoot(
	ctx context.Context,
	run Run,
	recovery RunBootRecovery,
	actor ActorContext,
) (*Run, error) {
	previousStatus := run.Status.Normalize()
	previousSessionID := run.SessionID
	switch recovery.Action.Normalize() {
	case RunBootRecoveryRequeue:
		switch previousStatus {
		case TaskRunStatusClaimed, TaskRunStatusStarting, TaskRunStatusRunning:
		default:
			return nil, invalidRunRecoveryTransition(run, previousStatus, recovery.Action)
		}
		run = requeueSessionRunLease(run)
	case RunBootRecoveryMarkRunning:
		switch previousStatus {
		case TaskRunStatusClaimed, TaskRunStatusStarting:
			run.Status = TaskRunStatusRunning
			if run.StartedAt.IsZero() {
				run.StartedAt = m.now().UTC()
			}
		case TaskRunStatusRunning:
			return &run, nil
		default:
			return nil, invalidRunRecoveryTransition(run, previousStatus, recovery.Action)
		}
		if previousSessionID == "" {
			return nil, invalidRunRecoveryTransition(run, previousStatus, recovery.Action)
		}
	case RunBootRecoveryFail:
		switch previousStatus {
		case TaskRunStatusStarting, TaskRunStatusRunning:
		default:
			return nil, invalidRunRecoveryTransition(run, previousStatus, recovery.Action)
		}
		run.Status = TaskRunStatusFailed
		run.Error = runBootRecoveryError(run, recovery)
		run.Result = nil
		run.LeaseUntil = time.Time{}
		run.HeartbeatAt = time.Time{}
		run.EndedAt = m.now().UTC()
	default:
		return nil, invalidRunRecoveryTransition(run, previousStatus, recovery.Action)
	}
	if err := m.store.UpdateTaskRun(ctx, run); err != nil {
		return nil, err
	}
	m.dispatchTaskRunLeaseRecovered(
		ctx,
		run,
		Task{},
		actor,
		previousStatus,
		previousSessionID,
		recovery,
	)
	return &run, nil
}
