package router

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/test/integration/worker/helpers"
)

// executeWorkflowAndGetState executes a real workflow and retrieves state from database
func executeWorkflowAndGetState(
	t *testing.T,
	fixture *helpers.TestFixture,
	dbHelper *helpers.DatabaseHelper,
) *workflow.State {
	agentConfig := helpers.CreateBasicAgentConfig()
	return helpers.ExecuteWorkflowAndGetState(t, fixture, dbHelper, "router-test-project", agentConfig)
}

// Verification functions for actual database state

func verifyRouterTaskSucceeded(t *testing.T, _ *helpers.TestFixture, result *workflow.State) {
	t.Helper()
	t.Log("Verifying router task succeeded from database state")

	// Find all router tasks and verify they all succeeded
	routerTasks := findAllRouterTasks(result)
	require.Greater(t, len(routerTasks), 0, "Should have at least one router task")

	for _, routerTask := range routerTasks {
		assert.Equal(t, core.StatusSuccess, routerTask.Status, "Router task %s should succeed", routerTask.TaskID)
	}
}

func verifyConditionalRouting(t *testing.T, fixture *helpers.TestFixture, result *workflow.State) {
	t.Helper()
	t.Log("Verifying conditional routing from database state")

	// Verify router task succeeded
	routerTasks := findAllRouterTasks(result)
	require.Greater(t, len(routerTasks), 0, "Should have at least one router task")

	// Verify expected basic tasks also executed successfully
	// For router tests, we mainly care that the routing logic worked and the right tasks ran
	expectedBasicTasks := 0
	actualBasicTasks := 0

	for _, taskState := range result.Tasks {
		if taskState.ExecutionType == task.ExecutionBasic && taskState.Status == core.StatusSuccess {
			actualBasicTasks++
		}
	}

	// Based on fixture expectations, we should have basic tasks that were routed to
	if len(fixture.Expected.TaskStates) > 1 { // More than just router task
		expectedBasicTasks = len(fixture.Expected.TaskStates) - len(routerTasks)
		assert.Equal(t, expectedBasicTasks, actualBasicTasks,
			"Should have exactly %d successful basic tasks (routed tasks)", expectedBasicTasks)
	}

	// Ensure we have some routed execution (more than just router tasks)
	assert.Greater(t, len(result.Tasks), len(routerTasks), "Should have tasks beyond just router tasks")
}

// Helper function to find all router tasks in workflow state
func findAllRouterTasks(result *workflow.State) []*task.State {
	var routerTasks []*task.State
	for _, taskState := range result.Tasks {
		if taskState.ExecutionType == task.ExecutionRouter {
			routerTasks = append(routerTasks, taskState)
		}
	}
	return routerTasks
}
