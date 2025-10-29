package config

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	pkgconfig "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	testhelpers "github.com/compozy/compozy/test/helpers"
)

// TestConfigShow_Goldens verifies mode fields appear in config show output and match goldens.
func TestConfigShow_Goldens(t *testing.T) {
	t.Run("Should match golden file for standalone config", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		mgr := pkgconfig.NewManager(ctx, pkgconfig.NewService())
		_, err := mgr.Load(ctx, pkgconfig.NewDefaultProvider(), pkgconfig.NewEnvProvider())
		require.NoError(t, err)
		cfg := mgr.Get()
		cfg.Mode = "standalone"
		cfg.Redis.Mode = "standalone"
		// Capture stdout
		r, w, _ := os.Pipe()
		old := os.Stdout
		os.Stdout = w
		t.Cleanup(func() { os.Stdout = old })
		require.NoError(t, formatConfigOutput(cfg, nil, "table", false))
		require.NoError(t, w.Close())
		out, err := io.ReadAll(r)
		require.NoError(t, err)
		testhelpers.CompareWithGolden(t, out, "testdata/config-show-standalone.golden")
	})

	t.Run("Should match golden file for mixed mode config", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		mgr := pkgconfig.NewManager(ctx, pkgconfig.NewService())
		_, err := mgr.Load(ctx, pkgconfig.NewDefaultProvider(), pkgconfig.NewEnvProvider())
		require.NoError(t, err)
		cfg := mgr.Get()
		cfg.Mode = "distributed"
		cfg.Redis.Mode = "standalone"
		// Capture stdout
		r, w, _ := os.Pipe()
		old := os.Stdout
		os.Stdout = w
		t.Cleanup(func() { os.Stdout = old })
		require.NoError(t, formatConfigOutput(cfg, nil, "table", false))
		require.NoError(t, w.Close())
		out, err := io.ReadAll(r)
		require.NoError(t, err)
		testhelpers.CompareWithGolden(t, out, "testdata/config-show-mixed.golden")
	})
}

// TestDiagnostics_EffectiveModes verifies diagnostics JSON includes effective mode resolution
func TestDiagnostics_EffectiveModes(t *testing.T) {
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	mgr := pkgconfig.NewManager(ctx, pkgconfig.NewService())
	_, err := mgr.Load(ctx, pkgconfig.NewDefaultProvider(), pkgconfig.NewEnvProvider())
	require.NoError(t, err)
	cfg := mgr.Get()
	cfg.Mode = "standalone"
	ctx = pkgconfig.ContextWithManager(ctx, mgr)
	// Capture stdout
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })
	require.NoError(t, outputDiagnosticsResults(ctx, ".", cfg, nil, true))
	require.NoError(t, w.Close())
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	testhelpers.CompareWithGolden(t, out, "testdata/config-diagnostics-standalone.golden")
}
