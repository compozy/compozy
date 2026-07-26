package loop

import "github.com/compozy/agh/internal/task"

func noOpCoordinatorPlan(run Run) task.CoordinatorCompletionPlan {
	return terminalCoordinatorPlan(run, &task.CoordinatorTerminal{
		Status: string(StatusNoOp),
		Cause:  string(TransitionCauseContract),
	})
}

func terminalCoordinatorPlan(run Run, terminal *task.CoordinatorTerminal) task.CoordinatorCompletionPlan {
	return task.CoordinatorCompletionPlan{
		Snapshot: task.GenerationSnapshot{
			LoopRunID:  string(run.ID),
			Generation: max(1, run.Generation),
		},
		Terminal: terminal,
	}
}
