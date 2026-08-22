package session

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/providerexec"
	"github.com/compozy/compozy/internal/sandbox"
	"github.com/compozy/compozy/internal/subprocess"
)

func resolveProviderNativeCLI(
	ctx context.Context,
	resolved compozyconfig.ResolvedAgent,
	opts acp.StartOpts,
) (acp.StartOpts, error) {
	provider := providerConfigFromResolvedAgent(resolved)
	provider.Command = opts.Command
	strategy := providerexec.StrategyFor(provider)
	if strategy.Kind != providerexec.StrategyNativeCLIBridge {
		return opts, nil
	}

	executable, err := resolveProviderNativeCLIPath(ctx, strategy.NativeCLI.Command, opts)
	if err != nil {
		return acp.StartOpts{}, fmt.Errorf(
			"session: resolve native CLI for provider %q: %w",
			strings.TrimSpace(resolved.Provider),
			err,
		)
	}
	next := opts
	next.Env = setSessionStartEnvValue(next.Env, strategy.NativeCLI.BridgeEnvKey, executable)
	return next, nil
}

func resolveProviderNativeCLIPath(ctx context.Context, command string, opts acp.StartOpts) (string, error) {
	if provider, ok := opts.Launcher.(sandbox.CommandRuntimeProvider); ok && provider.CommandRuntime() != nil {
		return provider.CommandRuntime().Resolve(
			ctx,
			command,
			append([]string(nil), opts.Env...),
			opts.Cwd,
		)
	}
	return subprocess.ResolveExecutable(command, append([]string(nil), opts.Env...), opts.Cwd)
}
