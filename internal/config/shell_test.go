package config

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Canonical suite: shell defaults, pointer overlays, and validation.
func TestShellConfig(t *testing.T) {
	t.Parallel()

	t.Run("Should expose valid operator shell defaults", func(t *testing.T) {
		t.Parallel()

		got := DefaultShellConfig()
		if got.Sessions.Sort != ShellSessionSortCreated || got.Sessions.Scope != ShellSessionScopeWorkspace {
			t.Fatalf("DefaultShellConfig() = %#v, want created/workspace", got)
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("Validate(defaults) error = %v", err)
		}
	})

	for _, sortKey := range []ShellSessionSort{ShellSessionSortCreated, ShellSessionSortName, ShellSessionSortLastActivity, ShellSessionSortAttention} {
		t.Run("Should preserve explicitly persisted "+string(sortKey)+" ordering", func(t *testing.T) {
			t.Parallel()
			homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
			if err != nil {
				t.Fatal(err)
			}
			cfg := DefaultWithHome(homePaths)
			overlayPath := filepath.Join(t.TempDir(), "overlay.toml")
			writeFile(t, overlayPath, "[shell.sessions]\nsort = \""+string(sortKey)+"\"\n")
			if err := ApplyConfigOverlayFile(overlayPath, &cfg); err != nil {
				t.Fatal(err)
			}
			if cfg.Shell.Sessions.Sort != sortKey {
				t.Fatalf("persisted sort = %q, want %q", cfg.Shell.Sessions.Sort, sortKey)
			}
			if err := cfg.Shell.Validate(); err != nil {
				t.Fatal(err)
			}
		})
	}

	t.Run("Should preserve an omitted preference through a pointer overlay", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		cfg := DefaultWithHome(homePaths)
		cfg.Shell.Sessions.Scope = ShellSessionScopeAllWorkspaces
		overlayPath := filepath.Join(t.TempDir(), "overlay.toml")
		writeFile(t, overlayPath, "[shell.sessions]\nsort = \"attention\"\n")
		if err := ApplyConfigOverlayFile(overlayPath, &cfg); err != nil {
			t.Fatalf("ApplyConfigOverlayFile() error = %v", err)
		}
		if cfg.Shell.Sessions.Sort != ShellSessionSortAttention ||
			cfg.Shell.Sessions.Scope != ShellSessionScopeAllWorkspaces {
			t.Fatalf("Shell after overlay = %#v, want attention/all-workspaces", cfg.Shell)
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
					Sort: "priority", Scope: ShellSessionScopeWorkspace,
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
				validationErr, validationErrOK := errors.AsType[ValidationError](err)
				if !validationErrOK {
					t.Fatalf("Validate() error = %v, want ValidationError", err)
				}
				if validationErr.Path != testCase.path {
					t.Fatalf("ValidationError.Path = %q, want %q", validationErr.Path, testCase.path)
				}
			})
		}
	})
}

func TestShellGroupingCompatibility(t *testing.T) {
	t.Run("Should derive legacy grouping only until an explicit preference is present", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultShellConfig()
		legacy := ShellSessionScopeAllWorkspaces
		shellOverlay{Sessions: shellSessionsOverlay{Scope: &legacy}}.Apply(&cfg)
		if cfg.Sessions.GroupBy != ShellSessionGroupByProject || !cfg.Sessions.ScopeExplicit {
			t.Fatalf("legacy = %#v", cfg)
		}
		group := ShellSessionGroupByProfile
		shellOverlay{Sessions: shellSessionsOverlay{GroupBy: &group}}.Apply(&cfg)
		legacy = ShellSessionScopeWorkspace
		shellOverlay{Sessions: shellSessionsOverlay{Scope: &legacy}}.Apply(&cfg)
		if cfg.Sessions.GroupBy != ShellSessionGroupByProfile {
			t.Fatalf("explicit grouping overwritten: %#v", cfg)
		}
		cfg.Sessions.GroupBy = "folders"
		if err := cfg.Validate(); err == nil {
			t.Fatal("invalid grouping accepted")
		}
	})
}

func TestShellScopeDeprecationLoggedOnce(t *testing.T) {
	if os.Getenv("COMPOZY_TEST_SCOPE_WARNING") == "1" {
		for range 5 {
			if _, err := loadConfigOverlayBytes(
				[]byte("[shell.sessions]\nscope = 'all-workspaces'\n"),
				"user.toml",
			); err != nil {
				t.Fatal(err)
			}
		}
		return
	}
	t.Parallel()
	t.Run("Should warn once across repeated config decodes", func(t *testing.T) {
		t.Parallel()
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.CommandContext(t.Context(), executable, "-test.run=^TestShellScopeDeprecationLoggedOnce$")
		cmd.Env = append(os.Environ(), "COMPOZY_TEST_SCOPE_WARNING=1")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("child: %v: %s", err, output)
		}
		if count := strings.Count(string(output), ShellScopeDeprecationWarning); count != 1 {
			t.Fatalf("warning count = %d: %s", count, output)
		}
	})
}
