package acp

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	diagcontract "github.com/compozy/compozy/internal/diagnosticcontract"
	"github.com/compozy/compozy/internal/diagnostics"
	authproviders "github.com/compozy/compozy/internal/providers"
)

// ClassifyProbeError maps one ProbeAuthentication failure onto the canonical
// provider-auth taxonomy shared with native-command probes. A nil error is an
// authenticated readiness proof; it never reports account identity.
func ClassifyProbeError(providerName string, err error) authproviders.Classification {
	if err == nil {
		return authproviders.Classification{
			State:   authproviders.ProviderAuthStateAuthenticated,
			Message: "ACP session creation succeeded.",
		}
	}
	if errors.Is(err, exec.ErrNotFound) {
		return authproviders.ClassifyError(err)
	}
	if IsAuthRequiredError(err) {
		return authproviders.Classification{
			State: authproviders.ProviderAuthStateNeedsLogin,
			Code:  diagcontract.CodeProviderNotAuthenticated,
			Message: fmt.Sprintf(
				"Sign in locally with compozy provider auth login %s.",
				strings.TrimSpace(providerName),
			),
			Kind:   authproviders.ProviderFailureNotAuthenticated,
			Action: authproviders.ProviderFailureActionLogin,
		}
	}
	if item, ok := diagnostics.ItemFromError(err); ok && strings.HasPrefix(item.ID, "provider.adapter.") {
		classification := authproviders.Classification{
			State: authproviders.ProviderAuthStateTransient, Code: item.Code, Message: item.Message,
			Kind: authproviders.ProviderFailureUnknown, Action: authproviders.ProviderFailureActionRetry,
		}
		if item.Code == diagcontract.CodeProviderClassificationUnknown {
			classification.State = authproviders.ProviderAuthStateUnknown
			classification.Action = authproviders.ProviderFailureActionInspect
		}
		return classification
	}
	if unavailable, ok := errors.AsType[*AuthMethodUnavailableError](err); ok && unavailable != nil {
		return authproviders.Classification{
			State: authproviders.ProviderAuthStateUnknown,
			Code:  diagcontract.CodeProviderClassificationUnknown,
			Message: fmt.Sprintf(
				"Provider ACP auth method %q is not available as an agent method; inspect the server version and method selection.",
				unavailable.MethodID,
			),
			Kind:   authproviders.ProviderFailureUnknown,
			Action: authproviders.ProviderFailureActionInspect,
		}
	}
	if classification, ok := authproviders.ClassifyErrorText(err.Error()); ok {
		return classification
	}
	return authproviders.Classification{
		State:   authproviders.ProviderAuthStateUnknown,
		Code:    diagcontract.CodeProviderClassificationUnknown,
		Message: "Provider ACP probe failed: " + diagnostics.RedactAndBound(err.Error(), 1024),
		Kind:    authproviders.ProviderFailureUnknown,
		Action:  authproviders.ProviderFailureActionInspect,
	}
}
