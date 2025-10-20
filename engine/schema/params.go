package schema

import (
	"context"
	"errors"
	"fmt"
)

type ParamsValidator struct {
	id     string
	params any
	schema *Schema
}

func NewParamsValidator[T any](with T, schema *Schema, id string) *ParamsValidator {
	return &ParamsValidator{
		id:     id,
		params: with,
		schema: schema,
	}
}

func (v *ParamsValidator) Validate(ctx context.Context) error {
	if v.schema == nil {
		return nil
	}
	if v.params == nil {
		return fmt.Errorf(
			"%w for %s: %s",
			errors.New("validation error"),
			v.id,
			"parameters are nil but a schema is defined",
		)
	}
	result, err := v.schema.Validate(ctx, v.params)
	if err != nil {
		return fmt.Errorf("validation error for %s: %w", v.id, err)
	}
	if !result.Valid {
		return fmt.Errorf("validation error for %s: %v", v.id, result.Errors)
	}
	return nil
}
