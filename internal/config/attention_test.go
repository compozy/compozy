package config

import (
	"path/filepath"
	"testing"
)

// Canonical suite: attention delivery defaults and pointer overlays.
func TestAttentionConfig(t *testing.T) {
	t.Parallel()

	t.Run("Should expose the operator attention defaults", func(t *testing.T) {
		t.Parallel()
		got := DefaultAttentionConfig()
		if !got.Toasts || !got.Sound || got.System {
			t.Fatalf("DefaultAttentionConfig() = %#v, want true/true/false", got)
		}
	})

	t.Run("Should preserve unset values through a pointer overlay", func(t *testing.T) {
		t.Parallel()
		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		cfg := DefaultWithHome(homePaths)
		cfg.Attention.System = true
		overlayPath := filepath.Join(t.TempDir(), "overlay.toml")
		writeFile(t, overlayPath, "[attention]\nsound = false\n")
		if err := ApplyConfigOverlayFile(overlayPath, &cfg); err != nil {
			t.Fatalf("ApplyConfigOverlayFile() error = %v", err)
		}
		if !cfg.Attention.Toasts || cfg.Attention.Sound || !cfg.Attention.System {
			t.Fatalf("Attention after overlay = %#v, want true/false/true", cfg.Attention)
		}
	})
}
