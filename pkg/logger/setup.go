package logger

import (
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
)

func SetupLogger(logLevel string, logJSON, logSource bool) {
	var level log.Level
	switch logLevel {
	case "debug":
		level = log.DebugLevel
	case "info":
		level = log.InfoLevel
	case "warn":
		level = log.WarnLevel
	case "error":
		level = log.ErrorLevel
	default:
		level = log.InfoLevel
	}

	// Initialize logger with development-friendly settings
	Init(&Config{
		Level:      level,
		JSON:       logJSON,
		AddSource:  logSource,
		TimeFormat: "15:04:05", // Use time format with seconds
	})
}

func GetLoggerConfig(cmd *cobra.Command) (string, bool, bool, error) {
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

	return logLevel, logJSON, logSource, nil
}
