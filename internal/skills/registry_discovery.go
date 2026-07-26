package skills

import (
	"context"
	"errors"
	"fmt"

	"slices"
	"strings"

	"github.com/compozy/agh/internal/filesnap"
	"github.com/compozy/agh/internal/resources"

	workspacepkg "github.com/compozy/agh/internal/workspace"
)

// DiscoverGlobal loads global skill definitions for resource publication without
// making the file-system scan authoritative in the registry.
func (r *Registry) DiscoverGlobal(ctx context.Context) ([]*Skill, map[string]filesnap.Snapshot, error) {
	if err := checkRegistryContext(ctx); err != nil {
		return nil, nil, err
	}
	disabledSkills := r.globalDisabledSkillsSnapshot()
	loaded, snapshots, _, err := r.loadGlobalSkills(ctx, disabledSkills)
	if err != nil {
		return nil, nil, err
	}
	return mergedSkillList(loaded, nil), filesnap.Clone(snapshots), nil
}

// DiscoverWorkspace loads workspace-visible skill definitions for resource publication.
func (r *Registry) DiscoverWorkspace(
	ctx context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
) ([]*Skill, map[string]filesnap.Snapshot, error) {
	if err := checkRegistryContext(ctx); err != nil {
		return nil, nil, err
	}
	load, err := r.workspaceLoadFromResolved(ctx, resolved)
	if err != nil {
		return nil, nil, err
	}
	if len(load.paths) == 0 {
		return nil, load.snapshots, nil
	}
	workspaceDisabled := r.workspaceDisabledSkillsSnapshot(
		workspaceCacheKey(resolved, load.paths),
		resolved.Config.Skills.DisabledSkills,
	)
	loaded, _, err := r.loadWorkspaceSkills(ctx, load.paths, workspaceDisabled)
	if err != nil {
		return nil, nil, err
	}
	return mergedSkillList(nil, loaded), load.snapshots, nil
}

// ApplyResourceRecords atomically replaces the runtime skill catalog with the
// canonical resource projection.
func (r *Registry) ApplyResourceRecords(revision int64, records []resources.Record[SkillResourceSpec]) error {
	if r == nil {
		return errors.New("skills: registry is required")
	}
	globalSkills := make(map[string]*Skill)
	workspaceSkills := make(map[string]map[string]*Skill)

	ordered := append([]resources.Record[SkillResourceSpec](nil), records...)
	slices.SortFunc(ordered, func(left, right resources.Record[SkillResourceSpec]) int {
		return strings.Compare(skillRecordSortKey(left), skillRecordSortKey(right))
	})

	for _, record := range ordered {
		skill, err := SkillFromResourceSpec(record.Spec)
		if err != nil {
			return fmt.Errorf("skills: convert resource %q: %w", record.ID, err)
		}
		applySkillResourceOrigin(record, skill)
		name := strings.TrimSpace(skill.Meta.Name)
		if name == "" {
			continue
		}
		switch record.Scope.Kind.Normalize() {
		case resources.ResourceScopeKindGlobal:
			r.overlaySkill(globalSkills, skill)
		case resources.ResourceScopeKindWorkspace:
			workspaceID := strings.TrimSpace(record.Scope.ID)
			if workspaceID == "" {
				continue
			}
			if workspaceSkills[workspaceID] == nil {
				workspaceSkills[workspaceID] = make(map[string]*Skill)
			}
			r.overlaySkill(workspaceSkills[workspaceID], skill)
		}
	}
	r.emitEventSummaries(
		context.Background(),
		r.buildSkillShadowSummariesFromResolved(mergedSkillList(globalSkills, nil), "global", "", ""),
	)

	workspaceIDs := make([]string, 0, len(workspaceSkills))
	for workspaceID := range workspaceSkills {
		workspaceIDs = append(workspaceIDs, workspaceID)
	}
	slices.Sort(workspaceIDs)
	for _, workspaceID := range workspaceIDs {
		r.logWorkspaceSkillOverrides(globalSkills, workspaceSkills[workspaceID], workspaceID)
		r.emitEventSummaries(
			context.Background(),
			r.buildSkillShadowSummaries(
				globalSkills,
				workspaceSkills[workspaceID],
				skillSourceWorkspaceName,
				workspaceID,
				"",
			),
		)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.resourceAuthority = true
	r.resourceRevision = revision
	r.resourceWorkspaces = workspaceSkills
	r.globalSkills = globalSkills
	r.globalDiagnostics = nil
	r.wsCache = make(map[string]*wsCache)
	r.globalLoaded = true
	r.globalVersion.Add(1)
	return nil
}
