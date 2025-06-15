package worker

import (
	"github.com/kaptinlin/jsonschema"

	"github.com/compozy/compozy/engine/core"
)

// validatePayloadAgainstCompiledSchema validates a payload against a pre-compiled JSON schema
func validatePayloadAgainstCompiledSchema(
	payload core.Input,
	compiledSchema *jsonschema.Schema,
) (bool, []string) {
	if compiledSchema == nil {
		// No schema means no validation required
		return true, nil
	}

	// Validate payload directly without unnecessary conversion
	result := compiledSchema.Validate(payload)
	if result.Valid {
		return true, nil
	}

	// Extract validation errors from the result with pre-allocation
	validationErrors := make([]string, 0, len(result.Errors))
	for _, err := range result.Errors {
		validationErrors = append(validationErrors, err.Error())
	}

	return false, validationErrors
}
