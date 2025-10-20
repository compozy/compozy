package postgres

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultMaxConns           = 20
	defaultMinConns           = 0
	defaultHealthCheckPeriod  = 30 * time.Second
	defaultConnectTimeout     = 5 * time.Second
	defaultPingTimeout        = 3 * time.Second
	defaultHealthCheckTimeout = 1 * time.Second
)

// Store is the concrete PostgreSQL driver backed by pgxpool.Pool.
// It intentionally does not leak pgx types through its public API.
type Store struct {
	pool               *pgxpool.Pool
	metrics            *poolMetrics
	healthCheckTimeout time.Duration
}

// NewStore initializes the pgx pool using the provided config and performs a
// health check. It emits observability labels including store_driver=postgres.
func NewStore(ctx context.Context, cfg *Config) (*Store, error) {
	if cfg == nil {
		return nil, fmt.Errorf("postgres: config is required")
	}
	poolCfg, metricsTracker, err := buildPoolConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: new pool: %w", err)
	}
	pingTimeout := defaultPingTimeout
	if cfg.PingTimeout > 0 {
		pingTimeout = cfg.PingTimeout
	}
	if err := verifyPoolConnection(ctx, pool, metricsTracker, pingTimeout); err != nil {
		return nil, err
	}
	if metricsTracker != nil {
		metricsTracker.attach(pool)
	}
	healthCheckTimeout := defaultHealthCheckTimeout
	if cfg.HealthCheckTimeout > 0 {
		healthCheckTimeout = cfg.HealthCheckTimeout
	}
	logStoreInitialization(ctx, cfg, poolCfg.MaxConns, poolCfg.MinConns)
	return &Store{pool: pool, metrics: metricsTracker, healthCheckTimeout: healthCheckTimeout}, nil
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
	timeout := s.healthCheckTimeout
	if timeout <= 0 {
		timeout = defaultHealthCheckTimeout
	}
	hctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := s.pool.Ping(hctx); err != nil {
		return fmt.Errorf("postgres: health check failed: %w", err)
	}
	return nil
}

// clampIntToInt32WithLimit clamps value to [0, limit] and int32 bounds.
// Non-positive values return 0, and value is clamped to the provided limit and to MaxInt32.
func clampIntToInt32WithLimit(value int, limit int32) int32 {
	if value <= 0 || limit <= 0 {
		return 0
	}
	if value > int(math.MaxInt32) {
		if limit < math.MaxInt32 {
			return limit
		}
		return math.MaxInt32
	}
	if value >= int(limit) {
		return limit
	}
	return int32(value)
}

// buildPoolConfig parses the DSN, configures metrics, and applies pool settings.
func buildPoolConfig(ctx context.Context, cfg *Config) (*pgxpool.Config, *poolMetrics, error) {
	poolCfg, err := pgxpool.ParseConfig(dsn(cfg))
	if err != nil {
		return nil, nil, fmt.Errorf("postgres: parse config: %w", err)
	}
	metricsTracker, mErr := configurePostgresMetrics(cfg, poolCfg)
	if mErr != nil {
		logger.FromContext(ctx).With("err", mErr).Warn("Postgres metrics not initialized; continuing without metrics")
	}
	maxConns, minConns := deriveConnectionBounds(cfg)
	poolCfg.MaxConns = maxConns
	poolCfg.MinConns = minConns
	if cfg.HealthCheckPeriod > 0 {
		poolCfg.HealthCheckPeriod = cfg.HealthCheckPeriod
	} else {
		poolCfg.HealthCheckPeriod = defaultHealthCheckPeriod
	}
	if cfg.ConnectTimeout > 0 {
		poolCfg.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	} else {
		poolCfg.ConnConfig.ConnectTimeout = defaultConnectTimeout
	}
	applyLifetimeSettings(cfg, poolCfg)
	return poolCfg, metricsTracker, nil
}

// deriveConnectionBounds computes max/min connections respecting defaults and limits.
func deriveConnectionBounds(cfg *Config) (int32, int32) {
	maxConns := int32(defaultMaxConns)
	if cfg.MaxOpenConns > 0 {
		if cfg.MaxOpenConns > int(math.MaxInt32) {
			maxConns = math.MaxInt32
		} else {
			maxConns = int32(cfg.MaxOpenConns)
		}
	}
	minConns := int32(defaultMinConns)
	if cfg.MaxIdleConns > 0 {
		if candidate := clampIntToInt32WithLimit(cfg.MaxIdleConns, maxConns); candidate > 0 {
			minConns = candidate
		}
	}
	if minConns > maxConns {
		minConns = maxConns
	}
	return maxConns, minConns
}

// applyLifetimeSettings applies connection lifetime and idle time configuration.
func applyLifetimeSettings(cfg *Config, poolCfg *pgxpool.Config) {
	if cfg.ConnMaxLifetime > 0 {
		poolCfg.MaxConnLifetime = cfg.ConnMaxLifetime
	}
	if cfg.ConnMaxIdleTime > 0 {
		poolCfg.MaxConnIdleTime = cfg.ConnMaxIdleTime
	}
}

// verifyPoolConnection pings the pool and cleans up on failure.
func verifyPoolConnection(
	ctx context.Context,
	pool *pgxpool.Pool,
	metricsTracker *poolMetrics,
	pingTimeout time.Duration,
) error {
	if pingTimeout <= 0 {
		pingTimeout = defaultPingTimeout
	}
	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		if metricsTracker != nil {
			metricsTracker.unregister()
		}
		return fmt.Errorf("postgres: ping: %w", err)
	}
	return nil
}

// logStoreInitialization emits a standardized initialization message.
func logStoreInitialization(ctx context.Context, cfg *Config, maxConns int32, minConns int32) {
	logger.FromContext(ctx).With(
		"store_driver", "postgres",
		"host", cfg.Host,
		"port", cfg.Port,
		"db_name", cfg.DBName,
		"ssl_mode", cfg.SSLMode,
		"max_conns", maxConns,
		"min_conns", minConns,
	).Info("Store initialized")
}
