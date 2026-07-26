package task

import (
	"context"

	"fmt"

	"strings"
)

// AddDependency adds one dependency edge through the manager, reconciles the
// task status, and records the canonical audit event.
func (m *Service) AddDependency(ctx context.Context, spec AddDependency, actor ActorContext) error {
	if err := requireWriteAuthority(actor); err != nil {
		return err
	}

	normalizedSpec, err := normalizeAddDependencySpec(spec)
	if err != nil {
		return err
	}
	if _, err := m.loadAuthorizedTask(ctx, m.store, normalizedSpec.TaskID, actor); err != nil {
		return err
	}
	if _, err := m.loadAuthorizedTask(ctx, m.store, normalizedSpec.DependsOnTaskID, actor); err != nil {
		return err
	}
	if err := m.store.CreateDependency(ctx, Dependency{
		TaskID:          normalizedSpec.TaskID,
		DependsOnTaskID: normalizedSpec.DependsOnTaskID,
		Kind:            normalizedSpec.Kind,
	}); err != nil {
		return err
	}

	record, err := m.reconcileTaskCascade(ctx, normalizedSpec.TaskID, actor)
	if err != nil {
		return err
	}
	return m.recordTaskEvent(
		ctx,
		normalizedSpec.TaskID,
		"",
		taskEventDependencyAdded,
		actor,
		dependencyTaskPayload{
			DependsOnTaskID: normalizedSpec.DependsOnTaskID,
			Kind:            normalizedSpec.Kind,
			Status:          record.Status,
		},
	)
}

// RemoveDependency deletes one dependency edge through the manager, reconciles
// the task status, and records the canonical audit event.
func (m *Service) RemoveDependency(
	ctx context.Context,
	taskID string,
	dependsOnID string,
	actor ActorContext,
) error {
	if err := requireWriteAuthority(actor); err != nil {
		return err
	}

	trimmedTaskID := strings.TrimSpace(taskID)
	if trimmedTaskID == "" {
		return fmt.Errorf("%w: task id is required", ErrValidation)
	}
	trimmedDependsOnID := strings.TrimSpace(dependsOnID)
	if trimmedDependsOnID == "" {
		return fmt.Errorf("%w: depends_on_task_id is required", ErrValidation)
	}
	if _, err := m.loadAuthorizedTask(ctx, m.store, trimmedTaskID, actor); err != nil {
		return err
	}
	if _, err := m.loadAuthorizedTask(ctx, m.store, trimmedDependsOnID, actor); err != nil {
		return err
	}

	if err := m.store.DeleteDependency(ctx, trimmedTaskID, trimmedDependsOnID); err != nil {
		return err
	}

	record, err := m.reconcileTaskCascade(ctx, trimmedTaskID, actor)
	if err != nil {
		return err
	}
	return m.recordTaskEvent(
		ctx,
		trimmedTaskID,
		"",
		taskEventDependencyRemoved,
		actor,
		dependencyTaskPayload{
			DependsOnTaskID: trimmedDependsOnID,
			Kind:            DependencyKindBlocks,
			Status:          record.Status,
		},
	)
}
