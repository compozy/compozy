package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/filesnap"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/resources"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// registeredMCPWorkspaces reads identities without loading inherited config, so a malformed
// declaration in one scope cannot prevent publication of independent scopes.
func registeredMCPWorkspaces(ctx context.Context, registry workspaceRegistryReader) ([]workspacepkg.Workspace, error) {
	if registry == nil {
		return nil, nil
	}
	workspaces, err := registry.ListWorkspaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("daemon: list workspaces for MCP publication: %w", err)
	}
	slices.SortFunc(workspaces, func(left, right workspacepkg.Workspace) int {
		return strings.Compare(left.ID, right.ID)
	})
	return workspaces, nil
}

func appendProfileConfigMCPDeclarations(
	ctx context.Context,
	desired *toolMCPDesiredResources,
	home compozyconfig.HomePaths,
	workspaces []workspacepkg.Workspace,
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
	appendUserMCPServers(desired, servers)
	return servers
}

func appendUserMCPServers(desired *toolMCPDesiredResources, servers []compozyconfig.MCPServer) {
	for _, server := range servers {
		desired.mcpServers = append(desired.mcpServers, mcpServerPublicationInput{
			sourceKey: "config/global/" + strings.TrimSpace(server.Name),
			scope:     resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
			spec:      cloneDaemonMCPServer(server),
		})
	}
}

// configMCPUserDeclarations retains the active user projection while Settings' desired file is pending.
// Editing that layer externally releases the pin; edits in other scopes do not apply a rejected user file.
type configMCPUserDeclarations struct {
	home     compozyconfig.HomePaths
	baseline []compozyconfig.MCPServer
	active   *compozyconfig.Config
	observed map[string]filesnap.Snapshot
}

func (u *configMCPUserDeclarations) append(
	ctx context.Context,
	desired *toolMCPDesiredResources,
	override *compozyconfig.Config,
	logger *slog.Logger,
) []compozyconfig.MCPServer {
	snapshot, err := u.snapshot()
	switch {
	case override != nil:
		u.active = &compozyconfig.Config{}
		for _, server := range override.MCPServers {
			u.active.MCPServers = append(u.active.MCPServers, cloneDaemonMCPServer(server))
		}
		u.observed = snapshot
	case u.active != nil && (err != nil || u.observed == nil || filesnap.Equal(u.observed, snapshot)):
		override = u.active
		if err != nil {
			u.observed = nil
		} else if u.observed == nil {
			u.observed = snapshot
		}
	default:
		u.active, u.observed = nil, nil
	}
	if override != nil {
		appendUserMCPServers(desired, override.MCPServers)
		return override.MCPServers
	}
	return appendUserConfigMCPDeclarations(ctx, desired, u.home, u.baseline, logger)
}

func (u *configMCPUserDeclarations) snapshot() (map[string]filesnap.Snapshot, error) {
	snapshot := make(map[string]filesnap.Snapshot)
	for _, path := range []string{u.home.ConfigFile, filepath.Join(u.home.HomeDir, compozyconfig.MCPJSONName)} {
		value, err := filesnap.FromPath(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		snapshot[path] = value
	}
	return snapshot, nil
}
