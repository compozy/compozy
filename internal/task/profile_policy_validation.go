package task

import "fmt"

func validateCoordinatorProfile(profile CoordinatorProfile, options ExecutionProfileValidationOptions) error {
	switch profile.Mode.Normalize() {
	case CoordinatorModeInherit, CoordinatorModeGuided:
	default:
		return fmt.Errorf(
			"%w: task_execution_profile.coordinator.mode must be %q or %q: %q",
			ErrValidation,
			CoordinatorModeInherit,
			CoordinatorModeGuided,
			profile.Mode,
		)
	}
	if len(profile.Guidance) > coordinatorGuidanceMaxBytes(options) {
		return fmt.Errorf(
			"%w: task_execution_profile.coordinator.guidance exceeds %d bytes",
			ErrValidation,
			coordinatorGuidanceMaxBytes(options),
		)
	}
	return validateProviderModelGate(
		profile.Provider,
		profile.Model,
		options,
		"task_execution_profile.coordinator",
	)
}

func validateWorkerProfile(profile WorkerProfile, options ExecutionProfileValidationOptions) error {
	switch profile.Mode.Normalize() {
	case WorkerModeInherit, WorkerModeSelect:
	default:
		return fmt.Errorf(
			"%w: task_execution_profile.worker.mode must be %q or %q: %q",
			ErrValidation,
			WorkerModeInherit,
			WorkerModeSelect,
			profile.Mode,
		)
	}
	if err := validateProviderModelGate(
		profile.Provider,
		profile.Model,
		options,
		"task_execution_profile.worker",
	); err != nil {
		return err
	}
	if err := validateProfileRuntime(
		profile.ReasoningEffort,
		profile.Speed,
		profile.ACPOptions,
		"task_execution_profile.worker",
	); err != nil {
		return err
	}
	if err := validateProfileSelectorIDs(
		profile.AllowedAgentNames,
		"task_execution_profile.worker.allowed_agent_names",
	); err != nil {
		return err
	}
	if err := validateProfileSelectorIDs(
		profile.PreferredAgentNames,
		"task_execution_profile.worker.preferred_agent_names",
	); err != nil {
		return err
	}
	if profile.AgentName != "" && !profileAgentAllowed(profile.AgentName, profile.AllowedAgentNames) {
		return fmt.Errorf(
			"%w: task_execution_profile.worker.agent_name %q is outside allowed_agent_names",
			ErrValidation,
			profile.AgentName,
		)
	}
	if err := ValidateCapabilityIDs(
		profile.RequiredCapabilities,
		"task_execution_profile.worker.required_capabilities",
	); err != nil {
		return err
	}
	return ValidateCapabilityIDs(
		profile.PreferredCapabilities,
		"task_execution_profile.worker.preferred_capabilities",
	)
}

func validateReviewProfile(profile ReviewProfile, options ExecutionProfileValidationOptions) error {
	if err := validateProviderModelGate(
		profile.Provider,
		profile.Model,
		options,
		"task_execution_profile.review",
	); err != nil {
		return err
	}
	if err := validateProfileRuntime(
		profile.ReasoningEffort,
		profile.Speed,
		profile.ACPOptions,
		"task_execution_profile.review",
	); err != nil {
		return err
	}
	if err := validateAgentSelectors(
		profile.AgentName,
		profile.AllowedAgentNames,
		profile.PreferredAgentNames,
		"review",
	); err != nil {
		return err
	}
	return validateCapabilitySelectors(profile.RequiredCapabilities, profile.PreferredCapabilities, "review")
}

func validateParticipantPolicy(policy ParticipantPolicy) error {
	if err := validateAgentSelectors(
		"",
		policy.AllowedAgentNames,
		policy.PreferredAgentNames,
		"participants",
	); err != nil {
		return err
	}
	return validateCapabilitySelectors(
		policy.RequiredCapabilities,
		policy.PreferredCapabilities,
		"participants",
	)
}

func validateRuntimePolicy(policy RuntimePolicy) error {
	switch policy.Mode.Normalize() {
	case RuntimeModeDefault, RuntimeModeEvidence:
		return nil
	default:
		return fmt.Errorf(
			"%w: task_execution_profile.runtime.mode must be %q or %q: %q",
			ErrValidation,
			RuntimeModeDefault,
			RuntimeModeEvidence,
			policy.Mode,
		)
	}
}
