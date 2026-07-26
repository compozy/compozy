package memory

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	memoryrecall "github.com/compozy/agh/internal/memory/recall"
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
			metricsEnabled: true,
		},
		recallRecorders:           &recallRecorderRegistry{recorders: make(map[string]*memoryrecall.SignalRecorder)},
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
func WithRecallSignalRecorderConfig(config aghconfig.MemoryRecallSignalsConfig) StoreOption {
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
		store.recallSignals.metricsEnabled = config.MetricsEnabled
	}
}

// WithFileLimits configures both curated body limits and prompt-index caps.
func WithFileLimits(config aghconfig.MemoryFileConfig) StoreOption {
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
	s.recallRecorders.mu.Lock()
	recorder := s.recallRecorders.recorders[key]
	s.recallRecorders.mu.Unlock()
	return recorder.Stats()
}

// CloseRecallSignalRecorders drains and stops every async recall-signal worker.
func (s *Store) CloseRecallSignalRecorders(ctx context.Context) error {
	if s == nil || s.recallRecorders == nil {
		return nil
	}
	s.recallRecorders.mu.Lock()
	recorders := make([]*memoryrecall.SignalRecorder, 0, len(s.recallRecorders.recorders))
	for key, recorder := range s.recallRecorders.recorders {
		recorders = append(recorders, recorder)
		delete(s.recallRecorders.recorders, key)
	}
	s.recallRecorders.mu.Unlock()
	var errs []error
	for _, recorder := range recorders {
		if err := recorder.Close(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
