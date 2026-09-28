package core

import (
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/diagnostics"
)

const (
	automationDiagnosticID = "doctor.scheduler.status"
)

func automationDiagnosticItem(status contract.AutomationHealthPayload) contract.DiagnosticItem {
	if !status.Enabled {
		return diagnostics.NewItem(diagnostics.ItemSpec{
			ID:            automationDiagnosticID,
			Code:          contract.CodeSchedulerPaused,
			Category:      contract.CategoryTask,
			Title:         "Automation scheduler is disabled",
			Message:       "Automation is disabled in runtime config.",
			Severity:      contract.SeverityInfo,
			DataFreshness: contract.FreshnessLive,
		})
	}
	if status.SchedulerRunning {
		return diagnostics.NewItem(diagnostics.ItemSpec{
			ID:            automationDiagnosticID,
			Code:          contract.CodeSchedulerReady,
			Category:      contract.CategoryTask,
			Title:         "Automation scheduler is running",
			Message:       "Scheduled automation is available.",
			Severity:      contract.SeverityOK,
			DataFreshness: contract.FreshnessLive,
		},
			diagnostics.WithEvidence(map[string]any{
				"jobs":     status.Jobs.Total,
				"triggers": status.Triggers.Total,
			}),
		)
	}
	return diagnostics.NewItem(diagnostics.ItemSpec{
		ID:            automationDiagnosticID,
		Code:          contract.CodeSchedulerPaused,
		Category:      contract.CategoryTask,
		Title:         "Automation scheduler is not running",
		Message:       "Automation is enabled but the scheduler is not reporting a running state.",
		Severity:      contract.SeverityWarn,
		DataFreshness: contract.FreshnessLive,
	})
}

func skillDiagnosticItem(status contract.SkillRuntimeStatusPayload) contract.DiagnosticItem {
	if status.RuntimeAvailable {
		return diagnostics.NewItem(diagnostics.ItemSpec{
			ID:            "doctor.skills.status",
			Code:          contract.CodeSkillRegistryReady,
			Category:      contract.CategoryExtension,
			Title:         "Skill registry is available",
			Message:       "Skill registry is loaded and can be queried.",
			Severity:      contract.SeverityOK,
			DataFreshness: contract.FreshnessLive,
		},
			diagnostics.WithEvidence(map[string]any{
				"discovered": status.DiscoveredCount,
				"disabled":   status.DisabledCount,
			}),
		)
	}
	return diagnostics.NewItem(diagnostics.ItemSpec{
		ID:            "doctor.skills.status",
		Code:          contract.CodeSkillNotFound,
		Category:      contract.CategoryExtension,
		Title:         "Skill registry is unavailable",
		Message:       "Skill registry was not configured for this daemon.",
		Severity:      contract.SeverityWarn,
		DataFreshness: contract.FreshnessLive,
	})
}

func logTailDiagnosticItem(status contract.LogTailStatusPayload) contract.DiagnosticItem {
	if status.Available {
		return diagnostics.NewItem(diagnostics.ItemSpec{
			ID:            "doctor.logs.tail",
			Code:          contract.CodeDaemonStatusOK,
			Category:      contract.CategoryDaemon,
			Title:         "Log tail is available",
			Message:       "Runtime log-tail support is available.",
			Severity:      contract.SeverityOK,
			DataFreshness: contract.FreshnessLive,
		},
			diagnostics.WithEvidence(map[string]any{statusKey: status.Status}),
		)
	}
	return diagnostics.NewItem(diagnostics.ItemSpec{
		ID:            "doctor.logs.tail",
		Code:          contract.CodeDaemonStateSuspect,
		Category:      contract.CategoryDaemon,
		Title:         "Log tail is unavailable",
		Message:       "Runtime log-tail support is not currently available.",
		Severity:      contract.SeverityInfo,
		DataFreshness: contract.FreshnessLive,
	},
		diagnostics.WithEvidence(map[string]any{statusKey: status.Status}),
	)
}
