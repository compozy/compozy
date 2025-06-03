package store

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBInterface defines the minimal interface needed by repositories.
// This allows both real pgxpool.Pool and pgxmock.PgxPoolIface to be used.
type DBInterface interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Config holds PostgreSQL connection settings.
type Config struct {
	ConnString string
	Host       string
	Port       string
	User       string
	Password   string
	DBName     string
	SSLMode    string
}

type DB struct {
	pool *pgxpool.Pool
}

// NewPool creates a new pgxpool.Pool with the provided configuration.
func NewDB(ctx context.Context, cfg *Config) (*DB, error) {
	connString := cfg.ConnString
	if connString == "" {
		connString = fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			getEnvOrDefault(cfg.Host, "localhost"),
			getEnvOrDefault(cfg.Port, "5432"),
			getEnvOrDefault(cfg.User, "postgres"),
			getEnvOrDefault(cfg.Password, ""),
			getEnvOrDefault(cfg.DBName, "postgres"),
			getEnvOrDefault(cfg.SSLMode, "disable"),
		)
	}

	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("parsing connection string: %w", err)
	}

	config.MaxConns = 20
	config.MinConns = 2
	config.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("creating pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	logger.With(
		"host", cfg.Host,
		"port", cfg.Port,
		"user", cfg.User,
		"db_name", cfg.DBName,
		"ssl_mode", cfg.SSLMode,
	).Info("Database connection established")
	return &DB{pool: pool}, nil
}

// Close shuts down the connection pool.
func (db *DB) Close() error {
	db.pool.Close()
	logger.Info("Database connection closed")
	return nil
}

// Pool returns the underlying pgxpool.Pool
func (db *DB) Pool() *pgxpool.Pool {
	return db.pool
}

// WithTx executes a function within a transaction.
func (db *DB) WithTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			err := tx.Rollback(ctx)
			if err != nil {
				logger.Error("error rolling back transaction", "error", err)
			}
		} else if err != nil {
			err := tx.Rollback(ctx)
			if err != nil {
				logger.Error("error rolling back transaction", "error", err)
			}
		} else {
			err := tx.Commit(ctx)
			if err != nil {
				logger.Error("error committing transaction", "error", err)
			}
		}
	}()

	err = fn(tx)
	return err
}

func getEnvOrDefault(val, def string) string {
	if val == "" {
		return def
	}
	return val
}
