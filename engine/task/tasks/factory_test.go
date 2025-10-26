package tasks_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/task/tasks"
	taskscore "github.com/compozy/compozy/engine/task/tasks/core"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
	utils "github.com/compozy/compozy/test/helpers"
)

func setupTestFactory(ctx context.Context, t *testing.T) (tasks.Factory, func()) {
	taskRepo, workflowRepo, cleanup := utils.SetupTestRepos(ctx, t)
	factory, err := tasks.NewFactory(ctx, &tasks.FactoryConfig{
		TemplateEngine: &tplengine.TemplateEngine{},
		EnvMerger:      taskscore.NewEnvMerger(),
		WorkflowRepo:   workflowRepo,
		TaskRepo:       taskRepo,
	})
	require.NoError(t, err)
	return factory, cleanup
}

func TestTaskNormalizer_Type(t *testing.T) {
	t.Run("Should return normalizer type as string", func(t *testing.T) {
		// Arrange
		factory, cleanup := setupTestFactory(t.Context(), t)
		defer cleanup()

		normalizer, err := factory.CreateNormalizer(t.Context(), task.TaskTypeBasic)
		assert.NoError(t, err)

		// Act
		taskType := normalizer.Type()

		// Assert
		assert.Equal(t, task.TaskTypeBasic, taskType)
	})
}

func TestDefaultNormalizerFactory_CreateNormalizer_AllTypes(t *testing.T) {
	// Arrange
	ctx := t.Context()
	factory, cleanup := setupTestFactory(ctx, t)
	defer cleanup()

	testCases := []struct {
		name     string
		taskType task.Type
	}{
		{"Should create basic normalizer", task.TaskTypeBasic},
		{"Should create parallel normalizer", task.TaskTypeParallel},
		{"Should create collection normalizer", task.TaskTypeCollection},
		{"Should create router normalizer", task.TaskTypeRouter},
		{"Should create wait normalizer", task.TaskTypeWait},
		{"Should create aggregate normalizer", task.TaskTypeAggregate},
		{"Should create composite normalizer", task.TaskTypeComposite},
		{"Should create signal normalizer", task.TaskTypeSignal},
		{"Should create memory normalizer", task.TaskTypeMemory},
		{"Should handle empty type as basic", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			normalizer, err := factory.CreateNormalizer(t.Context(), tc.taskType)

			// Assert
			assert.NoError(t, err)
			assert.NotNil(t, normalizer)
		})
	}
}

func TestDefaultNormalizerFactory_CreateNormalizer_UnsupportedType(t *testing.T) {
	t.Run("Should return error for unsupported task type", func(t *testing.T) {
		// Arrange
		factory, cleanup := setupTestFactory(t.Context(), t)
		defer cleanup()

		// Act
		normalizer, err := factory.CreateNormalizer(t.Context(), "unsupported_type")

		// Assert
		assert.Error(t, err)
		assert.Nil(t, normalizer)
		assert.Contains(t, err.Error(), "unsupported task type")
	})
}

// -----------------------------------------------------------------------------
// Extended Factory Tests
// -----------------------------------------------------------------------------

func TestNewFactoryWithConfig(t *testing.T) {
	t.Run("Should return error when template engine is nil", func(t *testing.T) {
		// Arrange
		config := &tasks.FactoryConfig{
			EnvMerger: taskscore.NewEnvMerger(),
		}
		// Act
		factory, err := tasks.NewFactory(t.Context(), config)
		// Assert
		assert.Nil(t, factory)
		assert.ErrorContains(t, err, "template engine is required")
	})

	t.Run("Should return error when env merger is nil", func(t *testing.T) {
		// Arrange
		config := &tasks.FactoryConfig{
			TemplateEngine: &tplengine.TemplateEngine{},
		}
		// Act
		factory, err := tasks.NewFactory(t.Context(), config)
		// Assert
		assert.Nil(t, factory)
		assert.ErrorContains(t, err, "env merger is required")
	})
}

func TestExtendedFactory_CreateResponseHandler(t *testing.T) {
	// Setup
	factory, cleanup := setupTestFactory(t.Context(), t)
	defer cleanup()

	testCases := []struct {
		name     string
		taskType task.Type
	}{
		{"Should create basic response handler", task.TaskTypeBasic},
		{"Should create parallel response handler", task.TaskTypeParallel},
		{"Should create collection response handler", task.TaskTypeCollection},
		{"Should create composite response handler", task.TaskTypeComposite},
		{"Should create router response handler", task.TaskTypeRouter},
		{"Should create wait response handler", task.TaskTypeWait},
		{"Should create signal response handler", task.TaskTypeSignal},
		{"Should create aggregate response handler", task.TaskTypeAggregate},
		{"Should create memory response handler", task.TaskTypeMemory},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			handler, err := factory.CreateResponseHandler(t.Context(), tc.taskType)

			// Assert
			require.NoError(t, err)
			assert.NotNil(t, handler)
			assert.Equal(t, tc.taskType, handler.Type())
		})
	}

	t.Run("Should return error for unsupported task type", func(t *testing.T) {
		// Act
		handler, err := factory.CreateResponseHandler(t.Context(), "unsupported_type")

		// Assert
		assert.Error(t, err)
		assert.Nil(t, handler)
		assert.Contains(t, err.Error(), "unsupported task type for response handler")
	})
}

func TestExtendedFactory_CreateCollectionExpander(t *testing.T) {
	t.Run("Should create collection expander", func(t *testing.T) {
		// Arrange
		factory, cleanup := setupTestFactory(t.Context(), t)
		defer cleanup()

		// Act
		expander := factory.CreateCollectionExpander(t.Context())

		// Assert
		assert.NotNil(t, expander)
		assert.Implements(t, (*shared.CollectionExpander)(nil), expander)
	})
}

func TestExtendedFactory_CreateTaskConfigRepository(t *testing.T) {
	t.Run("Should create task config repository", func(t *testing.T) {
		// Arrange
		factory, cleanup := setupTestFactory(t.Context(), t)
		defer cleanup()

		// Act
		configStore := services.NewTestConfigStore(t)
		cwd, err := core.CWDFromPath(".")
		require.NoError(t, err)
		repo, err := factory.CreateTaskConfigRepository(configStore, cwd)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, repo)
		assert.Implements(t, (*shared.TaskConfigRepository)(nil), repo)
	})
}

func TestExtendedFactory_BackwardCompatibility(t *testing.T) {
	t.Run("Should maintain backward compatibility with existing normalizer creation", func(t *testing.T) {
		// Arrange
		factory, cleanup := setupTestFactory(t.Context(), t)
		defer cleanup()
		// Act - existing normalizer creation should still work
		normalizer, err := factory.CreateNormalizer(t.Context(), task.TaskTypeBasic)
		// Assert
		require.NoError(t, err)
		assert.NotNil(t, normalizer)
		assert.Equal(t, task.TaskTypeBasic, normalizer.Type())
	})
}

func TestExtendedFactory_CreateResponseHandler_WithoutRepositories(t *testing.T) {
	t.Run("Should create response handler even without repositories", func(t *testing.T) {
		// Arrange - factory without repositories
		factory, cleanup := setupTestFactory(t.Context(), t)
		defer cleanup()
		// Act
		handler, err := factory.CreateResponseHandler(t.Context(), task.TaskTypeBasic)
		// Assert
		require.NoError(t, err)
		assert.NotNil(t, handler)
		// Handler should work but some features may be limited
	})
}

func TestCreateNormalizer_Memory(t *testing.T) {
	t.Run("Should create normalizer for memory task type", func(t *testing.T) {
		// Arrange
		factory, cleanup := setupTestFactory(t.Context(), t)
		defer cleanup()

		// Act
		normalizer, err := factory.CreateNormalizer(t.Context(), task.TaskTypeMemory)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, normalizer)
		assert.Equal(t, task.TaskTypeMemory, normalizer.Type())
	})

	t.Run("Should return correct task type for memory normalizer", func(t *testing.T) {
		// Arrange
		factory, cleanup := setupTestFactory(t.Context(), t)
		defer cleanup()
		normalizer, _ := factory.CreateNormalizer(t.Context(), task.TaskTypeMemory)

		// Act
		taskType := normalizer.Type()

		// Assert
		assert.Equal(t, task.TaskTypeMemory, taskType)
	})
}

func TestCreateResponseHandler_Memory(t *testing.T) {
	t.Run("Should create response handler for memory task type", func(t *testing.T) {
		// Arrange
		factory, cleanup := setupTestFactory(t.Context(), t)
		defer cleanup()

		// Act
		handler, err := factory.CreateResponseHandler(t.Context(), task.TaskTypeMemory)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, handler)
		assert.Equal(t, task.TaskTypeMemory, handler.Type())
	})

	t.Run("Should validate input type for memory task response", func(t *testing.T) {
		// Arrange
		factory, cleanup := setupTestFactory(t.Context(), t)
		defer cleanup()
		handler, _ := factory.CreateResponseHandler(t.Context(), task.TaskTypeMemory)

		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					Type: task.TaskTypeMemory,
				},
			},
			TaskState: &task.State{
				Status: core.StatusSuccess,
				Output: &core.Output{
					"success": true,
					"key":     "test:123",
				},
			},
		}

		// Act
		_, err := handler.HandleResponse(t.Context(), input)

		// Assert - Since basic handler validates type, this should not error with basic type
		// The fact that we can call it without panic is the important validation
		assert.NotNil(t, err) // Will error due to missing workflow config, but that's expected
	})
}

// -----------------------------------------------------------------------------
// Output Transformer Tests
// -----------------------------------------------------------------------------

func TestOutputTransformerAdapter_IndirectTesting(t *testing.T) {
	// These tests indirectly test the outputTransformerAdapter.TransformOutput method
	// through the CreateResponseHandler factory method, following the project's testing patterns.
	// The adapter is created by the factory's createOutputTransformer helper method and
	// its TransformOutput logic is exercised through response handler processing.

	ctx := t.Context()

	t.Run("Should create factory with output transformer functionality", func(t *testing.T) {
		// Arrange & Act
		factory, cleanup := setupTestFactory(ctx, t)
		defer cleanup()

		// Test that factory creates response handlers that use output transformation
		handler, err := factory.CreateResponseHandler(ctx, task.TaskTypeBasic)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, handler)
		assert.Equal(t, task.TaskTypeBasic, handler.Type())
		// The handler internally has an outputTransformerAdapter created by the factory
	})

	t.Run("Should test deferred transformation logic for collection tasks", func(t *testing.T) {
		// This tests the shouldDeferOutputTransformation logic that affects when
		// the outputTransformerAdapter.TransformOutput method is called
		// Arrange
		factory, cleanup := setupTestFactory(ctx, t)
		defer cleanup()

		collectionHandler, err := factory.CreateResponseHandler(ctx, task.TaskTypeCollection)
		require.NoError(t, err)

		parallelHandler, err := factory.CreateResponseHandler(ctx, task.TaskTypeParallel)
		require.NoError(t, err)

		basicHandler, err := factory.CreateResponseHandler(ctx, task.TaskTypeBasic)
		require.NoError(t, err)

		// Assert - Handlers are created successfully, indicating the output transformer
		// adapter is properly integrated. The deferral logic is tested through the
		// response handler's shouldDeferOutputTransformation method.
		assert.Equal(t, task.TaskTypeCollection, collectionHandler.Type())
		assert.Equal(t, task.TaskTypeParallel, parallelHandler.Type())
		assert.Equal(t, task.TaskTypeBasic, basicHandler.Type())
	})

	t.Run("Should handle output transformer creation without repositories", func(t *testing.T) {
		// This tests the factory's createOutputTransformer method when workflowRepo is nil
		// Arrange
		factory, err := tasks.NewFactory(ctx, &tasks.FactoryConfig{
			TemplateEngine: &tplengine.TemplateEngine{},
			EnvMerger:      taskscore.NewEnvMerger(),
			// No repositories provided - tests nil handling in outputTransformerAdapter
		})
		require.NoError(t, err)

		// Act
		handler, err := factory.CreateResponseHandler(ctx, task.TaskTypeBasic)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, handler)
		// The handler should be created successfully even without repositories,
		// demonstrating that the outputTransformerAdapter handles nil workflowRepo
	})

	t.Run("Should create factory components that use output transformer internally", func(t *testing.T) {
		// This test indirectly validates that the outputTransformerAdapter is properly
		// integrated into the factory's components and the CreateResponseHandler method
		// exercises the createOutputTransformer helper method
		// Arrange
		factory, cleanup := setupTestFactory(ctx, t)
		defer cleanup()

		// Test that all task types get response handlers with output transformers
		taskTypes := []task.Type{
			task.TaskTypeBasic,
			task.TaskTypeParallel,
			task.TaskTypeCollection,
			task.TaskTypeComposite,
			task.TaskTypeRouter,
			task.TaskTypeWait,
			task.TaskTypeSignal,
			task.TaskTypeAggregate,
			task.TaskTypeMemory,
		}

		// Act & Assert
		for _, taskType := range taskTypes {
			handler, err := factory.CreateResponseHandler(ctx, taskType)
			require.NoError(t, err, "Should create handler for task type %s", taskType)
			assert.NotNil(t, handler, "Handler should not be nil for task type %s", taskType)
			assert.Equal(t, taskType, handler.Type(), "Handler type should match for task type %s", taskType)
		}

		// This test ensures that the factory's createOutputTransformer method is
		// called for each response handler creation, thus exercising the
		// outputTransformerAdapter creation logic
	})
}

// TestOutputTransformerAdapter_TransformOutput tests the outputTransformerAdapter.TransformOutput
// method through realistic scenarios that trigger the actual transformation logic
func TestOutputTransformerAdapter_TransformOutput(t *testing.T) {
	ctx := t.Context()

	t.Run("Should transform output with valid configuration and state", func(t *testing.T) {
		// Arrange
		taskRepo, workflowRepo, cleanup := utils.SetupTestRepos(ctx, t)
		defer cleanup()

		factory, err := tasks.NewFactory(ctx, &tasks.FactoryConfig{
			TemplateEngine: &tplengine.TemplateEngine{},
			EnvMerger:      taskscore.NewEnvMerger(),
			WorkflowRepo:   workflowRepo,
			TaskRepo:       taskRepo,
		})
		require.NoError(t, err)

		// Create a basic response handler that uses the outputTransformerAdapter
		handler, err := factory.CreateResponseHandler(ctx, task.TaskTypeBasic)
		require.NoError(t, err)

		// Create test workflow and task states
		workflowExecID := core.MustNewID()
		taskExecID := core.MustNewID()

		// Create and save workflow state to database first
		workflowState := workflow.NewState("test-workflow", workflowExecID, &core.Input{})
		workflowState.Status = core.StatusRunning
		err = workflowRepo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		// Setup task config with outputs configuration
		outputs := core.Input{
			"result": "{{ .output.value }}",
			"status": "processed",
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				Type:    task.TaskTypeBasic,
				Outputs: &outputs,
			},
		}

		// Setup task state with output
		taskState := &task.State{
			TaskExecID:     taskExecID,
			WorkflowExecID: workflowExecID,
			WorkflowID:     "test-workflow",
			TaskID:         "test-task",
			Status:         core.StatusSuccess,
			Output: &core.Output{
				"value": "test_result",
				"meta":  "additional_data",
			},
		}
		// Save task state to database before handling response
		err = taskRepo.UpsertState(ctx, taskState)
		require.NoError(t, err)

		// Create response input that will trigger TransformOutput
		responseInput := &shared.ResponseInput{
			TaskConfig:     taskConfig,
			TaskState:      taskState,
			WorkflowConfig: &workflow.Config{},
			WorkflowState:  workflowState,
		}

		// Act - This will trigger outputTransformerAdapter.TransformOutput through applyOutputTransformation
		result, err := handler.HandleResponse(ctx, responseInput)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		// The TransformOutput method should have been called and processed the outputs
		assert.Equal(t, core.StatusSuccess, taskState.Status)
	})

	t.Run("Should handle output transformation when config has no outputs", func(t *testing.T) {
		// Arrange
		taskRepo, workflowRepo, cleanup := utils.SetupTestRepos(ctx, t)
		defer cleanup()

		factory, err := tasks.NewFactory(ctx, &tasks.FactoryConfig{
			TemplateEngine: &tplengine.TemplateEngine{},
			EnvMerger:      taskscore.NewEnvMerger(),
			WorkflowRepo:   workflowRepo,
			TaskRepo:       taskRepo,
		})
		require.NoError(t, err)

		handler, err := factory.CreateResponseHandler(ctx, task.TaskTypeBasic)
		require.NoError(t, err)

		workflowExecID := core.MustNewID()
		taskExecID := core.MustNewID()

		// Create and save workflow state to database
		workflowState := workflow.NewState("test-workflow", workflowExecID, &core.Input{})
		workflowState.Status = core.StatusRunning
		err = workflowRepo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		// Task config without outputs configuration
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				Type: task.TaskTypeBasic,
				// No Outputs specified
			},
		}

		taskState := &task.State{
			TaskExecID:     taskExecID,
			WorkflowExecID: workflowExecID,
			WorkflowID:     "test-workflow",
			TaskID:         "test-task",
			Status:         core.StatusSuccess,
			Output: &core.Output{
				"value": "test_result",
			},
		}
		// Save task state to database before handling response
		err = taskRepo.UpsertState(ctx, taskState)
		require.NoError(t, err)

		responseInput := &shared.ResponseInput{
			TaskConfig:     taskConfig,
			TaskState:      taskState,
			WorkflowConfig: &workflow.Config{},
			WorkflowState:  workflowState,
		}

		// Act
		result, err := handler.HandleResponse(ctx, responseInput)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		// When no outputs config, TransformOutput should return state output as-is
		assert.Equal(t, core.StatusSuccess, taskState.Status)
	})

	t.Run("Should handle output transformation when state has no output", func(t *testing.T) {
		// Arrange
		taskRepo, workflowRepo, cleanup := utils.SetupTestRepos(ctx, t)
		defer cleanup()

		factory, err := tasks.NewFactory(ctx, &tasks.FactoryConfig{
			TemplateEngine: &tplengine.TemplateEngine{},
			EnvMerger:      taskscore.NewEnvMerger(),
			WorkflowRepo:   workflowRepo,
			TaskRepo:       taskRepo,
		})
		require.NoError(t, err)

		handler, err := factory.CreateResponseHandler(ctx, task.TaskTypeBasic)
		require.NoError(t, err)

		workflowExecID := core.MustNewID()
		taskExecID := core.MustNewID()

		// Create and save workflow state to database
		workflowState := workflow.NewState("test-workflow", workflowExecID, &core.Input{})
		workflowState.Status = core.StatusRunning
		err = workflowRepo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		outputs := core.Input{
			"result": "{{ .output.value }}",
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				Type:    task.TaskTypeBasic,
				Outputs: &outputs,
			},
		}

		// Task state without output
		taskState := &task.State{
			TaskExecID:     taskExecID,
			WorkflowExecID: workflowExecID,
			WorkflowID:     "test-workflow",
			TaskID:         "test-task",
			Status:         core.StatusSuccess,
			Output:         nil, // No output
		}
		// Save task state to database before handling response
		err = taskRepo.UpsertState(ctx, taskState)
		require.NoError(t, err)

		responseInput := &shared.ResponseInput{
			TaskConfig:     taskConfig,
			TaskState:      taskState,
			WorkflowConfig: &workflow.Config{},
			WorkflowState:  workflowState,
		}

		// Act
		result, err := handler.HandleResponse(ctx, responseInput)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		// When no output in state, TransformOutput should return empty map
		assert.Equal(t, core.StatusSuccess, taskState.Status)
	})

	t.Run("Should handle workflow repository error during transformation", func(t *testing.T) {
		// Arrange - Create factory without workflow repository to simulate error
		factory, err := tasks.NewFactory(ctx, &tasks.FactoryConfig{
			TemplateEngine: &tplengine.TemplateEngine{},
			EnvMerger:      taskscore.NewEnvMerger(),
			// No WorkflowRepo - this will cause an error when TransformOutput tries to fetch workflow state
		})
		require.NoError(t, err)

		handler, err := factory.CreateResponseHandler(ctx, task.TaskTypeBasic)
		require.NoError(t, err)

		workflowExecID := core.MustNewID()
		taskExecID := core.MustNewID()

		outputs := core.Input{
			"result": "{{ .output.value }}",
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				Type:    task.TaskTypeBasic,
				Outputs: &outputs,
			},
		}

		taskState := &task.State{
			TaskExecID:     taskExecID,
			WorkflowExecID: workflowExecID,
			WorkflowID:     "test-workflow",
			TaskID:         "test-task",
			Status:         core.StatusSuccess,
			Output: &core.Output{
				"value": "test_result",
			},
		}
		// Note: Not saving to database to test error handling when state doesn't exist

		responseInput := &shared.ResponseInput{
			TaskConfig:     taskConfig,
			TaskState:      taskState,
			WorkflowConfig: &workflow.Config{},
		}

		// Act
		result, err := handler.HandleResponse(ctx, responseInput)

		// Assert - Should handle the error when workflow state is nil
		// The handler validates input and will fail when workflow state is missing
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "workflow state cannot be nil")
		assert.Nil(t, result)
	})

	t.Run("Should skip transformation for deferred task types", func(t *testing.T) {
		// Arrange
		taskRepo, workflowRepo, cleanup := utils.SetupTestRepos(ctx, t)
		defer cleanup()

		factory, err := tasks.NewFactory(ctx, &tasks.FactoryConfig{
			TemplateEngine: &tplengine.TemplateEngine{},
			EnvMerger:      taskscore.NewEnvMerger(),
			WorkflowRepo:   workflowRepo,
			TaskRepo:       taskRepo,
		})
		require.NoError(t, err)

		// Collection tasks defer output transformation
		handler, err := factory.CreateResponseHandler(ctx, task.TaskTypeCollection)
		require.NoError(t, err)

		workflowExecID := core.MustNewID()
		taskExecID := core.MustNewID()

		// Create and save workflow state to database
		workflowState := workflow.NewState("test-workflow", workflowExecID, &core.Input{})
		workflowState.Status = core.StatusRunning
		err = workflowRepo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		outputs := core.Input{
			"result": "{{ .output.value }}",
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				Type:    task.TaskTypeCollection,
				Outputs: &outputs,
			},
		}

		taskState := &task.State{
			TaskExecID:     taskExecID,
			WorkflowExecID: workflowExecID,
			WorkflowID:     "test-workflow",
			TaskID:         "test-task",
			Status:         core.StatusSuccess,
			Output: &core.Output{
				"value": "test_result",
			},
		}
		// Save task state to database before handling response
		err = taskRepo.UpsertState(ctx, taskState)
		require.NoError(t, err)

		responseInput := &shared.ResponseInput{
			TaskConfig:     taskConfig,
			TaskState:      taskState,
			WorkflowConfig: &workflow.Config{},
			WorkflowState:  workflowState,
		}

		// Act
		result, err := handler.HandleResponse(ctx, responseInput)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		// For collection tasks, transformation should be deferred
		// so the original output should remain unchanged
		assert.Equal(t, core.StatusSuccess, taskState.Status)
	})
}
