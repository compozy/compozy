package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/providerauth"
	authproviders "github.com/compozy/compozy/internal/providers"
)

type providerAuthStatusOptions struct {
	providerRef string
	probe       bool
	nativeCLI   *providerNativeCLIStatusRecord
}

func buildProviderAuthStatus(
	ctx context.Context,
	deps commandDeps,
	runtime *runtimeContext,
	options providerAuthStatusOptions,
) (providerAuthStatusRecord, error) {
	if runtime == nil {
		return providerAuthStatusRecord{}, errors.New("cli: provider auth runtime is required")
	}
	providerName, provider, err := resolveProviderAuthTarget(&runtime.Config, options.providerRef)
	if err != nil {
		return providerAuthStatusRecord{}, err
	}
	probeEnv, err := providerAuthProbeEnv(runtime.HomePaths, providerName, provider, deps)
	if err != nil {
		return providerAuthStatusRecord{}, err
	}
	pinProviderNativeCLIResolution(&probeEnv, options.nativeCLI)
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
		Login:         authproviders.DescribeProviderLogin(provider, nil, classification),
		Credentials:   credentials,
	}
	nativeReady, err := populateProviderNativeCLIStatus(ctx, providerNativeCLIStatusInput{
		record:       &record,
		providerName: providerName,
		provider:     provider,
		probeEnv:     &probeEnv,
		nativeCLI:    options.nativeCLI,
	})
	if err != nil || !nativeReady {
		return record, err
	}
	if !options.probe || strings.TrimSpace(provider.AuthStatusCmd) == "" {
		return record, nil
	}
	commandSpec, err := authproviders.PrepareAuthStatusCommand(ctx, provider, &probeEnv)
	if err != nil {
		return providerAuthStatusRecord{}, fmt.Errorf("cli: resolve provider auth status command: %w", err)
	}
	if err := populateProviderAuthProbe(ctx, &record, provider, &probeEnv, commandSpec); err != nil {
		return providerAuthStatusRecord{}, err
	}
	return record, nil
}

func pinProviderNativeCLIResolution(
	probeEnv *authproviders.ProbeEnv,
	nativeCLI *providerNativeCLIStatusRecord,
) {
	if probeEnv == nil || nativeCLI == nil || strings.TrimSpace(nativeCLI.Command) == "" {
		return
	}
	normalized := probeEnv.Normalize()
	lookPath := normalized.LookPath
	resolveCommand := probeEnv.ResolveCommand
	if resolveCommand == nil {
		resolveCommand = authproviders.DefaultProviderAuthCommandResolver
	}
	resolvePinned := func(command string) (string, error, bool) {
		if strings.TrimSpace(command) != nativeCLI.Command {
			return "", nil, false
		}
		if !nativeCLI.Present {
			return "", exec.ErrNotFound, true
		}
		return nativeCLI.Path, nil, true
	}
	probeEnv.LookPath = func(command string) (string, error) {
		if path, err, ok := resolvePinned(command); ok {
			return path, err
		}
		return lookPath(command)
	}
	probeEnv.ResolveCommand = func(
		ctx context.Context,
		command string,
		env []string,
		dir string,
	) (string, error) {
		if path, err, ok := resolvePinned(command); ok {
			return path, err
		}
		return resolveCommand(ctx, command, env, dir)
	}
}

type providerNativeCLIStatusInput struct {
	record       *providerAuthStatusRecord
	providerName string
	provider     compozyconfig.ProviderConfig
	probeEnv     *authproviders.ProbeEnv
	nativeCLI    *providerNativeCLIStatusRecord
}

func populateProviderNativeCLIStatus(
	ctx context.Context,
	input providerNativeCLIStatusInput,
) (bool, error) {
	if input.provider.EffectiveAuthMode() != compozyconfig.ProviderAuthModeNativeCLI {
		return true, nil
	}
	nativeCLI := input.nativeCLI
	_, source := providerauth.NativeCLICommand(input.provider)
	if nativeCLI == nil || nativeCLI.Source != source {
		var err error
		nativeCLI, err = authproviders.NativeCLIStatus(ctx, input.provider, input.probeEnv)
		if err != nil {
			return false, err
		}
	}
	input.record.Login = authproviders.DescribeProviderLogin(input.provider, nativeCLI, authproviders.Classification{
		State:   authproviders.ProviderAuthState(input.record.State),
		Code:    input.record.Code,
		Message: input.record.Message,
	})
	if nativeCLI == nil || nativeCLI.Command == "" || nativeCLI.Present {
		return true, nil
	}
	if strings.TrimSpace(nativeCLI.Error) != "" {
		return false, nil
	}
	missing := authproviders.ClassifyProbeResultContext(ctx, input.provider, authproviders.ProbeOutcome{
		ExitCode: -1,
		Stderr:   providerNativeCLIMissingMessage(input.providerName, nativeCLI),
	}, input.probeEnv)
	input.record.State = string(missing.State)
	input.record.Code = missing.Code
	input.record.Message = missing.Message
	input.record.Login = authproviders.DescribeProviderLogin(input.provider, nativeCLI, missing)
	return false, nil
}

func populateProviderAuthProbe(
	ctx context.Context,
	record *providerAuthStatusRecord,
	provider compozyconfig.ProviderConfig,
	probeEnv *authproviders.ProbeEnv,
	commandSpec providerAuthCommandSpec,
) error {
	if commandSpec.Executable == "" {
		return errors.New("cli: provider auth status command is not pinned")
	}
	runner := probeEnv.Normalize().RunCommand
	result, err := runner(ctx, commandSpec)
	if err != nil {
		return err
	}
	record.Probe = &result
	if provider.EffectiveAuthMode() == compozyconfig.ProviderAuthModeNativeCLI {
		classification := authproviders.ClassifyProbeResultContext(ctx, provider, authproviders.ProbeOutcome{
			ExitCode: result.ExitCode,
			Stdout:   result.Stdout,
			Stderr:   result.Stderr,
		}, probeEnv)
		record.State = string(classification.State)
		record.Code = classification.Code
		record.Message = classification.Message
		record.Login.RecommendedAction = classification.Action
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
			"run the provider's own login command outside CompozyOS or set providers.%s.auth_login_command",
		providerName,
		providerName,
	)
}

func providerNativeCLIMissingMessage(
	providerName string,
	nativeCLI *providerNativeCLIStatusRecord,
) string {
	return providerauth.NativeCLIMissingMessage(providerName, nativeCLI)
}

func providerNativeCLILoginEnv(
	homePaths compozyconfig.HomePaths,
	providerName string,
	provider compozyconfig.ProviderConfig,
) ([]string, error) {
	return providerauth.NativeCLILoginEnv(homePaths, providerName, provider, os.Environ())
}
