package task

import "context"

type taskRunMutationSettlement struct {
	Run               Run
	Task              Task
	StatusTransitions []StatusTransition
	Events            []Event
}

func (m *Service) completeRunMutationSettlementWithStore(
	ctx context.Context,
	store runMutationStore,
	mutation TerminalRunMutation,
	actor ActorContext,
) (CompletedRunSettlement, error) {
	run, err := store.TransitionTerminalRun(ctx, mutation)
	if err != nil {
		return CompletedRunSettlement{}, err
	}
	taskRecord, transitions, err := m.reconcileTaskSettlementCascadeWithStore(
		ctx,
		store,
		run.TaskID,
		actor,
		run.EndedAt,
	)
	if err != nil {
		return CompletedRunSettlement{}, err
	}
	hierarchy, err := store.SettleCompletedTaskHierarchy(
		ctx,
		run.TaskID,
		actor,
		run.EndedAt,
	)
	if err != nil {
		return CompletedRunSettlement{}, err
	}
	return CompletedRunSettlement{
		Run:               run,
		Task:              taskRecord,
		StatusTransitions: append(transitions, hierarchy.StatusTransitions...),
		RolledUpRuns:      hierarchy.RolledUpRuns,
	}, nil
}

func (m *Service) publishTaskRunMutationSettlement(
	ctx context.Context,
	settlement *taskRunMutationSettlement,
	actor ActorContext,
) {
	m.publishLeaseSettlement(ctx, settlement.Events, settlement.StatusTransitions, actor)
	m.dispatchTerminalWake(ctx, settlement.Task, settlement.Run, actor)
}
