package task

import (
	"context"
	"fmt"
	"strings"
)

func (m *Service) startCoordinatorRun(
	ctx context.Context,
	_ Task,
	run Run,
	req StartRun,
	actor ActorContext,
) (*Run, error) {
	if err := m.validateCoordinatorRuntime(run.ID, req); err != nil {
		return nil, err
	}
	if !VerifyClaimToken(req.ClaimToken, run.ClaimTokenHash) {
		return nil, fmt.Errorf("%w: coordinator run %q claim token mismatch", ErrInvalidClaimToken, run.ID)
	}

	lifecycleCtx := taskRunLifecycleContext(ctx)
	if err := m.preflightCoordinatorRunStarting(run, actor); err != nil {
		return nil, err
	}
	startingAt := m.now().UTC()
	startingSettlement, err := m.commitNominalRunSettlement(
		lifecycleCtx,
		"transition coordinator run starting",
		taskEventRunStarting,
		actor,
		startingAt,
		func(store runMutationStore) (NominalRunMutationResult, error) {
			return store.TransitionRunStarting(
				lifecycleCtx,
				NewRunStartingMutation(run),
			)
		},
		transitionRunPayload,
	)
	if err != nil {
		return nil, err
	}
	run = startingSettlement.mutation.Run

	if err := m.preflightCoordinatorRunStarted(run, actor); err != nil {
		return nil, err
	}
	runningAt := m.now().UTC()
	runningSettlement, err := m.commitNominalRunSettlement(
		lifecycleCtx,
		"transition coordinator run running",
		taskEventRunStarted,
		actor,
		runningAt,
		func(store runMutationStore) (NominalRunMutationResult, error) {
			return store.TransitionRunRunning(
				lifecycleCtx,
				NewRunRunningMutation(run, runningAt),
			)
		},
		transitionRunPayload,
	)
	if err != nil {
		return nil, err
	}
	run = runningSettlement.mutation.Run

	plan, err := m.coordinatorRunner.Run(lifecycleCtx, RunID(run.ID))
	if err != nil {
		failedRun, failErr := m.FailRunLease(lifecycleCtx, LeaseFailure{
			RunID:      run.ID,
			ClaimToken: req.ClaimToken,
			Failure:    coordinatorRunFailure(err),
			Now:        m.now().UTC(),
		}, actor)
		if failErr != nil {
			return nil, errorsJoin(err, failErr)
		}
		return failedRun, fmt.Errorf("task: coordinator run %q failed: %w", run.ID, err)
	}
	plan = plan.Normalize()
	if err := m.validateCoordinatorPlan(plan, "coordinator_completion.plan"); err != nil {
		return nil, err
	}
	return m.completeCoordinatorRun(lifecycleCtx, run, req.ClaimToken, plan, actor)
}

func (m *Service) applyCoordinatorPostCommit(
	ctx context.Context,
	parentCloses []CoordinatorParentCloseSpec,
	actor ActorContext,
) error {
	if len(parentCloses) == 0 {
		return nil
	}
	if m.coordinatorPostCommit == nil {
		return fmt.Errorf("%w: coordinator parent-close handler is required", ErrValidation)
	}
	return m.coordinatorPostCommit.ApplyCoordinatorPostCommit(ctx, parentCloses, actor)
}

func (m *Service) armCoordinatorTimers(
	ctx context.Context,
	timers []CoordinatorTimerSpec,
	actor ActorContext,
) error {
	if len(timers) == 0 {
		return nil
	}
	if m.coordinatorTimerArmer == nil {
		keys := make([]string, 0, len(timers))
		for _, timer := range timers {
			keys = append(keys, timer.Normalize().IdempotencyKey)
		}
		return fmt.Errorf(
			"%w: coordinator timer armer is required for timers %q",
			ErrValidation,
			strings.Join(keys, ", "),
		)
	}
	var armErrs []error
	for _, timer := range timers {
		if err := m.coordinatorTimerArmer.ArmCoordinatorTimer(ctx, timer.Normalize(), actor); err != nil {
			armErrs = append(armErrs, fmt.Errorf("task: arm coordinator timer %q: %w", timer.IdempotencyKey, err))
		}
	}
	return errorsJoin(armErrs...)
}

func (m *Service) validateCoordinatorRuntime(runID string, req StartRun) error {
	if m.coordinatorRunner == nil {
		return fmt.Errorf("%w: coordinator runner is required", ErrValidation)
	}
	if m.generationFinalizer == nil {
		return fmt.Errorf("%w: generation state finalizer is required", ErrValidation)
	}
	if strings.TrimSpace(req.ClaimToken) == "" {
		return fmt.Errorf(
			"%w: coordinator run %q requires claim_token",
			ErrInvalidClaimToken,
			runID,
		)
	}
	return nil
}

func (m *Service) recordCoordinatorCompletionEvents(
	ctx context.Context,
	result *CoordinatorCompletionResult,
	enqueuedEventIDs []string,
	actor ActorContext,
) error {
	if result == nil {
		return fmt.Errorf("%w: coordinator completion result is required", ErrValidation)
	}
	publicationCtx, publicationCancel := completedSettlementPublicationContext(ctx)
	defer publicationCancel()

	var publicationErrs []error
	if result.Settlement == nil {
		return fmt.Errorf("%w: coordinator completion settlement is required", ErrValidation)
	}
	if strings.TrimSpace(result.CompletionEvent.ID) == "" {
		return fmt.Errorf("%w: coordinator completion event is required", ErrValidation)
	}
	m.publishTaskEventsAfterCommand(publicationCtx, []Event{result.CompletionEvent})
	completedTask, err := m.publishCompletedRunSettlement(publicationCtx, result.Settlement, actor)
	if err != nil {
		publicationErrs = append(publicationErrs, err)
	}
	if strings.TrimSpace(completedTask.ID) == "" {
		completedTask, err = m.store.GetTask(publicationCtx, result.Run.TaskID)
		if err != nil {
			publicationErrs = append(publicationErrs, fmt.Errorf(
				"task: load committed coordinator task %q for publication: %w",
				result.Run.TaskID,
				err,
			))
		}
	}
	m.dispatchTaskRunCompleted(publicationCtx, result.Run, completedTask, actor)
	if err := m.publishCoordinatorEnqueuedRuns(
		publicationCtx,
		result.EnqueuedRuns,
		enqueuedEventIDs,
		actor,
	); err != nil {
		publicationErrs = append(publicationErrs, err)
	}
	return errorsJoin(publicationErrs...)
}
