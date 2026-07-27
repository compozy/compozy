package core

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

func daemonConfigFromPayload(payload contract.SettingsDaemonPayload) (compozyconfig.DaemonConfig, error) {
	interval, err := parseSettingsDurationOrDefault(
		payload.MemoryReportInterval,
		compozyconfig.DefaultDaemonMemoryReportInterval,
	)
	if err != nil {
		return compozyconfig.DaemonConfig{}, NewSettingsValidationError(
			fmt.Errorf("general.config.daemon.memory_report_interval: %w", err),
		)
	}
	reloadTimeouts, err := daemonReloadTimeoutsFromPayload(payload.ReloadTimeouts)
	if err != nil {
		return compozyconfig.DaemonConfig{}, err
	}
	return compozyconfig.DaemonConfig{
		Socket:               strings.TrimSpace(payload.Socket),
		MemoryReportInterval: interval,
		ReloadTimeouts:       reloadTimeouts,
	}, nil
}

func daemonReloadTimeoutsFromPayload(
	payload contract.SettingsDaemonReloadTimeoutsPayload,
) (compozyconfig.DaemonReloadTimeoutsConfig, error) {
	defaults := compozyconfig.DefaultDaemonReloadTimeoutsConfig()
	providers, err := parseSettingsDurationOrDefault(payload.Providers, defaults.Providers)
	if err != nil {
		return compozyconfig.DaemonReloadTimeoutsConfig{}, NewSettingsValidationError(
			fmt.Errorf("general.config.daemon.reload_timeouts.providers: %w", err),
		)
	}
	mcp, err := parseSettingsDurationOrDefault(payload.MCP, defaults.MCP)
	if err != nil {
		return compozyconfig.DaemonReloadTimeoutsConfig{}, NewSettingsValidationError(
			fmt.Errorf("general.config.daemon.reload_timeouts.mcp: %w", err),
		)
	}
	bridges, err := parseSettingsDurationOrDefault(payload.Bridges, defaults.Bridges)
	if err != nil {
		return compozyconfig.DaemonReloadTimeoutsConfig{}, NewSettingsValidationError(
			fmt.Errorf("general.config.daemon.reload_timeouts.bridges: %w", err),
		)
	}
	return compozyconfig.DaemonReloadTimeoutsConfig{Providers: providers, MCP: mcp, Bridges: bridges}, nil
}
