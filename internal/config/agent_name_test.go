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
		{name: "Should reject internal whitespace", input: "audio designer", wantErr: "must match"},
		{name: "Should reject uppercase letters", input: "AudioDesigner", wantErr: "must match"},
		{name: "Should reject a leading number", input: "2designer", wantErr: "must match"},
		{name: "Should reject dots", input: "audio.designer", wantErr: "must match"},
		{name: "Should reject path separators", input: "audio/designer", wantErr: "must match"},
		{name: "Should reject non-ASCII letters", input: "áudio_designer", wantErr: "must match"},
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
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
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
		if err == nil || !strings.Contains(err.Error(), "must match") {
			t.Fatalf("ParseAgentDef() error = %v, want name grammar error", err)
		}
	})
}
