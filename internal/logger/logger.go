package logger

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	defaultDaemonLogPerm     = 0o600
	defaultDaemonLogMaxBytes = 50 << 20
	defaultDaemonLogRetained = 5
)

// Mode controls daemon sink ownership for foreground and detached runs.
type Mode string

const (
	ModeForeground Mode = "foreground"
	ModeDetached   Mode = "detached"
)

// DaemonConfig configures the daemon logger sink and mirroring policy.
type DaemonConfig struct {
	FilePath         string
	Mode             Mode
	Stderr           io.Writer
	MaxFileSizeBytes int64
	MaxRetainedFiles int
	Level            slog.Leveler
}

// Runtime owns one installed daemon logger instance.
type Runtime struct {
	logger   *slog.Logger
	previous *slog.Logger
	sink     io.Closer
}

// ValidateDaemonFilePath verifies that the daemon log file can be opened.
func ValidateDaemonFilePath(path string) error {
	cleanPath, err := normalizeFilePath(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o755); err != nil {
		return fmt.Errorf("logger: create daemon log directory for %q: %w", cleanPath, err)
	}

	file, err := os.OpenFile(cleanPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, defaultDaemonLogPerm)
	if err != nil {
		return fmt.Errorf("logger: open daemon log file %q: %w", cleanPath, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("logger: close daemon log file %q: %w", cleanPath, err)
	}
	return nil
}

// InstallDaemonLogger configures and installs the process-wide daemon logger.
func InstallDaemonLogger(cfg DaemonConfig) (*Runtime, error) {
	cleanPath, err := normalizeFilePath(cfg.FilePath)
	if err != nil {
		return nil, err
	}

	sink, err := openRotatingFile(rotatingFileConfig{
		path:             cleanPath,
		maxFileSizeBytes: resolveMaxFileSize(cfg.MaxFileSizeBytes),
		maxRetainedFiles: resolveMaxRetainedFiles(cfg.MaxRetainedFiles),
		filePerm:         defaultDaemonLogPerm,
	})
	if err != nil {
		return nil, err
	}

	output := io.Writer(sink)
	if resolveMode(cfg.Mode) == ModeForeground {
		stderr := cfg.Stderr
		if stderr == nil {
			stderr = os.Stderr
		}
		output = io.MultiWriter(sink, stderr)
	}

	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{
		Level: resolveLevel(cfg.Level),
	})
	logger := slog.New(handler).With("component", "daemon")
	runtime := &Runtime{
		logger:   logger,
		previous: slog.Default(),
		sink:     sink,
	}
	slog.SetDefault(logger)
	return runtime, nil
}

// Logger returns the installed daemon logger.
func (r *Runtime) Logger() *slog.Logger {
	if r == nil {
		return nil
	}
	return r.logger
}

// Close restores the previous default logger and closes the daemon sink.
func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	if r.previous != nil {
		slog.SetDefault(r.previous)
	}
	if r.sink != nil {
		return r.sink.Close()
	}
	return nil
}

type rotatingFileConfig struct {
	path             string
	maxFileSizeBytes int64
	maxRetainedFiles int
	filePerm         os.FileMode
}

type rotatingFile struct {
	mu sync.Mutex

	path             string
	maxFileSizeBytes int64
	maxRetainedFiles int
	filePerm         os.FileMode

	file *os.File
	size int64
}

func openRotatingFile(cfg rotatingFileConfig) (*rotatingFile, error) {
	cleanPath, err := normalizeFilePath(cfg.path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o755); err != nil {
		return nil, fmt.Errorf("logger: create daemon log directory for %q: %w", cleanPath, err)
	}

	file, size, err := openLogFile(cleanPath, cfg.filePerm)
	if err != nil {
		return nil, err
	}
	return &rotatingFile{
		path:             cleanPath,
		maxFileSizeBytes: resolveMaxFileSize(cfg.maxFileSizeBytes),
		maxRetainedFiles: resolveMaxRetainedFiles(cfg.maxRetainedFiles),
		filePerm:         cfg.filePerm,
		file:             file,
		size:             size,
	}, nil
}

func (r *rotatingFile) Write(p []byte) (int, error) {
	if r == nil {
		return 0, errors.New("logger: rotating file is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.file == nil {
		return 0, errors.New("logger: rotating file is closed")
	}
	if err := r.rotateIfNeededLocked(int64(len(p))); err != nil {
		return 0, err
	}

	written, err := r.file.Write(p)
	r.size += int64(written)
	if err != nil {
		return written, fmt.Errorf("logger: write daemon log %q: %w", r.path, err)
	}
	return written, nil
}

func (r *rotatingFile) Close() error {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.file == nil {
		return nil
	}
	if err := r.file.Sync(); err != nil {
		return fmt.Errorf("logger: sync daemon log %q: %w", r.path, err)
	}
	if err := r.file.Close(); err != nil {
		return fmt.Errorf("logger: close daemon log %q: %w", r.path, err)
	}
	r.file = nil
	r.size = 0
	return nil
}

func (r *rotatingFile) rotateIfNeededLocked(writeSize int64) error {
	if r.file == nil {
		return errors.New("logger: rotating file is closed")
	}
	if r.maxFileSizeBytes <= 0 || r.size+writeSize <= r.maxFileSizeBytes {
		return nil
	}

	if err := r.file.Sync(); err != nil {
		return fmt.Errorf("logger: sync daemon log before rotation %q: %w", r.path, err)
	}
	if err := r.file.Close(); err != nil {
		return fmt.Errorf("logger: close daemon log before rotation %q: %w", r.path, err)
	}
	if err := rotateLogFiles(r.path, r.maxRetainedFiles); err != nil {
		return err
	}

	file, size, err := openLogFile(r.path, r.filePerm)
	if err != nil {
		return err
	}
	r.file = file
	r.size = size
	return nil
}

func rotateLogFiles(path string, maxRetainedFiles int) error {
	if maxRetainedFiles > 0 {
		oldest := fmt.Sprintf("%s.%d", path, maxRetainedFiles)
		if err := os.Remove(oldest); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("logger: remove rotated daemon log %q: %w", oldest, err)
		}
		for index := maxRetainedFiles - 1; index >= 1; index-- {
			source := fmt.Sprintf("%s.%d", path, index)
			target := fmt.Sprintf("%s.%d", path, index+1)
			if err := renameLogFile(source, target); err != nil {
				return err
			}
		}
	}
	if err := renameLogFile(path, path+".1"); err != nil {
		return err
	}
	return nil
}

func renameLogFile(source string, target string) error {
	if err := os.Rename(source, target); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("logger: rotate daemon log %q -> %q: %w", source, target, err)
	}
	return nil
}

func openLogFile(path string, perm os.FileMode) (*os.File, int64, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, perm)
	if err != nil {
		return nil, 0, fmt.Errorf("logger: open daemon log file %q: %w", path, err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, 0, fmt.Errorf("logger: stat daemon log file %q: %w", path, err)
	}
	return file, info.Size(), nil
}

func normalizeFilePath(path string) (string, error) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return "", errors.New("logger: daemon log file path is required")
	}
	return cleanPath, nil
}

func resolveMode(mode Mode) Mode {
	switch mode {
	case ModeDetached:
		return ModeDetached
	default:
		return ModeForeground
	}
}

func resolveMaxFileSize(size int64) int64 {
	if size > 0 {
		return size
	}
	return defaultDaemonLogMaxBytes
}

func resolveMaxRetainedFiles(count int) int {
	if count > 0 {
		return count
	}
	return defaultDaemonLogRetained
}

func resolveLevel(level slog.Leveler) slog.Leveler {
	if level != nil {
		return level
	}
	return slog.LevelInfo
}
