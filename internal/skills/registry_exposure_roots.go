package skills

import (
	compozyconfig "github.com/compozy/compozy/internal/config"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// ExposureRoots returns the active roots that can participate in exposure for one catalog scope.
func (r *Registry) ExposureRoots(resolved *workspacepkg.ResolvedWorkspace) []compozyconfig.SkillRootSpec {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	roots := append([]compozyconfig.SkillRootSpec(nil), r.cfg.GlobalSkillRoots...)
	r.mu.RUnlock()
	return append(roots, workspaceResolvedSkillRoots(resolved)...)
}
