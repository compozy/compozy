package store

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/compozy/compozy/engine/domain/agent"
	"github.com/compozy/compozy/engine/domain/task"
	"github.com/compozy/compozy/engine/domain/tool"
	"github.com/compozy/compozy/engine/domain/workflow"
	"github.com/compozy/compozy/engine/state"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/dgraph-io/badger/v3"
)

type Store struct {
	db      *badger.DB
	dataDir string
}

type Option func(*Store)

func NewStore(dataPath string, opts ...Option) (*Store, error) {
	dataPath = filepath.Clean(dataPath)
	badgerOpts := badger.DefaultOptions(dataPath)
	badgerOpts.Logger = nil // Disable default BadgerDB logger
	db, err := badger.Open(badgerOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to open BadgerDB at %s: %w", dataPath, err)
	}
	store := &Store{
		db:      db,
		dataDir: dataPath,
	}
	for _, opt := range opts {
		opt(store)
	}
	return store, nil
}

func (s *Store) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("failed to close BadgerDB: %w", err)
	}
	return nil
}

func (s *Store) CloseWithContext(ctx context.Context) error {
	done := make(chan error, 1)

	go func() {
		if err := s.db.Close(); err != nil {
			done <- fmt.Errorf("failed to close BadgerDB: %w", err)
			return
		}
		done <- nil
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("context canceled while closing BadgerDB: %w", ctx.Err())
	}
}

func (s *Store) DataDir() string {
	return s.dataDir
}

func (s *Store) stateKey(id state.ID) []byte {
	return fmt.Appendf(nil, "%s", id.String())
}

func (s *Store) getStateType(id state.ID) (any, error) {
	switch id.Component {
	case nats.ComponentWorkflow:
		return &workflow.State{}, nil
	case nats.ComponentTask:
		return &task.State{}, nil
	case nats.ComponentAgent:
		return &agent.State{}, nil
	case nats.ComponentTool:
		return &tool.State{}, nil
	default:
		return nil, fmt.Errorf("unknown component type: %v", id.Component)
	}
}

func (s *Store) UpsertState(state state.State) error {
	if state == nil {
		return fmt.Errorf("cannot upsert nil state")
	}
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}
	key := s.stateKey(state.GetID())
	err = s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, data)
	})
	if err != nil {
		return fmt.Errorf("failed to upsert state %s: %w", state.GetID().String(), err)
	}
	return nil
}

func (s *Store) GetState(id state.ID) (state.State, error) {
	key := s.stateKey(id)
	concreteState, err := s.getStateType(id)
	if err != nil {
		return nil, err
	}
	err = s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, concreteState)
		})
	})
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, fmt.Errorf("state not found for ID %s", id.String())
		}
		return nil, fmt.Errorf("failed to get state for ID %s: %w", id.String(), err)
	}
	switch state := concreteState.(type) {
	case *workflow.State:
		return state, nil
	case *task.State:
		return state, nil
	case *agent.State:
		return state, nil
	case *tool.State:
		return state, nil
	default:
		return nil, fmt.Errorf("unknown state type for ID %s", id.String())
	}
}

func (s *Store) DeleteState(id state.ID) error {
	key := s.stateKey(id)
	err := s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
	if err != nil {
		return fmt.Errorf("failed to delete state for ID %s: %w", id.String(), err)
	}
	return nil
}

type StateFilter func(state state.State) bool

func (s *Store) getStatesWithFilter(cType nats.ComponentType, ft StateFilter) ([]state.State, error) {
	prefixBytes := []byte(cType)
	var states []state.State
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(prefixBytes); it.ValidForPrefix(prefixBytes); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				id, err := state.IDFromString(string(item.Key()))
				if err != nil {
					return fmt.Errorf("failed to parse state ID from key: %w", err)
				}
				concreteState, err := s.getStateType(id)
				if err != nil {
					return err
				}
				if err := json.Unmarshal(val, concreteState); err != nil {
					return fmt.Errorf("failed to unmarshal state: %w", err)
				}
				var stateInterface state.State
				switch state := concreteState.(type) {
				case *workflow.State:
					stateInterface = state
				case *task.State:
					stateInterface = state
				case *agent.State:
					stateInterface = state
				case *tool.State:
					stateInterface = state
				default:
					return fmt.Errorf("unknown state type for ID %s", id.String())
				}
				if ft == nil || ft(stateInterface) {
					states = append(states, stateInterface)
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query states: %w", err)
	}
	return states, nil
}

func (s *Store) GetTaskStatesForWorkflow(workflowID state.ID) ([]state.State, error) {
	return s.getStatesWithFilter(nats.ComponentTask, func(state state.State) bool {
		return state.GetCorrelationID() == workflowID.CorrID
	})
}

func (s *Store) GetAgentStatesForTask(taskID state.ID) ([]state.State, error) {
	return s.getStatesWithFilter(nats.ComponentAgent, func(state state.State) bool {
		return state.GetCorrelationID() == taskID.CorrID
	})
}

func (s *Store) GetToolStatesForTask(taskID state.ID) ([]state.State, error) {
	return s.getStatesWithFilter(nats.ComponentTool, func(state state.State) bool {
		return state.GetCorrelationID() == taskID.CorrID
	})
}

func (s *Store) GetStatesByComponent(componentType nats.ComponentType) ([]state.State, error) {
	return s.getStatesWithFilter(componentType, nil)
}
