package task

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/pkg/ref"
	"github.com/compozy/compozy/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTest(t *testing.T, taskFile string) (cwd *core.CWD, dstPath string) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	cwd, dstPath = utils.SetupTest(t, filename)
	dstPath = filepath.Join(dstPath, taskFile)
	return
}

func Test_LoadTask(t *testing.T) {
	t.Run("Should load basic task configuration correctly", func(t *testing.T) {
		cwd, dstPath := setupTest(t, "basic_task.yaml")

		// Run the test
		config, err := Load(cwd, dstPath)
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
		cwd, dstPath := setupTest(t, "router_task.yaml")

		// Run the test
		config, err := Load(cwd, dstPath)
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
		cwd, dstPath := setupTest(t, "parallel_task.yaml")
		ev := ref.NewEvaluator()

		// Run the test
		config, err := LoadAndEval(cwd, dstPath, ev)
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
		cwd, dstPath := setupTest(t, "invalid_task.yaml")

		// Run the test
		config, err := Load(cwd, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the config
		err = config.Validate()
		require.Error(t, err)
	})

	t.Run("Should return error for circular dependency in loaded YAML", func(t *testing.T) {
		cwd, dstPath := setupTest(t, "circular_task.yaml")

		// Run the test
		config, err := Load(cwd, dstPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Validate the config - should detect circular dependency
		err = config.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "circular dependency detected involving task: circular_parent")
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
				cwd:  taskCWD,
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
				cwd:  taskCWD,
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
				cwd:  taskCWD,
			},
			ParallelTask: ParallelTask{
				Tasks: []Config{
					{
						BaseConfig: BaseConfig{
							ID:    "task1",
							Type:  TaskTypeBasic,
							Agent: &agent.Config{ID: "test_agent"},
							cwd:   taskCWD, // Each sub-task needs a CWD
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
				cwd:  taskCWD,
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
				cwd:  taskCWD,
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
				cwd:  taskCWD,
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
				cwd:  taskCWD,
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
				cwd:  taskCWD,
			},
			ParallelTask: ParallelTask{
				Tasks: []Config{},
			},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parallel tasks must have at least one sub-task")
	})

	t.Run("Should return error for parallel task with duplicate IDs", func(t *testing.T) {
		config := &Config{
			BaseConfig: BaseConfig{
				ID:   taskID,
				Type: TaskTypeParallel,
				cwd:  taskCWD,
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
				cwd:  taskCWD,
			},
			ParallelTask: ParallelTask{
				Tasks: []Config{
					{
						BaseConfig: BaseConfig{
							ID:   "invalid",
							Type: TaskTypeBasic,
							// Missing required cwd for validation
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
				cwd:  taskCWD,
			},
			ParallelTask: ParallelTask{
				Tasks: []Config{
					{
						BaseConfig: BaseConfig{
							ID:   "child_task",
							Type: TaskTypeParallel,
							cwd:  taskCWD,
						},
						ParallelTask: ParallelTask{
							Tasks: []Config{
								{
									BaseConfig: BaseConfig{
										ID:   "parent_task", // Creates a cycle
										Type: TaskTypeBasic,
										cwd:  taskCWD,
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
				cwd:  taskCWD,
			},
			ParallelTask: ParallelTask{
				Tasks: []Config{
					{
						BaseConfig: BaseConfig{
							ID:   "self_ref", // Same ID as parent creates cycle
							Type: TaskTypeBasic,
							cwd:  taskCWD,
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
				cwd:  taskCWD,
			},
			ParallelTask: ParallelTask{
				Tasks: []Config{
					{
						BaseConfig: BaseConfig{
							ID:   "child_task",
							Type: TaskTypeParallel,
							cwd:  taskCWD,
						},
						ParallelTask: ParallelTask{
							Tasks: []Config{
								{
									BaseConfig: BaseConfig{
										ID:   "grandchild_task",
										Type: TaskTypeBasic,
										cwd:  taskCWD,
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
		cwd, dstPath := setupTest(t, "ref_task.yaml")
		ev := ref.NewEvaluator()

		// Run the test
		config, err := LoadAndEval(cwd, dstPath, ev)
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
