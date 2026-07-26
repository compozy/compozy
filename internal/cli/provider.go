package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/providerauth"
	authproviders "github.com/compozy/agh/internal/providers"

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

const (
	defaultProviderAuthCommandTimeout = authproviders.DefaultProviderAuthCommandTimeout
)

type providerAuthCommandRunner = authproviders.ProviderAuthCommandRunner

type providerAuthCommandSpec = authproviders.ProviderAuthCommandSpec

type providerAuthCommandResult = authproviders.ProviderAuthCommandResult

type providerAuthStatusRecord struct {
	Provider      string                         `json:"provider"`
	DisplayName   string                         `json:"display_name,omitempty"`
	AuthMode      string                         `json:"auth_mode"`
	EnvPolicy     string                         `json:"env_policy"`
	HomePolicy    string                         `json:"home_policy"`
	State         string                         `json:"state"`
	Code          string                         `json:"code,omitempty"`
	Message       string                         `json:"message,omitempty"`
	StatusCommand string                         `json:"status_command,omitempty"`
	LoginCommand  string                         `json:"login_command,omitempty"`
	LoginEnv      []string                       `json:"login_env,omitempty"`
	NativeCLI     *providerNativeCLIStatusRecord `json:"native_cli,omitempty"`
	Credentials   []providerCredentialStatusItem `json:"credentials,omitempty"`
	Probe         *providerAuthCommandResult     `json:"probe,omitempty"`
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
	return d
}

func newProviderCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   providerProviderKey,
		Short: "Inspect and manage provider authentication",
	}
	cmd.AddCommand(newProviderAuthCommand(deps))
	cmd.AddCommand(newProviderListCommand(deps))
	cmd.AddCommand(newProviderModelsCommand(deps))
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
			record, err := buildProviderAuthStatus(cmd.Context(), deps, runtime, args[0], !noProbe)
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
	var printCommand bool
	var noTTY bool
	var timeout time.Duration
	options := func() providerAuthLoginOptions {
		return providerAuthLoginOptions{printCommand: printCommand, noTTY: noTTY, timeout: timeout}
	}
	cmd := &cobra.Command{
		Use:   "login <provider>",
		Short: "Run the provider native login command locally",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProviderAuthLoginCommand(cmd, deps, args[0], options())
		},
	}
	cmd.Flags().
		BoolVar(&printCommand, "print-command", false, "Print only the resolved provider login command")
	cmd.Flags().BoolVar(&noTTY, "no-tty", false, "Disable TTY attachment for the login command")
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "Optional login command timeout")
	return cmd
}

type providerAuthLoginOptions struct {
	printCommand bool
	noTTY        bool
	timeout      time.Duration
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
	target, err := providerAuthLoginTarget(runtime, deps, providerRef)
	if err != nil {
		return err
	}
	if options.printCommand {
		if err := rejectPrintCommandOutputFormat(cmd); err != nil {
			return err
		}
		return writeRawCommandOutput(cmd, target.OperatorCommand+"\n")
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
	record, err := buildProviderAuthStatus(cmd.Context(), deps, runtime, target.ProviderName, true)
	if err != nil {
		return err
	}
	applyProviderAuthLoginResult(&record, target.Provider, loginResult)
	record.LoginCommand = firstNonEmptyString(record.LoginCommand, target.LoginCommand)
	record.LoginEnv = target.LoginEnv
	record.NativeCLI = target.NativeCLI
	return writeCommandOutput(cmd, providerAuthStatusBundle(record))
}

type providerAuthLoginTargetRecord struct {
	ProviderName    string
	Provider        aghconfig.ProviderConfig
	LoginCommand    string
	LoginEnv        []string
	NativeCLI       *providerNativeCLIStatusRecord
	OperatorCommand string
}

func providerAuthLoginTarget(
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
		return providerAuthLoginTargetRecord{}, providerMissingAuthLoginCommandError(providerName, provider)
	}
	if provider.EffectiveAuthMode() != aghconfig.ProviderAuthModeNativeCLI {
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
		return providerAuthLoginTargetRecord{}, fmt.Errorf(
			"cli: provider %q native CLI %q was not found on PATH; install it before running %q",
			providerName,
			nativeCLI.Command,
			loginCommand,
		)
	}
	loginEnv, err := providerNativeCLILoginEnv(runtime.HomePaths, providerName, provider)
	if err != nil {
		return providerAuthLoginTargetRecord{}, err
	}
	operatorCommand, err := providerOperatorLoginCommand(loginCommand, loginEnv)
	if err != nil {
		return providerAuthLoginTargetRecord{}, err
	}
	return providerAuthLoginTargetRecord{
		ProviderName:    providerName,
		Provider:        provider,
		LoginCommand:    loginCommand,
		LoginEnv:        loginEnv,
		NativeCLI:       nativeCLI,
		OperatorCommand: operatorCommand,
	}, nil
}

func applyProviderAuthLoginResult(
	record *providerAuthStatusRecord,
	provider aghconfig.ProviderConfig,
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
