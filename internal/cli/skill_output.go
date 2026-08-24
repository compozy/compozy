package cli

import (
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	registrypkg "github.com/compozy/compozy/internal/registry"
)

const (
	skillOutputSlugValue = "Slug"
	skillOutputSlugKey   = "slug"
)

const (
	skillOutputActionValue       = "Action"
	skillOutputDescriptionValue  = "Description"
	skillOutputEnabledValue      = authoredContextEnabledValue
	skillOutputActiveValue       = "Active"
	skillOutputInactiveValue     = "Inactive reason"
	skillOutputPathValue         = "Path"
	skillOutputStatusValue       = "Status"
	skillOutputValueValue        = "Value"
	skillOutputActionKey         = authoredContextActionKey
	skillOutputCurrentVersionKey = "current_version"
	skillOutputDescriptionKey    = "description"
	skillOutputEnabledKey        = automationEnabledKey
	skillOutputActiveKey         = authoredContextActiveKey
	skillOutputInactiveKey       = "inactive_reason"
	skillOutputPathKey           = "path"
	skillOutputStatusKey         = automationStatusKey
	skillOutputValueKey          = "value"
)

func skillSearchBundle(items []registrypkg.Listing) outputBundle {
	return listBundle(
		items,
		items,
		"Marketplace Skills",
		[]string{
			skillOutputSlugValue,
			automationNameValue,
			skillOutputDescriptionValue,
			"Author",
			versionValue,
			"Downloads",
		},
		"skills",
		[]string{
			skillOutputSlugKey,
			automationNameKey,
			skillOutputDescriptionKey,
			"author",
			versionKey,
			"downloads",
		},
		func(item registrypkg.Listing) []string {
			return []string{
				stringOrDash(item.Slug),
				stringOrDash(item.Name),
				stringOrDash(item.Description),
				stringOrDash(item.Author),
				stringOrDash(item.Version),
				strconv.Itoa(item.Downloads),
			}
		},
		func(item registrypkg.Listing) []string {
			return []string{
				item.Slug,
				item.Name,
				item.Description,
				item.Author,
				item.Version,
				strconv.Itoa(item.Downloads),
			}
		},
	)
}

func skillListBundle(items []skillListItem) outputBundle {
	return listBundle(
		items,
		items,
		"Skills",
		[]string{
			automationNameValue,
			skillOutputDescriptionValue,
			authoredContextSourceValue,
			skillOutputEnabledValue,
			skillOutputActiveValue,
			skillOutputInactiveValue,
		},
		"skills",
		[]string{
			automationNameKey,
			skillOutputDescriptionKey,
			automationSourceKey,
			skillOutputEnabledKey,
			skillOutputActiveKey,
			skillOutputInactiveKey,
		},
		func(item skillListItem) []string {
			return []string{
				stringOrDash(item.Name),
				stringOrDash(item.Description),
				stringOrDash(item.Source),
				strconv.FormatBool(item.Enabled),
				strconv.FormatBool(item.Activation.Active),
				stringOrDash(skillInactiveReason(item.Activation)),
			}
		},
		func(item skillListItem) []string {
			return []string{
				item.Name,
				item.Description,
				item.Source,
				strconv.FormatBool(item.Enabled),
				strconv.FormatBool(item.Activation.Active),
				skillInactiveReason(item.Activation),
			}
		},
	)
}

func skillViewBundle(item skillViewItem, rendered string) outputBundle {
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			return rendered, nil
		},
		toon: func() (string, error) {
			return rendered, nil
		},
	}
}

func skillInfoBundle(item skillInfoItem) outputBundle {
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			base := renderHumanSection("Skill", []keyValue{
				{Label: automationNameValue, Value: stringOrDash(item.Name)},
				{Label: skillOutputDescriptionValue, Value: stringOrDash(item.Description)},
				{Label: versionValue, Value: stringOrDash(item.Version)},
				{Label: authoredContextSourceValue, Value: stringOrDash(item.Source)},
				{Label: skillOutputPathValue, Value: stringOrDash(item.Path)},
				{Label: skillOutputEnabledValue, Value: strconv.FormatBool(item.Enabled)},
				{Label: skillOutputActiveValue, Value: strconv.FormatBool(item.Activation.Active)},
				{Label: skillOutputInactiveValue, Value: stringOrDash(skillInactiveReason(item.Activation))},
			})
			provenanceRows := skillProvenanceRows(item.Provenance)
			provenance := renderHumanTable(
				"Provenance",
				[]string{cliFieldValue, skillOutputValueValue},
				provenanceRows,
			)

			metadataRows := make([][]string, 0, len(item.Metadata))
			for _, entry := range sortedSkillMetadataEntries(item.Metadata) {
				metadataRows = append(metadataRows, []string{entry.Label, entry.Value})
			}
			metadata := renderHumanTable("Metadata", []string{cliKeyValue, skillOutputValueValue}, metadataRows)

			resourceRows := make([][]string, 0, len(item.Resources))
			for _, resource := range item.Resources {
				resourceRows = append(resourceRows, []string{resource})
			}
			resources := renderHumanTable("Resources", []string{skillOutputPathValue}, resourceRows)

			return renderHumanBlocks(base, provenance, metadata, resources), nil
		},
		toon: func() (string, error) {
			metadataRows := make([][]string, 0, len(item.Metadata))
			for _, entry := range sortedSkillMetadataEntries(item.Metadata) {
				metadataRows = append(metadataRows, []string{entry.Label, entry.Value})
			}

			resourceRows := make([][]string, 0, len(item.Resources))
			for _, resource := range item.Resources {
				resourceRows = append(resourceRows, []string{resource})
			}

			return renderHumanBlocks(
				renderToonObject(
					"skill",
					[]string{
						automationNameKey,
						skillOutputDescriptionKey,
						versionKey,
						automationSourceKey,
						skillOutputPathKey,
						skillOutputEnabledKey,
						skillOutputActiveKey,
						skillOutputInactiveKey,
					},
					[]string{
						item.Name,
						item.Description,
						item.Version,
						item.Source,
						item.Path,
						strconv.FormatBool(item.Enabled),
						strconv.FormatBool(item.Activation.Active),
						skillInactiveReason(item.Activation),
					},
				),
				renderToonArray(
					"provenance",
					[]string{cliFieldKey, skillOutputValueKey},
					skillProvenanceRows(item.Provenance),
				),
				renderToonArray("metadata", []string{cliKeyKey, skillOutputValueKey}, metadataRows),
				renderToonArray("resources", []string{skillOutputPathKey}, resourceRows),
			), nil
		},
	}
}

func skillInactiveReason(activation contract.SkillActivationPayload) string {
	if activation.Active || len(activation.Reasons) == 0 {
		return ""
	}
	messages := make([]string, 0, len(activation.Reasons))
	for _, reason := range activation.Reasons {
		if message := strings.TrimSpace(reason.Message); message != "" {
			messages = append(messages, message)
		}
	}
	return strings.Join(messages, "; ")
}

func skillProvenanceRows(provenance *SkillProvenanceRecord) [][]string {
	if provenance == nil {
		return nil
	}
	rows := [][]string{
		{"precedence_tier", stringOrDash(provenance.PrecedenceTier)},
	}
	if provenance.Slug != "" {
		rows = append(rows, []string{"slug", provenance.Slug})
	}
	if provenance.Registry != "" {
		rows = append(rows, []string{skillOutputRegistryKey, provenance.Registry})
	}
	if provenance.Version != "" {
		rows = append(rows, []string{versionKey, provenance.Version})
	}
	if provenance.InstalledFromExtension != "" {
		rows = append(rows, []string{"installed_from_extension", provenance.InstalledFromExtension})
	}
	if count := len(provenance.ShadowedBy); count > 0 {
		rows = append(rows, []string{"shadowed_definitions", strconv.Itoa(count)})
	}
	return rows
}

func skillWhereBundle(record SkillShadowsRecord) outputBundle {
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			rows := skillWhereRows(record)
			return renderHumanBlocks(
				renderHumanSection("Skill Resolution", []keyValue{
					{Label: automationNameValue, Value: stringOrDash(record.Name)},
					{Label: "Winner", Value: stringOrDash(record.Winner.Path)},
					{Label: extensionMarketplaceTierValue, Value: stringOrDash(record.Winner.Tier)},
				}),
				renderHumanTable(
					"Locations",
					[]string{"Winner", extensionMarketplaceTierValue, skillOutputPathValue},
					rows,
				),
			), nil
		},
		toon: func() (string, error) {
			return renderHumanBlocks(
				renderToonObject(
					"skill_resolution",
					[]string{automationNameKey, "winner_path", "winner_tier"},
					[]string{record.Name, record.Winner.Path, record.Winner.Tier},
				),
				renderToonArray(
					"locations",
					[]string{"winner", extensionMarketplaceTierKey, skillOutputPathKey},
					skillWhereRows(record),
				),
			), nil
		},
	}
}

func skillWhereRows(record SkillShadowsRecord) [][]string {
	rows := make([][]string, 0, len(record.Shadows))
	for _, entry := range record.Shadows {
		winner := "no"
		if entry.ResolvedToWinner {
			winner = yesFlagName
		}
		rows = append(rows, []string{
			winner,
			stringOrDash(entry.Tier),
			stringOrDash(entry.Path),
		})
	}
	return rows
}

func skillCreateBundle(item skillCreateItem) outputBundle {
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			entries := []keyValue{
				{Label: automationNameValue, Value: stringOrDash(item.Name)},
			}
			if item.Group != "" {
				entries = append(entries, keyValue{Label: "Group", Value: item.Group})
			}
			entries = append(entries,
				keyValue{Label: authoredContextSourceValue, Value: stringOrDash(item.Source)},
				keyValue{Label: skillOutputPathValue, Value: stringOrDash(item.Path)},
				keyValue{Label: "File", Value: stringOrDash(item.File)},
				keyValue{Label: skillOutputStatusValue, Value: stringOrDash(item.Status)},
			)
			return renderHumanSection("Skill", entries), nil
		},
		toon: func() (string, error) {
			keys := []string{automationNameKey}
			values := []string{item.Name}
			if item.Group != "" {
				keys = append(keys, "group")
				values = append(values, item.Group)
			}
			keys = append(keys, automationSourceKey, skillOutputPathKey, "file", skillOutputStatusKey)
			values = append(values, item.Source, item.Path, item.File, item.Status)
			return renderToonObject(
				"skill",
				keys,
				values,
			), nil
		},
	}
}

func skillActionBundle(name string, action string, record SkillActionRecord) outputBundle {
	item := struct {
		Name   string `json:"name"`
		Action string `json:"action"`
		OK     bool   `json:"ok"`
	}{
		Name:   name,
		Action: action,
		OK:     record.OK,
	}
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			return renderHumanSection("Skill Action", []keyValue{
				{Label: automationNameValue, Value: stringOrDash(item.Name)},
				{Label: skillOutputActionValue, Value: stringOrDash(item.Action)},
				{Label: "OK", Value: strconv.FormatBool(item.OK)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject("skill_action", []string{automationNameKey, skillOutputActionKey, "ok"}, []string{
				item.Name,
				item.Action,
				strconv.FormatBool(item.OK),
			}), nil
		},
	}
}
