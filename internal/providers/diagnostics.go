package providers

import (
	"fmt"
	"strings"

	diagcontract "github.com/compozy/compozy/internal/diagnosticcontract"
	"github.com/compozy/compozy/internal/diagnostics"
)

// DiagnosticItem builds the canonical provider diagnostic for a classifier result.
func DiagnosticItem(providerName string, classification Classification) diagcontract.DiagnosticItem {
	code := strings.TrimSpace(classification.Code)
	if code == "" {
		code = diagcontract.CodeProviderClassificationUnknown
	}
	severity := severityForCode(code)
	message := strings.TrimSpace(classification.Message)
	if message == "" {
		message = "Provider authentication status requires operator attention."
	}
	title := "Provider auth status"
	if strings.TrimSpace(providerName) != "" {
		title = fmt.Sprintf("Provider %q auth status", strings.TrimSpace(providerName))
	}
	id := "provider.auth"
	if strings.TrimSpace(providerName) != "" {
		id = "provider." + strings.TrimSpace(providerName) + ".auth"
	}
	return diagnostics.NewItem(diagnostics.ItemSpec{
		ID:            id,
		Code:          code,
		Category:      diagcontract.CategoryProvider,
		Title:         title,
		Message:       message,
		Severity:      severity,
		DataFreshness: diagcontract.FreshnessLive,
	},
		diagnostics.WithEvidence(map[string]any{
			"provider": strings.TrimSpace(providerName),
			"state":    string(classification.State),
			"action":   string(actionForClassification(classification)),
		}),
	)
}

func severityForCode(code string) string {
	switch code {
	case diagcontract.CodeProviderCLIMissing,
		diagcontract.CodeProviderCredentialUnresolved,
		diagcontract.CodeProviderPermissionDenied,
		diagcontract.CodeProviderNotInstalled:
		return diagcontract.SeverityError
	case diagcontract.CodeProviderTransientFailure:
		return diagcontract.SeverityInfo
	default:
		return diagcontract.SeverityWarn
	}
}

func actionForClassification(classification Classification) ProviderFailureAction {
	if classification.Action != "" {
		return classification.Action
	}
	switch classification.State {
	case ProviderAuthStateNeedsLogin:
		return ProviderFailureActionLogin
	case ProviderAuthStateMissingCredential:
		return ProviderFailureActionBindSecret
	case ProviderAuthStateMissingCLI:
		return ProviderFailureActionInstallCLI
	case ProviderAuthStateRateLimited, ProviderAuthStateTransient:
		return ProviderFailureActionRetry
	case ProviderAuthStatePermissionDenied:
		return ProviderFailureActionNoRetry
	case ProviderAuthStateUnknown:
		return ProviderFailureActionInspect
	default:
		return ProviderFailureActionNone
	}
}
