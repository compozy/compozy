package daemon

import "context"

// stdioMCPCloser is the pooled stdio MCP runtime, which tool calls use until their session stops.
type stdioMCPCloser interface {
	Close(context.Context) error
}

type shutdownTargets struct {
	daemonRuntimeState
	closeLogger func() error
	infoPath    string
	retention   observerRetentionStopper
	stdioMCP    stdioMCPCloser
	// A failed manager shutdown still owns the shared database and singleton.
	sessionShutdownIncomplete bool
}

// snapshotShutdownTargetsLocked captures the current runtime generation.
// The caller must hold d.mu. Targets remain published until teardown succeeds.
func (d *Daemon) snapshotShutdownTargetsLocked() shutdownTargets {
	targets := shutdownTargets{
		daemonRuntimeState: d.daemonRuntimeState,
		closeLogger:        d.closeLogger,
		infoPath:           d.homePaths.DaemonInfo,
	}
	if stopper, ok := d.observer.(observerRetentionStopper); ok {
		targets.retention = stopper
	}
	if d.mcpStdioPool != nil {
		targets.stdioMCP = d.mcpStdioPool
	}
	return targets
}

func (d *Daemon) resetRuntimeStateLocked() {
	d.daemonRuntimeState = daemonRuntimeState{}
	d.booting = false
	d.restartRequested = false
	d.closeLogger = func() error { return nil }
}
