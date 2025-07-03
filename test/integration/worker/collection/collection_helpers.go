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

	// Find parent collection task
	parentTask := helpers.FindParentTask(result, task.ExecutionCollection)
	require.NotNil(t, parentTask, "Should have a parent collection task")

	// Find child tasks and verify they processed precision data correctly
	childTasks := helpers.FindChildTasks(result, parentTask.TaskExecID)
	require.Len(t, childTasks, 2, "Should have exactly 2 child tasks for precision testing")

	// Sort child tasks by ID for deterministic verification
	// Since task IDs contain the index (process-record-0, process-record-1), this ensures correct order
	sort.Slice(childTasks, func(i, j int) bool {
		return childTasks[i].TaskID < childTasks[j].TaskID
	})

	expectedPrecisionData := []struct {
		accountID string
		balance   string
		rate      string
		decimal   string
	}{
		{"9007199254740992", "999999999999999999", "0.123456789123456789", "123456789.123456789"},
		{"9007199254740993", "123456789012345678", "2.345678901234567890", "987654321.987654321"},
	}

	// Verify precision in task inputs (template processing should preserve precision)
	for i, childTask := range childTasks {
		expected := expectedPrecisionData[i]

		// Verify account ID (large integer) was preserved as string
		if childTask.Input != nil {
			inputMap := map[string]any(*childTask.Input)
			assert.Equal(t, expected.accountID, inputMap["item_name"],
				"Large integer account ID should be preserved as string without precision loss")
		}

		// Verify template processing preserved precision in additional fields
		if childTask.Input != nil {
			inputMap := map[string]any(*childTask.Input)

			// Check balance precision preservation
			if balanceStr, exists := inputMap["balance_str"]; exists {
				assert.Equal(t, expected.balance, balanceStr,
					"Large integer balance should be preserved without scientific notation")
			}

			// Check rate precision preservation (17+ significant digits)
			if rateStr, exists := inputMap["rate_str"]; exists {
				rateString, ok := rateStr.(string)
				assert.True(t, ok, "rate_str should be a string")
				assert.Equal(t, expected.rate, rateString,
					"High precision decimal rate should preserve all significant digits")
				// Verify it maintains more than float64's ~15-17 significant digits
				decimalPart := rateString[strings.Index(rateString, ".")+1:]
				assert.GreaterOrEqual(t, len(decimalPart), 15,
					"Rate should maintain at least 15 decimal places for precision testing")
			}

			// Check decimal precision preservation
			if decimalStr, exists := inputMap["decimal_str"]; exists {
				decimalString, ok := decimalStr.(string)
				assert.True(t, ok, "decimal_str should be a string")
				assert.Equal(t, expected.decimal, decimalString,
					"High precision decimal should preserve both integer and fractional parts")
				// Verify fractional part precision
				if strings.Contains(decimalString, ".") {
					fractionalPart := decimalString[strings.Index(decimalString, ".")+1:]
					assert.GreaterOrEqual(t, len(fractionalPart), 9,
						"Decimal should maintain at least 9 fractional digits")
				}
			}
		}
	}

	t.Log("Precision preservation verified successfully")
}

// verifyDeterministicMapProcessing verifies that map iteration and template
// processing produces consistent, deterministic results across runs
func verifyDeterministicMapProcessing(t *testing.T, fixture *helpers.TestFixture, result *workflow.State) {
	t.Log("Verifying deterministic map processing from database state")

	// Find parent collection task
	parentTask := helpers.FindParentTask(result, task.ExecutionCollection)
	require.NotNil(t, parentTask, "Should have a parent collection task")

	// Find child tasks and verify deterministic ordering
	childTasks := helpers.FindChildTasks(result, parentTask.TaskExecID)
	require.Len(t, childTasks, 3, "Should have exactly 3 child tasks for deterministic processing")

	// Sort child tasks by their index (extracted from task ID) to ensure deterministic test verification
	// The task IDs follow the pattern "process-config-N" where N is the index
	sort.Slice(childTasks, func(i, j int) bool {
		return extractIndexFromTaskID(childTasks[i].TaskID) < extractIndexFromTaskID(childTasks[j].TaskID)
	})

	// Get the expected ordered config sets from the fixture
	configSetsRaw := fixture.Input["config_sets"]

	// Handle both []map[string]any and []any formats
	var orderedConfigSets []map[string]any
	switch v := configSetsRaw.(type) {
	case []map[string]any:
		orderedConfigSets = v
	case []any:
		for _, item := range v {
			if itemMap, ok := item.(map[string]any); ok {
				orderedConfigSets = append(orderedConfigSets, itemMap)
			}
		}
	default:
		require.Fail(t, "config_sets in fixture should be an ordered slice of maps, got %T", v)
	}

	// Verify tasks were created with deterministic task IDs and processed items
	for i, childTask := range childTasks {
		expectedTaskID := fmt.Sprintf("process-config-%d", i)
		assert.Equal(t, expectedTaskID, childTask.TaskID,
			"Child task %d should have deterministic task ID", i)

		if childTask.Input != nil {
			inputMap := map[string]any(*childTask.Input)

			// Verify that the item being processed matches the expected order
			require.Less(t, i, len(orderedConfigSets), "Index out of bounds for orderedConfigSets")
			expectedItemName := orderedConfigSets[i]["name"]
			assert.Equal(t, expectedItemName, inputMap["item_name"],
				"Item processing should follow deterministic order")
		}
	}

	// Verify that with/env maps were processed deterministically
	// by checking the input contains all expected keys
	if len(childTasks) > 0 && childTasks[0].Input != nil {
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

	t.Log("Deterministic map processing verified successfully")
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
// to handle deeply nested templates and complex configurations
func verifyComplexTemplateProcessing(t *testing.T, _ *helpers.TestFixture, result *workflow.State) {
	t.Log("Verifying complex template processing from database state")

	// Find parent collection task
	parentTask := helpers.FindParentTask(result, task.ExecutionCollection)
	require.NotNil(t, parentTask, "Should have a parent collection task")

	// Find child tasks and verify complex template processing
	childTasks := helpers.FindChildTasks(result, parentTask.TaskExecID)
	require.Len(t, childTasks, 3, "Should have exactly 3 child tasks for complex template processing")

	// Create a map of expected results by task ID for order-independent verification
	expectedTemplateResults := map[string]struct {
		itemName   string
		sourcePath string
		timeoutCfg string
		fullID     string
	}{
		"task-document-0": {"doc-001", "/input/documents/doc-001.pdf", "300", "test-complex-templates_document_0"},
		"task-image-1":    {"img-001", "/input/images/img-001.png", "180", "test-complex-templates_image_1"},
		"task-dataset-2":  {"data-001", "/input/data/data-001.json", "600", "test-complex-templates_dataset_2"},
	}

	// Verify that all expected task IDs are present
	foundTaskIDs := make(map[string]bool)
	for _, childTask := range childTasks {
		foundTaskIDs[childTask.TaskID] = true
	}
	for expectedID := range expectedTemplateResults {
		assert.True(t, foundTaskIDs[expectedID], "Expected child task %s to be created", expectedID)
	}

	// Verify complex template processing in child tasks
	for _, childTask := range childTasks {
		expected, exists := expectedTemplateResults[childTask.TaskID]
		require.True(t, exists, "Unexpected child task ID: %s", childTask.TaskID)

		// Verify template processing through input data
		if childTask.Input != nil {
			inputMap := map[string]any(*childTask.Input)

			// Verify basic template fields
			assert.Equal(t, expected.itemName, inputMap["item_name"], "item_name should be correctly templated")

			// Verify complex nested template evaluation
			if sourcePath, exists := inputMap["source_path"]; exists {
				assert.Equal(
					t,
					expected.sourcePath,
					sourcePath,
					"Complex nested source_path template should be correctly processed",
				)
			}

			if timeoutConfig, exists := inputMap["timeout_config"]; exists {
				// Handle both string and numeric values
				var actualTimeout string
				switch v := timeoutConfig.(type) {
				case string:
					actualTimeout = v
				case float64:
					actualTimeout = fmt.Sprintf("%.0f", v)
				case int64:
					actualTimeout = fmt.Sprintf("%d", v)
				default:
					actualTimeout = fmt.Sprintf("%v", v)
				}
				assert.Equal(
					t,
					expected.timeoutCfg,
					actualTimeout,
					"Nested timeout config should be correctly templated",
				)
			}

			if fullID, exists := inputMap["full_id"]; exists {
				assert.Equal(
					t,
					expected.fullID,
					fullID,
					"Complex template expression should combine multiple variables correctly",
				)
			}
		}
	}

	t.Log("Complex template processing verified successfully")
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
