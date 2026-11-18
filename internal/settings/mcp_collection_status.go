package settings

import (
	"context"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
)

func (s *service) populateMCPServerStatuses(
	ctx context.Context,
	item *MCPServerItem,
	entry mcpSourceEntry,
	skipRuntimeStatus, cachedRuntimeStatus bool,
) error {
	ctx = mcpauth.WithConsentScope(
		ctx,
		mcpauth.ConsentScope{ProfileName: item.ProfileName, WorkspaceID: item.WorkspaceID},
	)
	target, err := mcpAuthTargetForSource(entry)
	if err != nil {
		return err
	}
	if s.mcpConsent != nil {
		req := consentRequestForTarget(target, entry.Server)
		// The listing profile owns consent even when credentials are inherited.
		req.ProfileName = item.ProfileName
		req.WorkspaceID = item.WorkspaceID
		decision, consentErr := s.mcpConsent.MCPConsent(ctx, req)
		if consentErr != nil {
			return consentErr
		}
		required := (target.Scope == mcpauth.ScopeWorkspace || target.Scope == mcpauth.ScopeWorkspaceProfile) &&
			!strings.HasPrefix(target.Owner, "extension:")
		item.Approval = approvalState(decision, required)
		item.CallableInSessions = decision.Admitted
		skipRuntimeStatus = skipRuntimeStatus || !decision.Admitted
	}
	name := entry.Server.Name
	if s.mcpAuth != nil && mcpauth.OAuthCapable(entry.Server) && entry.Server.Validate("mcp_server") == nil {
		status, statusErr := s.mcpAuth.MCPAuthStatus(ctx, target, entry.Server)
		if statusErr != nil {
			return fmt.Errorf("settings: load MCP auth status for %q: %w", name, statusErr)
		}
		item.AuthStatus = &status
	}
	if s.mcpRuntime != nil && !skipRuntimeStatus {
		var status MCPServerRuntimeStatus
		var statusErr error
		if cachedRuntimeStatus {
			status = MCPServerRuntimeStatus{
				Configured: true,
				State:      MCPServerRuntimeStateUnknown,
				Probe:      MCPServerProbeSkipped,
				Reason:     "not_observed",
			}
			if reader, ok := s.mcpRuntime.(MCPCachedRuntimeProvider); ok {
				status, statusErr = reader.MCPServerCachedRuntimeStatus(ctx, target, entry.Server)
			}
		} else {
			status, statusErr = s.mcpRuntime.MCPServerRuntimeStatus(ctx, target, entry.Server)
		}
		if statusErr != nil {
			return fmt.Errorf("settings: load MCP runtime status for %q: %w", name, statusErr)
		}
		setMCPRuntimeObservation(item, entry.Server, status)
	}
	return nil
}

func setMCPRuntimeObservation(item *MCPServerItem, server compozyconfig.MCPServer, status MCPServerRuntimeStatus) {
	item.RuntimeStatus = &status
	if item.AuthStatus != nil && item.AuthStatus.Status == mcpauth.StatusUnconfigured &&
		status.State == MCPServerRuntimeStateAuthRequired && mcpauth.OAuthWithPluginHeaders(server) {
		item.AuthStatus.Status = mcpauth.StatusNeedsLogin
		item.AuthStatus.AuthType = "oauth"
	}
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
