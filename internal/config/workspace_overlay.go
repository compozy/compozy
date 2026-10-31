package config

import (
	"errors"
	"fmt"
	"strings"
)

func applyWorkspaceConfigOverlayFile(path string, dst *Config) error {
	if dst == nil {
		return errors.New("config: destination config is required")
	}
	overlay, err := loadPersistedConfigOverlay(path, loadWorkspaceConfigOverlayBytes)
	if err != nil {
		return err
	}
	if err := applyConfigOverlay(dst, &overlay, RoleFieldSourceWorkspace); err != nil {
		return err
	}
	return nil
}

func loadWorkspaceOverlayForWrite(
	path string,
	target WriteTarget,
	rendered []byte,
) (configOverlay, error) {
	if target.isConfigTarget() && samePath(target.path, path) {
		return loadWorkspaceConfigOverlayBytes(rendered, path)
	}
	return loadPersistedConfigOverlay(path, loadWorkspaceConfigOverlayBytes)
}

func loadWorkspaceConfigOverlayBytes(contents []byte, source string) (configOverlay, error) {
	overlay, err := loadConfigOverlayBytes(contents, source)
	if err != nil {
		if _, ok := errors.AsType[*UnknownKeysError](err); ok {
			return configOverlay{}, fmt.Errorf(
				"%w: %w. Remove the unknown keys, or rename or remove this file "+
					"if it belongs to the legacy CLI. CompozyOS does not support legacy CLI configuration",
				ErrWorkspaceConfigInvalid,
				err,
			)
		}
		return configOverlay{}, err
	}
	if err := validateWorkspaceConfigOverlay(source, &overlay); err != nil {
		return configOverlay{}, err
	}
	return overlay, nil
}

func validateWorkspaceConfigOverlay(path string, overlay *configOverlay) error {
	if overlay == nil {
		return nil
	}
	if overlay.ProviderUsage != nil {
		return fmt.Errorf("workspace config %q: provider_usage settings are global-only", path)
	}
	if overlay.Marketplace != nil {
		return fmt.Errorf("workspace config %q: marketplace catalog settings are global-only", path)
	}
	if overlay.Gateway != nil {
		return fmt.Errorf("workspace config %q: gateway settings are global-only", path)
	}
	if overlay.Shell != nil {
		return fmt.Errorf("workspace config %q: shell settings are global-only", path)
	}
	if overlay.Vault != nil {
		return fmt.Errorf("workspace config %q: vault key settings are global-only", path)
	}
	if overlay.MCP.StatusProbeTimeout != nil || overlay.MCP.Stdio != (mcpStdioOverlay{}) {
		return fmt.Errorf("workspace config %q: mcp.status_probe_timeout and mcp.stdio settings are global-only", path)
	}
	return nil
}

func applyWorkspaceConfigWrite(
	workspaceRoot string,
	target WriteTarget,
	rendered []byte,
	cfg *Config,
) error {
	if strings.TrimSpace(workspaceRoot) == "" {
		return nil
	}
	overlay, err := loadWorkspaceOverlayForWrite(workspaceConfigFile(workspaceRoot), target, rendered)
	if err != nil {
		return err
	}
	if err := applyConfigOverlay(cfg, &overlay, RoleFieldSourceWorkspace); err != nil {
		return fmt.Errorf("apply workspace config overlay: %w", err)
	}
	if err := applyConfigMCPSidecarContent(
		workspaceMCPJSONFile(workspaceRoot),
		target,
		rendered,
		cfg,
	); err != nil {
		return fmt.Errorf("load workspace MCP JSON: %w", err)
	}
	return nil
}
