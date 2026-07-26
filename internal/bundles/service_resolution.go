package bundles

import (
	"context"

	"fmt"

	"slices"
	"strings"

	extensionpkg "github.com/compozy/agh/internal/extension"

	"github.com/compozy/agh/internal/resources"
)

func (s *Service) resolveRequest(
	ctx context.Context,
	req ActivateRequest,
	mode workspaceResolutionMode,
) (resolvedActivation, error) {
	scope := req.Scope.Normalize()
	if scope == "" {
		scope = ScopeGlobal
	}

	workspaceID, err := s.resolveWorkspace(ctx, scope, req.Workspace, mode)
	if err != nil {
		return resolvedActivation{}, err
	}

	activation := Activation{
		ID: ActivationResourceID(
			strings.TrimSpace(req.ExtensionName),
			strings.TrimSpace(req.BundleName),
			strings.TrimSpace(req.ProfileName),
			scope,
			workspaceID,
		),
		ExtensionName: strings.TrimSpace(req.ExtensionName),
		BundleName:    strings.TrimSpace(req.BundleName),
		ProfileName:   strings.TrimSpace(req.ProfileName),
		Scope:         scope,
		WorkspaceID:   workspaceID,
	}
	resolved, err := s.resolveActivation(ctx, activation)
	if err != nil {
		return resolvedActivation{}, err
	}
	resolved.activation.SpecContentHash = resolved.specContentHash
	return resolved, nil
}

func (s *Service) resolveActivation(ctx context.Context, activation Activation) (resolvedActivation, error) {
	activation, err := activation.Validated()
	if err != nil {
		return resolvedActivation{}, err
	}

	definition, err := s.resolveActivationDefinition(ctx, activation)
	if err != nil {
		return resolvedActivation{}, err
	}
	resolved := resolvedActivation{
		activation:      activation,
		bundleRecord:    definition.bundleRecord,
		bundle:          definition.bundle,
		profile:         definition.profile,
		specContentHash: definition.specContentHash,
	}

	resolved.channels = declaredChannelsForProfile(activation, definition.bundle, definition.profile)
	materialized, err := s.materializeActivationResources(
		ctx,
		activation,
		definition.bundleRecord,
		definition.bundle,
		definition.profile,
	)
	if err != nil {
		return resolvedActivation{}, err
	}
	if err := s.validateActivationAgentBindings(ctx, activation, materialized); err != nil {
		return resolvedActivation{}, err
	}
	resolved.layouts = materialized.layouts
	resolved.agents = materialized.agents
	resolved.souls = materialized.souls
	resolved.heartbeats = materialized.heartbeats
	resolved.jobs = materialized.jobs
	resolved.triggers = materialized.triggers
	resolved.bridges = materialized.bridges
	resolved.inventory = materialized.inventory
	return resolved, nil
}

func (s *Service) collectDesiredState(ctx context.Context, activations []Activation) (reconcileState, error) {
	bundleRecords, err := s.store.ListBundleResources(ctx)
	if err != nil {
		return reconcileState{}, err
	}
	return s.collectDesiredStateFromBundleRecords(ctx, activations, bundleRecords)
}

func (s *Service) applyNetworkSettings(declaredChannels []DeclaredChannel) {
	slices.SortFunc(declaredChannels, func(left, right DeclaredChannel) int {
		if cmp := strings.Compare(left.ExtensionName, right.ExtensionName); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(left.BundleName, right.BundleName); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(left.ProfileName, right.ProfileName); cmp != 0 {
			return cmp
		}
		return strings.Compare(left.Name, right.Name)
	})

	s.settingsMu.Lock()
	s.settings = NetworkSettings{
		DeclaredChannels: append([]DeclaredChannel(nil), declaredChannels...),
	}
	s.settingsMu.Unlock()
}

func (s *Service) resolveActivationDefinition(
	ctx context.Context,
	activation Activation,
) (activationDefinition, error) {
	bundleRecord, ok, err := s.findBundleResource(ctx, activation.ExtensionName, activation.BundleName)
	if err != nil {
		return activationDefinition{}, err
	}
	if !ok {
		return activationDefinition{}, fmt.Errorf(
			"%w: %s/%s",
			ErrBundleNotFound,
			activation.ExtensionName,
			activation.BundleName,
		)
	}
	bundle := bundleRecord.Spec.Bundle
	profile, ok := findProfile(bundle.Profiles, activation.ProfileName)
	if !ok {
		return activationDefinition{}, fmt.Errorf(
			"%w: %s/%s/%s",
			ErrProfileNotFound,
			activation.ExtensionName,
			activation.BundleName,
			activation.ProfileName,
		)
	}
	specContentHash, err := bundleProfileSpecContentHash(bundle, profile)
	if err != nil {
		return activationDefinition{}, err
	}
	return activationDefinition{
		bundleRecord:    bundleRecord,
		bundle:          bundle,
		profile:         profile,
		specContentHash: specContentHash,
	}, nil
}

func (s *Service) findBundleResource(
	ctx context.Context,
	extensionName string,
	bundleName string,
) (resources.Record[BundleResourceSpec], bool, error) {
	records, err := s.store.ListBundleResources(ctx)
	if err != nil {
		return resources.Record[BundleResourceSpec]{}, false, err
	}
	return findBundleResourceRecord(records, extensionName, bundleName)
}

func findBundleResourceRecord(
	records []resources.Record[BundleResourceSpec],
	extensionName string,
	bundleName string,
) (resources.Record[BundleResourceSpec], bool, error) {
	trimmedExtension := strings.TrimSpace(extensionName)
	trimmedBundle := strings.TrimSpace(bundleName)
	for _, record := range records {
		if strings.EqualFold(strings.TrimSpace(record.Spec.ExtensionName), trimmedExtension) &&
			strings.EqualFold(strings.TrimSpace(record.Spec.Bundle.Name), trimmedBundle) {
			return record, true, nil
		}
	}
	return resources.Record[BundleResourceSpec]{}, false, nil
}

func declaredChannelsForProfile(
	activation Activation,
	bundle extensionpkg.BundleSpec,
	profile extensionpkg.BundleProfile,
) []DeclaredChannel {
	channels := make([]DeclaredChannel, 0, len(profile.Channels.Items))
	for _, item := range profile.Channels.Items {
		channels = append(channels, DeclaredChannel{
			ActivationID:  activation.ID,
			ExtensionName: activation.ExtensionName,
			BundleName:    bundle.Name,
			ProfileName:   profile.Name,
			WorkspaceID:   activation.WorkspaceID,
			Name:          strings.TrimSpace(item.Name),
			Description:   strings.TrimSpace(item.Description),
			Primary:       strings.TrimSpace(profile.Channels.Primary) == strings.TrimSpace(item.Name),
		})
	}
	return channels
}

func (s *Service) materializeActivationResources(
	ctx context.Context,
	activation Activation,
	bundleRecord resources.Record[BundleResourceSpec],
	bundle extensionpkg.BundleSpec,
	profile extensionpkg.BundleProfile,
) (materializedActivationResources, error) {
	scope := resourceScopeForActivation(activation)
	layouts, layoutInventory, err := materializeActivationLayouts(ctx, activation, bundle, profile, scope)
	if err != nil {
		return materializedActivationResources{}, err
	}
	agentResources, err := materializeActivationAgentResources(activation, bundle, profile, scope)
	if err != nil {
		return materializedActivationResources{}, err
	}
	jobs, triggers, automationInventory, err := materializeActivationAutomation(activation, bundle, profile)
	if err != nil {
		return materializedActivationResources{}, err
	}
	bridges, bridgeInventory, err := s.materializeActivationBridges(
		ctx,
		activation,
		bundleRecord,
		bundle,
		profile,
	)
	if err != nil {
		return materializedActivationResources{}, err
	}
	inventory := make([]InventoryItem, 0, materializedInventoryCapacity(profile))
	inventory = append(inventory, layoutInventory...)
	inventory = append(inventory, agentResources.inventory...)
	inventory = append(inventory, automationInventory...)
	inventory = append(inventory, bridgeInventory...)
	return materializedActivationResources{
		layouts:    layouts,
		agents:     agentResources.agents,
		souls:      agentResources.souls,
		heartbeats: agentResources.heartbeats,
		jobs:       jobs,
		triggers:   triggers,
		bridges:    bridges,
		inventory:  inventory,
	}, nil
}
