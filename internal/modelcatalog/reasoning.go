package modelcatalog

import (
	"maps"
	"strings"

	"github.com/compozy/agh/internal/reasoning"
)

// ReasoningEffort identifies one canonical model reasoning level.
type ReasoningEffort = reasoning.Effort

const (
	// ReasoningEffortNone disables model reasoning explicitly.
	ReasoningEffortNone = reasoning.EffortNone
	// ReasoningEffortMinimal is the smallest non-zero reasoning level.
	ReasoningEffortMinimal = reasoning.EffortMinimal
	// ReasoningEffortLow is the low reasoning level.
	ReasoningEffortLow = reasoning.EffortLow
	// ReasoningEffortMedium is the medium reasoning level.
	ReasoningEffortMedium = reasoning.EffortMedium
	// ReasoningEffortHigh is the high reasoning level.
	ReasoningEffortHigh = reasoning.EffortHigh
	// ReasoningEffortXHigh is the extra-high reasoning level.
	ReasoningEffortXHigh = reasoning.EffortXHigh
	// ReasoningEffortMax requests the provider's maximum supported reasoning level.
	ReasoningEffortMax = reasoning.EffortMax
)

// ReasoningSource identifies where a selectable reasoning profile came from.
type ReasoningSource string

const (
	// ReasoningSourceACP identifies an active ACP session observation.
	ReasoningSourceACP ReasoningSource = "acp"
	// ReasoningSourceCatalog identifies static or provider-discovery catalog data.
	ReasoningSourceCatalog ReasoningSource = "catalog"
)

// ReasoningProfile is one model's effective reasoning capability.
type ReasoningProfile struct {
	Supported bool
	Efforts   []ReasoningEffort
	Default   ReasoningEffort
	Source    ReasoningSource
}

// ReasoningEffortValues returns the canonical explicit effort vocabulary in display order.
func ReasoningEffortValues() []string {
	return reasoning.Values()
}

// ReasoningSourceValues returns the canonical reasoning provenance vocabulary.
func ReasoningSourceValues() []string {
	return []string{
		string(ReasoningSourceACP),
		string(ReasoningSourceCatalog),
	}
}

// IsValidEffort reports whether value is one canonical explicit effort.
// Empty is deliberately invalid here: it is the separate provider-default sentinel.
func IsValidEffort(value string) bool {
	return reasoning.IsValid(value)
}

// MergeOptions supplies provider capabilities needed to keep catalog claims truthful.
type MergeOptions struct {
	ReasoningApply map[string]bool
}

func (o MergeOptions) canApplyReasoning(providerID string) bool {
	return o.ReasoningApply[strings.TrimSpace(providerID)]
}

func cloneMergeOptions(options MergeOptions) MergeOptions {
	cloned := MergeOptions{ReasoningApply: make(map[string]bool, len(options.ReasoningApply))}
	maps.Copy(cloned.ReasoningApply, options.ReasoningApply)
	return cloned
}
