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
	payload := dependencyTaskPayload{
		DependsOnTaskID: normalizedSpec.DependsOnTaskID,
		Kind:            normalizedSpec.Kind,
		Status:          TaskStatusNeedsAttention,
	}
	if err := m.preflightTaskEvent(
		normalizedSpec.TaskID,
		"",
		taskEventDependencyAdded,
		actor,
		payload,
	); err != nil {
		return err
	}
	eventID, err := m.reserveTaskEventID()
	if err != nil {
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
	payload.Status = record.Status
	return m.recordTaskEventWithID(
		ctx,
		eventID,
		normalizedSpec.TaskID,
		"",
		taskEventDependencyAdded,
		actor,
		payload,
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
	if err := ValidateReferenceSize(trimmedTaskID, "remove_dependency.task_id"); err != nil {
		return err
	}
	trimmedDependsOnID := strings.TrimSpace(dependsOnID)
	if trimmedDependsOnID == "" {
		return fmt.Errorf("%w: depends_on_task_id is required", ErrValidation)
	}
	if err := ValidateReferenceSize(trimmedDependsOnID, "remove_dependency.depends_on_task_id"); err != nil {
		return err
	}
	if _, err := m.loadAuthorizedTask(ctx, m.store, trimmedTaskID, actor); err != nil {
		return err
	}
	if _, err := m.loadAuthorizedTask(ctx, m.store, trimmedDependsOnID, actor); err != nil {
		return err
	}
	payload := dependencyTaskPayload{
		DependsOnTaskID: trimmedDependsOnID,
		Kind:            DependencyKindBlocks,
		Status:          TaskStatusNeedsAttention,
	}
	if err := m.preflightTaskEvent(
		trimmedTaskID,
		"",
		taskEventDependencyRemoved,
		actor,
		payload,
	); err != nil {
		return err
	}
	eventID, err := m.reserveTaskEventID()
	if err != nil {
		return err
	}

	if err := m.store.DeleteDependency(ctx, trimmedTaskID, trimmedDependsOnID); err != nil {
		return err
	}

	record, err := m.reconcileTaskCascade(ctx, trimmedTaskID, actor)
	if err != nil {
		return err
	}
	payload.Status = record.Status
	return m.recordTaskEventWithID(
		ctx,
		eventID,
		trimmedTaskID,
		"",
		taskEventDependencyRemoved,
		actor,
		payload,
	)
}
