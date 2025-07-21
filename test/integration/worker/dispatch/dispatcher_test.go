package worker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/worker"
	wf "github.com/compozy/compozy/engine/workflow"
	wfacts "github.com/compozy/compozy/engine/workflow/activities"
	"github.com/compozy/compozy/pkg/config"
)

func TestEventSignal_Structure(t *testing.T) {
	t.Run("Should create EventSignal correctly", func(t *testing.T) {
		signal := worker.EventSignal{
			Name:          "order.created",
			Payload:       core.Input{"orderId": "123"},
			CorrelationID: "test-correlation-id",
		}
		assert.Equal(t, "order.created", signal.Name)
		assert.Equal(t, "123", signal.Payload["orderId"])
		assert.Equal(t, "test-correlation-id", signal.CorrelationID)
	})

	t.Run("Should handle empty payload", func(t *testing.T) {
		signal := worker.EventSignal{
			Name:    "test.event",
			Payload: core.Input{},
		}
		assert.Equal(t, "test.event", signal.Name)
		assert.NotNil(t, signal.Payload)
		assert.Len(t, signal.Payload, 0)
	})
}

// Workflow tests are temporarily disabled due to complex Temporal test setup requirements
// The dispatcher workflow functionality is tested through integration tests
func TestDispatcherWorkflow_SuccessfulDispatch(t *testing.T) {
	t.Run("Should dispatch event to matching workflow", func(t *testing.T) {
		testSuite := &testsuite.WorkflowTestSuite{}
		env := testSuite.NewTestWorkflowEnvironment()
		defer func() {
			env.AssertExpectations(t)
		}()

		mockWorkflows := []*wf.Config{
			{
				ID: "order-processor",
				Triggers: []wf.Trigger{
					{Type: wf.TriggerTypeSignal, Name: "order.created"},
				},
			},
		}

		// Create a mock project config with default heartbeat settings
		mockProjectConfig := &project.Config{
			Name: "test-project",
		}

		// Create a GetData activity instance for testing
		getData := &wfacts.GetData{
			ProjectConfig: mockProjectConfig,
			Workflows:     mockWorkflows,
		}

		// Register the activity with the test environment using the correct activity label
		env.RegisterActivityWithOptions(getData.Run, activity.RegisterOptions{Name: wfacts.GetDataLabel})
		env.OnActivity(wfacts.GetDataLabel, mock.Anything, mock.Anything).
			Return(&wfacts.GetData{
				ProjectConfig: mockProjectConfig,
				Workflows:     mockWorkflows,
				AppConfig:     config.Default(),
			}, nil)

		// Expect exactly one child workflow to be started
		env.OnWorkflow("CompozyWorkflow", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Once()
		env.RegisterWorkflow(worker.DispatcherWorkflow)
		env.RegisterWorkflow(worker.CompozyWorkflow)

		workflowFinished := make(chan struct{})
		// Execute the workflow in a goroutine to avoid hanging
		go func() {
			defer close(workflowFinished)
			env.ExecuteWorkflow(worker.DispatcherWorkflow, "test-project", "test-server")
		}()

		// Use RegisterDelayedCallback for more reliable timing
		env.RegisterDelayedCallback(func() {
			env.SignalWorkflow(worker.DispatcherEventChannel, worker.EventSignal{
				Name:          "order.created",
				Payload:       core.Input{"orderId": "123"},
				CorrelationID: "test-correlation-id",
			})
		}, 10*time.Millisecond)

		// Give the workflow time to process the signal and start the child workflow
		env.RegisterDelayedCallback(func() {
			// Cancel the workflow to allow the test to finish
			env.CancelWorkflow()
		}, 100*time.Millisecond)

		// Wait for workflow completion
		<-workflowFinished

		// Verify the workflow was canceled (expected for long-running workflows)
		assert.Error(t, env.GetWorkflowError(), "Workflow should be canceled")
		env.AssertExpectations(t)
	})
}

func TestDispatcherWorkflow_UnknownSignal(t *testing.T) {
	t.Run("Should handle unknown signal gracefully", func(t *testing.T) {
		testSuite := &testsuite.WorkflowTestSuite{}
		env := testSuite.NewTestWorkflowEnvironment()
		defer func() {
			env.AssertExpectations(t)
		}()

		mockWorkflows := []*wf.Config{
			{
				ID: "order-processor",
				Triggers: []wf.Trigger{
					{Type: wf.TriggerTypeSignal, Name: "order.created"},
				},
			},
		}

		// Create a mock project config with default heartbeat settings
		mockProjectConfig := &project.Config{
			Name: "test-project",
		}

		// Create a GetData activity instance for testing
		getData := &wfacts.GetData{
			ProjectConfig: mockProjectConfig,
			Workflows:     mockWorkflows,
		}

		// Register the activity with the test environment using the correct activity label
		env.RegisterActivityWithOptions(getData.Run, activity.RegisterOptions{Name: wfacts.GetDataLabel})
		env.OnActivity(wfacts.GetDataLabel, mock.Anything, mock.Anything).
			Return(&wfacts.GetData{
				ProjectConfig: mockProjectConfig,
				Workflows:     mockWorkflows,
				AppConfig:     config.Default(),
			}, nil)

		// No expectations for child workflows since unknown signals should not trigger any
		env.RegisterWorkflow(worker.DispatcherWorkflow)
		env.RegisterWorkflow(worker.CompozyWorkflow)

		workflowFinished := make(chan struct{})
		// Execute the workflow in a goroutine to avoid hanging
		go func() {
			defer close(workflowFinished)
			env.ExecuteWorkflow(worker.DispatcherWorkflow, "test-project", "test-server")
		}()

		// Use RegisterDelayedCallback for more reliable timing
		env.RegisterDelayedCallback(func() {
			env.SignalWorkflow(worker.DispatcherEventChannel, worker.EventSignal{
				Name:    "unknown.event",
				Payload: core.Input{},
			})
		}, 10*time.Millisecond)

		// Allow some workflow time to pass to ensure no action is taken
		env.RegisterDelayedCallback(func() {
			// Cancel the workflow to finish the test
			env.CancelWorkflow()
		}, 100*time.Millisecond)

		// Wait for workflow completion
		<-workflowFinished

		// Verify the workflow was canceled and no child workflow was started
		assert.Error(t, env.GetWorkflowError(), "Workflow should be canceled")
		env.AssertExpectations(t)
	})
}

func TestGetRegisteredSignalNames(t *testing.T) {
	t.Run("Should return empty slice for empty signal map", func(t *testing.T) {
		signalMap := make(map[string]*worker.CompiledTrigger)
		names := worker.GetRegisteredSignalNames(signalMap)
		assert.Empty(t, names)
	})

	t.Run("Should return all signal names", func(t *testing.T) {
		signalMap := map[string]*worker.CompiledTrigger{
			"signal1": {Config: &wf.Config{ID: "workflow1"}},
			"signal2": {Config: &wf.Config{ID: "workflow2"}},
			"signal3": {Config: &wf.Config{ID: "workflow3"}},
		}
		names := worker.GetRegisteredSignalNames(signalMap)
		assert.Len(t, names, 3)
		assert.Contains(t, names, "signal1")
		assert.Contains(t, names, "signal2")
		assert.Contains(t, names, "signal3")
	})
}

// TestDispatcherWorkflow_PayloadValidationLogic tests the validation logic without full workflow setup
func TestDispatcherWorkflow_PayloadValidationLogic(t *testing.T) {
	t.Run("Should create compiled triggers with schema", func(t *testing.T) {
		schemaDefinition := &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
			"required": []string{"name"},
		}

		compiled, err := schemaDefinition.Compile()
		assert.NoError(t, err)
		assert.NotNil(t, compiled)

		trigger := &worker.CompiledTrigger{
			Config: &wf.Config{ID: "test-workflow"},
			Trigger: &wf.Trigger{
				Type:   wf.TriggerTypeSignal,
				Name:   "test-signal",
				Schema: schemaDefinition,
			},
			CompiledSchema: compiled,
		}

		assert.Equal(t, "test-workflow", trigger.Config.ID)
		assert.Equal(t, "test-signal", trigger.Trigger.Name)
		assert.NotNil(t, trigger.CompiledSchema)
	})

	t.Run("Should validate payload with compiled schema", func(t *testing.T) {
		schemaDefinition := &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
			"required": []string{"name"},
		}

		compiled, err := schemaDefinition.Compile()
		assert.NoError(t, err)

		// Test valid payload
		validPayload := core.Input{"name": "John"}
		isValid, errors := worker.ValidatePayloadAgainstCompiledSchema(validPayload, compiled)
		assert.True(t, isValid)
		assert.Nil(t, errors)

		// Test invalid payload
		invalidPayload := core.Input{"age": 30} // missing required "name"
		isValid, errors = worker.ValidatePayloadAgainstCompiledSchema(invalidPayload, compiled)
		assert.False(t, isValid)
		assert.NotEmpty(t, errors)
	})
}
