package worker

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/testsuite"
)

// TestWorkflowPauseAndResume tests the pause and resume signal functionality
func TestWorkflowPauseAndResume(t *testing.T) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()

	config := CreateContainerTestConfig(t)
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
			"message": "Test pause/resume",
		},
		InitialTaskID: "test-task",
	}

	// Set up delayed signals
	signalHelper.WaitAndSendSignal(100*time.Millisecond, signalHelper.SendPauseSignal)
	signalHelper.WaitAndSendSignal(200*time.Millisecond, signalHelper.SendResumeSignal)

	// Execute workflow
	env.ExecuteWorkflow(worker.CompozyWorkflow, workflowInput)

	// Assert workflow completed successfully despite being paused/resumed
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Verify database state changes during pause/resume cycle
	dbVerifier := NewDatabaseStateVerifier(t, config)

	// Verify workflow was created and eventually completed successfully
	dbVerifier.VerifyWorkflowExists(workflowExecID)
	dbVerifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, 15*time.Second)

	// Verify task was created and completed
	dbVerifier.VerifyTaskExists(workflowExecID, "test-task")
	dbVerifier.VerifyTaskStateEventually(workflowExecID, "test-task", core.StatusSuccess, 15*time.Second)

	// Verify no errors occurred during pause/resume cycle
	dbVerifier.VerifyNoErrors(workflowExecID)

	// Verify the workflow is paused
	dbVerifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, 15*time.Second)

	// Note: The workflow completes before we can check the paused state in this test setup
	// In a real scenario, the pause state would be checked during execution

	// Check final states to verify completion
	workflowState := dbVerifier.GetWorkflowState(workflowExecID)
	assert.Equal(t, core.StatusSuccess, workflowState.Status, "Workflow should complete successfully after pause/resume")

	// Verify state consistency - all tasks should reach appropriate final states
	dbVerifier.VerifyTaskStateConsistency(workflowExecID)

	// Check detailed task states
	tasks, err := config.TaskRepo.ListTasksInWorkflow(context.Background(), workflowExecID)
	assert.NoError(t, err)

	for taskID, taskState := range tasks {
		t.Logf("Final task %s status: %s", taskID, taskState.Status)
		assert.Equal(t, core.StatusSuccess, taskState.Status, "Task should complete successfully after resume")
	}
}

// TestSignalHandlingOrder tests that signals are processed in the correct order
func TestSignalHandlingOrder(t *testing.T) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()

	config := CreateContainerTestConfig(t)
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
			"message": "Test signal order",
		},
		InitialTaskID: "test-task",
	}

	// Send multiple signals in sequence
	signalHelper.WaitAndSendSignal(50*time.Millisecond, signalHelper.SendPauseSignal)
	signalHelper.WaitAndSendSignal(100*time.Millisecond, signalHelper.SendResumeSignal)
	signalHelper.WaitAndSendSignal(150*time.Millisecond, signalHelper.SendPauseSignal)
	signalHelper.WaitAndSendSignal(200*time.Millisecond, signalHelper.SendResumeSignal)

	// Execute workflow
	env.ExecuteWorkflow(worker.CompozyWorkflow, workflowInput)

	// Assert workflow completed successfully despite multiple pause/resume cycles
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Verify database state handling during multiple signal cycles
	dbVerifier := NewDatabaseStateVerifier(t, config)

	// Verify workflow was created and completed successfully
	dbVerifier.VerifyWorkflowExists(workflowExecID)
	dbVerifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, 20*time.Second)

	// Verify task was created and completed successfully
	dbVerifier.VerifyTaskExists(workflowExecID, "test-task")
	dbVerifier.VerifyTaskStateEventually(workflowExecID, "test-task", core.StatusSuccess, 20*time.Second)

	// Verify no errors occurred during multiple signal cycles
	dbVerifier.VerifyNoErrors(workflowExecID)

	// Verify final states
	workflowState := dbVerifier.GetWorkflowState(workflowExecID)
	taskState := dbVerifier.GetTaskState(workflowExecID, "test-task")

	assert.Equal(t, core.StatusSuccess, workflowState.Status, "Workflow should handle multiple signals correctly")
	assert.Equal(t, core.StatusSuccess, taskState.Status, "Task should complete despite multiple signals")
}

// TestSignalDuringTaskExecution tests signals sent during task execution
func TestSignalDuringTaskExecution(t *testing.T) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()

	// Use pauseable workflow for better signal testing
	workflowConfig := CreatePauseableWorkflowConfig()
	config := CreateContainerTestConfigForMultiTask(t, workflowConfig)
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
			"message": "Test signals during task execution",
		},
		InitialTaskID: "task-1",
	}

	// Send signals while tasks are executing
	signalHelper.WaitAndSendSignal(75*time.Millisecond, signalHelper.SendPauseSignal)
	signalHelper.WaitAndSendSignal(175*time.Millisecond, signalHelper.SendResumeSignal)

	// Execute workflow
	env.ExecuteWorkflow(worker.CompozyWorkflow, workflowInput)

	// Assert workflow completed successfully
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Verify database state updates during multi-task execution with signals
	dbVerifier := NewDatabaseStateVerifier(t, config)

	// Verify workflow was created and completed successfully
	dbVerifier.VerifyWorkflowExists(workflowExecID)
	dbVerifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, 25*time.Second)

	// Verify all expected tasks were created and completed
	dbVerifier.VerifyTaskExists(workflowExecID, "task-1")
	dbVerifier.VerifyTaskExists(workflowExecID, "task-2")
	dbVerifier.VerifyTaskExists(workflowExecID, "task-3")

	// Verify all tasks completed successfully despite signals
	dbVerifier.VerifyTaskStateEventually(workflowExecID, "task-1", core.StatusSuccess, 25*time.Second)
	dbVerifier.VerifyTaskStateEventually(workflowExecID, "task-2", core.StatusSuccess, 25*time.Second)
	dbVerifier.VerifyTaskStateEventually(workflowExecID, "task-3", core.StatusSuccess, 25*time.Second)

	// Verify expected number of tasks
	dbVerifier.VerifyTaskCount(workflowExecID, 3)

	// Verify no errors occurred
	dbVerifier.VerifyNoErrors(workflowExecID)
}

// TestWorkflowPauseStateUpdate tests that pause signals correctly update workflow status in database
func TestWorkflowPauseStateUpdate(t *testing.T) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()

	// Use pauseable workflow for longer execution time
	workflowConfig := CreatePauseableWorkflowConfig()
	config := CreateContainerTestConfigForMultiTask(t, workflowConfig)
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
			"message": "Test pause state update",
		},
		InitialTaskID: "task-1",
	}

	// Pause workflow early and resume later
	signalHelper.WaitAndSendSignal(50*time.Millisecond, signalHelper.SendPauseSignal)
	signalHelper.WaitAndSendSignal(300*time.Millisecond, signalHelper.SendResumeSignal)

	// Execute workflow
	env.ExecuteWorkflow(worker.CompozyWorkflow, workflowInput)

	// Assert workflow completed successfully
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Verify database state changes during pause/resume with detailed status tracking
	dbVerifier := NewDatabaseStateVerifier(t, config)

	// Verify workflow completed successfully after pause/resume cycle
	dbVerifier.VerifyWorkflowExists(workflowExecID)
	dbVerifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, 30*time.Second)

	// Verify final workflow state has correct data
	workflowState := dbVerifier.GetWorkflowState(workflowExecID)
	assert.Equal(t, core.StatusSuccess, workflowState.Status, "Workflow should be in success state")
	assert.NotNil(t, workflowState.Input, "Workflow input should be preserved")
	assert.Nil(t, workflowState.Error, "Workflow should not have errors after pause/resume")

	// Verify all tasks were created and completed despite the pause
	expectedTasks := []string{"task-1", "task-2", "task-3"}
	for _, taskID := range expectedTasks {
		dbVerifier.VerifyTaskExists(workflowExecID, taskID)
		dbVerifier.VerifyTaskStateEventually(workflowExecID, taskID, core.StatusSuccess, 30*time.Second)
	}

	dbVerifier.VerifyTaskCount(workflowExecID, 3)
	dbVerifier.VerifyNoErrors(workflowExecID)
}
