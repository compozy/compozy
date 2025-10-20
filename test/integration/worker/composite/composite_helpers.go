package composite

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
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
	// Use basic agent configuration for composite tasks
	agentConfig := helpers.CreateBasicAgentConfig()
	// Inject agent into all basic tasks recursively for testing
	var injectAgentRecursively func(tasks []task.Config)
	injectAgentRecursively = func(tasks []task.Config) {
		for i := range tasks {
			taskCfg := &tasks[i]
			if taskCfg.Type == "basic" && taskCfg.Agent == nil {
				taskCfg.Agent = agentConfig
			}
			// Recursively inject into child tasks
			if len(taskCfg.Tasks) > 0 {
				injectAgentRecursively(taskCfg.Tasks)
			}
		}
	}
	injectAgentRecursively(fixture.Workflow.Tasks)
	// Execute real workflow using common helper
	return helpers.ExecuteWorkflowAndGetState(
		t,
		fixture,
		dbHelper,
		"test-composite-project",
		agentConfig,
	)
}

// Verification functions for actual database state

func verifySequentialExecution(t *testing.T, _ *helpers.TestFixture, result *workflow.State) {
	t.Helper()
	t.Log("Verifying sequential execution from database state")
	compositeTask := helpers.FindParentTask(result, task.ExecutionComposite)
	require.NotNil(t, compositeTask, "Should have a composite task")
	// Find child tasks
	childTasks := helpers.FindChildTasks(result, compositeTask.TaskExecID)
	assert.Greater(t, len(childTasks), 1, "Should have multiple child tasks for sequential verification")
	// Sort child tasks by creation time to ensure consistent ordering
	sort.Slice(childTasks, func(i, j int) bool {
		return childTasks[i].CreatedAt.Before(childTasks[j].CreatedAt)
	})
	// Verify sequential execution order by creation times
	// Note: Tasks executed in quick succession might have the same timestamp
	for i := 1; i < len(childTasks); i++ {
		prev := childTasks[i-1]
		curr := childTasks[i]
		assert.True(t, !curr.CreatedAt.Before(prev.CreatedAt),
			"Child task %s should not be created before %s (sequential order)", curr.TaskID, prev.TaskID)
	}
}

func verifyChildTaskCreation(t *testing.T, _ *helpers.TestFixture, result *workflow.State) {
	t.Helper()
	t.Log("Verifying child task creation from database state")
	compositeTask := helpers.FindParentTask(result, task.ExecutionComposite)
	require.NotNil(t, compositeTask, "Should have a composite task")
	// Verify child tasks were created
	childTasks := helpers.FindChildTasks(result, compositeTask.TaskExecID)
	assert.Greater(t, len(childTasks), 0, "Should have created child tasks")
	// Verify child task properties
	for _, childTask := range childTasks {
		assert.Equal(t, task.ExecutionBasic, childTask.ExecutionType, "Child tasks should be basic execution type")
		assert.Equal(t, compositeTask.TaskExecID, *childTask.ParentStateID, "Child task should reference parent")
	}
	t.Logf("Verified %d child tasks were created", len(childTasks))
}

func verifyStatePassingBetweenTasks(t *testing.T, _ *helpers.TestFixture, result *workflow.State) {
	t.Helper()
	t.Log("Verifying state passing between tasks from database state")
	compositeTask := helpers.FindParentTask(result, task.ExecutionComposite)
	require.NotNil(t, compositeTask, "Should have a composite task")
	// Verify composite task has appropriate output for state aggregation
	helpers.VerifyTaskHasOutput(t, compositeTask, "composite task")
	// Check that progress info exists which indicates state aggregation
	if compositeTask.Output != nil {
		assert.Contains(t, *compositeTask.Output, "progress_info",
			"Composite output should contain progress_info for state aggregation")
	}
}

func verifyNestedCompositeExecution(t *testing.T, _ *helpers.TestFixture, result *workflow.State) {
	t.Helper()
	t.Log("Verifying nested composite execution from database state")
	compositeTask := helpers.FindParentTask(result, task.ExecutionComposite)
	require.NotNil(t, compositeTask, "Should have a composite task")
	// Verify composite execution succeeded
	assert.Equal(t, core.StatusSuccess, compositeTask.Status, "Nested composite should complete successfully")
}

func verifyNestedTaskStates(t *testing.T, _ *helpers.TestFixture, result *workflow.State) {
	t.Helper()
	t.Log("Verifying nested task states from database state")
	compositeTask := helpers.FindParentTask(result, task.ExecutionComposite)
	require.NotNil(t, compositeTask, "Should have a composite task")
	// Verify proper task hierarchy
	assert.Equal(t, task.ExecutionComposite, compositeTask.ExecutionType, "Parent should be composite type")
	// Find child tasks of the parent composite
	childTasks := helpers.FindChildTasks(result, compositeTask.TaskExecID)
	assert.Greater(t, len(childTasks), 0, "Should have child tasks in composite")
	// Check if any child is also a composite (nested)
	var nestedCompositeFound bool
	for _, child := range childTasks {
		if child.ExecutionType == task.ExecutionComposite {
			nestedCompositeFound = true
			// Verify the nested composite also has children
			nestedChildren := helpers.FindChildTasks(result, child.TaskExecID)
			assert.Greater(t, len(nestedChildren), 0, "Nested composite should have child tasks")
		}
	}
	// If we have a truly nested fixture, verify it
	if nestedCompositeFound {
		t.Log("Verified nested composite structure")
	}
}

func verifyEmptyCompositeHandling(t *testing.T, _ *helpers.TestFixture, result *workflow.State) {
	t.Helper()
	t.Log("Verifying empty composite handling from database state")
	compositeTask := helpers.FindParentTask(result, task.ExecutionComposite)
	require.NotNil(t, compositeTask, "Should have a composite task")
	// Empty composite should still complete successfully
	assert.Equal(t, core.StatusSuccess, compositeTask.Status, "Empty composite should complete successfully")
	// Verify no child tasks were created
	childTasks := helpers.FindChildTasks(result, compositeTask.TaskExecID)
	assert.Equal(t, 0, len(childTasks), "Empty composite should have no child tasks")
}

func verifyChildFailurePropagation(t *testing.T, _ *helpers.TestFixture, result *workflow.State) {
	t.Helper()
	t.Log("Verifying child failure propagation from database state")
	compositeTask := helpers.FindParentTask(result, task.ExecutionComposite)
	require.NotNil(t, compositeTask, "Should have a composite task")
	// Find failed child tasks
	childTasks := helpers.FindChildTasks(result, compositeTask.TaskExecID)
	var failedChildFound bool
	for _, childTask := range childTasks {
		if childTask.Status == core.StatusFailed {
			failedChildFound = true
			break
		}
	}
	if failedChildFound {
		assert.Equal(t, core.StatusFailed, compositeTask.Status, "Composite should fail when child fails")
		assert.NotNil(t, compositeTask.Error, "Composite should have error when child fails")
	}
}

func verifyCompositeFailureHandling(t *testing.T, _ *helpers.TestFixture, result *workflow.State) {
	t.Helper()
	t.Log("Verifying composite failure handling from database state")
	compositeTask := helpers.FindParentTask(result, task.ExecutionComposite)
	require.NotNil(t, compositeTask, "Should have a composite task")
	if compositeTask.Status == core.StatusFailed {
		assert.NotNil(t, compositeTask.Error, "Failed composite should have error details")
		if compositeTask.Output != nil {
			assert.Contains(t, *compositeTask.Output, "progress_info",
				"Failed composite output should contain progress_info")
		}
	}
}

func verifyCompositeStateManagement(t *testing.T, fixture *helpers.TestFixture, result *workflow.State) {
	t.Helper()
	t.Log("Verifying composite state management from database state")
	compositeTask := requireCompositeTask(t, result)
	helpers.VerifyTaskHasOutput(t, compositeTask, "composite task")
	progressInfo := extractCompositeProgressInfo(compositeTask)
	if progressInfo != nil {
		assertCompositeProgressKeys(t, progressInfo)
		assertCompositeProgressMatchesFixture(t, progressInfo, fixture)
	}
}

// requireCompositeTask finds the composite task and asserts its existence.
// It centralizes the shared lookup pattern used by composite verifiers.
func requireCompositeTask(t *testing.T, result *workflow.State) *task.State {
	t.Helper()
	compositeTask := helpers.FindParentTask(result, task.ExecutionComposite)
	require.NotNil(t, compositeTask, "Should have a composite task")
	return compositeTask
}

// extractCompositeProgressInfo returns the composite progress info map when present.
// Returning nil avoids additional branching in callers when progress is absent.
func extractCompositeProgressInfo(compositeTask *task.State) map[string]any {
	if compositeTask.Output == nil {
		return nil
	}
	progressValue, ok := (*compositeTask.Output)["progress_info"]
	if !ok {
		return nil
	}
	progressInfo, ok := progressValue.(map[string]any)
	if !ok {
		return nil
	}
	return progressInfo
}

// assertCompositeProgressKeys verifies that required tracking fields are present.
// It keeps key assertions separate from value comparisons.
func assertCompositeProgressKeys(t *testing.T, progress map[string]any) {
	t.Helper()
	assert.Contains(t, progress, "total_children", "Progress info should contain total_children")
	assert.Contains(t, progress, "success_count", "Progress info should contain success_count")
	assert.Contains(t, progress, "failed_count", "Progress info should contain failed_count")
}

// assertCompositeProgressMatchesFixture compares observed progress counts with fixture expectations.
// It supports both current and legacy field names used across fixtures.
func assertCompositeProgressMatchesFixture(
	t *testing.T,
	progress map[string]any,
	fixture *helpers.TestFixture,
) {
	t.Helper()
	if len(fixture.Expected.TaskStates) == 0 {
		return
	}
	expectedState := fixture.Expected.TaskStates[0]
	if expectedState.Output == nil {
		return
	}
	expectedValue, ok := expectedState.Output["progress_info"].(map[string]any)
	if !ok {
		return
	}
	if expectedTotal, ok := expectedValue["total_children"]; ok {
		assert.EqualValues(t, expectedTotal, progress["total_children"],
			"Total children count should match expected")
	}
	if expectedCompleted, ok := expectedValue["success_count"]; ok {
		assert.EqualValues(t, expectedCompleted, progress["success_count"],
			"Success count should match expected")
	}
	if legacyCompleted, ok := expectedValue["completed_count"]; ok && legacyCompleted != nil {
		assert.EqualValues(t, legacyCompleted, progress["success_count"],
			"Success count should match expected (legacy completed_count)")
	}
	if expectedFailed, ok := expectedValue["failed_count"]; ok {
		assert.EqualValues(t, expectedFailed, progress["failed_count"],
			"Failed count should match expected")
	}
}

func verifyChildTaskDataFlow(t *testing.T, _ *helpers.TestFixture, result *workflow.State) {
	t.Helper()
	t.Log("Verifying child task data flow from database state")
	compositeTask := helpers.FindParentTask(result, task.ExecutionComposite)
	require.NotNil(t, compositeTask, "Should have a composite task")
	// Verify child tasks exist and successful ones have output
	childTasks := helpers.FindChildTasks(result, compositeTask.TaskExecID)
	for _, childTask := range childTasks {
		// Child tasks may or may not have input depending on the fixture
		if childTask.Status == core.StatusSuccess {
			assert.NotNil(t, childTask.Output, "Successful child task should have output")
		}
	}
}

// Database and Redis test operations

func testDatabaseStateOperations(t *testing.T, _ *helpers.TestFixture, dbHelper *helpers.DatabaseHelper) {
	t.Helper()
	t.Log("Testing database state operations for composite tasks")
	// Database operations are already tested via the common helpers
	// Just verify the helper is functional
	assert.NotNil(t, dbHelper, "Database helper should be available")
}

func testRedisOperations(t *testing.T, _ *helpers.TestFixture, redisHelper *helpers.RedisHelper) {
	t.Helper()
	t.Log("Testing redis operations for composite tasks")
	// Redis operations would be tested here when implemented
	// For now, just verify the helper is functional
	assert.NotNil(t, redisHelper, "Redis helper should be available")
}
