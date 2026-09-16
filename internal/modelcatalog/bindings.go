package modelcatalog

import (
	"cmp"
	"slices"
	"strings"
)

// ModelTransportBinding maps one logical model configuration to its provider-owned identifier.
type ModelTransportBinding struct {
	TransportModelID string
	Label            string
	ReasoningEffort  *ReasoningEffort
	Fast             *bool
	Thinking         *bool
	OptionSelections []ModelOptionSelection
	// ConfigOptions is the complete inspected snapshot for this transport, not a union.
	ConfigOptions []ModelOptionDescriptor
}

// cloneTransportBindings copies binding snapshots so callers cannot mutate cached capabilities.
func cloneTransportBindings(bindings []ModelTransportBinding) []ModelTransportBinding {
	if len(bindings) == 0 {
		return nil
	}
	cloned := make([]ModelTransportBinding, len(bindings))
	for index, binding := range bindings {
		cloned[index] = binding
		cloned[index].ReasoningEffort = cloneModelRowPointer(binding.ReasoningEffort)
		cloned[index].Fast = cloneModelRowPointer(binding.Fast)
		cloned[index].Thinking = cloneModelRowPointer(binding.Thinking)
		cloned[index].ConfigOptions = CloneModelOptionDescriptors(binding.ConfigOptions)
		cloned[index].OptionSelections = CloneModelOptionSelections(binding.OptionSelections)
	}
	return cloned
}

// appendTransportBinding merges one transport identity while retaining its own option snapshot.
func appendTransportBinding(bindings []ModelTransportBinding, binding ModelTransportBinding) []ModelTransportBinding {
	if binding.TransportModelID == "" {
		return bindings
	}
	for index := range bindings {
		if bindings[index].TransportModelID == binding.TransportModelID {
			mergeTransportBinding(&bindings[index], binding)
			return sortTransportBindings(bindings)
		}
	}
	binding.ConfigOptions = CloneModelOptionDescriptors(binding.ConfigOptions)
	binding.OptionSelections = CloneModelOptionSelections(binding.OptionSelections)
	return sortTransportBindings(append(bindings, binding))
}

// sortTransportBindings keeps persisted transport ordering deterministic.
func sortTransportBindings(bindings []ModelTransportBinding) []ModelTransportBinding {
	slices.SortFunc(bindings, func(left, right ModelTransportBinding) int {
		return cmp.Compare(
			strings.TrimSpace(left.TransportModelID),
			strings.TrimSpace(right.TransportModelID),
		)
	})
	return bindings
}

// PreferredTransportBinding uses the same stable route for catalog presentation and session launch.
// Provider defaults sort last; explicit configured transport IDs remain separate logical rows.
func PreferredTransportBinding(bindings []ModelTransportBinding) (ModelTransportBinding, bool) {
	var chosen ModelTransportBinding
	for _, binding := range bindings {
		id := strings.TrimSpace(binding.TransportModelID)
		if id == "" {
			continue
		}
		if chosen.TransportModelID == "" ||
			(chosen.TransportModelID == providerDefaultOption && id != providerDefaultOption) ||
			(id != providerDefaultOption && id < chosen.TransportModelID) {
			chosen = binding
		}
	}
	return chosen, chosen.TransportModelID != ""
}
