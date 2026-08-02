package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	authproviders "github.com/compozy/compozy/internal/providers"
	"github.com/compozy/compozy/internal/sandbox"
)

func configureProviderCommandRuntime(
	probe *authproviders.ProbeEnv,
	launcher sandbox.Launcher,
) {
	if probe == nil {
		return
	}
	provider, ok := launcher.(sandbox.CommandRuntimeProvider)
	if !ok || provider.CommandRuntime() == nil {
		probe.LookPath = providerLookPath(probe.CommandEnv, probe.CommandDir)
		probe.ResolveCommand = authproviders.DefaultProviderAuthCommandResolver
		probe.RunCommand = authproviders.DefaultProviderAuthCommandRunner
		return
	}
	runtime := provider.CommandRuntime()
	probe.LookPath = nil
	probe.ResolveCommand = func(
		ctx context.Context,
		command string,
		env []string,
		cwd string,
	) (string, error) {
		return runtime.Resolve(ctx, command, append([]string(nil), env...), cwd)
	}
	probe.RunCommand = providerRuntimeCommandRunner(runtime)
}

func providerRuntimeCommandRunner(
	runtime sandbox.CommandRuntime,
) authproviders.ProviderAuthCommandRunner {
	return func(
		ctx context.Context,
		spec authproviders.ProviderAuthCommandSpec,
	) (authproviders.ProviderAuthCommandResult, error) {
		if ctx == nil {
			return authproviders.ProviderAuthCommandResult{}, errors.New(
				"session: provider command context is required",
			)
		}
		if runtime == nil {
			return authproviders.ProviderAuthCommandResult{}, errors.New(
				"session: provider command runtime is required",
			)
		}
		if spec.Executable == "" {
			return authproviders.ProviderAuthCommandResult{}, errors.New(
				"session: resolved provider executable is required",
			)
		}
		timeout := spec.Timeout
		if timeout <= 0 {
			timeout = authproviders.DefaultProviderAuthCommandTimeout
		}
		commandCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		startedAt := time.Now()
		result, err := runtime.Run(commandCtx, sandbox.CommandSpec{
			Executable: spec.Executable,
			Args:       append([]string(nil), spec.Args...),
			Cwd:        spec.Dir,
			Env:        append([]string(nil), spec.Env...),
		})
		if cause := context.Cause(commandCtx); cause != nil {
			return authproviders.ProviderAuthCommandResult{}, cause
		}
		if err != nil {
			return authproviders.ProviderAuthCommandResult{}, fmt.Errorf(
				"session: run provider command in sandbox: %w",
				err,
			)
		}
		duration := result.Duration
		if duration <= 0 {
			duration = time.Since(startedAt)
		}
		return authproviders.ProviderAuthCommandResult{
			ExitCode:   result.ExitCode,
			Stdout:     diagnostics.RedactAndBound(result.Stdout, 4096),
			Stderr:     diagnostics.RedactAndBound(result.Stderr, 4096),
			DurationMs: duration.Milliseconds(),
		}, nil
	}
}
