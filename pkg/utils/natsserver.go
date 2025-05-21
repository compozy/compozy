package utils

import (
	"context"
	"testing"

	"github.com/compozy/compozy/pkg/nats"
	"github.com/stretchr/testify/require"
)

func SetupNatsServer(t *testing.T, ctx context.Context) (*nats.Server, *nats.Client) {
	// Start an embedded NATS server
	opts := nats.DefaultServerOptions()
	opts.EnableJetStream = true

	natsServer, err := nats.NewNatsServer(opts)
	require.NoError(t, err, "Failed to start NATS server")

	// Create a client for tests
	natsClient, err := nats.NewClient(natsServer.Conn)
	require.NoError(t, err)

	// Setup JetStream for the client
	err = natsClient.Setup(ctx)
	require.NoError(t, err, "Failed to setup NATS client")

	return natsServer, natsClient
}
