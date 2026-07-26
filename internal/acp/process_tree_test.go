//go:build !windows

package acp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/kballard/go-shellquote"
)

func TestStopTerminatesWrappedProcessTree(t *testing.T) {
	t.Run("Should stop wrapped runtime process trees", func(t *testing.T) {
		t.Parallel()

		driver := New(WithStopTimeout(100 * time.Millisecond))
		pidFile := filepath.Join(t.TempDir(), "runtime-child.pid")

		proc := startHelperProcess(t, driver, "", "", StartOpts{
			Command: helperWrapperCommand(t),
			Env: append(
				helperEnv("stream_updates", ""),
				testWrapperEnvKey+"=1",
				testWrapperPIDFileEnvKey+"="+pidFile,
			),
		})

		childPID := waitForWrapperChildPID(t, pidFile)

		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		startedAt := time.Now()
		if err := driver.Stop(stopCtx, proc); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		if elapsed := time.Since(startedAt); elapsed > time.Second {
			t.Fatalf("Stop() elapsed = %v, want <= 1s", elapsed)
		}

		waitForProcessExit(t, childPID, time.Second)
	})
}

func TestTerminalKillTerminatesWrappedProcessTree(t *testing.T) {
	t.Run("Should kill wrapped terminal process trees", func(t *testing.T) {
		t.Parallel()

		proc := newDirectProcess(t, aghconfig.PermissionModeApproveAll)
		pidFile := filepath.Join(t.TempDir(), "terminal-child.pid")

		createResult, reqErr := proc.handleInbound(
			context.Background(),
			acpsdk.ClientMethodTerminalCreate,
			mustMarshalJSON(acpsdk.CreateTerminalRequest{
				SessionId: "sess-direct",
				Command:   "sh",
				Args:      wrappedCommandArgs("sleep", "30"),
				Cwd:       new(proc.Cwd),
				Env: []acpsdk.EnvVariable{
					{Name: testWrapperPIDFileEnvKey, Value: pidFile},
				},
			}),
		)
		if reqErr != nil {
			t.Fatalf("handleInbound(create wrapped terminal) error = %v", reqErr)
		}
		createResponse, ok := createResult.(acpsdk.CreateTerminalResponse)
		if !ok {
			t.Fatalf("handleInbound(create wrapped terminal) type = %T, want CreateTerminalResponse", createResult)
		}

		childPID := waitForWrapperChildPID(t, pidFile)

		if _, reqErr := proc.handleInbound(
			context.Background(),
			acpsdk.ClientMethodTerminalKill,
			mustMarshalJSON(acpsdk.KillTerminalRequest{
				SessionId:  "sess-direct",
				TerminalId: createResponse.TerminalId,
			}),
		); reqErr != nil {
			t.Fatalf("handleInbound(kill wrapped terminal) error = %v", reqErr)
		}

		waitCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if _, err := proc.terminals.wait(waitCtx, createResponse.TerminalId); err != nil {
			t.Fatalf("terminals.wait() error = %v", err)
		}

		waitForProcessExit(t, childPID, time.Second)
	})
}

func helperWrapperCommand(t *testing.T) string {
	t.Helper()

	bin, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}

	return shellquote.Join(bin, "-test.run=TestACPWrapperProcess")
}

func wrappedCommandArgs(command string, args ...string) []string {
	wrapped := []string{
		"-c",
		wrappedCommandScript(),
		"sh",
		command,
	}
	return append(wrapped, args...)
}

func wrappedCommandScript() string {
	return strings.Join([]string{
		`"$@" &`,
		`child=$!`,
		`printf '%s\n' "$child" > "$` + testWrapperPIDFileEnvKey + `"`,
		`wait "$child"`,
	}, "\n")
}

func waitForWrapperChildPID(t *testing.T, path string) int {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			text := strings.TrimSpace(string(data))
			if text == "" {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			pid, convErr := strconv.Atoi(text)
			if convErr != nil {
				t.Fatalf("strconv.Atoi(%q) error = %v", string(data), convErr)
			}
			if pid <= 0 {
				t.Fatalf("wrapper child pid = %d, want > 0", pid)
			}
			return pid
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("os.ReadFile(%q) error = %v", path, err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for wrapper child pid file %q", path)
	return 0
}

func waitForProcessExit(t *testing.T, pid int, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("process %d is still alive after %v", pid, timeout)
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
