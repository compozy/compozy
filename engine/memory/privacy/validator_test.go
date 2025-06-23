package privacy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateRedactionPattern(t *testing.T) {
	t.Run("Should accept safe patterns", func(t *testing.T) {
		safePatterns := []string{
			`\b\d{4}-\d{4}-\d{4}-\d{4}\b`,
			`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`,
			`\b\d{3}-\d{2}-\d{4}\b`,
			`password:\s*\S+`,
			`api[_-]?key:\s*\S+`,
		}
		for _, pattern := range safePatterns {
			err := validateRedactionPattern(pattern)
			assert.NoError(t, err, "Pattern should be safe: %s", pattern)
		}
	})
	t.Run("Should reject dangerous patterns", func(t *testing.T) {
		dangerousPatterns := []struct {
			pattern string
			reason  string
		}{
			{`(\w+)+`, "nested quantifiers"},
			{`(a+)+`, "nested quantifiers"},
			{`(x+x+)+y`, "nested quantifiers"},
			{`([a-zA-Z]+)*`, "nested star quantifiers"},
			{`(a|a)*`, "alternation with star quantifier"},
			{`(a|b)*abb`, "alternation with star quantifier"},
			{`^(a+)+$`, "nested quantifiers"},
			{`(.*a){10,}`, "excessive repetition"},
			{`(.{0,50000})*`, "memory exhaustion"},
			{`a{999999}`, "excessive repetition"},
		}
		for _, test := range dangerousPatterns {
			err := validateRedactionPattern(test.pattern)
			assert.Error(t, err, "Pattern should be rejected: %s", test.pattern)
			if err != nil {
				assert.Contains(t, err.Error(), test.reason)
			}
		}
	})
	t.Run("Should handle empty and malformed patterns", func(t *testing.T) {
		err := validateRedactionPattern("")
		assert.NoError(t, err, "Empty pattern should be accepted")
		err = validateRedactionPattern("[unclosed")
		assert.NoError(t, err, "Pattern validation focuses on ReDoS, not syntax")
	})
	t.Run("Should validate complex safe patterns", func(t *testing.T) {
		complexSafePatterns := []string{
			`(?i)(?:password|pwd|pass|secret|key|token|auth)\s*[:=]\s*["']?([^"'\s]+)["']?`,
			`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`,
			`\b[A-F0-9]{8}-[A-F0-9]{4}-[A-F0-9]{4}-[A-F0-9]{4}-[A-F0-9]{12}\b`,
			`\b(?:sk_|pk_|rk_)[a-zA-Z0-9_]{20,}\b`,
		}
		for _, pattern := range complexSafePatterns {
			err := validateRedactionPattern(pattern)
			assert.NoError(t, err, "Complex safe pattern should be accepted: %s", pattern)
		}
	})
}
