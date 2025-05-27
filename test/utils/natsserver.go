package utils

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/nats"
	"github.com/stretchr/testify/require"
)

func SetupNatsServer(ctx context.Context, t *testing.T) (*nats.Server, *nats.Client) {
	tempDir := t.TempDir()
	cwd, err := core.CWDFromPath(tempDir)
	require.NoError(t, err, "CWD creation should succeed")
	opts := nats.DefaultServerOptions(cwd)
	opts.EnableJetStream = true

	natsServer, err := nats.NewNatsServer(opts)
	require.NoError(t, err, "Failed to start NATS server")

	natsClient, err := nats.NewClient(natsServer.Conn)
	require.NoError(t, err)

	err = natsClient.Setup(ctx)
	require.NoError(t, err, "Failed to setup NATS client")

	return natsServer, natsClient
}
