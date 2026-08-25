package settings

import compozyconfig "github.com/compozy/compozy/internal/config"

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
	checks := []struct {
		changed bool
		path    string
	}{
		{current.DefaultShell != desired.DefaultShell, "terminal.default_shell"},
		{current.ShellIntegration != desired.ShellIntegration, "terminal.shell_integration"},
		{current.ScrollbackBytes != desired.ScrollbackBytes, "terminal.scrollback_bytes"},
		{current.DetachedTTL != desired.DetachedTTL, "terminal.detached_ttl"},
		{current.ExitRetention != desired.ExitRetention, "terminal.exit_retention"},
		{current.Recording != desired.Recording, "terminal.recording"},
		{current.RecordingRetentionDays != desired.RecordingRetentionDays, "terminal.recording_retention_days"},
		{current.MaxPerWorkspace != desired.MaxPerWorkspace, "terminal.max_per_workspace"},
		{current.MaxPerDaemon != desired.MaxPerDaemon, "terminal.max_per_daemon"},
		{current.MaxSubscribers != desired.MaxSubscribers, "terminal.max_subscribers"},
	}
	for _, check := range checks {
		if check.changed {
			paths = append(paths, check.path)
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
		{path: []string{"terminal", "default_shell"}, value: settings.Terminal.DefaultShell},
		{path: []string{"terminal", "shell_integration"}, value: settings.Terminal.ShellIntegration},
		{path: []string{"terminal", "scrollback_bytes"}, value: settings.Terminal.ScrollbackBytes},
		{path: []string{"terminal", "detached_ttl"}, value: settings.Terminal.DetachedTTL.String()},
		{path: []string{"terminal", "exit_retention"}, value: settings.Terminal.ExitRetention.String()},
		{path: []string{"terminal", "recording"}, value: settings.Terminal.Recording},
		{path: []string{"terminal", "recording_retention_days"}, value: settings.Terminal.RecordingRetentionDays},
		{path: []string{"terminal", "max_per_workspace"}, value: settings.Terminal.MaxPerWorkspace},
		{path: []string{"terminal", "max_per_daemon"}, value: settings.Terminal.MaxPerDaemon},
		{path: []string{"terminal", "max_subscribers"}, value: settings.Terminal.MaxSubscribers},
	}
	return applyValueUpdates(editor, updates)
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
