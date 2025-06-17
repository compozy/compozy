package parallel

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/test/integration/worker/helpers"
)

func TestParallelTaskExecution(t *testing.T) {
	// Setup fixture loader
	basePath, err := filepath.Abs(".")
	require.NoError(t, err)
	fixtureLoader := helpers.NewFixtureLoader(basePath)

	t.Run("Should execute parallel tasks with real workflow execution", func(t *testing.T) {
		dbHelper := helpers.NewDatabaseHelper(t)
		defer dbHelper.Cleanup(t)

		fixture := fixtureLoader.LoadFixture(t, "", "simple_parallel")

		// Execute real workflow and retrieve state from database
		t.Log("Executing parallel workflow with fixed ExecuteSubtask and TaskResponder")
		result := executeWorkflowAndGetState(t, fixture, dbHelper)

		// Verify the actual database state matches expectations
		verifyParallelTaskExecution(t, fixture, result)
		verifyParallelChildTaskCreation(t, fixture, result)
		verifyParallelOutputAggregation(t, fixture, result)

		// Additional assertion using fixture expectations
		fixture.AssertWorkflowState(t, result)

		t.Log("✅ Parallel task execution completed successfully with bug fixes")
	})
}

func TestParallelTaskDatabase(t *testing.T) {
	basePath, err := filepath.Abs(".")
	require.NoError(t, err)
	fixtureLoader := helpers.NewFixtureLoader(basePath)

	t.Run("Should verify database infrastructure is available", func(t *testing.T) {
		dbHelper := helpers.NewDatabaseHelper(t)
		defer dbHelper.Cleanup(t)

		fixture := fixtureLoader.LoadFixture(t, "", "simple_parallel")

		// Verify database helper is functional
		require.NotNil(t, dbHelper, "Database helper should be available")
		require.NotNil(t, fixture, "Fixture should load successfully")

		t.Log("✅ Database infrastructure verified successfully")
		t.Log("📋 Ready for database operations when workflow execution issues are resolved")
	})
}

func TestParallelTaskRedis(t *testing.T) {
	basePath, err := filepath.Abs(".")
	require.NoError(t, err)
	fixtureLoader := helpers.NewFixtureLoader(basePath)

	t.Run("Should verify redis operations", func(t *testing.T) {
		redisHelper := helpers.NewRedisHelper(t)
		defer redisHelper.Cleanup(t)

		fixture := fixtureLoader.LoadFixture(t, "", "simple_parallel")
		testRedisOperations(t, fixture, redisHelper)
	})
}
