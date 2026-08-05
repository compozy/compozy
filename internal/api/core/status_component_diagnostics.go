package core

import (
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/diagnostics"
)

const (
	automationDiagnosticID = "doctor.scheduler.status"
	bridgeDiagnosticID     = "doctor.bridge.status"
	networkDiagnosticID    = "doctor.network.status"
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

func bridgeDiagnosticItem(status contract.BridgeAggregateHealthPayload) contract.DiagnosticItem {
	if status.StatusCounts.Error > 0 || status.AuthFailuresTotal > 0 {
		return diagnostics.NewItem(diagnostics.ItemSpec{
			ID:            bridgeDiagnosticID,
			Code:          contract.CodeBridgeHealthUnavailable,
			Category:      contract.CategoryBridge,
			Title:         "Bridge health has failures",
			Message:       "One or more bridge instances report errors or authentication failures.",
			Severity:      contract.SeverityError,
			DataFreshness: contract.FreshnessLive,
		},
			diagnostics.WithEvidence(bridgeDiagnosticEvidence(status)),
		)
	}
	if status.StatusCounts.Degraded > 0 || status.DeliveryBacklog > 0 || status.DeliveryFailuresTotal > 0 {
		return diagnostics.NewItem(diagnostics.ItemSpec{
			ID:            bridgeDiagnosticID,
			Code:          contract.CodeBridgeHealthUnavailable,
			Category:      contract.CategoryBridge,
			Title:         "Bridge health is degraded",
			Message:       "Bridge delivery is not fully healthy.",
			Severity:      contract.SeverityWarn,
			DataFreshness: contract.FreshnessLive,
		},
			diagnostics.WithEvidence(bridgeDiagnosticEvidence(status)),
		)
	}
	return diagnostics.NewItem(diagnostics.ItemSpec{
		ID:            bridgeDiagnosticID,
		Code:          contract.CodeBridgeReady,
		Category:      contract.CategoryBridge,
		Title:         "Bridge health is ready",
		Message:       "Bridge registry health has no reported failures.",
		Severity:      contract.SeverityOK,
		DataFreshness: contract.FreshnessLive,
	},
		diagnostics.WithEvidence(bridgeDiagnosticEvidence(status)),
	)
}

func bridgeDiagnosticEvidence(status contract.BridgeAggregateHealthPayload) map[string]any {
	return map[string]any{
		"instances":       status.TotalInstances,
		"routes":          status.RouteCount,
		"backlog":         status.DeliveryBacklog,
		"delivery_errors": status.DeliveryFailuresTotal,
		"auth_errors":     status.AuthFailuresTotal,
	}
}

func networkDiagnosticItem(status *contract.NetworkStatusPayload) contract.DiagnosticItem {
	evidence := map[string]any{}
	if status != nil {
		evidence = map[string]any{
			statusKey:  status.Status,
			"channels": status.Channels,
			"peers":    status.LocalPeers,
		}
	}
	if status == nil || !status.Enabled {
		return diagnostics.NewItem(diagnostics.ItemSpec{
			ID:            networkDiagnosticID,
			Code:          contract.CodeNetworkDisabled,
			Category:      contract.CategoryNetwork,
			Title:         "Network is disabled",
			Message:       "Compozy Network is not enabled in runtime config.",
			Severity:      contract.SeverityInfo,
			DataFreshness: contract.FreshnessLive,
		})
	}
	if strings.EqualFold(strings.TrimSpace(status.Status), memoryHealthStatusUnavailable) {
		return diagnostics.NewItem(diagnostics.ItemSpec{
			ID:            networkDiagnosticID,
			Code:          contract.CodeNetworkUnavailable,
			Category:      contract.CategoryNetwork,
			Title:         "Network status is unavailable",
			Message:       "Compozy Network is enabled but runtime status could not be collected.",
			Severity:      contract.SeverityWarn,
			DataFreshness: contract.FreshnessLive,
		},
			diagnostics.WithEvidence(evidence),
		)
	}
	return diagnostics.NewItem(diagnostics.ItemSpec{
		ID:            networkDiagnosticID,
		Code:          contract.CodeNetworkReady,
		Category:      contract.CategoryNetwork,
		Title:         "Network status is available",
		Message:       "Compozy Network status is available from the daemon.",
		Severity:      contract.SeverityOK,
		DataFreshness: contract.FreshnessLive,
	},
		diagnostics.WithEvidence(evidence),
	)
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
