package modelcatalog

import (
	"cmp"
	"slices"
	"strings"
)

func mergeModelOptionDescriptors(
	descriptors []ModelOptionDescriptor,
	incoming []ModelOptionDescriptor,
) []ModelOptionDescriptor {
	if len(incoming) == 0 {
		return descriptors
	}
	indexes := make(map[string]int, len(descriptors)+len(incoming))
	for index := range descriptors {
		id := strings.TrimSpace(descriptors[index].ID)
		if id != "" {
			indexes[id] = index
		}
	}
	for _, incomingDescriptor := range incoming {
		id := strings.TrimSpace(incomingDescriptor.ID)
		if id == "" {
			continue
		}
		incomingDescriptor.ID = id
		if index, exists := indexes[id]; exists {
			mergeModelOptionDescriptor(&descriptors[index], incomingDescriptor)
			continue
		}
		indexes[id] = len(descriptors)
		descriptors = append(descriptors, cloneModelOptionDescriptor(incomingDescriptor))
	}
	slices.SortFunc(descriptors, func(left ModelOptionDescriptor, right ModelOptionDescriptor) int {
		return cmp.Compare(left.ID, right.ID)
	})
	for index := range descriptors {
		sortModelOptionValues(descriptors[index].Values)
	}
	return descriptors
}

func cloneModelOptionDescriptor(option ModelOptionDescriptor) ModelOptionDescriptor {
	cloned := option
	cloned.CurrentBool = cloneModelRowPointer(option.CurrentBool)
	cloned.Values = slices.Clone(option.Values)
	return cloned
}

func mergeModelOptionDescriptor(
	descriptor *ModelOptionDescriptor,
	incoming ModelOptionDescriptor,
) {
	if descriptor.Label == "" {
		descriptor.Label = incoming.Label
	}
	if descriptor.Description == "" {
		descriptor.Description = incoming.Description
	}
	if descriptor.Category == "" {
		descriptor.Category = incoming.Category
	}
	if descriptor.Kind == "" {
		descriptor.Kind = incoming.Kind
	}
	if descriptor.CurrentValueID == "" {
		descriptor.CurrentValueID = incoming.CurrentValueID
	}
	if descriptor.CurrentBool == nil {
		descriptor.CurrentBool = cloneModelRowPointer(incoming.CurrentBool)
	}
	descriptor.Values = mergeModelOptionValues(descriptor.Values, incoming.Values)
}

func mergeModelOptionValues(
	values []ModelOptionValue,
	incoming []ModelOptionValue,
) []ModelOptionValue {
	if len(incoming) == 0 {
		return values
	}
	indexes := make(map[string]int, len(values)+len(incoming))
	for index := range values {
		id := strings.TrimSpace(values[index].ValueID)
		if id != "" {
			indexes[id] = index
		}
	}
	for _, incomingValue := range incoming {
		id := strings.TrimSpace(incomingValue.ValueID)
		if id == "" {
			continue
		}
		incomingValue.ValueID = id
		if index, exists := indexes[id]; exists {
			mergeModelOptionValue(&values[index], incomingValue)
			continue
		}
		indexes[id] = len(values)
		values = append(values, incomingValue)
	}
	sortModelOptionValues(values)
	return values
}

func mergeModelOptionValue(value *ModelOptionValue, incoming ModelOptionValue) {
	if value.Label == "" {
		value.Label = incoming.Label
	}
	if value.Description == "" {
		value.Description = incoming.Description
	}
	if value.GroupID == "" {
		value.GroupID = incoming.GroupID
	}
	if value.GroupLabel == "" {
		value.GroupLabel = incoming.GroupLabel
	}
}

func mergeTransportBinding(
	binding *ModelTransportBinding,
	incoming ModelTransportBinding,
) {
	if binding.Label == "" {
		binding.Label = incoming.Label
	}
	if binding.ReasoningEffort == nil {
		binding.ReasoningEffort = cloneModelRowPointer(incoming.ReasoningEffort)
	}
	if binding.Fast == nil {
		binding.Fast = cloneModelRowPointer(incoming.Fast)
	}
	if binding.Thinking == nil {
		binding.Thinking = cloneModelRowPointer(incoming.Thinking)
	}
	binding.OptionSelections = mergeModelOptionSelections(binding.OptionSelections, incoming.OptionSelections)
}

func mergeModelOptionSelections(
	selections []ModelOptionSelection,
	incoming []ModelOptionSelection,
) []ModelOptionSelection {
	if len(incoming) == 0 {
		return selections
	}
	indexes := make(map[string]int, len(selections)+len(incoming))
	for index := range selections {
		id := strings.TrimSpace(selections[index].ID)
		if id != "" {
			indexes[id] = index
		}
	}
	for _, incomingSelection := range incoming {
		id := strings.TrimSpace(incomingSelection.ID)
		if id == "" {
			continue
		}
		incomingSelection.ID = id
		if _, exists := indexes[id]; exists {
			continue
		}
		candidate := incomingSelection
		candidate.BoolValue = cloneModelRowPointer(incomingSelection.BoolValue)
		indexes[id] = len(selections)
		selections = append(selections, candidate)
	}
	slices.SortFunc(selections, func(left ModelOptionSelection, right ModelOptionSelection) int {
		return cmp.Compare(left.ID, right.ID)
	})
	return selections
}
