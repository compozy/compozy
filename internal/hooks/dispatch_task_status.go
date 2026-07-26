package hooks

import "context"

// DispatchTaskStatusChanged runs the task.status_changed hook dispatch.
func (h *Hooks) DispatchTaskStatusChanged(
	ctx context.Context,
	payload TaskStatusChangedPayload,
) (TaskStatusChangedPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookTaskStatusChanged,
		payload,
		dispatchConfig[TaskStatusChangedPayload, TaskObservationPatch]{
			match: matchTaskStatusChanged,
			apply: applyNoop[TaskStatusChangedPayload, TaskObservationPatch],
		},
	)
}

func matchTaskStatusChanged(matcher HookMatcher, payload TaskStatusChangedPayload) bool {
	return matcher.MatchesTask(payload.TaskContext)
}
