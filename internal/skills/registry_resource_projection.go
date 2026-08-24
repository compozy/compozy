package skills

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/store"

	workspacepkg "github.com/compozy/compozy/internal/workspace"
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
	profileSkills := resourceProfileSkills(r.resourceProfiles, resolved)
	workspaceSkills := r.resourceWorkspaces[resourceWorkspaceKey(resolved)]
	workspaceProfileKey := resourceWorkspaceProfileKey(resolved)
	workspaceProfileSkills := r.resourceWorkspaceProfiles[workspaceProfileKey]
	skills := mergedSkillLayers(r.globalSkills, profileSkills, workspaceSkills, workspaceProfileSkills)
	applyDisabledSkillList(skills, r.profileDisabled[resourceProfileKey(resolved)])
	applyDisabledSkillList(skills, r.workspaceDisabled[resourceWorkspaceKey(resolved)])
	applyDisabledSkillList(skills, r.workspaceProfileDisabled[workspaceProfileKey])
	return skills, true
}

func (r *Registry) resourceSkillTargetLocked(
	name string,
	resolved *workspacepkg.ResolvedWorkspace,
) (skillToggleScope, string, *Skill) {
	if r == nil || !r.resourceAuthority {
		return skillToggleScopeGlobal, "", nil
	}
	if profileSkills := resourceProfileSkills(r.resourceProfiles, resolved); profileSkills != nil {
		if skill := profileSkills[name]; skill != nil {
			return skillToggleScopeProfile, resourceProfileKey(resolved), skill
		}
	}
	if profileSkills := r.resourceWorkspaceProfiles[resourceWorkspaceProfileKey(resolved)]; profileSkills != nil {
		if skill := profileSkills[name]; skill != nil {
			return skillToggleScopeWorkspaceProfile, resourceWorkspaceProfileKey(resolved), skill
		}
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

func resourceProfileKey(resolved *workspacepkg.ResolvedWorkspace) string {
	if resolved == nil {
		return ""
	}
	if profileID := strings.TrimSpace(resolved.ProfileID); profileID != "" {
		return profileID
	}
	if profileName := strings.TrimSpace(resolved.ProfileName); profileName != "" {
		return profileName
	}
	return store.DefaultProfileID
}

func resourceProfileSkills(
	profiles map[string]map[string]*Skill,
	resolved *workspacepkg.ResolvedWorkspace,
) map[string]*Skill {
	if resolved == nil {
		return nil
	}
	profileID := strings.TrimSpace(resolved.ProfileID)
	profileName := strings.TrimSpace(resolved.ProfileName)
	if profileID == "" && profileName == "" {
		profileID = store.DefaultProfileID
		profileName = "default"
	}
	keys := []string{
		profileID,
		profileName,
		strings.TrimSpace(resolved.ProfileRoot),
	}
	for _, key := range keys {
		if key == "" {
			continue
		}
		if skillMap := profiles[key]; skillMap != nil {
			return skillMap
		}
		base := filepath.Base(key)
		if base != "." && base != string(filepath.Separator) {
			if skillMap := profiles[base]; skillMap != nil {
				return skillMap
			}
		}
	}
	return nil
}

func resourceWorkspaceProfileKey(resolved *workspacepkg.ResolvedWorkspace) string {
	if resolved == nil {
		return ""
	}
	workspaceID := strings.TrimSpace(resolved.ID)
	profileName := strings.TrimSpace(resolved.ProfileName)
	profileID := strings.TrimSpace(resolved.ProfileID)
	if profileName == "" && (profileID == "" || profileID == store.DefaultProfileID) {
		profileName = "default"
	}
	if workspaceID == "" || profileName == "" {
		return ""
	}
	return workspaceID + "@pf:" + profileName
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

func applySkillExtensionOrigin(record resources.Record[SkillResourceSpec], skill *Skill) {
	if skill == nil {
		return
	}
	source := record.Source.Normalize()
	if source.Kind == resources.ResourceSourceKind("extension") &&
		strings.TrimSpace(skill.InstalledFromExtension) == "" {
		skill.InstalledFromExtension = strings.TrimSpace(source.ID)
	}
}

func mergeDisabledSkills(base []string, extra []string) []string {
	merged := slices.Clone(base)
	for _, name := range extra {
		merged = addDisabledSkill(merged, strings.TrimSpace(name))
	}
	return merged
}
