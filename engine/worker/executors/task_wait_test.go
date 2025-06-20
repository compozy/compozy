package executors

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/task"
	tkacts "github.com/compozy/compozy/engine/task/activities"
	wfconfig "github.com/compozy/compozy/engine/workflow"
	wfacts "github.com/compozy/compozy/engine/workflow/activities"
)

// Test helper to create a wait task config
func createWaitTaskConfig(waitFor, condition, timeout string) *task.Config {
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
func createTestContextBuilder() *ContextBuilder {
	CWD, _ := core.CWDFromPath("/tmp")
	return &ContextBuilder{
		WorkflowInput: &WorkflowInput{
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
		},
		ProjectConfig: &project.Config{
			Opts: project.Opts{},
		},
		WorkflowConfig: &wfconfig.Config{
			ID:  "test-workflow",
			CWD: CWD,
		},
	}
}

func TestTaskWaitExecutor_Execute(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	t.Run("Should wait for signal and meet condition", func(t *testing.T) {
		contextBuilder := createTestContextBuilder()
		executionHandler := func(_ workflow.Context, _ *task.Config, _ ...int) (task.Response, error) {
			return &task.MainTaskResponse{}, nil
		}
		executor := NewTaskWaitExecutor(contextBuilder, executionHandler)
		config := createWaitTaskConfig("approval", `signal.payload.status == "approved"`, "30s")
		// Mock activities
		env.OnActivity(tkacts.ExecuteWaitLabel, mock.Anything, mock.Anything).Return(
			&task.MainTaskResponse{
				State: &task.State{
					TaskID:         config.ID,
					TaskExecID:     core.MustNewID(),
					WorkflowID:     contextBuilder.WorkflowID,
					WorkflowExecID: contextBuilder.WorkflowExecID,
					Status:         core.StatusWaiting,
					Output: &core.Output{
						"wait_status":   "waiting",
						"signal_name":   config.WaitFor,
						"has_processor": false,
					},
				},
			}, nil)
		env.OnActivity(wfacts.UpdateStateLabel, mock.Anything, mock.Anything).Return(nil)
		env.OnActivity(tkacts.EvaluateConditionLabel, mock.Anything, mock.Anything).Return(true, nil)
		env.OnActivity(tkacts.LoadTaskConfigLabel, mock.Anything, mock.Anything).Return(nil, nil)
		// Execute workflow
		env.RegisterDelayedCallback(func() {
			signal := task.SignalEnvelope{
				Metadata: task.SignalMetadata{
					SignalID:      "signal-1",
					WorkflowID:    contextBuilder.WorkflowID,
					ReceivedAtUTC: time.Now(),
				},
				Payload: map[string]any{
					"status": "approved",
				},
			}
			env.SignalWorkflow("approval", signal)
		}, 100*time.Millisecond)
		env.ExecuteWorkflow(func(ctx workflow.Context) error {
			result, err := executor.Execute(ctx, config)
			require.NoError(t, err)
			require.NotNil(t, result)
			response := result.(*task.MainTaskResponse)
			assert.Equal(t, core.StatusSuccess, response.State.Status)
			assert.NotNil(t, response.State.Output)
			output := *response.State.Output
			assert.Equal(t, "completed", output["wait_status"])
			assert.NotNil(t, output["signal"])
			return nil
		})
		require.True(t, env.IsWorkflowCompleted())
		require.NoError(t, env.GetWorkflowError())
	})

	t.Run("Should handle timeout", func(t *testing.T) {
		contextBuilder := createTestContextBuilder()
		executionHandler := func(_ workflow.Context, _ *task.Config, _ ...int) (task.Response, error) {
			return &task.MainTaskResponse{}, nil
		}
		executor := NewTaskWaitExecutor(contextBuilder, executionHandler)
		config := createWaitTaskConfig("approval", `signal.payload.status == "approved"`, "100ms")
		// Mock activities
		env.OnActivity(tkacts.ExecuteWaitLabel, mock.Anything, mock.Anything).Return(
			&task.MainTaskResponse{
				State: &task.State{
					TaskID:         config.ID,
					TaskExecID:     core.MustNewID(),
					WorkflowID:     contextBuilder.WorkflowID,
					WorkflowExecID: contextBuilder.WorkflowExecID,
					Status:         core.StatusWaiting,
				},
			}, nil)
		env.OnActivity(wfacts.UpdateStateLabel, mock.Anything, mock.Anything).Return(nil)
		// Don't send any signal, let it timeout
		env.ExecuteWorkflow(func(ctx workflow.Context) error {
			result, err := executor.Execute(ctx, config)
			require.NoError(t, err)
			require.NotNil(t, result)
			response := result.(*task.MainTaskResponse)
			assert.Equal(t, core.StatusFailed, response.State.Status)
			assert.NotNil(t, response.State.Error)
			assert.Equal(t, "WAIT_TIMEOUT", response.State.Error.Code)
			return nil
		})
		require.True(t, env.IsWorkflowCompleted())
		require.NoError(t, env.GetWorkflowError())
	})

	t.Run("Should execute processor before evaluating condition", func(t *testing.T) {
		contextBuilder := createTestContextBuilder()
		processorExecuted := false
		executionHandler := func(_ workflow.Context, taskConfig *task.Config, _ ...int) (task.Response, error) {
			if taskConfig.ID == "processor-task" {
				processorExecuted = true
				return &task.MainTaskResponse{
					State: &task.State{
						Output: &core.Output{
							"validated": true,
							"score":     95,
						},
					},
				}, nil
			}
			return &task.MainTaskResponse{}, nil
		}
		executor := NewTaskWaitExecutor(contextBuilder, executionHandler)
		// Create processor config
		CWD, _ := core.CWDFromPath("/tmp")
		processorConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "processor-task",
				Type: task.TaskTypeBasic,
				CWD:  CWD,
			},
			BasicTask: task.BasicTask{
				Action: "validate_signal",
			},
		}
		config := createWaitTaskConfig(
			"data",
			`processor.output.validated == true && processor.output.score > 90`,
			"5s",
		)
		config.Processor = processorConfig
		// Mock activities
		env.OnActivity(tkacts.ExecuteWaitLabel, mock.Anything, mock.Anything).Return(
			&task.MainTaskResponse{
				State: &task.State{
					TaskID:         config.ID,
					TaskExecID:     core.MustNewID(),
					WorkflowID:     contextBuilder.WorkflowID,
					WorkflowExecID: contextBuilder.WorkflowExecID,
					Status:         core.StatusWaiting,
				},
			}, nil)
		env.OnActivity(wfacts.UpdateStateLabel, mock.Anything, mock.Anything).Return(nil)
		env.OnActivity(tkacts.EvaluateConditionLabel, mock.Anything, mock.Anything).
			Return(func(_ workflow.Context, input *tkacts.EvaluateConditionInput) bool {
				// Verify processor output is included
				assert.NotNil(t, input.ProcessorOutput)
				assert.NotNil(t, input.ProcessorOutput.Output)
				if outputMap, ok := input.ProcessorOutput.Output.(*core.Output); ok {
					return (*outputMap)["validated"] == true && (*outputMap)["score"].(int) > 90
				}
				return false
			}, nil)
		// Send signal
		env.RegisterDelayedCallback(func() {
			signal := task.SignalEnvelope{
				Metadata: task.SignalMetadata{
					SignalID:   "signal-1",
					WorkflowID: contextBuilder.WorkflowID,
				},
				Payload: map[string]any{
					"data": "test-data",
				},
			}
			env.SignalWorkflow("data", signal)
		}, 100*time.Millisecond)
		env.ExecuteWorkflow(func(ctx workflow.Context) error {
			result, err := executor.Execute(ctx, config)
			require.NoError(t, err)
			require.NotNil(t, result)
			response := result.(*task.MainTaskResponse)
			assert.Equal(t, core.StatusSuccess, response.State.Status)
			assert.True(t, processorExecuted, "Processor should have been executed")
			return nil
		})
		require.True(t, env.IsWorkflowCompleted())
		require.NoError(t, env.GetWorkflowError())
	})

	t.Run("Should handle processor failure with OnError continue", func(t *testing.T) {
		contextBuilder := createTestContextBuilder()
		executionHandler := func(_ workflow.Context, taskConfig *task.Config, _ ...int) (task.Response, error) {
			if taskConfig.ID == "processor-task" {
				return nil, assert.AnError
			}
			return &task.MainTaskResponse{}, nil
		}
		executor := NewTaskWaitExecutor(contextBuilder, executionHandler)
		// Create processor config
		CWD, _ := core.CWDFromPath("/tmp")
		processorConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "processor-task",
				Type: task.TaskTypeBasic,
				CWD:  CWD,
			},
			BasicTask: task.BasicTask{
				Action: "process_signal",
			},
		}
		config := createWaitTaskConfig("data", `processor.output.valid == true`, "5s")
		config.Processor = processorConfig
		// Mock activities
		env.OnActivity(tkacts.ExecuteWaitLabel, mock.Anything, mock.Anything).Return(
			&task.MainTaskResponse{
				State: &task.State{
					TaskID:         config.ID,
					TaskExecID:     core.MustNewID(),
					WorkflowID:     contextBuilder.WorkflowID,
					WorkflowExecID: contextBuilder.WorkflowExecID,
					Status:         core.StatusWaiting,
				},
			}, nil)
		env.OnActivity(wfacts.UpdateStateLabel, mock.Anything, mock.Anything).Return(nil)
		// Send signal
		env.RegisterDelayedCallback(func() {
			signal := task.SignalEnvelope{
				Metadata: task.SignalMetadata{
					SignalID:   "signal-1",
					WorkflowID: contextBuilder.WorkflowID,
				},
				Payload: map[string]any{
					"data": "test-data",
				},
			}
			env.SignalWorkflow("data", signal)
		}, 100*time.Millisecond)
		env.ExecuteWorkflow(func(ctx workflow.Context) error {
			result, err := executor.Execute(ctx, config)
			require.NoError(t, err)
			require.NotNil(t, result)
			response := result.(*task.MainTaskResponse)
			// Should complete successfully due to OnError continue
			assert.Equal(t, core.StatusSuccess, response.State.Status)
			return nil
		})
		require.True(t, env.IsWorkflowCompleted())
		require.NoError(t, env.GetWorkflowError())
	})

	t.Run("Should handle condition not met", func(t *testing.T) {
		contextBuilder := createTestContextBuilder()
		executionHandler := func(_ workflow.Context, _ *task.Config, _ ...int) (task.Response, error) {
			return &task.MainTaskResponse{}, nil
		}
		executor := NewTaskWaitExecutor(contextBuilder, executionHandler)
		config := createWaitTaskConfig("approval", `signal.payload.status == "approved"`, "1s")
		// Mock activities
		env.OnActivity(tkacts.ExecuteWaitLabel, mock.Anything, mock.Anything).Return(
			&task.MainTaskResponse{
				State: &task.State{
					TaskID:         config.ID,
					TaskExecID:     core.MustNewID(),
					WorkflowID:     contextBuilder.WorkflowID,
					WorkflowExecID: contextBuilder.WorkflowExecID,
					Status:         core.StatusWaiting,
				},
			}, nil)
		env.OnActivity(wfacts.UpdateStateLabel, mock.Anything, mock.Anything).Return(nil)
		env.OnActivity(tkacts.EvaluateConditionLabel, mock.Anything, mock.Anything).Return(false, nil)
		// Send multiple signals that don't meet condition
		env.RegisterDelayedCallback(func() {
			signal := task.SignalEnvelope{
				Metadata: task.SignalMetadata{
					SignalID:   "signal-1",
					WorkflowID: contextBuilder.WorkflowID,
				},
				Payload: map[string]any{
					"status": "rejected",
				},
			}
			env.SignalWorkflow("approval", signal)
		}, 100*time.Millisecond)
		env.RegisterDelayedCallback(func() {
			signal := task.SignalEnvelope{
				Metadata: task.SignalMetadata{
					SignalID:   "signal-2",
					WorkflowID: contextBuilder.WorkflowID,
				},
				Payload: map[string]any{
					"status": "pending",
				},
			}
			env.SignalWorkflow("approval", signal)
		}, 200*time.Millisecond)
		env.ExecuteWorkflow(func(ctx workflow.Context) error {
			result, err := executor.Execute(ctx, config)
			require.NoError(t, err)
			require.NotNil(t, result)
			response := result.(*task.MainTaskResponse)
			// Should timeout since condition is never met
			assert.Equal(t, core.StatusFailed, response.State.Status)
			assert.NotNil(t, response.State.Error)
			assert.Equal(t, "WAIT_TIMEOUT", response.State.Error.Code)
			return nil
		})
		require.True(t, env.IsWorkflowCompleted())
		require.NoError(t, env.GetWorkflowError())
	})

	t.Run("Should handle OnTimeout task", func(t *testing.T) {
		contextBuilder := createTestContextBuilder()
		executionHandler := func(_ workflow.Context, _ *task.Config, _ ...int) (task.Response, error) {
			return &task.MainTaskResponse{}, nil
		}
		executor := NewTaskWaitExecutor(contextBuilder, executionHandler)
		config := createWaitTaskConfig("approval", `signal.payload.status == "approved"`, "100ms")
		config.OnTimeout = "timeout-handler"
		// Create timeout handler config
		CWD, _ := core.CWDFromPath("/tmp")
		timeoutTaskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "timeout-handler",
				Type: task.TaskTypeBasic,
				CWD:  CWD,
			},
			BasicTask: task.BasicTask{
				Action: "handle_timeout",
			},
		}
		// Mock activities
		env.OnActivity(tkacts.ExecuteWaitLabel, mock.Anything, mock.Anything).Return(
			&task.MainTaskResponse{
				State: &task.State{
					TaskID:         config.ID,
					TaskExecID:     core.MustNewID(),
					WorkflowID:     contextBuilder.WorkflowID,
					WorkflowExecID: contextBuilder.WorkflowExecID,
					Status:         core.StatusWaiting,
				},
			}, nil)
		env.OnActivity(wfacts.UpdateStateLabel, mock.Anything, mock.Anything).Return(nil)
		env.OnActivity(tkacts.LoadTaskConfigLabel, mock.Anything, mock.MatchedBy(func(input *tkacts.LoadTaskConfigInput) bool {
			return input.TaskID == "timeout-handler"
		})).
			Return(timeoutTaskConfig, nil)
		// Don't send any signal, let it timeout
		env.ExecuteWorkflow(func(ctx workflow.Context) error {
			result, err := executor.Execute(ctx, config)
			require.NoError(t, err)
			require.NotNil(t, result)
			response := result.(*task.MainTaskResponse)
			assert.Equal(t, core.StatusFailed, response.State.Status)
			assert.NotNil(t, response.State.Error)
			// Should have next task set to timeout handler
			assert.NotNil(t, response.NextTask)
			assert.Equal(t, "timeout-handler", response.NextTask.ID)
			return nil
		})
		require.True(t, env.IsWorkflowCompleted())
		require.NoError(t, env.GetWorkflowError())
	})
}

func TestTaskWaitExecutor_EdgeCases(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	t.Run("Should handle empty condition as always true", func(t *testing.T) {
		contextBuilder := createTestContextBuilder()
		executionHandler := func(_ workflow.Context, _ *task.Config, _ ...int) (task.Response, error) {
			return &task.MainTaskResponse{}, nil
		}
		executor := NewTaskWaitExecutor(contextBuilder, executionHandler)
		config := createWaitTaskConfig("any-signal", "", "5s") // Empty condition
		// Mock activities
		env.OnActivity(tkacts.ExecuteWaitLabel, mock.Anything, mock.Anything).Return(
			&task.MainTaskResponse{
				State: &task.State{
					TaskID:         config.ID,
					TaskExecID:     core.MustNewID(),
					WorkflowID:     contextBuilder.WorkflowID,
					WorkflowExecID: contextBuilder.WorkflowExecID,
					Status:         core.StatusWaiting,
				},
			}, nil)
		env.OnActivity(wfacts.UpdateStateLabel, mock.Anything, mock.Anything).Return(nil)
		env.OnActivity(tkacts.EvaluateConditionLabel, mock.Anything, mock.Anything).Return(true, nil)
		// Send any signal
		env.RegisterDelayedCallback(func() {
			signal := task.SignalEnvelope{
				Metadata: task.SignalMetadata{
					SignalID:   "signal-1",
					WorkflowID: contextBuilder.WorkflowID,
				},
				Payload: map[string]any{
					"foo": "bar",
				},
			}
			env.SignalWorkflow("any-signal", signal)
		}, 100*time.Millisecond)
		env.ExecuteWorkflow(func(ctx workflow.Context) error {
			result, err := executor.Execute(ctx, config)
			require.NoError(t, err)
			require.NotNil(t, result)
			response := result.(*task.MainTaskResponse)
			assert.Equal(t, core.StatusSuccess, response.State.Status)
			return nil
		})
		require.True(t, env.IsWorkflowCompleted())
		require.NoError(t, env.GetWorkflowError())
	})

	t.Run("Should update workflow status to paused while waiting", func(t *testing.T) {
		contextBuilder := createTestContextBuilder()
		executionHandler := func(_ workflow.Context, _ *task.Config, _ ...int) (task.Response, error) {
			return &task.MainTaskResponse{}, nil
		}
		executor := NewTaskWaitExecutor(contextBuilder, executionHandler)
		config := createWaitTaskConfig("approval", `signal.payload.approved == true`, "5s")
		statusUpdates := []core.StatusType{}
		// Mock activities
		env.OnActivity(tkacts.ExecuteWaitLabel, mock.Anything, mock.Anything).Return(
			&task.MainTaskResponse{
				State: &task.State{
					TaskID:         config.ID,
					TaskExecID:     core.MustNewID(),
					WorkflowID:     contextBuilder.WorkflowID,
					WorkflowExecID: contextBuilder.WorkflowExecID,
					Status:         core.StatusWaiting,
				},
			}, nil)
		env.OnActivity(wfacts.UpdateStateLabel, mock.Anything, mock.MatchedBy(func(input *wfacts.UpdateStateInput) bool {
			statusUpdates = append(statusUpdates, input.Status)
			return true
		})).
			Return(nil)
		env.OnActivity(tkacts.EvaluateConditionLabel, mock.Anything, mock.Anything).Return(true, nil)
		// Send signal
		env.RegisterDelayedCallback(func() {
			signal := task.SignalEnvelope{
				Metadata: task.SignalMetadata{
					SignalID:   "signal-1",
					WorkflowID: contextBuilder.WorkflowID,
				},
				Payload: map[string]any{
					"approved": true,
				},
			}
			env.SignalWorkflow("approval", signal)
		}, 100*time.Millisecond)
		env.ExecuteWorkflow(func(ctx workflow.Context) error {
			result, err := executor.Execute(ctx, config)
			require.NoError(t, err)
			require.NotNil(t, result)
			// Verify workflow status was updated to paused then running
			require.Len(t, statusUpdates, 2)
			assert.Equal(t, core.StatusPaused, statusUpdates[0])
			assert.Equal(t, core.StatusRunning, statusUpdates[1])
			return nil
		})
		require.True(t, env.IsWorkflowCompleted())
		require.NoError(t, env.GetWorkflowError())
	})
}
