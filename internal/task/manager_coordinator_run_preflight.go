package task

func (m *Service) preflightCoordinatorRunStarting(run Run, actor ActorContext) error {
	return m.preflightTaskEvent(
		run.TaskID,
		run.ID,
		taskEventRunStarting,
		actor,
		runTransitionPayload{
			Status:     TaskRunStatusStarting,
			TaskStatus: TaskStatusNeedsAttention,
		},
	)
}

func (m *Service) preflightCoordinatorRunStarted(run Run, actor ActorContext) error {
	return m.preflightTaskEvent(
		run.TaskID,
		run.ID,
		taskEventRunStarted,
		actor,
		runTransitionPayload{
			Status:     TaskRunStatusRunning,
			TaskStatus: TaskStatusNeedsAttention,
		},
	)
}

func (m *Service) preflightCoordinatorRunCompletion(run Run, actor ActorContext) error {
	return m.preflightTaskEvent(
		run.TaskID,
		run.ID,
		taskEventRunCompleted,
		actor,
		completedRunPayload{
			Status:         TaskRunStatusCompleted,
			TaskStatus:     TaskStatusNeedsAttention,
			Result:         CoordinatorCompletedRunResult(),
			ClaimTokenHash: run.ClaimTokenHash,
		},
	)
}
