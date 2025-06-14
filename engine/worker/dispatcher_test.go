package worker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	wf "github.com/compozy/compozy/engine/workflow"
	wfacts "github.com/compozy/compozy/engine/workflow/activities"
)

func TestEventSignal_Structure(t *testing.T) {
	t.Run("Should create EventSignal correctly", func(t *testing.T) {
		signal := EventSignal{
			Name:          "order.created",
			Payload:       core.Input{"orderId": "123"},
			CorrelationID: "test-correlation-id",
		}
		assert.Equal(t, "order.created", signal.Name)
		assert.Equal(t, "123", signal.Payload["orderId"])
	})

	t.Run("Should handle empty payload", func(t *testing.T) {
		signal := EventSignal{
			Name:    "test.event",
			Payload: core.Input{},
		}
		assert.Equal(t, "test.event", signal.Name)
		assert.NotNil(t, signal.Payload)
		assert.Len(t, signal.Payload, 0)
	})
}

type DispatcherWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

// Workflow tests are temporarily disabled due to complex Temporal test setup requirements
// The dispatcher workflow functionality is tested through integration tests
// func TestDispatcherWorkflow(t *testing.T) {
// 	suite.Run(t, new(DispatcherWorkflowTestSuite))
// }

func (s *DispatcherWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *DispatcherWorkflowTestSuite) AfterTest(_, _ string) {
	s.env.AssertExpectations(s.T())
}

func (s *DispatcherWorkflowTestSuite) TestSuccessfulDispatch() {
	s.T().Run("Should dispatch event to matching workflow", func(_ *testing.T) {
		mockWorkflows := []*wf.Config{
			{
				ID: "order-processor",
				Triggers: []wf.Trigger{
					{Type: wf.TriggerTypeSignal, Name: "order.created"},
				},
			},
		}
		s.env.OnActivity("GetWorkflowData", mock.Anything).Return(&wfacts.GetData{Workflows: mockWorkflows}, nil)
		s.env.OnWorkflow("CompozyWorkflow", mock.Anything).Return(nil, nil)
		s.env.RegisterWorkflow(DispatcherWorkflow)
		go s.env.ExecuteWorkflow(DispatcherWorkflow, "test-project")
		time.Sleep(50 * time.Millisecond) // Allow workflow to start
		s.env.SignalWorkflow(DispatcherEventChannel, EventSignal{
			Name:          "order.created",
			Payload:       core.Input{"orderId": "123"},
			CorrelationID: "test-correlation-id",
		})
		time.Sleep(100 * time.Millisecond)
		s.env.AssertExpectations(s.T())
	})
}

func (s *DispatcherWorkflowTestSuite) TestUnknownSignal() {
	s.T().Run("Should handle unknown signal gracefully", func(_ *testing.T) {
		mockWorkflows := []*wf.Config{
			{
				ID: "order-processor",
				Triggers: []wf.Trigger{
					{Type: wf.TriggerTypeSignal, Name: "order.created"},
				},
			},
		}
		s.env.OnActivity("GetWorkflowData", mock.Anything).Return(&wfacts.GetData{Workflows: mockWorkflows}, nil)
		s.env.RegisterWorkflow(DispatcherWorkflow)
		go s.env.ExecuteWorkflow(DispatcherWorkflow, "test-project")
		time.Sleep(50 * time.Millisecond) // Allow workflow to start
		s.env.SignalWorkflow(DispatcherEventChannel, EventSignal{
			Name:    "unknown.event",
			Payload: core.Input{},
		})
		time.Sleep(100 * time.Millisecond)
		s.env.AssertExpectations(s.T())
	})
}

func TestGetRegisteredSignalNames(t *testing.T) {
	t.Run("Should return empty slice for empty signal map", func(t *testing.T) {
		signalMap := make(map[string]*compiledTrigger)
		names := getRegisteredSignalNames(signalMap)
		assert.Empty(t, names)
	})

	t.Run("Should return all signal names", func(t *testing.T) {
		signalMap := map[string]*compiledTrigger{
			"signal1": {config: &wf.Config{ID: "workflow1"}},
			"signal2": {config: &wf.Config{ID: "workflow2"}},
			"signal3": {config: &wf.Config{ID: "workflow3"}},
		}
		names := getRegisteredSignalNames(signalMap)
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

		trigger := &compiledTrigger{
			config: &wf.Config{ID: "test-workflow"},
			trigger: &wf.Trigger{
				Type:   wf.TriggerTypeSignal,
				Name:   "test-signal",
				Schema: schemaDefinition,
			},
			compiledSchema: compiled,
		}

		assert.Equal(t, "test-workflow", trigger.config.ID)
		assert.Equal(t, "test-signal", trigger.trigger.Name)
		assert.NotNil(t, trigger.compiledSchema)
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
		isValid, errors := validatePayloadAgainstCompiledSchema(validPayload, compiled)
		assert.True(t, isValid)
		assert.Nil(t, errors)

		// Test invalid payload
		invalidPayload := core.Input{"age": 30} // missing required "name"
		isValid, errors = validatePayloadAgainstCompiledSchema(invalidPayload, compiled)
		assert.False(t, isValid)
		assert.NotEmpty(t, errors)
	})
}
