package core

import (
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/diagnostics"
)

func daemonDiagnosticItem(status *contract.StatusPayload) contract.DiagnosticItem {
	switch strings.TrimSpace(status.Daemon.Status) {
	case statusStateRunning:
		return diagnostics.NewItem(
			"doctor.daemon.status",
			contract.CodeDaemonStatusOK,
			contract.CategoryDaemon,
			"Daemon is running",
			"Compozy daemon process and status transport are responding.",
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
			"Compozy is refusing new work while admitted work finishes.",
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
			"Compozy daemon returned an unknown status.",
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
