package collection

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/test/integration/worker/helpers"
)

func TestCollectionTask_OutputTransformation(t *testing.T) {
	t.Run("Should transform child outputs with item context", func(t *testing.T) {
		// Setup test infrastructure
		basePath := getTestDir()

		fixtureLoader := helpers.NewFixtureLoader(basePath)
		dbHelper := helpers.NewDatabaseHelper(t)

		t.Cleanup(func() { dbHelper.Cleanup(t) })

		// Load fixture
		fixture := fixtureLoader.LoadFixture(t, "", "output_transformation")

		// Execute real workflow and retrieve state from database
		t.Log("Executing collection workflow with output transformation")
		result := executeWorkflowAndGetState(t, fixture, dbHelper)

		// Verify the output transformation worked correctly
		verifyCollectionOutputTransformation(t, fixture, result)

		// Additional assertion using fixture expectations
		fixture.AssertWorkflowState(t, result)
	})

	t.Run("Should handle custom item and index variable names", func(t *testing.T) {
		// Setup test infrastructure
		basePath := getTestDir()

		fixtureLoader := helpers.NewFixtureLoader(basePath)
		dbHelper := helpers.NewDatabaseHelper(t)

		t.Cleanup(func() { dbHelper.Cleanup(t) })

		// Load fixture
		fixture := fixtureLoader.LoadFixture(t, "", "custom_vars")

		// Execute real workflow and retrieve state from database
		t.Log("Executing collection workflow with custom variable names")
		result := executeWorkflowAndGetState(t, fixture, dbHelper)

		// Verify the output transformation worked with custom variables
		verifyCustomVariableOutputTransformation(t, fixture, result)

		// Additional assertion using fixture expectations
		fixture.AssertWorkflowState(t, result)
	})
}

// verifyCollectionOutputTransformation verifies that child outputs are transformed correctly with item context
func verifyCollectionOutputTransformation(t *testing.T, _ *helpers.TestFixture, result *workflow.State) {
	t.Log("Verifying collection child output transformation from database state")

	// Find parent collection task
	parentTask := helpers.FindParentTask(result, task.ExecutionCollection)
	require.NotNil(t, parentTask, "Should have a parent collection task")

	// Find child tasks
	childTasks := helpers.FindChildTasks(result, parentTask.TaskExecID)
	require.Greater(t, len(childTasks), 0, "Should have child tasks")

	// Verify each child task's output transformation
	for _, childTask := range childTasks {
		// Child task should be successful
		helpers.VerifyTaskStatus(t, childTask, string(core.StatusSuccess), "Child task")

		// Verify output exists
		helpers.VerifyTaskHasOutput(t, childTask, "Child task")

		if childTask.Output != nil {
			output := *childTask.Output

			// Verify that the item from the collection is included in the output
			// This tests that {{ .item }} was available during output transformation
			if activity, exists := output["activity"]; exists {
				assert.NotNil(t, activity, "Activity should not be nil")
				assert.NotEmpty(t, activity, "Activity should not be empty")
			}

			// Verify that the index is included - no need to match specific index
			// as child tasks may be returned in different order
			if index, exists := output["index"]; exists {
				assert.NotNil(t, index, "Index should not be nil")
				// Verify index is within expected range
				switch v := index.(type) {
				case float64:
					assert.GreaterOrEqual(t, v, float64(0), "Index should be >= 0")
					assert.Less(t, v, float64(3), "Index should be < 3")
				case int:
					assert.GreaterOrEqual(t, v, 0, "Index should be >= 0")
					assert.Less(t, v, 3, "Index should be < 3")
				case string:
					// Try to parse string index
					if idx, err := fmt.Sscanf(v, "%d", new(int)); err == nil && idx == 1 {
						indexInt := 0
						fmt.Sscanf(v, "%d", &indexInt)
						assert.GreaterOrEqual(t, indexInt, 0, "Index should be >= 0")
						assert.Less(t, indexInt, 3, "Index should be < 3")
					}
				default:
					t.Errorf("Unexpected index type: %T", index)
				}
			}

			// Verify that the analysis result is included
			if result, exists := output["result"]; exists {
				assert.NotNil(t, result, "Result should not be nil")
			}
		}
	}

	// Verify parent task completed successfully
	helpers.VerifyTaskStatus(t, parentTask, string(core.StatusSuccess), "Parent collection task")
}

// verifyCustomVariableOutputTransformation verifies output transformation with custom item/index variable names
func verifyCustomVariableOutputTransformation(t *testing.T, _ *helpers.TestFixture, result *workflow.State) {
	t.Log("Verifying custom variable output transformation from database state")

	// Find parent collection task
	parentTask := helpers.FindParentTask(result, task.ExecutionCollection)
	require.NotNil(t, parentTask, "Should have a parent collection task")

	// Find child tasks
	childTasks := helpers.FindChildTasks(result, parentTask.TaskExecID)
	require.Greater(t, len(childTasks), 0, "Should have child tasks")

	// Verify each child task's output transformation with custom variables
	for _, childTask := range childTasks {
		// Child task should be successful
		helpers.VerifyTaskStatus(t, childTask, string(core.StatusSuccess), "Child task")

		// Verify output exists
		helpers.VerifyTaskHasOutput(t, childTask, "Child task")

		if childTask.Output != nil {
			output := *childTask.Output

			// Verify custom variable names were used in output transformation
			// For example, if the custom item var is "city" and index var is "position"
			if cityName, exists := output["city_name"]; exists {
				assert.NotNil(t, cityName, "City name should not be nil")
				assert.NotEmpty(t, cityName, "City name should not be empty")
			}

			if position, exists := output["position"]; exists {
				assert.NotNil(t, position, "Position should not be nil")
				// Verify position is within expected range
				switch v := position.(type) {
				case float64:
					assert.GreaterOrEqual(t, v, float64(0), "Position should be >= 0")
					assert.Less(t, v, float64(3), "Position should be < 3")
				case int:
					assert.GreaterOrEqual(t, v, 0, "Position should be >= 0")
					assert.Less(t, v, 3, "Position should be < 3")
				case string:
					// Try to parse string position
					if idx, err := fmt.Sscanf(v, "%d", new(int)); err == nil && idx == 1 {
						posInt := 0
						fmt.Sscanf(v, "%d", &posInt)
						assert.GreaterOrEqual(t, posInt, 0, "Position should be >= 0")
						assert.Less(t, posInt, 3, "Position should be < 3")
					}
				default:
					t.Errorf("Unexpected position type: %T", position)
				}
			}

			// Verify the message was properly generated
			if message, exists := output["message"]; exists {
				assert.NotNil(t, message, "Message should not be nil")
				assert.Contains(t, message.(string), "Processing", "Message should contain processing text")
			}
		}
	}
}
