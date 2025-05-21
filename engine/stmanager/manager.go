package stmanager

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/domain/project"
	"github.com/compozy/compozy/engine/domain/workflow"
	"github.com/compozy/compozy/engine/state"
	"github.com/compozy/compozy/engine/store"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/nats-io/nats.go/jetstream"
)

// -----------------------------------------------------------------------------
// Manager Types
// -----------------------------------------------------------------------------

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

// -----------------------------------------------------------------------------
// Manager Creation
// -----------------------------------------------------------------------------

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

// -----------------------------------------------------------------------------
// Event Handling
// -----------------------------------------------------------------------------

func (m *Manager) subscribeToStateEvents(ctx context.Context, comp nats.ComponentType) error {
	subCtx := context.Background()
	cs, err := m.natsClient.GetConsumerEvt(ctx, comp, nats.EvtTypeAll)
	if err != nil {
		return fmt.Errorf("failed to get consumer: %w", err)
	}

	subOpts := nats.DefaultSubscribeOpts(cs)
	errCh := nats.SubscribeConsumer(subCtx, m.handleUpdateStatus, subOpts)

	// Monitor for errors in a separate goroutine
	go func() {
		for err := range errCh {
			if err != nil {
				fmt.Printf("Error in %s state event subscription: %v\n", comp, err)
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

	stID := state.NewID(subj.CompType, subj.CorrID, subj.ExecID)
	st, err := m.store.GetState(stID)
	if err != nil {
		st = state.NewEmptyState()
	}

	event, err := nats.ParseEvent(subj.CompType, subj.EventType, data)
	if err != nil {
		return fmt.Errorf("failed to parse event data: %w", err)
	}

	if err := st.UpdateFromEvent(event); err != nil {
		return fmt.Errorf("failed to update state from event: %w", err)
	}
	if err := m.store.UpsertState(st); err != nil {
		return fmt.Errorf("failed to save updated state: %w", err)
	}
	return nil
}

// -----------------------------------------------------------------------------
// Create
// -----------------------------------------------------------------------------

func (m *Manager) CreateWorkflowState(
	tgInput *common.Input,
	pj *project.Config,
) (*workflow.Execution, *workflow.State, error) {
	exec, err := workflow.NewExecution(tgInput, pj.GetEnv())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create execution: %w", err)
	}
	st, err := workflow.NewState(exec)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create state: %w", err)
	}
	if err := m.SaveState(st); err != nil {
		return nil, nil, fmt.Errorf("failed to save state: %w", err)
	}
	return exec, st, nil
}
