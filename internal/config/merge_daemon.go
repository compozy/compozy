package config

import "time"

type daemonOverlay struct {
	ShutdownGrace                       *time.Duration              `toml:"shutdown_grace"`
	Socket                              *string                     `toml:"socket"`
	MemoryReportInterval                *time.Duration              `toml:"memory_report_interval"`
	SubprocessHealthEscalationThreshold *int                        `toml:"subprocess_health_escalation_threshold"`
	ReloadTimeouts                      daemonReloadTimeoutsOverlay `toml:"reload_timeouts"`
	Load                                daemonLoadOverlay           `toml:"load"`
}

type daemonLoadOverlay struct {
	AgentNiceness      *int           `toml:"agent_niceness"`
	AdmissionThreshold *float64       `toml:"admission_threshold"`
	AdmissionMaxWait   *time.Duration `toml:"admission_max_wait"`
}

type daemonReloadTimeoutsOverlay struct {
	Providers *time.Duration `toml:"providers"`
	MCP       *time.Duration `toml:"mcp"`
}

func (o daemonOverlay) Apply(dst *DaemonConfig) {
	if o.ShutdownGrace != nil {
		dst.ShutdownGrace = *o.ShutdownGrace
	}
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
	o.Load.Apply(&dst.Load)
}

func (o daemonLoadOverlay) Apply(dst *DaemonLoadConfig) {
	if o.AgentNiceness != nil {
		dst.AgentNiceness = *o.AgentNiceness
	}
	if o.AdmissionThreshold != nil {
		dst.AdmissionThreshold = *o.AdmissionThreshold
	}
	if o.AdmissionMaxWait != nil {
		dst.AdmissionMaxWait = *o.AdmissionMaxWait
	}
}

func (o daemonReloadTimeoutsOverlay) Apply(dst *DaemonReloadTimeoutsConfig) {
	if o.Providers != nil {
		dst.Providers = *o.Providers
	}
	if o.MCP != nil {
		dst.MCP = *o.MCP
	}
}
