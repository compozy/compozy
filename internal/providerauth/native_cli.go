// Package providerauth contains provider-auth diagnostics shared by CLI and daemon settings surfaces.
package providerauth

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/procutil"
	"github.com/compozy/agh/internal/providerenv"
	"github.com/kballard/go-shellquote"
)

const (
	// NativeCLISourceAuthStatus reports that the probe binary came from auth_status_command.
	NativeCLISourceAuthStatus = "auth_status_command"
	// NativeCLISourceAuthLogin reports that the probe binary came from auth_login_command.
	NativeCLISourceAuthLogin = "auth_login_command"
	// NativeCLISourceCommand reports that the probe binary came from the provider launch command.
	NativeCLISourceCommand = "provider_command"
)

// NativeCLIStatus reports whether the provider-owned CLI binary is available.
type NativeCLIStatus struct {
	Command string `json:"command,omitempty"`
	Present bool   `json:"present"`
	Path    string `json:"path,omitempty"`
	Source  string `json:"source,omitempty"`
	Error   string `json:"error,omitempty"`
}

// NativeCLIStatusForProvider resolves the native CLI command used for auth diagnostics.
func NativeCLIStatusForProvider(
	provider aghconfig.ProviderConfig,
	lookPath func(string) (string, error),
) (*NativeCLIStatus, error) {
	command, source := NativeCLICommand(provider)
	return NativeCLIStatusForCommand(command, source, lookPath)
}

// NativeCLICommand returns the command string and source used for native CLI auth diagnostics.
func NativeCLICommand(provider aghconfig.ProviderConfig) (string, string) {
	if command := strings.TrimSpace(provider.AuthStatusCmd); command != "" {
		return command, NativeCLISourceAuthStatus
	}
	if command := strings.TrimSpace(provider.AuthLoginCmd); command != "" {
		return command, NativeCLISourceAuthLogin
	}
	return strings.TrimSpace(provider.Command), NativeCLISourceCommand
}

// NativeCLIStatusForCommand resolves the first argv token in a provider-owned CLI command.
func NativeCLIStatusForCommand(
	command string,
	source string,
	lookPath func(string) (string, error),
) (*NativeCLIStatus, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, errors.New("provider auth: native CLI command is required")
	}
	argv, err := shellquote.Split(command)
	if err != nil {
		return nil, fmt.Errorf("provider auth: parse native CLI command: %w", err)
	}
	if len(argv) == 0 {
		return nil, errors.New("provider auth: native CLI command is empty")
	}
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	status := &NativeCLIStatus{
		Command: argv[0],
		Source:  source,
	}
	path, err := lookPath(argv[0])
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
			return status, nil
		}
		status.Error = diagnostics.RedactAndBound(err.Error(), 1024)
		return status, nil
	}
	status.Present = true
	status.Path = path
	return status, nil
}

// NativeCLIMissingMessage explains how to recover when the configured native CLI is unavailable.
func NativeCLIMissingMessage(
	providerName string,
	provider aghconfig.ProviderConfig,
	nativeCLI *NativeCLIStatus,
) string {
	if nativeCLI == nil || nativeCLI.Command == "" {
		return "Provider native CLI command is not configured."
	}
	if loginCommand := strings.TrimSpace(provider.AuthLoginCmd); loginCommand != "" {
		return fmt.Sprintf(
			"Native CLI %q was not found on PATH; install it, then run %q.",
			nativeCLI.Command,
			loginCommand,
		)
	}
	return fmt.Sprintf(
		"Native CLI %q was not found on PATH; install it or update providers.%s.command.",
		nativeCLI.Command,
		providerName,
	)
}

// NativeCLIReadyMessage explains the provider-owned login boundary when the CLI is present.
func NativeCLIReadyMessage(
	providerName string,
	provider aghconfig.ProviderConfig,
	nativeCLI *NativeCLIStatus,
) string {
	if nativeCLI == nil || nativeCLI.Command == "" {
		return "Provider owns authentication through its native CLI login state."
	}
	if loginCommand := strings.TrimSpace(provider.AuthLoginCmd); loginCommand != "" {
		return fmt.Sprintf(
			"Native CLI %q is present; AGH does not manage this login state. Run %q if authentication is required.",
			nativeCLI.Command,
			loginCommand,
		)
	}
	return fmt.Sprintf(
		"Native CLI %q is present; AGH does not manage this login state. "+
			"Use the provider's own login command if authentication is required, "+
			"or set providers.%s.auth_login_command.",
		nativeCLI.Command,
		providerName,
	)
}

// NativeCLIAuthProblemMessage explains how to recover from a failed native auth status probe.
func NativeCLIAuthProblemMessage(provider aghconfig.ProviderConfig) string {
	if loginCommand := strings.TrimSpace(provider.AuthLoginCmd); loginCommand != "" {
		return fmt.Sprintf("Provider status command reported an auth problem; run %q.", loginCommand)
	}
	return "Provider status command reported an auth problem; " +
		"use the provider's native login command or set auth_login_command."
}

// NativeCLILoginCommandMessage explains that AGH prints native login commands instead of running them.
func NativeCLILoginCommandMessage(providerName string, operatorCommand string) string {
	if operatorCommand != "" {
		return fmt.Sprintf(
			"AGH does not execute native provider login flows. Run %q in an interactive terminal.",
			operatorCommand,
		)
	}
	return fmt.Sprintf(
		"AGH does not manage provider %q login state. Use the provider's native login command, "+
			"or set providers.%s.auth_login_command.",
		providerName,
		providerName,
	)
}

// CommandEnv returns the provider-auth command environment used by CLI probes and settings diagnostics.
func CommandEnv(
	homePaths aghconfig.HomePaths,
	providerName string,
	provider aghconfig.ProviderConfig,
	environ []string,
) ([]string, error) {
	env := procutil.FilteredDaemonEnv(environ)
	if provider.EffectiveEnvPolicy() == aghconfig.ProviderEnvPolicyIsolated {
		env = procutil.IsolatedDaemonEnv(environ)
	}
	env = providerenv.SetEnvValue(env, "AGH_PROVIDER", strings.TrimSpace(providerName))
	env = providerenv.SetEnvValue(env, "AGH_PROVIDER_HARNESS", string(provider.EffectiveHarness()))
	env = providerenv.SetEnvValue(env, "AGH_PROVIDER_AUTH_MODE", string(provider.EffectiveAuthMode()))
	env = providerenv.SetEnvValue(env, "AGH_PROVIDER_ENV_POLICY", string(provider.EffectiveEnvPolicy()))
	env = providerenv.SetEnvValue(env, "AGH_PROVIDER_HOME_POLICY", string(provider.EffectiveHomePolicy()))
	var err error
	env, err = providerenv.ApplyHomePolicy(homePaths, providerName, provider.EffectiveHomePolicy(), env)
	if err != nil {
		return nil, err
	}
	if provider.EffectiveHarness() != aghconfig.ProviderHarnessPiACP ||
		provider.EffectiveAuthMode() != aghconfig.ProviderAuthModeNativeCLI {
		return env, nil
	}
	return providerenv.ApplyPiAgentDirPolicy(homePaths, providerName, provider.EffectiveHomePolicy(), env)
}

// NativeCLILoginEnv returns only the env assignments operators need for native CLI login commands.
func NativeCLILoginEnv(
	homePaths aghconfig.HomePaths,
	providerName string,
	provider aghconfig.ProviderConfig,
	_ []string,
) ([]string, error) {
	if provider.EffectiveHomePolicy() != aghconfig.ProviderHomePolicyIsolated {
		return nil, nil
	}
	env, err := providerenv.ResolveHomeEnv(
		homePaths,
		providerName,
		provider.EffectiveHomePolicy(),
		nil,
	)
	if err != nil {
		return nil, err
	}
	if provider.EffectiveHarness() == aghconfig.ProviderHarnessPiACP &&
		provider.EffectiveAuthMode() == aghconfig.ProviderAuthModeNativeCLI {
		env, err = providerenv.ResolvePiAgentDirEnv(
			homePaths,
			providerName,
			provider.EffectiveHomePolicy(),
			env,
		)
		if err != nil {
			return nil, err
		}
	}
	return nativeCLIHomeEnv(env), nil
}

func nativeCLIHomeEnv(env []string) []string {
	keys := []string{
		"PROVIDER_HOME",
		"HOME",
		"XDG_CONFIG_HOME",
		"XDG_DATA_HOME",
		"XDG_CACHE_HOME",
		"CLAUDE_CONFIG_DIR",
		"CODEX_HOME",
		"PROVIDER_CODEX_HOME",
		"OPENCODE_CONFIG_DIR",
		"PI_CODING_AGENT_DIR",
	}
	selected := make([]string, 0, len(keys))
	for _, key := range keys {
		if value, ok := envValue(env, key); ok {
			selected = append(selected, key+"="+value)
		}
	}
	return selected
}

func envValue(env []string, key string) (string, bool) {
	prefix := key + "="
	for _, entry := range env {
		if value, ok := strings.CutPrefix(entry, prefix); ok {
			return value, true
		}
	}
	return "", false
}

// OperatorLoginCommand prefixes a native login command with required env assignments.
func OperatorLoginCommand(command string, loginEnv []string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", errors.New("provider auth: native login command is required")
	}
	if len(loginEnv) == 0 {
		return command, nil
	}
	argv, err := shellquote.Split(command)
	if err != nil {
		return "", fmt.Errorf("provider auth: parse native login command: %w", err)
	}
	if len(argv) == 0 {
		return "", errors.New("provider auth: native login command is empty")
	}
	parts := make([]string, 0, 1+len(loginEnv)+len(argv))
	parts = append(parts, "env")
	parts = append(parts, loginEnv...)
	parts = append(parts, argv...)
	return shellquote.Join(parts...), nil
}
