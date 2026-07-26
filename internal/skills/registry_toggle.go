package skills

import (
	"context"
	"errors"
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/filesnap"

	workspacepkg "github.com/compozy/agh/internal/workspace"
)

// SetEnabled updates the runtime enabled state for a named skill and keeps the
// disabled-skills overlay in sync so future reloads preserve the change.
func (r *Registry) SetEnabled(name string, resolved *workspacepkg.ResolvedWorkspace, enabled bool) error {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return errors.New("skills: skill name is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.resourceAuthority {
		scope, cacheKey, skill := r.resourceSkillTargetLocked(trimmedName, resolved)
		if skill != nil {
			skill.Enabled = enabled
			switch scope {
			case skillToggleScopeWorkspace:
				r.workspaceDisabled[cacheKey] = setDisabledSkill(r.workspaceDisabled[cacheKey], trimmedName, enabled)
			default:
				r.cfg.DisabledSkills = setDisabledSkill(r.cfg.DisabledSkills, trimmedName, enabled)
			}
			r.globalVersion.Add(1)
			return nil
		}
		return fmt.Errorf("skills: skill %q not found", trimmedName)
	}

	if cacheKey, workspaceSkill := r.workspaceSkillTargetLocked(trimmedName, resolved); workspaceSkill != nil {
		workspaceSkill.Enabled = enabled
		r.workspaceDisabled[cacheKey] = setDisabledSkill(r.workspaceDisabled[cacheKey], trimmedName, enabled)
		return nil
	}

	globalSkill := r.globalSkills[trimmedName]
	if globalSkill == nil {
		return fmt.Errorf("skills: skill %q not found", trimmedName)
	}

	globalSkill.Enabled = enabled
	r.cfg.DisabledSkills = setDisabledSkill(r.cfg.DisabledSkills, trimmedName, enabled)

	return nil
}

func (r *Registry) reloadGlobal(ctx context.Context) error {
	if err := checkRegistryContext(ctx); err != nil {
		return err
	}
	if r.usesResourceAuthority() {
		return nil
	}

	disabledSkills := r.globalDisabledSkillsSnapshot()
	loaded, snapshots, diagnostics, err := r.loadGlobalSkills(ctx, disabledSkills)
	if err != nil {
		return err
	}
	if err := checkRegistryContext(ctx); err != nil {
		return err
	}

	r.mu.Lock()
	if err := checkRegistryContext(ctx); err != nil {
		r.mu.Unlock()
		return err
	}

	r.evictExpiredWorkspaceLocked(r.now())
	if r.globalLoaded && filesnap.Equal(r.globalSnapshots, snapshots) {
		r.mu.Unlock()
		return nil
	}

	r.globalSnapshots = filesnap.Clone(snapshots)
	r.globalDiagnostics = cloneDiagnostics(diagnostics)
	r.globalLoaded = true
	r.globalSkills = loaded
	r.globalVersion.Add(1)
	shadowEvents := r.buildSkillShadowSummariesFromResolved(mergedSkillList(loaded, nil), "global", "", "")
	r.mu.Unlock()
	r.emitEventSummaries(ctx, shadowEvents)

	return nil
}
