package logger

import (
	"context"
	"io"
	"os"

	charmlog "github.com/charmbracelet/log"
)

var defaultLogger *charmlog.Logger

type LogLevel string
type Logger = charmlog.Logger

const (
	DebugLevel LogLevel = "debug"
	InfoLevel  LogLevel = "info"
	WarnLevel  LogLevel = "warn"
	ErrorLevel LogLevel = "error"
	NoLevel    LogLevel = ""
)

func (c *LogLevel) String() string {
	return string(*c)
}

func (c *LogLevel) ToCharmlogLevel() charmlog.Level {
	switch *c {
	case DebugLevel:
		return charmlog.DebugLevel
	case InfoLevel:
		return charmlog.InfoLevel
	case WarnLevel:
		return charmlog.WarnLevel
	case ErrorLevel:
		return charmlog.ErrorLevel
	default:
		return charmlog.InfoLevel
	}
}

type Config struct {
	Level      LogLevel
	Output     io.Writer
	JSON       bool
	AddSource  bool
	TimeFormat string
}

func DefaultConfig() *Config {
	return &Config{
		Level:      InfoLevel,
		Output:     os.Stdout,
		JSON:       false,
		AddSource:  false,
		TimeFormat: "15:04:05",
	}
}

func Init(cfg *Config) {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	logger := charmlog.NewWithOptions(cfg.Output, charmlog.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		TimeFormat:      cfg.TimeFormat,
		Level:           cfg.Level.ToCharmlogLevel(),
	})
	if cfg.JSON {
		logger.SetFormatter(charmlog.JSONFormatter)
	} else {
		logger.SetFormatter(charmlog.TextFormatter)

		logger.SetStyles(getDefaultStyles())
	}
	defaultLogger = logger
}

func FromContext(_ context.Context) *charmlog.Logger {
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
