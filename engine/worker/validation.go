package worker

import (
	"github.com/compozy/compozy/engine/core"
	"github.com/kaptinlin/jsonschema"
)

// ValidatePayloadAgainstCompiledSchema validates a payload against a pre-compiled JSON schema
func ValidatePayloadAgainstCompiledSchema(
	payload core.Input,
	compiledSchema *jsonschema.Schema,
) (bool, []string) {
	if compiledSchema == nil {
		return true, nil
	}
	result := compiledSchema.Validate(payload)
	if result.Valid {
		return true, nil
	}
	validationErrors := make([]string, 0, len(result.Errors))
	for _, err := range result.Errors {
		validationErrors = append(validationErrors, err.Error())
	}
	return false, validationErrors
}
