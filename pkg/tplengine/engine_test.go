package tplengine

import (
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

	result, err := engine.ProcessString(yamlStr, ctx)
	assert.NoError(t, err)
	assert.NotNil(t, result.YAML)

	// Convert to map for easier assertions
	var yamlMap map[string]any
	yamlBytes, err := yaml.Marshal(result.YAML)
	assert.NoError(t, err)
	err = yaml.Unmarshal(yamlBytes, &yamlMap)
	assert.NoError(t, err)

	assert.Equal(t, "John", yamlMap["name"])
	assert.Equal(t, 30, yamlMap["age"])

	// JSON format
	engine = NewEngine(FormatJSON)

	// Simple JSON with template
	jsonStr := `{
  "name": "{{ .user.name }}",
  "age": {{ .user.age }}
}`

	result, err = engine.ProcessString(jsonStr, ctx)
	assert.NoError(t, err)
	assert.NotNil(t, result.JSON)

	jsonMap, ok := result.JSON.(map[string]any)
	assert.True(t, ok)
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

	result, err := engine.ProcessFile(yamlPath, ctx)
	assert.NoError(t, err)
	assert.NotNil(t, result.YAML)

	// Convert to map for easier assertions
	var yamlMap map[string]any
	yamlBytes, err := yaml.Marshal(result.YAML)
	assert.NoError(t, err)
	err = yaml.Unmarshal(yamlBytes, &yamlMap)
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
		result, err := engine.RenderString("{{ if hasKey .config \"missing\" }}{{ .config.missing }}{{ else }}default-value{{ end }}", context)
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
