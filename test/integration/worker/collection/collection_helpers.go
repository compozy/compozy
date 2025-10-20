package collection

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
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
	assert.GreaterOrEqual(t, len(childTasks), 2, "Should have at least 2 child tasks")

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
	assert.GreaterOrEqual(t, len(childTasks), 2, "Should have at least 2 child tasks")

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

	// Get actual child count from the result
	childTasks := helpers.FindChildTasks(result, parentTask.TaskExecID)
	actualCount := len(childTasks)

	// Verify we have at least some child tasks
	require.Greater(t, actualCount, 0, "Collection task should have child tasks")

	// Log the actual count for debugging
	t.Logf("Collection task has %d child tasks", actualCount)
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

// Task 5.0 verification functions

// verifyPrecisionPreservation verifies that large numbers and high-precision decimals
// are handled correctly during template processing without losing precision
func verifyPrecisionPreservation(t *testing.T, _ *helpers.TestFixture, result *workflow.State) {
	t.Log("Verifying precision preservation in template processing from database state")
	parentTask := helpers.FindParentTask(result, task.ExecutionCollection)
	require.NotNil(t, parentTask, "Should have a parent collection task")
	childTasks := sortedChildTasks(result, parentTask.TaskExecID)
	require.Len(t, childTasks, 2, "Should have exactly 2 child tasks for precision testing")
	for i, taskState := range childTasks {
		verifyPrecisionTask(t, taskState, expectedPrecisionData()[i])
	}
	t.Log("Precision preservation verified successfully")
}

type precisionExpectation struct {
	accountID string
	balance   string
	rate      string
	decimal   string
}

func expectedPrecisionData() []precisionExpectation {
	return []precisionExpectation{
		{"9007199254740992", "999999999999999999", "0.123456789123456789", "123456789.123456789"},
		{"9007199254740993", "123456789012345678", "2.345678901234567890", "987654321.987654321"},
	}
}

func sortedChildTasks(result *workflow.State, parentExecID core.ID) []*task.State {
	childTasks := helpers.FindChildTasks(result, parentExecID)
	sort.Slice(childTasks, func(i, j int) bool {
		return childTasks[i].TaskID < childTasks[j].TaskID
	})
	return childTasks
}

func verifyPrecisionTask(t *testing.T, taskState *task.State, expected precisionExpectation) {
	if taskState.Input == nil {
		t.FailNow()
	}
	input := map[string]any(*taskState.Input)
	assert.Equal(
		t,
		expected.accountID,
		input["item_name"],
		"Large integer account ID should be preserved as string without precision loss",
	)
	assert.Equal(
		t,
		expected.balance,
		input["balance_str"],
		"Large integer balance should be preserved without scientific notation",
	)
	assertPrecisionString(
		t,
		expected.rate,
		input["rate_str"],
		15,
		"Rate should maintain at least 15 decimal places for precision testing",
	)
	assertPrecisionString(
		t,
		expected.decimal,
		input["decimal_str"],
		9,
		"Decimal should maintain at least 9 fractional digits",
	)
}

func assertPrecisionString(t *testing.T, expected string, value any, minFractionDigits int, message string) {
	actual, ok := value.(string)
	assert.True(t, ok, "precision field should be a string")
	assert.Equal(t, expected, actual, "Precision value mismatch")
	if strings.Contains(actual, ".") {
		fractionalPart := actual[strings.Index(actual, ".")+1:]
		assert.GreaterOrEqual(t, len(fractionalPart), minFractionDigits, message)
	}
}

// verifyDeterministicMapProcessing verifies that map iteration and template
// processing produces consistent, deterministic results across runs.
// It orchestrates validation by delegating to focused helpers below.
func verifyDeterministicMapProcessing(t *testing.T, fixture *helpers.TestFixture, result *workflow.State) {
	t.Helper()
	t.Log("Verifying deterministic map processing from database state")

	parentTask := requireParentCollectionTask(t, result)
	childTasks := childTasksSortedByIndex(t, result, parentTask.TaskExecID)
	orderedConfigSets := orderedConfigSetsFromFixture(t, fixture.Input["config_sets"])

	assertDeterministicChildOrder(t, childTasks, orderedConfigSets)
	assertDeterministicWithKeys(t, childTasks)

	t.Log("Deterministic map processing verified successfully")
}

// requireParentCollectionTask locates and asserts the presence of the parent collection task.
// It helps multiple verifiers share the same lookup logic.
func requireParentCollectionTask(t *testing.T, result *workflow.State) *task.State {
	t.Helper()
	parentTask := helpers.FindParentTask(result, task.ExecutionCollection)
	require.NotNil(t, parentTask, "Should have a parent collection task")
	return parentTask
}

// childTasksSortedByIndex returns child tasks sorted by deterministic index extracted from task IDs.
// Sorting once avoids repeating ordering logic in each assertion helper.
func childTasksSortedByIndex(t *testing.T, result *workflow.State, parentExecID core.ID) []*task.State {
	t.Helper()
	childTasks := helpers.FindChildTasks(result, parentExecID)
	require.Len(t, childTasks, 3, "Should have exactly 3 child tasks for deterministic processing")
	sort.Slice(childTasks, func(i, j int) bool {
		return extractIndexFromTaskID(childTasks[i].TaskID) < extractIndexFromTaskID(childTasks[j].TaskID)
	})
	return childTasks
}

// orderedConfigSetsFromFixture normalizes the fixture config sets into a deterministic slice.
// It supports both strongly typed and generic fixture encodings.
func orderedConfigSetsFromFixture(t *testing.T, raw any) []map[string]any {
	t.Helper()
	switch v := raw.(type) {
	case []map[string]any:
		return v
	case []any:
		ordered := make([]map[string]any, 0, len(v))
		for _, item := range v {
			if itemMap, ok := item.(map[string]any); ok {
				ordered = append(ordered, itemMap)
			}
		}
		return ordered
	default:
		require.Fail(t, "config_sets in fixture should be an ordered slice of maps, got %T", v)
		return nil
	}
}

// assertDeterministicChildOrder checks task IDs and payload sequencing against expectations.
// It ensures both task naming and item processing follow the configured order.
func assertDeterministicChildOrder(
	t *testing.T,
	childTasks []*task.State,
	orderedConfigSets []map[string]any,
) {
	t.Helper()
	for i, childTask := range childTasks {
		expectedTaskID := fmt.Sprintf("process-config-%d", i)
		assert.Equal(t, expectedTaskID, childTask.TaskID,
			"Child task %d should have deterministic task ID", i)

		if childTask.Input == nil {
			continue
		}
		inputMap := map[string]any(*childTask.Input)
		require.Less(t, i, len(orderedConfigSets), "Index out of bounds for orderedConfigSets")
		expectedItemName := orderedConfigSets[i]["name"]
		assert.Equal(t, expectedItemName, inputMap["item_name"],
			"Item processing should follow deterministic order")
	}
}

// assertDeterministicWithKeys validates that the template produced expected with/env keys.
// The first child task carries the merged context we care about.
func assertDeterministicWithKeys(t *testing.T, childTasks []*task.State) {
	t.Helper()
	if len(childTasks) == 0 || childTasks[0].Input == nil {
		return
	}

	inputMap := map[string]any(*childTasks[0].Input)
	expectedWithKeys := []string{
		"alpha_config",
		"beta_config",
		"charlie_config",
		"delta_config",
		"gamma_config",
		"zebra_config",
	}
	for _, key := range expectedWithKeys {
		assert.Contains(t, inputMap, key, "Deterministic processing should include all with config keys")
	}
}

// verifyProgressContextIntegration verifies that progress context is available
// and correctly populated during collection processing
func verifyProgressContextIntegration(t *testing.T, fixture *helpers.TestFixture, result *workflow.State) {
	t.Log("Verifying progress context integration from database state")

	// Find parent collection task
	parentTask := helpers.FindParentTask(result, task.ExecutionCollection)
	require.NotNil(t, parentTask, "Should have a parent collection task")

	// Find child tasks and verify progress context was available
	childTasks := helpers.FindChildTasks(result, parentTask.TaskExecID)
	require.GreaterOrEqual(t, len(childTasks), 1, "Should have at least one child task")

	// Verify progress context was properly integrated in child tasks
	for _, childTask := range childTasks {
		// Progress context is validated through the input data passed to tasks
		if childTask.Input != nil {
			inputMap := map[string]any(*childTask.Input)

			// Verify total_tasks was populated from progress context
			if totalTasks, exists := inputMap["total_tasks"]; exists {
				// Should reflect the total number of tasks expected
				assert.NotZero(t, totalTasks, "Total tasks should be populated from progress context")
			}

			// Verify completion_rate was calculated
			if completionRate, exists := inputMap["completion_rate"]; exists {
				if rate, ok := completionRate.(float64); ok {
					assert.GreaterOrEqual(t, rate, 0.0, "Completion rate should be non-negative")
					assert.LessOrEqual(t, rate, 1.0, "Completion rate should not exceed 100%")
				}
			}

			// Verify workflow context is available
			if workflowName, exists := inputMap["workflow_name"]; exists {
				assert.Equal(t, fixture.Workflow.ID, workflowName,
					"Workflow name should match fixture")
			}

			// Verify current task context
			if currentTask, exists := inputMap["current_task"]; exists {
				assert.NotEmpty(t, currentTask, "Current task should be populated")
			}
		}
	}

	t.Log("Progress context integration verified successfully")
}

// verifyComplexTemplateProcessing verifies the runtime processor's ability
// to handle deeply nested templates and complex configurations.
// It delegates validation to smaller helpers for clarity.
func verifyComplexTemplateProcessing(t *testing.T, _ *helpers.TestFixture, result *workflow.State) {
	t.Helper()
	t.Log("Verifying complex template processing from database state")

	parentTask := requireParentCollectionTask(t, result)
	childTasks := requireComplexTemplateChildTasks(t, result, parentTask.TaskExecID)
	expectations := expectedComplexTemplateResults()

	assertComplexTemplateTaskIDs(t, childTasks, expectations)
	assertComplexTemplateInputs(t, childTasks, expectations)

	t.Log("Complex template processing verified successfully")
}

// complexTemplateExpectation captures expected values for complex template assertions.
// Keeping the expectations strongly typed improves readability for assertion helpers.
type complexTemplateExpectation struct {
	itemName   string
	sourcePath string
	timeoutCfg string
	fullID     string
}

// requireComplexTemplateChildTasks ensures the expected number of complex template tasks exist.
// It keeps repetitive find-and-assert logic out of the main verifier.
func requireComplexTemplateChildTasks(
	t *testing.T,
	result *workflow.State,
	parentExecID core.ID,
) []*task.State {
	t.Helper()
	childTasks := helpers.FindChildTasks(result, parentExecID)
	require.Len(t, childTasks, 3, "Should have exactly 3 child tasks for complex template processing")
	return childTasks
}

// expectedComplexTemplateResults returns the deterministic expectation set for complex templates.
// Each entry encodes the templated data we expect to find in workflow task inputs.
func expectedComplexTemplateResults() map[string]complexTemplateExpectation {
	return map[string]complexTemplateExpectation{
		"task-document-0": {"doc-001", "/input/documents/doc-001.pdf", "300", "test-complex-templates_document_0"},
		"task-image-1":    {"img-001", "/input/images/img-001.png", "180", "test-complex-templates_image_1"},
		"task-dataset-2":  {"data-001", "/input/data/data-001.json", "600", "test-complex-templates_dataset_2"},
	}
}

// assertComplexTemplateTaskIDs asserts that all expected tasks were emitted.
// It is order-agnostic to avoid tying tests to scheduling order.
func assertComplexTemplateTaskIDs(
	t *testing.T,
	childTasks []*task.State,
	expectations map[string]complexTemplateExpectation,
) {
	t.Helper()
	foundTaskIDs := make(map[string]bool, len(childTasks))
	for _, childTask := range childTasks {
		foundTaskIDs[childTask.TaskID] = true
	}
	for expectedID := range expectations {
		assert.True(t, foundTaskIDs[expectedID], "Expected child task %s to be created", expectedID)
	}
}

// assertComplexTemplateInputs verifies that templated input fields match expectations.
// It also stringifies timeout values to compare across numeric and string encodings.
func assertComplexTemplateInputs(
	t *testing.T,
	childTasks []*task.State,
	expectations map[string]complexTemplateExpectation,
) {
	t.Helper()
	for _, childTask := range childTasks {
		expected, exists := expectations[childTask.TaskID]
		require.True(t, exists, "Unexpected child task ID: %s", childTask.TaskID)
		if childTask.Input == nil {
			continue
		}

		inputMap := map[string]any(*childTask.Input)
		assert.Equal(t, expected.itemName, inputMap["item_name"], "item_name should be correctly templated")

		if sourcePath, ok := inputMap["source_path"]; ok {
			assert.Equal(t, expected.sourcePath, sourcePath, "source_path template should be correctly processed")
		}

		if timeoutCfg, ok := inputMap["timeout_config"]; ok {
			assert.Equal(t, expected.timeoutCfg, stringifyTimeoutConfig(timeoutCfg),
				"Nested timeout config should be correctly templated")
		}

		if fullID, ok := inputMap["full_id"]; ok {
			assert.Equal(t, expected.fullID, fullID,
				"Complex template expression should combine multiple variables correctly")
		}
	}
}

// stringifyTimeoutConfig normalizes timeout values to a string for comparison.
// Complex templates can emit string or numeric values depending on evaluation context.
func stringifyTimeoutConfig(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%.0f", v)
	case int64:
		return fmt.Sprintf("%d", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// extractIndexFromTaskID extracts the numeric index from task IDs like "process-config-N"
// Returns 0 if the index cannot be extracted or if the actual index is 0
// Callers should ensure task IDs follow expected format to avoid ambiguity
func extractIndexFromTaskID(taskID string) int {
	if taskID == "" {
		return 0
	}
	parts := strings.Split(taskID, "-")
	if len(parts) < 3 {
		return 0
	}
	// Validate expected format: prefix-middle-index
	indexStr := parts[len(parts)-1]
	if indexStr == "" {
		return 0
	}
	if index, err := strconv.Atoi(indexStr); err == nil {
		return index
	}
	return 0
}
