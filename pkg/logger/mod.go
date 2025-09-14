package logger

import (
	"context"
	"io"
	"os"
	"strings"
	"sync"

	charmlog "github.com/charmbracelet/log"
)

type LogLevel string

// ContextKey is an alias for string type
type ContextKey string

const (
	// LoggerCtxKey is the string used to extract logger
	LoggerCtxKey ContextKey = "logger"
	DebugLevel   LogLevel   = "debug"
	InfoLevel    LogLevel   = "info"
	WarnLevel    LogLevel   = "warn"
	ErrorLevel   LogLevel   = "error"
	NoLevel      LogLevel   = ""
	// DisabledLevel effectively disables all logging
	DisabledLevel LogLevel = "disabled"
)

type Logger interface {
	Debug(msg string, keyvals ...any)
	Info(msg string, keyvals ...any)
	Warn(msg string, keyvals ...any)
	Error(msg string, keyvals ...any)
	With(args ...any) Logger
}

// ContextWithLogger stores a logger in the context
func ContextWithLogger(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, LoggerCtxKey, l)
}

var defaultLogger Logger
var defaultLoggerOnce sync.Once
var defaultLoggerMu sync.RWMutex

// FromContext retrieves a logger from the context, returning a default logger if none is found
func FromContext(ctx context.Context) Logger {
    if ctx == nil {
        return getDefaultLogger()
    }
	// Try to get logger from context - check both type assertion success AND non-nil
	if l, ok := ctx.Value(LoggerCtxKey).(Logger); ok && l != nil {
		return l
	}
	// Fallback to default logger
	return getDefaultLogger()
}

// getDefaultLogger returns the singleton default logger, initializing it if needed
func getDefaultLogger() Logger {
    defaultLoggerOnce.Do(func() {
        // Initialize default logger exactly once, under lock
        l := NewLogger(nil)
        defaultLoggerMu.Lock()
        defaultLogger = l
        defaultLoggerMu.Unlock()
    })
    defaultLoggerMu.RLock()
    l := defaultLogger
    defaultLoggerMu.RUnlock()
    return l
}

// SetDefaultLogger replaces the default package logger in a thread-safe manner.
func SetDefaultLogger(l Logger) {
    defaultLoggerMu.Lock()
    defaultLogger = l
    defaultLoggerMu.Unlock()
}

type loggerImpl struct {
	charmLogger *charmlog.Logger
}

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
	case DisabledLevel:
		// Set to a very high level to disable all logging
		return charmlog.Level(1000)
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

func (l *loggerImpl) With(args ...any) Logger {
	return &loggerImpl{charmLogger: l.charmLogger.With(args...)}
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

// TestConfig returns a configuration suitable for tests (suppresses output)
func TestConfig() *Config {
	return &Config{
		Level:      DisabledLevel,
		Output:     io.Discard,
		JSON:       false,
		AddSource:  false,
		TimeFormat: "15:04:05",
	}
}

// IsTestEnvironment detects if we're running in a test environment
func IsTestEnvironment() bool {
	// Check if we're running under go test by looking for the test binary pattern
	for _, arg := range os.Args {
		if strings.HasSuffix(arg, ".test") {
			return true
		}
	}

	// Check for test-related environment variables
	if os.Getenv("GO_TEST") == "1" || os.Getenv("TESTING") == "1" {
		return true
	}

	// Check if the program name indicates a test binary
	if len(os.Args) > 0 {
		progName := os.Args[0]
		if strings.HasSuffix(progName, ".test") || strings.Contains(progName, "___") {
			return true
		}
	}

	return false
}

func NewLogger(cfg *Config) Logger {
	if cfg == nil {
		cfg = DefaultConfig()
		// Auto-detect test environment and use test config
		if IsTestEnvironment() {
			cfg = TestConfig()
		}
	}

	charmLogger := charmlog.NewWithOptions(cfg.Output, charmlog.Options{
		ReportCaller:    cfg.AddSource,
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

func NewForTests() Logger {
	return NewLogger(TestConfig())
}
