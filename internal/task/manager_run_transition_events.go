package task

func (m *Service) preflightRunTransition(
	run Run,
	eventType string,
	status RunStatus,
	actor ActorContext,
) error {
	return m.preflightTaskEvent(
		run.TaskID,
		run.ID,
		eventType,
		actor,
		runTransitionPayload{
			Status:     status,
			TaskStatus: TaskStatusNeedsAttention,
			SessionID:  run.SessionID,
		},
	)
}
