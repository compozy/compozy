package cli

import (
	"fmt"
	"path/filepath"
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

func renderSkillInfoTranscript(item skillInfoItem) string {
	rows := []struct {
		label string
		value string
	}{
		{label: "NAME", value: stringOrDash(item.Name)},
		{label: "SOURCE", value: stringOrDash(item.Source)},
		{label: "DIR", value: stringOrDash(item.Path)},
	}
	if len(item.Exposures) == 0 {
		rows = append(rows, struct {
			label string
			value string
		}{label: "EXPOSED TO", value: "— none —"})
	} else {
		for index, exposure := range item.Exposures {
			label := ""
			if index == 0 {
				label = "EXPOSED TO"
			}
			rows = append(rows, struct {
				label string
				value string
			}{label: label, value: fmt.Sprintf(
				"%s → %s (%s)", exposure.Target, exposure.Path, skillExposureStatusLabel(exposure.Status),
			)})
		}
	}
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf("%-10s   %s", row.label, row.value))
	}
	return strings.Join(lines, "\n")
}

func skillExposureStatusLabel(status contract.SkillExposureStatus) string {
	trimmed := strings.TrimSpace(string(status))
	switch trimmed {
	case "missing":
		return "missing — re-expose repairs"
	case "broken":
		return "broken — unexpose or re-expose repairs"
	case "foreign_conflict":
		return "foreign conflict — not our link; no action"
	default:
		return strings.ReplaceAll(trimmed, "_", " ")
	}
}

func skillListBundle(items []skillListItem) outputBundle {
	bundle := listBundle(
		items,
		items,
		"",
		[]string{
			automationNameValue,
			authoredContextSourceValue,
			"Origin",
			skillOutputDescriptionValue,
		},
		"skills",
		[]string{
			automationNameKey,
			skillOutputDescriptionKey,
			automationSourceKey,
			"origin",
			skillOutputEnabledKey,
			skillOutputActiveKey,
			skillOutputInactiveKey,
		},
		func(item skillListItem) []string {
			return []string{
				stringOrDash(item.Name),
				stringOrDash(item.Source),
				stringOrDash(item.Origin),
				stringOrDash(item.Description),
			}
		},
		func(item skillListItem) []string {
			return []string{
				item.Name,
				item.Description,
				item.Source,
				item.Origin,
				strconv.FormatBool(item.Enabled),
				strconv.FormatBool(item.Activation.Active),
				skillInactiveReason(item.Activation),
			}
		},
	)
	bundle.human = func() (string, error) {
		return renderSkillListTranscript(items), nil
	}
	return bundle
}

func renderSkillListTranscript(items []skillListItem) string {
	rows := make([][]string, 0, len(items)+1)
	rows = append(rows, []string{"NAME", "SOURCE", "ORIGIN", "DESCRIPTION"})
	for _, item := range items {
		origin := strings.TrimSpace(item.Origin)
		if origin == "" {
			origin = "—"
		}
		rows = append(rows, []string{
			stringOrDash(item.Name),
			stringOrDash(item.Source),
			origin,
			stringOrDash(item.Description),
		})
	}
	widths := humanTableColumnWidths(rows)
	var builder strings.Builder
	for _, row := range rows {
		writeHumanTableRow(&builder, row, widths)
	}
	return strings.TrimRight(builder.String(), "\n")
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
			return renderSkillInfoTranscript(item), nil
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

func skillWhereBundle(record skillWhereItem) outputBundle {
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			return renderSkillWhereTranscript(record), nil
		},
		toon: func() (string, error) {
			return renderSkillWhereTranscript(record), nil
		},
	}
}

func renderSkillWhereTranscript(record skillWhereItem) string {
	winnerOrigin := strings.TrimSpace(record.Origin)
	if winnerOrigin == "" {
		winnerOrigin = "compozy"
	}
	winnerPath := strings.TrimSpace(record.Dir)
	if winnerPath == "" {
		winnerPath = skillDefinitionDirectory(record.Winner.Path)
	}
	lines := []string{fmt.Sprintf(
		"WINNER   %s (%s · %s)", winnerPath, record.Source, winnerOrigin,
	)}
	alternatives := make([]contract.SkillShadowEntryPayload, 0, len(record.Shadows))
	for _, entry := range record.Shadows {
		if !entry.ResolvedToWinner {
			alternatives = append(alternatives, entry)
		}
	}
	if len(alternatives) == 0 {
		lines = append(lines, "ALSO     — none —")
	} else {
		qualifiedOrigins := make(map[string]struct{})
		for index, entry := range alternatives {
			label := "         "
			if index == 0 {
				label = "ALSO     "
			}
			origin := strings.TrimSpace(entry.Origin)
			if origin == "" {
				origin = skillOriginFromPath(entry.Path)
			}
			hint := ""
			if origin != "compozy" && origin != "custom" {
				_, alreadyHinted := qualifiedOrigins[origin]
				if !alreadyHinted {
					qualifiedOrigins[origin] = struct{}{}
					hint = " — invoke as " + origin + ":" + record.Name
				}
			}
			lines = append(lines, fmt.Sprintf(
				"%s%s (%s · %s · shadowed%s)",
				label, skillDefinitionDirectory(entry.Path), entry.Tier, origin, hint,
			))
		}
	}
	for index, exposure := range record.Exposures {
		label := "         "
		if index == 0 {
			label = "LINKS    "
		}
		lines = append(lines, fmt.Sprintf(
			"%s%s → %s (exposure · %s)",
			label, exposure.Path, record.Dir, skillExposureStatusLabel(exposure.Status),
		))
	}
	return strings.Join(lines, "\n")
}

func skillDefinitionDirectory(path string) string {
	trimmed := strings.TrimSpace(path)
	if strings.EqualFold(filepath.Base(trimmed), skillMarkdownFileName) {
		return filepath.Dir(trimmed)
	}
	return trimmed
}

func skillOriginFromPath(path string) string {
	normalized := strings.ReplaceAll(strings.TrimSpace(path), "\\", "/")
	switch {
	case strings.Contains(normalized, "/.agents/skills/"):
		return "agents"
	case strings.Contains(normalized, "/.claude/skills/"):
		return "claude"
	case strings.Contains(normalized, "/.compozy/skills/"):
		return "compozy"
	default:
		return "custom"
	}
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
