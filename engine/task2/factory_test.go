package task2_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2"
	"github.com/compozy/compozy/engine/task2/core"
	"github.com/compozy/compozy/pkg/tplengine"
)

func TestTaskNormalizer_Type(t *testing.T) {
	t.Run("Should return normalizer type as string", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		envMerger := core.NewEnvMerger()
		factory, err := task2.NewNormalizerFactory(templateEngine, envMerger)
		assert.NoError(t, err)

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
	templateEngine := &tplengine.TemplateEngine{}
	envMerger := core.NewEnvMerger()
	factory, err := task2.NewNormalizerFactory(templateEngine, envMerger)
	assert.NoError(t, err)

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
		templateEngine := &tplengine.TemplateEngine{}
		envMerger := core.NewEnvMerger()
		factory, err := task2.NewNormalizerFactory(templateEngine, envMerger)
		assert.NoError(t, err)

		// Act
		normalizer, err := factory.CreateNormalizer("unsupported_type")

		// Assert
		assert.Error(t, err)
		assert.Nil(t, normalizer)
		assert.Contains(t, err.Error(), "unsupported task type")
	})
}
