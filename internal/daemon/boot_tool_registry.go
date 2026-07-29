package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
	if err := d.bootToolArtifacts(ctx, state, cleanup); err != nil {
		return err
	}
	approvalTokens, approvalBridge, err := d.bootToolApprovalServices(state)
	if err != nil {
		return err
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
		toolspkg.WithWorkspaceIDResolver(func(ctx context.Context, ref string) (string, error) {
			resolved, resolveErr := state.workspaceResolver.Resolve(ctx, ref)
			if resolveErr != nil {
				return "", resolveErr
			}
			workspaceID := strings.TrimSpace(resolved.WorkspaceID)
			if workspaceID == "" {
				return "", errors.New("daemon: resolved tool workspace id is required")
			}
			return workspaceID, nil
		}),
		toolspkg.WithCallInputBinder(workspaceBinder),
		toolspkg.WithCallInputAuthorizer(workspaceBinder),
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
