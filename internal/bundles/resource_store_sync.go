package bundles

import (
	"context"

	"fmt"

	"strings"

	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/soul"
)

func (s *ResourceStore) syncOwnedAgentResources(
	ctx context.Context,
	plan BundleActivationResourcePlan,
	changed map[resources.ResourceKind]struct{},
) error {
	desired := make(map[string]map[string]ownedAgentResource)
	for _, agent := range plan.desiredAgents {
		ownerID := strings.TrimSpace(plan.agentOwners[strings.TrimSpace(agent.ID)])
		if ownerID == "" {
			return fmt.Errorf("bundles: owned agent %q has no activation owner", agent.ID)
		}
		if desired[ownerID] == nil {
			desired[ownerID] = make(map[string]ownedAgentResource)
		}
		desired[ownerID][agent.ID] = agent
	}
	return syncOwnedResources(
		aghconfig.AgentResourceKind,
		plan.activeActivationIDs,
		changed,
		func(owner resources.ResourceOwner) ([]resources.Record[aghconfig.AgentDef], error) {
			return s.agents.List(ctx, s.actor, resources.ResourceFilter{
				Kind:  aghconfig.AgentResourceKind,
				Owner: &owner,
			})
		},
		func(ownerID string, current map[string]resources.Record[aghconfig.AgentDef]) error {
			return s.upsertOwnedAgents(ctx, ownerID, current, desired[ownerID], changed)
		},
		func(ownerID string) resources.MutationActor { return activationResourceActor(s.actor, ownerID) },
		func(actor resources.MutationActor, stale resources.Record[aghconfig.AgentDef]) error {
			return s.agents.Delete(ctx, actor, stale.ID, stale.Version)
		},
		func() ([]resources.Record[aghconfig.AgentDef], error) {
			return s.agents.List(ctx, s.actor, resources.ResourceFilter{Kind: aghconfig.AgentResourceKind})
		},
	)
}

func (s *ResourceStore) syncOwnedSoulResources(
	ctx context.Context,
	plan BundleActivationResourcePlan,
	changed map[resources.ResourceKind]struct{},
) error {
	desired := make(map[string]map[string]ownedSoulResource)
	for _, spec := range plan.desiredSouls {
		ownerID := strings.TrimSpace(plan.soulOwners[strings.TrimSpace(spec.ID)])
		if ownerID == "" {
			return fmt.Errorf("bundles: owned soul resource %q has no activation owner", spec.ID)
		}
		if desired[ownerID] == nil {
			desired[ownerID] = make(map[string]ownedSoulResource)
		}
		desired[ownerID][spec.ID] = spec
	}
	return syncOwnedResources(
		soul.ResourceKind,
		plan.activeActivationIDs,
		changed,
		func(owner resources.ResourceOwner) ([]resources.Record[soul.ResourceSpec], error) {
			return s.souls.List(ctx, s.actor, resources.ResourceFilter{Kind: soul.ResourceKind, Owner: &owner})
		},
		func(ownerID string, current map[string]resources.Record[soul.ResourceSpec]) error {
			return s.upsertOwnedSouls(ctx, ownerID, current, desired[ownerID], changed)
		},
		func(ownerID string) resources.MutationActor { return activationResourceActor(s.actor, ownerID) },
		func(actor resources.MutationActor, stale resources.Record[soul.ResourceSpec]) error {
			return s.souls.Delete(ctx, actor, stale.ID, stale.Version)
		},
		func() ([]resources.Record[soul.ResourceSpec], error) {
			return s.souls.List(ctx, s.actor, resources.ResourceFilter{Kind: soul.ResourceKind})
		},
	)
}

func (s *ResourceStore) syncOwnedHeartbeatResources(
	ctx context.Context,
	plan BundleActivationResourcePlan,
	changed map[resources.ResourceKind]struct{},
) error {
	desired := make(map[string]map[string]ownedHeartbeatResource)
	for _, spec := range plan.desiredHeartbeats {
		ownerID := strings.TrimSpace(plan.heartbeatOwners[strings.TrimSpace(spec.ID)])
		if ownerID == "" {
			return fmt.Errorf("bundles: owned heartbeat resource %q has no activation owner", spec.ID)
		}
		if desired[ownerID] == nil {
			desired[ownerID] = make(map[string]ownedHeartbeatResource)
		}
		desired[ownerID][spec.ID] = spec
	}
	return syncOwnedResources(
		heartbeat.ResourceKind,
		plan.activeActivationIDs,
		changed,
		func(owner resources.ResourceOwner) ([]resources.Record[heartbeat.ResourceSpec], error) {
			return s.heartbeats.List(
				ctx,
				s.actor,
				resources.ResourceFilter{Kind: heartbeat.ResourceKind, Owner: &owner},
			)
		},
		func(ownerID string, current map[string]resources.Record[heartbeat.ResourceSpec]) error {
			return s.upsertOwnedHeartbeats(ctx, ownerID, current, desired[ownerID], changed)
		},
		func(ownerID string) resources.MutationActor { return activationResourceActor(s.actor, ownerID) },
		func(actor resources.MutationActor, stale resources.Record[heartbeat.ResourceSpec]) error {
			return s.heartbeats.Delete(ctx, actor, stale.ID, stale.Version)
		},
		func() ([]resources.Record[heartbeat.ResourceSpec], error) {
			return s.heartbeats.List(ctx, s.actor, resources.ResourceFilter{Kind: heartbeat.ResourceKind})
		},
	)
}

func (s *ResourceStore) syncOwnedJobResources(
	ctx context.Context,
	plan BundleActivationResourcePlan,
	changed map[resources.ResourceKind]struct{},
) error {
	desired := make(map[string]map[string]automationpkg.Job)
	for _, job := range plan.desiredJobs {
		ownerID := strings.TrimSpace(plan.jobOwners[strings.TrimSpace(job.ID)])
		if ownerID == "" {
			return fmt.Errorf("bundles: owned automation job %q has no activation owner", job.ID)
		}
		if desired[ownerID] == nil {
			desired[ownerID] = make(map[string]automationpkg.Job)
		}
		desired[ownerID][job.ID] = job
	}
	return syncOwnedResources(
		automationpkg.JobResourceKind,
		plan.activeActivationIDs,
		changed,
		func(owner resources.ResourceOwner) ([]resources.Record[automationpkg.Job], error) {
			return s.jobs.List(
				ctx,
				s.actor,
				resources.ResourceFilter{Kind: automationpkg.JobResourceKind, Owner: &owner},
			)
		},
		func(ownerID string, current map[string]resources.Record[automationpkg.Job]) error {
			return s.upsertOwnedJobs(ctx, ownerID, current, desired[ownerID], changed)
		},
		func(ownerID string) resources.MutationActor { return activationResourceActor(s.actor, ownerID) },
		func(actor resources.MutationActor, stale resources.Record[automationpkg.Job]) error {
			return s.jobs.Delete(ctx, actor, stale.ID, stale.Version)
		},
		func() ([]resources.Record[automationpkg.Job], error) {
			return s.jobs.List(ctx, s.actor, resources.ResourceFilter{Kind: automationpkg.JobResourceKind})
		},
	)
}

func (s *ResourceStore) syncOwnedTriggerResources(
	ctx context.Context,
	plan BundleActivationResourcePlan,
	changed map[resources.ResourceKind]struct{},
) error {
	desired := make(map[string]map[string]automationpkg.Trigger)
	for _, trigger := range plan.desiredTriggers {
		ownerID := strings.TrimSpace(plan.triggerOwners[strings.TrimSpace(trigger.ID)])
		if ownerID == "" {
			return fmt.Errorf("bundles: owned automation trigger %q has no activation owner", trigger.ID)
		}
		if desired[ownerID] == nil {
			desired[ownerID] = make(map[string]automationpkg.Trigger)
		}
		desired[ownerID][trigger.ID] = trigger
	}
	return syncOwnedResources(
		automationpkg.TriggerResourceKind,
		plan.activeActivationIDs,
		changed,
		func(owner resources.ResourceOwner) ([]resources.Record[automationpkg.Trigger], error) {
			return s.triggers.List(ctx, s.actor, resources.ResourceFilter{
				Kind:  automationpkg.TriggerResourceKind,
				Owner: &owner,
			})
		},
		func(ownerID string, current map[string]resources.Record[automationpkg.Trigger]) error {
			return s.upsertOwnedTriggers(ctx, ownerID, current, desired[ownerID], changed)
		},
		func(ownerID string) resources.MutationActor { return activationResourceActor(s.actor, ownerID) },
		func(actor resources.MutationActor, stale resources.Record[automationpkg.Trigger]) error {
			return s.triggers.Delete(ctx, actor, stale.ID, stale.Version)
		},
		func() ([]resources.Record[automationpkg.Trigger], error) {
			return s.triggers.List(ctx, s.actor, resources.ResourceFilter{Kind: automationpkg.TriggerResourceKind})
		},
	)
}

func (s *ResourceStore) syncOwnedBridgeResources(
	ctx context.Context,
	plan BundleActivationResourcePlan,
	changed map[resources.ResourceKind]struct{},
) error {
	desired := make(map[string]map[string]bridgepkg.BridgeInstanceSpec)
	for _, instance := range plan.desiredBridges {
		ownerID := strings.TrimSpace(plan.bridgeOwners[strings.TrimSpace(instance.ID)])
		if ownerID == "" {
			return fmt.Errorf("bundles: owned bridge instance %q has no activation owner", instance.ID)
		}
		if desired[ownerID] == nil {
			desired[ownerID] = make(map[string]bridgepkg.BridgeInstanceSpec)
		}
		desired[ownerID][instance.ID] = bridgepkg.BridgeInstanceSpecFromInstance(instance)
	}
	return syncOwnedResources(
		bridgepkg.BridgeInstanceResourceKind,
		plan.activeActivationIDs,
		changed,
		func(owner resources.ResourceOwner) ([]resources.Record[bridgepkg.BridgeInstanceSpec], error) {
			return s.bridges.List(ctx, s.actor, resources.ResourceFilter{
				Kind:  bridgepkg.BridgeInstanceResourceKind,
				Owner: &owner,
			})
		},
		func(ownerID string, current map[string]resources.Record[bridgepkg.BridgeInstanceSpec]) error {
			return s.upsertOwnedBridges(ctx, ownerID, current, desired[ownerID], changed)
		},
		func(ownerID string) resources.MutationActor { return activationResourceActor(s.actor, ownerID) },
		func(actor resources.MutationActor, stale resources.Record[bridgepkg.BridgeInstanceSpec]) error {
			return s.bridges.Delete(ctx, actor, stale.ID, stale.Version)
		},
		func() ([]resources.Record[bridgepkg.BridgeInstanceSpec], error) {
			return s.bridges.List(ctx, s.actor, resources.ResourceFilter{Kind: bridgepkg.BridgeInstanceResourceKind})
		},
	)
}
