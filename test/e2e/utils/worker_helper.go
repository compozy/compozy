package utils

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/worker"
	wf "github.com/compozy/compozy/engine/workflow"
	testhelpers "github.com/compozy/compozy/test/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

// -----
// Worker Test Helper
// -----

// WorkerTestHelper provides utilities for worker lifecycle management and testing
type WorkerTestHelper struct {
	config *WorkerTestConfig
	t      *testing.T
}

// NewWorkerTestHelper creates a new worker test helper
func NewWorkerTestHelper(t *testing.T, config *WorkerTestConfig) *WorkerTestHelper {
	return &WorkerTestHelper{
		config: config,
		t:      t,
	}
}

// SetupWorkflowEnvironment sets up the Temporal workflow environment for testing
func (h *WorkerTestHelper) SetupWorkflowEnvironment(t *testing.T) {
	env := h.config.TemporalTestEnv
	config := h.config.ContainerTestConfig

	// Use the existing SetupWorkflowEnvironment function with our config
	testhelpers.SetupWorkflowEnvironment(t, env, config)
}

// RegisterWorkflows registers workflows for testing
func (h *WorkerTestHelper) RegisterWorkflows(workflows ...any) {
	for _, workflow := range workflows {
		h.config.TemporalTestEnv.RegisterWorkflow(workflow)
	}
}

// RegisterActivities registers activities for testing
func (h *WorkerTestHelper) RegisterActivities(activities ...any) {
	for _, activity := range activities {
		h.config.TemporalTestEnv.RegisterActivity(activity)
	}
}

// ExecuteWorkflow executes a workflow in the test environment
func (h *WorkerTestHelper) ExecuteWorkflow(
	workflowFn any,
	input any,
) {
	env := h.config.TemporalTestEnv
	env.ExecuteWorkflow(workflowFn, input)
}

// ExecuteWorkflowWithTimeout executes a workflow with a custom timeout
func (h *WorkerTestHelper) ExecuteWorkflowWithTimeout(
	timeout time.Duration,
	workflowFn any,
	input any,
) {
	env := h.config.TemporalTestEnv
	env.SetTestTimeout(timeout)
	env.ExecuteWorkflow(workflowFn, input)
}

// ValidateWorkflowCompleted validates that the workflow completed successfully
func (h *WorkerTestHelper) ValidateWorkflowCompleted() {
	// Assert expectations and ensure workflow completed
	h.config.TemporalTestEnv.AssertExpectations(h.t)

	// Additional validation - check if workflow is in completed state
	if !h.config.TemporalTestEnv.IsWorkflowCompleted() {
		h.t.Error("Workflow did not complete successfully")
	}
}

// -----
// State Validation Helpers
// -----

// ValidateWorkflowState validates the workflow state in the database
func (h *WorkerTestHelper) ValidateWorkflowState(
	ctx context.Context,
	workflowExecID core.ID,
	expectedStatus string,
) *wf.State {
	state, err := h.config.WorkflowRepo.GetState(ctx, workflowExecID)
	require.NoError(h.t, err)
	assert.Equal(h.t, expectedStatus, string(state.Status))
	return state
}

// ValidateTaskState validates a specific task state in the workflow
func (h *WorkerTestHelper) ValidateTaskState(
	state *wf.State,
	taskID string,
	expectedStatus string,
	expectedExecType task.ExecutionType,
) *task.State {
	require.Contains(h.t, state.Tasks, taskID)
	taskState := state.Tasks[taskID]
	assert.Equal(h.t, expectedStatus, string(taskState.Status))
	assert.Equal(h.t, expectedExecType, taskState.ExecutionType)
	return taskState
}

// ValidateTaskOutput validates task output contains expected data
func (h *WorkerTestHelper) ValidateTaskOutput(
	taskState *task.State,
	expectedKey string,
	expectedValue any,
) {
	require.NotNil(h.t, taskState.Output)
	output := *taskState.Output
	require.Contains(h.t, output, expectedKey)
	assert.Equal(h.t, expectedValue, output[expectedKey])
}

// -----
// Workflow Execution Helpers
// -----

// ExecuteBasicWorkflow executes a basic workflow test scenario
func (h *WorkerTestHelper) ExecuteBasicWorkflow(
	workflowID string,
	input *core.Input,
) core.ID {
	// Setup environment
	h.SetupWorkflowEnvironment(h.t)

	// Generate unique workflow execution ID
	workflowExecID := core.MustNewID()

	// Execute workflow
	h.ExecuteWorkflow(worker.CompozyWorkflow, worker.WorkflowInput{
		WorkflowID:     workflowID,
		WorkflowExecID: workflowExecID,
		Input:          input,
	})

	// Validate completion
	h.ValidateWorkflowCompleted()

	return workflowExecID
}

// -----
// Signal Helpers
// -----

// SendSignalAfterDelay sends a signal to the workflow after a delay
func (h *WorkerTestHelper) SendSignalAfterDelay(
	delay time.Duration,
	signalName string,
	signalData any,
) {
	h.config.TemporalTestEnv.RegisterDelayedCallback(func() {
		h.config.TemporalTestEnv.SignalWorkflow(signalName, signalData)
	}, delay)
}

// -----
// Cleanup and Utilities
// -----

// Cleanup performs test cleanup
func (h *WorkerTestHelper) Cleanup() {
	h.config.Cleanup(h.t)
}

// GetConfig returns the underlying test configuration
func (h *WorkerTestHelper) GetConfig() *WorkerTestConfig {
	return h.config
}

// GetTemporalEnv returns the temporal test environment
func (h *WorkerTestHelper) GetTemporalEnv() *testsuite.TestWorkflowEnvironment {
	return h.config.TemporalTestEnv
}

// GetWorkflows returns the loaded workflow configurations
func (h *WorkerTestHelper) GetWorkflows() []*wf.Config {
	if h.config.ContainerTestConfig == nil {
		return nil
	}
	return []*wf.Config{h.config.WorkflowConfig}
}

// CheckDatabaseState logs current database state for debugging
func (h *WorkerTestHelper) CheckDatabaseState(t *testing.T, workflowExecID string) {
	// This would connect to the test database and check states
	// For now, just log that we're checking
	t.Logf("Checking database state for workflow: %s", workflowExecID)
}

// TaskStateInfo represents a task state from database for debugging
type TaskStateInfo struct {
	TaskID        string
	Component     string
	Status        string
	ExecutionType string
	ParentStateID *string
}

// GetTaskStatesFromDB gets task states from database for debugging
func (h *WorkerTestHelper) GetTaskStatesFromDB(_ string) []TaskStateInfo {
	// For now, return empty slice - would need database connection
	// to implement properly
	return []TaskStateInfo{}
}
