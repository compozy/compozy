package settings

import aghconfig "github.com/compozy/agh/internal/config"

const (
	sectionsDaemonKey         = "daemon"
	sectionsReloadTimeoutsKey = "reload_timeouts"
)

func diffGeneralSettings(cfg *aghconfig.Config, desired GeneralSettings) []string {
	var changed []string
	if cfg.Defaults.Agent != desired.Defaults.Agent {
		changed = append(changed, "defaults.agent")
	}
	if cfg.Defaults.Provider != desired.Defaults.Provider {
		changed = append(changed, "defaults.provider")
	}
	if cfg.Defaults.Sandbox != desired.Defaults.Sandbox {
		changed = append(changed, "defaults.sandbox")
	}
	if cfg.Limits.MaxConcurrentAgents != desired.Limits.MaxConcurrentAgents {
		changed = append(changed, "limits.max_concurrent_agents")
	}
	if cfg.Session.Limits.Timeout != desired.SessionTimeout {
		changed = append(changed, "session.limits.timeout")
	}
	if cfg.Permissions.Mode != desired.Permissions.Mode {
		changed = append(changed, "permissions.mode")
	}
	if cfg.HTTP.Host != desired.HTTP.Host {
		changed = append(changed, "http.host")
	}
	if cfg.HTTP.Port != desired.HTTP.Port {
		changed = append(changed, "http.port")
	}
	if cfg.Daemon.Socket != desired.Daemon.Socket {
		changed = append(changed, "daemon.socket")
	}
	if cfg.Daemon.MemoryReportInterval != desired.Daemon.MemoryReportInterval {
		changed = append(changed, "daemon.memory_report_interval")
	}
	if cfg.Daemon.ReloadTimeouts.Providers != desired.Daemon.ReloadTimeouts.Providers {
		changed = append(changed, "daemon.reload_timeouts.providers")
	}
	if cfg.Daemon.ReloadTimeouts.MCP != desired.Daemon.ReloadTimeouts.MCP {
		changed = append(changed, "daemon.reload_timeouts.mcp")
	}
	if cfg.Daemon.ReloadTimeouts.Bridges != desired.Daemon.ReloadTimeouts.Bridges {
		changed = append(changed, "daemon.reload_timeouts.bridges")
	}
	if cfg.Redact.Enabled != desired.Redact.Enabled {
		changed = append(changed, "redact.enabled")
	}
	return changed
}

func applyGeneralSettings(editor *aghconfig.OverlayEditor, settings GeneralSettings) error {
	updates := []struct {
		path  []string
		value any
	}{
		{path: []string{sectionsDefaultsKey, "agent"}, value: settings.Defaults.Agent},
		{path: []string{sectionsDefaultsKey, sectionsProviderKey}, value: settings.Defaults.Provider},
		{path: []string{sectionsDefaultsKey, "sandbox"}, value: settings.Defaults.Sandbox},
		{path: []string{"limits", "max_concurrent_agents"}, value: settings.Limits.MaxConcurrentAgents},
		{path: []string{sectionsSessionKey, "limits", sectionsTimeoutKey}, value: settings.SessionTimeout.String()},
		{path: []string{"permissions", sectionsModeKey}, value: string(settings.Permissions.Mode)},
		{path: []string{sectionsHTTPKey, "host"}, value: settings.HTTP.Host},
		{path: []string{sectionsHTTPKey, "port"}, value: settings.HTTP.Port},
		{path: []string{sectionsDaemonKey, "socket"}, value: settings.Daemon.Socket},
		{
			path:  []string{sectionsDaemonKey, "memory_report_interval"},
			value: settings.Daemon.MemoryReportInterval.String(),
		},
		{
			path:  []string{sectionsDaemonKey, sectionsReloadTimeoutsKey, string(CollectionProviders)},
			value: settings.Daemon.ReloadTimeouts.Providers.String(),
		},
		{
			path:  []string{sectionsDaemonKey, sectionsReloadTimeoutsKey, "mcp"},
			value: settings.Daemon.ReloadTimeouts.MCP.String(),
		},
		{
			path:  []string{sectionsDaemonKey, sectionsReloadTimeoutsKey, "bridges"},
			value: settings.Daemon.ReloadTimeouts.Bridges.String(),
		},
		{path: []string{"redact", sectionsEnabledKey}, value: settings.Redact.Enabled},
	}
	return applyValueUpdates(editor, updates)
}

func normalizeDaemonReloadTimeouts(
	value aghconfig.DaemonReloadTimeoutsConfig,
) aghconfig.DaemonReloadTimeoutsConfig {
	defaults := aghconfig.DefaultDaemonReloadTimeoutsConfig()
	if value.Providers == 0 {
		value.Providers = defaults.Providers
	}
	if value.MCP == 0 {
		value.MCP = defaults.MCP
	}
	if value.Bridges == 0 {
		value.Bridges = defaults.Bridges
	}
	return value
}
