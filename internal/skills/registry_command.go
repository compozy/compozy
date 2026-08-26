package skills

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	commandpkg "github.com/compozy/compozy/internal/command"
	compozyconfig "github.com/compozy/compozy/internal/config"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// CommandCandidatesForAgentSession resolves one named session agent before
// projecting its slash-command candidates.
func (r *Registry) CommandCandidatesForAgentSession(
	ctx context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
	agentName string,
	sessionID string,
) ([]CommandCandidate, error) {
	if err := checkRegistryContext(ctx); err != nil {
		return nil, err
	}
	target := compozyconfig.NormalizeAgentName(agentName)
	if err := compozyconfig.ValidateAgentName(target); err != nil {
		return nil, err
	}
	agent, err := r.resolveAgentScope(resolved, target)
	if err != nil {
		return nil, err
	}
	return r.CommandCandidatesForAgentDefSession(ctx, resolved, agent, sessionID)
}

// CommandCandidatesForAgentDefSession returns bare effective skills plus every
// source-qualified external skill visible to one session.
func (r *Registry) CommandCandidatesForAgentDefSession(
	ctx context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
	agent compozyconfig.AgentDef,
	sessionID string,
) ([]CommandCandidate, error) {
	if r == nil {
		return nil, errors.New("skills: registry is required")
	}
	effective, err := r.ForAgentDefSession(ctx, resolved, agent, sessionID)
	if err != nil {
		return nil, err
	}

	generation := r.globalVersion.Load()
	candidates := make([]CommandCandidate, 0, len(effective)*2)
	for _, skill := range effective {
		if skill == nil {
			continue
		}
		candidates = append(candidates, commandCandidateFromSkill(skill, false, generation))
	}

	allByKey := r.commandCandidateSnapshot(resolved)
	for _, skill := range effective {
		appendCommandSkill(allByKey, skill)
	}
	all := slices.Collect(maps.Values(allByKey))
	applyDisabledSkillList(
		all,
		r.workspaceDisabledSkillsSnapshot(
			workspaceCacheKey(resolved),
			workspaceConfiguredDisabledSkills(resolved),
		),
	)
	profileDisabled, workspaceProfileDisabled := r.profileDisabledSkillSnapshots(resolved)
	applyDisabledSkillList(all, profileDisabled)
	applyDisabledSkillList(all, workspaceProfileDisabled)
	applyDisabledSkillList(all, agent.Skills.Disabled)
	all, err = r.projectAgentSkillActivation(ctx, resolved, agent, sessionID, all)
	if err != nil {
		return nil, err
	}
	nameCounts := make(map[string]int, len(all))
	for _, skill := range all {
		if skill != nil {
			nameCounts[commandpkg.Slug(skill.Meta.Name)]++
		}
	}
	qualifiedSourceIDs := collisionSafeQualifiedSourceIDs(all, generation)
	for _, skill := range all {
		if skill == nil || !shouldProjectQualifiedSkill(skill, nameCounts) {
			continue
		}
		candidate := commandCandidateFromSkill(skill, true, generation)
		if sourceID := qualifiedSourceIDs[commandCandidateIdentity(skill)]; sourceID != "" {
			candidate.SourceID = sourceID
		}
		candidates = append(candidates, candidate)
	}

	slices.SortStableFunc(candidates, func(left CommandCandidate, right CommandCandidate) int {
		leftKey := left.SourceKind + "\x00" + left.SourceID + "\x00" + left.Skill.Meta.Name
		rightKey := right.SourceKind + "\x00" + right.SourceID + "\x00" + right.Skill.Meta.Name
		return strings.Compare(leftKey, rightKey)
	})
	return candidates, nil
}

func (r *Registry) profileDisabledSkillSnapshots(
	resolved *workspacepkg.ResolvedWorkspace,
) ([]string, []string) {
	if r == nil || resolved == nil {
		return nil, nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.Clone(r.profileDisabled[resourceProfileKey(resolved)]),
		slices.Clone(r.workspaceProfileDisabled[resourceWorkspaceProfileKey(resolved)])
}

func (r *Registry) commandCandidateSnapshot(
	resolved *workspacepkg.ResolvedWorkspace,
) map[string]*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*Skill)
	if !r.resourceAuthority {
		for _, skill := range r.globalCommandCandidates {
			appendCommandSkill(result, skill)
		}
		if cached := r.wsCache[workspaceCacheKey(resolved)]; cached != nil {
			for _, skill := range cached.commandCandidates {
				appendCommandSkill(result, skill)
			}
		}
		return result
	}
	for _, skill := range r.resourceGlobalCommandCandidates {
		appendCommandSkill(result, skill)
	}
	for _, skill := range r.profileCommandCandidatesForResolved(resolved) {
		appendCommandSkill(result, skill)
	}
	for _, skill := range r.resourceWorkspaceCommandCandidates[resourceWorkspaceKey(resolved)] {
		appendCommandSkill(result, skill)
	}
	for _, skill := range r.workspaceProfileCommandCandidatesForResolved(resolved) {
		appendCommandSkill(result, skill)
	}
	return result
}

func cloneCommandSkillSlice(skills []*Skill) []*Skill {
	cloned := make([]*Skill, 0, len(skills))
	for _, skill := range skills {
		if skill != nil {
			cloned = append(cloned, cloneSkill(skill))
		}
	}
	return cloned
}

func (r *Registry) profileCommandCandidatesForResolved(
	resolved *workspacepkg.ResolvedWorkspace,
) []*Skill {
	if resolved == nil {
		return nil
	}
	for _, key := range []string{resourceProfileKey(resolved), strings.TrimSpace(resolved.ProfileRoot)} {
		if candidates := r.resourceProfileCommandCandidates[key]; len(candidates) > 0 {
			return candidates
		}
	}
	return nil
}

func (r *Registry) workspaceProfileCommandCandidatesForResolved(
	resolved *workspacepkg.ResolvedWorkspace,
) []*Skill {
	return r.resourceWorkspaceProfileCommandCandidates[resourceWorkspaceProfileKey(resolved)]
}

func appendCommandSkill(destination map[string]*Skill, skill *Skill) {
	if skill == nil {
		return
	}
	key, ok := commandSkillCandidateKey(skill)
	if !ok {
		return
	}
	destination[key] = cloneSkill(skill)
}

func appendCommandResourceCandidate(
	seen map[string]struct{},
	scope string,
	skill *Skill,
) error {
	if skill == nil {
		return nil
	}
	key, ok := commandSkillCandidateKey(skill)
	if !ok {
		return nil
	}
	key = scope + "\x00" + key
	if _, duplicate := seen[key]; duplicate {
		return fmt.Errorf("skills: duplicate external slash command %q", key)
	}
	seen[key] = struct{}{}
	return nil
}

func commandCandidateFromSkill(skill *Skill, qualified bool, generation int64) CommandCandidate {
	source := commandSourceForCandidate(skill, generation)
	return CommandCandidate{
		Skill:      cloneSkill(skill),
		SourceKind: source.Kind,
		SourceID:   source.ID,
		SourceKey:  source.Key,
		Scope:      source.Scope,
		Qualified:  qualified,
		Available:  skill != nil && skill.Enabled && skillIsActive(skill),
		Origin:     strings.TrimSpace(skill.Origin),
		RootID:     strings.TrimSpace(skill.RootID),
		Generation: generation,
	}
}

// CommandSourceForSkill returns the opaque exact-source identity used by slash command refs.
func CommandSourceForSkill(skill *Skill) commandpkg.Source {
	return commandSourceForCandidate(skill, 0)
}

func commandSourceForCandidate(skill *Skill, generation int64) commandpkg.Source {
	kind, id, key := commandSkillSource(skill)
	if rootID := strings.TrimSpace(skill.RootID); rootID != "" {
		kind = SkillPrecedenceTierName(skill.Source)
		id = strings.TrimSpace(skill.Origin)
		if id == "" {
			id = compozyconfig.SkillSourceCompozy
		}
		key = fmt.Sprintf("%s@generation:%d", rootID, generation)
	}
	return commandpkg.Source{
		Kind: kind, ID: id, Key: key, Scope: commandSkillScope(skill),
		Origin: strings.TrimSpace(skill.Origin),
	}
}

func commandSkillSource(skill *Skill) (string, string, string) {
	if skill == nil {
		return skillSourceUnknownName, "", ""
	}
	if extensionID := strings.TrimSpace(skill.InstalledFromExtension); extensionID != "" {
		return skillSourceExtensionName, extensionID, extensionID
	}
	if skill.Source == SourceMarketplace {
		if skill.Provenance == nil {
			return skillSourceMarketplaceName, "", ""
		}
		registryID := strings.TrimSpace(skill.Provenance.Registry)
		slug := strings.TrimSpace(skill.Provenance.Slug)
		return skillSourceMarketplaceName, registryID, registryID + ":" + slug
	}
	return SkillPrecedenceTierName(skill.Source), "", commandSkillPathKey(skill)
}

func commandSkillPathKey(skill *Skill) string {
	if skill == nil {
		return ""
	}
	path := strings.TrimSpace(skill.FilePath)
	if path == "" {
		path = strings.TrimSpace(skill.Dir)
	}
	if path == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(path))
	return "path-sha256:" + hex.EncodeToString(digest[:])
}

func commandSkillScope(skill *Skill) string {
	if skill == nil {
		return "session"
	}
	if scope := strings.TrimSpace(skill.CommandScope); scope != "" {
		return scope
	}
	switch skill.Source {
	case SourceWorkspace:
		return skillSourceWorkspaceName
	case SourceAgentLocal:
		return skillAgentKind
	default:
		return registryGlobalKey
	}
}

func isExternalCommandSkill(skill *Skill) bool {
	return skill != nil && (strings.TrimSpace(skill.InstalledFromExtension) != "" || skill.Source == SourceMarketplace)
}

func externalCommandSkillKey(skill *Skill) (string, bool) {
	sourceKind, sourceID, _ := commandSkillSource(skill)
	name := commandpkg.Slug(skill.Meta.Name)
	sourceSlug := commandpkg.Slug(sourceID)
	if name == "" || sourceSlug == "" {
		return "", false
	}
	return sourceKind + "\x00" + sourceSlug + "\x00" + name, true
}

func commandSkillCandidateKey(skill *Skill) (string, bool) {
	if skill == nil {
		return "", false
	}
	if rootID := strings.TrimSpace(skill.RootID); rootID != "" {
		path := strings.TrimSpace(skill.FilePath)
		return rootID + "\x00" + path, true
	}
	return externalCommandSkillKey(skill)
}

func shouldProjectQualifiedSkill(skill *Skill, nameCounts map[string]int) bool {
	if skill == nil {
		return false
	}
	name := commandpkg.Slug(skill.Meta.Name)
	if name == "goal" || name == "worktree" || name == "run" {
		return true
	}
	return strings.TrimSpace(skill.Origin) != "" || isExternalCommandSkill(skill) || nameCounts[name] > 1
}

func collisionSafeQualifiedSourceIDs(skills []*Skill, generation int64) map[string]string {
	type sourceIdentity struct {
		candidateKey string
		sourceID     string
		rootID       string
	}
	groups := make(map[string][]sourceIdentity)
	for _, skill := range skills {
		if skill == nil {
			continue
		}
		candidateKey := commandCandidateIdentity(skill)
		if candidateKey == "" {
			continue
		}
		source := commandSourceForCandidate(skill, generation)
		groupKey := commandpkg.Slug(source.ID) + "\x00" + commandpkg.Slug(skill.Meta.Name)
		groups[groupKey] = append(groups[groupKey], sourceIdentity{
			candidateKey: candidateKey,
			sourceID:     commandpkg.Slug(source.ID),
			rootID:       strings.TrimSpace(skill.RootID),
		})
	}

	result := make(map[string]string)
	for _, group := range groups {
		if len(group) < 2 {
			continue
		}
		for _, identity := range group {
			if identity.rootID == "" || identity.sourceID == "" {
				continue
			}
			result[identity.candidateKey] = rootQualifiedSourceID(identity.sourceID, identity.rootID)
		}
	}
	return result
}

func rootQualifiedSourceID(sourceID string, rootID string) string {
	base := commandpkg.Slug(sourceID)
	stableRootID := strings.TrimSpace(rootID)
	if base == "" || stableRootID == "" {
		return base
	}
	// RootID is a deterministic digest of the normalized physical root. Four
	// digest bytes keep the command token short while preserving stable identity
	// across scans and generations; collision grouping adds this suffix only when
	// two physical roots would otherwise publish the same qualified command.
	digest := sha256.Sum256([]byte(stableRootID))
	return base + "-" + hex.EncodeToString(digest[:4])
}

func commandCandidateIdentity(skill *Skill) string {
	key, _ := commandSkillCandidateKey(skill)
	return key
}
