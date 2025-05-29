package ref

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestConfig demonstrates the intended usage pattern with WithRef composition and Node field
type TestConfig struct {
	WithRef
	Ref     Node   `json:"$ref" yaml:"$ref"`
	Name    string `json:"name" yaml:"name"`
	Enabled bool   `json:"enabled" yaml:"enabled"`
}

// -----------------------------------------------------------------------------
// End-to-End Tests - Complete YAML to struct to resolution workflow
// -----------------------------------------------------------------------------

func TestNode_EndToEnd(t *testing.T) {
	fixturesDir, _ := setupRefTest(t)
	fixturePath := filepath.Join(fixturesDir, "node_test.yaml")
	fixtureDoc := loadYAMLFile(t, fixturePath)
	fixtureData, err := fixtureDoc.Get("")
	require.NoError(t, err)

	t.Run("Should unmarshal fixture YAML with string ref and resolve completely", func(t *testing.T) {
		// Load the actual fixture data
		rawFixtureData, ok := fixtureData.(map[string]any)
		require.True(t, ok)

		// Get the test_config_string from fixture
		testConfigData := rawFixtureData["test_config_string"]
		configYAML, err := yaml.Marshal(testConfigData)
		require.NoError(t, err)

		// This is the complete e2e flow: fixture YAML → struct → resolution
		var config TestConfig
		config.SetRefMetadata(fixturePath, fixturesDir)
		err = yaml.Unmarshal(configYAML, &config)
		require.NoError(t, err)

		// Verify the struct was populated correctly from fixture
		assert.Equal(t, "String Ref Config", config.Name)
		assert.True(t, config.Enabled)
		assert.False(t, config.Ref.IsEmpty())
		assert.Equal(t, TypeProperty, config.Ref.InnerRef().Type)
		assert.Equal(t, `schemas.#(id=="user")`, config.Ref.InnerRef().Path)

		// Now test the actual resolution using WithRef composition
		ctx := context.Background()
		result, err := config.ResolveRef(ctx, &config.Ref, fixtureData)
		require.NoError(t, err)

		// Verify we get the actual resolved schema from fixture
		schema, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "user", schema["id"])
		assert.Equal(t, "object", schema["type"])

		// Test round-trip marshaling preserves everything
		marshaled, err := yaml.Marshal(config)
		require.NoError(t, err)

		var roundTrip TestConfig
		err = yaml.Unmarshal(marshaled, &roundTrip)
		require.NoError(t, err)
		assert.Equal(t, config.Name, roundTrip.Name)
		assert.Equal(t, config.Enabled, roundTrip.Enabled)
		assert.Equal(t, config.Ref.String(), roundTrip.Ref.String())
	})

	t.Run("Should unmarshal fixture YAML with object ref", func(t *testing.T) {
		// Load the actual fixture data
		rawFixtureData, ok := fixtureData.(map[string]any)
		require.True(t, ok)

		// Get the test_config_object from fixture
		testConfigData := rawFixtureData["test_config_object"]
		configYAML, err := yaml.Marshal(testConfigData)
		require.NoError(t, err)

		var config TestConfig
		config.SetRefMetadata(fixturePath, fixturesDir)
		err = yaml.Unmarshal(configYAML, &config)
		require.NoError(t, err)

		// Verify object form was parsed correctly from fixture
		assert.Equal(t, "Object Ref Config", config.Name)
		assert.False(t, config.Enabled)
		assert.Equal(t, TypeProperty, config.Ref.InnerRef().Type)
		assert.Equal(t, ModeReplace, config.Ref.InnerRef().Mode)
		assert.Equal(t, `schemas.#(id=="config")`, config.Ref.InnerRef().Path)

		// Resolve the reference
		ctx := context.Background()
		refResult, err := config.ResolveRef(ctx, &config.Ref, fixtureData)
		require.NoError(t, err)

		// Verify we get the config schema from fixture
		schema, ok := refResult.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "config", schema["id"])
		assert.Equal(t, "object", schema["type"])
	})

	t.Run("Should handle merge mode from fixture", func(t *testing.T) {
		// Load the actual fixture data
		rawFixtureData, ok := fixtureData.(map[string]any)
		require.True(t, ok)

		// Get the test_config_merge from fixture
		testConfigData := rawFixtureData["test_config_merge"]
		configYAML, err := yaml.Marshal(testConfigData)
		require.NoError(t, err)

		var config TestConfig
		config.SetRefMetadata(fixturePath, fixturesDir)
		err = yaml.Unmarshal(configYAML, &config)
		require.NoError(t, err)

		// Verify merge config was parsed correctly from fixture
		assert.Equal(t, "Merge Ref Config", config.Name)
		assert.True(t, config.Enabled)
		assert.Equal(t, TypeProperty, config.Ref.InnerRef().Type)
		assert.Equal(t, ModeMerge, config.Ref.InnerRef().Mode)

		// Resolve the reference
		ctx := context.Background()
		refResult, err := config.ResolveRef(ctx, &config.Ref, fixtureData)
		require.NoError(t, err)

		// Test merge operation with inline data
		inlineData := map[string]any{
			"type":     "enhanced", // should override
			"newField": "added",
		}

		merged, err := config.MergeRefValue(&config.Ref, refResult, inlineData)
		require.NoError(t, err)

		mergedSchema, ok := merged.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "user", mergedSchema["id"])        // from ref
		assert.Equal(t, "enhanced", mergedSchema["type"])  // inline wins
		assert.Equal(t, "added", mergedSchema["newField"]) // inline only
	})

	t.Run("Should handle file reference e2e workflow", func(t *testing.T) {
		yamlContent := `
$ref: ./external.yaml::external_schemas.#(id=="user_input")
name: "External Config"
enabled: true
`
		var config TestConfig
		config.SetRefMetadata(fixturePath, fixturesDir)
		err := yaml.Unmarshal([]byte(yamlContent), &config)
		require.NoError(t, err)

		// Verify file reference was parsed
		assert.Equal(t, TypeFile, config.Ref.InnerRef().Type)
		assert.Equal(t, "./external.yaml", config.Ref.InnerRef().File)

		// Resolve the external file reference
		ctx := context.Background()
		result, err := config.ResolveRef(ctx, &config.Ref, fixtureData)
		require.NoError(t, err)

		// Verify external schema was loaded
		schema, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "user_input", schema["id"])

		// Verify properties from external file
		properties, ok := schema["properties"].(map[string]any)
		require.True(t, ok)
		assert.Contains(t, properties, "name")
		assert.Contains(t, properties, "email")
	})

	t.Run("Should handle global reference e2e workflow", func(t *testing.T) {
		yamlContent := `
$ref: $global::global_providers.#(id=="groq_llama")
name: "Global Config"
enabled: true
`
		var config TestConfig
		config.SetRefMetadata(fixturePath, fixturesDir)
		err := yaml.Unmarshal([]byte(yamlContent), &config)
		require.NoError(t, err)

		// Verify global reference was parsed
		assert.Equal(t, TypeGlobal, config.Ref.InnerRef().Type)

		// Resolve the global reference
		ctx := context.Background()
		result, err := config.ResolveRef(ctx, &config.Ref, fixtureData)
		require.NoError(t, err)

		// Verify global provider was loaded
		provider, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "groq_llama", provider["id"])
		assert.Equal(t, "groq", provider["provider"])
		assert.Contains(t, provider, "model")
	})

	t.Run("Should handle array of configs with different reference types", func(t *testing.T) {
		yamlContent := `
configs:
  - $ref: schemas.#(id=="user")
    name: "User Schema"
    enabled: true

  - $ref:
      type: file
      file: ./external.yaml
      path: external_config.database
      mode: merge
    name: "DB Config"
    enabled: false

  - $ref: $global::global_providers.#(id=="openai_gpt4")
    name: "AI Provider"
    enabled: true
`
		var doc struct {
			Configs []TestConfig `yaml:"configs"`
		}
		err := yaml.Unmarshal([]byte(yamlContent), &doc)
		require.NoError(t, err)

		// Verify all configs were parsed
		require.Len(t, doc.Configs, 3)

		ctx := context.Background()

		// Test first config (property reference)
		config1 := doc.Configs[0]
		config1.SetRefMetadata(fixturePath, fixturesDir)
		assert.Equal(t, "User Schema", config1.Name)
		assert.Equal(t, TypeProperty, config1.Ref.InnerRef().Type)

		result1, err := config1.ResolveRef(ctx, &config1.Ref, fixtureData)
		require.NoError(t, err)
		schema1 := result1.(map[string]any)
		assert.Equal(t, "user", schema1["id"])

		// Test second config (file reference with merge)
		config2 := doc.Configs[1]
		config2.SetRefMetadata(fixturePath, fixturesDir)
		assert.Equal(t, "DB Config", config2.Name)
		assert.Equal(t, TypeFile, config2.Ref.InnerRef().Type)
		assert.Equal(t, ModeMerge, config2.Ref.InnerRef().Mode)

		result2, err := config2.ResolveRef(ctx, &config2.Ref, fixtureData)
		require.NoError(t, err)
		dbConfig := result2.(map[string]any)
		assert.Contains(t, dbConfig, "host")
		assert.Contains(t, dbConfig, "port")

		// Test third config (global reference)
		config3 := doc.Configs[2]
		config3.SetRefMetadata(fixturePath, fixturesDir)
		assert.Equal(t, "AI Provider", config3.Name)
		assert.Equal(t, TypeGlobal, config3.Ref.InnerRef().Type)

		result3, err := config3.ResolveRef(ctx, &config3.Ref, fixtureData)
		require.NoError(t, err)
		provider := result3.(map[string]any)
		assert.Equal(t, "openai_gpt4", provider["id"])
	})
}

// -----------------------------------------------------------------------------
// Essential Edge Cases
// -----------------------------------------------------------------------------

func TestNode_EdgeCases(t *testing.T) {
	t.Run("Should handle empty nodes gracefully", func(t *testing.T) {
		var emptyNode Node
		assert.True(t, emptyNode.IsEmpty())

		config := TestConfig{Ref: emptyNode}
		// Should return nil for empty refs
		ctx := context.Background()
		result, err := config.ResolveRef(ctx, &config.Ref, nil)
		require.NoError(t, err)
		assert.Nil(t, result)

		// Should pass through inline values for empty refs
		mergeResult, err := config.MergeRefValue(&config.Ref, nil, map[string]any{"test": "value"})
		require.NoError(t, err)
		assert.Equal(t, map[string]any{"test": "value"}, mergeResult)
	})

	t.Run("Should handle null marshaling", func(t *testing.T) {
		var emptyNode Node

		jsonData, err := json.Marshal(emptyNode)
		require.NoError(t, err)
		assert.Equal(t, "null", string(jsonData))

		var nullNode Node
		err = json.Unmarshal([]byte("null"), &nullNode)
		require.NoError(t, err)
		assert.True(t, nullNode.IsEmpty())
	})
}
