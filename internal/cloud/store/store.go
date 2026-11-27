// SPDX-License-Identifier: FSL-1.1-MIT

package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/compozy/compozy/internal/cloud/observe"
	"github.com/compozy/compozy/internal/cloud/store/sqlcgen"
)

type Config struct {
	PooledDSN string
	DirectDSN string
	Metrics   *observe.Metrics
}

type Store struct {
	*Queries
	pool   *pgxpool.Pool
	direct *pgx.ConnConfig
}

type Queries struct {
	generated *sqlcgen.Queries
	inTx      bool
}

func Open(ctx context.Context, config Config) (*Store, error) {
	direct, err := postgresConfig(config.DirectDSN)
	if err != nil {
		return nil, fmt.Errorf("direct endpoint: %w", err)
	}
	if strings.TrimSpace(config.PooledDSN) == "" {
		return nil, errors.New("pooled Postgres DSN is required")
	}
	pooled, err := pgxpool.ParseConfig(config.PooledDSN)
	if err != nil {
		return nil, errors.New("invalid pooled Postgres DSN")
	}
	// Request connections must work through PgBouncer in transaction mode.
	pooled.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	pooled.ConnConfig.ConnectTimeout = 5 * time.Second
	if config.Metrics != nil {
		pooled.ConnConfig.Tracer = queryTiming{config.Metrics}
	}
	pool, err := pgxpool.NewWithConfig(ctx, pooled)
	if err != nil {
		return nil, connectionError("create cloud Postgres pool", err)
	}
	return &Store{Queries: &Queries{generated: sqlcgen.New(pool)}, pool: pool, direct: direct}, nil
}

func postgresConfig(dsn string) (*pgx.ConnConfig, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("direct Postgres DSN is required")
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, errors.New("invalid direct Postgres DSN")
	}
	config.ConnectTimeout = 5 * time.Second
	return config, nil
}

// Connection errors can include the DSN or credentials in pgx diagnostics.
func connectionError(operation string, err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%s: %w", operation, err)
	}
	return fmt.Errorf("%s: connection failed", operation)
}

func (s *Store) Ping(ctx context.Context) error {
	if _, err := s.generated.GetGatePolicy(ctx); err != nil {
		return connectionError("check cloud Postgres readiness", err)
	}
	return nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) ConnectDirect(ctx context.Context) (*pgx.Conn, error) {
	conn, err := pgx.ConnectConfig(ctx, s.direct.Copy())
	if err != nil {
		return nil, connectionError("connect to direct cloud Postgres", err)
	}
	return conn, nil
}

func (s *Store) Transaction(ctx context.Context, run func(*Queries) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		return run(&Queries{generated: sqlcgen.New(tx), inTx: true})
	})
}
