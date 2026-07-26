package skills

import (
	"fmt"
	"strings"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

func refreshSkillHookDecls(skill *Skill) {
	if skill == nil || len(skill.Hooks) == 0 {
		return
	}

	normalized := make([]hookspkg.HookDecl, len(skill.Hooks))
	for idx, decl := range skill.Hooks {
		normalized[idx] = normalizeSkillHookDecl(skill, decl, idx, len(skill.Hooks))
	}

	skill.Hooks = normalized
}

func normalizeSkillHookDecl(skill *Skill, decl hookspkg.HookDecl, index int, total int) hookspkg.HookDecl {
	normalized := decl
	if strings.TrimSpace(normalized.Name) == "" {
		normalized.Name = skillHookName(skill, index, total)
	}
	normalized.Source = hookspkg.HookSourceSkill
	normalized.SkillSource = skillHookSource(skillSource(skill))
	return normalized
}

func skillHookName(skill *Skill, index int, total int) string {
	base := skillIdentifier(skill)
	if total <= 1 {
		return base
	}

	return fmt.Sprintf("%s#%d", base, index+1)
}

func skillHookSource(source SkillSource) hookspkg.HookSkillSource {
	switch source {
	case SourceBundled:
		return hookspkg.HookSkillSourceBundled
	case SourceMarketplace:
		return hookspkg.HookSkillSourceMarketplace
	case SourceUser:
		return hookspkg.HookSkillSourceUser
	case SourceAdditional:
		return hookspkg.HookSkillSourceAdditional
	case SourceWorkspace:
		return hookspkg.HookSkillSourceWorkspace
	default:
		return ""
	}
}

func skillSource(skill *Skill) SkillSource {
	if skill == nil {
		return SourceUser
	}

	return skill.Source
}
