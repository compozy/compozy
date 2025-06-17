package router

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/test/integration/worker/helpers"
)

func TestRouterTasks(t *testing.T) {
	basePath, err := filepath.Abs(".")
	require.NoError(t, err)
	fixtureLoader := helpers.NewFixtureLoader(basePath)

	testCases := []struct {
		name        string
		fixtureName string
		verifiers   []func(t *testing.T, fixture *helpers.TestFixture, result *workflow.State)
	}{
		{
			name:        "Should execute simple conditional routing",
			fixtureName: "simple_condition",
			verifiers: []func(t *testing.T, fixture *helpers.TestFixture, result *workflow.State){
				verifyRouterTaskSucceeded,
				verifyConditionalRouting,
			},
		},
		{
			name:        "Should handle multiple routes",
			fixtureName: "multiple_routes",
			verifiers: []func(t *testing.T, fixture *helpers.TestFixture, result *workflow.State){
				verifyRouterTaskSucceeded,
				verifyConditionalRouting,
			},
		},
		{
			name:        "Should handle staging route selection",
			fixtureName: "staging_route",
			verifiers: []func(t *testing.T, fixture *helpers.TestFixture, result *workflow.State){
				verifyRouterTaskSucceeded,
				verifyConditionalRouting,
			},
		},
		{
			name:        "Should handle complex routing conditions",
			fixtureName: "complex_condition",
			verifiers: []func(t *testing.T, fixture *helpers.TestFixture, result *workflow.State){
				verifyRouterTaskSucceeded,
				verifyConditionalRouting,
			},
		},
		{
			name:        "Should handle dynamic user-based routing",
			fixtureName: "dynamic_routing",
			verifiers: []func(t *testing.T, fixture *helpers.TestFixture, result *workflow.State){
				verifyRouterTaskSucceeded,
				verifyConditionalRouting,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dbHelper := helpers.NewDatabaseHelper(t)
			defer dbHelper.Cleanup(t)

			fixture := fixtureLoader.LoadFixture(t, "", tc.fixtureName)

			result := executeWorkflowAndGetState(t, fixture, dbHelper)
			require.NotNil(t, result, "Should have workflow result")

			for _, verify := range tc.verifiers {
				verify(t, fixture, result)
			}
			fixture.AssertWorkflowState(t, result)
		})
	}
}
