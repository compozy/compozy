package router

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	testhelpers "github.com/compozy/compozy/test/helpers"
	"github.com/compozy/compozy/test/integration/worker/helpers"
	"github.com/stretchr/testify/require"
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

func TestRouterTasks_SimpleCondition(t *testing.T) {
	t.Run("Should execute simple conditional routing", func(t *testing.T) {
		t.Parallel()
		basePath := getTestDir()
		fixtureLoader := helpers.NewFixtureLoader(basePath)
		dbHelper := helpers.NewDatabaseHelper(t)
		t.Cleanup(func() { dbHelper.Cleanup(t) })

		fixture := fixtureLoader.LoadFixture(t, "", "simple_condition")
		result := executeWorkflowAndGetState(t, fixture, dbHelper)
		require.NotNil(t, result, "Should have workflow result")

		verifyRouterTaskSucceeded(t, fixture, result)
		verifyConditionalRouting(t, fixture, result)
		fixture.AssertWorkflowState(t, result)
	})
}

func TestRouterTasks_MultipleRoutes(t *testing.T) {
	t.Run("Should handle multiple routes", func(t *testing.T) {
		t.Parallel()
		basePath := getTestDir()
		fixtureLoader := helpers.NewFixtureLoader(basePath)
		dbHelper := helpers.NewDatabaseHelper(t)
		t.Cleanup(func() { dbHelper.Cleanup(t) })

		fixture := fixtureLoader.LoadFixture(t, "", "multiple_routes")
		result := executeWorkflowAndGetState(t, fixture, dbHelper)
		require.NotNil(t, result, "Should have workflow result")

		verifyRouterTaskSucceeded(t, fixture, result)
		verifyConditionalRouting(t, fixture, result)
		fixture.AssertWorkflowState(t, result)
	})
}

func TestRouterTasks_StagingRoute(t *testing.T) {
	t.Run("Should handle staging route selection", func(t *testing.T) {
		t.Parallel()
		basePath := getTestDir()
		fixtureLoader := helpers.NewFixtureLoader(basePath)
		dbHelper := helpers.NewDatabaseHelper(t)
		t.Cleanup(func() { dbHelper.Cleanup(t) })

		fixture := fixtureLoader.LoadFixture(t, "", "staging_route")
		result := executeWorkflowAndGetState(t, fixture, dbHelper)
		require.NotNil(t, result, "Should have workflow result")

		verifyRouterTaskSucceeded(t, fixture, result)
		verifyConditionalRouting(t, fixture, result)
		fixture.AssertWorkflowState(t, result)
	})
}

func TestRouterTasks_ComplexCondition(t *testing.T) {
	t.Run("Should handle complex routing conditions", func(t *testing.T) {
		t.Parallel()
		basePath := getTestDir()
		fixtureLoader := helpers.NewFixtureLoader(basePath)
		dbHelper := helpers.NewDatabaseHelper(t)
		t.Cleanup(func() { dbHelper.Cleanup(t) })

		fixture := fixtureLoader.LoadFixture(t, "", "complex_condition")
		result := executeWorkflowAndGetState(t, fixture, dbHelper)
		require.NotNil(t, result, "Should have workflow result")

		verifyRouterTaskSucceeded(t, fixture, result)
		verifyConditionalRouting(t, fixture, result)
		fixture.AssertWorkflowState(t, result)
	})
}

func TestRouterTasks_DynamicRouting(t *testing.T) {
	t.Run("Should handle dynamic user-based routing", func(t *testing.T) {
		t.Parallel()
		basePath := getTestDir()
		fixtureLoader := helpers.NewFixtureLoader(basePath)
		dbHelper := helpers.NewDatabaseHelper(t)
		t.Cleanup(func() { dbHelper.Cleanup(t) })

		fixture := fixtureLoader.LoadFixture(t, "", "dynamic_routing")
		result := executeWorkflowAndGetState(t, fixture, dbHelper)
		require.NotNil(t, result, "Should have workflow result")

		verifyRouterTaskSucceeded(t, fixture, result)
		verifyConditionalRouting(t, fixture, result)
		fixture.AssertWorkflowState(t, result)
	})
}
