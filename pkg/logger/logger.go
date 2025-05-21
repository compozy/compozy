package logger

import (
	"context"
	"io"
	"os"

	charmlog "github.com/charmbracelet/log"
)

var defaultLogger *charmlog.Logger

type LogLevel = charmlog.Level

const (
	DebugLevel LogLevel = charmlog.DebugLevel
	InfoLevel  LogLevel = charmlog.InfoLevel
	WarnLevel  LogLevel = charmlog.WarnLevel
	ErrorLevel LogLevel = charmlog.ErrorLevel
)

type Config struct {
	Level      charmlog.Level
	Output     io.Writer
	JSON       bool
	AddSource  bool
	TimeFormat string
}

func DefaultConfig() *Config {
	return &Config{
		Level:      charmlog.InfoLevel,
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
		Level:           cfg.Level,
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
