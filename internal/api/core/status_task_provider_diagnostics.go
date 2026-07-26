package core

import (
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/diagnostics"
)

func taskDiagnosticItem(status contract.TaskHealthPayload) contract.DiagnosticItem {
	if len(status.StuckRuns) > 0 {
		return diagnostics.NewItem(
			"doctor.tasks.health",
			contract.CodeTaskRunStuck,
			contract.CategoryTask,
			"Task runs are stuck",
			"One or more task runs exceeded the configured health threshold.",
			contract.SeverityWarn,
			contract.FreshnessLive,
			diagnostics.WithEvidence(map[string]any{"stuck_runs": len(status.StuckRuns)}),
		)
	}
	if status.ActiveOrphanRuns > 0 {
		return diagnostics.NewItem(
			"doctor.tasks.health",
			contract.CodeTaskRunOrphan,
			contract.CategoryTask,
			"Task runs are orphaned",
			"One or more active task runs no longer have a valid owner.",
			contract.SeverityWarn,
			contract.FreshnessLive,
			diagnostics.WithEvidence(map[string]any{"active_orphan_runs": status.ActiveOrphanRuns}),
		)
	}
	return diagnostics.NewItem(
		"doctor.tasks.health",
		contract.CodeSchedulerReady,
		contract.CategoryTask,
		"Task health is ready",
		"Task queue health has no stuck or orphaned active runs.",
		contract.SeverityOK,
		contract.FreshnessLive,
		diagnostics.WithEvidence(map[string]any{"queue_depth": status.QueueDepthTotal}),
	)
}

func providerDiagnosticItem(status contract.ProviderStatusPayload) contract.DiagnosticItem {
	severity, code := providerDiagnosticSeverityAndCode(status.State)
	title := "Provider auth is ready"
	if severity != contract.SeverityOK {
		title = "Provider auth needs attention"
	}
	message := status.Message
	if strings.TrimSpace(message) == "" {
		message = fmt.Sprintf("Provider %q auth state is %q.", status.Name, status.State)
	}
	return diagnostics.NewItem(
		"doctor.provider."+status.Name,
		code,
		contract.CategoryProvider,
		title,
		message,
		severity,
		contract.FreshnessLive,
		diagnostics.WithSuggestedCommand(status.SuggestedCommand),
		diagnostics.WithEvidence(map[string]any{
			"provider": status.Name,
			"state":    status.State,
			"mode":     status.Mode,
		}),
	)
}

func providerDiagnosticSeverityAndCode(state string) (string, string) {
	switch strings.TrimSpace(state) {
	case contract.ProviderAuthStateAuthenticated, contract.ProviderAuthStateNone:
		return contract.SeverityOK, contract.CodeProviderAuthenticated
	case contract.ProviderAuthStateNeedsLogin:
		return contract.SeverityWarn, contract.CodeProviderNotAuthenticated
	case contract.ProviderAuthStateMissingCLI:
		return contract.SeverityError, contract.CodeProviderCLIMissing
	case contract.ProviderAuthStateMissingCredential:
		return contract.SeverityError, contract.CodeProviderCredentialUnresolved
	case contract.ProviderAuthStatePermissionDenied:
		return contract.SeverityError, contract.CodeProviderPermissionDenied
	case contract.ProviderAuthStateRateLimited:
		return contract.SeverityWarn, contract.CodeProviderRateLimited
	case contract.ProviderAuthStateTransient:
		return contract.SeverityWarn, contract.CodeProviderTransientFailure
	default:
		return contract.SeverityWarn, contract.CodeProviderClassificationUnknown
	}
}

func diagnosticStatus(items []contract.DiagnosticItem) string {
	result := statusStateOK
	for _, item := range items {
		switch item.Severity {
		case contract.SeverityCritical, contract.SeverityError:
			return statusStateError
		case contract.SeverityWarn:
			result = statusStateWarn
		}
	}
	return result
}

func doctorSummary(items []contract.DiagnosticItem) contract.DoctorSummaryPayload {
	counts := make(map[string]int)
	for _, item := range items {
		counts[item.Severity]++
	}
	return contract.DoctorSummaryPayload{
		Total:            len(items),
		CountsBySeverity: counts,
	}
}
