package task

import (
	"fmt"
	"slices"

	"strings"
)

func validateProviderModelGate(
	provider string,
	model string,
	options ExecutionProfileValidationOptions,
	path string,
) error {
	if (provider != "" || model != "") && !options.AllowProviderOverride {
		return fmt.Errorf("%w: %s provider/model override is disabled by config", ErrValidation, path)
	}
	if err := validateSelectorAtom(provider, nestedPath(path, "provider"), true); err != nil {
		return err
	}
	return validateSelectorAtom(model, nestedPath(path, "model"), true)
}

func validateAgentSelectors(exact string, allowed []string, preferred []string, role string) error {
	base := "task_execution_profile." + role
	if err := validateSelectorAtom(exact, nestedPath(base, "agent_name"), true); err != nil {
		return err
	}
	if err := validateProfileSelectorIDs(allowed, nestedPath(base, "allowed_agent_names")); err != nil {
		return err
	}
	if err := validateProfileSelectorIDs(preferred, nestedPath(base, "preferred_agent_names")); err != nil {
		return err
	}
	if exact != "" && !profileAgentAllowed(exact, allowed) {
		return fmt.Errorf("%w: %s.agent_name %q is outside allowed_agent_names", ErrValidation, base, exact)
	}
	return nil
}

func validateChannelSelectors(allowed []string, preferred []string, role string) error {
	base := "task_execution_profile." + role
	if err := validateProfileSelectorIDs(allowed, nestedPath(base, "allowed_channel_ids")); err != nil {
		return err
	}
	return validateProfileSelectorIDs(preferred, nestedPath(base, "preferred_channel_ids"))
}

func validatePeerSelectors(allowed []string, preferred []string, role string) error {
	base := "task_execution_profile." + role
	if err := validateProfileSelectorIDs(allowed, nestedPath(base, "allowed_peer_ids")); err != nil {
		return err
	}
	return validateProfileSelectorIDs(preferred, nestedPath(base, "preferred_peer_ids"))
}

func validateCapabilitySelectors(required []string, preferred []string, role string) error {
	base := "task_execution_profile." + role
	if err := ValidateCapabilityIDs(required, nestedPath(base, "required_capabilities")); err != nil {
		return err
	}
	return ValidateCapabilityIDs(preferred, nestedPath(base, "preferred_capabilities"))
}

func validateProfileSelectorIDs(values []string, path string) error {
	for idx, value := range values {
		if err := validateSelectorAtom(value, fmt.Sprintf("%s[%d]", path, idx), false); err != nil {
			return err
		}
	}
	return nil
}

func validateSelectorAtom(value string, path string, allowEmpty bool) error {
	if value == "" {
		if allowEmpty {
			return nil
		}
		return fmt.Errorf("%w: %s is required", ErrValidation, path)
	}
	if len(value) > profileSelectorMaxBytes {
		return fmt.Errorf("%w: %s exceeds %d bytes", ErrValidation, path, profileSelectorMaxBytes)
	}
	if strings.ContainsAny(value, "\t\n\r") {
		return fmt.Errorf("%w: %s must not contain control whitespace", ErrValidation, path)
	}
	return nil
}

func profileAgentAllowed(agentName string, allowed []string) bool {
	return len(allowed) == 0 || slices.Contains(allowed, agentName)
}

func coordinatorGuidanceMaxBytes(options ExecutionProfileValidationOptions) int {
	if options.MaxCoordinatorGuidanceBytes > 0 {
		return options.MaxCoordinatorGuidanceBytes
	}
	return defaultCoordinatorGuidanceMaxBytes
}
