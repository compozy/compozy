package temporal

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/worker/embedded"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/test/helpers"
)

func TestDefaultModeIsMemory(t *testing.T) {
	cfg := config.Default()
	require.Equal(t, "", cfg.Temporal.Mode)
	require.Equal(t, config.ModeMemory, cfg.EffectiveTemporalMode())
	require.NotEmpty(t, cfg.Temporal.HostPort)
}

func TestEmbeddedModeActivation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping temporal integration tests in short mode")
	}

	t.Helper()
	ctx := helpers.NewTestContext(t)
	cfg := config.FromContext(ctx)

	t.Run("Should activate embedded mode and run workflow", func(t *testing.T) {
		oldHostPort := "remote.example:7233"
		cfg.Temporal.HostPort = oldHostPort
		cfg.Temporal.Mode = config.ModePersistent
		cfg.Temporal.Namespace = defaultNamespace()
		cfg.Temporal.Standalone.DatabaseFile = filepath.Join(t.TempDir(), "temporal-mode.db")
		cfg.Temporal.Standalone.EnableUI = false
		cfg.Temporal.Standalone.Namespace = cfg.Temporal.Namespace
		cfg.Temporal.Standalone.FrontendPort = findAvailablePortRange(ctx, t, 4)
		embeddedCfg := toEmbeddedConfig(&cfg.Temporal.Standalone)
		server := startStandaloneServer(ctx, t, embeddedCfg)
		t.Cleanup(func() {
			stopTemporalServer(ctx, t, server)
		})
		cfg.Temporal.HostPort = server.FrontendAddress()
		require.NotEqual(t, oldHostPort, cfg.Temporal.HostPort)

		exec := executeTestWorkflow(ctx, t, cfg.Temporal.HostPort, cfg.Temporal.Namespace)
		require.Equal(t, strings.ToUpper(exec.Input), exec.Result)
	})
}

func toEmbeddedConfig(cfg *config.StandaloneConfig) *embedded.Config {
	if cfg == nil {
		return newEmbeddedConfigFromDefaults()
	}
	return &embedded.Config{
		DatabaseFile: cfg.DatabaseFile,
		FrontendPort: cfg.FrontendPort,
		BindIP:       cfg.BindIP,
		Namespace:    cfg.Namespace,
		ClusterName:  cfg.ClusterName,
		EnableUI:     cfg.EnableUI,
		RequireUI:    cfg.RequireUI,
		UIPort:       cfg.UIPort,
		LogLevel:     cfg.LogLevel,
		StartTimeout: cfg.StartTimeout,
	}
}
