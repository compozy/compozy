package daemon

import (
	"context"
	"errors"
	"fmt"

	toolspkg "github.com/compozy/agh/internal/tools"
	builtintools "github.com/compozy/agh/internal/tools/builtin"
)

func (d *Daemon) bootToolRegistry(
	ctx context.Context,
	state *bootState,
	cleanup *bootCleanup,
) error {
	if state == nil {
		return errors.New("daemon: tool registry state is required")
	}
	if state.mcpServerCatalog == nil {
		state.mcpServerCatalog = newResourceCatalog(cloneDaemonMCPServer)
	}
	if err := d.bootToolArtifacts(ctx, state, cleanup); err != nil {
		return err
	}
	approvalTokens, approvalBridge, err := d.bootToolApprovalServices(state)
	if err != nil {
		return err
	}
	var registry *toolspkg.RuntimeRegistry
	var mcpAuth toolspkg.MCPAuthStatusProvider
	deps := d.nativeToolsDeps(state, func() toolspkg.Registry {
		return registry
	})
	deps.MCPAuth = func() toolspkg.MCPAuthStatusProvider {
		return mcpAuth
	}
	provider, err := newDaemonNativeProvider(&deps)
	if err != nil {
		return fmt.Errorf("daemon: create native tool provider: %w", err)
	}
	toolsets, err := builtintools.ToolsetCatalog()
	if err != nil {
		return fmt.Errorf("daemon: build native toolset catalog: %w", err)
	}
	policyResolver, err := newNativeToolPolicyResolverForBoot(state)
	if err != nil {
		return fmt.Errorf("daemon: build native tool policy resolver: %w", err)
	}
	providers := []toolspkg.Provider{provider}
	extensionProvider, err := newDaemonExtensionToolProvider(state)
	if err != nil {
		return fmt.Errorf("daemon: create extension tool provider: %w", err)
	}
	if extensionProvider != nil {
		providers = append(providers, extensionProvider)
	}
	mcpProvider, mcpAuthProvider, err := d.newDaemonMCPToolProvider(state)
	if err != nil {
		return fmt.Errorf("daemon: create mcp tool provider: %w", err)
	}
	mcpAuth = mcpAuthProvider
	if mcpProvider != nil {
		providers = append(providers, mcpProvider)
	}
	registryOptions := []toolspkg.RegistryOption{
		toolspkg.WithProviders(providers...),
		toolspkg.WithPolicyInputResolver(policyResolver, toolsets),
		toolspkg.WithApprovalBridge(approvalBridge),
		toolspkg.WithDefaultMaxResultBytes(state.cfg.Tools.DefaultMaxResultBytes),
		toolspkg.WithResultProcessor(toolspkg.NewResultProcessor(
			state.cfg.Tools.DefaultMaxResultBytes,
			state.toolArtifacts,
		)),
	}
	registryOptions = appendToolEventSinkOption(registryOptions, state.registry, d.now)
	registry, err = toolspkg.NewRegistry(registryOptions...)
	if err != nil {
		return fmt.Errorf("daemon: create tool registry: %w", err)
	}
	state.toolRegistry = registry
	state.toolsets = registry
	state.toolApprovals = approvalTokens
	state.deps.ToolRegistry = registry
	state.deps.Toolsets = registry
	state.deps.ToolApprovals = approvalTokens
	return nil
}
