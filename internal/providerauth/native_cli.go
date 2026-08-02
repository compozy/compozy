// Package providerauth contains provider-auth diagnostics shared by CLI and daemon settings surfaces.
package providerauth

import (
	"errors"
	"fmt"
	"os/exec"
	pathpkg "path"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/procutil"
	"github.com/compozy/compozy/internal/providerenv"
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
	provider compozyconfig.ProviderConfig,
	lookPath func(string) (string, error),
) (*NativeCLIStatus, error) {
	command, source := NativeCLICommand(provider)
	return NativeCLIStatusForCommand(command, source, lookPath)
}

// NativeCLICommand returns the command string and source used for native CLI auth diagnostics.
func NativeCLICommand(provider compozyconfig.ProviderConfig) (string, string) {
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
	parsed, err := ParseCommand(command)
	if err != nil {
		return nil, fmt.Errorf("provider auth: parse native CLI command: %w", err)
	}
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	status := &NativeCLIStatus{
		Command: parsed.Executable,
		Source:  source,
	}
	path, err := lookPath(parsed.Executable)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
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
	nativeCLI *NativeCLIStatus,
) string {
	if nativeCLI == nil || nativeCLI.Command == "" {
		return "Provider native CLI command is not configured."
	}
	return fmt.Sprintf(
		"Native CLI %q was not found on PATH; install it or update providers.%s.command.",
		nativeCLIExecutableBase(nativeCLI.Command),
		providerName,
	)
}

func nativeCLIExecutableBase(executable string) string {
	normalized := strings.ReplaceAll(strings.TrimSpace(executable), `\`, "/")
	if normalized == "" {
		return "provider"
	}
	return pathpkg.Base(normalized)
}

// CommandEnv returns the provider-auth command environment used by CLI probes and settings diagnostics.
func CommandEnv(
	homePaths compozyconfig.HomePaths,
	providerName string,
	provider compozyconfig.ProviderConfig,
	environ []string,
) ([]string, error) {
	return providerCommandEnv(homePaths, providerName, provider, environ, true)
}

// DiagnosticCommandEnv returns the provider auth environment without creating provider state.
func DiagnosticCommandEnv(
	homePaths compozyconfig.HomePaths,
	providerName string,
	provider compozyconfig.ProviderConfig,
	environ []string,
) ([]string, error) {
	return providerCommandEnv(homePaths, providerName, provider, environ, false)
}

func providerCommandEnv(
	homePaths compozyconfig.HomePaths,
	providerName string,
	provider compozyconfig.ProviderConfig,
	environ []string,
	createProviderState bool,
) ([]string, error) {
	env := procutil.FilteredDaemonEnv(environ)
	if provider.EffectiveEnvPolicy() == compozyconfig.ProviderEnvPolicyIsolated {
		env = procutil.IsolatedDaemonEnv(environ)
	}
	env = providerenv.SetEnvValue(env, "COMPOZY_PROVIDER", strings.TrimSpace(providerName))
	env = providerenv.SetEnvValue(env, "COMPOZY_PROVIDER_HARNESS", string(provider.EffectiveHarness()))
	env = providerenv.SetEnvValue(env, "COMPOZY_PROVIDER_AUTH_MODE", string(provider.EffectiveAuthMode()))
	env = providerenv.SetEnvValue(env, "COMPOZY_PROVIDER_ENV_POLICY", string(provider.EffectiveEnvPolicy()))
	env = providerenv.SetEnvValue(env, "COMPOZY_PROVIDER_HOME_POLICY", string(provider.EffectiveHomePolicy()))
	var err error
	if createProviderState {
		env, err = providerenv.ApplyHomePolicy(homePaths, providerName, provider.EffectiveHomePolicy(), env)
	} else {
		env, err = providerenv.ResolveHomeEnv(homePaths, providerName, provider.EffectiveHomePolicy(), env)
	}
	if err != nil {
		return nil, err
	}
	if provider.EffectiveHarness() != compozyconfig.ProviderHarnessPiACP ||
		provider.EffectiveAuthMode() != compozyconfig.ProviderAuthModeNativeCLI {
		return env, nil
	}
	if createProviderState {
		return providerenv.ApplyPiAgentDirPolicy(homePaths, providerName, provider.EffectiveHomePolicy(), env)
	}
	return providerenv.ResolvePiAgentDirEnv(homePaths, providerName, provider.EffectiveHomePolicy(), env)
}

// NativeCLILoginEnv returns only the env assignments operators need for native CLI login commands.
func NativeCLILoginEnv(
	homePaths compozyconfig.HomePaths,
	providerName string,
	provider compozyconfig.ProviderConfig,
	_ []string,
) ([]string, error) {
	if provider.EffectiveHomePolicy() != compozyconfig.ProviderHomePolicyIsolated {
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
	if provider.EffectiveHarness() == compozyconfig.ProviderHarnessPiACP &&
		provider.EffectiveAuthMode() == compozyconfig.ProviderAuthModeNativeCLI {
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
