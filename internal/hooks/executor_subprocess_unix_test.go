//go:build !windows

package hooks

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestSubprocessExecutorExecuteGracefulShutdownSignalsBeforeKill(t *testing.T) {
	t.Parallel()

	signalFile := filepath.Join(t.TempDir(), "signal.txt")
	scriptPath := filepath.Join(t.TempDir(), "trap-term.sh")
	script := strings.Join([]string{
		"#!/bin/sh",
		"set -eu",
		"trap 'printf term > \"$HOOK_SIGNAL_FILE\"; while :; do :; done' TERM",
		"while :; do :; done",
	}, "\n")
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", scriptPath, err)
	}

	executor := NewSubprocessExecutor(
		"/bin/sh",
		[]string{scriptPath},
		WithSubprocessEnv(map[string]string{"HOOK_SIGNAL_FILE": signalFile}),
	)

	_, err := executor.Execute(t.Context(), RegisteredHook{
		Name:    "graceful-timeout-hook",
		Timeout: 120 * time.Millisecond,
	}, nil)
	if err == nil {
		t.Fatal("Execute() error = nil, want timeout error")
	}

	signalBytes, readErr := os.ReadFile(signalFile)
	if readErr != nil {
		t.Fatalf("ReadFile(%q) error = %v", signalFile, readErr)
	}
	if got := string(signalBytes); got != "term" {
		t.Fatalf("signal file = %q, want %q", got, "term")
	}
}

func TestSubprocessExecutorExecuteKillsDescendantProcessesOnTimeout(t *testing.T) {
	skillDir := t.TempDir()
	pidFile := filepath.Join(skillDir, "child.pid")
	scriptPath := filepath.Join(skillDir, "spawn-child.sh")
	script := strings.Join([]string{
		"#!/bin/sh",
		"set -eu",
		"/bin/sh -c 'while :; do :; done' &",
		"child=$!",
		"printf '%s' \"$child\" > \"$HOOK_CHILD_PID_FILE\"",
		"while :; do :; done",
	}, "\n")
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", scriptPath, err)
	}

	executor := NewSubprocessExecutor(
		"/bin/sh",
		[]string{scriptPath},
		WithSubprocessEnv(map[string]string{"HOOK_CHILD_PID_FILE": pidFile}),
	)

	_, err := executor.Execute(t.Context(), RegisteredHook{
		Name:    "descendant-cleanup-hook",
		Timeout: 120 * time.Millisecond,
	}, nil)
	if err == nil {
		t.Fatal("Execute() error = nil, want timeout error")
	}

	pidBytes, readErr := os.ReadFile(pidFile)
	if readErr != nil {
		t.Fatalf("ReadFile(%q) error = %v", pidFile, readErr)
	}
	pid, atoiErr := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if atoiErr != nil {
		t.Fatalf("Atoi(%q) error = %v", strings.TrimSpace(string(pidBytes)), atoiErr)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if syscall.Kill(pid, 0) != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("child process %d still alive after subprocess timeout cleanup", pid)
}
