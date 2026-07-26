package task

import (
	"context"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

func (m *Service) dispatchTaskStatusChanged(
	ctx context.Context,
	taskRecord Task,
	from Status,
	to Status,
	actor ActorContext,
) {
	if from.Normalize() == to.Normalize() {
		return
	}
	payload := hookspkg.TaskStatusChangedPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookTaskStatusChanged,
			Timestamp: m.now().UTC(),
		},
		TaskContext: m.taskHookContext(taskRecord, actor, nil),
		FromStatus:  string(from.Normalize()),
		ToStatus:    string(to.Normalize()),
	}
	_, err := m.taskHooks.DispatchTaskStatusChanged(taskRunObservationHookContext(ctx), payload)
	m.reportTaskHookFailure(hookspkg.HookTaskStatusChanged, err, taskRecord)
}

func (m *Service) dispatchTaskStatusChangedAfterWrite(
	ctx context.Context,
	taskRecord Task,
	from Status,
	to Status,
	actor ActorContext,
) {
	if _, postCommit := m.store.(EventCommitObserverStore); postCommit {
		return
	}
	m.dispatchTaskStatusChanged(ctx, taskRecord, from, to, actor)
}
