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
	ShellSessionSortLastActivity ShellSessionSort = "last_activity"
	ShellSessionSortAttention    ShellSessionSort = "attention"
)

// ShellSessionScope identifies the operator's preferred session list breadth.
type ShellSessionScope string

const (
	ShellSessionScopeWorkspace     ShellSessionScope = "workspace"
	ShellSessionScopeAllWorkspaces ShellSessionScope = "all-workspaces"
)

// ShellConfig persists operator preferences owned by the web shell.
type ShellConfig struct {
	Sessions ShellSessionsConfig `toml:"sessions"`
}

// ShellSessionsConfig persists session-list preferences across shell restarts.
type ShellSessionsConfig struct {
	Sort  ShellSessionSort  `toml:"sort"`
	Scope ShellSessionScope `toml:"scope"`
}

// DefaultShellConfig returns the built-in operator shell preferences.
func DefaultShellConfig() ShellConfig {
	return ShellConfig{Sessions: ShellSessionsConfig{
		Sort:  ShellSessionSortLastActivity,
		Scope: ShellSessionScopeWorkspace,
	}}
}

// Validate rejects unsupported shell preference values.
func (c ShellConfig) Validate() error {
	switch c.Sessions.Sort {
	case ShellSessionSortLastActivity, ShellSessionSortAttention:
	default:
		return shellValidationError(
			shellSessionsSortPath,
			fmt.Sprintf(
				"must be one of %q or %q: %q",
				ShellSessionSortLastActivity,
				ShellSessionSortAttention,
				c.Sessions.Sort,
			),
		)
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
