package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store is the concrete PostgreSQL driver backed by pgxpool.Pool.
// It intentionally does not leak pgx types through its public API.
type Store struct {
	pool *pgxpool.Pool
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
	poolCfg.MaxConns = 20
	poolCfg.MinConns = 2
	poolCfg.HealthCheckPeriod = 30 * time.Second
	poolCfg.ConnConfig.ConnectTimeout = 5 * time.Second
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: new pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	log.With(
		"store_driver", "postgres",
		"host", cfg.Host,
		"port", cfg.Port,
		"db_name", cfg.DBName,
		"ssl_mode", cfg.SSLMode,
	).Info("Store initialized")
	return &Store{pool: pool}, nil
}

// Close shuts down the connection pool.
func (s *Store) Close(ctx context.Context) error {
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
