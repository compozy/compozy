package daemon

import (
	"context"

	"fmt"

	"time"

	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	bundlepkg "github.com/compozy/agh/internal/bundles"
	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/soul"
	"github.com/compozy/agh/internal/windowmanager"
)

func newBundleResourceStore(
	state *bootState,
	now func() time.Time,
) (*bundlepkg.ResourceStore, error) {
	if state == nil || state.resourceKernel == nil || state.resourceCodecs == nil {
		return nil, nil
	}
	raw := resourceRawStore(state.resourceKernel)
	if raw == nil {
		return nil, nil
	}

	deps, err := resolveBundleResourceStoreDeps(state, raw)
	if err != nil {
		return nil, err
	}

	return bundlepkg.NewResourceStore(bundlepkg.ResourceStoreConfig{
		Bundles:         deps.bundleStore,
		BundleCodec:     deps.bundleCodec,
		Activations:     deps.activationStore,
		ActivationCodec: deps.activationCodec,
		Layouts:         deps.layoutStore,
		LayoutCodec:     deps.layoutCodec,
		Agents:          deps.agentStore,
		AgentCodec:      deps.agentCodec,
		Souls:           deps.soulStore,
		SoulCodec:       deps.soulCodec,
		Heartbeats:      deps.heartbeatStore,
		HeartbeatCodec:  deps.heartbeatCodec,
		Jobs:            deps.jobStore,
		JobCodec:        deps.jobCodec,
		Triggers:        deps.triggerStore,
		TriggerCodec:    deps.triggerCodec,
		Bridges:         deps.bridgeStore,
		BridgeCodec:     deps.bridgeCodec,
		Actor:           resourceReconcileActor(),
		Trigger: func(ctx context.Context, kind resources.ResourceKind, reason resources.ReconcileReason) error {
			if state.resourceReconcile == nil {
				return nil
			}
			return state.resourceReconcile.Trigger(ctx, kind, reason)
		},
		Now: now,
	})
}

type bundleResourceStoreDeps struct {
	bundleCodec     resources.KindCodec[bundlepkg.BundleResourceSpec]
	bundleStore     resources.Store[bundlepkg.BundleResourceSpec]
	activationCodec resources.KindCodec[bundlepkg.ActivationResourceSpec]
	activationStore resources.Store[bundlepkg.ActivationResourceSpec]
	layoutCodec     resources.KindCodec[windowmanager.LayoutResource]
	layoutStore     resources.Store[windowmanager.LayoutResource]
	agentCodec      resources.KindCodec[aghconfig.AgentDef]
	agentStore      resources.Store[aghconfig.AgentDef]
	soulCodec       resources.KindCodec[soul.ResourceSpec]
	soulStore       resources.Store[soul.ResourceSpec]
	heartbeatCodec  resources.KindCodec[heartbeat.ResourceSpec]
	heartbeatStore  resources.Store[heartbeat.ResourceSpec]
	jobCodec        resources.KindCodec[automationpkg.Job]
	jobStore        resources.Store[automationpkg.Job]
	triggerCodec    resources.KindCodec[automationpkg.Trigger]
	triggerStore    resources.Store[automationpkg.Trigger]
	bridgeCodec     resources.KindCodec[bridgepkg.BridgeInstanceSpec]
	bridgeStore     resources.Store[bridgepkg.BridgeInstanceSpec]
}

func resolveBundleResourceStoreDeps(
	state *bootState,
	raw resources.RawStore,
) (bundleResourceStoreDeps, error) {
	deps, err := resolveBundleBaseResourceStoreDeps(state, raw)
	if err != nil {
		return bundleResourceStoreDeps{}, err
	}
	projectionDeps, err := resolveBundleProjectionResourceStoreDeps(state, raw)
	if err != nil {
		return bundleResourceStoreDeps{}, err
	}
	deps.jobCodec = projectionDeps.jobCodec
	deps.jobStore = projectionDeps.jobStore
	deps.triggerCodec = projectionDeps.triggerCodec
	deps.triggerStore = projectionDeps.triggerStore
	deps.bridgeCodec = projectionDeps.bridgeCodec
	deps.bridgeStore = projectionDeps.bridgeStore
	deps.layoutCodec = projectionDeps.layoutCodec
	deps.layoutStore = projectionDeps.layoutStore
	return deps, nil
}

func resolveBundleBaseResourceStoreDeps(
	state *bootState,
	raw resources.RawStore,
) (bundleResourceStoreDeps, error) {
	bundleCodec, bundleStore, err := resolveDaemonResourceStore[bundlepkg.BundleResourceSpec](
		state,
		raw,
		bundlepkg.BundleResourceKind,
		"bundle",
	)
	if err != nil {
		return bundleResourceStoreDeps{}, err
	}
	activationCodec, activationStore, err := resolveDaemonResourceStore[bundlepkg.ActivationResourceSpec](
		state,
		raw,
		bundlepkg.BundleActivationResourceKind,
		"bundle activation",
	)
	if err != nil {
		return bundleResourceStoreDeps{}, err
	}
	agentCodec, agentStore, err := resolveDaemonResourceStore[aghconfig.AgentDef](
		state,
		raw,
		aghconfig.AgentResourceKind,
		"bundle agent",
	)
	if err != nil {
		return bundleResourceStoreDeps{}, err
	}
	soulCodec, soulStore, err := resolveDaemonResourceStore[soul.ResourceSpec](
		state,
		raw,
		soul.ResourceKind,
		"bundle soul",
	)
	if err != nil {
		return bundleResourceStoreDeps{}, err
	}
	heartbeatCodec, heartbeatStore, err := resolveDaemonResourceStore[heartbeat.ResourceSpec](
		state,
		raw,
		heartbeat.ResourceKind,
		"bundle heartbeat",
	)
	if err != nil {
		return bundleResourceStoreDeps{}, err
	}
	return bundleResourceStoreDeps{
		bundleCodec:     bundleCodec,
		bundleStore:     bundleStore,
		activationCodec: activationCodec,
		activationStore: activationStore,
		agentCodec:      agentCodec,
		agentStore:      agentStore,
		soulCodec:       soulCodec,
		soulStore:       soulStore,
		heartbeatCodec:  heartbeatCodec,
		heartbeatStore:  heartbeatStore,
	}, nil
}

func resolveBundleProjectionResourceStoreDeps(
	state *bootState,
	raw resources.RawStore,
) (bundleResourceStoreDeps, error) {
	jobCodec, jobStore, err := resolveDaemonResourceStore[automationpkg.Job](
		state,
		raw,
		automationpkg.JobResourceKind,
		"bundle automation job",
	)
	if err != nil {
		return bundleResourceStoreDeps{}, err
	}
	triggerCodec, triggerStore, err := resolveDaemonResourceStore[automationpkg.Trigger](
		state,
		raw,
		automationpkg.TriggerResourceKind,
		"bundle automation trigger",
	)
	if err != nil {
		return bundleResourceStoreDeps{}, err
	}
	bridgeCodec, bridgeStore, err := resolveDaemonResourceStore[bridgepkg.BridgeInstanceSpec](
		state,
		raw,
		bridgepkg.BridgeInstanceResourceKind,
		"bundle bridge instance",
	)
	if err != nil {
		return bundleResourceStoreDeps{}, err
	}
	layoutCodec, layoutStore, err := resolveDaemonResourceStore[windowmanager.LayoutResource](
		state,
		raw,
		windowmanager.WindowLayoutResourceKind,
		"bundle window layout",
	)
	if err != nil {
		return bundleResourceStoreDeps{}, err
	}
	return bundleResourceStoreDeps{
		jobCodec:     jobCodec,
		jobStore:     jobStore,
		triggerCodec: triggerCodec,
		triggerStore: triggerStore,
		bridgeCodec:  bridgeCodec,
		bridgeStore:  bridgeStore,
		layoutCodec:  layoutCodec,
		layoutStore:  layoutStore,
	}, nil
}

func resolveDaemonResourceStore[T any](
	state *bootState,
	raw resources.RawStore,
	kind resources.ResourceKind,
	label string,
) (resources.KindCodec[T], resources.Store[T], error) {
	codec, err := resources.ResolveCodec[T](state.resourceCodecs, kind)
	if err != nil {
		return nil, nil, fmt.Errorf("daemon: resolve %s codec: %w", label, err)
	}
	store, err := resources.NewStore(raw, codec)
	if err != nil {
		return nil, nil, fmt.Errorf("daemon: create %s resource store: %w", label, err)
	}
	return codec, store, nil
}
