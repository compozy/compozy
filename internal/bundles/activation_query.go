package bundles

import (
	"context"
	"fmt"
	"strings"
)

func (s *Service) ListActivations(ctx context.Context) ([]ActivationPreview, error) {
	if err := s.checkReady(ctx); err != nil {
		return nil, err
	}

	activations, err := s.store.ListBundleActivations(ctx)
	if err != nil {
		return nil, fmt.Errorf("bundles: list bundle activations: %w", err)
	}
	if len(activations) == 0 {
		return []ActivationPreview{}, nil
	}
	bundleRecords, err := s.store.ListBundleResources(ctx)
	if err != nil {
		return nil, fmt.Errorf("bundles: list bundle resources for activations: %w", err)
	}
	bundleLookup := newBundleRecordLookup(bundleRecords)

	items := make([]ActivationPreview, 0, len(activations))
	for _, activation := range activations {
		resolved, resolveErr := s.resolveActivationFromBundleLookup(ctx, activation, bundleLookup)
		if resolveErr != nil {
			return nil, fmt.Errorf("bundles: resolve activation %q for listing: %w", activation.ID, resolveErr)
		}
		inventory, inventoryErr := s.store.ListBundleActivationInventory(ctx, activation.ID)
		if inventoryErr != nil {
			return nil, fmt.Errorf("bundles: list activation inventory for %q: %w", activation.ID, inventoryErr)
		}
		if len(inventory) > 0 {
			resolved.inventory = inventory
		}
		items = append(items, ActivationPreview{
			Activation: cloneActivation(resolved.activation),
			Bundle:     cloneBundleSpec(resolved.bundle),
			Profile:    cloneBundleProfile(resolved.profile),
			Inventory:  cloneInventoryItems(resolved.inventory),
			SpecDrift:  activationSpecDrift(activation.SpecContentHash, resolved.specContentHash),
		})
	}
	return items, nil
}

func (s *Service) GetActivation(ctx context.Context, id string) (ActivationPreview, error) {
	if err := s.checkReady(ctx); err != nil {
		return ActivationPreview{}, err
	}

	activation, err := s.store.GetBundleActivation(ctx, strings.TrimSpace(id))
	if err != nil {
		return ActivationPreview{}, err
	}
	resolved, err := s.resolveActivation(ctx, activation)
	if err != nil {
		return ActivationPreview{}, err
	}
	inventory, err := s.store.ListBundleActivationInventory(ctx, activation.ID)
	if err != nil {
		return ActivationPreview{}, err
	}
	if len(inventory) > 0 {
		resolved.inventory = inventory
	}
	return ActivationPreview{
		Activation: cloneActivation(resolved.activation),
		Bundle:     cloneBundleSpec(resolved.bundle),
		Profile:    cloneBundleProfile(resolved.profile),
		Inventory:  cloneInventoryItems(resolved.inventory),
		SpecDrift:  activationSpecDrift(activation.SpecContentHash, resolved.specContentHash),
	}, nil
}
