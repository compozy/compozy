package postgres

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store is the concrete PostgreSQL driver backed by pgxpool.Pool.
// It intentionally does not leak pgx types through its public API.
type Store struct {
	pool    *pgxpool.Pool
	metrics *poolMetrics
}

// NewStore initializes the pgx pool using the provided config and performs a
// health check. It emits observability labels including store_driver=postgres.
func NewStore(ctx context.Context, cfg *Config) (*Store, error) {
	if cfg == nil {
		return nil, fmt.Errorf("postgres: config is required")
	}
	log := logger.FromContext(ctx)
	poolCfg, err := pgxpool.ParseConfig(dsn(cfg))
	if err != nil {
		return nil, fmt.Errorf("postgres: parse config: %w", err)
	}
	metricsTracker := configurePostgresMetrics(cfg, poolCfg)
	maxConns := int32(20)
	if cfg.MaxOpenConns > 0 {
		if cfg.MaxOpenConns > int(math.MaxInt32) {
			maxConns = math.MaxInt32
		} else {
			maxConns = int32(cfg.MaxOpenConns)
		}
	}
	minConns := int32(2)
	if cfg.MaxIdleConns > 0 {
		candidate := clampIntToInt32WithLimit(cfg.MaxIdleConns, maxConns)
		if candidate > 0 {
			minConns = candidate
		}
	}
	poolCfg.MaxConns = maxConns
	poolCfg.MinConns = minConns
	poolCfg.HealthCheckPeriod = 30 * time.Second
	poolCfg.ConnConfig.ConnectTimeout = 5 * time.Second
	if cfg.ConnMaxLifetime > 0 {
		poolCfg.MaxConnLifetime = cfg.ConnMaxLifetime
	}
	if cfg.ConnMaxIdleTime > 0 {
		poolCfg.MaxConnIdleTime = cfg.ConnMaxIdleTime
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: new pool: %w", err)
	}
	if metricsTracker != nil {
		metricsTracker.attach(pool)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		if metricsTracker != nil {
			metricsTracker.unregister()
		}
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	log.With(
		"store_driver", "postgres",
		"host", cfg.Host,
		"port", cfg.Port,
		"db_name", cfg.DBName,
		"ssl_mode", cfg.SSLMode,
	).Info("Store initialized")
	return &Store{pool: pool, metrics: metricsTracker}, nil
}

// Close shuts down the connection pool.
func (s *Store) Close(ctx context.Context) error {
	if s.metrics != nil {
		s.metrics.unregister()
	}
	s.pool.Close()
	logger.FromContext(ctx).Info("Postgres store closed")
	return nil
}

// Pool exposes the internal pool for driver-local usage. Do not export pgx
// types through higher layers; keep them local to the driver.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// HealthCheck verifies the connection is alive.
func (s *Store) HealthCheck(ctx context.Context) error {
	hctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	if err := s.pool.Ping(hctx); err != nil {
		return fmt.Errorf("postgres: health check failed: %w", err)
	}
	return nil
}

func clampIntToInt32WithLimit(value int, limit int32) int32 {
	if value <= 0 {
		return 0
	}
	if value >= int(limit) {
		return limit
	}
	if value > int(math.MaxInt32) {
		return math.MaxInt32
	}
	return int32(value)
}
