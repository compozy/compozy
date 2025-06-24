package task2_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2"
	"github.com/compozy/compozy/engine/task2/core"
	"github.com/compozy/compozy/pkg/tplengine"
)

func TestNormalizerFactory_NewNormalizerFactory(t *testing.T) {
	t.Run("Should create factory with template engine and env merger", func(t *testing.T) {
		// Arrange
		tplEngine := tplengine.NewEngine(tplengine.FormatText)
		templateEngine := task2.NewTemplateEngineAdapter(tplEngine)
		envMerger := core.NewEnvMerger()

		// Act
		factory, err := task2.NewNormalizerFactory(templateEngine, envMerger)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, factory)
	})
}

func TestNormalizerFactory_CreateNormalizer(t *testing.T) {
	// Setup
	tplEngine := tplengine.NewEngine(tplengine.FormatText)
	templateEngine := task2.NewTemplateEngineAdapter(tplEngine)
	envMerger := core.NewEnvMerger()
	factory, err := task2.NewNormalizerFactory(templateEngine, envMerger)
	require.NoError(t, err)

	t.Run("Should create basic normalizer", func(t *testing.T) {
		// Act
		normalizer, err := factory.CreateNormalizer(task.TaskTypeBasic)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, normalizer)
		assert.Equal(t, string(task.TaskTypeBasic), string(normalizer.Type()))
	})

	t.Run("Should create basic normalizer for empty type", func(t *testing.T) {
		// Act
		normalizer, err := factory.CreateNormalizer("")

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, normalizer)
		assert.Equal(t, string(task.TaskTypeBasic), string(normalizer.Type()))
	})

	t.Run("Should create parallel normalizer", func(t *testing.T) {
		// Act
		normalizer, err := factory.CreateNormalizer(task.TaskTypeParallel)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, normalizer)
		assert.Equal(t, string(task.TaskTypeParallel), string(normalizer.Type()))
	})

	t.Run("Should create collection normalizer", func(t *testing.T) {
		// Act
		normalizer, err := factory.CreateNormalizer(task.TaskTypeCollection)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, normalizer)
		assert.Equal(t, string(task.TaskTypeCollection), string(normalizer.Type()))
	})

	t.Run("Should create composite normalizer", func(t *testing.T) {
		// Act
		normalizer, err := factory.CreateNormalizer(task.TaskTypeComposite)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, normalizer)
		assert.Equal(t, string(task.TaskTypeComposite), string(normalizer.Type()))
	})

	t.Run("Should create wait normalizer", func(t *testing.T) {
		// Act
		normalizer, err := factory.CreateNormalizer(task.TaskTypeWait)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, normalizer)
		assert.Equal(t, string(task.TaskTypeWait), string(normalizer.Type()))
	})

	t.Run("Should create router normalizer", func(t *testing.T) {
		// Act
		normalizer, err := factory.CreateNormalizer(task.TaskTypeRouter)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, normalizer)
		assert.Equal(t, string(task.TaskTypeRouter), string(normalizer.Type()))
	})

	t.Run("Should create aggregate normalizer", func(t *testing.T) {
		// Act
		normalizer, err := factory.CreateNormalizer(task.TaskTypeAggregate)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, normalizer)
		assert.Equal(t, string(task.TaskTypeAggregate), string(normalizer.Type()))
	})

	t.Run("Should create signal normalizer", func(t *testing.T) {
		// Act
		normalizer, err := factory.CreateNormalizer(task.TaskTypeSignal)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, normalizer)
		assert.Equal(t, string(task.TaskTypeSignal), string(normalizer.Type()))
	})

	t.Run("Should return error for unknown task type", func(t *testing.T) {
		// Act
		normalizer, err := factory.CreateNormalizer("unknown")

		// Assert
		require.Error(t, err)
		assert.Nil(t, normalizer)
		assert.Contains(t, err.Error(), "unsupported task type")
	})
}

func TestNormalizerFactory_CreateCoreNormalizers(t *testing.T) {
	// Setup
	tplEngine := tplengine.NewEngine(tplengine.FormatText)
	templateEngine := task2.NewTemplateEngineAdapter(tplEngine)
	envMerger := core.NewEnvMerger()
	factory, err := task2.NewNormalizerFactory(templateEngine, envMerger)
	require.NoError(t, err)

	t.Run("Should create agent normalizer", func(t *testing.T) {
		// Act
		normalizer := factory.CreateAgentNormalizer()

		// Assert
		assert.NotNil(t, normalizer)
	})

	t.Run("Should create tool normalizer", func(t *testing.T) {
		// Act
		normalizer := factory.CreateToolNormalizer()

		// Assert
		assert.NotNil(t, normalizer)
	})

	t.Run("Should create success transition normalizer", func(t *testing.T) {
		// Act
		normalizer := factory.CreateSuccessTransitionNormalizer()

		// Assert
		assert.NotNil(t, normalizer)
	})

	t.Run("Should create error transition normalizer", func(t *testing.T) {
		// Act
		normalizer := factory.CreateErrorTransitionNormalizer()

		// Assert
		assert.NotNil(t, normalizer)
	})

	t.Run("Should create output transformer", func(t *testing.T) {
		// Act
		transformer := factory.CreateOutputTransformer()

		// Assert
		assert.NotNil(t, transformer)
	})
}
