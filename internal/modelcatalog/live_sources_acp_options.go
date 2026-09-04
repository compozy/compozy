package modelcatalog

import (
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/acp"
)

func applyACPConfigOptions(rows []ModelRow, options []acp.SessionConfigOption) []ModelRow {
	descriptors := acpModelOptionDescriptors(options)
	reasoningOption, hasReasoning := acp.ReasoningConfigOption(options)
	efforts := acpReasoningEfforts(reasoningOption)
	defaultEffort := acpDefaultReasoningEffort(reasoningOption, efforts)

	for index := range rows {
		rows[index].ConfigOptions = mergeModelOptionDescriptors(rows[index].ConfigOptions, descriptors)
		if !hasReasoning {
			continue
		}
		rows[index].ReasoningEfforts = slices.Clone(efforts)
		rows[index].DefaultReasoningEffort = cloneModelRowPointer(defaultEffort)
		supportsReasoning := slices.ContainsFunc(efforts, func(effort ReasoningEffort) bool {
			return effort != ReasoningEffortNone
		})
		rows[index].SupportsReasoning = new(supportsReasoning)
	}
	return rows
}

func acpModelOptionDescriptors(options []acp.SessionConfigOption) []ModelOptionDescriptor {
	descriptors := make([]ModelOptionDescriptor, 0, len(options))
	for _, option := range options {
		id := strings.TrimSpace(option.ID)
		if id == "" {
			continue
		}
		descriptor := ModelOptionDescriptor{
			ID:             id,
			Label:          strings.TrimSpace(option.Label),
			Description:    strings.TrimSpace(option.Description),
			Category:       strings.TrimSpace(option.Category),
			Kind:           ModelOptionKind(option.Kind),
			CurrentValueID: strings.TrimSpace(option.CurrentValueID),
			CurrentBool:    cloneModelRowPointer(option.CurrentBool),
			Values:         make([]ModelOptionValue, 0, len(option.Values)),
		}
		for order, value := range option.Values {
			valueID := strings.TrimSpace(value.Value)
			if valueID == "" {
				continue
			}
			descriptor.Values = append(descriptor.Values, ModelOptionValue{
				ValueID:     valueID,
				Label:       strings.TrimSpace(value.Label),
				Description: strings.TrimSpace(value.Description),
				GroupID:     strings.TrimSpace(value.GroupID),
				GroupLabel:  strings.TrimSpace(value.GroupLabel),
				Order:       order,
			})
		}
		descriptors = append(descriptors, descriptor)
	}
	return descriptors
}

func acpReasoningEfforts(option acp.SessionConfigOption) []ReasoningEffort {
	efforts := make([]ReasoningEffort, 0, len(option.Values))
	for _, value := range option.Values {
		effort, ok := normalizeReasoningEffort(value.Value)
		if !ok || slices.Contains(efforts, effort) {
			continue
		}
		efforts = append(efforts, effort)
	}
	return efforts
}

func acpDefaultReasoningEffort(
	option acp.SessionConfigOption,
	efforts []ReasoningEffort,
) *ReasoningEffort {
	effort, ok := normalizeReasoningEffort(option.CurrentValueID)
	if !ok || !slices.Contains(efforts, effort) {
		return nil
	}
	return new(effort)
}
