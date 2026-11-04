package session

const (
	daemonStoppedDetail     = "daemon stopped by user"
	daemonRestartStopDetail = "Interrupted by daemon restart."
)

// daemonShutdownStop is the stop classification the daemon applies while it is shutting down.
type daemonShutdownStop struct {
	cause  StopCause
	detail string
}

// DaemonShutdownStopCause returns the cause and detail the daemon applies to sessions it stops.
// A restart interrupts sessions for resume; any other shutdown is the operator stopping the daemon.
func DaemonShutdownStopCause(restart bool) (StopCause, string) {
	if restart {
		return CauseShutdown, daemonRestartStopDetail
	}
	return CauseUserRequested, daemonStoppedDetail
}

func (m *Manager) markDaemonShutdown(restart bool) {
	cause, detail := DaemonShutdownStopCause(restart)
	m.daemonShutdown.Store(&daemonShutdownStop{cause: cause, detail: detail})
}

// daemonShuttingDown reports that agent runtime loss is the daemon's shutdown, so a prompt ends as interrupted.
func (m *Manager) daemonShuttingDown() bool {
	return m.daemonShutdown.Load() != nil
}

// classifyDaemonShutdownExit reclassifies an unrequested process exit observed during daemon shutdown.
// The OS or desktop host signals the daemon and its agents together, so an agent can exit before the
// daemon stops it; that exit is the shutdown interruption, not a crash. It returns the wait error that
// finalization should use: nil once reclassified, so no failure, crash bundle or agent_crashed follows.
func (m *Manager) classifyDaemonShutdownExit(session *Session, waitErr error) error {
	stop := m.daemonShutdown.Load()
	if stop == nil || !session.replaceStopCause(CauseProcessExited, stop.cause, stop.detail) {
		return waitErr
	}
	logArgs := []any{"stop_cause", stop.cause.String()}
	if waitErr != nil {
		logArgs = append(logArgs, "exit_error", waitErr.Error())
	}
	m.sessionLogger(session).Info("session: agent exit during daemon shutdown classified as interrupted", logArgs...)
	return nil
}
