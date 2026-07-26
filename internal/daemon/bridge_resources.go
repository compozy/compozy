package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/resources"
)

type bridgeResourceProjectorTarget interface {
	BuildBridgeResourceState(
		context.Context,
		[]resources.Record[bridgepkg.BridgeInstanceSpec],
	) (resources.ProjectionPlan, error)
	ApplyBridgeResourceState(context.Context, resources.ProjectionPlan) error
}

func bridgeResourceTarget(runtime *bridgeRuntime) bridgeResourceProjectorTarget {
	if runtime == nil {
		return nil
	}
	return runtime
}

type bridgeInstanceProjector struct {
	target bridgeResourceProjectorTarget
}

var _ resources.TypedProjector[bridgepkg.BridgeInstanceSpec] = (*bridgeInstanceProjector)(nil)

func newBridgeInstanceProjector(
	target bridgeResourceProjectorTarget,
) resources.TypedProjector[bridgepkg.BridgeInstanceSpec] {
	if target == nil {
		return nil
	}
	return &bridgeInstanceProjector{target: target}
}

func (p *bridgeInstanceProjector) Kind() resources.ResourceKind {
	return bridgepkg.BridgeInstanceResourceKind
}

func (p *bridgeInstanceProjector) DependsOn() []resources.ResourceKind {
	return nil
}

func (p *bridgeInstanceProjector) Build(
	ctx context.Context,
	records []resources.Record[bridgepkg.BridgeInstanceSpec],
) (resources.ProjectionPlan, error) {
	if p == nil || p.target == nil {
		return nil, errors.New("daemon: bridge instance projector target is required")
	}
	return p.target.BuildBridgeResourceState(ctx, records)
}

func (p *bridgeInstanceProjector) Apply(ctx context.Context, plan resources.ProjectionPlan) error {
	if p == nil || p.target == nil {
		return errors.New("daemon: bridge instance projector target is required")
	}
	return p.target.ApplyBridgeResourceState(ctx, plan)
}

func bridgeInstanceResourceStore(
	raw resources.RawStore,
	codecs *resources.CodecRegistry,
) (resources.Store[bridgepkg.BridgeInstanceSpec], error) {
	if raw == nil || codecs == nil {
		return nil, nil
	}
	codec, err := resources.ResolveCodec[bridgepkg.BridgeInstanceSpec](
		codecs,
		bridgepkg.BridgeInstanceResourceKind,
	)
	if err != nil {
		return nil, fmt.Errorf("daemon: resolve bridge instance codec: %w", err)
	}
	store, err := resources.NewStore(raw, codec)
	if err != nil {
		return nil, fmt.Errorf("daemon: create bridge instance resource store: %w", err)
	}
	return store, nil
}

func bridgeProviderLookup(runtime *bridgeRuntime) bridgepkg.BridgeProviderLookup {
	if runtime == nil {
		return nil
	}
	return func(ctx context.Context, extensionName string) (bridgepkg.BridgeProvider, bool, error) {
		providers, err := runtime.ListProviders(ctx)
		if err != nil {
			return bridgepkg.BridgeProvider{}, false, err
		}
		trimmed := strings.TrimSpace(extensionName)
		for _, provider := range providers {
			if strings.TrimSpace(provider.ExtensionName) == trimmed {
				return provider, true, nil
			}
		}
		return bridgepkg.BridgeProvider{}, false, nil
	}
}

func appendBridgeProjectorRegistration(
	registrations []resources.ProjectorRegistration,
	deps *resourceReconcileDriverDeps,
) ([]resources.ProjectorRegistration, error) {
	codec, err := resources.ResolveCodec[bridgepkg.BridgeInstanceSpec](
		deps.CodecRegistry,
		bridgepkg.BridgeInstanceResourceKind,
	)
	if err != nil {
		return nil, err
	}
	registration, err := resources.NewTypedProjectorRegistration(
		codec,
		newBridgeInstanceProjector(deps.Bridges),
	)
	if err != nil {
		return nil, err
	}
	return append(registrations, registration), nil
}
