package memory

import (
	"context"
	"log/slog"
	"sync"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	memoryrecall "github.com/compozy/compozy/internal/memory/recall"
)

// NewStore constructs a Store for the provided global memory directory.
func NewStore(globalDir string, opts ...StoreOption) *Store {
	store := &Store{
		globalDir:        cleanDirPath(globalDir),
		maxIndexLines:    defaultIndexLines,
		maxIndexBytes:    defaultIndexBytes,
		maxFileLines:     defaultIndexLines,
		maxFileBytes:     defaultIndexBytes,
		logger:           slog.Default(),
		mu:               &sync.Mutex{},
		decisionMu:       &sync.Mutex{},
		mutationRevision: &storeMutationRevision{},
		recallSignals: recallSignalRecorderConfig{
			queueCapacity:  256,
			workerRetryMax: 3,
		},
		recallRecorders:           newRecallRecorderRegistry(),
		decisionControllerFactory: &decisionControllerFactoryState{},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(store)
		}
	}
	return store
}

// StoreOption customizes a Store instance.
type StoreOption func(*Store)

// WithRecallSignalLifecycle binds asynchronous recall-signal work to a subsystem owner.
func WithRecallSignalLifecycle(ctx context.Context) StoreOption {
	return func(store *Store) {
		if store != nil {
			store.recallSignalLifecycle = ctx
		}
	}
}

// WithCatalogDatabasePath enables the derived SQLite-backed memory catalog in
// the shared global database file.
func WithCatalogDatabasePath(path string) StoreOption {
	return func(store *Store) {
		if store == nil {
			return
		}
		store.catalog = newCatalog(path, func() time.Time {
			return time.Now().UTC()
		})
	}
}

// WithRecallSignalRecorderConfig configures asynchronous recall-signal writes.
func WithRecallSignalRecorderConfig(config compozyconfig.MemoryRecallSignalsConfig) StoreOption {
	return func(store *Store) {
		if store == nil {
			return
		}
		if config.QueueCapacity > 0 {
			store.recallSignals.queueCapacity = config.QueueCapacity
		}
		if config.WorkerRetryMax >= 0 {
			store.recallSignals.workerRetryMax = config.WorkerRetryMax
		}
	}
}

// WithFileLimits configures both curated body limits and prompt-index caps.
func WithFileLimits(config compozyconfig.MemoryFileConfig) StoreOption {
	return func(store *Store) {
		if store == nil {
			return
		}
		if config.MaxLines > 0 {
			store.maxFileLines = config.MaxLines
			store.maxIndexLines = config.MaxLines
		}
		if config.MaxBytes > 0 {
			store.maxFileBytes = config.MaxBytes
			store.maxIndexBytes = int(config.MaxBytes)
		}
	}
}

// RecallSignalRecorderStats returns per-workspace async signal recorder counters.
func (s *Store) RecallSignalRecorderStats(workspaceID string) memoryrecall.SignalRecorderStats {
	if s == nil || s.recallRecorders == nil {
		return memoryrecall.SignalRecorderStats{}
	}
	key := recallSignalRecorderKey(workspaceID)
	return s.recallRecorders.stats(key)
}

// CloseRecallSignalRecorders drains and stops every async recall-signal worker.
func (s *Store) CloseRecallSignalRecorders(ctx context.Context) error {
	if s == nil || s.recallRecorders == nil {
		return nil
	}
	return s.recallRecorders.close(ctx)
}
