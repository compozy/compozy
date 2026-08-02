package core

import (
	"fmt"

	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/diagnostics"
)

const taskHealthDiagnosticID = "doctor.tasks.health"

func taskDiagnosticItem(status contract.TaskHealthPayload) contract.DiagnosticItem {
	if len(status.StuckRuns) > 0 {
		return diagnostics.NewItem(diagnostics.ItemSpec{
			ID:            taskHealthDiagnosticID,
			Code:          contract.CodeTaskRunStuck,
			Category:      contract.CategoryTask,
			Title:         "Task runs are stuck",
			Message:       "One or more task runs exceeded the configured health threshold.",
			Severity:      contract.SeverityWarn,
			DataFreshness: contract.FreshnessLive,
		},
			diagnostics.WithEvidence(map[string]any{"stuck_runs": len(status.StuckRuns)}),
		)
	}
	if status.ActiveOrphanRuns > 0 {
		return diagnostics.NewItem(diagnostics.ItemSpec{
			ID:            taskHealthDiagnosticID,
			Code:          contract.CodeTaskRunOrphan,
			Category:      contract.CategoryTask,
			Title:         "Task runs are orphaned",
			Message:       "One or more active task runs no longer have a valid owner.",
			Severity:      contract.SeverityWarn,
			DataFreshness: contract.FreshnessLive,
		},
			diagnostics.WithEvidence(map[string]any{"active_orphan_runs": status.ActiveOrphanRuns}),
		)
	}
	return diagnostics.NewItem(diagnostics.ItemSpec{
		ID:            taskHealthDiagnosticID,
		Code:          contract.CodeSchedulerReady,
		Category:      contract.CategoryTask,
		Title:         "Task health is ready",
		Message:       "Task queue health has no stuck or orphaned active runs.",
		Severity:      contract.SeverityOK,
		DataFreshness: contract.FreshnessLive,
	},
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
	return diagnostics.NewItem(diagnostics.ItemSpec{
		ID:            "doctor.provider." + status.Name,
		Code:          code,
		Category:      contract.CategoryProvider,
		Title:         title,
		Message:       message,
		Severity:      severity,
		DataFreshness: contract.FreshnessLive,
	},
		diagnostics.WithEvidence(map[string]any{
			"provider":           status.Name,
			"state":              status.State,
			"mode":               status.Mode,
			"recommended_action": status.Login.RecommendedAction,
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
