package daemon

type shutdownTargets struct {
	daemonRuntimeState
	closeLogger func() error
	infoPath    string
	retention   observerRetentionStopper
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
	return targets
}

func (d *Daemon) resetRuntimeStateLocked() {
	d.daemonRuntimeState = daemonRuntimeState{}
	d.booting = false
	d.closeLogger = func() error { return nil }
}
