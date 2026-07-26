package providers

import (
	"fmt"
	"strings"

	diagcontract "github.com/compozy/agh/internal/diagnosticcontract"
	"github.com/compozy/agh/internal/diagnostics"
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
	return diagnostics.NewItem(
		id,
		code,
		diagcontract.CategoryProvider,
		title,
		message,
		severity,
		diagcontract.FreshnessLive,
		diagnostics.WithSuggestedCommand(SuggestedCommand(providerName, classification)),
		diagnostics.WithEvidence(map[string]any{
			"provider": strings.TrimSpace(providerName),
			"state":    string(classification.State),
			"action":   string(classification.Action),
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

// SuggestedCommand returns the canonical operator command for a provider-auth classification.
func SuggestedCommand(providerName string, classification Classification) string {
	name := strings.TrimSpace(providerName)
	if name == "" {
		return "agh provider auth status"
	}
	switch actionForClassification(classification) {
	case ProviderFailureActionLogin:
		return "agh provider auth login " + name
	case ProviderFailureActionBindSecret,
		ProviderFailureActionInstallCLI,
		ProviderFailureActionInspect,
		ProviderFailureActionNoRetry:
		return "agh provider auth status " + name
	case ProviderFailureActionRetry:
		return "agh provider auth status " + name + " --remote"
	default:
		return ""
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
