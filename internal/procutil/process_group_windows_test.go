//go:build windows

package procutil

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsProcessJobIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should not resolve a reused PID to a different command owner", func(t *testing.T) {
		t.Parallel()

		const pid = 987654
		first := &exec.Cmd{Process: &os.Process{Pid: pid}}
		second := &exec.Cmd{Process: &os.Process{Pid: pid}}
		job := &windowsProcessJob{command: first, pid: pid}
		windowsProcessJobs.Store(pid, job)
		t.Cleanup(func() { windowsProcessJobs.CompareAndDelete(pid, job) })

		if got, ok := windowsJobForCommand(first); !ok || got != job {
			t.Fatalf("windowsJobForCommand(owner) = (%p, %t), want (%p, true)", got, ok, job)
		}
		if got, ok := windowsJobForCommand(second); ok || got != nil {
			t.Fatalf("windowsJobForCommand(reused PID) = (%p, %t), want (nil, false)", got, ok)
		}
	})
}

func TestWindowsStrictProcessGroupSignals(t *testing.T) {
	t.Parallel()

	t.Run("Should report a closed registered job as missing", func(t *testing.T) {
		t.Parallel()

		const pid = 1 << 29
		job := &windowsProcessJob{pid: pid, closed: true}
		windowsProcessJobs.Store(pid, job)
		t.Cleanup(func() {
			windowsProcessJobs.CompareAndDelete(pid, job)
		})

		err := SignalProcessGroupIDStrict(pid, syscall.SIGTERM)
		if !errors.Is(err, os.ErrProcessDone) {
			t.Fatalf("SignalProcessGroupIDStrict() error = %v, want os.ErrProcessDone", err)
		}
	})

	t.Run("Should classify Windows invalid-parameter errors as missing", func(t *testing.T) {
		t.Parallel()

		err := fmt.Errorf("open process: %w", windows.ERROR_INVALID_PARAMETER)
		if !IsProcessMissingError(err) {
			t.Fatalf("IsProcessMissingError(%v) = false, want true", err)
		}
	})

	t.Run("Should classify a completed Windows process as missing", func(t *testing.T) {
		t.Parallel()

		err := fmt.Errorf("closed job: %w", os.ErrProcessDone)
		if !IsProcessMissingError(err) {
			t.Fatalf("IsProcessMissingError(%v) = false, want true", err)
		}
	})

	t.Run("Should preserve Windows access-denied errors", func(t *testing.T) {
		t.Parallel()

		err := fmt.Errorf("open process: %w", windows.ERROR_ACCESS_DENIED)
		if IsProcessMissingError(err) {
			t.Fatalf("IsProcessMissingError(%v) = true, want false", err)
		}
	})
}
