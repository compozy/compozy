package skills

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/filesnap"
	"github.com/compozy/compozy/internal/store"

	workspacepkg "github.com/compozy/compozy/internal/workspace"
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
			case skillToggleScopeProfile:
				r.profileDisabled[cacheKey] = setDisabledSkill(r.profileDisabled[cacheKey], trimmedName, enabled)
			case skillToggleScopeWorkspaceProfile:
				r.workspaceProfileDisabled[cacheKey] = setDisabledSkill(
					r.workspaceProfileDisabled[cacheKey],
					trimmedName,
					enabled,
				)
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

	cfg := r.registryConfigSnapshot(ctx)
	scans, snapshots, err := r.scanConfiguredGlobalRoots(ctx, cfg.GlobalSkillRoots)
	if err != nil {
		return err
	}
	if r.globalScanCurrent(cfg.GlobalSkillRoots, snapshots) {
		return r.evictExpiredWorkspaces(ctx)
	}
	disabledSkills := append([]string(nil), cfg.DisabledSkills...)
	loaded, diagnostics, commandCandidates, err := r.parseGlobalSkills(ctx, disabledSkills, cfg, scans)
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
	if r.globalScanCurrentLocked(cfg.GlobalSkillRoots, snapshots) {
		r.mu.Unlock()
		return nil
	}

	r.globalSnapshots = filesnap.Clone(snapshots)
	r.globalSnapshotRoots = slices.Clone(cfg.GlobalSkillRoots)
	r.globalDiagnostics = cloneDiagnostics(diagnostics)
	r.globalLoaded = true
	r.globalSkills = loaded
	r.bundledRuntimeSkills = bundledRuntimeSkillsFromCandidates(commandCandidates)
	r.globalCommandCandidates = cloneCommandSkillSlice(commandCandidates)
	r.globalVersion.Add(1)
	shadowEvents := r.buildSkillShadowSummariesFromResolved(
		mergedSkillList(loaded, nil), "global", store.DefaultProfileID, "", "",
	)
	r.mu.Unlock()
	r.emitEventSummaries(ctx, shadowEvents)

	return nil
}

func (r *Registry) globalScanCurrent(
	roots []compozyconfig.SkillRootSpec,
	snapshots map[string]filesnap.Snapshot,
) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.globalScanCurrentLocked(roots, snapshots)
}

func (r *Registry) globalScanCurrentLocked(
	roots []compozyconfig.SkillRootSpec,
	snapshots map[string]filesnap.Snapshot,
) bool {
	return r.globalLoaded &&
		slices.Equal(r.globalSnapshotRoots, roots) &&
		filesnap.Equal(r.globalSnapshots, snapshots)
}

func (r *Registry) evictExpiredWorkspaces(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := checkRegistryContext(ctx); err != nil {
		return err
	}
	r.evictExpiredWorkspaceLocked(r.now())
	return nil
}
