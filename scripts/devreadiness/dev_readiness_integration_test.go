//go:build !windows

// Suite: Development process readiness integration.
// Invariant: Vite starts only after the current Air run reports a ready daemon exactly once.
// Boundary IN: Bash supervision, FIFO events, Air lifecycle, daemon replacement, and Vite launch ordering.
// Boundary OUT: Daemon HTTP health and browser behavior, which are covered by isolated QA.
package devreadiness

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestDevDaemonReadiness(t *testing.T) {
	t.Run("Should publish the started binary identity exactly once for the current run", func(t *testing.T) {
		t.Parallel()

		repoRoot := repositoryRoot(t)
		tempDir := t.TempDir()
		fifoPath := createFIFO(t, tempDir, "daemon-ready.fifo")
		markerPath := filepath.Join(tempDir, "ready-sent")
		stateDir := filepath.Join(tempDir, "state")
		startLog := filepath.Join(tempDir, "daemon-started")
		binaryPath := writeExecutable(t, tempDir, "agh", `#!/usr/bin/env bash
set -euo pipefail
case "${1:-} ${2:-}" in
  "daemon stop")
    printf '%s\n' 'cli: daemon is not running' >&2
    exit 1
    ;;
  "daemon start")
    printf '%s\n' 'started' > "$FAKE_DAEMON_START_LOG"
    ;;
  *)
    printf 'unexpected daemon command: %s %s\n' "${1:-}" "${2:-}" >&2
    exit 2
    ;;
esac
`)

		const runID = "run-current"
		awaitContext, cancelAwait := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancelAwait()
		awaitCommand := exec.CommandContext(
			awaitContext,
			"bash",
			filepath.Join(repoRoot, "scripts", "dev-readiness.sh"),
			"await",
			fifoPath,
			runID,
		)
		var readyOutput bytes.Buffer
		var awaitErrorOutput bytes.Buffer
		awaitCommand.Stdout = &readyOutput
		awaitCommand.Stderr = &awaitErrorOutput
		if err := awaitCommand.Start(); err != nil {
			t.Fatalf("start readiness waiter: %v", err)
		}

		runDevDaemon(t, repoRoot, binaryPath, stateDir, fifoPath, markerPath, startLog, runID)
		if err := awaitCommand.Wait(); err != nil {
			t.Fatalf("wait for daemon readiness: %v; stderr=%s", err, awaitErrorOutput.String())
		}

		wantBinaryID := checksumIdentity(t, binaryPath)
		if got := strings.TrimSpace(readyOutput.String()); got != wantBinaryID {
			t.Fatalf("ready binary identity = %q, want %q", got, wantBinaryID)
		}
		started, err := os.ReadFile(startLog)
		if err != nil {
			t.Fatalf("read daemon start log: %v", err)
		}
		if got := strings.TrimSpace(string(started)); got != "started" {
			t.Fatalf("daemon start log = %q, want started", got)
		}

		secondContext, cancelSecond := context.WithTimeout(t.Context(), 500*time.Millisecond)
		defer cancelSecond()
		secondCommand := devDaemonCommand(
			secondContext,
			repoRoot,
			binaryPath,
			stateDir,
			fifoPath,
			markerPath,
			startLog,
			runID,
		)
		if output, err := secondCommand.CombinedOutput(); err != nil {
			t.Fatalf("repeat dev-daemon run: %v; output=%s", err, output)
		}
	})

	t.Run("Should ignore stale readiness and propagate current Air failure", func(t *testing.T) {
		t.Parallel()

		repoRoot := repositoryRoot(t)
		tempDir := t.TempDir()
		fifoPath := createFIFO(t, tempDir, "stale-ready.fifo")
		channelKeeper, err := os.OpenFile(fifoPath, os.O_RDWR, 0)
		if err != nil {
			t.Fatalf("open readiness FIFO: %v", err)
		}
		t.Cleanup(func() {
			if err := channelKeeper.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
				t.Errorf("close readiness FIFO: %v", err)
			}
		})
		readinessScript := filepath.Join(repoRoot, "scripts", "dev-readiness.sh")

		awaitContext, cancelAwait := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancelAwait()
		awaitCommand := exec.CommandContext(awaitContext, "bash", readinessScript, "await", fifoPath, "run-current")
		var awaitErrorOutput bytes.Buffer
		awaitCommand.Stderr = &awaitErrorOutput
		if err := awaitCommand.Start(); err != nil {
			t.Fatalf("start readiness waiter: %v", err)
		}

		staleContext, cancelStale := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancelStale()
		staleCommand := exec.CommandContext(
			staleContext,
			"bash",
			readinessScript,
			"ready",
			fifoPath,
			"run-stale",
			filepath.Join(tempDir, "stale-sent"),
			"stale-binary",
		)
		if output, err := staleCommand.CombinedOutput(); err != nil {
			t.Fatalf("publish stale readiness: %v; output=%s", err, output)
		}

		exitContext, cancelExit := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancelExit()
		exitCommand := exec.CommandContext(
			exitContext,
			"bash",
			readinessScript,
			"air-exit",
			fifoPath,
			"run-current",
			"37",
		)
		if output, err := exitCommand.CombinedOutput(); err != nil {
			t.Fatalf("publish Air exit: %v; output=%s", err, output)
		}

		err = awaitCommand.Wait()
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) || exitError.ExitCode() != 37 {
			t.Fatalf("readiness waiter error = %v, want exit status 37; stderr=%s", err, awaitErrorOutput.String())
		}
	})
}

func TestDevSupervisorReadiness(t *testing.T) {
	t.Run("Should start Vite only after the current Air run reports daemon readiness", func(t *testing.T) {
		t.Parallel()

		repoRoot := repositoryRoot(t)
		tempDir := t.TempDir()
		fakeBinDir := filepath.Join(tempDir, "bin")
		if err := os.MkdirAll(fakeBinDir, 0o755); err != nil {
			t.Fatalf("create fake bin directory: %v", err)
		}
		stateDir := filepath.Join(tempDir, "state")
		airCacheDir := filepath.Join(tempDir, "air-cache")
		airInstallDir := filepath.Join(airCacheDir, "test-version")
		if err := os.MkdirAll(airInstallDir, 0o755); err != nil {
			t.Fatalf("create fake Air install directory: %v", err)
		}

		airStartedFIFO := createFIFO(t, tempDir, "air-started.fifo")
		viteStartedFIFO := createFIFO(t, tempDir, "vite-started.fifo")
		readyGateFIFO := createFIFO(t, tempDir, "ready-gate.fifo")
		holdFIFO := createFIFO(t, tempDir, "hold.fifo")
		holdFile, err := os.OpenFile(holdFIFO, os.O_RDWR, 0)
		if err != nil {
			t.Fatalf("open process hold FIFO: %v", err)
		}
		t.Cleanup(func() {
			if err := holdFile.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
				t.Errorf("close process hold FIFO: %v", err)
			}
		})

		writeExecutable(t, fakeBinDir, "go", `#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "run" && "${2:-}" == "./scripts/air-state-dir" ]]; then
  printf '%s\n' "$AGH_TEST_AIR_STATE_DIR"
  exit 0
fi
if [[ "${1:-}" == "run" && "${2:-}" == "./scripts/air-state-lock" ]]; then
  while (($# > 0)) && [[ "$1" != "--" ]]; do shift; done
  if (($# == 0)); then
    printf '%s\n' 'missing serialized command' >&2
    exit 2
  fi
  shift
  exec "$@"
fi
printf 'unexpected go command: %s\n' "$*" >&2
exit 2
`)
		writeExecutable(t, fakeBinDir, "bun", `#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "scripts/find-dev-port.mjs" ]]; then
  printf '%s\n' '3000'
  exit 0
fi
if [[ "${1:-}" == "run" ]]; then
  printf '%s\n' 'vite-started' > "$AGH_TEST_VITE_STARTED_FIFO"
  trap 'exit 143' INT TERM
  read -r _ < "$AGH_TEST_HOLD_FIFO"
  exit 0
fi
printf 'unexpected bun command: %s\n' "$*" >&2
exit 2
`)
		writeExecutable(t, airInstallDir, "air", `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' 'air-started' > "$AGH_TEST_AIR_STARTED_FIFO"
read -r _ < "$AGH_TEST_READY_GATE_FIFO"
bash "$AGH_TEST_REPO_ROOT/scripts/dev-readiness.sh" ready \
  "$AGH_AIR_READY_FIFO" "$AGH_AIR_DEV_RUN_ID" "$AGH_AIR_READY_MARKER" "fake-binary"
trap 'exit 143' INT TERM
read -r _ < "$AGH_TEST_HOLD_FIFO"
`)

		airStarted := readFIFOEvent(airStartedFIFO)
		viteStarted := readFIFOEvent(viteStartedFIFO)
		commandContext, cancelCommand := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancelCommand()
		command := exec.CommandContext(commandContext, "bash", filepath.Join(repoRoot, "scripts", "dev.sh"))
		command.Dir = repoRoot
		command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		command.Env = append(os.Environ(),
			"PATH="+fakeBinDir+string(os.PathListSeparator)+os.Getenv("PATH"),
			"AIR_VERSION=test-version",
			"AGH_WEB_API_PROXY_TARGET=http://127.0.0.1:2123",
			"AGH_AIR_CACHE_DIR="+airCacheDir,
			"AGH_TEST_AIR_STATE_DIR="+stateDir,
			"AGH_TEST_AIR_STARTED_FIFO="+airStartedFIFO,
			"AGH_TEST_VITE_STARTED_FIFO="+viteStartedFIFO,
			"AGH_TEST_READY_GATE_FIFO="+readyGateFIFO,
			"AGH_TEST_HOLD_FIFO="+holdFIFO,
			"AGH_TEST_REPO_ROOT="+repoRoot,
		)
		var commandOutput bytes.Buffer
		command.Stdout = &commandOutput
		command.Stderr = &commandOutput
		if err := command.Start(); err != nil {
			t.Fatalf("start dev supervisor: %v", err)
		}
		commandDone := make(chan error, 1)
		go func() {
			commandDone <- command.Wait()
		}()
		commandWaited := false
		t.Cleanup(func() {
			if commandWaited {
				return
			}
			if err := signalProcessGroup(command.Process.Pid, syscall.SIGTERM); err != nil {
				t.Errorf("signal dev supervisor during cleanup: %v", err)
			}
			err := awaitProcessExit(commandDone, 2*time.Second)
			if errors.Is(err, context.DeadlineExceeded) {
				if killErr := signalProcessGroup(command.Process.Pid, syscall.SIGKILL); killErr != nil {
					t.Errorf("kill dev supervisor process group during cleanup: %v", killErr)
				}
				err = awaitProcessExit(commandDone, 2*time.Second)
			}
			if err != nil {
				var exitError *exec.ExitError
				if !errors.As(err, &exitError) || exitError.ExitCode() != 143 {
					t.Errorf("wait for dev supervisor during cleanup: %v; output=%s", err, commandOutput.String())
				}
			}
		})

		if event := awaitFIFOEvent(t, airStarted, 2*time.Second, "Air start"); event != "air-started" {
			t.Fatalf("Air event = %q, want air-started", event)
		}
		absenceTimer := time.NewTimer(150 * time.Millisecond)
		select {
		case result := <-viteStarted:
			absenceTimer.Stop()
			if result.err != nil {
				t.Fatalf("read early Vite event: %v", result.err)
			}
			t.Fatalf("Vite started before daemon readiness: %q", result.value)
		case <-absenceTimer.C:
		}

		if err := writeFIFOEvent(readyGateFIFO, "release"); err != nil {
			t.Fatalf("release daemon readiness gate: %v", err)
		}
		if event := awaitFIFOEvent(t, viteStarted, 2*time.Second, "Vite start"); event != "vite-started" {
			t.Fatalf("Vite event = %q, want vite-started", event)
		}

		if err := signalProcessGroup(command.Process.Pid, syscall.SIGTERM); err != nil {
			t.Fatalf("signal dev supervisor: %v", err)
		}
		err = awaitProcessExit(commandDone, 2*time.Second)
		commandWaited = true
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) || exitError.ExitCode() != 143 {
			t.Fatalf("dev supervisor exit = %v, want status 143; output=%s", err, commandOutput.String())
		}
	})
}

type fifoResult struct {
	value string
	err   error
}

func repositoryRoot(t *testing.T) string {
	t.Helper()

	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	root, err := filepath.Abs(filepath.Join(workingDirectory, "..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return root
}

func createFIFO(t *testing.T, directory, name string) string {
	t.Helper()

	path := filepath.Join(directory, name)
	if err := unix.Mkfifo(path, 0o600); err != nil {
		t.Fatalf("create FIFO %q: %v", path, err)
	}
	return path
}

func writeExecutable(t *testing.T, directory, name, content string) string {
	t.Helper()

	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatalf("write executable %q: %v", path, err)
	}
	return path
}

func runDevDaemon(
	t *testing.T,
	repoRoot, binaryPath, stateDir, fifoPath, markerPath, startLog, runID string,
) {
	t.Helper()

	command := devDaemonCommand(
		t.Context(),
		repoRoot,
		binaryPath,
		stateDir,
		fifoPath,
		markerPath,
		startLog,
		runID,
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("run dev-daemon: %v; output=%s", err, output)
	}
}

func devDaemonCommand(
	ctx context.Context,
	repoRoot, binaryPath, stateDir, fifoPath, markerPath, startLog, runID string,
) *exec.Cmd {
	command := exec.CommandContext(ctx, "bash", filepath.Join(repoRoot, "scripts", "dev-daemon.sh"), binaryPath)
	command.Dir = repoRoot
	command.Env = append(os.Environ(),
		"AGH_AIR_STATE_LOCK_HELD=1",
		"AGH_AIR_STATE_DIR="+stateDir,
		"AGH_AIR_DEV_RUN_ID="+runID,
		"AGH_AIR_READY_FIFO="+fifoPath,
		"AGH_AIR_READY_MARKER="+markerPath,
		"FAKE_DAEMON_START_LOG="+startLog,
	)
	return command
}

func checksumIdentity(t *testing.T, path string) string {
	t.Helper()

	command := exec.CommandContext(t.Context(), "cksum", path)
	output, err := command.Output()
	if err != nil {
		t.Fatalf("identify test daemon binary: %v", err)
	}
	fields := strings.Fields(string(output))
	if len(fields) < 2 {
		t.Fatalf("cksum output = %q, want checksum and byte count", output)
	}
	if _, err := strconv.ParseUint(fields[0], 10, 64); err != nil {
		t.Fatalf("parse checksum %q: %v", fields[0], err)
	}
	if _, err := strconv.ParseUint(fields[1], 10, 64); err != nil {
		t.Fatalf("parse byte count %q: %v", fields[1], err)
	}
	return fmt.Sprintf("%s:%s", fields[0], fields[1])
}

func readFIFOEvent(path string) <-chan fifoResult {
	result := make(chan fifoResult, 1)
	go func() {
		file, err := os.Open(path)
		if err != nil {
			result <- fifoResult{err: err}
			return
		}
		buffer := make([]byte, 128)
		count, readErr := file.Read(buffer)
		closeErr := file.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			result <- fifoResult{err: err}
			return
		}
		result <- fifoResult{value: strings.TrimSpace(string(buffer[:count]))}
	}()
	return result
}

func awaitFIFOEvent(t *testing.T, result <-chan fifoResult, timeout time.Duration, name string) string {
	t.Helper()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case event := <-result:
		if event.err != nil {
			t.Fatalf("read %s event: %v", name, event.err)
		}
		return event.value
	case <-timer.C:
		t.Fatalf("timed out waiting for %s event", name)
		return ""
	}
}

func writeFIFOEvent(path, value string) error {
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("open FIFO: %w", err)
	}
	_, writeErr := fmt.Fprintln(file, value)
	closeErr := file.Close()
	return errors.Join(writeErr, closeErr)
}

func signalProcessGroup(pid int, signal syscall.Signal) error {
	err := unix.Kill(-pid, signal)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

func awaitProcessExit(done <-chan error, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-done:
		return err
	case <-timer.C:
		return context.DeadlineExceeded
	}
}
