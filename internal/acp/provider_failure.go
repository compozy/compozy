package acp

import (
	"errors"
	"fmt"
	execpkg "os/exec"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
	aghconfig "github.com/compozy/agh/internal/config"
	authproviders "github.com/compozy/agh/internal/providers"
)

// ProviderFailureKind classifies provider-facing failures into the recovery
// branches operators and agents need at the CLI/API boundary.
type ProviderFailureKind string

const (
	ProviderFailureMissingCLI       ProviderFailureKind = "missing_cli"
	ProviderFailureUnauthenticated  ProviderFailureKind = "not_authenticated"
	ProviderFailureInvalidModel     ProviderFailureKind = "invalid_model"
	ProviderFailureModelUnavailable ProviderFailureKind = "model_unavailable"
	ProviderFailurePermissionDenied ProviderFailureKind = "permission_denied"
	ProviderFailureRateLimited      ProviderFailureKind = "rate_limited"
	ProviderFailureTransient        ProviderFailureKind = "transient"
)

// ProviderFailureAction is the stable next action paired with a provider
// failure classification.
type ProviderFailureAction string

const (
	ProviderFailureActionInstallCLI        ProviderFailureAction = "install_cli"
	ProviderFailureActionLogin             ProviderFailureAction = "login"
	ProviderFailureActionChangeModel       ProviderFailureAction = "change_model"
	ProviderFailureActionRequestPermission ProviderFailureAction = "request_permission"
	ProviderFailureActionWait              ProviderFailureAction = "wait"
	ProviderFailureActionRetry             ProviderFailureAction = "retry"
	ProviderFailureActionNoRetry           ProviderFailureAction = "no_retry"
)

// ProviderFailureDiagnostic is the typed provider-specific recovery metadata
// embedded into redacted session failure summaries.
type ProviderFailureDiagnostic struct {
	Kind     ProviderFailureKind
	Action   ProviderFailureAction
	Guidance string
}

type providerFailurePattern struct {
	kind     ProviderFailureKind
	action   ProviderFailureAction
	guidance string
	needles  []string
}

var providerFailurePatterns = []providerFailurePattern{
	{
		kind:     ProviderFailureInvalidModel,
		action:   ProviderFailureActionChangeModel,
		guidance: "choose a model configured for this provider",
		needles: []string{
			"invalid model",
			"unknown model",
			"model not found",
			"model does not exist",
			"unsupported model",
			"invalid model id",
		},
	},
	{
		kind:     ProviderFailureModelUnavailable,
		action:   ProviderFailureActionChangeModel,
		guidance: "choose an available model or refresh the provider model catalog",
		needles: []string{
			"model unavailable",
			"model is unavailable",
			"model not available",
			"model is not available",
			"not available for this provider",
			"not available in your region",
		},
	},
}

// ProviderFailureDiagnosticFromError returns stable provider recovery metadata
// for known ACP/native-provider failure signals.
func ProviderFailureDiagnosticFromError(err error) (ProviderFailureDiagnostic, bool) {
	if err == nil {
		return ProviderFailureDiagnostic{}, false
	}
	if errors.Is(err, execpkg.ErrNotFound) {
		return providerFailureDiagnosticFromClassification(authproviders.ClassifyError(err))
	}
	text := err.Error()
	if reqErr, ok := errors.AsType[*acpsdk.RequestError](err); ok {
		text = requestErrorDiagnosticText(reqErr)
	}
	if diagnostic, matched := providerFailureDiagnosticFromText(text); matched {
		return diagnostic, true
	}
	return providerAuthFailureDiagnosticFromText(text)
}

func providerAuthFailureDiagnosticFromError(err error) (ProviderFailureDiagnostic, bool) {
	if errors.Is(err, execpkg.ErrNotFound) {
		return providerFailureDiagnosticFromClassification(authproviders.ClassifyError(err))
	}
	text := err.Error()
	if reqErr, ok := errors.AsType[*acpsdk.RequestError](err); ok {
		text = requestErrorDiagnosticText(reqErr)
	}
	classification := authproviders.ClassifyProbeResult(
		aghconfig.ProviderConfig{AuthMode: aghconfig.ProviderAuthModeNativeCLI, Command: "provider"},
		authproviders.ProbeOutcome{ExitCode: 1, Stderr: text},
		&authproviders.ProbeEnv{
			ProviderName: "provider",
			LookPath: func(string) (string, error) {
				return "", execpkg.ErrNotFound
			},
		},
	)
	return providerFailureDiagnosticFromClassification(classification)
}

func providerAuthFailureDiagnosticFromText(text string) (ProviderFailureDiagnostic, bool) {
	if strings.TrimSpace(text) == "" {
		return ProviderFailureDiagnostic{}, false
	}
	return providerAuthFailureDiagnosticFromError(errors.New(text))
}

func providerFailureDiagnosticFromClassification(
	classification authproviders.Classification,
) (ProviderFailureDiagnostic, bool) {
	if classification.Kind == "" || classification.Kind == authproviders.ProviderFailureUnknown {
		return ProviderFailureDiagnostic{}, false
	}
	return providerFailureDiagnostic(
		ProviderFailureKind(classification.Kind),
		ProviderFailureAction(classification.Action),
		providerFailureGuidance(classification),
	), true
}

func providerFailureGuidance(classification authproviders.Classification) string {
	switch classification.Action {
	case authproviders.ProviderFailureActionInstallCLI:
		return "install the provider CLI and retry"
	case authproviders.ProviderFailureActionLogin:
		return "run provider auth login for this provider"
	case authproviders.ProviderFailureActionBindSecret:
		return "bind the required provider credential and retry"
	case authproviders.ProviderFailureActionRetry:
		return "retry after the provider recovers"
	case authproviders.ProviderFailureActionNoRetry:
		return "inspect provider access before retrying"
	case authproviders.ProviderFailureActionInspect:
		return "inspect provider auth status output"
	default:
		return strings.TrimSpace(classification.Message)
	}
}

func providerFailureDiagnosticFromText(text string) (ProviderFailureDiagnostic, bool) {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" {
		return ProviderFailureDiagnostic{}, false
	}
	for _, pattern := range providerFailurePatterns {
		if !providerTextContainsAny(normalized, pattern.needles...) {
			continue
		}
		return providerFailureDiagnostic(pattern.kind, pattern.action, pattern.guidance), true
	}
	return ProviderFailureDiagnostic{}, false
}

func providerFailureDiagnostic(
	kind ProviderFailureKind,
	action ProviderFailureAction,
	guidance string,
) ProviderFailureDiagnostic {
	return ProviderFailureDiagnostic{
		Kind:     kind,
		Action:   action,
		Guidance: strings.TrimSpace(guidance),
	}
}

func providerFailureDiagnosticSummary(err error, summary string) string {
	diagnostic, ok := ProviderFailureDiagnosticFromError(err)
	if !ok {
		return summary
	}
	return diagnostic.Summary(summary)
}

// Summary attaches stable recovery metadata to the redacted failure text.
func (d ProviderFailureDiagnostic) Summary(summary string) string {
	trimmed := strings.TrimSpace(summary)
	if providerTextContainsAny(strings.ToLower(trimmed), "provider_failure_kind=") {
		return trimmed
	}
	metadata := fmt.Sprintf(
		"provider_failure_kind=%s; next_action=%s",
		strings.TrimSpace(string(d.Kind)),
		strings.TrimSpace(string(d.Action)),
	)
	if guidance := strings.TrimSpace(d.Guidance); guidance != "" {
		metadata += "; guidance=" + guidance
	}
	if trimmed == "" {
		return metadata
	}
	return trimmed + "; " + metadata
}

func providerTextContainsAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}
