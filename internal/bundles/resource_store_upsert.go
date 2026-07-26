package bundles

import (
	"context"

	"fmt"

	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/soul"
)

func (s *ResourceStore) upsertOwnedJobs(
	ctx context.Context,
	ownerID string,
	current map[string]resources.Record[automationpkg.Job],
	desired map[string]automationpkg.Job,
	changed map[resources.ResourceKind]struct{},
) error {
	actor := activationResourceActor(s.actor, ownerID)
	for id, job := range desired {
		existing, ok := current[id]
		if ok && existing.Scope == automationpkg.ResourceScopeForAutomation(job.Scope, job.WorkspaceID) &&
			s.sameJob(existing, job) {
			delete(current, id)
			continue
		}
		expectedVersion := int64(0)
		if ok {
			expectedVersion = existing.Version
		}
		if _, err := s.jobs.Put(ctx, actor, resources.Draft[automationpkg.Job]{
			ID:              id,
			Scope:           automationpkg.ResourceScopeForAutomation(job.Scope, job.WorkspaceID),
			ExpectedVersion: expectedVersion,
			Spec:            job,
		}); err != nil {
			return fmt.Errorf("bundles: upsert owned automation job %q: %w", id, err)
		}
		changed[automationpkg.JobResourceKind] = struct{}{}
		delete(current, id)
	}
	return deleteStaleOwnedRecords(ctx, actor, current, changed, automationpkg.JobResourceKind, s.jobs.Delete)
}

func (s *ResourceStore) upsertOwnedAgents(
	ctx context.Context,
	ownerID string,
	current map[string]resources.Record[aghconfig.AgentDef],
	desired map[string]ownedAgentResource,
	changed map[resources.ResourceKind]struct{},
) error {
	actor := activationResourceActor(s.actor, ownerID)
	for id, desiredAgent := range desired {
		spec := desiredAgent.Spec
		scope := desiredAgent.Scope.Normalize()
		existing, ok := current[id]
		if ok && existing.Scope == scope && s.sameAgent(existing, spec) {
			delete(current, id)
			continue
		}
		expectedVersion := int64(0)
		if ok {
			expectedVersion = existing.Version
		}
		if _, err := s.agents.Put(ctx, actor, resources.Draft[aghconfig.AgentDef]{
			ID:              id,
			Scope:           scope,
			ExpectedVersion: expectedVersion,
			Spec:            spec,
		}); err != nil {
			return fmt.Errorf("bundles: upsert owned agent %q: %w", id, err)
		}
		changed[aghconfig.AgentResourceKind] = struct{}{}
		delete(current, id)
	}
	return deleteStaleOwnedRecords(ctx, actor, current, changed, aghconfig.AgentResourceKind, s.agents.Delete)
}

func (s *ResourceStore) upsertOwnedSouls(
	ctx context.Context,
	ownerID string,
	current map[string]resources.Record[soul.ResourceSpec],
	desired map[string]ownedSoulResource,
	changed map[resources.ResourceKind]struct{},
) error {
	actor := activationResourceActor(s.actor, ownerID)
	for id, desiredSoul := range desired {
		spec := desiredSoul.Spec
		scope := desiredSoul.Scope.Normalize()
		existing, ok := current[id]
		if ok && existing.Scope == scope && s.sameSoul(existing, spec) {
			delete(current, id)
			continue
		}
		expectedVersion := int64(0)
		if ok {
			expectedVersion = existing.Version
		}
		if _, err := s.souls.Put(ctx, actor, resources.Draft[soul.ResourceSpec]{
			ID:              id,
			Scope:           scope,
			ExpectedVersion: expectedVersion,
			Spec:            spec,
		}); err != nil {
			return fmt.Errorf("bundles: upsert owned soul resource %q: %w", id, err)
		}
		changed[soul.ResourceKind] = struct{}{}
		delete(current, id)
	}
	return deleteStaleOwnedRecords(ctx, actor, current, changed, soul.ResourceKind, s.souls.Delete)
}

func (s *ResourceStore) upsertOwnedHeartbeats(
	ctx context.Context,
	ownerID string,
	current map[string]resources.Record[heartbeat.ResourceSpec],
	desired map[string]ownedHeartbeatResource,
	changed map[resources.ResourceKind]struct{},
) error {
	actor := activationResourceActor(s.actor, ownerID)
	for id, desiredHeartbeat := range desired {
		spec := desiredHeartbeat.Spec
		scope := desiredHeartbeat.Scope.Normalize()
		existing, ok := current[id]
		if ok && existing.Scope == scope && s.sameHeartbeat(existing, spec) {
			delete(current, id)
			continue
		}
		expectedVersion := int64(0)
		if ok {
			expectedVersion = existing.Version
		}
		if _, err := s.heartbeats.Put(ctx, actor, resources.Draft[heartbeat.ResourceSpec]{
			ID:              id,
			Scope:           scope,
			ExpectedVersion: expectedVersion,
			Spec:            spec,
		}); err != nil {
			return fmt.Errorf("bundles: upsert owned heartbeat resource %q: %w", id, err)
		}
		changed[heartbeat.ResourceKind] = struct{}{}
		delete(current, id)
	}
	return deleteStaleOwnedRecords(ctx, actor, current, changed, heartbeat.ResourceKind, s.heartbeats.Delete)
}

func (s *ResourceStore) upsertOwnedTriggers(
	ctx context.Context,
	ownerID string,
	current map[string]resources.Record[automationpkg.Trigger],
	desired map[string]automationpkg.Trigger,
	changed map[resources.ResourceKind]struct{},
) error {
	actor := activationResourceActor(s.actor, ownerID)
	for id, trigger := range desired {
		existing, ok := current[id]
		if ok && existing.Scope == automationpkg.ResourceScopeForAutomation(trigger.Scope, trigger.WorkspaceID) &&
			s.sameTrigger(existing, trigger) {
			delete(current, id)
			continue
		}
		expectedVersion := int64(0)
		if ok {
			expectedVersion = existing.Version
		}
		if _, err := s.triggers.Put(ctx, actor, resources.Draft[automationpkg.Trigger]{
			ID:              id,
			Scope:           automationpkg.ResourceScopeForAutomation(trigger.Scope, trigger.WorkspaceID),
			ExpectedVersion: expectedVersion,
			Spec:            trigger,
		}); err != nil {
			return fmt.Errorf("bundles: upsert owned automation trigger %q: %w", id, err)
		}
		changed[automationpkg.TriggerResourceKind] = struct{}{}
		delete(current, id)
	}
	return deleteStaleOwnedRecords(ctx, actor, current, changed, automationpkg.TriggerResourceKind, s.triggers.Delete)
}

func (s *ResourceStore) upsertOwnedBridges(
	ctx context.Context,
	ownerID string,
	current map[string]resources.Record[bridgepkg.BridgeInstanceSpec],
	desired map[string]bridgepkg.BridgeInstanceSpec,
	changed map[resources.ResourceKind]struct{},
) error {
	actor := activationResourceActor(s.actor, ownerID)
	for id, spec := range desired {
		existing, ok := current[id]
		if ok && existing.Scope == bridgepkg.ResourceScopeForBridge(spec.Scope, spec.WorkspaceID) &&
			s.sameBridge(existing, spec) {
			delete(current, id)
			continue
		}
		expectedVersion := int64(0)
		if ok {
			expectedVersion = existing.Version
		}
		if _, err := s.bridges.Put(ctx, actor, resources.Draft[bridgepkg.BridgeInstanceSpec]{
			ID:              id,
			Scope:           bridgepkg.ResourceScopeForBridge(spec.Scope, spec.WorkspaceID),
			ExpectedVersion: expectedVersion,
			Spec:            spec,
		}); err != nil {
			return fmt.Errorf("bundles: upsert owned bridge instance %q: %w", id, err)
		}
		changed[bridgepkg.BridgeInstanceResourceKind] = struct{}{}
		delete(current, id)
	}
	return deleteStaleOwnedRecords(ctx, actor, current, changed, bridgepkg.BridgeInstanceResourceKind, s.bridges.Delete)
}
