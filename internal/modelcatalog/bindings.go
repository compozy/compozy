package modelcatalog

// ModelTransportBinding maps one logical model configuration to its provider-owned identifier.
type ModelTransportBinding struct {
	TransportModelID string
	Label            string
	ReasoningEffort  *ReasoningEffort
	Fast             *bool
	Thinking         *bool
	OptionSelections []ModelOptionSelection
}

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
		cloned[index].OptionSelections = CloneModelOptionSelections(binding.OptionSelections)
	}
	return cloned
}

func appendTransportBinding(bindings []ModelTransportBinding, binding ModelTransportBinding) []ModelTransportBinding {
	if binding.TransportModelID == "" {
		return bindings
	}
	for index := range bindings {
		if bindings[index].TransportModelID == binding.TransportModelID {
			mergeTransportBinding(&bindings[index], binding)
			return bindings
		}
	}
	binding.OptionSelections = CloneModelOptionSelections(binding.OptionSelections)
	return append(bindings, binding)
}
