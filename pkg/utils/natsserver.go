package utils

import (
	"context"
	"testing"

	"github.com/compozy/compozy/pkg/nats"
	"github.com/stretchr/testify/require"
)

func SetupNatsServer(ctx context.Context, t *testing.T) (*nats.Server, *nats.Client) {
	opts := nats.DefaultServerOptions()
	opts.EnableJetStream = true

	natsServer, err := nats.NewNatsServer(opts)
	require.NoError(t, err, "Failed to start NATS server")

	natsClient, err := nats.NewClient(natsServer.Conn)
	require.NoError(t, err)

	err = natsClient.Setup(ctx)
	require.NoError(t, err, "Failed to setup NATS client")

	return natsServer, natsClient
}
