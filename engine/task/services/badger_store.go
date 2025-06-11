package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/dgraph-io/badger/v4"

	"github.com/compozy/compozy/engine/task"
)

const (
	// DefaultConfigStoreDir is the default directory for storing task configs
	DefaultConfigStoreDir = ".compozy/cache/task_configs"
)

// badgerConfigStore implements ConfigStore using BadgerDB for persistence
type badgerConfigStore struct {
	db  *badger.DB
	dir string
}

// NewBadgerConfigStore creates a new BadgerDB-backed config store
func NewBadgerConfigStore(dir string) (ConfigStore, error) {
	if dir == "" {
		dir = DefaultConfigStoreDir
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config store directory %s: %w", dir, err)
	}

	// Open BadgerDB with sync writes for durability
	opts := badger.DefaultOptions(dir).
		WithSyncWrites(true).
		WithLogger(nil) // Disable BadgerDB's internal logging to avoid noise

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open BadgerDB at %s: %w", dir, err)
	}

	return &badgerConfigStore{
		db:  db,
		dir: dir,
	}, nil
}

// Save persists a task configuration with the given taskExecID as key
func (s *badgerConfigStore) Save(_ context.Context, taskExecID string, config *task.Config) error {
	if taskExecID == "" {
		return fmt.Errorf("taskExecID cannot be empty")
	}
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	// Marshal config to JSON
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config for taskExecID %s: %w", taskExecID, err)
	}

	// Save to BadgerDB
	err = s.db.Update(func(txn *badger.Txn) error {
		key := []byte(taskExecID)
		return txn.Set(key, data)
	})

	if err != nil {
		return fmt.Errorf("failed to save config for taskExecID %s: %w", taskExecID, err)
	}

	return nil
}

// Get retrieves a task configuration by taskExecID
func (s *badgerConfigStore) Get(_ context.Context, taskExecID string) (*task.Config, error) {
	if taskExecID == "" {
		return nil, fmt.Errorf("taskExecID cannot be empty")
	}

	var config *task.Config
	err := s.db.View(func(txn *badger.Txn) error {
		key := []byte(taskExecID)
		item, err := txn.Get(key)
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			config = &task.Config{}
			return json.Unmarshal(val, config)
		})
	})

	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, fmt.Errorf("config not found for taskExecID %s", taskExecID)
		}
		return nil, fmt.Errorf("failed to get config for taskExecID %s: %w", taskExecID, err)
	}

	return config, nil
}

// Delete removes a task configuration by taskExecID
func (s *badgerConfigStore) Delete(_ context.Context, taskExecID string) error {
	if taskExecID == "" {
		return fmt.Errorf("taskExecID cannot be empty")
	}

	err := s.db.Update(func(txn *badger.Txn) error {
		key := []byte(taskExecID)
		return txn.Delete(key)
	})

	if err != nil && err != badger.ErrKeyNotFound {
		return fmt.Errorf("failed to delete config for taskExecID %s: %w", taskExecID, err)
	}

	return nil
}

// Close closes the underlying BadgerDB and releases resources
func (s *badgerConfigStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// GetStoreDir returns the directory where the store is located
func (s *badgerConfigStore) GetStoreDir() string {
	return s.dir
}
