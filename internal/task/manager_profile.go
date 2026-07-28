package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type profileMutationEventPayload struct {
	TaskID          string          `json:"task_id"`
	CoordinatorMode CoordinatorMode `json:"coordinator_mode,omitempty"`
	WorkerMode      WorkerMode      `json:"worker_mode,omitempty"`
	SandboxMode     SandboxMode     `json:"sandbox_mode,omitempty"`
}

// GetExecutionProfile returns the persisted profile or the default inherit profile.
func (m *Service) GetExecutionProfile(
	ctx context.Context,
	taskID string,
	actor ActorContext,
) (ExecutionProfile, error) {
	if err := requireReadAuthority(actor); err != nil {
		return ExecutionProfile{}, err
	}
	trimmedID := strings.TrimSpace(taskID)
	if trimmedID == "" {
		return ExecutionProfile{}, fmt.Errorf("%w: task id is required", ErrValidation)
	}
	if _, err := m.loadAuthorizedTask(ctx, m.store, trimmedID, actor); err != nil {
		return ExecutionProfile{}, err
	}

	profile, err := m.store.GetExecutionProfile(ctx, trimmedID)
	switch {
	case errors.Is(err, ErrExecutionProfileNotFound):
		return defaultExecutionProfile(trimmedID), nil
	case err != nil:
		return ExecutionProfile{}, err
	default:
		return profile, nil
	}
}

// SetExecutionProfile validates and persists one task-owned execution profile.
func (m *Service) SetExecutionProfile(
	ctx context.Context,
	taskID string,
	profile *ExecutionProfile,
	actor ActorContext,
) (ExecutionProfile, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return ExecutionProfile{}, err
	}
	trimmedID := strings.TrimSpace(taskID)
	if trimmedID == "" {
		return ExecutionProfile{}, fmt.Errorf("%w: task id is required", ErrValidation)
	}
	if profile == nil {
		return ExecutionProfile{}, fmt.Errorf("%w: task_execution_profile is required", ErrValidation)
	}
	if strings.TrimSpace(profile.TaskID) != "" && strings.TrimSpace(profile.TaskID) != trimmedID {
		return ExecutionProfile{}, fmt.Errorf(
			"%w: task_execution_profile.task_id must match task id %q",
			ErrValidation,
			trimmedID,
		)
	}

	record, err := m.loadAuthorizedTask(ctx, m.store, trimmedID, actor)
	if err != nil {
		return ExecutionProfile{}, err
	}
	if strings.TrimSpace(record.CurrentRunID) != "" {
		return ExecutionProfile{}, fmt.Errorf(
			"%w: task %q execution profile cannot change while run %q is active",
			ErrInvalidStatusTransition,
			record.ID,
			record.CurrentRunID,
		)
	}

	profile.TaskID = trimmedID
	normalized, err := profile.Normalize(m.profileValidation)
	if err != nil {
		return ExecutionProfile{}, err
	}
	normalized.UpdatedAt = m.now().UTC()
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = normalized.UpdatedAt
	}

	event, err := m.newTaskEvent(trimmedID, "", taskEventProfileUpdated, actor, profileMutationEventPayload{
		TaskID:          trimmedID,
		CoordinatorMode: normalized.Coordinator.Mode,
		WorkerMode:      normalized.Worker.Mode,
		SandboxMode:     normalized.Sandbox.Mode,
	})
	if err != nil {
		return ExecutionProfile{}, err
	}
	stored, err := m.store.SetTaskExecutionProfile(ctx, &ExecutionProfileMutation{
		Profile: normalized,
		Event:   event,
	})
	if err != nil {
		return ExecutionProfile{}, err
	}
	m.publishTaskEventsAfterCommand(ctx, []Event{event})
	return stored, nil
}

// DeleteExecutionProfile removes the persisted profile after active-run drift checks.
func (m *Service) DeleteExecutionProfile(
	ctx context.Context,
	taskID string,
	actor ActorContext,
) error {
	if err := requireWriteAuthority(actor); err != nil {
		return err
	}
	trimmedID := strings.TrimSpace(taskID)
	if trimmedID == "" {
		return fmt.Errorf("%w: task id is required", ErrValidation)
	}
	record, err := m.loadAuthorizedTask(ctx, m.store, trimmedID, actor)
	if err != nil {
		return err
	}
	if strings.TrimSpace(record.CurrentRunID) != "" {
		return fmt.Errorf(
			"%w: task %q execution profile cannot change while run %q is active",
			ErrInvalidStatusTransition,
			record.ID,
			record.CurrentRunID,
		)
	}
	event, err := m.newTaskEvent(trimmedID, "", taskEventProfileDeleted, actor, profileMutationEventPayload{
		TaskID: trimmedID,
	})
	if err != nil {
		return err
	}
	if err := m.store.DeleteTaskExecutionProfile(ctx, ExecutionProfileDeleteMutation{
		TaskID: trimmedID,
		Event:  event,
	}); err != nil {
		return err
	}
	m.publishTaskEventsAfterCommand(ctx, []Event{event})
	return nil
}

func defaultExecutionProfile(taskID string) ExecutionProfile {
	return ExecutionProfile{
		TaskID:      taskID,
		Coordinator: CoordinatorProfile{Mode: CoordinatorModeInherit},
		Worker:      WorkerProfile{Mode: WorkerModeInherit},
		Sandbox:     SandboxPolicy{Mode: SandboxModeInherit},
		Runtime:     RuntimePolicy{Mode: RuntimeModeDefault},
	}
}

// DefaultExecutionProfile returns the inherit-mode profile for one task.
func DefaultExecutionProfile(taskID string) ExecutionProfile {
	return defaultExecutionProfile(strings.TrimSpace(taskID))
}
