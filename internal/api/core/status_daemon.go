package core

import (
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/diagnostics"
)

const daemonDiagnosticID = "doctor.daemon.status"

func daemonDiagnosticItem(status *contract.StatusPayload) contract.DiagnosticItem {
	switch strings.TrimSpace(status.Daemon.Status) {
	case statusStateRunning:
		return diagnostics.NewItem(diagnostics.ItemSpec{
			ID:            daemonDiagnosticID,
			Code:          contract.CodeDaemonStatusOK,
			Category:      contract.CategoryDaemon,
			Title:         "Daemon is running",
			Message:       "Compozy daemon process and status transport are responding.",
			Severity:      contract.SeverityOK,
			DataFreshness: contract.FreshnessLive,
		},
			diagnostics.WithEvidence(daemonDiagnosticEvidence(status)),
		)
	case string(contract.DrainStateDraining):
		return diagnostics.NewItem(diagnostics.ItemSpec{
			ID:            daemonDiagnosticID,
			Code:          contract.CodeDaemonDraining,
			Category:      contract.CategoryDaemon,
			Title:         "Daemon is draining",
			Message:       "Compozy is refusing new work while admitted work finishes.",
			Severity:      contract.SeverityInfo,
			DataFreshness: contract.FreshnessLive,
		},
			diagnostics.WithEvidence(daemonDiagnosticEvidence(status)),
		)
	default:
		return diagnostics.NewItem(diagnostics.ItemSpec{
			ID:            daemonDiagnosticID,
			Code:          contract.CodeDaemonStateSuspect,
			Category:      contract.CategoryDaemon,
			Title:         "Daemon state is suspect",
			Message:       "Compozy daemon returned an unknown status.",
			Severity:      contract.SeverityWarn,
			DataFreshness: contract.FreshnessLive,
		},
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
