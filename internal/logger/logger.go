package logger

import (
	"context"
	"io"
	"os"

	charmlog "github.com/charmbracelet/log"
)

var (
	defaultLogger *charmlog.Logger
)

// Config holds the logger configuration
type Config struct {
	Level      charmlog.Level
	Output     io.Writer
	JSON       bool
	AddSource  bool
	TimeFormat string
}

// DefaultConfig returns the default logger configuration
func DefaultConfig() *Config {
	return &Config{
		Level:      charmlog.InfoLevel,
		Output:     os.Stdout,
		JSON:       false,
		AddSource:  false,
		TimeFormat: "15:04:05",
	}
}

// Init initializes the logger with the given configuration
func Init(cfg *Config) {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	logger := charmlog.NewWithOptions(cfg.Output, charmlog.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		TimeFormat:      cfg.TimeFormat,
		Level:           cfg.Level,
	})
	if cfg.JSON {
		logger.SetFormatter(charmlog.JSONFormatter)
	} else {
		logger.SetFormatter(charmlog.TextFormatter)

		// Apply custom styles
		logger.SetStyles(getDefaultStyles())
	}
	defaultLogger = logger
}

// FromContext returns a logger with values from the context
func FromContext(ctx context.Context) *charmlog.Logger {
	return defaultLogger
}

func Debug(msg string, args ...any) {
	defaultLogger.Debug(msg, args...)
}

func Info(msg string, args ...any) {
	defaultLogger.Info(msg, args...)
}

func Warn(msg string, args ...any) {
	defaultLogger.Warn(msg, args...)
}

func Error(msg string, args ...any) {
	defaultLogger.Error(msg, args...)
}

func With(args ...any) *charmlog.Logger {
	return defaultLogger.With(args...)
}
