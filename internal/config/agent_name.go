package config

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	// AgentNameMaxLength keeps an authored name safe when a normal generated
	// session id is composed into a Network Live peer id. Network peers allow
	// 128 characters; generated session ids use 21, and the separator uses one.
	AgentNameMaxLength = 106
	// agentNamePatternBody is the shared canonical grammar without anchors.
	agentNamePatternBody = `[a-z][a-z0-9_-]{0,105}`
	// AgentNamePattern is the canonical grammar for authored agent identities
	// and references to them.
	AgentNamePattern = `^` + agentNamePatternBody + `$`
	// OptionalAgentNamePattern accepts an empty canonical reference for fields
	// whose empty value means clear or inherit.
	OptionalAgentNamePattern = `^(?:|` + agentNamePatternBody + `)$`
)

var (
	agentNamePattern = regexp.MustCompile(AgentNamePattern)
)

// NormalizeAgentName returns the canonical in-memory agent identity.
func NormalizeAgentName(name string) string {
	return strings.TrimSpace(name)
}

// ValidateAgentName rejects names outside the canonical authored identity grammar.
func ValidateAgentName(name string) error {
	trimmed := NormalizeAgentName(name)
	if trimmed == "" {
		return errors.New("agent name is required")
	}
	if !agentNamePattern.MatchString(trimmed) {
		return fmt.Errorf(
			"agent name %q must start with a lowercase letter, use only lowercase letters, numbers, "+
				"hyphens, or underscores, and be at most %d characters",
			trimmed,
			AgentNameMaxLength,
		)
	}
	return nil
}

func validateAgentNameAtPath(name string, path string) error {
	if err := ValidateAgentName(name); err != nil {
		return ValidationError{Path: path, Message: err.Error()}
	}
	return nil
}

// ValidateAuthoredAgentName rejects names that cannot be materialized in an agent catalog.
func ValidateAuthoredAgentName(name string) error {
	trimmed := NormalizeAgentName(name)
	if err := ValidateAgentName(trimmed); err != nil {
		return err
	}
	if IsReservedAgentName(trimmed) {
		return fmt.Errorf("%w: %q", ErrAgentNameReserved, trimmed)
	}
	return nil
}
