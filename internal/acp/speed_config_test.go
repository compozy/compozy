package acp

import (
	"testing"

	acpsdk "github.com/coder/acp-go-sdk"
	speedpkg "github.com/compozy/compozy/internal/speed"
)

func TestMatchSpeedConfig(t *testing.T) {
	t.Parallel()

	selectOption := SessionConfigOption{
		ID:       "provider-speed",
		Label:    "Speed",
		Category: "model_config",
		Kind:     SessionConfigOptionKindSelect,
		Current:  "standard-tier",
		Values: []SessionConfigOptionValue{
			{Value: "standard-tier", Label: "Standard"},
			{Value: "accelerated-tier", Label: "Enabled"},
			{Value: "balanced-tier", Label: "Balanced"},
		},
	}
	booleanOption := SessionConfigOption{
		ID:       "fast",
		Label:    "Fast Mode",
		Category: "model_config",
		Kind:     SessionConfigOptionKindBoolean,
		Current:  "false",
	}

	tests := []struct {
		name       string
		requested  speedpkg.Speed
		options    []SessionConfigOption
		wantTarget string
		wantReason speedpkg.ResolutionReason
	}{
		{
			name:       "Should match a select option by normalized label and values",
			requested:  speedpkg.SpeedFast,
			options:    []SessionConfigOption{selectOption},
			wantTarget: "accelerated-tier",
		},
		{
			name:       "Should keep ACP boolean speed unsupported in V1",
			requested:  speedpkg.SpeedFast,
			options:    []SessionConfigOption{booleanOption},
			wantReason: speedpkg.ReasonCapabilityAbsent,
		},
		{
			name:      "Should reject a missing model config category",
			requested: speedpkg.SpeedFast,
			options: []SessionConfigOption{{
				ID:     "speed",
				Label:  "Speed",
				Kind:   SessionConfigOptionKindSelect,
				Values: selectOption.Values,
			}},
			wantReason: speedpkg.ReasonCapabilityAbsent,
		},
		{
			name:      "Should normalize category and option name tokens",
			requested: speedpkg.SpeedNormal,
			options: []SessionConfigOption{withSpeedOptionIdentity(
				selectOption,
				"provider-toggle",
				" FAST_MODE ",
				" MODEL - CONFIG ",
			)},
			wantTarget: "standard-tier",
		},
		{
			name:      "Should reject multiple speed candidates",
			requested: speedpkg.SpeedFast,
			options: []SessionConfigOption{
				selectOption,
				withSpeedOptionIdentity(selectOption, speedConfigKey, "Speed", speedConfigCategory),
			},
			wantReason: speedpkg.ReasonCapabilityAmbiguous,
		},
		{
			name:      "Should reject a duplicate option ID outside the speed category",
			requested: speedpkg.SpeedFast,
			options: []SessionConfigOption{
				selectOption,
				{
					ID:       "provider-speed",
					Label:    "Quality",
					Category: sessionConfigModeKey,
					Kind:     SessionConfigOptionKindBoolean,
				},
			},
			wantReason: speedpkg.ReasonCapabilityAmbiguous,
		},
		{
			name:      "Should reject a missing normal value",
			requested: speedpkg.SpeedFast,
			options: []SessionConfigOption{withSpeedOptionValues(selectOption,
				SessionConfigOptionValue{Value: "accelerated-tier", Label: "Fast"},
			)},
			wantReason: speedpkg.ReasonValueAmbiguous,
		},
		{
			name:      "Should reject conflicting value signals",
			requested: speedpkg.SpeedFast,
			options: []SessionConfigOption{withSpeedOptionValues(selectOption,
				SessionConfigOptionValue{Value: "normal", Label: "Fast"},
				SessionConfigOptionValue{Value: "fast", Label: "Fast"},
			)},
			wantReason: speedpkg.ReasonValueAmbiguous,
		},
		{
			name:      "Should reject duplicate target value IDs",
			requested: speedpkg.SpeedNormal,
			options: []SessionConfigOption{withSpeedOptionValues(selectOption,
				SessionConfigOptionValue{Value: "normal-tier", Label: "Normal"},
				SessionConfigOptionValue{Value: "fast-tier", Label: "Fast"},
				SessionConfigOptionValue{Value: "normal-tier", Label: "Legacy"},
			)},
			wantReason: speedpkg.ReasonValueAmbiguous,
		},
		{
			name:       "Should reject an empty compatible option ID",
			requested:  speedpkg.SpeedFast,
			options:    []SessionConfigOption{withSpeedOptionIdentity(selectOption, "", "Speed", "model_config")},
			wantReason: speedpkg.ReasonValueAmbiguous,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			match, reason := matchSpeedConfig(test.requested, test.options)
			if test.wantReason != "" {
				if match != nil || reason != test.wantReason {
					t.Fatalf("matchSpeedConfig() = %#v, %q, want nil, %q", match, reason, test.wantReason)
				}
				return
			}
			if match == nil || reason != "" || match.target != test.wantTarget {
				t.Fatalf("matchSpeedConfig() = %#v, %q, want target %q", match, reason, test.wantTarget)
			}
		})
	}
}

func TestSpeedConfigMatchRequestAndConfirmation(t *testing.T) {
	t.Parallel()

	t.Run("Should build and confirm an exact select value request", func(t *testing.T) {
		t.Parallel()

		option := SessionConfigOption{
			ID:       "provider-speed",
			Label:    "Speed",
			Category: "model_config",
			Kind:     SessionConfigOptionKindSelect,
			Current:  "normal-tier",
			Values: []SessionConfigOptionValue{
				{Value: "normal-tier", Label: "Normal"},
				{Value: "fast-tier", Label: "Fast"},
			},
		}
		match, reason := matchSpeedConfig(speedpkg.SpeedFast, []SessionConfigOption{option})
		if match == nil || reason != "" {
			t.Fatalf("matchSpeedConfig() = %#v, %q, want supported", match, reason)
		}
		if match.alreadyApplied() {
			t.Fatal("alreadyApplied() = true, want false")
		}

		request := match.request(acpsdk.SessionId("session-123"))
		if request.ValueId == nil ||
			request.ValueId.SessionId != "session-123" ||
			request.ValueId.ConfigId != "provider-speed" ||
			request.ValueId.Value != "fast-tier" ||
			request.Boolean != nil {
			t.Fatalf("request() = %#v, want select provider-speed=fast-tier", request)
		}

		confirmed := option
		confirmed.Current = "fast-tier"
		if !match.confirmed([]SessionConfigOption{confirmed}) {
			t.Fatal("confirmed() = false, want true")
		}
		if match.confirmed([]SessionConfigOption{confirmed, confirmed}) {
			t.Fatal("confirmed(duplicate response) = true, want false")
		}
	})
}

func withSpeedOptionIdentity(
	option SessionConfigOption,
	id string,
	label string,
	category string,
) SessionConfigOption {
	option.ID = id
	option.Label = label
	option.Category = category
	return option
}

func withSpeedOptionValues(
	option SessionConfigOption,
	values ...SessionConfigOptionValue,
) SessionConfigOption {
	option.Values = values
	return option
}
