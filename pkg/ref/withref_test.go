package ref

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestConfig demonstrates the usage pattern with is_ref tag
type TestConfig struct {
	WithRef
	Ref     any    `json:"$ref" yaml:"$ref" is_ref:"true"`
	Name    string `json:"name" yaml:"name"`
	Enabled bool   `json:"enabled" yaml:"enabled"`
}

// TestMultiRefConfig has multiple reference fields
type TestMultiRefConfig struct {
	WithRef
	BaseRef   any    `json:"base_ref" yaml:"base_ref" is_ref:"true"`
	SchemaRef any    `json:"schema_ref" yaml:"schema_ref" is_ref:"true"`
	Name      string `json:"name" yaml:"name"`
	Version   string `json:"version" yaml:"version"`
}

// -----------------------------------------------------------------------------
// Test Helpers
// -----------------------------------------------------------------------------

func setupTest(t *testing.T) (string, string) {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok, "failed to get current file path")
	testDir := filepath.Dir(filename)
	fixturesDir := filepath.Join(testDir, "fixtures")
	mainYAML := filepath.Join(fixturesDir, "main.yaml")
	_, err := os.Stat(mainYAML)
	require.NoError(t, err, "fixtures/main.yaml not found")
	return fixturesDir, mainYAML
}

// -----------------------------------------------------------------------------
// Core Reference Resolution Tests
// -----------------------------------------------------------------------------

func TestWithRef_ResolveReferences(t *testing.T) {
	fixturesDir, mainPath := setupTest(t)
	mainDoc := loadYAMLFile(t, mainPath)
	mainData, err := mainDoc.Get("")
	require.NoError(t, err)

	t.Run("Should resolve string reference", func(t *testing.T) {
		yamlContent := `
$ref: schemas.#(id=="city_input")
name: "String Ref Config"
enabled: true
`
		var config TestConfig
		config.SetRefMetadata(mainPath, fixturesDir)
		err := yaml.Unmarshal([]byte(yamlContent), &config)
		require.NoError(t, err)

		ctx := context.Background()
		err = config.ResolveReferences(ctx, &config, mainData)
		require.NoError(t, err)

		schema, ok := config.Ref.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "city_input", schema["id"])
		assert.Equal(t, "object", schema["type"])
		assert.Equal(t, "String Ref Config", config.Name)
		assert.True(t, config.Enabled)
	})

	t.Run("Should resolve object reference", func(t *testing.T) {
		yamlContent := `
$ref:
  type: property
  path: schemas.#(id=="weather_output")
  mode: replace
name: "Object Ref Config"
enabled: false
`
		var config TestConfig
		config.SetRefMetadata(mainPath, fixturesDir)
		err := yaml.Unmarshal([]byte(yamlContent), &config)
		require.NoError(t, err)

		ctx := context.Background()
		err = config.ResolveReferences(ctx, &config, mainData)
		require.NoError(t, err)

		schema, ok := config.Ref.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "weather_output", schema["id"])
		assert.Equal(t, "object", schema["type"])
	})

	t.Run("Should resolve file reference", func(t *testing.T) {
		yamlContent := `
$ref: ./external.yaml::external_schemas.#(id=="user_input")
name: "External Config"
`
		var config TestConfig
		config.SetRefMetadata(mainPath, fixturesDir)
		err := yaml.Unmarshal([]byte(yamlContent), &config)
		require.NoError(t, err)

		ctx := context.Background()
		err = config.ResolveReferences(ctx, &config, mainData)
		require.NoError(t, err)

		schema, ok := config.Ref.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "user_input", schema["id"])
		properties, ok := schema["properties"].(map[string]any)
		require.True(t, ok)
		assert.Contains(t, properties, "name")
		assert.Contains(t, properties, "email")
	})

	t.Run("Should resolve global reference", func(t *testing.T) {
		yamlContent := `
$ref: $global::global_providers.#(id=="groq_llama")
name: "Global Config"
`
		var config TestConfig
		config.SetRefMetadata(mainPath, fixturesDir)
		err := yaml.Unmarshal([]byte(yamlContent), &config)
		require.NoError(t, err)

		ctx := context.Background()
		err = config.ResolveReferences(ctx, &config, mainData)
		require.NoError(t, err)

		provider, ok := config.Ref.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "groq_llama", provider["id"])
		assert.Equal(t, "groq", provider["provider"])
	})

	t.Run("Should resolve multiple reference fields", func(t *testing.T) {
		yamlContent := `
base_ref: schemas.#(id=="city_input")
schema_ref: ./external.yaml::external_config.database
name: "Multi Ref Config"
version: "1.0"
`
		var config TestMultiRefConfig
		config.SetRefMetadata(mainPath, fixturesDir)
		err := yaml.Unmarshal([]byte(yamlContent), &config)
		require.NoError(t, err)

		ctx := context.Background()
		err = config.ResolveReferences(ctx, &config, mainData)
		require.NoError(t, err)

		baseSchema, ok := config.BaseRef.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "city_input", baseSchema["id"])

		dbConfig, ok := config.SchemaRef.(map[string]any)
		require.True(t, ok)
		assert.Contains(t, dbConfig, "host")
		assert.Contains(t, dbConfig, "port")

		assert.Equal(t, "Multi Ref Config", config.Name)
		assert.Equal(t, "1.0", config.Version)
	})
}

// -----------------------------------------------------------------------------
// Merge Functionality Tests
// -----------------------------------------------------------------------------

func TestWithRef_MergeReferences(t *testing.T) {
	fixturesDir, mainPath := setupTest(t)
	mainDoc := loadYAMLFile(t, mainPath)
	mainData, err := mainDoc.Get("")
	require.NoError(t, err)

	t.Run("Should merge references into struct", func(t *testing.T) {
		yamlContent := `
$ref: schemas.#(id=="city_input")
name: "Merge Test Config"
enabled: true
`
		var config TestConfig
		config.SetRefMetadata(mainPath, fixturesDir)
		err := yaml.Unmarshal([]byte(yamlContent), &config)
		require.NoError(t, err)

		ctx := context.Background()
		err = config.ResolveAndMergeReferences(ctx, &config, mainData, ModeMerge)
		require.NoError(t, err)

		// Original fields preserved, reference field unchanged (excluded from merge)
		assert.Equal(t, "Merge Test Config", config.Name)
		assert.True(t, config.Enabled)
		assert.Equal(t, `schemas.#(id=="city_input")`, config.Ref)
	})
}

// -----------------------------------------------------------------------------
// Map Resolution Tests
// -----------------------------------------------------------------------------

func TestWithRef_MapResolution(t *testing.T) {
	fixturesDir, mainPath := setupTest(t)
	mainDoc := loadYAMLFile(t, mainPath)
	mainData, err := mainDoc.Get("")
	require.NoError(t, err)

	t.Run("Should resolve nested map references", func(t *testing.T) {
		config := TestConfig{}
		config.SetRefMetadata(mainPath, fixturesDir)
		data := map[string]any{
			"config": map[string]any{
				"$ref":  "schemas.#(id==\"city_input\")",
				"extra": "value",
			},
			"array": []any{
				map[string]any{
					"$ref":      "schemas.#(id==\"weather_output\")",
					"new_field": "added",
				},
			},
		}

		ctx := context.Background()
		result, err := config.ResolveMapReference(ctx, data, mainData)
		require.NoError(t, err)

		configMap, ok := result["config"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "city_input", configMap["id"])
		assert.Equal(t, "value", configMap["extra"])

		array, ok := result["array"].([]any)
		require.True(t, ok)
		arrayMap, ok := array[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "weather_output", arrayMap["id"])
		assert.Equal(t, "added", arrayMap["new_field"])
	})

	t.Run("Should handle map without references", func(t *testing.T) {
		config := TestConfig{}
		config.SetRefMetadata(mainPath, fixturesDir)
		data := map[string]any{
			"name":  "test",
			"array": []any{1, 2, 3},
		}

		ctx := context.Background()
		result, err := config.ResolveMapReference(ctx, data, mainData)
		require.NoError(t, err)
		assert.Equal(t, data, result)
	})

	t.Run("Should resolve chained file references", func(t *testing.T) {
		config := TestConfig{}
		chainedDir := filepath.Join(fixturesDir, "chained")
		file1Path := filepath.Join(chainedDir, "superup", "upup", "up", "doc", "file1.yaml")
		file1Doc := loadYAMLFile(t, file1Path)
		file1Data, err := file1Doc.Get("")
		config.SetRefMetadata(file1Path, chainedDir)
		require.NoError(t, err)

		data := map[string]any{
			"$ref": "../file2.yaml::data",
		}

		ctx := context.Background()
		merged, err := config.ResolveMapReference(ctx, data, file1Data)
		require.NoError(t, err)

		assert.Equal(t, "file4_value", merged["value"])
		assert.NotContains(t, merged, "$ref")
	})
}

// -----------------------------------------------------------------------------
// Edge Cases Tests
// -----------------------------------------------------------------------------

func TestWithRef_EdgeCases(t *testing.T) {
	fixturesDir, mainPath := setupTest(t)
	mainDoc := loadYAMLFile(t, mainPath)
	mainData, err := mainDoc.Get("")
	require.NoError(t, err)

	t.Run("Should handle empty reference field", func(t *testing.T) {
		var config TestConfig
		config.SetRefMetadata(mainPath, fixturesDir)
		config.Name = "Test"
		config.Enabled = true

		ctx := context.Background()
		err := config.ResolveReferences(ctx, &config, mainData)
		require.NoError(t, err)

		assert.Equal(t, "Test", config.Name)
		assert.True(t, config.Enabled)
		assert.Nil(t, config.Ref)
	})

	t.Run("Should handle struct without is_ref fields", func(t *testing.T) {
		type NoRefConfig struct {
			WithRef
			Name    string `json:"name" yaml:"name"`
			Enabled bool   `json:"enabled" yaml:"enabled"`
		}

		var config NoRefConfig
		config.SetRefMetadata(mainPath, fixturesDir)
		config.Name = "Test"
		config.Enabled = true

		ctx := context.Background()
		err := config.ResolveReferences(ctx, &config, mainData)
		require.NoError(t, err)

		assert.Equal(t, "Test", config.Name)
		assert.True(t, config.Enabled)
	})

	t.Run("Should handle invalid reference gracefully", func(t *testing.T) {
		var config TestConfig
		config.SetRefMetadata(mainPath, fixturesDir)
		config.Ref = "nonexistent.path"

		ctx := context.Background()
		err := config.ResolveReferences(ctx, &config, mainData)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("Should skip non-string and non-map reference values", func(t *testing.T) {
		var config TestConfig
		config.SetRefMetadata(mainPath, fixturesDir)
		config.Ref = 123 // invalid type
		config.Name = "Test"

		ctx := context.Background()
		err := config.ResolveReferences(ctx, &config, mainData)
		require.NoError(t, err)

		assert.Equal(t, 123, config.Ref) // unchanged
		assert.Equal(t, "Test", config.Name)
	})
}
