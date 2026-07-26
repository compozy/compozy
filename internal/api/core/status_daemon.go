package core

import (
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/diagnostics"
)

func daemonDiagnosticItem(status *contract.StatusPayload) contract.DiagnosticItem {
	switch strings.TrimSpace(status.Daemon.Status) {
	case statusStateRunning:
		return diagnostics.NewItem(
			"doctor.daemon.status",
			contract.CodeDaemonStatusOK,
			contract.CategoryDaemon,
			"Daemon is running",
			"AGH daemon process and status transport are responding.",
			contract.SeverityOK,
			contract.FreshnessLive,
			diagnostics.WithEvidence(daemonDiagnosticEvidence(status)),
		)
	case string(contract.DrainStateDraining):
		return diagnostics.NewItem(
			"doctor.daemon.status",
			contract.CodeDaemonDraining,
			contract.CategoryDaemon,
			"Daemon is draining",
			"AGH is refusing new work while admitted work finishes.",
			contract.SeverityInfo,
			contract.FreshnessLive,
			diagnostics.WithEvidence(daemonDiagnosticEvidence(status)),
		)
	default:
		return diagnostics.NewItem(
			"doctor.daemon.status",
			contract.CodeDaemonStateSuspect,
			contract.CategoryDaemon,
			"Daemon state is suspect",
			"AGH daemon returned an unknown status.",
			contract.SeverityWarn,
			contract.FreshnessLive,
			diagnostics.WithEvidence(daemonDiagnosticEvidence(status)),
		)
	}
}

func daemonDiagnosticEvidence(status *contract.StatusPayload) map[string]any {
	return map[string]any{
		statusKey:         status.Daemon.Status,
		"pid":             status.Daemon.PID,
		"active_sessions": status.Daemon.ActiveSessions,
		"total_sessions":  status.Daemon.TotalSessions,
	}
}
