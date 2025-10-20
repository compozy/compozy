package logger

import (
	"fmt"

	"github.com/spf13/cobra"
)

func SetupLogger(lvl LogLevel, json, source bool) Logger {
	// Create the configured logger
	l := NewLogger(&Config{
		Level:      lvl,
		JSON:       json,
		AddSource:  source,
		TimeFormat: "15:04:05", // Use time format with seconds
	})
	// Also set this as the package default so components that retrieve
	// a logger from context (and fall back to default) inherit the
	// configured level and formatting (e.g., activities without an
	// injected logger in their context).
	// This helps propagate --debug from CLI into background workers.
	SetDefaultLogger(l)
	return l
}

func GetLoggerConfig(cmd *cobra.Command) (LogLevel, bool, bool, error) {
	logLevel, err := cmd.Flags().GetString("log-level")
	if err != nil {
		return "", false, false, fmt.Errorf("failed to get log-level flag: %w", err)
	}
	logJSON, err := cmd.Flags().GetBool("log-json")
	if err != nil {
		return "", false, false, fmt.Errorf("failed to get log-json flag: %w", err)
	}
	logSource, err := cmd.Flags().GetBool("log-source")
	if err != nil {
		return "", false, false, fmt.Errorf("failed to get log-source flag: %w", err)
	}
	return LogLevel(logLevel), logJSON, logSource, nil
}
