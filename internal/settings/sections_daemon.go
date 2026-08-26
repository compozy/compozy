package settings

import (
	"reflect"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

const (
	sectionsDaemonKey         = "daemon"
	sectionsReloadTimeoutsKey = "reload_timeouts"
)

func diffGeneralSettings(cfg *compozyconfig.Config, desired GeneralSettings) []string {
	var changed []string
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
	changed = append(changed, diffTerminalSettings(cfg.Terminal, desired.Terminal)...)
	return changed
}

func diffTerminalSettings(current, desired compozyconfig.TerminalConfig) []string {
	if current == desired {
		return nil
	}
	paths := make([]string, 0, 10)
	currentFields := terminalSettingUpdates(current)
	desiredFields := terminalSettingUpdates(desired)
	for index, field := range desiredFields {
		if !reflect.DeepEqual(currentFields[index].value, field.value) {
			paths = append(paths, field.path[0]+"."+field.path[1])
		}
	}
	return paths
}

func applyGeneralSettings(editor *compozyconfig.OverlayEditor, settings GeneralSettings) error {
	updates := []struct {
		path  []string
		value any
	}{
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
	updates = append(updates, terminalSettingUpdates(settings.Terminal)...)
	return applyValueUpdates(editor, updates)
}

func terminalSettingUpdates(settings compozyconfig.TerminalConfig) []struct {
	path  []string
	value any
} {
	return []struct {
		path  []string
		value any
	}{
		{path: []string{"terminal", "default_shell"}, value: settings.DefaultShell},
		{path: []string{"terminal", "shell_integration"}, value: settings.ShellIntegration},
		{path: []string{"terminal", "scrollback_bytes"}, value: settings.ScrollbackBytes},
		{path: []string{"terminal", "detached_ttl"}, value: settings.DetachedTTL.String()},
		{path: []string{"terminal", "exit_retention"}, value: settings.ExitRetention.String()},
		{path: []string{"terminal", "recording"}, value: settings.Recording},
		{path: []string{"terminal", "recording_retention_days"}, value: settings.RecordingRetentionDays},
		{path: []string{"terminal", "max_per_workspace"}, value: settings.MaxPerWorkspace},
		{path: []string{"terminal", "max_per_daemon"}, value: settings.MaxPerDaemon},
		{path: []string{"terminal", "max_subscribers"}, value: settings.MaxSubscribers},
	}
}

func normalizeDaemonReloadTimeouts(
	value compozyconfig.DaemonReloadTimeoutsConfig,
) compozyconfig.DaemonReloadTimeoutsConfig {
	defaults := compozyconfig.DefaultDaemonReloadTimeoutsConfig()
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
