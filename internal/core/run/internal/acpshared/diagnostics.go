package acpshared

import (
	"log/slog"
	"time"

	"github.com/compozy/compozy/internal/core/agent"
)

// stallDiagnosticsJournal exposes the journal drop counters surfaced in the
// per-run progress-signal diagnostics. The concrete implementation is
// *journal.Journal; its methods are nil-safe.
type stallDiagnosticsJournal interface {
	DropsOnSubmit() uint64
	DroppedEventCounts() (uint64, uint64)
}

// logProgressSignalDiagnostics emits the session backpressure/drop counters and
// the journal drop counters as one structured record. Emitted at session
// execution end so every run records whether a stall was caused by our own
// dropped progress signal or by genuine agent silence. Counters are always
// logged, including when zero, so a clean run is observably clean.
func logProgressSignalDiagnostics(
	logger *slog.Logger,
	sessionID string,
	session agent.Session,
	jrnl stallDiagnosticsJournal,
) {
	if logger == nil || session == nil {
		return
	}
	args := []any{
		"session_id", sessionID,
		"dropped_updates", session.DroppedUpdates(),
		"slow_publishes", session.SlowPublishes(),
	}
	if jrnl != nil {
		terminalDrops, nonTerminalDrops := jrnl.DroppedEventCounts()
		args = append(args,
			"journal_drops_on_submit", jrnl.DropsOnSubmit(),
			"journal_terminal_drops", terminalDrops,
			"journal_non_terminal_drops", nonTerminalDrops,
		)
	}
	logger.Info("acp progress-signal diagnostics", args...)
}

// logIdleThresholdDiagnostic emits a structured record when the stall watchdog's
// idle window crosses a warning threshold (50% / 80%), reusing the per-run
// progress-signal diagnostics channel so a stalling run is observable before it
// is actually canceled. Emitted at most once per threshold per attempt.
func logIdleThresholdDiagnostic(
	logger *slog.Logger,
	sessionID string,
	thresholdPct int,
	idle time.Duration,
	idleTimeout time.Duration,
) {
	if logger == nil {
		return
	}
	logger.Info(
		"acp stall watchdog idle threshold crossed",
		"session_id", sessionID,
		"threshold_pct", thresholdPct,
		"idle", idle,
		"idle_timeout", idleTimeout,
	)
}
