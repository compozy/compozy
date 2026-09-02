package skills

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/filesnap"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// ForProfile returns the global skill set overlaid with one personal profile's skills.
func (r *Registry) ForProfile(ctx context.Context, profileName string, profileRoot string) ([]*Skill, error) {
	return r.ForWorkspace(ctx, &workspacepkg.ResolvedWorkspace{
		ProfileName: profileName,
		ProfileRoot: profileRoot,
	})
}

// ForWorkspace returns the global skill set overlaid with resolver-provided workspace skills.
func (r *Registry) ForWorkspace(ctx context.Context, resolved *workspacepkg.ResolvedWorkspace) ([]*Skill, error) {
	skills, err := r.workspaceSkills(ctx, resolved)
	if err != nil {
		return nil, err
	}
	return r.projectSkillActivation(ctx, skills, ActivationTarget{
		ProfileID:   resolvedSkillProfileID(resolved),
		WorkspaceID: resourceWorkspaceKey(resolved),
	})
}

func (r *Registry) workspaceSkills(ctx context.Context, resolved *workspacepkg.ResolvedWorkspace) ([]*Skill, error) {
	if err := checkRegistryContext(ctx); err != nil {
		return nil, err
	}

	if skills, ok := r.resourceBackedWorkspaceSkills(resolved); ok {
		applyDisabledSkillList(
			skills,
			r.workspaceDisabledSkillsSnapshot(
				workspaceCacheKey(resolved),
				workspaceConfiguredDisabledSkills(resolved),
			),
		)
		return skills, nil
	}
	cacheKey := workspaceCacheKey(resolved)
	workspaceDisabled := r.workspaceDisabledSkillsSnapshot(cacheKey, workspaceConfiguredDisabledSkills(resolved))
	if skills, ok := r.cachedWorkspaceSkillsIfFresh(ctx, resolved, cacheKey, workspaceDisabled); ok {
		return skills, nil
	}
	load, err := r.workspaceLoadFromResolved(ctx, resolved)
	if err != nil {
		return nil, err
	}
	if len(load.paths) == 0 {
		skills := r.List()
		applyDisabledSkillList(skills, workspaceDisabled)
		return skills, nil
	}

	if cacheKey == "" {
		return nil, errors.New("skills: workspace cache key is required")
	}

	now := r.now()
	currentGlobalVersion := r.GlobalVersion()

	r.mu.Lock()
	r.evictExpiredWorkspaceLocked(now)

	if cached := r.wsCache[cacheKey]; cached != nil &&
		cached.globalVersion == currentGlobalVersion &&
		filesnap.Equal(cached.snapshots, load.snapshots) {
		cached.lastAccess = now
		skills := mergedSkillListWithDisabled(r.globalSkills, cached.skills, workspaceDisabled)
		r.mu.Unlock()
		return skills, nil
	}
	r.mu.Unlock()

	workspaceSkills, workspaceDiagnostics, commandCandidates, err := r.loadWorkspaceSkills(
		ctx, load.paths, workspaceDisabled,
	)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	skills, shadowEvents := r.refreshWorkspaceCacheLocked(
		resolved,
		load,
		cacheKey,
		workspaceSkills,
		workspaceDiagnostics,
		commandCandidates,
		workspaceDisabled,
		now,
	)
	r.mu.Unlock()
	r.emitEventSummaries(ctx, shadowEvents)

	return skills, nil
}
