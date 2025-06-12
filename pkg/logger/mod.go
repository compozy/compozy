package logger

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

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
	// DisabledLevel effectively disables all logging
	DisabledLevel LogLevel = "disabled"
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
	// Check if we're running under go test
	for _, arg := range os.Args {
		if strings.Contains(arg, "test") || strings.HasSuffix(arg, ".test") {
			return true
		}
	}

	// Check for test-related environment variables
	if os.Getenv("GO_TEST") == "1" || os.Getenv("TESTING") == "1" {
		return true
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

func Init(cfg *Config) error {
	logger := NewLogger(cfg)
	loggerImpl, ok := logger.(*loggerImpl)
	if !ok {
		return fmt.Errorf("failed to initialize logger")
	}
	defaultLogger = loggerImpl
	return nil
}

// InitForTests initializes the logger with test-friendly settings
func InitForTests() error {
	return Init(TestConfig())
}

// DisableLogging completely disables logging by setting output to io.Discard
func DisableLogging() {
	if defaultLogger != nil {
		defaultLogger.charmLogger.SetOutput(io.Discard)
		defaultLogger.charmLogger.SetLevel(charmlog.Level(1000)) // Very high level
	}
}

// EnableLogging re-enables logging with the given config
func EnableLogging(cfg *Config) error {
	return Init(cfg)
}

// SetupTestLogger is a helper for tests that automatically disables logging
// Usage in tests: defer logger.SetupTestLogger(t)()
func SetupTestLogger(t *testing.T) func() {
	originalLogger := defaultLogger

	// Initialize with test config
	if err := InitForTests(); err != nil {
		t.Fatalf("failed to initialize test logger: %v", err)
	}

	// Return cleanup function
	return func() {
		defaultLogger = originalLogger
	}
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
