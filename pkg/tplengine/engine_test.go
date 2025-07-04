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

// TestNewEngine is a basic test for the NewEngine function
// More comprehensive tests are in test/integration/tplengine_test.go
func TestNewEngine(t *testing.T) {
	engine := NewEngine(FormatYAML)
	assert.Equal(t, FormatYAML, engine.format)
	assert.NotNil(t, engine.templates)
	assert.NotNil(t, engine.globalValues)
}

func TestWithFormat(t *testing.T) {
	engine := NewEngine(FormatYAML)
	assert.Equal(t, FormatYAML, engine.format)

	engine = engine.WithFormat(FormatJSON)
	assert.Equal(t, FormatJSON, engine.format)
}

// TestHasTemplate is a basic test for the HasTemplate function
// More comprehensive tests are in test/integration/tplengine_test.go
func TestHasTemplate(t *testing.T) {
	assert.True(t, HasTemplate("Hello, {{ .name }}!"))
	assert.False(t, HasTemplate("Hello, World!"))
}

func TestAddTemplate(t *testing.T) {
	engine := NewEngine(FormatYAML)
	err := engine.AddTemplate("greeting", "Hello, {{ .name }}!")
	assert.NoError(t, err)

	// Test with an invalid template
	err = engine.AddTemplate("invalid", "Hello, {{ .name !")
	assert.Error(t, err)
}

func TestRender(t *testing.T) {
	engine := NewEngine(FormatYAML)
	err := engine.AddTemplate("greeting", "Hello, {{ .name }}!")
	assert.NoError(t, err)

	result, err := engine.Render("greeting", map[string]any{"name": "World"})
	assert.NoError(t, err)
	assert.Equal(t, "Hello, World!", result)

	_, err = engine.Render("non-existent", nil)
	assert.Error(t, err)
}

func TestRenderString(t *testing.T) {
	engine := NewEngine(FormatYAML)

	// Simple variable substitution
	result, err := engine.RenderString("Hello, {{ .name }}!", map[string]any{"name": "World"})
	assert.NoError(t, err)
	assert.Equal(t, "Hello, World!", result)

	// No template markers
	result, err = engine.RenderString("Hello, World!", nil)
	assert.NoError(t, err)
	assert.Equal(t, "Hello, World!", result)

	// Nested property access
	result, err = engine.RenderString("{{ .user.profile.name }}", map[string]any{
		"user": map[string]any{
			"profile": map[string]any{
				"name": "John",
			},
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, "John", result)

	// Invalid template
	_, err = engine.RenderString("Hello, {{ .name !", nil)
	assert.Error(t, err)
}

func TestProcessString(t *testing.T) {
	// YAML format
	engine := NewEngine(FormatYAML)

	// Simple YAML with template
	yamlStr := `
name: {{ .user.name }}
age: {{ .user.age }}
`
	ctx := map[string]any{
		"user": map[string]any{
			"name": "John",
			"age":  30,
		},
	}

	// For YAML format, we need to first render the template, then parse it
	renderedYAML, err := engine.RenderString(yamlStr, ctx)
	assert.NoError(t, err)
	assert.NotNil(t, renderedYAML)

	// Convert to map for easier assertions
	var yamlMap map[string]any
	// Parse the rendered YAML
	err = yaml.Unmarshal([]byte(renderedYAML), &yamlMap)
	assert.NoError(t, err)

	assert.Equal(t, "John", yamlMap["name"])
	assert.Equal(t, 30, yamlMap["age"])

	// JSON format
	engine = NewEngine(FormatJSON)

	// For JSON format, we need to first render the template, then parse it
	jsonStr := `{
  "name": "{{ .user.name }}",
  "age": {{ .user.age }}
}`

	// First render the template string
	renderedJSON, err := engine.RenderString(jsonStr, ctx)
	assert.NoError(t, err)
	assert.NotNil(t, renderedJSON)

	// Then parse the rendered JSON
	var jsonMap map[string]any
	err = json.Unmarshal([]byte(renderedJSON), &jsonMap)
	assert.NoError(t, err)
	assert.Equal(t, "John", jsonMap["name"])
	assert.Equal(t, float64(30), jsonMap["age"]) // JSON numbers are float64
}

func TestProcessFile(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "tplengine-test")
	require.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	// Create test YAML file
	yamlContent := `
name: {{ .user.name }}
age: {{ .user.age }}
`
	yamlPath := filepath.Join(tempDir, "test.yaml")
	err = os.WriteFile(yamlPath, []byte(yamlContent), 0o644)
	require.NoError(t, err)

	// Process YAML file
	engine := NewEngine(FormatYAML)
	ctx := map[string]any{
		"user": map[string]any{
			"name": "John",
			"age":  30,
		},
	}

	// Read the file content manually and render it
	content, err := os.ReadFile(yamlPath)
	require.NoError(t, err)

	renderedYAML, err := engine.RenderString(string(content), ctx)
	assert.NoError(t, err)
	assert.NotNil(t, renderedYAML)

	// Convert to map for easier assertions
	var yamlMap map[string]any
	// Parse the rendered YAML
	err = yaml.Unmarshal([]byte(renderedYAML), &yamlMap)
	assert.NoError(t, err)

	assert.Equal(t, "John", yamlMap["name"])
	assert.Equal(t, 30, yamlMap["age"])
}

func TestPreprocessContext(t *testing.T) {
	engine := NewEngine(FormatYAML)
	ctx := map[string]any{
		"name": "John",
	}

	processed := engine.preprocessContext(ctx)

	// Check that original data is preserved
	assert.Equal(t, "John", processed["name"])

	// Check that default fields are added
	assert.NotNil(t, processed["env"])
	assert.NotNil(t, processed["input"])
	assert.Nil(t, processed["output"])
	assert.NotNil(t, processed["trigger"])
	assert.NotNil(t, processed["tools"])
	assert.NotNil(t, processed["tasks"])
	assert.NotNil(t, processed["agents"])
}

// TestSprigFunctions tests the integration of Sprig functions
func TestSprigFunctions(t *testing.T) {
	engine := NewEngine(FormatYAML)

	// Test contains function
	result, err := engine.RenderString("{{ contains \"world\" \"hello world\" }}", nil)
	assert.NoError(t, err)
	assert.Equal(t, "true", result)

	// Test hasPrefix function
	result, err = engine.RenderString("{{ hasPrefix \"hello\" \"hello world\" }}", nil)
	assert.NoError(t, err)
	assert.Equal(t, "true", result)

	// Test hasSuffix function
	result, err = engine.RenderString("{{ hasSuffix \"world\" \"hello world\" }}", nil)
	assert.NoError(t, err)
	assert.Equal(t, "true", result)

	// Test regexMatch function
	result, err = engine.RenderString("{{ regexMatch \"[a-z]+\" \"hello\" }}", nil)
	assert.NoError(t, err)
	assert.Equal(t, "true", result)

	// Test toString function with a map
	ctx := map[string]any{
		"myMap": map[string]any{
			"key":    "value",
			"number": 123,
		},
	}
	result, err = engine.RenderString("{{ .myMap | toString }}", ctx)
	assert.NoError(t, err)
	assert.Contains(t, result, "key:value")
	assert.Contains(t, result, "number:123")

	// Test toJson function with a map
	result, err = engine.RenderString("{{ .myMap | toJson }}", ctx)
	assert.NoError(t, err)
	expectedJSON := `{"key":"value","number":123}`
	assert.JSONEq(t, expectedJSON, result)
}

func TestMissingKeyError(t *testing.T) {
	engine := NewEngine(FormatYAML)

	t.Run("Should return error for missing key", func(t *testing.T) {
		// This should now fail instead of rendering "<no value>"
		_, err := engine.RenderString("{{ .nonexistent.field }}", map[string]any{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nonexistent")
	})

	t.Run("Should return error for misspelled key", func(t *testing.T) {
		context := map[string]any{
			"user": map[string]any{
				"name": "John",
			},
		}
		// Common typo: "usr" instead of "user"
		_, err := engine.RenderString("{{ .usr.name }}", context)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "usr")
	})

	t.Run("Should return error for accessing field on non-existent object", func(t *testing.T) {
		context := map[string]any{
			"user": map[string]any{
				"name": "John",
			},
		}
		// "profile" doesn't exist on user
		_, err := engine.RenderString("{{ .user.profile.age }}", context)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "profile")
	})

	t.Run("Should work with default filter for missing keys", func(t *testing.T) {
		context := map[string]any{
			"user": map[string]any{
				"name": "John",
			},
		}
		// With missingkey=error, even default filter won't work for missing keys
		// This is the expected behavior - you need to provide the key or handle it differently
		_, err := engine.RenderString("{{ .user.age | default \"25\" }}", context)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "age")
	})

	t.Run("Should work with default filter when root key is missing", func(t *testing.T) {
		context := map[string]any{
			"config": map[string]any{
				"timeout": 30,
			},
		}
		// This approach works - use hasKey to check safely
		result, err := engine.RenderString(
			"{{ if hasKey .config \"missing\" }}{{ .config.missing }}{{ else }}default-value{{ end }}",
			context,
		)
		assert.NoError(t, err)
		assert.Equal(t, "default-value", result)
	})

	t.Run("Should work when key exists", func(t *testing.T) {
		context := map[string]any{
			"user": map[string]any{
				"name": "John",
			},
		}
		result, err := engine.RenderString("{{ .user.name }}", context)
		assert.NoError(t, err)
		assert.Equal(t, "John", result)
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
