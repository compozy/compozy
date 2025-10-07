package helpers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

func NewTestContext(t *testing.T) context.Context {
	t.Helper()
	ctx := t.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = logger.ContextWithLogger(ctx, logger.NewForTests())
	manager := config.NewManager(config.NewService())
	_, err := manager.Load(ctx, config.NewDefaultProvider(), config.NewEnvProvider())
	require.NoError(t, err)
	ctx = config.ContextWithManager(ctx, manager)
	require.NotNil(t, logger.FromContext(ctx))
	require.NotNil(t, config.FromContext(ctx))
	t.Cleanup(func() {
		require.NoError(t, manager.Close(context.Background()))
	})
	return ctx
}
