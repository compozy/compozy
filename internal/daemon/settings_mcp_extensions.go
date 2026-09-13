package daemon

import (
	"context"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	settingspkg "github.com/compozy/compozy/internal/settings"
)

type settingsMCPExtensionDefinitions struct {
	state *bootState
	auth  interface {
		MCPAuthInvalidate(mcpauth.Target) error
		MCPAuthDeleteState(context.Context, mcpauth.Target) error
	}
}

func (s settingsMCPExtensionDefinitions) ResolveMCPExtensionDefinition(
	ctx context.Context, req settingspkg.MCPAuthTargetRequest,
) (mcpauth.Target, compozyconfig.MCPServer, bool, error) {
	if s.state == nil || s.state.mcpServerCatalog == nil {
		return mcpauth.Target{}, compozyconfig.MCPServer{}, false, nil
	}
	var foundTarget mcpauth.Target
	var foundServer compozyconfig.MCPServer
	found, foundExact := false, false
	for _, record := range s.state.mcpServerCatalog.Snapshot() {
		owner := record.Owner.Normalize()
		if owner.Kind != extensionResourceOwnerKind {
			continue
		}
		if req.Owner != "" && strings.TrimSpace(req.Owner) != "extension:"+owner.ID {
			continue
		}
		name := strings.TrimSpace(req.Name)
		if name != record.Spec.RuntimeName && (req.Owner == "" || name != record.Spec.Name) {
			continue
		}
		target, err := mcpAuthTargetForResource(ctx, s.state, record.Scope, record.Spec.Name, owner)
		if err != nil {
			return mcpauth.Target{}, compozyconfig.MCPServer{}, false, err
		}
		if !settingsMCPExtensionTargetVisible(req, target) {
			continue
		}
		exact := settingsMCPExtensionTargetMatches(req, target)
		if found && foundExact && !exact {
			continue
		}
		if found && foundExact == exact {
			return mcpauth.Target{}, compozyconfig.MCPServer{}, false,
				fmt.Errorf("daemon: multiple extension MCP definitions address %q", name)
		}
		server, err := projectExtensionSecretHeaders(ctx, s.state, record, "")
		if err != nil {
			return mcpauth.Target{}, compozyconfig.MCPServer{}, false, err
		}
		foundTarget, foundServer, found, foundExact = target, server, true, exact
	}
	return foundTarget, foundServer, found, nil
}

func settingsMCPExtensionTargetMatches(req settingspkg.MCPAuthTargetRequest, target mcpauth.Target) bool {
	scope := mcpauth.Scope(req.Scope)
	scopeID := strings.TrimSpace(req.WorkspaceID)
	if req.Scope == settingspkg.ScopeProfile {
		if scopeID == "" {
			scopeID = strings.TrimSpace(req.ProfileName)
		} else {
			scope = mcpauth.ScopeWorkspaceProfile
			scopeID += "@pf:" + strings.TrimSpace(req.ProfileName)
		}
	}
	return target.Scope == scope && target.WorkspaceID == scopeID
}
