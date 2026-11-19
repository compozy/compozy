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
	definition, found, err := s.resolveMCPExtensionDefinition(ctx, req)
	return definition.Target, definition.Server, found, err
}

func (s settingsMCPExtensionDefinitions) resolveMCPExtensionDefinition(
	ctx context.Context, req settingspkg.MCPAuthTargetRequest,
) (settingspkg.MCPExtensionDefinition, bool, error) {
	if s.state == nil || s.state.mcpServerCatalog == nil {
		return settingspkg.MCPExtensionDefinition{}, false, nil
	}
	var definition settingspkg.MCPExtensionDefinition
	found, foundExact := false, false
	for _, record := range s.state.mcpServerCatalog.Snapshot() {
		owner := record.Owner.Normalize()
		if owner.Kind != extensionResourceOwnerKind {
			continue
		}
		if strings.TrimSpace(req.Owner) != "extension:"+owner.ID {
			continue
		}
		name := strings.TrimSpace(req.Name)
		if name != record.Spec.Name ||
			(req.DefinitionSource != "" && req.DefinitionSource != record.Spec.DefinitionSource) {
			continue
		}
		target, err := mcpAuthTargetForResource(ctx, s.state, record.Scope, record.Spec.Name, owner)
		if err != nil {
			return settingspkg.MCPExtensionDefinition{}, false, err
		}
		management := publishedMCPManagementTarget(record.Scope, target)
		if !settingsMCPExtensionTargetVisible(req, target) && !settingsMCPExtensionTargetVisible(req, management) {
			continue
		}
		exact := settingsMCPExtensionTargetMatches(req, target) || settingsMCPExtensionTargetMatches(req, management)
		if found && foundExact && !exact {
			continue
		}
		if found && foundExact == exact {
			return settingspkg.MCPExtensionDefinition{}, false,
				fmt.Errorf("daemon: multiple extension MCP definitions address %q", name)
		}
		server, err := projectExtensionSecretHeaders(ctx, s.state, record, "")
		if err != nil {
			return settingspkg.MCPExtensionDefinition{}, false, err
		}
		definition = settingspkg.MCPExtensionDefinition{Target: target, ManagementTarget: management, Server: server}
		found, foundExact = true, exact
	}
	return definition, found, nil
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
