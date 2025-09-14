package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupGlobalConfig_InjectsModeFromYAML(t *testing.T) {
	t.Run("YAML overrides default and is injected into context", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "compozy.yaml")
		// In tests, default is standalone; set YAML to distributed to verify override + injection
		require.NoError(t, os.WriteFile(cfgPath, []byte("mode: distributed\n"), 0o600))

		cmd := RootCmd()
		// Avoid env-file lookup errors
		require.NoError(t, cmd.PersistentFlags().Set("env-file", ""))
		require.NoError(t, cmd.PersistentFlags().Set("config", cfgPath))

		// Act
		err := SetupGlobalConfig(cmd)
		require.NoError(t, err)

		// Assert
		injected := config.ModeFrom(cmd.Context())
		assert.Equal(t, config.ModeDistributed, injected)
	})
}
