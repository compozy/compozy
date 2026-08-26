package session

import (
	"fmt"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

// WithToolsetCatalog injects the catalog used to validate toolset-backed overrides.
func WithToolsetCatalog(catalog toolspkg.ToolsetCatalog) Option {
	return func(manager *Manager) {
		manager.toolsetCatalog = catalog
	}
}

// WithToolUniverse injects the concrete native tool IDs used to materialize
// root-session delegation budgets from agent tool patterns and toolsets.
func WithToolUniverse(ids []toolspkg.ToolID) Option {
	return func(manager *Manager) {
		manager.toolUniverse = append([]toolspkg.ToolID(nil), ids...)
	}
}

func (s *sessionStartSpec) applyAllowedToolsOverride(
	resolved *compozyconfig.ResolvedAgent,
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

func (s *sessionStartSpec) materializeRootDelegationTools(
	resolved compozyconfig.ResolvedAgent,
	catalog toolspkg.ToolsetCatalog,
	universe []toolspkg.ToolID,
) error {
	if s == nil || normalizeSessionType(s.sessionType) != SessionTypeUser {
		return nil
	}
	lineage := store.NormalizeSessionLineage(s.sessionID, s.lineage)
	if strings.TrimSpace(lineage.ParentSessionID) != "" || len(lineage.PermissionPolicy.Tools) > 0 {
		return nil
	}
	tools, err := concreteDelegationTools(resolved, catalog, universe)
	if err != nil {
		return err
	}
	if len(tools) == 0 {
		return nil
	}
	lineage.PermissionPolicy.Tools = tools
	lineage.PermissionPolicy = store.NormalizeSessionPermissionPolicy(lineage.PermissionPolicy)
	if err := store.ValidateSessionLineage(s.sessionID, lineage); err != nil {
		return fmt.Errorf("%w: root delegation lineage policy: %w", ErrValidation, err)
	}
	s.lineage = lineage
	return nil
}

func concreteDelegationTools(
	resolved compozyconfig.ResolvedAgent,
	catalog toolspkg.ToolsetCatalog,
	universe []toolspkg.ToolID,
) ([]string, error) {
	denyPatterns, err := toolspkg.ParseToolPatterns(resolved.DenyTools)
	if err != nil {
		return nil, fmt.Errorf("%w: agent deny_tools policy is invalid: %w", ErrValidation, err)
	}
	candidates := make(map[toolspkg.ToolID]struct{})
	if len(resolved.Tools) == 0 && len(resolved.Toolsets) == 0 {
		for _, id := range universe {
			candidates[id] = struct{}{}
		}
	}
	for idx, raw := range resolved.Tools {
		pattern, parseErr := toolspkg.ParseToolPattern(raw)
		if parseErr != nil {
			return nil, fmt.Errorf("%w: agent tools[%d] is invalid: %w", ErrValidation, idx, parseErr)
		}
		if !strings.Contains(pattern.String(), "*") {
			candidates[toolspkg.ToolID(pattern.String())] = struct{}{}
			continue
		}
		for _, id := range universe {
			if pattern.Match(id) {
				candidates[id] = struct{}{}
			}
		}
	}
	if len(universe) > 0 {
		toolsets, parseErr := validateAllowedToolsOverrideToolsets(resolved.Toolsets)
		if parseErr != nil {
			return nil, parseErr
		}
		for _, toolsetID := range toolsets {
			expanded, expandErr := catalog.Expand(toolsetID, universe)
			if expandErr != nil {
				return nil, fmt.Errorf("%w: expand agent toolset %q: %w", ErrValidation, toolsetID, expandErr)
			}
			for _, id := range expanded {
				candidates[id] = struct{}{}
			}
		}
	}
	tools := make([]string, 0, len(candidates))
	for id := range candidates {
		if matchesAllowedToolsPattern(denyPatterns, id) {
			continue
		}
		tools = append(tools, id.String())
	}
	slices.Sort(tools)
	return tools, nil
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
	resolved compozyconfig.ResolvedAgent,
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
