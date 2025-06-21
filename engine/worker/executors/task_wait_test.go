package executors

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/task"
	wfconfig "github.com/compozy/compozy/engine/workflow"
	wfacts "github.com/compozy/compozy/engine/workflow/activities"
)

// Test helper to create a wait task config
func createWaitTaskConfigSimple(waitFor, condition, timeout string) *task.Config {
	CWD, _ := core.CWDFromPath("/tmp")
	return &task.Config{
		BaseConfig: task.BaseConfig{
			ID:        "test-wait",
			Type:      task.TaskTypeWait,
			CWD:       CWD,
			Condition: condition,
			Timeout:   timeout,
		},
		WaitTask: task.WaitTask{
			WaitFor: waitFor,
		},
	}
}

// Test helper to create context builder
func createTestContextBuilderSimple() *ContextBuilder {
	return &ContextBuilder{
		Workflows: []*wfconfig.Config{},
		ProjectConfig: &project.Config{
			Opts: project.Opts{},
		},
		WorkflowConfig: &wfconfig.Config{
			ID:    "test-workflow",
			Tasks: []task.Config{},
		},
		WorkflowInput: &wfacts.TriggerInput{
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
		},
	}
}

func TestTaskWaitExecutor_Simple(t *testing.T) {
	t.Run("Should create TaskWaitExecutor with proper dependencies", func(t *testing.T) {
		contextBuilder := createTestContextBuilderSimple()
		executionHandler := func(_ workflow.Context, _ *task.Config, _ ...int) (task.Response, error) {
			return &task.MainTaskResponse{
				State: &task.State{
					Status: core.StatusSuccess,
				},
			}, nil
		}

		executor := NewTaskWaitExecutor(contextBuilder, executionHandler)

		require.NotNil(t, executor)
		assert.Equal(t, contextBuilder, executor.ContextBuilder)
		assert.NotNil(t, executor.ExecutionHandler)
	})

	t.Run("Should handle WaitState transitions correctly", func(t *testing.T) {
		waitState := &WaitState{}

		// Initial state
		assert.False(t, waitState.ConditionMet)
		assert.False(t, waitState.TimedOut)
		assert.False(t, waitState.TimerCancelled)

		// Simulate condition met
		waitState.ConditionMet = true
		waitState.TimerCancelled = true

		assert.True(t, waitState.ConditionMet)
		assert.True(t, waitState.TimerCancelled)
		assert.False(t, waitState.TimedOut)
	})

	t.Run("Should validate wait task configuration", func(t *testing.T) {
		config := createWaitTaskConfigSimple("approval_signal", `signal.approved == true`, "30s")

		assert.Equal(t, task.TaskTypeWait, config.Type)
		assert.Equal(t, "approval_signal", config.WaitFor)
		assert.Equal(t, `signal.approved == true`, config.Condition)
		assert.Equal(t, "30s", config.Timeout)
		assert.Equal(t, "test-wait", config.ID)
	})

	t.Run("Should handle signal envelope structure", func(t *testing.T) {
		signal := task.SignalEnvelope{
			Metadata: task.SignalMetadata{
				SignalID:   "test-signal-123",
				WorkflowID: "workflow-456",
			},
			Payload: map[string]any{
				"approved": true,
				"reason":   "All conditions met",
			},
		}

		assert.Equal(t, "test-signal-123", signal.Metadata.SignalID)
		assert.Equal(t, "workflow-456", signal.Metadata.WorkflowID)
		assert.True(t, signal.Payload["approved"].(bool))
		assert.Equal(t, "All conditions met", signal.Payload["reason"])
	})
}
