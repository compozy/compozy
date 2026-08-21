package daemon

import (
	"context"

	"fmt"
	"log/slog"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/resources"

	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func daemonConfigMCPDeclarationProvider(
	cfg *compozyconfig.Config,
	registry workspaceRegistryReader,
	workspaceResolver workspacepkg.RuntimeResolver,
	logger *slog.Logger,
) toolMCPConfigDeclarationProvider {
	return func(ctx context.Context, override *compozyconfig.Config) (toolMCPDesiredResources, error) {
		desired := toolMCPDesiredResources{}
		active := cfg
		if override != nil {
			active = override
		}
		if active == nil {
			return desired, nil
		}
		globalScope := resources.ResourceScope{Kind: resources.ResourceScopeKindUser}
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
		return collectExtensionManifestToolMCPDeclarations(ctx, registry, runtime, getenv, logger)
	}
}

func collectExtensionManifestToolMCPDeclarations(
	ctx context.Context,
	registry *extensionpkg.Registry,
	runtime func() extensionRuntime,
	getenv func(string) string,
	logger *slog.Logger,
) (toolMCPDesiredResources, error) {
	if err := ctx.Err(); err != nil {
		return toolMCPDesiredResources{}, err
	}
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
	globalScope := resources.ResourceScope{Kind: resources.ResourceScopeKindUser}
	for _, info := range infos {
		if !info.Enabled {
			continue
		}
		ext, err := loadExtensionSnapshot(registry, manager, logger, info.Name)
		if err != nil {
			return toolMCPDesiredResources{}, fmt.Errorf(
				"daemon: load extension %q for tool/mcp sync: %w",
				info.Name,
				err,
			)
		}
		if ext == nil || ext.Manifest == nil || !ext.Status.Registered {
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
				owner:     extensionOwner(ext.Info.Name),
				spec:      cloneToolSpec(tool),
			})
		}

		if err := appendExtensionMCPServerDeclarations(&desired, ext, getenv, globalScope); err != nil {
			return toolMCPDesiredResources{}, err
		}
	}

	return desired, nil
}

func appendExtensionMCPServerDeclarations(
	desired *toolMCPDesiredResources,
	ext *extensionpkg.Extension,
	getenv func(string) string,
	scope resources.ResourceScope,
) error {
	servers, err := extensionpkg.ResolveManifestMCPServerResources(ext.RootDir, ext.Manifest, getenv)
	if err != nil {
		return fmt.Errorf("daemon: resolve extension %q mcp servers: %w", ext.Info.Name, err)
	}
	for _, server := range servers {
		desired.mcpServers = append(desired.mcpServers, mcpServerPublicationInput{
			sourceKey: "extension/" + ext.Info.Name + "/mcp_server/" + strings.TrimSpace(server.Name),
			scope:     scope,
			owner:     extensionOwner(ext.Info.Name),
			spec:      cloneDaemonMCPServer(server),
		})
	}
	return nil
}
