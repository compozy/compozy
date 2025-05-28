package utils

import (
	"context"
	"fmt"
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

// NatsServerOptions provides configuration for test NATS servers
type NatsServerOptions struct {
	EnableJetStream bool
	Port            int
	StoreDir        string
	ServerName      string
	Debug           bool
	Trace           bool
}

// DefaultNatsServerOptions returns sensible defaults for test servers
func DefaultNatsServerOptions() NatsServerOptions {
	return NatsServerOptions{
		EnableJetStream: true,
		Port:            0, // Random port
		ServerName:      "compozy_test_server",
		Debug:           false,
		Trace:           false,
	}
}

// GetSharedNatsServer returns a shared NATS server and client for tests
// This avoids the overhead of creating multiple NATS servers during test execution
func GetSharedNatsServer(t *testing.T) (*nats.Server, *nats.Client) {
	t.Helper()
	return GetSharedNatsServerWithOptions(t, DefaultNatsServerOptions())
}

// GetSharedNatsServerWithOptions returns a shared NATS server with custom options
func GetSharedNatsServerWithOptions(t *testing.T, opts NatsServerOptions) (*nats.Server, *nats.Client) {
	t.Helper()

	natsSetupOnce.Do(func() {
		var err error
		sharedTempDir, err = os.MkdirTemp("", "compozy_shared_nats_")
		if err != nil {
			natsSetupError = fmt.Errorf("failed to create temp dir: %w", err)
			return
		}

		cwd, err := core.CWDFromPath(sharedTempDir)
		if err != nil {
			natsSetupError = fmt.Errorf("failed to create CWD: %w", err)
			return
		}

		serverOpts := nats.DefaultServerOptions(cwd)
		serverOpts.EnableJetStream = opts.EnableJetStream
		serverOpts.Port = opts.Port
		serverOpts.ServerName = opts.ServerName
		serverOpts.EnableLogging = opts.Debug || opts.Trace

		if opts.StoreDir != "" {
			serverOpts.StoreDir = opts.StoreDir
		}

		sharedNatsServer, err = nats.NewNatsServer(serverOpts)
		if err != nil {
			natsSetupError = fmt.Errorf("failed to create NATS server: %w", err)
			return
		}

		// Wait for server to be ready with timeout
		if !sharedNatsServer.IsRunning() {
			natsSetupError = fmt.Errorf("NATS server failed to start")
			return
		}

		sharedNatsClient, err = nats.NewClient(sharedNatsServer.Conn)
		if err != nil {
			natsSetupError = fmt.Errorf("failed to create NATS client: %w", err)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if opts.EnableJetStream {
			err = sharedNatsClient.SetupStreams(ctx)
			if err != nil {
				natsSetupError = fmt.Errorf("failed to setup streams: %w", err)
				return
			}
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
	return SetupNatsServerWithOptions(ctx, t, DefaultNatsServerOptions())
}

// SetupNatsServerWithOptions creates individual NATS server with custom options
func SetupNatsServerWithOptions(
	ctx context.Context,
	t *testing.T,
	opts NatsServerOptions,
) (*nats.Server, *nats.Client) {
	t.Helper()

	tempDir := t.TempDir()
	cwd, err := core.CWDFromPath(tempDir)
	require.NoError(t, err, "CWD creation should succeed")

	serverOpts := nats.DefaultServerOptions(cwd)
	serverOpts.EnableJetStream = opts.EnableJetStream
	serverOpts.Port = opts.Port
	serverOpts.ServerName = opts.ServerName
	serverOpts.EnableLogging = opts.Debug || opts.Trace

	if opts.StoreDir != "" {
		serverOpts.StoreDir = opts.StoreDir
	}

	natsServer, err := nats.NewNatsServer(serverOpts)
	require.NoError(t, err, "Failed to start NATS server")

	natsClient, err := nats.NewClient(natsServer.Conn)
	require.NoError(t, err, "Failed to create NATS client")

	if opts.EnableJetStream {
		err = natsClient.Setup(ctx)
		require.NoError(t, err, "Failed to setup NATS client")
	}

	require.True(t, natsServer.IsRunning(), "NATS server should be running")

	t.Logf("Individual NATS server successfully started for test %s", t.Name())
	return natsServer, natsClient
}

// RunBasicNatsServer creates a minimal NATS server for simple tests (inspired by official patterns)
func RunBasicNatsServer(t *testing.T) (*nats.Server, *nats.Client) {
	t.Helper()

	opts := NatsServerOptions{
		EnableJetStream: false,
		Port:            0,
		ServerName:      "basic_test_server",
		Debug:           false,
		Trace:           false,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return SetupNatsServerWithOptions(ctx, t, opts)
}

// RunJetStreamServer creates a NATS server with JetStream enabled (inspired by official patterns)
func RunJetStreamServer(t *testing.T) (*nats.Server, *nats.Client) {
	t.Helper()

	opts := NatsServerOptions{
		EnableJetStream: true,
		Port:            0,
		ServerName:      "jetstream_test_server",
		Debug:           false,
		Trace:           false,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return SetupNatsServerWithOptions(ctx, t, opts)
}

// InProcessNatsServer creates a minimal in-process NATS server (inspired by official patterns)
// This is useful for simple tests that don't need JetStream or persistence
func InProcessNatsServer(t *testing.T) (*nats.Server, *nats.Client) {
	t.Helper()

	tempDir := t.TempDir()
	cwd, err := core.CWDFromPath(tempDir)
	require.NoError(t, err, "CWD creation should succeed")

	serverOpts := nats.DefaultServerOptions(cwd)
	serverOpts.EnableJetStream = false
	serverOpts.Port = 0
	serverOpts.ServerName = "in_process_test_server"

	natsServer, err := nats.NewNatsServer(serverOpts)
	require.NoError(t, err, "Failed to start in-process NATS server")

	natsClient, err := nats.NewClient(natsServer.Conn)
	require.NoError(t, err, "Failed to create NATS client")

	require.True(t, natsServer.IsRunning(), "In-process NATS server should be running")

	t.Logf("In-process NATS server started for test %s", t.Name())
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

// WaitForServerReady waits for a NATS server to be ready for connections
func WaitForServerReady(t *testing.T, server *nats.Server, timeout time.Duration) {
	t.Helper()

	start := time.Now()
	for time.Since(start) < timeout {
		if server.IsRunning() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	require.Fail(t, "NATS server did not become ready within timeout", "timeout: %v", timeout)
}
