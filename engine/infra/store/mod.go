package store

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/dgraph-io/badger/v3"
)

// Option configures the Store during initialization.
type Option func(*Store)

// WithLogger sets a custom logger for BadgerDB.
func WithLogger(logger badger.Logger) Option {
	return func(s *Store) {
		s.logger = logger
	}
}

// Store is a BadgerDB-backed key-value store with JSON support.
type Store struct {
	db      *badger.DB
	dataDir string
	logger  badger.Logger
	closed  bool
	mu      sync.Mutex
}

// NewStore creates a new Store instance at the given dataPath.
func NewStore(dataPath string, opts ...Option) (*Store, error) {
	dataPath = filepath.Clean(dataPath)
	badgerOpts := badger.DefaultOptions(dataPath)
	badgerOpts.Logger = nil // Default to no logger
	store := &Store{
		dataDir: dataPath,
		logger:  nil,
	}
	for _, opt := range opts {
		opt(store)
	}
	badgerOpts.Logger = store.logger
	db, err := badger.Open(badgerOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to open BadgerDB at %s: %w", dataPath, err)
	}
	store.db = db
	return store, nil
}

func (s *Store) GetDB() *badger.DB {
	return s.db
}

// SaveJSON serializes and saves an object as JSON.
func (s *Store) SaveJSON(ctx context.Context, key []byte, obj any) error {
	if err := s.validateKey(key); err != nil {
		return err
	}
	value, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal object: %w", err)
	}
	return s.Save(ctx, key, value)
}

// Save stores a key-value pair, failing if the key already exists.
func (s *Store) Save(ctx context.Context, key, value []byte) error {
	if err := s.validateKeyValue(key, value); err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := txn.Get(key); err == nil {
			return fmt.Errorf("key %s already exists", string(key))
		} else if err != badger.ErrKeyNotFound {
			return fmt.Errorf("failed to check key existence: %w", err)
		}
		return txn.Set(key, value)
	})
}

// Get retrieves a value by key.
func (s *Store) Get(ctx context.Context, key []byte) ([]byte, error) {
	if err := s.validateKey(key); err != nil {
		return nil, err
	}
	var value []byte
	err := s.db.View(func(txn *badger.Txn) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		item, err := txn.Get(key)
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return fmt.Errorf("key %s not found", string(key))
			}
			return fmt.Errorf("failed to read key: %w", err)
		}
		value, err = item.ValueCopy(nil)
		return err
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

// UpdateJSON serializes and updates an existing key with a JSON object.
func (s *Store) UpdateJSON(ctx context.Context, key []byte, obj any) error {
	if err := s.validateKey(key); err != nil {
		return err
	}
	value, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal object: %w", err)
	}
	return s.Update(ctx, key, value)
}

// Update updates an existing key-value pair.
func (s *Store) Update(ctx context.Context, key, value []byte) error {
	if err := s.validateKeyValue(key, value); err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := txn.Get(key); err != nil {
			if err == badger.ErrKeyNotFound {
				return fmt.Errorf("key %s not found", string(key))
			}
			return fmt.Errorf("failed to check key existence: %w", err)
		}
		return txn.Set(key, value)
	})
}

// UpsertJSON serializes and upserts a JSON object.
func (s *Store) UpsertJSON(ctx context.Context, key []byte, obj any) error {
	if err := s.validateKey(key); err != nil {
		return err
	}
	value, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal object: %w", err)
	}
	return s.Upsert(ctx, key, value)
}

// Upsert stores or updates a key-value pair.
func (s *Store) Upsert(ctx context.Context, key, value []byte) error {
	if err := s.validateKeyValue(key, value); err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return txn.Set(key, value)
	})
}

// Delete removes a key-value pair.
func (s *Store) Delete(ctx context.Context, key []byte) error {
	if err := s.validateKey(key); err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := txn.Get(key); err != nil {
			if err == badger.ErrKeyNotFound {
				return fmt.Errorf("key %s not found", string(key))
			}
			return fmt.Errorf("failed to check key existence: %w", err)
		}
		return txn.Delete(key)
	})
}

// Close shuts down the store.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("failed to close BadgerDB: %w", err)
	}
	s.closed = true
	return nil
}

// CloseWithContext shuts down the store with context support.
func (s *Store) CloseWithContext(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled: %w", err)
	}
	return s.Close()
}

// DataDir returns the store's data directory.
func (s *Store) DataDir() string {
	return s.dataDir
}

// validateKey ensures the key is non-empty.
func (s *Store) validateKey(key []byte) error {
	if len(key) == 0 {
		return fmt.Errorf("key cannot be empty")
	}
	return nil
}

func (s *Store) validateKeyValue(key, value []byte) error {
	if err := s.validateKey(key); err != nil {
		return err
	}
	if len(value) == 0 {
		return fmt.Errorf("value cannot be empty")
	}
	return nil
}

// GetExecutionsByFilter retrieves executions matching a prefix and filter.
func GetExecutionsByFilter[T any](
	ctx context.Context,
	db *badger.DB,
	prefix []byte,
	filter func(execution T) bool,
) ([]T, error) {
	executions := make([]T, 0) // TODO: This could be optimized to pre-allocate capacity
	err := db.View(func(txn *badger.Txn) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		opts.PrefetchSize = 100
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			var execution T
			err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &execution)
			})
			if err != nil {
				return fmt.Errorf("failed to unmarshal execution: %w", err)
			}
			if filter == nil || filter(execution) {
				executions = append(executions, execution)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query executions: %w", err)
	}
	return executions, nil
}

func (s *Store) NewWorkflowRepository(projectConfig *project.Config, workflows []*workflow.Config) *WorkflowRepository {
	return NewWorkflowRepository(s, projectConfig, workflows)
}

func (s *Store) NewTaskRepository(projectConfig *project.Config, workflows []*workflow.Config) *TaskRepository {
	workflowRepo := s.NewWorkflowRepository(projectConfig, workflows)
	return NewTaskRepository(s, workflowRepo)
}

func (s *Store) NewAgentRepository(projectConfig *project.Config, workflows []*workflow.Config) *AgentRepository {
	workflowRepo := s.NewWorkflowRepository(projectConfig, workflows)
	taskRepo := s.NewTaskRepository(projectConfig, workflows)
	return NewAgentRepository(s, workflowRepo, taskRepo)
}

func (s *Store) NewToolRepository(projectConfig *project.Config, workflows []*workflow.Config) *ToolRepository {
	workflowRepo := s.NewWorkflowRepository(projectConfig, workflows)
	taskRepo := s.NewTaskRepository(projectConfig, workflows)
	return NewToolRepository(s, workflowRepo, taskRepo)
}

func GetAndUnmarshalJSON[T any](ctx context.Context, s *Store, key []byte) (*T, error) {
	value, err := s.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	var obj T
	if err := json.Unmarshal(value, &obj); err != nil {
		return nil, fmt.Errorf("failed to unmarshal object: %w", err)
	}
	return &obj, nil
}
