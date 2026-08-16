package config

import (
	"errors"
	"path/filepath"
	"testing"
)

// Canonical suite: attention defaults, pointer overlays, and validation.
func TestAttentionConfig(t *testing.T) {
	t.Parallel()

	t.Run("Should expose the operator attention defaults", func(t *testing.T) {
		t.Parallel()
		got := DefaultAttentionConfig()
		if !got.Toasts || !got.Sound || got.System || len(got.MutedWorkspaces) != 0 {
			t.Fatalf("DefaultAttentionConfig() = %#v, want true/true/false/empty", got)
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("Validate(defaults) error = %v", err)
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

	t.Run("Should name malformed muted workspace entries", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultAttentionConfig()
		cfg.MutedWorkspaces = []string{"not-a-workspace-id"}
		err := cfg.Validate()
		var validationErr ValidationError
		if !errors.As(err, &validationErr) {
			t.Fatalf("Validate() error = %v, want ValidationError", err)
		}
		if validationErr.Path != attentionMutedPath {
			t.Fatalf("ValidationError.Path = %q, want %q", validationErr.Path, attentionMutedPath)
		}
	})
}
