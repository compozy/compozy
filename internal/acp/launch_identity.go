package acp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	authproviders "github.com/compozy/compozy/internal/providers"
	"github.com/compozy/compozy/internal/subprocess"
	shellquote "github.com/kballard/go-shellquote"
)

type preparedLaunchIdentity struct {
	prepared   bool
	spec       LaunchSpec
	resolution subprocess.ExecutableResolution
}

func clonePreparedLaunchIdentity(identity *preparedLaunchIdentity) *preparedLaunchIdentity {
	if identity == nil {
		return nil
	}
	clone := *identity
	clone.spec = cloneLaunchSpec(identity.spec)
	clone.resolution = identity.resolution.Clone()
	return &clone
}

func startOptsPreparedLaunchIdentity(opts StartOpts) preparedLaunchIdentity {
	if opts.launchIdentity == nil {
		return preparedLaunchIdentity{}
	}
	return *opts.launchIdentity
}

func (d *Driver) resolvedAgentLaunch(
	ctx context.Context,
	opts StartOpts,
) (StartOpts, string, []string, error) {
	next := opts
	if next.launchIdentity == nil || !next.launchIdentity.prepared {
		var err error
		next, err = d.prepareLaunchIdentity(ctx, next)
		if err != nil {
			return StartOpts{}, "", nil, err
		}
	}
	identity := startOptsPreparedLaunchIdentity(next)
	if identity.resolution.Valid() {
		command, err := identity.resolution.Consume(
			identity.spec.Command,
			identity.spec.Env,
			identity.spec.Cwd,
		)
		if err != nil {
			return StartOpts{}, "", nil, err
		}
		return next, command, append([]string(nil), identity.spec.Args...), nil
	}
	command, args, err := parseCommandString(next.Command)
	return next, command, args, err
}

func (d *Driver) prepareLaunchIdentity(
	ctx context.Context,
	opts StartOpts,
) (StartOpts, error) {
	next := opts
	next.Launcher = d.launcherForStart(opts)
	if next.launchIdentity != nil && next.launchIdentity.prepared {
		return next, nil
	}
	baseSpec := launchSpecFromStartOpts(next)
	preparer, ok := next.Launcher.(LaunchPreparer)
	if !ok {
		next.launchIdentity = &preparedLaunchIdentity{prepared: true, spec: baseSpec}
		next = applyProviderLaunchIdentity(next)
		return next, nil
	}

	spec, err := preparer.PrepareLaunch(ctx, baseSpec)
	next = applyLaunchSpecToStartOpts(next, spec)
	next.launchIdentity = &preparedLaunchIdentity{
		prepared: true,
		spec:     cloneLaunchSpec(spec),
		resolution: subprocess.NewExecutableResolution(
			spec.Command,
			spec.Env,
			spec.Cwd,
			spec.ResolvedExecutable,
			err,
		),
	}
	next = applyProviderLaunchIdentity(next)
	if err != nil {
		return next, fmt.Errorf(
			"acp: start agent %q subprocess %s in %q: %w",
			strings.TrimSpace(opts.AgentName),
			launchCommandIdentity(opts.Command),
			strings.TrimSpace(opts.Cwd),
			err,
		)
	}
	return next, nil
}

// launchCommandIdentity names a launch command in errors and logs without its
// raw text: a route command can carry private NAME=value assignments and
// account arguments (fallback-account ADR-001/002). It keeps the executable
// name and the shared sha256 fingerprint.
func launchCommandIdentity(command string) string {
	fingerprint := compozyconfig.CommandFingerprint(strings.TrimSpace(command))
	parsed, err := compozyconfig.ParseLaunchCommand(command)
	if err != nil || parsed.Executable == "" {
		return fmt.Sprintf("(command %s)", fingerprint)
	}
	return fmt.Sprintf("%q (command %s)", parsed.Executable, fingerprint)
}

func launchSpecFromStartOpts(opts StartOpts) LaunchSpec {
	return LaunchSpec{
		Command:        opts.Command,
		Cwd:            opts.Cwd,
		AdditionalDirs: append([]string(nil), opts.AdditionalDirs...),
		Env:            append([]string(nil), opts.Env...),
	}
}

func applyLaunchSpecToStartOpts(opts StartOpts, spec LaunchSpec) StartOpts {
	next := opts
	next.Command = spec.Command
	next.Cwd = spec.Cwd
	next.AdditionalDirs = append([]string(nil), spec.AdditionalDirs...)
	next.Env = append([]string(nil), spec.Env...)
	return next
}

func cloneLaunchSpec(spec LaunchSpec) LaunchSpec {
	clone := spec
	clone.Args = append([]string(nil), spec.Args...)
	clone.AdditionalDirs = append([]string(nil), spec.AdditionalDirs...)
	clone.Env = append([]string(nil), spec.Env...)
	return clone
}

func (d *Driver) launcherForStart(opts StartOpts) Launcher {
	if opts.Launcher != nil {
		return opts.Launcher
	}
	if d.launcher != nil {
		return d.launcher
	}
	return newLocalLauncher(d.logger, d.stopTimeout)
}

func (l *localLauncher) PrepareLaunch(
	ctx context.Context,
	spec LaunchSpec,
) (LaunchSpec, error) {
	if ctx == nil {
		return spec, errors.New("acp: prepare launch context is required")
	}
	if err := ctx.Err(); err != nil {
		return spec, err
	}

	next := spec
	next.Env = DaemonMatchedEnv(spec.Env)
	if next.ResolvedExecutable != "" {
		next.Args = append([]string(nil), next.Args...)
		return next, nil
	}

	// Leading NAME=value tokens (a route's account, e.g. CLAUDE_CONFIG_DIR=… claude --acp)
	// are private environment for this process, never the executable.
	parsed, err := compozyconfig.ParseLaunchCommand(next.Command)
	if err != nil {
		return next, fmt.Errorf("acp: %w", err)
	}
	for _, assignment := range parsed.Environment {
		key, value, _ := strings.Cut(assignment, "=")
		next.Env = setEnvValue(next.Env, key, value)
	}
	command, args := parsed.Executable, parsed.Args
	next.Args = append([]string(nil), args...)
	resolved, err := subprocess.ResolveExecutable(command, next.Env, next.Cwd)
	if err != nil {
		return next, err
	}
	if err := ctx.Err(); err != nil {
		return next, err
	}
	next.ResolvedExecutable = resolved
	next.Command = effectiveCommandString(resolved, args)
	return next, nil
}

func effectiveCommandString(executable string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, executable)
	parts = append(parts, args...)
	return shellquote.Join(parts...)
}

func applyProviderLaunchIdentity(opts StartOpts) StartOpts {
	next := opts
	if next.ProviderConfig != nil {
		provider := *next.ProviderConfig
		provider.Command = next.Command
		next.ProviderConfig = &provider
	}
	if next.ProviderAuthEnv == nil {
		return next
	}

	probe := *next.ProviderAuthEnv
	commandEnv := append([]string(nil), next.Env...)
	probe.CommandEnv = commandEnv
	probe.CommandDir = next.Cwd
	probe.LookupEnv = subprocess.LookupEnvFunc(commandEnv)
	if probe.ResolveCommand == nil {
		probe.ResolveCommand = authproviders.DefaultProviderAuthCommandResolver
	}
	probe.LookPath = nil
	identity := startOptsPreparedLaunchIdentity(next)
	if identity.resolution.Valid() {
		probe = probe.WithLaunchExecutableResolution(identity.resolution)
	}
	next.ProviderAuthEnv = &probe
	return next
}
