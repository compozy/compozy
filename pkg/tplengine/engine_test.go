package tplengine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestTemplateEngine_Configuration(t *testing.T) {
	t.Run("Should create engine with specified format and allow format changes", func(t *testing.T) {
		// Test YAML format initialization
		engine := NewEngine(FormatYAML)
		assert.Equal(t, FormatYAML, engine.format, "Engine should initialize with YAML format")

		// Test format change behavior
		engine = engine.WithFormat(FormatJSON)
		assert.Equal(t, FormatJSON, engine.format, "Engine should change to JSON format")

		// Test precision preservation configuration
		engine = engine.WithPrecisionPreservation(true)
		assert.True(t, engine.preserveNumericPrecision, "Engine should enable precision preservation")
	})
}

func TestTemplateEngine_HasTemplate(t *testing.T) {
	t.Run("Should correctly identify template markers", func(t *testing.T) {
		// Test various template marker patterns
		assert.True(t, HasTemplate("Hello, {{ .name }}!"), "Should detect simple template markers")
		assert.True(t, HasTemplate("{{.user.profile.name}}"), "Should detect template markers without spaces")
		assert.True(t, HasTemplate("{{ if .condition }}yes{{ end }}"), "Should detect conditional templates")
		assert.False(t, HasTemplate("Hello, World!"), "Should not detect templates in plain text")
		assert.False(t, HasTemplate("{ single brace }"), "Should not detect single braces as templates")
	})
}

func TestTemplateEngine_AddTemplate(t *testing.T) {
	t.Run("Should add valid templates and reject invalid syntax", func(t *testing.T) {
		engine := NewEngine(FormatYAML)

		// Test adding valid template
		err := engine.AddTemplate("greeting", "Hello, {{ .name }}!")
		require.NoError(t, err, "Valid template should be added successfully")

		// Test template with complex expressions
		err = engine.AddTemplate("complex", "{{ if .user }}Hello {{ .user.name | upper }}{{ else }}Anonymous{{ end }}")
		require.NoError(t, err, "Complex template should be added successfully")

		// Test invalid template syntax
		err = engine.AddTemplate("invalid", "Hello, {{ .name !")
		require.Error(t, err, "Invalid template syntax should be rejected")
		assert.Contains(t, err.Error(), "template", "Error should mention template parsing failure")
	})
}

func TestTemplateEngine_Render(t *testing.T) {
	t.Run("Should render registered templates with context data", func(t *testing.T) {
		engine := NewEngine(FormatYAML)
		err := engine.AddTemplate("greeting", "Hello, {{ .name }}!")
		require.NoError(t, err)

		// Test successful template rendering
		result, err := engine.Render("greeting", map[string]any{"name": "World"})
		require.NoError(t, err)
		assert.Equal(t, "Hello, World!", result, "Template should render with provided context")

		// Test rendering with complex context
		err = engine.AddTemplate("user_info", "User: {{ .user.name }} ({{ .user.role | upper }})")
		require.NoError(t, err)

		result, err = engine.Render("user_info", map[string]any{
			"user": map[string]any{
				"name": "John",
				"role": "admin",
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "User: John (ADMIN)", result, "Template should handle nested context and filters")

		// Test error for non-existent template
		_, err = engine.Render("non-existent", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "template", "Error should indicate template not found")
	})
}

func TestTemplateEngine_RenderString(t *testing.T) {
	t.Run("Should render template strings with context data and handle edge cases", func(t *testing.T) {
		engine := NewEngine(FormatYAML)

		// Test simple variable substitution
		result, err := engine.RenderString("Hello, {{ .name }}!", map[string]any{"name": "World"})
		require.NoError(t, err)
		assert.Equal(t, "Hello, World!", result, "Simple template substitution should work")

		// Test strings without template markers
		result, err = engine.RenderString("Hello, World!", nil)
		require.NoError(t, err)
		assert.Equal(t, "Hello, World!", result, "Plain text should pass through unchanged")

		// Test nested property access
		result, err = engine.RenderString("{{ .user.profile.name }}", map[string]any{
			"user": map[string]any{
				"profile": map[string]any{
					"name": "John",
				},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "John", result, "Nested property access should work")

		// Test template with Sprig functions
		result, err = engine.RenderString("{{ .text | upper | trim }}", map[string]any{"text": "  hello world  "})
		require.NoError(t, err)
		assert.Equal(t, "HELLO WORLD", result, "Template should support chained Sprig functions")

		// Test invalid template syntax
		_, err = engine.RenderString("Hello, {{ .name !", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "template", "Invalid template syntax should produce meaningful error")
	})
}

func TestTemplateEngine_FormatSpecificRendering(t *testing.T) {
	t.Run("Should render templates correctly for different output formats", func(t *testing.T) {
		ctx := map[string]any{
			"user": map[string]any{
				"name":   "John",
				"age":    30,
				"active": true,
			},
		}

		// Test YAML format rendering
		yamlEngine := NewEngine(FormatYAML)
		yamlStr := `name: {{ .user.name }}
age: {{ .user.age }}
active: {{ .user.active }}`

		renderedYAML, err := yamlEngine.RenderString(yamlStr, ctx)
		require.NoError(t, err)
		assert.Contains(t, renderedYAML, "name: John", "YAML should render string values correctly")
		assert.Contains(t, renderedYAML, "age: 30", "YAML should render numeric values correctly")
		assert.Contains(t, renderedYAML, "active: true", "YAML should render boolean values correctly")

		// Test JSON format rendering
		jsonEngine := NewEngine(FormatJSON)
		jsonStr := `{
  "name": "{{ .user.name }}",
  "age": {{ .user.age }},
  "active": {{ .user.active }}
}`

		renderedJSON, err := jsonEngine.RenderString(jsonStr, ctx)
		require.NoError(t, err)

		// Verify JSON is valid by parsing it
		var jsonMap map[string]any
		err = json.Unmarshal([]byte(renderedJSON), &jsonMap)
		require.NoError(t, err, "Rendered JSON should be valid")
		assert.Equal(t, "John", jsonMap["name"], "JSON should preserve string values")
		assert.Equal(t, float64(30), jsonMap["age"], "JSON should convert numbers to float64")
		assert.Equal(t, true, jsonMap["active"], "JSON should preserve boolean values")
	})
}

func TestTemplateEngine_FileProcessing(t *testing.T) {
	t.Run("Should process template files with context data", func(t *testing.T) {
		// Create temporary directory and test file
		tempDir, err := os.MkdirTemp("", "tplengine-test")
		require.NoError(t, err)
		defer func() {
			if err := os.RemoveAll(tempDir); err != nil {
				t.Logf("Failed to remove temp dir: %v", err)
			}
		}()

		// Create test template file with complex structure
		templateContent := `workflow:
  name: {{ .workflow.name }}
  user: {{ .user.name }}
  tasks:
    - name: {{ .tasks.primary.name }}
      status: {{ .tasks.primary.status }}
    - name: {{ .tasks.secondary.name }}
      status: {{ .tasks.secondary.status }}`

		filePath := filepath.Join(tempDir, "workflow.yaml")
		err = os.WriteFile(filePath, []byte(templateContent), 0o644)
		require.NoError(t, err)

		// Process file with comprehensive context
		engine := NewEngine(FormatYAML)
		ctx := map[string]any{
			"workflow": map[string]any{"name": "TestWorkflow"},
			"user":     map[string]any{"name": "John"},
			"tasks": map[string]any{
				"primary":   map[string]any{"name": "ProcessData", "status": "completed"},
				"secondary": map[string]any{"name": "SendNotification", "status": "pending"},
			},
		}

		// Read and render file content
		content, err := os.ReadFile(filePath)
		require.NoError(t, err)

		renderedContent, err := engine.RenderString(string(content), ctx)
		require.NoError(t, err)

		// Verify template processing produced valid YAML with expected values
		var result map[string]any
		err = yaml.Unmarshal([]byte(renderedContent), &result)
		require.NoError(t, err)

		workflow := result["workflow"].(map[string]any)
		assert.Equal(t, "TestWorkflow", workflow["name"], "Workflow name should be processed correctly")
		assert.Equal(t, "John", workflow["user"], "User name should be processed correctly")

		tasks := workflow["tasks"].([]any)
		assert.Len(t, tasks, 2, "Should process both task entries")

		primaryTask := tasks[0].(map[string]any)
		assert.Equal(t, "ProcessData", primaryTask["name"], "Primary task name should be processed")
		assert.Equal(t, "completed", primaryTask["status"], "Primary task status should be processed")
	})
}

func TestTemplateEngine_ContextPreprocessing(t *testing.T) {
	t.Run("Should preprocess context with default fields while preserving user data", func(t *testing.T) {
		engine := NewEngine(FormatYAML)
		userContext := map[string]any{
			"name":   "John",
			"custom": "value",
		}

		processed := engine.preprocessContext(userContext)

		// Verify user data is preserved
		assert.Equal(t, "John", processed["name"], "User-provided name should be preserved")
		assert.Equal(t, "value", processed["custom"], "User-provided custom field should be preserved")

		// Verify default template context fields are added
		assert.NotNil(t, processed["env"], "Default env field should be added")
		assert.NotNil(t, processed["input"], "Default input field should be added")
		assert.Nil(t, processed["output"], "Output field should be nil by default")
		assert.NotNil(t, processed["trigger"], "Default trigger field should be added")
		assert.NotNil(t, processed["tools"], "Default tools field should be added")
		assert.NotNil(t, processed["tasks"], "Default tasks field should be added")
		assert.NotNil(t, processed["agents"], "Default agents field should be added")

		// Verify default fields don't override user data
		userContextWithConflict := map[string]any{
			"env":  "user-env",
			"name": "Jane",
		}
		processedWithConflict := engine.preprocessContext(userContextWithConflict)
		assert.Equal(t, "user-env", processedWithConflict["env"], "User-provided env should not be overridden")
		assert.Equal(t, "Jane", processedWithConflict["name"], "User-provided name should be preserved")
	})
}

func TestTemplateEngine_SprigFunctionIntegration(t *testing.T) {
	t.Run("Should provide comprehensive Sprig function support for template operations", func(t *testing.T) {
		engine := NewEngine(FormatYAML)

		// Test string manipulation functions
		result, err := engine.RenderString("{{ contains \"world\" \"hello world\" }}", nil)
		require.NoError(t, err)
		assert.Equal(t, "true", result, "Contains function should work correctly")

		result, err = engine.RenderString("{{ \"hello world\" | hasPrefix \"hello\" }}", nil)
		require.NoError(t, err)
		assert.Equal(t, "true", result, "HasPrefix function should work with pipe syntax")

		result, err = engine.RenderString("{{ \"hello world\" | hasSuffix \"world\" }}", nil)
		require.NoError(t, err)
		assert.Equal(t, "true", result, "HasSuffix function should work with pipe syntax")

		// Test regex pattern matching
		result, err = engine.RenderString("{{ regexMatch \"^[a-z]+$\" \"hello\" }}", nil)
		require.NoError(t, err)
		assert.Equal(t, "true", result, "RegexMatch should validate patterns correctly")

		// Test data transformation functions with complex data
		ctx := map[string]any{
			"workflow": map[string]any{
				"name":    "ProcessData",
				"version": 2,
				"active":  true,
				"tasks":   []string{"validate", "transform", "store"},
			},
		}

		// Test JSON serialization of complex data
		result, err = engine.RenderString("{{ .workflow | toJson }}", ctx)
		require.NoError(t, err)

		// Verify JSON output contains expected structure
		var parsed map[string]any
		err = json.Unmarshal([]byte(result), &parsed)
		require.NoError(t, err, "toJson should produce valid JSON")
		assert.Equal(t, "ProcessData", parsed["name"], "JSON should preserve string values")
		assert.Equal(t, float64(2), parsed["version"], "JSON should preserve numeric values")
		assert.Equal(t, true, parsed["active"], "JSON should preserve boolean values")

		// Test toString function
		result, err = engine.RenderString("{{ .workflow | toString }}", ctx)
		require.NoError(t, err)
		assert.Contains(t, result, "name:ProcessData", "toString should include key-value pairs")
		assert.Contains(t, result, "version:2", "toString should include numeric values")
	})
}

func TestTemplateEngine_MissingKeyHandling(t *testing.T) {
	t.Run("Should enforce strict key validation and provide error handling mechanisms", func(t *testing.T) {
		engine := NewEngine(FormatYAML)

		// Test strict error handling for missing keys
		_, err := engine.RenderString("{{ .nonexistent.field }}", map[string]any{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nonexistent", "Error should identify the missing key")

		// Test error for common typos in field names
		context := map[string]any{
			"user": map[string]any{"name": "John"},
		}
		_, err = engine.RenderString("{{ .usr.name }}", context)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "usr", "Error should identify the misspelled key")

		// Test error for accessing nested fields on non-existent objects
		_, err = engine.RenderString("{{ .user.profile.age }}", context)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "profile", "Error should identify the missing nested key")

		// Test proper conditional handling for missing keys
		result, err := engine.RenderString(
			"{{ if hasKey .config \"timeout\" }}{{ .config.timeout }}{{ else }}30{{ end }}",
			map[string]any{"config": map[string]any{"timeout": 60}},
		)
		require.NoError(t, err)
		assert.Equal(t, "60", result, "Should access existing keys with conditional checks")

		// Test default value for missing nested keys
		result, err = engine.RenderString(
			"{{ if hasKey .config \"missing\" }}{{ .config.missing }}{{ else }}default-value{{ end }}",
			map[string]any{"config": map[string]any{"timeout": 30}},
		)
		require.NoError(t, err)
		assert.Equal(t, "default-value", result, "Should provide default values for missing keys using conditionals")

		// Test successful access to existing keys
		result, err = engine.RenderString("{{ .user.name }}", context)
		require.NoError(t, err)
		assert.Equal(t, "John", result, "Should successfully access existing keys")
	})
}

// TestHyphenHandling tests the preprocessing of templates with hyphens in field names
func TestHyphenHandling(t *testing.T) {
	engine := NewEngine(FormatText)

	t.Run("Should handle simple hyphenated field names", func(t *testing.T) {
		context := map[string]any{
			"tasks": map[string]any{
				"task-one": map[string]any{
					"output": map[string]any{
						"result": "success",
					},
				},
			},
		}

		result, err := engine.RenderString("{{ .tasks.task-one.output.result }}", context)
		assert.NoError(t, err)
		assert.Equal(t, "success", result)
	})

	t.Run("Should handle multiple hyphenated field names", func(t *testing.T) {
		context := map[string]any{
			"tasks": map[string]any{
				"first-task": map[string]any{
					"output": map[string]any{
						"result": "first-result",
					},
				},
				"second-task": map[string]any{
					"output": map[string]any{
						"result": "second-result",
					},
				},
			},
		}

		result, err := engine.RenderString("{{ .tasks.first-task.output.result }}", context)
		assert.NoError(t, err)
		assert.Equal(t, "first-result", result)

		result, err = engine.RenderString("{{ .tasks.second-task.output.result }}", context)
		assert.NoError(t, err)
		assert.Equal(t, "second-result", result)
	})

	t.Run("Should handle hyphenated fields with filters", func(t *testing.T) {
		context := map[string]any{
			"tasks": map[string]any{
				"format-task": map[string]any{
					"output": map[string]any{
						"code": "hello world",
					},
				},
			},
		}

		result, err := engine.RenderString("{{ .tasks.format-task.output.code | upper }}", context)
		assert.NoError(t, err)
		assert.Equal(t, "HELLO WORLD", result)
	})

	t.Run("Should handle hyphenated fields with toJson filter", func(t *testing.T) {
		context := map[string]any{
			"tasks": map[string]any{
				"data-task": map[string]any{
					"output": map[string]any{
						"items": []string{"a", "b", "c"},
						"count": 3,
					},
				},
			},
		}

		result, err := engine.RenderString("{{ .tasks.data-task.output | toJson }}", context)
		assert.NoError(t, err)
		expected := `{"count":3,"items":["a","b","c"]}`
		assert.JSONEq(t, expected, result)
	})

	t.Run("Should handle mixed hyphenated and non-hyphenated fields", func(t *testing.T) {
		context := map[string]any{
			"workflow": map[string]any{
				"input": map[string]any{
					"name": "test-workflow",
				},
			},
			"tasks": map[string]any{
				"setup-task": map[string]any{
					"output": map[string]any{
						"config": map[string]any{
							"timeout": 30,
						},
					},
				},
				"process": map[string]any{
					"output": map[string]any{
						"result": "processed",
					},
				},
			},
		}

		// Mix of hyphenated and non-hyphenated
		result, err := engine.RenderString(
			"{{ .workflow.input.name }}-{{ .tasks.setup-task.output.config.timeout }}",
			context,
		)
		assert.NoError(t, err)
		assert.Equal(t, "test-workflow-30", result)

		// Non-hyphenated task reference
		result, err = engine.RenderString("{{ .tasks.process.output.result }}", context)
		assert.NoError(t, err)
		assert.Equal(t, "processed", result)
	})

	t.Run("Should handle deeply nested hyphenated fields", func(t *testing.T) {
		context := map[string]any{
			"tasks": map[string]any{
				"complex-task": map[string]any{
					"output": map[string]any{
						"nested-data": map[string]any{
							"sub-field": map[string]any{
								"final-value": "deep-success",
							},
						},
					},
				},
			},
		}

		result, err := engine.RenderString(
			"{{ .tasks.complex-task.output.nested-data.sub-field.final-value }}",
			context,
		)
		assert.NoError(t, err)
		assert.Equal(t, "deep-success", result)
	})

	t.Run("Should handle hyphens at different levels", func(t *testing.T) {
		context := map[string]any{
			"my-workflow": map[string]any{
				"tasks": map[string]any{
					"task-one": map[string]any{
						"output": "result1",
					},
				},
			},
		}

		result, err := engine.RenderString("{{ .my-workflow.tasks.task-one.output }}", context)
		assert.NoError(t, err)
		assert.Equal(t, "result1", result)
	})

	t.Run("Should preserve non-hyphenated templates unchanged", func(t *testing.T) {
		context := map[string]any{
			"user": map[string]any{
				"profile": map[string]any{
					"name": "John",
				},
			},
		}

		result, err := engine.RenderString("{{ .user.profile.name }}", context)
		assert.NoError(t, err)
		assert.Equal(t, "John", result)
	})

	t.Run("Should handle conditional expressions with hyphens", func(t *testing.T) {
		context := map[string]any{
			"tasks": map[string]any{
				"check-task": map[string]any{
					"output": map[string]any{
						"status": "success",
					},
				},
			},
		}

		result, err := engine.RenderString(
			"{{ if eq .tasks.check-task.output.status \"success\" }}PASS{{ else }}FAIL{{ end }}",
			context,
		)
		assert.NoError(t, err)
		assert.Equal(t, "PASS", result)
	})
}

// TestPreprocessTemplateForHyphens tests the preprocessing function directly
func TestPreprocessTemplateForHyphens(t *testing.T) {
	engine := NewEngine(FormatText)

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple hyphenated field",
			input:    "{{ .tasks.task-one.output }}",
			expected: "{{ (index . \"tasks\" \"task-one\" \"output\") }}",
		},
		{
			name:     "No hyphens should remain unchanged",
			input:    "{{ .user.profile.name }}",
			expected: "{{ .user.profile.name }}",
		},
		{
			name:     "Multiple hyphens in path",
			input:    "{{ .tasks.complex-task-name.output.sub-field }}",
			expected: "{{ (index . \"tasks\" \"complex-task-name\" \"output\" \"sub-field\") }}",
		},
		{
			name:     "With filter",
			input:    "{{ .tasks.data-task.output | toJson }}",
			expected: "{{ (index . \"tasks\" \"data-task\" \"output\") | toJson }}",
		},
		{
			name:     "With complex filter",
			input:    "{{ .tasks.format-task.output.code | upper | trim }}",
			expected: "{{ (index . \"tasks\" \"format-task\" \"output\" \"code\") | upper | trim }}",
		},
		{
			name:     "Hyphen at root level",
			input:    "{{ .my-workflow.tasks.output }}",
			expected: "{{ (index . \"my-workflow\" \"tasks\" \"output\") }}",
		},
		{
			name:     "Mixed with conditional",
			input:    "{{ if .tasks.check-task.output.status }}true{{ end }}",
			expected: "{{ if (index . \"tasks\" \"check-task\" \"output\" \"status\") }}true{{ end }}",
		},
		{
			name:     "Non-template text should remain unchanged",
			input:    "This is just text with task-name hyphens",
			expected: "This is just text with task-name hyphens",
		},
		{
			name:     "Multiple templates in same string",
			input:    "First: {{ .tasks.task-one.output }}, Second: {{ .tasks.task-two.result }}",
			expected: "First: {{ (index . \"tasks\" \"task-one\" \"output\") }}, Second: {{ (index . \"tasks\" \"task-two\" \"result\") }}",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := engine.preprocessTemplateForHyphens(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}
