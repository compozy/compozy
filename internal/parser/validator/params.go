package validator

import (
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/parser/schema"
)

type ParamsValidator struct {
	id     string
	params map[string]any
	schema schema.Schema
}

func NewParamsValidator(with map[string]any, schema schema.Schema, id string) *ParamsValidator {
	return &ParamsValidator{
		id:     id,
		params: with,
		schema: schema,
	}
}

func (v *ParamsValidator) Validate() error {
	if v.params == nil || v.schema == nil {
		return nil
	}
	if err := v.schema.Validate(v.params); err != nil {
		return fmt.Errorf("%w for %s: %w", errors.New("with parameters invalid"), v.id, err)
	}
	return nil
}
