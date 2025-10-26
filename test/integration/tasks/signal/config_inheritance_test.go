package signal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/engine/task/tasks/signal"
	tkhelpers "github.com/compozy/compozy/test/integration/tasks/helpers"
)

// TestSignalConfigInheritance validates that signal tasks properly inherit
// CWD and FilePath when used as child tasks in parent task configurations
func TestSignalConfigInheritance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests")
	}
	t.Parallel()

	t.Run("Should inherit CWD and FilePath as child task", func(t *testing.T) {
		// Setup
		ts := tkhelpers.NewTestSetup(t)

		// Create signal task config without explicit CWD/FilePath
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-signal-child",
				Type: task.TaskTypeSignal,
				// No CWD/FilePath - will be inherited by parent normalizer
			},
			SignalTask: task.SignalTask{
				Signal: &task.SignalConfig{
					ID: "{{ .signal_name }}",
					Payload: map[string]any{
						"status": "{{ .current_status }}",
						"data":   "{{ .signal_data }}",
					},
				},
			},
		}

		// Simulate inheritance by parent normalizer
		parentConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				CWD:      &core.PathCWD{Path: "/parent/signal/directory"},
				FilePath: "configs/parent.yaml",
			},
		}

		// Apply inheritance like a parent normalizer would
		shared.InheritTaskConfig(taskConfig, parentConfig)

		// Create normalizer to test normalization with inherited context
		normalizer := signal.NewNormalizer(
			t.Context(),
			ts.TemplateEngine,
			ts.ContextBuilder,
		)

		// Setup normalization context
		normCtx := &shared.NormalizationContext{
			Variables: map[string]any{
				"signal_name":    "user_action_complete",
				"current_status": "ready",
				"signal_data":    "operation_completed",
			},
		}

		// Normalize the signal task
		err := normalizer.Normalize(t.Context(), taskConfig, normCtx)
		require.NoError(t, err, "Signal normalization should succeed")

		// Verify signal task inherited context
		require.NotNil(t, taskConfig.CWD, "Signal task should have inherited CWD")
		assert.Equal(t, "/parent/signal/directory", taskConfig.CWD.Path,
			"Signal task should inherit parent CWD")
		assert.Equal(t, "configs/parent.yaml", taskConfig.FilePath,
			"Signal task should inherit parent FilePath")

		// Verify signal-specific fields were normalized
		require.NotNil(t, taskConfig.Signal, "Signal config should be present")
		assert.Equal(t, "user_action_complete", taskConfig.Signal.ID,
			"Signal ID should be processed with template variables")
		assert.Equal(t, "ready", taskConfig.Signal.Payload["status"],
			"Signal payload should be processed with template variables")
	})

	t.Run("Should preserve explicit CWD and FilePath", func(t *testing.T) {
		// Setup
		ts := tkhelpers.NewTestSetup(t)

		// Create signal task with explicit CWD/FilePath
		explicitCWD := &core.PathCWD{Path: "/explicit/signal/path"}
		explicitFilePath := "explicit_signal.yaml"

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:       "test-signal-explicit",
				Type:     task.TaskTypeSignal,
				CWD:      explicitCWD,      // Explicit CWD
				FilePath: explicitFilePath, // Explicit FilePath
			},
			SignalTask: task.SignalTask{
				Signal: &task.SignalConfig{
					ID: "explicit_signal",
					Payload: map[string]any{
						"message": "explicit configuration",
					},
				},
			},
		}

		// Try to apply inheritance (should not override existing values)
		parentConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				CWD:      &core.PathCWD{Path: "/parent/path"},
				FilePath: "parent.yaml",
			},
		}

		shared.InheritTaskConfig(taskConfig, parentConfig)

		// Create normalizer
		normalizer := signal.NewNormalizer(
			t.Context(),
			ts.TemplateEngine,
			ts.ContextBuilder,
		)

		// Normalize
		normCtx := &shared.NormalizationContext{
			Variables: map[string]any{
				"context": "test",
			},
		}
		err := normalizer.Normalize(t.Context(), taskConfig, normCtx)
		require.NoError(t, err)

		// Verify signal task preserved its explicit values
		require.NotNil(t, taskConfig.CWD)
		assert.Equal(t, "/explicit/signal/path", taskConfig.CWD.Path,
			"Signal task should preserve explicit CWD")
		assert.Equal(t, "explicit_signal.yaml", taskConfig.FilePath,
			"Signal task should preserve explicit FilePath")
	})

	t.Run("Should handle signal task with templated signal data", func(t *testing.T) {
		// Setup
		ts := tkhelpers.NewTestSetup(t)

		// Create signal task with complex templated configuration
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-templated-signal",
				Type: task.TaskTypeSignal,
			},
			SignalTask: task.SignalTask{
				Signal: &task.SignalConfig{
					ID: "{{ .workflow_id }}_{{ .task_phase }}",
					Payload: map[string]any{
						"workflow_status": "{{ .status }}",
						"completion_time": "{{ .timestamp }}",
						"metadata": map[string]any{
							"phase":    "{{ .task_phase }}",
							"priority": "{{ .task_priority }}",
						},
					},
				},
			},
		}

		// Apply inheritance
		parentConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				CWD:      &core.PathCWD{Path: "/templated/signal/dir"},
				FilePath: "templated.yaml",
			},
		}

		shared.InheritTaskConfig(taskConfig, parentConfig)

		// Create normalizer
		normalizer := signal.NewNormalizer(
			t.Context(),
			ts.TemplateEngine,
			ts.ContextBuilder,
		)

		// Normalize with complex template context
		normCtx := &shared.NormalizationContext{
			Variables: map[string]any{
				"workflow_id":   "workflow_123",
				"task_phase":    "completion",
				"status":        "success",
				"timestamp":     "2023-07-04T10:00:00Z",
				"task_priority": "high",
			},
		}
		err := normalizer.Normalize(t.Context(), taskConfig, normCtx)
		require.NoError(t, err)

		// Verify inheritance and template processing
		require.NotNil(t, taskConfig.CWD)
		assert.Equal(t, "/templated/signal/dir", taskConfig.CWD.Path,
			"Signal task should inherit CWD for templated configuration")

		// Verify template processing
		require.NotNil(t, taskConfig.Signal)
		assert.Equal(t, "workflow_123_completion", taskConfig.Signal.ID,
			"Signal ID should be processed with template variables")
		assert.Equal(t, "success", taskConfig.Signal.Payload["workflow_status"],
			"Signal payload status should be processed")

		// Verify nested payload processing
		metadata, ok := taskConfig.Signal.Payload["metadata"].(map[string]any)
		require.True(t, ok, "Metadata should be processed as map")
		assert.Equal(t, "completion", metadata["phase"],
			"Nested template variables should be processed")
		assert.Equal(t, "high", metadata["priority"],
			"Nested template variables should be processed")
	})

	t.Run("Should handle signal task with empty payload", func(t *testing.T) {
		// Setup
		ts := tkhelpers.NewTestSetup(t)

		// Create signal task with minimal configuration
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-minimal-signal",
				Type: task.TaskTypeSignal,
			},
			SignalTask: task.SignalTask{
				Signal: &task.SignalConfig{
					ID: "minimal_signal",
					// No payload
				},
			},
		}

		// Apply inheritance
		parentConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				CWD:      &core.PathCWD{Path: "/minimal/signal/dir"},
				FilePath: "minimal.yaml",
			},
		}

		shared.InheritTaskConfig(taskConfig, parentConfig)

		// Create normalizer
		normalizer := signal.NewNormalizer(
			t.Context(),
			ts.TemplateEngine,
			ts.ContextBuilder,
		)

		// Normalize
		normCtx := &shared.NormalizationContext{
			Variables: map[string]any{
				"context": "minimal",
			},
		}
		err := normalizer.Normalize(t.Context(), taskConfig, normCtx)
		require.NoError(t, err)

		// Verify inheritance works with minimal configuration
		require.NotNil(t, taskConfig.CWD)
		assert.Equal(t, "/minimal/signal/dir", taskConfig.CWD.Path,
			"Signal task should inherit CWD even with minimal configuration")
		assert.Equal(t, "minimal.yaml", taskConfig.FilePath,
			"Signal task should inherit FilePath")

		// Verify signal configuration is preserved
		require.NotNil(t, taskConfig.Signal)
		assert.Equal(t, "minimal_signal", taskConfig.Signal.ID,
			"Signal ID should be preserved")
		assert.Nil(t, taskConfig.Signal.Payload,
			"Empty payload should remain nil")
	})
}
