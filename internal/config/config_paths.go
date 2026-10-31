package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

func resolveWorkspaceRoot(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", nil
	}

	return resolveAbsoluteDir(root)
}

func applyConfigMCPSidecarFile(path string, cfg *Config) error {
	if cfg == nil {
		return errors.New("config is required")
	}

	servers, err := LoadMCPServersJSONFile(path)
	if err != nil {
		return err
	}
	if len(servers) == 0 {
		return nil
	}

	cfg.MCPServers = OverrideMCPServers(cfg.MCPServers, servers)
	return nil
}

func globalMCPJSONFile(homePaths HomePaths) string {
	return filepath.Join(homePaths.HomeDir, MCPJSONName)
}

func workspaceMCPJSONFile(root string) string {
	trimmed := strings.TrimSpace(root)
	if trimmed == "" {
		return ""
	}

	return filepath.Join(trimmed, DirName, MCPJSONName)
}

func workspaceConfigFile(root string) string {
	return filepath.Join(root, DirName, ConfigName)
}

func profileConfigFile(homePaths HomePaths, profileName string) string {
	return filepath.Join(homePaths.ProfilesDir, strings.TrimSpace(profileName), ConfigName)
}

func profileMCPJSONFile(homePaths HomePaths, profileName string) string {
	return filepath.Join(homePaths.ProfilesDir, strings.TrimSpace(profileName), MCPJSONName)
}

func workspaceProfileConfigFile(root string, profileName string) string {
	return filepath.Join(root, DirName, ProfilesDirName, strings.TrimSpace(profileName), ConfigName)
}

func workspaceProfileMCPJSONFile(root string, profileName string) string {
	return filepath.Join(root, DirName, ProfilesDirName, strings.TrimSpace(profileName), MCPJSONName)
}

func hasDistinctWorkspaceOverlay(homePaths HomePaths, root string) bool {
	if strings.TrimSpace(root) == "" {
		return false
	}

	globalConfig := filepath.Clean(homePaths.ConfigFile)
	workspaceConfig := filepath.Clean(workspaceConfigFile(root))
	return !samePath(globalConfig, workspaceConfig)
}

// WorkspaceDeclaredMCPServerNames returns the MCP server names that one workspace declares in
// its own `.compozy/config.toml` and `.compozy/mcp.json` layers. Definitions the workspace only
// inherits from the user or profile layers are excluded, so callers can keep the declaring
// scope as the OAuth credential owner.
func WorkspaceDeclaredMCPServerNames(homePaths HomePaths, workspaceRoot string) (map[string]struct{}, error) {
	names := make(map[string]struct{})
	if !hasDistinctWorkspaceOverlay(homePaths, workspaceRoot) {
		return names, nil
	}
	cfg := DefaultWithHome(homePaths)
	if err := applyWorkspaceConfigOverlayFile(workspaceConfigFile(workspaceRoot), &cfg); err != nil {
		return nil, fmt.Errorf("load workspace config: %w", err)
	}
	if err := applyConfigMCPSidecarFile(workspaceMCPJSONFile(workspaceRoot), &cfg); err != nil {
		return nil, fmt.Errorf("load workspace MCP JSON: %w", err)
	}
	for _, server := range cfg.MCPServers {
		if name := strings.TrimSpace(server.Name); name != "" {
			names[name] = struct{}{}
		}
	}
	return names, nil
}
