package privacy

import (
	memcore "github.com/compozy/compozy/engine/memory/core"
)

// Validator provides validation for privacy policies
type Validator struct{}

// NewValidator creates a new privacy policy validator
func NewValidator() *Validator {
	return &Validator{}
}

// ValidatePolicy validates a privacy policy configuration
func (v *Validator) ValidatePolicy(policy *memcore.PrivacyPolicyConfig) error {
	if policy == nil {
		return nil
	}
	// Validate regex patterns
	return v.ValidateRedactionPatterns(policy.RedactPatterns)
}

// ValidateRedactionPatterns validates all redaction patterns in a policy
func (v *Validator) ValidateRedactionPatterns(patterns []string) error {
	// Use the common validation function from patterns.go but wrap errors
	// in memcore.NewMemoryError for consistent error handling in validator context
	for _, pattern := range patterns {
		if pattern == "" {
			continue // Skip empty patterns
		}
		if err := ValidateRedactionPattern(pattern); err != nil {
			// ValidateRedactionPattern already returns memcore.NewMemoryError
			return err
		}
	}
	return nil
}
