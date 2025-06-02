package schema

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/engine/core"
)

type ParamsValidator struct {
	id     string
	params *core.Input
	schema *Schema
}

func NewParamsValidator(with *core.Input, schema *Schema, id string) *ParamsValidator {
	return &ParamsValidator{
		id:     id,
		params: with,
		schema: schema,
	}
}

func (v *ParamsValidator) Validate(ctx context.Context) error {
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
	result, err := v.schema.Validate(ctx, v.params)
	if err != nil {
		return fmt.Errorf("validation error for %s: %w", v.id, err)
	}
	if !result.Valid {
		return fmt.Errorf("validation error for %s: %v", v.id, result.Errors)
	}
	return nil
}
