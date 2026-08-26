package cli

import (
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/skills"
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
			Origin:      strings.TrimSpace(skill.Origin),
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
			Origin:      strings.TrimSpace(record.Origin),
			Enabled:     record.Enabled,
			Activation:  record.Activation,
		})
	}

	return items, nil
}

func skillInfoItemFromRecord(record SkillRecord) skillInfoItem {
	item := skillInfoItem{
		Name:        record.Name,
		Description: record.Description,
		Version:     record.Version,
		Source:      strings.TrimSpace(record.Source),
		Origin:      strings.TrimSpace(record.Origin),
		Path:        record.Dir,
		Enabled:     record.Enabled,
		Activation:  record.Activation,
		Metadata:    cloneMetadata(record.Metadata),
		Provenance:  record.Provenance,
		Exposures:   []contract.SkillExposurePayload{},
	}
	if record.Exposures != nil {
		item.Exposures = append([]contract.SkillExposurePayload(nil), (*record.Exposures)...)
	}
	return item
}

func skillInfoItemFromSkill(skill *skills.Skill, resources []string, now time.Time) skillInfoItem {
	item := skillInfoItem{
		Name:        skill.Meta.Name,
		Description: skill.Meta.Description,
		Version:     skill.Meta.Version,
		Source:      skillSourceLabel(skill.Source),
		Origin:      strings.TrimSpace(skill.Origin),
		Path:        skill.FilePath,
		Enabled:     skill.Enabled,
		Activation:  skillActivationPayloadFromSkill(skill),
		Metadata:    cloneMetadata(skill.Meta.Metadata),
		Resources:   resources,
		Exposures:   []contract.SkillExposurePayload{},
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

func skillWhereItemFromRecords(detail SkillRecord, shadows SkillShadowsRecord) skillWhereItem {
	item := skillWhereItem{
		Name: detail.Name, Source: strings.TrimSpace(detail.Source), Origin: strings.TrimSpace(detail.Origin),
		Dir: detail.Dir, Winner: shadows.Winner,
		Shadows:   append([]contract.SkillShadowEntryPayload(nil), shadows.Shadows...),
		Exposures: []contract.SkillExposurePayload{},
	}
	if detail.Exposures != nil {
		item.Exposures = append([]contract.SkillExposurePayload(nil), (*detail.Exposures)...)
	}
	return item
}

func skillWhereItemFromSkill(skill *skills.Skill, shadows SkillShadowsRecord) skillWhereItem {
	if skill == nil {
		return skillWhereItem{}
	}
	return skillWhereItem{
		Name: skill.Meta.Name, Source: skillSourceLabel(skill.Source), Origin: strings.TrimSpace(skill.Origin),
		Dir: skill.Dir, Winner: shadows.Winner,
		Shadows:   append([]contract.SkillShadowEntryPayload(nil), shadows.Shadows...),
		Exposures: []contract.SkillExposurePayload{},
	}
}
