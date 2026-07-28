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
	overlay, err := loadConfigOverlayFile(path)
	if err != nil {
		return err
	}
	if err := validateWorkspaceConfigOverlay(path, &overlay); err != nil {
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
	overlay, err := loadConfigOverlayForWrite(path, target, rendered)
	if err != nil {
		return configOverlay{}, err
	}
	if err := validateWorkspaceConfigOverlay(path, &overlay); err != nil {
		return configOverlay{}, err
	}
	return overlay, nil
}

func validateWorkspaceConfigOverlay(path string, overlay *configOverlay) error {
	if overlay == nil {
		return nil
	}
	if overlay.Marketplace != nil {
		return fmt.Errorf("workspace config %q: marketplace catalog settings are global-only", path)
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
