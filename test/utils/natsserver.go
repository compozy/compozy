package utils

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/nats"
	"github.com/stretchr/testify/require"
)

var (
	// Shared NATS server for all tests
	sharedNatsServer *nats.Server
	sharedNatsClient *nats.Client
	sharedTempDir    string
	natsSetupOnce    sync.Once
	natsSetupError   error
)

// GetSharedNatsServer returns a shared NATS server and client for tests
// This avoids the overhead of creating multiple NATS servers during test execution
func GetSharedNatsServer(t *testing.T) (*nats.Server, *nats.Client) {
	t.Helper()

	natsSetupOnce.Do(func() {
		// Create a shared temp directory for the NATS server that persists across tests
		var err error
		sharedTempDir, err = os.MkdirTemp("", "compozy_shared_nats_")
		if err != nil {
			natsSetupError = err
			return
		}

		cwd, err := core.CWDFromPath(sharedTempDir)
		if err != nil {
			natsSetupError = err
			return
		}

		opts := nats.DefaultServerOptions(cwd)
		opts.EnableJetStream = true
		opts.Port = 0 // Use random port

		sharedNatsServer, err = nats.NewNatsServer(opts)
		if err != nil {
			natsSetupError = err
			return
		}

		sharedNatsClient, err = nats.NewClient(sharedNatsServer.Conn)
		if err != nil {
			natsSetupError = err
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err = sharedNatsClient.SetupStreams(ctx)
		if err != nil {
			natsSetupError = err
			return
		}

		t.Logf("Shared NATS server successfully started for tests in directory: %s", sharedTempDir)
	})

	require.NoError(t, natsSetupError, "Failed to setup shared NATS server")
	require.NotNil(t, sharedNatsServer, "Shared NATS server should not be nil")
	require.NotNil(t, sharedNatsClient, "Shared NATS client should not be nil")
	require.True(t, sharedNatsServer.IsRunning(), "Shared NATS server should be running")

	return sharedNatsServer, sharedNatsClient
}

// SetupNatsServer creates individual NATS server for tests that need isolation
// Use GetSharedNatsServer() instead for better performance in most cases
func SetupNatsServer(ctx context.Context, t *testing.T) (*nats.Server, *nats.Client) {
	t.Helper()

	tempDir := t.TempDir()
	cwd, err := core.CWDFromPath(tempDir)
	require.NoError(t, err, "CWD creation should succeed")

	opts := nats.DefaultServerOptions(cwd)
	opts.EnableJetStream = true
	opts.Port = 0 // Use random port

	natsServer, err := nats.NewNatsServer(opts)
	require.NoError(t, err, "Failed to start NATS server")

	natsClient, err := nats.NewClient(natsServer.Conn)
	require.NoError(t, err, "Failed to create NATS client")

	err = natsClient.Setup(ctx)
	require.NoError(t, err, "Failed to setup NATS client")

	require.True(t, natsServer.IsRunning(), "NATS server should be running")

	t.Logf("Individual NATS server successfully started for test %s", t.Name())
	return natsServer, natsClient
}

// CleanupSharedNats should be called in TestMain to cleanup the shared NATS server
func CleanupSharedNats() {
	if sharedNatsClient != nil {
		_ = sharedNatsClient.Close()
	}
	if sharedNatsServer != nil {
		if err := sharedNatsServer.Shutdown(); err != nil {
			// Log error but don't panic during cleanup
			os.Stderr.WriteString("Failed to shutdown shared NATS server: " + err.Error() + "\n")
		}
		sharedNatsServer.WaitForShutdown()
	}
	if sharedTempDir != "" {
		_ = os.RemoveAll(sharedTempDir)
	}
}
