package modelcatalog

import "slices"

// applyEffectiveReasoningProfile separates observed capability from provider permission to apply it.
func applyEffectiveReasoningProfile(model *Model, rows []ModelRow, opts MergeOptions) {
	profileRow, hasProfile := explicitReasoningProfileRow(rows)
	model.ReasoningEfforts = nil
	model.DefaultReasoningEffort = nil
	model.ReasoningSource = ReasoningSourceCatalog
	model.ReasoningKnown = hasProfile && (len(profileRow.ReasoningEfforts) > 0 || hasACPModelOptions(profileRow) ||
		(profileRow.SupportsReasoning != nil && !*profileRow.SupportsReasoning))
	model.ReasoningApply = "none"
	if opts.canApplyReasoning(model.ProviderID) {
		model.ReasoningApply = "acp_option"
	} else {
		// Explicitly disabled negotiation is intentional provider management, not missing discovery.
		model.ReasoningKnown = true
	}
	if hasProfile {
		if profileRow.SupportsReasoning != nil {
			model.SupportsReasoning = cloneBoolPtr(profileRow.SupportsReasoning)
		}
		model.ReasoningEfforts = append([]ReasoningEffort(nil), profileRow.ReasoningEfforts...)
		if len(model.ReasoningEfforts) > 0 && model.SupportsReasoning == nil {
			value := true
			model.SupportsReasoning = &value
		}
		model.ReasoningSource = reasoningSourceForKind(profileRow.SourceKind)
	}
	if defaultEffort := explicitDefaultReasoningEffort(rows); defaultEffort != nil &&
		slices.Contains(model.ReasoningEfforts, *defaultEffort) {
		model.DefaultReasoningEffort = cloneEffortPtr(defaultEffort)
	}

	if model.SupportsReasoning != nil && !*model.SupportsReasoning {
		model.ReasoningEfforts = nil
		model.DefaultReasoningEffort = nil
	}
	if !opts.canApplyReasoning(model.ProviderID) && !hasReasoningTransportBindings(model) {
		model.ReasoningEfforts = nil
		model.DefaultReasoningEffort = nil
	}
}

func hasReasoningTransportBindings(model *Model) bool {
	return len(model.ReasoningEfforts) > 0 && len(model.TransportBindings) > 0 &&
		slices.ContainsFunc(model.TransportBindings, func(binding ModelTransportBinding) bool {
			return binding.ReasoningEffort != nil && slices.Contains(model.ReasoningEfforts, *binding.ReasoningEffort)
		})
}

// explicitReasoningProfileRow selects authoritative reasoning metadata, excluding enrichment-only claims.
func explicitReasoningProfileRow(rows []ModelRow) (ModelRow, bool) {
	for _, row := range rows {
		if row.SourceKind == SourceKindModelsDev {
			continue
		}
		if row.SupportsReasoning != nil || len(row.ReasoningEfforts) > 0 || hasACPModelOptions(row) {
			return row, true
		}
	}
	return ModelRow{}, false
}

// explicitDefaultReasoningEffort stops at a complete ACP snapshot, including its provider-default choice.
func explicitDefaultReasoningEffort(rows []ModelRow) *ReasoningEffort {
	for _, row := range rows {
		if row.SourceKind == SourceKindModelsDev {
			continue
		}
		if row.DefaultReasoningEffort != nil || hasACPModelOptions(row) {
			return row.DefaultReasoningEffort
		}
	}
	return nil
}

func reasoningSourceForKind(kind SourceKind) ReasoningSource {
	if kind == SourceKindACPSession {
		return ReasoningSourceACP
	}
	return ReasoningSourceCatalog
}

func cloneBoolPtr(value *bool) *bool {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneEffortPtr(value *ReasoningEffort) *ReasoningEffort {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

// hasACPModelOptions recognizes a complete selected-model observation, even when effort is absent.
func hasACPModelOptions(row ModelRow) bool {
	return row.SourceKind == SourceKindProviderLive &&
		slices.ContainsFunc(row.ConfigOptions, func(option ModelOptionDescriptor) bool {
			return option.ID == "model" || option.Category == "model"
		})
}
