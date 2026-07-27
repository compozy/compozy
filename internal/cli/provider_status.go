package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/providerauth"
	authproviders "github.com/compozy/compozy/internal/providers"

	"github.com/spf13/cobra"
)

func buildProviderAuthStatus(
	ctx context.Context,
	deps commandDeps,
	runtime *runtimeContext,
	providerRef string,
	probe bool,
) (providerAuthStatusRecord, error) {
	if runtime == nil {
		return providerAuthStatusRecord{}, errors.New("cli: provider auth runtime is required")
	}
	providerName, provider, err := resolveProviderAuthTarget(&runtime.Config, providerRef)
	if err != nil {
		return providerAuthStatusRecord{}, err
	}
	probeEnv, err := providerAuthProbeEnv(runtime.HomePaths, providerName, provider, deps)
	if err != nil {
		return providerAuthStatusRecord{}, err
	}
	credentials, err := providerCredentialStatuses(ctx, provider, &probeEnv)
	if err != nil {
		return providerAuthStatusRecord{}, err
	}
	classification, err := authproviders.ClassifyDeclared(ctx, provider, &probeEnv)
	if err != nil {
		return providerAuthStatusRecord{}, err
	}
	record := providerAuthStatusRecord{
		Provider:      providerName,
		DisplayName:   strings.TrimSpace(provider.DisplayName),
		AuthMode:      string(provider.EffectiveAuthMode()),
		EnvPolicy:     string(provider.EffectiveEnvPolicy()),
		HomePolicy:    string(provider.EffectiveHomePolicy()),
		State:         string(classification.State),
		Code:          classification.Code,
		Message:       classification.Message,
		StatusCommand: strings.TrimSpace(provider.AuthStatusCmd),
		LoginCommand:  strings.TrimSpace(provider.AuthLoginCmd),
		Credentials:   credentials,
	}
	nativeReady, err := populateProviderNativeCLIStatus(&record, providerName, provider, deps, &probeEnv)
	if err != nil || !nativeReady {
		return record, err
	}
	if !probe || strings.TrimSpace(provider.AuthStatusCmd) == "" {
		return record, nil
	}
	if err := populateProviderAuthProbe(ctx, &record, provider, deps, &probeEnv); err != nil {
		return providerAuthStatusRecord{}, err
	}
	return record, nil
}

func populateProviderNativeCLIStatus(
	record *providerAuthStatusRecord,
	providerName string,
	provider compozyconfig.ProviderConfig,
	deps commandDeps,
	probeEnv *authproviders.ProbeEnv,
) (bool, error) {
	if provider.EffectiveAuthMode() != compozyconfig.ProviderAuthModeNativeCLI {
		return true, nil
	}
	nativeCLI, err := providerNativeCLIStatus(provider, deps.lookPath)
	if err != nil {
		return false, err
	}
	record.NativeCLI = nativeCLI
	if nativeCLI == nil || nativeCLI.Command == "" || nativeCLI.Present {
		return true, nil
	}
	missing := authproviders.ClassifyProbeResult(provider, authproviders.ProbeOutcome{
		ExitCode: -1,
		Stderr:   providerNativeCLIMissingMessage(providerName, provider, nativeCLI),
	}, probeEnv)
	record.State = string(missing.State)
	record.Code = missing.Code
	record.Message = missing.Message
	return false, nil
}

func populateProviderAuthProbe(
	ctx context.Context,
	record *providerAuthStatusRecord,
	provider compozyconfig.ProviderConfig,
	deps commandDeps,
	probeEnv *authproviders.ProbeEnv,
) error {
	result, err := deps.runProviderAuthCommand(ctx, providerAuthCommandSpec{
		Command: strings.TrimSpace(provider.AuthStatusCmd),
		Env:     probeEnv.CommandEnv,
		Timeout: defaultProviderAuthCommandTimeout,
		NoTTY:   true,
	})
	if err != nil {
		return err
	}
	record.Probe = &result
	if provider.EffectiveAuthMode() == compozyconfig.ProviderAuthModeNativeCLI {
		classification := authproviders.ClassifyProbeResult(provider, authproviders.ProbeOutcome{
			ExitCode: result.ExitCode,
			Stdout:   result.Stdout,
			Stderr:   result.Stderr,
		}, probeEnv)
		record.State = string(classification.State)
		record.Code = classification.Code
		record.Message = classification.Message
	}
	return nil
}

func resolveProviderAuthTarget(
	cfg *compozyconfig.Config,
	providerRef string,
) (string, compozyconfig.ProviderConfig, error) {
	providerName := compozyconfig.CanonicalProviderName(providerRef)
	if providerName == "" {
		return "", compozyconfig.ProviderConfig{}, errors.New("cli: provider is required")
	}
	var effective compozyconfig.Config
	if cfg != nil {
		effective = *cfg
	}
	provider, err := effective.ResolveProvider(providerName)
	if err != nil {
		return "", compozyconfig.ProviderConfig{}, fmt.Errorf("cli: resolve provider %q: %w", providerName, err)
	}
	return providerName, provider, nil
}

func providerNativeCLIStatus(
	provider compozyconfig.ProviderConfig,
	lookPath func(string) (string, error),
) (*providerNativeCLIStatusRecord, error) {
	return providerauth.NativeCLIStatusForProvider(provider, lookPath)
}

func providerNativeCLIStatusForCommand(
	command string,
	source string,
	lookPath func(string) (string, error),
) (*providerNativeCLIStatusRecord, error) {
	return providerauth.NativeCLIStatusForCommand(command, source, lookPath)
}

func providerMissingAuthLoginCommandError(providerName string, provider compozyconfig.ProviderConfig) error {
	if provider.EffectiveAuthMode() != compozyconfig.ProviderAuthModeNativeCLI {
		return fmt.Errorf("cli: provider %q does not define auth_login_command", providerName)
	}
	return fmt.Errorf(
		"cli: provider %q does not define auth_login_command; "+
			"run the provider's own login command outside Compozy or set providers.%s.auth_login_command",
		providerName,
		providerName,
	)
}

func providerNativeCLIMissingMessage(
	providerName string,
	provider compozyconfig.ProviderConfig,
	nativeCLI *providerNativeCLIStatusRecord,
) string {
	return providerauth.NativeCLIMissingMessage(providerName, provider, nativeCLI)
}

func rejectPrintCommandOutputFormat(cmd *cobra.Command) error {
	outputFlag := cmd.Flag(outputFlagName)
	if outputFlag != nil && outputFlag.Changed {
		return errors.New("cli: --print-command emits raw shell text and cannot be combined with --output")
	}
	jsonFlag := cmd.Flag(jsonFlagName)
	if jsonFlag != nil && jsonFlag.Changed {
		return errors.New("cli: --print-command emits raw shell text and cannot be combined with --json")
	}
	return nil
}

func providerNativeCLILoginEnv(
	homePaths compozyconfig.HomePaths,
	providerName string,
	provider compozyconfig.ProviderConfig,
) ([]string, error) {
	return providerauth.NativeCLILoginEnv(homePaths, providerName, provider, os.Environ())
}

func providerOperatorLoginCommand(command string, loginEnv []string) (string, error) {
	return providerauth.OperatorLoginCommand(command, loginEnv)
}
