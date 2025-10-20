package privacy

import (
	"fmt"
	"regexp"

	memcore "github.com/compozy/compozy/engine/memory/core"
)

// ValidateRedactionPattern validates a regex pattern for safety and correctness
// It checks for valid regex syntax and potential ReDoS vulnerabilities
func ValidateRedactionPattern(pattern string) error {
	if _, err := regexp.Compile(pattern); err != nil {
		return memcore.NewMemoryError(
			memcore.ErrCodePrivacyValidation,
			fmt.Sprintf("invalid regex pattern '%s'", pattern),
			err,
		).WithContext("pattern", pattern)
	}
	if err := validateRedactionPattern(pattern); err != nil {
		return memcore.NewMemoryError(
			memcore.ErrCodePrivacyValidation,
			fmt.Sprintf("unsafe regex pattern '%s'", pattern),
			err,
		).WithContext("pattern", pattern)
	}
	return nil
}

// ValidateRedactionPatterns validates multiple regex patterns
func ValidateRedactionPatterns(patterns []string) error {
	for _, pattern := range patterns {
		if pattern == "" {
			continue // Skip empty patterns
		}
		if err := ValidateRedactionPattern(pattern); err != nil {
			return err
		}
	}
	return nil
}
