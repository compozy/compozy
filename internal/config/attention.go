package config

import (
	"fmt"
	"strings"

	"github.com/oklog/ulid"
)

const (
	attentionConfigInvalidCode = "attention_config_invalid"
	attentionMutedPath         = "attention.muted_workspaces"
)

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
		workspaceID := strings.TrimSpace(value)
		if workspaceID == "" {
			return attentionValidationError(index, "must be a canonical workspace id")
		}
		if _, err := ulid.ParseStrict(workspaceID); err != nil {
			return attentionValidationError(index, "must be a canonical workspace id")
		}
		if _, exists := seen[workspaceID]; exists {
			return attentionValidationError(index, "must not contain duplicates")
		}
		seen[workspaceID] = struct{}{}
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
