package stmanager

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/domain/agent"
	"github.com/compozy/compozy/engine/domain/task"
	"github.com/compozy/compozy/engine/domain/tool"
	"github.com/compozy/compozy/engine/domain/workflow"
	"github.com/compozy/compozy/engine/state"
	"github.com/compozy/compozy/engine/store"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/nats-io/nats.go/jetstream"
)

type Manager struct {
	store      *store.Store
	natsClient *nats.Client
	components []nats.ComponentType
}

type ManagerOption func(*Manager)

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

func WithStore(store *store.Store) ManagerOption {
	return func(m *Manager) {
		m.store = store
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

func NewManager(opts ...ManagerOption) (*Manager, error) {
	manager := &Manager{
		components: defaultConsumers(),
	}
	for _, opt := range opts {
		opt(manager)
	}
	if manager.natsClient == nil {
		return nil, fmt.Errorf("NATS client is required")
	}
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

func (m *Manager) SaveState(state state.State) error {
	if err := m.store.UpsertState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}
	return nil
}

func (m *Manager) subscribeToStateEvents(ctx context.Context, comp nats.ComponentType) error {
	subCtx := context.Background()
	cs, err := m.natsClient.GetConsumerEvt(ctx, comp, nats.EvtTypeAll)
	if err != nil {
		return fmt.Errorf("failed to get consumer: %w", err)
	}
	subOpts := nats.DefaultSubscribeOpts(cs)
	errCh := nats.SubscribeConsumer(subCtx, m.handleUpdateStatus, subOpts)
	go func() {
		for err := range errCh {
			if err != nil {
				logger.Error("Error subscribing to state events", "error", err)
			}
		}
	}()
	return nil
}

func (m *Manager) handleUpdateStatus(subject string, data []byte, _ jetstream.Msg) error {
	subj, err := nats.ParseEvtSubject(subject)
	if err != nil {
		return fmt.Errorf("failed to parse event subject %s: %w", subject, err)
	}
	event, err := nats.ParseEvent(subj.CompType, subj.EventType, data)
	if err != nil {
		return fmt.Errorf("failed to parse event data: %w", err)
	}
	switch subj.CompType {
	case nats.ComponentWorkflow:
		return m.handleWorkflowStateUpdate(subj, event)
	case nats.ComponentTask:
		return m.handleTaskStateUpdate(subj, event)
	case nats.ComponentAgent:
		return m.handleAgentStateUpdate(subj, event)
	case nats.ComponentTool:
		return m.handleToolStateUpdate(subj, event)
	default:
		err := fmt.Errorf("unsupported component type for state update: %s", subj.CompType)
		return err
	}
}

func (m *Manager) handleWorkflowStateUpdate(subj *nats.EventSubject, event any) error {
	st, err := m.GetWorkflowState(subj.CorrID, subj.ExecID)
	if err != nil {
		return fmt.Errorf("failed to get workflow state: %w", err)
	}
	wfSt, ok := st.(*workflow.State)
	if !ok {
		return fmt.Errorf("failed to cast state to workflow state")
	}
	if err := wfSt.UpdateFromEvent(event); err != nil {
		return fmt.Errorf("failed to update workflow state from event: %w", err)
	}
	if err := m.store.UpsertState(wfSt); err != nil {
		return fmt.Errorf("failed to save updated workflow state: %w", err)
	}
	return nil
}

func (m *Manager) handleTaskStateUpdate(subj *nats.EventSubject, event any) error {
	st, err := m.GetTaskState(subj.CorrID, subj.ExecID)
	if err != nil {
		return fmt.Errorf("failed to get task state: %w", err)
	}
	tSt, ok := st.(*task.State)
	if !ok {
		return fmt.Errorf("failed to cast state to task state")
	}
	if err := tSt.UpdateFromEvent(event); err != nil {
		return fmt.Errorf("failed to update task state from event: %w", err)
	}
	if err := m.store.UpsertState(tSt); err != nil {
		return fmt.Errorf("failed to save updated task state: %w", err)
	}
	return nil
}

func (m *Manager) handleAgentStateUpdate(subj *nats.EventSubject, event any) error {
	st, err := m.GetAgentState(subj.CorrID, subj.ExecID)
	if err != nil {
		return fmt.Errorf("failed to get agent state: %w", err)
	}
	aSt, ok := st.(*agent.State)
	if !ok {
		return fmt.Errorf("failed to cast state to agent state")
	}
	if err := aSt.UpdateFromEvent(event); err != nil {
		return fmt.Errorf("failed to update agent state from event: %w", err)
	}
	if err := m.store.UpsertState(aSt); err != nil {
		return fmt.Errorf("failed to save updated agent state: %w", err)
	}
	return nil
}

func (m *Manager) handleToolStateUpdate(subj *nats.EventSubject, event any) error {
	st, err := m.GetToolState(subj.CorrID, subj.ExecID)
	if err != nil {
		return fmt.Errorf("failed to get tool state: %w", err)
	}
	tSt, ok := st.(*tool.State)
	if !ok {
		return fmt.Errorf("failed to cast state to tool state")
	}
	if err := tSt.UpdateFromEvent(event); err != nil {
		return fmt.Errorf("failed to update tool state from event: %w", err)
	}
	if err := m.store.UpsertState(tSt); err != nil {
		return fmt.Errorf("failed to save updated tool state: %w", err)
	}
	return nil
}
