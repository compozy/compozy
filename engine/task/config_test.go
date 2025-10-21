package task

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
	fixtures "github.com/compozy/compozy/test/fixtures"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTest(t *testing.T, taskFile string) (*core.PathCWD, string) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	cwd, dstPath := fixtures.SetupConfigTest(t, filename)
	dstPath = filepath.Join(dstPath, taskFile)
	return cwd, dstPath
}

func Test_LoadTask(t *testing.T) {
	t.Run("Should load basic task configuration correctly", func(t *testing.T) {
		CWD, dstPath := setupTest(t, "basic_task.yaml")

		// Run the test
		config, err := Load(t.Context(), CWD, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the config
		err = config.Validate(t.Context())
		require.NoError(t, err)

		require.NotNil(t, config.ID)
		require.NotNil(t, config.Type)
		require.NotNil(t, config.Action)
		require.NotNil(t, config.InputSchema)
		require.NotNil(t, config.OutputSchema)
		require.NotNil(t, config.Env)
		require.NotNil(t, config.With)
		require.NotNil(t, config.OnSuccess)
		require.NotNil(t, config.OnError)

		assert.Equal(t, "code-format", config.ID)
		assert.Equal(t, TaskTypeBasic, config.Type)

		// Validate input schema
		schema := config.InputSchema
		compiledSchema, err := schema.Compile(t.Context())
		require.NoError(t, err)
		assert.Equal(t, []string{"object"}, []string(compiledSchema.Type))
		require.NotNil(t, compiledSchema.Properties)
		assert.Contains(t, (*compiledSchema.Properties), "code")
		assert.Contains(t, (*compiledSchema.Properties), "language")
		assert.Contains(t, compiledSchema.Required, "code")

		// Validate output schema
		outSchema := config.OutputSchema
		compiledOutSchema, err := outSchema.Compile(t.Context())
		require.NoError(t, err)
		assert.Equal(t, []string{"object"}, []string(compiledOutSchema.Type))
		require.NotNil(t, compiledOutSchema.Properties)
		assert.Contains(t, (*compiledOutSchema.Properties), "formatted_code")
		assert.Contains(t, compiledOutSchema.Required, "formatted_code")

		// Validate env and with
		assert.Equal(t, "1.0.0", config.GetEnv().Prop("FORMATTER_VERSION"))
		assert.Equal(t, 2, (*config.With)["indent_size"])
		assert.Equal(t, false, (*config.With)["use_tabs"])

		// Validate transitions
		assert.Equal(t, "next-task", *config.OnSuccess.Next)
		assert.Equal(t, "retry-task", *config.OnError.Next)
	})

	t.Run("Should load router task configuration correctly", func(t *testing.T) {
		CWD, dstPath := setupTest(t, "router_task.yaml")

		// Run the test
		config, err := Load(t.Context(), CWD, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the config
		err = config.Validate(t.Context())
		require.NoError(t, err)

		require.NotNil(t, config.ID)
		require.NotNil(t, config.Type)
		require.NotEmpty(t, config.Condition)
		require.NotNil(t, config.Routes)
		require.NotNil(t, config.InputSchema)
		require.NotNil(t, config.OutputSchema)
		require.NotNil(t, config.Env)
		require.NotNil(t, config.With)
		require.NotNil(t, config.OnError)

		assert.Equal(t, "document-classifier", config.ID)
		assert.Equal(t, TaskTypeRouter, config.Type)
		assert.Equal(t, "tasks.analyze.output.type", config.Condition)
		assert.Equal(t, 4, len(config.Routes))

		// Validate routes
		assert.Equal(t, "process-invoice", config.Routes["invoice"])
		assert.Equal(t, "process-contract", config.Routes["contract"])
		assert.Equal(t, "process-receipt", config.Routes["receipt"])
		assert.Equal(t, "manual-review", config.Routes["unknown"])

		// Validate input schema
		schema := config.InputSchema
		compiledSchema2, err := schema.Compile(t.Context())
		require.NoError(t, err)
		assert.Equal(t, []string{"object"}, []string(compiledSchema2.Type))
		require.NotNil(t, compiledSchema2.Properties)
		assert.Contains(t, (*compiledSchema2.Properties), "document")
		assert.Contains(t, (*compiledSchema2.Properties), "confidence_threshold")
		assert.Contains(t, compiledSchema2.Required, "document")

		// Validate output schema
		outSchema := config.OutputSchema
		compiledOutSchema2, err := outSchema.Compile(t.Context())
		require.NoError(t, err)
		assert.Equal(t, []string{"object"}, []string(compiledOutSchema2.Type))
		require.NotNil(t, compiledOutSchema2.Properties)
		assert.Contains(t, (*compiledOutSchema2.Properties), "classification")
		assert.Contains(t, (*compiledOutSchema2.Properties), "route_taken")
		assert.Contains(t, (*compiledOutSchema2.Properties), "confidence")
		assert.Contains(t, compiledOutSchema2.Required, "classification")
		assert.Contains(t, compiledOutSchema2.Required, "route_taken")

		// Validate env and with
		assert.Equal(t, "0.8", config.GetEnv().Prop("CONFIDENCE_THRESHOLD"))
		assert.Equal(t, "{{ .workflow.input.document }}", (*config.With)["document"])
		assert.Equal(t, 0.8, (*config.With)["confidence_threshold"])

		// Validate error transition
		assert.Equal(t, "retry-classification", *config.OnError.Next)
	})

	t.Run("Should load parallel task configuration correctly", func(t *testing.T) {
		CWD, dstPath := setupTest(t, "parallel_task.yaml")
		// Run the test
		config, err := Load(t.Context(), CWD, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the config
		err = config.Validate(t.Context())
		require.NoError(t, err)

		require.NotNil(t, config.ID)
		require.NotNil(t, config.Type)
		require.NotNil(t, config.Tasks)
		require.NotNil(t, config.InputSchema)
		require.NotNil(t, config.OutputSchema)
		require.NotNil(t, config.Env)
		require.NotNil(t, config.With)
		require.NotNil(t, config.OnSuccess)
		require.NotNil(t, config.OnError)

		assert.Equal(t, "process_data_parallel", config.ID)
		assert.Equal(t, TaskTypeParallel, config.Type)
		assert.Equal(t, StrategyWaitAll, config.GetStrategy())
		assert.Equal(t, 4, config.MaxWorkers)
		assert.Equal(t, "5m", config.Timeout)

		// Validate that there's a task reference field
		require.NotNil(t, config.Task)

		// Validate parallel tasks array
		assert.Equal(t, 3, len(config.Tasks))

		// Validate first task (sentiment analysis)
		task1 := config.Tasks[0]
		assert.Equal(t, "sentiment_analysis", task1.ID)
		assert.Equal(t, TaskTypeBasic, task1.Type)
		assert.Equal(t, "analyze_sentiment", task1.Action)
		require.NotNil(t, task1.Agent)
		assert.Equal(t, "text_analyzer", task1.Agent.ID)

		// Validate second task (extract keywords)
		task2 := config.Tasks[1]
		assert.Equal(t, "extract_keywords", task2.ID)
		assert.Equal(t, TaskTypeBasic, task2.Type)
		require.NotNil(t, task2.Tool)
		assert.Equal(t, "nlp_processor", task2.Tool.ID)

		// Validate third task (should be a reference task)
		// This one uses $ref so it might not have all fields populated until evaluation
		// We can just verify it exists
		assert.Equal(t, 3, len(config.Tasks))

		// Validate input schema
		schema := config.InputSchema
		compiledSchema, err := schema.Compile(t.Context())
		require.NoError(t, err)
		assert.Equal(t, []string{"object"}, []string(compiledSchema.Type))
		require.NotNil(t, compiledSchema.Properties)
		assert.Contains(t, (*compiledSchema.Properties), "raw_data")
		assert.Contains(t, (*compiledSchema.Properties), "content")
		assert.Contains(t, compiledSchema.Required, "raw_data")
		assert.Contains(t, compiledSchema.Required, "content")

		// Validate env and with
		assert.Equal(t, "5m", config.GetEnv().Prop("PARALLEL_TIMEOUT"))
		assert.Equal(t, "sample data", (*config.With)["raw_data"])
		assert.Equal(t, "This is a great product! I love it.", (*config.With)["content"])

		// Validate transitions
		assert.Equal(t, "merge_results", *config.OnSuccess.Next)
		assert.Equal(t, "handle_error", *config.OnError.Next)
	})

	t.Run("Should return error for invalid task configuration", func(t *testing.T) {
		CWD, dstPath := setupTest(t, "invalid_task.yaml")

		// Run the test
		config, err := Load(t.Context(), CWD, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the config
		err = config.Validate(t.Context())
		require.Error(t, err)
	})

	t.Run("Should return error for circular dependency in loaded YAML", func(t *testing.T) {
		CWD, dstPath := setupTest(t, "circular_task.yaml")

		// Run the test
		config, err := Load(t.Context(), CWD, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the config - should detect circular dependency
		err = config.Validate(t.Context())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "circular dependency detected involving task: circular_parent")
	})

	t.Run("Should load collection task configuration correctly", func(t *testing.T) {
		CWD, dstPath := setupTest(t, "collection_task.yaml")

		// Run the test
		config, err := Load(t.Context(), CWD, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the config
		err = config.Validate(t.Context())
		require.NoError(t, err)

		// Validate basic task properties
		assert.Equal(t, "process-user-data", config.ID)
		assert.Equal(t, TaskTypeCollection, config.Type)

		// Validate collection-specific configuration
		assert.Equal(t, "{{ .workflow.input.users }}", config.Items)
		assert.Equal(t, "{{ ne .item.status 'inactive' }}", config.Filter)
		assert.Equal(t, "user", config.GetItemVar())
		assert.Equal(t, "idx", config.GetIndexVar())
		assert.Equal(t, CollectionModeSequential, config.GetMode())
		assert.Equal(t, 5, config.Batch)

		// Validate task template
		require.NotNil(t, config.Task)
		assert.Equal(t, "process-user-{{ .idx }}", config.Task.ID)
		assert.Equal(t, TaskTypeBasic, config.Task.Type)
		assert.Equal(t, "process_user_data", config.Task.Action)

		// Validate task template agent
		require.NotNil(t, config.Task.Agent)
		assert.Equal(t, "user-processor", config.Task.Agent.ID)

		// Validate task template with parameters
		require.NotNil(t, config.Task.With)
		assert.Equal(t, "{{ .user.id }}", (*config.Task.With)["user_id"])
		assert.Equal(t, "{{ .user.name }}", (*config.Task.With)["user_name"])
		assert.Equal(t, "{{ .user.email }}", (*config.Task.With)["user_email"])
		assert.Equal(t, "{{ .input.mode }}", (*config.Task.With)["processing_mode"])

		// Validate parallel task properties
		assert.Equal(t, StrategyBestEffort, config.GetStrategy())
		assert.Equal(t, 10, config.MaxWorkers)
		assert.Equal(t, "5m", config.Timeout)
		assert.Equal(t, 2, config.Retries)

		// Validate input schema
		require.NotNil(t, config.InputSchema)
		schema := config.InputSchema
		compiledSchema, err := schema.Compile(t.Context())
		require.NoError(t, err)
		assert.Equal(t, []string{"object"}, []string(compiledSchema.Type))
		require.NotNil(t, compiledSchema.Properties)
		assert.Contains(t, (*compiledSchema.Properties), "users")
		assert.Contains(t, (*compiledSchema.Properties), "mode")
		assert.Contains(t, compiledSchema.Required, "users")

		// Validate environment variables
		assert.Equal(t, "10m", config.GetEnv().Prop("COLLECTION_TIMEOUT"))
		assert.Equal(t, "5", config.GetEnv().Prop("PARALLEL_WORKERS"))
		assert.Equal(t, "https://api.example.com/users", config.GetEnv().Prop("USER_SERVICE_URL"))

		// Validate with parameters
		require.NotNil(t, config.With)
		assert.Equal(t, "batch", (*config.With)["mode"])

		// Validate outputs
		require.NotNil(t, config.Outputs)
		assert.Equal(
			t,
			"Processed {{ .output.processed_count }} users, {{ .output.failed_count }} failed",
			(*config.Outputs)["summary"],
		)

		// Validate transitions
		require.NotNil(t, config.OnSuccess)
		assert.Equal(t, "notify-completion", *config.OnSuccess.Next)
		require.NotNil(t, config.OnError)
		assert.Equal(t, "handle-batch-error", *config.OnError.Next)
	})

	t.Run("Should load signal task configuration correctly", func(t *testing.T) {
		CWD, dstPath := setupTest(t, "signal_task.yaml")

		// Run the test
		config, err := Load(t.Context(), CWD, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the config
		err = config.Validate(t.Context())
		require.NoError(t, err)

		// Validate basic task properties
		assert.Equal(t, "test-signal-task", config.ID)
		assert.Equal(t, TaskTypeSignal, config.Type)

		// Validate signal-specific configuration
		require.NotNil(t, config.Signal)
		assert.Equal(t, "workflow-ready", config.Signal.ID)
		require.NotNil(t, config.Signal.Payload)
		assert.Equal(t, "completed", config.Signal.Payload["status"])
		assert.Equal(t, "{{ .now }}", config.Signal.Payload["timestamp"])
		assert.Equal(t, "{{ .workflow.id }}", config.Signal.Payload["workflow_id"])
	})

	t.Run("Should load prompt-only basic task fixture", func(t *testing.T) {
		CWD, dstPath := setupTest(t, "basic_task_prompt_only.yaml")

		config, err := Load(t.Context(), CWD, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate it loaded correctly
		err = config.Validate(t.Context())
		require.NoError(t, err)

		assert.Equal(t, "text-analysis-prompt", config.ID)
		assert.Equal(t, TaskTypeBasic, config.Type)
		assert.Empty(t, config.Action, "Should have no action in prompt-only mode")
		assert.Equal(t, "Analyze the following text for sentiment and key themes", config.Prompt)

		// Validate with parameters
		require.NotNil(t, config.With)
		assert.Equal(t, "{{ .input.text }}", (*config.With)["text"])

		// Validate outputs
		require.NotNil(t, config.Outputs)
		assert.Equal(t, "{{ .output.sentiment }}", (*config.Outputs)["sentiment"])
		assert.Equal(t, "{{ .output.themes }}", (*config.Outputs)["themes"])
		assert.Equal(t, "{{ .output.summary }}", (*config.Outputs)["summary"])
	})

	t.Run("Should load combined action+prompt basic task fixture", func(t *testing.T) {
		CWD, dstPath := setupTest(t, "basic_task_combined.yaml")

		config, err := Load(t.Context(), CWD, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate it loaded correctly
		err = config.Validate(t.Context())
		require.NoError(t, err)

		assert.Equal(t, "enhanced-analysis", config.ID)
		assert.Equal(t, TaskTypeBasic, config.Type)
		assert.Equal(t, "standard-analysis", config.Action)
		assert.Equal(t, "Additionally, focus on security implications and performance concerns", config.Prompt)

		// Validate with parameters
		require.NotNil(t, config.With)
		assert.Equal(t, "{{ .input.data }}", (*config.With)["data"])
		assert.Equal(t, "{{ .input.context }}", (*config.With)["context"])

		// Validate outputs
		require.NotNil(t, config.Outputs)
		assert.Equal(t, "{{ .output.analysis }}", (*config.Outputs)["analysis"])
		assert.Equal(t, "{{ .output.security_findings }}", (*config.Outputs)["security_findings"])
		assert.Equal(t, "{{ .output.performance_metrics }}", (*config.Outputs)["performance_metrics"])
		assert.Equal(t, "{{ .output.recommendations }}", (*config.Outputs)["recommendations"])
	})
}

func Test_TaskConfigValidation(t *testing.T) {
	taskID := "test-task"
	taskCWD, err := core.CWDFromPath("/test/path")
	require.NoError(t, err)

	t.Run("Should validate valid basic task", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   taskID,
				Type: TaskTypeBasic,
				CWD:  taskCWD,
			},
			BasicTask: BasicTask{
				Action: "test-action",
			},
		}

		err := config.Validate(t.Context())
		assert.NoError(t, err)
	})

	// New validation cases for agent prompt/action either-or
	t.Run("Agent: action only is valid", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{ID: taskID, Type: TaskTypeBasic, CWD: taskCWD, Agent: &agent.Config{ID: "a"}},
			BasicTask:  BasicTask{Action: "do_thing"},
		}

		err := config.Validate(t.Context())
		assert.NoError(t, err)
	})

	t.Run("Agent: prompt only is valid", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{ID: taskID, Type: TaskTypeBasic, CWD: taskCWD, Agent: &agent.Config{ID: "a"}},
			BasicTask:  BasicTask{Prompt: "Analyze this input"},
		}

		err := config.Validate(t.Context())
		assert.NoError(t, err)
	})

	t.Run("Agent: neither action nor prompt returns error", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{ID: taskID, Type: TaskTypeBasic, CWD: taskCWD, Agent: &agent.Config{ID: "a"}},
		}

		err := config.Validate(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must specify at least one of 'action' or 'prompt'")
	})

	t.Run("Should support combined action and prompt for enhanced context", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{ID: taskID, Type: TaskTypeBasic, CWD: taskCWD, Agent: &agent.Config{ID: "a"}},
			BasicTask:  BasicTask{Action: "analyze", Prompt: "Focus on security issues"},
		}

		err := config.Validate(t.Context())
		assert.NoError(t, err, "Combined action and prompt should be valid for enhanced context")
	})

	t.Run("Should support prompt-only mode without action", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{ID: taskID, Type: TaskTypeBasic, CWD: taskCWD, Agent: &agent.Config{ID: "a"}},
			BasicTask:  BasicTask{Prompt: "Analyze the following text for sentiment"},
		}

		err := config.Validate(t.Context())
		assert.NoError(t, err, "Prompt-only mode should be valid")
	})

	t.Run("Should support action-only mode for backward compatibility", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{ID: taskID, Type: TaskTypeBasic, CWD: taskCWD, Agent: &agent.Config{ID: "a"}},
			BasicTask:  BasicTask{Action: "process"},
		}

		err := config.Validate(t.Context())
		assert.NoError(t, err, "Action-only mode should remain valid for backward compatibility")
	})

	t.Run("Tool: prompt is not allowed", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{ID: taskID, Type: TaskTypeBasic, CWD: taskCWD, Tool: &tool.Config{ID: "t"}},
			BasicTask:  BasicTask{Prompt: "not allowed"},
		}

		err := config.Validate(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "prompt is not allowed when executor type is tool")
	})

	t.Run("Tool: action is not allowed", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{ID: taskID, Type: TaskTypeBasic, CWD: taskCWD, Tool: &tool.Config{ID: "t"}},
			BasicTask:  BasicTask{Action: "also-not-allowed"},
		}
		err := config.Validate(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "action is not allowed when executor type is tool")
	})

	t.Run("Should validate valid router task", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:        taskID,
				Type:      TaskTypeRouter,
				CWD:       taskCWD,
				Condition: "workflow.input.route",
			},
			RouterTask: RouterTask{
				Routes: map[string]any{
					"route1": map[string]any{
						"id": "next1",
					},
				},
			},
		}

		err := config.Validate(t.Context())
		assert.NoError(t, err)
	})

	t.Run("Should validate valid parallel task", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   taskID,
				Type: TaskTypeParallel,
				CWD:  taskCWD,
			},
			Tasks: []Config{
				{
					BaseConfig: BaseConfig{
						ID:    "task1",
						Type:  TaskTypeBasic,
						Agent: &agent.Config{ID: "test_agent"},
						CWD:   taskCWD, // Each sub-task needs a CWD
					},
					BasicTask: BasicTask{
						Action: "test_action",
					},
				},
			},
			ParallelTask: ParallelTask{
				Strategy: StrategyWaitAll,
			},
		}

		err := config.Validate(t.Context())
		assert.NoError(t, err)
	})

	t.Run("Should return error when CWD is missing", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   "test-task",
				Type: TaskTypeBasic,
			},
		}

		err := config.Validate(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "current working directory is required for test-task")
	})

	t.Run("Should return error for invalid task type", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   "test-task",
				Type: "invalid",
				CWD:  taskCWD,
			},
		}

		err := config.Validate(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid task type: invalid")
	})

	t.Run("Should return error for router task missing configuration", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   taskID,
				Type: TaskTypeRouter,
				CWD:  taskCWD,
			},
		}

		err := config.Validate(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "condition is required for router tasks")
	})

	t.Run("Should return error for router task missing routes", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:        "test-task",
				Type:      TaskTypeRouter,
				CWD:       taskCWD,
				Condition: "workflow.input.route",
			},
			RouterTask: RouterTask{
				Routes: map[string]any{},
			},
		}

		err := config.Validate(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "routes are required for router tasks")
	})

	t.Run("Should return error for task with invalid parameters", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   "test-task",
				Type: TaskTypeBasic,
				CWD:  taskCWD,
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
			},
		}

		err := config.ValidateInput(t.Context(), config.With)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Required property 'name' is missing")
	})

	t.Run("Should return error for parallel task with no sub-tasks", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   taskID,
				Type: TaskTypeParallel,
				CWD:  taskCWD,
			},
			Tasks:        []Config{},
			ParallelTask: ParallelTask{},
		}

		err := config.Validate(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parallel tasks must have at least one sub-task")
	})

	t.Run("Should validate valid signal task", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   taskID,
				Type: TaskTypeSignal,
				CWD:  taskCWD,
			},
			SignalTask: SignalTask{
				Signal: &SignalConfig{
					ID: "test-signal",
					Payload: map[string]any{
						"message": "hello world",
					},
				},
			},
		}

		err := config.Validate(t.Context())
		assert.NoError(t, err)
	})

	t.Run("Should return error for signal task missing signal.id", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   taskID,
				Type: TaskTypeSignal,
				CWD:  taskCWD,
			},
			SignalTask: SignalTask{
				Signal: &SignalConfig{
					ID: "",
				},
			},
		}

		err := config.Validate(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "signal.id is required for signal tasks")
	})

	t.Run("Should return error for signal task with whitespace signal.id", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   taskID,
				Type: TaskTypeSignal,
				CWD:  taskCWD,
			},
			SignalTask: SignalTask{
				Signal: &SignalConfig{
					ID: "   ",
				},
			},
		}

		err := config.Validate(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "signal.id is required for signal tasks")
	})

	t.Run("Should return error for parallel task with duplicate IDs", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   taskID,
				Type: TaskTypeParallel,
				CWD:  taskCWD,
			},
			Tasks: []Config{
				{
					BaseConfig: BaseConfig{
						ID:   "duplicate",
						Type: TaskTypeBasic,
					},
				},
				{
					BaseConfig: BaseConfig{
						ID:   "duplicate",
						Type: TaskTypeBasic,
					},
				},
			},
		}

		err := config.Validate(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate task ID in parallel execution: duplicate")
	})

	t.Run("Should return error for invalid parallel task item", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   taskID,
				Type: TaskTypeParallel,
				CWD:  taskCWD,
			},
			Tasks: []Config{
				{
					BaseConfig: BaseConfig{
						ID:   "invalid",
						Type: TaskTypeBasic,
						// Missing required CWD for validation
					},
				},
			},
		}

		err := config.Validate(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid task configuration")
	})

	t.Run("Should return error for circular dependency in parallel tasks", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   "parent_task",
				Type: TaskTypeParallel,
				CWD:  taskCWD,
			},
			Tasks: []Config{
				{
					BaseConfig: BaseConfig{
						ID:   "child_task",
						Type: TaskTypeParallel,
						CWD:  taskCWD,
					},
					Tasks: []Config{
						{
							BaseConfig: BaseConfig{
								ID:   "parent_task", // Creates a cycle
								Type: TaskTypeBasic,
								CWD:  taskCWD,
							},
						},
					},
				},
			},
		}

		err := config.Validate(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "circular dependency detected involving task: parent_task")
	})

	t.Run("Should return error for self-referencing task", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   "self_ref",
				Type: TaskTypeParallel,
				CWD:  taskCWD,
			},
			Tasks: []Config{
				{
					BaseConfig: BaseConfig{
						ID:   "self_ref", // Same ID as parent creates cycle
						Type: TaskTypeBasic,
						CWD:  taskCWD,
					},
					BasicTask: BasicTask{
						Action: "test",
					},
				},
			},
		}

		err := config.Validate(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "circular dependency detected involving task: self_ref")
	})

	t.Run("Should allow valid nested parallel tasks without cycles", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   "parent_task",
				Type: TaskTypeParallel,
				CWD:  taskCWD,
			},
			Tasks: []Config{
				{
					BaseConfig: BaseConfig{
						ID:   "child_task",
						Type: TaskTypeParallel,
						CWD:  taskCWD,
					},
					Tasks: []Config{
						{
							BaseConfig: BaseConfig{
								ID:   "grandchild_task",
								Type: TaskTypeBasic,
								CWD:  taskCWD,
							},
							BasicTask: BasicTask{
								Action: "test",
							},
						},
					},
				},
			},
		}

		err := config.Validate(t.Context())
		assert.NoError(t, err)
	})

	t.Run("Should validate valid collection task", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   taskID,
				Type: TaskTypeCollection,
				CWD:  taskCWD,
			},
			CollectionConfig: CollectionConfig{
				Items: `["item1", "item2", "item3"]`,
			},
			Task: &Config{
				BaseConfig: BaseConfig{
					ID:   "template-task",
					Type: TaskTypeBasic,
					CWD:  taskCWD,
				},
				BasicTask: BasicTask{
					Action: "process {{ .item }}",
				},
			},
		}

		err := config.Validate(t.Context())
		assert.NoError(t, err)
	})
	t.Run("Should validate collection task with sequential mode", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   taskID,
				Type: TaskTypeCollection,
				CWD:  taskCWD,
			},
			CollectionConfig: CollectionConfig{
				Items: `["item1", "item2", "item3"]`,
				Mode:  CollectionModeSequential,
			},
			Task: &Config{
				BaseConfig: BaseConfig{
					ID:   "template-task",
					Type: TaskTypeBasic,
					CWD:  taskCWD,
				},
				BasicTask: BasicTask{
					Action: "process {{ .item }}",
				},
			},
		}
		err := config.Validate(t.Context())
		assert.NoError(t, err)
		assert.Equal(t, CollectionModeSequential, config.GetMode())
	})
	t.Run("Should default to parallel mode when mode is not specified", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   taskID,
				Type: TaskTypeCollection,
				CWD:  taskCWD,
			},
			CollectionConfig: CollectionConfig{
				Items: `["item1", "item2", "item3"]`,
			},
		}
		// Apply defaults
		config.Default()
		assert.Equal(t, CollectionModeParallel, config.GetMode())
	})

	t.Run("Should return error for collection task missing items", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   taskID,
				Type: TaskTypeCollection,
				CWD:  taskCWD,
			},
			CollectionConfig: CollectionConfig{
				Items: "", // Missing items
			},
			Task: &Config{
				BaseConfig: BaseConfig{ID: "template", Type: TaskTypeBasic, CWD: taskCWD},
			},
		}

		err := config.Validate(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "items field is required")
	})

	t.Run("Should return error for collection task with invalid mode", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   taskID,
				Type: TaskTypeCollection,
				CWD:  taskCWD,
			},
			CollectionConfig: CollectionConfig{
				Items: `["item1", "item2"]`,
				Mode:  "invalid-mode",
			},
			Task: &Config{
				BaseConfig: BaseConfig{ID: "template", Type: TaskTypeBasic, CWD: taskCWD},
			},
		}

		err := config.Validate(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid mode 'invalid-mode'")
	})

	t.Run("Should return error for collection task with negative batch", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   taskID,
				Type: TaskTypeCollection,
				CWD:  taskCWD,
			},
			CollectionConfig: CollectionConfig{
				Items: `["item1", "item2"]`,
				Batch: -1,
			},
			Task: &Config{
				BaseConfig: BaseConfig{ID: "template", Type: TaskTypeBasic, CWD: taskCWD},
			},
		}

		err := config.Validate(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "batch size cannot be negative")
	})

	t.Run("Should return error for collection task with both task and tasks", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   taskID,
				Type: TaskTypeCollection,
				CWD:  taskCWD,
			},
			CollectionConfig: CollectionConfig{
				Items: `["item1", "item2"]`,
			},
			Task: &Config{
				BaseConfig: BaseConfig{ID: "template", Type: TaskTypeBasic, CWD: taskCWD},
			},
			Tasks: []Config{
				{BaseConfig: BaseConfig{ID: "task1", Type: TaskTypeBasic, CWD: taskCWD}},
			},
		}

		err := config.Validate(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot have both 'task' template and 'tasks' array configured")
	})

	t.Run("Should return error for collection task with neither task nor tasks", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   taskID,
				Type: TaskTypeCollection,
				CWD:  taskCWD,
			},
			CollectionConfig: CollectionConfig{
				Items: `["item1", "item2"]`,
			},
			ParallelTask: ParallelTask{
				// No Task or Tasks
			},
		}

		err := config.Validate(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must have either a 'task' template or 'tasks' array configured")
	})
}

func Test_TaskConfigCWD(t *testing.T) {
	t.Run("Should handle CWD operations correctly", func(t *testing.T) {
		config := &Config{}
		assert.Nil(t, config.GetCWD())

		config.SetCWD("/test/path")
		assert.Equal(t, "/test/path", config.GetCWD().PathStr())

		config.SetCWD("/new/path")
		assert.Equal(t, "/new/path", config.GetCWD().PathStr())
	})
}

func Test_TaskConfigMerge(t *testing.T) {
	t.Run("Should merge configurations correctly", func(t *testing.T) {
		next1 := "next1"
		next2 := "next2"
		base := &Config{
			BaseConfig: BaseConfig{
				Env: &core.EnvMap{
					"KEY1": "value1",
				},
				With: &core.Input{
					"param1": "value1",
				},
				OnSuccess: &core.SuccessTransition{
					Next: &next1,
				},
				OnError: &core.ErrorTransition{
					Next: &next1,
				},
			},
		}

		other := &Config{
			BaseConfig: BaseConfig{
				Env: &core.EnvMap{
					"KEY2": "value2",
				},
				With: &core.Input{
					"param2": "value2",
				},
				OnSuccess: &core.SuccessTransition{
					Next: &next2,
				},
				OnError: &core.ErrorTransition{
					Next: &next2,
				},
			},
		}

		err := base.Merge(other)
		require.NoError(t, err)

		// Check merged values
		assert.Equal(t, "value1", base.GetEnv().Prop("KEY1"))
		assert.Equal(t, "value2", base.GetEnv().Prop("KEY2"))
		assert.Equal(t, "value1", (*base.With)["param1"])
		assert.Equal(t, "value2", (*base.With)["param2"])
		assert.Equal(t, "next2", *base.OnSuccess.Next)
		assert.Equal(t, "next2", *base.OnError.Next)
	})
}

func Test_TaskReference(t *testing.T) {
	t.Run("Should load task reference correctly", func(t *testing.T) {
		CWD, dstPath := setupTest(t, "ref_task.yaml")
		// Run the test
		config, err := Load(t.Context(), CWD, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the config
		err = config.Validate(t.Context())
		require.NoError(t, err)

		require.NotNil(t, config.ID)
		require.NotNil(t, config.Type)
		require.NotNil(t, config.Action)
		require.NotNil(t, config.With)

		assert.Equal(t, "code-format", config.ID)
		assert.Equal(t, TaskTypeBasic, config.Type)
		assert.Equal(t, "format", config.Action)

		// Validate the with parameters
		assert.Equal(t, "console.log('hello')", (*config.With)["code"])
		assert.Equal(t, "javascript", (*config.With)["language"])
		assert.Equal(t, 2, (*config.With)["indent_size"])
		assert.Equal(t, false, (*config.With)["use_tabs"])

		// In the new ID-based approach, $use directives are rejected at load time.
		// This fixture no longer uses $use, so Agent may be nil here.
		assert.Nil(t, config.GetAgent())
	})
}

func Test_ValidateStrategy(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
		expected bool
	}{
		{
			name:     "wait_all should be valid",
			strategy: "wait_all",
			expected: true,
		},
		{
			name:     "fail_fast should be valid",
			strategy: "fail_fast",
			expected: true,
		},
		{
			name:     "best_effort should be valid",
			strategy: "best_effort",
			expected: true,
		},
		{
			name:     "race should be valid",
			strategy: "race",
			expected: true,
		},
		{
			name:     "invalid_strategy should be invalid",
			strategy: "invalid_strategy",
			expected: false,
		},
		{
			name:     "empty string should be invalid",
			strategy: "",
			expected: false,
		},
		{
			name:     "wait-all (with hyphen) should be invalid",
			strategy: "wait-all",
			expected: false,
		},
		{
			name:     "WAIT_ALL (uppercase) should be invalid",
			strategy: "WAIT_ALL",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateStrategy(tt.strategy)
			assert.Equal(
				t,
				tt.expected,
				result,
				"ValidateStrategy(%q) = %v, expected %v",
				tt.strategy,
				result,
				tt.expected,
			)
		})
	}
}

func TestAggregate_TaskValidation(t *testing.T) {
	t.Run("Should validate aggregate task with outputs", func(t *testing.T) {
		cwd, err := core.CWDFromPath("/test")
		require.NoError(t, err)
		outputs := &core.Input{
			"total": "{{ add .tasks.task1.output.count .tasks.task2.output.count }}",
			"summary": map[string]any{
				"task1_result": "{{ .tasks.task1.output }}",
				"task2_result": "{{ .tasks.task2.output }}",
			},
		}
		config := &Config{
			BaseConfig: BaseConfig{
				ID:      "aggregate-task",
				Type:    TaskTypeAggregate,
				Outputs: outputs,
				CWD:     cwd,
			},
		}
		err = config.Validate(t.Context())
		assert.NoError(t, err)
	})
	t.Run("Should fail validation when aggregate task has no outputs", func(t *testing.T) {
		cwd, err := core.CWDFromPath("/test")
		require.NoError(t, err)
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   "aggregate-task",
				Type: TaskTypeAggregate,
				CWD:  cwd,
			},
		}
		err = config.Validate(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "aggregate tasks must have outputs defined")
	})
	t.Run("Should fail validation when aggregate task has action", func(t *testing.T) {
		cwd, err := core.CWDFromPath("/test")
		require.NoError(t, err)
		outputs := &core.Input{
			"result": "{{ .tasks.task1.output }}",
		}
		config := &Config{
			BasicTask: BasicTask{
				Action: "some action",
			},
			BaseConfig: BaseConfig{
				ID:      "aggregate-task",
				Type:    TaskTypeAggregate,
				Outputs: outputs,
				CWD:     cwd,
			},
		}
		err = config.Validate(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "aggregate tasks cannot have an action field")
	})
	t.Run("Should map aggregate task to ExecutionBasic", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				Type: TaskTypeAggregate,
			},
		}
		execType := config.GetExecType()
		assert.Equal(t, ExecutionBasic, execType)
	})
}

func TestAggregate_LoadTask(t *testing.T) {
	t.Run("Should load aggregate task from YAML", func(t *testing.T) {
		cwd, dstPath := setupTest(t, "aggregate_task.yaml")
		// Load the aggregate task
		config, err := Load(t.Context(), cwd, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)
		// Verify task type
		assert.Equal(t, TaskTypeAggregate, config.Type)
		assert.Equal(t, "aggregate-results", config.ID)
		// Verify outputs are loaded
		require.NotNil(t, config.Outputs)
		outputs := *config.Outputs
		// Check simple field
		assert.Contains(t, outputs, "total_count")
		assert.Equal(t, "{{ add .tasks.task1.output.count .tasks.task2.output.count }}", outputs["total_count"])
		// Check complex aggregation
		assert.Contains(t, outputs, "summary")
		// YAML loading may preserve the structure differently
		// Let's just check that summary exists and is not nil
		assert.NotNil(t, outputs["summary"])
		// Check conditional logic
		assert.Contains(t, outputs, "status")
		assert.Equal(
			t,
			"{{ if gt .tasks.task2.output.failed_count 0 }}partial_success{{ else }}success{{ end }}",
			outputs["status"],
		)
		// Check transitions
		require.NotNil(t, config.OnSuccess)
		assert.Equal(t, "notify-completion", *config.OnSuccess.Next)
		require.NotNil(t, config.OnError)
		assert.Equal(t, "handle-aggregation-error", *config.OnError.Next)
		// Validate the loaded config
		err = config.Validate(t.Context())
		assert.NoError(t, err)
	})
}

func TestCompositeTask(t *testing.T) {
	// Create a test CWD for all tests
	cwd, err := core.CWDFromPath("/test")
	require.NoError(t, err)

	t.Run("Should create valid composite task configuration", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   "user-onboarding",
				Type: TaskTypeComposite,
				CWD:  cwd,
			},
			Tasks: []Config{
				{
					BaseConfig: BaseConfig{
						ID:   "create-profile",
						Type: TaskTypeBasic,
						CWD:  cwd,
					},
					BasicTask: BasicTask{
						Action: "create_profile",
					},
				},
				{
					BaseConfig: BaseConfig{
						ID:   "send-welcome-email",
						Type: TaskTypeBasic,
						CWD:  cwd,
					},
					BasicTask: BasicTask{
						Action: "send_email",
					},
				},
				{
					BaseConfig: BaseConfig{
						ID:   "setup-preferences",
						Type: TaskTypeBasic,
						CWD:  cwd,
					},
					BasicTask: BasicTask{
						Action: "setup_preferences",
					},
				},
			},
			// Composite tasks should not have ParallelTask fields set
		}

		// Test execution type
		assert.Equal(t, ExecutionComposite, config.GetExecType())

		// Test that it's a composite type
		assert.Equal(t, TaskTypeComposite, config.Type)

		// Validate the configuration
		err := config.Validate(t.Context())
		require.NoError(t, err)
	})

	t.Run("Should fail validation for composite task with no subtasks", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   "empty-composite",
				Type: TaskTypeComposite,
				CWD:  cwd,
			},
			Tasks: []Config{},
		}

		err := config.Validate(t.Context())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "composite tasks must have at least one sub-task")
	})

	t.Run("Should support nested container tasks in composite", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   "valid-nested-composite",
				Type: TaskTypeComposite,
				CWD:  cwd,
			},
			Tasks: []Config{
				{
					BaseConfig: BaseConfig{
						ID:   "nested-parallel",
						Type: TaskTypeParallel,
						CWD:  cwd,
					},
					Tasks: []Config{
						{
							BaseConfig: BaseConfig{
								ID:   "nested-basic",
								Type: TaskTypeBasic,
								CWD:  cwd,
							},
							BasicTask: BasicTask{
								Action: "test",
							},
						},
					},
					// Nested parallel task can have its own strategy
					ParallelTask: ParallelTask{
						Strategy: StrategyWaitAll,
					},
				},
			},
			// Composite tasks should not have ParallelTask fields set
		}

		err := config.Validate(t.Context())
		require.NoError(t, err)
	})

	t.Run("Should validate composite tasks run sequentially without strategies", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   "sequential-composite",
				Type: TaskTypeComposite,
				CWD:  cwd,
			},
			Tasks: []Config{
				{
					BaseConfig: BaseConfig{
						ID:   "step1",
						Type: TaskTypeBasic,
						CWD:  cwd,
					},
					BasicTask: BasicTask{
						Action: "first_step",
					},
				},
				{
					BaseConfig: BaseConfig{
						ID:   "step2",
						Type: TaskTypeBasic,
						CWD:  cwd,
					},
					BasicTask: BasicTask{
						Action: "second_step",
					},
				},
			},
			// Composite tasks should not have ParallelTask fields set
		}

		err := config.Validate(t.Context())
		require.NoError(t, err)

		// Verify that composite tasks don't have strategy-related behavior
		assert.Equal(t, TaskTypeComposite, config.Type)
		assert.Equal(t, ExecutionComposite, config.GetExecType())
	})
}

func Test_ConfigSerializationWithPublicFields(t *testing.T) {
	t.Run("Should serialize and deserialize FilePath and CWD correctly with JSON", func(t *testing.T) {
		// Create a config with all fields set
		CWD, err := core.CWDFromPath("/test/working/directory")
		require.NoError(t, err)

		originalConfig := &Config{
			BaseConfig: BaseConfig{
				ID:       "test-task",
				Type:     TaskTypeBasic,
				FilePath: "/test/config/file.yaml",
				CWD:      CWD,
				Config: core.GlobalOpts{
					StartToCloseTimeout: "5m",
				},
				With: &core.Input{
					"param1": "value1",
				},
			},
			BasicTask: BasicTask{
				Action: "test-action",
			},
		}

		// Serialize to JSON
		jsonData, err := json.Marshal(originalConfig)
		require.NoError(t, err)

		// Deserialize from JSON
		var deserializedConfig Config
		err = json.Unmarshal(jsonData, &deserializedConfig)
		require.NoError(t, err)

		// Verify all fields are preserved
		assert.Equal(t, originalConfig.ID, deserializedConfig.ID)
		assert.Equal(t, originalConfig.Type, deserializedConfig.Type)
		assert.Equal(t, originalConfig.FilePath, deserializedConfig.FilePath)
		assert.NotNil(t, deserializedConfig.CWD)
		assert.Equal(t, originalConfig.CWD.PathStr(), deserializedConfig.CWD.PathStr())
		assert.Equal(t, originalConfig.Action, deserializedConfig.Action)
		assert.Equal(t, originalConfig.Config.StartToCloseTimeout, deserializedConfig.Config.StartToCloseTimeout)
		assert.Equal(t, (*originalConfig.With)["param1"], (*deserializedConfig.With)["param1"])
	})

	t.Run("Should serialize and deserialize FilePath and CWD correctly with YAML", func(t *testing.T) {
		// Create a config with all fields set
		CWD, err := core.CWDFromPath("/test/working/directory")
		require.NoError(t, err)

		originalConfig := &Config{
			BaseConfig: BaseConfig{
				ID:       "test-task",
				Type:     TaskTypeBasic,
				FilePath: "/test/config/file.yaml",
				CWD:      CWD,
				Config: core.GlobalOpts{
					StartToCloseTimeout: "5m",
				},
				With: &core.Input{
					"param1": "value1",
				},
			},
			BasicTask: BasicTask{
				Action: "test-action",
			},
		}

		// Serialize to YAML
		yamlData, err := yaml.Marshal(originalConfig)
		require.NoError(t, err)

		// Deserialize from YAML
		var deserializedConfig Config
		err = yaml.Unmarshal(yamlData, &deserializedConfig)
		require.NoError(t, err)

		// Verify all fields are preserved
		assert.Equal(t, originalConfig.ID, deserializedConfig.ID)
		assert.Equal(t, originalConfig.Type, deserializedConfig.Type)
		assert.Equal(t, originalConfig.FilePath, deserializedConfig.FilePath)
		assert.NotNil(t, deserializedConfig.CWD)
		assert.Equal(t, originalConfig.CWD.PathStr(), deserializedConfig.CWD.PathStr())
		assert.Equal(t, originalConfig.Action, deserializedConfig.Action)
		assert.Equal(t, originalConfig.Config.StartToCloseTimeout, deserializedConfig.Config.StartToCloseTimeout)
		assert.Equal(t, (*originalConfig.With)["param1"], (*deserializedConfig.With)["param1"])
	})

	t.Run("Should handle nil CWD correctly during serialization", func(t *testing.T) {
		originalConfig := &Config{
			BaseConfig: BaseConfig{
				ID:       "test-task",
				Type:     TaskTypeBasic,
				FilePath: "/test/config/file.yaml",
				CWD:      nil, // Explicitly nil
			},
			BasicTask: BasicTask{
				Action: "test-action",
			},
		}

		// Serialize to JSON
		jsonData, err := json.Marshal(originalConfig)
		require.NoError(t, err)

		// Deserialize from JSON
		var deserializedConfig Config
		err = json.Unmarshal(jsonData, &deserializedConfig)
		require.NoError(t, err)

		// Verify CWD is still nil
		assert.Nil(t, deserializedConfig.CWD)
		assert.Equal(t, originalConfig.FilePath, deserializedConfig.FilePath)
	})

	t.Run("Should work correctly with Redis-like serialization scenario", func(t *testing.T) {
		// Create a complex config similar to what would be stored in Redis
		CWD, err := core.CWDFromPath("/project/root")
		require.NoError(t, err)

		originalConfig := &Config{
			BaseConfig: BaseConfig{
				ID:       "parallel-task",
				Type:     TaskTypeParallel,
				FilePath: "/project/tasks/parallel.yaml",
				CWD:      CWD,
			},
			ParallelTask: ParallelTask{
				Strategy:   StrategyWaitAll,
				MaxWorkers: 5,
			},
			Tasks: []Config{
				{
					BaseConfig: BaseConfig{
						ID:       "child-1",
						Type:     TaskTypeBasic,
						FilePath: "/project/tasks/child1.yaml",
						CWD:      CWD,
					},
					BasicTask: BasicTask{
						Action: "process",
					},
				},
				{
					BaseConfig: BaseConfig{
						ID:       "child-2",
						Type:     TaskTypeBasic,
						FilePath: "/project/tasks/child2.yaml",
						CWD:      CWD,
					},
					BasicTask: BasicTask{
						Action: "transform",
					},
				},
			},
		}

		// Simulate Redis storage: serialize to JSON
		jsonData, err := json.Marshal(originalConfig)
		require.NoError(t, err)

		// Simulate Redis retrieval: deserialize from JSON
		var retrievedConfig Config
		err = json.Unmarshal(jsonData, &retrievedConfig)
		require.NoError(t, err)

		// Verify parent config
		assert.Equal(t, originalConfig.ID, retrievedConfig.ID)
		assert.Equal(t, originalConfig.Type, retrievedConfig.Type)
		assert.Equal(t, originalConfig.FilePath, retrievedConfig.FilePath)
		assert.NotNil(t, retrievedConfig.CWD)
		assert.Equal(t, originalConfig.CWD.PathStr(), retrievedConfig.CWD.PathStr())

		// Verify child configs
		assert.Len(t, retrievedConfig.Tasks, 2)
		for i, childTask := range retrievedConfig.Tasks {
			assert.Equal(t, originalConfig.Tasks[i].ID, childTask.ID)
			assert.Equal(t, originalConfig.Tasks[i].FilePath, childTask.FilePath)
			assert.NotNil(t, childTask.CWD)
			assert.Equal(t, originalConfig.Tasks[i].CWD.PathStr(), childTask.CWD.PathStr())
			assert.Equal(t, originalConfig.Tasks[i].Action, childTask.Action)
		}
	})
}

func TestWaitTaskConfig_YAMLParsing(t *testing.T) {
	t.Run("Should parse minimal wait task configuration", func(t *testing.T) {
		yamlContent := `
id: wait-for-approval
type: wait
wait_for: approval_signal
condition: 'signal.payload.status == "approved"'
timeout: 1h
on_success:
  next: approved_task
on_error:
  next: rejected_task
`
		var config Config
		err := yaml.Unmarshal([]byte(yamlContent), &config)
		require.NoError(t, err)
		assert.Equal(t, "wait-for-approval", config.ID)
		assert.Equal(t, TaskTypeWait, config.Type)
		assert.Equal(t, "approval_signal", config.WaitFor)
		assert.Equal(t, `signal.payload.status == "approved"`, config.Condition)
		assert.Equal(t, "1h", config.Timeout)
		assert.NotNil(t, config.OnSuccess)
		assert.Equal(t, "approved_task", *config.OnSuccess.Next)
		assert.NotNil(t, config.OnError)
		assert.Equal(t, "rejected_task", *config.OnError.Next)
	})
	t.Run("Should parse wait task with processor configuration", func(t *testing.T) {
		yamlContent := `
id: wait-with-processor
type: wait
wait_for: data_signal
condition: 'processor.output.valid && processor.output.score > 0.8'
processor:
  id: validate_data
  type: basic
  $use: tool(local::tools.#(id=="validator"))
  with:
    input_data: "{{ .signal.payload }}"
    threshold: 0.8
timeout: 2h
on_timeout: timeout_handler
on_success:
  next: process_data
on_error:
  next: handle_error
`
		var config Config
		err := yaml.Unmarshal([]byte(yamlContent), &config)
		require.NoError(t, err)
		assert.Equal(t, "wait-with-processor", config.ID)
		assert.Equal(t, TaskTypeWait, config.Type)
		assert.Equal(t, "data_signal", config.WaitFor)
		assert.Equal(t, `processor.output.valid && processor.output.score > 0.8`, config.Condition)
		assert.Equal(t, "timeout_handler", config.OnTimeout)
		require.NotNil(t, config.Processor)
		assert.Equal(t, "validate_data", config.Processor.ID)
		assert.Equal(t, TaskTypeBasic, config.Processor.Type)
		assert.NotNil(t, config.Processor.With)
		inputData := (*config.Processor.With)["input_data"]
		threshold := (*config.Processor.With)["threshold"]
		assert.Equal(t, "{{ .signal.payload }}", inputData)
		assert.Equal(t, float64(0.8), threshold)
	})
	t.Run("Should serialize wait task configuration to YAML", func(t *testing.T) {
		config := Config{
			BaseConfig: BaseConfig{
				ID:        "wait-task",
				Type:      TaskTypeWait,
				Timeout:   "30m",
				Condition: `signal.action == "continue"`,
			},
			WaitTask: WaitTask{
				WaitFor:   "user_signal",
				OnTimeout: "handle_timeout",
			},
		}
		data, err := yaml.Marshal(&config)
		require.NoError(t, err)
		yamlStr := string(data)
		assert.Contains(t, yamlStr, "id: wait-task")
		assert.Contains(t, yamlStr, "type: wait")
		assert.Contains(t, yamlStr, "wait_for: user_signal")
		assert.Contains(t, yamlStr, `signal.action == "continue"`)
		assert.Contains(t, yamlStr, "timeout: 30m")
		assert.Contains(t, yamlStr, "on_timeout: handle_timeout")
	})
	t.Run("Should handle processor with BaseConfig fields", func(t *testing.T) {
		yamlContent := `
id: wait-processor-timeout
type: wait
wait_for: processing_signal
condition: 'processor.output.success'
processor:
  id: processor_with_timeout
  type: basic
  timeout: 10s
  retries: 3
  $use: tool(local::tools.#(id=="processor"))
`
		var config Config
		err := yaml.Unmarshal([]byte(yamlContent), &config)
		require.NoError(t, err)
		require.NotNil(t, config.Processor)
		assert.Equal(t, "processor_with_timeout", config.Processor.ID)
		assert.Equal(t, "10s", config.Processor.Timeout)
		assert.Equal(t, 3, config.Processor.Retries)
	})
}

func TestWaitTaskConfig_GetExecType(t *testing.T) {
	t.Run("Should return ExecutionWait for wait task type", func(t *testing.T) {
		config := Config{
			BaseConfig: BaseConfig{
				Type: TaskTypeWait,
			},
		}
		execType := config.GetExecType()
		assert.Equal(t, ExecutionWait, execType)
	})
}

func TestConfig_validateWaitTask(t *testing.T) {
	t.Run("Should validate basic wait task configuration", func(t *testing.T) {
		CWD, _ := core.CWDFromPath("/tmp")
		config := &Config{
			BaseConfig: BaseConfig{
				ID:        "test-wait",
				Type:      TaskTypeWait,
				CWD:       CWD,
				Condition: `signal.payload.status == "approved"`,
			},
			WaitTask: WaitTask{
				WaitFor: "approval_signal",
			},
		}
		err := config.validateWaitTask(t.Context())
		assert.NoError(t, err)
	})

	t.Run("Should require wait_for field", func(t *testing.T) {
		CWD, _ := core.CWDFromPath("/tmp")
		config := &Config{
			BaseConfig: BaseConfig{
				ID:        "test-wait",
				Type:      TaskTypeWait,
				CWD:       CWD,
				Condition: `signal.payload.status == "approved"`,
			},
		}
		err := config.validateWaitTask(t.Context())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "wait_for field is required")
	})

	t.Run("Should require condition field", func(t *testing.T) {
		CWD, _ := core.CWDFromPath("/tmp")
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   "test-wait",
				Type: TaskTypeWait,
				CWD:  CWD,
				// Missing Condition field
			},
			WaitTask: WaitTask{
				WaitFor: "approval_signal",
			},
		}
		err := config.validateWaitTask(t.Context())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "condition field is required")
	})

	t.Run("Should validate CEL expression syntax", func(t *testing.T) {
		CWD, _ := core.CWDFromPath("/tmp")
		config := &Config{
			BaseConfig: BaseConfig{
				ID:        "test-wait",
				Type:      TaskTypeWait,
				CWD:       CWD,
				Condition: `signal.payload.status ==`, // Invalid syntax
			},
			WaitTask: WaitTask{
				WaitFor: "approval_signal",
			},
		}
		err := config.validateWaitTask(t.Context())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid condition")
	})

	t.Run("Should validate timeout format", func(t *testing.T) {
		CWD, _ := core.CWDFromPath("/tmp")
		config := &Config{
			BaseConfig: BaseConfig{
				ID:        "test-wait",
				Type:      TaskTypeWait,
				CWD:       CWD,
				Timeout:   "invalid-timeout",
				Condition: `signal.payload.status == "approved"`,
			},
			WaitTask: WaitTask{
				WaitFor: "approval_signal",
			},
		}
		err := config.validateWaitTask(t.Context())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid timeout")
	})

	t.Run("Should validate positive timeout value", func(t *testing.T) {
		CWD, _ := core.CWDFromPath("/tmp")
		config := &Config{
			BaseConfig: BaseConfig{
				ID:        "test-wait",
				Type:      TaskTypeWait,
				CWD:       CWD,
				Timeout:   "0s",
				Condition: `signal.payload.status == "approved"`,
			},
			WaitTask: WaitTask{
				WaitFor: "approval_signal",
			},
		}
		err := config.validateWaitTask(t.Context())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "timeout must be positive")
	})

	t.Run("Should accept valid timeout values", func(t *testing.T) {
		CWD, _ := core.CWDFromPath("/tmp")
		testCases := []string{"1h", "30m", "5s", "2 hours", "10 minutes"}
		for _, timeout := range testCases {
			config := &Config{
				BaseConfig: BaseConfig{
					ID:        "test-wait",
					Type:      TaskTypeWait,
					CWD:       CWD,
					Timeout:   timeout,
					Condition: `signal.payload.status == "approved"`,
				},
				WaitTask: WaitTask{
					WaitFor: "approval_signal",
				},
			}
			err := config.validateWaitTask(t.Context())
			assert.NoError(t, err, "timeout %s should be valid", timeout)
		}
	})

	t.Run("Should validate processor configuration", func(t *testing.T) {
		CWD, _ := core.CWDFromPath("/tmp")
		config := &Config{
			BaseConfig: BaseConfig{
				ID:        "test-wait",
				Type:      TaskTypeWait,
				CWD:       CWD,
				Condition: `signal.payload.status == "approved"`,
			},
			WaitTask: WaitTask{
				WaitFor: "approval_signal",
				Processor: &Config{
					BaseConfig: BaseConfig{
						// Missing ID and Type
						CWD: CWD,
					},
				},
			},
		}
		err := config.validateWaitTask(t.Context())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "processor ID is required")
	})

	t.Run("Should validate processor type", func(t *testing.T) {
		CWD, _ := core.CWDFromPath("/tmp")
		config := &Config{
			BaseConfig: BaseConfig{
				ID:        "test-wait",
				Type:      TaskTypeWait,
				CWD:       CWD,
				Condition: `signal.payload.status == "approved"`,
			},
			WaitTask: WaitTask{
				WaitFor: "approval_signal",
				Processor: &Config{
					BaseConfig: BaseConfig{
						ID:  "processor1",
						CWD: CWD,
						// Missing Type
					},
				},
			},
		}
		err := config.validateWaitTask(t.Context())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "processor type is required")
	})

	t.Run("Should validate complete processor configuration", func(t *testing.T) {
		CWD, _ := core.CWDFromPath("/tmp")
		config := &Config{
			BaseConfig: BaseConfig{
				ID:        "test-wait",
				Type:      TaskTypeWait,
				CWD:       CWD,
				Condition: `signal.payload.status == "approved"`,
			},
			WaitTask: WaitTask{
				WaitFor: "approval_signal",
				Processor: &Config{
					BaseConfig: BaseConfig{
						ID:   "processor1",
						Type: TaskTypeBasic,
						CWD:  CWD,
					},
				},
			},
		}
		err := config.validateWaitTask(t.Context())
		assert.NoError(t, err)
	})

	t.Run("Should allow optional timeout", func(t *testing.T) {
		CWD, _ := core.CWDFromPath("/tmp")
		config := &Config{
			BaseConfig: BaseConfig{
				ID:        "test-wait",
				Type:      TaskTypeWait,
				CWD:       CWD,
				Condition: `signal.payload.status == "approved"`,
				// No timeout specified
			},
			WaitTask: WaitTask{
				WaitFor: "approval_signal",
			},
		}
		err := config.validateWaitTask(t.Context())
		assert.NoError(t, err)
	})

	t.Run("Should allow optional processor", func(t *testing.T) {
		CWD, _ := core.CWDFromPath("/tmp")
		config := &Config{
			BaseConfig: BaseConfig{
				ID:        "test-wait",
				Type:      TaskTypeWait,
				CWD:       CWD,
				Condition: `signal.payload.status == "approved"`,
			},
			WaitTask: WaitTask{
				WaitFor: "approval_signal",
				// No processor specified
			},
		}
		err := config.validateWaitTask(t.Context())
		assert.NoError(t, err)
	})
}

func TestConfig_validateWaitCondition(t *testing.T) {
	t.Run("Should validate valid CEL expressions", func(t *testing.T) {
		CWD, _ := core.CWDFromPath("/tmp")
		config := &Config{
			BaseConfig: BaseConfig{
				ID:        "test-wait",
				Type:      TaskTypeWait,
				CWD:       CWD,
				Condition: `signal.payload.status == "approved"`,
			},
		}
		validExpressions := []string{
			`signal.payload.status == "approved"`,
			`signal.payload.value > 100`,
			`has(signal.payload.field) && signal.payload.field != ""`,
			`processor.output.valid && processor.output.score > 0.8`,
			`size(signal.payload.items) > 0`,
		}
		for _, expr := range validExpressions {
			config.Condition = expr
			err := config.validateWaitCondition(t.Context())
			assert.NoError(t, err, "expression should be valid: %s", expr)
		}
	})

	t.Run("Should reject invalid CEL expressions", func(t *testing.T) {
		CWD, _ := core.CWDFromPath("/tmp")
		config := &Config{
			BaseConfig: BaseConfig{
				ID:        "test-wait",
				Type:      TaskTypeWait,
				CWD:       CWD,
				Condition: `signal.payload.status == "approved"`,
			},
		}
		invalidExpressions := []string{
			`signal.payload.status ==`,       // Incomplete expression
			`signal.payload.status == `,      // Missing value
			`invalid syntax here`,            // Invalid syntax
			`signal.payload.status = "test"`, // Assignment instead of comparison
		}
		for _, expr := range invalidExpressions {
			config.Condition = expr
			err := config.validateWaitCondition(t.Context())
			assert.Error(t, err, "expression should be invalid: %s", expr)
		}
	})
}

func TestConfig_validateWaitTimeout(t *testing.T) {
	t.Run("Should accept empty timeout", func(t *testing.T) {
		CWD, _ := core.CWDFromPath("/tmp")
		config := &Config{
			BaseConfig: BaseConfig{
				ID:        "test-wait",
				Type:      TaskTypeWait,
				CWD:       CWD,
				Condition: `signal.payload.status == "approved"`,
			},
		}
		err := config.validateWaitTimeout(t.Context())
		assert.NoError(t, err)
	})

	t.Run("Should allow templated timeout for runtime resolution", func(t *testing.T) {
		CWD, _ := core.CWDFromPath("/tmp")
		config := &Config{
			BaseConfig: BaseConfig{
				ID:        "test-wait",
				Type:      TaskTypeWait,
				CWD:       CWD,
				Condition: `signal.payload.status == "approved"`,
				Timeout:   "{{ .tasks.calculate_delay.output.duration }}",
			},
		}
		err := config.validateWaitTimeout(t.Context())
		assert.NoError(t, err)
	})

	t.Run("Should validate duration formats", func(t *testing.T) {
		CWD, _ := core.CWDFromPath("/tmp")
		config := &Config{
			BaseConfig: BaseConfig{
				ID:        "test-wait",
				Type:      TaskTypeWait,
				CWD:       CWD,
				Condition: `signal.payload.status == "approved"`,
			},
		}
		validTimeouts := []string{
			"1s", "30s", "5m", "1h", "2h30m",
			"1 second", "30 seconds", "5 minutes", "1 hour", "2 hours",
		}
		for _, timeout := range validTimeouts {
			config.Timeout = timeout
			err := config.validateWaitTimeout(t.Context())
			assert.NoError(t, err, "timeout should be valid: %s", timeout)
		}
	})

	t.Run("Should reject invalid timeout formats", func(t *testing.T) {
		CWD, _ := core.CWDFromPath("/tmp")
		config := &Config{
			BaseConfig: BaseConfig{
				ID:        "test-wait",
				Type:      TaskTypeWait,
				CWD:       CWD,
				Condition: `signal.payload.status == "approved"`,
			},
		}
		invalidTimeouts := []string{
			"invalid", "1x", "abc", "-1s", "0s",
		}
		for _, timeout := range invalidTimeouts {
			config.Timeout = timeout
			err := config.validateWaitTimeout(t.Context())
			assert.Error(t, err, "timeout should be invalid: %s", timeout)
		}
	})
}

func TestConfig_validateWaitProcessor(t *testing.T) {
	t.Run("Should require processor ID", func(t *testing.T) {
		CWD, _ := core.CWDFromPath("/tmp")
		config := &Config{
			BaseConfig: BaseConfig{
				ID:        "test-wait",
				Type:      TaskTypeWait,
				CWD:       CWD,
				Condition: `signal.payload.status == "approved"`,
			},
			WaitTask: WaitTask{
				Processor: &Config{
					BaseConfig: BaseConfig{
						Type: TaskTypeBasic,
						CWD:  CWD,
					},
				},
			},
		}
		err := config.validateWaitProcessor(t.Context())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "processor ID is required")
	})

	t.Run("Should require processor type", func(t *testing.T) {
		CWD, _ := core.CWDFromPath("/tmp")
		config := &Config{
			BaseConfig: BaseConfig{
				ID:        "test-wait",
				Type:      TaskTypeWait,
				CWD:       CWD,
				Condition: `signal.payload.status == "approved"`,
			},
			WaitTask: WaitTask{
				Processor: &Config{
					BaseConfig: BaseConfig{
						ID:  "processor1",
						CWD: CWD,
					},
				},
			},
		}
		err := config.validateWaitProcessor(t.Context())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "processor type is required")
	})

	t.Run("Should validate processor configuration recursively", func(t *testing.T) {
		CWD, _ := core.CWDFromPath("/tmp")
		config := &Config{
			BaseConfig: BaseConfig{
				ID:        "test-wait",
				Type:      TaskTypeWait,
				CWD:       CWD,
				Condition: `signal.payload.status == "approved"`,
			},
			WaitTask: WaitTask{
				Processor: &Config{
					BaseConfig: BaseConfig{
						ID:   "processor1",
						Type: TaskTypeBasic,
						CWD:  CWD,
					},
				},
			},
		}
		err := config.validateWaitProcessor(t.Context())
		assert.NoError(t, err)
	})
}

func TestConfig_Validate_WaitTaskIntegration(t *testing.T) {
	t.Run("Should integrate wait task validation with main Validate method", func(t *testing.T) {
		CWD, _ := core.CWDFromPath("/tmp")
		config := &Config{
			BaseConfig: BaseConfig{
				ID:        "test-wait",
				Type:      TaskTypeWait,
				CWD:       CWD,
				Condition: `signal.payload.status == "approved"`,
			},
			WaitTask: WaitTask{
				WaitFor: "approval_signal",
			},
		}
		err := config.Validate(t.Context())
		assert.NoError(t, err)
	})

	t.Run("Should propagate wait task validation errors", func(t *testing.T) {
		CWD, _ := core.CWDFromPath("/tmp")
		config := &Config{
			BaseConfig: BaseConfig{
				ID:        "test-wait",
				Type:      TaskTypeWait,
				CWD:       CWD,
				Condition: `signal.payload.status == "approved"`,
			},
			// Missing required fields
		}
		err := config.Validate(t.Context())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid wait task")
	})
}

func TestTypeValidator_validateWaitTask(t *testing.T) {
	t.Run("Should reject wait task with action field", func(t *testing.T) {
		CWD, _ := core.CWDFromPath("/tmp")
		config := &Config{
			BaseConfig: BaseConfig{
				ID:        "test-wait",
				Type:      TaskTypeWait,
				CWD:       CWD,
				Condition: `signal.payload.status == "approved"`,
			},
			BasicTask: BasicTask{
				Action: "some_action", // Should not be allowed
			},
			WaitTask: WaitTask{
				WaitFor: "approval_signal",
			},
		}
		validator := NewTaskTypeValidator(config)
		err := validator.validateWaitTask(t.Context())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "wait tasks cannot have an action field")
	})

	t.Run("Should reject wait task with agent", func(t *testing.T) {
		CWD, _ := core.CWDFromPath("/tmp")
		config := &Config{
			BaseConfig: BaseConfig{
				ID:        "test-wait",
				Type:      TaskTypeWait,
				CWD:       CWD,
				Condition: `signal.payload.status == "approved"`,
				Agent:     &agent.Config{}, // Should not be allowed
			},
			WaitTask: WaitTask{
				WaitFor: "approval_signal",
			},
		}
		validator := NewTaskTypeValidator(config)
		err := validator.validateWaitTask(t.Context())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "wait tasks cannot have an agent")
	})

	t.Run("Should reject wait task with tool", func(t *testing.T) {
		CWD, _ := core.CWDFromPath("/tmp")
		config := &Config{
			BaseConfig: BaseConfig{
				ID:        "test-wait",
				Type:      TaskTypeWait,
				CWD:       CWD,
				Condition: `signal.payload.status == "approved"`,
				Tool:      &tool.Config{}, // Should not be allowed
			},
			WaitTask: WaitTask{
				WaitFor: "approval_signal",
			},
		}
		validator := NewTaskTypeValidator(config)
		err := validator.validateWaitTask(t.Context())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "wait tasks cannot have a tool")
	})

	t.Run("Should accept valid wait task", func(t *testing.T) {
		CWD, _ := core.CWDFromPath("/tmp")
		config := &Config{
			BaseConfig: BaseConfig{
				ID:        "test-wait",
				Type:      TaskTypeWait,
				CWD:       CWD,
				Condition: `signal.payload.status == "approved"`,
			},
			WaitTask: WaitTask{
				WaitFor: "approval_signal",
			},
		}
		validator := NewTaskTypeValidator(config)
		err := validator.validateWaitTask(t.Context())
		assert.NoError(t, err)
	})
}
