package modelcatalog

import (
	"slices"
	"strings"
)

// applyEffectiveReasoningProfile separates observed capability from provider permission to apply it.
func applyEffectiveReasoningProfile(model *Model, rows []ModelRow, opts MergeOptions) {
	profileRow, hasProfile := explicitReasoningProfileRow(rows)
	model.ReasoningEfforts = nil
	model.DefaultReasoningEffort = nil
	model.ReasoningSource = ReasoningSourceCatalog
	model.ReasoningKnown = hasProfile && (len(profileRow.ReasoningEfforts) > 0 || hasACPModelOptions(profileRow) ||
		(profileRow.SupportsReasoning != nil && !*profileRow.SupportsReasoning))
	model.ReasoningApply = ""
	canApply, policyKnown := opts.ReasoningApply[strings.TrimSpace(model.ProviderID)]
	if canApply {
		model.ReasoningApply = "acp_option"
	} else if policyKnown {
		model.ReasoningApply = "none"
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

// hasReasoningTransportBindings permits effort encoded in an advertised transport identity.
func hasReasoningTransportBindings(model *Model) bool {
	return len(model.ReasoningEfforts) > 0 && len(model.TransportBindings) > 0 &&
		slices.ContainsFunc(model.TransportBindings, func(binding ModelTransportBinding) bool {
			return binding.ReasoningEffort != nil && slices.Contains(model.ReasoningEfforts, *binding.ReasoningEffort)
		})
}

// explicitReasoningProfileRow selects authoritative reasoning metadata, excluding enrichment-only claims.
// A fresh live row whose option observation failed is authoritative too: its capability
// is unknown, and lower-priority seeds must not stand in as verified levels.
func explicitReasoningProfileRow(rows []ModelRow) (ModelRow, bool) {
	for _, row := range rows {
		if row.SourceKind == SourceKindModelsDev {
			continue
		}
		if row.SupportsReasoning != nil || len(row.ReasoningEfforts) > 0 || hasACPModelOptions(row) {
			return row, true
		}
		if unobservedLiveModel(row) {
			return row, true
		}
	}
	return ModelRow{}, false
}

// unobservedLiveModel reports a current live row that advertised the model but could not observe its options.
func unobservedLiveModel(row ModelRow) bool {
	return row.SourceKind == SourceKindProviderLive && !row.Stale && strings.TrimSpace(row.LastError) != ""
}

// explicitDefaultReasoningEffort stops at a complete ACP snapshot, including its provider-default choice.
func explicitDefaultReasoningEffort(rows []ModelRow) *ReasoningEffort {
	for _, row := range rows {
		if row.SourceKind == SourceKindModelsDev {
			continue
		}
		if row.DefaultReasoningEffort != nil || hasACPModelOptions(row) || unobservedLiveModel(row) {
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
	return new(*value)
}

func cloneEffortPtr(value *ReasoningEffort) *ReasoningEffort {
	if value == nil {
		return nil
	}
	return new(*value)
}

// cloneStringPtr prevents merged optional metadata from aliasing source-owned storage.
func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	return new(*value)
}

// hasACPModelOptions recognizes a complete selected-model observation, even when effort is absent.
func hasACPModelOptions(row ModelRow) bool {
	if row.SourceKind != SourceKindProviderLive {
		return false
	}
	if binding, ok := PreferredTransportBinding(row.TransportBindings); ok && binding.ConfigOptions != nil {
		return true
	}
	return slices.ContainsFunc(row.ConfigOptions, func(option ModelOptionDescriptor) bool {
		return option.ID == "model" || option.Category == "model"
	})
}
