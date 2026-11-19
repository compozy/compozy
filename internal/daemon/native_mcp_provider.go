package daemon

import (
	"context"
	"errors"

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
	if state.mcpDigestKeys == nil {
		return nil, nil, errors.New("daemon: MCP digest key provider is required")
	}
	digestKey, err := state.mcpDigestKeys.Key()
	if err != nil {
		return nil, nil, err
	}
	resolver := newDaemonMCPServerResolver(state)
	options := []mcppkg.CallExecutorOption{
		mcppkg.WithToolCatalogNotifications(),
		mcppkg.WithAuthRecovery(func(ctx context.Context, resolved *mcppkg.ResolvedServer) error {
			return recoverResolvedMCPAuthentication(ctx, state, resolved)
		}),
		mcppkg.WithDefinitionDigestKey(digestKey),
		mcppkg.WithConsentCheck(
			func(ctx context.Context, source toolspkg.SourceRef, resolved mcppkg.ResolvedServer) error {
				return requireResolvedMCPConsent(ctx, state, source, &resolved)
			},
		),
		mcppkg.WithAuthNetworkAuthorization(func(ctx context.Context, cfg mcpauth.ServerConfig) error {
			return authorizeMCPAuthNetwork(ctx, state, cfg)
		}),
	}
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
	if state.mcpStdioPool != nil {
		options = append(options, mcppkg.WithStdioSessionPool(state.mcpStdioPool))
	}
	if store, ok := state.registry.(mcpauth.TokenStore); ok {
		options = append(
			options,
			mcppkg.WithTokenStore(store),
			mcppkg.WithAuthMutationGeneration(state.mcpAuthGeneration),
			mcppkg.WithAuthRedirectURI(state.cfg.MCP.OAuth.RedirectURI),
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
		toolspkg.WithMCPBackgroundDiscovery(func() { state.toolProjectionEpoch.Advance() }, state.logger),
		toolspkg.WithMCPDeadEntityService(state.deadEntities),
		toolspkg.WithMCPConsent(func(ctx context.Context, source toolspkg.SourceRef, profileID string) (bool, error) {
			decision, err := mcpSourceConsent(
				ctx,
				state,
				toolspkg.Scope{ProfileID: profileID, WorkspaceID: source.WorkspaceID},
				source,
			)
			return decision.Admitted, err
		}),
		toolspkg.WithMCPDiscoveryBudget(
			min(state.cfg.MCP.StatusProbeTimeout, state.cfg.Tools.HostedMCP.BindNonceTTL()/2),
		),
	)
	if err != nil {
		return nil, nil, err
	}
	return provider, executor, nil
}
