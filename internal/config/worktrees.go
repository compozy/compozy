package config

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

var worktreeNamespacePattern = regexp.MustCompile(`^[a-z0-9._-]+(?:/[a-z0-9._-]+)*/$`)

const (
	worktreeConfigInvalidCode    = "worktree_config_invalid"
	positiveWorktreeValueMessage = "must be positive"
	worktreeRootConfigPath       = "worktrees.root"
	worktreeNamespaceConfigPath  = "worktrees.run_branch_namespace"
	worktreeCopyListConfigPath   = "worktrees.copy_list"
	worktreeSetupCommandPath     = "worktrees.setup_command"
	worktreeSetupTimeoutPath     = "worktrees.setup_timeout"
	worktreeDiscoveryTTLPath     = "worktrees.discovery_cache_ttl"
)

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
		return ValidationError{
			Code:    worktreeConfigInvalidCode,
			Path:    worktreeRootConfigPath,
			Message: "must be absolute or empty",
		}
	}
	if !worktreeNamespacePattern.MatchString(c.RunBranchNamespace) {
		return ValidationError{
			Code: worktreeConfigInvalidCode, Path: worktreeNamespaceConfigPath,
			Message: "must be a lowercase slash-terminated namespace",
		}
	}
	for segment := range strings.SplitSeq(strings.TrimSuffix(c.RunBranchNamespace, "/"), "/") {
		if strings.HasPrefix(segment, ".") || strings.HasSuffix(segment, ".lock") || strings.Contains(segment, "..") {
			return ValidationError{
				Code: worktreeConfigInvalidCode, Path: worktreeNamespaceConfigPath,
				Message: "must be a valid Git branch namespace",
			}
		}
	}
	for _, pattern := range c.CopyList {
		trimmedPattern := strings.TrimSpace(pattern)
		if trimmedPattern == "" {
			return ValidationError{
				Code: worktreeConfigInvalidCode, Path: worktreeCopyListConfigPath,
				Message: "entries must not be empty",
			}
		}
		if filepath.IsAbs(trimmedPattern) || copyPatternEscapesRoot(trimmedPattern) {
			return ValidationError{
				Code: worktreeConfigInvalidCode, Path: worktreeCopyListConfigPath,
				Message: "entries must stay within the repository root",
			}
		}
	}
	if c.SetupTimeout <= 0 {
		return ValidationError{
			Code:    worktreeConfigInvalidCode,
			Path:    worktreeSetupTimeoutPath,
			Message: positiveWorktreeValueMessage,
		}
	}
	if c.DiscoveryCacheTTL <= 0 {
		return ValidationError{
			Code:    worktreeConfigInvalidCode,
			Path:    worktreeDiscoveryTTLPath,
			Message: positiveWorktreeValueMessage,
		}
	}
	return nil
}

func copyPatternEscapesRoot(pattern string) bool {
	return slices.Contains(strings.Split(filepath.ToSlash(pattern), "/"), "..")
}
