package config

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Canonical suite: worktrees configuration defaults, overlays, and validation.
func TestWorktreesConfig(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve zero-config defaults under Compozy home", func(t *testing.T) {
		t.Parallel()
		home := filepath.Join(t.TempDir(), "compozy-home")
		paths, err := ResolveHomePathsFrom(home)
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		got := DefaultWorktreesConfig()
		if root := got.ResolveRoot(paths); root != filepath.Join(home, "worktrees") {
			t.Fatalf("ResolveRoot() = %q, want %q", root, filepath.Join(home, "worktrees"))
		}
		if got.RunBranchNamespace != "run/" || got.SetupTimeout != 10*time.Minute ||
			got.DiscoveryCacheTTL != 30*time.Second {
			t.Fatalf("DefaultWorktreesConfig() = %#v, want run/, 10m, 30s", got)
		}
	})

	t.Run("Should overlay one worktree key without clobbering global values", func(t *testing.T) {
		t.Parallel()
		paths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		cfg := DefaultWithHome(paths)
		cfg.Worktrees.CopyList = []string{".env*"}
		overlayPath := filepath.Join(t.TempDir(), "overlay.toml")
		writeFile(t, overlayPath, "[worktrees]\nsetup_command = \"bun install\"\n")
		if err := ApplyConfigOverlayFile(overlayPath, &cfg); err != nil {
			t.Fatalf("ApplyConfigOverlayFile() error = %v", err)
		}
		if cfg.Worktrees.SetupCommand != "bun install" || len(cfg.Worktrees.CopyList) != 1 ||
			cfg.Worktrees.CopyList[0] != ".env*" {
			t.Fatalf(
				"ApplyConfigOverlayFile() Worktrees = %#v, want setup override plus preserved copy list",
				cfg.Worktrees,
			)
		}
	})

	t.Run("Should merge global and workspace worktree overlays", func(t *testing.T) {
		t.Parallel()
		workspaceRoot := t.TempDir()
		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		globalRoot := filepath.Join(t.TempDir(), "global-worktrees")
		writeFile(t, homePaths.ConfigFile, "[worktrees]\nroot = \""+globalRoot+"\"\ncopy_list = [\".env\"]\n")
		writeFile(
			t,
			filepath.Join(workspaceRoot, DirName, ConfigName),
			"[worktrees]\nsetup_command = \"bun install\"\ndiscovery_cache_ttl = \"5s\"\n",
		)

		cfg, err := LoadForHome(homePaths, WithWorkspaceRoot(workspaceRoot))
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		if cfg.Worktrees.Root != globalRoot || cfg.Worktrees.SetupCommand != "bun install" ||
			cfg.Worktrees.DiscoveryCacheTTL != 5*time.Second ||
			len(cfg.Worktrees.CopyList) != 1 || cfg.Worktrees.CopyList[0] != ".env" {
			t.Fatalf("LoadForHome() Worktrees = %#v, want merged global and workspace values", cfg.Worktrees)
		}
	})

	t.Run("Should name the exact invalid worktree key", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name   string
			mutate func(*WorktreesConfig)
			key    string
		}{
			{
				name:   "relative root",
				mutate: func(cfg *WorktreesConfig) { cfg.Root = "relative" },
				key:    "worktrees.root",
			},
			{
				name:   "namespace",
				mutate: func(cfg *WorktreesConfig) { cfg.RunBranchNamespace = "Run Branch/" },
				key:    "worktrees.run_branch_namespace",
			},
			{
				name:   "Git-invalid namespace",
				mutate: func(cfg *WorktreesConfig) { cfg.RunBranchNamespace = "feature..run/" },
				key:    "worktrees.run_branch_namespace",
			},
			{
				name:   "empty copy pattern",
				mutate: func(cfg *WorktreesConfig) { cfg.CopyList = []string{" "} },
				key:    "worktrees.copy_list",
			},
			{
				name:   "copy pattern outside the repository",
				mutate: func(cfg *WorktreesConfig) { cfg.CopyList = []string{"../secret"} },
				key:    "worktrees.copy_list",
			},
			{
				name:   "setup timeout",
				mutate: func(cfg *WorktreesConfig) { cfg.SetupTimeout = 0 },
				key:    "worktrees.setup_timeout",
			},
			{
				name:   "discovery ttl",
				mutate: func(cfg *WorktreesConfig) { cfg.DiscoveryCacheTTL = 0 },
				key:    "worktrees.discovery_cache_ttl",
			},
		}
		for _, test := range tests {
			t.Run("Should reject "+test.name, func(t *testing.T) {
				t.Parallel()
				cfg := DefaultWorktreesConfig()
				test.mutate(&cfg)
				err := cfg.Validate()
				if err == nil || !strings.Contains(err.Error(), test.key) {
					t.Fatalf("Validate() error = %v, want key %q", err, test.key)
				}
				var validationErr ValidationError
				if !errors.As(err, &validationErr) || validationErr.Code != worktreeConfigInvalidCode ||
					validationErr.Path != test.key {
					t.Fatalf("Validate() error = %#v, want structured worktree config error for %q", err, test.key)
				}
			})
		}
	})
}
