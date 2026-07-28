package config

import (
	"errors"

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
