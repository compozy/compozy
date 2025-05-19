package state

import (
	"context"
	"fmt"

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

	// Subscribe to state events
	_, err = js.Subscribe(
		subj,
		func(msg *nats.Msg) {
			if err := m.handleStatus(msg); err != nil {
				logger.Error("Error handling state event", "error", err, "subject", msg.Subject)
			}
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

// handleStatus processes a state event and updates the state store
func (m *Manager) handleStatus(msg *nats.Msg) error {
	componentType, componentID, correlationID, eventType, err := natspkg.ParseStateEventSubject(msg.Subject)
	if err != nil {
		return fmt.Errorf("failed to parse event subject %s: %w", msg.Subject, err)
	}

	stateID := NewID(componentType, componentID, correlationID)
	state, err := m.store.GetState(stateID)
	if err != nil {
		state = NewEmptyState()
	}

	if err := state.UpdateStatus(natspkg.NewEventData(msg.Subject, msg.Data, eventType)); err != nil {
		return fmt.Errorf("failed to update state from event: %w", err)
	}
	if err := m.store.UpsertState(state); err != nil {
		return fmt.Errorf("failed to save updated state: %w", err)
	}
	return nil
}

// -----------------------------------------------------------------------------
// State Retrieval
// -----------------------------------------------------------------------------

func (m *Manager) GetWorkflowState(workflowID, correlationID string) (State, error) {
	id := NewID(natspkg.ComponentWorkflow, workflowID, correlationID)
	return m.store.GetState(id)
}

func (m *Manager) GetTaskState(taskID, correlationID string) (State, error) {
	id := NewID(natspkg.ComponentTask, taskID, correlationID)
	return m.store.GetState(id)
}

func (m *Manager) GetAgentState(agentID, correlationID string) (State, error) {
	id := NewID(natspkg.ComponentAgent, agentID, correlationID)
	return m.store.GetState(id)
}

func (m *Manager) GetToolState(toolID, correlationID string) (State, error) {
	id := NewID(natspkg.ComponentTool, toolID, correlationID)
	return m.store.GetState(id)
}

func (m *Manager) GetTaskStatesForWorkflow(workflowID, correlationID string) ([]State, error) {
	id := NewID(natspkg.ComponentWorkflow, workflowID, correlationID)
	return m.store.GetTaskStatesForWorkflow(id)
}

func (m *Manager) GetAgentStatesForTask(taskID, correlationID string) ([]State, error) {
	id := NewID(natspkg.ComponentTask, taskID, correlationID)
	return m.store.GetAgentStatesForTask(id)
}

func (m *Manager) GetToolStatesForTask(taskID, correlationID string) ([]State, error) {
	id := NewID(natspkg.ComponentTask, taskID, correlationID)
	return m.store.GetToolStatesForTask(id)
}

func (m *Manager) GetAllWorkflowStates() ([]State, error) {
	return m.store.GetStatesByComponent(natspkg.ComponentWorkflow)
}

func (m *Manager) GetAllTaskStates() ([]State, error) {
	return m.store.GetStatesByComponent(natspkg.ComponentTask)
}

func (m *Manager) GetAllAgentStates() ([]State, error) {
	return m.store.GetStatesByComponent(natspkg.ComponentAgent)
}

func (m *Manager) GetAllToolStates() ([]State, error) {
	return m.store.GetStatesByComponent(natspkg.ComponentTool)
}

// -----------------------------------------------------------------------------
// State Management
// -----------------------------------------------------------------------------

func (m *Manager) DeleteWorkflowState(workflowID, correlationID string) error {
	workflowStateID := NewID(natspkg.ComponentWorkflow, workflowID, correlationID)
	if err := m.store.DeleteState(workflowStateID); err != nil {
		return fmt.Errorf("failed to delete workflow state: %w", err)
	}

	taskStates, err := m.store.GetTaskStatesForWorkflow(workflowStateID)
	if err != nil {
		return fmt.Errorf("failed to get task states for workflow: %w", err)
	}

	for _, taskState := range taskStates {
		taskID := taskState.GetID()
		agentStates, err := m.store.GetAgentStatesForTask(taskID)
		if err != nil {
			return fmt.Errorf("failed to get agent states for task: %w", err)
		}

		for _, agentState := range agentStates {
			if err := m.store.DeleteState(agentState.GetID()); err != nil {
				return fmt.Errorf("failed to delete agent state: %w", err)
			}
		}

		toolStates, err := m.store.GetToolStatesForTask(taskID)
		if err != nil {
			return fmt.Errorf("failed to get tool states for task: %w", err)
		}

		for _, toolState := range toolStates {
			if err := m.store.DeleteState(toolState.GetID()); err != nil {
				return fmt.Errorf("failed to delete tool state: %w", err)
			}
		}

		if err := m.store.DeleteState(taskID); err != nil {
			return fmt.Errorf("failed to delete task state: %w", err)
		}
	}

	return nil
}
