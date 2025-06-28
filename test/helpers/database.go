package utils

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var gooseDialectOnce sync.Once

// CreateTestContainerDatabase sets up a new PostgreSQL container for integration tests.
func CreateTestContainerDatabase(ctx context.Context, t *testing.T) (*pgxpool.Pool, func()) {
	pgContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("test-db"),
		postgres.WithUsername("user"),
		postgres.WithPassword("password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second),
		),
	)
	require.NoError(t, err)

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)

	err = ensureTablesExist(pool)
	require.NoError(t, err)

	cleanup := func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	}

	return pool, cleanup
}

// ensureTablesExist runs goose migrations to create the required tables
func ensureTablesExist(db *pgxpool.Pool) error {
	// Convert pgxpool to standard sql.DB for goose
	sqlDB := stdlib.OpenDBFromPool(db)
	defer sqlDB.Close()

	gooseDialectOnce.Do(func() {
		if err := goose.SetDialect("postgres"); err != nil {
			panic(fmt.Sprintf("failed to set goose dialect: %v", err))
		}
	})

	// Find the project root using common utility
	projectRoot, err := FindProjectRoot()
	if err != nil {
		return err
	}

	migrationDir := filepath.Join(projectRoot, "engine", "infra", "store", "migrations")

	// Run migrations up to the latest version
	if err := goose.Up(sqlDB, migrationDir); err != nil {
		return fmt.Errorf("failed to run goose migrations: %w", err)
	}

	return nil
}
