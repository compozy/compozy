package schema

import (
	"encoding/json"

	"github.com/compozy/compozy/internal/parser/common"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

// WithParamsValidator validates With parameters against InputSchema using jsonschema/v5
type WithParamsValidator struct {
	id          string
	with        *common.WithParams
	inputSchema *InputSchema
}

func NewWithParamsValidator(with *common.WithParams, inputSchema *InputSchema, id string) *WithParamsValidator {
	return &WithParamsValidator{
		with:        with,
		inputSchema: inputSchema,
		id:          id,
	}
}

func (v *WithParamsValidator) Validate() error {
	if v.with == nil || v.inputSchema == nil {
		// Skip validation if either is nil
		return nil
	}

	// Convert InputSchema to JSON for jsonschema
	schemaJSON, err := json.Marshal(v.inputSchema)
	if err != nil {
		return NewSchemaErrorf(ErrCodeInvalidWithParams, "Failed to marshal InputSchema for %s: %s", v.id, err.Error())
	}

	// Compile the schema
	schema, err := jsonschema.CompileString("schema.json", string(schemaJSON))
	if err != nil {
		return NewSchemaErrorf(ErrCodeInvalidWithParams, "Invalid InputSchema for %s: %s", v.id, err.Error())
	}

	// Convert With to a map for validation
	withData := map[string]any(*v.with)

	// Perform validation
	if err := schema.Validate(withData); err != nil {
		// Handle validation errors
		if vErr, ok := err.(*jsonschema.ValidationError); ok {
			return NewSchemaErrorf(ErrCodeInvalidWithParams, ErrMsgInvalidWithParams, v.id, vErr.Message)
		}
		return NewSchemaErrorf(ErrCodeInvalidWithParams, "Validation error for %s: %s", v.id, err.Error())
	}

	return nil
}
