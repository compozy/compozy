//go:build !windows

package subprocess

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestProcessWaitForEchoCommand(t *testing.T) {
	t.Parallel()

	process, err := Launch(context.Background(), LaunchConfig{
		Command:         shellCommand(t, "printf 'hello'"),
		WaitErrorPrefix: "wait for test subprocess",
	})
	if err != nil {
		t.Fatalf("launch process: %v", err)
	}
	if process.Stdout() == nil {
		t.Fatal("expected stdout pipe")
	}
	if err := process.Wait(); err != nil {
		t.Fatalf("wait process: %v", err)
	}
}

func TestKillTerminatesUnixProcessGroupDescendants(t *testing.T) {
	process, err := Launch(context.Background(), LaunchConfig{
		Command:           shellCommand(t, "(sleep 30) & child=$!; printf '%s\\n' \"$child\"; wait \"$child\""),
		WaitErrorPrefix:   "wait for test subprocess",
		SignalGracePeriod: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("launch process: %v", err)
	}

	reader := bufio.NewReader(process.Stdout())
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read child pid: %v", err)
	}
	childPID, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatalf("parse child pid %q: %v", line, err)
	}

	if err := process.Kill(); err != nil {
		t.Fatalf("kill process: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		err := syscall.Kill(childPID, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("child pid %d still alive after process kill", childPID)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestShutdownEscalatesFromSIGTERMToSIGKILL(t *testing.T) {
	tempDir := t.TempDir()
	markerPath := filepath.Join(tempDir, "sigterm.marker")

	process, err := Launch(context.Background(), LaunchConfig{
		Command: shellCommand(
			t,
			"trap 'printf term > "+shellQuote(markerPath)+"' TERM; while :; do sleep 1; done",
		),
		WaitErrorPrefix:   "wait for test subprocess",
		SignalGracePeriod: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("launch process: %v", err)
	}

	if err := process.Shutdown(20 * time.Millisecond); err != nil {
		t.Fatalf("shutdown process: %v", err)
	}
	if !process.Forced() {
		t.Fatal("expected forced shutdown after timeout")
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(markerPath); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("stat marker: %v", err)
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for SIGTERM marker")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestProcessHelpers(t *testing.T) {
	t.Parallel()

	env := MergeEnvironment(map[string]string{"COMPOZY_TEST_BASE": "1"}, map[string]string{"COMPOZY_TEST_EXTRA": "2"})
	if !containsPrefix(env, "COMPOZY_TEST_BASE=1") {
		t.Fatalf("missing merged base env assignment: %#v", env)
	}
	if !containsPrefix(env, "COMPOZY_TEST_EXTRA=2") {
		t.Fatalf("missing merged extra env assignment: %#v", env)
	}

	if got := NormalizeWaitError("", io.EOF); got == nil || !strings.Contains(got.Error(), "wait for subprocess") {
		t.Fatalf("unexpected default wait error: %v", got)
	}
	if got := suppressWaitDelay(exec.ErrWaitDelay); got != nil {
		t.Fatalf("expected exec.ErrWaitDelay suppression, got %v", got)
	}

	buffer := &LockedBuffer{}
	if _, err := buffer.Write([]byte("stderr")); err != nil {
		t.Fatalf("write locked buffer: %v", err)
	}
	if got := buffer.String(); got != "stderr" {
		t.Fatalf("unexpected locked buffer contents: %q", got)
	}

	var nilProcess *Process
	if nilProcess.Stdin() != nil || nilProcess.Stdout() != nil || nilProcess.StderrBuffer() != nil {
		t.Fatal("expected nil process accessors to return nil")
	}
	select {
	case <-nilProcess.Done():
	default:
		t.Fatal("expected nil process Done channel to be closed")
	}

	waiter := &Process{waitDone: make(chan struct{})}
	if waiter.waitWithin(0) {
		t.Fatal("expected open wait channel to time out immediately")
	}
	close(waiter.waitDone)
	if !waiter.waitWithin(0) {
		t.Fatal("expected closed wait channel to complete immediately")
	}
	if err := terminateProcess(nil); err != nil {
		t.Fatalf("terminate nil command: %v", err)
	}
	if err := forceTerminateProcess(nil); err != nil {
		t.Fatalf("force terminate nil command: %v", err)
	}
	if err := configureCommand(nil); err == nil {
		t.Fatal("expected nil command configuration error")
	}
}

func TestProcessCapturesStderrAndSignalsCompletion(t *testing.T) {
	t.Parallel()

	process, err := Launch(context.Background(), LaunchConfig{
		Command:         shellCommand(t, "printf 'stderr' >&2"),
		WaitErrorPrefix: "wait for test subprocess",
	})
	if err != nil {
		t.Fatalf("launch process: %v", err)
	}
	if process.Stdin() == nil {
		t.Fatal("expected stdin pipe")
	}
	if process.Stdout() == nil {
		t.Fatal("expected stdout pipe")
	}
	if process.StderrBuffer() == nil {
		t.Fatal("expected stderr buffer")
	}

	<-process.Done()
	if err := process.Wait(); err != nil {
		t.Fatalf("wait process: %v", err)
	}
	if got := process.StderrBuffer().String(); got != "stderr" {
		t.Fatalf("unexpected stderr buffer: %q", got)
	}
}

func TestLaunchUsesConfiguredWorkingDir(t *testing.T) {
	t.Parallel()

	t.Run("Should use configured working dir", func(t *testing.T) {
		t.Parallel()

		workingDir := t.TempDir()
		process, err := Launch(context.Background(), LaunchConfig{
			Command:         shellCommand(t, "pwd"),
			WorkingDir:      workingDir,
			WaitErrorPrefix: "wait for test subprocess",
		})
		if err != nil {
			t.Fatalf("launch process: %v", err)
		}

		output, err := io.ReadAll(process.Stdout())
		if err != nil {
			t.Fatalf("read stdout: %v", err)
		}
		if err := process.Wait(); err != nil {
			t.Fatalf("wait process: %v", err)
		}

		got := strings.TrimSpace(string(output))
		if got != workingDir {
			t.Fatalf("pwd output = %q, want %q", got, workingDir)
		}
	})
}

func shellCommand(t *testing.T, script string) []string {
	t.Helper()

	shellPath, err := exec.LookPath("sh")
	if err != nil {
		t.Fatalf("look up sh: %v", err)
	}
	return []string{shellPath, "-c", script}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func containsPrefix(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
