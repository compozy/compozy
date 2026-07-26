package cli

import (
	"encoding/json"
	"errors"
	"fmt"

	"maps"

	"path/filepath"
	"sort"
	"strings"

	"github.com/compozy/agh/internal/skills"
)

func normalizeSkillName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	switch {
	case trimmed == "":
		return "", errors.New("skill name is required")
	case trimmed == ".", trimmed == "..":
		return "", errors.New("skill name must not be a relative path segment")
	case filepath.IsAbs(trimmed):
		return "", errors.New("skill name must be relative")
	case strings.Contains(trimmed, "/"), strings.Contains(trimmed, `\`):
		return "", errors.New("skill name must not include path separators")
	case !validSkillNamePattern.MatchString(trimmed):
		return "", errors.New("skill name must contain only letters, numbers, dots, underscores, and hyphens")
	default:
		return trimmed, nil
	}
}

func defaultSkillTemplate(name string) string {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		trimmedName = defaultSkillName
	}

	return fmt.Sprintf(`---
name: %q
description: Describe when to use this skill.
---

# %s

Describe the workflow, constraints, and expected outcome for this skill.
`, trimmedName, titleizeSkillName(trimmedName))
}

func titleizeSkillName(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	if len(parts) == 0 {
		return "New Skill"
	}

	titled := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}

		lower := strings.ToLower(part)
		titled = append(titled, strings.ToUpper(lower[:1])+lower[1:])
	}
	if len(titled) == 0 {
		return "New Skill"
	}
	return strings.Join(titled, " ")
}

func skillSourceLabel(source skills.SkillSource) string {
	switch source {
	case skills.SourceBundled:
		return bundledSkillSource
	case skills.SourceMarketplace:
		return marketplaceSkillSource
	case skills.SourceUser:
		return userSkillSource
	case skills.SourceAdditional:
		return additionalSkillSource
	case skills.SourceWorkspace:
		return workspaceSkillSource
	case skills.SourceAgentLocal:
		return agentLocalSkillSource
	default:
		return providerModelAvailabilityUnknown
	}
}

func sortedSkillMetadataEntries(metadata map[string]any) []keyValue {
	if len(metadata) == 0 {
		return nil
	}

	keys := make([]string, 0, len(metadata))
	for key := range metadata {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	entries := make([]keyValue, 0, len(keys))
	for _, key := range keys {
		entries = append(entries, keyValue{
			Label: key,
			Value: formatSkillMetadataValue(metadata[key]),
		})
	}
	return entries
}

func formatSkillMetadataValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	default:
		payload, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return compactJSON(payload)
	}
}

func cloneMetadata(metadata map[string]any) map[string]any {
	if metadata == nil {
		return nil
	}

	clone := make(map[string]any, len(metadata))
	maps.Copy(clone, metadata)
	return clone
}
