package skills

import (
	"slices"
	"strings"
)

func (r *Registry) applyDisabled(skill *Skill, disabledSkills []string) {
	if skill == nil {
		return
	}

	for _, disabled := range disabledSkills {
		if strings.TrimSpace(disabled) == skill.Meta.Name {
			skill.Enabled = false
			return
		}
	}
}

func addDisabledSkill(disabled []string, name string) []string {
	for _, existing := range disabled {
		if strings.TrimSpace(existing) == name {
			return disabled
		}
	}
	return append(disabled, name)
}

func removeDisabledSkill(disabled []string, name string) []string {
	if len(disabled) == 0 {
		return nil
	}

	filtered := make([]string, 0, len(disabled))
	for _, existing := range disabled {
		if strings.TrimSpace(existing) == name {
			continue
		}
		filtered = append(filtered, existing)
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func setDisabledSkill(disabled []string, name string, enabled bool) []string {
	if enabled {
		return removeDisabledSkill(disabled, name)
	}
	return addDisabledSkill(disabled, name)
}

func (r *Registry) globalDisabledSkillsSnapshot() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return slices.Clone(r.cfg.DisabledSkills)
}
