package skills

import (
	"strings"
	"time"
)

func bundledRuntimeSkillsFromCandidates(candidates []*Skill) map[string]*Skill {
	protected := make(map[string]*Skill)
	for _, candidate := range candidates {
		if !isBundledRuntimeSkill(candidate) || strings.TrimSpace(candidate.RootID) != "" ||
			strings.TrimSpace(candidate.InstalledFromExtension) != "" {
			continue
		}
		protected[strings.TrimSpace(candidate.Meta.Name)] = cloneSkill(candidate)
	}
	return protected
}

func removeBundledRuntimeOverrides(skillsByName map[string]*Skill, protected map[string]*Skill) {
	for name := range protected {
		delete(skillsByName, name)
	}
}

func isBundledRuntimeSkill(skill *Skill) bool {
	if skill == nil || skill.Source != SourceBundled || skill.Meta.Metadata == nil {
		return false
	}
	rawCompozy, ok := skill.Meta.Metadata["compozy"]
	if !ok {
		return false
	}
	compozy, ok := rawCompozy.(map[string]any)
	if !ok {
		return false
	}
	kind, kindOK := compozy["kind"].(string)
	bundled, bundledOK := compozy["bundled"].(bool)
	return kindOK && bundledOK && strings.EqualFold(strings.TrimSpace(kind), "runtime") && bundled
}

func protectBundledRuntimeSkillMap(skillsByName map[string]*Skill, protected map[string]*Skill) {
	for name, canonical := range protected {
		if canonical == nil {
			continue
		}
		winner := cloneSkill(canonical)
		if existing := skillsByName[name]; existing != nil && !sameSkillDefinition(existing, canonical) {
			winner.Diagnostics.ShadowedDefinitions = append(
				cloneSkillDefinitionRefs(winner.Diagnostics.ShadowedDefinitions),
				shadowDefinitionRefsForWinner(existing, time.Time{})...,
			)
		}
		skillsByName[name] = winner
	}
}

func sameSkillDefinition(left *Skill, right *Skill) bool {
	return left != nil && right != nil && left.Source == right.Source &&
		strings.TrimSpace(left.FilePath) == strings.TrimSpace(right.FilePath) &&
		strings.TrimSpace(left.RootID) == strings.TrimSpace(right.RootID)
}

func bundledRuntimeCommandAlias(canonical *Skill, authored *Skill) *Skill {
	alias := cloneSkill(canonical)
	if alias == nil || authored == nil {
		return alias
	}
	alias.Enabled = alias.Enabled && authored.Enabled
	alias.CommandScope = authored.CommandScope
	alias.Origin = authored.Origin
	alias.RootID = authored.RootID
	alias.RootDir = authored.RootDir
	alias.ResourceScope = authored.ResourceScope
	return alias
}

func protectBundledRuntimeCommandMap(candidates map[string]*Skill, protected map[string]*Skill) {
	for key, candidate := range candidates {
		if candidate == nil {
			continue
		}
		canonical := protected[strings.TrimSpace(candidate.Meta.Name)]
		if canonical == nil || sameSkillDefinition(candidate, canonical) {
			continue
		}
		candidates[key] = bundledRuntimeCommandAlias(canonical, candidate)
	}
}

func (r *Registry) bundledRuntimeSkillsSnapshot() map[string]*Skill {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return cloneSkillMap(r.bundledRuntimeSkills)
}

func cloneSkillMap(source map[string]*Skill) map[string]*Skill {
	cloned := make(map[string]*Skill, len(source))
	for name, skill := range source {
		cloned[name] = cloneSkill(skill)
	}
	return cloned
}
