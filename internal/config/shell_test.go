package config

import (
	"errors"
	"path/filepath"
	"testing"
)

// Canonical suite: shell defaults, pointer overlays, and validation.
func TestShellConfig(t *testing.T) {
	t.Parallel()

	t.Run("Should expose valid operator shell defaults", func(t *testing.T) {
		t.Parallel()

		got := DefaultShellConfig()
		if got.Sessions.Sort != ShellSessionSortLastActivity || got.Sessions.Scope != ShellSessionScopeRecent {
			t.Fatalf("DefaultShellConfig() = %#v, want last_activity/recent", got)
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("Validate(defaults) error = %v", err)
		}
	})

	t.Run("Should preserve an omitted preference through a pointer overlay", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		cfg := DefaultWithHome(homePaths)
		cfg.Shell.Sessions.Scope = ShellSessionScopeAll
		overlayPath := filepath.Join(t.TempDir(), "overlay.toml")
		writeFile(t, overlayPath, "[shell.sessions]\nsort = \"attention\"\n")
		if err := ApplyConfigOverlayFile(overlayPath, &cfg); err != nil {
			t.Fatalf("ApplyConfigOverlayFile() error = %v", err)
		}
		if cfg.Shell.Sessions.Sort != ShellSessionSortAttention || cfg.Shell.Sessions.Scope != ShellSessionScopeAll {
			t.Fatalf("Shell after overlay = %#v, want attention/all", cfg.Shell)
		}
	})

	t.Run("Should reject unsupported session preference values with their config path", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			cfg  ShellConfig
			path string
		}{
			{
				name: "Should reject an unsupported sort",
				cfg: ShellConfig{Sessions: ShellSessionsConfig{
					Sort: "priority", Scope: ShellSessionScopeRecent,
				}},
				path: shellSessionsSortPath,
			},
			{
				name: "Should reject an unsupported scope",
				cfg: ShellConfig{Sessions: ShellSessionsConfig{
					Sort: ShellSessionSortLastActivity, Scope: "global",
				}},
				path: shellSessionsScopePath,
			},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				err := testCase.cfg.Validate()
				var validationErr ValidationError
				if !errors.As(err, &validationErr) {
					t.Fatalf("Validate() error = %v, want ValidationError", err)
				}
				if validationErr.Path != testCase.path {
					t.Fatalf("ValidationError.Path = %q, want %q", validationErr.Path, testCase.path)
				}
			})
		}
	})
}
