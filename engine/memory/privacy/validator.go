package privacy

import (
	memcore "github.com/compozy/compozy/engine/memory/core"
)

// ValidatePrivacyPolicy validates a privacy policy configuration
func ValidatePrivacyPolicy(policy *memcore.PrivacyPolicyConfig) error {
	if policy == nil {
		return nil
	}
	return ValidateRedactionPatterns(policy.RedactPatterns)
}
