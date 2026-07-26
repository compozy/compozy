package daemon

import (
	"context"
	"errors"
	"fmt"

	core "github.com/compozy/agh/internal/api/core"
	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	bundlepkg "github.com/compozy/agh/internal/bundles"
	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/heartbeat"

	looppkg "github.com/compozy/agh/internal/loop"

	"github.com/compozy/agh/internal/resources"

	"github.com/compozy/agh/internal/skills"
	"github.com/compozy/agh/internal/soul"
	"github.com/compozy/agh/internal/windowmanager"

	toolspkg "github.com/compozy/agh/internal/tools"
)

func (d *Daemon) buildResourceKernel(registry Registry) (*resources.Kernel, error) {
	if registry == nil {
		return nil, errors.New("daemon: resource service registry is required")
	}

	dbSource, ok := registry.(extensionDBSource)
	if !ok || dbSource.DB() == nil {
		return nil, nil
	}

	kernel, err := resources.NewKernel(dbSource.DB())
	if err != nil {
		return nil, fmt.Errorf("daemon: create resource kernel: %w", err)
	}
	return kernel, nil
}

func (d *Daemon) buildResourceCodecs(bridges *bridgeRuntime) (*resources.CodecRegistry, error) {
	registry := resources.NewCodecRegistry()
	if err := registerDaemonResourceCodecs(registry, bridges); err != nil {
		return nil, err
	}
	return registry, nil
}

func registerDaemonResourceCodecs(registry *resources.CodecRegistry, bridges *bridgeRuntime) error {
	if err := registerDaemonResourceCodec(registry, "hook binding", newHookBindingCodec); err != nil {
		return err
	}
	if err := registerDaemonResourceCodec(registry, "tool", toolspkg.NewResourceCodec); err != nil {
		return err
	}
	if err := registerDaemonResourceCodec(registry, "mcp server", aghconfig.NewMCPServerResourceCodec); err != nil {
		return err
	}
	if err := registerDaemonResourceCodec(registry, "agent", aghconfig.NewAgentResourceCodec); err != nil {
		return err
	}
	if err := registerDaemonResourceCodec(registry, "agent soul", soul.NewResourceCodec); err != nil {
		return err
	}
	if err := registerDaemonResourceCodec(registry, "agent heartbeat", heartbeat.NewResourceCodec); err != nil {
		return err
	}
	if err := registerDaemonResourceCodec(registry, "skill", skills.NewResourceCodec); err != nil {
		return err
	}
	if err := registerDaemonResourceCodec(registry, "loop", looppkg.NewResourceCodec); err != nil {
		return err
	}
	if err := registerDaemonResourceCodec(
		registry,
		"window layout",
		windowmanager.NewLayoutResourceCodec,
	); err != nil {
		return err
	}
	if err := registerDaemonResourceCodec(registry, "automation job", automationpkg.NewJobResourceCodec); err != nil {
		return err
	}
	if err := registerDaemonResourceCodec(
		registry,
		"automation trigger",
		automationpkg.NewTriggerResourceCodec,
	); err != nil {
		return err
	}
	if err := registerDaemonResourceCodec(registry, "bridge instance", func() (
		resources.KindCodec[bridgepkg.BridgeInstanceSpec],
		error,
	) {
		return bridgepkg.NewBridgeInstanceResourceCodec(bridgeProviderLookup(bridges))
	}); err != nil {
		return err
	}
	if err := registerDaemonResourceCodec(registry, "bundle", bundlepkg.NewBundleResourceCodec); err != nil {
		return err
	}
	return registerDaemonResourceCodec(
		registry,
		"bundle activation",
		bundlepkg.NewActivationResourceCodec,
	)
}

func registerDaemonResourceCodec[T any](
	registry *resources.CodecRegistry,
	label string,
	build func() (resources.KindCodec[T], error),
) error {
	codec, err := build()
	if err != nil {
		return fmt.Errorf("daemon: build %s codec: %w", label, err)
	}
	if err := resources.RegisterCodec(registry, codec); err != nil {
		return fmt.Errorf("daemon: register %s codec: %w", label, err)
	}
	return nil
}

func (d *Daemon) buildResourceService(state *bootState) (core.ResourceService, error) {
	if state == nil {
		return nil, nil
	}
	rawStore := resourceRawStore(state.resourceKernel)
	if rawStore == nil {
		return nil, nil
	}

	service, err := core.NewOperatorResourceService(&core.ResourceServiceConfig{
		RawStore:      rawStore,
		CodecRegistry: state.resourceCodecs,
		Trigger: func(ctx context.Context, kind resources.ResourceKind, reason resources.ReconcileReason) error {
			if state == nil || state.resourceReconcile == nil {
				return nil
			}
			return state.resourceReconcile.Trigger(ctx, kind, reason)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("daemon: create resource service: %w", err)
	}
	return service, nil
}

func (d *Daemon) bootResourceReconcile(
	ctx context.Context,
	state *bootState,
	cleanup *bootCleanup,
) error {
	if state == nil {
		return errors.New("daemon: reconcile boot state is required")
	}
	if d.newResourceReconcile == nil {
		return errors.New("daemon: resource reconcile driver factory is required")
	}
	if state.agentCatalog == nil {
		state.agentCatalog = newResourceCatalog(cloneAgentDef)
	}
	if state.soulCatalog == nil {
		state.soulCatalog = newResourceCatalog(cloneSoulResourceSpec)
	}
	if state.heartbeatCatalog == nil {
		state.heartbeatCatalog = newResourceCatalog(cloneHeartbeatResourceSpec)
	}
	if state.toolCatalog == nil {
		state.toolCatalog = newResourceCatalog(cloneToolSpec)
	}
	if state.mcpServerCatalog == nil {
		state.mcpServerCatalog = newResourceCatalog(cloneDaemonMCPServer)
	}
	if state.loopCatalog == nil {
		state.loopCatalog = newResourceCatalog(looppkg.CloneResourceSpec)
	}
	if state.windowLayoutCatalog == nil {
		state.windowLayoutCatalog = newResourceCatalog(windowmanager.CloneLayoutResource)
	}

	driver, err := d.newResourceReconcile(ctx, resourceReconcileDriverDeps{
		Config:              state.cfg,
		Logger:              state.logger,
		Registry:            state.registry,
		ResourceStore:       resourceRawStore(state.resourceKernel),
		CodecRegistry:       state.resourceCodecs,
		Hooks:               state.hookDispatcher,
		AgentCatalog:        state.agentCatalog,
		SoulCatalog:         state.soulCatalog,
		HeartbeatCatalog:    state.heartbeatCatalog,
		ToolCatalog:         state.toolCatalog,
		MCPServerCatalog:    state.mcpServerCatalog,
		LoopCatalog:         state.loopCatalog,
		WindowLayoutCatalog: state.windowLayoutCatalog,
		SkillsRegistry:      state.skillsRegistry,
		Automation:          automationResourceTarget(state.automation),
		Bridges:             bridgeResourceTarget(state.bridges),
		Bundles:             state.bundles,
	})
	if err != nil {
		return fmt.Errorf("daemon: create resource reconcile driver: %w", err)
	}
	if driver == nil {
		return errors.New("daemon: resource reconcile driver factory returned nil")
	}

	state.resourceReconcile = driver
	cleanup.add(func(ctx context.Context) error {
		return driver.Close(ctx)
	})
	return nil
}
