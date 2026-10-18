package config

import (
	"fmt"
	"math"
	"time"
)

const (
	// DefaultDaemonLoadAgentNiceness is the background CPU niceness applied to
	// agent process groups. Ten matches nice(1)'s default: under contention a
	// niced tree gets roughly a tenth of a normal-priority tree's CPU on Linux
	// CFS (weight 110 vs 1024), so tests, linters, and builds an agent launches
	// stop starving the desktop UI, while an idle host still runs them at full speed.
	DefaultDaemonLoadAgentNiceness = 10
	// DefaultDaemonLoadAdmissionThreshold is the normalized 1-minute load
	// (load average / logical CPUs) above which background work waits.
	DefaultDaemonLoadAdmissionThreshold = 1.5
	// DefaultDaemonLoadAdmissionMaxWait bounds one background admission wait.
	DefaultDaemonLoadAdmissionMaxWait = 2 * time.Minute

	maxDaemonLoadAgentNiceness     = 19
	maxDaemonLoadAdmissionMaxWait  = 30 * time.Minute
	daemonLoadAgentNicenessPath    = "daemon.load.agent_niceness"
	daemonLoadAdmissionThreshold   = "daemon.load.admission_threshold"
	daemonLoadAdmissionMaxWaitPath = "daemon.load.admission_max_wait"
)

// DaemonLoadConfig controls how agent work yields to the rest of the host.
type DaemonLoadConfig struct {
	// AgentNiceness lowers the CPU priority of every agent process tree (Unix
	// only); 0 keeps the daemon's priority.
	AgentNiceness int `toml:"agent_niceness"`
	// AdmissionThreshold delays new background (automation, Loop, scheduler)
	// sessions and prompts while normalized load exceeds it; 0 disables sampling.
	AdmissionThreshold float64 `toml:"admission_threshold"`
	// AdmissionMaxWait bounds one delayed background admission before it proceeds.
	AdmissionMaxWait time.Duration `toml:"admission_max_wait"`
}

// DefaultDaemonLoadConfig returns the built-in load-management defaults.
func DefaultDaemonLoadConfig() DaemonLoadConfig {
	return DaemonLoadConfig{
		AgentNiceness:      DefaultDaemonLoadAgentNiceness,
		AdmissionThreshold: DefaultDaemonLoadAdmissionThreshold,
		AdmissionMaxWait:   DefaultDaemonLoadAdmissionMaxWait,
	}
}

// Validate ensures load-management values stay within supported ranges.
func (c DaemonLoadConfig) Validate() error {
	if c.AgentNiceness < 0 || c.AgentNiceness > maxDaemonLoadAgentNiceness {
		return fmt.Errorf("%s must be between 0 and %d: %d",
			daemonLoadAgentNicenessPath, maxDaemonLoadAgentNiceness, c.AgentNiceness)
	}
	if c.AdmissionThreshold < 0 || math.IsNaN(c.AdmissionThreshold) || math.IsInf(c.AdmissionThreshold, 0) {
		return fmt.Errorf("%s must be zero or a finite positive number: %v",
			daemonLoadAdmissionThreshold, c.AdmissionThreshold)
	}
	if c.AdmissionThreshold == 0 {
		if c.AdmissionMaxWait < 0 {
			return fmt.Errorf("%s must be zero or positive: %s", daemonLoadAdmissionMaxWaitPath, c.AdmissionMaxWait)
		}
		return nil
	}
	if c.AdmissionMaxWait < time.Second || c.AdmissionMaxWait > maxDaemonLoadAdmissionMaxWait {
		return fmt.Errorf("%s must be between %s and %s when %s is enabled: %s",
			daemonLoadAdmissionMaxWaitPath, time.Second, maxDaemonLoadAdmissionMaxWait,
			daemonLoadAdmissionThreshold, c.AdmissionMaxWait)
	}
	return nil
}
