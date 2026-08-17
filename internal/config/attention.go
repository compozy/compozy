package config

import (
	"fmt"
	"regexp"
)

const (
	attentionConfigInvalidCode = "attention_config_invalid"
	attentionMutedPath         = "attention.muted_workspaces"
)

var attentionWorkspaceRegistrationIDPattern = regexp.MustCompile(`^ws_[0-9a-f]{16}$`)

// AttentionConfig controls operator-facing attention delivery.
type AttentionConfig struct {
	Toasts          bool     `toml:"toasts"`
	Sound           bool     `toml:"sound"`
	System          bool     `toml:"system"`
	MutedWorkspaces []string `toml:"muted_workspaces,omitempty"`
}

// DefaultAttentionConfig returns the built-in operator attention policy.
func DefaultAttentionConfig() AttentionConfig {
	return AttentionConfig{Toasts: true, Sound: true}
}

// Validate rejects malformed or duplicate workspace identities.
func (c AttentionConfig) Validate() error {
	seen := make(map[string]struct{}, len(c.MutedWorkspaces))
	for index, value := range c.MutedWorkspaces {
		if !attentionWorkspaceRegistrationIDPattern.MatchString(value) {
			return attentionValidationError(index, "must be a public workspace registration id")
		}
		if _, exists := seen[value]; exists {
			return attentionValidationError(index, "must not contain duplicates")
		}
		seen[value] = struct{}{}
	}
	return nil
}

func attentionValidationError(index int, message string) error {
	return ValidationError{
		Code:    attentionConfigInvalidCode,
		Path:    attentionMutedPath,
		Message: fmt.Sprintf("entry %d %s", index, message),
	}
}
