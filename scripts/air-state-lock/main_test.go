// Suite: Air state lock process integration.
// Invariant: The lock is held only for the serialized command's lifetime, never a descendant's lifetime.
// Boundary IN: The helper binary, flock, child execution, and descriptor inheritance.
// Boundary OUT: Air orchestration and daemon lifecycle, which are covered by the final development smoke test.
package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

const descendantHelperEnv = "AGH_AIR_STATE_LOCK_DESCENDANT_HELPER"

func TestAirStateLockProcess(t *testing.T) {
	t.Run("Should hold the lock while the serialized command is running", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		binary := buildAirStateLock(t, tempDir)
		lockPath := filepath.Join(tempDir, "dev-owner.lock")
		readyPath := filepath.Join(tempDir, "command-ready")

		stdinReader, stdinWriter, err := os.Pipe()
		if err != nil {
			t.Fatalf("create command stdin pipe: %v", err)
		}
		t.Cleanup(func() {
			if err := stdinReader.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
				t.Errorf("close command stdin reader: %v", err)
			}
			if err := stdinWriter.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
				t.Errorf("close command stdin writer: %v", err)
			}
		})

		command := exec.CommandContext(
			t.Context(),
			binary,
			lockPath,
			"--",
			"sh",
			"-c",
			`printf 'ready\n' > "$1"; read -r _`,
			"air-state-lock-test",
			readyPath,
		)
		command.Stdin = stdinReader
		if err := command.Start(); err != nil {
			t.Fatalf("start lock helper: %v", err)
		}
		commandWaited := false
		t.Cleanup(func() {
			if commandWaited {
				return
			}
			if err := command.Wait(); err != nil {
				t.Errorf("wait for lock helper during cleanup: %v", err)
			}
		})
		if err := stdinReader.Close(); err != nil {
			t.Fatalf("close parent command stdin reader: %v", err)
		}

		waitForPath(t, readyPath)
		assertLockUnavailable(t, lockPath)

		if _, err := stdinWriter.WriteString("release\n"); err != nil {
			t.Fatalf("release serialized command: %v", err)
		}
		if err := stdinWriter.Close(); err != nil {
			t.Fatalf("close command stdin writer: %v", err)
		}
		if err := command.Wait(); err != nil {
			t.Fatalf("wait for lock helper: %v", err)
		}
		commandWaited = true

		assertLockAvailable(t, lockPath)
	})

	t.Run("Should release the lock when a surviving descendant outlives the command", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		binary := buildAirStateLock(t, tempDir)
		lockPath := filepath.Join(tempDir, "dev-owner.lock")
		pidPath := filepath.Join(tempDir, "descendant.pid")

		command := exec.CommandContext(
			t.Context(),
			binary,
			lockPath,
			"--",
			os.Args[0],
			"-test.run=^TestAirStateLockDescendantHelper$",
			"--",
			"spawn",
			pidPath,
		)
		command.Env = append(os.Environ(), descendantHelperEnv+"=1")
		if err := command.Run(); err != nil {
			t.Fatalf("run lock helper: %v", err)
		}

		descendantPID := readPID(t, pidPath)
		t.Cleanup(func() {
			terminateProcess(t, descendantPID)
		})
		if err := unix.Kill(descendantPID, 0); err != nil {
			t.Fatalf("descendant process is not running: %v", err)
		}

		assertLockAvailable(t, lockPath)
	})
}

func TestAirStateLockDescendantHelper(t *testing.T) {
	if os.Getenv(descendantHelperEnv) != "1" {
		return
	}

	t.Run("Should execute the requested descendant helper mode", func(t *testing.T) {
		args := helperProcessArgs(t)
		switch args[0] {
		case "spawn":
			spawnLockHoldingDescendant(t, args[1])
		case "hold":
			signals := make(chan os.Signal, 1)
			signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
			defer signal.Stop(signals)
			<-signals
		default:
			t.Fatalf("unknown descendant helper mode %q", args[0])
		}
	})
}

func helperProcessArgs(t *testing.T) []string {
	t.Helper()

	for index, arg := range os.Args {
		if arg != "--" {
			continue
		}
		args := os.Args[index+1:]
		if len(args) == 0 {
			t.Fatal("descendant helper mode is required")
		}
		if args[0] == "spawn" && len(args) != 2 {
			t.Fatalf("spawn helper args = %v, want mode and pid path", args)
		}
		return args
	}
	t.Fatal("descendant helper separator is required")
	return nil
}

func spawnLockHoldingDescendant(t *testing.T, pidPath string) {
	t.Helper()

	command := exec.CommandContext(
		context.WithoutCancel(t.Context()),
		os.Args[0],
		"-test.run=^TestAirStateLockDescendantHelper$",
		"--",
		"hold",
	)
	command.Env = append(os.Environ(), descendantHelperEnv+"=1")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		t.Fatalf("start lock-holding descendant: %v", err)
	}

	pid := command.Process.Pid
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0o600); err != nil {
		terminateStartedProcess(t, command)
		t.Fatalf("write descendant pid: %v", err)
	}
	if err := command.Process.Release(); err != nil {
		terminateStartedProcess(t, command)
		t.Fatalf("release descendant process handle: %v", err)
	}
}

func terminateStartedProcess(t *testing.T, command *exec.Cmd) {
	t.Helper()

	if err := command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		t.Errorf("kill started descendant: %v", err)
	}
	if _, err := command.Process.Wait(); err != nil {
		t.Errorf("wait for killed descendant: %v", err)
	}
}

func buildAirStateLock(t *testing.T, outputDir string) string {
	t.Helper()

	binary := filepath.Join(outputDir, "air-state-lock")
	command := exec.CommandContext(t.Context(), "go", "build", "-o", binary, ".")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("build air-state-lock helper: %v\n%s", err, output)
	}
	return binary
}

func waitForPath(t *testing.T, path string) {
	t.Helper()

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if _, err := os.Stat(path); err == nil {
			return
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("stat readiness path: %v", err)
		}

		select {
		case <-ticker.C:
		case <-timer.C:
			t.Fatalf("timed out waiting for readiness path %q", path)
		}
	}
}

func assertLockUnavailable(t *testing.T, lockPath string) {
	t.Helper()

	lockFile := openLockFile(t, lockPath)
	defer closeTestFile(t, lockFile, "lock probe")

	err := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if err == nil {
		if unlockErr := unix.Flock(int(lockFile.Fd()), unix.LOCK_UN); unlockErr != nil {
			t.Fatalf("release unexpectedly available lock: %v", unlockErr)
		}
		t.Fatal("lock is available while the serialized command is running")
	}
	if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
		t.Fatalf("probe held lock: %v", err)
	}
}

func assertLockAvailable(t *testing.T, lockPath string) {
	t.Helper()

	lockFile := openLockFile(t, lockPath)
	defer closeTestFile(t, lockFile, "lock probe")

	if err := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Fatalf("acquire released lock: %v", err)
	}
	if err := unix.Flock(int(lockFile.Fd()), unix.LOCK_UN); err != nil {
		t.Fatalf("release probed lock: %v", err)
	}
}

func openLockFile(t *testing.T, lockPath string) *os.File {
	t.Helper()

	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("open lock probe: %v", err)
	}
	return lockFile
}

func closeTestFile(t *testing.T, file *os.File, label string) {
	t.Helper()

	if err := file.Close(); err != nil {
		t.Errorf("close %s: %v", label, err)
	}
}

func readPID(t *testing.T, pidPath string) int {
	t.Helper()

	contents, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatalf("read descendant pid: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(contents)))
	if err != nil {
		t.Fatalf("parse descendant pid: %v", err)
	}
	if pid <= 0 {
		t.Fatalf("descendant pid = %d, want positive", pid)
	}
	return pid
}

func terminateProcess(t *testing.T, pid int) {
	t.Helper()

	if err := unix.Kill(pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		t.Errorf("terminate descendant process %d: %v", pid, err)
		return
	}
	if waitForProcessExit(pid, 2*time.Second) {
		return
	}
	if err := unix.Kill(pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		t.Errorf("kill descendant process %d: %v", pid, err)
		return
	}
	if !waitForProcessExit(pid, 2*time.Second) {
		t.Errorf("descendant process %d did not exit", pid)
	}
}

func waitForProcessExit(pid int, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		err := unix.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return true
		}
		if err != nil {
			return false
		}

		select {
		case <-ticker.C:
		case <-timer.C:
			return false
		}
	}
}
