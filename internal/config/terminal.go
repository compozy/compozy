package config

import (
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultTerminalScrollbackBytes        = 1 << 20
	DefaultTerminalDetachedTTL            = 24 * time.Hour
	DefaultTerminalExitRetention          = 15 * time.Minute
	DefaultTerminalRecordingRetentionDays = 30
	DefaultTerminalMaxPerWorkspace        = 8
	DefaultTerminalMaxPerDaemon           = 32
	DefaultTerminalMaxSubscribers         = 16
)

// TerminalConfig controls terminal process, retention, and capacity policy.
type TerminalConfig struct {
	DefaultShell           string        `toml:"default_shell"`
	ShellIntegration       bool          `toml:"shell_integration"`
	ScrollbackBytes        int           `toml:"scrollback_bytes"`
	DetachedTTL            time.Duration `toml:"detached_ttl"`
	ExitRetention          time.Duration `toml:"exit_retention"`
	Recording              bool          `toml:"recording"`
	RecordingRetentionDays int           `toml:"recording_retention_days"`
	MaxPerWorkspace        int           `toml:"max_per_workspace"`
	MaxPerDaemon           int           `toml:"max_per_daemon"`
	MaxSubscribers         int           `toml:"max_subscribers"`
}

// DefaultTerminalConfig returns the built-in terminal policy.
func DefaultTerminalConfig() TerminalConfig {
	return TerminalConfig{
		ShellIntegration:       true,
		ScrollbackBytes:        DefaultTerminalScrollbackBytes,
		DetachedTTL:            DefaultTerminalDetachedTTL,
		ExitRetention:          DefaultTerminalExitRetention,
		RecordingRetentionDays: DefaultTerminalRecordingRetentionDays,
		MaxPerWorkspace:        DefaultTerminalMaxPerWorkspace,
		MaxPerDaemon:           DefaultTerminalMaxPerDaemon,
		MaxSubscribers:         DefaultTerminalMaxSubscribers,
	}
}

// Validate checks terminal policy without probing the local executable set.
func (c TerminalConfig) Validate() error {
	if err := validateTerminalShell(c.DefaultShell); err != nil {
		return err
	}
	checks := []struct {
		path  string
		value int
	}{
		{"terminal.scrollback_bytes", c.ScrollbackBytes},
		{"terminal.recording_retention_days", c.RecordingRetentionDays},
		{"terminal.max_per_workspace", c.MaxPerWorkspace},
		{"terminal.max_per_daemon", c.MaxPerDaemon},
		{"terminal.max_subscribers", c.MaxSubscribers},
	}
	for _, check := range checks {
		if check.value <= 0 {
			return ValidationError{Path: check.path, Message: "must be greater than zero"}
		}
	}
	if c.DetachedTTL <= 0 {
		return ValidationError{Path: "terminal.detached_ttl", Message: "must be greater than zero"}
	}
	if c.ExitRetention <= 0 {
		return ValidationError{Path: "terminal.exit_retention", Message: "must be greater than zero"}
	}
	return nil
}

func validateTerminalShell(shell string) error {
	if shell == "" {
		return nil
	}
	if strings.ContainsRune(shell, '\x00') || strings.TrimSpace(shell) != shell {
		return ValidationError{Path: "terminal.default_shell", Message: "must be a bare executable name or absolute path"}
	}
	if strings.ContainsAny(shell, `/\\`) && !filepath.IsAbs(shell) {
		return ValidationError{Path: "terminal.default_shell", Message: "must be a bare executable name or absolute path"}
	}
	return nil
}
