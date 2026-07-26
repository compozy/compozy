package skills

import (
	"context"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

// SkillDiagnostics returns diagnostics for the same resolution scope used by
// List, ForWorkspace, and ForAgent. It does not change resolution semantics.
func (r *Registry) SkillDiagnostics(
	ctx context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
	agentName string,
) ([]SkillDiagnostic, error) {
	if err := checkRegistryContext(ctx); err != nil {
		return nil, err
	}
	if strings.TrimSpace(agentName) != "" {
		return r.agentSkillDiagnostics(ctx, resolved, agentName)
	}
	if resolved != nil {
		return r.workspaceSkillDiagnostics(ctx, resolved)
	}
	return r.globalSkillDiagnostics(), nil
}

func (r *Registry) globalSkillDiagnostics() []SkillDiagnostic {
	r.mu.RLock()
	defer r.mu.RUnlock()

	diagnostics := skillDiagnosticsForList(mergedSkillList(r.globalSkills, nil))
	diagnostics = append(diagnostics, cloneDiagnostics(r.globalDiagnostics)...)
	return diagnostics
}

func (r *Registry) workspaceSkillDiagnostics(
	ctx context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
) ([]SkillDiagnostic, error) {
	skills, err := r.ForWorkspace(ctx, resolved)
	if err != nil {
		return nil, err
	}
	diagnostics := skillDiagnosticsForList(skills)

	r.mu.RLock()
	diagnostics = append(diagnostics, cloneDiagnostics(r.globalDiagnostics)...)
	diagnostics = r.appendWorkspaceLoadDiagnosticsLocked(diagnostics, resolved)
	r.mu.RUnlock()
	return diagnostics, nil
}

func (r *Registry) agentSkillDiagnostics(
	ctx context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
	agentName string,
) ([]SkillDiagnostic, error) {
	target := aghconfig.NormalizeAgentName(agentName)
	skills, err := r.ForAgent(ctx, resolved, target)
	if err != nil {
		return nil, err
	}
	diagnostics := skillDiagnosticsForList(skills)
	r.mu.RLock()
	diagnostics = append(diagnostics, cloneDiagnostics(r.globalDiagnostics)...)
	diagnostics = r.appendWorkspaceLoadDiagnosticsLocked(diagnostics, resolved)
	r.mu.RUnlock()
	return diagnostics, nil
}

func (r *Registry) appendWorkspaceLoadDiagnosticsLocked(
	diagnostics []SkillDiagnostic,
	resolved *workspacepkg.ResolvedWorkspace,
) []SkillDiagnostic {
	if r.resourceAuthority {
		return diagnostics
	}
	if paths, ok := workspaceCacheKeyPaths(resolved); ok {
		if cacheKey := workspaceCacheKey(resolved, paths); cacheKey != "" {
			if cached := r.wsCache[cacheKey]; cached != nil {
				diagnostics = append(diagnostics, cloneDiagnostics(cached.diagnostics)...)
			}
		}
	}
	return diagnostics
}
