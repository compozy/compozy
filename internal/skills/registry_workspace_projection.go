package skills

import (
	"context"
	"errors"

	"github.com/compozy/agh/internal/filesnap"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

// ForWorkspace returns the global skill set overlaid with resolver-provided workspace skills.
func (r *Registry) ForWorkspace(ctx context.Context, resolved *workspacepkg.ResolvedWorkspace) ([]*Skill, error) {
	skills, err := r.workspaceSkills(ctx, resolved)
	if err != nil {
		return nil, err
	}
	return r.projectSkillActivation(ctx, skills, ActivationTarget{WorkspaceID: resourceWorkspaceKey(resolved)})
}

func (r *Registry) workspaceSkills(ctx context.Context, resolved *workspacepkg.ResolvedWorkspace) ([]*Skill, error) {
	if err := checkRegistryContext(ctx); err != nil {
		return nil, err
	}

	if skills, ok := r.resourceBackedWorkspaceSkills(resolved); ok {
		applyDisabledSkillList(
			skills,
			r.workspaceDisabledSkillsSnapshot(
				workspaceCacheKey(resolved, nil),
				workspaceConfiguredDisabledSkills(resolved),
			),
		)
		return skills, nil
	}
	if skills, ok, err := r.cachedWorkspaceSkillsFromResolved(ctx, resolved); ok || err != nil {
		return skills, err
	}

	load, err := r.workspaceLoadFromResolved(ctx, resolved)
	if err != nil {
		return nil, err
	}
	cacheKey := workspaceCacheKey(resolved, load.paths)
	workspaceDisabled := r.workspaceDisabledSkillsSnapshot(cacheKey, workspaceConfiguredDisabledSkills(resolved))
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

	workspaceSkills, workspaceDiagnostics, err := r.loadWorkspaceSkills(ctx, load.paths, workspaceDisabled)
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
		workspaceDisabled,
		now,
	)
	r.mu.Unlock()
	r.emitEventSummaries(ctx, shadowEvents)

	return skills, nil
}
