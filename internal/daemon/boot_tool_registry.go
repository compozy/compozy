package daemon

import (
	"context"
	"errors"
	"fmt"

	toolspkg "github.com/compozy/compozy/internal/tools"
	builtintools "github.com/compozy/compozy/internal/tools/builtin"
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
	bindToolProjectionCatalogInvalidation(state)
	if err := d.bootToolArtifacts(ctx, state, cleanup); err != nil {
		return err
	}
	approvalTokens, approvalBridge, err := d.bootToolApprovalServices(state)
	if err != nil {
		return err
	}
	if state.terminalPermissions != nil {
		state.terminalPermissions.bind(approvalBridge)
	}
	workspaceBinder := nativeWorkspaceAccessBinderForBoot(state)
	var registry *toolspkg.RuntimeRegistry
	providers, err := d.bootToolProviders(state, func() toolspkg.Registry { return registry })
	if err != nil {
		return err
	}
	toolsets, err := builtintools.ToolsetCatalog()
	if err != nil {
		return fmt.Errorf("daemon: build native toolset catalog: %w", err)
	}
	policyResolver, err := newNativeToolPolicyResolverForBoot(state)
	if err != nil {
		return fmt.Errorf("daemon: build native tool policy resolver: %w", err)
	}
	registryOptions := []toolspkg.RegistryOption{
		toolspkg.WithProviders(providers...),
		toolspkg.WithPolicyInputResolver(policyResolver, toolsets),
		toolspkg.WithApprovalBridge(approvalBridge),
		toolspkg.WithWorkspaceAccessPolicy(state.accessPolicy),
		toolspkg.WithWorkspaceIDResolver(nativeWorkspaceIDResolver(state)),
		toolspkg.WithCallInputBinder(workspaceBinder),
		toolspkg.WithCallInputAuthorizer(workspaceBinder),
		toolspkg.WithDefaultMaxResultBytes(state.cfg.Tools.DefaultMaxResultBytes),
		toolspkg.WithResultProcessor(toolspkg.NewResultProcessor(
			state.cfg.Tools.DefaultMaxResultBytes,
			state.toolArtifacts,
		)),
	}
	if state.toolProjectionEpoch != nil {
		registryOptions = append(
			registryOptions,
			toolspkg.WithProjectionInvalidator(func() { state.toolProjectionEpoch.Advance() }),
			toolspkg.WithProjectionGeneration(state.toolProjectionEpoch.Snapshot),
		)
	}
	registryOptions = appendToolEventSinkOption(
		registryOptions,
		state.registry,
		d.now,
		optionalDaemonSessionProfileResolver(state.sessions, "daemon: tool event session profile is unavailable"),
	)
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

func nativeWorkspaceIDResolver(state *bootState) func(context.Context, string) (string, error) {
	return func(ctx context.Context, ref string) (string, error) {
		resolved, err := state.workspaceResolver.Resolve(ctx, ref)
		if err != nil {
			return "", err
		}
		return nativeResolvedRegistryWorkspaceID(&resolved)
	}
}

func optionalDaemonSessionProfileResolver(
	sessions SessionManager,
	unavailableMessage string,
) func(context.Context, string) (string, error) {
	if sessions == nil {
		return nil
	}
	return daemonSessionProfileResolver(sessions, unavailableMessage)
}
