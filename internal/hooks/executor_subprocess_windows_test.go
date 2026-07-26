//go:build windows

package hooks

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/procutil"
)

func TestSubprocessExecutorExecuteKillsDescendantProcessesOnTimeoutWindows(t *testing.T) {
	t.Parallel()

	t.Run("Should kill descendant processes on timeout", func(t *testing.T) {
		t.Parallel()

		pidFile := filepath.Join(t.TempDir(), "child.pid")
		script := strings.Join([]string{
			"$ErrorActionPreference = 'Stop'",
			"$child = Start-Process -FilePath 'powershell.exe' -ArgumentList @('-NoProfile','-Command','Start-Sleep -Seconds 300') -PassThru",
			"Set-Content -NoNewline -Path $env:HOOK_CHILD_PID_FILE -Value $child.Id",
			"Start-Sleep -Seconds 300",
		}, "; ")

		executor := NewSubprocessExecutor(
			"powershell.exe",
			[]string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script},
			WithSubprocessEnv(map[string]string{"HOOK_CHILD_PID_FILE": pidFile}),
		)

		_, err := executor.Execute(t.Context(), RegisteredHook{
			Name:    "windows-descendant-cleanup-hook",
			Timeout: 2 * time.Second,
		}, nil)
		if err == nil {
			t.Fatal("Execute() error = nil, want timeout error")
		}

		childPID := readWindowsChildPID(t, pidFile)
		waitForWindowsChildExit(t, childPID, 3*time.Second)
	})
}

func readWindowsChildPID(t *testing.T, path string) int {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("Atoi(%q) error = %v", strings.TrimSpace(string(data)), err)
	}
	return pid
}

func waitForWindowsChildExit(t *testing.T, pid int, timeout time.Duration) {
	t.Helper()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()

	for procutil.Alive(pid) {
		select {
		case <-ticker.C:
		case <-timer.C:
			t.Fatalf("child process %d is still alive after timeout cleanup", pid)
		case <-t.Context().Done():
			t.Fatalf("timed out waiting for child process %d to exit", pid)
		}
	}
}
