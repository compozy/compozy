package config

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const agentNamePatternSource = `^[a-z][a-z0-9_-]*$`

var agentNamePattern = regexp.MustCompile(agentNamePatternSource)

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
		return fmt.Errorf("agent name %q must match %s", trimmed, agentNamePatternSource)
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
