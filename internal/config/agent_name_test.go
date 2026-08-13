package config

import (
	"strings"
	"testing"
)

func TestAgentNameValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "Should accept lowercase letters", input: "designer"},
		{name: "Should accept numbers after the first letter", input: "designer2"},
		{name: "Should accept hyphens", input: "audio-designer"},
		{name: "Should accept underscores", input: "audio_designer"},
		{name: "Should normalize outer whitespace", input: "  audio_designer  "},
		{name: "Should reject an empty name", input: "", wantErr: "agent name is required"},
		{name: "Should accept the maximum length", input: "a" + repeatAgentNameCharacter(105)},
		{name: "Should reject internal whitespace", input: "audio designer", wantErr: `agent name "audio designer" must start with a lowercase letter, use only lowercase letters, numbers, hyphens, or underscores, and be at most 106 characters`},
		{name: "Should reject uppercase letters", input: "AudioDesigner", wantErr: `agent name "AudioDesigner" must start with a lowercase letter, use only lowercase letters, numbers, hyphens, or underscores, and be at most 106 characters`},
		{name: "Should reject a leading number", input: "2designer", wantErr: `agent name "2designer" must start with a lowercase letter, use only lowercase letters, numbers, hyphens, or underscores, and be at most 106 characters`},
		{name: "Should reject dots", input: "audio.designer", wantErr: `agent name "audio.designer" must start with a lowercase letter, use only lowercase letters, numbers, hyphens, or underscores, and be at most 106 characters`},
		{name: "Should reject path separators", input: "audio/designer", wantErr: `agent name "audio/designer" must start with a lowercase letter, use only lowercase letters, numbers, hyphens, or underscores, and be at most 106 characters`},
		{name: "Should reject non-ASCII letters", input: "áudio_designer", wantErr: `agent name "áudio_designer" must start with a lowercase letter, use only lowercase letters, numbers, hyphens, or underscores, and be at most 106 characters`},
		{name: "Should reject a name above the maximum length", input: "a" + repeatAgentNameCharacter(106), wantErr: `agent name "` + "a" + repeatAgentNameCharacter(106) + `" must start with a lowercase letter, use only lowercase letters, numbers, hyphens, or underscores, and be at most 106 characters`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateAgentName(tt.input)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateAgentName(%q) error = %v, want nil", tt.input, err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateAgentName(%q) error = %v, want %q", tt.input, err, tt.wantErr)
			}
		})
	}

	t.Run("Should reject a noncanonical name while parsing", func(t *testing.T) {
		t.Parallel()

		_, err := ParseAgentDef([]byte(`---
name: audio designer
provider: claude
---

prompt`))
		wantErr := `agent name "audio designer" must start with a lowercase letter, use only lowercase letters, numbers, hyphens, or underscores, and be at most 106 characters`
		if err == nil || err.Error() != wantErr {
			t.Fatalf("ParseAgentDef() error = %v, want %q", err, wantErr)
		}
	})
}

func repeatAgentNameCharacter(count int) string {
	return strings.Repeat("a", count)
}
