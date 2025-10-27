package temporal

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/worker/embedded"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/test/helpers"
)

func TestDefaultModeIsRemote(t *testing.T) {
	cfg := config.Default()
	require.Equal(t, "remote", cfg.Temporal.Mode)
	require.NotEmpty(t, cfg.Temporal.HostPort)
}

func TestStandaloneModeActivation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping temporal integration tests in short mode")
	}

	t.Helper()
	ctx := helpers.NewTestContext(t)
	cfg := config.FromContext(ctx)

	t.Run("Should activate standalone mode and run workflow", func(t *testing.T) {
		oldHostPort := "remote.example:7233"
		cfg.Temporal.HostPort = oldHostPort
		cfg.Temporal.Mode = "standalone"
		cfg.Temporal.Namespace = defaultNamespace()
		cfg.Temporal.Standalone.DatabaseFile = ":memory:"
		cfg.Temporal.Standalone.EnableUI = false
		cfg.Temporal.Standalone.Namespace = cfg.Temporal.Namespace
		cfg.Temporal.Standalone.FrontendPort = findAvailablePortRange(ctx, t, 4)
		embeddedCfg := toEmbeddedConfig(&cfg.Temporal.Standalone)
		server := startStandaloneServer(ctx, t, embeddedCfg)
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
