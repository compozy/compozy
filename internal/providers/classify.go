package providers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	diagcontract "github.com/compozy/agh/internal/diagnosticcontract"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/providerauth"
)

// ProviderAuthState is the canonical provider authentication state.
type ProviderAuthState string

const (
	ProviderAuthStateAuthenticated     ProviderAuthState = "authenticated"
	ProviderAuthStateNeedsLogin        ProviderAuthState = "needs_login"
	ProviderAuthStateMissingCLI        ProviderAuthState = "missing_cli"
	ProviderAuthStateMissingCredential ProviderAuthState = "missing_credential"
	ProviderAuthStatePermissionDenied  ProviderAuthState = "permission_denied"
	ProviderAuthStateRateLimited       ProviderAuthState = "rate_limited"
	ProviderAuthStateTransient         ProviderAuthState = "transient"
	ProviderAuthStateNone              ProviderAuthState = "none"
	ProviderAuthStateUnknown           ProviderAuthState = "unknown"
)

// ProviderFailureKind identifies a classified provider failure bucket.
type ProviderFailureKind string

const (
	ProviderFailureNone       ProviderFailureKind = ""
	ProviderFailureCLIMissing ProviderFailureKind = "missing_cli"
	// #nosec G101 -- diagnostic enum value, not a credential.
	ProviderFailureCredentialUnresolved ProviderFailureKind = "credential_unresolved"
	ProviderFailureNotAuthenticated     ProviderFailureKind = "not_authenticated"
	ProviderFailurePermissionDenied     ProviderFailureKind = "permission_denied"
	ProviderFailureRateLimited          ProviderFailureKind = "rate_limited"
	ProviderFailureTransient            ProviderFailureKind = "transient"
	ProviderFailureUnknown              ProviderFailureKind = "unknown"
)

// ProviderFailureAction is the agent-facing recovery class for a provider failure.
type ProviderFailureAction string

const (
	ProviderFailureActionNone       ProviderFailureAction = ""
	ProviderFailureActionInstallCLI ProviderFailureAction = "install_cli"
	ProviderFailureActionLogin      ProviderFailureAction = "login"
	ProviderFailureActionBindSecret ProviderFailureAction = "bind_secret"
	ProviderFailureActionRetry      ProviderFailureAction = "retry"
	ProviderFailureActionInspect    ProviderFailureAction = "inspect"
	ProviderFailureActionNoRetry    ProviderFailureAction = "no_retry"
)

// ProviderAuthNoAuthRequiredMessage is the canonical no-auth provider status.
const ProviderAuthNoAuthRequiredMessage = "No auth required."

var providerTransientTransportNeedles = []string{
	"timeout",
	"timed out",
	"connection refused",
	"connection reset",
	"network is unreachable",
	"temporary failure",
	"temporarily unavailable",
	"server error",
	"provider overloaded",
	"overloaded",
	"http 500",
	"status 500",
	"http 502",
	"status 502",
	"http 503",
	"status 503",
	"http 504",
	"status 504",
	"http 529",
	"status 529",
	"529",
}

// ProbeOutcome is the redacted output from one provider auth status command.
type ProbeOutcome struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// Classification is one canonical provider-auth classifier result.
type Classification struct {
	State   ProviderAuthState
	Code    string
	Message string
	Kind    ProviderFailureKind
	Action  ProviderFailureAction
}

// ClassifyDeclared classifies provider readiness without executing a probe.
func ClassifyDeclared(
	ctx context.Context,
	provider aghconfig.ProviderConfig,
	env *ProbeEnv,
) (Classification, error) {
	authMode := provider.EffectiveAuthMode()
	if authMode == aghconfig.ProviderAuthModeNone {
		return Classification{
			State:   ProviderAuthStateNone,
			Message: ProviderAuthNoAuthRequiredMessage,
		}, nil
	}
	if authMode == aghconfig.ProviderAuthModeBoundSecret {
		statuses, err := CredentialStatuses(ctx, provider, env)
		if err != nil {
			return Classification{}, err
		}
		if missing, ok := firstMissingRequiredCredential(statuses); ok {
			return missingCredentialClassification(missing), nil
		}
		return Classification{
			State:   ProviderAuthStateAuthenticated,
			Message: "Required AGH-managed provider credentials are present.",
		}, nil
	}
	nativeCLI, err := NativeCLIStatus(provider, env)
	if err != nil {
		return Classification{}, err
	}
	if nativeCLI != nil && nativeCLI.Command != "" && !nativeCLI.Present {
		return missingCLIClassification(env.Normalize().ProviderName, provider, nativeCLI), nil
	}
	return Classification{
		State:   ProviderAuthStateUnknown,
		Code:    diagcontract.CodeProviderClassificationUnknown,
		Message: "Provider owns authentication through its native CLI; run a provider auth probe for live status.",
		Kind:    ProviderFailureUnknown,
		Action:  ProviderFailureActionInspect,
	}, nil
}

// ClassifyProbe classifies a live provider auth status command outcome.
func ClassifyProbe(
	provider aghconfig.ProviderConfig,
	outcome ProbeOutcome,
	env *ProbeEnv,
) (state string, code string, message string) {
	result := ClassifyProbeResult(provider, outcome, env)
	return string(result.State), result.Code, result.Message
}

// ClassifyProbeResult returns the full canonical classifier result.
func ClassifyProbeResult(
	provider aghconfig.ProviderConfig,
	outcome ProbeOutcome,
	env *ProbeEnv,
) Classification {
	authMode := provider.EffectiveAuthMode()
	if authMode == aghconfig.ProviderAuthModeNone {
		return Classification{
			State:   ProviderAuthStateNone,
			Message: ProviderAuthNoAuthRequiredMessage,
		}
	}
	combined := strings.ToLower(outcome.Stdout + "\n" + outcome.Stderr)
	nativeCLI, classification, classified := classifyNativeCLIProbePrecondition(provider, env, authMode, combined)
	if classified {
		return classification
	}
	if outcome.ExitCode == 0 && outputLooksAuthenticated(outcome) {
		return Classification{
			State:   ProviderAuthStateAuthenticated,
			Message: "Provider status command completed successfully.",
		}
	}
	if classification, classified := classifyProbeOutput(provider, combined); classified {
		return classification
	}
	if nativeCLI != nil && nativeCLI.Command != "" && !nativeCLI.Present &&
		hasAny(combined, "not found on path", "not found", "not installed") {
		return missingCLIClassification(env.Normalize().ProviderName, provider, nativeCLI)
	}
	return Classification{
		State:   ProviderAuthStateUnknown,
		Code:    diagcontract.CodeProviderClassificationUnknown,
		Message: "Provider auth probe completed but AGH could not classify the result.",
		Kind:    ProviderFailureUnknown,
		Action:  ProviderFailureActionInspect,
	}
}

func classifyNativeCLIProbePrecondition(
	provider aghconfig.ProviderConfig,
	env *ProbeEnv,
	authMode aghconfig.ProviderAuthMode,
	combined string,
) (*providerauth.NativeCLIStatus, Classification, bool) {
	if authMode != aghconfig.ProviderAuthModeNativeCLI {
		return nil, Classification{}, false
	}
	nativeCLI, err := NativeCLIStatus(provider, env)
	if err != nil {
		return nil, unknownClassification(err), true
	}
	if nativeCLI != nil && nativeCLI.Command != "" && !nativeCLI.Present && strings.TrimSpace(combined) == "" {
		return nativeCLI, missingCLIClassification(env.Normalize().ProviderName, provider, nativeCLI), true
	}
	return nativeCLI, Classification{}, false
}

func classifyProbeOutput(provider aghconfig.ProviderConfig, combined string) (Classification, bool) {
	switch {
	case hasAny(
		combined,
		"http 429",
		"status 429",
		"429",
		"rate limit",
		"rate_limit",
		"too many requests",
		"quota exceeded",
		"insufficient_quota",
	):
		return Classification{
			State:   ProviderAuthStateRateLimited,
			Code:    diagcontract.CodeProviderRateLimited,
			Message: "Provider auth probe was rate limited; retry later.",
			Kind:    ProviderFailureRateLimited,
			Action:  ProviderFailureActionRetry,
		}, true
	case hasAny(
		combined,
		"http 403",
		"status 403",
		"403",
		"forbidden",
		"permission denied",
		"access denied",
		"not entitled",
		"entitlement",
	):
		return Classification{
			State:   ProviderAuthStatePermissionDenied,
			Code:    diagcontract.CodeProviderPermissionDenied,
			Message: "Provider auth probe was denied by the provider.",
			Kind:    ProviderFailurePermissionDenied,
			Action:  ProviderFailureActionNoRetry,
		}, true
	case hasAny(combined, providerTransientTransportNeedles...):
		return Classification{
			State:   ProviderAuthStateTransient,
			Code:    diagcontract.CodeProviderTransientFailure,
			Message: "Provider auth probe failed with a transient transport error.",
			Kind:    ProviderFailureTransient,
			Action:  ProviderFailureActionRetry,
		}, true
	case hasAny(
		combined,
		"not logged in",
		"not authenticated",
		"unauthorized",
		"authentication required",
		"authentication failed",
		"login required",
		"invalid api key",
		"missing api key",
		"no api key",
		"expired token",
		"http 401",
		"status 401",
		"401",
	):
		return Classification{
			State:   ProviderAuthStateNeedsLogin,
			Code:    diagcontract.CodeProviderNotAuthenticated,
			Message: loginGuidance(provider),
			Kind:    ProviderFailureNotAuthenticated,
			Action:  ProviderFailureActionLogin,
		}, true
	}
	return Classification{}, false
}

// ClassifyError maps provider startup errors onto the canonical auth taxonomy when possible.
func ClassifyError(err error) Classification {
	if err == nil {
		return Classification{State: ProviderAuthStateAuthenticated}
	}
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
		return Classification{
			State:   ProviderAuthStateMissingCLI,
			Code:    diagcontract.CodeProviderCLIMissing,
			Message: "Provider CLI is not installed or not available on PATH.",
			Kind:    ProviderFailureCLIMissing,
			Action:  ProviderFailureActionInstallCLI,
		}
	}
	return unknownClassification(err)
}

func outputLooksAuthenticated(outcome ProbeOutcome) bool {
	combined := strings.ToLower(strings.TrimSpace(outcome.Stdout + "\n" + outcome.Stderr))
	return combined == "" || hasAny(combined, "authenticated", "logged in", "login ok", "authorized")
}

func hasAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

// MissingCredentialClassification builds the canonical missing-credential classifier.
func MissingCredentialClassification(missing CredentialStatus) Classification {
	return missingCredentialClassification(missing)
}

func missingCredentialClassification(missing CredentialStatus) Classification {
	target := strings.TrimSpace(missing.TargetEnv)
	if target == "" {
		target = strings.TrimSpace(missing.Name)
	}
	message := "Required AGH-managed provider credential is unresolved."
	if target != "" {
		message = fmt.Sprintf("Required AGH-managed provider credential %q is unresolved.", target)
	}
	return Classification{
		State:   ProviderAuthStateMissingCredential,
		Code:    diagcontract.CodeProviderCredentialUnresolved,
		Message: message,
		Kind:    ProviderFailureCredentialUnresolved,
		Action:  ProviderFailureActionBindSecret,
	}
}

func missingCLIClassification(
	providerName string,
	provider aghconfig.ProviderConfig,
	nativeCLI *providerauth.NativeCLIStatus,
) Classification {
	message := providerauth.NativeCLIMissingMessage(providerName, provider, nativeCLI)
	return Classification{
		State:   ProviderAuthStateMissingCLI,
		Code:    diagcontract.CodeProviderCLIMissing,
		Message: message,
		Kind:    ProviderFailureCLIMissing,
		Action:  ProviderFailureActionInstallCLI,
	}
}

func loginGuidance(provider aghconfig.ProviderConfig) string {
	if loginCommand := strings.TrimSpace(provider.AuthLoginCmd); loginCommand != "" {
		return fmt.Sprintf("Provider is not authenticated; run %q in a local terminal.", loginCommand)
	}
	return "Provider is not authenticated; run the provider's native login command in a local terminal."
}

func unknownClassification(err error) Classification {
	message := "Provider auth status is unknown."
	if err != nil {
		message = diagnostics.RedactAndBound(err.Error(), 1024)
	}
	return Classification{
		State:   ProviderAuthStateUnknown,
		Code:    diagcontract.CodeProviderClassificationUnknown,
		Message: message,
		Kind:    ProviderFailureUnknown,
		Action:  ProviderFailureActionInspect,
	}
}
