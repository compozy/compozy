package monitor

import (
	"log/slog"
	"strings"
)

// logLifecycle deliberately excludes descriptions, patterns, output, and raw errors.
func (s *Service) logLifecycle(event string, mon Monitor, reason string) {
	switch reason {
	case "registered", "requested", "session_stopped", "daemon_shutdown", "admission_retry",
		ReasonExit, ReasonMatch, ReasonTimeout, ReasonStalled, ReasonTimerElapsed, ReasonTerminalFailed:
	default:
		// Owner restart-eligibility reasons come from a fixed vocabulary, never from user input.
		if !strings.HasPrefix(reason, ReasonOwnerPrefix) {
			reason = "unknown"
		}
	}
	s.logger.Log(s.ctx, slog.LevelInfo, event,
		"event", event,
		"monitor_id", mon.ID,
		"session_id", mon.SessionID,
		"kind", mon.Kind,
		"status", mon.Status,
		"reason", reason,
	)
}
