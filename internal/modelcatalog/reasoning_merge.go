package modelcatalog

import "slices"

func applyEffectiveReasoningProfile(model *Model, rows []ModelRow, opts MergeOptions) {
	profileRow, hasProfile := explicitReasoningProfileRow(rows)
	model.ReasoningEfforts = nil
	model.DefaultReasoningEffort = nil
	model.ReasoningSource = ReasoningSourceCatalog
	if hasProfile {
		model.SupportsReasoning = cloneBoolPtr(profileRow.SupportsReasoning)
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
	if !opts.canApplyReasoning(model.ProviderID) {
		model.ReasoningEfforts = nil
		model.DefaultReasoningEffort = nil
	}
}

func explicitReasoningProfileRow(rows []ModelRow) (ModelRow, bool) {
	for _, row := range rows {
		if row.SourceKind == SourceKindModelsDev {
			continue
		}
		if row.SupportsReasoning != nil || len(row.ReasoningEfforts) > 0 {
			return row, true
		}
	}
	return ModelRow{}, false
}

func explicitDefaultReasoningEffort(rows []ModelRow) *ReasoningEffort {
	for _, row := range rows {
		if row.SourceKind == SourceKindModelsDev || row.DefaultReasoningEffort == nil {
			continue
		}
		return row.DefaultReasoningEffort
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
