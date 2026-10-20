package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/providerauth"
	authproviders "github.com/compozy/compozy/internal/providers"

	"github.com/spf13/cobra"
)

const (
	providerMessageValue  = "Message"
	providerNameValue     = "Name"
	providerProviderValue = "Provider"
	providerStateValue    = "State"
	providerAuthKey       = "auth"
	providerAuthModeKey   = cliAuthModeKey
	providerAuthModeValue = "Auth Mode"
	providerEnvPolicyKey  = "env_policy"
	providerHomePolicyKey = "home_policy"
	providerProviderKey   = "provider"
	providerStateKey      = "state"
	providerVaultKey      = "vault"
)

type providerAuthCommandRunner = authproviders.ProviderAuthCommandRunner

type providerAuthCommandSpec = authproviders.ProviderAuthCommandSpec

type providerAuthCommandResult = authproviders.ProviderAuthCommandResult

type providerAuthStatusRecord struct {
	SessionEnv    []providerCredentialStatusItem        `json:"session_env,omitempty"`
	Provider      string                                `json:"provider"`
	DisplayName   string                                `json:"display_name,omitempty"`
	AuthMode      string                                `json:"auth_mode"`
	EnvPolicy     string                                `json:"env_policy"`
	HomePolicy    string                                `json:"home_policy"`
	State         string                                `json:"state"`
	Code          string                                `json:"code,omitempty"`
	Message       string                                `json:"message,omitempty"`
	StatusCommand string                                `json:"status_command,omitempty"`
	Login         authproviders.ProviderLoginDescriptor `json:"login"`
	Credentials   []providerCredentialStatusItem        `json:"credentials,omitempty"`
	Probe         *providerAuthCommandResult            `json:"probe,omitzero"`
}

type providerNativeCLIStatusRecord = providerauth.NativeCLIStatus

type providerCredentialStatusItem struct {
	Name      string `json:"name"`
	TargetEnv string `json:"target_env"`
	SecretRef string `json:"secret_ref"`
	Kind      string `json:"kind,omitempty"`
	Required  bool   `json:"required"`
	Present   bool   `json:"present"`
	Source    string `json:"source,omitempty"`
}

func (d commandDeps) withProviderAuthDefaults() commandDeps {
	if d.runProviderAuthCommand == nil {
		d.runProviderAuthCommand = authproviders.DefaultProviderAuthCommandRunner
	}
	if d.runProviderAuthLoginCommand == nil {
		d.runProviderAuthLoginCommand = authproviders.DefaultProviderAuthLoginRunner
	}
	if d.resolveProviderAuthCommand == nil {
		d.resolveProviderAuthCommand = authproviders.DefaultProviderAuthCommandResolver
	}
	if d.runACPProbe == nil {
		d.runACPProbe = acp.ProbeAuthentication
	}
	if d.runACPLogin == nil {
		d.runACPLogin = acp.Authenticate
	}
	if d.acpLoginStderr == nil {
		d.acpLoginStderr = os.Stderr
	}
	return d
}

func newProviderCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   providerProviderKey,
		Short: "Inspect and manage provider authentication",
	}
	cmd.AddCommand(newProviderAuthCommand(deps))
	cmd.AddCommand(newProviderInspectCommand(deps))
	cmd.AddCommand(newProviderListCommand(deps))
	cmd.AddCommand(newProviderModelsCommand(deps))
	return cmd
}

func newProviderInspectCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect <provider>",
		Short: "Inspect effective provider authentication and credential sources",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			profiles, client, err := profileResolutionClientFromDeps(deps)
			if err != nil {
				return err
			}
			resolution, err := resolveCommandProfile(cmd.Context(), cmd, deps, profiles, client)
			if err != nil {
				return err
			}
			runtime, err := providerAuthRuntime(deps)
			if err != nil {
				return err
			}
			record, err := buildProviderAuthStatus(cmd.Context(), deps, runtime, providerAuthStatusOptions{
				providerRef: args[0],
				profileName: resolution.Profile.Name,
				probe:       false,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, providerAuthStatusBundle(record))
		},
	}
	return cmd
}

func newProviderAuthCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   providerAuthKey,
		Short: "Inspect native CLI and bound-secret provider authentication",
	}
	cmd.AddCommand(newProviderAuthStatusCommand(deps))
	cmd.AddCommand(newProviderAuthLoginCommand(deps))
	return cmd
}

func newProviderAuthStatusCommand(deps commandDeps) *cobra.Command {
	var noProbe bool
	var remote bool
	cmd := &cobra.Command{
		Use:   "status <provider>",
		Short: "Show provider authentication status",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
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
			record, err := buildProviderAuthStatus(cmd.Context(), deps, runtime, providerAuthStatusOptions{
				providerRef: args[0],
				probe:       !noProbe,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, providerAuthStatusBundle(record))
		},
	}
	cmd.Flags().BoolVar(&noProbe, "no-probe", false, "Do not run the provider status command")
	cmd.Flags().BoolVar(&remote, "remote", false, "Run the provider status probe through the daemon")
	return cmd
}

func newProviderAuthLoginCommand(deps commandDeps) *cobra.Command {
	var noTTY bool
	var timeout time.Duration
	options := func() providerAuthLoginOptions {
		return providerAuthLoginOptions{noTTY: noTTY, timeout: timeout}
	}
	cmd := &cobra.Command{
		Use:   "login <provider>",
		Short: "Run the provider native login command locally",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProviderAuthLoginCommand(cmd, deps, args[0], options())
		},
	}
	cmd.Flags().BoolVar(&noTTY, "no-tty", false, "Disable TTY attachment for the login command")
	cmd.Flags().DurationVar(&timeout, cliTimeoutKey, 0, "Optional login command timeout")
	return cmd
}

type providerAuthLoginOptions struct {
	noTTY   bool
	timeout time.Duration
}

func newProviderListCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   extensionListKey,
		Short: "List providers and declared auth state",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			record, err := client.ListProviders(cmd.Context())
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, providerListBundle(record))
		},
	}
	return cmd
}

func providerLoginCommandEnv(loginEnv []string) []string {
	if len(loginEnv) == 0 {
		return os.Environ()
	}
	return append(os.Environ(), loginEnv...)
}

func providerAuthRuntime(deps commandDeps) (*runtimeContext, error) {
	runtime, err := loadRuntimeContext(deps)
	if err != nil {
		return nil, err
	}
	if err := deps.ensureHome(runtime.HomePaths); err != nil {
		return nil, err
	}
	return runtime, nil
}

func runProviderAuthLoginCommand(
	cmd *cobra.Command,
	deps commandDeps,
	providerRef string,
	options providerAuthLoginOptions,
) error {
	runtime, err := providerAuthRuntime(deps)
	if err != nil {
		return err
	}
	target, err := providerAuthLoginTarget(cmd.Context(), runtime, deps, providerRef)
	if err != nil {
		return err
	}
	if target.UseACP {
		return runACPProviderLogin(cmd, deps, runtime, &target, options)
	}
	loginResult, err := deps.runProviderAuthLoginCommand(cmd.Context(), providerAuthCommandSpec{
		Command: target.LoginCommand,
		Env:     providerLoginCommandEnv(target.LoginEnv),
		Timeout: options.timeout,
		NoTTY:   options.noTTY,
	})
	if err != nil {
		return err
	}
	record, err := buildProviderAuthStatus(cmd.Context(), deps, runtime, providerAuthStatusOptions{
		providerRef: target.ProviderName,
		probe:       true,
		nativeCLI:   target.nativeCLI,
	})
	if err != nil {
		return err
	}
	applyProviderAuthLoginResult(&record, target.Provider, loginResult)
	return writeCommandOutput(cmd, providerAuthStatusBundle(record))
}

type providerAuthLoginTargetRecord struct {
	ProviderName string
	Provider     compozyconfig.ProviderConfig
	LoginCommand string
	LoginEnv     []string
	// UseACP selects protocol-owned login through the configured auth method
	// when no native login command is present.
	UseACP     bool
	ACPMethod  string
	ACPCommand string
	ACPEnv     []string
	ACPCwd     string
	nativeCLI  *providerNativeCLIStatusRecord
}

// runACPProviderLogin performs one explicit protocol-owned login, then
// reports the verified readiness as a single JSON document. Live progress
// goes only to the operator's stderr.
func runACPProviderLogin(
	cmd *cobra.Command,
	deps commandDeps,
	runtime *runtimeContext,
	target *providerAuthLoginTargetRecord,
	options providerAuthLoginOptions,
) error {
	ctx := cmd.Context()
	if options.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.timeout)
		defer cancel()
	}
	err := deps.runACPLogin(ctx, acp.AuthenticationRequest{
		AgentName:         target.ProviderName,
		Command:           target.ACPCommand,
		Cwd:               target.ACPCwd,
		Env:               target.ACPEnv,
		MethodID:          target.ACPMethod,
		InteractiveOutput: deps.acpLoginStderr,
	})
	if err != nil {
		return mapACPLoginError(target.ACPMethod, options.timeout > 0, err)
	}
	record, err := buildProviderAuthStatus(cmd.Context(), deps, runtime, providerAuthStatusOptions{
		providerRef: target.ProviderName,
		probe:       true,
	})
	if err != nil {
		return err
	}
	return writeCommandOutput(cmd, providerAuthStatusBundle(record))
}

// mapACPLoginError keeps the documented login error contract: an unadvertised
// method and a supplied login timeout use stable messages, while canceled or
// failed readiness returns the ordinary CLI error envelope.
func mapACPLoginError(methodID string, hasTimeout bool, err error) error {
	if _, ok := errors.AsType[*acp.AuthMethodUnavailableError](err); ok {
		return fmt.Errorf(
			"provider auth: ACP method %q is not advertised as an agent method",
			methodID,
		)
	}
	if hasTimeout && errors.Is(err, context.DeadlineExceeded) {
		return errors.New("provider auth: ACP login timed out")
	}
	return err
}

func providerAuthLoginTarget(
	ctx context.Context,
	runtime *runtimeContext,
	deps commandDeps,
	providerRef string,
) (providerAuthLoginTargetRecord, error) {
	providerName, provider, err := resolveProviderAuthTarget(&runtime.Config, providerRef)
	if err != nil {
		return providerAuthLoginTargetRecord{}, err
	}
	loginCommand := strings.TrimSpace(provider.AuthLoginCmd)
	if loginCommand == "" {
		if provider.EffectiveAuthMode() == compozyconfig.ProviderAuthModeNativeCLI &&
			authproviders.UsesACPLogin(provider) {
			return providerACPLoginTarget(ctx, runtime, deps, providerName, provider)
		}
		return providerAuthLoginTargetRecord{}, providerMissingAuthLoginCommandError(providerName, provider)
	}
	if provider.EffectiveAuthMode() != compozyconfig.ProviderAuthModeNativeCLI {
		return providerAuthLoginTargetRecord{}, fmt.Errorf(
			"cli: provider %q uses auth_mode %q; provider auth login only exposes native_cli login commands",
			providerName,
			provider.EffectiveAuthMode(),
		)
	}
	nativeCLI, err := providerNativeCLIStatusForCommand(
		loginCommand,
		providerauth.NativeCLISourceAuthLogin,
		deps.lookPath,
	)
	if err != nil {
		return providerAuthLoginTargetRecord{}, err
	}
	if nativeCLI != nil && !nativeCLI.Present {
		return providerAuthLoginTargetRecord{}, providerMissingNativeCLIError(providerName, provider, nativeCLI)
	}
	loginEnv, err := providerNativeCLILoginEnv(runtime.HomePaths, providerName, provider)
	if err != nil {
		return providerAuthLoginTargetRecord{}, err
	}
	return providerAuthLoginTargetRecord{
		ProviderName: providerName,
		Provider:     provider,
		LoginCommand: loginCommand,
		LoginEnv:     loginEnv,
		nativeCLI:    nativeCLI,
	}, nil
}

// providerMissingNativeCLIError owns the stable missing-CLI message for both login paths.
func providerMissingNativeCLIError(
	providerName string,
	provider compozyconfig.ProviderConfig,
	nativeCLI *providerNativeCLIStatusRecord,
) error {
	executable := authproviders.DescribeProviderLogin(
		provider,
		nativeCLI,
		authproviders.Classification{},
	).Executable
	return fmt.Errorf(
		"cli: provider %q native CLI %q was not found on PATH; install it before retrying login",
		providerName,
		executable,
	)
}

// providerACPLoginTarget resolves the effective launch command, environment,
// and presence for one protocol-owned login. The executable stays a safe
// basename in errors; a missing binary fails before any process starts.
func providerACPLoginTarget(
	ctx context.Context,
	runtime *runtimeContext,
	deps commandDeps,
	providerName string,
	provider compozyconfig.ProviderConfig,
) (providerAuthLoginTargetRecord, error) {
	method := strings.TrimSpace(provider.AuthACPMethod)
	command := strings.TrimSpace(provider.Command)
	probeEnv, err := providerAuthProbeEnv(runtime.HomePaths, providerName, provider, deps)
	if err != nil {
		return providerAuthLoginTargetRecord{}, err
	}
	parsed, err := providerauth.ParseCommand(command)
	if err != nil {
		return providerAuthLoginTargetRecord{}, fmt.Errorf(
			"cli: provider %q command is not runnable: %w",
			providerName,
			err,
		)
	}
	resolve := probeEnv.Normalize().ResolveCommand
	if resolve == nil {
		resolve = authproviders.DefaultProviderAuthCommandResolver
	}
	if _, err := resolve(ctx, parsed.Executable, probeEnv.CommandEnv, probeEnv.CommandDir); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return providerAuthLoginTargetRecord{}, providerMissingNativeCLIError(
				providerName,
				provider,
				&providerNativeCLIStatusRecord{Command: parsed.Executable},
			)
		}
		return providerAuthLoginTargetRecord{}, fmt.Errorf(
			"cli: resolve provider %q executable: %w",
			providerName,
			err,
		)
	}
	return providerAuthLoginTargetRecord{
		ProviderName: providerName,
		Provider:     provider,
		UseACP:       true,
		ACPMethod:    method,
		ACPCommand:   command,
		ACPEnv:       probeEnv.CommandEnv,
		ACPCwd:       probeEnv.CommandDir,
	}, nil
}

func applyProviderAuthLoginResult(
	record *providerAuthStatusRecord,
	provider compozyconfig.ProviderConfig,
	loginResult providerAuthCommandResult,
) {
	if record == nil || loginResult.ExitCode == 0 {
		return
	}
	classification := authproviders.ClassifyProbeResult(provider, authproviders.ProbeOutcome{
		ExitCode: loginResult.ExitCode,
		Stdout:   loginResult.Stdout,
		Stderr:   loginResult.Stderr,
	}, nil)
	record.State = string(classification.State)
	record.Code = classification.Code
	record.Message = classification.Message
	record.Probe = &loginResult
}
