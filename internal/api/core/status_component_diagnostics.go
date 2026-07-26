package core

import (
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/diagnostics"
)

func automationDiagnosticItem(status contract.AutomationHealthPayload) contract.DiagnosticItem {
	if !status.Enabled {
		return diagnostics.NewItem(
			"doctor.scheduler.status",
			contract.CodeSchedulerPaused,
			contract.CategoryTask,
			"Automation scheduler is disabled",
			"Automation is disabled in runtime config.",
			contract.SeverityInfo,
			contract.FreshnessLive,
		)
	}
	if status.SchedulerRunning {
		return diagnostics.NewItem(
			"doctor.scheduler.status",
			contract.CodeSchedulerReady,
			contract.CategoryTask,
			"Automation scheduler is running",
			"Scheduled automation is available.",
			contract.SeverityOK,
			contract.FreshnessLive,
			diagnostics.WithEvidence(map[string]any{
				"jobs":     status.Jobs.Total,
				"triggers": status.Triggers.Total,
			}),
		)
	}
	return diagnostics.NewItem(
		"doctor.scheduler.status",
		contract.CodeSchedulerPaused,
		contract.CategoryTask,
		"Automation scheduler is not running",
		"Automation is enabled but the scheduler is not reporting a running state.",
		contract.SeverityWarn,
		contract.FreshnessLive,
	)
}

func bridgeDiagnosticItem(status contract.BridgeAggregateHealthPayload) contract.DiagnosticItem {
	if status.StatusCounts.Error > 0 || status.AuthFailuresTotal > 0 {
		return diagnostics.NewItem(
			"doctor.bridge.status",
			contract.CodeBridgeHealthUnavailable,
			contract.CategoryBridge,
			"Bridge health has failures",
			"One or more bridge instances report errors or authentication failures.",
			contract.SeverityError,
			contract.FreshnessLive,
			diagnostics.WithEvidence(bridgeDiagnosticEvidence(status)),
		)
	}
	if status.StatusCounts.Degraded > 0 || status.DeliveryBacklog > 0 || status.DeliveryFailuresTotal > 0 {
		return diagnostics.NewItem(
			"doctor.bridge.status",
			contract.CodeBridgeHealthUnavailable,
			contract.CategoryBridge,
			"Bridge health is degraded",
			"Bridge delivery is not fully healthy.",
			contract.SeverityWarn,
			contract.FreshnessLive,
			diagnostics.WithEvidence(bridgeDiagnosticEvidence(status)),
		)
	}
	return diagnostics.NewItem(
		"doctor.bridge.status",
		contract.CodeBridgeReady,
		contract.CategoryBridge,
		"Bridge health is ready",
		"Bridge registry health has no reported failures.",
		contract.SeverityOK,
		contract.FreshnessLive,
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
		return diagnostics.NewItem(
			"doctor.network.status",
			contract.CodeNetworkDisabled,
			contract.CategoryNetwork,
			"Network is disabled",
			"AGH Network is not enabled in runtime config.",
			contract.SeverityInfo,
			contract.FreshnessLive,
		)
	}
	if strings.EqualFold(strings.TrimSpace(status.Status), memoryHealthStatusUnavailable) {
		return diagnostics.NewItem(
			"doctor.network.status",
			contract.CodeNetworkUnavailable,
			contract.CategoryNetwork,
			"Network status is unavailable",
			"AGH Network is enabled but runtime status could not be collected.",
			contract.SeverityWarn,
			contract.FreshnessLive,
			diagnostics.WithEvidence(evidence),
		)
	}
	return diagnostics.NewItem(
		"doctor.network.status",
		contract.CodeNetworkReady,
		contract.CategoryNetwork,
		"Network status is available",
		"AGH Network status is available from the daemon.",
		contract.SeverityOK,
		contract.FreshnessLive,
		diagnostics.WithEvidence(evidence),
	)
}

func skillDiagnosticItem(status contract.SkillRuntimeStatusPayload) contract.DiagnosticItem {
	if status.RuntimeAvailable {
		return diagnostics.NewItem(
			"doctor.skills.status",
			contract.CodeSkillRegistryReady,
			contract.CategoryExtension,
			"Skill registry is available",
			"Skill registry is loaded and can be queried.",
			contract.SeverityOK,
			contract.FreshnessLive,
			diagnostics.WithEvidence(map[string]any{
				"discovered": status.DiscoveredCount,
				"disabled":   status.DisabledCount,
			}),
		)
	}
	return diagnostics.NewItem(
		"doctor.skills.status",
		contract.CodeSkillNotFound,
		contract.CategoryExtension,
		"Skill registry is unavailable",
		"Skill registry was not configured for this daemon.",
		contract.SeverityWarn,
		contract.FreshnessLive,
	)
}

func logTailDiagnosticItem(status contract.LogTailStatusPayload) contract.DiagnosticItem {
	if status.Available {
		return diagnostics.NewItem(
			"doctor.logs.tail",
			contract.CodeDaemonStatusOK,
			contract.CategoryDaemon,
			"Log tail is available",
			"Runtime log-tail support is available.",
			contract.SeverityOK,
			contract.FreshnessLive,
		)
	}
	return diagnostics.NewItem(
		"doctor.logs.tail",
		contract.CodeDaemonStateSuspect,
		contract.CategoryDaemon,
		"Log tail is unavailable",
		"Runtime log-tail support is not currently available.",
		contract.SeverityInfo,
		contract.FreshnessLive,
		diagnostics.WithEvidence(map[string]any{statusKey: status.Status}),
	)
}
