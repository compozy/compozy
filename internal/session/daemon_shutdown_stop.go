package session

import (
	"context"
	"errors"
	"time"
)

const (
	daemonStoppedDetail     = "daemon stopped by user"
	daemonRestartStopDetail = "Interrupted by daemon restart."
	// daemonShutdownTurnTerminalGrace is the default bound on how long a shutdown stop keeps a recorder
	// open for the terminal event of the turn it interrupted, once the agent process is gone.
	daemonShutdownTurnTerminalGrace = 5 * time.Second
)

// ErrDaemonShuttingDown reports that the daemon has begun shutting down, so no agent runtime starts.
var ErrDaemonShuttingDown = errors.New("session: daemon is shutting down")

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

// BeginDaemonShutdown closes agent-runtime admission the moment the daemon starts shutting down,
// before it drains: no provider spawn or automatic recovery is admitted from here on, and an agent
// exit is the daemon's interruption rather than a crash. PrepareDaemonShutdown later writes the
// restart checkpoints with the same decision.
func (m *Manager) BeginDaemonShutdown(restart bool) {
	if m == nil {
		return
	}
	m.markDaemonShutdown(restart)
}

// interruptStartForDaemonShutdown settles a start that failed after shutdown began as the shutdown
// stop it is. Whether the spawn was refused or its dependencies were torn down, the failure says
// nothing about the agent, so it must not be retained as a session failure or as failure transcript
// rows; the next boot resumes the session. The caller holds the start's launch commit.
func (m *Manager) interruptStartForDaemonShutdown(
	ctx context.Context,
	accepted *acceptedSessionStart,
	startErr error,
) bool {
	stop := m.daemonShutdown.Load()
	if stop == nil || accepted == nil || accepted.run == nil {
		return false
	}
	session := accepted.session
	writeMeta, _, err := session.prepareStop(m.now(), stop.cause, stop.detail)
	if err != nil {
		m.sessionLogger(session).Warn(
			"session: start failure during daemon shutdown could not settle as interrupted",
			"error", err,
			"start_error", startErr,
		)
		return false
	}
	accepted.run.markStopRequested()
	if writeMeta {
		// As for any stop that lands during startup, the catalog leaves Starting with the stop intent.
		if err := m.persistSessionLifecycleState(ctx, session, true); err != nil {
			m.sessionLogger(session).Warn("session: persist daemon shutdown start interruption failed", "error", err)
		}
	}
	m.sessionLogger(session).Info(
		"session: start interrupted by daemon shutdown",
		"stop_cause", stop.cause.String(),
		"start_error", startErr,
	)
	return true
}

// awaitDaemonShutdownTurnTerminal keeps the recorder open until the turn a shutdown stop interrupted
// has recorded its terminal event. The prompt pump only learns of the exit once the process is gone,
// and finalization closes the recorder, so finalizing first would drop that event. The wait ends
// with the caller's stop budget or the grace, whichever comes first.
func (m *Manager) awaitDaemonShutdownTurnTerminal(ctx context.Context, promptDone <-chan struct{}) {
	if promptDone == nil || !m.daemonShuttingDown() {
		return
	}
	grace := m.daemonShutdownTurnGrace
	if grace <= 0 {
		grace = daemonShutdownTurnTerminalGrace
	}
	timer := time.NewTimer(grace)
	defer timer.Stop()
	select {
	case <-promptDone:
	case <-timer.C:
	case <-ctx.Done():
	}
}

// daemonShutdownCaptureReason is the restart checkpoint reason for a turn interrupted by daemon shutdown.
// A restart leaves it empty so the turn continues after boot; a stop records that the user stopped it.
func daemonShutdownCaptureReason(restart bool) string {
	if restart {
		return ""
	}
	return "daemon_stopped_by_user"
}

// daemonShutdownCaptureAttempts bounds retries of a transient checkpoint write before the turn ends.
const daemonShutdownCaptureAttempts = 3

// captureDaemonShutdownTurn records the restart checkpoint of a turn the pump is about to end as
// canceled because the daemon is shutting down. The canceled terminal would otherwise land first
// and make the turn look finished to restart capture. Capture is idempotent, so racing the
// PrepareDaemonShutdown scan and retrying are safe. A turn the user already canceled is not resumable.
func (m *Manager) captureDaemonShutdownTurn(ctx context.Context, session *Session, turnID string) {
	stop := m.daemonShutdown.Load()
	if stop == nil || session == nil || session.promptCancellationRequested(turnID) {
		return
	}
	captureCtx, cancel := detachedPromptStopContext(ctx, m)
	defer cancel()
	meta := session.Meta()
	reason := daemonShutdownCaptureReason(stop.restart)
	var err error
	for range daemonShutdownCaptureAttempts {
		if err = m.captureRestartInterruption(captureCtx, &meta, reason); err == nil || captureCtx.Err() != nil {
			break
		}
	}
	if session.promptCancellationRequested(turnID) {
		m.skipDaemonShutdownResume(ctx, session, turnID)
	}
	if err != nil {
		m.sessionLogger(session).Warn("session: capture daemon shutdown interruption failed",
			"turn_id", turnID, "attempts", daemonShutdownCaptureAttempts, "error", err)
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

// skipDaemonShutdownResume settles a turn stopped during daemon shutdown. The turn was ended
// deliberately, so restart must not continue it. The skip is written whether or not the shutdown scan
// has captured the interruption yet, because restart continuation rejects any turn with a skip.
func (m *Manager) skipDaemonShutdownResume(ctx context.Context, session *Session, turnID string) {
	if !m.daemonShuttingDown() || session == nil || turnID == "" {
		return
	}
	skipCtx, cancel := detachedPromptStopContext(ctx, m)
	defer cancel()
	var err error
	for range daemonShutdownCaptureAttempts {
		if err = m.appendDaemonShutdownResumeSkip(skipCtx, session, turnID); err == nil || skipCtx.Err() != nil {
			break
		}
	}
	if err != nil {
		m.sessionLogger(session).Warn("session: skip restart continuation of canceled turn failed",
			"turn_id", turnID, "error", err)
	}
}

func (m *Manager) appendDaemonShutdownResumeSkip(ctx context.Context, session *Session, turnID string) error {
	meta := session.Meta()
	checkpoint, found, err := m.latestRestartBoundary(ctx, &meta)
	if err != nil || !found || checkpoint.TurnID != turnID {
		return err
	}
	checkpoint.Reason = "turn_canceled"
	return m.appendRestartEvent(ctx, &meta, restartSkippedEvent, checkpoint, "Turn canceled.")
}
