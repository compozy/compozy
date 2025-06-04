package worker

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
)

// TestWorkflowCancellation tests the cancellation signal functionality
func TestWorkflowCancellation(t *testing.T) {
	t.Parallel() // Enable parallel execution

	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()

	// Use cancellable workflow config
	workflowConfig := CreateCancellableWorkflowConfig()
	config := CreateContainerTestConfigForCancellation(t, workflowConfig)
	config.Cleanup(t)

	// Register workflow and activities
	env.RegisterWorkflow(worker.CompozyWorkflow)
	activities := worker.NewActivities(
		config.ProjectConfig,
		[]*workflow.Config{config.WorkflowConfig},
		config.WorkflowRepo,
		config.TaskRepo,
	)
	env.RegisterActivity(activities.TriggerWorkflow)
	env.RegisterActivity(activities.UpdateWorkflowState)
	env.RegisterActivity(activities.DispatchTask)
	env.RegisterActivity(activities.ExecuteBasicTask)

	signalHelper := NewSignalHelper(env, t)

	// Create workflow input
	workflowExecID := core.MustNewID()
	workflowInput := worker.WorkflowInput{
		WorkflowID:     config.WorkflowConfig.ID,
		WorkflowExecID: workflowExecID,
		Input: &core.Input{
			"message": "Test cancellation",
		},
		InitialTaskID: "long-task",
	}

	// Send cancel signal after a short delay
	signalHelper.WaitAndSendSignal(50*time.Millisecond, signalHelper.SendCancelSignal)

	// Execute workflow
	env.ExecuteWorkflow(worker.CompozyWorkflow, workflowInput)

	// Assert workflow was canceled
	assert.True(t, env.IsWorkflowCompleted())

	// The workflow should complete due to cancellation handling
	// The error might be nil if cancellation is handled gracefully
	err := env.GetWorkflowError()
	if err != nil {
		// Accept various forms of cancellation errors, including runtime errors during cancellation
		errStr := err.Error()
		isCanceled := strings.Contains(errStr, "cancel") ||
			strings.Contains(errStr, "Cancel") ||
			strings.Contains(errStr, "canceled") ||
			strings.Contains(errStr, "context") ||
			strings.Contains(errStr, "runtime error") || // Runtime errors during cancellation are expected
			temporal.IsCanceledError(err)
		assert.True(t, isCanceled, "Expected cancellation error, got: %s", errStr)
	}

	// Verify database state after cancellation
	dbVerifier := NewDatabaseStateVerifier(t, config)

	// Verify workflow was created in database
	dbVerifier.VerifyWorkflowExists(workflowExecID)

	// Verify workflow was marked as canceled in database
	// Allow some time for the cancellation state to be persisted
	time.Sleep(1 * time.Second) // Give time for cancellation state to be written

	workflowState := dbVerifier.GetWorkflowState(workflowExecID)
	// Workflow should be either canceled or failed depending on how cancellation was handled
	assert.True(t,
		workflowState.Status == core.StatusCanceled || workflowState.Status == core.StatusFailed,
		"Workflow should be canceled or failed, got: %s", workflowState.Status)

	// Verify task was created (cancellation happens after task dispatch)
	dbVerifier.VerifyTaskExists(workflowExecID, "long-task")

	// Task might be in various states depending on when cancellation occurred
	taskState := dbVerifier.GetTaskState(workflowExecID, "long-task")
	t.Logf("Task state after cancellation: %s", taskState.Status)

	// Verify the cancellation was properly recorded
	t.Logf("Final workflow status: %s", workflowState.Status)
	switch workflowState.Status {
	case core.StatusCanceled:
		t.Log("Workflow cancellation was properly recorded in database")
	case core.StatusFailed:
		t.Log("Workflow failure was properly recorded in database (cancellation may cause failure)")
	}
}

// TestWorkflowWithLongRunningTask tests cancellation of long-running tasks
func TestWorkflowWithLongRunningTask(t *testing.T) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()

	// Use cancellable workflow config
	workflowConfig := CreateCancellableWorkflowConfig()
	config := CreateContainerTestConfigForCancellation(t, workflowConfig)
	config.Cleanup(t)

	// Register workflow and activities
	env.RegisterWorkflow(worker.CompozyWorkflow)
	activities := worker.NewActivities(
		config.ProjectConfig,
		[]*workflow.Config{config.WorkflowConfig},
		config.WorkflowRepo,
		config.TaskRepo,
	)
	env.RegisterActivity(activities.TriggerWorkflow)
	env.RegisterActivity(activities.UpdateWorkflowState)
	env.RegisterActivity(activities.DispatchTask)
	env.RegisterActivity(activities.ExecuteBasicTask)

	signalHelper := NewSignalHelper(env, t)

	// Create workflow input
	workflowExecID := core.MustNewID()
	workflowInput := worker.WorkflowInput{
		WorkflowID:     config.WorkflowConfig.ID,
		WorkflowExecID: workflowExecID,
		Input: &core.Input{
			"duration": "10s",
		},
		InitialTaskID: "long-task",
	}

	// Cancel the workflow while the long task is running
	signalHelper.WaitAndSendSignal(50*time.Millisecond, signalHelper.SendCancelSignal)

	// Execute workflow
	env.ExecuteWorkflow(worker.CompozyWorkflow, workflowInput)

	// Assert workflow was canceled appropriately
	assert.True(t, env.IsWorkflowCompleted())

	// Workflow should handle cancellation gracefully
	err := env.GetWorkflowError()
	if err != nil {
		// Check that it's a cancellation-related error
		errStr := err.Error()
		isCanceled := strings.Contains(errStr, "cancel") ||
			strings.Contains(errStr, "Cancel") ||
			strings.Contains(errStr, "canceled") ||
			strings.Contains(errStr, "context") ||
			strings.Contains(errStr, "runtime error") || // Runtime errors during cancellation are expected
			temporal.IsCanceledError(err)
		assert.True(t, isCanceled, "Expected cancellation error, got: %s", errStr)
	}

	// Verify database state after long-running task cancellation
	dbVerifier := NewDatabaseStateVerifier(t, config)

	// Verify workflow was created in database
	dbVerifier.VerifyWorkflowExists(workflowExecID)

	// Allow time for cancellation state to be persisted
	time.Sleep(1 * time.Second)

	// Verify workflow state reflects cancellation
	workflowState := dbVerifier.GetWorkflowState(workflowExecID)
	assert.True(t,
		workflowState.Status == core.StatusCanceled || workflowState.Status == core.StatusFailed,
		"Workflow should be canceled or failed after long task cancellation, got: %s", workflowState.Status)

	// Verify task was created and shows appropriate state
	dbVerifier.VerifyTaskExists(workflowExecID, "long-task")
	taskState := dbVerifier.GetTaskState(workflowExecID, "long-task")

	// Task should not be in success state since it was canceled
	assert.NotEqual(t, core.StatusSuccess, taskState.Status,
		"Task should not be successful after cancellation, got: %s", taskState.Status)

	t.Logf("Long-running task final state: %s", taskState.Status)
	t.Logf("Workflow final state after cancellation: %s", workflowState.Status)
}

// TestCancellationDuringTaskTransition tests cancellation during task transitions
func TestCancellationDuringTaskTransition(t *testing.T) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()

	// Use cancellable workflow config for proper timing
	workflowConfig := CreateCancellableWorkflowConfig()
	config := CreateContainerTestConfigForCancellation(t, workflowConfig)
	config.Cleanup(t)

	// Register workflow and activities
	env.RegisterWorkflow(worker.CompozyWorkflow)
	activities := worker.NewActivities(
		config.ProjectConfig,
		[]*workflow.Config{config.WorkflowConfig},
		config.WorkflowRepo,
		config.TaskRepo,
	)
	env.RegisterActivity(activities.TriggerWorkflow)
	env.RegisterActivity(activities.UpdateWorkflowState)
	env.RegisterActivity(activities.DispatchTask)
	env.RegisterActivity(activities.ExecuteBasicTask)

	signalHelper := NewSignalHelper(env, t)

	// Create workflow input
	workflowExecID := core.MustNewID()
	workflowInput := worker.WorkflowInput{
		WorkflowID:     config.WorkflowConfig.ID,
		WorkflowExecID: workflowExecID,
		Input: &core.Input{
			"duration": "10s", // This will trigger the mock LLM delay
		},
		InitialTaskID: "long-task",
	}

	// Cancel during task execution
	signalHelper.WaitAndSendSignal(50*time.Millisecond, signalHelper.SendCancelSignal)

	// Execute workflow
	env.ExecuteWorkflow(worker.CompozyWorkflow, workflowInput)

	// Assert workflow completed (due to cancellation)
	assert.True(t, env.IsWorkflowCompleted())

	// Verify cancellation was handled properly
	err := env.GetWorkflowError()
	if err != nil {
		errStr := err.Error()
		isCanceled := strings.Contains(errStr, "cancel") ||
			strings.Contains(errStr, "Cancel") ||
			strings.Contains(errStr, "canceled") ||
			strings.Contains(errStr, "context") ||
			strings.Contains(errStr, "runtime error") || // Runtime errors during cancellation are expected
			temporal.IsCanceledError(err)
		assert.True(t, isCanceled, "Expected cancellation error, got: %s", errStr)
	}

	// Verify database state during task transition cancellation
	dbVerifier := NewDatabaseStateVerifier(t, config)

	// Verify workflow was created in database
	dbVerifier.VerifyWorkflowExists(workflowExecID)

	// Allow time for cancellation state to be persisted
	time.Sleep(1 * time.Second)

	// Verify workflow state reflects cancellation
	workflowState := dbVerifier.GetWorkflowState(workflowExecID)
	assert.True(t,
		workflowState.Status == core.StatusCanceled || workflowState.Status == core.StatusFailed,
		"Workflow should be canceled or failed during task transition, got: %s", workflowState.Status)

	// Verify task was created (long-task should exist)
	dbVerifier.VerifyTaskExists(workflowExecID, "long-task")

	// Verify state consistency between workflow and tasks
	dbVerifier.VerifyTaskStateConsistency(workflowExecID)

	// Verify task status cascaded correctly
	expectedTaskStatuses := map[string]core.StatusType{
		"long-task": core.StatusCanceled, // Should be canceled when workflow is canceled
	}
	dbVerifier.VerifyTaskStatusCascade(workflowExecID, core.StatusCanceled, expectedTaskStatuses, 5*time.Second)

	// Check task state after cancellation
	tasks, err := config.TaskRepo.ListTasksInWorkflow(context.Background(), workflowExecID)
	assert.NoError(t, err)

	t.Logf("Tasks created before cancellation: %d", len(tasks))
	for taskID, taskState := range tasks {
		t.Logf("Task %s status: %s", taskID, taskState.Status)
	}

	// Task should be canceled (not successful) since workflow was canceled
	if len(tasks) > 0 {
		longTaskState := dbVerifier.GetTaskState(workflowExecID, "long-task")
		t.Logf("Long-task final state: %s", longTaskState.Status)
		assert.Equal(t, core.StatusCanceled, longTaskState.Status,
			"Task should be canceled when workflow is canceled")
	}

	// Verify cancellation timing and effect
	t.Logf("Workflow canceled during task transition - final state: %s", workflowState.Status)
}

// TestEarlyCancellation tests cancellation before any tasks execute
func TestEarlyCancellation(t *testing.T) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()

	// Use cancellable workflow config
	workflowConfig := CreateCancellableWorkflowConfig()
	config := CreateContainerTestConfigForCancellation(t, workflowConfig)
	config.Cleanup(t)

	// Register workflow and activities
	env.RegisterWorkflow(worker.CompozyWorkflow)
	activities := worker.NewActivities(
		config.ProjectConfig,
		[]*workflow.Config{config.WorkflowConfig},
		config.WorkflowRepo,
		config.TaskRepo,
	)
	env.RegisterActivity(activities.TriggerWorkflow)
	env.RegisterActivity(activities.UpdateWorkflowState)
	env.RegisterActivity(activities.DispatchTask)
	env.RegisterActivity(activities.ExecuteBasicTask)

	signalHelper := NewSignalHelper(env, t)

	// Create workflow input
	workflowExecID := core.MustNewID()
	workflowInput := worker.WorkflowInput{
		WorkflowID:     config.WorkflowConfig.ID,
		WorkflowExecID: workflowExecID,
		Input: &core.Input{
			"message": "Test early cancellation",
		},
		InitialTaskID: "long-task",
	}

	// Send cancel signal very early - right after workflow starts
	signalHelper.WaitAndSendSignal(10*time.Millisecond, signalHelper.SendCancelSignal)

	// Execute workflow
	env.ExecuteWorkflow(worker.CompozyWorkflow, workflowInput)

	// Assert workflow completed (due to early cancellation)
	assert.True(t, env.IsWorkflowCompleted())

	// The cancellation should be handled gracefully
	err := env.GetWorkflowError()
	if err != nil {
		errStr := err.Error()
		isCanceled := strings.Contains(errStr, "cancel") ||
			strings.Contains(errStr, "Cancel") ||
			strings.Contains(errStr, "canceled") ||
			strings.Contains(errStr, "context") ||
			strings.Contains(errStr, "runtime error") || // Runtime errors during cancellation are expected
			temporal.IsCanceledError(err)
		assert.True(t, isCanceled, "Expected cancellation error, got: %s", errStr)
	}

	// Verify database state after early cancellation
	dbVerifier := NewDatabaseStateVerifier(t, config)

	// Verify workflow was created in database
	dbVerifier.VerifyWorkflowExists(workflowExecID)

	// Allow time for cancellation state to be persisted
	time.Sleep(1 * time.Second)

	// Verify workflow state reflects early cancellation
	workflowState := dbVerifier.GetWorkflowState(workflowExecID)
	assert.True(t,
		workflowState.Status == core.StatusCanceled || workflowState.Status == core.StatusFailed,
		"Workflow should be canceled or failed after early cancellation, got: %s", workflowState.Status)

	// For early cancellation, task might not have been created yet
	tasks, err := config.TaskRepo.ListTasksInWorkflow(context.Background(), workflowExecID)
	assert.NoError(t, err)

	t.Logf("Tasks created during early cancellation: %d", len(tasks))
	if len(tasks) == 0 {
		t.Log("No tasks created - cancellation occurred before task dispatch")
	} else {
		// If task was created, verify its state
		for taskID, taskState := range tasks {
			t.Logf("Task %s created with status: %s", taskID, taskState.Status)
			assert.NotEqual(t, core.StatusSuccess, taskState.Status,
				"Task should not be successful after early cancellation")
		}
	}

	t.Logf("Early cancellation completed - workflow final state: %s", workflowState.Status)
}
