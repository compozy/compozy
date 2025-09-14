package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMode_NormalizationAndValidation(t *testing.T) {
	t.Run("Should normalize whitespace and case", func(t *testing.T) {
		// Prepare a YAML config in a temp dir
		dir := t.TempDir()
		yaml := []byte("mode: \"  StandAlone  \"\n")
		cfgPath := filepath.Join(dir, "compozy.yaml")
		require.NoError(t, os.WriteFile(cfgPath, yaml, 0o600))

		m := NewManager(NewService())
		cfg, err := m.Load(context.Background(), NewDefaultProvider(), NewYAMLProvider(cfgPath))
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, ModeStandalone, cfg.Mode)
	})

	t.Run("Should reject invalid mode values", func(t *testing.T) {
		cfg := Default()
		cfg.Mode = "solo"
		svc := NewService()
		err := svc.Validate(cfg)
		require.Error(t, err)
	})
}
