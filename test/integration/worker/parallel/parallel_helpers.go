package parallel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/test/integration/worker/helpers"
)

// executeWorkflowAndGetState executes a real Temporal workflow and retrieves final state from database
func executeWorkflowAndGetState(
	t *testing.T,
	fixture *helpers.TestFixture,
	dbHelper *helpers.DatabaseHelper,
) *workflow.State {
	// Use basic agent configuration which has process_message action required by simple_parallel fixture
	agentConfig := helpers.CreateBasicAgentConfig()

	// Execute real workflow using common helper
	return helpers.ExecuteWorkflowAndGetState(
		t,
		fixture,
		dbHelper,
		"test-parallel-project",
		agentConfig,
	)
}

// Verification functions for actual database state

func verifyParallelTaskExecution(t *testing.T, _ *helpers.TestFixture, result *workflow.State) {
	t.Helper()
	t.Log("Verifying parallel task execution from database state")

	parentTask := helpers.FindParentTask(result, task.ExecutionParallel)
	require.NotNil(t, parentTask, "Should have a parent parallel task")

	// Verify parent task has correct execution type and is properly configured
	assert.Equal(t, task.ExecutionParallel, parentTask.ExecutionType, "Parent task should be parallel type")
	assert.NotEmpty(t, parentTask.TaskID, "Parent task should have ID")
	assert.NotEmpty(t, parentTask.TaskExecID, "Parent task should have execution ID")

	// Find and verify child tasks
	childTasks := helpers.FindChildTasks(result, parentTask.TaskExecID)
	assert.Greater(t, len(childTasks), 0, "Should have child tasks")

	// Verify each child task has proper configuration
	for _, childTask := range childTasks {
		assert.NotEmpty(t, childTask.TaskID, "Child task should have ID")
		assert.NotEmpty(t, childTask.TaskExecID, "Child task should have execution ID")
		assert.Equal(t, task.ExecutionBasic, childTask.ExecutionType, "Child tasks should be basic execution type")
		assert.Equal(t, parentTask.TaskExecID, *childTask.ParentStateID, "Child task should reference parent")
	}
}

func verifyParallelChildTaskCreation(t *testing.T, fixture *helpers.TestFixture, result *workflow.State) {
	t.Helper()
	t.Log("Verifying parallel child task creation from database state")

	parentTask := helpers.FindParentTask(result, task.ExecutionParallel)
	require.NotNil(t, parentTask, "Should have a parent parallel task")

	// Expected child count from fixture (subtract parent task)
	expectedChildCount := len(fixture.Expected.TaskStates) - 1
	helpers.VerifyChildTaskCount(t, result, parentTask.TaskExecID, expectedChildCount, "parallel task")

	// Verify child task properties
	childTasks := helpers.FindChildTasks(result, parentTask.TaskExecID)
	for _, childTask := range childTasks {
		assert.NotEmpty(t, childTask.TaskID, "Child task should have ID")
		assert.NotEmpty(t, childTask.TaskExecID, "Child task should have execution ID")
		assert.Equal(t, task.ExecutionBasic, childTask.ExecutionType, "Child tasks should be basic execution type")
		assert.Equal(t, parentTask.TaskExecID, *childTask.ParentStateID, "Child task should reference parent")
	}
}

func verifyParallelOutputAggregation(t *testing.T, fixture *helpers.TestFixture, result *workflow.State) {
	t.Helper()
	t.Log("Verifying parallel output aggregation from database state")

	parentTask := helpers.FindParentTask(result, task.ExecutionParallel)
	require.NotNil(t, parentTask, "Should have a parent parallel task")

	// Verify parent task has expected output
	for _, expected := range fixture.Expected.TaskStates {
		if expected.Name == parentTask.TaskID && expected.Output != nil {
			helpers.VerifyTaskHasOutput(t, parentTask, "parallel parent task")
			// Verify specific output fields from fixture
			if expectedOutput, ok := expected.Output["total_tasks"]; ok {
				assert.Contains(t, *parentTask.Output, "total_tasks", "Parent output should contain total_tasks")
				if actualTotal, exists := (*parentTask.Output)["total_tasks"]; exists {
					assert.Equal(t, expectedOutput, actualTotal, "Total tasks should match expected")
				}
			}
			break
		}
	}
}

// Database and Redis test operations

func testRedisOperations(t *testing.T, _ *helpers.TestFixture, redisHelper *helpers.RedisHelper) {
	t.Helper()
	t.Log("Testing Redis operations for parallel workflow")

	// Redis operations would be tested here when implemented
	// For now, just verify the helper is functional
	assert.NotNil(t, redisHelper, "Redis helper should be available")
}
