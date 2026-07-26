package skills

import (
	"strings"
	"time"
)

// ShadowsForSkill returns the resolver winner and every known shadowed
// definition for one effective skill. It does not perform resolution itself.
func ShadowsForSkill(skill *Skill, fallbackDetectedAt time.Time) (SkillShadows, bool) {
	if skill == nil || strings.TrimSpace(skill.Meta.Name) == "" {
		return SkillShadows{}, false
	}
	winner := shadowEntryForWinner(skill, fallbackDetectedAt)
	entries := make([]ShadowEntry, 0, len(skill.Diagnostics.ShadowedDefinitions)+1)
	entries = append(entries, winner)
	for _, ref := range skill.Diagnostics.ShadowedDefinitions {
		entries = append(entries, shadowEntryForRef(ref, fallbackDetectedAt))
	}
	return SkillShadows{
		Name:    strings.TrimSpace(skill.Meta.Name),
		Winner:  winner,
		Shadows: entries,
	}, true
}

// ShadowsForSkillList returns shadow evidence for a named winner in a resolved
// skill list.
func ShadowsForSkillList(skills []*Skill, name string, fallbackDetectedAt time.Time) (SkillShadows, bool) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return SkillShadows{}, false
	}
	for _, skill := range skills {
		if skill == nil || strings.TrimSpace(skill.Meta.Name) != trimmed {
			continue
		}
		return ShadowsForSkill(skill, fallbackDetectedAt)
	}
	return SkillShadows{}, false
}

func shadowEntryForWinner(skill *Skill, fallbackDetectedAt time.Time) ShadowEntry {
	return ShadowEntry{
		Path:             skillPathForShadow(skill),
		Tier:             SkillPrecedenceTierName(skill.Source),
		ResolvedToWinner: true,
		DetectedAt:       normalizedShadowDetectedAt(fallbackDetectedAt),
	}
}

func shadowEntryForRef(ref SkillDefinitionRef, fallbackDetectedAt time.Time) ShadowEntry {
	detectedAt := ref.DetectedAt
	if detectedAt.IsZero() {
		detectedAt = fallbackDetectedAt
	}
	return ShadowEntry{
		Path:             strings.TrimSpace(ref.Path),
		Tier:             precedenceTierFromSourceLabel(ref.Source),
		ResolvedToWinner: false,
		DetectedAt:       normalizedShadowDetectedAt(detectedAt),
	}
}

func shadowDefinitionRefsForWinner(skill *Skill, detectedAt time.Time) []SkillDefinitionRef {
	if skill == nil {
		return nil
	}
	refs := []SkillDefinitionRef{{
		Source:     skillSourceName(skill.Source),
		Path:       skillPathForShadow(skill),
		DetectedAt: normalizedShadowDetectedAt(detectedAt),
	}}
	for _, ref := range skill.Diagnostics.ShadowedDefinitions {
		cloned := ref
		if cloned.DetectedAt.IsZero() {
			cloned.DetectedAt = refs[0].DetectedAt
		}
		refs = append(refs, cloned)
	}
	return refs
}

func precedenceTierFromSourceLabel(source string) string {
	trimmed := strings.TrimSpace(source)
	if trimmed == registryAgentLocalValue {
		return "agent_local"
	}
	return trimmed
}

func skillPathForShadow(skill *Skill) string {
	if skill == nil {
		return ""
	}
	if path := strings.TrimSpace(skill.FilePath); path != "" {
		return path
	}
	return strings.TrimSpace(skill.Dir)
}

func normalizedShadowDetectedAt(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value.UTC()
}
