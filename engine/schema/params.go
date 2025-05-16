package schema

import (
	"errors"
	"fmt"
)

type ParamsValidator struct {
	id     string
	params map[string]any
	schema Schema
}

func NewParamsValidator(with map[string]any, schema Schema, id string) *ParamsValidator {
	return &ParamsValidator{
		id:     id,
		params: with,
		schema: schema,
	}
}

func (v *ParamsValidator) Validate() error {
	// If there is no schema, there's nothing to validate against.
	if v.schema == nil {
		return nil
	}

	// If there is a schema, but no parameters are provided, this is an error.
	if v.params == nil {
		return fmt.Errorf(
			"%w for %s: %s",
			errors.New("validation error"),
			v.id,
			"parameters are nil but a schema is defined",
		)
	}

	// Both schema and parameters are present, proceed with validation.
	if err := v.schema.Validate(v.params); err != nil {
		return fmt.Errorf("%w for %s: %w", errors.New("with parameters invalid"), v.id, err)
	}

	return nil
}
