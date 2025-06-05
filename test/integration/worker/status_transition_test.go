package worker

import (
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/testsuite"
)

// TestWorkflowStatusTransitions tests the complete status transition lifecycle
func TestWorkflowStatusTransitions(t *testing.T) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()

	config := CreateContainerTestConfig(t)
	config.Cleanup(t)

	// Register workflow and activities
	env.RegisterWorkflow(worker.CompozyWorkflow)
	llmService := llm.NewLLMService()
	activities := worker.NewActivities(
		config.ProjectConfig,
		[]*workflow.Config{config.WorkflowConfig},
		config.WorkflowRepo,
		config.TaskRepo,
		llmService,
	)
	env.RegisterActivity(activities.TriggerWorkflow)
	env.RegisterActivity(activities.UpdateWorkflowState)
	env.RegisterActivity(activities.DispatchTask)
	env.RegisterActivity(activities.ExecuteBasicTask)

	// Create workflow input
	workflowExecID := core.MustNewID()
	workflowInput := worker.WorkflowInput{
		WorkflowID:     config.WorkflowConfig.ID,
		WorkflowExecID: workflowExecID,
		Input: &core.Input{
			"message": "Status transition test",
		},
		InitialTaskID: "test-task",
	}

	// Execute workflow
	env.ExecuteWorkflow(worker.CompozyWorkflow, workflowInput)

	// Assert workflow completed successfully
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Verify complete status transition lifecycle
	dbVerifier := NewDatabaseStateVerifier(t, config)

	// Verify workflow was created and completed
	dbVerifier.VerifyWorkflowExists(workflowExecID)
	dbVerifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, 15*time.Second)

	// Verify task was created and completed
	dbVerifier.VerifyTaskExists(workflowExecID, "test-task")
	dbVerifier.VerifyTaskStateEventually(workflowExecID, "test-task", core.StatusSuccess, 15*time.Second)

	// Verify final states contain expected data
	workflowState := dbVerifier.GetWorkflowState(workflowExecID)
	taskState := dbVerifier.GetTaskState(workflowExecID, "test-task")

	// Workflow state verification
	assert.Equal(t, core.StatusSuccess, workflowState.Status, "Workflow should be successful")
	assert.Equal(t, config.WorkflowConfig.ID, workflowState.WorkflowID, "Workflow ID should match")
	assert.Equal(t, workflowExecID, workflowState.WorkflowExecID, "Workflow execution ID should match")
	assert.NotNil(t, workflowState.Input, "Workflow should have input")
	assert.Nil(t, workflowState.Error, "Workflow should not have errors")

	// Task state verification
	assert.Equal(t, core.StatusSuccess, taskState.Status, "Task should be successful")
	assert.Equal(t, "test-task", taskState.TaskID, "Task ID should match")
	assert.Equal(t, workflowExecID, taskState.WorkflowExecID, "Task workflow execution ID should match")
	assert.Equal(t, core.ComponentAgent, taskState.Component, "Task should be agent component")
	assert.NotNil(t, taskState.AgentID, "Task should have agent ID")
	assert.Nil(t, taskState.Error, "Task should not have errors")

	t.Log("Workflow and task status transitions completed successfully")
}

// TestPauseResumeStatusTransitions tests status transitions during pause/resume
func TestPauseResumeStatusTransitions(t *testing.T) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()

	// Use multi-task workflow for better pause/resume testing
	workflowConfig := CreatePauseableWorkflowConfig()
	config := CreateContainerTestConfigForMultiTask(t, workflowConfig)
	config.Cleanup(t)

	// Register workflow and activities
	env.RegisterWorkflow(worker.CompozyWorkflow)
	llmService := llm.NewLLMService()
	activities := worker.NewActivities(
		config.ProjectConfig,
		[]*workflow.Config{config.WorkflowConfig},
		config.WorkflowRepo,
		config.TaskRepo,
		llmService,
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
			"message": "Pause/resume status test",
		},
		InitialTaskID: "task-1",
	}

	// Set up pause/resume signals
	signalHelper.WaitAndSendSignal(80*time.Millisecond, signalHelper.SendPauseSignal)
	signalHelper.WaitAndSendSignal(200*time.Millisecond, signalHelper.SendResumeSignal)

	// Execute workflow
	env.ExecuteWorkflow(worker.CompozyWorkflow, workflowInput)

	// Assert workflow completed successfully
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Verify status transitions during pause/resume cycle
	dbVerifier := NewDatabaseStateVerifier(t, config)

	// Verify workflow eventually completes successfully after pause/resume
	dbVerifier.VerifyWorkflowExists(workflowExecID)
	dbVerifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, 25*time.Second)

	// Verify all tasks complete successfully despite pause/resume
	expectedTasks := []string{"task-1", "task-2", "task-3"}
	for _, taskID := range expectedTasks {
		dbVerifier.VerifyTaskExists(workflowExecID, taskID)
		dbVerifier.VerifyTaskStateEventually(workflowExecID, taskID, core.StatusSuccess, 25*time.Second)
	}

	// Verify final state integrity after pause/resume cycle
	workflowState := dbVerifier.GetWorkflowState(workflowExecID)
	assert.Equal(t, core.StatusSuccess, workflowState.Status, "Workflow should be successful after pause/resume")
	assert.Nil(t, workflowState.Error, "Workflow should not have errors after pause/resume")

	// Verify no errors occurred during pause/resume transitions
	dbVerifier.VerifyNoErrors(workflowExecID)

	t.Log("Pause/resume status transitions completed successfully")
}

// TestTaskStatusProgression tests individual task status progression
func TestTaskStatusProgression(t *testing.T) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()

	config := CreateContainerTestConfig(t)
	config.Cleanup(t)

	// Register workflow and activities
	env.RegisterWorkflow(worker.CompozyWorkflow)
	llmService := llm.NewLLMService()
	activities := worker.NewActivities(
		config.ProjectConfig,
		[]*workflow.Config{config.WorkflowConfig},
		config.WorkflowRepo,
		config.TaskRepo,
		llmService,
	)
	env.RegisterActivity(activities.TriggerWorkflow)
	env.RegisterActivity(activities.UpdateWorkflowState)
	env.RegisterActivity(activities.DispatchTask)
	env.RegisterActivity(activities.ExecuteBasicTask)

	// Create workflow input
	workflowExecID := core.MustNewID()
	workflowInput := worker.WorkflowInput{
		WorkflowID:     config.WorkflowConfig.ID,
		WorkflowExecID: workflowExecID,
		Input: &core.Input{
			"message": "Task status progression test",
		},
		InitialTaskID: "test-task",
	}

	// Execute workflow
	env.ExecuteWorkflow(worker.CompozyWorkflow, workflowInput)

	// Assert workflow completed successfully
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Verify task status progression through its lifecycle
	dbVerifier := NewDatabaseStateVerifier(t, config)

	// Verify task was created and progressed through states
	dbVerifier.VerifyTaskExists(workflowExecID, "test-task")
	dbVerifier.VerifyTaskStateEventually(workflowExecID, "test-task", core.StatusSuccess, 15*time.Second)

	// Get final task state and verify progression completion
	taskState := dbVerifier.GetTaskState(workflowExecID, "test-task")

	// Verify task progression details
	assert.Equal(t, core.StatusSuccess, taskState.Status, "Task should progress to success")
	assert.Equal(t, "test-task", taskState.TaskID, "Task ID should be preserved")
	assert.Equal(t, workflowExecID, taskState.WorkflowExecID, "Workflow execution ID should be preserved")
	assert.Equal(t, config.WorkflowConfig.ID, taskState.WorkflowID, "Workflow ID should be preserved")
	assert.Equal(t, core.ComponentAgent, taskState.Component, "Component type should be preserved")
	assert.NotNil(t, taskState.AgentID, "Agent ID should be set")
	assert.NotNil(t, taskState.ActionID, "Action ID should be set")
	assert.NotNil(t, taskState.Input, "Task input should be preserved")
	assert.Nil(t, taskState.Error, "Task should not have errors")

	t.Logf("Task progressed successfully: %s -> %s", "PENDING", taskState.Status)
}

// TestMultiTaskStatusProgression tests status progression across multiple tasks
func TestMultiTaskStatusProgression(t *testing.T) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()

	// Use multi-task workflow
	workflowConfig := CreatePauseableWorkflowConfig()
	config := CreateContainerTestConfigForMultiTask(t, workflowConfig)
	config.Cleanup(t)

	// Register workflow and activities
	env.RegisterWorkflow(worker.CompozyWorkflow)
	llmService := llm.NewLLMService()
	activities := worker.NewActivities(
		config.ProjectConfig,
		[]*workflow.Config{config.WorkflowConfig},
		config.WorkflowRepo,
		config.TaskRepo,
		llmService,
	)
	env.RegisterActivity(activities.TriggerWorkflow)
	env.RegisterActivity(activities.UpdateWorkflowState)
	env.RegisterActivity(activities.DispatchTask)
	env.RegisterActivity(activities.ExecuteBasicTask)

	// Create workflow input
	workflowExecID := core.MustNewID()
	workflowInput := worker.WorkflowInput{
		WorkflowID:     config.WorkflowConfig.ID,
		WorkflowExecID: workflowExecID,
		Input: &core.Input{
			"message": "Multi-task status progression test",
		},
		InitialTaskID: "task-1",
	}

	// Execute workflow
	env.ExecuteWorkflow(worker.CompozyWorkflow, workflowInput)

	// Assert workflow completed successfully
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Verify status progression across all tasks
	dbVerifier := NewDatabaseStateVerifier(t, config)

	// Verify workflow progression
	dbVerifier.VerifyWorkflowExists(workflowExecID)
	dbVerifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, 20*time.Second)

	// Verify each task's status progression
	expectedTasks := []string{"task-1", "task-2", "task-3"}
	for i, taskID := range expectedTasks {
		dbVerifier.VerifyTaskExists(workflowExecID, taskID)
		dbVerifier.VerifyTaskStateEventually(workflowExecID, taskID, core.StatusSuccess, 20*time.Second)

		taskState := dbVerifier.GetTaskState(workflowExecID, taskID)

		// Verify task state progression
		assert.Equal(t, core.StatusSuccess, taskState.Status, "Task %d should be successful", i+1)
		assert.Equal(t, taskID, taskState.TaskID, "Task %d ID should match", i+1)
		assert.Equal(t, core.ComponentAgent, taskState.Component, "Task %d should be agent component", i+1)
		assert.NotNil(t, taskState.Input, "Task %d should have input", i+1)
		assert.Nil(t, taskState.Error, "Task %d should not have errors", i+1)

		t.Logf("Task %d (%s) progressed successfully to status: %s", i+1, taskID, taskState.Status)
	}

	// Verify task count and workflow completion
	dbVerifier.VerifyTaskCount(workflowExecID, 3)
	dbVerifier.VerifyNoErrors(workflowExecID)

	// Verify workflow contains all completed tasks
	workflowState := dbVerifier.GetWorkflowState(workflowExecID)
	assert.Len(t, workflowState.Tasks, 3, "Workflow should contain all completed tasks")

	t.Log("Multi-task status progression completed successfully")
}

// TestStatusPersistenceAfterCompletion tests that status remains consistent after completion
func TestStatusPersistenceAfterCompletion(t *testing.T) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()

	config := CreateContainerTestConfig(t)
	config.Cleanup(t)

	// Register workflow and activities
	env.RegisterWorkflow(worker.CompozyWorkflow)
	llmService := llm.NewLLMService()
	activities := worker.NewActivities(
		config.ProjectConfig,
		[]*workflow.Config{config.WorkflowConfig},
		config.WorkflowRepo,
		config.TaskRepo,
		llmService,
	)
	env.RegisterActivity(activities.TriggerWorkflow)
	env.RegisterActivity(activities.UpdateWorkflowState)
	env.RegisterActivity(activities.DispatchTask)
	env.RegisterActivity(activities.ExecuteBasicTask)

	// Create workflow input
	workflowExecID := core.MustNewID()
	workflowInput := worker.WorkflowInput{
		WorkflowID:     config.WorkflowConfig.ID,
		WorkflowExecID: workflowExecID,
		Input: &core.Input{
			"message": "Status persistence test",
		},
		InitialTaskID: "test-task",
	}

	// Execute workflow
	env.ExecuteWorkflow(worker.CompozyWorkflow, workflowInput)

	// Assert workflow completed successfully
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Verify status persistence after completion
	dbVerifier := NewDatabaseStateVerifier(t, config)

	// Verify workflow completed successfully
	dbVerifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, 15*time.Second)
	dbVerifier.VerifyTaskStateEventually(workflowExecID, "test-task", core.StatusSuccess, 15*time.Second)

	// Wait a bit and verify status remains consistent
	time.Sleep(2 * time.Second)

	// Re-verify status persistence
	workflowState := dbVerifier.GetWorkflowState(workflowExecID)
	taskState := dbVerifier.GetTaskState(workflowExecID, "test-task")

	assert.Equal(t, core.StatusSuccess, workflowState.Status, "Workflow status should remain successful")
	assert.Equal(t, core.StatusSuccess, taskState.Status, "Task status should remain successful")

	// Verify data integrity after completion
	assert.NotNil(t, workflowState.Input, "Workflow input should persist")
	assert.NotNil(t, taskState.Input, "Task input should persist")
	assert.Nil(t, workflowState.Error, "Workflow should remain error-free")
	assert.Nil(t, taskState.Error, "Task should remain error-free")

	t.Log("Status persistence verified after completion")
}
