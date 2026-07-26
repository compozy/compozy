package automation

import modelpkg "github.com/compozy/agh/internal/automation/model"

// DefaultRetryConfig returns the default retry policy for automation definitions.
func DefaultRetryConfig() RetryConfig {
	return modelpkg.DefaultRetryConfig()
}

// DefaultBackoffRetryConfig returns the default exponential backoff retry policy.
func DefaultBackoffRetryConfig() RetryConfig {
	return modelpkg.DefaultBackoffRetryConfig()
}

// DefaultFireLimitConfig returns the default rolling fire-limit policy.
func DefaultFireLimitConfig() FireLimitConfig {
	return modelpkg.DefaultFireLimitConfig()
}

// ValidateScopeBinding enforces the global/workspace binding invariants shared by jobs, triggers, and envelopes.
func ValidateScopeBinding(scope Scope, workspaceBinding string, path string, workspaceField string) error {
	return modelpkg.ValidateScopeBinding(scope, workspaceBinding, path, workspaceField)
}

// ValidateTriggerFilter ensures trigger filters only reference supported activation-envelope field paths.
func ValidateTriggerFilter(filter map[string]string, path string) error {
	return modelpkg.ValidateTriggerFilter(filter, path)
}
