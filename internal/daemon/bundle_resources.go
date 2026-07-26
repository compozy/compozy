package daemon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"

	bundlepkg "github.com/compozy/agh/internal/bundles"

	"github.com/compozy/agh/internal/resources"
)

const bundleManagedIDPrefix = "daemon.sync.bundle."

type bundleResourcePublisher interface {
	Sync(context.Context) error
}

type bundleResourcePublisherFunc func(context.Context) error

func (f bundleResourcePublisherFunc) Sync(ctx context.Context) error {
	if f == nil {
		return nil
	}
	return f(ctx)
}

type bundleNoopProjectionPlan struct {
	revision   int64
	operations int
}

func (p *bundleNoopProjectionPlan) Kind() resources.ResourceKind {
	return bundlepkg.BundleResourceKind
}

func (p *bundleNoopProjectionPlan) Revision() int64 {
	if p == nil {
		return 0
	}
	return p.revision
}

func (p *bundleNoopProjectionPlan) OperationCount() int {
	if p == nil {
		return 0
	}
	return p.operations
}

type bundleNoopProjector struct{}

var _ resources.TypedProjector[bundlepkg.BundleResourceSpec] = (*bundleNoopProjector)(nil)

func (p *bundleNoopProjector) Kind() resources.ResourceKind {
	return bundlepkg.BundleResourceKind
}

func (p *bundleNoopProjector) DependsOn() []resources.ResourceKind {
	return nil
}

func (p *bundleNoopProjector) Build(
	_ context.Context,
	records []resources.Record[bundlepkg.BundleResourceSpec],
) (resources.ProjectionPlan, error) {
	var revision int64
	for _, record := range records {
		if record.Version > revision {
			revision = record.Version
		}
	}
	return &bundleNoopProjectionPlan{revision: revision, operations: len(records)}, nil
}

func (p *bundleNoopProjector) Apply(context.Context, resources.ProjectionPlan) error {
	return nil
}

type bundlePublicationInput struct {
	sourceKey string
	scope     resources.ResourceScope
	spec      bundlepkg.BundleResourceSpec
}

type bundleDeclarationProvider func(context.Context) ([]bundlePublicationInput, error)

type bundleSourceSyncer struct {
	store     resources.Store[bundlepkg.BundleResourceSpec]
	codec     resources.KindCodec[bundlepkg.BundleResourceSpec]
	actor     resources.MutationActor
	logger    *slog.Logger
	trigger   func(context.Context, resources.ResourceKind, resources.ReconcileReason) error
	providers []bundleDeclarationProvider
}

func newBundleSourceSyncer(
	store resources.Store[bundlepkg.BundleResourceSpec],
	codec resources.KindCodec[bundlepkg.BundleResourceSpec],
	actor resources.MutationActor,
	logger *slog.Logger,
	trigger func(context.Context, resources.ResourceKind, resources.ReconcileReason) error,
	providers ...bundleDeclarationProvider,
) bundleResourcePublisher {
	if store == nil || codec == nil {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &bundleSourceSyncer{
		store:     store,
		codec:     codec,
		actor:     actor,
		logger:    logger,
		trigger:   trigger,
		providers: append([]bundleDeclarationProvider(nil), providers...),
	}
}

func bundleSyncActor() resources.MutationActor {
	return resources.MutationActor{
		Kind: resources.MutationActorKindDaemon,
		ID:   "bundle-sync",
		Source: resources.ResourceSource{
			Kind: resources.ResourceSourceKind("daemon"),
			ID:   "bundle-sync",
		},
		MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
	}
}

func (s *bundleSourceSyncer) Sync(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("daemon: bundle sync context is required")
	}

	desired, err := s.desiredBundles(ctx)
	if err != nil {
		return err
	}
	changed, err := s.syncBundles(ctx, desired)
	if err != nil {
		return err
	}
	if changed && s.trigger != nil {
		return s.trigger(ctx, bundlepkg.BundleResourceKind, resources.ReconcileReasonWrite)
	}
	return nil
}

type desiredBundleResource struct {
	id      string
	scope   resources.ResourceScope
	spec    bundlepkg.BundleResourceSpec
	encoded []byte
}

func (s *bundleSourceSyncer) desiredBundles(
	ctx context.Context,
) (map[string]desiredBundleResource, error) {
	desired := make(map[string]desiredBundleResource)
	for _, provider := range s.providers {
		if provider == nil {
			continue
		}
		items, err := provider(ctx)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			spec, encoded, err := validateAndEncodeBundle(ctx, s.codec, item.scope, item.spec)
			if err != nil {
				return nil, err
			}
			id := bundlepkg.BundleResourceID(spec.ExtensionName, spec.Bundle.Name)
			if id == "" {
				id = managedResourceID(bundleManagedIDPrefix, item.scope.Normalize(), item.sourceKey, encoded)
			}
			desired[id] = desiredBundleResource{
				id:      id,
				scope:   item.scope.Normalize(),
				spec:    spec,
				encoded: encoded,
			}
		}
	}
	return desired, nil
}

func (s *bundleSourceSyncer) syncBundles(
	ctx context.Context,
	desired map[string]desiredBundleResource,
) (bool, error) {
	source := s.actor.Source
	current, err := s.store.List(ctx, s.actor, resources.ResourceFilter{
		Kind:   bundlepkg.BundleResourceKind,
		Source: &source,
	})
	if err != nil {
		return false, fmt.Errorf("daemon: list managed bundles: %w", err)
	}
	currentByID := make(map[string]resources.Record[bundlepkg.BundleResourceSpec], len(current))
	for _, record := range current {
		currentByID[record.ID] = record
	}

	changed := false
	for id, desiredBundle := range desired {
		existing, ok := currentByID[id]
		if ok && s.sameBundle(existing, desiredBundle.scope, desiredBundle.encoded) {
			delete(currentByID, id)
			continue
		}
		expectedVersion := int64(0)
		if ok {
			expectedVersion = existing.Version
		}
		if _, err := s.store.Put(ctx, s.actor, resources.Draft[bundlepkg.BundleResourceSpec]{
			ID:              desiredBundle.id,
			Scope:           desiredBundle.scope,
			ExpectedVersion: expectedVersion,
			Spec:            desiredBundle.spec,
		}); err != nil {
			return false, fmt.Errorf("daemon: sync bundle %q: %w", id, err)
		}
		changed = true
		delete(currentByID, id)
	}
	for _, stale := range currentByID {
		if err := s.store.Delete(ctx, s.actor, stale.ID, stale.Version); err != nil {
			return false, fmt.Errorf("daemon: delete stale bundle %q: %w", stale.ID, err)
		}
		changed = true
	}
	return changed, nil
}

func (s *bundleSourceSyncer) sameBundle(
	record resources.Record[bundlepkg.BundleResourceSpec],
	scope resources.ResourceScope,
	encoded []byte,
) bool {
	if record.Scope != scope {
		return false
	}
	currentEncoded, err := s.codec.Encode(record.Spec)
	if err != nil {
		return false
	}
	return bytes.Equal(currentEncoded, encoded)
}
