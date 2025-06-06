package worker

import (
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/worker"
	utils "github.com/compozy/compozy/test/integration/helper"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/testsuite"
)

// TestBasicWorkflowExecution tests a simple workflow execution without signals
func TestBasicWorkflowExecution(t *testing.T) {
	t.Parallel() // Enable parallel execution

	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()

	config := utils.CreateContainerTestConfig(t)
	config.Cleanup(t)

	// Register workflow and activities
	utils.SetupWorkflowEnvironment(env, config)

	// Create workflow input
	workflowExecID := core.MustNewID()
	workflowInput := worker.WorkflowInput{
		WorkflowID:     config.WorkflowConfig.ID,
		WorkflowExecID: workflowExecID,
		Input: &core.Input{
			"message": "Hello, World!",
		},
		InitialTaskID: "test-task",
	}

	// Execute workflow
	env.ExecuteWorkflow(worker.CompozyWorkflow, workflowInput)

	// Assert workflow completed successfully
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Verify database states
	dbVerifier := utils.NewDatabaseStateVerifier(t, config)

	// Verify workflow was created and completed successfully
	dbVerifier.VerifyWorkflowExists(workflowExecID)
	dbVerifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, 20*time.Second)

	// Verify task was created and completed successfully
	dbVerifier.VerifyTaskExists(workflowExecID, "test-task")
	dbVerifier.VerifyTaskStateEventually(workflowExecID, "test-task", core.StatusSuccess, 20*time.Second)

	// Verify no errors occurred
	dbVerifier.VerifyNoErrors(workflowExecID)

	// Verify expected number of tasks
	dbVerifier.VerifyTaskCount(workflowExecID, 1)
}

// TestWorkflowStatusUpdates tests that workflow status is properly updated during execution
func TestWorkflowStatusUpdates(t *testing.T) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()

	config := utils.CreateContainerTestConfig(t)
	config.Cleanup(t)

	// Register workflow and activities
	utils.SetupWorkflowEnvironment(env, config)

	// Create workflow input
	workflowExecID := core.MustNewID()
	workflowInput := worker.WorkflowInput{
		WorkflowID:     config.WorkflowConfig.ID,
		WorkflowExecID: workflowExecID,
		Input: &core.Input{
			"message": "Test status updates",
		},
		InitialTaskID: "test-task",
	}

	// Execute workflow
	env.ExecuteWorkflow(worker.CompozyWorkflow, workflowInput)

	// Verify workflow completed
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Verify database state updates throughout the workflow lifecycle
	dbVerifier := utils.NewDatabaseStateVerifier(t, config)

	// Verify workflow was persisted and completed
	dbVerifier.VerifyWorkflowExists(workflowExecID)
	dbVerifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, 10*time.Second)

	// Verify task progression through states
	dbVerifier.VerifyTaskExists(workflowExecID, "test-task")

	// Get final task state and verify it completed successfully
	taskState := dbVerifier.GetTaskState(workflowExecID, "test-task")
	assert.Equal(t, core.StatusSuccess, taskState.Status, "Task should have completed successfully")
	assert.NotNil(t, taskState.Input, "Task should have input data")
	assert.Equal(t, "test-task", taskState.TaskID, "Task ID should match")
	assert.Equal(t, workflowExecID, taskState.WorkflowExecID, "Workflow execution ID should match")

	// Verify workflow final state
	workflowState := dbVerifier.GetWorkflowState(workflowExecID)
	assert.Equal(t, core.StatusSuccess, workflowState.Status, "Workflow should have completed successfully")
	assert.NotNil(t, workflowState.Input, "Workflow should have input data")
	assert.Nil(t, workflowState.Error, "Workflow should not have errors")

	// Verify the workflow contains the expected task
	assert.NotNil(t, workflowState.Tasks, "Workflow should have tasks")
	assert.Len(t, workflowState.Tasks, 1, "Workflow should have exactly one task")
}

// TestWorkflowDatabasePersistence tests that workflow data is correctly persisted
func TestWorkflowDatabasePersistence(t *testing.T) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()

	config := utils.CreateContainerTestConfig(t)
	config.Cleanup(t)

	// Register workflow and activities
	utils.SetupWorkflowEnvironment(env, config)

	// Create workflow input with specific test data
	workflowExecID := core.MustNewID()
	testMessage := "Database persistence test message"
	workflowInput := worker.WorkflowInput{
		WorkflowID:     config.WorkflowConfig.ID,
		WorkflowExecID: workflowExecID,
		Input: &core.Input{
			"message": testMessage,
		},
		InitialTaskID: "test-task",
	}

	// Execute workflow
	env.ExecuteWorkflow(worker.CompozyWorkflow, workflowInput)

	// Verify workflow completed
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Verify detailed database persistence
	dbVerifier := utils.NewDatabaseStateVerifier(t, config)

	// Verify workflow data persistence
	workflowState := dbVerifier.GetWorkflowState(workflowExecID)
	assert.Equal(t, config.WorkflowConfig.ID, workflowState.WorkflowID, "Workflow ID should match config")
	assert.Equal(t, workflowExecID, workflowState.WorkflowExecID, "Workflow execution ID should match")
	assert.Equal(t, core.StatusSuccess, workflowState.Status, "Workflow should be successful")

	// Verify input data was correctly persisted
	assert.NotNil(t, workflowState.Input, "Workflow input should be persisted")
	inputMap := map[string]any(*workflowState.Input)
	assert.Equal(t, testMessage, inputMap["message"], "Input message should match")

	// Verify task data persistence
	taskState := dbVerifier.GetTaskState(workflowExecID, "test-task")
	assert.Equal(t, "test-task", taskState.TaskID, "Task ID should be correct")
	assert.Equal(t, core.ComponentAgent, taskState.Component, "Task component should be agent")
	assert.Equal(t, core.StatusSuccess, taskState.Status, "Task should be successful")
	assert.NotNil(t, taskState.AgentID, "Task should have agent ID")
	assert.Equal(t, "test-agent", *taskState.AgentID, "Agent ID should match config")
}
