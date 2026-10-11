package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	compozylogger "github.com/compozy/compozy/internal/logger"
)

// not parallel: detached children inherit the test mode and isolated home through the environment.
func TestSpawnDetachedDaemonProcess(t *testing.T) {
	t.Run("Should launch and reap a detached child", func(t *testing.T) {
		t.Setenv("COMPOZY_TEST_DAEMON_MODE", "success")
		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		process, err := spawnDetachedDaemonProcess(t.Context(), homePaths, os.Executable)
		if err != nil {
			t.Fatal(err)
		}
		if process.PID() <= 0 {
			t.Fatalf("pid = %d", process.PID())
		}
		if err := process.Wait(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Should rotate structured logs while detached stdio remains open", func(t *testing.T) {
		t.Setenv("COMPOZY_TEST_DAEMON_MODE", "rotate")
		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		t.Setenv("COMPOZY_HOME", homePaths.HomeDir)
		process, err := spawnDetachedDaemonProcess(t.Context(), homePaths, os.Executable)
		if err != nil {
			t.Fatal(err)
		}
		if err := process.Wait(); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(homePaths.LogFile)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "after-rotation") {
			t.Fatal("structured logging stopped after rotation")
		}
		backups, err := filepath.Glob(filepath.Join(filepath.Dir(homePaths.LogFile), "compozy-*.log"))
		if err != nil {
			t.Fatal(err)
		}
		if len(backups) == 0 {
			t.Fatal("structured log did not rotate")
		}
		stdio, err := os.ReadFile(daemonStdioLogPath(homePaths))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(stdio), "raw-stderr") || strings.Contains(string(data), "raw-stderr") {
			t.Fatalf("raw stderr was not isolated: %s", stdio)
		}
	})
}

func TestSpawnDetachedDaemonProcessInjectsMirrorOverrideEnv(t *testing.T) {
	t.Run("Should disable stderr mirroring in detached child", func(t *testing.T) {
		t.Setenv("COMPOZY_TEST_DAEMON_MODE", "mirror")
		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		process, err := spawnDetachedDaemonProcess(t.Context(), homePaths, os.Executable)
		if err != nil {
			t.Fatal(err)
		}
		waitErr := process.Wait()
		if waitErr == nil || !strings.Contains(waitErr.Error(), "mirror=0") {
			t.Fatalf("child error = %v", waitErr)
		}
		data, err := os.ReadFile(daemonStdioLogPath(homePaths))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "mirror=0") {
			t.Fatalf("stdio = %q", data)
		}
	})
}

func runDetachedDaemonTestChild() int {
	switch os.Getenv("COMPOZY_TEST_DAEMON_MODE") {
	case "success":
		return 0
	case "mirror":
		if _, err := fmt.Fprintf(
			os.Stderr,
			"mirror=%s\n",
			os.Getenv("COMPOZY_INTERNAL_LOG_MIRROR_STDERR"),
		); err != nil {
			return 2
		}
		return 1
	case "rotate":
		paths, err := compozyconfig.ResolveHomePathsFrom(os.Getenv("COMPOZY_HOME"))
		if err != nil {
			return 2
		}
		log, closeFn, err := compozylogger.New(compozylogger.WithFile(paths.LogFile),
			compozylogger.WithFileRotation(compozylogger.FileRotationConfig{MaxSizeMB: 1, MaxBackups: 2}),
			compozylogger.WithMirrorToStderr(false))
		if err != nil {
			return 2
		}
		for range 20 {
			log.Info("rotation", "payload", strings.Repeat("x", 64*1024))
		}
		log.Info("after-rotation")
		if err := closeFn(); err != nil {
			return 2
		}
		if _, err := fmt.Fprintln(os.Stderr, "raw-stderr"); err != nil {
			return 2
		}
		return 0
	default:
		return 2
	}
}
