package logger

import (
	"context"
	"fmt"
	"io"
	"os"

	charmlog "github.com/charmbracelet/log"
)

var defaultLogger *loggerImpl

type (
	LogLevel string
	// Logger defines the interface for structured logging
	Logger interface {
		Debug(msg string, keyvals ...any)
		Info(msg string, keyvals ...any)
		Warn(msg string, keyvals ...any)
		Error(msg string, keyvals ...any)
	}

	// loggerImpl implements Logger interface using charm logger
	loggerImpl struct {
		charmLogger *charmlog.Logger
	}
)

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

func (l *loggerImpl) Debug(msg string, keyvals ...any) {
	l.charmLogger.Debug(msg, keyvals...)
}

func (l *loggerImpl) Info(msg string, keyvals ...any) {
	l.charmLogger.Info(msg, keyvals...)
}

func (l *loggerImpl) Warn(msg string, keyvals ...any) {
	l.charmLogger.Warn(msg, keyvals...)
}

func (l *loggerImpl) Error(msg string, keyvals ...any) {
	l.charmLogger.Error(msg, keyvals...)
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

func NewLogger(cfg *Config) Logger {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	charmLogger := charmlog.NewWithOptions(cfg.Output, charmlog.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		TimeFormat:      cfg.TimeFormat,
		Level:           cfg.Level.ToCharmlogLevel(),
	})
	if cfg.JSON {
		charmLogger.SetFormatter(charmlog.JSONFormatter)
	} else {
		charmLogger.SetFormatter(charmlog.TextFormatter)
		charmLogger.SetStyles(getDefaultStyles())
	}
	return &loggerImpl{charmLogger: charmLogger}
}

func Init(cfg *Config) error {
	logger := NewLogger(cfg)
	loggerImpl, ok := logger.(*loggerImpl)
	if !ok {
		return fmt.Errorf("failed to initialize logger")
	}
	defaultLogger = loggerImpl
	return nil
}

func FromContext(_ context.Context) Logger {
	return defaultLogger
}

func GetDefault() Logger {
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
	return defaultLogger.charmLogger.With(args...)
}
