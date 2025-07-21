package basic

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	testhelpers "github.com/compozy/compozy/test/helpers"
	"github.com/compozy/compozy/test/integration/worker/helpers"
)

func TestMain(m *testing.M) {
	// Initialize config for all tests in this package
	if err := testhelpers.InitializeTestConfig(); err != nil {
		panic("failed to initialize test config: " + err.Error())
	}
	os.Exit(m.Run())
}

func getTestDir() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("failed to get caller info")
	}
	return filepath.Dir(filename)
}

func TestBasicTask_SuccessfulExecution(t *testing.T) {
	t.Run("Should execute basic task successfully and verify database state", func(t *testing.T) {
		t.Parallel()
		// Setup test infrastructure
		basePath := getTestDir()

		fixtureLoader := helpers.NewFixtureLoader(basePath)
		dbHelper := helpers.NewDatabaseHelper(t)
		redisHelper := helpers.NewRedisHelper(t)

		t.Cleanup(func() {
			dbHelper.Cleanup(t)
			redisHelper.Cleanup(t)
		})

		// Load fixture
		fixture := fixtureLoader.LoadFixture(t, "", "simple_success")

		// Execute real workflow and retrieve state from database
		t.Log("Executing basic workflow and verifying database state")
		result := executeWorkflowAndGetState(t, fixture, dbHelper)

		// Verify the actual database state matches expectations
		verifyBasicTaskExecution(t, fixture, result)
		verifyBasicTaskInputs(t, fixture, result)

		// Additional assertion using fixture expectations
		fixture.AssertWorkflowState(t, result)
	})
}

func TestBasicTask_WithError(t *testing.T) {
	t.Run("Should handle basic task with error and verify database state", func(t *testing.T) {
		t.Parallel()
		// Setup test infrastructure
		basePath := getTestDir()

		fixtureLoader := helpers.NewFixtureLoader(basePath)
		dbHelper := helpers.NewDatabaseHelper(t)
		redisHelper := helpers.NewRedisHelper(t)

		t.Cleanup(func() {
			dbHelper.Cleanup(t)
			redisHelper.Cleanup(t)
		})

		// Load fixture
		fixture := fixtureLoader.LoadFixture(t, "", "with_error")

		// Execute real workflow and retrieve state from database
		t.Log("Executing error handling workflow and verifying database state")
		result := executeWorkflowAndGetState(t, fixture, dbHelper)

		// Verify the actual database state handles error correctly
		verifyBasicErrorHandling(t, fixture, result)

		// Additional assertion using fixture expectations
		fixture.AssertWorkflowState(t, result)
	})
}

func TestBasicTask_WithNextTransitions(t *testing.T) {
	t.Run("Should handle basic task with next transitions and verify database state", func(t *testing.T) {
		t.Parallel()
		// Setup test infrastructure
		basePath := getTestDir()

		fixtureLoader := helpers.NewFixtureLoader(basePath)
		dbHelper := helpers.NewDatabaseHelper(t)
		redisHelper := helpers.NewRedisHelper(t)

		t.Cleanup(func() {
			dbHelper.Cleanup(t)
			redisHelper.Cleanup(t)
		})

		// Load fixture
		fixture := fixtureLoader.LoadFixture(t, "", "with_next_task")

		// Execute real workflow and retrieve state from database
		t.Log("Executing next task transition workflow and verifying database state")
		result := executeWorkflowAndGetState(t, fixture, dbHelper)

		// Verify the actual database state handles transitions correctly
		verifyBasicTaskTransitions(t, fixture, result)
		verifyBasicTaskInputs(t, fixture, result)

		// Additional assertion using fixture expectations
		fixture.AssertWorkflowState(t, result)
	})
}

func TestBasicTask_WithFinalFlag(t *testing.T) {
	t.Run("Should handle basic task with final flag and verify database state", func(t *testing.T) {
		t.Parallel()
		// Setup test infrastructure
		basePath := getTestDir()

		fixtureLoader := helpers.NewFixtureLoader(basePath)
		dbHelper := helpers.NewDatabaseHelper(t)
		redisHelper := helpers.NewRedisHelper(t)

		t.Cleanup(func() {
			dbHelper.Cleanup(t)
			redisHelper.Cleanup(t)
		})

		// Load fixture
		fixture := fixtureLoader.LoadFixture(t, "", "final_task")

		// Execute real workflow and retrieve state from database
		t.Log("Executing final task workflow and verifying database state")
		result := executeWorkflowAndGetState(t, fixture, dbHelper)

		// Verify the actual database state handles final task correctly
		verifyFinalTaskBehavior(t, fixture, result)
		verifyBasicTaskExecution(t, fixture, result)

		// Additional assertion using fixture expectations
		fixture.AssertWorkflowState(t, result)
	})
}
