package privacy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPatternValidation(t *testing.T) {
	t.Run("Should accept valid regex patterns", func(t *testing.T) {
		validPatterns := []string{
			`\b[A-Z]{2,4}\b`,
			`\d{4}-\d{4}`,
			`\b\d{3}-\d{2}-\d{4}\b`,
			`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`,
		}
		for _, pattern := range validPatterns {
			err := ValidateRedactionPattern(pattern)
			assert.NoError(t, err, "Pattern %s should be valid", pattern)
		}
	})
	t.Run("Should reject invalid regex patterns", func(t *testing.T) {
		invalidPattern := `[invalid(`
		err := ValidateRedactionPattern(invalidPattern)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid regex pattern")
	})
	t.Run("Should reject ReDoS vulnerable patterns", func(t *testing.T) {
		dangerousPatterns := []string{
			`(a+)+`,
			`(a*)*`,
			`(a|a)*`,
			`(.+)+`,
		}
		for _, pattern := range dangerousPatterns {
			err := ValidateRedactionPattern(pattern)
			assert.Error(t, err, "Pattern %s should be rejected as dangerous", pattern)
			assert.Contains(t, err.Error(), "unsafe regex pattern")
		}
	})
}

func TestMultiplePatternValidation(t *testing.T) {
	t.Run("Should validate multiple patterns", func(t *testing.T) {
		patterns := []string{
			`\b\d{3}-\d{2}-\d{4}\b`,
			`\b[A-Z]{2,4}\b`,
			`\d{4}-\d{4}`,
		}
		err := ValidateRedactionPatterns(patterns)
		require.NoError(t, err)
	})
	t.Run("Should skip empty patterns", func(t *testing.T) {
		patterns := []string{
			`\b\d{3}-\d{2}-\d{4}\b`,
			"",
			`\b[A-Z]{2,4}\b`,
			"",
		}
		err := ValidateRedactionPatterns(patterns)
		require.NoError(t, err)
	})
	t.Run("Should return error for any invalid pattern", func(t *testing.T) {
		patterns := []string{
			`\b\d{3}-\d{2}-\d{4}\b`,
			`[invalid(`, // Invalid pattern
			`\b[A-Z]{2,4}\b`,
		}
		err := ValidateRedactionPatterns(patterns)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid regex pattern")
	})
	t.Run("Should return error for ReDoS patterns", func(t *testing.T) {
		patterns := []string{
			`\b\d{3}-\d{2}-\d{4}\b`,
			`(a+)+`, // ReDoS vulnerable
			`\b[A-Z]{2,4}\b`,
		}
		err := ValidateRedactionPatterns(patterns)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsafe regex pattern")
	})
}
