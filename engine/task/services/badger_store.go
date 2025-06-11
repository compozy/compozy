package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
		// Try XDG_CACHE_HOME first, then user home directory, fallback to CWD
		if cacheHome := os.Getenv("XDG_CACHE_HOME"); cacheHome != "" {
			dir = filepath.Join(cacheHome, "compozy", "task_configs")
		} else if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, DefaultConfigStoreDir)
		} else {
			dir = DefaultConfigStoreDir
		}
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
func (s *badgerConfigStore) Save(ctx context.Context, taskExecID string, config *task.Config) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled: %w", err)
	}

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
func (s *badgerConfigStore) Get(ctx context.Context, taskExecID string) (*task.Config, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context canceled: %w", err)
	}

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
func (s *badgerConfigStore) Delete(ctx context.Context, taskExecID string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled: %w", err)
	}

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

// SaveMetadata persists arbitrary metadata with the given key
func (s *badgerConfigStore) SaveMetadata(ctx context.Context, key string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled: %w", err)
	}

	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}
	if data == nil {
		return fmt.Errorf("data cannot be nil")
	}

	// Save to BadgerDB with metadata prefix to avoid collisions
	err := s.db.Update(func(txn *badger.Txn) error {
		prefixedKey := []byte("metadata:" + key)
		return txn.Set(prefixedKey, data)
	})

	if err != nil {
		return fmt.Errorf("failed to save metadata for key %s: %w", key, err)
	}

	return nil
}

// GetMetadata retrieves metadata by key
func (s *badgerConfigStore) GetMetadata(ctx context.Context, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context canceled: %w", err)
	}

	if key == "" {
		return nil, fmt.Errorf("key cannot be empty")
	}

	var data []byte
	err := s.db.View(func(txn *badger.Txn) error {
		prefixedKey := []byte("metadata:" + key)
		item, err := txn.Get(prefixedKey)
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			data = make([]byte, len(val))
			copy(data, val)
			return nil
		})
	})

	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, fmt.Errorf("metadata not found for key %s", key)
		}
		return nil, fmt.Errorf("failed to get metadata for key %s: %w", key, err)
	}

	return data, nil
}

// DeleteMetadata removes metadata by key
func (s *badgerConfigStore) DeleteMetadata(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled: %w", err)
	}

	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	err := s.db.Update(func(txn *badger.Txn) error {
		prefixedKey := []byte("metadata:" + key)
		return txn.Delete(prefixedKey)
	})

	if err != nil && err != badger.ErrKeyNotFound {
		return fmt.Errorf("failed to delete metadata for key %s: %w", key, err)
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
