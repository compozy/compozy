package skills

import (
	"slices"
	"strings"
)

func (r *Registry) logWorkspaceSkillOverrides(
	globalSkills map[string]*Skill,
	workspaceSkills map[string]*Skill,
	workspaceID string,
) {
	if len(globalSkills) == 0 || len(workspaceSkills) == 0 {
		return
	}

	names := make([]string, 0, len(workspaceSkills))
	for name, skill := range workspaceSkills {
		if skill == nil {
			continue
		}
		if globalSkills[name] != nil {
			names = append(names, name)
		}
	}
	slices.Sort(names)

	for _, name := range names {
		r.logSkillOverride(globalSkills[name], workspaceSkills[name], workspaceID)
	}
}

func (r *Registry) logSkillOverride(existing *Skill, skill *Skill, workspaceID string) {
	if r == nil || r.logger == nil || existing == nil || skill == nil {
		return
	}

	attrs := []any{
		"name", skill.Meta.Name,
		"old_source", skillSourceName(existing.Source),
		"new_source", skillSourceName(skill.Source),
		"old_path", existing.FilePath,
		"new_path", skill.FilePath,
	}
	if trimmedWorkspaceID := strings.TrimSpace(workspaceID); trimmedWorkspaceID != "" {
		attrs = append(attrs, "workspace_id", trimmedWorkspaceID)
	}

	r.logger.Warn("skills: overriding skill", attrs...)
}

func (r *Registry) overlaySkill(dst map[string]*Skill, skill *Skill) {
	if existing, ok := dst[skill.Meta.Name]; ok {
		r.logSkillOverride(existing, skill, "")
		skill.Diagnostics.ShadowedDefinitions = append(
			cloneSkillDefinitionRefs(skill.Diagnostics.ShadowedDefinitions),
			shadowDefinitionRefsForWinner(existing, r.now())...,
		)
	}

	dst[skill.Meta.Name] = skill
}

func appendSkillDiagnostic(dst *[]SkillDiagnostic, diagnostic SkillDiagnostic) {
	if dst == nil {
		return
	}
	*dst = append(*dst, cloneDiagnostic(diagnostic))
}

func (r *Registry) logVerificationWarnings(skill *Skill, warnings []Warning) {
	for _, warning := range warnings {
		if warning.Severity == SeverityInfo {
			r.logger.Info(
				"skills: verification warning",
				"name", skill.Meta.Name,
				"source", skillSourceName(skill.Source),
				"path", skill.FilePath,
				"severity", warningSeverityName(warning.Severity),
				"pattern", warning.Pattern,
				"message", warning.Message,
			)
			continue
		}

		r.logger.Warn(
			"skills: verification warning",
			"name", skill.Meta.Name,
			"source", skillSourceName(skill.Source),
			"path", skill.FilePath,
			"severity", warningSeverityName(warning.Severity),
			"pattern", warning.Pattern,
			"message", warning.Message,
		)
	}
}
