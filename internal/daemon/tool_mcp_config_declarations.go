package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func appendProfileConfigMCPDeclarations(
	ctx context.Context,
	desired *toolMCPDesiredResources,
	home compozyconfig.HomePaths,
	workspaces []workspacepkg.ResolvedWorkspace,
	profiles extensionProfileCatalog,
	logger *slog.Logger,
	user []compozyconfig.MCPServer,
) error {
	if profiles == nil {
		return nil
	}
	rows, err := profiles.List(ctx)
	if err != nil {
		return fmt.Errorf("daemon: list profiles for MCP publication: %w", err)
	}
	for _, profile := range activeExtensionProfileLenses(rows) {
		if profile.ID == store.DefaultProfileID {
			continue
		}
		targets := []struct{ id, root string }{{}}
		for idx := range workspaces {
			workspace := &workspaces[idx]
			targets = append(targets, struct{ id, root string }{workspace.ID, workspace.RootDir})
		}
		for _, target := range targets {
			scope := resources.ResourceScope{Kind: resources.ResourceScopeKindProfile, ID: profile.ID}
			if target.id != "" {
				scope = resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspaceProfile,
					ID: target.id + "@pf:" + profile.Name}
			}
			servers, err := compozyconfig.DeclaredMCPServers(
				home,
				target.root,
				compozyconfig.WriteScopeProfile,
				profile.Name,
				user,
			)
			if err != nil {
				if logger != nil {
					logger.WarnContext(
						ctx,
						"daemon: profile MCP declarations unavailable; keeping published servers",
						"scope",
						scope.Kind,
						"scope_id",
						scope.ID,
						"error",
						err,
					)
				}
				desired.heldMCPScopes = append(desired.heldMCPScopes, scope)
				continue
			}
			for _, server := range servers {
				desired.mcpServers = append(desired.mcpServers, mcpServerPublicationInput{
					sourceKey: "config/" + string(scope.Kind) + "/" + scope.ID + "/" + strings.TrimSpace(server.Name),
					scope:     scope, spec: cloneDaemonMCPServer(server),
				})
			}
		}
	}
	return nil
}

func appendUserConfigMCPDeclarations(
	ctx context.Context,
	desired *toolMCPDesiredResources,
	home compozyconfig.HomePaths,
	base []compozyconfig.MCPServer,
	logger *slog.Logger,
) []compozyconfig.MCPServer {
	scope := resources.ResourceScope{Kind: resources.ResourceScopeKindUser}
	servers, err := compozyconfig.DeclaredMCPServers(home, "", compozyconfig.WriteScopeUser, "", base)
	if err != nil {
		desired.heldMCPScopes = append(desired.heldMCPScopes, scope)
		if logger != nil {
			logger.WarnContext(ctx, "daemon: user MCP declarations unavailable; keeping published servers")
		}
		return base
	}
	for _, server := range servers {
		desired.mcpServers = append(desired.mcpServers, mcpServerPublicationInput{
			sourceKey: "config/global/" + strings.TrimSpace(
				server.Name,
			),
			scope: scope,
			spec:  cloneDaemonMCPServer(server),
		})
	}
	return servers
}
