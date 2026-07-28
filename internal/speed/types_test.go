package speed

import "testing"

func TestVocabulary(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"normal", "fast"} {
		t.Run("Should accept "+value, func(t *testing.T) {
			t.Parallel()
			if got, err := Parse(value); err != nil || string(got) != value {
				t.Fatalf("Parse(%q) = %q, %v", value, got, err)
			}
		})
	}
	for _, value := range []string{"", "FAST", "turbo"} {
		t.Run("Should reject "+value, func(t *testing.T) {
			t.Parallel()
			if _, err := Parse(value); err == nil {
				t.Fatalf("Parse(%q) error = nil", value)
			}
		})
	}
}

func TestResolutionValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   Resolution
		wantErr bool
	}{
		{
			name:  "Should accept an applied outcome without a reason",
			value: Resolution{Requested: SpeedFast, Status: ResolutionApplied},
		},
		{
			name: "Should accept an unsupported capability outcome",
			value: Resolution{
				Requested: SpeedNormal,
				Status:    ResolutionUnsupported,
				Reason:    ReasonCapabilityAbsent,
			},
		},
		{
			name: "Should accept a provider rejection",
			value: Resolution{
				Requested: SpeedFast,
				Status:    ResolutionRejected,
				Reason:    ReasonProviderRejected,
			},
		},
		{
			name: "Should reject a reason on an applied outcome",
			value: Resolution{
				Requested: SpeedFast,
				Status:    ResolutionApplied,
				Reason:    ReasonCapabilityAbsent,
			},
			wantErr: true,
		},
		{
			name:    "Should reject an empty request",
			value:   Resolution{Status: ResolutionApplied},
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateResolution(test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("ValidateResolution(%#v) error = %v, wantErr %t", test.value, err, test.wantErr)
			}
		})
	}
}
