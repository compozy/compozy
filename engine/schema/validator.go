package schema

import (
	"context"

	"github.com/go-playground/validator/v10"
)

// -----------------------------------------------------------------------------
// Validator interface
// -----------------------------------------------------------------------------

type Validator interface {
	Validate(ctx context.Context) error
}

// -----------------------------------------------------------------------------
// CompositeValidator
// -----------------------------------------------------------------------------

// CompositeValidator allows combining multiple validators
type CompositeValidator struct {
	validators []Validator
}

func NewCompositeValidator(validators ...Validator) *CompositeValidator {
	return &CompositeValidator{
		validators: validators,
	}
}

func (v *CompositeValidator) AddValidator(validator Validator) {
	v.validators = append(v.validators, validator)
}

func (v *CompositeValidator) Validate(ctx context.Context) error {
	for _, validator := range v.validators {
		if err := validator.Validate(ctx); err != nil {
			return err
		}
	}
	return nil
}

// -----------------------------------------------------------------------------
// StructValidator
// -----------------------------------------------------------------------------

type StructValidator struct {
	validate *validator.Validate
	value    any
}

func NewStructValidator(value any) *StructValidator {
	return &StructValidator{
		validate: validator.New(),
		value:    value,
	}
}

func (v *StructValidator) Validate(_ context.Context) error {
	return v.validate.Struct(v.value)
}

func (v *StructValidator) RegisterValidation(tag string, fn validator.Func) error {
	return v.validate.RegisterValidation(tag, fn)
}
