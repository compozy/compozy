package config

import "time"

type daemonOverlay struct {
	Socket                              *string                     `toml:"socket"`
	MemoryReportInterval                *time.Duration              `toml:"memory_report_interval"`
	SubprocessHealthEscalationThreshold *int                        `toml:"subprocess_health_escalation_threshold"`
	ReloadTimeouts                      daemonReloadTimeoutsOverlay `toml:"reload_timeouts"`
}

type daemonReloadTimeoutsOverlay struct {
	Providers *time.Duration `toml:"providers"`
	MCP       *time.Duration `toml:"mcp"`
	Bridges   *time.Duration `toml:"bridges"`
}

func (o daemonOverlay) Apply(dst *DaemonConfig) {
	if o.Socket != nil {
		dst.Socket = *o.Socket
	}
	if o.MemoryReportInterval != nil {
		dst.MemoryReportInterval = *o.MemoryReportInterval
	}
	if o.SubprocessHealthEscalationThreshold != nil {
		dst.SubprocessHealthEscalationThreshold = *o.SubprocessHealthEscalationThreshold
	}
	o.ReloadTimeouts.Apply(&dst.ReloadTimeouts)
}

func (o daemonReloadTimeoutsOverlay) Apply(dst *DaemonReloadTimeoutsConfig) {
	if o.Providers != nil {
		dst.Providers = *o.Providers
	}
	if o.MCP != nil {
		dst.MCP = *o.MCP
	}
	if o.Bridges != nil {
		dst.Bridges = *o.Bridges
	}
}
