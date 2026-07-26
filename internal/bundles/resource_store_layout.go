package bundles

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/windowmanager"
)

func (s *ResourceStore) listOwnedLayoutInventory(
	ctx context.Context,
	owner resources.ResourceOwner,
) ([]InventoryItem, error) {
	return listOwnedInventoryForKind(
		ctx,
		s.layouts,
		s.actor,
		owner,
		windowmanager.WindowLayoutResourceKind,
		"window layouts",
		func(spec windowmanager.LayoutResource) string { return spec.DisplayName },
	)
}

func (s *ResourceStore) syncOwnedLayoutResources(
	ctx context.Context,
	plan BundleActivationResourcePlan,
	changed map[resources.ResourceKind]struct{},
) error {
	desired := make(map[string]map[string]ownedLayoutResource)
	for _, layout := range plan.desiredLayouts {
		ownerID := strings.TrimSpace(plan.layoutOwners[strings.TrimSpace(layout.ID)])
		if ownerID == "" {
			return fmt.Errorf("bundles: owned window layout %q has no activation owner", layout.ID)
		}
		if desired[ownerID] == nil {
			desired[ownerID] = make(map[string]ownedLayoutResource)
		}
		desired[ownerID][layout.ID] = layout
	}
	return syncOwnedResources(
		windowmanager.WindowLayoutResourceKind,
		plan.activeActivationIDs,
		changed,
		func(owner resources.ResourceOwner) ([]resources.Record[windowmanager.LayoutResource], error) {
			return s.layouts.List(ctx, s.actor, resources.ResourceFilter{
				Kind:  windowmanager.WindowLayoutResourceKind,
				Owner: &owner,
			})
		},
		func(ownerID string, current map[string]resources.Record[windowmanager.LayoutResource]) error {
			return s.upsertOwnedLayouts(ctx, ownerID, current, desired[ownerID], changed)
		},
		func(ownerID string) resources.MutationActor { return activationResourceActor(s.actor, ownerID) },
		func(actor resources.MutationActor, stale resources.Record[windowmanager.LayoutResource]) error {
			return s.layouts.Delete(ctx, actor, stale.ID, stale.Version)
		},
		func() ([]resources.Record[windowmanager.LayoutResource], error) {
			return s.layouts.List(
				ctx,
				s.actor,
				resources.ResourceFilter{Kind: windowmanager.WindowLayoutResourceKind},
			)
		},
	)
}

func (s *ResourceStore) upsertOwnedLayouts(
	ctx context.Context,
	ownerID string,
	current map[string]resources.Record[windowmanager.LayoutResource],
	desired map[string]ownedLayoutResource,
	changed map[resources.ResourceKind]struct{},
) error {
	actor := activationResourceActor(s.actor, ownerID)
	for id, desiredLayout := range desired {
		spec := windowmanager.CloneLayoutResource(desiredLayout.Spec)
		scope := desiredLayout.Scope.Normalize()
		existing, ok := current[id]
		if ok && existing.Scope == scope && s.sameLayout(existing, spec) {
			delete(current, id)
			continue
		}
		expectedVersion := int64(0)
		if ok {
			expectedVersion = existing.Version
		}
		if _, err := s.layouts.Put(ctx, actor, resources.Draft[windowmanager.LayoutResource]{
			ID:              id,
			Scope:           scope,
			ExpectedVersion: expectedVersion,
			Spec:            spec,
		}); err != nil {
			return fmt.Errorf("bundles: upsert owned window layout %q: %w", id, err)
		}
		changed[windowmanager.WindowLayoutResourceKind] = struct{}{}
		delete(current, id)
	}
	return deleteStaleOwnedRecords(
		ctx,
		actor,
		current,
		changed,
		windowmanager.WindowLayoutResourceKind,
		s.layouts.Delete,
	)
}

func (s *ResourceStore) sameLayout(
	record resources.Record[windowmanager.LayoutResource],
	desired windowmanager.LayoutResource,
) bool {
	return sameEncodedSpec(s.layoutCodec, record.Scope, record.Spec, desired)
}
