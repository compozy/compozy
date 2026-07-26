package task

import (
	"fmt"

	"strings"

	networkrules "github.com/compozy/agh/internal/network/rules"
)

// Validate reports whether the task-query filters are internally consistent.
func (q Query) Validate(path string) error {
	if q.Scope.Normalize() != "" {
		if err := ValidateScopeBinding(q.Scope, q.WorkspaceID, path, "workspace_id"); err != nil {
			return err
		}
	}
	if q.Status.Normalize() != "" {
		if err := q.Status.Validate(nestedPath(path, "status")); err != nil {
			return err
		}
	}
	if q.Priority.Normalize() != "" {
		if err := q.Priority.Validate(nestedPath(path, "priority")); err != nil {
			return err
		}
	}
	if q.ApprovalState.Normalize() != "" {
		if err := q.ApprovalState.Validate(nestedPath(path, "approval_state")); err != nil {
			return err
		}
	}
	if q.OwnerKind.Normalize() != "" {
		if err := q.OwnerKind.Validate(nestedPath(path, "owner_kind")); err != nil {
			return err
		}
	}
	if q.CreatedByKind.Normalize() != "" {
		if err := q.CreatedByKind.Validate(nestedPath(path, "created_by_kind")); err != nil {
			return err
		}
	}
	if q.Limit < 0 {
		return fmt.Errorf(
			"%w: %s must be zero or positive: %d",
			ErrValidation,
			nestedPath(path, "limit"),
			q.Limit,
		)
	}
	return nil
}

// Validate reports whether the scheduler backlog filters are internally consistent.
func (q SchedulerBacklogQuery) Validate(path string) error {
	if q.Scope.Normalize() == "" {
		return nil
	}
	return ValidateScopeBinding(q.Scope, q.WorkspaceID, path, "workspace_id")
}

// Validate reports whether the task-run query filters are internally consistent.
func (q RunQuery) Validate(path string) error {
	if q.Status.Normalize() != TaskRunStatusUnknown {
		if err := q.Status.Validate(nestedPath(path, "status")); err != nil {
			return err
		}
	}
	participationChannel := strings.TrimSpace(q.ParticipationChannel)
	if participationChannel != "" && !networkrules.ValidChannel(participationChannel) {
		return fmt.Errorf(
			"%w: %s is not a valid network channel: %q",
			ErrValidation,
			nestedPath(path, "participation_channel"),
			q.ParticipationChannel,
		)
	}
	if q.Limit < 0 {
		return fmt.Errorf(
			"%w: %s must be zero or positive: %d",
			ErrValidation,
			nestedPath(path, "limit"),
			q.Limit,
		)
	}
	return nil
}

// Validate reports whether the task-event query filters are internally consistent.
func (q EventQuery) Validate(path string) error {
	if q.Limit < 0 {
		return fmt.Errorf(
			"%w: %s must be zero or positive: %d",
			ErrValidation,
			nestedPath(path, "limit"),
			q.Limit,
		)
	}
	return nil
}

// Validate reports whether the task timeline query filters are internally consistent.
func (q TimelineQuery) Validate(path string) error {
	if q.AfterSequence < 0 {
		return fmt.Errorf(
			"%w: %s must be zero or positive: %d",
			ErrValidation,
			nestedPath(path, "after_sequence"),
			q.AfterSequence,
		)
	}
	if q.Limit < 0 {
		return fmt.Errorf(
			"%w: %s must be zero or positive: %d",
			ErrValidation,
			nestedPath(path, "limit"),
			q.Limit,
		)
	}
	return nil
}

// Validate reports whether the task stream query filters are internally consistent.
func (q StreamQuery) Validate(path string) error {
	if q.AfterSequence < 0 {
		return fmt.Errorf(
			"%w: %s must be zero or positive: %d",
			ErrValidation,
			nestedPath(path, "after_sequence"),
			q.AfterSequence,
		)
	}
	return nil
}

// Validate reports whether the sequenced event record query is internally consistent.
func (q EventRecordQuery) Validate(path string) error {
	if strings.TrimSpace(q.TaskID) == "" && !q.AllTasks {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "task_id"))
	}
	if strings.TrimSpace(q.TaskID) != "" && q.AllTasks {
		return fmt.Errorf(
			"%w: %s and %s are mutually exclusive",
			ErrValidation,
			nestedPath(path, "task_id"),
			nestedPath(path, "all_tasks"),
		)
	}
	if q.AfterSequence < 0 {
		return fmt.Errorf(
			"%w: %s must be zero or positive: %d",
			ErrValidation,
			nestedPath(path, "after_sequence"),
			q.AfterSequence,
		)
	}
	if q.Limit < 0 {
		return fmt.Errorf(
			"%w: %s must be zero or positive: %d",
			ErrValidation,
			nestedPath(path, "limit"),
			q.Limit,
		)
	}
	return nil
}

// Validate reports whether the session-start request contains the task and run context required by the bridge.
func (r *StartTaskSession) Validate() error {
	if r == nil {
		return fmt.Errorf("%w: start_task_session is required", ErrValidation)
	}
	if err := r.Task.Validate(); err != nil {
		return err
	}
	if err := r.Run.Validate(); err != nil {
		return err
	}
	if err := r.Actor.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(r.Run.TaskID) != strings.TrimSpace(r.Task.ID) {
		return fmt.Errorf(
			"%w: start_task_session.run.task_id must match start_task_session.task.id",
			ErrValidation,
		)
	}
	if r.ExecutionProfile != nil {
		if strings.TrimSpace(r.ExecutionProfile.TaskID) != strings.TrimSpace(r.Task.ID) {
			return fmt.Errorf(
				"%w: start_task_session.execution_profile.task_id must match start_task_session.task.id",
				ErrValidation,
			)
		}
		if err := r.ExecutionProfile.Validate(DefaultExecutionProfileValidationOptions()); err != nil {
			return err
		}
	}
	return nil
}
