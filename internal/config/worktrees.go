package config

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var worktreeNamespacePattern = regexp.MustCompile(`^[a-z0-9._-]+(?:/[a-z0-9._-]+)*/$`)

const worktreeConfigInvalidCode = "worktree_config_invalid"

// WorktreesConfig controls worktree placement, discovery, and bootstrap.
type WorktreesConfig struct {
	Root               string        `toml:"root"`
	RunBranchNamespace string        `toml:"run_branch_namespace"`
	CopyList           []string      `toml:"copy_list,omitempty"`
	SetupCommand       string        `toml:"setup_command"`
	SetupTimeout       time.Duration `toml:"setup_timeout"`
	DiscoveryCacheTTL  time.Duration `toml:"discovery_cache_ttl"`
}

// DefaultWorktreesConfig returns the built-in worktree defaults.
func DefaultWorktreesConfig() WorktreesConfig {
	return WorktreesConfig{
		RunBranchNamespace: "run/",
		SetupTimeout:       10 * time.Minute,
		DiscoveryCacheTTL:  30 * time.Second,
	}
}

// ResolveRoot resolves the central placement root without changing existing worktrees.
func (c WorktreesConfig) ResolveRoot(homePaths HomePaths) string {
	if strings.TrimSpace(c.Root) != "" {
		return filepath.Clean(c.Root)
	}
	return filepath.Join(homePaths.HomeDir, "worktrees")
}

// Validate rejects invalid worktree configuration and names the offending key.
func (c WorktreesConfig) Validate() error {
	if c.Root != "" && !filepath.IsAbs(c.Root) {
		return ValidationError{Code: worktreeConfigInvalidCode, Path: "worktrees.root", Message: "must be absolute or empty"}
	}
	if !worktreeNamespacePattern.MatchString(c.RunBranchNamespace) {
		return ValidationError{
			Code: worktreeConfigInvalidCode, Path: "worktrees.run_branch_namespace",
			Message: "must be a lowercase slash-terminated namespace",
		}
	}
	for _, pattern := range c.CopyList {
		if strings.TrimSpace(pattern) == "" {
			return ValidationError{
				Code: worktreeConfigInvalidCode, Path: "worktrees.copy_list",
				Message: "entries must not be empty",
			}
		}
	}
	if c.SetupTimeout <= 0 {
		return ValidationError{Code: worktreeConfigInvalidCode, Path: "worktrees.setup_timeout", Message: "must be positive"}
	}
	if c.DiscoveryCacheTTL <= 0 {
		return ValidationError{
			Code: worktreeConfigInvalidCode, Path: "worktrees.discovery_cache_ttl", Message: "must be positive",
		}
	}
	return nil
}
