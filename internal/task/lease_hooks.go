package task

import (
	"context"
	"strings"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

func (m *Service) dispatchTaskRunLeaseExtended(
	ctx context.Context,
	run Run,
	taskRecord Task,
	actor ActorContext,
) {
	payload := hookspkg.TaskRunLeaseExtendedPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookTaskRunLeaseExtended,
			Timestamp: m.now().UTC(),
		},
		TaskRunContext: m.taskRunHookContext(run, taskRecord, actor),
	}
	_, err := m.taskHooks.DispatchTaskRunLeaseExtended(taskRunObservationHookContext(ctx), payload)
	m.reportTaskRunHookFailure(hookspkg.HookTaskRunLeaseExtended, err, run, taskRecord)
}

func (m *Service) dispatchTaskRunLeaseExpired(
	ctx context.Context,
	run Run,
	taskRecord Task,
	actor ActorContext,
	recovery *ExpiredLeaseRecoveryResult,
) {
	if recovery == nil {
		return
	}
	contextPayload := m.taskRunHookContext(run, taskRecord, actor)
	contextPayload.ReleaseReason = strings.TrimSpace(recovery.Reason)
	contextPayload.LeaseUntil = recovery.PreviousLeaseUntil
	payload := hookspkg.TaskRunLeaseExpiredPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookTaskRunLeaseExpired,
			Timestamp: m.now().UTC(),
		},
		TaskRunContext:    contextPayload,
		PreviousRunStatus: recovery.PreviousRunStatus.Normalize().String(),
		PreviousSessionID: strings.TrimSpace(recovery.PreviousSessionID),
		RecoveryReason:    strings.TrimSpace(recovery.Reason),
	}
	_, err := m.taskHooks.DispatchTaskRunLeaseExpired(taskRunObservationHookContext(ctx), payload)
	m.reportTaskRunHookFailure(hookspkg.HookTaskRunLeaseExpired, err, run, taskRecord)
}

func (m *Service) dispatchTaskRunLeaseRecoveredFromExpiration(
	ctx context.Context,
	run Run,
	taskRecord Task,
	actor ActorContext,
	recovery *ExpiredLeaseRecoveryResult,
) {
	if recovery == nil {
		return
	}
	payload := hookspkg.TaskRunLeaseRecoveredPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookTaskRunLeaseRecovered,
			Timestamp: m.now().UTC(),
		},
		TaskRunContext:    m.taskRunHookContext(run, taskRecord, actor),
		PreviousRunStatus: recovery.PreviousRunStatus.Normalize().String(),
		PreviousSessionID: strings.TrimSpace(recovery.PreviousSessionID),
		RecoveryAction:    string(RunBootRecoveryRequeue),
		RecoveryReason:    strings.TrimSpace(recovery.Reason),
	}
	_, err := m.taskHooks.DispatchTaskRunLeaseRecovered(taskRunObservationHookContext(ctx), payload)
	m.reportTaskRunHookFailure(hookspkg.HookTaskRunLeaseRecovered, err, run, taskRecord)
}

func (m *Service) dispatchTaskRunReleased(
	ctx context.Context,
	run Run,
	taskRecord Task,
	actor ActorContext,
	previous Run,
	reason string,
) {
	contextPayload := m.taskRunHookContext(run, taskRecord, actor)
	contextPayload.ReleaseReason = strings.TrimSpace(reason)
	payload := hookspkg.TaskRunReleasedPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookTaskRunReleased,
			Timestamp: m.now().UTC(),
		},
		TaskRunContext:    contextPayload,
		PreviousRunStatus: previous.Status.Normalize().String(),
		PreviousSessionID: strings.TrimSpace(previous.SessionID),
		RecoveryReason:    strings.TrimSpace(reason),
	}
	_, err := m.taskHooks.DispatchTaskRunReleased(taskRunObservationHookContext(ctx), payload)
	m.reportTaskRunHookFailure(hookspkg.HookTaskRunReleased, err, run, taskRecord)
}

func (m *Service) dispatchTaskRunCompleted(
	ctx context.Context,
	run Run,
	taskRecord Task,
	actor ActorContext,
) {
	payload := hookspkg.TaskRunCompletedPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookTaskRunCompleted,
			Timestamp: m.now().UTC(),
		},
		TaskRunContext: m.taskRunHookContext(run, taskRecord, actor),
	}
	_, err := m.taskHooks.DispatchTaskRunCompleted(taskRunObservationHookContext(ctx), payload)
	m.reportTaskRunHookFailure(hookspkg.HookTaskRunCompleted, err, run, taskRecord)
}

func (m *Service) dispatchTaskRunFailed(
	ctx context.Context,
	run Run,
	taskRecord Task,
	actor ActorContext,
) {
	payload := hookspkg.TaskRunFailedPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookTaskRunFailed,
			Timestamp: m.now().UTC(),
		},
		TaskRunContext: m.taskRunHookContext(run, taskRecord, actor),
	}
	_, err := m.taskHooks.DispatchTaskRunFailed(taskRunObservationHookContext(ctx), payload)
	m.reportTaskRunHookFailure(hookspkg.HookTaskRunFailed, err, run, taskRecord)
}
