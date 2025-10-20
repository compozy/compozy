package agent

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/schema"
	mcpproxy "github.com/compozy/compozy/pkg/mcp-proxy"
	fixtures "github.com/compozy/compozy/test/fixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTest(t *testing.T, agentFile string) (*core.PathCWD, string) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	cwd, dstPath := fixtures.SetupConfigTest(t, filename)
	dstPath = filepath.Join(dstPath, agentFile)
	return cwd, dstPath
}

func Test_LoadAgent(t *testing.T) {
	t.Run("Should load basic agent configuration correctly", func(t *testing.T) {
		cwd, dstPath := setupTest(t, "basic_agent.yaml")
		config, err := Load(t.Context(), cwd, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		require.NotNil(t, config.ID)
		// Model config is resolved via the Model field

		assert.Equal(t, "code-assistant", config.ID)
		assert.Equal(t, core.ProviderAnthropic, config.Model.Config.Provider)
		assert.Equal(t, "claude-4-opus", config.Model.Config.Model)
		assert.InDelta(t, 0.7, config.Model.Config.Params.Temperature, 1e-6)
		assert.Equal(t, int32(4000), config.Model.Config.Params.MaxTokens)

		require.Len(t, config.Actions, 1)
		action := config.Actions[0]
		assert.Equal(t, "review-code", action.ID)

		require.NotNil(t, action.InputSchema)
		schema := action.InputSchema
		compiledSchema, err := schema.Compile(t.Context())
		require.NoError(t, err)
		assert.Equal(t, []string{"object"}, []string(compiledSchema.Type))
		require.NotNil(t, compiledSchema.Properties)
		assert.Contains(t, (*compiledSchema.Properties), "code")
		assert.Contains(t, (*compiledSchema.Properties), "language")
		assert.Contains(t, compiledSchema.Required, "code")

		require.NotNil(t, action.OutputSchema)
		outSchema := action.OutputSchema
		compiledOutSchema, err := outSchema.Compile(t.Context())
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

func Test_AgentMCPs_Decode_YAML_And_FromMap(t *testing.T) {
	t.Run("Should decode mcps from YAML with scalar and object forms", func(t *testing.T) {
		cwd, dstPath := setupTest(t, "agent_mcps_dual.yaml")
		cfg, err := Load(t.Context(), cwd, dstPath)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.Len(t, cfg.MCPs, 2)
		assert.Equal(t, "fs", cfg.MCPs[0].ID)
		assert.Equal(t, "http", cfg.MCPs[1].ID)
		assert.Equal(t, "http://localhost:3000/mcp", cfg.MCPs[1].URL)
	})

	t.Run("Should decode mcps from map with string IDs", func(t *testing.T) {
		var dst Config
		m := map[string]any{
			"id":           "x",
			"instructions": "i",
			"model": map[string]any{
				"provider": string(core.ProviderOpenAI),
				"model":    "gpt-4o-mini",
			},
			"mcps": []any{
				"srv1",
				map[string]any{"id": "srv2", "url": "http://example"},
			},
		}
		err := dst.FromMap(m)
		require.NoError(t, err)
		require.Len(t, dst.MCPs, 2)
		assert.Equal(t, "srv1", dst.MCPs[0].ID)
		assert.Equal(t, "srv2", dst.MCPs[1].ID)
		assert.Equal(t, "http://example", dst.MCPs[1].URL)
	})

	t.Run("Should fail when mcps contains invalid non-string value", func(t *testing.T) {
		var dst Config
		m := map[string]any{
			"id":           "x",
			"instructions": "i",
			"model": map[string]any{
				"provider": string(core.ProviderOpenAI),
				"model":    "gpt-4o-mini",
			},
			// invalid element (number) should cause decode error
			"mcps": []any{12345},
		}
		err := dst.FromMap(m)
		require.Error(t, err)
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
		err := config.Validate(t.Context())
		assert.NoError(t, err)
	})

	t.Run("Should return error when CWD is missing", func(t *testing.T) {
		config := &ActionConfig{
			ID:     "test-action",
			Prompt: "test prompt",
		}
		err := config.Validate(t.Context())
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
		err := config.ValidateInput(t.Context(), config.With)
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
			Model:        Model{Config: core.ProviderConfig{}},
			Instructions: "test instructions",
			CWD:          agentCWD,
		}
		err := config.Validate(t.Context())
		assert.NoError(t, err)
	})

	t.Run("Should return error when CWD is missing", func(t *testing.T) {
		config := &Config{
			ID:           agentID,
			Model:        Model{Config: core.ProviderConfig{}},
			Instructions: "test instructions",
		}
		err := config.Validate(t.Context())
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
			CWD:          originalCWD,
		}

		// Act
		copied, err := original.Clone()
		assert.NoError(t, err)

		// Assert - verify basic fields are copied
		assert.Equal(t, original.ID, copied.ID)
		assert.Equal(t, original.Prompt, copied.Prompt)

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
		return &Config{
			ID:           "test-agent",
			Model:        Model{Config: core.ProviderConfig{}},
			Instructions: "do stuff",
			CWD:          agentCWD,
		}
	}

	t.Run("Should handle single memory reference", func(t *testing.T) {
		cfg := validBaseConfig()
		cfg.Memory = []core.MemoryReference{
			{ID: "mem-id-1", Key: "key-{{.workflow.id}}", Mode: "read-write"},
		}
		err := cfg.NormalizeAndValidateMemoryConfig()
		require.NoError(t, err)
		refs := cfg.Memory
		require.Len(t, refs, 1)
		assert.Equal(t, "mem-id-1", refs[0].ID)
		assert.Equal(t, "key-{{.workflow.id}}", refs[0].Key)
		assert.Equal(t, "read-write", refs[0].Mode)
	})

	t.Run("Should handle multiple memory references", func(t *testing.T) {
		cfg := validBaseConfig()
		cfg.Memory = []core.MemoryReference{
			{ID: "mem-id-1", Key: "shared-key-{{.user.id}}", Mode: "read-write"},
			{ID: "mem-id-2", Key: "shared-key-{{.user.id}}", Mode: "read-write"},
		}
		err := cfg.NormalizeAndValidateMemoryConfig()
		require.NoError(t, err)
		refs := cfg.Memory
		require.Len(t, refs, 2)
		assert.Equal(t, "mem-id-1", refs[0].ID)
		assert.Equal(t, "shared-key-{{.user.id}}", refs[0].Key)
		assert.Equal(t, "read-write", refs[0].Mode)
		assert.Equal(t, "mem-id-2", refs[1].ID)
		assert.Equal(t, "shared-key-{{.user.id}}", refs[1].Key)
		assert.Equal(t, "read-write", refs[1].Mode)
	})

	t.Run("Should handle memory references with different modes", func(t *testing.T) {
		cfg := validBaseConfig()
		cfg.Memory = []core.MemoryReference{
			{ID: "mem1", Key: "key1", Mode: "read-only"},
			{ID: "mem2", Key: "key2", Mode: "read-write"},
		}
		err := cfg.NormalizeAndValidateMemoryConfig()
		require.NoError(t, err)
		refs := cfg.Memory
		require.Len(t, refs, 2)
		assert.Equal(t, "mem1", refs[0].ID)
		assert.Equal(t, "key1", refs[0].Key)
		assert.Equal(t, "read-only", refs[0].Mode)
		assert.Equal(t, "mem2", refs[1].ID)
		assert.Equal(t, "key2", refs[1].Key)
		assert.Equal(t, "read-write", refs[1].Mode)
	})

	t.Run("Should default to read-write mode when not specified", func(t *testing.T) {
		cfg := validBaseConfig()
		cfg.Memory = []core.MemoryReference{
			{ID: "mem1", Key: "key1"}, // Mode not specified
		}
		err := cfg.NormalizeAndValidateMemoryConfig()
		require.NoError(t, err)
		refs := cfg.Memory
		require.Len(t, refs, 1)
		assert.Equal(t, "mem1", refs[0].ID)
		assert.Equal(t, "key1", refs[0].Key)
		assert.Equal(t, "read-write", refs[0].Mode)
	})

	t.Run("Should require ID in memory reference", func(t *testing.T) {
		cfg := validBaseConfig()
		cfg.Memory = []core.MemoryReference{
			{Key: "key1"}, // Missing ID
		}
		err := cfg.NormalizeAndValidateMemoryConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing required 'id' field")
	})

	t.Run("Should allow missing Key in memory reference (fallback handled at runtime)", func(t *testing.T) {
		cfg := validBaseConfig()
		cfg.Memory = []core.MemoryReference{
			{ID: "mem1"}, // Missing Key now allowed
		}
		err := cfg.NormalizeAndValidateMemoryConfig()
		require.NoError(t, err)
	})

	t.Run("Should reject invalid mode", func(t *testing.T) {
		cfg := validBaseConfig()
		cfg.Memory = []core.MemoryReference{
			{ID: "mem1", Key: "key1", Mode: "write-only"}, // Invalid mode
		}
		err := cfg.NormalizeAndValidateMemoryConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "has invalid mode 'write-only'")
	})

	t.Run("Should handle no memory configuration", func(t *testing.T) {
		cfg := validBaseConfig()
		err := cfg.NormalizeAndValidateMemoryConfig()
		require.NoError(t, err)
		assert.Empty(t, cfg.Memory)
	})

	t.Run("Should handle empty memory array", func(t *testing.T) {
		cfg := validBaseConfig()
		cfg.Memory = []core.MemoryReference{}
		err := cfg.NormalizeAndValidateMemoryConfig()
		require.NoError(t, err)
		assert.Empty(t, cfg.Memory)
	})
}

// Test_Config_Validate_WithMemory integrates testing Validate's call to normalizeAndValidateMemoryConfig
func Test_Config_Validate_WithMemory(t *testing.T) {
	agentCWD, _ := core.CWDFromPath("/test")
	baseCfg := func() Config {
		return Config{ID: "test", Model: Model{Config: core.ProviderConfig{}}, Instructions: "test", CWD: agentCWD}
	}

	t.Run("Valid memory config passes full validation", func(t *testing.T) {
		cfg := baseCfg()
		cfg.Memory = []core.MemoryReference{
			{ID: "mem1", Key: "key1", Mode: "read-write"},
		}
		// Mock or skip registry check for now in AgentMemoryValidator for this test to pass
		// by ensuring AgentMemoryValidator doesn't error on placeholder registry logic.
		err := cfg.Validate(t.Context())
		assert.NoError(
			t,
			err,
		) // This will fail if AgentMemoryValidator tries to use a nil registry to check ID existence
	})

	t.Run("Missing key is allowed at validation (runtime will enforce via default_key_template)", func(t *testing.T) {
		cfg := baseCfg()
		cfg.Memory = []core.MemoryReference{
			{ID: "mem1"}, // Missing Key
		}
		err := cfg.Validate(t.Context())
		assert.NoError(t, err)
	})
}

func Test_Config_Merge_Clone_AsMap_FromMap(t *testing.T) {
	t.Run("Should merge with override precedence", func(t *testing.T) {
		base := &Config{ID: "a", LLMProperties: LLMProperties{MaxIterations: 1}, Instructions: "x"}
		other := &Config{LLMProperties: LLMProperties{MaxIterations: 5}, Instructions: "y"}
		require.NoError(t, base.Merge(other))
		assert.Equal(t, 5, base.MaxIterations)
		assert.Equal(t, "y", base.Instructions)
	})

	t.Run("Should deep clone without sharing pointers", func(t *testing.T) {
		cwd, _ := core.CWDFromPath("/tmp")
		original := &Config{
			ID:            "id",
			CWD:           cwd,
			LLMProperties: LLMProperties{Memory: []core.MemoryReference{{ID: "m", Key: "k", Mode: "read-write"}}},
		}
		cloned, err := original.Clone()
		require.NoError(t, err)
		require.NotNil(t, cloned)
		assert.NotSame(t, original, cloned)
		if original.CWD != nil && cloned.CWD != nil {
			assert.NotSame(t, original.CWD, cloned.CWD)
		}
		// content preserved
		assert.Equal(t, original.ID, cloned.ID)
		assert.Equal(t, original.Memory[0].ID, cloned.Memory[0].ID)
	})

	t.Run("Should round-trip via AsMap and FromMap", func(t *testing.T) {
		cfg := &Config{ID: "agent-x", Instructions: "do"}
		m, err := cfg.AsMap()
		require.NoError(t, err)
		m["instructions"] = "updated"
		var dst Config
		require.NoError(t, dst.FromMap(m))
		assert.Equal(t, "agent-x", dst.ID)
		assert.Equal(t, "updated", dst.Instructions)
	})
}

func Test_Load_Basic_WithNoEvaluator(t *testing.T) {
	t.Run("Should load agent without evaluator", func(t *testing.T) {
		cwd, dstPath := setupTest(t, "basic_agent.yaml")
		cfg, err := Load(t.Context(), cwd, dstPath)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, "code-assistant", cfg.ID)
	})
}

func Test_Config_Validate_MCPErrorAggregation(t *testing.T) {
	t.Run("Should aggregate MCP validation errors", func(t *testing.T) {
		cwd, _ := core.CWDFromPath("/tmp")
		cfg := &Config{ID: "a", Model: Model{Config: core.ProviderConfig{}}, Instructions: "i", CWD: cwd}
		cfg.MCPs = []mcp.Config{{ID: "srv", Resource: "mcp", Transport: mcpproxy.TransportStdio}}
		err := cfg.Validate(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mcp validation error")
	})
}

func Test_Config_Validate_Noops(t *testing.T) {
	t.Run("Should return nil for ValidateInput and ValidateOutput", func(t *testing.T) {
		var cfg Config
		assert.NoError(t, cfg.ValidateInput(t.Context(), &core.Input{}))
		assert.NoError(t, cfg.ValidateOutput(t.Context(), &core.Output{}))
	})
}
