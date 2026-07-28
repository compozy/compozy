package config

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	defaultDaemonProviderReloadTimeout = 5 * time.Second
	defaultDaemonMCPReloadTimeout      = 10 * time.Second
	defaultDaemonBridgeReloadTimeout   = 30 * time.Second

	minDaemonReloadTimeout       = time.Second
	maxDaemonProviderReloadLimit = 60 * time.Second
	maxDaemonMCPReloadLimit      = 60 * time.Second
	maxDaemonBridgeReloadLimit   = 300 * time.Second

	daemonMemoryReportIntervalPath                = "daemon.memory_report_interval"
	daemonSubprocessHealthEscalationThresholdPath = "daemon.subprocess_health_escalation_threshold"
	daemonReloadTimeoutProvidersPath              = "daemon.reload_timeouts.providers"
	daemonReloadTimeoutMCPPath                    = "daemon.reload_timeouts.mcp"
	daemonReloadTimeoutBridgesPath                = "daemon.reload_timeouts.bridges"
)

// DefaultDaemonMemoryReportInterval is the built-in process memory sampling cadence.
const DefaultDaemonMemoryReportInterval = 5 * time.Minute

// DefaultDaemonSubprocessHealthEscalationThreshold is the number of consecutive
// failed health checks that escalates a task-bound session.
const DefaultDaemonSubprocessHealthEscalationThreshold = 3

// DaemonConfig controls daemon-local socket, runtime reporting, and hot-reload settings.
type DaemonConfig struct {
	Socket                              string                     `toml:"socket"`
	MemoryReportInterval                time.Duration              `toml:"memory_report_interval"`
	SubprocessHealthEscalationThreshold int                        `toml:"subprocess_health_escalation_threshold"`
	ReloadTimeouts                      DaemonReloadTimeoutsConfig `toml:"reload_timeouts"`
}

// DaemonReloadTimeoutsConfig bounds subsystem reload attempts during config apply.
type DaemonReloadTimeoutsConfig struct {
	Providers time.Duration `toml:"providers"`
	MCP       time.Duration `toml:"mcp"`
	Bridges   time.Duration `toml:"bridges"`
}

func defaultDaemonConfig(homePaths HomePaths) DaemonConfig {
	return DaemonConfig{
		Socket:                              homePaths.DaemonSocket,
		MemoryReportInterval:                DefaultDaemonMemoryReportInterval,
		SubprocessHealthEscalationThreshold: DefaultDaemonSubprocessHealthEscalationThreshold,
		ReloadTimeouts:                      DefaultDaemonReloadTimeoutsConfig(),
	}
}

// DefaultDaemonReloadTimeoutsConfig returns daemon hot-reload timeout defaults.
func DefaultDaemonReloadTimeoutsConfig() DaemonReloadTimeoutsConfig {
	return DaemonReloadTimeoutsConfig{
		Providers: defaultDaemonProviderReloadTimeout,
		MCP:       defaultDaemonMCPReloadTimeout,
		Bridges:   defaultDaemonBridgeReloadTimeout,
	}
}

// Validate ensures daemon config values are internally consistent.
func (c DaemonConfig) Validate() error {
	if strings.TrimSpace(c.Socket) == "" {
		return errors.New("daemon.socket is required")
	}
	if c.MemoryReportInterval < 0 {
		return fmt.Errorf(
			"%s must be zero or positive: %s",
			daemonMemoryReportIntervalPath,
			c.MemoryReportInterval,
		)
	}
	if c.SubprocessHealthEscalationThreshold < 0 {
		return fmt.Errorf(
			"%s must be zero or positive: %d",
			daemonSubprocessHealthEscalationThresholdPath,
			c.SubprocessHealthEscalationThreshold,
		)
	}
	return c.ReloadTimeouts.Validate()
}

// Validate ensures daemon reload timeout values stay within bounded retry budgets.
func (c DaemonReloadTimeoutsConfig) Validate() error {
	if err := validateDaemonReloadTimeout(
		daemonReloadTimeoutProvidersPath,
		c.Providers,
		maxDaemonProviderReloadLimit,
	); err != nil {
		return err
	}
	if err := validateDaemonReloadTimeout(
		daemonReloadTimeoutMCPPath,
		c.MCP,
		maxDaemonMCPReloadLimit,
	); err != nil {
		return err
	}
	return validateDaemonReloadTimeout(
		daemonReloadTimeoutBridgesPath,
		c.Bridges,
		maxDaemonBridgeReloadLimit,
	)
}

func validateDaemonReloadTimeout(path string, value time.Duration, maxDuration time.Duration) error {
	if value < minDaemonReloadTimeout || value > maxDuration {
		return fmt.Errorf("%s must be between %s and %s: %s", path, minDaemonReloadTimeout, maxDuration, value)
	}
	return nil
}
