package modelcatalog

import (
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/acp"
)

// applyACPConfigOptions layers session config options discovered by one short-lived ACP
// inspection onto the parsed model rows. The inspection session only ever has one model
// active at a time, so its reasoning select reflects that model alone — attributing it to
// every row would claim effort levels a model was never confirmed to support.
func applyACPConfigOptions(rows []ModelRow, options []acp.SessionConfigOption) []ModelRow {
	descriptors := acpModelOptionDescriptors(options)
	sharedDescriptors := descriptorsExcludingReasoning(descriptors)
	reasoningOption, hasReasoning := acp.ReasoningConfigOption(options)
	efforts := acpReasoningEfforts(reasoningOption)
	defaultEffort := acpDefaultReasoningEffort(reasoningOption, efforts)
	activeIndex := activeModelRowIndex(rows, options)

	for index := range rows {
		merge := sharedDescriptors
		if index == activeIndex {
			merge = descriptors
		}
		rows[index].ConfigOptions = mergeModelOptionDescriptors(rows[index].ConfigOptions, merge)
		if !hasReasoning || index != activeIndex {
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

// activeModelRowIndex finds the row matching the model transport id selected in this
// inspection session, or -1 when the ACP session did not advertise a resolvable current
// model. Only that row's reasoning descriptor is verified for the model it names.
func activeModelRowIndex(rows []ModelRow, options []acp.SessionConfigOption) int {
	modelOption, ok := acp.ModelConfigOption(options)
	if !ok {
		return -1
	}
	activeTransportID := strings.TrimSpace(modelOption.CurrentValueID)
	if activeTransportID == "" {
		return -1
	}
	for index, row := range rows {
		for _, binding := range row.TransportBindings {
			if strings.EqualFold(strings.TrimSpace(binding.TransportModelID), activeTransportID) {
				return index
			}
		}
	}
	return -1
}

func descriptorsExcludingReasoning(descriptors []ModelOptionDescriptor) []ModelOptionDescriptor {
	filtered := make([]ModelOptionDescriptor, 0, len(descriptors))
	for _, descriptor := range descriptors {
		if isReasoningOptionDescriptor(descriptor) {
			continue
		}
		filtered = append(filtered, descriptor)
	}
	return filtered
}

func isReasoningOptionDescriptor(descriptor ModelOptionDescriptor) bool {
	id := strings.TrimSpace(descriptor.ID)
	return id == "reasoning_effort" || id == "effort" || strings.TrimSpace(descriptor.Category) == "thought_level"
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
