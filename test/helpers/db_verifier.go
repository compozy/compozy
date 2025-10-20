package helpers

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----
// Database State Verification
// -----

// DatabaseStateVerifier helps verify workflow and task states in the database
type DatabaseStateVerifier struct {
	t            *testing.T
	workflowRepo workflow.Repository
	taskRepo     task.Repository
}

// NewDatabaseStateVerifier creates a new database state verifier
func NewDatabaseStateVerifier(
	t *testing.T,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
) *DatabaseStateVerifier {
	return &DatabaseStateVerifier{
		t:            t,
		workflowRepo: workflowRepo,
		taskRepo:     taskRepo,
	}
}

// VerifyWorkflowState verifies that a workflow has the expected status in the database
func (v *DatabaseStateVerifier) VerifyWorkflowState(
	workflowExecID core.ID,
	expectedStatus core.StatusType,
	timeoutDuration ...time.Duration,
) {
	v.t.Helper()
	timeout := DefaultTestTimeout
	if len(timeoutDuration) > 0 {
		timeout = timeoutDuration[0]
	}
	ctx, cancel := context.WithTimeout(v.t.Context(), timeout)
	defer cancel()
	ticker := time.NewTicker(DefaultPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			v.t.Fatalf("Timeout waiting for workflow %s to reach status %s", workflowExecID, expectedStatus)
		case <-ticker.C:
			state, err := v.workflowRepo.GetState(v.t.Context(), workflowExecID)
			if err != nil {
				v.t.Logf("Warning: could not get workflow state: %v", err)
				continue
			}

			v.t.Logf("Workflow %s current status: %s (expecting %s)", workflowExecID, state.Status, expectedStatus)

			if state.Status == expectedStatus {
				return
			}
		}
	}
}

// VerifyWorkflowStateEventually verifies workflow state with retry logic
func (v *DatabaseStateVerifier) VerifyWorkflowStateEventually(
	workflowExecID core.ID,
	expectedStatus core.StatusType,
	maxWait time.Duration,
) {
	v.t.Helper()
	require.Eventually(
		v.t,
		func() bool {
			state, err := v.workflowRepo.GetState(v.t.Context(), workflowExecID)
			if err != nil {
				v.t.Logf("Could not get workflow state: %v", err)
				return false
			}
			v.t.Logf("Workflow %s status: %s (expecting %s)", workflowExecID, state.Status, expectedStatus)
			return state.Status == expectedStatus
		},
		maxWait,
		DefaultPollInterval,
		"Expected workflow %s to reach status %s within %v",
		workflowExecID,
		expectedStatus,
		maxWait,
	)
}

// VerifyTaskState verifies that a task has the expected status in the database
func (v *DatabaseStateVerifier) VerifyTaskState(
	workflowExecID core.ID,
	taskID string,
	expectedStatus core.StatusType,
	timeoutDuration ...time.Duration,
) {
	v.t.Helper()
	timeout := DefaultTestTimeout
	if len(timeoutDuration) > 0 {
		timeout = timeoutDuration[0]
	}
	ctx, cancel := context.WithTimeout(v.t.Context(), timeout)
	defer cancel()
	ticker := time.NewTicker(DefaultPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			v.t.Fatalf(
				"Timeout waiting for task %s in workflow %s to reach status %s",
				taskID,
				workflowExecID,
				expectedStatus,
			)
		case <-ticker.C:
			tasks, err := v.taskRepo.ListTasksInWorkflow(v.t.Context(), workflowExecID)
			if err != nil {
				v.t.Logf("Warning: could not get tasks for workflow: %v", err)
				continue
			}

			for _, taskState := range tasks {
				if taskState.TaskID == taskID {
					v.t.Logf("Task %s current status: %s (expecting %s)", taskID, taskState.Status, expectedStatus)
					if taskState.Status == expectedStatus {
						return
					}
				}
			}
		}
	}
}

// VerifyTaskStateEventually verifies task state with retry logic
func (v *DatabaseStateVerifier) VerifyTaskStateEventually(
	workflowExecID core.ID,
	taskID string,
	expectedStatus core.StatusType,
	maxWait time.Duration,
) {
	v.t.Helper()
	require.Eventually(v.t, func() bool {
		tasks, err := v.taskRepo.ListTasksInWorkflow(v.t.Context(), workflowExecID)
		if err != nil {
			v.t.Logf("Could not get tasks for workflow: %v", err)
			return false
		}

		for _, taskState := range tasks {
			if taskState.TaskID == taskID {
				v.t.Logf("Task %s status: %s (expecting %s)", taskID, taskState.Status, expectedStatus)
				return taskState.Status == expectedStatus
			}
		}
		return false
	}, maxWait, DefaultPollInterval, "Expected task %s to reach status %s within %v", taskID, expectedStatus, maxWait)
}

// VerifyWorkflowExists checks that a workflow state exists in the database
func (v *DatabaseStateVerifier) VerifyWorkflowExists(workflowExecID core.ID) {
	v.t.Helper()
	state, err := v.workflowRepo.GetState(v.t.Context(), workflowExecID)
	require.NoError(v.t, err, "Workflow state should exist in database")
	assert.NotNil(v.t, state, "Workflow state should not be nil")
	assert.Equal(v.t, workflowExecID, state.WorkflowExecID, "Workflow execution ID should match")
}

// VerifyTaskExists checks that a task state exists in the database
func (v *DatabaseStateVerifier) VerifyTaskExists(workflowExecID core.ID, taskID string) {
	v.t.Helper()
	tasks, err := v.taskRepo.ListTasksInWorkflow(v.t.Context(), workflowExecID)
	require.NoError(v.t, err, "Should be able to list tasks in workflow")
	found := false
	for _, taskState := range tasks {
		if taskState.TaskID == taskID {
			found = true
			assert.Equal(v.t, workflowExecID, taskState.WorkflowExecID, "Task workflow execution ID should match")
			break
		}
	}
	assert.True(v.t, found, "Task %s should exist in workflow %s", taskID, workflowExecID)
}

// VerifyWorkflowCompletesWithStatus verifies that a workflow eventually reaches a completion status
func (v *DatabaseStateVerifier) VerifyWorkflowCompletesWithStatus(
	workflowExecID core.ID,
	expectedStatus core.StatusType,
	maxWait time.Duration,
) {
	v.t.Helper()
	v.VerifyWorkflowStateEventually(workflowExecID, expectedStatus, maxWait)
	state, err := v.workflowRepo.GetState(v.t.Context(), workflowExecID)
	require.NoError(v.t, err)
	assert.Equal(v.t, expectedStatus, state.Status)
	assert.NotNil(v.t, state.Input, "Workflow should have input")
	switch expectedStatus {
	case core.StatusSuccess:
		v.t.Logf("Workflow completed successfully with status: %s", state.Status)
	case core.StatusFailed:
		assert.NotNil(v.t, state.Error, "Failed workflow should have error details")
		v.t.Logf("Workflow failed with error: %v", state.Error)
	case core.StatusCanceled:
		v.t.Logf("Workflow was canceled with status: %s", state.Status)
	}
}

// StatusTransition represents a status transition with timing
type StatusTransition struct {
	Status    core.StatusType
	MaxWait   time.Duration
	Component string // "workflow" or task ID
}

// VerifyStatusTransitionSequence verifies that status transitions happen in the expected order
func (v *DatabaseStateVerifier) VerifyStatusTransitionSequence(workflowExecID core.ID, transitions []StatusTransition) {
	v.t.Helper()
	for i, transition := range transitions {
		v.t.Logf("Verifying transition %d: %s -> %s", i+1, transition.Component, transition.Status)

		if transition.Component == "workflow" {
			v.VerifyWorkflowStateEventually(workflowExecID, transition.Status, transition.MaxWait)
		} else {
			v.VerifyTaskStateEventually(workflowExecID, transition.Component, transition.Status, transition.MaxWait)
		}
	}
}

// GetWorkflowState returns the current workflow state from database
func (v *DatabaseStateVerifier) GetWorkflowState(workflowExecID core.ID) *workflow.State {
	v.t.Helper()
	state, err := v.workflowRepo.GetState(v.t.Context(), workflowExecID)
	require.NoError(v.t, err)
	return state
}

// GetTaskState returns the current task state from database
func (v *DatabaseStateVerifier) GetTaskState(workflowExecID core.ID, taskID string) *task.State {
	v.t.Helper()
	tasks, err := v.taskRepo.ListTasksInWorkflow(v.t.Context(), workflowExecID)
	require.NoError(v.t, err)
	for _, taskState := range tasks {
		if taskState.TaskID == taskID {
			return taskState
		}
	}
	v.t.Fatalf("Task %s not found in workflow %s", taskID, workflowExecID)
	return nil
}

// VerifyTaskCount verifies the expected number of tasks exist for a workflow
func (v *DatabaseStateVerifier) VerifyTaskCount(workflowExecID core.ID, expectedCount int) {
	v.t.Helper()
	tasks, err := v.taskRepo.ListTasksInWorkflow(v.t.Context(), workflowExecID)
	require.NoError(v.t, err)
	assert.Len(v.t, tasks, expectedCount, "Expected %d tasks in workflow %s", expectedCount, workflowExecID)
}

// VerifyNoErrors verifies that workflow and tasks have no error states
func (v *DatabaseStateVerifier) VerifyNoErrors(workflowExecID core.ID) {
	v.t.Helper()
	workflowState := v.GetWorkflowState(workflowExecID)
	assert.Nil(v.t, workflowState.Error, "Workflow should not have errors")
	tasks, err := v.taskRepo.ListTasksInWorkflow(v.t.Context(), workflowExecID)
	require.NoError(v.t, err)
	for _, taskState := range tasks {
		assert.Nil(v.t, taskState.Error, "Task %s should not have errors", taskState.TaskID)
	}
}

// VerifyTaskStatusCascade verifies that task states cascade correctly when workflow state changes
func (v *DatabaseStateVerifier) VerifyTaskStatusCascade(
	workflowExecID core.ID,
	expectedWorkflowStatus core.StatusType,
	expectedTaskStatuses map[string]core.StatusType,
	maxWait time.Duration,
) {
	v.t.Helper()
	require.Eventually(v.t, func() bool {
		workflowState, err := v.workflowRepo.GetState(v.t.Context(), workflowExecID)
		if err != nil {
			v.t.Logf("Failed to get workflow state: %v", err)
			return false
		}

		if workflowState.Status != expectedWorkflowStatus {
			v.t.Logf("Workflow status: %s (expecting %s)", workflowState.Status, expectedWorkflowStatus)
			return false
		}

		tasks, err := v.taskRepo.ListTasksInWorkflow(v.t.Context(), workflowExecID)
		if err != nil {
			v.t.Logf("Failed to get tasks: %v", err)
			return false
		}

		for taskID, expectedStatus := range expectedTaskStatuses {
			taskState, exists := tasks[taskID]
			if !exists {
				v.t.Logf("Task %s not found", taskID)
				return false
			}

			if taskState.Status != expectedStatus {
				v.t.Logf("Task %s status: %s (expecting %s)", taskID, taskState.Status, expectedStatus)
				return false
			}
		}

		return true
	}, maxWait, DefaultPollInterval,
		"Expected workflow %s to reach status %s with task states cascaded within %v",
		workflowExecID, expectedWorkflowStatus, maxWait)
}

// VerifyTaskStateConsistency verifies that all task states are consistent with workflow state
func (v *DatabaseStateVerifier) VerifyTaskStateConsistency(workflowExecID core.ID) {
	v.t.Helper()
	workflowState, err := v.workflowRepo.GetState(v.t.Context(), workflowExecID)
	require.NoError(v.t, err, "Failed to get workflow state")
	tasks, err := v.taskRepo.ListTasksInWorkflow(v.t.Context(), workflowExecID)
	require.NoError(v.t, err, "Failed to get tasks")
	for taskID, taskState := range tasks {
		v.t.Logf("Checking task %s consistency: workflow=%s, task=%s",
			taskID, workflowState.Status, taskState.Status)

		switch workflowState.Status {
		case core.StatusPaused:
			if taskState.Status != core.StatusSuccess && taskState.Status != core.StatusFailed {
				assert.Equal(v.t, core.StatusPaused, taskState.Status,
					"Task %s should be paused when workflow is paused", taskID)
			}
		case core.StatusCanceled:
			if taskState.Status != core.StatusSuccess {
				assert.Equal(v.t, core.StatusCanceled, taskState.Status,
					"Task %s should be canceled when workflow is canceled", taskID)
			}
		case core.StatusFailed:
			if taskState.Status != core.StatusSuccess {
				assert.True(v.t,
					taskState.Status == core.StatusFailed || taskState.Status == core.StatusCanceled,
					"Task %s should be failed/canceled when workflow failed", taskID)
			}
		}
	}
}
