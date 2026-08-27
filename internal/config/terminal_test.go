package config

// Suite: terminal configuration lifecycle.
// Invariant: all ten settings default, overlay, validate, and clone through the owning config layer.
// Boundary IN: TOML and runtime config copies. Boundary OUT: validated TerminalConfig values.

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTerminalConfigLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should expose every documented default [UT-060]", func(t *testing.T) {
		t.Parallel()
		got := DefaultTerminalConfig()
		if got.DefaultShell != "" || !got.ShellIntegration || got.ScrollbackBytes != 1<<20 ||
			got.DetachedTTL != 24*time.Hour || got.ExitRetention != 15*time.Minute || got.Recording ||
			got.RecordingRetentionDays != 30 || got.MaxPerWorkspace != 8 || got.MaxPerDaemon != 32 ||
			got.MaxSubscribers != 16 {
			t.Fatalf("DefaultTerminalConfig() = %#v", got)
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("Validate(defaults) error = %v", err)
		}
	})

	t.Run(
		"Should validate values by exact key while treating shell availability as runtime policy [UT-061]",
		func(t *testing.T) {
			t.Parallel()
			cases := []struct {
				name string
				path string
				edit func(*TerminalConfig)
			}{
				{
					name: "Should reject zero workspace cap",
					path: "terminal.max_per_workspace",
					edit: func(c *TerminalConfig) { c.MaxPerWorkspace = 0 },
				},
				{
					name: "Should reject negative detached ttl",
					path: "terminal.detached_ttl",
					edit: func(c *TerminalConfig) { c.DetachedTTL = -time.Second },
				},
				{
					name: "Should reject a relative shell path",
					path: "terminal.default_shell",
					edit: func(c *TerminalConfig) { c.DefaultShell = "bin/sh" },
				},
				{
					name: "Should reject a NUL shell",
					path: "terminal.default_shell",
					edit: func(c *TerminalConfig) { c.DefaultShell = "sh\x00bad" },
				},
			}
			for _, testCase := range cases {
				t.Run(testCase.name, func(t *testing.T) {
					t.Parallel()
					config := DefaultTerminalConfig()
					testCase.edit(&config)
					assertTerminalValidationPath(t, config.Validate(), testCase.path)
				})
			}
			validMissing := DefaultTerminalConfig()
			validMissing.DefaultShell = "/not/installed/but/well-formed"
			if err := validMissing.Validate(); err != nil {
				t.Fatalf("Validate(well-formed unavailable shell) error = %v", err)
			}
		},
	)

	t.Run("Should deny only the daemon cap in profile overlays", func(t *testing.T) {
		t.Parallel()
		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		config := DefaultWithHome(homePaths)
		allowed := filepath.Join(t.TempDir(), "allowed.toml")
		writeFile(t, allowed, "[terminal]\nmax_per_workspace = 4\n")
		if err := applyProfileConfigOverlayFile(allowed, &config, "test-profile"); err != nil {
			t.Fatalf("profile max_per_workspace overlay error = %v", err)
		}
		if config.Terminal.MaxPerWorkspace != 4 {
			t.Fatalf("MaxPerWorkspace = %d, want 4", config.Terminal.MaxPerWorkspace)
		}

		denied := filepath.Join(t.TempDir(), "denied.toml")
		writeFile(t, denied, "[terminal]\nmax_per_daemon = 64\n")
		err = applyProfileConfigOverlayFile(denied, &config, "test-profile")
		var validationErr ValidationError
		if !errors.As(err, &validationErr) || validationErr.Code != profileConfigKeyDeniedCode ||
			validationErr.Path != "terminal.max_per_daemon" {
			t.Fatalf("profile max_per_daemon overlay error = %#v", err)
		}
	})

	t.Run("Should merge terminal overlays without replacing unrelated values [UT-062]", func(t *testing.T) {
		t.Parallel()
		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		config := DefaultWithHome(homePaths)
		config.Terminal.MaxSubscribers = 9
		path := filepath.Join(t.TempDir(), "workspace.toml")
		writeFile(t, path, "[terminal]\ndetached_ttl = \"2h\"\n")
		if err := ApplyConfigOverlayFile(path, &config); err != nil {
			t.Fatalf("ApplyConfigOverlayFile() error = %v", err)
		}
		if config.Terminal.DetachedTTL != 2*time.Hour || config.Terminal.MaxSubscribers != 9 ||
			config.Terminal.MaxPerWorkspace != 8 {
			t.Fatalf("merged Terminal = %#v", config.Terminal)
		}
	})

	t.Run("Should clone terminal settings without sharing active state [UT-063]", func(t *testing.T) {
		t.Parallel()
		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		active := DefaultWithHome(homePaths)
		cloned := CloneConfig(&active)
		cloned.Terminal.DefaultShell = "fish"
		cloned.Terminal.MaxSubscribers = 3
		if active.Terminal.DefaultShell == cloned.Terminal.DefaultShell ||
			active.Terminal.MaxSubscribers == cloned.Terminal.MaxSubscribers {
			t.Fatalf("CloneConfig() aliased terminal config: active=%#v clone=%#v", active.Terminal, cloned.Terminal)
		}
	})
}

func assertTerminalValidationPath(t *testing.T, err error, want string) {
	t.Helper()
	var validationErr ValidationError
	if !errors.As(err, &validationErr) || validationErr.Path != want || !strings.Contains(err.Error(), want) {
		t.Fatalf("Validate() error = %#v, want path %q", err, want)
	}
}
