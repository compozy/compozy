package task

import (
	"fmt"
	"strings"
)

// Normalize returns a canonical copy with trimmed fields, default modes, and stable selector sets.
func (p *ExecutionProfile) Normalize(options ExecutionProfileValidationOptions) (ExecutionProfile, error) {
	if p == nil {
		return ExecutionProfile{}, fmt.Errorf("%w: task_execution_profile is required", ErrValidation)
	}
	normalized := *p
	normalized.TaskID = strings.TrimSpace(normalized.TaskID)
	normalized.Coordinator = normalizeCoordinatorProfile(normalized.Coordinator)
	var err error
	normalized.Worker, err = normalizeWorkerProfile(normalized.Worker)
	if err != nil {
		return ExecutionProfile{}, err
	}
	normalized.Review, err = normalizeReviewProfile(normalized.Review)
	if err != nil {
		return ExecutionProfile{}, err
	}
	normalized.Participants = normalizeParticipantPolicy(normalized.Participants)
	normalized.Worktree = normalizeWorktreePolicy(normalized.Worktree)
	normalized.Runtime = normalizeRuntimePolicy(normalized.Runtime)

	if err := (&normalized).Validate(options); err != nil {
		return ExecutionProfile{}, err
	}
	return normalized, nil
}

// Validate reports whether the profile can be persisted as typed orchestration state.
func (p *ExecutionProfile) Validate(options ExecutionProfileValidationOptions) error {
	if p == nil {
		return fmt.Errorf("%w: task_execution_profile is required", ErrValidation)
	}
	if strings.TrimSpace(p.TaskID) == "" {
		return fmt.Errorf("%w: task_execution_profile.task_id is required", ErrValidation)
	}
	if err := validateCoordinatorProfile(p.Coordinator, options); err != nil {
		return err
	}
	if err := validateWorkerProfile(p.Worker, options); err != nil {
		return err
	}
	if err := validateReviewProfile(p.Review, options); err != nil {
		return err
	}
	if err := validateParticipantPolicy(p.Participants); err != nil {
		return err
	}
	if err := validateWorktreePolicy(p.Worktree); err != nil {
		return err
	}
	return validateRuntimePolicy(p.Runtime)
}
