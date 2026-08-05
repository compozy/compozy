package skills

import (
	"context"
	"errors"
	"fmt"

	"slices"
	"strings"

	"github.com/compozy/compozy/internal/filesnap"
	"github.com/compozy/compozy/internal/resources"

	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type resourceSkillProjection struct {
	globalSkills               map[string]*Skill
	workspaceSkills            map[string]map[string]*Skill
	globalCommandCandidates    []*Skill
	workspaceCommandCandidates map[string][]*Skill
	seenCommandCandidates      map[string]struct{}
}

func newResourceSkillProjection() resourceSkillProjection {
	return resourceSkillProjection{
		globalSkills:               make(map[string]*Skill),
		workspaceSkills:            make(map[string]map[string]*Skill),
		workspaceCommandCandidates: make(map[string][]*Skill),
		seenCommandCandidates:      make(map[string]struct{}),
	}
}

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
func (r *Registry) ApplyResourceRecords(
	ctx context.Context,
	revision int64,
	records []resources.Record[SkillResourceSpec],
) error {
	if r == nil {
		return errors.New("skills: registry is required")
	}
	if err := checkRegistryContext(ctx); err != nil {
		return err
	}
	projection, err := r.projectResourceSkillRecords(records)
	if err != nil {
		return err
	}
	r.emitEventSummaries(
		ctx,
		r.buildSkillShadowSummariesFromResolved(
			mergedSkillList(projection.globalSkills, nil),
			registryGlobalKey,
			"",
			"",
		),
	)

	workspaceIDs := make([]string, 0, len(projection.workspaceSkills))
	for workspaceID := range projection.workspaceSkills {
		workspaceIDs = append(workspaceIDs, workspaceID)
	}
	slices.Sort(workspaceIDs)
	for _, workspaceID := range workspaceIDs {
		r.logWorkspaceSkillOverrides(
			projection.globalSkills,
			projection.workspaceSkills[workspaceID],
			workspaceID,
		)
		r.emitEventSummaries(
			ctx,
			r.buildSkillShadowSummaries(
				projection.globalSkills,
				projection.workspaceSkills[workspaceID],
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
	r.resourceWorkspaces = projection.workspaceSkills
	r.resourceGlobalCommandCandidates = projection.globalCommandCandidates
	r.resourceWorkspaceCommandCandidates = projection.workspaceCommandCandidates
	r.globalSkills = projection.globalSkills
	r.globalDiagnostics = nil
	r.wsCache = make(map[string]*wsCache)
	r.globalLoaded = true
	r.globalVersion.Add(1)
	return nil
}

func (r *Registry) projectResourceSkillRecords(
	records []resources.Record[SkillResourceSpec],
) (resourceSkillProjection, error) {
	projection := newResourceSkillProjection()
	ordered := append([]resources.Record[SkillResourceSpec](nil), records...)
	slices.SortFunc(ordered, func(left, right resources.Record[SkillResourceSpec]) int {
		return strings.Compare(skillRecordSortKey(left), skillRecordSortKey(right))
	})
	for _, record := range ordered {
		if err := r.projectResourceSkillRecord(&projection, record); err != nil {
			return resourceSkillProjection{}, err
		}
	}
	return projection, nil
}

func (r *Registry) projectResourceSkillRecord(
	projection *resourceSkillProjection,
	record resources.Record[SkillResourceSpec],
) error {
	skill, err := SkillFromResourceSpec(record.Spec)
	if err != nil {
		return fmt.Errorf("skills: convert resource %q: %w", record.ID, err)
	}
	applySkillExtensionOrigin(record, skill)
	if strings.TrimSpace(skill.Meta.Name) == "" {
		return nil
	}
	switch record.Scope.Kind.Normalize() {
	case resources.ResourceScopeKindGlobal:
		skill.CommandScope = registryGlobalKey
		if err := appendCommandResourceCandidate(
			projection.seenCommandCandidates,
			registryGlobalKey,
			skill,
		); err != nil {
			return err
		}
		projection.globalCommandCandidates = append(
			projection.globalCommandCandidates,
			cloneSkill(skill),
		)
		r.overlaySkill(projection.globalSkills, skill)
	case resources.ResourceScopeKindWorkspace:
		return r.projectWorkspaceResourceSkill(projection, record, skill)
	}
	return nil
}

func (r *Registry) projectWorkspaceResourceSkill(
	projection *resourceSkillProjection,
	record resources.Record[SkillResourceSpec],
	skill *Skill,
) error {
	workspaceID := strings.TrimSpace(record.Scope.ID)
	if workspaceID == "" {
		return nil
	}
	if projection.workspaceSkills[workspaceID] == nil {
		projection.workspaceSkills[workspaceID] = make(map[string]*Skill)
	}
	skill.CommandScope = skillSourceWorkspaceName
	if err := appendCommandResourceCandidate(
		projection.seenCommandCandidates,
		skillSourceWorkspaceName+":"+workspaceID,
		skill,
	); err != nil {
		return err
	}
	projection.workspaceCommandCandidates[workspaceID] = append(
		projection.workspaceCommandCandidates[workspaceID],
		cloneSkill(skill),
	)
	r.overlaySkill(projection.workspaceSkills[workspaceID], skill)
	return nil
}
