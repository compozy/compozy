package collection

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/tplengine"
)

func TestNewRuntimeProcessor(t *testing.T) {
	t.Run("Should create new runtime processor with template engine", func(t *testing.T) {
		engine := tplengine.NewEngine(tplengine.FormatJSON)
		processor := NewRuntimeProcessor(engine)
		assert.NotNil(t, processor)
		assert.Equal(t, engine, processor.templateEngine)
	})
}

func TestRuntimeProcessor_ProcessItemConfig(t *testing.T) {
	engine := tplengine.NewEngine(tplengine.FormatJSON)
	processor := NewRuntimeProcessor(engine)

	t.Run("Should process task ID templates with item context and index", func(t *testing.T) {
		baseConfig := &task.Config{}
		baseConfig.ID = "task-{{.item.name}}-{{.index}}"
		baseConfig.Type = task.TaskTypeBasic

		itemContext := map[string]any{
			"item":  map[string]any{"name": "test"},
			"index": 0,
		}

		result, err := processor.ProcessItemConfig(baseConfig, itemContext)
		require.NoError(t, err)
		assert.Equal(t, "task-test-0", result.ID)
	})

	t.Run("Should process action field templates with existing values", func(t *testing.T) {
		baseConfig := &task.Config{}
		baseConfig.ID = "test"
		baseConfig.Type = task.TaskTypeBasic
		baseConfig.Action = "{{.item.action}}"

		itemContext := map[string]any{
			"item": map[string]any{
				"action": "process",
			},
		}

		result, err := processor.ProcessItemConfig(baseConfig, itemContext)
		require.NoError(t, err)
		assert.Equal(t, "process", result.Action)
	})

	t.Run("Should deep process nested 'with' parameters containing templates", func(t *testing.T) {
		withInput := core.Input{
			"message": "Hello {{.item.name}}",
			"nested": map[string]any{
				"value": "{{.item.value}}",
			},
		}
		baseConfig := &task.Config{}
		baseConfig.ID = "test"
		baseConfig.Type = task.TaskTypeBasic
		baseConfig.With = &withInput

		itemContext := map[string]any{
			"item": map[string]any{
				"name":  "world",
				"value": 42,
			},
		}

		result, err := processor.ProcessItemConfig(baseConfig, itemContext)
		require.NoError(t, err)
		require.NotNil(t, result.With)

		withMap := map[string]any(*result.With)
		assert.Equal(t, "Hello world", withMap["message"])
		nestedMap := withMap["nested"].(map[string]any)
		assert.Equal(t, 42, nestedMap["value"])
	})

	t.Run("Should process agent configuration templates including provider and model", func(t *testing.T) {
		baseConfig := &task.Config{}
		baseConfig.ID = "test"
		baseConfig.Type = task.TaskTypeBasic
		baseConfig.Agent = &agent.Config{
			ID:           "test-agent",
			Instructions: "Process: {{.item.prompt}}",
			Config: core.ProviderConfig{
				Provider: "{{.item.provider}}",
				Model:    "{{.item.model}}",
			},
		}

		itemContext := map[string]any{
			"item": map[string]any{
				"provider": "openai",
				"model":    "gpt-4",
				"prompt":   "analyze data",
			},
		}

		result, err := processor.ProcessItemConfig(baseConfig, itemContext)
		require.NoError(t, err)
		require.NotNil(t, result.Agent)
		assert.Equal(t, "Process: analyze data", result.Agent.Instructions)
		assert.Equal(t, core.ProviderName("openai"), result.Agent.Config.Provider)
		assert.Equal(t, "gpt-4", result.Agent.Config.Model)
	})

	t.Run("Should process tool configuration templates with nested parameters", func(t *testing.T) {
		baseConfig := &task.Config{}
		baseConfig.ID = "test"
		baseConfig.Type = task.TaskTypeBasic
		baseConfig.Tool = &tool.Config{
			ID: "{{.item.tool_name}}",
			With: &core.Input{
				"input": "{{.item.input}}",
				"nested": map[string]any{
					"value": "{{.item.nested_value}}",
				},
			},
		}

		itemContext := map[string]any{
			"item": map[string]any{
				"tool_name":    "processor",
				"input":        "test data",
				"nested_value": "deep value",
			},
		}

		result, err := processor.ProcessItemConfig(baseConfig, itemContext)
		require.NoError(t, err)
		require.NotNil(t, result.Tool)
		assert.Equal(t, "processor", result.Tool.ID)
		withMap := map[string]any(*result.Tool.With)
		assert.Equal(t, "test data", withMap["input"])
		nestedParams := withMap["nested"].(map[string]any)
		assert.Equal(t, "deep value", nestedParams["value"])
	})

	t.Run("Should handle templates with available variables", func(t *testing.T) {
		baseConfig := &task.Config{}
		baseConfig.ID = "test"
		baseConfig.Type = task.TaskTypeBasic
		baseConfig.Action = "{{.item.available}}"

		itemContext := map[string]any{
			"item": map[string]any{
				"available": "fallback",
			},
		}

		result, err := processor.ProcessItemConfig(baseConfig, itemContext)
		require.NoError(t, err)
		assert.Equal(t, "fallback", result.Action)
	})

	t.Run("Should preserve non-template strings without modification", func(t *testing.T) {
		baseConfig := &task.Config{}
		baseConfig.ID = "static-id"
		baseConfig.Type = task.TaskTypeBasic
		baseConfig.Action = "static-action"

		itemContext := map[string]any{
			"item": map[string]any{"name": "test"},
		}

		result, err := processor.ProcessItemConfig(baseConfig, itemContext)
		require.NoError(t, err)
		assert.Equal(t, "static-id", result.ID)
		assert.Equal(t, "static-action", result.Action)
	})

	t.Run("Should handle complex nested template expressions", func(t *testing.T) {
		withInput := core.Input{
			"complex": "{{.item.prefix}}-{{.item.suffix}}-{{.index}}",
		}
		baseConfig := &task.Config{}
		baseConfig.ID = "test"
		baseConfig.Type = task.TaskTypeBasic
		baseConfig.With = &withInput

		itemContext := map[string]any{
			"item": map[string]any{
				"prefix": "task",
				"suffix": "done",
			},
			"index": 5,
		}

		result, err := processor.ProcessItemConfig(baseConfig, itemContext)
		require.NoError(t, err)
		require.NotNil(t, result.With)

		withMap := map[string]any(*result.With)
		assert.Equal(t, "task-done-5", withMap["complex"])
	})

	t.Run("Should properly merge item context with workflow and task contexts", func(t *testing.T) {
		baseConfig := &task.Config{}
		baseConfig.ID = "{{.workflow.name}}-{{.task.id}}-{{.item.name}}"
		baseConfig.Type = task.TaskTypeBasic

		itemContext := map[string]any{
			"workflow": map[string]any{"name": "test-workflow"},
			"task":     map[string]any{"id": "parent-task"},
			"item":     map[string]any{"name": "item1"},
		}

		result, err := processor.ProcessItemConfig(baseConfig, itemContext)
		require.NoError(t, err)
		assert.Equal(t, "test-workflow-parent-task-item1", result.ID)
	})

	t.Run("Should return error for nil base config", func(t *testing.T) {
		itemContext := map[string]any{"item": map[string]any{}}

		result, err := processor.ProcessItemConfig(nil, itemContext)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "base config cannot be nil")
	})
}

func TestRuntimeProcessor_processWithParameters(t *testing.T) {
	engine := tplengine.NewEngine(tplengine.FormatJSON)
	processor := NewRuntimeProcessor(engine)

	t.Run("Should process map parameters", func(t *testing.T) {
		withParams := map[string]any{
			"key": "{{.item.value}}",
		}
		context := map[string]any{
			"item": map[string]any{"value": "test"},
		}

		result, err := processor.processWithParameters(withParams, context)
		require.NoError(t, err)
		resultMap := result.(map[string]any)
		assert.Equal(t, "test", resultMap["key"])
	})

	t.Run("Should process string templates", func(t *testing.T) {
		withParams := "{{.item.value}}"
		context := map[string]any{
			"item": map[string]any{"value": "test"},
		}

		result, err := processor.processWithParameters(withParams, context)
		require.NoError(t, err)
		assert.Equal(t, "test", result)
	})

	t.Run("Should parse JSON strings and process templates", func(t *testing.T) {
		withParams := `{"key": "{{.item.value}}"}`
		context := map[string]any{
			"item": map[string]any{"value": "test"},
		}

		result, err := processor.processWithParameters(withParams, context)
		require.NoError(t, err)
		resultMap := result.(map[string]any)
		assert.Equal(t, "test", resultMap["key"])
	})
}

func TestRuntimeProcessor_processAgentConfig(t *testing.T) {
	engine := tplengine.NewEngine(tplengine.FormatJSON)
	processor := NewRuntimeProcessor(engine)

	t.Run("Should process agent configuration with nested settings", func(t *testing.T) {
		agentConfig := map[string]any{
			"provider": "{{.item.provider}}",
			"settings": map[string]any{
				"temperature": "{{.item.temp}}",
			},
		}
		context := map[string]any{
			"item": map[string]any{
				"provider": "openai",
				"temp":     0.7,
			},
		}

		result, err := processor.processAgentConfig(agentConfig, context)
		require.NoError(t, err)
		resultMap := result.(map[string]any)
		assert.Equal(t, "openai", resultMap["provider"])
		settings := resultMap["settings"].(map[string]any)
		assert.Equal(t, float64(0.7), settings["temperature"])
	})
}

func TestRuntimeProcessor_processToolConfig(t *testing.T) {
	engine := tplengine.NewEngine(tplengine.FormatJSON)
	processor := NewRuntimeProcessor(engine)

	t.Run("Should process tool configuration with nested params", func(t *testing.T) {
		toolConfig := map[string]any{
			"name": "{{.item.tool}}",
			"params": map[string]any{
				"input": "{{.item.input}}",
			},
		}
		context := map[string]any{
			"item": map[string]any{
				"tool":  "processor",
				"input": "test data",
			},
		}

		result, err := processor.processToolConfig(toolConfig, context)
		require.NoError(t, err)
		resultMap := result.(map[string]any)
		assert.Equal(t, "processor", resultMap["name"])
		params := resultMap["params"].(map[string]any)
		assert.Equal(t, "test data", params["input"])
	})
}

func TestRuntimeProcessor_ErrorHandling(t *testing.T) {
	engine := tplengine.NewEngine(tplengine.FormatJSON)
	processor := NewRuntimeProcessor(engine)

	t.Run("Should handle template processing errors gracefully", func(t *testing.T) {
		baseConfig := &task.Config{}
		baseConfig.ID = "{{.missing.field}}"
		baseConfig.Type = task.TaskTypeBasic

		itemContext := map[string]any{
			"item": "test",
		}

		result, err := processor.ProcessItemConfig(baseConfig, itemContext)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to process config templates")
	})

	t.Run("Should handle malformed JSON in with parameters", func(t *testing.T) {
		withParams := `{"invalid": json}`
		context := map[string]any{}

		result, err := processor.processWithParameters(withParams, context)
		require.NoError(t, err)
		assert.Equal(t, `{"invalid": json}`, result)
	})

	t.Run("Should handle non-map agent config", func(t *testing.T) {
		agentConfig := "invalid-config"
		context := map[string]any{}

		result, err := processor.processAgentConfig(agentConfig, context)
		require.NoError(t, err)
		assert.Equal(t, "invalid-config", result)
	})

	t.Run("Should handle non-map tool config", func(t *testing.T) {
		toolConfig := []string{"invalid", "config"}
		context := map[string]any{}

		result, err := processor.processToolConfig(toolConfig, context)
		require.NoError(t, err)
		assert.Equal(t, []string{"invalid", "config"}, result)
	})
}

func TestRuntimeProcessor_ComplexScenarios(t *testing.T) {
	engine := tplengine.NewEngine(tplengine.FormatJSON)
	processor := NewRuntimeProcessor(engine)

	t.Run("Should handle deeply nested template structures", func(t *testing.T) {
		withInput := core.Input{
			"database": map[string]any{
				"host": "{{.item.db.host}}",
				"config": map[string]any{
					"timeout": "{{.item.db.timeout}}",
					"retries": "{{.item.db.retries}}",
				},
			},
		}
		baseConfig := &task.Config{}
		baseConfig.ID = "db-task-{{.index}}"
		baseConfig.Type = task.TaskTypeBasic
		baseConfig.With = &withInput

		itemContext := map[string]any{
			"item": map[string]any{
				"db": map[string]any{
					"host":    "localhost",
					"timeout": 30,
					"retries": 3,
				},
			},
			"index": 1,
		}

		result, err := processor.ProcessItemConfig(baseConfig, itemContext)
		require.NoError(t, err)
		assert.Equal(t, "db-task-1", result.ID)

		withMap := map[string]any(*result.With)
		database := withMap["database"].(map[string]any)
		assert.Equal(t, "localhost", database["host"])

		config := database["config"].(map[string]any)
		assert.Equal(t, 30, config["timeout"])
		assert.Equal(t, 3, config["retries"])
	})

	t.Run("Should preserve outputs field during processing", func(t *testing.T) {
		outputs := core.Input{
			"result": "{{.output}}",
		}
		baseConfig := &task.Config{}
		baseConfig.ID = "{{.item.name}}"
		baseConfig.Type = task.TaskTypeBasic
		baseConfig.Outputs = &outputs

		itemContext := map[string]any{
			"item": map[string]any{"name": "test"},
		}

		result, err := processor.ProcessItemConfig(baseConfig, itemContext)
		require.NoError(t, err)
		assert.Equal(t, "test", result.ID)
		assert.NotNil(t, result.Outputs)

		outputsMap := map[string]any(*result.Outputs)
		assert.Equal(t, "{{.output}}", outputsMap["result"])
	})

	t.Run("Should handle empty context gracefully", func(t *testing.T) {
		baseConfig := &task.Config{}
		baseConfig.ID = "static-task"
		baseConfig.Type = task.TaskTypeBasic
		baseConfig.Action = "static-action"

		result, err := processor.ProcessItemConfig(baseConfig, map[string]any{})
		require.NoError(t, err)
		assert.Equal(t, "static-task", result.ID)
		assert.Equal(t, "static-action", result.Action)
	})

	t.Run("Should handle mixed template and static values", func(t *testing.T) {
		withInput := core.Input{
			"template_value": "{{.item.name}}",
			"static_value":   "constant",
			"numeric_value":  42,
			"bool_value":     true,
		}
		baseConfig := &task.Config{}
		baseConfig.ID = "mixed-task"
		baseConfig.Type = task.TaskTypeBasic
		baseConfig.With = &withInput

		itemContext := map[string]any{
			"item": map[string]any{"name": "dynamic"},
		}

		result, err := processor.ProcessItemConfig(baseConfig, itemContext)
		require.NoError(t, err)

		withMap := map[string]any(*result.With)
		assert.Equal(t, "dynamic", withMap["template_value"])
		assert.Equal(t, "constant", withMap["static_value"])
		assert.Equal(t, float64(42), withMap["numeric_value"]) // JSON parsing returns float64
		assert.Equal(t, true, withMap["bool_value"])
	})
}

func TestRuntimeProcessor_EdgeCases(t *testing.T) {
	engine := tplengine.NewEngine(tplengine.FormatJSON)
	processor := NewRuntimeProcessor(engine)

	t.Run("Should handle nil base config", func(t *testing.T) {
		result, err := processor.ProcessItemConfig(nil, map[string]any{})
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "base config cannot be nil")
	})

	t.Run("Should handle empty item context", func(t *testing.T) {
		baseConfig := &task.Config{}
		baseConfig.ID = "test-{{.item.name}}"
		baseConfig.Type = task.TaskTypeBasic

		// Template engine needs the item key to exist, even if empty
		_, err := processor.ProcessItemConfig(baseConfig, map[string]any{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tplengine: missing key")
	})

	t.Run("Should handle ID field with no template", func(t *testing.T) {
		baseConfig := &task.Config{}
		baseConfig.ID = "static-id"
		baseConfig.Type = task.TaskTypeBasic

		result, err := processor.ProcessItemConfig(baseConfig, map[string]any{})
		require.NoError(t, err)
		assert.Equal(t, "static-id", result.ID)
	})

	t.Run("Should handle action field with no template", func(t *testing.T) {
		baseConfig := &task.Config{}
		baseConfig.ID = "test"
		baseConfig.Type = task.TaskTypeBasic
		baseConfig.Action = "static-action"

		result, err := processor.ProcessItemConfig(baseConfig, map[string]any{})
		require.NoError(t, err)
		assert.Equal(t, "static-action", result.Action)
	})

	t.Run("Should handle with parameters containing plain strings", func(t *testing.T) {
		withInput := core.Input{
			"plain_string":  "not a json",
			"number_string": "42",
		}
		baseConfig := &task.Config{}
		baseConfig.ID = "test"
		baseConfig.Type = task.TaskTypeBasic
		baseConfig.With = &withInput

		result, err := processor.ProcessItemConfig(baseConfig, map[string]any{})
		require.NoError(t, err)
		require.NotNil(t, result.With)

		withMap := map[string]any(*result.With)
		// Plain strings remain as strings
		assert.Equal(t, "not a json", withMap["plain_string"])
		assert.Equal(t, "42", withMap["number_string"])
	})

	t.Run("Should handle agent config with actions", func(t *testing.T) {
		baseConfig := &task.Config{}
		baseConfig.ID = "test"
		baseConfig.Type = task.TaskTypeBasic
		baseConfig.Agent = &agent.Config{
			ID:           "test-agent",
			Instructions: "Process item",
			Config: core.ProviderConfig{
				Provider: "openai",
				Model:    "gpt-4",
			},
			Actions: []*agent.ActionConfig{
				{
					ID:     "action1",
					Prompt: "Do {{.item.task}}",
				},
			},
		}

		itemContext := map[string]any{
			"item": map[string]any{
				"task": "analysis",
			},
		}

		result, err := processor.ProcessItemConfig(baseConfig, itemContext)
		require.NoError(t, err)
		require.NotNil(t, result.Agent)
		require.Len(t, result.Agent.Actions, 1)
		assert.Equal(t, "Do analysis", result.Agent.Actions[0].Prompt)
	})

	t.Run("Should handle tool config with execute template", func(t *testing.T) {
		baseConfig := &task.Config{}
		baseConfig.ID = "test"
		baseConfig.Type = task.TaskTypeBasic
		baseConfig.Tool = &tool.Config{
			ID:          "test-tool",
			Description: "Execute {{.item.command}}",
		}

		itemContext := map[string]any{
			"item": map[string]any{
				"command": "analysis",
				"script":  "run.sh",
			},
		}

		result, err := processor.ProcessItemConfig(baseConfig, itemContext)
		require.NoError(t, err)
		require.NotNil(t, result.Tool)
		assert.Equal(t, "Execute analysis", result.Tool.Description)
	})

	t.Run("Should handle with parameters as array", func(t *testing.T) {
		withInput := core.Input{
			"items": []any{"item1", "item2", "{{.item.extra}}"},
		}
		baseConfig := &task.Config{}
		baseConfig.ID = "test"
		baseConfig.Type = task.TaskTypeBasic
		baseConfig.With = &withInput

		itemContext := map[string]any{
			"item": map[string]any{
				"extra": "item3",
			},
		}

		result, err := processor.ProcessItemConfig(baseConfig, itemContext)
		require.NoError(t, err)
		require.NotNil(t, result.With)

		withMap := map[string]any(*result.With)
		items := withMap["items"].([]any)
		assert.Len(t, items, 3)
		assert.Equal(t, "item1", items[0])
		assert.Equal(t, "item2", items[1])
		assert.Equal(t, "item3", items[2])
	})

	t.Run("Should handle with parameters as template string", func(t *testing.T) {
		withInput := core.Input{
			"name":  "{{.item.name}}",
			"value": "{{.item.value}}",
		}
		baseConfig := &task.Config{}
		baseConfig.ID = "test"
		baseConfig.Type = task.TaskTypeBasic
		baseConfig.With = &withInput

		itemContext := map[string]any{
			"item": map[string]any{
				"name":  "test-item",
				"value": "test-value",
			},
		}

		result, err := processor.ProcessItemConfig(baseConfig, itemContext)
		require.NoError(t, err)
		require.NotNil(t, result.With)

		withMap := map[string]any(*result.With)
		assert.Equal(t, "test-item", withMap["name"])
		assert.Equal(t, "test-value", withMap["value"])
	})

	t.Run("Should handle invalid JSON in with parameters", func(t *testing.T) {
		withInput := core.Input{
			"bad_json": "{invalid json}",
		}
		baseConfig := &task.Config{}
		baseConfig.ID = "test"
		baseConfig.Type = task.TaskTypeBasic
		baseConfig.With = &withInput

		result, err := processor.ProcessItemConfig(baseConfig, map[string]any{})
		require.NoError(t, err)
		require.NotNil(t, result.With)

		withMap := map[string]any(*result.With)
		// Invalid JSON should be kept as string
		assert.Equal(t, "{invalid json}", withMap["bad_json"])
	})

	t.Run("Should handle nil fields gracefully", func(t *testing.T) {
		baseConfig := &task.Config{}
		baseConfig.ID = "test"
		baseConfig.Type = task.TaskTypeBasic
		// All optional fields are nil

		result, err := processor.ProcessItemConfig(baseConfig, map[string]any{})
		require.NoError(t, err)
		assert.Equal(t, "test", result.ID)
		assert.Nil(t, result.With)
		assert.Nil(t, result.Agent)
		assert.Nil(t, result.Tool)
	})

	t.Run("Should handle agent with tools", func(t *testing.T) {
		baseConfig := &task.Config{}
		baseConfig.ID = "test"
		baseConfig.Type = task.TaskTypeBasic
		baseConfig.Agent = &agent.Config{
			ID:           "test-agent",
			Instructions: "Process",
			Config: core.ProviderConfig{
				Provider: "openai",
				Model:    "gpt-4",
			},
			LLMProperties: agent.LLMProperties{
				Tools: []tool.Config{
					{
						ID:          "tool1",
						Description: "Tool for {{.item.purpose}}",
					},
				},
			},
		}

		itemContext := map[string]any{
			"item": map[string]any{
				"purpose": "testing",
			},
		}

		result, err := processor.ProcessItemConfig(baseConfig, itemContext)
		require.NoError(t, err)
		require.NotNil(t, result.Agent)
		require.Len(t, result.Agent.Tools, 1)
		assert.Equal(t, "Tool for testing", result.Agent.Tools[0].Description)
	})
}
