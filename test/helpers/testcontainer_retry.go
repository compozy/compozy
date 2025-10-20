package helpers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RetryConfig defines configuration for test retries
type RetryConfig struct {
	MaxAttempts   int
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
}

// DefaultRetryConfig returns the default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  1 * time.Second,
		MaxDelay:      10 * time.Second,
		BackoffFactor: 2.0,
	}
}

// SetupTestReposWithRetry sets up test repositories with retry logic for testcontainer failures
func SetupTestReposWithRetry(ctx context.Context, t *testing.T, config ...RetryConfig) (*pgxpool.Pool, func(), error) {
	retryConfig := DefaultRetryConfig()
	if len(config) > 0 {
		retryConfig = config[0]
	}
	var lastErr error
	delay := retryConfig.InitialDelay
	for attempt := 1; attempt <= retryConfig.MaxAttempts; attempt++ {
		pool, cleanup, err := trySetupTestContainer(t)
		if err == nil {
			if attempt > 1 {
				t.Logf("Successfully created test container on attempt %d", attempt)
			}
			return pool, cleanup, nil
		}

		lastErr = err
		t.Logf("Failed to create test container on attempt %d/%d: %v", attempt, retryConfig.MaxAttempts, err)

		if attempt < retryConfig.MaxAttempts {
			t.Logf("Retrying in %v...", delay)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, nil, fmt.Errorf("context canceled while retrying: %w", ctx.Err())
			}

			delay = min(time.Duration(float64(delay)*retryConfig.BackoffFactor), retryConfig.MaxDelay)
		}
	}
	return nil, nil, fmt.Errorf(
		"failed to create test container after %d attempts: %w",
		retryConfig.MaxAttempts,
		lastErr,
	)
}

// trySetupTestContainer attempts to get the shared container and returns pool, cleanup, and error
func trySetupTestContainer(t *testing.T) (*pgxpool.Pool, func(), error) {
	pool, cleanup := GetSharedPostgresDB(t)
	if err := pool.Ping(t.Context()); err != nil {
		if cleanup != nil {
			cleanup()
		}
		return nil, nil, fmt.Errorf("failed to ping database: %w", err)
	}
	return pool, cleanup, nil
}

// RunWithTestRepos is a helper that sets up test repos with retry and runs a test function
func RunWithTestRepos(t *testing.T, testFunc func(ctx context.Context, pool *pgxpool.Pool)) {
	t.Helper()
	pool, cleanup, err := SetupTestReposWithRetry(t.Context(), t)
	if err != nil {
		t.Fatalf("Failed to set up test repositories: %v", err)
	}
	defer cleanup()
	testFunc(t.Context(), pool)
}

// TestContainerHealthCheck performs additional health checks on the test container
func TestContainerHealthCheck(ctx context.Context, pool *pgxpool.Pool) error {
	var result int
	err := pool.QueryRow(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		return fmt.Errorf("health check query failed: %w", err)
	}
	if result != 1 {
		return fmt.Errorf("unexpected health check result: %d", result)
	}
	var tableCount int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM information_schema.tables
		WHERE table_schema = 'public'
		AND table_name IN ('workflow_states', 'task_states')
	`).Scan(&tableCount)
	if err != nil {
		return fmt.Errorf("failed to check tables: %w", err)
	}
	if tableCount != 2 {
		return fmt.Errorf("expected 2 tables, found %d", tableCount)
	}
	return nil
}
