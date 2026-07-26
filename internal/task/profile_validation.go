package task

import (
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/network/participation"
)

// Normalize returns a canonical copy with trimmed fields, default modes, and stable selector sets.
func (p *ExecutionProfile) Normalize(options ExecutionProfileValidationOptions) (ExecutionProfile, error) {
	if p == nil {
		return ExecutionProfile{}, fmt.Errorf("%w: task_execution_profile is required", ErrValidation)
	}
	normalized := *p
	normalized.TaskID = strings.TrimSpace(normalized.TaskID)
	normalized.Coordinator = normalizeCoordinatorProfile(normalized.Coordinator)
	normalized.Worker = normalizeWorkerProfile(normalized.Worker)
	normalized.Review = normalizeReviewProfile(normalized.Review)
	normalized.Participants = normalizeParticipantPolicy(normalized.Participants)
	normalized.Sandbox = normalizeSandboxPolicy(normalized.Sandbox)
	normalized.Runtime = normalizeRuntimePolicy(normalized.Runtime)
	if normalized.NetworkParticipation != nil {
		request, err := participation.NormalizeIntent(*normalized.NetworkParticipation)
		if err != nil {
			return ExecutionProfile{}, fmt.Errorf("task_execution_profile.network_participation: %w", err)
		}
		if request == (participation.Request{}) {
			normalized.NetworkParticipation = nil
		} else {
			normalized.NetworkParticipation = &request
		}
	}

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
	if err := validateSandboxPolicy(p.Sandbox, options); err != nil {
		return err
	}
	return validateRuntimePolicy(p.Runtime)
}
