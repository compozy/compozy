package config

import (
	"fmt"
	"slices"
	"strings"
)

// DeclaredMCPServers resolves the servers authored in one config layer over its
// inherited user servers. Inherited names alone never change credential owners.
// Profile scope with a workspace root selects the workspace-profile layer.
func DeclaredMCPServers(
	home HomePaths,
	root string,
	scope WriteScope,
	profile string,
	user []MCPServer,
) ([]MCPServer, error) {
	cfg := DefaultWithHome(home)
	cfg.MCPServers = cloneMCPServers(user)
	if scope == WriteScopeUser {
		if err := ApplyConfigOverlayFile(home.ConfigFile, &cfg); err != nil {
			return nil, err
		}
		if err := applyConfigMCPSidecarFile(globalMCPJSONFile(home), &cfg); err != nil {
			return nil, err
		}
		return cfg.MCPServers, nil
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if root != "" && !hasDistinctWorkspaceOverlay(home, root) {
		return nil, nil
	}
	if scope == WriteScopeProfile {
		if err := ValidateResourceProfileName(profile); err != nil {
			return nil, err
		}
		if root != "" {
			if err := applyMCPDeclarationParents(home, root, profile, &cfg); err != nil {
				return nil, err
			}
		}
	}
	declared := DefaultWithHome(home)
	if err := applyMCPDeclarationLayer(home, root, scope, profile, &declared); err != nil {
		return nil, fmt.Errorf("config: load MCP declaring layer: %w", err)
	}
	if err := applyMCPDeclarationLayer(home, root, scope, profile, &cfg); err != nil {
		return nil, fmt.Errorf("config: resolve MCP declaring layer: %w", err)
	}
	return slices.DeleteFunc(cfg.MCPServers, func(server MCPServer) bool {
		return !slices.ContainsFunc(declared.MCPServers, func(authored MCPServer) bool {
			return strings.TrimSpace(authored.Name) == strings.TrimSpace(server.Name)
		})
	}), nil
}

func applyMCPDeclarationParents(home HomePaths, root, profile string, cfg *Config) error {
	if err := applyProfileConfigOverlayFile(profileConfigFile(home, profile), cfg, RoleFieldSourceProfile); err != nil {
		return err
	}
	if err := applyConfigMCPSidecarFile(profileMCPJSONFile(home, profile), cfg); err != nil {
		return err
	}
	if err := applyWorkspaceConfigOverlayFile(workspaceConfigFile(root), cfg); err != nil {
		return err
	}
	return applyConfigMCPSidecarFile(workspaceMCPJSONFile(root), cfg)
}

func applyMCPDeclarationLayer(home HomePaths, root string, scope WriteScope, profile string, cfg *Config) error {
	if scope == WriteScopeWorkspace {
		if err := applyWorkspaceConfigOverlayFile(workspaceConfigFile(root), cfg); err != nil {
			return err
		}
		return applyConfigMCPSidecarFile(workspaceMCPJSONFile(root), cfg)
	}
	configPath := profileConfigFile(home, profile)
	mcpPath := profileMCPJSONFile(home, profile)
	source := RoleFieldSourceProfile
	if root != "" {
		configPath = workspaceProfileConfigFile(root, profile)
		mcpPath = workspaceProfileMCPJSONFile(root, profile)
		source = RoleFieldSourceWorkspaceProfile
	}
	if err := applyProfileConfigOverlayFile(configPath, cfg, source); err != nil {
		return err
	}
	return applyConfigMCPSidecarFile(mcpPath, cfg)
}
