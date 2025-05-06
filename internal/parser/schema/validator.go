package schema

import (
	"github.com/go-playground/validator/v10"
)

// Validator defines the interface for validation
type Validator interface {
	Validate() error
}

// CompositeValidator allows combining multiple validators
type CompositeValidator struct {
	validators []Validator
}

// NewCompositeValidator creates a new CompositeValidator with the given validators
func NewCompositeValidator(validators ...Validator) *CompositeValidator {
	return &CompositeValidator{
		validators: validators,
	}
}

// AddValidator adds a validator to the composite
func (v *CompositeValidator) AddValidator(validator Validator) {
	v.validators = append(v.validators, validator)
}

// Validate runs all validators in sequence
func (v *CompositeValidator) Validate() error {
	for _, validator := range v.validators {
		if err := validator.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// StructValidator wraps go-playground/validator for struct validation
type StructValidator struct {
	validate *validator.Validate
	value    any
}

// NewStructValidator creates a new StructValidator for the given value
func NewStructValidator(value any) *StructValidator {
	return &StructValidator{
		validate: validator.New(),
		value:    value,
	}
}

// Validate performs struct validation using go-playground/validator
func (v *StructValidator) Validate() error {
	return v.validate.Struct(v.value)
}

// RegisterValidation registers a custom validation function
func (v *StructValidator) RegisterValidation(tag string, fn validator.Func) error {
	return v.validate.RegisterValidation(tag, fn)
}
