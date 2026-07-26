// Package logger configures AGH structured logging.
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	loggerInfoKey = "info"
)

const mirrorToStderrEnvKey = "AGH_INTERNAL_LOG_MIRROR_STDERR"

type options struct {
	level          string
	filePath       string
	rotation       FileRotationConfig
	rotationActive bool
	mirrorStderr   bool
}

// FileRotationConfig configures the daemon log sink's file rotation policy.
type FileRotationConfig struct {
	MaxSizeMB       int
	MaxBackups      int
	MaxAgeDays      int
	CompressBackups bool
}

// Option customizes logger construction.
type Option func(*options)

// WithLevel sets the slog level using the project config value.
func WithLevel(level string) Option {
	return func(opts *options) {
		opts.level = level
	}
}

// WithFile enables JSON log output to the supplied file path.
func WithFile(path string) Option {
	return func(opts *options) {
		opts.filePath = path
	}
}

// WithFileRotation enables lumberjack-backed rotation for the structured file sink.
func WithFileRotation(rotation FileRotationConfig) Option {
	return func(opts *options) {
		opts.rotation = rotation
		opts.rotationActive = true
	}
}

// WithMirrorToStderr mirrors logs to stderr in addition to any file target.
func WithMirrorToStderr(enabled bool) Option {
	return func(opts *options) {
		opts.mirrorStderr = enabled
	}
}

// New constructs a structured logger and returns a close function for any opened file handle.
func New(opts ...Option) (*slog.Logger, func() error, error) {
	options := options{
		level:        loggerInfoKey,
		mirrorStderr: true,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}

	level, err := ParseLevel(options.level)
	if err != nil {
		return nil, nil, err
	}

	writers := make([]io.Writer, 0, 2)
	closeFn := func() error { return nil }

	if strings.TrimSpace(options.filePath) != "" {
		if err := os.MkdirAll(filepath.Dir(options.filePath), 0o755); err != nil {
			return nil, nil, fmt.Errorf("create log directory for %q: %w", options.filePath, err)
		}

		fileSink, err := openFileSink(options.filePath, options.rotation, options.rotationActive)
		if err != nil {
			return nil, nil, err
		}
		writers = append(writers, fileSink)
		closeFn = fileSink.Close
	}

	if options.mirrorStderr || len(writers) == 0 {
		writers = append(writers, os.Stderr)
	}

	output := writers[0]
	if len(writers) > 1 {
		output = io.MultiWriter(writers...)
	}

	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{
		Level: level,
	})

	return slog.New(newRedactingHandler(handler)), closeFn, nil
}

// WithRedaction wraps an injected logger with the process-snapshotted redaction engine.
func WithRedaction(log *slog.Logger) *slog.Logger {
	if log == nil {
		return nil
	}
	return slog.New(newRedactingHandler(log.Handler()))
}

func openFileSink(path string, rotation FileRotationConfig, enabled bool) (io.WriteCloser, error) {
	if enabled {
		return &lumberjack.Logger{
			Filename:   path,
			MaxSize:    rotation.MaxSizeMB,
			MaxBackups: rotation.MaxBackups,
			MaxAge:     rotation.MaxAgeDays,
			Compress:   rotation.CompressBackups,
		}, nil
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file %q: %w", path, err)
	}
	return file, nil
}

// MirrorToStderrEnabled reports whether process-local logging should still be mirrored to stderr.
func MirrorToStderrEnabled(getenv func(string) string) bool {
	if getenv == nil {
		getenv = os.Getenv
	}

	switch strings.ToLower(strings.TrimSpace(getenv(mirrorToStderrEnvKey))) {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

// WithMirrorToStderrEnv sets the detached-process env override controlling stderr mirroring.
func WithMirrorToStderrEnv(sandbox []string, enabled bool) []string {
	value := "1"
	if !enabled {
		value = "0"
	}

	prefix := mirrorToStderrEnvKey + "="
	result := make([]string, 0, len(sandbox)+1)
	replaced := false
	for _, entry := range sandbox {
		if strings.HasPrefix(entry, prefix) {
			result = append(result, prefix+value)
			replaced = true
			continue
		}
		result = append(result, entry)
	}
	if !replaced {
		result = append(result, prefix+value)
	}

	return result
}

// ParseLevel converts the configured string level into slog's level type.
func ParseLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug, nil
	case loggerInfoKey, "":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unsupported log level %q", raw)
	}
}
