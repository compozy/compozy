package config

import (
	"fmt"
	"strings"

	toolspkg "github.com/compozy/agh/internal/tools"
)

func normalizeAgentToolPatterns(values []string) []string {
	return normalizeAgentStringRefs(values)
}

func normalizeAgentToolsetRefs(values []string) []string {
	return normalizeAgentStringRefs(values)
}

func normalizeAgentStringRefs(values []string) []string {
	refs := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		refs = append(refs, trimmed)
	}
	return refs
}

func validateAgentToolPatterns(values []string, path string) error {
	for idx, value := range values {
		if err := validateAgentToolPattern(value, fmt.Sprintf("%s[%d]", path, idx)); err != nil {
			return err
		}
	}
	return nil
}

func validateAgentToolPattern(value string, path string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("%s is required", path)
	}
	if strings.Contains(trimmed, "*") {
		return validateAgentToolWildcard(trimmed, path)
	}
	return validateAgentToolIDAtom(trimmed, path)
}

func validateAgentToolWildcard(value string, path string) error {
	if strings.Count(value, "*") != 1 || !strings.HasSuffix(value, "*") {
		return fmt.Errorf("%s must be exact canonical ToolID or namespace-prefix wildcard: %q", path, value)
	}
	prefix := strings.TrimSuffix(value, "*")
	if prefix == "" || (!strings.HasSuffix(prefix, "_") && !strings.HasSuffix(prefix, "__")) {
		return fmt.Errorf("%s must be exact canonical ToolID or namespace-prefix wildcard: %q", path, value)
	}
	return validateAgentToolIDAtom(prefix+"x", path)
}

func validateAgentToolIDAtom(value string, path string) error {
	if err := toolspkg.ToolID(value).Validate(); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if !strings.Contains(value, "__") {
		return fmt.Errorf("%s must include canonical namespace separator %q: %q", path, "__", value)
	}
	return nil
}

func validateAgentToolsets(values []string, path string) error {
	for idx, value := range values {
		trimmed := strings.TrimSpace(value)
		field := fmt.Sprintf("%s[%d]", path, idx)
		if trimmed == "" {
			return fmt.Errorf("%s is required", field)
		}
		if err := toolspkg.ToolsetID(trimmed).Validate(); err != nil {
			return fmt.Errorf("%s: %w", field, err)
		}
		if !strings.Contains(trimmed, "__") {
			return fmt.Errorf("%s must include canonical namespace separator %q: %q", field, "__", trimmed)
		}
	}
	return nil
}
