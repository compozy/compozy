package composite

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/test/integration/worker/helpers"
)

func TestCompositeTaskExecution(t *testing.T) {
	// Setup fixture loader
	basePath, err := filepath.Abs(".")
	require.NoError(t, err)
	fixtureLoader := helpers.NewFixtureLoader(basePath)

	// Sequential execution tests
	t.Run("Should execute tasks in sequence", func(t *testing.T) {
		dbHelper := helpers.NewDatabaseHelper(t)
		defer dbHelper.Cleanup(t)

		fixture := fixtureLoader.LoadFixture(t, "", "sequential_execution")
		result := executeWorkflowAndGetState(t, fixture, dbHelper)

		verifySequentialExecution(t, fixture, result)
		verifyChildTaskCreation(t, fixture, result)
		verifyStatePassingBetweenTasks(t, fixture, result)
	})

	// Nested composite tests
	t.Run("Should handle nested composite tasks", func(t *testing.T) {
		dbHelper := helpers.NewDatabaseHelper(t)
		defer dbHelper.Cleanup(t)

		fixture := fixtureLoader.LoadFixture(t, "", "nested_composite")
		result := executeWorkflowAndGetState(t, fixture, dbHelper)

		verifyNestedCompositeExecution(t, fixture, result)
		verifyNestedTaskStates(t, fixture, result)
	})

	// Empty composite tests
	t.Run("Should handle empty composite tasks", func(t *testing.T) {
		dbHelper := helpers.NewDatabaseHelper(t)
		defer dbHelper.Cleanup(t)

		fixture := fixtureLoader.LoadFixture(t, "", "empty_composite")
		result := executeWorkflowAndGetState(t, fixture, dbHelper)

		verifyEmptyCompositeHandling(t, fixture, result)
	})

	// Failure propagation tests
	t.Run("Should handle child task failures", func(t *testing.T) {
		dbHelper := helpers.NewDatabaseHelper(t)
		defer dbHelper.Cleanup(t)

		fixture := fixtureLoader.LoadFixture(t, "", "child_failure")
		result := executeWorkflowAndGetState(t, fixture, dbHelper)

		verifyChildFailurePropagation(t, fixture, result)
		verifyCompositeFailureHandling(t, fixture, result)
	})

	// State management tests
	t.Run("Should manage composite state correctly", func(t *testing.T) {
		dbHelper := helpers.NewDatabaseHelper(t)
		defer dbHelper.Cleanup(t)

		fixture := fixtureLoader.LoadFixture(t, "", "state_passing")
		result := executeWorkflowAndGetState(t, fixture, dbHelper)

		verifyCompositeStateManagement(t, fixture, result)
		verifyChildTaskDataFlow(t, fixture, result)
	})
}

func TestCompositeTaskDatabase(t *testing.T) {
	basePath, err := filepath.Abs(".")
	require.NoError(t, err)
	fixtureLoader := helpers.NewFixtureLoader(basePath)

	t.Run("Should verify database operations", func(t *testing.T) {
		dbHelper := helpers.NewDatabaseHelper(t)
		defer dbHelper.Cleanup(t)

		fixture := fixtureLoader.LoadFixture(t, "", "sequential_execution")
		testDatabaseStateOperations(t, fixture, dbHelper)
	})
}

func TestCompositeTaskRedis(t *testing.T) {
	basePath, err := filepath.Abs(".")
	require.NoError(t, err)
	fixtureLoader := helpers.NewFixtureLoader(basePath)

	t.Run("Should verify redis operations", func(t *testing.T) {
		redisHelper := helpers.NewRedisHelper(t)
		defer redisHelper.Cleanup(t)

		fixture := fixtureLoader.LoadFixture(t, "", "sequential_execution")
		testRedisOperations(t, fixture, redisHelper)
	})
}
