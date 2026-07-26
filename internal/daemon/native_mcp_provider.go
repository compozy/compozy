package daemon

import (
	"context"

	mcppkg "github.com/compozy/agh/internal/mcp"
	mcpauth "github.com/compozy/agh/internal/mcp/auth"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func (d *Daemon) newDaemonMCPToolProvider(
	state *bootState,
) (toolspkg.Provider, toolspkg.MCPAuthStatusProvider, error) {
	if state == nil {
		return nil, nil, nil
	}
	resolver := newDaemonMCPServerResolver(state)
	options := []mcppkg.CallExecutorOption{
		mcppkg.WithTimeout(state.cfg.Observability.AgentProbeTimeoutOrDefault()),
	}
	if d != nil && d.getenv != nil {
		options = append(options, mcppkg.WithSecretLookup(d.getenv))
	}
	if state.providerVault != nil {
		options = append(options, mcppkg.WithSecretResolver(state.providerVault))
	}
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
		toolspkg.MCPSourceListerFunc(func(context.Context) ([]toolspkg.SourceRef, error) {
			return daemonMCPSources(state), nil
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
