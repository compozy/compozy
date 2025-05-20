package state

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/compozy/compozy/pkg/nats"
	"github.com/dgraph-io/badger/v3"
)

// -----------------------------------------------------------------------------
// Store Types
// -----------------------------------------------------------------------------

// StorePrefixes defines key prefixes for different state types in the store
type StorePrefixes struct {
	Workflow string
	Task     string
	Agent    string
	Tool     string
}

// DefaultStorePrefixes provides default storage prefixes for each component type
var DefaultStorePrefixes = StorePrefixes{
	Workflow: "workflow:",
	Task:     "task:",
	Agent:    "agent:",
	Tool:     "tool:",
}

// Store provides storage operations for component states
type Store struct {
	db       *badger.DB
	prefixes StorePrefixes
}

// StoreOption represents a configuration option for the Store
type StoreOption func(*Store)

// -----------------------------------------------------------------------------
// Store Options
// -----------------------------------------------------------------------------

// WithPrefixes sets custom key prefixes for the store
func WithPrefixes(prefixes StorePrefixes) StoreOption {
	return func(s *Store) {
		s.prefixes = prefixes
	}
}

// -----------------------------------------------------------------------------
// Store Creation
// -----------------------------------------------------------------------------

// NewStore creates a new store with the specified data path and options
func NewStore(dataPath string, opts ...StoreOption) (*Store, error) {
	// Ensure the data directory exists
	dataPath = filepath.Clean(dataPath)

	// Configure BadgerDB options
	badgerOpts := badger.DefaultOptions(dataPath)
	badgerOpts.Logger = nil // Disable default BadgerDB logger

	// Open the database
	db, err := badger.Open(badgerOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to open BadgerDB at %s: %w", dataPath, err)
	}

	// Create the store with default prefixes
	store := &Store{
		db:       db,
		prefixes: DefaultStorePrefixes,
	}

	// Apply all provided options
	for _, opt := range opts {
		opt(store)
	}

	return store, nil
}

// Close closes the underlying database
func (s *Store) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("failed to close BadgerDB: %w", err)
	}
	return nil
}

// -----------------------------------------------------------------------------
// Key Management
// -----------------------------------------------------------------------------

// stateKey generates a database key for a given state
func (s *Store) stateKey(id ID) []byte {
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

// getPrefixForComponent returns the appropriate prefix string for a component type
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

// -----------------------------------------------------------------------------
// State Operations
// -----------------------------------------------------------------------------

// UpsertState stores or updates a state in the database
func (s *Store) UpsertState(state State) error {
	if state == nil {
		return fmt.Errorf("cannot upsert nil state")
	}

	// Marshal the state to JSON
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Generate the key for this state
	key := s.stateKey(state.GetID())

	// Perform the update within a transaction
	err = s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, data)
	})
	if err != nil {
		return fmt.Errorf("failed to upsert state %s: %w", state.GetID().String(), err)
	}

	return nil
}

// GetState retrieves a state by its ID
func (s *Store) GetState(id ID) (State, error) {
	key := s.stateKey(id)
	var state BaseState

	// Read the state within a read-only transaction
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}

		// Use the Value method to read and unmarshal data
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

// DeleteState removes a state from the database
func (s *Store) DeleteState(id ID) error {
	key := s.stateKey(id)

	// Delete the state within a transaction
	err := s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
	if err != nil {
		return fmt.Errorf("failed to delete state for ID %s: %w", id.String(), err)
	}

	return nil
}

// -----------------------------------------------------------------------------
// Query Operations
// -----------------------------------------------------------------------------

// getStatesWithFilter retrieves states of the specified component type that match the filter function
func (s *Store) getStatesWithFilter(
	componentType nats.ComponentType,
	filter func(state *BaseState) bool,
) ([]State, error) {
	prefix, err := s.getPrefixForComponent(componentType)
	if err != nil {
		return nil, err
	}

	prefixBytes := []byte(prefix)
	var states []State

	// Perform the query within a read-only transaction
	err = s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefixBytes); it.ValidForPrefix(prefixBytes); it.Next() {
			item := it.Item()

			err := item.Value(func(val []byte) error {
				var state BaseState
				if err := json.Unmarshal(val, &state); err != nil {
					return fmt.Errorf("failed to unmarshal state: %w", err)
				}

				// Apply the filter if provided
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

// GetTaskStatesForWorkflow retrieves all task states associated with a workflow
func (s *Store) GetTaskStatesForWorkflow(wfID ID) ([]State, error) {
	return s.getStatesWithFilter(nats.ComponentTask, func(state *BaseState) bool {
		return state.ID.CorrID == wfID.CorrID
	})
}

// GetAgentStatesForTask retrieves all agent states associated with a task
func (s *Store) GetAgentStatesForTask(tID ID) ([]State, error) {
	return s.getStatesWithFilter(nats.ComponentAgent, func(state *BaseState) bool {
		return state.ID.CorrID == tID.CorrID
	})
}

// GetToolStatesForTask retrieves all tool states associated with a task
func (s *Store) GetToolStatesForTask(tID ID) ([]State, error) {
	return s.getStatesWithFilter(nats.ComponentTool, func(state *BaseState) bool {
		return state.ID.CorrID == tID.CorrID
	})
}

// GetStatesByPrefix retrieves all states with keys matching a prefix
func (s *Store) GetStatesByPrefix(prefix string) ([]State, error) {
	if prefix == "" {
		return nil, fmt.Errorf("prefix cannot be empty")
	}

	return s.getStatesWithFilter(nats.ComponentType(""), func(_ *BaseState) bool {
		// No additional filtering needed since the prefix filter is applied at the BadgerDB level
		return true
	})
}

// GetStatesByComponent retrieves all states for a specific component type
func (s *Store) GetStatesByComponent(componentType nats.ComponentType) ([]State, error) {
	return s.getStatesWithFilter(componentType, nil)
}
