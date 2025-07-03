package task2_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/task2"
	"github.com/compozy/compozy/engine/task2/core"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/pkg/tplengine"
	utils "github.com/compozy/compozy/test/helpers"
)

func setupTestFactory(ctx context.Context, t *testing.T) (task2.Factory, func()) {
	taskRepo, workflowRepo, cleanup := utils.SetupTestRepos(ctx, t)
	factory, err := task2.NewFactory(&task2.FactoryConfig{
		TemplateEngine: &tplengine.TemplateEngine{},
		EnvMerger:      core.NewEnvMerger(),
		WorkflowRepo:   workflowRepo,
		TaskRepo:       taskRepo,
	})
	require.NoError(t, err)
	return factory, cleanup
}

func TestTaskNormalizer_Type(t *testing.T) {
	t.Run("Should return normalizer type as string", func(t *testing.T) {
		// Arrange
		factory, cleanup := setupTestFactory(context.Background(), t)
		defer cleanup()

		normalizer, err := factory.CreateNormalizer(task.TaskTypeBasic)
		assert.NoError(t, err)

		// Act
		taskType := normalizer.Type()

		// Assert
		assert.Equal(t, task.TaskTypeBasic, taskType)
	})
}

func TestDefaultNormalizerFactory_CreateNormalizer_AllTypes(t *testing.T) {
	// Arrange
	ctx := context.Background()
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
		{"Should handle empty type as basic", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			normalizer, err := factory.CreateNormalizer(tc.taskType)

			// Assert
			assert.NoError(t, err)
			assert.NotNil(t, normalizer)
		})
	}
}

func TestDefaultNormalizerFactory_CreateNormalizer_UnsupportedType(t *testing.T) {
	t.Run("Should return error for unsupported task type", func(t *testing.T) {
		// Arrange
		factory, cleanup := setupTestFactory(context.Background(), t)
		defer cleanup()

		// Act
		normalizer, err := factory.CreateNormalizer("unsupported_type")

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
	t.Run("Should create extended factory with all dependencies", func(t *testing.T) {
		// Arrange
		factory, cleanup := setupTestFactory(context.Background(), t)
		defer cleanup()
		// Act
		// Assert
		assert.NotNil(t, factory)
	})

	t.Run("Should return error when template engine is nil", func(t *testing.T) {
		// Arrange
		config := &task2.FactoryConfig{
			EnvMerger: core.NewEnvMerger(),
		}
		// Act
		factory, err := task2.NewFactory(config)
		// Assert
		assert.Error(t, err)
		assert.Nil(t, factory)
		assert.Contains(t, err.Error(), "template engine is required")
	})

	t.Run("Should return error when env merger is nil", func(t *testing.T) {
		// Arrange
		config := &task2.FactoryConfig{
			TemplateEngine: &tplengine.TemplateEngine{},
		}
		// Act
		factory, err := task2.NewFactory(config)
		// Assert
		assert.Error(t, err)
		assert.Nil(t, factory)
		assert.Contains(t, err.Error(), "env merger is required")
	})
}

func TestExtendedFactory_CreateResponseHandler(t *testing.T) {
	// Setup
	factory, cleanup := setupTestFactory(context.Background(), t)
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			handler, err := factory.CreateResponseHandler(tc.taskType)

			// Assert
			require.NoError(t, err)
			assert.NotNil(t, handler)
			assert.Equal(t, tc.taskType, handler.Type())
		})
	}

	t.Run("Should return error for unsupported task type", func(t *testing.T) {
		// Act
		handler, err := factory.CreateResponseHandler("unsupported_type")

		// Assert
		assert.Error(t, err)
		assert.Nil(t, handler)
		assert.Contains(t, err.Error(), "unsupported task type for response handler")
	})
}

func TestExtendedFactory_CreateCollectionExpander(t *testing.T) {
	t.Run("Should create collection expander", func(t *testing.T) {
		// Arrange
		factory, cleanup := setupTestFactory(context.Background(), t)
		defer cleanup()

		// Act
		expander := factory.CreateCollectionExpander()

		// Assert
		assert.NotNil(t, expander)
		assert.Implements(t, (*shared.CollectionExpander)(nil), expander)
	})
}

func TestExtendedFactory_CreateTaskConfigRepository(t *testing.T) {
	t.Run("Should create task config repository", func(t *testing.T) {
		// Arrange
		factory, cleanup := setupTestFactory(context.Background(), t)
		defer cleanup()

		// Act
		configStore := services.NewTestConfigStore(t)
		repo := factory.CreateTaskConfigRepository(configStore)

		// Assert
		assert.NotNil(t, repo)
		assert.Implements(t, (*shared.TaskConfigRepository)(nil), repo)
	})
}

func TestExtendedFactory_BackwardCompatibility(t *testing.T) {
	t.Run("Should maintain backward compatibility with existing normalizer creation", func(t *testing.T) {
		// Arrange
		factory, cleanup := setupTestFactory(context.Background(), t)
		defer cleanup()
		// Act - existing normalizer creation should still work
		normalizer, err := factory.CreateNormalizer(task.TaskTypeBasic)
		// Assert
		require.NoError(t, err)
		assert.NotNil(t, normalizer)
		assert.Equal(t, task.TaskTypeBasic, normalizer.Type())
	})
}

func TestExtendedFactory_CreateResponseHandler_WithoutRepositories(t *testing.T) {
	t.Run("Should create response handler even without repositories", func(t *testing.T) {
		// Arrange - factory without repositories
		factory, cleanup := setupTestFactory(context.Background(), t)
		defer cleanup()
		// Act
		handler, err := factory.CreateResponseHandler(task.TaskTypeBasic)
		// Assert
		require.NoError(t, err)
		assert.NotNil(t, handler)
		// Handler should work but some features may be limited
	})
}
