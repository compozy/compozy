package globaldb

import (
	"github.com/compozy/compozy/internal/loop"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func shouldApplyCoordinatorBudgetExceededBoundary(
	plan taskpkg.CoordinatorCompletionPlan,
	budgetExceeded bool,
) bool {
	return !plan.Yield && plan.Terminal == nil && budgetExceeded
}

func coordinatorPlanHasContinuation(plan taskpkg.CoordinatorCompletionPlan) bool {
	return len(plan.NodeRuns) > 0 || plan.NextCoordinator != nil
}

func shouldDeferCoordinatorBoundary(
	plan taskpkg.CoordinatorCompletionPlan,
	loopRun loop.Run,
	budgetExceeded bool,
) bool {
	if !plan.GenerationInFlight {
		return false
	}
	return plan.Terminal != nil || budgetExceeded || loopRun.PauseRequested
}
