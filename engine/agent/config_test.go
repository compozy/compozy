package agent

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTest(t *testing.T, agentFile string) (*core.PathCWD, string) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	cwd, dstPath := utils.SetupTest(t, filename)
	dstPath = filepath.Join(dstPath, agentFile)
	return cwd, dstPath
}

func Test_LoadAgent(t *testing.T) {
	t.Run("Should load basic agent configuration correctly", func(t *testing.T) {
		cwd, dstPath := setupTest(t, "basic_agent.yaml")
		config, err := Load(cwd, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		require.NotNil(t, config.ID)
		require.NotNil(t, config.Config)
		require.NotNil(t, config.Config.Params.Temperature)
		require.NotNil(t, config.Config.Params.MaxTokens)

		assert.Equal(t, "code-assistant", config.ID)
		assert.Equal(t, core.ProviderAnthropic, config.Config.Provider)
		assert.Equal(t, "claude-3-opus-20240229", config.Config.Model)
		assert.InDelta(t, float32(0.7), config.Config.Params.Temperature, 0.0001)
		assert.Equal(t, int32(4000), config.Config.Params.MaxTokens)

		require.Len(t, config.Actions, 1)
		action := config.Actions[0]
		assert.Equal(t, "review-code", action.ID)

		require.NotNil(t, action.InputSchema)
		schema := action.InputSchema
		compiledSchema, err := schema.Compile()
		require.NoError(t, err)
		assert.Equal(t, []string{"object"}, []string(compiledSchema.Type))
		require.NotNil(t, compiledSchema.Properties)
		assert.Contains(t, (*compiledSchema.Properties), "code")
		assert.Contains(t, (*compiledSchema.Properties), "language")
		assert.Contains(t, compiledSchema.Required, "code")

		require.NotNil(t, action.OutputSchema)
		outSchema := action.OutputSchema
		compiledOutSchema, err := outSchema.Compile()
		require.NoError(t, err)
		assert.Equal(t, []string{"object"}, []string(compiledOutSchema.Type))
		require.NotNil(t, compiledOutSchema.Properties)
		assert.Contains(t, (*compiledOutSchema.Properties), "feedback")
		assert.Contains(t, compiledOutSchema.Required, "feedback")

		// Get the feedback property from compiled schema
		feedbackProp := (*compiledOutSchema.Properties)["feedback"]
		require.NotNil(t, feedbackProp)
		assert.Equal(t, []string{"array"}, []string(feedbackProp.Type))

		// Check array items structure
		require.NotNil(t, feedbackProp.Items)
		itemSchema := feedbackProp.Items
		assert.Equal(t, []string{"object"}, []string(itemSchema.Type))
		require.NotNil(t, itemSchema.Properties)

		// Check that the required properties exist in items
		itemProps := (*itemSchema.Properties)
		assert.Contains(t, itemProps, "category")
		assert.Contains(t, itemProps, "description")
		assert.Contains(t, itemProps, "suggestion")
	})
}

func Test_AgentActionConfigValidation(t *testing.T) {
	actionCWD, err := core.CWDFromPath("/test/path")
	require.NoError(t, err)

	t.Run("Should validate action config with all required fields", func(t *testing.T) {
		config := &ActionConfig{
			ID:     "test-action",
			Prompt: "test prompt",
			CWD:    actionCWD,
		}
		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should return error when CWD is missing", func(t *testing.T) {
		config := &ActionConfig{
			ID:     "test-action",
			Prompt: "test prompt",
		}
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "current working directory is required for test-action")
	})

	t.Run("Should return error when parameters are invalid", func(t *testing.T) {
		config := &ActionConfig{
			ID:     "test-action",
			Prompt: "test prompt",
			CWD:    actionCWD,
			InputSchema: &schema.Schema{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type": "string",
					},
				},
				"required": []string{"name"},
			},
			With: &core.Input{
				"age": 42,
			},
		}
		err := config.ValidateInput(context.Background(), config.With)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Required property 'name' is missing")
	})
}

func Test_AgentConfigCWD(t *testing.T) {
	t.Run("Should set and get CWD correctly", func(t *testing.T) {
		config := &Config{}
		config.SetCWD("/test/path")
		assert.Equal(t, "/test/path", config.GetCWD().PathStr())
	})

	t.Run("Should set CWD for all actions", func(t *testing.T) {
		config := &Config{}
		action := &ActionConfig{
			ID:     "test-action",
			Prompt: "test prompt",
		}
		config.Actions = []*ActionConfig{action}
		config.SetCWD("/new/path")
		assert.Equal(t, "/new/path", action.GetCWD().PathStr())
	})
}

func Test_AgentConfigMerge(t *testing.T) {
	t.Run("Should merge configurations correctly", func(t *testing.T) {
		baseConfig := &Config{
			Env: &core.EnvMap{
				"KEY1": "value1",
			},
			With: &core.Input{},
		}

		otherConfig := &Config{
			Env: &core.EnvMap{
				"KEY2": "value2",
			},
			With: &core.Input{},
		}

		err := baseConfig.Merge(otherConfig)
		require.NoError(t, err)

		// Check that base config has both env variables
		assert.Equal(t, "value1", baseConfig.GetEnv().Prop("KEY1"))
		assert.Equal(t, "value2", baseConfig.GetEnv().Prop("KEY2"))
	})
}

func Test_AgentConfigValidation(t *testing.T) {
	agentID := "test-agent"
	agentCWD, err := core.CWDFromPath("/test/path")
	require.NoError(t, err)

	t.Run("Should validate config with all required fields", func(t *testing.T) {
		config := &Config{
			ID:           agentID,
			Config:       core.ProviderConfig{},
			Instructions: "test instructions",
			CWD:          agentCWD,
		}
		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should return error when CWD is missing", func(t *testing.T) {
		config := &Config{
			ID:           agentID,
			Config:       core.ProviderConfig{},
			Instructions: "test instructions",
		}
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "current working directory is required for test-agent")
	})
}

func TestActionConfig_DeepCopy(t *testing.T) {
	t.Run("Should create independent copy that doesn't share pointer fields", func(t *testing.T) {
		// Arrange
		originalInput := &core.Input{"key": "original"}
		originalSchema := &schema.Schema{"type": "object"}
		originalCWD := &core.PathCWD{Path: "/original/path"}

		original := &ActionConfig{
			ID:           "test-action",
			Prompt:       "test prompt",
			InputSchema:  originalSchema,
			OutputSchema: originalSchema, // intentionally same reference
			With:         originalInput,
			JSONMode:     true,
			CWD:          originalCWD,
		}

		// Act
		copied, err := original.Clone()
		assert.NoError(t, err)

		// Assert - verify basic fields are copied
		assert.Equal(t, original.ID, copied.ID)
		assert.Equal(t, original.Prompt, copied.Prompt)
		assert.Equal(t, original.JSONMode, copied.JSONMode)

		// Assert - verify pointer fields are different instances
		assert.NotSame(t, original.With, copied.With)
		assert.NotSame(t, original.InputSchema, copied.InputSchema)
		assert.NotSame(t, original.OutputSchema, copied.OutputSchema)
		assert.NotSame(t, original.CWD, copied.CWD)

		// Assert - verify content is the same
		assert.Equal(t, *original.With, *copied.With)
		assert.Equal(t, *original.InputSchema, *copied.InputSchema)
		assert.Equal(t, *original.OutputSchema, *copied.OutputSchema)
		assert.Equal(t, *original.CWD, *copied.CWD)

		// Assert - verify mutations don't affect original
		(*copied.With)["key"] = "modified"
		(*copied.InputSchema)["type"] = "string"
		copied.CWD.Path = "/modified/path"

		assert.Equal(t, "original", (*original.With)["key"])
		assert.Equal(t, "object", (*original.InputSchema)["type"])
		assert.Equal(t, "/original/path", original.CWD.Path)
	})

	t.Run("Should handle nil input gracefully", func(t *testing.T) {
		var original *ActionConfig
		copied, err := original.Clone()
		assert.Nil(t, copied)
		assert.NoError(t, err)
	})

	t.Run("Should handle action config with nil fields", func(t *testing.T) {
		original := &ActionConfig{
			ID:     "test",
			Prompt: "test prompt",
			// All pointer fields are nil
		}

		copied, err := original.Clone()
		assert.NoError(t, err)

		assert.Equal(t, original.ID, copied.ID)
		assert.Equal(t, original.Prompt, copied.Prompt)
		assert.Nil(t, copied.With)
		assert.Nil(t, copied.InputSchema)
		assert.Nil(t, copied.OutputSchema)
		assert.Nil(t, copied.CWD)
	})
}

func TestConfig_normalizeAndValidateMemoryConfig(t *testing.T) {
	agentCWD, _ := core.CWDFromPath("/test")
	validBaseConfig := func() *Config { // Helper to get a minimally valid config
		return &Config{ID: "test-agent", Config: core.ProviderConfig{}, Instructions: "do stuff", CWD: agentCWD}
	}

	t.Run("Level 1: memory string ID and memory_key", func(t *testing.T) {
		cfg := validBaseConfig()
		cfg.Memory = "mem-id-1"
		cfg.MemoryKey = "key-{{.workflow.id}}"
		err := cfg.NormalizeAndValidateMemoryConfig()
		require.NoError(t, err)
		refs := cfg.GetResolvedMemoryReferences()
		require.Len(t, refs, 1)
		assert.Equal(t, "mem-id-1", refs[0].ID)
		assert.Equal(t, "key-{{.workflow.id}}", refs[0].Key)
		assert.Equal(t, "read-write", refs[0].Mode)
	})

	t.Run("Level 1: missing memory_key", func(t *testing.T) {
		cfg := validBaseConfig()
		cfg.Memory = "mem-id-1"
		err := cfg.NormalizeAndValidateMemoryConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "'memory_key' is required for Level 1")
	})

	t.Run("Level 1: with memories field (invalid)", func(t *testing.T) {
		cfg := validBaseConfig()
		cfg.Memory = "mem-id-1"
		cfg.MemoryKey = "key"
		cfg.Memories = []string{"another-mem"}
		err := cfg.NormalizeAndValidateMemoryConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot use 'memories' field when 'memory' is a string ID (Level 1)")
	})

	t.Run("Level 2: memory:true, memories list, memory_key", func(t *testing.T) {
		cfg := validBaseConfig()
		cfg.Memory = true
		cfg.Memories = []any{"mem-id-1", "mem-id-2"} // YAML often unmarshals to []interface{}
		cfg.MemoryKey = "shared-key-{{.user.id}}"
		err := cfg.NormalizeAndValidateMemoryConfig()
		require.NoError(t, err)
		refs := cfg.GetResolvedMemoryReferences()
		require.Len(t, refs, 2)
		assert.Equal(t, "mem-id-1", refs[0].ID)
		assert.Equal(t, "shared-key-{{.user.id}}", refs[0].Key)
		assert.Equal(t, "read-write", refs[0].Mode)
		assert.Equal(t, "mem-id-2", refs[1].ID)
		assert.Equal(t, "shared-key-{{.user.id}}", refs[1].Key)
		assert.Equal(t, "read-write", refs[1].Mode)
	})

	t.Run("Level 2: memories as []string", func(t *testing.T) {
		cfg := validBaseConfig()
		cfg.Memory = true
		cfg.Memories = []string{"mem-id-1", "mem-id-2"}
		cfg.MemoryKey = "shared-key-{{.user.id}}"
		err := cfg.NormalizeAndValidateMemoryConfig()
		require.NoError(t, err)
		refs := cfg.GetResolvedMemoryReferences()
		require.Len(t, refs, 2)
		assert.Equal(t, "mem-id-1", refs[0].ID)
	})

	t.Run("Level 2: missing memory_key", func(t *testing.T) {
		cfg := validBaseConfig()
		cfg.Memory = true
		cfg.Memories = []string{"mem-id-1"}
		err := cfg.NormalizeAndValidateMemoryConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "'memory_key' is required for Level 2")
	})

	t.Run("Level 2: memories not a list or empty list", func(t *testing.T) {
		cfg := validBaseConfig()
		cfg.Memory = true
		cfg.Memories = "not-a-list"
		cfg.MemoryKey = "key"
		err := cfg.NormalizeAndValidateMemoryConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires 'memories' to be a non-empty list of memory ID strings")

		cfg.Memories = []any{}
		err = cfg.NormalizeAndValidateMemoryConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires 'memories' to be a non-empty list of memory ID strings")
	})

	t.Run("Level 3: memories list of objects", func(t *testing.T) {
		cfg := validBaseConfig()
		cfg.Memories = []any{
			map[string]any{"id": "mem1", "key": "key1", "mode": "read-only"},
			map[string]any{"id": "mem2", "key": "key2"}, // mode defaults to read-write
		}
		err := cfg.NormalizeAndValidateMemoryConfig()
		require.NoError(t, err)
		refs := cfg.GetResolvedMemoryReferences()
		require.Len(t, refs, 2)
		assert.Equal(t, "mem1", refs[0].ID)
		assert.Equal(t, "key1", refs[0].Key)
		assert.Equal(t, "read-only", refs[0].Mode)
		assert.Equal(t, "mem2", refs[1].ID)
		assert.Equal(t, "key2", refs[1].Key)
		assert.Equal(t, "read-write", refs[1].Mode)
	})

	t.Run("Level 3: with memory field (invalid)", func(t *testing.T) {
		cfg := validBaseConfig()
		cfg.Memory = "some-id" // Invalid for Level 3
		cfg.Memories = []any{
			map[string]any{"id": "mem1", "key": "key1"},
		}
		err := cfg.NormalizeAndValidateMemoryConfig()
		require.Error(t, err)
		assert.Contains(
			t,
			err.Error(),
			"cannot use 'memory' field (Level 1 or 2) when 'memories' is a list of objects (Level 3)",
		)
	})

	t.Run("Level 3: missing id in reference", func(t *testing.T) {
		cfg := validBaseConfig()
		cfg.Memories = []any{
			map[string]any{"key": "key1"}, // Missing id
		}
		err := cfg.NormalizeAndValidateMemoryConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing required 'id' field")
	})

	t.Run("Level 3: missing key in reference", func(t *testing.T) {
		cfg := validBaseConfig()
		cfg.Memories = []any{
			map[string]any{"id": "mem1"}, // Missing key
		}
		err := cfg.NormalizeAndValidateMemoryConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing required 'key' field")
	})

	t.Run("Level 3: invalid mode", func(t *testing.T) {
		cfg := validBaseConfig()
		cfg.Memories = []any{
			map[string]any{"id": "mem1", "key": "key1", "mode": "write-only"}, // invalid mode
		}
		err := cfg.NormalizeAndValidateMemoryConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "has invalid mode 'write-only'")
	})

	t.Run("No memory configuration", func(t *testing.T) {
		cfg := validBaseConfig()
		err := cfg.NormalizeAndValidateMemoryConfig()
		require.NoError(t, err)
		assert.Empty(t, cfg.GetResolvedMemoryReferences())
	})

	t.Run("memory: false (explicit no memory)", func(t *testing.T) {
		cfg := validBaseConfig()
		cfg.Memory = false
		err := cfg.NormalizeAndValidateMemoryConfig()
		require.NoError(t, err)
		assert.Empty(t, cfg.GetResolvedMemoryReferences())
	})

	t.Run("Invalid type for memory field", func(t *testing.T) {
		cfg := validBaseConfig()
		cfg.Memory = 123 // not string or bool
		err := cfg.NormalizeAndValidateMemoryConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid type for 'memory' field")
	})

	t.Run("Invalid structure for memories field", func(t *testing.T) {
		cfg := validBaseConfig()
		cfg.Memories = "not-a-list-of-anything-valid"
		err := cfg.NormalizeAndValidateMemoryConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid structure for 'memories' field")
	})
}

// Test_Config_Validate_WithMemory integrates testing Validate's call to normalizeAndValidateMemoryConfig
func Test_Config_Validate_WithMemory(t *testing.T) {
	agentCWD, _ := core.CWDFromPath("/test")
	baseCfg := func() Config {
		return Config{ID: "test", Config: core.ProviderConfig{}, Instructions: "test", CWD: agentCWD}
	}

	t.Run("Valid Level 1 memory config passes full validation", func(t *testing.T) {
		cfg := baseCfg()
		cfg.Memory = "mem1"
		cfg.MemoryKey = "key1"
		// Mock or skip registry check for now in AgentMemoryValidator for this test to pass
		// by ensuring AgentMemoryValidator doesn't error on placeholder registry logic.
		err := cfg.Validate()
		assert.NoError(
			t,
			err,
		) // This will fail if AgentMemoryValidator tries to use a nil registry to check ID existence
	})

	t.Run("Invalid Level 1 (missing key) fails full validation", func(t *testing.T) {
		cfg := baseCfg()
		cfg.Memory = "mem1"
		// MemoryKey is intentionally not set to test validation failure
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid memory configuration")
		assert.Contains(t, err.Error(), "'memory_key' is required for Level 1")
	})
}
