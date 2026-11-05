package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/providerauth"
	authproviders "github.com/compozy/compozy/internal/providers"

	"github.com/spf13/cobra"
)

type providerAuthStatusOptions struct {
	providerRef string
	profileName string
	probe       bool
	nativeCLI   *providerNativeCLIStatusRecord
	// daemonProbe runs native-CLI probes in the daemon; any failure falls back to the local probe.
	daemonProbe func(context.Context, string) (contract.ProviderAuthProbeResponse, error)
}

const (
	providerProbeSourceDaemon = "daemon"
	providerProbeSourceLocal  = "local"
)

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
	profileSlots, err := authproviders.ProfileCredentialSlots(
		ctx, providerName, options.profileName, provider.EffectiveCredentialSlots(), probeEnv.Vault,
	)
	if err != nil {
		return providerAuthStatusRecord{}, err
	}
	provider.CredentialSlots = profileSlots
	credentials, err := providerCredentialStatuses(ctx, provider, &probeEnv)
	if err != nil {
		return providerAuthStatusRecord{}, err
	}
	labelProviderCredentialSources(credentials, options.profileName)
	sessionEnv, err := authproviders.SessionEnvStatuses(ctx, provider.SessionEnv, &probeEnv)
	if err != nil {
		return providerAuthStatusRecord{}, err
	}
	classification, err := authproviders.ClassifyDeclared(ctx, provider, &probeEnv)
	if err != nil {
		return providerAuthStatusRecord{}, err
	}
	record := providerAuthStatusRecord{
		SessionEnv:    providerCredentialStatusItems(sessionEnv),
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
	return probeProviderAuthStatus(ctx, deps, providerName, provider, &probeEnv, record, options)
}

// probeProviderAuthStatus prefers the daemon for native-CLI providers, else probes in process.
func probeProviderAuthStatus(
	ctx context.Context,
	deps commandDeps,
	providerName string,
	provider compozyconfig.ProviderConfig,
	probeEnv *authproviders.ProbeEnv,
	record providerAuthStatusRecord,
	options providerAuthStatusOptions,
) (providerAuthStatusRecord, error) {
	statusCommand := strings.TrimSpace(provider.AuthStatusCmd) != ""
	if !options.probe || (!statusCommand && !authproviders.UsesACPProbe(provider)) {
		return record, nil
	}
	// The daemon reports no ACP command, so only status-command verdicts can be matched to this config.
	if options.daemonProbe != nil && statusCommand &&
		provider.EffectiveAuthMode() == compozyconfig.ProviderAuthModeNativeCLI {
		if response, err := options.daemonProbe(ctx, providerName); err == nil && daemonProbeMatches(record, response) {
			return applyDaemonProviderAuthProbe(record, response), nil
		}
	}
	record.ProbeSource = providerProbeSourceLocal
	if !statusCommand {
		return populateACPProviderAuthProbe(ctx, deps, providerName, provider, probeEnv, record)
	}
	commandSpec, err := authproviders.PrepareAuthStatusCommand(ctx, provider, probeEnv)
	if err != nil {
		return providerAuthStatusRecord{}, fmt.Errorf("cli: resolve provider auth status command: %w", err)
	}
	if err := populateProviderAuthProbe(ctx, &record, provider, probeEnv, commandSpec); err != nil {
		return providerAuthStatusRecord{}, err
	}
	return record, nil
}

// daemonProbeMatches accepts a daemon verdict only when the daemon probed the settings this CLI reads;
// a daemon that has not reloaded an edited config.toml falls back to the local probe.
func daemonProbeMatches(record providerAuthStatusRecord, response contract.ProviderAuthProbeResponse) bool {
	status := response.AuthStatus
	return status.Mode == record.AuthMode && status.EnvPolicy == record.EnvPolicy &&
		status.HomePolicy == record.HomePolicy && status.StatusCmd == record.StatusCommand
}

// applyDaemonProviderAuthProbe keeps the local record's shape and takes the daemon's live verdict.
func applyDaemonProviderAuthProbe(
	record providerAuthStatusRecord,
	response contract.ProviderAuthProbeResponse,
) providerAuthStatusRecord {
	record.State = response.AuthStatus.State
	record.Code = response.AuthStatus.Code
	record.Message = response.AuthStatus.Message
	record.Login.RecommendedAction = authproviders.ProviderFailureAction(response.AuthStatus.Login.RecommendedAction)
	record.Login.Presence = authproviders.ProviderLoginPresence(response.AuthStatus.Login.Presence)
	if probe := response.Probe; probe != nil {
		record.Probe = &providerAuthCommandResult{
			ExitCode:   probe.ExitCode,
			Stdout:     probe.Stdout,
			Stderr:     probe.Stderr,
			DurationMs: probe.DurationMs,
		}
	}
	record.ProbeSource = providerProbeSourceDaemon
	return record
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
	result.Stdout = authproviders.RedactAuthProbeOutput(result.Stdout, 4096)
	result.Stderr = authproviders.RedactAuthProbeOutput(result.Stderr, 4096)
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

// populateACPProviderAuthProbe runs one non-interactive disposable ACP probe
// for providers without a native status command. It never sends authenticate,
// never attaches account output, and leaves Probe unset: no native status
// command result exists.
func populateACPProviderAuthProbe(
	ctx context.Context,
	deps commandDeps,
	providerName string,
	provider compozyconfig.ProviderConfig,
	probeEnv *authproviders.ProbeEnv,
	record providerAuthStatusRecord,
) (providerAuthStatusRecord, error) {
	if probeEnv == nil {
		return providerAuthStatusRecord{}, errors.New("cli: provider auth probe env is required")
	}
	normalized := probeEnv.Normalize()
	probe := deps.runACPProbe
	if probe == nil {
		probe = acp.ProbeAuthentication
	}
	err := probe(ctx, acp.SessionInspectionRequest{
		AgentName: providerName,
		Command:   strings.TrimSpace(provider.Command),
		Cwd:       normalized.CommandDir,
		Env:       append([]string(nil), normalized.CommandEnv...),
	})
	if ctx.Err() != nil {
		return providerAuthStatusRecord{}, ctx.Err()
	}
	classification := acp.ClassifyProbeError(providerName, err)
	record.State = string(classification.State)
	record.Code = classification.Code
	record.Message = classification.Message
	record.Login.RecommendedAction = classification.Action
	return record, nil
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

// localDaemonProviderAuthProbe probes through the local daemon socket, never a remote connection profile.
func localDaemonProviderAuthProbe(
	deps commandDeps,
	runtime *runtimeContext,
) func(context.Context, string) (contract.ProviderAuthProbeResponse, error) {
	return func(ctx context.Context, providerName string) (contract.ProviderAuthProbeResponse, error) {
		target, err := resolveLocalClientTarget(deps, runtime.HomePaths)
		if err != nil {
			return contract.ProviderAuthProbeResponse{}, err
		}
		client, err := deps.newClient(target)
		if err != nil {
			return contract.ProviderAuthProbeResponse{}, err
		}
		if client == nil {
			return contract.ProviderAuthProbeResponse{}, errors.New("cli: daemon client unavailable")
		}
		return client.ProbeProviderAuth(ctx, providerName)
	}
}

func newProviderAuthStatusCommand(deps commandDeps) *cobra.Command {
	var noProbe bool
	var remote bool
	var local bool
	cmd := &cobra.Command{
		Use:   "status <provider>",
		Short: "Show provider authentication status",
		Long: "Show provider authentication status. For native_cli providers with an auth status command " +
			"the live probe runs in the local daemon when it is reachable, so Settings shows the same verdict; " +
			"otherwise it runs in this process. --local always probes in this process.",
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if local && remote {
				return errors.New("cli: --local and --remote cannot be used together")
			}
			if remote {
				client, err := clientFromDeps(deps)
				if err != nil {
					return err
				}
				record, err := client.ProbeProviderAuth(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				return writeCommandOutput(cmd, providerAuthProbeBundle(record))
			}
			runtime, err := providerAuthRuntime(deps)
			if err != nil {
				return err
			}
			options := providerAuthStatusOptions{providerRef: args[0], probe: !noProbe}
			if !local {
				options.daemonProbe = localDaemonProviderAuthProbe(deps, runtime)
			}
			record, err := buildProviderAuthStatus(cmd.Context(), deps, runtime, options)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, providerAuthStatusBundle(record))
		},
	}
	cmd.Flags().BoolVar(&noProbe, "no-probe", false, "Do not run the provider status command")
	cmd.Flags().BoolVar(
		&remote,
		"remote",
		false,
		"Return the daemon's probe payload through the configured client target",
	)
	cmd.Flags().BoolVar(
		&local,
		"local",
		false,
		"Run the provider status probe in this CLI process, even when the daemon is running",
	)
	return cmd
}
