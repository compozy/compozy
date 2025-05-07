package schema

import (
	"github.com/compozy/compozy/internal/parser/common"
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
		return nil
	}

	// Convert With to a map for validation
	withData := map[string]any(*v.with)

	// Use the Schema's Validate method
	if err := v.inputSchema.Schema.Validate(withData); err != nil {
		return NewInvalidWithParamsError(v.id, err)
	}

	return nil
}
