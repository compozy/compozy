package settings

import (
	"context"
	"fmt"
)

func (s *service) populateMCPServerStatuses(
	ctx context.Context,
	item *MCPServerItem,
	entry mcpSourceEntry,
) error {
	target, err := mcpAuthTargetForSource(entry)
	if err != nil {
		return err
	}
	name := entry.Server.Name
	if s.mcpAuth != nil && !entry.Server.Auth.IsZero() {
		status, statusErr := s.mcpAuth.MCPAuthStatus(ctx, target, entry.Server)
		if statusErr != nil {
			return fmt.Errorf("settings: load MCP auth status for %q: %w", name, statusErr)
		}
		item.AuthStatus = &status
	}
	if s.mcpRuntime != nil {
		status, statusErr := s.mcpRuntime.MCPServerRuntimeStatus(ctx, target, entry.Server)
		if statusErr != nil {
			return fmt.Errorf("settings: load MCP runtime status for %q: %w", name, statusErr)
		}
		item.RuntimeStatus = &status
	}
	return nil
}

// markInheritedMCPAuthStatuses applies GetMCPAuthStatus's rule to a listing: a status
// whose declaring scope is broader than the listed scope is the inherited credential.
func markInheritedMCPAuthStatuses(items []mcpCollectionItem, listed MCPAuthTargetRequest) {
	for index := range items {
		status := items[index].item.AuthStatus
		if status == nil {
			continue
		}
		resolved, err := mcpAuthTargetForSource(items[index].entry)
		if err != nil {
			continue
		}
		listed.Name, listed.Owner = resolved.ServerName, resolved.Owner
		requested, err := normalizeMCPAuthTarget(listed)
		if err != nil {
			continue
		}
		status.Inherited = !mcpAuthTargetCovers(requested, resolved.Normalize())
	}
}
