package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
	aghlogger "github.com/compozy/agh/internal/logger"
)

func TestSpawnDetachedDaemonProcess(t *testing.T) {
	t.Parallel()

	homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	scriptPath := filepath.Join(t.TempDir(), "agh-test-daemon.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("os.WriteFile(script) error = %v", err)
	}

	process, err := spawnDetachedDaemonProcess(context.Background(), homePaths, func() (string, error) {
		return scriptPath, nil
	})
	if err != nil {
		t.Fatalf("spawnDetachedDaemonProcess() error = %v", err)
	}
	if process.PID() <= 0 {
		t.Fatalf("process.PID() = %d, want positive pid", process.PID())
	}
	if err := process.Wait(); err != nil {
		t.Fatalf("process.Wait() error = %v", err)
	}
}

func TestSpawnDetachedDaemonProcessWaitIncludesStderr(t *testing.T) {
	t.Parallel()

	homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	scriptPath := filepath.Join(t.TempDir(), "agh-test-daemon-error.sh")
	script := "#!/bin/sh\nprintf 'bind failed on localhost:2123\\n' >&2\nexit 1\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("os.WriteFile(script) error = %v", err)
	}

	process, err := spawnDetachedDaemonProcess(context.Background(), homePaths, func() (string, error) {
		return scriptPath, nil
	})
	if err != nil {
		t.Fatalf("spawnDetachedDaemonProcess() error = %v", err)
	}

	waitErr := process.Wait()
	if waitErr == nil {
		t.Fatal("process.Wait() error = nil, want non-nil")
	}
	if !strings.Contains(waitErr.Error(), "bind failed on localhost:2123") {
		t.Fatalf("process.Wait() error = %v, want captured stderr", waitErr)
	}
}

func TestSpawnDetachedDaemonProcessInjectsMirrorOverrideEnv(t *testing.T) {
	t.Parallel()

	t.Run("ShouldDisableStderrMirroringInDetachedChild", func(t *testing.T) {
		t.Parallel()

		homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}

		scriptPath := filepath.Join(t.TempDir(), "agh-test-daemon-env.sh")
		script := "#!/bin/sh\nprintf 'mirror=%s\\n' \"$AGH_INTERNAL_LOG_MIRROR_STDERR\" >&2\nexit 1\n"
		if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
			t.Fatalf("os.WriteFile(script) error = %v", err)
		}

		process, err := spawnDetachedDaemonProcess(context.Background(), homePaths, func() (string, error) {
			return scriptPath, nil
		})
		if err != nil {
			t.Fatalf("spawnDetachedDaemonProcess() error = %v", err)
		}

		waitErr := process.Wait()
		if waitErr == nil {
			t.Fatal("process.Wait() error = nil, want non-nil")
		}
		if !strings.Contains(waitErr.Error(), "mirror=0") {
			t.Fatalf("process.Wait() error = %v, want detached mirror override", waitErr)
		}

		logData, err := os.ReadFile(homePaths.LogFile)
		if err != nil {
			t.Fatalf("os.ReadFile(logFile) error = %v", err)
		}
		if !strings.Contains(string(logData), "mirror=0") {
			t.Fatalf("log file = %q, want detached mirror override", string(logData))
		}

		if !strings.Contains(
			strings.Join(aghlogger.WithMirrorToStderrEnv(nil, false), "\n"),
			"AGH_INTERNAL_LOG_MIRROR_STDERR=0",
		) {
			t.Fatal("WithMirrorToStderrEnv(nil, false) did not inject AGH_INTERNAL_LOG_MIRROR_STDERR=0")
		}
	})
}
