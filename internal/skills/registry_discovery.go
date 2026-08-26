package skills

import (
	"context"
	"errors"
	"fmt"

	"slices"
	"strings"

	"github.com/compozy/compozy/internal/filesnap"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/store"

	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type resourceSkillProjection struct {
	globalSkills                      map[string]*Skill
	profileSkills                     map[string]map[string]*Skill
	workspaceSkills                   map[string]map[string]*Skill
	workspaceProfileSkills            map[string]map[string]*Skill
	globalCommandCandidates           []*Skill
	profileCommandCandidates          map[string][]*Skill
	workspaceCommandCandidates        map[string][]*Skill
	workspaceProfileCommandCandidates map[string][]*Skill
	seenCommandCandidates             map[string]struct{}
}

func newResourceSkillProjection() resourceSkillProjection {
	return resourceSkillProjection{
		globalSkills:                      make(map[string]*Skill),
		profileSkills:                     make(map[string]map[string]*Skill),
		workspaceSkills:                   make(map[string]map[string]*Skill),
		workspaceProfileSkills:            make(map[string]map[string]*Skill),
		profileCommandCandidates:          make(map[string][]*Skill),
		workspaceCommandCandidates:        make(map[string][]*Skill),
		workspaceProfileCommandCandidates: make(map[string][]*Skill),
		seenCommandCandidates:             make(map[string]struct{}),
	}
}

// DiscoverGlobal loads global skill definitions for resource publication without
// making the file-system scan authoritative in the registry.
func (r *Registry) DiscoverGlobal(ctx context.Context) ([]*Skill, map[string]filesnap.Snapshot, error) {
	if err := checkRegistryContext(ctx); err != nil {
		return nil, nil, err
	}
	cfg := r.registryConfigSnapshot(ctx)
	disabledSkills := append([]string(nil), cfg.DisabledSkills...)
	_, snapshots, diagnostics, candidates, err := r.loadGlobalSkills(ctx, disabledSkills, cfg)
	if err != nil {
		return nil, nil, err
	}
	r.recordDiscoveredGlobalDiagnostics(ctx, diagnostics)
	return cloneCommandSkillSlice(candidates), filesnap.Clone(snapshots), nil
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
		r.recordDiscoveredWorkspaceDiagnostics(ctx, resolved, nil)
		return nil, load.snapshots, nil
	}
	workspaceDisabled := r.workspaceDisabledSkillsSnapshot(
		workspaceCacheKey(resolved),
		resolved.Config.Skills.DisabledSkills,
	)
	_, diagnostics, candidates, err := r.loadWorkspaceSkills(ctx, load.paths, workspaceDisabled)
	if err != nil {
		return nil, nil, err
	}
	r.recordDiscoveredWorkspaceDiagnostics(ctx, resolved, diagnostics)
	return cloneCommandSkillSlice(candidates), load.snapshots, nil
}

func (r *Registry) recordDiscoveredGlobalDiagnostics(ctx context.Context, diagnostics []SkillDiagnostic) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if generation := ConfigGenerationFromContext(ctx); generation > 0 && generation == r.pendingGeneration {
		r.pendingGlobalDiagnostics = cloneDiagnostics(diagnostics)
		return
	}
	r.globalDiagnostics = cloneDiagnostics(diagnostics)
}

func (r *Registry) recordDiscoveredWorkspaceDiagnostics(
	ctx context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
	diagnostics []SkillDiagnostic,
) {
	key := workspaceCacheKey(resolved)
	if key == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if generation := ConfigGenerationFromContext(ctx); generation > 0 && generation == r.pendingGeneration {
		if r.pendingWorkspaceDiagnostics == nil {
			r.pendingWorkspaceDiagnostics = make(map[string][]SkillDiagnostic)
		}
		r.pendingWorkspaceDiagnostics[key] = cloneDiagnostics(diagnostics)
		return
	}
	r.resourceWorkspaceDiagnostics[key] = cloneDiagnostics(diagnostics)
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
	r.mu.Lock()
	generation := ConfigGenerationFromContext(ctx)
	activeGeneration := r.configGeneration.Load()
	if generation == 0 && r.pendingGeneration > 0 {
		winningGeneration := r.pendingGeneration
		r.mu.Unlock()
		return r.supersededGenerationError(ctx, generation, winningGeneration)
	}
	if generation > 0 && generation < activeGeneration {
		r.mu.Unlock()
		return r.supersededGenerationError(ctx, generation, activeGeneration)
	}
	if generation > 0 && r.pendingGeneration > 0 && generation != r.pendingGeneration {
		winningGeneration := r.pendingGeneration
		r.mu.Unlock()
		return r.supersededGenerationError(ctx, generation, winningGeneration)
	}
	if r.pendingConfig != nil && r.pendingGeneration == generation {
		r.pendingProjection = &projection
		r.pendingRevision = revision
		r.mu.Unlock()
		return nil
	}
	r.applyResourceProjectionLocked(revision, projection)
	r.mu.Unlock()

	r.emitResourceProjectionSummaries(ctx, projection)
	return nil
}

func (r *Registry) applyResourceProjectionLocked(revision int64, projection resourceSkillProjection) {
	r.resourceAuthority = true
	r.resourceRevision = revision
	r.resourceWorkspaces = projection.workspaceSkills
	r.resourceProfiles = projection.profileSkills
	r.resourceWorkspaceProfiles = projection.workspaceProfileSkills
	r.resourceGlobalCommandCandidates = projection.globalCommandCandidates
	r.resourceProfileCommandCandidates = projection.profileCommandCandidates
	r.resourceWorkspaceCommandCandidates = projection.workspaceCommandCandidates
	r.resourceWorkspaceProfileCommandCandidates = projection.workspaceProfileCommandCandidates
	r.globalSkills = projection.globalSkills
	r.wsCache = make(map[string]*wsCache)
	r.globalLoaded = true
	r.globalVersion.Add(1)
}

func (r *Registry) emitResourceProjectionSummaries(ctx context.Context, projection resourceSkillProjection) {
	r.emitResourceGlobalSkillSummaries(ctx, projection)
	r.emitResourceWorkspaceSkillSummaries(ctx, projection)
	r.emitResourceProfileSkillSummaries(ctx, projection)
	r.emitResourceWorkspaceProfileSkillSummaries(ctx, projection)
}

func (r *Registry) emitResourceGlobalSkillSummaries(ctx context.Context, projection resourceSkillProjection) {
	r.emitEventSummaries(ctx, r.buildSkillShadowSummariesFromResolved(
		mergedSkillList(projection.globalSkills, nil), registryGlobalKey, store.DefaultProfileID, "", "",
	))
}

func (r *Registry) emitResourceWorkspaceSkillSummaries(ctx context.Context, projection resourceSkillProjection) {
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
				store.DefaultProfileID,
				workspaceID,
				"",
			),
		)
	}
}

func (r *Registry) emitResourceProfileSkillSummaries(ctx context.Context, projection resourceSkillProjection) {
	profileIDs := make([]string, 0, len(projection.profileSkills))
	for profileID := range projection.profileSkills {
		profileIDs = append(profileIDs, profileID)
	}
	slices.Sort(profileIDs)
	for _, profileID := range profileIDs {
		r.emitEventSummaries(
			ctx,
			r.buildSkillShadowSummaries(
				projection.globalSkills,
				projection.profileSkills[profileID],
				skillSourceProfileName,
				profileID,
				"",
				"",
			),
		)
	}
}

func (r *Registry) emitResourceWorkspaceProfileSkillSummaries(
	ctx context.Context,
	projection resourceSkillProjection,
) {
	workspaceProfileKeys := make([]string, 0, len(projection.workspaceProfileSkills))
	for key := range projection.workspaceProfileSkills {
		workspaceProfileKeys = append(workspaceProfileKeys, key)
	}
	slices.Sort(workspaceProfileKeys)
	for _, key := range workspaceProfileKeys {
		workspaceID, profileID, ok := strings.Cut(key, "@pf:")
		if !ok || strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(profileID) == "" {
			continue
		}
		base := cloneSkillMapFromList(mergedSkillLayers(
			projection.globalSkills,
			projection.profileSkills[profileID],
			projection.workspaceSkills[workspaceID],
		))
		r.emitEventSummaries(
			ctx,
			r.buildSkillShadowSummaries(
				base,
				projection.workspaceProfileSkills[key],
				skillSourceWorkspaceProfileName,
				profileID,
				workspaceID,
				"",
			),
		)
	}
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
	skill.ResourceScope = record.Scope.Normalize()
	applySkillExtensionOrigin(record, skill)
	if strings.TrimSpace(skill.Meta.Name) == "" {
		return nil
	}
	switch record.Scope.Kind.Normalize() {
	case resources.ResourceScopeKindUser:
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
	case resources.ResourceScopeKindProfile:
		profileID := strings.TrimSpace(record.Scope.ID)
		if profileID == "" {
			return nil
		}
		if projection.profileSkills[profileID] == nil {
			projection.profileSkills[profileID] = make(map[string]*Skill)
		}
		skill.CommandScope = registryGlobalKey
		if err := appendCommandResourceCandidate(
			projection.seenCommandCandidates,
			"profile:"+profileID,
			skill,
		); err != nil {
			return err
		}
		r.overlaySkill(projection.profileSkills[profileID], skill)
		projection.profileCommandCandidates[profileID] = append(
			projection.profileCommandCandidates[profileID],
			cloneSkill(skill),
		)
	case resources.ResourceScopeKindWorkspaceProfile:
		return r.projectWorkspaceProfileResourceSkill(projection, record, skill)
	}
	return nil
}

func (r *Registry) projectWorkspaceProfileResourceSkill(
	projection *resourceSkillProjection,
	record resources.Record[SkillResourceSpec],
	skill *Skill,
) error {
	key := strings.TrimSpace(record.Scope.ID)
	if key == "" {
		return nil
	}
	if projection.workspaceProfileSkills[key] == nil {
		projection.workspaceProfileSkills[key] = make(map[string]*Skill)
	}
	skill.CommandScope = skillSourceWorkspaceName
	if err := appendCommandResourceCandidate(
		projection.seenCommandCandidates,
		"workspace_profile:"+key,
		skill,
	); err != nil {
		return err
	}
	r.overlaySkill(projection.workspaceProfileSkills[key], skill)
	projection.workspaceProfileCommandCandidates[key] = append(
		projection.workspaceProfileCommandCandidates[key],
		cloneSkill(skill),
	)
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
