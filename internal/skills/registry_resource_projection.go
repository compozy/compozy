package skills

import (
	"slices"
	"strings"

	"github.com/compozy/agh/internal/resources"

	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func (r *Registry) usesResourceAuthority() bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.resourceAuthority
}

func (r *Registry) resourceBackedWorkspaceSkills(resolved *workspacepkg.ResolvedWorkspace) ([]*Skill, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if !r.resourceAuthority {
		return nil, false
	}
	workspaceSkills := r.resourceWorkspaces[resourceWorkspaceKey(resolved)]
	return mergedSkillList(r.globalSkills, workspaceSkills), true
}

func (r *Registry) resourceSkillTargetLocked(
	name string,
	resolved *workspacepkg.ResolvedWorkspace,
) (skillToggleScope, string, *Skill) {
	if r == nil || !r.resourceAuthority {
		return skillToggleScopeGlobal, "", nil
	}
	if key := resourceWorkspaceKey(resolved); key != "" {
		if workspaceSkills := r.resourceWorkspaces[key]; workspaceSkills != nil {
			if skill := workspaceSkills[name]; skill != nil {
				return skillToggleScopeWorkspace, workspaceCacheKey(resolved, nil), skill
			}
		}
	}
	return skillToggleScopeGlobal, "", r.globalSkills[name]
}

func (r *Registry) lookupSkillLocked(name string) (*Skill, bool) {
	if r == nil {
		return nil, false
	}
	skill := r.globalSkills[strings.TrimSpace(name)]
	return skill, skill != nil
}

func resourceWorkspaceKey(resolved *workspacepkg.ResolvedWorkspace) string {
	if resolved == nil {
		return ""
	}
	return strings.TrimSpace(resolved.ID)
}

func skillRecordSortKey(record resources.Record[SkillResourceSpec]) string {
	return string(record.Scope.Kind.Normalize()) + "\x00" +
		strings.TrimSpace(record.Scope.ID) + "\x00" +
		string(record.Source.Kind.Normalize()) + "\x00" +
		strings.TrimSpace(record.Source.ID) + "\x00" +
		strings.TrimSpace(record.ID)
}

func applySkillResourceOrigin(record resources.Record[SkillResourceSpec], skill *Skill) {
	if skill == nil {
		return
	}
	source := record.Source.Normalize()
	if source.Kind == resources.ResourceSourceKind("extension") &&
		strings.TrimSpace(skill.InstalledFromExtension) == "" {
		skill.InstalledFromExtension = strings.TrimSpace(source.ID)
	}
	owner := record.Owner.Normalize()
	if owner.Kind == resources.ResourceOwnerKind("bundle.activation") &&
		strings.TrimSpace(skill.InstalledFromBundle) == "" {
		skill.InstalledFromBundle = strings.TrimSpace(owner.ID)
	}
}

func mergeDisabledSkills(base []string, extra []string) []string {
	merged := slices.Clone(base)
	for _, name := range extra {
		merged = addDisabledSkill(merged, strings.TrimSpace(name))
	}
	return merged
}
