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

	candidates := make([]CommandCandidate, 0, len(effective))
	externalByKey := make(map[string]*Skill)
	for _, skill := range effective {
		if skill == nil {
			continue
		}
		if isExternalCommandSkill(skill) {
			key, ok := externalCommandSkillKey(skill)
			if !ok {
				continue
			}
			externalByKey[key] = cloneSkill(skill)
			continue
		}
		candidates = append(candidates, commandCandidateFromSkill(skill, false))
	}

	externalSnapshot := r.externalCommandCandidateSnapshot(resolved)
	maps.Copy(externalByKey, externalSnapshot)
	external := make([]*Skill, 0, len(externalByKey))
	for _, skill := range externalByKey {
		external = append(external, skill)
	}
	applyDisabledSkillList(
		external,
		r.workspaceDisabledSkillsSnapshot(
			workspaceCacheKey(resolved, nil),
			workspaceConfiguredDisabledSkills(resolved),
		),
	)
	applyDisabledSkillList(external, agent.Skills.Disabled)
	external, err = r.projectAgentSkillActivation(ctx, resolved, agent, sessionID, external)
	if err != nil {
		return nil, err
	}
	for _, skill := range external {
		candidate := commandCandidateFromSkill(skill, true)
		candidates = append(candidates, candidate)
	}

	slices.SortStableFunc(candidates, func(left CommandCandidate, right CommandCandidate) int {
		leftKey := left.SourceKind + "\x00" + left.SourceID + "\x00" + left.Skill.Meta.Name
		rightKey := right.SourceKind + "\x00" + right.SourceID + "\x00" + right.Skill.Meta.Name
		return strings.Compare(leftKey, rightKey)
	})
	return candidates, nil
}

func (r *Registry) externalCommandCandidateSnapshot(
	resolved *workspacepkg.ResolvedWorkspace,
) map[string]*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*Skill)
	for _, skill := range r.resourceGlobalCommandCandidates {
		appendExternalCommandSkill(result, skill)
	}
	for _, skill := range r.profileCommandCandidatesForResolved(resolved) {
		appendExternalCommandSkill(result, skill)
	}
	for _, skill := range r.resourceWorkspaceCommandCandidates[resourceWorkspaceKey(resolved)] {
		appendExternalCommandSkill(result, skill)
	}
	for _, skill := range r.workspaceProfileCommandCandidatesForResolved(resolved) {
		appendExternalCommandSkill(result, skill)
	}
	return result
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

func appendExternalCommandSkill(destination map[string]*Skill, skill *Skill) {
	if skill == nil || !isExternalCommandSkill(skill) {
		return
	}
	key, ok := externalCommandSkillKey(skill)
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
	if skill == nil || !isExternalCommandSkill(skill) {
		return nil
	}
	key, ok := externalCommandSkillKey(skill)
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

func commandCandidateFromSkill(skill *Skill, qualified bool) CommandCandidate {
	source := CommandSourceForSkill(skill)
	return CommandCandidate{
		Skill:      cloneSkill(skill),
		SourceKind: source.Kind,
		SourceID:   source.ID,
		SourceKey:  source.Key,
		Scope:      source.Scope,
		Qualified:  qualified,
		Available:  skill != nil && skill.Enabled && skillIsActive(skill),
	}
}

// CommandSourceForSkill returns the opaque exact-source identity used by slash command refs.
func CommandSourceForSkill(skill *Skill) commandpkg.Source {
	kind, id, key := commandSkillSource(skill)
	return commandpkg.Source{Kind: kind, ID: id, Key: key, Scope: commandSkillScope(skill)}
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
		return "agent"
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
