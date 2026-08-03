package daemon

import (
	"bytes"
	"context"

	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/resources"
	skillspkg "github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/soul"
)

type desiredAgentResource struct {
	id      string
	scope   resources.ResourceScope
	owner   *resources.ResourceOwner
	spec    compozyconfig.AgentDef
	encoded []byte
}

type desiredSkillResource struct {
	id      string
	scope   resources.ResourceScope
	owner   *resources.ResourceOwner
	spec    skillspkg.SkillResourceSpec
	encoded []byte
}

type indexedAgentSkillResources struct {
	agents     map[string]desiredAgentResource
	souls      map[string]managedResourceValue[soul.ResourceSpec]
	heartbeats map[string]managedResourceValue[heartbeat.ResourceSpec]
	skills     map[string]desiredSkillResource
	mcpServers map[string]desiredMCPServerResource
}

func (s *agentSkillSourceSyncer) desiredResources(ctx context.Context) (indexedAgentSkillResources, error) {
	desired := newIndexedAgentSkillResources()
	agentIDsBySourceKey := make(map[string]string)

	for _, provider := range s.providers {
		if provider == nil {
			continue
		}
		items, err := provider(ctx)
		if err != nil {
			return desired, err
		}
		if err := s.appendDesiredResources(ctx, &desired, agentIDsBySourceKey, items); err != nil {
			return desired, err
		}
	}

	return desired, nil
}

func newIndexedAgentSkillResources() indexedAgentSkillResources {
	return indexedAgentSkillResources{
		agents:     make(map[string]desiredAgentResource),
		souls:      make(map[string]managedResourceValue[soul.ResourceSpec]),
		heartbeats: make(map[string]managedResourceValue[heartbeat.ResourceSpec]),
		skills:     make(map[string]desiredSkillResource),
		mcpServers: make(map[string]desiredMCPServerResource),
	}
}

func (s *agentSkillSourceSyncer) appendDesiredResources(
	ctx context.Context,
	desired *indexedAgentSkillResources,
	agentIDsBySourceKey map[string]string,
	items agentSkillDeclarations,
) error {
	if err := s.appendDesiredAgents(ctx, desired, agentIDsBySourceKey, items); err != nil {
		return err
	}
	if err := s.appendDesiredSkills(ctx, desired, items); err != nil {
		return err
	}
	if err := s.appendDesiredMCPServers(ctx, desired, items); err != nil {
		return err
	}
	return s.appendDesiredSidecars(ctx, desired, items, agentIDsBySourceKey)
}

func (s *agentSkillSourceSyncer) appendDesiredAgents(
	ctx context.Context,
	desired *indexedAgentSkillResources,
	agentIDsBySourceKey map[string]string,
	items agentSkillDeclarations,
) error {
	for _, item := range items.agents {
		spec, encoded, err := validateAndEncodeAgent(ctx, s.agentCodec, item.scope, item.spec)
		if err != nil {
			return err
		}
		spec.SourcePath = strings.TrimSpace(item.spec.SourcePath)
		id := managedPublicationID(agentManagedIDPrefix, item.scope.Normalize(), item.sourceKey, encoded, item.owner)
		desired.agents[id] = desiredAgentResource{
			id: id, scope: item.scope.Normalize(), owner: item.owner, spec: spec, encoded: encoded,
		}
		agentIDsBySourceKey[item.sourceKey] = id
	}
	return nil
}

func (s *agentSkillSourceSyncer) appendDesiredSkills(
	ctx context.Context,
	desired *indexedAgentSkillResources,
	items agentSkillDeclarations,
) error {
	for _, item := range items.skills {
		spec, encoded, err := validateAndEncodeSkill(ctx, s.skillCodec, item.scope, item.spec)
		if err != nil {
			return err
		}
		id := managedPublicationID(skillManagedIDPrefix, item.scope.Normalize(), item.sourceKey, encoded, item.owner)
		desired.skills[id] = desiredSkillResource{
			id: id, scope: item.scope.Normalize(), owner: item.owner, spec: spec, encoded: encoded,
		}
	}
	return nil
}

func (s *agentSkillSourceSyncer) appendDesiredMCPServers(
	ctx context.Context,
	desired *indexedAgentSkillResources,
	items agentSkillDeclarations,
) error {
	for _, item := range items.mcpServers {
		spec, encoded, err := validateAndEncodeMCPServer(ctx, s.mcpCodec, item.scope, item.spec)
		if err != nil {
			return err
		}
		id := managedPublicationID(
			mcpServerManagedIDPrefix,
			item.scope.Normalize(),
			item.sourceKey,
			encoded,
			item.owner,
		)
		desired.mcpServers[id] = desiredMCPServerResource{
			id: id, scope: item.scope.Normalize(), owner: item.owner, spec: spec, encoded: encoded,
		}
	}
	return nil
}

func (s *agentSkillSourceSyncer) stageAgentTransientSpecs(desired map[string]desiredAgentResource) {
	projector, ok := s.agentProjector.(*resourceCatalogProjector[compozyconfig.AgentDef])
	if !ok || projector.catalog == nil {
		return
	}
	specs := make(map[string]compozyconfig.AgentDef, len(desired))
	for id, agent := range desired {
		specs[id] = agent.spec
	}
	projector.catalog.MergeTransientSpecs(specs)
}

func (s *agentSkillSourceSyncer) syncAgents(
	ctx context.Context,
	desired map[string]desiredAgentResource,
) (bool, error) {
	source := s.actor.Source
	current, err := s.raw.ListRaw(ctx, s.actor, resources.ResourceFilter{
		Kind:   compozyconfig.AgentResourceKind,
		Source: &source,
	})
	if err != nil {
		return false, fmt.Errorf("daemon: list managed agents: %w", err)
	}
	currentByID := make(map[string]resources.RawRecord, len(current))
	for _, record := range current {
		currentByID[record.ID] = record
	}

	changed := false
	for id, desiredAgent := range desired {
		existing, ok := currentByID[id]
		if ok && sameManagedRawRecord(
			existing,
			desiredAgent.scope,
			desiredAgent.encoded,
			managedRecordAttributionFor(s.actor, desiredAgent.owner),
		) {
			delete(currentByID, id)
			continue
		}
		expectedVersion := int64(0)
		if ok {
			expectedVersion = existing.Version
		}
		if _, err := s.agentStore.Put(ctx, s.actor, resources.Draft[compozyconfig.AgentDef]{
			ID:              desiredAgent.id,
			Scope:           desiredAgent.scope,
			Owner:           desiredAgent.owner,
			ExpectedVersion: expectedVersion,
			Spec:            desiredAgent.spec,
		}); err != nil {
			return false, fmt.Errorf("daemon: sync agent %q: %w", id, err)
		}
		changed = true
		delete(currentByID, id)
	}
	for _, stale := range currentByID {
		if err := s.agentStore.Delete(ctx, s.actor, stale.ID, stale.Version); err != nil {
			return false, fmt.Errorf("daemon: delete stale agent %q: %w", stale.ID, err)
		}
		changed = true
	}
	return changed, nil
}

func (s *agentSkillSourceSyncer) syncSkills(
	ctx context.Context,
	desired map[string]desiredSkillResource,
) (bool, error) {
	source := s.actor.Source
	current, err := s.raw.ListRaw(ctx, s.actor, resources.ResourceFilter{
		Kind:   skillspkg.SkillResourceKind,
		Source: &source,
	})
	if err != nil {
		return false, fmt.Errorf("daemon: list managed skills: %w", err)
	}
	currentByID := make(map[string]resources.RawRecord, len(current))
	for _, record := range current {
		currentByID[record.ID] = record
	}

	changed := false
	for id, desiredSkill := range desired {
		existing, ok := currentByID[id]
		if ok && sameManagedRawRecord(
			existing,
			desiredSkill.scope,
			desiredSkill.encoded,
			managedRecordAttributionFor(s.actor, desiredSkill.owner),
		) {
			delete(currentByID, id)
			continue
		}
		expectedVersion := int64(0)
		if ok {
			expectedVersion = existing.Version
		}
		if _, err := s.skillStore.Put(ctx, s.actor, resources.Draft[skillspkg.SkillResourceSpec]{
			ID:              desiredSkill.id,
			Scope:           desiredSkill.scope,
			Owner:           desiredSkill.owner,
			ExpectedVersion: expectedVersion,
			Spec:            desiredSkill.spec,
		}); err != nil {
			return false, fmt.Errorf("daemon: sync skill %q: %w", id, err)
		}
		changed = true
		delete(currentByID, id)
	}
	for _, stale := range currentByID {
		if err := s.skillStore.Delete(ctx, s.actor, stale.ID, stale.Version); err != nil {
			return false, fmt.Errorf("daemon: delete stale skill %q: %w", stale.ID, err)
		}
		changed = true
	}
	return changed, nil
}

func (s *agentSkillSourceSyncer) syncMCPServers(
	ctx context.Context,
	desired map[string]desiredMCPServerResource,
) (bool, error) {
	source := s.actor.Source
	current, err := s.raw.ListRaw(ctx, s.actor, resources.ResourceFilter{
		Kind:   compozyconfig.MCPServerResourceKind,
		Source: &source,
	})
	if err != nil {
		return false, fmt.Errorf("daemon: list agent/skill mcp servers: %w", err)
	}
	currentByID := make(map[string]resources.RawRecord, len(current))
	for _, record := range current {
		currentByID[record.ID] = record
	}

	changed := false
	for id, desiredServer := range desired {
		existing, ok := currentByID[id]
		if ok && sameManagedRawRecord(
			existing,
			desiredServer.scope,
			desiredServer.encoded,
			managedRecordAttributionFor(s.actor, desiredServer.owner),
		) {
			delete(currentByID, id)
			continue
		}
		expectedVersion := int64(0)
		if ok {
			expectedVersion = existing.Version
		}
		if _, err := s.mcpStore.Put(ctx, s.actor, resources.Draft[compozyconfig.MCPServer]{
			ID:              desiredServer.id,
			Scope:           desiredServer.scope,
			Owner:           desiredServer.owner,
			ExpectedVersion: expectedVersion,
			Spec:            desiredServer.spec,
		}); err != nil {
			return false, fmt.Errorf("daemon: sync agent/skill mcp server %q: %w", id, err)
		}
		changed = true
		delete(currentByID, id)
	}
	for _, stale := range currentByID {
		if err := s.mcpStore.Delete(ctx, s.actor, stale.ID, stale.Version); err != nil {
			return false, fmt.Errorf("daemon: delete stale agent/skill mcp server %q: %w", stale.ID, err)
		}
		changed = true
	}
	return changed, nil
}

type managedRecordAttribution struct {
	owner  resources.ResourceOwner
	source resources.ResourceSource
}

func managedRecordAttributionFor(
	actor resources.MutationActor,
	explicitOwner *resources.ResourceOwner,
) managedRecordAttribution {
	return managedRecordAttribution{
		owner:  managedDraftOwner(actor, explicitOwner),
		source: actor.Source.Normalize(),
	}
}

func sameManagedRawRecord(
	record resources.RawRecord,
	scope resources.ResourceScope,
	encoded []byte,
	attribution ...managedRecordAttribution,
) bool {
	if record.Scope != scope {
		return false
	}
	if len(attribution) > 0 {
		expected := attribution[0]
		if record.Owner.Normalize() != expected.owner.Normalize() ||
			record.Source.Normalize() != expected.source.Normalize() {
			return false
		}
	}
	return bytes.Equal(record.SpecJSON, encoded)
}
