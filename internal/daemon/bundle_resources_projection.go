package daemon

import (
	"context"

	"fmt"
	"log/slog"
	"slices"
	"strings"

	bundlepkg "github.com/compozy/agh/internal/bundles"

	extensionpkg "github.com/compozy/agh/internal/extension"

	"github.com/compozy/agh/internal/resources"
)

func extensionManifestBundleDeclarationProvider(
	registry *extensionpkg.Registry,
	runtime func() extensionRuntime,
	logger *slog.Logger,
) bundleDeclarationProvider {
	return func(ctx context.Context) ([]bundlePublicationInput, error) {
		if registry == nil || runtime == nil {
			return nil, nil
		}
		manager := runtime()
		if manager == nil {
			return nil, nil
		}
		infos, err := registry.List()
		if err != nil {
			return nil, fmt.Errorf("daemon: list extensions for bundle sync: %w", err)
		}
		slices.SortFunc(infos, func(left, right extensionpkg.ExtensionInfo) int {
			return strings.Compare(left.Name, right.Name)
		})

		globalScope := resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal}
		var desired []bundlePublicationInput
		for _, info := range infos {
			if !info.Enabled {
				continue
			}
			ext, err := loadExtensionSnapshot(ctx, registry, manager, logger, info.Name)
			if err != nil {
				return nil, fmt.Errorf("daemon: load extension %q for bundle sync: %w", info.Name, err)
			}
			if ext == nil || ext.Manifest == nil || !ext.Status.Registered {
				continue
			}
			for _, bundle := range ext.Bundles {
				desired = append(desired, bundlePublicationInput{
					sourceKey: "extension/" + ext.Info.Name + "/bundle/" + strings.TrimSpace(bundle.Name),
					scope:     globalScope,
					spec: bundlepkg.BundleResourceSpec{
						ExtensionName:       strings.TrimSpace(ext.Info.Name),
						Bundle:              bundle,
						OwnerBridgePlatform: strings.TrimSpace(ext.Manifest.Bridge.Platform),
						OwnerProvidesBridgeAdapter: slices.Contains(
							ext.Manifest.Capabilities.Provides,
							"bridge.adapter",
						),
					},
				})
			}
		}
		return desired, nil
	}
}

func validateAndEncodeBundle(
	ctx context.Context,
	codec resources.KindCodec[bundlepkg.BundleResourceSpec],
	scope resources.ResourceScope,
	spec bundlepkg.BundleResourceSpec,
) (bundlepkg.BundleResourceSpec, []byte, error) {
	encoded, err := codec.Encode(spec)
	if err != nil {
		return bundlepkg.BundleResourceSpec{}, nil, err
	}
	validated, err := codec.DecodeAndValidate(ctx, scope.Normalize(), encoded)
	if err != nil {
		return bundlepkg.BundleResourceSpec{}, nil, err
	}
	canonical, err := codec.Encode(validated)
	if err != nil {
		return bundlepkg.BundleResourceSpec{}, nil, err
	}
	return validated, canonical, nil
}

func appendBundleProjectorRegistrations(
	registrations []resources.ProjectorRegistration,
	deps *resourceReconcileDriverDeps,
) ([]resources.ProjectorRegistration, error) {
	bundleCodec, err := resources.ResolveCodec[bundlepkg.BundleResourceSpec](
		deps.CodecRegistry,
		bundlepkg.BundleResourceKind,
	)
	if err != nil {
		return nil, err
	}
	bundleRegistration, err := resources.NewTypedProjectorRegistration(
		bundleCodec,
		&bundleNoopProjector{},
	)
	if err != nil {
		return nil, err
	}
	activationRegistration, err := resources.NewBundleActivationProjectorRegistration[
		bundlepkg.ActivationResourceSpec,
		bundlepkg.BundleResourceSpec,
	](deps.CodecRegistry, deps.Bundles)
	if err != nil {
		return nil, err
	}
	return append(registrations, bundleRegistration, activationRegistration), nil
}

func newBundleResourcePublisher(
	state *bootState,
	registry *extensionpkg.Registry,
) (bundleResourcePublisher, error) {
	publisher := bundleResourcePublisher(bundleResourcePublisherFunc(func(context.Context) error { return nil }))
	if state == nil || state.resourceKernel == nil || state.resourceCodecs == nil {
		return publisher, nil
	}
	codec, err := resources.ResolveCodec[bundlepkg.BundleResourceSpec](
		state.resourceCodecs,
		bundlepkg.BundleResourceKind,
	)
	if err != nil {
		return nil, fmt.Errorf("daemon: resolve bundle codec: %w", err)
	}
	store, err := resources.NewStore(state.resourceKernel, codec)
	if err != nil {
		return nil, fmt.Errorf("daemon: create bundle store: %w", err)
	}
	return newBundleSourceSyncer(
		store,
		codec,
		bundleSyncActor(),
		state.logger,
		func(ctx context.Context, kind resources.ResourceKind, reason resources.ReconcileReason) error {
			if state.resourceReconcile == nil {
				return nil
			}
			return state.resourceReconcile.Trigger(ctx, kind, reason)
		},
		extensionManifestBundleDeclarationProvider(registry, state.currentExtensionRuntime, state.logger),
	), nil
}

func (d *Daemon) newBundlePublisher(
	state *bootState,
	registry *extensionpkg.Registry,
) (bundleResourcePublisher, error) {
	return newBundleResourcePublisher(state, registry)
}
