package core

import (
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
)

func daemonConfigFromPayload(payload contract.SettingsDaemonPayload) (aghconfig.DaemonConfig, error) {
	interval, err := parseSettingsDurationOrDefault(
		payload.MemoryReportInterval,
		aghconfig.DefaultDaemonMemoryReportInterval,
	)
	if err != nil {
		return aghconfig.DaemonConfig{}, NewSettingsValidationError(
			fmt.Errorf("general.config.daemon.memory_report_interval: %w", err),
		)
	}
	reloadTimeouts, err := daemonReloadTimeoutsFromPayload(payload.ReloadTimeouts)
	if err != nil {
		return aghconfig.DaemonConfig{}, err
	}
	return aghconfig.DaemonConfig{
		Socket:               strings.TrimSpace(payload.Socket),
		MemoryReportInterval: interval,
		ReloadTimeouts:       reloadTimeouts,
	}, nil
}

func daemonReloadTimeoutsFromPayload(
	payload contract.SettingsDaemonReloadTimeoutsPayload,
) (aghconfig.DaemonReloadTimeoutsConfig, error) {
	defaults := aghconfig.DefaultDaemonReloadTimeoutsConfig()
	providers, err := parseSettingsDurationOrDefault(payload.Providers, defaults.Providers)
	if err != nil {
		return aghconfig.DaemonReloadTimeoutsConfig{}, NewSettingsValidationError(
			fmt.Errorf("general.config.daemon.reload_timeouts.providers: %w", err),
		)
	}
	mcp, err := parseSettingsDurationOrDefault(payload.MCP, defaults.MCP)
	if err != nil {
		return aghconfig.DaemonReloadTimeoutsConfig{}, NewSettingsValidationError(
			fmt.Errorf("general.config.daemon.reload_timeouts.mcp: %w", err),
		)
	}
	bridges, err := parseSettingsDurationOrDefault(payload.Bridges, defaults.Bridges)
	if err != nil {
		return aghconfig.DaemonReloadTimeoutsConfig{}, NewSettingsValidationError(
			fmt.Errorf("general.config.daemon.reload_timeouts.bridges: %w", err),
		)
	}
	return aghconfig.DaemonReloadTimeoutsConfig{Providers: providers, MCP: mcp, Bridges: bridges}, nil
}
