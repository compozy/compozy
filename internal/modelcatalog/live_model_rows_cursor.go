package modelcatalog

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"
)

const cursorThinkingOptionID = "thinking"

type cursorModelVariant struct {
	transportModelID string
	label            string
	reasoningEffort  *ReasoningEffort
	fast             bool
	fastKnown        bool
	thinking         bool
	thinkingKnown    bool
}

type cursorTransportSuffixes struct {
	parts         []string
	effort        *ReasoningEffort
	fast          bool
	fastKnown     bool
	thinking      bool
	thinkingKnown bool
}

// parseCursorModelRows parses the account-scoped output of cursor-agent models.
func parseCursorModelRows(providerID string, output string, now time.Time) ([]ModelRow, error) {
	trimmedProviderID := strings.TrimSpace(providerID)
	groups := make(map[string][]cursorModelVariant)
	seenTransportIDs := make(map[string]struct{})
	for rawLine := range strings.SplitSeq(output, "\n") {
		variant, ok := parseCursorModelLine(rawLine)
		if !ok {
			continue
		}
		if _, exists := seenTransportIDs[variant.transportModelID]; exists {
			continue
		}
		seenTransportIDs[variant.transportModelID] = struct{}{}
		logicalID := cursorLogicalModelID(variant.transportModelID)
		if logicalID == "" {
			continue
		}
		groups[logicalID] = append(groups[logicalID], variant)
	}
	if len(groups) == 0 {
		return nil, fmt.Errorf("model catalog: cursor model command returned no model rows")
	}

	logicalIDs := slices.Sorted(maps.Keys(groups))
	rows := make([]ModelRow, 0, len(logicalIDs))
	for _, logicalID := range logicalIDs {
		rows = append(rows, cursorModelRow(trimmedProviderID, logicalID, groups[logicalID], now))
	}
	return rows, nil
}

func parseCursorModelLine(rawLine string) (cursorModelVariant, bool) {
	line := strings.TrimSpace(rawLine)
	if line == "" || strings.EqualFold(line, "available models") {
		return cursorModelVariant{}, false
	}
	if strings.HasPrefix(strings.ToLower(line), "tip:") {
		return cursorModelVariant{}, false
	}
	transportModelID, label, ok := strings.Cut(line, " - ")
	transportModelID = strings.TrimSpace(transportModelID)
	label = strings.TrimSpace(label)
	if !ok || transportModelID == "" || label == "" || strings.ContainsAny(transportModelID, " \t") {
		return cursorModelVariant{}, false
	}
	effort, fast, fastKnown, thinking, thinkingKnown := cursorModelDimensions(transportModelID)
	return cursorModelVariant{
		transportModelID: transportModelID,
		label:            label,
		reasoningEffort:  effort,
		fast:             fast,
		fastKnown:        fastKnown,
		thinking:         thinking,
		thinkingKnown:    thinkingKnown,
	}, true
}

func cursorLogicalModelID(transportModelID string) string {
	parsed := parseCursorTransportSuffixes(transportModelID)
	logicalID, _ := strings.CutPrefix(strings.Join(parsed.parts, "-"), "cursor-")
	return strings.TrimSpace(logicalID)
}

func cursorModelDimensions(
	transportModelID string,
) (*ReasoningEffort, bool, bool, bool, bool) {
	parsed := parseCursorTransportSuffixes(transportModelID)
	return parsed.effort, parsed.fast, parsed.fastKnown, parsed.thinking, parsed.thinkingKnown
}

func parseCursorTransportSuffixes(transportModelID string) cursorTransportSuffixes {
	trimmed := strings.TrimSpace(transportModelID)
	if trimmed == "" {
		return cursorTransportSuffixes{}
	}
	base := strings.TrimSuffix(trimmed, "-fast")
	fast := base != trimmed
	parts := strings.Split(base, "-")
	var effort *ReasoningEffort
	thinking := false
	thinkingKnown := false
	for {
		if len(parts) > 0 && strings.EqualFold(parts[len(parts)-1], cursorThinkingOptionID) {
			thinking = true
			thinkingKnown = true
			parts = parts[:len(parts)-1]
			continue
		}
		if len(parts) >= 2 &&
			strings.EqualFold(parts[len(parts)-2], "extra") &&
			strings.EqualFold(parts[len(parts)-1], "high") {
			parsed := ReasoningEffortXHigh
			effort = new(parsed)
			parts = parts[:len(parts)-2]
			continue
		}
		if len(parts) == 0 {
			break
		}
		parsed, ok := normalizeReasoningEffort(parts[len(parts)-1])
		if !ok {
			break
		}
		effort = new(parsed)
		parts = parts[:len(parts)-1]
	}
	return cursorTransportSuffixes{
		parts:         parts,
		effort:        effort,
		fast:          fast,
		fastKnown:     fast,
		thinking:      thinking,
		thinkingKnown: thinkingKnown,
	}
}

func cursorModelRow(
	providerID string,
	logicalID string,
	variants []cursorModelVariant,
	now time.Time,
) ModelRow {
	slices.SortFunc(variants, func(left cursorModelVariant, right cursorModelVariant) int {
		return cmp.Compare(left.transportModelID, right.transportModelID)
	})
	hasFast := false
	hasThinking := false
	efforts := make([]ReasoningEffort, 0, len(variants))
	seenEfforts := make(map[ReasoningEffort]struct{}, len(variants))
	for _, variant := range variants {
		hasFast = hasFast || variant.fastKnown
		hasThinking = hasThinking || variant.thinkingKnown
		if variant.reasoningEffort == nil {
			continue
		}
		if _, exists := seenEfforts[*variant.reasoningEffort]; exists {
			continue
		}
		seenEfforts[*variant.reasoningEffort] = struct{}{}
		efforts = append(efforts, *variant.reasoningEffort)
	}
	slices.SortFunc(efforts, cursorReasoningEffortOrder)

	bindings := cursorModelBindings(variants, hasFast, hasThinking)
	displayName := cursorLogicalDisplayName(logicalID, variants)
	defaultReasoningEffort := cursorDefaultReasoningEffort(displayName, variants)

	supportsReasoning := false
	for _, effort := range efforts {
		if effort != ReasoningEffortNone {
			supportsReasoning = true
			break
		}
	}
	if hasThinking {
		supportsReasoning = true
	}
	var supportsReasoningPtr *bool
	if len(efforts) > 0 || hasThinking {
		supportsReasoningPtr = new(supportsReasoning)
	}
	available := true
	return ModelRow{
		ProviderID:             providerID,
		ModelID:                logicalID,
		DisplayName:            displayName,
		SourceID:               SourceKindProviderLiveID(providerID),
		SourceKind:             SourceKindProviderLive,
		Priority:               PriorityProviderLive,
		Available:              &available,
		SupportsReasoning:      supportsReasoningPtr,
		ReasoningEfforts:       efforts,
		DefaultReasoningEffort: defaultReasoningEffort,
		ConfigOptions:          cursorModelConfigOptions(hasThinking),
		TransportBindings:      bindings,
		RefreshedAt:            now,
	}
}

func cursorModelBindings(
	variants []cursorModelVariant,
	hasFast bool,
	hasThinking bool,
) []ModelTransportBinding {
	bindings := make([]ModelTransportBinding, 0, len(variants))
	for _, variant := range variants {
		binding := ModelTransportBinding{
			TransportModelID: variant.transportModelID,
			Label:            variant.label,
			ReasoningEffort:  cloneModelRowPointer(variant.reasoningEffort),
		}
		if hasFast {
			binding.Fast = new(variant.fast)
		}
		if hasThinking {
			binding.Thinking = new(variant.thinking)
			binding.OptionSelections = []ModelOptionSelection{{
				ID:        cursorThinkingOptionID,
				BoolValue: new(variant.thinking),
			}}
		}
		bindings = append(bindings, binding)
	}
	return bindings
}

func cursorModelConfigOptions(hasThinking bool) []ModelOptionDescriptor {
	if !hasThinking {
		return nil
	}
	return []ModelOptionDescriptor{{
		ID:          cursorThinkingOptionID,
		Label:       "Thinking",
		Category:    "thought_level",
		Kind:        ModelOptionKindBoolean,
		CurrentBool: new(false),
	}}
}

func cursorDefaultReasoningEffort(
	displayName string,
	variants []cursorModelVariant,
) *ReasoningEffort {
	for _, variant := range variants {
		if variant.label != displayName || variant.fast || variant.thinking || variant.reasoningEffort == nil {
			continue
		}
		return cloneModelRowPointer(variant.reasoningEffort)
	}
	return nil
}

func cursorLogicalDisplayName(logicalID string, variants []cursorModelVariant) string {
	best := ""
	for _, variant := range variants {
		candidate := trimCursorDisplayVariants(variant.label)
		if candidate == "" || (best != "" && len(candidate) > len(best)) {
			continue
		}
		if best == "" || len(candidate) < len(best) || candidate < best {
			best = candidate
		}
	}
	if best != "" {
		return best
	}
	return logicalID
}

func trimCursorDisplayVariants(label string) string {
	parts := strings.Fields(strings.TrimSpace(label))
	for len(parts) > 0 {
		last := strings.ToLower(parts[len(parts)-1])
		switch last {
		case "fast", "thinking", "none", "minimal", "low", "medium", "max":
			parts = parts[:len(parts)-1]
		case "high":
			if len(parts) >= 2 && strings.EqualFold(parts[len(parts)-2], "extra") {
				parts = parts[:len(parts)-2]
				continue
			}
			parts = parts[:len(parts)-1]
		default:
			return strings.Join(parts, " ")
		}
	}
	return strings.Join(parts, " ")
}

func cursorReasoningEffortOrder(left ReasoningEffort, right ReasoningEffort) int {
	leftIndex := slices.Index(cursorReasoningEffortValues, left)
	rightIndex := slices.Index(cursorReasoningEffortValues, right)
	if leftIndex != rightIndex {
		return cmp.Compare(leftIndex, rightIndex)
	}
	return cmp.Compare(left, right)
}

var cursorReasoningEffortValues = []ReasoningEffort{
	ReasoningEffortNone,
	ReasoningEffortMinimal,
	ReasoningEffortLow,
	ReasoningEffortMedium,
	ReasoningEffortHigh,
	ReasoningEffortXHigh,
	ReasoningEffortMax,
}
