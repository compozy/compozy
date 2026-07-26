package daemon

import (
	"context"

	"errors"

	"log/slog"

	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/resources"
	toolspkg "github.com/compozy/agh/internal/tools"
)

const (
	toolManagedIDPrefix      = "daemon.sync.tool."
	mcpServerManagedIDPrefix = "daemon.sync.mcp_server."
)

type toolMCPPublisher interface {
	Sync(context.Context) error
	SyncConfig(context.Context, *aghconfig.Config) error
}

type toolMCPPublisherFunc func(context.Context) error

func (f toolMCPPublisherFunc) Sync(ctx context.Context) error {
	if f == nil {
		return nil
	}
	return f(ctx)
}

func (f toolMCPPublisherFunc) SyncConfig(ctx context.Context, _ *aghconfig.Config) error {
	return f.Sync(ctx)
}

type toolPublicationInput struct {
	sourceKey string
	scope     resources.ResourceScope
	spec      toolspkg.Tool
}

type mcpServerPublicationInput struct {
	sourceKey string
	scope     resources.ResourceScope
	spec      aghconfig.MCPServer
}

type toolMCPDesiredResources struct {
	tools      []toolPublicationInput
	mcpServers []mcpServerPublicationInput
}

type toolMCPDeclarationProvider func(context.Context) (toolMCPDesiredResources, error)

type toolMCPConfigDeclarationProvider func(context.Context, *aghconfig.Config) (toolMCPDesiredResources, error)

type toolMCPSourceSyncer struct {
	raw       resources.RawStore
	toolStore resources.Store[toolspkg.Tool]
	toolCodec resources.KindCodec[toolspkg.Tool]
	mcpStore  resources.Store[aghconfig.MCPServer]
	mcpCodec  resources.KindCodec[aghconfig.MCPServer]
	actor     resources.MutationActor
	logger    *slog.Logger
	trigger   func(context.Context, resources.ResourceKind, resources.ReconcileReason) error
	config    toolMCPConfigDeclarationProvider
	providers []toolMCPDeclarationProvider
}

func newToolMCPSourceSyncer(
	raw resources.RawStore,
	toolStore resources.Store[toolspkg.Tool],
	toolCodec resources.KindCodec[toolspkg.Tool],
	mcpStore resources.Store[aghconfig.MCPServer],
	mcpCodec resources.KindCodec[aghconfig.MCPServer],
	actor resources.MutationActor,
	logger *slog.Logger,
	trigger func(context.Context, resources.ResourceKind, resources.ReconcileReason) error,
	providers ...toolMCPDeclarationProvider,
) toolMCPPublisher {
	return newToolMCPSourceSyncerWithConfigProvider(
		raw,
		toolStore,
		toolCodec,
		mcpStore,
		mcpCodec,
		actor,
		logger,
		trigger,
		nil,
		providers...,
	)
}

func newToolMCPSourceSyncerWithConfigProvider(
	raw resources.RawStore,
	toolStore resources.Store[toolspkg.Tool],
	toolCodec resources.KindCodec[toolspkg.Tool],
	mcpStore resources.Store[aghconfig.MCPServer],
	mcpCodec resources.KindCodec[aghconfig.MCPServer],
	actor resources.MutationActor,
	logger *slog.Logger,
	trigger func(context.Context, resources.ResourceKind, resources.ReconcileReason) error,
	configProvider toolMCPConfigDeclarationProvider,
	providers ...toolMCPDeclarationProvider,
) toolMCPPublisher {
	if raw == nil || toolStore == nil || toolCodec == nil || mcpStore == nil || mcpCodec == nil {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &toolMCPSourceSyncer{
		raw:       raw,
		toolStore: toolStore,
		toolCodec: toolCodec,
		mcpStore:  mcpStore,
		mcpCodec:  mcpCodec,
		actor:     actor,
		logger:    logger,
		trigger:   trigger,
		config:    configProvider,
		providers: append([]toolMCPDeclarationProvider(nil), providers...),
	}
}

func toolMCPSyncActor() resources.MutationActor {
	return resources.MutationActor{
		Kind: resources.MutationActorKindDaemon,
		ID:   "tool-mcp-sync",
		Source: resources.ResourceSource{
			Kind: resources.ResourceSourceKind("daemon"),
			ID:   "tool-mcp-sync",
		},
		MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
	}
}

func (s *toolMCPSourceSyncer) Sync(ctx context.Context) error {
	return s.syncConfig(ctx, nil)
}

func (s *toolMCPSourceSyncer) SyncConfig(ctx context.Context, cfg *aghconfig.Config) error {
	return s.syncConfig(ctx, cfg)
}

func (s *toolMCPSourceSyncer) syncConfig(ctx context.Context, cfg *aghconfig.Config) error {
	if s == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("daemon: tool/mcp sync context is required")
	}

	desired, err := s.desiredResources(ctx, cfg)
	if err != nil {
		return err
	}

	toolChanged, err := s.syncTools(ctx, desired.tools)
	if err != nil {
		return err
	}
	mcpChanged, err := s.syncMCPServers(ctx, desired.mcpServers)
	if err != nil {
		return err
	}

	if toolChanged && s.trigger != nil {
		if err := s.trigger(ctx, toolspkg.ToolResourceKind, resources.ReconcileReasonWrite); err != nil {
			return err
		}
	}
	if mcpChanged && s.trigger != nil {
		if err := s.trigger(ctx, aghconfig.MCPServerResourceKind, resources.ReconcileReasonWrite); err != nil {
			return err
		}
	}

	return nil
}

type desiredToolResource struct {
	id      string
	scope   resources.ResourceScope
	spec    toolspkg.Tool
	encoded []byte
}

type desiredMCPServerResource struct {
	id      string
	scope   resources.ResourceScope
	spec    aghconfig.MCPServer
	encoded []byte
}

func (s *toolMCPSourceSyncer) desiredResources(ctx context.Context, cfg *aghconfig.Config) (struct {
	tools      map[string]*desiredToolResource
	mcpServers map[string]desiredMCPServerResource
}, error) {
	desired := struct {
		tools      map[string]*desiredToolResource
		mcpServers map[string]desiredMCPServerResource
	}{
		tools:      make(map[string]*desiredToolResource),
		mcpServers: make(map[string]desiredMCPServerResource),
	}

	providers := make([]toolMCPDesiredResources, 0, len(s.providers)+1)
	if s.config != nil {
		items, err := s.config(ctx, cfg)
		if err != nil {
			return desired, err
		}
		providers = append(providers, items)
	}
	for _, provider := range s.providers {
		if provider == nil {
			continue
		}
		items, err := provider(ctx)
		if err != nil {
			return desired, err
		}
		providers = append(providers, items)
	}

	for _, items := range providers {
		for i := range items.tools {
			item := &items.tools[i]
			spec, encoded, err := validateAndEncodeTool(ctx, s.toolCodec, item.scope, item.spec)
			if err != nil {
				return desired, err
			}
			id := managedResourceID(toolManagedIDPrefix, item.scope.Normalize(), item.sourceKey, encoded)
			desired.tools[id] = &desiredToolResource{
				id:      id,
				scope:   item.scope.Normalize(),
				spec:    spec,
				encoded: encoded,
			}
		}
		for _, item := range items.mcpServers {
			spec, encoded, err := validateAndEncodeMCPServer(ctx, s.mcpCodec, item.scope, item.spec)
			if err != nil {
				return desired, err
			}
			id := managedResourceID(mcpServerManagedIDPrefix, item.scope.Normalize(), item.sourceKey, encoded)
			desired.mcpServers[id] = desiredMCPServerResource{
				id:      id,
				scope:   item.scope.Normalize(),
				spec:    spec,
				encoded: encoded,
			}
		}
	}

	return desired, nil
}
