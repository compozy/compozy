package task

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/network/participation"
)

// CreateTask derives one canonical task record from trusted actor context and
// persists the corresponding immutable audit event.
func (m *Service) CreateTask(
	ctx context.Context,
	spec CreateTask,
	actor ActorContext,
) (*Task, error) {
	if err := requireCreateAuthority(actor, spec.Scope); err != nil {
		return nil, err
	}

	normalizedSpec, err := normalizeCreateTaskSpec(spec)
	if err != nil {
		return nil, err
	}
	if err := m.authorizeTaskScope(
		ctx,
		actor,
		normalizedSpec.Scope,
		normalizedSpec.WorkspaceID,
	); err != nil {
		return nil, err
	}
	if err := m.validateParentConstraints(ctx, normalizedSpec); err != nil {
		return nil, err
	}

	record := m.newTaskRecord(normalizedSpec, actor)
	if err := record.Validate(); err != nil {
		return nil, err
	}
	profile, err := m.newDefinitionProfile(record.ID, normalizedSpec.NetworkParticipation)
	if err != nil {
		return nil, err
	}
	event, err := m.newTaskEvent(record.ID, "", taskEventCreated, actor, createdTaskPayload{
		Scope:        record.Scope,
		WorkspaceID:  record.WorkspaceID,
		ParentTaskID: record.ParentTaskID,
		Status:       record.Status,
		Owner:        cloneOwnership(record.Owner),
	})
	if err != nil {
		return nil, err
	}
	if err := m.store.CreateTaskDefinition(ctx, CreateTaskDefinitionMutation{
		Task: record, Profile: profile, Events: []Event{event},
	}); err != nil {
		return nil, err
	}
	m.publishTaskEventsAfterCommand(ctx, []Event{event})
	return &record, nil
}

func (m *Service) newTaskRecord(normalizedSpec CreateTask, actor ActorContext) Task {
	now := m.now().UTC()
	record := Task{
		ID:                 normalizedSpec.ID,
		Identifier:         normalizedSpec.Identifier,
		Scope:              normalizedSpec.Scope,
		WorkspaceID:        normalizedSpec.WorkspaceID,
		ParentTaskID:       normalizedSpec.ParentTaskID,
		Title:              normalizedSpec.Title,
		Description:        normalizedSpec.Description,
		Priority:           normalizedSpec.Priority,
		MaxAttempts:        createTaskMaxAttempts(normalizedSpec),
		AutoEnqueueOnReady: normalizedSpec.AutoEnqueueOnReady,
		Status:             createdTaskStatus(normalizedSpec),
		ApprovalPolicy:     normalizedSpec.ApprovalPolicy,
		ApprovalState:      defaultApprovalStateForPolicy(normalizedSpec.ApprovalPolicy),
		Owner:              cloneOwnership(normalizedSpec.Owner),
		WakeCreator:        createTaskWakeCreator(normalizedSpec),
		CreatedBy:          actor.Actor,
		Origin:             actor.Origin,
		CreatedAt:          now,
		UpdatedAt:          now,
		Metadata:           cloneRawJSON(normalizedSpec.Metadata),
	}
	if strings.TrimSpace(record.ID) == "" {
		record.ID = m.newID("task")
	}
	return record
}

// CreateChildTask creates one child task beneath the supplied parent and emits
// an additional parent-scoped audit event.
func (m *Service) CreateChildTask(
	ctx context.Context,
	parentTaskID string,
	spec CreateTask,
	actor ActorContext,
) (*Task, error) {
	trimmedParentID := strings.TrimSpace(parentTaskID)
	if trimmedParentID == "" {
		return nil, fmt.Errorf("%w: child parent task id is required", ErrValidation)
	}
	if strings.TrimSpace(spec.ParentTaskID) != "" &&
		strings.TrimSpace(spec.ParentTaskID) != trimmedParentID {
		return nil, fmt.Errorf(
			"%w: create_task.parent_task_id must match child parent task id",
			ErrValidation,
		)
	}
	if _, err := m.loadAuthorizedTask(ctx, m.store, trimmedParentID, actor); err != nil {
		return nil, err
	}

	spec.ParentTaskID = trimmedParentID
	if err := requireCreateAuthority(actor, spec.Scope); err != nil {
		return nil, err
	}
	normalizedSpec, err := normalizeCreateTaskSpec(spec)
	if err != nil {
		return nil, err
	}
	if err := m.authorizeTaskScope(
		ctx,
		actor,
		normalizedSpec.Scope,
		normalizedSpec.WorkspaceID,
	); err != nil {
		return nil, err
	}
	if err := m.validateParentConstraints(ctx, normalizedSpec); err != nil {
		return nil, err
	}
	child := m.newTaskRecord(normalizedSpec, actor)
	if err := child.Validate(); err != nil {
		return nil, err
	}
	profile, err := m.newDefinitionProfile(child.ID, normalizedSpec.NetworkParticipation)
	if err != nil {
		return nil, err
	}
	createdEvent, err := m.newTaskEvent(child.ID, "", taskEventCreated, actor, createdTaskPayload{
		Scope: child.Scope, WorkspaceID: child.WorkspaceID, ParentTaskID: child.ParentTaskID,
		Status: child.Status, Owner: cloneOwnership(child.Owner),
	})
	if err != nil {
		return nil, err
	}
	parentEvent, err := m.newTaskEvent(trimmedParentID, "", taskEventChildCreated, actor, childCreatedTaskPayload{
		ChildTaskID: child.ID, ChildScope: child.Scope, ChildWorkspaceID: child.WorkspaceID,
	})
	if err != nil {
		return nil, err
	}
	events := []Event{createdEvent, parentEvent}
	if err := m.store.CreateTaskDefinition(ctx, CreateTaskDefinitionMutation{
		Task: child, Profile: profile, Events: events,
	}); err != nil {
		return nil, err
	}
	m.publishTaskEventsAfterCommand(ctx, events)
	return &child, nil
}

// DeleteTask removes one task after verifying it is not still in use by child
// tasks or non-terminal runs, then reconciles any dependents unblocked by the
// cascade-deleted dependency edges.
func (m *Service) DeleteTask(ctx context.Context, id string, actor ActorContext) error {
	if err := requireWriteAuthority(actor); err != nil {
		return err
	}

	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return fmt.Errorf("%w: task id is required", ErrValidation)
	}

	if txStore, ok := m.store.(DeleteTaskTransactionStore); ok {
		return txStore.WithDeleteTaskTransaction(ctx, func(store DeleteTaskMutationStore) error {
			return m.deleteTaskWithStore(ctx, store, trimmedID, actor)
		})
	}

	return m.deleteTaskWithStore(ctx, m.store, trimmedID, actor)
}

func (m *Service) deleteTaskWithStore(
	ctx context.Context,
	store DeleteTaskMutationStore,
	trimmedID string,
	actor ActorContext,
) error {
	record, err := m.loadAuthorizedTask(ctx, store, trimmedID, actor)
	if err != nil {
		return fmt.Errorf("task: load task %q for delete: %w", trimmedID, err)
	}
	if err := m.ensureTaskDeleteAllowedWithStore(ctx, store, record); err != nil {
		return err
	}

	dependents, err := store.ListDependents(ctx, trimmedID)
	if err != nil {
		return fmt.Errorf("task: list dependents for task %q delete: %w", trimmedID, err)
	}
	dependentIDs := uniqueDependentTaskIDs(dependents)

	if err := store.DeleteTask(ctx, trimmedID); err != nil {
		return fmt.Errorf("task: delete task %q: %w", trimmedID, err)
	}

	for _, dependentID := range dependentIDs {
		if _, err := m.reconcileTaskCascadeWithStore(ctx, store, dependentID, actor); err != nil {
			return fmt.Errorf(
				"task: reconcile dependent task %q after deleting %q: %w",
				dependentID,
				trimmedID,
				err,
			)
		}
	}

	return nil
}

// UpdateTask applies one mutable patch while preserving immutable identity and
// structural fields under manager control.
func (m *Service) UpdateTask(
	ctx context.Context,
	id string,
	patch Patch,
	actor ActorContext,
) (*Task, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return nil, err
	}

	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return nil, fmt.Errorf("%w: task id is required", ErrValidation)
	}
	normalizedPatch, err := normalizeTaskPatch(patch)
	if err != nil {
		return nil, err
	}
	current, err := m.loadAuthorizedTask(ctx, m.store, trimmedID, actor)
	if err != nil {
		return nil, err
	}

	updated, changedFields := applyTaskPatch(current, normalizedPatch)
	updateTaskRow := len(changedFields) > 0
	if normalizedPatch.NetworkParticipation != nil {
		changedFields = append(changedFields, "network_participation")
	}
	if len(changedFields) == 0 {
		return &current, nil
	}

	if updateTaskRow {
		dependencies, err := m.store.ListDependencies(ctx, trimmedID)
		if err != nil {
			return nil, err
		}
		runs, err := m.store.ListTaskRuns(ctx, RunQuery{TaskID: trimmedID})
		if err != nil {
			return nil, err
		}
		canonicalStatus, err := m.canonicalTaskStatus(ctx, updated, dependencies, runs)
		if err != nil {
			return nil, err
		}
		updated.Status = canonicalStatus
		updated.UpdatedAt = m.now().UTC()
	}
	events, err := m.taskUpdateEvents(
		updated,
		changedFields,
		normalizedPatch.NetworkParticipation != nil,
		actor,
	)
	if err != nil {
		return nil, err
	}
	if _, err := m.store.UpdateTaskDefinition(ctx, &UpdateTaskDefinitionMutation{
		Task:                      updated,
		UpdateTaskRow:             updateTaskRow,
		PatchNetworkParticipation: normalizedPatch.NetworkParticipation != nil,
		NetworkParticipation:      participation.CloneRequest(normalizedPatch.NetworkParticipation),
		Actor:                     actor,
		Events:                    events,
	}); err != nil {
		return nil, err
	}
	m.publishTaskEventsAfterCommand(ctx, events)
	if current.Status.Normalize() != updated.Status.Normalize() {
		m.dispatchTaskStatusChangedAfterWrite(ctx, updated, current.Status, updated.Status, actor)
	}

	return &updated, nil
}

func (m *Service) taskUpdateEvents(
	updated Task,
	changedFields []string,
	includeProfileEvent bool,
	actor ActorContext,
) ([]Event, error) {
	updatedEvent, err := m.newTaskEvent(updated.ID, "", taskEventUpdated, actor, updatedTaskPayload{
		ChangedFields: append([]string(nil), changedFields...),
		Status:        updated.Status,
	})
	if err != nil {
		return nil, err
	}
	events := []Event{updatedEvent}
	if !includeProfileEvent {
		return events, nil
	}
	profileEvent, err := m.newTaskEvent(
		updated.ID,
		"",
		taskEventProfileUpdated,
		actor,
		profileMutationEventPayload{TaskID: updated.ID},
	)
	if err != nil {
		return nil, err
	}
	return append(events, profileEvent), nil
}

func (m *Service) newDefinitionProfile(
	taskID string,
	request *participation.Request,
) (*ExecutionProfile, error) {
	if request == nil {
		return nil, nil
	}
	profile := defaultExecutionProfile(taskID)
	profile.NetworkParticipation = participation.CloneRequest(request)
	normalized, err := profile.Normalize(m.profileValidation)
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}

// CancelTask propagates manager-owned cancellation through the target task,
// affected runs, and all non-terminal descendants.
func (m *Service) CancelTask(
	ctx context.Context,
	id string,
	req CancelTask,
	actor ActorContext,
) (*Task, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return nil, err
	}

	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return nil, fmt.Errorf("%w: task id is required", ErrValidation)
	}
	normalizedReq, err := normalizeCancelTask(req)
	if err != nil {
		return nil, err
	}

	tree, root, err := m.loadCancellationTree(ctx, trimmedID)
	if err != nil {
		return nil, err
	}
	if err := m.authorizeTaskTreeMutation(ctx, actor, tree); err != nil {
		return nil, err
	}
	if err := m.ensureTaskCancelable(ctx, root); err != nil {
		return nil, err
	}

	cancelledRoot := root
	for idx, record := range tree {
		record, err = m.cancelTaskTreeRecord(ctx, trimmedID, idx, record, normalizedReq, actor)
		if err != nil {
			return nil, err
		}
		if record.ID == trimmedID {
			cancelledRoot = record
		}
	}

	return &cancelledRoot, nil
}

func applyTaskPatch(current Task, patch Patch) (Task, []string) {
	updated := current
	changedFields := make([]string, 0, len(MutableTaskFields()))

	if patch.Title != nil && updated.Title != *patch.Title {
		updated.Title = *patch.Title
		changedFields = append(changedFields, TaskFieldTitle)
	}
	if patch.Description != nil && updated.Description != *patch.Description {
		updated.Description = *patch.Description
		changedFields = append(changedFields, TaskFieldDescription)
	}
	if patch.Priority != nil && updated.Priority != *patch.Priority {
		updated.Priority = *patch.Priority
		changedFields = append(changedFields, TaskFieldPriority)
	}
	if patch.MaxAttempts != nil && updated.MaxAttempts != *patch.MaxAttempts {
		updated.MaxAttempts = *patch.MaxAttempts
		changedFields = append(changedFields, TaskFieldMaxAttempts)
	}
	if patch.AutoEnqueueOnReady != nil && updated.AutoEnqueueOnReady != *patch.AutoEnqueueOnReady {
		updated.AutoEnqueueOnReady = *patch.AutoEnqueueOnReady
		changedFields = append(changedFields, TaskFieldAutoEnqueueOnReady)
	}
	if patch.ApprovalPolicy != nil && updated.ApprovalPolicy != *patch.ApprovalPolicy {
		updated.ApprovalPolicy = *patch.ApprovalPolicy
		updated.ApprovalState = defaultApprovalStateForPolicy(*patch.ApprovalPolicy)
		changedFields = append(changedFields, TaskFieldApprovalPolicy)
	}
	return applyTaskPatchReferences(updated, patch, changedFields)
}

func applyTaskPatchReferences(updated Task, patch Patch, changedFields []string) (Task, []string) {
	if patch.Metadata != nil && !sameRawJSON(updated.Metadata, *patch.Metadata) {
		updated.Metadata = cloneRawJSON(*patch.Metadata)
		changedFields = append(changedFields, TaskFieldMetadata)
	}
	if patch.Owner != nil && !sameOwnership(updated.Owner, patch.Owner) {
		updated.Owner = cloneOwnership(patch.Owner)
		changedFields = append(changedFields, TaskFieldOwner)
	}
	if patch.ClearOwner && updated.Owner != nil {
		updated.Owner = nil
		changedFields = append(changedFields, TaskFieldOwner)
	}

	return updated, changedFields
}

func createTaskMaxAttempts(spec CreateTask) int {
	if spec.MaxAttempts == nil {
		return DefaultTaskMaxAttempts
	}
	return normalizeTaskMaxAttemptsOrDefault(*spec.MaxAttempts)
}

func createdTaskStatus(spec CreateTask) Status {
	if spec.Draft {
		return TaskStatusDraft
	}
	if approvalStateBlocksExecution(
		normalizeApprovalPolicyOrDefault(spec.ApprovalPolicy),
		defaultApprovalStateForPolicy(spec.ApprovalPolicy),
	) {
		return TaskStatusBlocked
	}
	return TaskStatusReady
}
