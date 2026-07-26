package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/providerauth"
	authproviders "github.com/compozy/agh/internal/providers"

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
	provider aghconfig.ProviderConfig,
	deps commandDeps,
	probeEnv *authproviders.ProbeEnv,
) (bool, error) {
	if provider.EffectiveAuthMode() != aghconfig.ProviderAuthModeNativeCLI {
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
	provider aghconfig.ProviderConfig,
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
	if provider.EffectiveAuthMode() == aghconfig.ProviderAuthModeNativeCLI {
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
	cfg *aghconfig.Config,
	providerRef string,
) (string, aghconfig.ProviderConfig, error) {
	providerName := aghconfig.CanonicalProviderName(providerRef)
	if providerName == "" {
		return "", aghconfig.ProviderConfig{}, errors.New("cli: provider is required")
	}
	var effective aghconfig.Config
	if cfg != nil {
		effective = *cfg
	}
	provider, err := effective.ResolveProvider(providerName)
	if err != nil {
		return "", aghconfig.ProviderConfig{}, fmt.Errorf("cli: resolve provider %q: %w", providerName, err)
	}
	return providerName, provider, nil
}

func providerNativeCLIStatus(
	provider aghconfig.ProviderConfig,
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

func providerMissingAuthLoginCommandError(providerName string, provider aghconfig.ProviderConfig) error {
	if provider.EffectiveAuthMode() != aghconfig.ProviderAuthModeNativeCLI {
		return fmt.Errorf("cli: provider %q does not define auth_login_command", providerName)
	}
	return fmt.Errorf(
		"cli: provider %q does not define auth_login_command; "+
			"run the provider's own login command outside AGH or set providers.%s.auth_login_command",
		providerName,
		providerName,
	)
}

func providerNativeCLIMissingMessage(
	providerName string,
	provider aghconfig.ProviderConfig,
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
	homePaths aghconfig.HomePaths,
	providerName string,
	provider aghconfig.ProviderConfig,
) ([]string, error) {
	return providerauth.NativeCLILoginEnv(homePaths, providerName, provider, os.Environ())
}

func providerOperatorLoginCommand(command string, loginEnv []string) (string, error) {
	return providerauth.OperatorLoginCommand(command, loginEnv)
}
