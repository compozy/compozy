package collection

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/test/integration/worker/helpers"
)

func TestCollectionTask_SequentialExecution(t *testing.T) {
	t.Run("Should execute collection items sequentially and verify database state", func(t *testing.T) {
		// Setup test infrastructure
		basePath, err := filepath.Abs(".")
		require.NoError(t, err)

		fixtureLoader := helpers.NewFixtureLoader(basePath)
		dbHelper := helpers.NewDatabaseHelper(t)

		defer dbHelper.Cleanup(t)

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
		// Setup test infrastructure
		basePath, err := filepath.Abs(".")
		require.NoError(t, err)

		fixtureLoader := helpers.NewFixtureLoader(basePath)
		dbHelper := helpers.NewDatabaseHelper(t)

		defer dbHelper.Cleanup(t)

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
		// Setup test infrastructure
		basePath, err := filepath.Abs(".")
		require.NoError(t, err)

		fixtureLoader := helpers.NewFixtureLoader(basePath)
		dbHelper := helpers.NewDatabaseHelper(t)

		defer dbHelper.Cleanup(t)

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
