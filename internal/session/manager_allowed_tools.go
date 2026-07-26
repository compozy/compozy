package session

import (
	"fmt"
	"slices"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/store"
	toolspkg "github.com/compozy/agh/internal/tools"
)

// WithToolsetCatalog injects the catalog used to validate toolset-backed overrides.
func WithToolsetCatalog(catalog toolspkg.ToolsetCatalog) Option {
	return func(manager *Manager) {
		manager.toolsetCatalog = catalog
	}
}

func (s *sessionStartSpec) applyAllowedToolsOverride(
	resolved *aghconfig.ResolvedAgent,
	catalog toolspkg.ToolsetCatalog,
) error {
	override, hasOverride, err := normalizeAllowedToolsOverride(s.allowedToolsOverride)
	if err != nil {
		return err
	}
	if !hasOverride {
		return nil
	}
	if resolved == nil {
		return fmt.Errorf("%w: resolved agent is required for allowed_tools override", ErrValidation)
	}
	if err := validateAllowedToolsOverrideSubset(*resolved, override, catalog); err != nil {
		return err
	}

	resolved.Tools = append([]string(nil), override...)
	resolved.Toolsets = nil

	lineage := store.NormalizeSessionLineage(s.sessionID, s.lineage)
	lineage.PermissionPolicy.Tools = append([]string(nil), override...)
	lineage.PermissionPolicy = store.NormalizeSessionPermissionPolicy(lineage.PermissionPolicy)
	if err := store.ValidateSessionLineage(s.sessionID, lineage); err != nil {
		return fmt.Errorf("%w: allowed_tools override lineage policy: %w", ErrValidation, err)
	}
	s.lineage = lineage
	return nil
}

func normalizeAllowedToolsOverride(values []string) ([]string, bool, error) {
	if len(values) == 0 {
		return nil, false, nil
	}
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for idx, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return nil, false, fmt.Errorf("%w: allowed_tools[%d] is required", ErrValidation, idx)
		}
		id := toolspkg.ToolID(trimmed)
		if err := id.Validate(); err != nil {
			return nil, false, fmt.Errorf(
				"%w: allowed_tools[%d] %q must be a canonical ToolID: %w",
				ErrValidation,
				idx,
				trimmed,
				err,
			)
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	slices.Sort(normalized)
	return normalized, len(normalized) > 0, nil
}

func validateAllowedToolsOverrideSubset(
	resolved aghconfig.ResolvedAgent,
	requested []string,
	catalog toolspkg.ToolsetCatalog,
) error {
	allowPatterns, err := toolspkg.ParseToolPatterns(resolved.Tools)
	if err != nil {
		return fmt.Errorf("%w: agent tools policy is invalid: %w", ErrValidation, err)
	}
	denyPatterns, err := toolspkg.ParseToolPatterns(resolved.DenyTools)
	if err != nil {
		return fmt.Errorf("%w: agent deny_tools policy is invalid: %w", ErrValidation, err)
	}
	toolsets, err := validateAllowedToolsOverrideToolsets(resolved.Toolsets)
	if err != nil {
		return err
	}

	agentRestrictsTools := len(allowPatterns) > 0 || len(toolsets) > 0
	for _, raw := range requested {
		id := toolspkg.ToolID(raw)
		if matchesAllowedToolsPattern(denyPatterns, id) {
			return fmt.Errorf("%w: allowed_tools override tool %q is denied by agent profile", ErrValidation, raw)
		}
		toolsetMember, err := catalog.Contains(id, toolsets)
		if err != nil {
			return fmt.Errorf("%w: resolve agent toolsets for allowed_tools override: %w", ErrValidation, err)
		}
		if !agentRestrictsTools || matchesAllowedToolsPattern(allowPatterns, id) || toolsetMember {
			continue
		}
		return fmt.Errorf("%w: allowed_tools override tool %q widens agent profile", ErrValidation, raw)
	}
	return nil
}

func validateAllowedToolsOverrideToolsets(values []string) ([]toolspkg.ToolsetID, error) {
	toolsets := make([]toolspkg.ToolsetID, 0, len(values))
	for idx, raw := range values {
		trimmed := strings.TrimSpace(raw)
		id := toolspkg.ToolsetID(trimmed)
		if err := id.Validate(); err != nil {
			return nil, fmt.Errorf(
				"%w: agent toolsets[%d] %q must be a canonical ToolsetID: %w",
				ErrValidation,
				idx,
				trimmed,
				err,
			)
		}
		toolsets = append(toolsets, id)
	}
	return toolsets, nil
}

func matchesAllowedToolsPattern(patterns []toolspkg.ToolPattern, id toolspkg.ToolID) bool {
	for _, pattern := range patterns {
		if pattern.Match(id) {
			return true
		}
	}
	return false
}
