package tplengine

import (
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockExecution implements core.Execution for testing
type MockExecution struct {
	parentInput *core.Input
	input       *core.Input
	output      *core.Output
	env         *core.EnvMap
	id          core.ID
	componentID string
	workflowID  string
	status      core.StatusType
	startTime   time.Time
	endTime     time.Time
	duration    time.Duration
}

func NewMockExecution() *MockExecution {
	return &MockExecution{
		parentInput: &core.Input{},
		input:       &core.Input{},
		output:      &core.Output{},
		env:         &core.EnvMap{},
		id:          core.MustNewID(),
		componentID: "test-component",
		workflowID:  "test-workflow",
		status:      core.StatusPending,
		startTime:   time.Time{},
		endTime:     time.Time{},
		duration:    0,
	}
}

func (m *MockExecution) StoreKey() []byte                 { return []byte("test-key") }
func (m *MockExecution) IsRunning() bool                  { return false }
func (m *MockExecution) GetID() core.ID                   { return m.id }
func (m *MockExecution) GetWorkflowID() string            { return m.workflowID }
func (m *MockExecution) GetWorkflowExecID() core.ID       { return m.id }
func (m *MockExecution) GetComponent() core.ComponentType { return core.ComponentTask }
func (m *MockExecution) GetComponentID() string           { return m.componentID }
func (m *MockExecution) GetStatus() core.StatusType       { return m.status }
func (m *MockExecution) GetEnv() *core.EnvMap             { return m.env }
func (m *MockExecution) GetParentInput() *core.Input      { return m.parentInput }
func (m *MockExecution) GetInput() *core.Input            { return m.input }
func (m *MockExecution) GetOutput() *core.Output          { return m.output }
func (m *MockExecution) GetError() *core.Error            { return nil }
func (m *MockExecution) SetDuration()                     {}
func (m *MockExecution) CalcDuration() time.Duration      { return 0 }
func (m *MockExecution) GetStartTime() time.Time          { return time.Time{} }
func (m *MockExecution) GetEndTime() time.Time            { return time.Time{} }
func (m *MockExecution) GetDuration() time.Duration       { return 0 }

func (e *MockExecution) AsExecMap() *core.ExecutionMap {
	execMap := core.ExecutionMap{
		Status:         e.GetStatus(),
		Component:      e.GetComponent(),
		WorkflowID:     e.GetWorkflowID(),
		WorkflowExecID: e.GetWorkflowExecID(),
		Input:          e.GetInput(),
		Output:         e.GetOutput(),
		Error:          e.GetError(),
		StartTime:      e.GetStartTime(),
		EndTime:        e.GetEndTime(),
		Duration:       e.CalcDuration(),
	}
	return &execMap
}

func (e *MockExecution) AsMainExecMap() *core.MainExecutionMap {
	return nil
}

func TestNormalizer_Normalize(t *testing.T) {
	t.Run("Should create normalized context with all execution data", func(t *testing.T) {
		normalizer := NewNormalizer()
		exec := NewMockExecution()

		// Set up test data
		*exec.parentInput = core.Input{
			"trigger_data": "from_trigger",
			"user_id":      123,
		}
		*exec.input = core.Input{
			"task_data": "from_task",
			"action":    "process",
		}
		*exec.output = core.Output{
			"result": "success",
		}
		*exec.env = core.EnvMap{
			"ENV_VAR": "test_value",
			"MODE":    "development",
		}

		// Normalize
		normalized := normalizer.Normalize(exec)

		// Verify structure
		require.Contains(t, normalized, "trigger")
		require.Contains(t, normalized, "input")
		require.Contains(t, normalized, "output")
		require.Contains(t, normalized, "env")

		// Verify trigger data
		trigger, ok := normalized["trigger"].(map[string]any)
		require.True(t, ok)
		require.Contains(t, trigger, "input")
		triggerInput := trigger["input"].(*core.Input)
		assert.Equal(t, "from_trigger", triggerInput.Prop("trigger_data"))
		assert.Equal(t, 123, triggerInput.Prop("user_id"))

		// Verify input data
		input := normalized["input"].(*core.Input)
		assert.Equal(t, "from_task", input.Prop("task_data"))
		assert.Equal(t, "process", input.Prop("action"))

		// Verify output data
		output := normalized["output"].(*core.Output)
		assert.Equal(t, "success", output.Prop("result"))

		// Verify env data
		env := normalized["env"].(*core.EnvMap)
		assert.Equal(t, "test_value", env.Prop("ENV_VAR"))
		assert.Equal(t, "development", env.Prop("MODE"))
	})
}

func TestNormalizer_ParseExecution_Input(t *testing.T) {
	t.Run("Should parse simple templates in input", func(t *testing.T) {
		normalizer := NewNormalizer()
		exec := NewMockExecution()

		// Set up test data
		*exec.parentInput = core.Input{
			"user_name": "John Doe",
			"user_age":  30,
		}
		*exec.input = core.Input{
			"greeting": "Hello, {{ .trigger.input.user_name }}!",
			"message":  "User is {{ .trigger.input.user_age }} years old",
			"static":   "no template here",
		}
		*exec.env = core.EnvMap{
			"ENVIRONMENT": "production",
		}

		// Parse execution
		err := normalizer.ParseExecution(exec)
		require.NoError(t, err)

		// Verify templates were parsed
		input := exec.GetInput()
		assert.Equal(t, "Hello, John Doe!", input.Prop("greeting"))
		assert.Equal(t, "User is 30 years old", input.Prop("message"))
		assert.Equal(t, "no template here", input.Prop("static"))
	})

	t.Run("Should parse nested templates in input", func(t *testing.T) {
		normalizer := NewNormalizer()
		exec := NewMockExecution()

		// Set up test data
		*exec.parentInput = core.Input{
			"user": map[string]any{
				"profile": map[string]any{
					"name":  "Jane Smith",
					"email": "jane@example.com",
				},
				"settings": map[string]any{
					"theme": "dark",
				},
			},
		}
		*exec.input = core.Input{
			"user_info": map[string]any{
				"display_name": "{{ .trigger.input.user.profile.name }}",
				"contact":      "{{ .trigger.input.user.profile.email }}",
				"theme":        "{{ .trigger.input.user.settings.theme }}",
			},
			"simple_array": []any{
				"{{ .trigger.input.user.profile.name }}",
				"static_value",
				"{{ .trigger.input.user.settings.theme }}",
			},
		}
		*exec.env = core.EnvMap{
			"ENVIRONMENT": "test",
		}

		// Parse execution
		err := normalizer.ParseExecution(exec)
		require.NoError(t, err)

		// Verify nested templates were parsed
		input := exec.GetInput()

		userInfo, ok := input.Prop("user_info").(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "Jane Smith", userInfo["display_name"])
		assert.Equal(t, "jane@example.com", userInfo["contact"])
		assert.Equal(t, "dark", userInfo["theme"])

		simpleArray, ok := input.Prop("simple_array").([]any)
		require.True(t, ok)
		assert.Equal(t, "Jane Smith", simpleArray[0])
		assert.Equal(t, "static_value", simpleArray[1])
		assert.Equal(t, "dark", simpleArray[2])
	})

	t.Run("Should parse templates with environment references", func(t *testing.T) {
		normalizer := NewNormalizer()
		exec := NewMockExecution()

		// Set up test data
		*exec.parentInput = core.Input{
			"endpoint": "users",
		}
		*exec.input = core.Input{
			"api_url": "{{ .env.BASE_URL }}/{{ .env.API_VERSION }}/{{ .trigger.input.endpoint }}",
			"config":  "Environment: {{ .env.ENVIRONMENT }}",
		}
		*exec.env = core.EnvMap{
			"BASE_URL":    "https://api.example.com",
			"API_VERSION": "v1",
			"ENVIRONMENT": "production",
		}

		// Parse execution
		err := normalizer.ParseExecution(exec)
		require.NoError(t, err)

		// Verify templates with env references were parsed
		input := exec.GetInput()
		assert.Equal(t, "https://api.example.com/v1/users", input.Prop("api_url"))
		assert.Equal(t, "Environment: production", input.Prop("config"))
	})
}

func TestNormalizer_ParseExecution_Environment(t *testing.T) {
	t.Run("Should parse templates in environment variables", func(t *testing.T) {
		normalizer := NewNormalizer()
		exec := NewMockExecution()

		// Set up test data
		*exec.parentInput = core.Input{
			"service": "user-service",
			"version": "v2",
		}
		*exec.input = core.Input{
			"action": "create",
		}
		*exec.env = core.EnvMap{
			"SERVICE_URL":   "https://{{ .trigger.input.service }}.example.com",
			"API_ENDPOINT":  "{{ .env.SERVICE_URL }}/{{ .trigger.input.version }}/{{ .input.action }}",
			"STATIC_CONFIG": "no_template_here",
			"BASE_URL":      "https://api.example.com", // Used by API_ENDPOINT
		}

		// Parse execution
		err := normalizer.ParseExecution(exec)
		require.NoError(t, err)

		// Verify templates in environment were parsed
		env := exec.GetEnv()
		assert.Equal(t, "https://user-service.example.com", env.Prop("SERVICE_URL"))
		assert.Equal(t, "no_template_here", env.Prop("STATIC_CONFIG"))
		// Note: API_ENDPOINT references SERVICE_URL which gets parsed first
		// The exact result depends on the order of processing
	})

	t.Run("Should handle environment variables without templates", func(t *testing.T) {
		normalizer := NewNormalizer()
		exec := NewMockExecution()

		// Set up test data
		*exec.parentInput = core.Input{}
		*exec.input = core.Input{}
		*exec.env = core.EnvMap{
			"STATIC_VAR":  "static_value",
			"ANOTHER_VAR": "another_static_value",
			"NUMBER_VAR":  "123",
		}

		// Parse execution
		err := normalizer.ParseExecution(exec)
		require.NoError(t, err)

		// Verify static environment variables remain unchanged
		env := exec.GetEnv()
		assert.Equal(t, "static_value", env.Prop("STATIC_VAR"))
		assert.Equal(t, "another_static_value", env.Prop("ANOTHER_VAR"))
		assert.Equal(t, "123", env.Prop("NUMBER_VAR"))
	})
}

func TestNormalizer_ParseExecution_SprigFunctions(t *testing.T) {
	t.Run("Should parse templates with sprig string functions", func(t *testing.T) {
		normalizer := NewNormalizer()
		exec := NewMockExecution()

		// Set up test data
		*exec.parentInput = core.Input{
			"user_name": "john doe",
			"email":     "JOHN.DOE@EXAMPLE.COM",
		}
		*exec.input = core.Input{
			"formatted_name":  "{{ title .trigger.input.user_name }}",
			"lowercase_email": "{{ lower .trigger.input.email }}",
			"uppercase_name":  "{{ upper .trigger.input.user_name }}",
			"contains_check":  "{{ contains \"doe\" .trigger.input.user_name }}",
		}
		*exec.env = core.EnvMap{}

		// Parse execution
		err := normalizer.ParseExecution(exec)
		require.NoError(t, err)

		// Verify sprig functions were applied
		input := exec.GetInput()
		assert.Equal(t, "John Doe", input.Prop("formatted_name"))
		assert.Equal(t, "john.doe@example.com", input.Prop("lowercase_email"))
		assert.Equal(t, "JOHN DOE", input.Prop("uppercase_name"))
		assert.Equal(t, "true", input.Prop("contains_check"))
	})

	t.Run("Should parse templates with sprig math functions", func(t *testing.T) {
		normalizer := NewNormalizer()
		exec := NewMockExecution()

		// Set up test data
		*exec.parentInput = core.Input{
			"base_number": 10,
			"multiplier":  3,
		}
		*exec.input = core.Input{
			"sum":     "{{ add .trigger.input.base_number 5 }}",
			"product": "{{ mul .trigger.input.base_number .trigger.input.multiplier }}",
			"max":     "{{ max .trigger.input.base_number 15 }}",
		}
		*exec.env = core.EnvMap{}

		// Parse execution
		err := normalizer.ParseExecution(exec)
		require.NoError(t, err)

		// Verify math functions were applied
		input := exec.GetInput()
		assert.Equal(t, "15", input.Prop("sum"))
		assert.Equal(t, "30", input.Prop("product"))
		assert.Equal(t, "15", input.Prop("max"))
	})
}

func TestNormalizer_ParseExecution_Conditionals(t *testing.T) {
	t.Run("Should parse templates with if/else conditionals", func(t *testing.T) {
		normalizer := NewNormalizer()
		exec := NewMockExecution()

		// Set up test data
		*exec.parentInput = core.Input{
			"user": map[string]any{
				"is_admin": true,
				"name":     "Admin User",
			},
		}
		*exec.input = core.Input{
			"access_level": "{{ if .trigger.input.user.is_admin }}Administrator{{ else }}User{{ end }}",
			"greeting":     "Hello, {{ .trigger.input.user.name }}!",
		}
		*exec.env = core.EnvMap{
			"MODE":        "production",
			"ENVIRONMENT": "{{ if eq .env.MODE \"production\" }}PROD{{ else }}DEV{{ end }}",
		}

		// Parse execution
		err := normalizer.ParseExecution(exec)
		require.NoError(t, err)

		// Verify conditionals were processed
		input := exec.GetInput()
		assert.Equal(t, "Administrator", input.Prop("access_level"))
		assert.Equal(t, "Hello, Admin User!", input.Prop("greeting"))

		env := exec.GetEnv()
		assert.Equal(t, "PROD", env.Prop("ENVIRONMENT"))
	})

	t.Run("Should handle false conditions", func(t *testing.T) {
		normalizer := NewNormalizer()
		exec := NewMockExecution()

		// Set up test data
		*exec.parentInput = core.Input{
			"user": map[string]any{
				"is_admin": false,
				"name":     "Regular User",
			},
		}
		*exec.input = core.Input{
			"access_level": "{{ if .trigger.input.user.is_admin }}Administrator{{ else }}User{{ end }}",
			"permissions":  "{{ if .trigger.input.user.is_admin }}Full Access{{ end }}",
		}
		*exec.env = core.EnvMap{}

		// Parse execution
		err := normalizer.ParseExecution(exec)
		require.NoError(t, err)

		// Verify false conditions
		input := exec.GetInput()
		assert.Equal(t, "User", input.Prop("access_level"))
		assert.Equal(t, "", input.Prop("permissions")) // Empty when condition is false
	})
}

func TestNormalizer_ParseExecution_DefaultValues(t *testing.T) {
	t.Run("Should handle missing values with default function", func(t *testing.T) {
		normalizer := NewNormalizer()
		exec := NewMockExecution()

		// Set up test data
		*exec.parentInput = core.Input{
			"existing_field": "value",
		}
		*exec.input = core.Input{
			"existing":     "{{ .trigger.input.existing_field }}",
			"missing_safe": "{{ .trigger.input.non_existent | default \"default_value\" }}",
			"missing_env":  "{{ .env.NON_EXISTENT | default \"env_default\" }}",
		}
		*exec.env = core.EnvMap{
			"EXISTING_ENV": "env_value",
		}

		// Parse execution
		err := normalizer.ParseExecution(exec)
		require.NoError(t, err)

		// Verify default values work
		input := exec.GetInput()
		assert.Equal(t, "value", input.Prop("existing"))
		assert.Equal(t, "default_value", input.Prop("missing_safe"))
		assert.Equal(t, "env_default", input.Prop("missing_env"))
	})
}

func TestNormalizer_ParseExecution_ComplexScenarios(t *testing.T) {
	t.Run("Should handle complex nested templates with multiple data sources", func(t *testing.T) {
		normalizer := NewNormalizer()
		exec := NewMockExecution()

		// Set up complex test data
		*exec.parentInput = core.Input{
			"request": map[string]any{
				"user": map[string]any{
					"id":    "user123",
					"email": "user@example.com",
				},
				"metadata": map[string]any{
					"source":    "api",
					"timestamp": "2023-01-01T00:00:00Z",
				},
			},
		}
		*exec.input = core.Input{
			"processing_config": map[string]any{
				"user_identifier":    "{{ .trigger.input.request.user.id }}",
				"notification_email": "{{ lower .trigger.input.request.user.email }}",
				"log_message":        "Processing request from {{ .trigger.input.request.metadata.source }} for user {{ .trigger.input.request.user.id }} in {{ .env.ENVIRONMENT }} environment",
				"features": []any{
					"{{ if eq .env.ENVIRONMENT \"production\" }}audit{{ end }}",
					"{{ if .trigger.input.request.metadata.source }}logging{{ end }}",
					"validation",
				},
			},
			"api_config": map[string]any{
				"endpoint": "{{ .env.API_BASE }}/users/{{ .trigger.input.request.user.id }}",
				"headers": map[string]any{
					"Authorization": "Bearer {{ .env.API_TOKEN }}",
					"Content-Type":  "application/json",
					"X-Source":      "{{ .trigger.input.request.metadata.source }}",
				},
			},
		}
		*exec.env = core.EnvMap{
			"ENVIRONMENT": "production",
			"API_BASE":    "https://api.example.com/v1",
			"API_TOKEN":   "secret-token-123",
		}

		// Parse execution
		err := normalizer.ParseExecution(exec)
		require.NoError(t, err)

		// Verify complex nested parsing
		input := exec.GetInput()

		processingConfig, ok := input.Prop("processing_config").(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "user123", processingConfig["user_identifier"])
		assert.Equal(t, "user@example.com", processingConfig["notification_email"])
		assert.Equal(t, "Processing request from api for user user123 in production environment", processingConfig["log_message"])

		features, ok := processingConfig["features"].([]any)
		require.True(t, ok)
		assert.Equal(t, "audit", features[0])
		assert.Equal(t, "logging", features[1])
		assert.Equal(t, "validation", features[2])

		apiConfig, ok := input.Prop("api_config").(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "https://api.example.com/v1/users/user123", apiConfig["endpoint"])

		headers, ok := apiConfig["headers"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "Bearer secret-token-123", headers["Authorization"])
		assert.Equal(t, "application/json", headers["Content-Type"])
		assert.Equal(t, "api", headers["X-Source"])
	})
}

func TestNormalizer_ParseExecution_ErrorHandling(t *testing.T) {
	t.Run("Should return error for invalid template syntax", func(t *testing.T) {
		normalizer := NewNormalizer()
		exec := NewMockExecution()

		// Set up test data with invalid template
		*exec.parentInput = core.Input{}
		*exec.input = core.Input{
			"invalid_template": "{{ .trigger.input.user_name", // Missing closing }}
		}
		*exec.env = core.EnvMap{}

		// Parse execution should fail
		err := normalizer.ParseExecution(exec)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse template in input[invalid_template]")
	})

	t.Run("Should handle nil template engine", func(t *testing.T) {
		normalizer := &Normalizer{
			TemplateEngine: nil,
		}
		exec := NewMockExecution()

		// Parse execution should fail
		err := normalizer.ParseExecution(exec)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "template engine is not initialized")
	})

	t.Run("Should handle empty input and env gracefully", func(t *testing.T) {
		normalizer := NewNormalizer()
		exec := NewMockExecution()

		// Set up empty data
		*exec.parentInput = core.Input{}
		*exec.input = core.Input{}
		*exec.env = core.EnvMap{}

		// Parse execution should succeed
		err := normalizer.ParseExecution(exec)
		require.NoError(t, err)
	})
}

func TestNormalizer_ParseExecution_ParentInput(t *testing.T) {
	t.Run("Should parse templates in parent input", func(t *testing.T) {
		normalizer := NewNormalizer()
		exec := NewMockExecution()

		// Set up test data
		*exec.parentInput = core.Input{
			"dynamic_value": "{{ .env.BASE_VALUE }}_processed",
			"static_value":  "no_template",
		}
		*exec.input = core.Input{
			"reference": "{{ .trigger.input.dynamic_value }}",
		}
		*exec.env = core.EnvMap{
			"BASE_VALUE": "test",
		}

		// Parse execution
		err := normalizer.ParseExecution(exec)
		require.NoError(t, err)

		// Verify parent input templates were parsed
		parentInput := exec.GetParentInput()
		assert.Equal(t, "test_processed", parentInput.Prop("dynamic_value"))
		assert.Equal(t, "no_template", parentInput.Prop("static_value"))

		// Verify input can reference parsed parent input
		input := exec.GetInput()
		assert.Equal(t, "test_processed", input.Prop("reference"))
	})
}
