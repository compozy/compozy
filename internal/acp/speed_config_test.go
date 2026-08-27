package acp

import (
	"encoding/json"
	"testing"

	acpsdk "github.com/coder/acp-go-sdk"
	speedpkg "github.com/compozy/compozy/internal/speed"
)

func TestMatchSpeedConfig(t *testing.T) {
	t.Parallel()

	selectOption := SessionConfigOption{
		ID:             "provider-speed",
		Label:          "Speed",
		Category:       "model_config",
		Kind:           SessionConfigOptionKindSelect,
		CurrentValueID: "standard-tier",
		Values: []SessionConfigOptionValue{
			{Value: "standard-tier", Label: "Standard"},
			{Value: "accelerated-tier", Label: "Enabled"},
			{Value: "balanced-tier", Label: "Balanced"},
		},
	}
	booleanOption := SessionConfigOption{
		ID:          "fast",
		Label:       "Fast Mode",
		Category:    "model_config",
		Kind:        SessionConfigOptionKindBoolean,
		CurrentBool: boolPointer(false),
	}

	tests := []struct {
		name       string
		requested  speedpkg.Speed
		options    []SessionConfigOption
		wantTarget string
		wantBool   *bool
		wantReason speedpkg.ResolutionReason
	}{
		{
			name:       "Should match a select option by normalized label and values",
			requested:  speedpkg.SpeedFast,
			options:    []SessionConfigOption{selectOption},
			wantTarget: "accelerated-tier",
		},
		{
			name:      "Should match a boolean speed option",
			requested: speedpkg.SpeedFast,
			options:   []SessionConfigOption{booleanOption},
			wantBool:  boolPointer(true),
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
			if match == nil || reason != "" || match.selection.ValueID != test.wantTarget ||
				!equalOptionalBool(match.selection.BoolValue, test.wantBool) {
				t.Fatalf(
					"matchSpeedConfig() = %#v, %q, want value %q bool %#v",
					match,
					reason,
					test.wantTarget,
					test.wantBool,
				)
			}
		})
	}
}

func TestSpeedConfigMatchRequestAndConfirmation(t *testing.T) {
	t.Parallel()

	t.Run("Should build and confirm an exact select value request", func(t *testing.T) {
		t.Parallel()

		option := SessionConfigOption{
			ID:             "provider-speed",
			Label:          "Speed",
			Category:       "model_config",
			Kind:           SessionConfigOptionKindSelect,
			CurrentValueID: "normal-tier",
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

		request, err := match.request(acpsdk.SessionId("session-123"))
		if err != nil {
			t.Fatalf("request() error = %v", err)
		}
		if request.SessionID != "session-123" ||
			request.ConfigID != "provider-speed" ||
			request.Type != "id" ||
			request.Value != acpsdk.SessionConfigValueId("fast-tier") {
			t.Fatalf("request() = %#v, want select provider-speed=fast-tier", request)
		}
		assertConfigOptionRequestJSON(t, request, "id", "fast-tier")

		confirmed := option
		confirmed.CurrentValueID = "fast-tier"
		if !match.confirmed([]SessionConfigOption{confirmed}) {
			t.Fatal("confirmed() = false, want true")
		}
		if match.confirmed([]SessionConfigOption{confirmed, confirmed}) {
			t.Fatal("confirmed(duplicate response) = true, want false")
		}
	})

	t.Run("Should build and confirm a boolean request", func(t *testing.T) {
		t.Parallel()

		option := SessionConfigOption{
			ID:          "fast",
			Label:       "Fast Mode",
			Category:    "model_config",
			Kind:        SessionConfigOptionKindBoolean,
			CurrentBool: boolPointer(false),
		}
		match, reason := matchSpeedConfig(speedpkg.SpeedFast, []SessionConfigOption{option})
		if match == nil || reason != "" {
			t.Fatalf("matchSpeedConfig() = %#v, %q, want supported", match, reason)
		}
		if match.alreadyApplied() {
			t.Fatal("alreadyApplied() = true, want false")
		}

		request, err := match.request(acpsdk.SessionId("session-123"))
		if err != nil {
			t.Fatalf("request() error = %v", err)
		}
		if request.SessionID != "session-123" ||
			request.ConfigID != "fast" ||
			request.Type != "boolean" ||
			request.Value != true {
			t.Fatalf("request() = %#v, want boolean fast=true", request)
		}
		assertConfigOptionRequestJSON(t, request, "boolean", true)

		confirmed := option
		confirmed.CurrentBool = boolPointer(true)
		if !match.confirmed([]SessionConfigOption{confirmed}) {
			t.Fatal("confirmed() = false, want true")
		}
	})
}

func assertConfigOptionRequestJSON(
	t *testing.T,
	request setSessionConfigOptionWireRequest,
	wantType string,
	wantValue any,
) {
	t.Helper()

	data, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("json.Marshal(request) error = %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("json.Unmarshal(request) error = %v", err)
	}
	if payload["type"] != wantType || payload["value"] != wantValue {
		t.Fatalf("request JSON = %s, want type=%q value=%#v", data, wantType, wantValue)
	}
}

func equalOptionalBool(left *bool, right *bool) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
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
