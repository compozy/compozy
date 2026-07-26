package kinds

import (
	"encoding/json"
	"testing"
)

func TestSpeedContractLiterals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "normal speed", got: string(SpeedNormal), want: "normal"},
		{name: "fast speed", got: string(SpeedFast), want: "fast"},
		{name: "applied status", got: string(SpeedResolutionStatusApplied), want: "applied"},
		{name: "unsupported status", got: string(SpeedResolutionStatusUnsupported), want: "unsupported"},
		{name: "rejected status", got: string(SpeedResolutionStatusRejected), want: "rejected"},
		{
			name: "capability absent reason",
			got:  string(SpeedResolutionReasonCapabilityAbsent),
			want: "capability_absent",
		},
		{
			name: "capability ambiguous reason",
			got:  string(SpeedResolutionReasonCapabilityAmbiguous),
			want: "capability_ambiguous",
		},
		{
			name: "value ambiguous reason",
			got:  string(SpeedResolutionReasonValueAmbiguous),
			want: "value_ambiguous",
		},
		{
			name: "provider rejected reason",
			got:  string(SpeedResolutionReasonProviderRejected),
			want: "provider_rejected",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.got != tc.want {
				t.Fatalf("literal = %q, want %q", tc.got, tc.want)
			}
		})
	}
}

func TestSpeedResolutionJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		resolution SpeedResolution
		want       string
	}{
		{
			name:       "required fields remain present for zero values",
			resolution: SpeedResolution{},
			want:       `{"requested":"","status":""}`,
		},
		{
			name: "applied omits empty reason",
			resolution: SpeedResolution{
				Requested: SpeedFast,
				Status:    SpeedResolutionStatusApplied,
			},
			want: `{"requested":"fast","status":"applied"}`,
		},
		{
			name: "unsupported includes stable reason",
			resolution: SpeedResolution{
				Requested: SpeedNormal,
				Status:    SpeedResolutionStatusUnsupported,
				Reason:    SpeedResolutionReasonCapabilityAbsent,
			},
			want: `{"requested":"normal","status":"unsupported","reason":"capability_absent"}`,
		},
		{
			name: "rejected includes stable reason",
			resolution: SpeedResolution{
				Requested: SpeedFast,
				Status:    SpeedResolutionStatusRejected,
				Reason:    SpeedResolutionReasonProviderRejected,
			},
			want: `{"requested":"fast","status":"rejected","reason":"provider_rejected"}`,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			payload, err := json.Marshal(tc.resolution)
			if err != nil {
				t.Fatalf("marshal speed resolution: %v", err)
			}
			if string(payload) != tc.want {
				t.Fatalf("resolution JSON = %s, want %s", payload, tc.want)
			}

			var decoded SpeedResolution
			if err := json.Unmarshal(payload, &decoded); err != nil {
				t.Fatalf("unmarshal speed resolution: %v", err)
			}
			if decoded != tc.resolution {
				t.Fatalf("round-trip resolution = %#v, want %#v", decoded, tc.resolution)
			}
		})
	}
}
