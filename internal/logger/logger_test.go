package logger

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallDaemonLoggerForegroundMirrorsToStderrAndFile(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "daemon.log")
	var stderr bytes.Buffer

	previous := slog.Default()
	runtime, err := InstallDaemonLogger(DaemonConfig{
		FilePath: logPath,
		Mode:     ModeForeground,
		Stderr:   &stderr,
	})
	if err != nil {
		t.Fatalf("InstallDaemonLogger() error = %v", err)
	}

	runtime.Logger().Info("foreground started", "force", false)
	if err := runtime.Close(); err != nil {
		t.Fatalf("Runtime.Close() error = %v", err)
	}

	if slog.Default() != previous {
		t.Fatal("expected Runtime.Close to restore the previous default logger")
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", logPath, err)
	}
	if !strings.Contains(string(data), `"msg":"foreground started"`) {
		t.Fatalf("foreground log file missing JSON record:\n%s", data)
	}
	if !strings.Contains(stderr.String(), `"msg":"foreground started"`) {
		t.Fatalf("foreground stderr missing mirrored JSON record:\n%s", stderr.String())
	}
}

func TestInstallDaemonLoggerDetachedWritesOnlyFile(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "daemon.log")
	var stderr bytes.Buffer

	runtime, err := InstallDaemonLogger(DaemonConfig{
		FilePath: logPath,
		Mode:     ModeDetached,
		Stderr:   &stderr,
	})
	if err != nil {
		t.Fatalf("InstallDaemonLogger() error = %v", err)
	}

	runtime.Logger().Info("detached started", "force", true)
	if err := runtime.Close(); err != nil {
		t.Fatalf("Runtime.Close() error = %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", logPath, err)
	}
	if !strings.Contains(string(data), `"msg":"detached started"`) {
		t.Fatalf("detached log file missing JSON record:\n%s", data)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected detached mode to avoid stderr mirroring, got %q", stderr.String())
	}
}

func TestOpenRotatingFileRotatesAtConfiguredSize(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "daemon.log")
	writer, err := openRotatingFile(rotatingFileConfig{
		path:             logPath,
		maxFileSizeBytes: 32,
		maxRetainedFiles: 2,
		filePerm:         defaultDaemonLogPerm,
	})
	if err != nil {
		t.Fatalf("openRotatingFile() error = %v", err)
	}

	if _, err := writer.Write([]byte("first log line that exceeds the rotation threshold\n")); err != nil {
		t.Fatalf("Write(first) error = %v", err)
	}
	if _, err := writer.Write([]byte("second log line that exceeds the rotation threshold\n")); err != nil {
		t.Fatalf("Write(second) error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("Stat(%q) error = %v", logPath, err)
	}
	if _, err := os.Stat(logPath + ".1"); err != nil {
		t.Fatalf("Stat(%q) error = %v", logPath+".1", err)
	}
}

func TestOpenRotatingFileKeepsWritingAfterRotationFailure(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "daemon.log")
	writer, err := openRotatingFile(rotatingFileConfig{
		path:             logPath,
		maxFileSizeBytes: 16,
		maxRetainedFiles: 2,
		filePerm:         defaultDaemonLogPerm,
	})
	if err != nil {
		t.Fatalf("openRotatingFile() error = %v", err)
	}
	defer func() {
		_ = writer.Close()
	}()

	if _, err := writer.Write([]byte("seed-entry\n")); err != nil {
		t.Fatalf("Write(seed) error = %v", err)
	}

	blocker := logPath + ".2"
	if err := os.Mkdir(blocker, 0o755); err != nil {
		t.Fatalf("Mkdir(%q) error = %v", blocker, err)
	}
	if err := os.WriteFile(filepath.Join(blocker, "keep"), []byte("block"), 0o600); err != nil {
		t.Fatalf("WriteFile(blocker) error = %v", err)
	}

	if _, err := writer.Write([]byte("rotate-now\n")); err == nil {
		t.Fatal("Write(rotation failure) error = nil, want non-nil")
	}

	if err := os.Remove(filepath.Join(blocker, "keep")); err != nil {
		t.Fatalf("Remove(blocker file) error = %v", err)
	}
	if err := os.Remove(blocker); err != nil {
		t.Fatalf("Remove(%q) error = %v", blocker, err)
	}

	if _, err := writer.Write([]byte("rotate-ok\n")); err != nil {
		t.Fatalf("Write(after blocker removed) error = %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("Stat(%q) error = %v", logPath, err)
	}
	if _, err := os.Stat(logPath + ".1"); err != nil {
		t.Fatalf("Stat(%q) error = %v", logPath+".1", err)
	}
}

func TestValidateDaemonFilePathRejectsEmptyPath(t *testing.T) {
	if err := ValidateDaemonFilePath(" "); err == nil {
		t.Fatal("ValidateDaemonFilePath(empty) error = nil, want non-nil")
	}
}

func TestNormalizeFilePathCleansRelativeSegments(t *testing.T) {
	t.Parallel()

	got, err := normalizeFilePath("  " + filepath.Join("logs", "..", "daemon.log") + "  ")
	if err != nil {
		t.Fatalf("normalizeFilePath() error = %v", err)
	}
	if want := filepath.Clean(filepath.Join("logs", "..", "daemon.log")); got != want {
		t.Fatalf("normalizeFilePath() = %q, want %q", got, want)
	}
}
