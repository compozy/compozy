package wait

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/task2/wait"
	task2helpers "github.com/compozy/compozy/test/integration/task2/helpers"
)

// TestWaitConfigInheritance validates that wait tasks properly inherit
// CWD and FilePath to their processor configurations during normalization
func TestWaitConfigInheritance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests")
	}
	t.Parallel()

	t.Run("Should inherit CWD and FilePath to wait task processor", func(t *testing.T) {
		// Setup
		ts := task2helpers.NewTestSetup(t)

		// Create wait task config with CWD and FilePath
		waitCWD := &core.PathCWD{Path: "/wait/working/directory"}
		waitFilePath := "configs/wait.yaml"

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:        "test-wait-task",
				Type:      task.TaskTypeWait,
				CWD:       waitCWD,
				FilePath:  waitFilePath,
				Condition: "status == 'ready'",
				Timeout:   "30s",
			},
			WaitTask: task.WaitTask{
				WaitFor: "signal_ready",
				// Processor without explicit CWD/FilePath - should inherit
				Processor: &task.Config{
					BaseConfig: task.BaseConfig{
						ID:   "wait-processor",
						Type: task.TaskTypeBasic,
					},
					BasicTask: task.BasicTask{
						Action: "process_signal",
					},
				},
			},
		}

		// Create normalizer to test inheritance
		normalizer := wait.NewNormalizer(
			t.Context(),
			ts.TemplateEngine,
			ts.ContextBuilder,
		)

		// Setup normalization context
		normCtx := &shared.NormalizationContext{
			Variables: map[string]any{
				"signal_name": "ready_signal",
			},
		}

		// Normalize the wait task
		err := normalizer.Normalize(t.Context(), taskConfig, normCtx)
		require.NoError(t, err, "Wait normalization should succeed")

		// Verify processor inherited CWD and FilePath
		require.NotNil(t, taskConfig.Processor, "Wait task should have processor")
		require.NotNil(t, taskConfig.Processor.CWD, "Processor should inherit CWD")
		assert.Equal(t, "/wait/working/directory", taskConfig.Processor.CWD.Path,
			"Processor should inherit wait task CWD")
		assert.Equal(t, "configs/wait.yaml", taskConfig.Processor.FilePath,
			"Processor should inherit wait task FilePath")
	})

	t.Run("Should preserve explicit CWD and FilePath in processor", func(t *testing.T) {
		// Setup
		ts := task2helpers.NewTestSetup(t)

		// Create wait task with explicit processor CWD/FilePath
		waitCWD := &core.PathCWD{Path: "/wait/parent/path"}
		waitFilePath := "parent.yaml"
		processorCWD := &core.PathCWD{Path: "/processor/explicit/path"}
		processorFilePath := "processor.yaml"

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:        "test-wait-explicit",
				Type:      task.TaskTypeWait,
				CWD:       waitCWD,
				FilePath:  waitFilePath,
				Condition: "success == true",
				Timeout:   "60s",
			},
			WaitTask: task.WaitTask{
				WaitFor: "completion_signal",
				Processor: &task.Config{
					BaseConfig: task.BaseConfig{
						ID:       "explicit-processor",
						Type:     task.TaskTypeBasic,
						CWD:      processorCWD,      // Explicit CWD
						FilePath: processorFilePath, // Explicit FilePath
					},
					BasicTask: task.BasicTask{
						Action: "process_with_explicit_context",
					},
				},
			},
		}

		// Create normalizer
		normalizer := wait.NewNormalizer(
			t.Context(),
			ts.TemplateEngine,
			ts.ContextBuilder,
		)

		// Normalize
		normCtx := &shared.NormalizationContext{
			Variables: map[string]any{
				"wait_timeout": "60s",
			},
		}
		err := normalizer.Normalize(t.Context(), taskConfig, normCtx)
		require.NoError(t, err)

		// Verify processor preserved its explicit values
		require.NotNil(t, taskConfig.Processor.CWD)
		assert.Equal(t, "/processor/explicit/path", taskConfig.Processor.CWD.Path,
			"Processor should preserve explicit CWD")
		assert.Equal(t, "processor.yaml", taskConfig.Processor.FilePath,
			"Processor should preserve explicit FilePath")
	})

	t.Run("Should handle wait task with timeout inheritance", func(t *testing.T) {
		// Setup
		ts := task2helpers.NewTestSetup(t)

		// Create wait task with timeout configuration
		waitCWD := &core.PathCWD{Path: "/timeout/wait/dir"}

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:        "test-wait-timeout",
				Type:      task.TaskTypeWait,
				CWD:       waitCWD,
				Condition: "timeout_reached == false",
				Timeout:   "120s",
			},
			WaitTask: task.WaitTask{
				WaitFor:   "timeout_signal",
				OnTimeout: "timeout_handler",
				Processor: &task.Config{
					BaseConfig: task.BaseConfig{
						ID:   "timeout-processor",
						Type: task.TaskTypeBasic,
					},
					BasicTask: task.BasicTask{
						Action: "handle_timeout",
					},
				},
			},
		}

		// Create normalizer
		normalizer := wait.NewNormalizer(
			t.Context(),
			ts.TemplateEngine,
			ts.ContextBuilder,
		)

		// Normalize with timeout context
		normCtx := &shared.NormalizationContext{
			Variables: map[string]any{
				"max_wait_time": "120s",
			},
		}
		err := normalizer.Normalize(t.Context(), taskConfig, normCtx)
		require.NoError(t, err)

		// Verify inheritance and timeout configuration
		require.NotNil(t, taskConfig.Processor.CWD)
		assert.Equal(t, "/timeout/wait/dir", taskConfig.Processor.CWD.Path,
			"Processor should inherit CWD with timeout configuration")
		assert.Equal(t, "timeout_handler", taskConfig.OnTimeout,
			"Timeout handler should be preserved")
	})

	t.Run("Should handle wait task without processor", func(t *testing.T) {
		// Setup
		ts := task2helpers.NewTestSetup(t)

		// Create wait task without processor
		waitCWD := &core.PathCWD{Path: "/simple/wait/dir"}

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:        "test-wait-no-processor",
				Type:      task.TaskTypeWait,
				CWD:       waitCWD,
				FilePath:  "simple_wait.yaml",
				Condition: "simple_condition == true",
			},
			WaitTask: task.WaitTask{
				WaitFor: "simple_signal",
				// No processor defined
			},
		}

		// Create normalizer
		normalizer := wait.NewNormalizer(
			t.Context(),
			ts.TemplateEngine,
			ts.ContextBuilder,
		)

		// Normalize
		normCtx := &shared.NormalizationContext{
			Variables: map[string]any{
				"signal_type": "simple",
			},
		}
		err := normalizer.Normalize(t.Context(), taskConfig, normCtx)
		require.NoError(t, err)

		// Verify wait task configuration is valid without processor
		assert.Nil(t, taskConfig.Processor, "Wait task should not have processor when not configured")
		assert.Equal(t, "simple_signal", taskConfig.WaitFor, "WaitFor should be preserved")
	})
}
