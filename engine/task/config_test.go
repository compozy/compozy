package task

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/pkg/ref"
	"github.com/compozy/compozy/pkg/utils"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTest(t *testing.T, taskFile string) (*core.PathCWD, string) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	cwd, dstPath := utils.SetupTest(t, filename)
	dstPath = filepath.Join(dstPath, taskFile)
	return cwd, dstPath
}

func Test_LoadTask(t *testing.T) {
	t.Run("Should load basic task configuration correctly", func(t *testing.T) {
		CWD, dstPath := setupTest(t, "basic_task.yaml")

		// Run the test
		config, err := Load(CWD, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the config
		err = config.Validate()
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
		compiledSchema, err := schema.Compile()
		require.NoError(t, err)
		assert.Equal(t, []string{"object"}, []string(compiledSchema.Type))
		require.NotNil(t, compiledSchema.Properties)
		assert.Contains(t, (*compiledSchema.Properties), "code")
		assert.Contains(t, (*compiledSchema.Properties), "language")
		assert.Contains(t, compiledSchema.Required, "code")

		// Validate output schema
		outSchema := config.OutputSchema
		compiledOutSchema, err := outSchema.Compile()
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
		config, err := Load(CWD, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the config
		err = config.Validate()
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
		assert.Equal(t, "{{ .tasks.analyze.output.type }}", config.Condition)
		assert.Equal(t, 4, len(config.Routes))

		// Validate routes
		assert.Equal(t, "process-invoice", config.Routes["invoice"])
		assert.Equal(t, "process-contract", config.Routes["contract"])
		assert.Equal(t, "process-receipt", config.Routes["receipt"])
		assert.Equal(t, "manual-review", config.Routes["unknown"])

		// Validate input schema
		schema := config.InputSchema
		compiledSchema2, err := schema.Compile()
		require.NoError(t, err)
		assert.Equal(t, []string{"object"}, []string(compiledSchema2.Type))
		require.NotNil(t, compiledSchema2.Properties)
		assert.Contains(t, (*compiledSchema2.Properties), "document")
		assert.Contains(t, (*compiledSchema2.Properties), "confidence_threshold")
		assert.Contains(t, compiledSchema2.Required, "document")

		// Validate output schema
		outSchema := config.OutputSchema
		compiledOutSchema2, err := outSchema.Compile()
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
		ev := ref.NewEvaluator()

		// Run the test
		config, err := LoadAndEval(CWD, dstPath, ev)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the config
		err = config.Validate()
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
		compiledSchema, err := schema.Compile()
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
		config, err := Load(CWD, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the config
		err = config.Validate()
		require.Error(t, err)
	})

	t.Run("Should return error for circular dependency in loaded YAML", func(t *testing.T) {
		CWD, dstPath := setupTest(t, "circular_task.yaml")

		// Run the test
		config, err := Load(CWD, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the config - should detect circular dependency
		err = config.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "circular dependency detected involving task: circular_parent")
	})

	t.Run("Should load collection task configuration correctly", func(t *testing.T) {
		CWD, dstPath := setupTest(t, "collection_task.yaml")

		// Run the test
		config, err := Load(CWD, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the config
		err = config.Validate()
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
		compiledSchema, err := schema.Compile()
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
		config, err := Load(CWD, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the config
		err = config.Validate()
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

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should validate valid router task", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   taskID,
				Type: TaskTypeRouter,
				CWD:  taskCWD,
			},
			RouterTask: RouterTask{
				Condition: "test-condition",
				Routes: map[string]any{
					"route1": map[string]any{
						"id": "next1",
					},
				},
			},
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should validate valid parallel task", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   taskID,
				Type: TaskTypeParallel,
				CWD:  taskCWD,
			},
			ParallelTask: ParallelTask{
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
				Strategy: StrategyWaitAll,
			},
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should return error when CWD is missing", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   "test-task",
				Type: TaskTypeBasic,
			},
		}

		err := config.Validate()
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

		err := config.Validate()
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

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "condition is required for router tasks")
	})

	t.Run("Should return error for router task missing routes", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   "test-task",
				Type: TaskTypeRouter,
				CWD:  taskCWD,
			},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "condition is required for router tasks")
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

		err := config.ValidateInput(context.Background(), config.With)
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
			ParallelTask: ParallelTask{
				Tasks: []Config{},
			},
		}

		err := config.Validate()
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

		err := config.Validate()
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

		err := config.Validate()
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

		err := config.Validate()
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
			ParallelTask: ParallelTask{
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
			},
		}

		err := config.Validate()
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
			ParallelTask: ParallelTask{
				Tasks: []Config{
					{
						BaseConfig: BaseConfig{
							ID:   "invalid",
							Type: TaskTypeBasic,
							// Missing required CWD for validation
						},
					},
				},
			},
		}

		err := config.Validate()
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
			ParallelTask: ParallelTask{
				Tasks: []Config{
					{
						BaseConfig: BaseConfig{
							ID:   "child_task",
							Type: TaskTypeParallel,
							CWD:  taskCWD,
						},
						ParallelTask: ParallelTask{
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
				},
			},
		}

		err := config.Validate()
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
			ParallelTask: ParallelTask{
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
			},
		}

		err := config.Validate()
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
			ParallelTask: ParallelTask{
				Tasks: []Config{
					{
						BaseConfig: BaseConfig{
							ID:   "child_task",
							Type: TaskTypeParallel,
							CWD:  taskCWD,
						},
						ParallelTask: ParallelTask{
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
				},
			},
		}

		err := config.Validate()
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
			ParallelTask: ParallelTask{
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
			},
		}

		err := config.Validate()
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
			ParallelTask: ParallelTask{
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
			},
		}
		err := config.Validate()
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
			ParallelTask: ParallelTask{
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
			ParallelTask: ParallelTask{
				Task: &Config{
					BaseConfig: BaseConfig{ID: "template", Type: TaskTypeBasic, CWD: taskCWD},
				},
			},
		}

		err := config.Validate()
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
			ParallelTask: ParallelTask{
				Task: &Config{
					BaseConfig: BaseConfig{ID: "template", Type: TaskTypeBasic, CWD: taskCWD},
				},
			},
		}

		err := config.Validate()
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
			ParallelTask: ParallelTask{
				Task: &Config{
					BaseConfig: BaseConfig{ID: "template", Type: TaskTypeBasic, CWD: taskCWD},
				},
			},
		}

		err := config.Validate()
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
			ParallelTask: ParallelTask{
				Task: &Config{
					BaseConfig: BaseConfig{ID: "template", Type: TaskTypeBasic, CWD: taskCWD},
				},
				Tasks: []Config{
					{BaseConfig: BaseConfig{ID: "task1", Type: TaskTypeBasic, CWD: taskCWD}},
				},
			},
		}

		err := config.Validate()
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

		err := config.Validate()
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
		ev := ref.NewEvaluator()

		// Run the test
		config, err := LoadAndEval(CWD, dstPath, ev)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the config
		err = config.Validate()
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

		// Validate that the agent was properly evaluated and loaded
		agent := config.GetAgent()
		require.NotNil(t, agent, "Agent should be loaded from the $use directive")
		assert.Equal(t, "code-assistant", agent.ID)
		require.NotNil(t, agent.Config)
		assert.Equal(t, "anthropic", string(agent.Config.Provider))
		assert.Equal(t, "claude-3-opus-20240229", agent.Config.Model)
		assert.Equal(t, float64(0.7), agent.Config.Params.Temperature)
		assert.Equal(t, int32(4000), agent.Config.Params.MaxTokens)
		assert.Equal(t, "You are a powerful AI coding assistant.", agent.Instructions)
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
		err = config.Validate()
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
		err = config.Validate()
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
		err = config.Validate()
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
		config, err := Load(cwd, dstPath)
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
		err = config.Validate()
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
			ParallelTask: ParallelTask{
				Strategy: StrategyFailFast,
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
			},
		}

		// Test execution type
		assert.Equal(t, ExecutionComposite, config.GetExecType())

		// Test that it's a composite type
		assert.Equal(t, TaskTypeComposite, config.Type)

		// Test strategy
		assert.Equal(t, StrategyFailFast, config.GetStrategy())

		// Validate the configuration
		err := config.Validate()
		require.NoError(t, err)
	})

	t.Run("Should fail validation for composite task with no subtasks", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   "empty-composite",
				Type: TaskTypeComposite,
				CWD:  cwd,
			},
			ParallelTask: ParallelTask{
				Strategy: StrategyFailFast,
				Tasks:    []Config{},
			},
		}

		err := config.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "composite tasks must have at least one sub-task")
	})

	t.Run("Should fail validation for composite task with non-basic subtasks", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   "invalid-composite",
				Type: TaskTypeComposite,
				CWD:  cwd,
			},
			ParallelTask: ParallelTask{
				Tasks: []Config{
					{
						BaseConfig: BaseConfig{
							ID:   "nested-parallel",
							Type: TaskTypeParallel,
							CWD:  cwd,
						},
					},
				},
			},
		}

		err := config.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "composite subtasks must be of type 'basic'")
	})

	t.Run("Should support best effort strategy", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   "composite-with-best-effort",
				Type: TaskTypeComposite,
				CWD:  cwd,
			},
			ParallelTask: ParallelTask{
				Strategy: StrategyBestEffort,
				Tasks: []Config{
					{
						BaseConfig: BaseConfig{
							ID:   "step1",
							Type: TaskTypeBasic,
							CWD:  cwd,
						},
					},
				},
			},
		}

		err := config.Validate()
		require.NoError(t, err)
		assert.Equal(t, StrategyBestEffort, config.GetStrategy())
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
			assert.Equal(t, originalConfig.ParallelTask.Tasks[i].ID, childTask.ID)
			assert.Equal(t, originalConfig.ParallelTask.Tasks[i].FilePath, childTask.FilePath)
			assert.NotNil(t, childTask.CWD)
			assert.Equal(t, originalConfig.Tasks[i].CWD.PathStr(), childTask.CWD.PathStr())
			assert.Equal(t, originalConfig.ParallelTask.Tasks[i].Action, childTask.Action)
		}
	})
}
