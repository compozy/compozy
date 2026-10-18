package session

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/providerexec"
	"github.com/compozy/compozy/internal/subprocess"
)

func resolveProviderNativeCLI(
	resolved compozyconfig.ResolvedAgent,
	opts acp.StartOpts,
) (acp.StartOpts, error) {
	provider := providerConfigFromResolvedAgent(resolved)
	provider.Command = opts.Command
	strategy := providerexec.StrategyFor(provider)
	if strategy.Kind != providerexec.StrategyNativeCLIBridge {
		return opts, nil
	}

	executable, err := resolveProviderNativeCLIPath(strategy.NativeCLI.Command, opts)
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

func resolveProviderNativeCLIPath(command string, opts acp.StartOpts) (string, error) {
	return subprocess.ResolveExecutable(command, append([]string(nil), opts.Env...), opts.Cwd)
}
