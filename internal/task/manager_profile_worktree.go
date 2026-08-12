package task

import (
	"context"
	"fmt"
	"strings"
)

// SetWorktreePolicy patches only the worktree block of one task execution profile.
func (m *Service) SetWorktreePolicy(
	ctx context.Context,
	taskID string,
	policy WorktreePolicy,
	actor ActorContext,
) (ExecutionProfile, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return ExecutionProfile{}, err
	}
	trimmedID := strings.TrimSpace(taskID)
	if trimmedID == "" {
		return ExecutionProfile{}, fmt.Errorf("%w: task id is required", ErrValidation)
	}
	taskRecord, err := m.loadAuthorizedTask(ctx, m.store, trimmedID, actor)
	if err != nil {
		return ExecutionProfile{}, err
	}
	if strings.TrimSpace(taskRecord.CurrentRunID) != "" {
		return ExecutionProfile{}, fmt.Errorf(
			"%w: task %q execution profile cannot change while run %q is active",
			ErrInvalidStatusTransition,
			taskRecord.ID,
			taskRecord.CurrentRunID,
		)
	}
	normalized := normalizeWorktreePolicy(policy)
	if err := validateWorktreePolicy(normalized); err != nil {
		return ExecutionProfile{}, err
	}
	if err := m.validateTaskWorktreePolicy(ctx, taskRecord, normalized); err != nil {
		return ExecutionProfile{}, err
	}
	event, err := m.newTaskEvent(trimmedID, "", taskEventProfileUpdated, actor, profileMutationEventPayload{
		TaskID:       trimmedID,
		WorktreeMode: normalized.Mode,
	})
	if err != nil {
		return ExecutionProfile{}, err
	}
	stored, err := m.store.SetTaskWorktreePolicy(ctx, &WorktreePolicyMutation{
		TaskID: trimmedID,
		Policy: normalized,
		Event:  event,
	})
	if err != nil {
		return ExecutionProfile{}, err
	}
	m.publishTaskEventsAfterCommand(ctx, []Event{event})
	return stored, nil
}
