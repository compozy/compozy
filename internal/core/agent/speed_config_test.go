package agent

import (
	"reflect"
	"testing"

	acp "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestNormalizeSpeedToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "case", value: "SpEeD", want: "speed"},
		{name: "surrounding whitespace", value: "\t model_config \n", want: "modelconfig"},
		{name: "space", value: "fast mode", want: "fastmode"},
		{name: "underscore", value: "fast_mode", want: "fastmode"},
		{name: "hyphen", value: "fast-mode", want: "fastmode"},
		{name: "combined separators", value: " FAST_ - MODE ", want: "fastmode"},
		{name: "near match remains closed", value: "fast.mode", want: "fast.mode"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := normalizeSpeedToken(test.value); got != test.want {
				t.Fatalf("normalizeSpeedToken(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

func TestMatchSpeedConfigBoolean(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		requested      kinds.Speed
		optionID       string
		optionName     string
		category       string
		current        bool
		wantTarget     bool
		alreadyApplied bool
	}{
		{
			name:       "fast targets true through option ID",
			requested:  kinds.SpeedFast,
			optionID:   "speed",
			optionName: "Performance",
			category:   "model_config",
			current:    false,
			wantTarget: true,
		},
		{
			name:       "normal targets false through option name",
			requested:  kinds.SpeedNormal,
			optionID:   "provider-toggle",
			optionName: "Fast Mode",
			category:   "model_config",
			current:    true,
			wantTarget: false,
		},
		{
			name:           "fast already applied skips write",
			requested:      kinds.SpeedFast,
			optionID:       "provider-toggle",
			optionName:     " FAST_MODE ",
			category:       " MODEL - CONFIG ",
			current:        true,
			wantTarget:     true,
			alreadyApplied: true,
		},
		{
			name:           "normal already applied skips write",
			requested:      kinds.SpeedNormal,
			optionID:       "FAST-MODE",
			optionName:     "Provider toggle",
			category:       "model config",
			current:        false,
			wantTarget:     false,
			alreadyApplied: true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			option := completeBooleanConfigOption(
				test.optionID,
				test.optionName,
				speedTestCategory(test.category),
				test.current,
			)

			result := matchSpeedConfig(test.requested, []acp.SessionConfigOption{option})

			assertSupportedSpeedMatch(t, result, speedConfigShapeBoolean, acp.SessionConfigId(test.optionID))
			if result.match.booleanTarget != test.wantTarget {
				t.Fatalf("boolean target = %t, want %t", result.match.booleanTarget, test.wantTarget)
			}
			if result.match.currentBoolean != test.current {
				t.Fatalf("current boolean = %t, want %t", result.match.currentBoolean, test.current)
			}
			if got := result.match.alreadyApplied(); got != test.alreadyApplied {
				t.Fatalf("alreadyApplied() = %t, want %t", got, test.alreadyApplied)
			}
		})
	}
}

func TestMatchSpeedConfigSelectVocabulary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		normalSignal string
		fastSignal   string
		requested    kinds.Speed
		wantTarget   acp.SessionConfigValueId
	}{
		{
			name:         "normal and fast",
			normalSignal: "normal",
			fastSignal:   "fast",
			requested:    kinds.SpeedNormal,
			wantTarget:   "normal",
		},
		{
			name:         "standard and on",
			normalSignal: "standard",
			fastSignal:   "on",
			requested:    kinds.SpeedFast,
			wantTarget:   "on",
		},
		{
			name:         "off and enabled",
			normalSignal: "off",
			fastSignal:   "enabled",
			requested:    kinds.SpeedNormal,
			wantTarget:   "off",
		},
		{
			name:         "disabled and true",
			normalSignal: "disabled",
			fastSignal:   "true",
			requested:    kinds.SpeedFast,
			wantTarget:   "true",
		},
		{
			name:         "false and normalized enabled",
			normalSignal: " FALSE ",
			fastSignal:   " EN-ABLED ",
			requested:    kinds.SpeedFast,
			wantTarget:   " EN-ABLED ",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			option := completeFlatSelectConfigOption(
				"speed-selector",
				"Speed",
				speedTestCategory("model_config"),
				"provider-current",
				completeSelectValue(test.normalSignal, "Provider normal"),
				completeSelectValue(test.fastSignal, "Provider fast"),
				completeSelectValue("other", "Other"),
			)

			result := matchSpeedConfig(test.requested, []acp.SessionConfigOption{option})

			assertSupportedSpeedMatch(t, result, speedConfigShapeSelect, "speed-selector")
			if result.match.selectTarget != test.wantTarget {
				t.Fatalf("select target = %q, want %q", result.match.selectTarget, test.wantTarget)
			}
			if result.match.currentSelect != "provider-current" {
				t.Fatalf("current select = %q, want provider-current", result.match.currentSelect)
			}
		})
	}
}

func TestMatchSpeedConfigGroupedSelectUsesIDsAndNames(t *testing.T) {
	t.Parallel()

	option := completeGroupedSelectConfigOption(
		"provider-speed",
		" FAST MODE ",
		speedTestCategory(" MODEL_CONFIG "),
		"standard-tier",
		[]acp.SessionConfigSelectGroup{
			completeSelectGroup(
				"cost",
				"Cost",
				completeSelectValue("standard-tier", "STANDARD"),
			),
			completeSelectGroup(
				"latency",
				"Latency",
				completeSelectValue("accelerated-tier", "ON"),
				completeSelectValue("balanced-tier", "Balanced"),
			),
		},
	)

	normalResult := matchSpeedConfig(kinds.SpeedNormal, []acp.SessionConfigOption{option})
	assertSupportedSpeedMatch(t, normalResult, speedConfigShapeSelect, "provider-speed")
	if normalResult.match.selectTarget != "standard-tier" || !normalResult.match.alreadyApplied() {
		t.Fatalf("normal match = %#v, want already-applied standard-tier", *normalResult.match)
	}

	fastResult := matchSpeedConfig(kinds.SpeedFast, []acp.SessionConfigOption{option})
	assertSupportedSpeedMatch(t, fastResult, speedConfigShapeSelect, "provider-speed")
	if fastResult.match.selectTarget != "accelerated-tier" || fastResult.match.alreadyApplied() {
		t.Fatalf("fast match = %#v, want pending accelerated-tier", *fastResult.match)
	}
}

func TestMatchSpeedConfigRejectsUnsupportedShapes(t *testing.T) {
	t.Parallel()

	modelCategory := speedTestCategory("model_config")
	modeCategory := speedTestCategory("mode")
	descriptionOnly := completeBooleanConfigOption("quality", "Quality", modelCategory, false)
	descriptionOnly.Boolean.Description = speedTestPtr("Controls fast mode speed")
	descriptionOnly.Boolean.Meta = map[string]any{
		"provider": "fast",
		"model":    "speed",
	}

	validBoolean := completeBooleanConfigOption("speed", "Speed", modelCategory, false)
	validSelect := completeFlatSelectConfigOption(
		"fast-mode",
		"Fast Mode",
		modelCategory,
		"normal",
		completeSelectValue("normal", "Normal"),
		completeSelectValue("fast", "Fast"),
	)
	missingCategory := completeBooleanConfigOption("speed", "Speed", nil, false)
	wrongCategory := completeBooleanConfigOption("speed", "Speed", modeCategory, false)
	nearMatch := completeBooleanConfigOption("speedy", "Faster", modelCategory, false)
	emptyID := completeBooleanConfigOption("", "Speed", modelCategory, false)
	duplicateOptionID := completeBooleanConfigOption("speed", "Quality", modeCategory, false)

	bothTopLevelVariants := validBoolean
	bothTopLevelVariants.Select = validSelect.Select

	noSelectVariants := validSelect
	noSelectVariants.Select = speedTestCopySelect(validSelect.Select)
	noSelectVariants.Select.Options = acp.SessionConfigSelectOptions{}
	bothSelectVariants := validSelect
	bothSelectVariants.Select = speedTestCopySelect(validSelect.Select)
	flat := *validSelect.Select.Options.Ungrouped
	groups := acp.SessionConfigSelectOptionsGrouped{
		completeSelectGroup("all", "All", flat...),
	}
	bothSelectVariants.Select.Options.Grouped = &groups

	duplicateNormal := completeFlatSelectConfigOption(
		"speed",
		"Speed",
		modelCategory,
		"normal",
		completeSelectValue("normal", "Normal"),
		completeSelectValue("standard", "Standard"),
		completeSelectValue("fast", "Fast"),
	)
	duplicateFast := completeFlatSelectConfigOption(
		"speed",
		"Speed",
		modelCategory,
		"normal",
		completeSelectValue("normal", "Normal"),
		completeSelectValue("fast", "Fast"),
		completeSelectValue("enabled", "Enabled"),
	)
	conflictingSignals := completeFlatSelectConfigOption(
		"speed",
		"Speed",
		modelCategory,
		"normal",
		completeSelectValue("normal", "Fast"),
		completeSelectValue("fast", "Fast"),
	)
	missingNormal := completeFlatSelectConfigOption(
		"speed",
		"Speed",
		modelCategory,
		"other",
		completeSelectValue("economy", "Economy"),
		completeSelectValue("fast", "Fast"),
	)
	missingFast := completeFlatSelectConfigOption(
		"speed",
		"Speed",
		modelCategory,
		"normal",
		completeSelectValue("normal", "Normal"),
		completeSelectValue("accelerated", "Accelerated"),
	)
	sharedValueID := completeFlatSelectConfigOption(
		"speed",
		"Speed",
		modelCategory,
		"shared",
		completeSelectValue("shared", "Normal"),
		completeSelectValue("shared", "Fast"),
	)
	emptyFastValueID := completeFlatSelectConfigOption(
		"speed",
		"Speed",
		modelCategory,
		"normal",
		completeSelectValue("normal", "Normal"),
		completeSelectValue("", "Fast"),
	)
	duplicateTargetValueID := completeFlatSelectConfigOption(
		"speed",
		"Speed",
		modelCategory,
		"normal-tier",
		completeSelectValue("normal-tier", "Normal"),
		completeSelectValue("fast-tier", "Fast"),
		completeSelectValue("normal-tier", "Legacy"),
	)

	tests := []struct {
		name      string
		requested kinds.Speed
		options   []acp.SessionConfigOption
		want      kinds.SpeedResolutionReason
	}{
		{
			name:      "missing category",
			requested: kinds.SpeedFast,
			options:   []acp.SessionConfigOption{missingCategory},
			want:      kinds.SpeedResolutionReasonCapabilityAbsent,
		},
		{
			name:      "wrong category",
			requested: kinds.SpeedFast,
			options:   []acp.SessionConfigOption{wrongCategory},
			want:      kinds.SpeedResolutionReasonCapabilityAbsent,
		},
		{
			name:      "absent candidate",
			requested: kinds.SpeedFast,
			options:   []acp.SessionConfigOption{descriptionOnly},
			want:      kinds.SpeedResolutionReasonCapabilityAbsent,
		},
		{
			name:      "near match remains absent",
			requested: kinds.SpeedFast,
			options:   []acp.SessionConfigOption{nearMatch},
			want:      kinds.SpeedResolutionReasonCapabilityAbsent,
		},
		{
			name:      "empty option union remains absent",
			requested: kinds.SpeedFast,
			options:   []acp.SessionConfigOption{{}},
			want:      kinds.SpeedResolutionReasonCapabilityAbsent,
		},
		{
			name:      "multiple compatible options",
			requested: kinds.SpeedFast,
			options:   []acp.SessionConfigOption{validBoolean, validSelect},
			want:      kinds.SpeedResolutionReasonCapabilityAmbiguous,
		},
		{
			name:      "valid and malformed candidates",
			requested: kinds.SpeedFast,
			options:   []acp.SessionConfigOption{validBoolean, bothTopLevelVariants},
			want:      kinds.SpeedResolutionReasonCapabilityAmbiguous,
		},
		{
			name:      "duplicate option ID",
			requested: kinds.SpeedFast,
			options:   []acp.SessionConfigOption{validBoolean, duplicateOptionID},
			want:      kinds.SpeedResolutionReasonCapabilityAmbiguous,
		},
		{
			name:      "malformed top-level union",
			requested: kinds.SpeedFast,
			options:   []acp.SessionConfigOption{bothTopLevelVariants},
			want:      kinds.SpeedResolutionReasonValueAmbiguous,
		},
		{
			name:      "select option union missing",
			requested: kinds.SpeedFast,
			options:   []acp.SessionConfigOption{noSelectVariants},
			want:      kinds.SpeedResolutionReasonValueAmbiguous,
		},
		{
			name:      "select option union duplicated",
			requested: kinds.SpeedFast,
			options:   []acp.SessionConfigOption{bothSelectVariants},
			want:      kinds.SpeedResolutionReasonValueAmbiguous,
		},
		{
			name:      "duplicate normal semantics",
			requested: kinds.SpeedNormal,
			options:   []acp.SessionConfigOption{duplicateNormal},
			want:      kinds.SpeedResolutionReasonValueAmbiguous,
		},
		{
			name:      "duplicate fast semantics",
			requested: kinds.SpeedFast,
			options:   []acp.SessionConfigOption{duplicateFast},
			want:      kinds.SpeedResolutionReasonValueAmbiguous,
		},
		{
			name:      "conflicting ID and name semantics",
			requested: kinds.SpeedFast,
			options:   []acp.SessionConfigOption{conflictingSignals},
			want:      kinds.SpeedResolutionReasonValueAmbiguous,
		},
		{
			name:      "missing normal value",
			requested: kinds.SpeedNormal,
			options:   []acp.SessionConfigOption{missingNormal},
			want:      kinds.SpeedResolutionReasonValueAmbiguous,
		},
		{
			name:      "missing fast value",
			requested: kinds.SpeedFast,
			options:   []acp.SessionConfigOption{missingFast},
			want:      kinds.SpeedResolutionReasonValueAmbiguous,
		},
		{
			name:      "normal and fast share one value ID",
			requested: kinds.SpeedFast,
			options:   []acp.SessionConfigOption{sharedValueID},
			want:      kinds.SpeedResolutionReasonValueAmbiguous,
		},
		{
			name:      "semantic value has empty ID",
			requested: kinds.SpeedFast,
			options:   []acp.SessionConfigOption{emptyFastValueID},
			want:      kinds.SpeedResolutionReasonValueAmbiguous,
		},
		{
			name:      "target value ID is duplicated by a neutral value",
			requested: kinds.SpeedNormal,
			options:   []acp.SessionConfigOption{duplicateTargetValueID},
			want:      kinds.SpeedResolutionReasonValueAmbiguous,
		},
		{
			name:      "compatible option has empty ID",
			requested: kinds.SpeedFast,
			options:   []acp.SessionConfigOption{emptyID},
			want:      kinds.SpeedResolutionReasonValueAmbiguous,
		},
		{
			name:      "invalid canonical preference",
			requested: kinds.Speed("turbo"),
			options:   []acp.SessionConfigOption{validBoolean},
			want:      kinds.SpeedResolutionReasonValueAmbiguous,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := matchSpeedConfig(test.requested, test.options)
			if result.match != nil {
				t.Fatalf("unsupported result constructed write target: %#v", *result.match)
			}
			if result.reason != test.want {
				t.Fatalf("reason = %q, want %q", result.reason, test.want)
			}
		})
	}
}

func TestSpeedConfigMatchBuildsExactRequestOrSkips(t *testing.T) {
	t.Parallel()

	sessionID := acp.SessionId("session-123")
	booleanOption := completeBooleanConfigOption(
		"provider-fast",
		"Fast Mode",
		speedTestCategory("model_config"),
		false,
	)
	selectOption := completeFlatSelectConfigOption(
		"provider-speed",
		"Speed",
		speedTestCategory("model_config"),
		"standard-tier",
		completeSelectValue("standard-tier", "Standard"),
		completeSelectValue("accelerated-tier", "Enabled"),
	)

	tests := []struct {
		name      string
		requested kinds.Speed
		option    acp.SessionConfigOption
		wantShape speedConfigShape
		wantID    acp.SessionConfigId
		wantWrite bool
		want      acp.SetSessionConfigOptionRequest
	}{
		{
			name:      "boolean fast request",
			requested: kinds.SpeedFast,
			option:    booleanOption,
			wantShape: speedConfigShapeBoolean,
			wantID:    "provider-fast",
			wantWrite: true,
			want: acp.SetSessionConfigOptionRequest{
				Boolean: &acp.SetSessionConfigOptionBoolean{
					SessionId: sessionID,
					ConfigId:  "provider-fast",
					Type:      "boolean",
					Value:     true,
				},
			},
		},
		{
			name:      "boolean normal already applied",
			requested: kinds.SpeedNormal,
			option:    booleanOption,
			wantShape: speedConfigShapeBoolean,
			wantID:    "provider-fast",
			wantWrite: false,
		},
		{
			name:      "select fast request uses exact advertised ID",
			requested: kinds.SpeedFast,
			option:    selectOption,
			wantShape: speedConfigShapeSelect,
			wantID:    "provider-speed",
			wantWrite: true,
			want: acp.SetSessionConfigOptionRequest{
				ValueId: &acp.SetSessionConfigOptionValueId{
					SessionId: sessionID,
					ConfigId:  "provider-speed",
					Value:     "accelerated-tier",
				},
			},
		},
		{
			name:      "select normal already applied",
			requested: kinds.SpeedNormal,
			option:    selectOption,
			wantShape: speedConfigShapeSelect,
			wantID:    "provider-speed",
			wantWrite: false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := matchSpeedConfig(test.requested, []acp.SessionConfigOption{test.option})
			assertSupportedSpeedMatch(
				t,
				result,
				test.wantShape,
				test.wantID,
			)

			got, gotWrite := result.match.setRequest(sessionID)
			if gotWrite != test.wantWrite {
				t.Fatalf("setRequest() write = %t, want %t", gotWrite, test.wantWrite)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("setRequest() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestConfirmSpeedConfigBoolean(t *testing.T) {
	t.Parallel()

	category := speedTestCategory("model_config")
	original := completeBooleanConfigOption("provider-fast", "Fast Mode", category, false)
	result := matchSpeedConfig(kinds.SpeedFast, []acp.SessionConfigOption{original})
	assertSupportedSpeedMatch(t, result, speedConfigShapeBoolean, "provider-fast")

	confirmed := completeBooleanConfigOption("provider-fast", "Fast Mode", category, true)
	otherID := completeBooleanConfigOption("other-fast", "Fast Mode", category, true)
	opposite := completeBooleanConfigOption("provider-fast", "Fast Mode", category, false)
	changedShape := completeFlatSelectConfigOption(
		"provider-fast",
		"Fast Mode",
		category,
		"fast",
		completeSelectValue("normal", "Normal"),
		completeSelectValue("fast", "Fast"),
	)
	malformed := confirmed
	malformed.Select = changedShape.Select

	tests := []struct {
		name    string
		options []acp.SessionConfigOption
		want    bool
	}{
		{name: "confirmed exact original option", options: []acp.SessionConfigOption{otherID, confirmed}, want: true},
		{name: "missing original option", options: []acp.SessionConfigOption{otherID}, want: false},
		{name: "duplicate original option", options: []acp.SessionConfigOption{confirmed, confirmed}, want: false},
		{name: "changed shape", options: []acp.SessionConfigOption{changedShape}, want: false},
		{name: "unchanged or opposite state", options: []acp.SessionConfigOption{opposite}, want: false},
		{name: "malformed returned union", options: []acp.SessionConfigOption{malformed}, want: false},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := confirmSpeedConfig(*result.match, test.options); got != test.want {
				t.Fatalf("confirmSpeedConfig() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestConfirmSpeedConfigSelect(t *testing.T) {
	t.Parallel()

	category := speedTestCategory("model_config")
	original := completeGroupedSelectConfigOption(
		"provider-speed",
		"Speed",
		category,
		"normal-tier",
		[]acp.SessionConfigSelectGroup{
			completeSelectGroup(
				"tiers",
				"Tiers",
				completeSelectValue("normal-tier", "Normal"),
				completeSelectValue("fast-tier", "Fast"),
			),
		},
	)
	result := matchSpeedConfig(kinds.SpeedFast, []acp.SessionConfigOption{original})
	assertSupportedSpeedMatch(t, result, speedConfigShapeSelect, "provider-speed")

	confirmed := original
	confirmed.Select = speedTestCopySelect(original.Select)
	confirmed.Select.CurrentValue = "fast-tier"
	unchanged := original
	unchanged.Select = speedTestCopySelect(original.Select)
	otherID := original
	otherID.Select = speedTestCopySelect(original.Select)
	otherID.Select.Id = "other-speed"
	otherID.Select.CurrentValue = "fast-tier"

	tests := []struct {
		name    string
		options []acp.SessionConfigOption
		want    bool
	}{
		{name: "confirmed exact target value ID", options: []acp.SessionConfigOption{confirmed}, want: true},
		{name: "unchanged value", options: []acp.SessionConfigOption{unchanged}, want: false},
		{name: "target value on different option", options: []acp.SessionConfigOption{otherID}, want: false},
		{name: "missing option", options: nil, want: false},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := confirmSpeedConfig(*result.match, test.options); got != test.want {
				t.Fatalf("confirmSpeedConfig() = %t, want %t", got, test.want)
			}
		})
	}
}

func assertSupportedSpeedMatch(
	t *testing.T,
	result speedConfigMatchResult,
	wantShape speedConfigShape,
	wantOptionID acp.SessionConfigId,
) {
	t.Helper()
	if result.match == nil {
		t.Fatalf("match is nil with reason %q", result.reason)
	}
	if result.reason != "" {
		t.Fatalf("supported match has reason %q", result.reason)
	}
	if result.match.shape != wantShape {
		t.Fatalf("shape = %d, want %d", result.match.shape, wantShape)
	}
	if result.match.optionID != wantOptionID {
		t.Fatalf("option ID = %q, want %q", result.match.optionID, wantOptionID)
	}
}

func completeBooleanConfigOption(
	id string,
	name string,
	category *acp.SessionConfigOptionCategory,
	current bool,
) acp.SessionConfigOption {
	return acp.SessionConfigOption{
		Boolean: &acp.SessionConfigOptionBoolean{
			Meta: map[string]any{
				"provider": "ignored-provider",
				"model":    "ignored-model",
			},
			Category:     category,
			CurrentValue: current,
			Description:  speedTestPtr("Complete boolean fixture description"),
			Id:           acp.SessionConfigId(id),
			Name:         name,
			Type:         "boolean",
		},
	}
}

func completeFlatSelectConfigOption(
	id string,
	name string,
	category *acp.SessionConfigOptionCategory,
	current acp.SessionConfigValueId,
	values ...acp.SessionConfigSelectOption,
) acp.SessionConfigOption {
	ungrouped := acp.SessionConfigSelectOptionsUngrouped(values)
	return completeSelectConfigOption(
		id,
		name,
		category,
		current,
		acp.SessionConfigSelectOptions{Ungrouped: &ungrouped},
	)
}

func completeGroupedSelectConfigOption(
	id string,
	name string,
	category *acp.SessionConfigOptionCategory,
	current acp.SessionConfigValueId,
	groups []acp.SessionConfigSelectGroup,
) acp.SessionConfigOption {
	grouped := acp.SessionConfigSelectOptionsGrouped(groups)
	return completeSelectConfigOption(
		id,
		name,
		category,
		current,
		acp.SessionConfigSelectOptions{Grouped: &grouped},
	)
}

func completeSelectConfigOption(
	id string,
	name string,
	category *acp.SessionConfigOptionCategory,
	current acp.SessionConfigValueId,
	options acp.SessionConfigSelectOptions,
) acp.SessionConfigOption {
	return acp.SessionConfigOption{
		Select: &acp.SessionConfigOptionSelect{
			Meta: map[string]any{
				"provider": "ignored-provider",
				"model":    "ignored-model",
			},
			Category:     category,
			CurrentValue: current,
			Description:  speedTestPtr("Complete select fixture description"),
			Id:           acp.SessionConfigId(id),
			Name:         name,
			Options:      options,
			Type:         "select",
		},
	}
}

func completeSelectGroup(
	id string,
	name string,
	options ...acp.SessionConfigSelectOption,
) acp.SessionConfigSelectGroup {
	return acp.SessionConfigSelectGroup{
		Meta: map[string]any{
			"speed": "ignored",
		},
		Group:   acp.SessionConfigGroupId(id),
		Name:    name,
		Options: options,
	}
}

func completeSelectValue(value string, name string) acp.SessionConfigSelectOption {
	return acp.SessionConfigSelectOption{
		Meta: map[string]any{
			"speed": "ignored",
		},
		Description: speedTestPtr("Complete select value fixture description"),
		Name:        name,
		Value:       acp.SessionConfigValueId(value),
	}
}

func speedTestCategory(value string) *acp.SessionConfigOptionCategory {
	category := acp.SessionConfigOptionCategory(value)
	return &category
}

func speedTestCopySelect(
	option *acp.SessionConfigOptionSelect,
) *acp.SessionConfigOptionSelect {
	copied := *option
	return &copied
}

func speedTestPtr[T any](value T) *T {
	return &value
}
