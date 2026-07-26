package globaldb

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
)

var _ taskpkg.DefinitionStore = (*TaskRepo)(nil)

// CreateTaskDefinition commits the task row, optional profile, and causal
// events in one immediate transaction.
func (g *TaskRepo) CreateTaskDefinition(
	ctx context.Context,
	mutation taskpkg.CreateTaskDefinitionMutation,
) error {
	if err := g.checkReady(ctx, "create task definition"); err != nil {
		return err
	}
	normalized, err := g.normalizeTaskForCreate(mutation.Task)
	if err != nil {
		return err
	}
	if len(mutation.Events) == 0 {
		return fmt.Errorf("%w: create task definition requires an event", taskpkg.ErrValidation)
	}

	return g.withTaskImmediateTransaction(ctx, "create task definition", func(exec taskSQLExecutor) error {
		if err := g.ensureTaskCreateReferencesWithExecutor(ctx, exec, normalized); err != nil {
			return err
		}
		if err := insertTaskWithExecutor(ctx, exec, normalized); err != nil {
			return fmt.Errorf("store: create task %q: %w", normalized.ID, err)
		}
		if mutation.Profile != nil {
			if _, err := g.upsertExecutionProfileWithExecutor(ctx, exec, mutation.Profile); err != nil {
				return err
			}
		}
		return appendTaskEventsWithExecutor(ctx, exec, mutation.Events)
	})
}

// UpdateTaskDefinition commits task-row/profile changes and causal events in
// one immediate transaction.
func (g *TaskRepo) UpdateTaskDefinition(
	ctx context.Context,
	mutation *taskpkg.UpdateTaskDefinitionMutation,
) (stored taskpkg.ExecutionProfile, err error) {
	if mutation == nil {
		return taskpkg.ExecutionProfile{}, fmt.Errorf(
			"%w: update task definition mutation is required",
			taskpkg.ErrValidation,
		)
	}
	if err := g.checkReady(ctx, "update task definition"); err != nil {
		return taskpkg.ExecutionProfile{}, err
	}
	if !mutation.UpdateTaskRow && !mutation.PatchNetworkParticipation {
		return taskpkg.ExecutionProfile{}, fmt.Errorf(
			"%w: update task definition requires a row or profile mutation",
			taskpkg.ErrValidation,
		)
	}
	if len(mutation.Events) == 0 {
		return taskpkg.ExecutionProfile{}, fmt.Errorf(
			"%w: update task definition requires an event",
			taskpkg.ErrValidation,
		)
	}

	err = g.withTaskImmediateTransaction(ctx, "update task definition", func(exec taskSQLExecutor) error {
		if mutation.UpdateTaskRow {
			if err := g.updateTaskWithExecutor(ctx, exec, mutation.Task, mutation.Actor); err != nil {
				return err
			}
		} else if err := g.ensureTaskExistsWithExecutor(ctx, exec, mutation.Task.ID); err != nil {
			return err
		}
		if mutation.PatchNetworkParticipation {
			if err := ensureTaskProfileMutationAllowedWithExecutor(ctx, g, exec, mutation.Task.ID); err != nil {
				return err
			}
			profile, found, profileErr := loadExecutionProfile(ctx, exec, mutation.Task.ID)
			if profileErr != nil {
				return profileErr
			}
			if !found {
				profile = taskpkg.DefaultExecutionProfile(mutation.Task.ID)
			}
			profile.NetworkParticipation = participation.CloneRequest(mutation.NetworkParticipation)
			stored, profileErr = g.upsertExecutionProfileWithExecutor(ctx, exec, &profile)
			if profileErr != nil {
				return profileErr
			}
		}
		return appendTaskEventsWithExecutor(ctx, exec, mutation.Events)
	})
	if err != nil {
		return taskpkg.ExecutionProfile{}, err
	}
	return stored, nil
}

// SetTaskExecutionProfile commits a profile replacement and its event.
func (g *TaskRepo) SetTaskExecutionProfile(
	ctx context.Context,
	mutation *taskpkg.ExecutionProfileMutation,
) (stored taskpkg.ExecutionProfile, err error) {
	if mutation == nil {
		return taskpkg.ExecutionProfile{}, fmt.Errorf(
			"%w: execution profile mutation is required",
			taskpkg.ErrValidation,
		)
	}
	if err := g.checkReady(ctx, "set task execution profile"); err != nil {
		return taskpkg.ExecutionProfile{}, err
	}
	err = g.withTaskImmediateTransaction(ctx, "set task execution profile", func(exec taskSQLExecutor) error {
		if err := ensureTaskProfileMutationAllowedWithExecutor(
			ctx,
			g,
			exec,
			mutation.Profile.TaskID,
		); err != nil {
			return err
		}
		var storeErr error
		stored, storeErr = g.upsertExecutionProfileWithExecutor(ctx, exec, &mutation.Profile)
		if storeErr != nil {
			return storeErr
		}
		return appendTaskEventsWithExecutor(ctx, exec, []taskpkg.Event{mutation.Event})
	})
	if err != nil {
		return taskpkg.ExecutionProfile{}, err
	}
	return stored, nil
}

// DeleteTaskExecutionProfile commits a profile deletion and its event.
func (g *TaskRepo) DeleteTaskExecutionProfile(
	ctx context.Context,
	mutation taskpkg.ExecutionProfileDeleteMutation,
) error {
	if err := g.checkReady(ctx, "delete task execution profile command"); err != nil {
		return err
	}
	taskID := strings.TrimSpace(mutation.TaskID)
	if taskID == "" {
		return fmt.Errorf("%w: task id is required", taskpkg.ErrValidation)
	}
	return g.withTaskImmediateTransaction(
		ctx,
		"delete task execution profile command",
		func(exec taskSQLExecutor) error {
			if err := ensureTaskProfileMutationAllowedWithExecutor(ctx, g, exec, taskID); err != nil {
				return err
			}
			if err := deleteExecutionProfileWithExecutor(ctx, exec, taskID); err != nil {
				return err
			}
			return appendTaskEventsWithExecutor(ctx, exec, []taskpkg.Event{mutation.Event})
		},
	)
}

func ensureTaskProfileMutationAllowedWithExecutor(
	ctx context.Context,
	repo *TaskRepo,
	exec taskSQLExecutor,
	taskID string,
) error {
	record, err := repo.getTaskWithExecutor(ctx, exec, strings.TrimSpace(taskID))
	if err != nil {
		return err
	}
	if strings.TrimSpace(record.CurrentRunID) == "" {
		return nil
	}
	return fmt.Errorf(
		"%w: task %q execution profile cannot change while run %q is active",
		taskpkg.ErrInvalidStatusTransition,
		record.ID,
		record.CurrentRunID,
	)
}

func (g *TaskRepo) ensureTaskCreateReferencesWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	record taskpkg.Task,
) error {
	if err := taskpkg.ValidateScopeBinding(record.Scope, record.WorkspaceID, "task", "workspace_id"); err != nil {
		return err
	}
	if record.Scope == taskpkg.ScopeWorkspace {
		if err := g.ensureWorkspaceExistsWithExecutor(ctx, exec, record.WorkspaceID); err != nil {
			return err
		}
	}
	if strings.TrimSpace(record.ParentTaskID) != "" {
		if err := g.ensureTaskExistsWithExecutor(ctx, exec, record.ParentTaskID); err != nil {
			return err
		}
	}
	return nil
}

func appendTaskEventsWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	events []taskpkg.Event,
) error {
	for _, event := range events {
		if err := appendTaskEventWithExecutor(ctx, exec, EventRecordInsert{
			ID:        event.ID,
			TaskID:    event.TaskID,
			RunID:     event.RunID,
			EventType: event.EventType,
			Actor:     event.Actor,
			Origin:    event.Origin,
			Payload:   event.Payload,
			Timestamp: event.Timestamp,
		}); err != nil {
			return err
		}
	}
	return nil
}
