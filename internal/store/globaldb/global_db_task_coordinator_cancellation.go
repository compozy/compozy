package globaldb

import (
	"context"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func protectCanceledLoopFromStaleCoordinator(
	ctx context.Context,
	exec taskSQLExecutor,
	completion *taskpkg.CoordinatorCompletion,
	loopRunID string,
) (bool, error) {
	if completion == nil {
		return false, fmt.Errorf("%w: coordinator completion is required", taskpkg.ErrValidation)
	}
	run, err := getLoopRunByIDWithExecutor(ctx, exec, looppkg.RunID(strings.TrimSpace(loopRunID)))
	if err != nil {
		return false, err
	}
	if run.Status != looppkg.StatusCanceled {
		return false, nil
	}
	plan := taskpkg.CoordinatorCompletionPlan{
		Snapshot: taskpkg.GenerationSnapshot{
			LoopRunID:  string(run.ID),
			Generation: max(1, run.Generation),
		},
		Yield: true,
	}
	if !run.Status.Terminal() {
		plan.GenerationInFlight = true
		plan.PostCommitWakes = []taskpkg.CoordinatorWakeSpec{{
			LoopRunID: string(run.ID),
			IdempotencyKey: fmt.Sprintf(
				"loop.coordinator.cancel.%s.%s",
				run.ID,
				strings.TrimSpace(completion.RunID),
			),
		}}
	}
	completion.Plan = plan
	return true, nil
}

func requireCanceledCoordinatorCompletion(run taskpkg.Run, rawToken string) error {
	if strings.TrimSpace(run.ClaimTokenHash) == "" {
		return fmt.Errorf(
			"%w: task run %q has no retained claim token hash",
			taskpkg.ErrInvalidClaimToken,
			run.ID,
		)
	}
	if !taskpkg.VerifyClaimToken(rawToken, run.ClaimTokenHash) {
		return fmt.Errorf("%w: task run %q token mismatch", taskpkg.ErrInvalidClaimToken, run.ID)
	}
	if run.Status.Normalize() != taskpkg.TaskRunStatusCanceled {
		return fmt.Errorf(
			"%w: task run %q is not canceled",
			taskpkg.ErrInvalidStatusTransition,
			run.ID,
		)
	}
	return nil
}
