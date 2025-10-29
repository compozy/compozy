package cache

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

// MiniredisStandalone embeds a miniredis server and exposes a go-redis client
// connected to it. It optionally integrates with a SnapshotManager when
// persistence is enabled in configuration.
type MiniredisStandalone struct {
	server   *miniredis.Miniredis
	client   *redis.Client
	snapshot *SnapshotManager
	closed   atomic.Bool
}

// NewMiniredisStandalone creates and starts an embedded Redis server and a
// standard go-redis client connected to it. The function validates the
// connection with a Ping and, when enabled, wires the SnapshotManager
// persistence lifecycle.
func NewMiniredisStandalone(ctx context.Context) (*MiniredisStandalone, error) {
	log := logger.FromContext(ctx)
	cfg := config.FromContext(ctx)

	// Start embedded Redis on a random available port.
	mr := miniredis.NewMiniRedis()
	if err := mr.Start(); err != nil {
		return nil, fmt.Errorf("start miniredis: %w", err)
	}

	log.Info("Started embedded Redis server",
		"addr", mr.Addr(),
		"mode", "standalone",
	)

	// Create a standard go-redis client pointing to the embedded server.
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	// Validate connectivity before returning.
	if err := client.Ping(ctx).Err(); err != nil {
		mr.Close()
		return nil, fmt.Errorf("ping miniredis: %w", err)
	}

	standalone := &MiniredisStandalone{
		server: mr,
		client: client,
	}

	// Optional persistence layer via SnapshotManager.
	if cfg != nil && cfg.Redis.Standalone.Persistence.Enabled {
		log.Info("Initializing persistence layer",
			"data_dir", cfg.Redis.Standalone.Persistence.DataDir,
			"snapshot_interval", cfg.Redis.Standalone.Persistence.SnapshotInterval,
		)

		snapshot, err := NewSnapshotManager(ctx, mr, cfg.Redis.Standalone.Persistence)
		if err != nil {
			_ = standalone.Close(ctx)
			return nil, fmt.Errorf("create snapshot manager: %w", err)
		}
		standalone.snapshot = snapshot

		if cfg.Redis.Standalone.Persistence.RestoreOnStartup {
			if err := snapshot.Restore(ctx); err != nil {
				log.Warn("Failed to restore snapshot", "error", err)
			} else {
				log.Info("Restored last snapshot")
			}
		}

		snapshot.StartPeriodicSnapshots(ctx)
	}

	return standalone, nil
}

// Client returns the go-redis client connected to the embedded server.
func (m *MiniredisStandalone) Client() *redis.Client {
	return m.client
}

// Close gracefully shuts down the embedded Redis server and related resources.
func (m *MiniredisStandalone) Close(ctx context.Context) error {
	if m == nil {
		return nil
	}
	if !m.closed.CompareAndSwap(false, true) {
		return nil // already closed
	}

	log := logger.FromContext(ctx)
	cfg := config.FromContext(ctx)

	if m.snapshot != nil && cfg != nil && cfg.Redis.Standalone.Persistence.SnapshotOnShutdown {
		log.Info("Taking final snapshot before shutdown")
		if err := m.snapshot.Snapshot(ctx); err != nil {
			log.Error("Failed to snapshot on shutdown", "error", err)
		}
		m.snapshot.Stop()
	}

	if m.client != nil {
		if err := m.client.Close(); err != nil {
			log.Warn("Failed to close Redis client", "error", err)
		}
	}

	if m.server != nil {
		m.server.Close()
	}
	log.Info("Closed embedded Redis server")
	return nil
}
