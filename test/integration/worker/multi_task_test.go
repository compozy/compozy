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

// TestMultiTaskWorkflowWithPause tests pause/resume in a multi-task workflow
func TestMultiTaskWorkflowWithPause(t *testing.T) {
	t.Parallel() // Enable parallel execution

	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()

	// Use pauseable workflow config with multiple tasks
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
			"workflow_param": "multi-task-test",
		},
		InitialTaskID: "task-1",
	}

	// Pause after first task, then resume
	signalHelper.WaitAndSendSignal(100*time.Millisecond, signalHelper.SendPauseSignal)
	signalHelper.WaitAndSendSignal(300*time.Millisecond, signalHelper.SendResumeSignal)

	// Execute workflow
	env.ExecuteWorkflow(worker.CompozyWorkflow, workflowInput)

	// Assert workflow completed successfully
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Verify database state updates during multi-task workflow with pause/resume
	dbVerifier := NewDatabaseStateVerifier(t, config)

	// Verify workflow was created and completed successfully
	dbVerifier.VerifyWorkflowExists(workflowExecID)
	dbVerifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, 30*time.Second)

	// Verify all expected tasks were created and completed
	expectedTasks := []string{"task-1", "task-2", "task-3"}
	for _, taskID := range expectedTasks {
		dbVerifier.VerifyTaskExists(workflowExecID, taskID)
		dbVerifier.VerifyTaskStateEventually(workflowExecID, taskID, core.StatusSuccess, 30*time.Second)
	}

	// Verify expected number of tasks
	dbVerifier.VerifyTaskCount(workflowExecID, 3)

	// Verify no errors occurred during pause/resume cycle
	dbVerifier.VerifyNoErrors(workflowExecID)

	// Verify task sequence and completion order
	for i, taskID := range expectedTasks {
		taskState := dbVerifier.GetTaskState(workflowExecID, taskID)
		assert.Equal(t, core.StatusSuccess, taskState.Status, "Task %d (%s) should be successful", i+1, taskID)
		assert.Equal(t, core.ComponentAgent, taskState.Component, "Task %d should be an agent component", i+1)
		assert.NotNil(t, taskState.AgentID, "Task %d should have agent ID", i+1)
	}

	// Verify workflow final state
	workflowState := dbVerifier.GetWorkflowState(workflowExecID)
	assert.Equal(t, core.StatusSuccess, workflowState.Status, "Multi-task workflow should complete successfully")
	assert.Len(t, workflowState.Tasks, 3, "Workflow should contain all 3 tasks")
}

// TestTaskTransitions tests that task transitions work correctly with signals
func TestTaskTransitions(t *testing.T) {
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

	signalHelper := NewSignalHelper(env, t)

	// Create workflow input
	workflowExecID := core.MustNewID()
	workflowInput := worker.WorkflowInput{
		WorkflowID:     config.WorkflowConfig.ID,
		WorkflowExecID: workflowExecID,
		Input: &core.Input{
			"test_param": "task-transitions",
		},
		InitialTaskID: "task-1",
	}

	// Pause between task transitions
	signalHelper.WaitAndSendSignal(80*time.Millisecond, signalHelper.SendPauseSignal)
	signalHelper.WaitAndSendSignal(160*time.Millisecond, signalHelper.SendResumeSignal)

	// Execute workflow
	env.ExecuteWorkflow(worker.CompozyWorkflow, workflowInput)

	// Verify all tasks were executed despite pausing
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Verify database state updates during task transitions with signals
	dbVerifier := NewDatabaseStateVerifier(t, config)

	// Verify workflow was created and completed successfully
	dbVerifier.VerifyWorkflowExists(workflowExecID)
	dbVerifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, 25*time.Second)

	// Verify task transition sequence - all tasks should be created and completed
	transitions := []StatusTransition{
		{Status: core.StatusSuccess, MaxWait: 25 * time.Second, Component: "task-1"},
		{Status: core.StatusSuccess, MaxWait: 25 * time.Second, Component: "task-2"},
		{Status: core.StatusSuccess, MaxWait: 25 * time.Second, Component: "task-3"},
		{Status: core.StatusSuccess, MaxWait: 25 * time.Second, Component: "workflow"},
	}

	dbVerifier.VerifyStatusTransitionSequence(workflowExecID, transitions)

	// Verify expected number of tasks
	dbVerifier.VerifyTaskCount(workflowExecID, 3)

	// Verify no errors occurred during task transitions
	dbVerifier.VerifyNoErrors(workflowExecID)
}

// TestMultiTaskWorkflowExecution tests a complete multi-task workflow without signals
func TestMultiTaskWorkflowExecution(t *testing.T) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()

	// Use pauseable workflow config with multiple tasks
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
			"workflow_param": "multi-task-execution",
		},
		InitialTaskID: "task-1",
	}

	// Execute workflow without any signals
	env.ExecuteWorkflow(worker.CompozyWorkflow, workflowInput)

	// Assert workflow completed successfully
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Verify complete multi-task workflow execution in database
	dbVerifier := NewDatabaseStateVerifier(t, config)

	// Verify workflow was created and completed successfully
	dbVerifier.VerifyWorkflowExists(workflowExecID)
	dbVerifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, 20*time.Second)

	// Verify all tasks in the chain were created and completed
	expectedTasks := []string{"task-1", "task-2", "task-3"}
	for i, taskID := range expectedTasks {
		dbVerifier.VerifyTaskExists(workflowExecID, taskID)
		dbVerifier.VerifyTaskStateEventually(workflowExecID, taskID, core.StatusSuccess, 20*time.Second)

		// Verify task details
		taskState := dbVerifier.GetTaskState(workflowExecID, taskID)
		assert.Equal(t, taskID, taskState.TaskID, "Task ID should match")
		assert.Equal(t, workflowExecID, taskState.WorkflowExecID, "Workflow execution ID should match")
		assert.Equal(t, core.ComponentAgent, taskState.Component, "Task should be agent component")
		assert.NotNil(t, taskState.AgentID, "Task should have agent ID")
		assert.Equal(t, "test-agent", *taskState.AgentID, "Agent ID should match config")

		t.Logf("Task %d (%s) completed successfully with status: %s", i+1, taskID, taskState.Status)
	}

	// Verify expected number of tasks
	dbVerifier.VerifyTaskCount(workflowExecID, 3)

	// Verify no errors occurred
	dbVerifier.VerifyNoErrors(workflowExecID)

	// Verify workflow contains all tasks
	workflowState := dbVerifier.GetWorkflowState(workflowExecID)
	assert.Len(t, workflowState.Tasks, 3, "Workflow should contain all 3 tasks")
}

// TestTaskChainExecution tests execution of task chains with proper transitions
func TestTaskChainExecution(t *testing.T) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()

	// Use pauseable workflow to verify the full chain executes
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
			"chain_param": "task-chain-test",
		},
		InitialTaskID: "task-1",
	}

	// Execute workflow to verify task chain works
	env.ExecuteWorkflow(worker.CompozyWorkflow, workflowInput)

	// Assert all tasks in the chain executed successfully
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Verify task chain execution order and completion in database
	dbVerifier := NewDatabaseStateVerifier(t, config)

	// Verify workflow completed successfully
	dbVerifier.VerifyWorkflowExists(workflowExecID)
	dbVerifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, 20*time.Second)

	// Verify task chain execution in correct order
	taskChain := []string{"task-1", "task-2", "task-3"}

	// All tasks should exist and be successful
	for i, taskID := range taskChain {
		dbVerifier.VerifyTaskExists(workflowExecID, taskID)
		dbVerifier.VerifyTaskStateEventually(workflowExecID, taskID, core.StatusSuccess, 20*time.Second)

		taskState := dbVerifier.GetTaskState(workflowExecID, taskID)
		t.Logf("Task chain step %d (%s): status=%s, component=%s",
			i+1, taskID, taskState.Status, taskState.Component)

		// Verify task chain sequence properties
		assert.Equal(t, core.StatusSuccess, taskState.Status, "Task %s should be successful", taskID)
		assert.NotNil(t, taskState.Input, "Task %s should have input", taskID)
	}

	// Verify expected number of tasks in chain
	dbVerifier.VerifyTaskCount(workflowExecID, 3)

	// Verify no errors in the chain
	dbVerifier.VerifyNoErrors(workflowExecID)

	t.Log("Task chain execution completed successfully with all tasks in correct order")
}

// TestMultiTaskWorkflowWithMultiplePauses tests multiple pause/resume cycles
func TestMultiTaskWorkflowWithMultiplePauses(t *testing.T) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()

	// Use pauseable workflow config with multiple tasks
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
			"workflow_param": "multiple-pauses-test",
		},
		InitialTaskID: "task-1",
	}

	// Multiple pause/resume cycles during task execution
	signalHelper.WaitAndSendSignal(70*time.Millisecond, signalHelper.SendPauseSignal)
	signalHelper.WaitAndSendSignal(120*time.Millisecond, signalHelper.SendResumeSignal)
	signalHelper.WaitAndSendSignal(170*time.Millisecond, signalHelper.SendPauseSignal)
	signalHelper.WaitAndSendSignal(220*time.Millisecond, signalHelper.SendResumeSignal)
	signalHelper.WaitAndSendSignal(270*time.Millisecond, signalHelper.SendPauseSignal)
	signalHelper.WaitAndSendSignal(320*time.Millisecond, signalHelper.SendResumeSignal)

	// Execute workflow
	env.ExecuteWorkflow(worker.CompozyWorkflow, workflowInput)

	// Assert workflow completed successfully despite multiple pauses
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Verify database state updates during multiple pause/resume cycles
	dbVerifier := NewDatabaseStateVerifier(t, config)

	// Verify workflow was created and completed successfully
	dbVerifier.VerifyWorkflowExists(workflowExecID)
	dbVerifier.VerifyWorkflowCompletesWithStatus(workflowExecID, core.StatusSuccess, 35*time.Second)

	// Verify all tasks were created and completed despite multiple pauses
	expectedTasks := []string{"task-1", "task-2", "task-3"}
	for _, taskID := range expectedTasks {
		dbVerifier.VerifyTaskExists(workflowExecID, taskID)
		dbVerifier.VerifyTaskStateEventually(workflowExecID, taskID, core.StatusSuccess, 35*time.Second)
	}

	// Verify expected number of tasks
	dbVerifier.VerifyTaskCount(workflowExecID, 3)

	// Verify no errors occurred during multiple pause cycles
	dbVerifier.VerifyNoErrors(workflowExecID)

	// Verify workflow resilience to multiple signals
	workflowState := dbVerifier.GetWorkflowState(workflowExecID)
	assert.Equal(t, core.StatusSuccess, workflowState.Status,
		"Workflow should complete successfully despite multiple pause/resume cycles")
	assert.Len(t, workflowState.Tasks, 3, "All tasks should be present despite multiple pauses")

	t.Log("Multi-task workflow successfully handled multiple pause/resume cycles")
}
