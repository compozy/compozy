package settings

import (
	"context"
	"fmt"
	"slices"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
)

type mcpSourceEntry struct {
	Source SourceRef
	Target WriteTargetKind
	Server aghconfig.MCPServer
}

func (s *service) resolveMCPTargetContext(
	ctx context.Context,
	scope ScopeKind,
	workspaceID string,
) (string, map[string][]mcpSourceEntry, error) {
	resolved, err := s.resolveWorkspace(ctx, scope, workspaceID)
	if err != nil {
		return "", nil, err
	}
	root := ""
	if resolved != nil {
		root = resolved.RootDir
	}

	sources, err := s.loadMCPSources(workspaceID, root, scope)
	if err != nil {
		return "", nil, err
	}
	return root, sources, nil
}

func (s *service) loadMCPSources(
	workspaceID string,
	workspaceRoot string,
	scope ScopeKind,
) (map[string][]mcpSourceEntry, error) {
	sources := make(map[string][]mcpSourceEntry)

	appendServers := func(kind WriteTargetKind, serverList []aghconfig.MCPServer) {
		for _, server := range serverList {
			name := strings.TrimSpace(server.Name)
			if name == "" {
				continue
			}
			sources[name] = append(sources[name], mcpSourceEntry{
				Source: sourceRefForWriteTarget(kind, workspaceID),
				Target: kind,
				Server: server,
			})
		}
	}

	globalConfigServers, err := loadMCPServersFromConfigFile(s.homePaths.ConfigFile, s.homePaths)
	if err != nil {
		return nil, fmt.Errorf("settings: load global config MCP servers: %w", err)
	}
	appendServers(WriteTargetGlobalConfig, globalConfigServers)

	globalSidecarServers, err := aghconfig.LoadMCPServersJSONFile(globalMCPSidecarPath(s.homePaths))
	if err != nil {
		return nil, fmt.Errorf("settings: load global MCP sidecar: %w", err)
	}
	appendServers(WriteTargetGlobalMCPSidecar, globalSidecarServers)

	if scope == ScopeWorkspace {
		workspaceConfigServers, loadErr := loadMCPServersFromConfigFile(
			workspaceConfigPath(workspaceRoot),
			s.homePaths,
		)
		if loadErr != nil {
			return nil, fmt.Errorf("settings: load workspace config MCP servers: %w", loadErr)
		}
		appendServers(WriteTargetWorkspaceConfig, workspaceConfigServers)

		workspaceSidecarServers, loadErr := aghconfig.LoadMCPServersJSONFile(
			workspaceMCPSidecarPath(workspaceRoot),
		)
		if loadErr != nil {
			return nil, fmt.Errorf("settings: load workspace MCP sidecar: %w", loadErr)
		}
		appendServers(WriteTargetWorkspaceMCPSidecar, workspaceSidecarServers)
	}

	return sources, nil
}

func loadMCPServersFromConfigFile(path string, homePaths aghconfig.HomePaths) ([]aghconfig.MCPServer, error) {
	cfg := aghconfig.DefaultWithHome(homePaths)
	if err := aghconfig.ApplyConfigOverlayFile(path, &cfg); err != nil {
		return nil, err
	}
	return append([]aghconfig.MCPServer(nil), cfg.MCPServers...), nil
}

func (s *service) resolveMCPPutTarget(
	scope ScopeKind,
	workspaceRoot string,
	name string,
	selector TargetSelector,
	sources map[string][]mcpSourceEntry,
) (aghconfig.WriteTarget, error) {
	normalized, err := normalizeTargetSelector(selector)
	if err != nil {
		return aghconfig.WriteTarget{}, err
	}
	if normalized == TargetConfig {
		return aghconfig.ResolveConfigWriteTarget(s.homePaths, workspaceRoot, scope.configWriteScope())
	}
	if normalized == TargetSidecar {
		return aghconfig.ResolveMCPSidecarWriteTarget(s.homePaths, workspaceRoot, scope.configWriteScope())
	}

	targetKind := preferredMCPPutTarget(scope, name, sources)
	switch targetKind {
	case WriteTargetGlobalConfig, WriteTargetWorkspaceConfig:
		return aghconfig.ResolveConfigWriteTarget(s.homePaths, workspaceRoot, scope.configWriteScope())
	case WriteTargetGlobalMCPSidecar, WriteTargetWorkspaceMCPSidecar:
		return aghconfig.ResolveMCPSidecarWriteTarget(s.homePaths, workspaceRoot, scope.configWriteScope())
	default:
		return aghconfig.WriteTarget{}, conflictError(
			fmt.Errorf("settings: unsupported MCP write target %q for %q", targetKind, name),
		)
	}
}

func (s *service) resolveMCPDeleteTarget(
	scope ScopeKind,
	workspaceRoot string,
	name string,
	selector TargetSelector,
	sources map[string][]mcpSourceEntry,
) (aghconfig.WriteTarget, error) {
	normalized, err := normalizeTargetSelector(selector)
	if err != nil {
		return aghconfig.WriteTarget{}, err
	}
	if normalized == TargetConfig {
		return aghconfig.ResolveConfigWriteTarget(s.homePaths, workspaceRoot, scope.configWriteScope())
	}
	if normalized == TargetSidecar {
		return aghconfig.ResolveMCPSidecarWriteTarget(s.homePaths, workspaceRoot, scope.configWriteScope())
	}

	targetKind, ok := preferredMCPDeleteTarget(scope, name, sources)
	if !ok {
		return aghconfig.WriteTarget{}, notFoundError(
			fmt.Errorf("settings: MCP server %q has no definition in %s scope", name, scope),
		)
	}
	switch targetKind {
	case WriteTargetGlobalConfig, WriteTargetWorkspaceConfig:
		return aghconfig.ResolveConfigWriteTarget(s.homePaths, workspaceRoot, scope.configWriteScope())
	case WriteTargetGlobalMCPSidecar, WriteTargetWorkspaceMCPSidecar:
		return aghconfig.ResolveMCPSidecarWriteTarget(s.homePaths, workspaceRoot, scope.configWriteScope())
	default:
		return aghconfig.WriteTarget{}, conflictError(
			fmt.Errorf("settings: unsupported MCP write target %q for %q", targetKind, name),
		)
	}
}

func preferredMCPPutTarget(scope ScopeKind, name string, sources map[string][]mcpSourceEntry) WriteTargetKind {
	if targetKind, ok := preferredMCPDeleteTarget(scope, name, sources); ok {
		return targetKind
	}
	if scope == ScopeWorkspace {
		return WriteTargetWorkspaceMCPSidecar
	}
	return WriteTargetGlobalMCPSidecar
}

func preferredMCPDeleteTarget(
	scope ScopeKind,
	name string,
	sources map[string][]mcpSourceEntry,
) (WriteTargetKind, bool) {
	entries := sources[strings.TrimSpace(name)]
	if len(entries) == 0 {
		return "", false
	}

	switch scope {
	case ScopeWorkspace:
		for _, entry := range slices.Backward(entries) {
			switch entry.Target {
			case WriteTargetWorkspaceMCPSidecar, WriteTargetWorkspaceConfig:
				return entry.Target, true
			}
		}
	default:
		for _, entry := range slices.Backward(entries) {
			switch entry.Target {
			case WriteTargetGlobalMCPSidecar, WriteTargetGlobalConfig:
				return entry.Target, true
			}
		}
	}

	return "", false
}

func normalizeTargetSelector(selector TargetSelector) (TargetSelector, error) {
	if err := selector.Validate(); err != nil {
		return "", err
	}
	return selector.Normalize(), nil
}

func mcpSourceForTarget(
	name string,
	target WriteTargetKind,
	sources map[string][]mcpSourceEntry,
) (mcpSourceEntry, bool) {
	for _, entry := range sources[strings.TrimSpace(name)] {
		if entry.Target == target {
			return entry, true
		}
	}
	return mcpSourceEntry{}, false
}
