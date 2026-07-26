package daemon

import (
	"context"

	"fmt"
	"log/slog"
	"slices"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/compozy/agh/internal/resources"

	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func daemonConfigMCPDeclarationProvider(
	cfg *aghconfig.Config,
	registry Registry,
	workspaceResolver workspacepkg.RuntimeResolver,
	logger *slog.Logger,
) toolMCPConfigDeclarationProvider {
	return func(ctx context.Context, override *aghconfig.Config) (toolMCPDesiredResources, error) {
		desired := toolMCPDesiredResources{}
		active := cfg
		if override != nil {
			active = override
		}
		if active == nil {
			return desired, nil
		}
		globalScope := resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal}
		for _, server := range active.MCPServers {
			desired.mcpServers = append(desired.mcpServers, mcpServerPublicationInput{
				sourceKey: "config/global/" + strings.TrimSpace(server.Name),
				scope:     globalScope,
				spec:      cloneDaemonMCPServer(server),
			})
		}

		workspaces, err := registeredWorkspaces(ctx, registry, workspaceResolver, logger)
		if err != nil {
			return toolMCPDesiredResources{}, err
		}
		for idx := range workspaces {
			resolved := &workspaces[idx]
			scope := resources.ResourceScope{
				Kind: resources.ResourceScopeKindWorkspace,
				ID:   strings.TrimSpace(resolved.ID),
			}
			for _, server := range resolved.Config.MCPServers {
				desired.mcpServers = append(desired.mcpServers, mcpServerPublicationInput{
					sourceKey: "config/workspace/" + scope.ID + "/" + strings.TrimSpace(server.Name),
					scope:     scope,
					spec:      cloneDaemonMCPServer(server),
				})
			}
		}

		return desired, nil
	}
}

func extensionManifestToolMCPDeclarationProvider(
	registry *extensionpkg.Registry,
	runtime func() extensionRuntime,
	getenv func(string) string,
	logger *slog.Logger,
) toolMCPDeclarationProvider {
	return func(ctx context.Context) (toolMCPDesiredResources, error) {
		if registry == nil || runtime == nil {
			return toolMCPDesiredResources{}, nil
		}

		manager := runtime()
		if manager == nil {
			return toolMCPDesiredResources{}, nil
		}

		infos, err := registry.List()
		if err != nil {
			return toolMCPDesiredResources{}, fmt.Errorf("daemon: list extensions for tool/mcp sync: %w", err)
		}
		slices.SortFunc(infos, func(left, right extensionpkg.ExtensionInfo) int {
			return strings.Compare(left.Name, right.Name)
		})

		desired := toolMCPDesiredResources{}
		globalScope := resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal}
		for _, info := range infos {
			ext, err := loadExtensionSnapshot(ctx, registry, manager, logger, info.Name)
			if err != nil {
				return toolMCPDesiredResources{}, fmt.Errorf(
					"daemon: load extension %q for tool/mcp sync: %w",
					info.Name,
					err,
				)
			}
			if ext == nil || ext.Manifest == nil {
				continue
			}

			tools, err := extensionpkg.ResolveManifestToolResources(ext.Manifest)
			if err != nil {
				return toolMCPDesiredResources{}, fmt.Errorf(
					"daemon: resolve extension %q tools: %w",
					ext.Info.Name,
					err,
				)
			}
			for _, tool := range tools {
				desired.tools = append(desired.tools, toolPublicationInput{
					sourceKey: "extension/" + ext.Info.Name + "/tool/" + strings.TrimSpace(tool.ID.String()),
					scope:     globalScope,
					spec:      cloneToolSpec(tool),
				})
			}

			if !info.Enabled || !ext.Status.Registered {
				continue
			}
			servers, err := extensionpkg.ResolveManifestMCPServerResources(ext.RootDir, ext.Manifest, getenv)
			if err != nil {
				return toolMCPDesiredResources{}, fmt.Errorf(
					"daemon: resolve extension %q mcp servers: %w",
					ext.Info.Name,
					err,
				)
			}
			for _, server := range servers {
				desired.mcpServers = append(desired.mcpServers, mcpServerPublicationInput{
					sourceKey: "extension/" + ext.Info.Name + "/mcp_server/" + strings.TrimSpace(server.Name),
					scope:     globalScope,
					spec:      cloneDaemonMCPServer(server),
				})
			}
		}

		return desired, nil
	}
}
