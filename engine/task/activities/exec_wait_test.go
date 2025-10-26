package activities

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/task/tasks"
	taskscore "github.com/compozy/compozy/engine/task/tasks/core"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
	utils "github.com/compozy/compozy/test/helpers"
)

func TestExecuteWait_Run(t *testing.T) {
	t.Run("Should create wait task state successfully", func(t *testing.T) {
		// Arrange
		ctx := t.Context()

		// Setup real repositories with testcontainers
		taskRepo, workflowRepo, cleanup := utils.SetupTestRepos(ctx, t)
		defer cleanup()

		configStore := services.NewTestConfigStore(t)
		defer configStore.Close()

		workflowID := "test-workflow"
		workflowExecID := core.MustNewID()
		cwd, _ := core.CWDFromPath("/test")

		// Create workflow config
		workflowConfig := &workflow.Config{
			ID: workflowID,
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:        "wait-task",
						Type:      task.TaskTypeWait,
						Condition: `signal.payload.approved == true`,
					},
					WaitTask: task.WaitTask{
						WaitFor: "approval_signal",
					},
				},
			},
		}

		// Create and save workflow state
		workflowState := &workflow.State{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			Status:         core.StatusPending,
		}
		err := workflowRepo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		// Create task config
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:        "wait-task",
				Type:      task.TaskTypeWait,
				Condition: `signal.payload.approved == true`,
			},
			WaitTask: task.WaitTask{
				WaitFor: "approval_signal",
			},
		}

		// Create tasks factory
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		tasksFactory, err := tasks.NewFactory(t.Context(), &tasks.FactoryConfig{
			TemplateEngine: templateEngine,
			EnvMerger:      taskscore.NewEnvMerger(),
			WorkflowRepo:   workflowRepo,
			TaskRepo:       taskRepo,
		})
		require.NoError(t, err)

		// Create activity
		activity, err := NewExecuteWait(
			[]*workflow.Config{workflowConfig},
			workflowRepo,
			taskRepo,
			configStore,
			cwd,
			tasksFactory,
			templateEngine,
		)
		require.NoError(t, err)

		input := &ExecuteWaitInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			TaskConfig:     taskConfig,
		}

		// Act
		response, err := activity.Run(ctx, input)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.NotNil(t, response.State)
		assert.Equal(t, "wait-task", response.State.TaskID)
		assert.Equal(t, task.ExecutionWait, response.State.ExecutionType)
		assert.NotNil(t, response.State.Output)

		// Verify output metadata
		output := *response.State.Output
		assert.Equal(t, "waiting", output["wait_status"])
		assert.Equal(t, "approval_signal", output["signal_name"])
		assert.Equal(t, false, output["has_processor"])

		// Verify state was saved to database
		savedState, err := taskRepo.GetState(ctx, response.State.TaskExecID)
		require.NoError(t, err)
		assert.Equal(t, "wait-task", savedState.TaskID)
		assert.Equal(t, workflowExecID, savedState.WorkflowExecID)
		assert.Equal(t, task.ExecutionWait, savedState.ExecutionType)
	})

	t.Run("Should resolve templated timeout from prior task output", func(t *testing.T) {
		ctx := t.Context()

		taskRepo, workflowRepo, cleanup := utils.SetupTestRepos(ctx, t)
		defer cleanup()

		configStore := services.NewTestConfigStore(t)
		defer configStore.Close()

		workflowID := "wait-dynamic-timeout"
		workflowExecID := core.MustNewID()
		cwd, _ := core.CWDFromPath("/tmp")

		now := time.Now().UTC()
		calcExecID := core.MustNewID()
		calculateDelayState := &task.State{
			Component:      core.ComponentTask,
			Status:         core.StatusSuccess,
			TaskID:         "calculate-delay",
			TaskExecID:     calcExecID,
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			ExecutionType:  task.ExecutionBasic,
			Output: &core.Output{
				"duration": "2h5m",
			},
			CreatedAt: now,
			UpdatedAt: now,
		}
		workflowState := &workflow.State{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			Status:         core.StatusRunning,
			Tasks: map[string]*task.State{
				"calculate-delay": calculateDelayState,
			},
		}
		require.NoError(t, workflowRepo.UpsertState(ctx, workflowState))
		require.NoError(t, taskRepo.UpsertState(ctx, calculateDelayState))

		workflowConfig := &workflow.Config{
			ID: workflowID,
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "calculate-delay",
						Type: task.TaskTypeBasic,
					},
				},
				{
					BaseConfig: task.BaseConfig{
						ID:        "wait-reminder",
						Type:      task.TaskTypeWait,
						CWD:       cwd,
						Condition: "true",
						Timeout:   "{{ .tasks.calculate-delay.output.duration }}",
					},
					WaitTask: task.WaitTask{
						WaitFor: "reminder-signal",
					},
				},
			},
		}

		waitConfigCopy := workflowConfig.Tasks[1]
		require.NoError(t, (&waitConfigCopy).Validate(ctx))

		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		tasksFactory, err := tasks.NewFactory(t.Context(), &tasks.FactoryConfig{
			TemplateEngine: templateEngine,
			EnvMerger:      taskscore.NewEnvMerger(),
			WorkflowRepo:   workflowRepo,
			TaskRepo:       taskRepo,
		})
		require.NoError(t, err)

		activity, err := NewExecuteWait(
			[]*workflow.Config{workflowConfig},
			workflowRepo,
			taskRepo,
			configStore,
			cwd,
			tasksFactory,
			templateEngine,
		)
		require.NoError(t, err)

		input := &ExecuteWaitInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			TaskConfig:     &workflowConfig.Tasks[1],
		}

		response, err := activity.Run(ctx, input)
		require.NoError(t, err)
		require.NotNil(t, response)

		assert.Equal(t, "2h5m", workflowConfig.Tasks[1].Timeout)
		require.NotNil(t, response.State)
		assert.NotNil(t, response.State.Output)
		waitOutput := *response.State.Output
		assert.Equal(t, "reminder-signal", waitOutput["signal_name"])
	})
	t.Run("Should handle nil task config", func(t *testing.T) {
		// Arrange
		ctx := t.Context()

		// Setup real repositories with testcontainers
		taskRepo, workflowRepo, cleanup := utils.SetupTestRepos(ctx, t)
		defer cleanup()

		configStore := services.NewTestConfigStore(t)
		defer configStore.Close()

		workflowID := "test-workflow"
		workflowExecID := core.MustNewID()
		cwd, _ := core.CWDFromPath("/tmp")

		// Create tasks factory
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		tasksFactory, err := tasks.NewFactory(t.Context(), &tasks.FactoryConfig{
			TemplateEngine: templateEngine,
			EnvMerger:      taskscore.NewEnvMerger(),
			WorkflowRepo:   workflowRepo,
			TaskRepo:       taskRepo,
		})
		require.NoError(t, err)

		// Create activity
		activity, err := NewExecuteWait(
			[]*workflow.Config{},
			workflowRepo,
			taskRepo,
			configStore,
			cwd,
			tasksFactory,
			templateEngine,
		)
		require.NoError(t, err)

		input := &ExecuteWaitInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			TaskConfig:     nil, // nil config
		}

		// Act
		response, err := activity.Run(ctx, input)

		// Assert
		require.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "task_config is required")
	})
	t.Run("Should handle workflow not found", func(t *testing.T) {
		// Arrange
		ctx := t.Context()

		// Setup real repositories with testcontainers
		taskRepo, workflowRepo, cleanup := utils.SetupTestRepos(ctx, t)
		defer cleanup()

		configStore := services.NewTestConfigStore(t)
		defer configStore.Close()

		workflowID := "test-workflow"
		workflowExecID := core.MustNewID() // Non-existent workflow exec ID
		cwd, _ := core.CWDFromPath("/tmp")

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "wait-task",
				Type: task.TaskTypeWait,
			},
			WaitTask: task.WaitTask{
				WaitFor: "approval_signal",
			},
		}

		// Create tasks factory
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		tasksFactory, err := tasks.NewFactory(t.Context(), &tasks.FactoryConfig{
			TemplateEngine: templateEngine,
			EnvMerger:      taskscore.NewEnvMerger(),
			WorkflowRepo:   workflowRepo,
			TaskRepo:       taskRepo,
		})
		require.NoError(t, err)

		// Create activity
		activity, err := NewExecuteWait(
			[]*workflow.Config{},
			workflowRepo,
			taskRepo,
			configStore,
			cwd,
			tasksFactory,
			templateEngine,
		)
		require.NoError(t, err)

		input := &ExecuteWaitInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			TaskConfig:     taskConfig,
		}

		// Act
		response, err := activity.Run(ctx, input)

		// Assert
		require.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "workflow state not found")
	})
	t.Run("Should handle wrong task type", func(t *testing.T) {
		// Arrange
		ctx := t.Context()

		// Setup real repositories with testcontainers
		taskRepo, workflowRepo, cleanup := utils.SetupTestRepos(ctx, t)
		defer cleanup()

		configStore := services.NewTestConfigStore(t)
		defer configStore.Close()

		workflowID := "test-workflow"
		workflowExecID := core.MustNewID()
		cwd, _ := core.CWDFromPath("/tmp")

		// Create workflow config
		workflowConfig := &workflow.Config{
			ID: workflowID,
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "wait-task",
						Type: task.TaskTypeWait,
					},
				},
			},
		}

		// Create and save workflow state
		workflowState := &workflow.State{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			Status:         core.StatusPending,
		}
		err := workflowRepo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		// Create task config with wrong type
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "wait-task",
				Type: task.TaskTypeBasic, // Wrong type
			},
		}

		// Create tasks factory
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		tasksFactory, err := tasks.NewFactory(t.Context(), &tasks.FactoryConfig{
			TemplateEngine: templateEngine,
			EnvMerger:      taskscore.NewEnvMerger(),
			WorkflowRepo:   workflowRepo,
			TaskRepo:       taskRepo,
		})
		require.NoError(t, err)

		// Create activity
		activity, err := NewExecuteWait(
			[]*workflow.Config{workflowConfig},
			workflowRepo,
			taskRepo,
			configStore,
			cwd,
			tasksFactory,
			templateEngine,
		)
		require.NoError(t, err)

		input := &ExecuteWaitInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			TaskConfig:     taskConfig,
		}

		// Act
		response, err := activity.Run(ctx, input)

		// Assert
		require.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "wait normalizer cannot handle task type")
	})
	t.Run("Should include processor metadata when processor is configured", func(t *testing.T) {
		// Arrange
		ctx := t.Context()

		// Setup real repositories with testcontainers
		taskRepo, workflowRepo, cleanup := utils.SetupTestRepos(ctx, t)
		defer cleanup()

		configStore := services.NewTestConfigStore(t)
		defer configStore.Close()

		workflowID := "test-workflow"
		workflowExecID := core.MustNewID()
		cwd, _ := core.CWDFromPath("/test")

		// Create workflow config
		workflowConfig := &workflow.Config{
			ID: workflowID,
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:        "wait-task",
						Type:      task.TaskTypeWait,
						Condition: `processor.output.valid == true`,
					},
					WaitTask: task.WaitTask{
						WaitFor: "data_signal",
						Processor: &task.Config{
							BaseConfig: task.BaseConfig{
								ID:   "validator",
								Type: task.TaskTypeBasic,
							},
						},
					},
				},
			},
		}

		// Create and save workflow state
		workflowState := &workflow.State{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			Status:         core.StatusPending,
		}
		err := workflowRepo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		// Create task config with processor
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:        "wait-task",
				Type:      task.TaskTypeWait,
				Condition: `processor.output.valid == true`,
			},
			WaitTask: task.WaitTask{
				WaitFor: "data_signal",
				Processor: &task.Config{
					BaseConfig: task.BaseConfig{
						ID:   "validator",
						Type: task.TaskTypeBasic,
					},
				},
			},
		}

		// Create tasks factory
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		tasksFactory, err := tasks.NewFactory(t.Context(), &tasks.FactoryConfig{
			TemplateEngine: templateEngine,
			EnvMerger:      taskscore.NewEnvMerger(),
			WorkflowRepo:   workflowRepo,
			TaskRepo:       taskRepo,
		})
		require.NoError(t, err)

		// Create activity
		activity, err := NewExecuteWait(
			[]*workflow.Config{workflowConfig},
			workflowRepo,
			taskRepo,
			configStore,
			cwd,
			tasksFactory,
			templateEngine,
		)
		require.NoError(t, err)

		input := &ExecuteWaitInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			TaskConfig:     taskConfig,
		}

		// Act
		response, err := activity.Run(ctx, input)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.NotNil(t, response.State)
		assert.NotNil(t, response.State.Output)

		// Verify has_processor is true
		output := *response.State.Output
		assert.Equal(t, true, output["has_processor"])

		// Verify state was saved to database with has_processor = true
		savedState, err := taskRepo.GetState(ctx, response.State.TaskExecID)
		require.NoError(t, err)
		assert.NotNil(t, savedState.Output)
		assert.Equal(t, true, (*savedState.Output)["has_processor"])
	})
}
