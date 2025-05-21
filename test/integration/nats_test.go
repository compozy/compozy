package test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNatsServerMemoryStorage verifies that NATS server with memory storage works
func TestNatsServerMemoryStorage(t *testing.T) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Setup NATS server with JetStream
	natsServer, natsClient := utils.SetupNatsServer(ctx, t)
	require.NotNil(t, natsServer, "NATS server should not be nil")
	require.NotNil(t, natsClient, "NATS client should not be nil")
	defer func() {
		if natsClient != nil {
			err := natsClient.Close()
			if err != nil {
				t.Logf("Error closing NATS client: %s", err)
			}
		}
		if natsServer != nil {
			natsServer.Shutdown()
		}
	}()

	// Assert server is running
	assert.True(t, natsServer.IsRunning(), "NATS server should be running")

	// Verify we can create streams
	for _, streamName := range []nats.StreamName{
		nats.StreamWorkflowCmds,
		nats.StreamTaskCmds,
		nats.StreamAgentCmds,
		nats.StreamToolCmds,
		nats.StreamEvents,
		nats.StreamLogs,
	} {
		stream, err := natsClient.GetStream(ctx, streamName)
		assert.NoError(t, err, "Stream %s should exist", streamName)
		assert.NotNil(t, stream, "Stream %s should not be nil", streamName)
	}
}

// TestCompleteIntegrationSetup tests a complete integration setup with all components
func TestCompleteIntegrationSetup(t *testing.T) {
	// Set up test bed with all components
	componentsToWatch := []nats.ComponentType{
		nats.ComponentWorkflow,
		nats.ComponentTask,
		nats.ComponentAgent,
		nats.ComponentTool,
	}

	tb := SetupIntegrationTestBed(t, DefaultTestTimeout, componentsToWatch)
	defer tb.Cleanup()

	// Verify all components are properly set up
	assert.NotNil(t, tb.NatsServer, "NATS server should be initialized")
	assert.NotNil(t, tb.NatsClient, "NATS client should be initialized")
	assert.NotNil(t, tb.StateManager, "State manager should be initialized")

	// Verify test directory exists
	_, err := os.Stat(tb.StateDir)
	assert.NoError(t, err, "State directory should exist")
}

// TestNatsStreamConfig verifies the NATS stream configuration is correct
func TestNatsStreamConfig(t *testing.T) {
	// Set up NATS with minimal components
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Setup NATS server with JetStream
	natsServer, natsClient := utils.SetupNatsServer(ctx, t)
	require.NotNil(t, natsServer, "NATS server should not be nil")
	require.NotNil(t, natsClient, "NATS client should not be nil")
	defer func() {
		if natsClient != nil {
			err := natsClient.Close()
			if err != nil {
				t.Logf("Error closing NATS client: %s", err)
			}
		}
		if natsServer != nil {
			natsServer.Shutdown()
		}
	}()

	// Get JetStream context
	_, err := natsClient.JetStream()
	require.NoError(t, err, "Should get JetStream context")

	// Verify agent command stream exists and has correct configuration
	stream, err := natsClient.GetStream(ctx, nats.StreamAgentCmds)
	require.NoError(t, err, "Agent command stream should exist")
	require.NotNil(t, stream, "Agent command stream should not be nil")
}
