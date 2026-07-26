package cli

import (
	"strconv"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	registrypkg "github.com/compozy/agh/internal/registry"
)

const (
	skillOutputSlugValue = "Slug"
	skillOutputSlugKey   = "slug"
)

const (
	skillOutputActionValue       = "Action"
	skillOutputDescriptionValue  = "Description"
	skillOutputEnabledValue      = "Enabled"
	skillOutputActiveValue       = "Active"
	skillOutputInactiveValue     = "Inactive reason"
	skillOutputPathValue         = "Path"
	skillOutputStatusValue       = "Status"
	skillOutputValueValue        = "Value"
	skillOutputActionKey         = "action"
	skillOutputCurrentVersionKey = "current_version"
	skillOutputDescriptionKey    = "description"
	skillOutputEnabledKey        = "enabled"
	skillOutputActiveKey         = "active"
	skillOutputInactiveKey       = "inactive_reason"
	skillOutputPathKey           = "path"
	skillOutputStatusKey         = "status"
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
	if provenance.InstalledFromBundle != "" {
		rows = append(rows, []string{"installed_from_bundle", provenance.InstalledFromBundle})
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
					{Label: "Tier", Value: stringOrDash(record.Winner.Tier)},
				}),
				renderHumanTable("Locations", []string{"Winner", "Tier", skillOutputPathValue}, rows),
			), nil
		},
		toon: func() (string, error) {
			return renderHumanBlocks(
				renderToonObject(
					"skill_resolution",
					[]string{automationNameKey, "winner_path", "winner_tier"},
					[]string{record.Name, record.Winner.Path, record.Winner.Tier},
				),
				renderToonArray("locations", []string{"winner", "tier", skillOutputPathKey}, skillWhereRows(record)),
			), nil
		},
	}
}

func skillWhereRows(record SkillShadowsRecord) [][]string {
	rows := make([][]string, 0, len(record.Shadows))
	for _, entry := range record.Shadows {
		winner := "no"
		if entry.ResolvedToWinner {
			winner = "yes"
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
			return renderHumanSection("Skill", []keyValue{
				{Label: automationNameValue, Value: stringOrDash(item.Name)},
				{Label: authoredContextSourceValue, Value: stringOrDash(item.Source)},
				{Label: skillOutputPathValue, Value: stringOrDash(item.Path)},
				{Label: "File", Value: stringOrDash(item.File)},
				{Label: skillOutputStatusValue, Value: stringOrDash(item.Status)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"skill",
				[]string{automationNameKey, automationSourceKey, skillOutputPathKey, "file", skillOutputStatusKey},
				[]string{
					item.Name,
					item.Source,
					item.Path,
					item.File,
					item.Status,
				},
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

func skillInstallBundle(item skillInstallItem) outputBundle {
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			return renderHumanSection("Skill Install", []keyValue{
				{Label: automationNameValue, Value: stringOrDash(item.Name)},
				{Label: skillOutputSlugValue, Value: stringOrDash(item.Slug)},
				{Label: versionValue, Value: stringOrDash(item.Version)},
				{Label: "Registry", Value: stringOrDash(item.Registry)},
				{Label: skillOutputPathValue, Value: stringOrDash(item.Path)},
				{Label: cliHashValue, Value: stringOrDash(item.Hash)},
				{Label: skillOutputStatusValue, Value: stringOrDash(item.Status)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"skill_install",
				[]string{
					automationNameKey,
					skillOutputSlugKey,
					versionKey,
					skillOutputRegistryKey,
					skillOutputPathKey,
					"hash",
					skillOutputStatusKey,
				},
				[]string{
					item.Name,
					item.Slug,
					item.Version,
					item.Registry,
					item.Path,
					item.Hash,
					item.Status,
				},
			), nil
		},
	}
}

func skillRemoveBundle(item skillRemoveItem) outputBundle {
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			return renderHumanSection("Skill Remove", []keyValue{
				{Label: automationNameValue, Value: stringOrDash(item.Name)},
				{Label: skillOutputSlugValue, Value: stringOrDash(item.Slug)},
				{Label: skillOutputPathValue, Value: stringOrDash(item.Path)},
				{Label: skillOutputStatusValue, Value: stringOrDash(item.Status)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"skill_remove",
				[]string{automationNameKey, skillOutputSlugKey, skillOutputPathKey, skillOutputStatusKey},
				[]string{
					item.Name,
					item.Slug,
					item.Path,
					item.Status,
				},
			), nil
		},
	}
}

func skillUpdateBundle(items []skillUpdateItem) outputBundle {
	return listBundle(
		items,
		items,
		"Skill Updates",
		[]string{
			automationNameValue,
			skillOutputSlugValue,
			"Current",
			"Latest",
			skillOutputPathValue,
			skillOutputStatusValue,
		},
		"skill_updates",
		[]string{
			automationNameKey,
			skillOutputSlugKey,
			skillOutputCurrentVersionKey,
			"latest_version",
			skillOutputPathKey,
			skillOutputStatusKey,
		},
		func(item skillUpdateItem) []string {
			return []string{
				stringOrDash(item.Name),
				stringOrDash(item.Slug),
				stringOrDash(item.CurrentVersion),
				stringOrDash(item.LatestVersion),
				stringOrDash(item.Path),
				stringOrDash(item.Status),
			}
		},
		func(item skillUpdateItem) []string {
			return []string{
				item.Name,
				item.Slug,
				item.CurrentVersion,
				item.LatestVersion,
				item.Path,
				item.Status,
			}
		},
	)
}
