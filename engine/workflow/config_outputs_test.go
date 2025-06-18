package workflow

import (
	"testing"
	"text/template"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_ValidateOutputsWithTemplates(t *testing.T) {
	tests := []struct {
		name      string
		outputs   *core.Input
		expectErr bool
		errMsg    string
	}{
		{
			name: "Should validate valid template syntax",
			outputs: &core.Input{
				"summary": "{{ .tasks.process_data.output.summary }}",
				"count":   "{{ .tasks.count_items.output.total }}",
			},
			expectErr: false,
		},
		{
			name: "Should reject invalid template syntax",
			outputs: &core.Input{
				"summary": "{{ .tasks.process_data.output.summary",
			},
			expectErr: true,
			errMsg:    "invalid template in outputs.summary",
		},
		{
			name: "Should validate nested outputs with templates",
			outputs: &core.Input{
				"results": map[string]any{
					"processed": "{{ .tasks.process.output.data }}",
					"stats": map[string]any{
						"total": "{{ .tasks.stats.output.total }}",
						"avg":   "{{ .tasks.stats.output.average }}",
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Should reject nested invalid templates",
			outputs: &core.Input{
				"results": map[string]any{
					"data": map[string]any{
						"bad": "{{ unclosed template",
					},
				},
			},
			expectErr: true,
			errMsg:    "invalid template in outputs.results.data.bad",
		},
		{
			name: "Should allow non-template strings",
			outputs: &core.Input{
				"message": "This is a plain string",
				"number":  42,
				"boolean": true,
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				ID:      "test-workflow",
				Outputs: tt.outputs,
			}
			err := config.validateOutputs()
			if tt.expectErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateTemplateString(t *testing.T) {
	tests := []struct {
		name      string
		template  string
		expectErr bool
	}{
		{
			name:      "valid simple template",
			template:  "{{ .value }}",
			expectErr: false,
		},
		{
			name:      "valid complex template",
			template:  "{{ if .enabled }}Value: {{ .data.value }}{{ end }}",
			expectErr: false,
		},
		{
			name:      "invalid unclosed template",
			template:  "{{ .value",
			expectErr: true,
		},
		{
			name:      "invalid syntax",
			template:  "{{ if .value }}missing end",
			expectErr: true,
		},
		{
			name:      "plain string without template",
			template:  "Just a plain string",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTemplateString(tt.template)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateTemplateString_CompatibilityWithGoTemplates(t *testing.T) {
	// Ensure our validation matches Go's template parser
	testTemplate := "{{ .tasks.process.output.result }}"

	// Our validator
	err := validateTemplateString(testTemplate)
	require.NoError(t, err)

	// Go's template parser
	_, err = template.New("test").Parse(testTemplate)
	require.NoError(t, err)
}
