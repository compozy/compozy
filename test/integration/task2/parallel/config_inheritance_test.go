package parallel

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/contracts"
	"github.com/compozy/compozy/engine/task2/parallel"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

// MockNormalizerFactory for testing
type MockNormalizerFactory struct {
	mock.Mock
}

func (m *MockNormalizerFactory) CreateNormalizer(
	_ context.Context,
	taskType task.Type,
) (contracts.TaskNormalizer, error) {
	args := m.Called(taskType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(contracts.TaskNormalizer), args.Error(1)
}

// MockTaskNormalizer for testing
type MockTaskNormalizer struct {
	mock.Mock
}

func (m *MockTaskNormalizer) Type() task.Type {
	args := m.Called()
	return task.Type(args.String(0))
}

func (m *MockTaskNormalizer) Normalize(
	_ context.Context,
	config *task.Config,
	ctx contracts.NormalizationContext,
) error {
	args := m.Called(config, ctx)
	return args.Error(0)
}

func TestParallelNormalizer_ConfigInheritance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests")
	}

	t.Run("Should inherit CWD and FilePath to parallel child tasks", func(t *testing.T) {
		t.Parallel()

		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)

		mockFactory := &MockNormalizerFactory{}
		mockSubNormalizer := &MockTaskNormalizer{}
		mockFactory.On("CreateNormalizer", task.TaskTypeBasic).Return(mockSubNormalizer, nil)
		mockSubNormalizer.On("Normalize", mock.Anything, mock.Anything).Return(nil).Times(2)

		// Create parallel normalizer
		normalizer := parallel.NewNormalizer(t.Context(), templateEngine, contextBuilder, mockFactory)

		// Setup parent task with CWD and FilePath
		parentCWD := &core.PathCWD{Path: "/parallel/working/directory"}
		parentFilePath := "configs/parallel.yaml"

		config := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "test-parallel",
				Type:     task.TaskTypeParallel,
				CWD:      parentCWD,
				FilePath: parentFilePath,
			},
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "child-task-1",
						Type: task.TaskTypeBasic,
					},
				},
				{
					BaseConfig: task.BaseConfig{
						ID:   "child-task-2",
						Type: task.TaskTypeBasic,
					},
				},
			},
		}

		ctx := &shared.NormalizationContext{
			Variables: make(map[string]any),
		}

		// Act
		err = normalizer.Normalize(t.Context(), config, ctx)

		// Assert
		require.NoError(t, err)

		// Verify inheritance for both child tasks
		for i, childTask := range config.Tasks {
			assert.Equal(t, parentCWD, childTask.CWD, "child task %d should inherit parent CWD", i+1)
			assert.Equal(t, parentFilePath, childTask.FilePath, "child task %d should inherit parent FilePath", i+1)
		}

		mockFactory.AssertExpectations(t)
		mockSubNormalizer.AssertExpectations(t)
	})

	t.Run("Should not override existing CWD and FilePath in child tasks", func(t *testing.T) {
		t.Parallel()

		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)

		mockFactory := &MockNormalizerFactory{}
		mockSubNormalizer := &MockTaskNormalizer{}
		mockFactory.On("CreateNormalizer", task.TaskTypeBasic).Return(mockSubNormalizer, nil)
		mockSubNormalizer.On("Normalize", mock.Anything, mock.Anything).Return(nil)

		normalizer := parallel.NewNormalizer(t.Context(), templateEngine, contextBuilder, mockFactory)

		// Setup with both parent and child having different CWD/FilePath
		parentCWD := &core.PathCWD{Path: "/parallel/working/directory"}
		parentFilePath := "configs/parallel.yaml"
		childCWD := &core.PathCWD{Path: "/child/specific/directory"}
		childFilePath := "configs/child.yaml"

		config := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "test-parallel",
				Type:     task.TaskTypeParallel,
				CWD:      parentCWD,
				FilePath: parentFilePath,
			},
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:       "child-task-1",
						Type:     task.TaskTypeBasic,
						CWD:      childCWD,
						FilePath: childFilePath,
					},
				},
			},
		}

		ctx := &shared.NormalizationContext{
			Variables: make(map[string]any),
		}

		// Act
		err = normalizer.Normalize(t.Context(), config, ctx)

		// Assert
		require.NoError(t, err)

		// Verify child keeps its own values
		assert.Equal(t, childCWD, config.Tasks[0].CWD, "child task should keep its own CWD")
		assert.Equal(t, childFilePath, config.Tasks[0].FilePath, "child task should keep its own FilePath")
		assert.NotEqual(t, parentCWD, config.Tasks[0].CWD, "child CWD should not be overwritten by parent")
		assert.NotEqual(
			t,
			parentFilePath,
			config.Tasks[0].FilePath,
			"child FilePath should not be overwritten by parent",
		)

		mockFactory.AssertExpectations(t)
		mockSubNormalizer.AssertExpectations(t)
	})

	t.Run("Should inherit config with nested parallel tasks", func(t *testing.T) {
		t.Parallel()

		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)

		mockFactory := &MockNormalizerFactory{}
		mockSubNormalizer := &MockTaskNormalizer{}

		// Parent normalizer will call child parallel normalizer
		mockFactory.On("CreateNormalizer", task.TaskTypeParallel).Return(mockSubNormalizer, nil)
		mockSubNormalizer.On("Normalize", mock.Anything, mock.Anything).Return(nil)

		normalizer := parallel.NewNormalizer(t.Context(), templateEngine, contextBuilder, mockFactory)

		// Setup nested parallel tasks
		parentCWD := &core.PathCWD{Path: "/root/parallel/directory"}
		parentFilePath := "configs/root.yaml"

		config := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "root-parallel",
				Type:     task.TaskTypeParallel,
				CWD:      parentCWD,
				FilePath: parentFilePath,
			},
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "nested-parallel",
						Type: task.TaskTypeParallel,
					},
					Tasks: []task.Config{
						{
							BaseConfig: task.BaseConfig{
								ID:   "leaf-basic",
								Type: task.TaskTypeBasic,
							},
						},
					},
				},
			},
		}

		ctx := &shared.NormalizationContext{
			Variables: make(map[string]any),
		}

		// Act
		err = normalizer.Normalize(t.Context(), config, ctx)

		// Assert
		require.NoError(t, err)

		// Verify the nested parallel task inherited from root
		assert.Equal(t, parentCWD, config.Tasks[0].CWD, "nested parallel should inherit root CWD")
		assert.Equal(t, parentFilePath, config.Tasks[0].FilePath, "nested parallel should inherit root FilePath")

		mockFactory.AssertExpectations(t)
		mockSubNormalizer.AssertExpectations(t)
	})

	t.Run("Should inherit config with Task reference field", func(t *testing.T) {
		t.Parallel()

		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)

		mockFactory := &MockNormalizerFactory{}
		mockSubNormalizer := &MockTaskNormalizer{}
		mockFactory.On("CreateNormalizer", task.TaskTypeBasic).Return(mockSubNormalizer, nil)
		mockSubNormalizer.On("Normalize", mock.Anything, mock.Anything).Return(nil)

		normalizer := parallel.NewNormalizer(t.Context(), templateEngine, contextBuilder, mockFactory)

		// Setup with Task reference
		parentCWD := &core.PathCWD{Path: "/parallel/working/directory"}
		parentFilePath := "configs/parallel.yaml"

		config := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "test-parallel",
				Type:     task.TaskTypeParallel,
				CWD:      parentCWD,
				FilePath: parentFilePath,
			},
			Task: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "ref-task",
					Type: task.TaskTypeBasic,
				},
			},
		}

		ctx := &shared.NormalizationContext{
			Variables: make(map[string]any),
		}

		// Act
		err = normalizer.Normalize(t.Context(), config, ctx)

		// Assert
		require.NoError(t, err)

		// Verify Task reference inherited config
		assert.Equal(t, parentCWD, config.Task.CWD, "Task reference should inherit parent CWD")
		assert.Equal(t, parentFilePath, config.Task.FilePath, "Task reference should inherit parent FilePath")

		mockFactory.AssertExpectations(t)
		mockSubNormalizer.AssertExpectations(t)
	})
}
