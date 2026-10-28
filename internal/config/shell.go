package config

import "fmt"

const (
	shellConfigInvalidCode = "shell_config_invalid"
	shellSessionsSortPath  = "shell.sessions.sort"
	shellSessionsScopePath = "shell.sessions.scope"
)

// ShellSessionSort identifies the operator's preferred session ordering.
type ShellSessionSort string

const (
	ShellSessionSortCreated      ShellSessionSort = "created"
	ShellSessionSortName         ShellSessionSort = "name"
	ShellSessionSortLastActivity ShellSessionSort = "last_activity"
	ShellSessionSortAttention    ShellSessionSort = "attention"
	ShellSessionSortManual       ShellSessionSort = "manual"
)

// ShellSessionScope identifies the operator's preferred session list breadth.
type ShellSessionScope string

const (
	ShellSessionScopeWorkspace     ShellSessionScope = "workspace"
	ShellSessionScopeAllWorkspaces ShellSessionScope = "all-workspaces"
)

// ShellConfig persists operator preferences owned by the web shell.
type ShellConfig struct {
	Sessions   ShellSessionsConfig   `toml:"sessions"`
	Appearance ShellAppearanceConfig `toml:"appearance"`
}

// ShellSessionsConfig persists session-list preferences across shell restarts.
type ShellSessionsConfig struct {
	GroupBy         ShellSessionGroupBy `toml:"group_by"`
	GroupByExplicit bool                `toml:"-"        json:"-"`
	ScopeExplicit   bool                `toml:"-"        json:"-"`
	Sort            ShellSessionSort    `toml:"sort"`
	Scope           ShellSessionScope   `toml:"scope"`
}

// DefaultShellConfig returns the built-in operator shell preferences.
func DefaultShellConfig() ShellConfig {
	return ShellConfig{Sessions: ShellSessionsConfig{
		Sort:    ShellSessionSortCreated,
		GroupBy: ShellSessionGroupByNone,
		Scope:   ShellSessionScopeWorkspace,
	}}
}

// Validate rejects unsupported shell preference values.
func (c ShellConfig) Validate() error {
	if err := c.Appearance.Validate(); err != nil {
		return err
	}
	switch c.Sessions.Sort {
	case ShellSessionSortCreated, ShellSessionSortName, ShellSessionSortLastActivity, ShellSessionSortAttention,
		ShellSessionSortManual:
	default:
		return shellValidationError(
			shellSessionsSortPath,
			fmt.Sprintf(
				"must be one of %q, %q, %q, %q or %q: %q",
				ShellSessionSortCreated,
				ShellSessionSortName,
				ShellSessionSortLastActivity,
				ShellSessionSortAttention,
				ShellSessionSortManual,
				c.Sessions.Sort,
			),
		)
	}
	switch c.Sessions.GroupBy {
	case "":
		if c.Sessions.GroupByExplicit {
			return shellValidationError("shell.sessions.group_by", "must be none, project or profile")
		}
	case ShellSessionGroupByNone, ShellSessionGroupByProject, ShellSessionGroupByProfile:
	default:
		return shellValidationError("shell.sessions.group_by", "must be none, project or profile")
	}
	switch c.Sessions.Scope {
	case ShellSessionScopeWorkspace, ShellSessionScopeAllWorkspaces:
		return nil
	default:
		return shellValidationError(
			shellSessionsScopePath,
			fmt.Sprintf(
				"must be one of %q or %q: %q",
				ShellSessionScopeWorkspace,
				ShellSessionScopeAllWorkspaces,
				c.Sessions.Scope,
			),
		)
	}
}

func shellValidationError(path string, message string) error {
	return ValidationError{Code: shellConfigInvalidCode, Path: path, Message: message}
}

type ShellSessionGroupBy string

const (
	ShellSessionGroupByNone    ShellSessionGroupBy = "none"
	ShellSessionGroupByProject ShellSessionGroupBy = "project"
	ShellSessionGroupByProfile ShellSessionGroupBy = "profile"
)

// Scope remains functional through v0.4.0; remove this translation in v0.5.0.
const ShellScopeDeprecationWarning = "shell.sessions.scope is deprecated; use shell.sessions.group_by for grouping " +
	"and the topbar globe for All projects (removed in v0.5.0)"

func (c *ShellSessionsConfig) ResolveGroupBy() {
	if c.GroupByExplicit {
		return
	}
	c.GroupBy = ShellSessionGroupByNone
	if c.Scope == ShellSessionScopeAllWorkspaces {
		c.GroupBy = ShellSessionGroupByProject
	}
}
