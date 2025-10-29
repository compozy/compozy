package server

import (
	"testing"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaybeStartStandaloneTemporal_ModeResolver(t *testing.T) {
	t.Run("Should skip embedded Temporal in remote/distributed mode", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		cfg := config.FromContext(ctx)
		require.NotNil(t, cfg)
		cfg.Mode = "distributed"
		cfg.Temporal.Mode = "remote"
		cleanup, err := maybeStartStandaloneTemporal(ctx)
		require.NoError(t, err)
		assert.Nil(t, cleanup)
	})
}
