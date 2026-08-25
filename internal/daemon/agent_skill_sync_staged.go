package daemon

import (
	"context"
	"errors"
	"fmt"
	"sync"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/resources"
	skillspkg "github.com/compozy/compozy/internal/skills"
)

// SyncSkillsStaged republishes managed skills and returns a one-shot rollback
// that restores the exact resource-store snapshot present before the sync.
func (s *agentSkillSourceSyncer) SyncSkillsStaged(
	ctx context.Context,
) (func(context.Context) error, error) {
	if s == nil {
		return nil, nil
	}
	if ctx == nil {
		return nil, errors.New("daemon: staged skill sync context is required")
	}
	s.syncMu.Lock()
	previous, err := s.managedSkillSnapshot(ctx)
	if err != nil {
		s.syncMu.Unlock()
		return nil, err
	}
	desired, err := s.desiredResources(ctx)
	changes := agentSkillSyncChanges{}
	if err == nil {
		changes.skills, err = s.syncSkills(ctx, desired.skills)
	}
	if err == nil {
		changes.mcpServers, err = s.syncMCPServers(ctx, desired.mcpServers)
	}
	if err == nil {
		err = s.projectSkills(ctx)
	}
	if err == nil {
		err = s.triggerAgentSkillChanges(ctx, changes)
	}
	if err != nil {
		restoreErr := s.restoreManagedSkillsLocked(context.WithoutCancel(ctx), previous)
		s.syncMu.Unlock()
		return nil, errors.Join(err, restoreErr)
	}
	s.syncMu.Unlock()

	var once sync.Once
	var rollbackErr error
	rollback := func(rollbackCtx context.Context) error {
		once.Do(func() {
			s.syncMu.Lock()
			defer s.syncMu.Unlock()
			rollbackErr = s.restoreManagedSkillsLocked(rollbackCtx, previous)
		})
		return rollbackErr
	}
	return rollback, nil
}

type managedSkillSourceSnapshot struct {
	skills     []resources.Record[skillspkg.SkillResourceSpec]
	mcpServers []resources.RawRecord
}

func (s *agentSkillSourceSyncer) managedSkillSnapshot(
	ctx context.Context,
) (managedSkillSourceSnapshot, error) {
	source := s.actor.Source
	records, err := s.skillStore.List(ctx, s.actor, resources.ResourceFilter{Source: &source})
	if err != nil {
		return managedSkillSourceSnapshot{}, fmt.Errorf("daemon: snapshot managed skills: %w", err)
	}
	mcpServers, err := s.raw.ListRaw(ctx, s.actor, resources.ResourceFilter{
		Kind: compozyconfig.MCPServerResourceKind, Source: &source,
	})
	if err != nil {
		return managedSkillSourceSnapshot{}, fmt.Errorf("daemon: snapshot managed skill mcp servers: %w", err)
	}
	return managedSkillSourceSnapshot{skills: records, mcpServers: mcpServers}, nil
}

func (s *agentSkillSourceSyncer) restoreManagedSkillsLocked(
	ctx context.Context,
	snapshot managedSkillSourceSnapshot,
) error {
	desired := make(map[string]desiredSkillResource, len(snapshot.skills))
	for _, record := range snapshot.skills {
		spec, encoded, err := validateAndEncodeSkill(ctx, s.skillCodec, record.Scope, record.Spec)
		if err != nil {
			return fmt.Errorf("daemon: encode skill %q for rollback: %w", record.ID, err)
		}
		owner := record.Owner
		desired[record.ID] = desiredSkillResource{
			id: record.ID, scope: record.Scope.Normalize(), owner: &owner, spec: spec, encoded: encoded,
		}
	}
	changes := agentSkillSyncChanges{}
	var err error
	changes.skills, err = s.syncSkills(ctx, desired)
	if err != nil {
		return fmt.Errorf("daemon: restore managed skill snapshot: %w", err)
	}
	desiredMCP := make(map[string]desiredMCPServerResource, len(snapshot.mcpServers))
	for _, record := range snapshot.mcpServers {
		spec, decodeErr := s.mcpCodec.DecodeAndValidate(ctx, record.Scope, record.SpecJSON)
		if decodeErr != nil {
			return fmt.Errorf("daemon: decode mcp server %q for rollback: %w", record.ID, decodeErr)
		}
		encoded, encodeErr := s.mcpCodec.Encode(spec)
		if encodeErr != nil {
			return fmt.Errorf("daemon: encode mcp server %q for rollback: %w", record.ID, encodeErr)
		}
		owner := record.Owner
		desiredMCP[record.ID] = desiredMCPServerResource{
			id: record.ID, scope: record.Scope.Normalize(), owner: &owner, spec: spec, encoded: encoded,
		}
	}
	changes.mcpServers, err = s.syncMCPServers(ctx, desiredMCP)
	if err != nil {
		return fmt.Errorf("daemon: restore managed skill mcp snapshot: %w", err)
	}
	if err := s.projectSkills(ctx); err != nil {
		return err
	}
	return s.triggerAgentSkillChanges(ctx, changes)
}
