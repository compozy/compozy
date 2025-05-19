package state

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/pkg/logger"
	natspkg "github.com/compozy/compozy/pkg/nats"
	"github.com/nats-io/nats.go"
)

// -----------------------------------------------------------------------------
// Manager Types
// -----------------------------------------------------------------------------

// Manager handles the persistence and retrieval of state changes via events
type Manager struct {
	store      *Store
	natsClient natspkg.Client
	dataDir    string
	streams    []string
}

// ManagerConfig holds configuration for the Manager
type ManagerConfig struct {
	DataDir         string
	NatsClient      natspkg.Client
	StreamsToHandle []string
}

// DefaultManagerConfig provides default configuration values
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		DataDir: "data/state",
		StreamsToHandle: []string{
			string(natspkg.ComponentWorkflow),
			string(natspkg.ComponentTask),
			string(natspkg.ComponentAgent),
			string(natspkg.ComponentTool),
		},
	}
}

// -----------------------------------------------------------------------------
// Manager Creation
// -----------------------------------------------------------------------------

// NewManager creates a new state manager with the given configuration
func NewManager(_ context.Context, config ManagerConfig) (*Manager, error) {
	if config.NatsClient == nil {
		return nil, fmt.Errorf("NATS client is required")
	}

	if config.DataDir == "" {
		config.DataDir = DefaultManagerConfig().DataDir
	}

	if len(config.StreamsToHandle) == 0 {
		config.StreamsToHandle = DefaultManagerConfig().StreamsToHandle
	}

	// Set up the store
	store, err := NewStore(config.DataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create state store: %w", err)
	}

	manager := &Manager{
		store:      store,
		natsClient: config.NatsClient,
		dataDir:    config.DataDir,
		streams:    config.StreamsToHandle,
	}

	return manager, nil
}

// Start initializes the manager and subscribes to the relevant event streams
func (m *Manager) Start(ctx context.Context) error {
	// Subscribe to all state events
	for _, stream := range m.streams {
		if err := m.subscribeToStateEvents(ctx, stream); err != nil {
			return fmt.Errorf("failed to subscribe to %s state events: %w", stream, err)
		}
	}

	return nil
}

// Close cleans up resources used by the manager
func (m *Manager) Close() error {
	if err := m.store.Close(); err != nil {
		return fmt.Errorf("failed to close state store: %w", err)
	}
	return nil
}

// -----------------------------------------------------------------------------
// Event Handling
// -----------------------------------------------------------------------------

// subscribeToStateEvents subscribes to all state events for a specific stream
func (m *Manager) subscribeToStateEvents(_ context.Context, stream string) error {
	// Get JetStream context
	js, err := m.natsClient.JetStreamContext()
	if err != nil {
		return fmt.Errorf("failed to get JetStream context: %w", err)
	}

	// Create a durable consumer for this state manager
	consumerName := fmt.Sprintf("state_manager_%s", stream)
	subj := fmt.Sprintf("compozy.*.%s.events.>", stream)

	// Subscribe with a queue group so multiple instances can load balance
	_, err = js.QueueSubscribe(
		subj,
		"state_managers",
		func(msg *nats.Msg) {
			if err := m.handleStateEvent(context.Background(), msg); err != nil {
				// Use the logger instead of fmt.Printf
				logger.Error("Error handling state event", "error", err, "subject", msg.Subject)
			}
			// Acknowledge the message regardless of processing success
			if err := msg.Ack(); err != nil {
				logger.Error("Error acknowledging message", "error", err, "subject", msg.Subject)
			}
		},
		nats.Durable(consumerName),
		nats.ManualAck(),
	)

	if err != nil {
		return fmt.Errorf("failed to subscribe to %s events: %w", stream, err)
	}

	return nil
}

// handleStateEvent processes a state event and updates the state store
func (m *Manager) handleStateEvent(_ context.Context, msg *nats.Msg) error {
	// Parse the subject to extract component type and ID
	componentType, componentID, correlationID, eventType, err := natspkg.ParseStateEventSubject(msg.Subject)
	if err != nil {
		return fmt.Errorf("failed to parse event subject %s: %w", msg.Subject, err)
	}

	// Create a state ID
	stateID := NewID(componentType, componentID, correlationID)

	// Try to get existing state
	state, err := m.store.GetState(stateID)
	if err != nil {
		// If state doesn't exist, create a new base state
		state = &BaseState{
			ID:      stateID,
			Status:  natspkg.StatusPending, // Default status
			Input:   make(common.Input),
			Output:  make(common.Output),
			Env:     make(common.EnvMap),
			Context: make(map[string]interface{}),
		}
	}

	// Parse the event payload
	if err := state.UpdateFromEvent(natspkg.NewEventData(msg.Subject, msg.Data, eventType)); err != nil {
		return fmt.Errorf("failed to update state from event: %w", err)
	}

	// Save the updated state
	if err := m.store.UpsertState(state); err != nil {
		return fmt.Errorf("failed to save updated state: %w", err)
	}

	return nil
}

// -----------------------------------------------------------------------------
// State Retrieval
// -----------------------------------------------------------------------------

// GetWorkflowState retrieves the state of a workflow
func (m *Manager) GetWorkflowState(workflowID, correlationID string) (State, error) {
	id := NewID(natspkg.ComponentWorkflow, workflowID, correlationID)
	return m.store.GetState(id)
}

// GetTaskState retrieves the state of a task
func (m *Manager) GetTaskState(taskID, correlationID string) (State, error) {
	id := NewID(natspkg.ComponentTask, taskID, correlationID)
	return m.store.GetState(id)
}

// GetAgentState retrieves the state of an agent
func (m *Manager) GetAgentState(agentID, correlationID string) (State, error) {
	id := NewID(natspkg.ComponentAgent, agentID, correlationID)
	return m.store.GetState(id)
}

// GetToolState retrieves the state of a tool
func (m *Manager) GetToolState(toolID, correlationID string) (State, error) {
	id := NewID(natspkg.ComponentTool, toolID, correlationID)
	return m.store.GetState(id)
}

// GetTaskStatesForWorkflow retrieves all task states associated with a workflow
func (m *Manager) GetTaskStatesForWorkflow(workflowID, correlationID string) ([]State, error) {
	id := NewID(natspkg.ComponentWorkflow, workflowID, correlationID)
	return m.store.GetTaskStatesForWorkflow(id)
}

// GetAgentStatesForTask retrieves all agent states associated with a task
func (m *Manager) GetAgentStatesForTask(taskID, correlationID string) ([]State, error) {
	id := NewID(natspkg.ComponentTask, taskID, correlationID)
	return m.store.GetAgentStatesForTask(id)
}

// GetToolStatesForTask retrieves all tool states associated with a task
func (m *Manager) GetToolStatesForTask(taskID, correlationID string) ([]State, error) {
	id := NewID(natspkg.ComponentTask, taskID, correlationID)
	return m.store.GetToolStatesForTask(id)
}

// GetAllWorkflowStates retrieves all workflow states
func (m *Manager) GetAllWorkflowStates() ([]State, error) {
	return m.store.GetStatesByComponent(natspkg.ComponentWorkflow)
}

// GetAllTaskStates retrieves all task states
func (m *Manager) GetAllTaskStates() ([]State, error) {
	return m.store.GetStatesByComponent(natspkg.ComponentTask)
}

// GetAllAgentStates retrieves all agent states
func (m *Manager) GetAllAgentStates() ([]State, error) {
	return m.store.GetStatesByComponent(natspkg.ComponentAgent)
}

// GetAllToolStates retrieves all tool states
func (m *Manager) GetAllToolStates() ([]State, error) {
	return m.store.GetStatesByComponent(natspkg.ComponentTool)
}

// -----------------------------------------------------------------------------
// State Management
// -----------------------------------------------------------------------------

// DeleteWorkflowState deletes a workflow state and all associated task, agent, and tool states
func (m *Manager) DeleteWorkflowState(workflowID, correlationID string) error {
	// Delete the workflow state
	workflowStateID := NewID(natspkg.ComponentWorkflow, workflowID, correlationID)
	if err := m.store.DeleteState(workflowStateID); err != nil {
		return fmt.Errorf("failed to delete workflow state: %w", err)
	}

	// Get and delete all associated task states
	taskStates, err := m.store.GetTaskStatesForWorkflow(workflowStateID)
	if err != nil {
		return fmt.Errorf("failed to get task states for workflow: %w", err)
	}

	for _, taskState := range taskStates {
		taskID := taskState.GetID()

		// Get and delete all associated agent states
		agentStates, err := m.store.GetAgentStatesForTask(taskID)
		if err != nil {
			return fmt.Errorf("failed to get agent states for task: %w", err)
		}

		for _, agentState := range agentStates {
			if err := m.store.DeleteState(agentState.GetID()); err != nil {
				return fmt.Errorf("failed to delete agent state: %w", err)
			}
		}

		// Get and delete all associated tool states
		toolStates, err := m.store.GetToolStatesForTask(taskID)
		if err != nil {
			return fmt.Errorf("failed to get tool states for task: %w", err)
		}

		for _, toolState := range toolStates {
			if err := m.store.DeleteState(toolState.GetID()); err != nil {
				return fmt.Errorf("failed to delete tool state: %w", err)
			}
		}

		// Delete the task state
		if err := m.store.DeleteState(taskID); err != nil {
			return fmt.Errorf("failed to delete task state: %w", err)
		}
	}

	return nil
}
