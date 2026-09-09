package modelcatalog

import (
	"cmp"
	"maps"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/reasoning"
)

// ReasoningEffort identifies one provider-advertised model reasoning level.
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
	// ReasoningEffortUltra requests the provider's ultra reasoning level when advertised.
	ReasoningEffortUltra = reasoning.EffortUltra
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

// IsValidEffort reports whether value is a well-formed explicit effort identifier.
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

func compareReasoningEfforts(left ReasoningEffort, right ReasoningEffort) int {
	leftIndex := slices.Index(ReasoningEffortValues(), string(left))
	rightIndex := slices.Index(ReasoningEffortValues(), string(right))
	if leftIndex < 0 {
		leftIndex = len(ReasoningEffortValues())
	}
	if rightIndex < 0 {
		rightIndex = len(ReasoningEffortValues())
	}
	if leftIndex != rightIndex {
		return cmp.Compare(leftIndex, rightIndex)
	}
	return cmp.Compare(left, right)
}
