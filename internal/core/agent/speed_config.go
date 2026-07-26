package agent

import (
	"strings"
	"unicode"

	acp "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

type speedConfigShape uint8

const (
	speedConfigShapeBoolean speedConfigShape = iota + 1
	speedConfigShapeSelect
	speedConfigCategoryModelConfig = acp.SessionConfigOptionCategory("model_config")
)

type speedConfigMatch struct {
	optionID       acp.SessionConfigId
	shape          speedConfigShape
	booleanTarget  bool
	selectTarget   acp.SessionConfigValueId
	currentBoolean bool
	currentSelect  acp.SessionConfigValueId
}

type speedConfigMatchResult struct {
	match  *speedConfigMatch
	reason kinds.SpeedResolutionReason
}

type speedValueSemantic uint8

const (
	speedValueSemanticNone speedValueSemantic = iota
	speedValueSemanticNormal
	speedValueSemanticFast
)

func matchSpeedConfig(requested kinds.Speed, options []acp.SessionConfigOption) speedConfigMatchResult {
	if requested != kinds.SpeedNormal && requested != kinds.SpeedFast {
		return unsupportedSpeedConfigMatch(kinds.SpeedResolutionReasonValueAmbiguous)
	}

	candidates := make([]acp.SessionConfigOption, 0, 1)
	for _, option := range options {
		if isSpeedConfigCandidate(option) {
			candidates = append(candidates, option)
		}
	}

	switch len(candidates) {
	case 0:
		return unsupportedSpeedConfigMatch(kinds.SpeedResolutionReasonCapabilityAbsent)
	case 1:
	default:
		return unsupportedSpeedConfigMatch(kinds.SpeedResolutionReasonCapabilityAmbiguous)
	}

	candidate := candidates[0]
	if countSpeedConfigVariants(candidate) != 1 {
		return unsupportedSpeedConfigMatch(kinds.SpeedResolutionReasonValueAmbiguous)
	}
	var result speedConfigMatchResult
	if candidate.Boolean != nil {
		result = matchBooleanSpeedConfig(requested, *candidate.Boolean)
	} else {
		result = matchSelectSpeedConfig(requested, *candidate.Select)
	}
	if result.match != nil && countSpeedConfigOptionID(options, result.match.optionID) != 1 {
		return unsupportedSpeedConfigMatch(kinds.SpeedResolutionReasonCapabilityAmbiguous)
	}
	return result
}

func matchV1SpeedConfig(requested kinds.Speed, options []acp.SessionConfigOption) speedConfigMatchResult {
	result := matchSpeedConfig(requested, options)
	if result.match != nil && result.match.shape == speedConfigShapeBoolean {
		return unsupportedSpeedConfigMatch(kinds.SpeedResolutionReasonCapabilityAbsent)
	}
	return result
}

func unsupportedSpeedConfigMatch(reason kinds.SpeedResolutionReason) speedConfigMatchResult {
	return speedConfigMatchResult{reason: reason}
}

func isSpeedConfigCandidate(option acp.SessionConfigOption) bool {
	if option.Boolean != nil && isSpeedConfigOption(
		option.Boolean.Category,
		option.Boolean.Id,
		option.Boolean.Name,
	) {
		return true
	}
	return option.Select != nil && isSpeedConfigOption(
		option.Select.Category,
		option.Select.Id,
		option.Select.Name,
	)
}

func isSpeedConfigOption(
	category *acp.SessionConfigOptionCategory,
	id acp.SessionConfigId,
	name string,
) bool {
	if category == nil ||
		normalizeSpeedToken(string(*category)) != normalizeSpeedToken(string(speedConfigCategoryModelConfig)) {
		return false
	}
	return isSpeedOptionToken(normalizeSpeedToken(string(id))) ||
		isSpeedOptionToken(normalizeSpeedToken(name))
}

func isSpeedOptionToken(token string) bool {
	switch token {
	case "speed", "fast", "fastmode":
		return true
	default:
		return false
	}
}

func countSpeedConfigVariants(option acp.SessionConfigOption) int {
	count := 0
	if option.Boolean != nil {
		count++
	}
	if option.Select != nil {
		count++
	}
	return count
}

func countSpeedConfigOptionID(
	options []acp.SessionConfigOption,
	id acp.SessionConfigId,
) int {
	count := 0
	for _, option := range options {
		if option.Boolean != nil && option.Boolean.Id == id {
			count++
		}
		if option.Select != nil && option.Select.Id == id {
			count++
		}
	}
	return count
}

func matchBooleanSpeedConfig(
	requested kinds.Speed,
	option acp.SessionConfigOptionBoolean,
) speedConfigMatchResult {
	if option.Id == "" {
		return unsupportedSpeedConfigMatch(kinds.SpeedResolutionReasonValueAmbiguous)
	}
	target := requested == kinds.SpeedFast
	return speedConfigMatchResult{
		match: &speedConfigMatch{
			optionID:       option.Id,
			shape:          speedConfigShapeBoolean,
			booleanTarget:  target,
			currentBoolean: option.CurrentValue,
		},
	}
}

func matchSelectSpeedConfig(
	requested kinds.Speed,
	option acp.SessionConfigOptionSelect,
) speedConfigMatchResult {
	if option.Id == "" {
		return unsupportedSpeedConfigMatch(kinds.SpeedResolutionReasonValueAmbiguous)
	}
	values, ok := flattenSpeedConfigValues(option.Options)
	if !ok {
		return unsupportedSpeedConfigMatch(kinds.SpeedResolutionReasonValueAmbiguous)
	}

	normalValues := make([]acp.SessionConfigSelectOption, 0, 1)
	fastValues := make([]acp.SessionConfigSelectOption, 0, 1)
	valueIDCounts := make(map[acp.SessionConfigValueId]int, len(values))
	for _, value := range values {
		valueIDCounts[value.Value]++
		semantic, unambiguous := classifySpeedConfigValue(value)
		if !unambiguous {
			return unsupportedSpeedConfigMatch(kinds.SpeedResolutionReasonValueAmbiguous)
		}
		switch semantic {
		case speedValueSemanticNormal:
			normalValues = append(normalValues, value)
		case speedValueSemanticFast:
			fastValues = append(fastValues, value)
		case speedValueSemanticNone:
		}
	}
	if len(normalValues) != 1 || len(fastValues) != 1 {
		return unsupportedSpeedConfigMatch(kinds.SpeedResolutionReasonValueAmbiguous)
	}
	if normalValues[0].Value == "" ||
		fastValues[0].Value == "" ||
		valueIDCounts[normalValues[0].Value] != 1 ||
		valueIDCounts[fastValues[0].Value] != 1 {
		return unsupportedSpeedConfigMatch(kinds.SpeedResolutionReasonValueAmbiguous)
	}

	target := normalValues[0].Value
	if requested == kinds.SpeedFast {
		target = fastValues[0].Value
	}
	return speedConfigMatchResult{
		match: &speedConfigMatch{
			optionID:      option.Id,
			shape:         speedConfigShapeSelect,
			selectTarget:  target,
			currentSelect: option.CurrentValue,
		},
	}
}

func flattenSpeedConfigValues(options acp.SessionConfigSelectOptions) (
	[]acp.SessionConfigSelectOption,
	bool,
) {
	if (options.Ungrouped == nil) == (options.Grouped == nil) {
		return nil, false
	}
	if options.Ungrouped != nil {
		return append([]acp.SessionConfigSelectOption(nil), (*options.Ungrouped)...), true
	}

	var values []acp.SessionConfigSelectOption
	for _, group := range *options.Grouped {
		values = append(values, group.Options...)
	}
	return values, true
}

func classifySpeedConfigValue(value acp.SessionConfigSelectOption) (speedValueSemantic, bool) {
	idSemantic := speedConfigValueSemantic(normalizeSpeedToken(string(value.Value)))
	nameSemantic := speedConfigValueSemantic(normalizeSpeedToken(value.Name))
	if idSemantic != speedValueSemanticNone &&
		nameSemantic != speedValueSemanticNone &&
		idSemantic != nameSemantic {
		return speedValueSemanticNone, false
	}
	if idSemantic != speedValueSemanticNone {
		return idSemantic, true
	}
	return nameSemantic, true
}

func speedConfigValueSemantic(token string) speedValueSemantic {
	switch token {
	case "normal", "standard", "off", "disabled", "false":
		return speedValueSemanticNormal
	case "fast", "on", "enabled", "true":
		return speedValueSemanticFast
	default:
		return speedValueSemanticNone
	}
}

func normalizeSpeedToken(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '_' || r == '-' {
			return -1
		}
		return unicode.ToLower(r)
	}, strings.TrimSpace(value))
}

func (match speedConfigMatch) alreadyApplied() bool {
	switch match.shape {
	case speedConfigShapeBoolean:
		return match.currentBoolean == match.booleanTarget
	case speedConfigShapeSelect:
		return match.currentSelect == match.selectTarget
	default:
		return false
	}
}

func (match speedConfigMatch) setRequest(
	sessionID acp.SessionId,
) (acp.SetSessionConfigOptionRequest, bool) {
	if match.alreadyApplied() || match.optionID == "" {
		return acp.SetSessionConfigOptionRequest{}, false
	}
	switch match.shape {
	case speedConfigShapeBoolean:
		return acp.SetSessionConfigOptionRequest{
			Boolean: &acp.SetSessionConfigOptionBoolean{
				SessionId: sessionID,
				ConfigId:  match.optionID,
				Type:      "boolean",
				Value:     match.booleanTarget,
			},
		}, true
	case speedConfigShapeSelect:
		if match.selectTarget == "" {
			return acp.SetSessionConfigOptionRequest{}, false
		}
		return acp.SetSessionConfigOptionRequest{
			ValueId: &acp.SetSessionConfigOptionValueId{
				SessionId: sessionID,
				ConfigId:  match.optionID,
				Value:     match.selectTarget,
			},
		}, true
	default:
		return acp.SetSessionConfigOptionRequest{}, false
	}
}

func confirmSpeedConfig(match speedConfigMatch, options []acp.SessionConfigOption) bool {
	var matchedOption *acp.SessionConfigOption
	for index := range options {
		option := &options[index]
		if !speedConfigOptionHasID(*option, match.optionID) {
			continue
		}
		if matchedOption != nil {
			return false
		}
		matchedOption = option
	}
	if matchedOption == nil || countSpeedConfigVariants(*matchedOption) != 1 {
		return false
	}

	switch match.shape {
	case speedConfigShapeBoolean:
		return matchedOption.Boolean != nil &&
			matchedOption.Boolean.Id == match.optionID &&
			matchedOption.Boolean.CurrentValue == match.booleanTarget
	case speedConfigShapeSelect:
		return matchedOption.Select != nil &&
			matchedOption.Select.Id == match.optionID &&
			matchedOption.Select.CurrentValue == match.selectTarget
	default:
		return false
	}
}

func speedConfigOptionHasID(option acp.SessionConfigOption, id acp.SessionConfigId) bool {
	return option.Boolean != nil && option.Boolean.Id == id ||
		option.Select != nil && option.Select.Id == id
}
