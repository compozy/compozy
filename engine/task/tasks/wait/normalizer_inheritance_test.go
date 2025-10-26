package wait_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/engine/task/tasks/wait"
	"github.com/compozy/compozy/pkg/tplengine"
)

func TestWaitNormalizer_ProcessorInheritance(t *testing.T) {
	t.Run("Should inherit CWD to processor task", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)
		normalizer := wait.NewNormalizer(t.Context(), templateEngine, contextBuilder)

		waitTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "test-wait",
				Type:     task.TaskTypeWait,
				CWD:      &core.PathCWD{Path: "/wait/base"},
				FilePath: "wait.yaml",
			},
			WaitTask: task.WaitTask{
				WaitFor: "signal_1",
				Processor: &task.Config{
					BaseConfig: task.BaseConfig{
						ID:   "processor-task",
						Type: task.TaskTypeBasic,
						// No CWD or FilePath - should inherit from wait task
					},
					BasicTask: task.BasicTask{
						Action: "process_signal",
					},
				},
			},
		}

		ctx := &shared.NormalizationContext{
			Variables: make(map[string]any),
		}

		// Act
		err = normalizer.Normalize(t.Context(), waitTask, ctx)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, waitTask.Processor)

		// Check that processor inherited CWD
		require.NotNil(t, waitTask.Processor.CWD, "Processor should have inherited CWD")
		assert.Equal(t, "/wait/base", waitTask.Processor.CWD.Path, "Processor should inherit wait task CWD")

		// Check FilePath inheritance
		assert.Equal(t, "wait.yaml", waitTask.Processor.FilePath, "Processor should inherit wait task FilePath")
	})

	t.Run("Should not override existing processor CWD", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)
		normalizer := wait.NewNormalizer(t.Context(), templateEngine, contextBuilder)

		waitTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "test-wait",
				Type:     task.TaskTypeWait,
				CWD:      &core.PathCWD{Path: "/wait/base"},
				FilePath: "wait.yaml",
			},
			WaitTask: task.WaitTask{
				WaitFor: "signal_1",
				Processor: &task.Config{
					BaseConfig: task.BaseConfig{
						ID:       "processor-task",
						Type:     task.TaskTypeBasic,
						CWD:      &core.PathCWD{Path: "/processor/custom"},
						FilePath: "processor.yaml",
					},
					BasicTask: task.BasicTask{
						Action: "process_signal",
					},
				},
			},
		}

		ctx := &shared.NormalizationContext{
			Variables: make(map[string]any),
		}

		// Act
		err = normalizer.Normalize(t.Context(), waitTask, ctx)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, waitTask.Processor)

		// Check that processor kept its own CWD
		require.NotNil(t, waitTask.Processor.CWD)
		assert.Equal(t, "/processor/custom", waitTask.Processor.CWD.Path, "Processor should keep its own CWD")
		assert.Equal(t, "processor.yaml", waitTask.Processor.FilePath, "Processor should keep its own FilePath")
	})

	t.Run("Should handle nil processor gracefully", func(t *testing.T) {
		// Arrange
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)
		normalizer := wait.NewNormalizer(t.Context(), templateEngine, contextBuilder)

		waitTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "test-wait",
				Type:     task.TaskTypeWait,
				CWD:      &core.PathCWD{Path: "/wait/base"},
				FilePath: "wait.yaml",
			},
			WaitTask: task.WaitTask{
				WaitFor:   "signal_1",
				Processor: nil, // No processor
			},
		}

		ctx := &shared.NormalizationContext{
			Variables: make(map[string]any),
		}

		// Act
		err = normalizer.Normalize(t.Context(), waitTask, ctx)

		// Assert
		require.NoError(t, err, "Should handle nil processor without error")
	})
}
