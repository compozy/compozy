// Package providers owns provider authentication classification and probes.
package providers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/providerauth"
	"github.com/compozy/compozy/internal/subprocess"
	"github.com/compozy/compozy/internal/vault"
)

// VaultRefResolver resolves redacted provider credential metadata.
type VaultRefResolver interface {
	GetMetadata(ctx context.Context, ref string) (vault.Metadata, error)
}

// ProviderAuthCommandResolver resolves a command against its exact child environment.
type ProviderAuthCommandResolver func(context.Context, string, []string, string) (string, error)

// DefaultProviderAuthCommandResolver resolves against the final child process environment.
func DefaultProviderAuthCommandResolver(
	_ context.Context,
	command string,
	env []string,
	dir string,
) (string, error) {
	return subprocess.ResolveExecutable(command, env, dir)
}

// ProbeEnv supplies process, env, and vault access to provider auth probes.
type ProbeEnv struct {
	ProviderName   string
	HomePaths      compozyconfig.HomePaths
	PreStartScope  PreStartScope
	LookPath       func(string) (string, error)
	LookupEnv      func(string) (string, bool)
	Vault          VaultRefResolver
	CommandEnv     []string
	ResolveCommand ProviderAuthCommandResolver
	// CommandDir is the final working directory shared by local provider probes
	// and the provider process they authorize.
	CommandDir string
	RunCommand ProviderAuthCommandRunner

	authStatusCommandResolution *authStatusCommandResolution
	launchExecutableResolution  *subprocess.ExecutableResolution
}

type authStatusCommandResolutionState uint8

const (
	authStatusCommandResolutionResolved authStatusCommandResolutionState = iota + 1
	authStatusCommandResolutionNotFound
	authStatusCommandResolutionFailed
)

type authStatusCommandResolution struct {
	state   authStatusCommandResolutionState
	command string
	dir     string
	env     []string
	spec    ProviderAuthCommandSpec
	cause   error
}

// PreStartScope identifies the non-secret final runtime that owns one
// pre-start probe result. A missing required field makes a probe non-cacheable.
//
// The values are supplied by the daemon/session composition boundary. HomeIdentity
// may be the final launch HOME path, which is a non-secret scope identity; no
// other CommandEnv value, command text, credential, or secret reference belongs
// in this scope.
type PreStartScope struct {
	WorkspaceID       string
	ProfileID         string
	HomeIdentity      string
	SandboxID         string
	SandboxBackend    string
	SandboxProfile    string
	SandboxInstanceID string
}

// CredentialStatus reports one provider launch credential slot readiness.
type CredentialStatus struct {
	Name      string
	TargetEnv string
	SecretRef string
	Kind      string
	Required  bool
	Present   bool
	Source    string
}

// Normalize fills safe defaults without mutating the caller's env.
func (e *ProbeEnv) Normalize() ProbeEnv {
	if e == nil {
		return ProbeEnv{
			ResolveCommand: func(_ context.Context, command string, _ []string, _ string) (string, error) {
				return exec.LookPath(command)
			},
			LookupEnv:  os.LookupEnv,
			RunCommand: DefaultProviderAuthCommandRunner,
		}
	}
	normalized := *e
	normalized.CommandEnv = append([]string(nil), e.CommandEnv...)
	normalized.authStatusCommandResolution = e.authStatusCommandResolution.clone()
	normalized.launchExecutableResolution = cloneExecutableResolution(e.launchExecutableResolution)
	if normalized.LookPath == nil {
		environment := append([]string(nil), normalized.CommandEnv...)
		dir := normalized.CommandDir
		resolver := normalized.ResolveCommand
		if resolver == nil {
			resolver = DefaultProviderAuthCommandResolver
		}
		normalized.LookPath = func(command string) (string, error) {
			return resolver(context.Background(), command, environment, dir)
		}
	}
	if normalized.LookupEnv == nil {
		normalized.LookupEnv = os.LookupEnv
	}
	if normalized.RunCommand == nil {
		normalized.RunCommand = DefaultProviderAuthCommandRunner
	}
	normalized.ProviderName = strings.TrimSpace(normalized.ProviderName)
	return normalized
}

// WithLaunchExecutableResolution returns a copy carrying one terminal launch identity.
func (e *ProbeEnv) WithLaunchExecutableResolution(
	resolution subprocess.ExecutableResolution,
) ProbeEnv {
	next := *e
	next.CommandEnv = append([]string(nil), e.CommandEnv...)
	next.launchExecutableResolution = cloneExecutableResolution(&resolution)
	return next
}

func cloneExecutableResolution(
	resolution *subprocess.ExecutableResolution,
) *subprocess.ExecutableResolution {
	if resolution == nil {
		return nil
	}
	clone := resolution.Clone()
	return &clone
}

func (e *ProbeEnv) launchResolution() subprocess.ExecutableResolution {
	if e.launchExecutableResolution == nil {
		return subprocess.ExecutableResolution{}
	}
	return *e.launchExecutableResolution
}

// PrepareAuthStatusCommand resolves and pins one provider auth-status command identity.
func PrepareAuthStatusCommand(
	ctx context.Context,
	provider compozyconfig.ProviderConfig,
	env *ProbeEnv,
) (ProviderAuthCommandSpec, error) {
	if ctx == nil {
		return ProviderAuthCommandSpec{}, errors.New("providers: auth status command context is required")
	}
	if env == nil {
		return ProviderAuthCommandSpec{}, errors.New("providers: provider auth probe env is required")
	}

	normalized := env.Normalize()
	*env = normalized
	command := strings.TrimSpace(provider.AuthStatusCmd)
	if command == "" {
		return ProviderAuthCommandSpec{}, nil
	}
	if env.authStatusCommandResolution != nil {
		return env.authStatusCommandResolution.consume(command, env)
	}

	resolveExecutable := func(command string, _ []string, _ string) (string, error) {
		return env.LookPath(command)
	}
	if env.ResolveCommand != nil {
		resolveExecutable = func(command string, environment []string, dir string) (string, error) {
			return env.ResolveCommand(ctx, command, environment, dir)
		}
	}
	prepared, err := resolveProviderAuthCommandSpec(ProviderAuthCommandSpec{
		Command: command,
		Dir:     env.CommandDir,
		Env:     append([]string(nil), env.CommandEnv...),
		Timeout: DefaultProviderAuthCommandTimeout,
		NoTTY:   true,
	}, resolveExecutable)
	resolution := newAuthStatusCommandResolution(command, env, prepared, err)
	env.authStatusCommandResolution = resolution
	return resolution.consume(command, env)
}

func newAuthStatusCommandResolution(
	command string,
	env *ProbeEnv,
	prepared ProviderAuthCommandSpec,
	cause error,
) *authStatusCommandResolution {
	resolution := &authStatusCommandResolution{
		command: command,
		dir:     env.CommandDir,
		env:     append([]string(nil), env.CommandEnv...),
	}
	if cause == nil {
		resolution.state = authStatusCommandResolutionResolved
		resolution.spec = cloneProviderAuthCommandSpec(prepared)
		return resolution
	}
	resolution.cause = cause
	if isProviderAuthCommandNotFound(cause) {
		resolution.state = authStatusCommandResolutionNotFound
		return resolution
	}
	resolution.state = authStatusCommandResolutionFailed
	return resolution
}

func (r *authStatusCommandResolution) consume(
	command string,
	env *ProbeEnv,
) (ProviderAuthCommandSpec, error) {
	if r == nil {
		return ProviderAuthCommandSpec{}, errors.New("providers: auth status command resolution is required")
	}
	if r.command != command {
		return ProviderAuthCommandSpec{}, errors.New(
			"providers: auth status command resolution does not match provider config",
		)
	}
	if r.dir != env.CommandDir || !slices.Equal(r.env, env.CommandEnv) {
		return ProviderAuthCommandSpec{}, errors.New(
			"providers: auth status command resolution does not match final provider runtime",
		)
	}
	switch r.state {
	case authStatusCommandResolutionResolved:
		return cloneProviderAuthCommandSpec(r.spec), nil
	case authStatusCommandResolutionNotFound, authStatusCommandResolutionFailed:
		if r.cause == nil {
			return ProviderAuthCommandSpec{}, errors.New(
				"providers: auth status command failure cause is required",
			)
		}
		return ProviderAuthCommandSpec{}, r.cause
	default:
		return ProviderAuthCommandSpec{}, errors.New(
			"providers: auth status command resolution state is invalid",
		)
	}
}

func (r *authStatusCommandResolution) clone() *authStatusCommandResolution {
	if r == nil {
		return nil
	}
	clone := *r
	clone.env = append([]string(nil), r.env...)
	clone.spec = cloneProviderAuthCommandSpec(r.spec)
	return &clone
}

func cloneProviderAuthCommandSpec(spec ProviderAuthCommandSpec) ProviderAuthCommandSpec {
	clone := spec
	clone.Args = append([]string(nil), spec.Args...)
	clone.Env = append([]string(nil), spec.Env...)
	return clone
}

func isProviderAuthCommandNotFound(err error) bool {
	return errors.Is(err, exec.ErrNotFound)
}

// NativeCLIStatus resolves the CLI binary used by a native provider-auth probe.
func NativeCLIStatus(
	ctx context.Context,
	provider compozyconfig.ProviderConfig,
	env *ProbeEnv,
) (*providerauth.NativeCLIStatus, error) {
	command, source := providerauth.NativeCLICommand(provider)
	if source == providerauth.NativeCLISourceAuthStatus && env != nil {
		prepared, err := PrepareAuthStatusCommand(ctx, provider, env)
		if err != nil {
			return providerauth.NativeCLIStatusForCommand(
				command,
				source,
				func(string) (string, error) { return "", err },
			)
		}
		if strings.TrimSpace(prepared.Executable) != "" {
			return nativeCLIStatusForResolvedExecutable(
				command,
				source,
				prepared.Executable,
			)
		}
	}
	normalized := env.Normalize()
	return providerauth.NativeCLIStatusForCommand(command, source, normalized.LookPath)
}

// LaunchCommandStatus resolves the first token of the launch command used by a session start.
func LaunchCommandStatus(
	ctx context.Context,
	provider compozyconfig.ProviderConfig,
	env *ProbeEnv,
) (*providerauth.NativeCLIStatus, error) {
	normalized := env.Normalize()
	launchResolution := normalized.launchResolution()
	if launchResolution.Valid() {
		resolved, err := launchResolution.Consume(
			strings.TrimSpace(provider.Command),
			normalized.CommandEnv,
			normalized.CommandDir,
		)
		if err != nil {
			if launchResolution.State() != subprocess.ExecutableResolutionNotFound {
				return nil, err
			}
			return providerauth.NativeCLIStatusForCommand(
				strings.TrimSpace(provider.Command),
				providerauth.NativeCLISourceCommand,
				func(string) (string, error) { return "", err },
			)
		}
		return nativeCLIStatusForResolvedExecutable(
			strings.TrimSpace(provider.Command),
			providerauth.NativeCLISourceCommand,
			resolved,
		)
	}
	lookPath := normalized.LookPath
	if normalized.ResolveCommand != nil {
		lookPath = func(command string) (string, error) {
			return normalized.ResolveCommand(ctx, command, normalized.CommandEnv, normalized.CommandDir)
		}
	}
	return providerauth.NativeCLIStatusForCommand(
		strings.TrimSpace(provider.Command),
		providerauth.NativeCLISourceCommand,
		lookPath,
	)
}

func nativeCLIStatusForResolvedExecutable(
	command string,
	source string,
	resolved string,
) (*providerauth.NativeCLIStatus, error) {
	resolved = strings.TrimSpace(resolved)
	if !filepath.IsAbs(resolved) {
		return nil, fmt.Errorf("provider auth resolved executable %q must be an absolute path", resolved)
	}
	return providerauth.NativeCLIStatusForCommand(
		command,
		source,
		func(string) (string, error) { return resolved, nil },
	)
}

// CredentialStatuses resolves configured credential slots without reading plaintext secrets.
func CredentialStatuses(
	ctx context.Context,
	provider compozyconfig.ProviderConfig,
	env *ProbeEnv,
) ([]CredentialStatus, error) {
	normalized := env.Normalize()
	slots := provider.EffectiveCredentialSlots()
	if len(slots) == 0 {
		return nil, nil
	}
	statuses := make([]CredentialStatus, 0, len(slots))
	for _, slot := range slots {
		status, err := credentialStatus(ctx, slot, &normalized)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func credentialStatus(
	ctx context.Context,
	slot compozyconfig.ProviderCredentialSlot,
	env *ProbeEnv,
) (CredentialStatus, error) {
	secretRef := vault.NormalizeRef(slot.SecretRef)
	status := CredentialStatus{
		Name:      strings.TrimSpace(slot.Name),
		TargetEnv: strings.TrimSpace(slot.TargetEnv),
		SecretRef: secretRef,
		Kind:      strings.TrimSpace(slot.Kind),
		Required:  slot.Required,
	}
	switch {
	case vault.IsEnvRef(secretRef):
		status.Source = "env"
		envName, err := vault.EnvNameFromRef(secretRef)
		if err != nil {
			return CredentialStatus{}, err
		}
		value, ok := env.LookupEnv(envName)
		status.Present = ok && strings.TrimSpace(value) != ""
		return status, nil
	case vault.IsSecretRef(secretRef):
		status.Source = "vault"
		if env.Vault == nil {
			return status, nil
		}
		metadata, err := env.Vault.GetMetadata(ctx, secretRef)
		if err != nil {
			if errors.Is(err, vault.ErrSecretNotFound) || errors.Is(err, vault.ErrMissingSecret) {
				return status, nil
			}
			return CredentialStatus{}, fmt.Errorf("resolve provider credential metadata: %w", err)
		}
		status.Present = metadata.Present
		return status, nil
	default:
		status.Source = "unsupported"
		return status, nil
	}
}

func firstMissingRequiredCredential(statuses []CredentialStatus) (CredentialStatus, bool) {
	for _, status := range statuses {
		if status.Required && !status.Present {
			return status, true
		}
	}
	return CredentialStatus{}, false
}
