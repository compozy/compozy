package providers

import (
	"context"
	"os"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/providerauth"
)

// AuthStatusProbe is one auth_status_command run and its classification.
type AuthStatusProbe struct {
	Classification Classification
	Result         ProviderAuthCommandResult
	StartedAt      time.Time
	Ran            bool // false when the command could not be prepared; not a live verdict then
}

// RunAuthStatusProbe prepares, runs, and classifies auth_status_command without recording a verdict.
func RunAuthStatusProbe(
	ctx context.Context,
	provider compozyconfig.ProviderConfig,
	env *ProbeEnv,
	now func() time.Time,
) (AuthStatusProbe, error) {
	commandSpec, err := PrepareAuthStatusCommand(ctx, provider, env)
	if err != nil {
		return AuthStatusProbe{Classification: ClassifyError(err)}, nil
	}
	startedAt := now()
	result, err := env.Normalize().RunCommand(ctx, commandSpec)
	if err != nil {
		return AuthStatusProbe{}, err
	}
	classification := ClassifyProbeResultContext(ctx, provider, ProbeOutcome{
		ExitCode: result.ExitCode,
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
	}, env)
	return AuthStatusProbe{Classification: classification, Result: result, StartedAt: startedAt, Ran: true}, nil
}

// DaemonProbeEnvOptions carries the daemon-scoped dependencies of a provider sign-in probe.
type DaemonProbeEnvOptions struct {
	HomePaths      compozyconfig.HomePaths
	Vault          VaultRefResolver
	ResolveCommand ProviderAuthCommandResolver
	RunCommand     ProviderAuthCommandRunner
}

// DaemonProbeEnv builds the daemon-side environment for one provider's sign-in probe.
func DaemonProbeEnv(
	options DaemonProbeEnvOptions,
	providerName string,
	provider compozyconfig.ProviderConfig,
) (ProbeEnv, error) {
	commandEnv, err := providerauth.CommandEnv(options.HomePaths, providerName, provider, os.Environ())
	if err != nil {
		return ProbeEnv{}, err
	}
	// ACP probes need a working directory even when the daemon's launch directory was removed.
	commandDir, err := os.Getwd()
	info, statErr := os.Stat(commandDir)
	if err != nil || statErr != nil || !info.IsDir() {
		commandDir = os.TempDir()
	}
	return ProbeEnv{
		ProviderName:   providerName,
		HomePaths:      options.HomePaths,
		LookupEnv:      os.LookupEnv,
		Vault:          options.Vault,
		CommandEnv:     commandEnv,
		CommandDir:     commandDir,
		ResolveCommand: options.ResolveCommand,
		RunCommand:     options.RunCommand,
	}, nil
}
