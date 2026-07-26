package task

import (
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/network/participation"
)

// Validate reports whether the create-task request is internally consistent.
func (r CreateTask) Validate(path string) error {
	if err := ValidateScopeBinding(r.Scope, r.WorkspaceID, path, "workspace_id"); err != nil {
		return err
	}
	if strings.TrimSpace(r.Title) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "title"))
	}
	if strings.TrimSpace(r.ParentTaskID) != "" && strings.TrimSpace(r.ID) != "" &&
		strings.TrimSpace(r.ParentTaskID) == strings.TrimSpace(r.ID) {
		return fmt.Errorf(
			"%w: %s cannot equal %s",
			ErrValidation,
			nestedPath(path, "parent_task_id"),
			nestedPath(path, "id"),
		)
	}
	if r.Owner != nil {
		if err := r.Owner.Validate(nestedPath(path, "owner")); err != nil {
			return err
		}
	}
	if r.Priority.Normalize() != "" {
		if err := r.Priority.Validate(nestedPath(path, "priority")); err != nil {
			return err
		}
	}
	if r.MaxAttempts != nil {
		if err := validateTaskMaxAttempts(*r.MaxAttempts, nestedPath(path, "max_attempts"), false); err != nil {
			return err
		}
	}
	if r.ApprovalPolicy.Normalize() != "" {
		if err := r.ApprovalPolicy.Validate(nestedPath(path, "approval_policy")); err != nil {
			return err
		}
	}
	if err := ValidateMetadataSize(r.Metadata, nestedPath(path, "metadata")); err != nil {
		return err
	}
	if r.NetworkParticipation != nil {
		if _, err := participation.NormalizeIntent(*r.NetworkParticipation); err != nil {
			return fmt.Errorf("%w: %s: %w", ErrValidation, nestedPath(path, "network_participation"), err)
		}
	}
	return nil
}

// Validate reports whether the task patch contains at least one mutable field and valid values.
func (p Patch) Validate(path string) error {
	if !taskPatchHasMutableFields(p) {
		return fmt.Errorf("%w: %s requires at least one mutable field", ErrValidation, path)
	}
	if err := validateTaskPatchTextAndSemantics(p, path); err != nil {
		return err
	}
	if p.Owner != nil && p.ClearOwner {
		return fmt.Errorf(
			"%w: %s.owner and %s.clear_owner cannot both be set",
			ErrValidation,
			path,
			path,
		)
	}
	if p.Owner != nil {
		if err := p.Owner.Validate(nestedPath(path, "owner")); err != nil {
			return err
		}
	}
	if p.Metadata != nil {
		if err := ValidateMetadataSize(*p.Metadata, nestedPath(path, "metadata")); err != nil {
			return err
		}
	}
	if p.NetworkParticipation != nil {
		if _, err := participation.NormalizeIntent(*p.NetworkParticipation); err != nil {
			return fmt.Errorf("%w: %s: %w", ErrValidation, nestedPath(path, "network_participation"), err)
		}
	}
	return nil
}

// Validate reports whether the task-cancellation request is internally consistent.
func (r CancelTask) Validate(path string) error {
	return ValidatePayloadSize(r.Metadata, nestedPath(path, "metadata"))
}

// Validate reports whether the dependency-create request is internally consistent.
func (r AddDependency) Validate(path string) error {
	dependency := Dependency{
		TaskID:          r.TaskID,
		DependsOnTaskID: r.DependsOnTaskID,
		Kind:            r.Kind,
	}
	if err := dependency.Validate(); err != nil {
		return fmt.Errorf("%w: %s", err, path)
	}
	return nil
}

// Validate reports whether the enqueue-run request is internally consistent.
func (r EnqueueRun) Validate(path string) error {
	if strings.TrimSpace(r.TaskID) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "task_id"))
	}
	runKind := normalizeRunKindOrDefault(r.RunKind)
	if err := runKind.Validate(nestedPath(path, "run_kind")); err != nil {
		return err
	}
	if runKind == RunKindCoordinator && strings.TrimSpace(r.LoopRunID) == "" {
		return fmt.Errorf(
			"%w: %s is required for coordinator runs",
			ErrValidation,
			nestedPath(path, "loop_run_id"),
		)
	}
	if err := ValidateMetadataSize(r.Metadata, nestedPath(path, "metadata")); err != nil {
		return err
	}
	return nil
}

// Validate reports whether the task execution request is internally consistent.
func (r ExecutionRequest) Validate(path string) error {
	if err := ValidateMetadataSize(r.Metadata, nestedPath(path, "metadata")); err != nil {
		return err
	}
	return nil
}

// Validate reports whether the start-run request is internally consistent.
func (r StartRun) Validate(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("%w: start_run path is required", ErrValidation)
	}
	return nil
}

// Validate reports whether the cancel-run request is internally consistent.
func (r CancelRun) Validate(path string) error {
	return ValidatePayloadSize(r.Metadata, nestedPath(path, "metadata"))
}

// Validate reports whether the run failure contains a message and bounded metadata.
func (r RunFailure) Validate(path string) error {
	if strings.TrimSpace(r.Error) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "error"))
	}
	if err := ValidatePayloadSize(r.Metadata, nestedPath(path, "metadata")); err != nil {
		return err
	}
	if hasRawClaimTokenField(r.Metadata) {
		return fmt.Errorf(
			"%w: %s must not contain raw lease credentials",
			ErrValidation,
			nestedPath(path, "metadata"),
		)
	}
	return nil
}
