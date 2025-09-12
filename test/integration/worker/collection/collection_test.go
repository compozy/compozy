package collection

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/compozy/compozy/test/integration/worker/helpers"
)

func getTestDir() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("failed to get caller info")
	}
	return filepath.Dir(filename)
}

func TestCollectionTask_SequentialExecution(t *testing.T) {
	t.Run("Should execute collection items sequentially and verify database state", func(t *testing.T) {
		t.Parallel()
		// Setup test infrastructure
		basePath := getTestDir()

		fixtureLoader := helpers.NewFixtureLoader(basePath)
		dbHelper := helpers.NewDatabaseHelper(t)

		t.Cleanup(func() { dbHelper.Cleanup(t) })

		// Load fixture
		fixture := fixtureLoader.LoadFixture(t, "", "sequential_items")

		// Execute real workflow and retrieve state from database
		t.Log("Executing collection workflow and verifying database state")
		result := executeWorkflowAndGetState(t, fixture, dbHelper)

		// Verify the actual database state matches expectations
		verifyCollectionSequentialExecution(t, fixture, result)
		verifyCollectionChildTasks(t, fixture, result)
		verifyCollectionOutputAggregation(t, fixture, result)

		// Additional assertion using fixture expectations
		fixture.AssertWorkflowState(t, result)
	})
}

func TestCollectionTask_ParallelExecution(t *testing.T) {
	t.Run("Should execute collection items in parallel and verify database state", func(t *testing.T) {
		t.Parallel()
		// Setup test infrastructure
		basePath := getTestDir()

		fixtureLoader := helpers.NewFixtureLoader(basePath)
		dbHelper := helpers.NewDatabaseHelper(t)

		t.Cleanup(func() { dbHelper.Cleanup(t) })

		// Load fixture
		fixture := fixtureLoader.LoadFixture(t, "", "parallel_items")

		// Execute real workflow and retrieve state from database
		t.Log("Executing parallel collection workflow and verifying database state")
		result := executeWorkflowAndGetState(t, fixture, dbHelper)

		// Verify the actual database state matches expectations
		verifyCollectionParallelExecution(t, fixture, result)
		verifyCollectionChildTasks(t, fixture, result)
		verifyCollectionOutputAggregation(t, fixture, result)

		// Additional assertion using fixture expectations
		fixture.AssertWorkflowState(t, result)
	})
}

func TestCollectionTask_EmptyCollection(t *testing.T) {
	t.Run("Should handle empty collections gracefully and verify database state", func(t *testing.T) {
		t.Parallel()
		// Setup test infrastructure
		basePath := getTestDir()

		fixtureLoader := helpers.NewFixtureLoader(basePath)
		dbHelper := helpers.NewDatabaseHelper(t)

		t.Cleanup(func() { dbHelper.Cleanup(t) })

		// Load fixture
		fixture := fixtureLoader.LoadFixture(t, "", "empty_collection")

		// Execute real workflow and retrieve state from database
		t.Log("Executing empty collection workflow and verifying database state")
		result := executeWorkflowAndGetState(t, fixture, dbHelper)

		// Verify the actual database state handles empty collection correctly
		verifyEmptyCollectionHandling(t, fixture, result)
		verifyCollectionOutputAggregation(t, fixture, result)

		// Additional assertion using fixture expectations
		fixture.AssertWorkflowState(t, result)
	})
}
