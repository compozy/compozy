package temporal

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/worker/embedded"
	"github.com/compozy/compozy/test/helpers"
)

func TestPortConflict(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping temporal integration tests in short mode")
	}

	ctx := helpers.NewTestContext(t)
	frontendPort := findAvailablePortRange(ctx, t, 4)
	primaryCfg := newEmbeddedConfigFromDefaults()
	primaryCfg.EnableUI = false
	primaryCfg.FrontendPort = frontendPort
	server := startEmbeddedServer(ctx, t, primaryCfg)
	t.Cleanup(func() {
		stopTemporalServer(ctx, t, server)
	})

	conflictCtx := helpers.NewTestContext(t)
	conflictCfg := newEmbeddedConfigFromDefaults()
	conflictCfg.EnableUI = false
	conflictCfg.FrontendPort = frontendPort

	_, err := embedded.NewServer(conflictCtx, conflictCfg)
	require.Error(t, err)
	require.ErrorContains(t, err, "already in use")
	require.ErrorContains(t, err, fmt.Sprintf("%d", frontendPort))
	require.ErrorContains(t, err, "adjust configuration")
}

func TestStartupTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping temporal integration tests in short mode")
	}

	ctx := helpers.NewTestContext(t)
	cfg := newEmbeddedConfigFromDefaults()
	cfg.EnableUI = false
	cfg.FrontendPort = findAvailablePortRange(ctx, t, 4)
	cfg.StartTimeout = time.Nanosecond

	server, err := embedded.NewServer(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		stopTemporalServer(ctx, t, server)
	})

	startErr := server.Start(ctx)
	require.Error(t, startErr)
	require.ErrorContains(t, startErr, "wait for ready")
	require.ErrorContains(t, startErr, "context deadline exceeded")
}

func TestInvalidDatabasePath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping temporal integration tests in short mode")
	}

	ctx := helpers.NewTestContext(t)
	cfg := newEmbeddedConfigFromDefaults()
	cfg.EnableUI = false
	cfg.FrontendPort = findAvailablePortRange(ctx, t, 4)

	tempFile := filepath.Join(t.TempDir(), "existing.file")
	require.NoError(t, os.WriteFile(tempFile, []byte("placeholder"), 0o600))
	cfg.DatabaseFile = filepath.Join(tempFile, "temporal.db")

	_, err := embedded.NewServer(ctx, cfg)
	require.Error(t, err)
	require.ErrorContains(t, err, "is not a directory")
}

func TestMissingDatabaseDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping temporal integration tests in short mode")
	}

	ctx := helpers.NewTestContext(t)
	cfg := newEmbeddedConfigFromDefaults()
	cfg.EnableUI = false
	cfg.FrontendPort = findAvailablePortRange(ctx, t, 4)
	cfg.DatabaseFile = filepath.Join(t.TempDir(), "missing", "temporal.db")

	_, err := embedded.NewServer(ctx, cfg)
	require.Error(t, err)
	require.ErrorContains(t, err, "not accessible")
}

func TestDatabaseCorruption(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping temporal integration tests in short mode")
	}

	ctx := helpers.NewTestContext(t)
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "corrupt.db")
	require.NoError(t, os.WriteFile(dbPath, []byte("not-a-sqlite-database"), 0o600))

	cfg := newEmbeddedConfigFromDefaults()
	cfg.EnableUI = false
	cfg.DatabaseFile = dbPath

	_, err := embedded.NewServer(ctx, cfg)
	require.Error(t, err)
	require.ErrorContains(t, err, "create namespace")
	require.ErrorContains(t, err, "not a database")
}
