package task

import (
	"context"
	"strings"
	"time"
)

func (m *Service) reconcileTaskSettlementCascadeWithStore(
	ctx context.Context,
	store DeleteTaskMutationStore,
	taskID string,
	actor ActorContext,
	commandAt time.Time,
) (Task, []StatusTransition, error) {
	record, err := store.GetTask(ctx, taskID)
	if err != nil {
		return Task{}, nil, err
	}

	reconciled, transition, changed, err := m.reconcileTaskMutationWithStoreAt(
		ctx,
		store,
		record,
		actor,
		commandAt,
	)
	if err != nil {
		return Task{}, nil, err
	}
	if !changed {
		return reconciled, nil, nil
	}

	transitions := []StatusTransition{transition}
	dependentTransitions, err := m.reconcileDependentTaskSettlementsWithStore(
		ctx,
		store,
		taskID,
		map[string]struct{}{taskID: {}},
		actor,
		commandAt,
	)
	if err != nil {
		return Task{}, nil, err
	}
	return reconciled, append(transitions, dependentTransitions...), nil
}

func (m *Service) reconcileDependentTaskSettlementsWithStore(
	ctx context.Context,
	store DeleteTaskMutationStore,
	taskID string,
	visited map[string]struct{},
	actor ActorContext,
	commandAt time.Time,
) ([]StatusTransition, error) {
	dependents, err := store.ListDependents(ctx, taskID)
	if err != nil {
		return nil, err
	}

	transitions := make([]StatusTransition, 0)
	for _, dependent := range dependents {
		dependentTaskID := strings.TrimSpace(dependent.TaskID)
		if _, seen := visited[dependentTaskID]; seen {
			continue
		}
		visited[dependentTaskID] = struct{}{}

		record, err := store.GetTask(ctx, dependentTaskID)
		if err != nil {
			return nil, err
		}
		_, transition, changed, err := m.reconcileTaskMutationWithStoreAt(
			ctx,
			store,
			record,
			actor,
			commandAt,
		)
		if err != nil {
			return nil, err
		}
		if !changed {
			continue
		}
		transitions = append(transitions, transition)
		nested, err := m.reconcileDependentTaskSettlementsWithStore(
			ctx,
			store,
			dependentTaskID,
			visited,
			actor,
			commandAt,
		)
		if err != nil {
			return nil, err
		}
		transitions = append(transitions, nested...)
	}
	return transitions, nil
}
