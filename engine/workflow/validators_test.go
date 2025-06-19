package workflow

import (
	"context"
	"os"
	"testing"
	"text/template"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validation(t *testing.T) {
	workflowID := "test-workflow"

	t.Run("Should validate valid workflow configuration", func(t *testing.T) {
		cwd, err := core.CWDFromPath("/test/path")
		require.NoError(t, err)
		config := &Config{
			ID:   workflowID,
			Opts: Opts{},
			CWD:  cwd,
		}

		err = config.Validate()
		require.NoError(t, err)
	})

	t.Run("Should return error when CWD is missing", func(t *testing.T) {
		config := &Config{
			ID: "test-workflow",
		}

		err := config.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "current working directory is required for test-workflow")
	})
}

func TestConfig_TriggerValidation(t *testing.T) {
	t.Run("Should validate signal trigger correctly", func(t *testing.T) {
		cwd, err := core.CWDFromPath("/test/path")
		require.NoError(t, err)
		config := &Config{
			ID:  "test-workflow",
			CWD: cwd,
			Triggers: []Trigger{
				{
					Type: TriggerTypeSignal,
					Name: "order.created",
				},
			},
		}
		err = config.Validate()
		require.NoError(t, err)
	})

	t.Run("Should return error for unsupported trigger type", func(t *testing.T) {
		cwd, err := core.CWDFromPath("/test/path")
		require.NoError(t, err)
		config := &Config{
			ID:  "test-workflow",
			CWD: cwd,
			Triggers: []Trigger{
				{
					Type: "unsupported",
					Name: "test.event",
				},
			},
		}
		err = config.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported trigger type: unsupported")
	})

	t.Run("Should return error for empty trigger name", func(t *testing.T) {
		cwd, err := core.CWDFromPath("/test/path")
		require.NoError(t, err)
		config := &Config{
			ID:  "test-workflow",
			CWD: cwd,
			Triggers: []Trigger{
				{
					Type: TriggerTypeSignal,
					Name: "",
				},
			},
		}
		err = config.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "trigger name is required")
	})

	t.Run("Should return error for duplicate trigger names", func(t *testing.T) {
		cwd, err := core.CWDFromPath("/test/path")
		require.NoError(t, err)
		config := &Config{
			ID:  "test-workflow",
			CWD: cwd,
			Triggers: []Trigger{
				{
					Type: TriggerTypeSignal,
					Name: "order.created",
				},
				{
					Type: TriggerTypeSignal,
					Name: "order.created",
				},
			},
		}
		err = config.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate trigger name: order.created")
	})

	t.Run("Should validate trigger with valid schema", func(t *testing.T) {
		validSchema := &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"orderId": map[string]any{
					"type": "string",
				},
			},
			"required": []any{"orderId"},
		}
		cwd, err := core.CWDFromPath("/test/path")
		require.NoError(t, err)
		config := &Config{
			ID:  "test-workflow",
			CWD: cwd,
			Triggers: []Trigger{
				{
					Type:   TriggerTypeSignal,
					Name:   "order.created",
					Schema: validSchema,
				},
			},
		}
		err = config.Validate()
		require.NoError(t, err)
	})

	t.Run("Should return error for trigger with invalid schema", func(t *testing.T) {
		invalidSchema := &schema.Schema{
			"type":       "invalid-type",
			"properties": "should-be-object",
		}
		cwd, err := core.CWDFromPath("/test/path")
		require.NoError(t, err)
		config := &Config{
			ID:  "test-workflow",
			CWD: cwd,
			Triggers: []Trigger{
				{
					Type:   TriggerTypeSignal,
					Name:   "order.created",
					Schema: invalidSchema,
				},
			},
		}
		err = config.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid trigger schema for order.created")
	})
}

func TestConfig_MCPValidation(t *testing.T) {
	t.Run("Should validate individual MCP configurations", func(t *testing.T) {
		// Set required environment variable for MCP validation
		os.Setenv("MCP_PROXY_URL", "http://localhost:8081")
		defer os.Unsetenv("MCP_PROXY_URL")

		CWD, err := core.CWDFromPath("./fixtures")
		require.NoError(t, err)

		config, err := Load(CWD, "mcp_workflow.yaml")
		require.NoError(t, err)

		// Test that MCP configs are validated
		for i := range config.MCPs {
			config.MCPs[i].SetDefaults()
			err := config.MCPs[i].Validate()
			assert.NoError(t, err)
		}
	})
}

func TestConfig_ValidateInput(t *testing.T) {
	t.Run("Should validate input against schema", func(t *testing.T) {
		config := &Config{
			ID: "test-workflow",
			Opts: Opts{
				InputSchema: &schema.Schema{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{
							"type": "string",
						},
					},
					"required": []string{"name"},
				},
			},
		}

		input := &core.Input{
			"name": "test",
		}

		err := config.ValidateInput(context.Background(), input)
		assert.NoError(t, err)
	})

	t.Run("Should return error for invalid input", func(t *testing.T) {
		config := &Config{
			ID: "test-workflow",
			Opts: Opts{
				InputSchema: &schema.Schema{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{
							"type": "string",
						},
					},
					"required": []string{"name"},
				},
			},
		}

		input := &core.Input{
			"age": 30, // missing required "name"
		}

		err := config.ValidateInput(context.Background(), input)
		assert.Error(t, err)
	})

	t.Run("Should handle nil schema", func(t *testing.T) {
		config := &Config{
			ID: "test-workflow",
			// No input schema
		}

		input := &core.Input{
			"anything": "goes",
		}

		err := config.ValidateInput(context.Background(), input)
		assert.NoError(t, err)
	})
}

func TestOutputsValidator_Validate(t *testing.T) {
	t.Run("Should validate valid template syntax", func(t *testing.T) {
		config := &Config{
			ID: "test-workflow",
			Outputs: &core.Output{
				"summary": "{{ .tasks.process_data.output.summary }}",
				"count":   "{{ .tasks.count_items.output.total }}",
			},
		}
		validator := NewOutputsValidator(config)
		err := validator.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should reject invalid template syntax", func(t *testing.T) {
		config := &Config{
			ID: "test-workflow",
			Outputs: &core.Output{
				"summary": "{{ .tasks.process_data.output.summary",
			},
		}
		validator := NewOutputsValidator(config)
		err := validator.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid template in outputs.summary")
	})

	t.Run("Should validate nested outputs with templates", func(t *testing.T) {
		config := &Config{
			ID: "test-workflow",
			Outputs: &core.Output{
				"results": map[string]any{
					"processed": "{{ .tasks.process.output.data }}",
					"stats": map[string]any{
						"total": "{{ .tasks.stats.output.total }}",
						"avg":   "{{ .tasks.stats.output.average }}",
					},
				},
			},
		}
		validator := NewOutputsValidator(config)
		err := validator.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should reject nested invalid templates", func(t *testing.T) {
		config := &Config{
			ID: "test-workflow",
			Outputs: &core.Output{
				"results": map[string]any{
					"data": map[string]any{
						"bad": "{{ unclosed template",
					},
				},
			},
		}
		validator := NewOutputsValidator(config)
		err := validator.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid template in outputs.results.data.bad")
	})

	t.Run("Should allow non-template strings", func(t *testing.T) {
		config := &Config{
			ID: "test-workflow",
			Outputs: &core.Output{
				"message": "This is a plain string",
				"number":  42,
				"boolean": true,
			},
		}
		validator := NewOutputsValidator(config)
		err := validator.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should pass validation when outputs is nil", func(t *testing.T) {
		config := &Config{
			ID: "test-workflow",
		}
		validator := NewOutputsValidator(config)
		err := validator.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should fail validation with empty outputs", func(t *testing.T) {
		config := &Config{
			ID:      "test-workflow",
			Outputs: &core.Output{},
		}
		validator := NewOutputsValidator(config)
		err := validator.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "outputs cannot be empty when defined")
	})

	t.Run("Should fail validation with invalid template syntax in outputs", func(t *testing.T) {
		cwd, _ := setupTest(t, "basic_workflow.yaml")
		outputs := &core.Output{
			"result": "{{ .tasks[ }}",
		}
		config := &Config{
			ID:      "test-workflow",
			Outputs: outputs,
		}
		config.SetCWD(cwd.PathStr())

		err := config.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "template")
	})

	t.Run("Should validate template strings in arrays", func(t *testing.T) {
		config := &Config{
			ID: "test-workflow",
			Outputs: &core.Output{
				"results": []any{
					"{{ .task1.result }}",
					"{{ .task2.result }}",
					"static value",
				},
			},
		}
		validator := NewOutputsValidator(config)
		err := validator.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should reject invalid template syntax in arrays", func(t *testing.T) {
		config := &Config{
			ID: "test-workflow",
			Outputs: &core.Output{
				"results": []any{
					"{{ .task1.result }}",
					"{{ .task2.result",
				},
			},
		}
		validator := NewOutputsValidator(config)
		err := validator.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid template in outputs.results[1]")
	})

	t.Run("Should validate nested objects in arrays", func(t *testing.T) {
		config := &Config{
			ID: "test-workflow",
			Outputs: &core.Output{
				"items": []any{
					map[string]any{
						"name":  "{{ .task.name }}",
						"value": 42,
					},
					map[string]any{
						"name":  "static",
						"value": "{{ .task.value }}",
					},
				},
			},
		}
		validator := NewOutputsValidator(config)
		err := validator.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should reject invalid templates in nested objects within arrays", func(t *testing.T) {
		config := &Config{
			ID: "test-workflow",
			Outputs: &core.Output{
				"items": []any{
					map[string]any{
						"name": "{{ .task.name",
					},
				},
			},
		}
		validator := NewOutputsValidator(config)
		err := validator.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid template in outputs.items[0].name")
	})

	t.Run("Should validate nested arrays", func(t *testing.T) {
		config := &Config{
			ID: "test-workflow",
			Outputs: &core.Output{
				"matrix": []any{
					[]any{"{{ .row1.col1 }}", "{{ .row1.col2 }}"},
					[]any{"{{ .row2.col1 }}", "static"},
				},
			},
		}
		validator := NewOutputsValidator(config)
		err := validator.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should reject invalid templates in nested arrays", func(t *testing.T) {
		config := &Config{
			ID: "test-workflow",
			Outputs: &core.Output{
				"matrix": []any{
					[]any{"{{ .row1.col1 }}", "{{ .row1.col2"},
				},
			},
		}
		validator := NewOutputsValidator(config)
		err := validator.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid template in outputs.matrix[0][1]")
	})
}

func TestOutputsValidator_ValidateTemplateString(t *testing.T) {
	validator := NewOutputsValidator(&Config{})

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
			err := validator.validateTemplateString(tt.template)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOutputsValidator_CompatibilityWithGoTemplates(t *testing.T) {
	t.Run("Should match Go template parser validation", func(t *testing.T) {
		validator := NewOutputsValidator(&Config{})
		testTemplate := "{{ .tasks.process.output.result }}"

		// Our validator
		err := validator.validateTemplateString(testTemplate)
		require.NoError(t, err)

		// Go's template parser
		_, err = template.New("test").Parse(testTemplate)
		require.NoError(t, err)
	})
}

func TestScheduleValidator_Validate(t *testing.T) {
	t.Run("Should pass validation when schedule is nil", func(t *testing.T) {
		cwd, err := core.CWDFromPath("/test/path")
		require.NoError(t, err)
		config := &Config{
			ID:       "test-workflow",
			CWD:      cwd,
			Schedule: nil,
		}
		validator := NewScheduleValidator(config)
		err = validator.Validate()
		assert.NoError(t, err)
	})
	t.Run("Should validate schedule configuration", func(t *testing.T) {
		cwd, err := core.CWDFromPath("/test/path")
		require.NoError(t, err)
		enabled := true
		config := &Config{
			ID:  "test-workflow",
			CWD: cwd,
			Schedule: &Schedule{
				Cron:          "0 9 * * 1-5",
				Timezone:      "America/New_York",
				Enabled:       &enabled,
				Jitter:        "5m",
				OverlapPolicy: OverlapSkip,
			},
		}
		validator := NewScheduleValidator(config)
		err = validator.Validate()
		assert.NoError(t, err)
	})
	t.Run("Should fail with invalid cron expression", func(t *testing.T) {
		cwd, err := core.CWDFromPath("/test/path")
		require.NoError(t, err)
		config := &Config{
			ID:  "test-workflow",
			CWD: cwd,
			Schedule: &Schedule{
				Cron: "invalid cron",
			},
		}
		validator := NewScheduleValidator(config)
		err = validator.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "schedule validation error")
		assert.Contains(t, err.Error(), "invalid cron expression")
	})
	t.Run("Should validate schedule input against workflow input schema", func(t *testing.T) {
		cwd, err := core.CWDFromPath("/test/path")
		require.NoError(t, err)
		// Create a test schema that requires a "name" field
		inputSchema := &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type": "string",
				},
				"count": map[string]any{
					"type": "number",
				},
			},
			"required": []string{"name"},
		}
		config := &Config{
			ID:  "test-workflow",
			CWD: cwd,
			Opts: Opts{
				InputSchema: inputSchema,
			},
			Schedule: &Schedule{
				Cron: "0 9 * * *",
				Input: map[string]any{
					"name":  "test",
					"count": 42,
				},
			},
		}
		validator := NewScheduleValidator(config)
		err = validator.Validate()
		assert.NoError(t, err)
	})
	t.Run("Should fail when schedule input violates workflow input schema", func(t *testing.T) {
		cwd, err := core.CWDFromPath("/test/path")
		require.NoError(t, err)
		// Create a test schema that requires a "name" field
		inputSchema := &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type": "string",
				},
			},
			"required": []string{"name"},
		}
		config := &Config{
			ID:  "test-workflow",
			CWD: cwd,
			Opts: Opts{
				InputSchema: inputSchema,
			},
			Schedule: &Schedule{
				Cron: "0 9 * * *",
				Input: map[string]any{
					// Missing required "name" field
					"other": "value",
				},
			},
		}
		validator := NewScheduleValidator(config)
		err = validator.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "schedule input validation error")
	})
	t.Run("Should fail when schedule has no input but workflow requires it", func(t *testing.T) {
		cwd, err := core.CWDFromPath("/test/path")
		require.NoError(t, err)
		inputSchema := &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type": "string",
				},
			},
			"required": []string{"name"},
		}
		config := &Config{
			ID:  "test-workflow",
			CWD: cwd,
			Opts: Opts{
				InputSchema: inputSchema,
			},
			Schedule: &Schedule{
				Cron: "0 9 * * *",
				// No input specified - should fail because "name" is required
			},
		}
		validator := NewScheduleValidator(config)
		err = validator.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "schedule input validation error")
	})
	t.Run("Should pass when schedule has no input but schema has defaults", func(t *testing.T) {
		cwd, err := core.CWDFromPath("/test/path")
		require.NoError(t, err)
		inputSchema := &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":    "string",
					"default": "default-name",
				},
			},
			"required": []string{"name"},
		}
		config := &Config{
			ID:  "test-workflow",
			CWD: cwd,
			Opts: Opts{
				InputSchema: inputSchema,
			},
			Schedule: &Schedule{
				Cron: "0 9 * * *",
				// No input specified - should pass because default is provided
			},
		}
		validator := NewScheduleValidator(config)
		err = validator.Validate()
		assert.NoError(t, err)
	})
	t.Run("Should be integrated into workflow validation", func(t *testing.T) {
		cwd, err := core.CWDFromPath("/test/path")
		require.NoError(t, err)
		config := &Config{
			ID:  "test-workflow",
			CWD: cwd,
			Schedule: &Schedule{
				Cron: "invalid cron expression",
			},
		}
		// Test through the main workflow validator
		err = config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "schedule validation error")
	})
}
