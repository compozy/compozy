package config

import "time"

type terminalOverlay struct {
	DefaultShell           *string        `toml:"default_shell"`
	ShellIntegration       *bool          `toml:"shell_integration"`
	ScrollbackBytes        *int           `toml:"scrollback_bytes"`
	DetachedTTL            *time.Duration `toml:"detached_ttl"`
	ExitRetention          *time.Duration `toml:"exit_retention"`
	Recording              *bool          `toml:"recording"`
	RecordingRetentionDays *int           `toml:"recording_retention_days"`
	MaxPerWorkspace        *int           `toml:"max_per_workspace"`
	MaxPerDaemon           *int           `toml:"max_per_daemon"`
	MaxSubscribers         *int           `toml:"max_subscribers"`
}

func (o terminalOverlay) Apply(dst *TerminalConfig) {
	if o.DefaultShell != nil {
		dst.DefaultShell = *o.DefaultShell
	}
	if o.ShellIntegration != nil {
		dst.ShellIntegration = *o.ShellIntegration
	}
	if o.ScrollbackBytes != nil {
		dst.ScrollbackBytes = *o.ScrollbackBytes
	}
	if o.DetachedTTL != nil {
		dst.DetachedTTL = *o.DetachedTTL
	}
	if o.ExitRetention != nil {
		dst.ExitRetention = *o.ExitRetention
	}
	if o.Recording != nil {
		dst.Recording = *o.Recording
	}
	if o.RecordingRetentionDays != nil {
		dst.RecordingRetentionDays = *o.RecordingRetentionDays
	}
	if o.MaxPerWorkspace != nil {
		dst.MaxPerWorkspace = *o.MaxPerWorkspace
	}
	if o.MaxPerDaemon != nil {
		dst.MaxPerDaemon = *o.MaxPerDaemon
	}
	if o.MaxSubscribers != nil {
		dst.MaxSubscribers = *o.MaxSubscribers
	}
}
