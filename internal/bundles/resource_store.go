package bundles

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/soul"
	"github.com/compozy/agh/internal/windowmanager"
)

const (
	resourceStoreBundleResourceValue = "bundle-resource"
)

// ResourceStore persists bundles, activations, and owned activation fan-out through canonical resources.
type ResourceStore struct {
	bundles         resources.Store[BundleResourceSpec]
	bundleCodec     resources.KindCodec[BundleResourceSpec]
	activations     resources.Store[ActivationResourceSpec]
	activationCodec resources.KindCodec[ActivationResourceSpec]
	layouts         resources.Store[windowmanager.LayoutResource]
	layoutCodec     resources.KindCodec[windowmanager.LayoutResource]
	agents          resources.Store[aghconfig.AgentDef]
	agentCodec      resources.KindCodec[aghconfig.AgentDef]
	souls           resources.Store[soul.ResourceSpec]
	soulCodec       resources.KindCodec[soul.ResourceSpec]
	heartbeats      resources.Store[heartbeat.ResourceSpec]
	heartbeatCodec  resources.KindCodec[heartbeat.ResourceSpec]
	jobs            resources.Store[automationpkg.Job]
	jobCodec        resources.KindCodec[automationpkg.Job]
	triggers        resources.Store[automationpkg.Trigger]
	triggerCodec    resources.KindCodec[automationpkg.Trigger]
	bridges         resources.Store[bridgepkg.BridgeInstanceSpec]
	bridgeCodec     resources.KindCodec[bridgepkg.BridgeInstanceSpec]
	actor           resources.MutationActor
	trigger         func(context.Context, resources.ResourceKind, resources.ReconcileReason) error
	now             func() time.Time
}

// ResourceStoreConfig groups the typed stores used by bundle resource projection.
type ResourceStoreConfig struct {
	Bundles         resources.Store[BundleResourceSpec]
	BundleCodec     resources.KindCodec[BundleResourceSpec]
	Activations     resources.Store[ActivationResourceSpec]
	ActivationCodec resources.KindCodec[ActivationResourceSpec]
	Layouts         resources.Store[windowmanager.LayoutResource]
	LayoutCodec     resources.KindCodec[windowmanager.LayoutResource]
	Agents          resources.Store[aghconfig.AgentDef]
	AgentCodec      resources.KindCodec[aghconfig.AgentDef]
	Souls           resources.Store[soul.ResourceSpec]
	SoulCodec       resources.KindCodec[soul.ResourceSpec]
	Heartbeats      resources.Store[heartbeat.ResourceSpec]
	HeartbeatCodec  resources.KindCodec[heartbeat.ResourceSpec]
	Jobs            resources.Store[automationpkg.Job]
	JobCodec        resources.KindCodec[automationpkg.Job]
	Triggers        resources.Store[automationpkg.Trigger]
	TriggerCodec    resources.KindCodec[automationpkg.Trigger]
	Bridges         resources.Store[bridgepkg.BridgeInstanceSpec]
	BridgeCodec     resources.KindCodec[bridgepkg.BridgeInstanceSpec]
	Actor           resources.MutationActor
	Trigger         func(context.Context, resources.ResourceKind, resources.ReconcileReason) error
	Now             func() time.Time
}

var _ Store = (*ResourceStore)(nil)

var bundleActivationOwnedKindAllowlist = map[resources.ResourceKind]struct{}{
	windowmanager.WindowLayoutResourceKind: {},
	aghconfig.AgentResourceKind:            {},
	soul.ResourceKind:                      {},
	heartbeat.ResourceKind:                 {},
	automationpkg.JobResourceKind:          {},
	automationpkg.TriggerResourceKind:      {},
	bridgepkg.BridgeInstanceResourceKind:   {},
}

func bundleActivationOwnedKindAllowed(kind resources.ResourceKind) bool {
	_, ok := bundleActivationOwnedKindAllowlist[kind.Normalize()]
	return ok
}

// NewResourceStore constructs a resource-backed bundle store.
func NewResourceStore(cfg ResourceStoreConfig) (*ResourceStore, error) {
	if err := validateResourceStoreCoreConfig(cfg); err != nil {
		return nil, err
	}
	if err := validateResourceStoreProjectionConfig(cfg); err != nil {
		return nil, err
	}
	if cfg.Actor.Kind == "" {
		cfg.Actor = defaultBundleResourceActor()
	}
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Now().UTC() }
	}
	return &ResourceStore{
		bundles:         cfg.Bundles,
		bundleCodec:     cfg.BundleCodec,
		activations:     cfg.Activations,
		activationCodec: cfg.ActivationCodec,
		layouts:         cfg.Layouts,
		layoutCodec:     cfg.LayoutCodec,
		agents:          cfg.Agents,
		agentCodec:      cfg.AgentCodec,
		souls:           cfg.Souls,
		soulCodec:       cfg.SoulCodec,
		heartbeats:      cfg.Heartbeats,
		heartbeatCodec:  cfg.HeartbeatCodec,
		jobs:            cfg.Jobs,
		jobCodec:        cfg.JobCodec,
		triggers:        cfg.Triggers,
		triggerCodec:    cfg.TriggerCodec,
		bridges:         cfg.Bridges,
		bridgeCodec:     cfg.BridgeCodec,
		actor:           cfg.Actor,
		trigger:         cfg.Trigger,
		now:             cfg.Now,
	}, nil
}

func validateResourceStoreCoreConfig(cfg ResourceStoreConfig) error {
	if cfg.Bundles == nil {
		return errors.New("bundles: bundle resource store is required")
	}
	if cfg.BundleCodec == nil {
		return errors.New("bundles: bundle resource codec is required")
	}
	if cfg.Activations == nil {
		return errors.New("bundles: activation resource store is required")
	}
	if cfg.ActivationCodec == nil {
		return errors.New("bundles: activation resource codec is required")
	}
	if cfg.Layouts == nil || cfg.LayoutCodec == nil {
		return errors.New("bundles: window layout resource store and codec are required")
	}
	if cfg.Agents == nil || cfg.AgentCodec == nil {
		return errors.New("bundles: agent resource store and codec are required")
	}
	if cfg.Souls == nil || cfg.SoulCodec == nil {
		return errors.New("bundles: soul resource store and codec are required")
	}
	if cfg.Heartbeats == nil || cfg.HeartbeatCodec == nil {
		return errors.New("bundles: heartbeat resource store and codec are required")
	}
	return nil
}

func validateResourceStoreProjectionConfig(cfg ResourceStoreConfig) error {
	if cfg.Jobs == nil || cfg.JobCodec == nil {
		return errors.New("bundles: automation job resource store and codec are required")
	}
	if cfg.Triggers == nil || cfg.TriggerCodec == nil {
		return errors.New("bundles: automation trigger resource store and codec are required")
	}
	if cfg.Bridges == nil || cfg.BridgeCodec == nil {
		return errors.New("bundles: bridge instance resource store and codec are required")
	}
	return nil
}

func defaultBundleResourceActor() resources.MutationActor {
	return resources.MutationActor{
		Kind: resources.MutationActorKindDaemon,
		ID:   resourceStoreBundleResourceValue,
		Source: resources.ResourceSource{
			Kind: resources.ResourceSourceKind("daemon"),
			ID:   resourceStoreBundleResourceValue,
		},
		MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
	}
}

func (s *ResourceStore) ListBundleResources(
	ctx context.Context,
) ([]resources.Record[BundleResourceSpec], error) {
	records, err := s.bundles.List(ctx, s.actor, resources.ResourceFilter{Kind: BundleResourceKind})
	if err != nil {
		return nil, fmt.Errorf("bundles: list bundle resources: %w", err)
	}
	slices.SortFunc(records, func(left, right resources.Record[BundleResourceSpec]) int {
		if cmp := strings.Compare(left.Spec.ExtensionName, right.Spec.ExtensionName); cmp != 0 {
			return cmp
		}
		return strings.Compare(left.Spec.Bundle.Name, right.Spec.Bundle.Name)
	})
	return records, nil
}

func (s *ResourceStore) ListAgentResources(
	ctx context.Context,
) ([]resources.Record[aghconfig.AgentDef], error) {
	records, err := s.agents.List(ctx, s.actor, resources.ResourceFilter{Kind: aghconfig.AgentResourceKind})
	if err != nil {
		return nil, fmt.Errorf("bundles: list agent resources: %w", err)
	}
	return records, nil
}

func (s *ResourceStore) ListBundleActivationInventory(
	ctx context.Context,
	activationID string,
) ([]InventoryItem, error) {
	owner := ownerForActivation(activationID)
	if err := owner.Validate("owner"); err != nil {
		return nil, err
	}
	items := make([]InventoryItem, 0)
	collectors := []func(context.Context, resources.ResourceOwner) ([]InventoryItem, error){
		s.listOwnedLayoutInventory,
		s.listOwnedAgentInventory,
		s.listOwnedSoulInventory,
		s.listOwnedHeartbeatInventory,
		s.listOwnedJobInventory,
		s.listOwnedTriggerInventory,
		s.listOwnedBridgeInventory,
	}
	for _, collect := range collectors {
		collected, err := collect(ctx, owner)
		if err != nil {
			return nil, err
		}
		items = append(items, collected...)
	}
	slices.SortFunc(items, compareInventoryItems)
	return items, nil
}

func (s *ResourceStore) listOwnedAgentInventory(
	ctx context.Context,
	owner resources.ResourceOwner,
) ([]InventoryItem, error) {
	return listOwnedInventoryForKind(
		ctx,
		s.agents,
		s.actor,
		owner,
		aghconfig.AgentResourceKind,
		"agents",
		func(spec aghconfig.AgentDef) string { return spec.Name },
	)
}

func (s *ResourceStore) listOwnedSoulInventory(
	ctx context.Context,
	owner resources.ResourceOwner,
) ([]InventoryItem, error) {
	return listOwnedInventoryForKind(
		ctx,
		s.souls,
		s.actor,
		owner,
		soul.ResourceKind,
		"soul resources",
		func(spec soul.ResourceSpec) string { return spec.AgentName },
	)
}

func (s *ResourceStore) listOwnedHeartbeatInventory(
	ctx context.Context,
	owner resources.ResourceOwner,
) ([]InventoryItem, error) {
	return listOwnedInventoryForKind(
		ctx,
		s.heartbeats,
		s.actor,
		owner,
		heartbeat.ResourceKind,
		"heartbeat resources",
		func(spec heartbeat.ResourceSpec) string { return spec.AgentName },
	)
}

func (s *ResourceStore) listOwnedJobInventory(
	ctx context.Context,
	owner resources.ResourceOwner,
) ([]InventoryItem, error) {
	return listOwnedInventoryForKind(
		ctx,
		s.jobs,
		s.actor,
		owner,
		automationpkg.JobResourceKind,
		"automation jobs",
		func(spec automationpkg.Job) string { return spec.Name },
	)
}

func (s *ResourceStore) listOwnedTriggerInventory(
	ctx context.Context,
	owner resources.ResourceOwner,
) ([]InventoryItem, error) {
	return listOwnedInventoryForKind(
		ctx,
		s.triggers,
		s.actor,
		owner,
		automationpkg.TriggerResourceKind,
		"automation triggers",
		func(spec automationpkg.Trigger) string { return spec.Name },
	)
}

func (s *ResourceStore) listOwnedBridgeInventory(
	ctx context.Context,
	owner resources.ResourceOwner,
) ([]InventoryItem, error) {
	return listOwnedInventoryForKind(
		ctx,
		s.bridges,
		s.actor,
		owner,
		bridgepkg.BridgeInstanceResourceKind,
		"bridge instances",
		func(spec bridgepkg.BridgeInstanceSpec) string { return spec.DisplayName },
	)
}

func listOwnedInventoryForKind[T any](
	ctx context.Context,
	store resources.Store[T],
	actor resources.MutationActor,
	owner resources.ResourceOwner,
	kind resources.ResourceKind,
	label string,
	resourceName func(T) string,
) ([]InventoryItem, error) {
	records, err := store.List(ctx, actor, resources.ResourceFilter{Kind: kind, Owner: &owner})
	if err != nil {
		return nil, fmt.Errorf("bundles: list owned %s: %w", label, err)
	}
	items := make([]InventoryItem, 0, len(records))
	for _, record := range records {
		items = append(items, InventoryItem{
			ActivationID:  owner.ID,
			ResourceKind:  string(kind),
			ResourceID:    record.ID,
			ResourceName:  resourceName(record.Spec),
			RecordedAtUTC: record.UpdatedAt,
		})
	}
	return items, nil
}

func (s *ResourceStore) ApplyBundleActivationResources(
	ctx context.Context,
	plan BundleActivationResourcePlan,
) error {
	snapshot, err := s.snapshotOwnedBundleActivationResources(ctx)
	if err != nil {
		return err
	}
	changed := make(map[resources.ResourceKind]struct{}, 7)
	if err := s.applyBundleActivationResourcePlan(ctx, plan, changed); err != nil {
		rollbackCtx := context.WithoutCancel(ctx)
		if deadline, ok := ctx.Deadline(); ok {
			var cancel context.CancelFunc
			rollbackCtx, cancel = context.WithDeadline(rollbackCtx, deadline)
			defer cancel()
		}
		if rollbackErr := s.restoreOwnedBundleActivationResources(rollbackCtx, snapshot); rollbackErr != nil {
			return errors.Join(err, fmt.Errorf("bundles: rollback owned activation resources: %w", rollbackErr))
		}
		return err
	}
	return s.triggerChangedKinds(ctx, changed)
}

func (s *ResourceStore) applyBundleActivationResourcePlan(
	ctx context.Context,
	plan BundleActivationResourcePlan,
	changed map[resources.ResourceKind]struct{},
) error {
	if err := s.syncOwnedAgentResources(ctx, plan, changed); err != nil {
		return err
	}
	if err := s.syncOwnedSoulResources(ctx, plan, changed); err != nil {
		return err
	}
	if err := s.syncOwnedHeartbeatResources(ctx, plan, changed); err != nil {
		return err
	}
	if err := s.syncOwnedJobResources(ctx, plan, changed); err != nil {
		return err
	}
	if err := s.syncOwnedTriggerResources(ctx, plan, changed); err != nil {
		return err
	}
	if err := s.syncOwnedBridgeResources(ctx, plan, changed); err != nil {
		return err
	}
	return s.syncOwnedLayoutResources(ctx, plan, changed)
}
