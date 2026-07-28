// Suite: tool presentation metadata validation
// Invariant: descriptor presentation metadata remains bounded, single-line, and declarative.
// Boundary IN: toolmeta descriptor validation.
// Boundary OUT: descriptor transport and progress rendering, owned by their package suites.
package toolmeta

import (
	"strings"
	"testing"
)

func TestValidateDescriptorMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		friendlyVerb string
		preview      string
		wantError    string
	}{
		{
			name:         "Should accept bounded metadata with a supported hint",
			friendlyVerb: "Looking up",
			preview:      "query",
		},
		{
			name:         "Should accept a friendly verb exactly at the rune limit",
			friendlyVerb: strings.Repeat("界", maxDescriptorPresentationRunes),
			preview:      "none",
		},
		{
			name:         "Should preserve a camelCase declarative argument name",
			friendlyVerb: "Looking up",
			preview:      "arg:issueKey",
		},
		{
			name:         "Should reject a multiline friendly verb",
			friendlyVerb: "Looking\nup",
			preview:      "query",
			wantError:    "friendly_verb must be one line",
		},
		{
			name:         "Should reject a control character in the friendly verb",
			friendlyVerb: "Looking\x01up",
			preview:      "query",
			wantError:    "friendly_verb contains control characters",
		},
		{
			name:         "Should reject a friendly verb one rune over the limit",
			friendlyVerb: strings.Repeat("界", maxDescriptorPresentationRunes+1),
			preview:      "query",
			wantError:    "friendly_verb exceeds 80 runes",
		},
		{
			name:         "Should reject secret shaped material in a friendly verb",
			friendlyVerb: "api_key=raw-friendly-secret",
			preview:      "query",
			wantError:    "friendly_verb contains secret-shaped material",
		},
		{
			name:         "Should reject an executable preview strategy",
			friendlyVerb: "Running",
			preview:      "shell:command",
			wantError:    "preview must be a supported declarative hint",
		},
		{
			name:         "Should reject an invalid camelCase argument path",
			friendlyVerb: "Looking up",
			preview:      "arg:issue/Key",
			wantError:    "preview must be a supported declarative hint",
		},
		{
			name:         "Should reject a sensitive declarative argument",
			friendlyVerb: "Signing",
			preview:      "arg:privateKey",
			wantError:    "preview must be a supported declarative hint",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateDescriptorMetadata(test.friendlyVerb, test.preview)
			if test.wantError == "" {
				if err != nil {
					t.Fatalf("ValidateDescriptorMetadata() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateDescriptorMetadata() error = nil, want containing %q", test.wantError)
			}
			if !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("ValidateDescriptorMetadata() error = %q, want containing %q", err.Error(), test.wantError)
			}
		})
	}
}
