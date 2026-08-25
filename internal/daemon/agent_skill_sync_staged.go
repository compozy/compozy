package daemon

import (
	"context"
	"errors"
	"fmt"
	"sync"

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
	if err == nil {
		_, err = s.syncSkills(ctx, desired.skills)
	}
	if err == nil {
		err = s.projectSkills(ctx)
	}
	if err == nil && s.trigger != nil {
		err = s.trigger(ctx, skillspkg.SkillResourceKind, resources.ReconcileReasonWrite)
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

func (s *agentSkillSourceSyncer) managedSkillSnapshot(
	ctx context.Context,
) ([]resources.Record[skillspkg.SkillResourceSpec], error) {
	source := s.actor.Source
	records, err := s.skillStore.List(ctx, s.actor, resources.ResourceFilter{Source: &source})
	if err != nil {
		return nil, fmt.Errorf("daemon: snapshot managed skills: %w", err)
	}
	return records, nil
}

func (s *agentSkillSourceSyncer) restoreManagedSkillsLocked(
	ctx context.Context,
	records []resources.Record[skillspkg.SkillResourceSpec],
) error {
	desired := make(map[string]desiredSkillResource, len(records))
	for _, record := range records {
		spec, encoded, err := validateAndEncodeSkill(ctx, s.skillCodec, record.Scope, record.Spec)
		if err != nil {
			return fmt.Errorf("daemon: encode skill %q for rollback: %w", record.ID, err)
		}
		owner := record.Owner
		desired[record.ID] = desiredSkillResource{
			id: record.ID, scope: record.Scope.Normalize(), owner: &owner, spec: spec, encoded: encoded,
		}
	}
	if _, err := s.syncSkills(ctx, desired); err != nil {
		return fmt.Errorf("daemon: restore managed skill snapshot: %w", err)
	}
	return nil
}
