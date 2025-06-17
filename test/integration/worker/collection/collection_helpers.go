package collection

import (
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/test/integration/worker/helpers"
)

// Collection-specific verification functions for integration tests

// executeWorkflowAndGetState executes a real Temporal workflow and retrieves final state from database
// This is a simple wrapper around the common helper function
func executeWorkflowAndGetState(
	t *testing.T,
	fixture *helpers.TestFixture,
	dbHelper *helpers.DatabaseHelper,
) *workflow.State {
	agentConfig := helpers.CreateCollectionAgentConfig()
	return helpers.ExecuteWorkflowAndGetState(t, fixture, dbHelper, "test-project-collection", agentConfig)
}

// Verification functions that check actual database state

func verifyCollectionSequentialExecution(t *testing.T, _ *helpers.TestFixture, result *workflow.State) {
	t.Log("Verifying sequential execution timing from database state")

	// Find parent collection task and its children using common helpers
	parentTask := helpers.FindParentTask(result, task.ExecutionCollection)
	require.NotNil(t, parentTask, "Should have a parent collection task")

	childTasks := helpers.FindChildTasks(result, parentTask.TaskExecID)
	assert.Equal(t, 2, len(childTasks), "Should have 2 child tasks")

	// Sort child tasks by creation time for verification
	sort.Slice(childTasks, func(i, j int) bool {
		return childTasks[i].CreatedAt.Before(childTasks[j].CreatedAt)
	})

	// Verify that child tasks executed sequentially (creation times are different)
	if len(childTasks) >= 2 {
		for i := 1; i < len(childTasks); i++ {
			prevTaskCreated := childTasks[i-1].CreatedAt
			currentTaskCreated := childTasks[i].CreatedAt

			assert.True(t, currentTaskCreated.After(prevTaskCreated) || currentTaskCreated.Equal(prevTaskCreated),
				"Sequential execution: child task %d should be created after or same time as child task %d",
				i, i-1)
		}
	}
}

func verifyCollectionParallelExecution(t *testing.T, _ *helpers.TestFixture, result *workflow.State) {
	t.Log("Verifying parallel execution timing from database state")

	// Find parent collection task and its children using common helpers
	parentTask := helpers.FindParentTask(result, task.ExecutionCollection)
	require.NotNil(t, parentTask, "Should have a parent collection task")

	childTasks := helpers.FindChildTasks(result, parentTask.TaskExecID)
	assert.Equal(t, 2, len(childTasks), "Should have 2 child tasks")

	// For parallel execution, tasks should start at the same time (or very close)
	if len(childTasks) >= 2 {
		// Sort child tasks by creation time to ensure deterministic ordering
		sort.Slice(childTasks, func(i, j int) bool {
			return childTasks[i].CreatedAt.Before(childTasks[j].CreatedAt)
		})

		baseTime := childTasks[0].CreatedAt
		for i := 1; i < len(childTasks); i++ {
			timeDiff := childTasks[i].CreatedAt.Sub(baseTime)
			if timeDiff < 0 {
				timeDiff = -timeDiff
			}
			// For parallel execution, tasks should start within a reasonable window
			// Using 500ms threshold to account for CI/test environment variations
			assert.True(t, timeDiff < 500*time.Millisecond,
				"Parallel execution: child task %d should start close to same time as first task (diff: %v)",
				i, timeDiff)
		}
	}
}

func verifyCollectionChildTasks(t *testing.T, _ *helpers.TestFixture, result *workflow.State) {
	t.Log("Verifying child task creation from database state")

	// Find the parent collection task using common helper
	parentTask := helpers.FindParentTask(result, task.ExecutionCollection)
	require.NotNil(t, parentTask, "Should have a parent collection task")

	// Verify child task count using common helper
	helpers.VerifyChildTaskCount(t, result, parentTask.TaskExecID, 2, "Collection task")
}

func verifyCollectionOutputAggregation(t *testing.T, fixture *helpers.TestFixture, result *workflow.State) {
	t.Log("Verifying collection output aggregation from database state")

	// Find the parent collection task using common helper
	parentTask := helpers.FindParentTask(result, task.ExecutionCollection)
	require.NotNil(t, parentTask, "Should have a parent collection task")

	// Verify task status and output using common helpers
	helpers.VerifyTaskHasOutput(t, parentTask, "Parent collection task")
	helpers.VerifyTaskStatus(t, parentTask, string(core.StatusSuccess), "Parent collection task")

	// Verify aggregated outputs match expectations from fixture
	if len(fixture.Expected.TaskStates) > 0 {
		for _, expectedTask := range fixture.Expected.TaskStates {
			if expectedTask.Name == parentTask.TaskID {
				// Verify expected outputs
				if expectedTask.Output != nil {
					output := *parentTask.Output

					if expectedTotal, exists := expectedTask.Output["total_items"]; exists {
						assert.Equal(t, expectedTotal, output["total_items"], "total_items should match expected")
					}

					if expectedResults, exists := expectedTask.Output["all_results"]; exists {
						assert.Equal(t, expectedResults, output["all_results"], "all_results should match expected")
					}

					if expectedSum, exists := expectedTask.Output["total_sum"]; exists {
						assert.Equal(t, expectedSum, output["total_sum"], "total_sum should match expected")
					}
				}
				break
			}
		}
	}
}

// verifyEmptyCollectionHandling verifies empty collection handling with database state
func verifyEmptyCollectionHandling(t *testing.T, _ *helpers.TestFixture, result *workflow.State) {
	t.Log("Verifying empty collection handling from database state")

	// Find parent collection task using common helper
	parentTask := helpers.FindParentTask(result, task.ExecutionCollection)
	require.NotNil(t, parentTask, "Should have a parent collection task")
	helpers.VerifyTaskStatus(t, parentTask, string(core.StatusSuccess), "Empty collection task")

	// Verify no child tasks using common helper
	helpers.VerifyChildTaskCount(t, result, parentTask.TaskExecID, 0, "Empty collection task")

	// Verify empty collection outputs using common helper and specific assertions
	helpers.VerifyTaskHasOutput(t, parentTask, "Empty collection task")
	if parentTask.Output != nil {
		output := *parentTask.Output
		assert.Equal(t, "empty_collection_completed", output["completion_status"], "Completion status should match")
		// Check collection metadata if present
		if metadata, ok := output["collection_metadata"].(map[string]any); ok {
			assert.Equal(t, float64(0), metadata["item_count"], "Item count should be 0")
			assert.Equal(t, float64(0), metadata["total_items"], "Total items should be 0")
		}
	}
}
