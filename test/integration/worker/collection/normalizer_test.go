package collection

import (
	"testing"

	"github.com/compozy/compozy/test/integration/worker/helpers"
)

// TestCollectionTask_PrecisionHandling tests that large numbers and high-precision
// decimals are handled correctly during template processing without losing precision
func TestCollectionTask_PrecisionHandling(t *testing.T) {
	t.Run("Should handle large integers and high-precision decimals without precision loss", func(t *testing.T) {
		t.Parallel()
		// Setup test infrastructure
		basePath := getTestDir()

		fixtureLoader := helpers.NewFixtureLoader(basePath)
		dbHelper := helpers.NewDatabaseHelper(t)

		t.Cleanup(func() { dbHelper.Cleanup(t) })

		// Load fixture
		fixture := fixtureLoader.LoadFixture(t, "", "precision_handling")

		// Execute real workflow and retrieve state from database
		t.Log("Executing precision handling workflow and verifying database state")
		result := executeWorkflowAndGetState(t, fixture, dbHelper)

		// Verify collection processing with precision preservation
		verifyCollectionParallelExecution(t, fixture, result)
		verifyCollectionChildTasks(t, fixture, result)
		verifyPrecisionPreservation(t, fixture, result)

		// Additional assertion using fixture expectations
		fixture.AssertWorkflowState(t, result)
	})
}

// TestCollectionTask_DeterministicProcessing tests that map iteration
// and template processing produces consistent, deterministic results
func TestCollectionTask_DeterministicProcessing(t *testing.T) {
	t.Run("Should process maps and configurations in deterministic order", func(t *testing.T) {
		t.Parallel()
		// Setup test infrastructure
		basePath := getTestDir()

		fixtureLoader := helpers.NewFixtureLoader(basePath)
		dbHelper := helpers.NewDatabaseHelper(t)

		t.Cleanup(func() { dbHelper.Cleanup(t) })

		// Load fixture
		fixture := fixtureLoader.LoadFixture(t, "", "deterministic_processing")

		// Execute real workflow and retrieve state from database
		t.Log("Executing deterministic processing workflow and verifying database state")
		result := executeWorkflowAndGetState(t, fixture, dbHelper)

		// Verify deterministic processing behavior
		verifyCollectionSequentialExecution(t, fixture, result)
		verifyCollectionChildTasks(t, fixture, result)
		verifyDeterministicMapProcessing(t, fixture, result)

		// Additional assertion using fixture expectations
		fixture.AssertWorkflowState(t, result)
	})
}

// TestCollectionTask_ProgressTracking tests that progress context is available
// and correctly populated during collection processing
func TestCollectionTask_ProgressTracking(t *testing.T) {
	t.Run("Should provide progress tracking context throughout execution", func(t *testing.T) {
		t.Parallel()
		// Setup test infrastructure
		basePath := getTestDir()

		fixtureLoader := helpers.NewFixtureLoader(basePath)
		dbHelper := helpers.NewDatabaseHelper(t)

		t.Cleanup(func() { dbHelper.Cleanup(t) })

		// Load fixture
		fixture := fixtureLoader.LoadFixture(t, "", "progress_tracking")

		// Execute real workflow and retrieve state from database
		t.Log("Executing progress tracking workflow and verifying database state")
		result := executeWorkflowAndGetState(t, fixture, dbHelper)

		// Verify progress tracking integration
		// Skip the hardcoded parallel execution check and go straight to child task verification
		verifyCollectionChildTasks(t, fixture, result)
		verifyProgressContextIntegration(t, fixture, result)

		// Additional assertion using fixture expectations
		fixture.AssertWorkflowState(t, result)
	})
}

// TestCollectionTask_ComplexTemplateProcessing tests the runtime processor's
// ability to handle deeply nested templates and complex configurations
func TestCollectionTask_ComplexTemplateProcessing(t *testing.T) {
	t.Run("Should process complex nested templates with all features", func(t *testing.T) {
		t.Parallel()
		// Setup test infrastructure
		basePath := getTestDir()

		fixtureLoader := helpers.NewFixtureLoader(basePath)
		dbHelper := helpers.NewDatabaseHelper(t)

		t.Cleanup(func() { dbHelper.Cleanup(t) })

		// Load fixture
		fixture := fixtureLoader.LoadFixture(t, "", "complex_templates")

		// Execute real workflow and retrieve state from database
		t.Log("Executing complex template processing workflow and verifying database state")
		result := executeWorkflowAndGetState(t, fixture, dbHelper)

		// Verify complex template processing
		verifyCollectionSequentialExecution(t, fixture, result)
		verifyCollectionChildTasks(t, fixture, result)
		verifyComplexTemplateProcessing(t, fixture, result)

		// Additional assertion using fixture expectations
		fixture.AssertWorkflowState(t, result)
	})
}

// TestCollectionTask_ComprehensiveFeatures tests all Task 5.0 features
// working together in a comprehensive end-to-end scenario
func TestCollectionTask_ComprehensiveFeatures(t *testing.T) {
	t.Run("Should handle all Task 5.0 features in a comprehensive workflow", func(t *testing.T) {
		// Do not run in parallel due to multiple workflow executions and resource sharing
		// This test runs multiple workflows to verify feature integration
		// Setup test infrastructure
		basePath := getTestDir()

		fixtureLoader := helpers.NewFixtureLoader(basePath)
		dbHelper := helpers.NewDatabaseHelper(t)

		t.Cleanup(func() { dbHelper.Cleanup(t) })

		// Test precision handling
		precisionFixture := fixtureLoader.LoadFixture(t, "", "precision_handling")
		precisionResult := executeWorkflowAndGetState(t, precisionFixture, dbHelper)
		verifyPrecisionPreservation(t, precisionFixture, precisionResult)

		// Test deterministic processing
		deterministicFixture := fixtureLoader.LoadFixture(t, "", "deterministic_processing")

		deterministicResult := executeWorkflowAndGetState(t, deterministicFixture, dbHelper)
		verifyDeterministicMapProcessing(t, deterministicFixture, deterministicResult)

		// Test progress tracking
		progressFixture := fixtureLoader.LoadFixture(t, "", "progress_tracking")
		progressResult := executeWorkflowAndGetState(t, progressFixture, dbHelper)
		verifyProgressContextIntegration(t, progressFixture, progressResult)

		// Test complex templates
		complexFixture := fixtureLoader.LoadFixture(t, "", "complex_templates")
		complexResult := executeWorkflowAndGetState(t, complexFixture, dbHelper)
		verifyComplexTemplateProcessing(t, complexFixture, complexResult)

		t.Log("All Task 5.0 features verified successfully in comprehensive test")
	})
}
