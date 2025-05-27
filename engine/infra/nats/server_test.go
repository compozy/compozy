package nats

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNatsServerInitialization(t *testing.T) {
	// Create server with default options
	tempDir := t.TempDir()
	cwd, err := core.CWDFromPath(tempDir)
	require.NoError(t, err, "CWD creation should succeed")
	opts := DefaultServerOptions(cwd)
	opts.EnableJetStream = true

	server, err := NewNatsServer(opts)
	require.NoError(t, err, "Server creation should succeed")
	require.NotNil(t, server, "Server should not be nil")
	defer server.Shutdown()

	assert.True(t, server.IsRunning(), "Server should be running")
	assert.NotNil(t, server.Conn, "Connection should be available")
}

func TestJetStreamCreation(t *testing.T) {
	// Create server with JetStream enabled
	tempDir := t.TempDir()
	cwd, err := core.CWDFromPath(tempDir)
	require.NoError(t, err, "CWD creation should succeed")
	opts := DefaultServerOptions(cwd)
	opts.EnableJetStream = true

	server, err := NewNatsServer(opts)
	require.NoError(t, err, "Server creation should succeed")
	require.NotNil(t, server, "Server should not be nil")
	defer server.Shutdown()

	// Create a client with JetStream
	client, err := NewClient(server.Conn)
	require.NoError(t, err, "Client creation should succeed")
	require.NotNil(t, client, "Client should not be nil")
	defer client.Close()

	// Setup client with streams
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.Setup(ctx)
	require.NoError(t, err, "Client setup should succeed")

	// Verify JetStream is accessible
	js, err := client.JetStream()
	require.NoError(t, err, "JetStream context should be available")
	require.NotNil(t, js, "JetStream context should not be nil")

	// Verify streams were created
	streams := []core.StreamName{
		core.StreamCommands,
		core.StreamEvents,
		core.StreamLogs,
	}

	for _, streamName := range streams {
		stream, err := client.GetStream(ctx, streamName)
		assert.NoError(t, err, "Stream %s should exist", streamName)
		assert.NotNil(t, stream, "Stream %s should not be nil", streamName)
	}
}

func TestNatsServerShutdown(t *testing.T) {
	tempDir := t.TempDir()
	cwd, err := core.CWDFromPath(tempDir)
	require.NoError(t, err, "CWD creation should succeed")
	opts := DefaultServerOptions(cwd)
	server, err := NewNatsServer(opts)
	require.NoError(t, err, "Server creation should succeed")
	require.NotNil(t, server, "Server should not be nil")

	// Verify server is running
	assert.True(t, server.IsRunning(), "Server should be running")

	// Shut down server
	err = server.Shutdown()
	require.NoError(t, err, "Server shutdown should succeed")

	// Verify server is no longer running
	assert.False(t, server.IsRunning(), "Server should not be running after shutdown")
}
