package daemon

import (
	"bytes"
	"context"

	"fmt"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/resources"

	skillspkg "github.com/compozy/agh/internal/skills"
)

type desiredAgentResource struct {
	id      string
	scope   resources.ResourceScope
	spec    aghconfig.AgentDef
	encoded []byte
}

type desiredSkillResource struct {
	id      string
	scope   resources.ResourceScope
	spec    skillspkg.SkillResourceSpec
	encoded []byte
}

func (s *agentSkillSourceSyncer) desiredResources(ctx context.Context) (struct {
	agents     map[string]desiredAgentResource
	skills     map[string]desiredSkillResource
	mcpServers map[string]desiredMCPServerResource
}, error) {
	desired := struct {
		agents     map[string]desiredAgentResource
		skills     map[string]desiredSkillResource
		mcpServers map[string]desiredMCPServerResource
	}{
		agents:     make(map[string]desiredAgentResource),
		skills:     make(map[string]desiredSkillResource),
		mcpServers: make(map[string]desiredMCPServerResource),
	}

	for _, provider := range s.providers {
		if provider == nil {
			continue
		}
		items, err := provider(ctx)
		if err != nil {
			return desired, err
		}
		for _, item := range items.agents {
			spec, encoded, err := validateAndEncodeAgent(ctx, s.agentCodec, item.scope, item.spec)
			if err != nil {
				return desired, err
			}
			spec.SourcePath = strings.TrimSpace(item.spec.SourcePath)
			id := managedResourceID(agentManagedIDPrefix, item.scope.Normalize(), item.sourceKey, encoded)
			desired.agents[id] = desiredAgentResource{
				id:      id,
				scope:   item.scope.Normalize(),
				spec:    spec,
				encoded: encoded,
			}
		}
		for _, item := range items.skills {
			spec, encoded, err := validateAndEncodeSkill(ctx, s.skillCodec, item.scope, item.spec)
			if err != nil {
				return desired, err
			}
			id := managedResourceID(skillManagedIDPrefix, item.scope.Normalize(), item.sourceKey, encoded)
			desired.skills[id] = desiredSkillResource{
				id:      id,
				scope:   item.scope.Normalize(),
				spec:    spec,
				encoded: encoded,
			}
		}
		for _, item := range items.mcpServers {
			spec, encoded, err := validateAndEncodeMCPServer(ctx, s.mcpCodec, item.scope, item.spec)
			if err != nil {
				return desired, err
			}
			id := managedResourceID(mcpServerManagedIDPrefix, item.scope.Normalize(), item.sourceKey, encoded)
			desired.mcpServers[id] = desiredMCPServerResource{
				id:      id,
				scope:   item.scope.Normalize(),
				spec:    spec,
				encoded: encoded,
			}
		}
	}

	return desired, nil
}

func (s *agentSkillSourceSyncer) stageAgentTransientSpecs(desired map[string]desiredAgentResource) {
	projector, ok := s.agentProjector.(*resourceCatalogProjector[aghconfig.AgentDef])
	if !ok || projector.catalog == nil {
		return
	}
	specs := make(map[string]aghconfig.AgentDef, len(desired))
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
		Kind:   aghconfig.AgentResourceKind,
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
		if ok && sameManagedRawRecord(existing, desiredAgent.scope, desiredAgent.encoded) {
			delete(currentByID, id)
			continue
		}
		expectedVersion := int64(0)
		if ok {
			expectedVersion = existing.Version
		}
		if _, err := s.agentStore.Put(ctx, s.actor, resources.Draft[aghconfig.AgentDef]{
			ID:              desiredAgent.id,
			Scope:           desiredAgent.scope,
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
		if ok && sameManagedRawRecord(existing, desiredSkill.scope, desiredSkill.encoded) {
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
		Kind:   aghconfig.MCPServerResourceKind,
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
		if ok && sameManagedRawRecord(existing, desiredServer.scope, desiredServer.encoded) {
			delete(currentByID, id)
			continue
		}
		expectedVersion := int64(0)
		if ok {
			expectedVersion = existing.Version
		}
		if _, err := s.mcpStore.Put(ctx, s.actor, resources.Draft[aghconfig.MCPServer]{
			ID:              desiredServer.id,
			Scope:           desiredServer.scope,
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

func sameManagedRawRecord(
	record resources.RawRecord,
	scope resources.ResourceScope,
	encoded []byte,
) bool {
	if record.Scope != scope {
		return false
	}
	return bytes.Equal(record.SpecJSON, encoded)
}
