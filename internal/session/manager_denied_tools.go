package session

import (
	"fmt"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (s *sessionStartSpec) applyDeniedToolsOverride(resolved *compozyconfig.ResolvedAgent) error {
	if len(s.deniedToolsOverride) == 0 {
		return nil
	}
	if resolved == nil {
		return fmt.Errorf("%w: resolved agent is required for denied_tools override", ErrValidation)
	}
	merged := append([]string(nil), resolved.DenyTools...)
	merged = append(merged, s.deniedToolsOverride...)
	normalized, err := normalizeDeniedToolsOverride(merged)
	if err != nil {
		return err
	}
	resolved.DenyTools = normalized
	return nil
}

func normalizeDeniedToolsOverride(values []string) ([]string, error) {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for idx, raw := range values {
		trimmed := strings.TrimSpace(raw)
		pattern, err := toolspkg.ParseToolPattern(trimmed)
		if err != nil {
			return nil, fmt.Errorf(
				"%w: denied_tools[%d] %q must be a valid tool pattern: %w",
				ErrValidation,
				idx,
				trimmed,
				err,
			)
		}
		canonical := pattern.String()
		if _, ok := seen[canonical]; ok {
			continue
		}
		seen[canonical] = struct{}{}
		normalized = append(normalized, canonical)
	}
	slices.Sort(normalized)
	return normalized, nil
}
