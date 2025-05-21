package state

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/nats-io/nats.go/jetstream"
)

// -----------------------------------------------------------------------------
// Manager Types
// -----------------------------------------------------------------------------

// Manager handles the persistence and retrieval of state changes via events
type Manager struct {
	store      *Store
	natsClient *nats.Client
	dataDir    string
	components []nats.ComponentType
}

type ManagerOption func(*Manager)

func WithDataDir(dataDir string) ManagerOption {
	return func(m *Manager) {
		m.dataDir = dataDir
	}
}

func WithNatsClient(natsClient *nats.Client) ManagerOption {
	return func(m *Manager) {
		m.natsClient = natsClient
	}
}

func WithComponents(comps []nats.ComponentType) ManagerOption {
	return func(m *Manager) {
		m.components = comps
	}
}

func defaultConsumers() []nats.ComponentType {
	return []nats.ComponentType{
		nats.ComponentWorkflow,
		nats.ComponentTask,
		nats.ComponentAgent,
		nats.ComponentTool,
	}
}

// -----------------------------------------------------------------------------
// Manager Creation
// -----------------------------------------------------------------------------

func NewManager(opts ...ManagerOption) (*Manager, error) {
	manager := &Manager{
		dataDir:    "data/state",
		components: defaultConsumers(),
	}

	for _, opt := range opts {
		opt(manager)
	}

	if manager.natsClient == nil {
		return nil, fmt.Errorf("NATS client is required")
	}

	store, err := NewStore(manager.dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create state store: %w", err)
	}
	manager.store = store
	return manager, nil
}

func (m *Manager) Start(ctx context.Context) error {
	for _, cp := range m.components {
		if err := m.subscribeToStateEvents(ctx, cp); err != nil {
			return fmt.Errorf("failed to subscribe to %s state events: %w", cp, err)
		}
	}

	return nil
}

func (m *Manager) Close() error {
	if err := m.store.Close(); err != nil {
		return fmt.Errorf("failed to close state store: %w", err)
	}
	return nil
}

func (m *Manager) CloseWithContext(ctx context.Context) error {
	if err := m.store.CloseWithContext(ctx); err != nil {
		return fmt.Errorf("failed to close state store: %w", err)
	}
	return nil
}

// -----------------------------------------------------------------------------
// Event Handling
// -----------------------------------------------------------------------------

func (m *Manager) subscribeToStateEvents(ctx context.Context, comp nats.ComponentType) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cs, err := m.natsClient.GetConsumerEvt(ctx, comp, "*")
	if err != nil {
		return fmt.Errorf("failed to get consumer: %w", err)
	}

	subOpts := nats.DefaultSubscribeOpts(cs)
	err = nats.SubscribeConsumer(ctx, m.handleStatus, subOpts)
	if err != nil {
		return fmt.Errorf("failed to subscribe to state events: %w", err)
	}

	return nil
}

func (m *Manager) handleStatus(subject string, data []byte, msg jetstream.Msg) error {
	subj, err := nats.ParseEvtSubject(subject)
	if err != nil {
		return fmt.Errorf("failed to parse event subject %s: %w", subject, err)
	}

	stID := NewID(subj.CompType, subj.CorrID, subj.ExecID)
	state, err := m.store.GetState(stID)
	if err != nil {
		state = NewEmptyState()
	}

	// TODO: implement UpdateStatus on each component state
	evType := subj.EventType.String()
	if err := state.UpdateStatus(nats.NewEventData(subject, data, evType)); err != nil {
		return fmt.Errorf("failed to update state from event: %w", err)
	}
	if err := m.store.UpsertState(state); err != nil {
		return fmt.Errorf("failed to save updated state: %w", err)
	}
	return nil
}

func (m *Manager) SaveState(state State) error {
	if err := m.store.UpsertState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}
	return nil
}

// -----------------------------------------------------------------------------
// State Retrieval
// -----------------------------------------------------------------------------

func (m *Manager) GetWorkflowState(corrID common.CorrID, execID common.ExecID) (State, error) {
	id := NewID(nats.ComponentWorkflow, corrID, execID)
	return m.store.GetState(id)
}

func (m *Manager) GetTaskState(corrID common.CorrID, execID common.ExecID) (State, error) {
	id := NewID(nats.ComponentTask, corrID, execID)
	return m.store.GetState(id)
}

func (m *Manager) GetAgentState(corrID common.CorrID, execID common.ExecID) (State, error) {
	id := NewID(nats.ComponentAgent, corrID, execID)
	return m.store.GetState(id)
}

func (m *Manager) GetToolState(corrID common.CorrID, execID common.ExecID) (State, error) {
	id := NewID(nats.ComponentTool, corrID, execID)
	return m.store.GetState(id)
}

func (m *Manager) GetTaskStatesForWorkflow(corrID common.CorrID, execID common.ExecID) ([]State, error) {
	id := NewID(nats.ComponentWorkflow, corrID, execID)
	return m.store.GetTaskStatesForWorkflow(id)
}

func (m *Manager) GetAgentStatesForTask(corrID common.CorrID, execID common.ExecID) ([]State, error) {
	id := NewID(nats.ComponentTask, corrID, execID)
	return m.store.GetAgentStatesForTask(id)
}

func (m *Manager) GetToolStatesForTask(corrID common.CorrID, execID common.ExecID) ([]State, error) {
	id := NewID(nats.ComponentTask, corrID, execID)
	return m.store.GetToolStatesForTask(id)
}

func (m *Manager) GetAllWorkflowStates() ([]State, error) {
	return m.store.GetStatesByComponent(nats.ComponentWorkflow)
}

func (m *Manager) GetAllTaskStates() ([]State, error) {
	return m.store.GetStatesByComponent(nats.ComponentTask)
}

func (m *Manager) GetAllAgentStates() ([]State, error) {
	return m.store.GetStatesByComponent(nats.ComponentAgent)
}

func (m *Manager) GetAllToolStates() ([]State, error) {
	return m.store.GetStatesByComponent(nats.ComponentTool)
}

// -----------------------------------------------------------------------------
// State Management
// -----------------------------------------------------------------------------

func (m *Manager) DeleteWorkflowState(corrID common.CorrID, execID common.ExecID) error {
	stID := NewID(nats.ComponentWorkflow, corrID, execID)
	if err := m.store.DeleteState(stID); err != nil {
		return fmt.Errorf("failed to delete workflow state: %w", err)
	}

	taskStates, err := m.store.GetTaskStatesForWorkflow(stID)
	if err != nil {
		return fmt.Errorf("failed to get task states for workflow: %w", err)
	}

	for _, taskState := range taskStates {
		tID := taskState.GetID()
		agentStates, err := m.store.GetAgentStatesForTask(tID)
		if err != nil {
			return fmt.Errorf("failed to get agent states for task: %w", err)
		}

		for _, agState := range agentStates {
			if err := m.store.DeleteState(agState.GetID()); err != nil {
				return fmt.Errorf("failed to delete agent state: %w", err)
			}
		}

		toolStates, err := m.store.GetToolStatesForTask(tID)
		if err != nil {
			return fmt.Errorf("failed to get tool states for task: %w", err)
		}

		for _, toolState := range toolStates {
			if err := m.store.DeleteState(toolState.GetID()); err != nil {
				return fmt.Errorf("failed to delete tool state: %w", err)
			}
		}

		if err := m.store.DeleteState(tID); err != nil {
			return fmt.Errorf("failed to delete task state: %w", err)
		}
	}

	return nil
}
