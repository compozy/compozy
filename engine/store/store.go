package store

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/compozy/compozy/engine/state"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/dgraph-io/badger/v3"
)

type Prefixes struct {
	Workflow string
	Task     string
	Agent    string
	Tool     string
}

var DefaultPrefixes = Prefixes{
	Workflow: "workflow:",
	Task:     "task:",
	Agent:    "agent:",
	Tool:     "tool:",
}

type Store struct {
	db       *badger.DB
	prefixes Prefixes
	dataDir  string
}

type Option func(*Store)

func WithPrefixes(prefixes Prefixes) Option {
	return func(s *Store) {
		s.prefixes = prefixes
	}
}

func NewStore(dataPath string, opts ...Option) (*Store, error) {
	dataPath = filepath.Clean(dataPath)

	badgerOpts := badger.DefaultOptions(dataPath)
	badgerOpts.Logger = nil // Disable default BadgerDB logger

	db, err := badger.Open(badgerOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to open BadgerDB at %s: %w", dataPath, err)
	}

	store := &Store{
		db:       db,
		prefixes: DefaultPrefixes,
		dataDir:  dataPath,
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
	var prefix string

	switch id.Component {
	case nats.ComponentWorkflow:
		prefix = s.prefixes.Workflow
	case nats.ComponentTask:
		prefix = s.prefixes.Task
	case nats.ComponentAgent:
		prefix = s.prefixes.Agent
	case nats.ComponentTool:
		prefix = s.prefixes.Tool
	default:
		prefix = "unknown:"
	}

	return fmt.Appendf(nil, "%s%s", prefix, id.String())
}

func (s *Store) getPrefixForComponent(componentType nats.ComponentType) (string, error) {
	switch componentType {
	case nats.ComponentWorkflow:
		return s.prefixes.Workflow, nil
	case nats.ComponentTask:
		return s.prefixes.Task, nil
	case nats.ComponentAgent:
		return s.prefixes.Agent, nil
	case nats.ComponentTool:
		return s.prefixes.Tool, nil
	default:
		return "", fmt.Errorf("unknown component type: %v", componentType)
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
	var state state.BaseState

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &state)
		})
	})
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, fmt.Errorf("state not found for ID %s", id.String())
		}
		return nil, fmt.Errorf("failed to get state for ID %s: %w", id.String(), err)
	}

	return &state, nil
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

func (s *Store) getStatesWithFilter(
	componentType nats.ComponentType,
	filter func(state *state.BaseState) bool,
) ([]state.State, error) {
	prefix, err := s.getPrefixForComponent(componentType)
	if err != nil {
		return nil, err
	}

	prefixBytes := []byte(prefix)
	var states []state.State

	err = s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefixBytes); it.ValidForPrefix(prefixBytes); it.Next() {
			item := it.Item()

			err := item.Value(func(val []byte) error {
				var state state.BaseState
				if err := json.Unmarshal(val, &state); err != nil {
					return fmt.Errorf("failed to unmarshal state: %w", err)
				}

				if filter == nil || filter(&state) {
					states = append(states, &state)
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

func (s *Store) GetTaskStatesForWorkflow(wfID state.ID) ([]state.State, error) {
	return s.getStatesWithFilter(nats.ComponentTask, func(state *state.BaseState) bool {
		return state.StateID.CorrID == wfID.CorrID
	})
}

func (s *Store) GetAgentStatesForTask(tID state.ID) ([]state.State, error) {
	return s.getStatesWithFilter(nats.ComponentAgent, func(state *state.BaseState) bool {
		return state.StateID.CorrID == tID.CorrID
	})
}

func (s *Store) GetToolStatesForTask(tID state.ID) ([]state.State, error) {
	return s.getStatesWithFilter(nats.ComponentTool, func(state *state.BaseState) bool {
		return state.StateID.CorrID == tID.CorrID
	})
}

func (s *Store) GetStatesByPrefix(prefix string) ([]state.State, error) {
	if prefix == "" {
		return nil, fmt.Errorf("prefix cannot be empty")
	}

	return s.getStatesWithFilter(nats.ComponentType(""), func(_ *state.BaseState) bool {
		return true
	})
}

func (s *Store) GetStatesByComponent(componentType nats.ComponentType) ([]state.State, error) {
	return s.getStatesWithFilter(componentType, nil)
}
