package extensionpkg

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
)

const unknownHostAPIPermissionMessage = "unknown Host API permission"

var ErrExtensionKindRemoved = errors.New("extension_kind_removed")

func validateProvideCapabilities(values []string) error {
	valid := make([]string, 0)
	for _, contract := range extensioncontract.ProvideCapabilityContracts() {
		valid = append(valid, contract.Provide)
	}
	slices.Sort(valid)
	for idx, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "connectivity.provider" {
			return fmt.Errorf("%w: %w", ErrExtensionKindRemoved, &ManifestValidationError{
				Field: fmt.Sprintf("capabilities.provides[%d]", idx), Value: value,
				Message: "Connectivity providers were removed. See https://compozy.com/docs/computers.",
			})
		}
		if slices.Contains(valid, trimmed) {
			continue
		}
		return &ManifestValidationError{
			Field:   fmt.Sprintf("capabilities.provides[%d]", idx),
			Value:   value,
			Message: fmt.Sprintf("unknown provide capability; valid values: %s", strings.Join(valid, ", ")),
		}
	}
	return nil
}

func validatePermissionMethods(values []string) error {
	valid := extensioncontract.PermissionMethods()
	for idx, value := range values {
		trimmed := strings.TrimSpace(value)
		if slices.Contains(valid, trimmed) {
			continue
		}
		return &ManifestValidationError{
			Field:   fmt.Sprintf("permissions.requires[%d]", idx),
			Value:   value,
			Message: unknownHostAPIPermissionMessage,
		}
	}
	return nil
}
