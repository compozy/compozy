package model

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateTriggerPromptTemplateAcceptsSupportedReferences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		prompt string
	}{
		{
			name:   "Should accept top level fields",
			prompt: `Trigger kind: {{ .Kind }}`,
		},
		{
			name:   "Should accept data field chains",
			prompt: `Session: {{ .Data.session_id }}`,
		},
		{
			name:   "Should accept data index lookups",
			prompt: `Payload: {{ index .Data "payload" }}`,
		},
		{
			name:   "Should accept data scoped with blocks",
			prompt: `{{ with .Data }}{{ index . "session_id" }}{{ end }}`,
		},
		{
			name:   "Should accept data scoped field lookups",
			prompt: `{{ with .Data }}{{ .session_id }}{{ end }}`,
		},
		{
			name:   "Should accept range variables without variable rooted field lookups",
			prompt: `{{ range $key, $value := .Data }}{{ $key }}{{ end }}`,
		},
		{
			name:   "Should accept chained data expressions",
			prompt: `{{ (.Data).session_id }}`,
		},
		{
			name:   "Should accept defined templates with root envelope invocation",
			prompt: `{{ define "body" }}{{ .Source }}{{ end }}{{ template "body" . }}`,
		},
		{
			name:   "Should accept plain text without template delimiters",
			prompt: "Trigger kind: plain text only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := ValidateTriggerPromptTemplate(tt.prompt); err != nil {
				t.Fatalf("ValidateTriggerPromptTemplate() error = %v", err)
			}
		})
	}
}

func TestValidateTriggerPromptTemplateRejectsUnsupportedReferences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		prompt string
		want   []string
	}{
		{
			name:   "Should reject unknown top level fields",
			prompt: `{{ .EnvelopeID }}`,
			want:   []string{"EnvelopeID"},
		},
		{
			name:   "Should reject child fields on scalar values",
			prompt: `{{ .Scope.Name }}`,
			want:   []string{"Scope"},
		},
		{
			name:   "Should reject chained scalar field lookups",
			prompt: `{{ (.Source).Value }}`,
			want:   []string{"Source"},
		},
		{
			name:   "Should reject non data index targets",
			prompt: `{{ index .Kind "anything" }}`,
			want:   []string{".Kind"},
		},
		{
			name:   "Should reject root dot index targets",
			prompt: `{{ index . "payload" }}`,
			want:   []string{"only .Data"},
		},
		{
			name:   "Should reject variable rooted lookups",
			prompt: `{{ range $key, $value := .Data }}{{ $value.name }}{{ end }}`,
			want:   []string{"variable-rooted"},
		},
		{
			name:   "Should reject defined templates invoked with data scope",
			prompt: `{{ define "body" }}{{ .Source }}{{ end }}{{ template "body" .Data }}`,
			want:   []string{"template", ".Data", "root dot"},
		},
		{
			name:   "Should reject defined templates invoked without explicit root scope",
			prompt: `{{ define "body" }}static{{ end }}{{ template "body" }}`,
			want:   []string{"template", "root dot"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateTriggerPromptTemplate(tt.prompt)
			requireErrorContains(t, err, tt.want...)
		})
	}
}

func TestValidateTriggerPromptTemplateRejectsRequiredInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		prompt string
	}{
		{
			name:   "Should reject empty prompts",
			prompt: "",
		},
		{
			name:   "Should reject whitespace-only prompts",
			prompt: "   ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateTriggerPromptTemplate(tt.prompt)
			if !errors.Is(err, errTriggerPromptTemplateRequired) {
				t.Fatalf("ValidateTriggerPromptTemplate() error = %v, want %v", err, errTriggerPromptTemplateRequired)
			}
		})
	}
}

func TestParseTriggerPromptTemplateRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		prompt string
		want   []string
	}{
		{
			name:   "Should reject template syntax errors",
			prompt: "{{ if .Kind }}",
			want:   []string{"parse"},
		},
		{
			name:   "Should wrap validation failures with template context",
			prompt: `{{ .EnvelopeID }}`,
			want:   []string{`validate trigger prompt template "trigger_prompt"`, "EnvelopeID"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ParseTriggerPromptTemplate(tt.prompt)
			requireErrorContains(t, err, tt.want...)
		})
	}
}

func TestParseTriggerPromptTemplateRejectsRequiredInput(t *testing.T) {
	t.Parallel()

	t.Run("Should return the required-input sentinel", func(t *testing.T) {
		t.Parallel()

		_, err := ParseTriggerPromptTemplate("   ")
		if !errors.Is(err, errTriggerPromptTemplateRequired) {
			t.Fatalf("ParseTriggerPromptTemplate() error = %v, want %v", err, errTriggerPromptTemplateRequired)
		}
	})
}

func requireErrorContains(t *testing.T, err error, want ...string) {
	t.Helper()

	if err == nil {
		t.Fatalf("error = nil, want substrings %#v", want)
	}
	got := err.Error()
	for _, substring := range want {
		if !strings.Contains(got, substring) {
			t.Fatalf("error = %q, want substring %q", got, substring)
		}
	}
}
