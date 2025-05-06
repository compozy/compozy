package common

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
