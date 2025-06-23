package privacy

import (
	"fmt"
	"regexp"

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
	for _, pattern := range patterns {
		// First check for ReDoS vulnerability
		if err := validateRedactionPattern(pattern); err != nil {
			return memcore.NewMemoryError(
				memcore.ErrCodePrivacyValidation,
				fmt.Sprintf("unsafe regex pattern: %s", pattern),
				err,
			).WithContext("pattern", pattern)
		}
		// Then try to compile it
		if _, err := regexp.Compile(pattern); err != nil {
			return memcore.NewMemoryError(
				memcore.ErrCodePrivacyValidation,
				fmt.Sprintf("invalid regex pattern: %s", pattern),
				err,
			).WithContext("pattern", pattern)
		}
	}
	return nil
}
