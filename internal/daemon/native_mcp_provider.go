package daemon

import (
	"context"

	mcppkg "github.com/compozy/compozy/internal/mcp"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (d *Daemon) newDaemonMCPToolProvider(
	state *bootState,
) (*toolspkg.MCPProvider, toolspkg.MCPAuthStatusProvider, error) {
	if state == nil {
		return nil, nil, nil
	}
	resolver := newDaemonMCPServerResolver(state)
	var options []mcppkg.CallExecutorOption
	if d != nil && d.getenv != nil {
		options = append(options, mcppkg.WithSecretLookup(d.getenv))
	}
	if state.providerVault != nil {
		options = append(options, mcppkg.WithSecretResolver(state.providerVault))
	}
	if state.mcpRuntimeHealth == nil {
		state.mcpRuntimeHealth = mcppkg.NewRuntimeHealthRegistry()
	}
	options = append(options, mcppkg.WithRuntimeHealthRegistry(state.mcpRuntimeHealth))
	if store, ok := state.registry.(mcpauth.TokenStore); ok {
		options = append(
			options,
			mcppkg.WithTokenStore(store),
			mcppkg.WithAuthMutationGeneration(state.mcpAuthGeneration),
		)
	}
	executor, err := mcppkg.NewMCPCallExecutor(resolver, options...)
	if err != nil {
		return nil, nil, err
	}
	provider, err := toolspkg.NewMCPProvider(
		toolspkg.MCPSourceListerFunc(func(ctx context.Context) ([]toolspkg.SourceRef, error) {
			return daemonMCPSources(ctx, state)
		}),
		executor,
		executor,
		toolspkg.WithMCPDeadEntityService(state.deadEntities),
	)
	if err != nil {
		return nil, nil, err
	}
	return provider, executor, nil
}
