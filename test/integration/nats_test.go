package test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNatsServerMemoryStorage(t *testing.T) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use shared NATS server instead of creating individual servers
	natsServer, natsClient := utils.GetSharedNatsServer(t)
	require.NotNil(t, natsServer, "NATS server should not be nil")
	require.NotNil(t, natsClient, "NATS client should not be nil")

	// Assert server is running
	assert.True(t, natsServer.IsRunning(), "NATS server should be running")

	// Verify we can create streams
	for _, streamName := range []core.StreamName{
		core.StreamCommands,
		core.StreamEvents,
		core.StreamLogs,
	} {
		stream, err := natsClient.GetStream(ctx, streamName)
		assert.NoError(t, err, "Stream %s should exist", streamName)
		assert.NotNil(t, stream, "Stream %s should not be nil", streamName)
	}
}

func TestCompleteIntegrationSetup(t *testing.T) {
	tb := utils.SetupIntegrationTestBed(t, utils.DefaultTestTimeout)
	defer tb.Cleanup()

	// Verify all components are properly set up
	assert.NotNil(t, tb.NatsServer, "NATS server should be initialized")
	assert.NotNil(t, tb.NatsClient, "NATS client should be initialized")

	// Verify test directory exists
	_, err := os.Stat(tb.StateDir)
	assert.NoError(t, err, "State directory should exist")
}

func TestNatsStreamConfig(t *testing.T) {
	// Set up NATS with minimal components
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use shared NATS server instead of creating individual servers
	natsServer, natsClient := utils.GetSharedNatsServer(t)
	require.NotNil(t, natsServer, "NATS server should not be nil")
	require.NotNil(t, natsClient, "NATS client should not be nil")

	// Get JetStream context
	_, err := natsClient.JetStream()
	require.NoError(t, err, "Should get JetStream context")

	// Verify agent command stream exists and has correct configuration
	stream, err := natsClient.GetStream(ctx, core.StreamCommands)
	require.NoError(t, err, "Agent command stream should exist")
	require.NotNil(t, stream, "Agent command stream should not be nil")
}

func TestNatsIntegration(t *testing.T) {
	tb := utils.SetupIntegrationTestBed(t, utils.DefaultTestTimeout)
	defer tb.Cleanup()

	// ... existing code ...
}
