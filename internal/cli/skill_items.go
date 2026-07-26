package cli

import (
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/skills"
)

func skillListItems(allSkills []*skills.Skill, sourceFilter string) ([]skillListItem, error) {
	filter, err := normalizeSkillSourceFilter(sourceFilter)
	if err != nil {
		return nil, err
	}

	items := make([]skillListItem, 0, len(allSkills))
	for _, skill := range allSkills {
		if skill == nil {
			continue
		}

		source := skillSourceLabel(skill.Source)
		if filter != "" && source != filter {
			continue
		}

		items = append(items, skillListItem{
			Name:        skill.Meta.Name,
			Description: skill.Meta.Description,
			Source:      source,
			Enabled:     skill.Enabled,
			Activation:  skillActivationPayloadFromSkill(skill),
		})
	}

	return items, nil
}

func skillListItemsFromRecords(records []SkillRecord, sourceFilter string) ([]skillListItem, error) {
	filter, err := normalizeSkillSourceFilter(sourceFilter)
	if err != nil {
		return nil, err
	}

	items := make([]skillListItem, 0, len(records))
	for _, record := range records {
		source := strings.TrimSpace(record.Source)
		if filter != "" && source != filter {
			continue
		}

		items = append(items, skillListItem{
			Name:        record.Name,
			Description: record.Description,
			Source:      source,
			Enabled:     record.Enabled,
			Activation:  record.Activation,
		})
	}

	return items, nil
}

func skillInfoItemFromRecord(record SkillRecord) skillInfoItem {
	return skillInfoItem{
		Name:        record.Name,
		Description: record.Description,
		Version:     record.Version,
		Source:      strings.TrimSpace(record.Source),
		Path:        record.Dir,
		Enabled:     record.Enabled,
		Activation:  record.Activation,
		Metadata:    cloneMetadata(record.Metadata),
		Provenance:  record.Provenance,
	}
}

func skillInfoItemFromSkill(skill *skills.Skill, resources []string, now time.Time) skillInfoItem {
	item := skillInfoItem{
		Name:        skill.Meta.Name,
		Description: skill.Meta.Description,
		Version:     skill.Meta.Version,
		Source:      skillSourceLabel(skill.Source),
		Path:        skill.FilePath,
		Enabled:     skill.Enabled,
		Activation:  skillActivationPayloadFromSkill(skill),
		Metadata:    cloneMetadata(skill.Meta.Metadata),
		Resources:   resources,
	}
	shadows, ok := skills.ShadowsForSkill(skill, now)
	if ok {
		provenance := skillProvenanceRecordFromSkill(skill, shadows)
		item.Provenance = &provenance
	}
	return item
}

func skillActivationPayloadFromSkill(skill *skills.Skill) contract.SkillActivationPayload {
	if skill == nil {
		return contract.SkillActivationPayload{}
	}
	reasons := make([]contract.SkillActivationReasonPayload, 0, len(skill.Activation.Reasons))
	for _, reason := range skill.Activation.Reasons {
		reasons = append(reasons, contract.SkillActivationReasonPayload{
			Gate:    string(reason.Gate),
			Code:    contract.SkillActivationReasonCode(reason.Code),
			Missing: append([]string(nil), reason.Missing...),
			Message: reason.Message,
		})
	}
	return contract.SkillActivationPayload{
		Active:  !skill.Activation.Evaluated || skill.Activation.Active,
		Reasons: reasons,
	}
}
