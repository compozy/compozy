package bundles

import (
	"context"
	"fmt"
	"strings"

	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/windowmanager"
)

type ownedLayoutResource struct {
	ID    string
	Scope resources.ResourceScope
	Spec  windowmanager.LayoutResource
}

func materializeActivationLayouts(
	ctx context.Context,
	activation Activation,
	bundle extensionpkg.BundleSpec,
	profile extensionpkg.BundleProfile,
	scope resources.ResourceScope,
) ([]ownedLayoutResource, []InventoryItem, error) {
	codec, err := windowmanager.NewLayoutResourceCodec()
	if err != nil {
		return nil, nil, fmt.Errorf("bundles: create window layout codec: %w", err)
	}
	layouts := make([]ownedLayoutResource, 0, len(profile.Layouts))
	inventory := make([]InventoryItem, 0, len(profile.Layouts))
	for _, definition := range profile.Layouts {
		materializedID := stableID("lay", activation.ID, definition.Layout.ID)
		spec := windowmanager.CloneLayoutResource(definition.Layout)
		spec.ID = materializedID
		if scope.Kind == resources.ResourceScopeKindWorkspace {
			spec.Document.WorkspaceID = windowmanager.WorkspaceID(scope.ID)
		} else {
			spec.Document.WorkspaceID = ""
		}
		encoded, err := codec.Encode(spec)
		if err != nil {
			return nil, nil, fmt.Errorf(
				"bundles: encode layout %s/%s/%s/%s: %w",
				activation.ExtensionName,
				bundle.Name,
				profile.Name,
				strings.TrimSpace(definition.Layout.ID),
				err,
			)
		}
		spec, err = codec.DecodeAndValidate(ctx, scope, encoded)
		if err != nil {
			return nil, nil, fmt.Errorf(
				"bundles: materialize layout %s/%s/%s/%s: %w",
				activation.ExtensionName,
				bundle.Name,
				profile.Name,
				strings.TrimSpace(definition.Layout.ID),
				err,
			)
		}
		layouts = append(layouts, ownedLayoutResource{
			ID:    materializedID,
			Scope: scope.Normalize(),
			Spec:  windowmanager.CloneLayoutResource(spec),
		})
		inventory = append(inventory, InventoryItem{
			ActivationID: activation.ID,
			ResourceKind: string(windowmanager.WindowLayoutResourceKind),
			ResourceID:   materializedID,
			ResourceName: spec.DisplayName,
		})
	}
	return layouts, inventory, nil
}

func cloneOwnedLayoutResources(values []ownedLayoutResource) []ownedLayoutResource {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]ownedLayoutResource, 0, len(values))
	for _, value := range values {
		cloned = append(cloned, ownedLayoutResource{
			ID:    strings.TrimSpace(value.ID),
			Scope: value.Scope.Normalize(),
			Spec:  windowmanager.CloneLayoutResource(value.Spec),
		})
	}
	return cloned
}
