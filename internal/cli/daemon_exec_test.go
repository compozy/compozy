package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestSpawnDetachedDaemonProcess(t *testing.T) {
	t.Parallel()

	homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	scriptPath := filepath.Join(t.TempDir(), "compozy-test-daemon.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("os.WriteFile(script) error = %v", err)
	}

	process, err := spawnDetachedDaemonProcess(t.Context(), homePaths, func() (string, error) {
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

func TestSpawnDetachedDaemonProcessInjectsMirrorOverrideEnv(t *testing.T) {
	t.Parallel()

	t.Run("ShouldDisableStderrMirroringInDetachedChild", func(t *testing.T) {
		t.Parallel()

		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}

		scriptPath := filepath.Join(t.TempDir(), "compozy-test-daemon-env.sh")
		script := "#!/bin/sh\nprintf 'mirror=%s\\n' \"$COMPOZY_INTERNAL_LOG_MIRROR_STDERR\" >&2\nexit 1\n"
		if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
			t.Fatalf("os.WriteFile(script) error = %v", err)
		}

		process, err := spawnDetachedDaemonProcess(t.Context(), homePaths, func() (string, error) {
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
	})
}
