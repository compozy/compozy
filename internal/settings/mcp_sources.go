package settings

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

type mcpSourceEntry struct {
	Source SourceRef
	Target WriteTargetKind
	Server compozyconfig.MCPServer
}

func (s *service) resolveMCPTargetContext(
	ctx context.Context,
	scope ScopeKind,
	workspaceID string,
	profileName string,
) (string, map[string][]mcpSourceEntry, error) {
	resolved, err := s.resolveWorkspace(ctx, scope, workspaceID)
	if err != nil {
		return "", nil, err
	}
	root := ""
	if resolved != nil {
		root = resolved.RootDir
	}

	sources, err := s.loadMCPSources(workspaceID, root, scope, profileName)
	if err != nil {
		return "", nil, err
	}
	return root, sources, nil
}

func (s *service) loadMCPSources(
	workspaceID string,
	workspaceRoot string,
	scope ScopeKind,
	profileName string,
) (map[string][]mcpSourceEntry, error) {
	sources := make(map[string][]mcpSourceEntry)
	if err := s.loadGlobalMCPSources(sources, workspaceID, profileName); err != nil {
		return nil, err
	}
	if scope == ScopeProfile {
		if err := s.loadProfileMCPSources(sources, workspaceID, workspaceRoot, profileName); err != nil {
			return nil, err
		}
	}
	if scope == ScopeWorkspace || (scope == ScopeProfile && strings.TrimSpace(workspaceRoot) != "") {
		if err := s.loadWorkspaceMCPSources(sources, workspaceID, workspaceRoot, scope, profileName); err != nil {
			return nil, err
		}
	}
	return sources, nil
}

func (s *service) loadGlobalMCPSources(
	sources map[string][]mcpSourceEntry,
	workspaceID string,
	profileName string,
) error {
	globalConfigServers, err := loadMCPServersFromConfigFile(s.homePaths.ConfigFile, s.homePaths)
	if err != nil {
		return fmt.Errorf("settings: load global config MCP servers: %w", err)
	}
	appendMCPSourceServers(sources, WriteTargetGlobalConfig, workspaceID, profileName, globalConfigServers)

	globalSidecarServers, err := compozyconfig.LoadMCPServersJSONFile(globalMCPSidecarPath(s.homePaths))
	if err != nil {
		return fmt.Errorf("settings: load global MCP sidecar: %w", err)
	}
	appendMCPSourceServers(sources, WriteTargetGlobalMCPSidecar, workspaceID, profileName, globalSidecarServers)
	return nil
}

func (s *service) loadProfileMCPSources(
	sources map[string][]mcpSourceEntry,
	workspaceID string,
	workspaceRoot string,
	profileName string,
) error {
	profileConfigTarget, err := compozyconfig.ResolveConfigWriteTarget(
		s.homePaths, workspaceRoot, compozyconfig.WriteScopeProfile, profileName,
	)
	if err != nil {
		return err
	}
	profileConfigServers, err := loadMCPServersFromConfigFile(profileConfigTarget.Path(), s.homePaths)
	if err != nil {
		return fmt.Errorf("settings: load profile config MCP servers: %w", err)
	}
	appendMCPSourceServers(sources, WriteTargetProfileConfig, workspaceID, profileName, profileConfigServers)

	profileSidecarTarget, err := compozyconfig.ResolveMCPSidecarWriteTarget(
		s.homePaths, workspaceRoot, compozyconfig.WriteScopeProfile, profileName,
	)
	if err != nil {
		return err
	}
	profileSidecarServers, err := compozyconfig.LoadMCPServersJSONFile(profileSidecarTarget.Path())
	if err != nil {
		return fmt.Errorf("settings: load profile MCP sidecar: %w", err)
	}
	appendMCPSourceServers(sources, WriteTargetProfileMCPSidecar, workspaceID, profileName, profileSidecarServers)
	return nil
}

func (s *service) loadWorkspaceMCPSources(
	sources map[string][]mcpSourceEntry,
	workspaceID string,
	workspaceRoot string,
	scope ScopeKind,
	profileName string,
) error {
	workspaceConfigServers, err := loadMCPServersFromConfigFile(workspaceConfigPath(workspaceRoot), s.homePaths)
	if err != nil {
		return fmt.Errorf("settings: load workspace config MCP servers: %w", err)
	}
	appendMCPSourceServers(sources, WriteTargetWorkspaceConfig, workspaceID, profileName, workspaceConfigServers)

	workspaceSidecarServers, err := compozyconfig.LoadMCPServersJSONFile(workspaceMCPSidecarPath(workspaceRoot))
	if err != nil {
		return fmt.Errorf("settings: load workspace MCP sidecar: %w", err)
	}
	appendMCPSourceServers(sources, WriteTargetWorkspaceMCPSidecar, workspaceID, profileName, workspaceSidecarServers)
	if scope != ScopeProfile {
		return nil
	}
	workspaceProfileRoot := filepath.Join(
		workspaceRoot, compozyconfig.DirName, compozyconfig.ProfilesDirName, profileName,
	)
	workspaceProfileConfigServers, err := loadMCPServersFromConfigFile(
		filepath.Join(workspaceProfileRoot, compozyconfig.ConfigName), s.homePaths,
	)
	if err != nil {
		return fmt.Errorf("settings: load workspace profile config MCP servers: %w", err)
	}
	appendMCPSourceServers(
		sources, WriteTargetWorkspaceProfileConfig, workspaceID, profileName, workspaceProfileConfigServers,
	)
	workspaceProfileSidecarServers, err := compozyconfig.LoadMCPServersJSONFile(
		filepath.Join(workspaceProfileRoot, compozyconfig.MCPJSONName),
	)
	if err != nil {
		return fmt.Errorf("settings: load workspace profile MCP sidecar: %w", err)
	}
	appendMCPSourceServers(
		sources, WriteTargetWorkspaceProfileMCPSidecar, workspaceID, profileName, workspaceProfileSidecarServers,
	)
	return nil
}

func appendMCPSourceServers(
	sources map[string][]mcpSourceEntry,
	kind WriteTargetKind,
	workspaceID string,
	profileName string,
	servers []compozyconfig.MCPServer,
) {
	for _, server := range servers {
		name := strings.TrimSpace(server.Name)
		if name == "" {
			continue
		}
		sources[name] = append(sources[name], mcpSourceEntry{
			Source: sourceRefForWriteTarget(kind, workspaceID, profileName), Target: kind, Server: server,
		})
	}
}

func loadMCPServersFromConfigFile(path string, homePaths compozyconfig.HomePaths) ([]compozyconfig.MCPServer, error) {
	cfg := compozyconfig.DefaultWithHome(homePaths)
	if err := compozyconfig.ApplyConfigOverlayFile(path, &cfg); err != nil {
		return nil, err
	}
	return append([]compozyconfig.MCPServer(nil), cfg.MCPServers...), nil
}

func (s *service) resolveMCPPutTarget(
	scope ScopeKind,
	workspaceRoot string,
	profileName string,
	name string,
	selector TargetSelector,
	sources map[string][]mcpSourceEntry,
) (compozyconfig.WriteTarget, error) {
	normalized, err := normalizeTargetSelector(selector)
	if err != nil {
		return compozyconfig.WriteTarget{}, err
	}
	if normalized == TargetConfig {
		return compozyconfig.ResolveConfigWriteTarget(s.homePaths, workspaceRoot, scope.configWriteScope(), profileName)
	}
	if normalized == TargetSidecar {
		return compozyconfig.ResolveMCPSidecarWriteTarget(
			s.homePaths,
			workspaceRoot,
			scope.configWriteScope(),
			profileName,
		)
	}

	targetKind := preferredMCPPutTarget(scope, name, sources)
	switch targetKind {
	case WriteTargetGlobalConfig, WriteTargetProfileConfig, WriteTargetWorkspaceConfig:
		return compozyconfig.ResolveConfigWriteTarget(s.homePaths, workspaceRoot, scope.configWriteScope(), profileName)
	case WriteTargetGlobalMCPSidecar, WriteTargetProfileMCPSidecar, WriteTargetWorkspaceMCPSidecar:
		return compozyconfig.ResolveMCPSidecarWriteTarget(
			s.homePaths,
			workspaceRoot,
			scope.configWriteScope(),
			profileName,
		)
	default:
		return compozyconfig.WriteTarget{}, conflictError(
			fmt.Errorf("settings: unsupported MCP write target %q for %q", targetKind, name),
		)
	}
}

func (s *service) resolveMCPDeleteTarget(
	scope ScopeKind,
	workspaceRoot string,
	profileName string,
	name string,
	selector TargetSelector,
	sources map[string][]mcpSourceEntry,
) (compozyconfig.WriteTarget, error) {
	normalized, err := normalizeTargetSelector(selector)
	if err != nil {
		return compozyconfig.WriteTarget{}, err
	}
	if normalized == TargetConfig {
		return compozyconfig.ResolveConfigWriteTarget(s.homePaths, workspaceRoot, scope.configWriteScope(), profileName)
	}
	if normalized == TargetSidecar {
		return compozyconfig.ResolveMCPSidecarWriteTarget(
			s.homePaths,
			workspaceRoot,
			scope.configWriteScope(),
			profileName,
		)
	}

	targetKind, ok := preferredMCPDeleteTarget(scope, name, sources)
	if !ok {
		return compozyconfig.WriteTarget{}, notFoundError(
			fmt.Errorf("settings: MCP server %q has no definition in %s scope", name, scope),
		)
	}
	switch targetKind {
	case WriteTargetGlobalConfig, WriteTargetProfileConfig, WriteTargetWorkspaceConfig:
		return compozyconfig.ResolveConfigWriteTarget(s.homePaths, workspaceRoot, scope.configWriteScope(), profileName)
	case WriteTargetGlobalMCPSidecar, WriteTargetProfileMCPSidecar, WriteTargetWorkspaceMCPSidecar:
		return compozyconfig.ResolveMCPSidecarWriteTarget(
			s.homePaths,
			workspaceRoot,
			scope.configWriteScope(),
			profileName,
		)
	default:
		return compozyconfig.WriteTarget{}, conflictError(
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
	if scope == ScopeProfile {
		return WriteTargetProfileMCPSidecar
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
	case ScopeProfile:
		for _, entry := range slices.Backward(entries) {
			switch entry.Target {
			case WriteTargetProfileMCPSidecar, WriteTargetProfileConfig:
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
