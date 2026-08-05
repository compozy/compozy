package task

import (
	"context"
	"strings"
)

func (m *Service) completeCoordinatorRun(
	ctx context.Context,
	run Run,
	claimToken string,
	plan CoordinatorCompletionPlan,
	actor ActorContext,
) (*Run, error) {
	if err := m.preflightCoordinatorRunCompletion(run, actor); err != nil {
		return nil, err
	}
	publicationEventIDs, err := m.reserveTaskEventIDs(coordinatorPublicationEventCount(plan))
	if err != nil {
		return nil, err
	}

	result, completionErr := m.store.CompleteCoordinatorAndEnqueueNext(ctx, CoordinatorCompletion{
		RunID:      run.ID,
		ClaimToken: claimToken,
		Plan:       plan,
		Actor:      actor,
		Now:        m.now().UTC(),
	}, m.generationFinalizer)
	if strings.TrimSpace(result.Run.ID) == "" {
		return nil, completionErr
	}
	var postCommitErr error
	var timerErr error
	if !result.PlanSuperseded {
		postCommitErr = m.applyCoordinatorPostCommit(ctx, plan.ParentCloses, actor)
		timerErr = m.armCoordinatorTimers(ctx, plan.PostCommitTimers, actor)
	}
	eventErr := m.recordCoordinatorCompletionEvents(ctx, &result, publicationEventIDs, actor)
	m.dispatchCoordinatorTerminal(ctx, &result, actor)
	return &result.Run, errorsJoin(completionErr, postCommitErr, timerErr, eventErr)
}

func coordinatorPublicationEventCount(plan CoordinatorCompletionPlan) int {
	count := len(plan.NodeRuns)
	if plan.NextCoordinator != nil {
		count++
	}
	return count
}
