package task

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const completionPublicationTimeout = 30 * time.Second

type completionPublicationContextKey struct{}

func (m *Service) publishCompletedLeaseSettlement(
	ctx context.Context,
	settlement *CompletedRunSettlement,
	actor ActorContext,
) (*Run, error) {
	publicationCtx, publicationCancel := completedSettlementPublicationContext(ctx)
	defer publicationCancel()

	run := settlement.Run
	if run.IsNetworkWake() {
		m.dispatchTaskRunCompleted(publicationCtx, run, Task{}, actor)
		return &run, nil
	}
	reconciledTask, publicationErr := m.publishCompletedRunSettlement(publicationCtx, settlement, actor)
	m.dispatchTerminalWake(publicationCtx, reconciledTask, run, actor)
	advisoryCtx, advisoryCancel := context.WithTimeout(publicationCtx, autoEnqueueDispatchTimeout)
	defer advisoryCancel()
	m.recordCompletionHallucinationSuspected(advisoryCtx, run, actor)
	m.dispatchTaskRunCompleted(publicationCtx, run, reconciledTask, actor)
	if !run.IsLoopWorker() {
		autoCtx, cancel := context.WithTimeout(publicationCtx, autoEnqueueDispatchTimeout)
		defer cancel()
		for _, transition := range settlement.StatusTransitions {
			m.autoEnqueueReadyDependents(autoCtx, transition.Task.ID, autoEnqueueTrigger{
				Kind: autoEnqueueTriggerDependencyCompletion,
				Ref:  run.ID,
			}, actor)
		}
	}
	return &run, publicationErr
}

func (m *Service) publishCompletedRunSettlement(
	ctx context.Context,
	settlement *CompletedRunSettlement,
	actor ActorContext,
) (Task, error) {
	publicationCtx, publicationCancel := completedSettlementPublicationContext(ctx)
	defer publicationCancel()

	for _, transition := range settlement.StatusTransitions {
		m.dispatchTaskStatusChangedAfterWrite(
			publicationCtx,
			transition.Task,
			transition.PreviousStatus,
			transition.Task.Status,
			actor,
		)
	}

	rolledUpTasks := make(map[string]Task, len(settlement.StatusTransitions))
	for _, transition := range settlement.StatusTransitions {
		rolledUpTasks[transition.Task.ID] = transition.Task
	}
	for _, run := range settlement.RolledUpRuns {
		rolledUpTask, ok := rolledUpTasks[run.TaskID]
		if !ok {
			continue
		}
		m.dispatchTerminalWake(publicationCtx, rolledUpTask, run, actor)
		m.dispatchTaskRunCompleted(publicationCtx, run, rolledUpTask, actor)
	}

	var publicationErrs []error
	for _, transition := range settlement.StatusTransitions {
		if err := m.reconcileDependentTasks(
			publicationCtx,
			transition.Task.ID,
			map[string]struct{}{transition.Task.ID: {}},
			actor,
		); err != nil {
			publicationErrs = append(publicationErrs, fmt.Errorf(
				"task: reconcile dependents after committed task %q transition: %w",
				transition.Task.ID,
				err,
			))
		}
	}

	return settlement.Task, errors.Join(publicationErrs...)
}

func completedSettlementPublicationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if isCompletedSettlementPublicationContext(ctx) {
		return ctx, func() {}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	bounded, cancel := context.WithTimeout(context.WithoutCancel(ctx), completionPublicationTimeout)
	return context.WithValue(bounded, completionPublicationContextKey{}, true), cancel
}

func isCompletedSettlementPublicationContext(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	marked, ok := ctx.Value(completionPublicationContextKey{}).(bool)
	return ok && marked
}
