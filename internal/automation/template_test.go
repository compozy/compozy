package automation

import "testing"

func TestValidateTriggerPromptTemplateAcceptsSupportedEnvelopeReferences(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		prompt string
	}{
		{
			name:   "top level field",
			prompt: `Trigger kind: {{ .Kind }}`,
		},
		{
			name:   "data field chain",
			prompt: `Session: {{ .Data.session_id }}`,
		},
		{
			name:   "index data lookup",
			prompt: `Payload: {{ index .Data "payload" }}`,
		},
		{
			name:   "control flow",
			prompt: `{{ if .Data.session_id }}ready{{ else }}missing{{ end }}`,
		},
		{
			name:   "with block",
			prompt: `{{ with .Data }}{{ index . "session_id" }}{{ end }}`,
		},
		{
			name:   "with block field lookup",
			prompt: `{{ with .Data }}{{ .session_id }}{{ end }}`,
		},
		{
			name:   "range block",
			prompt: `{{ range $key, $value := .Data }}{{ $key }}{{ end }}`,
		},
		{
			name:   "chain expression",
			prompt: `{{ (.Data).session_id }}`,
		},
		{
			name:   "defined template",
			prompt: `{{ define "body" }}{{ .Source }}{{ end }}{{ template "body" . }}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := ValidateTriggerPromptTemplate(tc.prompt); err != nil {
				t.Fatalf("ValidateTriggerPromptTemplate() error = %v", err)
			}
		})
	}
}

func TestValidateTriggerPromptTemplateRejectsUnsupportedEnvelopeReferences(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		prompt string
		want   string
	}{
		{
			name:   "unknown top level field",
			prompt: `{{ .EnvelopeID }}`,
			want:   "EnvelopeID",
		},
		{
			name:   "child field on scalar",
			prompt: `{{ .Scope.Name }}`,
			want:   "Scope",
		},
		{
			name:   "chain expression on scalar",
			prompt: `{{ (.Source).Value }}`,
			want:   "Source",
		},
		{
			name:   "unsupported index target",
			prompt: `{{ index .Kind "anything" }}`,
			want:   ".Kind",
		},
		{
			name:   "root dot index at top level",
			prompt: `{{ index . "payload" }}`,
			want:   "only .Data",
		},
		{
			name:   "with non-data dot index",
			prompt: `{{ with .Kind }}{{ index . "payload" }}{{ end }}`,
			want:   ".Kind",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateTriggerPromptTemplate(tc.prompt)
			if err == nil {
				t.Fatal("ValidateTriggerPromptTemplate() error = nil, want non-nil")
			}
			if got := err.Error(); !containsAll(got, tc.want) {
				t.Fatalf("ValidateTriggerPromptTemplate() error = %q, want substring %q", got, tc.want)
			}
		})
	}
}

func TestParseTriggerPromptTemplateRejectsEmptyAndSyntaxErrors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		prompt string
		want   string
	}{
		{
			name:   "empty prompt",
			prompt: "   ",
			want:   "required",
		},
		{
			name:   "syntax error",
			prompt: "{{ if .Kind }}",
			want:   "parse",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := ParseTriggerPromptTemplate(tc.prompt)
			if err == nil {
				t.Fatal("ParseTriggerPromptTemplate() error = nil, want non-nil")
			}
			if got := err.Error(); !containsAll(got, tc.want) {
				t.Fatalf("ParseTriggerPromptTemplate() error = %q, want substring %q", got, tc.want)
			}
		})
	}
}

func TestTriggerPromptTemplateErrorsIncludeValidationContext(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		run  func() error
		want []string
	}{
		{
			name: "parse wraps validation failures with template context",
			run: func() error {
				_, err := ParseTriggerPromptTemplate(`{{ .EnvelopeID }}`)
				return err
			},
			want: []string{`validate trigger prompt template "trigger_prompt"`, "EnvelopeID"},
		},
		{
			name: "validate wraps parser results with api context",
			run: func() error {
				return ValidateTriggerPromptTemplate(`{{ .EnvelopeID }}`)
			},
			want: []string{"validate trigger prompt template", "EnvelopeID"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.run()
			if err == nil {
				t.Fatal("trigger prompt template validation error = nil, want non-nil")
			}
			if got := err.Error(); !containsAll(got, tc.want...) {
				t.Fatalf("trigger prompt template validation error = %q, want substrings %#v", got, tc.want)
			}
		})
	}
}
