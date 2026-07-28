package task

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

func (m *Service) ensureTaskDeleteAllowedWithStore(
	ctx context.Context,
	store DeleteTaskMutationStore,
	record Task,
) error {
	childCount, err := store.CountDirectChildren(ctx, record.ID)
	if err != nil {
		return fmt.Errorf("task: count child tasks for delete %q: %w", record.ID, err)
	}
	if childCount > 0 {
		return fmt.Errorf(
			"%w: task %q has %d child tasks; delete children first",
			ErrValidation,
			record.ID,
			childCount,
		)
	}

	runs, err := store.ListTaskRuns(ctx, RunQuery{TaskID: record.ID})
	if err != nil {
		return fmt.Errorf("task: list runs for delete %q: %w", record.ID, err)
	}
	if hasOpenRun(runs) {
		return fmt.Errorf(
			"%w: task %q has active or queued runs; cancel or finish them first",
			ErrValidation,
			record.ID,
		)
	}

	return nil
}

func uniqueDependentTaskIDs(dependents []Dependency) []string {
	if len(dependents) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(dependents))
	ids := make([]string, 0, len(dependents))
	for _, dependent := range dependents {
		taskID := strings.TrimSpace(dependent.TaskID)
		if taskID == "" {
			continue
		}
		if _, ok := seen[taskID]; ok {
			continue
		}
		seen[taskID] = struct{}{}
		ids = append(ids, taskID)
	}

	sort.Strings(ids)
	return ids
}

func (m *Service) hasUnresolvedDependenciesReadOnlyWithStore(
	ctx context.Context,
	store DeleteTaskMutationStore,
	dependencies []Dependency,
	visited map[string]struct{},
) (bool, error) {
	ids, err := m.unresolvedDependencyTaskIDsReadOnlyWithStore(ctx, store, dependencies, visited)
	if err != nil {
		return false, err
	}
	return len(ids) > 0, nil
}

func (m *Service) unresolvedDependencyTaskIDsReadOnlyWithStore(
	ctx context.Context,
	store DeleteTaskMutationStore,
	dependencies []Dependency,
	visited map[string]struct{},
) ([]string, error) {
	unresolved := make([]string, 0)
	for _, dependency := range dependencies {
		dependencyTaskID := strings.TrimSpace(dependency.DependsOnTaskID)
		record, err := store.GetTask(ctx, dependencyTaskID)
		if err != nil {
			return nil, err
		}
		nestedDependencies, err := store.ListDependencies(ctx, dependencyTaskID)
		if err != nil {
			return nil, err
		}
		nestedRuns, err := store.ListTaskRuns(ctx, RunQuery{TaskID: dependencyTaskID})
		if err != nil {
			return nil, err
		}
		status, err := m.canonicalTaskStatusReadOnlyWithStore(
			ctx,
			store,
			record,
			nestedDependencies,
			nestedRuns,
			visited,
		)
		if err != nil {
			return nil, err
		}
		if status.Normalize() != TaskStatusCompleted {
			unresolved = append(unresolved, dependencyTaskID)
		}
	}
	sort.Strings(unresolved)
	return unresolved, nil
}

func (m *Service) blockedReasonsForSnapshot(
	ctx context.Context,
	record Task,
	dependencies []Dependency,
) ([]BlockedReason, error) {
	reasons := make([]BlockedReason, 0)
	dependencyIDs, err := m.unresolvedDependencyTaskIDsReadOnlyWithStore(
		ctx,
		m.store,
		dependencies,
		make(map[string]struct{}, len(dependencies)+1),
	)
	if err != nil {
		return nil, err
	}
	if len(dependencyIDs) > 0 {
		reasons = append(reasons, BlockedReason{
			Source:           BlockedSourceDependency,
			DependsOnTaskIDs: dependencyIDs,
		})
	}
	if taskApprovalBlocked(record) {
		reasons = append(reasons, BlockedReason{Source: BlockedSourceApproval})
	}
	if taskPausedBlocked(record) {
		reasons = append(reasons, BlockedReason{
			Source: BlockedSourcePaused,
			Reason: RedactClaimTokens(strings.TrimSpace(record.PausedReason)),
		})
	}
	blocks, err := m.store.ListTaskBlocks(ctx, record.ID, false)
	if err != nil {
		return nil, err
	}
	for _, block := range blocks {
		reasons = append(reasons, BlockedReason{
			Source:  BlockedSourceBlock,
			Kind:    block.Kind.Normalize(),
			Reason:  RedactClaimTokens(strings.TrimSpace(block.Reason)),
			BlockID: strings.TrimSpace(block.ID),
		})
	}
	return reasons, nil
}

func (m *Service) reconcileDependentTasks(
	ctx context.Context,
	taskID string,
	visited map[string]struct{},
	actor ActorContext,
) error {
	return m.reconcileDependentTasksWithStore(ctx, m.store, taskID, visited, actor)
}

func (m *Service) reconcileDependentTasksWithStore(
	ctx context.Context,
	store DeleteTaskMutationStore,
	taskID string,
	visited map[string]struct{},
	actor ActorContext,
) error {
	dependents, err := store.ListDependents(ctx, taskID)
	if err != nil {
		return err
	}

	for _, dependent := range dependents {
		dependentTaskID := strings.TrimSpace(dependent.TaskID)
		if _, seen := visited[dependentTaskID]; seen {
			continue
		}
		visited[dependentTaskID] = struct{}{}

		previous, err := store.GetTask(ctx, dependentTaskID)
		if err != nil {
			return err
		}
		reconciled, err := m.reconcileTaskWithStore(ctx, store, previous, actor)
		if err != nil {
			return err
		}
		if previous.Status.Normalize() != reconciled.Status.Normalize() {
			if err := m.reconcileDependentTasksWithStore(ctx, store, dependentTaskID, visited, actor); err != nil {
				return err
			}
		}
	}

	return nil
}
