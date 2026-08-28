package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type coordinatorPlanSupersededError interface {
	error
	CoordinatorPlanSuperseded()
}

func (m *Service) completeStartedCoordinatorRun(
	ctx context.Context,
	run Run,
	claimToken string,
	plan CoordinatorCompletionPlan,
	actor ActorContext,
) (*Run, error) {
	completed, err := m.completeCoordinatorRun(ctx, run, claimToken, plan, actor)
	if err == nil || completed != nil {
		return completed, err
	}
	if isCoordinatorPlanSuperseded(err) {
		reconciled, reconcileErr := m.completeCoordinatorRun(
			ctx,
			run,
			claimToken,
			supersededCoordinatorPlan(run, plan),
			actor,
		)
		if reconcileErr != nil && reconciled == nil {
			return m.failStartedCoordinatorRun(
				ctx,
				run,
				claimToken,
				actor,
				errors.Join(err, reconcileErr),
			)
		}
		return reconciled, reconcileErr
	}
	return m.failStartedCoordinatorRun(ctx, run, claimToken, actor, err)
}

func isCoordinatorPlanSuperseded(err error) bool {
	marker, ok := errors.AsType[coordinatorPlanSupersededError](err)
	return ok && marker != nil
}

func supersededCoordinatorPlan(run Run, stale CoordinatorCompletionPlan) CoordinatorCompletionPlan {
	loopRunID := strings.TrimSpace(run.LoopRunID)
	return CoordinatorCompletionPlan{
		Snapshot: GenerationSnapshot{
			LoopRunID:  loopRunID,
			Generation: stale.Snapshot.Generation,
		},
		GenerationInFlight: true,
		Yield:              true,
		PostCommitWakes: []CoordinatorWakeSpec{{
			LoopRunID:      loopRunID,
			IdempotencyKey: fmt.Sprintf("loop.coordinator.superseded.%s.%s", loopRunID, run.ID),
		}},
	}
}
