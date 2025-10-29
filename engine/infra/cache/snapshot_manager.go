package cache

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/dgraph-io/badger/v4"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

const (
	metaPrefix              = "_meta:"
	metaKeyTimestamp        = metaPrefix + "timestamp"
	metaKeyVersion          = metaPrefix + "version"
	snapshotFormatVersion   = "1.0"
	defaultSnapshotInterval = 5 * time.Minute
)

// SnapshotManager persists/restores miniredis state using BadgerDB. All
// operations are non-blocking with a background goroutine for periodic
// snapshots. Configuration must be obtained via config.FromContext(ctx).
type SnapshotManager struct {
	miniredis *miniredis.Miniredis
	db        *badger.DB

	stopCh chan struct{}
	wg     sync.WaitGroup
	mu     sync.RWMutex

	metrics *SnapshotMetrics
}

// NewSnapshotManager initializes Badger in the configured data directory and
// wires it to the provided miniredis instance. The function creates the data
// directory when missing.
func NewSnapshotManager(
	ctx context.Context,
	mr *miniredis.Miniredis,
	cfg config.RedisPersistenceConfig,
) (*SnapshotManager, error) {
	log := logger.FromContext(ctx)

	if mr == nil {
		return nil, fmt.Errorf("miniredis instance is required")
	}

	if cfg.DataDir == "" {
		return nil, fmt.Errorf("persistence data directory is required")
	}

	// Ensure directory exists with sane permissions.
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	opts := badger.DefaultOptions(cfg.DataDir)
	// Quiet Badger logs during normal operation; our project uses its own logger.
	opts = opts.WithLogger(nil)

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("open badger: %w", err)
	}
	log.Info("Opened BadgerDB for Redis snapshots", "data_dir", cfg.DataDir)

	return &SnapshotManager{
		miniredis: mr,
		db:        db,
		stopCh:    make(chan struct{}),
		metrics:   &SnapshotMetrics{},
	}, nil
}

// Snapshot writes the entire miniredis keyspace into Badger as a single
// atomic transaction. The previous snapshot is dropped before writing the new
// one to avoid stale keys.
func (sm *SnapshotManager) Snapshot(ctx context.Context) error {
	log := logger.FromContext(ctx)
	if sm == nil || sm.db == nil {
		return fmt.Errorf("snapshot manager not initialized")
	}
	start := time.Now()

	// Capture keys first to keep transaction short.
	keys := sm.miniredis.Keys()

	// Best-effort clear of previous contents to avoid stale data.
	if err := sm.db.DropAll(); err != nil {
		log.Warn("Failed to drop previous snapshot", "error", err)
	}

	var totalBytes int64
	err := sm.db.Update(func(txn *badger.Txn) error {
		// Metadata
		if err := txn.Set([]byte(metaKeyTimestamp), []byte(time.Now().Format(time.RFC3339Nano))); err != nil {
			return err
		}
		if err := txn.Set([]byte(metaKeyVersion), []byte(snapshotFormatVersion)); err != nil {
			return err
		}
		// Write all keys
		for _, k := range keys {
			v, gErr := sm.miniredis.Get(k)
			if gErr != nil {
				log.Debug("skip non-string or missing key during snapshot", "key", k, "error", gErr)
				continue
			}
			// Use Set with default options; this is snapshot, not TTL aware.
			if err := txn.Set([]byte(k), []byte(v)); err != nil {
				return err
			}
			totalBytes += int64(len(k)) + int64(len(v))
		}
		return nil
	})
	if err != nil {
		sm.metrics.mu.Lock()
		sm.metrics.snapshotFailures++
		sm.metrics.mu.Unlock()
		log.Error("Snapshot failed", "error", err)
		return fmt.Errorf("snapshot transaction: %w", err)
	}

	duration := time.Since(start)
	sm.metrics.mu.Lock()
	sm.metrics.snapshotsTaken++
	sm.metrics.lastDuration = duration
	sm.metrics.lastSizeBytes = totalBytes
	sm.metrics.mu.Unlock()

	log.Info("Snapshot completed", "duration", duration, "keys", len(keys), "bytes", totalBytes)
	return nil
}

// Restore reads the last snapshot from Badger and populates miniredis. The
// operation is idempotent and ignores metadata keys.
func (sm *SnapshotManager) Restore(ctx context.Context) error {
	log := logger.FromContext(ctx)
	if sm == nil || sm.db == nil {
		return fmt.Errorf("snapshot manager not initialized")
	}
	start := time.Now()
	var count int
	err := sm.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := string(item.Key())
			if strings.HasPrefix(key, metaPrefix) {
				continue
			}
			if err := item.Value(func(val []byte) error {
				if err := sm.miniredis.Set(key, string(val)); err != nil {
					return err
				}
				count++
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		sm.metrics.mu.Lock()
		sm.metrics.restoreFailures++
		sm.metrics.mu.Unlock()
		log.Error("Restore failed", "error", err)
		return fmt.Errorf("restore transaction: %w", err)
	}
	sm.metrics.mu.Lock()
	sm.metrics.restores++
	sm.metrics.lastDuration = time.Since(start)
	sm.metrics.mu.Unlock()
	log.Info("Restore completed", "keys", count, "duration", time.Since(start))
	return nil
}

// StartPeriodicSnapshots launches a goroutine that takes snapshots at the
// configured interval until Stop is called.
func (sm *SnapshotManager) StartPeriodicSnapshots(ctx context.Context) {
	if sm == nil {
		return
	}
	cfg := config.FromContext(ctx)
	log := logger.FromContext(ctx)
	interval := defaultSnapshotInterval
	if cfg != nil {
		if v := cfg.Redis.Standalone.Persistence.SnapshotInterval; v > 0 {
			interval = v
		}
	}
	log.Info("Starting periodic snapshots", "interval", interval)

	sm.wg.Add(1)
	go func() {
		defer sm.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := sm.Snapshot(ctx); err != nil {
					log.Error("Periodic snapshot failed", "error", err)
				}
			case <-sm.stopCh:
				log.Info("Stopping periodic snapshots")
				return
			case <-ctx.Done():
				log.Info("Context canceled; stopping periodic snapshots")
				return
			}
		}
	}()
}

// Stop stops the background goroutine and closes the Badger database.
func (sm *SnapshotManager) Stop() {
	if sm == nil {
		return
	}
	// Safe close: prevent panic if closed twice.
	sm.mu.Lock()
	select {
	case <-sm.stopCh:
		// already closed
	default:
		close(sm.stopCh)
	}
	sm.mu.Unlock()
	sm.wg.Wait()
	if sm.db != nil {
		_ = sm.db.Close()
	}
}

// GetSnapshotMetrics returns a snapshot of metrics values.
func (sm *SnapshotManager) GetSnapshotMetrics() SnapshotMetricsView {
	if sm == nil || sm.metrics == nil {
		return SnapshotMetricsView{}
	}
	return sm.metrics.copy()
}

// Intentionally no directory size helpers; size metrics are computed from
// serialized key/value bytes during snapshot transactions.
