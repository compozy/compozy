package session

import "context"

const (
	daemonStoppedDetail     = "daemon stopped by user"
	daemonRestartStopDetail = "Interrupted by daemon restart."
)

// daemonShutdownStop is the stop classification the daemon applies while it is shutting down.
type daemonShutdownStop struct {
	cause   StopCause
	detail  string
	restart bool
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
	m.daemonShutdown.Store(&daemonShutdownStop{cause: cause, detail: detail, restart: restart})
}

// daemonShutdownCaptureReason is the restart checkpoint reason for a turn interrupted by daemon shutdown.
// A restart leaves it empty so the turn continues after boot; a stop records that the user stopped it.
func daemonShutdownCaptureReason(restart bool) string {
	if restart {
		return ""
	}
	return "daemon_stopped_by_user"
}

// captureDaemonShutdownTurn records the restart checkpoint of a turn the pump is about to end as
// canceled because the daemon is shutting down. The canceled terminal would otherwise land first
// and make the turn look finished to restart capture. Capture is idempotent, so racing the
// PrepareDaemonShutdown scan is safe.
func (m *Manager) captureDaemonShutdownTurn(ctx context.Context, session *Session) {
	stop := m.daemonShutdown.Load()
	if stop == nil || session == nil {
		return
	}
	captureCtx, cancel := detachedPromptStopContext(ctx, m)
	defer cancel()
	meta := session.Meta()
	if err := m.captureRestartInterruption(captureCtx, &meta, daemonShutdownCaptureReason(stop.restart)); err != nil {
		m.sessionLogger(session).Warn("session: capture daemon shutdown interruption failed", "error", err)
	}
}

// DaemonShutdownStop returns the shutdown stop classification recorded by PrepareDaemonShutdown, so
// the daemon stops sessions with the same restart decision it captured.
func (m *Manager) DaemonShutdownStop() (StopCause, string, bool) {
	stop := m.daemonShutdown.Load()
	if stop == nil {
		return CauseNone, "", false
	}
	return stop.cause, stop.detail, true
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
	if stop == nil || !session.reclassifyDaemonShutdownExit(stop.cause, stop.detail) {
		return waitErr
	}
	if waitErr != nil {
		m.sessionLogger(session).Info(
			"session: agent exit during daemon shutdown classified as interrupted",
			"stop_cause", stop.cause.String(),
			"exit_error", waitErr.Error(),
		)
	}
	return nil
}
