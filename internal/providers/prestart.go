package providers

import (
	"context"
	"errors"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	diagcontract "github.com/compozy/compozy/internal/diagnosticcontract"
)

// PreStartReport carries a structured diagnostic when the pre-start probe fails.
type PreStartReport struct {
	Item  *diagcontract.DiagnosticItem
	Cause error
}

func runPreStart(
	ctx context.Context,
	provider compozyconfig.ProviderConfig,
	env *ProbeEnv,
) PreStartReport {
	normalized := env.Normalize()
	if ctx == nil {
		return preStartErrorReport(&normalized, errors.New("providers: pre-start context is required"))
	}
	if err := ctx.Err(); err != nil {
		return preStartErrorReport(&normalized, err)
	}

	if provider.EffectiveAuthMode() == compozyconfig.ProviderAuthModeNone {
		return PreStartReport{}
	}
	launchCLI, err := LaunchCommandStatus(ctx, provider, &normalized)
	if err != nil {
		return preStartErrorReport(&normalized, err)
	}
	if launchCLI != nil && launchCLI.Command != "" && !launchCLI.Present {
		classification := Classification{
			State:   ProviderAuthStateMissingCLI,
			Code:    diagcontract.CodeProviderCLIMissing,
			Message: "Provider CLI is not installed or not available on PATH.",
			Kind:    ProviderFailureCLIMissing,
			Action:  ProviderFailureActionInstallCLI,
		}
		return PreStartReport{Item: new(DiagnosticItem(normalized.ProviderName, classification))}
	}
	classification, err := ClassifyDeclared(ctx, provider, &normalized)
	if err != nil {
		return preStartErrorReport(&normalized, err)
	}
	if classification.Code != "" && classification.State != ProviderAuthStateUnknown {
		return PreStartReport{Item: new(DiagnosticItem(normalized.ProviderName, classification))}
	}
	if strings.TrimSpace(provider.AuthStatusCmd) == "" {
		return PreStartReport{}
	}
	commandSpec, err := PrepareAuthStatusCommand(ctx, provider, &normalized)
	if err != nil {
		return preStartErrorReport(&normalized, err)
	}
	result, err := normalized.RunCommand(ctx, commandSpec)
	if err != nil {
		return preStartErrorReport(&normalized, err)
	}
	if err := ctx.Err(); err != nil {
		return preStartErrorReport(&normalized, err)
	}
	probeClassification := ClassifyProbeResultContext(ctx, provider, ProbeOutcome{
		ExitCode: result.ExitCode,
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
	}, &normalized)
	if probeClassification.Code == "" {
		return PreStartReport{}
	}
	return PreStartReport{Item: new(DiagnosticItem(normalized.ProviderName, probeClassification))}
}

func preStartErrorReport(env *ProbeEnv, cause error) PreStartReport {
	return PreStartReport{Item: new(DiagnosticItem(env.ProviderName, ClassifyError(cause))), Cause: cause}
}
