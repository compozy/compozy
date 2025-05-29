package ref

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// WithRefTestConfig demonstrates the intended usage pattern with WithRef composition
type WithRefTestConfig struct {
	WithRef
	Ref     Node   `json:"$ref" yaml:"$ref"`
	Name    string `json:"name" yaml:"name"`
	Enabled bool   `json:"enabled" yaml:"enabled"`
}

// -----------------------------------------------------------------------------
// Test Helpers
// -----------------------------------------------------------------------------

// setupWithRefTest sets up the test environment by returning the fixtures directory and main YAML path.
func setupWithRefTest(t *testing.T) (string, string) {
	t.Helper()

	// Get the directory of this test file
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok, "failed to get current file path")

	testDir := filepath.Dir(filename)
	fixturesDir := filepath.Join(testDir, "fixtures")
	mainYAML := filepath.Join(fixturesDir, "main.yaml")

	// Verify fixtures exist
	_, err := os.Stat(mainYAML)
	require.NoError(t, err, "fixtures/main.yaml not found")

	return fixturesDir, mainYAML
}

// -----------------------------------------------------------------------------
// WithRef Resolve* Tests
// -----------------------------------------------------------------------------

func TestWithRef_ResolveFunctions(t *testing.T) {
	fixturesDir, mainPath := setupWithRefTest(t)
	mainDoc := loadYAMLFile(t, mainPath)
	mainData, err := mainDoc.Get("")
	require.NoError(t, err)

	// Test configuration struct for ResolveAndMergeNode
	type TestStruct struct {
		Name    string `yaml:"name"`
		Value   any    `yaml:"value"`
		Enabled bool   `yaml:"enabled"`
	}

	t.Run("Should resolve property reference", func(t *testing.T) {
		config := WithRefTestConfig{}
		config.SetRefMetadata(mainPath, fixturesDir)
		node, err := NewNodeFromString("schemas.#(id==\"city_input\")")
		require.NoError(t, err)
		config.Ref = *node

		ctx := context.Background()
		result, err := config.ResolveRef(ctx, &config.Ref, mainData)
		require.NoError(t, err)

		schema, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "city_input", schema["id"])
		assert.Equal(t, "object", schema["type"])
	})

	t.Run("Should resolve file reference", func(t *testing.T) {
		config := WithRefTestConfig{}
		config.SetRefMetadata(mainPath, fixturesDir)
		node, err := NewNodeFromString("./external.yaml::external_schemas.#(id==\"user_input\")")
		require.NoError(t, err)
		config.Ref = *node

		ctx := context.Background()
		result, err := config.ResolveRef(ctx, &config.Ref, mainData)
		require.NoError(t, err)

		schema, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "user_input", schema["id"])
		assert.Equal(t, "object", schema["type"])
	})

	t.Run("Should resolve global reference", func(t *testing.T) {
		config := WithRefTestConfig{}
		config.SetRefMetadata(mainPath, fixturesDir)
		node, err := NewNodeFromString("$global::global_providers.#(id==\"groq_llama\")")
		require.NoError(t, err)
		config.Ref = *node

		ctx := context.Background()
		result, err := config.ResolveRef(ctx, &config.Ref, mainData)
		require.NoError(t, err)

		provider, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "groq_llama", provider["id"])
		assert.Equal(t, "groq", provider["provider"])
	})

	t.Run("Should handle empty node in ResolveRef", func(t *testing.T) {
		config := WithRefTestConfig{}
		config.SetRefMetadata(mainPath, fixturesDir)
		var node Node
		config.Ref = node

		ctx := context.Background()
		result, err := config.ResolveRef(ctx, &config.Ref, mainData)
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("Should handle invalid path in ResolveRef", func(t *testing.T) {
		config := WithRefTestConfig{}
		config.SetRefMetadata(mainPath, fixturesDir)
		node, err := NewNodeFromString("nonexistent.path")
		require.NoError(t, err)
		config.Ref = *node

		ctx := context.Background()
		_, err = config.ResolveRef(ctx, &config.Ref, mainData)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("Should resolve and merge with merge mode", func(t *testing.T) {
		config := WithRefTestConfig{}
		config.SetRefMetadata(mainPath, fixturesDir)
		node, err := NewNodeFromString("schemas.#(id==\"city_input\")")
		require.NoError(t, err)
		config.Ref = *node

		inlineValue := map[string]any{
			"extra_field": "inline_value",
			"type":        "modified_object",
		}

		ctx := context.Background()
		result, err := config.ResolveAndMergeRef(ctx, &config.Ref, inlineValue, mainData)
		require.NoError(t, err)

		merged, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "city_input", merged["id"])
		assert.Equal(t, "modified_object", merged["type"]) // inline wins
		assert.Equal(t, "inline_value", merged["extra_field"])
	})

	t.Run("Should resolve and merge with replace mode", func(t *testing.T) {
		config := WithRefTestConfig{}
		config.SetRefMetadata(mainPath, fixturesDir)
		node, err := NewNodeFromString("schemas.#(id==\"city_input\")!replace")
		require.NoError(t, err)
		config.Ref = *node

		inlineValue := map[string]any{
			"extra_field": "inline_value",
		}

		ctx := context.Background()
		result, err := config.ResolveAndMergeRef(ctx, &config.Ref, inlineValue, mainData)
		require.NoError(t, err)

		merged, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "city_input", merged["id"])
		assert.Equal(t, "object", merged["type"])
		assert.NotContains(t, merged, "extra_field") // inline ignored
	})

	t.Run("Should resolve and merge with append mode", func(t *testing.T) {
		config := WithRefTestConfig{}
		config.SetRefMetadata(mainPath, fixturesDir)
		node, err := NewNodeFromString("test_arrays.base_items!append")
		require.NoError(t, err)
		config.Ref = *node

		inlineValue := []any{"inline1", "inline2"}

		ctx := context.Background()
		result, err := config.ResolveAndMergeRef(ctx, &config.Ref, inlineValue, mainData)
		require.NoError(t, err)

		merged, ok := result.([]any)
		require.True(t, ok)
		assert.Equal(t, []any{"inline1", "inline2", "item1", "item2"}, merged)
	})

	t.Run("Should handle empty node in ResolveAndMergeRef", func(t *testing.T) {
		config := WithRefTestConfig{}
		config.SetRefMetadata(mainPath, fixturesDir)
		var node Node
		config.Ref = node
		inlineValue := map[string]any{"test": "value"}

		ctx := context.Background()
		result, err := config.ResolveAndMergeRef(ctx, &config.Ref, inlineValue, mainData)
		require.NoError(t, err)
		assert.Equal(t, inlineValue, result)
	})

	t.Run("Should resolve and merge node with property reference", func(t *testing.T) {
		config := WithRefTestConfig{}
		config.SetRefMetadata(mainPath, fixturesDir)
		node, err := NewNodeFromString("schemas.#(id==\"city_input\")")
		require.NoError(t, err)
		config.Ref = *node

		// Create a struct that represents a schema config
		type SchemaConfig struct {
			ID         string `yaml:"id"`
			Type       string `yaml:"type"`
			ExtraField string `yaml:"extra_field"`
		}

		target := &SchemaConfig{
			ExtraField: "inline",
		}

		ctx := context.Background()
		err = config.ResolveAndMergeNode(ctx, &config.Ref, target, mainData, ModeMerge)
		require.NoError(t, err)

		// Verify that the resolved reference data was merged into the struct
		assert.Equal(t, "city_input", target.ID)
		assert.Equal(t, "object", target.Type)
		assert.Equal(t, "inline", target.ExtraField) // inline field preserved
	})

	t.Run("Should handle nil target in ResolveAndMergeNode", func(t *testing.T) {
		config := WithRefTestConfig{}
		config.SetRefMetadata(mainPath, fixturesDir)
		node, err := NewNodeFromString("schemas.#(id==\"city_input\")")
		require.NoError(t, err)
		config.Ref = *node

		ctx := context.Background()
		err = config.ResolveAndMergeNode(ctx, &config.Ref, nil, mainData, ModeMerge)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "target must not be nil")
	})

	t.Run("Should handle empty node in ResolveAndMergeNode", func(t *testing.T) {
		config := WithRefTestConfig{}
		config.SetRefMetadata(mainPath, fixturesDir)
		var node Node
		config.Ref = node
		target := &TestStruct{Name: "Test"}

		ctx := context.Background()
		err := config.ResolveAndMergeNode(ctx, &config.Ref, target, mainData, ModeMerge)
		require.NoError(t, err)
		assert.Equal(t, "Test", target.Name)
	})

	t.Run("Should resolve map with nested references", func(t *testing.T) {
		config := WithRefTestConfig{}
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

		// Verify config section
		configMap, ok := result["config"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "city_input", configMap["id"])
		assert.Equal(t, "value", configMap["extra"])

		// Verify array section
		array, ok := result["array"].([]any)
		require.True(t, ok)
		arrayMap, ok := array[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "weather_output", arrayMap["id"])
		assert.Equal(t, "added", arrayMap["new_field"])
	})

	t.Run("Should handle invalid reference in ResolveMapReference", func(t *testing.T) {
		config := WithRefTestConfig{}
		config.SetRefMetadata(mainPath, fixturesDir)
		data := map[string]any{
			"$ref": "nonexistent.path",
		}

		ctx := context.Background()
		_, err := config.ResolveMapReference(ctx, data, mainData)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("Should handle map with no references in ResolveMapReference", func(t *testing.T) {
		config := WithRefTestConfig{}
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

	t.Run("Should resolve map with chained file references", func(t *testing.T) {
		config := WithRefTestConfig{}
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
		assert.NotContains(t, merged, "$ref") // Ensure all references resolved
	})
}
