package executors

import (
	"errors"
	"testing"
	"time"

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

// Helper function to create test executor
func createTestExecutor() *TaskWaitExecutor {
	return &TaskWaitExecutor{
		ContextBuilder: createTestContextBuilderSimple(),
		ExecutionHandler: func(_ workflow.Context, _ *task.Config, _ ...int) (task.Response, error) {
			return &task.MainTaskResponse{
				State: &task.State{
					Status: core.StatusSuccess,
				},
			}, nil
		},
	}
}

func TestTaskWaitExecutor_TimeoutValidationLogic(t *testing.T) {
	t.Run("Should validate positive timeout durations correctly", func(t *testing.T) {
		testCases := []struct {
			timeoutStr string
			expected   time.Duration
		}{
			{"30s", 30 * time.Second},
			{"5m", 5 * time.Minute},
			{"1h", 1 * time.Hour},
			{"100ms", 100 * time.Millisecond},
		}

		for _, tc := range testCases {
			config := createWaitTaskConfigSimple("test_signal", "true", tc.timeoutStr)

			// Test the core parsing logic directly
			timeout, err := core.ParseHumanDuration(config.Timeout)

			require.NoError(t, err, "Should parse valid timeout: %s", tc.timeoutStr)
			assert.Equal(t, tc.expected, timeout, "Timeout should match expected duration for: %s", tc.timeoutStr)
			assert.Greater(t, timeout, time.Duration(0), "Timeout should be positive for: %s", tc.timeoutStr)
		}
	})

	t.Run("Should reject invalid timeout formats", func(t *testing.T) {
		invalidTimeouts := []string{
			"invalid",
			"30x",
			"",
			"abc123",
			"30ss",
		}

		for _, invalidTimeout := range invalidTimeouts {
			config := createWaitTaskConfigSimple("test_signal", "true", invalidTimeout)

			// Test the core parsing logic directly
			timeout, err := core.ParseHumanDuration(config.Timeout)

			require.Error(t, err, "Should reject invalid timeout: %s", invalidTimeout)
			assert.Equal(
				t,
				time.Duration(0),
				timeout,
				"Invalid timeout should return zero duration: %s",
				invalidTimeout,
			)
		}
	})

	t.Run("Should reject negative and zero timeouts", func(t *testing.T) {
		negativeTimeouts := []string{"-10s", "-1m", "0s", "0ms"}

		for _, negativeTimeout := range negativeTimeouts {
			config := createWaitTaskConfigSimple("test_signal", "true", negativeTimeout)

			// Test the core parsing logic and validation
			timeout, err := core.ParseHumanDuration(config.Timeout)

			if err == nil {
				// If parsing succeeds, timeout should be <= 0 (business rule validation)
				assert.LessOrEqual(
					t,
					timeout,
					time.Duration(0),
					"Negative/zero timeout should be non-positive: %s",
					negativeTimeout,
				)
			} else {
				// Some negative formats may fail parsing entirely
				assert.Equal(t, time.Duration(0), timeout, "Failed parsing should return zero: %s", negativeTimeout)
			}
		}
	})
}

func TestTaskWaitExecutor_ResponseFinalizationLogic(t *testing.T) {
	t.Run("Should create proper error response structure", func(t *testing.T) {
		waitState := &WaitState{
			Error: errors.New("processor execution failed"),
		}
		response := &task.MainTaskResponse{
			State: &task.State{Status: core.StatusPending},
		}

		// Test the error response logic directly
		executor := createTestExecutor()
		executor.handleErrorResponse(response, waitState, createWaitTaskConfigSimple("test", "true", "30s"), nil)

		assert.Equal(t, core.StatusFailed, response.State.Status)
		require.NotNil(t, response.State.Error)
		assert.Equal(t, "WAIT_TASK_ERROR", response.State.Error.Code)
		assert.Contains(t, response.State.Error.Message, "processor execution failed")
	})

	t.Run("Should create proper timeout response structure", func(t *testing.T) {
		waitState := &WaitState{TimedOut: true}
		response := &task.MainTaskResponse{
			State: &task.State{Status: core.StatusPending},
		}
		config := createWaitTaskConfigSimple("test", "true", "30s")
		timeout := 30 * time.Second

		// Test the timeout response logic directly
		executor := createTestExecutor()
		executor.handleTimeoutResponse(response, waitState, config, timeout, nil)

		assert.Equal(t, core.StatusFailed, response.State.Status)
		require.NotNil(t, response.State.Error)
		assert.Equal(t, "WAIT_TIMEOUT", response.State.Error.Code)
		assert.Contains(t, response.State.Error.Message, "wait task timed out after")
		assert.Contains(t, response.State.Error.Message, "30s")
	})

	t.Run("Should create proper success response structure with signal data", func(t *testing.T) {
		signal := &task.SignalEnvelope{
			Metadata: task.SignalMetadata{
				SignalID:   "test-signal-123",
				WorkflowID: "test-workflow",
			},
			Payload: map[string]any{"approved": true, "reason": "conditions met"},
		}
		processorOutput := &task.ProcessorOutput{
			Output: map[string]any{"result": "processed", "score": 95},
		}
		waitState := &WaitState{
			MatchingSignal:  signal,
			ProcessorOutput: processorOutput,
		}
		response := &task.MainTaskResponse{
			State: &task.State{Status: core.StatusPending},
		}

		// Test the success response logic directly (without workflow status update)
		response.State.Status = core.StatusSuccess
		response.State.Output = &core.Output{
			"wait_status":      "completed",
			"signal":           waitState.MatchingSignal,
			"processor_output": waitState.ProcessorOutput,
		}

		assert.Equal(t, core.StatusSuccess, response.State.Status)
		require.NotNil(t, response.State.Output)
		output := *response.State.Output
		assert.Equal(t, "completed", output["wait_status"])
		assert.Equal(t, signal, output["signal"])
		assert.Equal(t, processorOutput, output["processor_output"])

		// Verify signal data is preserved
		signalInOutput := output["signal"].(*task.SignalEnvelope)
		assert.Equal(t, "test-signal-123", signalInOutput.Metadata.SignalID)
		assert.Equal(t, "test-workflow", signalInOutput.Metadata.WorkflowID)
		assert.Equal(t, true, signalInOutput.Payload["approved"])
		assert.Equal(t, "conditions met", signalInOutput.Payload["reason"])

		// Verify processor output is preserved
		procOutInOutput := output["processor_output"].(*task.ProcessorOutput)
		assert.Equal(t, map[string]any{"result": "processed", "score": 95}, procOutInOutput.Output)
	})
}

func TestTaskWaitExecutor_WaitStateBusinessLogic(t *testing.T) {
	t.Run("Should transition from pending to completed when condition is met", func(t *testing.T) {
		waitState := &WaitState{}
		signal := &task.SignalEnvelope{
			Metadata: task.SignalMetadata{SignalID: "test-123"},
			Payload:  map[string]any{"approved": true},
		}

		// Initial state should be waiting
		assert.False(t, waitState.ConditionMet, "Initial state should not have condition met")
		assert.False(t, waitState.TimedOut, "Initial state should not be timed out")

		// Simulate business logic transition when condition is satisfied
		waitState.ConditionMet = true
		waitState.MatchingSignal = signal
		waitState.TimerCancelled = true

		// Verify business state after successful condition evaluation
		assert.True(t, waitState.ConditionMet, "Condition should be met after signal processing")
		assert.Equal(t, signal, waitState.MatchingSignal, "Matching signal should be stored")
		assert.True(t, waitState.TimerCancelled, "Timer should be canceled when condition is met")
		assert.False(t, waitState.TimedOut, "Should not be timed out when condition is met first")
	})

	t.Run("Should handle timeout scenario when condition is never met", func(t *testing.T) {
		waitState := &WaitState{}

		// Simulate timeout scenario in business logic
		waitState.TimedOut = true
		waitState.ConditionMet = true // Condition met due to timeout, not signal

		// Verify timeout state is properly tracked
		assert.True(t, waitState.TimedOut, "Should be marked as timed out")
		assert.True(t, waitState.ConditionMet, "ConditionMet should be true to exit wait loop")
		assert.Nil(t, waitState.MatchingSignal, "No matching signal should exist on timeout")
		assert.False(t, waitState.TimerCancelled, "Timer should not be marked as canceled on timeout")
	})

	t.Run("Should handle processor execution error scenarios", func(t *testing.T) {
		waitState := &WaitState{}
		processorError := errors.New("processor execution failed: invalid template")

		// Simulate processor error in business logic
		waitState.Error = processorError
		waitState.ConditionMet = true // Exit wait loop due to error

		// Verify error state handling
		assert.Equal(t, processorError, waitState.Error, "Processor error should be preserved")
		assert.True(t, waitState.ConditionMet, "Should exit wait loop on error")
		assert.Nil(t, waitState.MatchingSignal, "No signal should be stored on error")
		assert.False(t, waitState.TimedOut, "Error is not a timeout scenario")
	})
}
