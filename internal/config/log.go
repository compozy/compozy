package config

import (
	"fmt"
	"strings"
)

const (
	configLogLevelDebug = "debug"
	configLogLevelError = "error"
	configLogLevelInfo  = "info"
	configLogLevelWarn  = "warn"
)

// LogConfig controls structured logging.
type LogConfig struct {
	Level           string `toml:"level"`
	MaxSizeMB       int    `toml:"max_size_mb"`
	MaxBackups      int    `toml:"max_backups"`
	MaxAgeDays      int    `toml:"max_age_days"`
	CompressBackups bool   `toml:"compress_backups"`
}

// Validate ensures the log level is supported.
func (c LogConfig) Validate() error {
	switch strings.ToLower(strings.TrimSpace(c.Level)) {
	case configLogLevelDebug, configLogLevelInfo, configLogLevelWarn, configLogLevelError:
	default:
		return fmt.Errorf(
			"log.level must be one of %q, %q, %q, %q: %q",
			configLogLevelDebug,
			configLogLevelInfo,
			configLogLevelWarn,
			configLogLevelError,
			c.Level,
		)
	}
	if c.MaxSizeMB <= 0 {
		return fmt.Errorf("log.max_size_mb must be positive: %d", c.MaxSizeMB)
	}
	if c.MaxBackups < 0 {
		return fmt.Errorf("log.max_backups must be zero or positive: %d", c.MaxBackups)
	}
	if c.MaxAgeDays < 0 {
		return fmt.Errorf("log.max_age_days must be zero or positive: %d", c.MaxAgeDays)
	}
	return nil
}
