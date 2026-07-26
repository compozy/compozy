package daemon

import (
	"context"

	"fmt"

	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/compozy/agh/internal/resources"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func (s *toolMCPSourceSyncer) syncTools(ctx context.Context, desired map[string]*desiredToolResource) (bool, error) {
	source := s.actor.Source
	current, err := s.raw.ListRaw(ctx, s.actor, resources.ResourceFilter{
		Kind:   toolspkg.ToolResourceKind,
		Source: &source,
	})
	if err != nil {
		return false, fmt.Errorf("daemon: list managed tools: %w", err)
	}

	currentByID := make(map[string]resources.RawRecord, len(current))
	for _, record := range current {
		currentByID[record.ID] = record
	}

	changed := false
	for id, desiredTool := range desired {
		if desiredTool == nil {
			continue
		}
		existing, ok := currentByID[id]
		if ok && sameManagedRawRecord(existing, desiredTool.scope, desiredTool.encoded) {
			delete(currentByID, id)
			continue
		}

		expectedVersion := int64(0)
		if ok {
			expectedVersion = existing.Version
		}
		if _, err := s.toolStore.Put(ctx, s.actor, resources.Draft[toolspkg.Tool]{
			ID:              desiredTool.id,
			Scope:           desiredTool.scope,
			ExpectedVersion: expectedVersion,
			Spec:            desiredTool.spec,
		}); err != nil {
			return false, fmt.Errorf("daemon: sync tool %q: %w", id, err)
		}
		changed = true
		delete(currentByID, id)
	}

	for _, stale := range currentByID {
		if err := s.toolStore.Delete(ctx, s.actor, stale.ID, stale.Version); err != nil {
			return false, fmt.Errorf("daemon: delete stale tool %q: %w", stale.ID, err)
		}
		changed = true
	}

	return changed, nil
}

func (s *toolMCPSourceSyncer) syncMCPServers(
	ctx context.Context,
	desired map[string]desiredMCPServerResource,
) (bool, error) {
	source := s.actor.Source
	current, err := s.raw.ListRaw(ctx, s.actor, resources.ResourceFilter{
		Kind:   aghconfig.MCPServerResourceKind,
		Source: &source,
	})
	if err != nil {
		return false, fmt.Errorf("daemon: list managed mcp servers: %w", err)
	}

	currentByID := make(map[string]resources.RawRecord, len(current))
	for _, record := range current {
		currentByID[record.ID] = record
	}

	changed := false
	for id, desiredServer := range desired {
		existing, ok := currentByID[id]
		if ok && sameManagedRawRecord(existing, desiredServer.scope, desiredServer.encoded) {
			delete(currentByID, id)
			continue
		}

		expectedVersion := int64(0)
		if ok {
			expectedVersion = existing.Version
		}
		if _, err := s.mcpStore.Put(ctx, s.actor, resources.Draft[aghconfig.MCPServer]{
			ID:              desiredServer.id,
			Scope:           desiredServer.scope,
			ExpectedVersion: expectedVersion,
			Spec:            desiredServer.spec,
		}); err != nil {
			return false, fmt.Errorf("daemon: sync mcp server %q: %w", id, err)
		}
		changed = true
		delete(currentByID, id)
	}

	for _, stale := range currentByID {
		if err := s.mcpStore.Delete(ctx, s.actor, stale.ID, stale.Version); err != nil {
			return false, fmt.Errorf("daemon: delete stale mcp server %q: %w", stale.ID, err)
		}
		changed = true
	}

	return changed, nil
}

func (d *Daemon) newToolMCPPublisher(
	state *bootState,
	registry *extensionpkg.Registry,
) (toolMCPPublisher, error) {
	publisher := toolMCPPublisher(toolMCPPublisherFunc(func(context.Context) error { return nil }))
	if state == nil {
		return publisher, nil
	}
	if state.resourceKernel == nil || state.resourceCodecs == nil {
		return publisher, nil
	}

	toolCodec, err := resources.ResolveCodec[toolspkg.Tool](state.resourceCodecs, toolspkg.ToolResourceKind)
	if err != nil {
		return nil, fmt.Errorf("daemon: resolve tool codec: %w", err)
	}
	toolStore, err := resources.NewStore(state.resourceKernel, toolCodec)
	if err != nil {
		return nil, fmt.Errorf("daemon: create tool store: %w", err)
	}

	mcpCodec, err := resources.ResolveCodec[aghconfig.MCPServer](state.resourceCodecs, aghconfig.MCPServerResourceKind)
	if err != nil {
		return nil, fmt.Errorf("daemon: resolve mcp server codec: %w", err)
	}
	mcpStore, err := resources.NewStore(state.resourceKernel, mcpCodec)
	if err != nil {
		return nil, fmt.Errorf("daemon: create mcp server store: %w", err)
	}

	return newToolMCPSourceSyncerWithConfigProvider(
		state.resourceKernel,
		toolStore,
		toolCodec,
		mcpStore,
		mcpCodec,
		toolMCPSyncActor(),
		state.logger,
		func(ctx context.Context, kind resources.ResourceKind, reason resources.ReconcileReason) error {
			if state.resourceReconcile == nil {
				return nil
			}
			return state.resourceReconcile.Trigger(ctx, kind, reason)
		},
		daemonConfigMCPDeclarationProvider(&state.cfg, state.registry, state.workspaceResolver, state.logger),
		extensionManifestToolMCPDeclarationProvider(registry, state.currentExtensionRuntime, d.getenv, state.logger),
	), nil
}
