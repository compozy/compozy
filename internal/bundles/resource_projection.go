package bundles

import (
	"context"
	"errors"
	"fmt"
	"strings"

	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/soul"
	"github.com/compozy/agh/internal/windowmanager"
)

// BundleActivationResourcePlan is the owned-resource composition plan produced from bundle.activation records.
type BundleActivationResourcePlan struct {
	revision            int64
	operations          int
	activeActivationIDs map[string]struct{}
	desiredLayouts      []ownedLayoutResource
	desiredAgents       []ownedAgentResource
	desiredSouls        []ownedSoulResource
	desiredHeartbeats   []ownedHeartbeatResource
	desiredJobs         []automationpkg.Job
	desiredTriggers     []automationpkg.Trigger
	desiredBridges      []bridgepkg.BridgeInstance
	layoutOwners        map[string]string
	agentOwners         map[string]string
	soulOwners          map[string]string
	heartbeatOwners     map[string]string
	jobOwners           map[string]string
	triggerOwners       map[string]string
	bridgeOwners        map[string]string
	declaredChannels    []DeclaredChannel
}

var _ resources.ProjectionPlan = (*BundleActivationResourcePlan)(nil)

func (p *BundleActivationResourcePlan) Kind() resources.ResourceKind {
	return BundleActivationResourceKind
}

func (p *BundleActivationResourcePlan) Revision() int64 {
	if p == nil {
		return 0
	}
	return p.revision
}

func (p *BundleActivationResourcePlan) OperationCount() int {
	if p == nil {
		return 0
	}
	return p.operations
}

// Build composes active bundle activations with bundle catalog dependency records.
func (s *Service) Build(
	ctx context.Context,
	activationRecords []resources.Record[ActivationResourceSpec],
	bundleRecords []resources.Record[BundleResourceSpec],
) (resources.ProjectionPlan, error) {
	if err := s.checkReady(ctx); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()

	activations := make([]Activation, 0, len(activationRecords))
	var revision int64
	for _, record := range activationRecords {
		if record.Version > revision {
			revision = record.Version
		}
		activations = append(activations, activationFromResourceRecord(record))
	}
	for _, record := range bundleRecords {
		if record.Version > revision {
			revision = record.Version
		}
	}
	if _, err := s.reconcileNetworkRequirementDigests(ctx, activations); err != nil {
		return nil, err
	}

	state, err := s.collectDesiredStateFromBundleRecords(ctx, activations, bundleRecords)
	if err != nil {
		return nil, err
	}
	operations := len(state.desiredLayouts) + len(state.desiredAgents) +
		len(state.desiredSouls) + len(state.desiredHeartbeats) +
		len(state.desiredJobs) + len(state.desiredTriggers) + len(state.desiredBridges)
	owners := ownedResourceMaps(state.inventoryByActivation)
	return &BundleActivationResourcePlan{
		revision:            revision,
		operations:          operations,
		activeActivationIDs: state.activeActivationIDs,
		desiredLayouts:      state.desiredLayouts,
		desiredAgents:       state.desiredAgents,
		desiredSouls:        state.desiredSouls,
		desiredHeartbeats:   state.desiredHeartbeats,
		desiredJobs:         state.desiredJobs,
		desiredTriggers:     state.desiredTriggers,
		desiredBridges:      state.desiredBridges,
		layoutOwners:        owners.layouts,
		agentOwners:         owners.agents,
		soulOwners:          owners.souls,
		heartbeatOwners:     owners.heartbeats,
		jobOwners:           owners.jobs,
		triggerOwners:       owners.triggers,
		bridgeOwners:        owners.bridges,
		declaredChannels:    state.declaredChannels,
	}, nil
}

type ownedResourceOwnerMaps struct {
	layouts    map[string]string
	agents     map[string]string
	souls      map[string]string
	heartbeats map[string]string
	jobs       map[string]string
	triggers   map[string]string
	bridges    map[string]string
}

func ownedResourceMaps(inventoryByActivation map[string][]InventoryItem) ownedResourceOwnerMaps {
	counts := ownedResourceOwnerCounts{}
	for _, items := range inventoryByActivation {
		for _, item := range items {
			counts.add(resources.ResourceKind(strings.TrimSpace(item.ResourceKind)))
		}
	}
	owners := ownedResourceOwnerMaps{
		layouts:    make(map[string]string, counts.layouts),
		agents:     make(map[string]string, counts.agents),
		souls:      make(map[string]string, counts.souls),
		heartbeats: make(map[string]string, counts.heartbeats),
		jobs:       make(map[string]string, counts.jobs),
		triggers:   make(map[string]string, counts.triggers),
		bridges:    make(map[string]string, counts.bridges),
	}
	for activationID, items := range inventoryByActivation {
		ownerID := strings.TrimSpace(activationID)
		for _, item := range items {
			switch resources.ResourceKind(strings.TrimSpace(item.ResourceKind)) {
			case windowmanager.WindowLayoutResourceKind:
				owners.layouts[strings.TrimSpace(item.ResourceID)] = ownerID
			case aghconfig.AgentResourceKind:
				owners.agents[strings.TrimSpace(item.ResourceID)] = ownerID
			case soul.ResourceKind:
				owners.souls[strings.TrimSpace(item.ResourceID)] = ownerID
			case heartbeat.ResourceKind:
				owners.heartbeats[strings.TrimSpace(item.ResourceID)] = ownerID
			case automationpkg.JobResourceKind:
				owners.jobs[strings.TrimSpace(item.ResourceID)] = ownerID
			case automationpkg.TriggerResourceKind:
				owners.triggers[strings.TrimSpace(item.ResourceID)] = ownerID
			case bridgepkg.BridgeInstanceResourceKind:
				owners.bridges[strings.TrimSpace(item.ResourceID)] = ownerID
			}
		}
	}
	return owners
}

type ownedResourceOwnerCounts struct {
	layouts    int
	agents     int
	souls      int
	heartbeats int
	jobs       int
	triggers   int
	bridges    int
}

func (c *ownedResourceOwnerCounts) add(kind resources.ResourceKind) {
	switch kind {
	case windowmanager.WindowLayoutResourceKind:
		c.layouts++
	case aghconfig.AgentResourceKind:
		c.agents++
	case soul.ResourceKind:
		c.souls++
	case heartbeat.ResourceKind:
		c.heartbeats++
	case automationpkg.JobResourceKind:
		c.jobs++
	case automationpkg.TriggerResourceKind:
		c.triggers++
	case bridgepkg.BridgeInstanceResourceKind:
		c.bridges++
	}
}

// Apply writes activation-owned desired-state records through canonical stores.
func (s *Service) Apply(ctx context.Context, plan resources.ProjectionPlan) error {
	if err := s.checkReady(ctx); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	typed, ok := plan.(*BundleActivationResourcePlan)
	if !ok {
		return fmt.Errorf("bundles: activation resource plan has type %T", plan)
	}
	if typed == nil {
		return errors.New("bundles: activation resource plan is required")
	}
	if err := s.store.ApplyBundleActivationResources(ctx, *typed); err != nil {
		return err
	}
	s.applyNetworkSettings(typed.declaredChannels)
	return nil
}

func (s *Service) collectDesiredStateFromBundleRecords(
	ctx context.Context,
	activations []Activation,
	bundleRecords []resources.Record[BundleResourceSpec],
) (reconcileState, error) {
	bundleLookup := newBundleRecordLookup(bundleRecords)
	capacity := estimateDesiredStateCapacity(activations, bundleLookup)
	state := reconcileState{
		activeActivationIDs:   make(map[string]struct{}, len(activations)),
		desiredLayouts:        make([]ownedLayoutResource, 0, capacity.layouts),
		desiredAgents:         make([]ownedAgentResource, 0, capacity.agents),
		desiredSouls:          make([]ownedSoulResource, 0, capacity.souls),
		desiredHeartbeats:     make([]ownedHeartbeatResource, 0, capacity.heartbeats),
		desiredJobs:           make([]automationpkg.Job, 0, capacity.jobs),
		desiredTriggers:       make([]automationpkg.Trigger, 0, capacity.triggers),
		desiredBridges:        make([]bridgepkg.BridgeInstance, 0, capacity.bridges),
		inventoryByActivation: make(map[string][]InventoryItem, len(activations)),
		declaredChannels:      make([]DeclaredChannel, 0, capacity.channels),
	}

	errs := make([]error, 0)
	for _, activation := range activations {
		state.activeActivationIDs[strings.TrimSpace(activation.ID)] = struct{}{}
		resolved, resolveErr := s.resolveActivationFromBundleLookup(ctx, activation, bundleLookup)
		if resolveErr != nil {
			errs = append(errs, resolveErr)
			state.inventoryByActivation[activation.ID] = nil
			continue
		}

		state.inventoryByActivation[activation.ID] = cloneInventoryItems(resolved.inventory)
		state.declaredChannels = append(state.declaredChannels, resolved.channels...)
		state.desiredLayouts = append(state.desiredLayouts, resolved.layouts...)
		state.desiredAgents = append(state.desiredAgents, resolved.agents...)
		state.desiredSouls = append(state.desiredSouls, resolved.souls...)
		state.desiredHeartbeats = append(state.desiredHeartbeats, resolved.heartbeats...)
		state.desiredJobs = append(state.desiredJobs, resolved.jobs...)
		state.desiredTriggers = append(state.desiredTriggers, resolved.triggers...)
		state.desiredBridges = append(state.desiredBridges, resolved.bridges...)
		s.warnSpecHashDrift(ctx, activation, resolved.specContentHash)
	}
	if err := validateDesiredAgentScopeConflicts(state.desiredAgents); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return reconcileState{}, errors.Join(errs...)
	}
	return state, nil
}

type desiredStateCapacity struct {
	layouts    int
	agents     int
	souls      int
	heartbeats int
	jobs       int
	triggers   int
	bridges    int
	channels   int
}

func estimateDesiredStateCapacity(
	activations []Activation,
	bundleLookup bundleRecordLookup,
) desiredStateCapacity {
	var capacity desiredStateCapacity
	for _, activation := range activations {
		bundleRecord, ok := findBundleResourceRecordIndexed(
			bundleLookup,
			activation.ExtensionName,
			activation.BundleName,
		)
		if !ok {
			continue
		}
		profile, ok := findProfile(bundleRecord.Spec.Bundle.Profiles, activation.ProfileName)
		if !ok {
			continue
		}
		agentCount := len(profile.Agents)
		capacity.layouts += len(profile.Layouts)
		capacity.agents += agentCount
		capacity.souls += agentCount
		capacity.heartbeats += agentCount
		capacity.jobs += len(profile.Jobs)
		capacity.triggers += len(profile.Triggers)
		capacity.bridges += len(profile.Bridges)
		capacity.channels += len(profile.Channels.Items)
	}
	return capacity
}

func (s *Service) resolveActivationFromBundleLookup(
	ctx context.Context,
	activation Activation,
	bundleLookup bundleRecordLookup,
) (resolvedActivation, error) {
	activation, err := activation.Validated()
	if err != nil {
		return resolvedActivation{}, err
	}
	bundleRecord, ok := findBundleResourceRecordIndexed(
		bundleLookup,
		activation.ExtensionName,
		activation.BundleName,
	)
	if !ok {
		return resolvedActivation{}, fmt.Errorf(
			"%w: %s/%s",
			ErrBundleNotFound,
			activation.ExtensionName,
			activation.BundleName,
		)
	}
	bundle := cloneBundleSpec(bundleRecord.Spec.Bundle)
	profile, ok := findProfile(bundle.Profiles, activation.ProfileName)
	if !ok {
		return resolvedActivation{}, fmt.Errorf(
			"%w: %s/%s/%s",
			ErrProfileNotFound,
			activation.ExtensionName,
			activation.BundleName,
			activation.ProfileName,
		)
	}
	specContentHash, err := bundleProfileSpecContentHash(bundle, profile)
	if err != nil {
		return resolvedActivation{}, err
	}
	resolved := resolvedActivation{
		activation:      activation,
		bundleRecord:    bundleRecord,
		bundle:          bundle,
		profile:         profile,
		specContentHash: specContentHash,
	}
	resolved.channels = declaredChannelsForProfile(activation, bundle, profile)
	materialized, err := s.materializeActivationResources(ctx, activation, bundleRecord, bundle, profile)
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

func cloneStringSet(values map[string]struct{}) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]struct{}, len(values))
	for value := range values {
		cloned[strings.TrimSpace(value)] = struct{}{}
	}
	return cloned
}

func cloneOwnedAgentResources(values []ownedAgentResource) []ownedAgentResource {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]ownedAgentResource, 0, len(values))
	for _, value := range values {
		cloned = append(cloned, ownedAgentResource{
			ID:    strings.TrimSpace(value.ID),
			Scope: value.Scope.Normalize(),
			Spec:  aghconfig.CloneAgentDef(value.Spec),
		})
	}
	return cloned
}

func cloneOwnedSoulResources(values []ownedSoulResource) []ownedSoulResource {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]ownedSoulResource, 0, len(values))
	for _, value := range values {
		next := value
		next.ID = strings.TrimSpace(next.ID)
		next.Scope = next.Scope.Normalize()
		cloned = append(cloned, next)
	}
	return cloned
}

func cloneOwnedHeartbeatResources(values []ownedHeartbeatResource) []ownedHeartbeatResource {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]ownedHeartbeatResource, 0, len(values))
	for _, value := range values {
		next := value
		next.ID = strings.TrimSpace(next.ID)
		next.Scope = next.Scope.Normalize()
		cloned = append(cloned, next)
	}
	return cloned
}

func cloneJobsForBundle(values []automationpkg.Job) []automationpkg.Job {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]automationpkg.Job, 0, len(values))
	for _, value := range values {
		next := value
		if value.Schedule != nil {
			schedule := *value.Schedule
			next.Schedule = &schedule
		}
		next.Task = cloneTaskConfig(value.Task)
		cloned = append(cloned, next)
	}
	return cloned
}

func cloneTriggersForBundle(values []automationpkg.Trigger) []automationpkg.Trigger {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]automationpkg.Trigger, 0, len(values))
	for _, value := range values {
		next := value
		next.Filter = cloneStringMap(value.Filter)
		cloned = append(cloned, next)
	}
	return cloned
}

func cloneBridgeInstancesForBundle(values []bridgepkg.BridgeInstance) []bridgepkg.BridgeInstance {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]bridgepkg.BridgeInstance, 0, len(values))
	for _, value := range values {
		next := value
		next.ProviderConfig = cloneRawMessage(value.ProviderConfig)
		next.DeliveryDefaults = cloneRawMessage(value.DeliveryDefaults)
		if value.Degradation != nil {
			degradation := *value.Degradation
			next.Degradation = &degradation
		}
		cloned = append(cloned, next)
	}
	return cloned
}
