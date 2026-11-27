//go:build windows

package procutil

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
)

// Invariant: process inspection returns the target account, not the caller's account.
// Owner: procutil Windows process identity; canonical suite: native Windows process tests.
func TestWindowsProcessIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should inspect the owner of a running system service", func(t *testing.T) {
		t.Parallel()
		manager, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
		requireWindowsProcessQueryAccess(t, err)
		t.Cleanup(func() {
			if err := windows.CloseServiceHandle(manager); err != nil {
				t.Error(err)
			}
		})
		name, err := windows.UTF16PtrFromString("Schedule")
		if err != nil {
			t.Fatal(err)
		}
		handle, err := windows.OpenService(manager, name, windows.SERVICE_QUERY_STATUS|windows.SERVICE_QUERY_CONFIG)
		requireWindowsProcessQueryAccess(t, err)
		service := mgr.Service{Handle: handle}
		t.Cleanup(func() {
			if err := service.Close(); err != nil {
				t.Error(err)
			}
		})
		config, err := service.Config()
		requireWindowsProcessQueryAccess(t, err)
		status, err := service.Query()
		requireWindowsProcessQueryAccess(t, err)
		if !strings.EqualFold(config.ServiceStartName, "LocalSystem") || status.ProcessId == 0 {
			t.Skip("Task Scheduler is not running as LocalSystem on this host")
		}
		caller, err := windows.GetCurrentProcessToken().GetTokenUser()
		if err != nil {
			t.Fatal(err)
		}
		if caller.User.Sid.String() == "S-1-5-18" {
			t.Skip("the test itself is running as LocalSystem")
		}
		owner, executable, err := InspectWindowsProcess(status.ProcessId)
		requireWindowsProcessQueryAccess(t, err)
		if owner != "S-1-5-18" || owner == caller.User.Sid.String() || executable == "" {
			t.Fatalf("InspectWindowsProcess() = (%q, %q), want the LocalSystem service identity", owner, executable)
		}
	})
}

func requireWindowsProcessQueryAccess(t *testing.T, err error) {
	t.Helper()
	if errors.Is(err, windows.ERROR_ACCESS_DENIED) && os.Getenv("GITHUB_ACTIONS") != "true" {
		t.Skip("this account cannot query the system service identity")
	}
	if err != nil {
		t.Fatal(err)
	}
}

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

func TestWindowsProcessHandleTermination(t *testing.T) {
	t.Parallel()

	t.Run("Should terminate the process owned by the supplied handle", func(t *testing.T) {
		t.Parallel()

		command := exec.Command(
			"powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", "Start-Sleep -Seconds 300",
		)
		if err := command.Start(); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		waited := false
		t.Cleanup(func() {
			if waited {
				return
			}
			killErr := command.Process.Kill()
			waitErr := command.Wait()
			if killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
				t.Errorf("Kill() cleanup error = %v", killErr)
			}
			if _, ok := errors.AsType[*exec.ExitError](waitErr); waitErr != nil && !ok {
				t.Errorf("Wait() cleanup error = %v", waitErr)
			}
		})

		var terminateErr error
		handleErr := command.Process.WithHandle(func(handle uintptr) {
			terminateErr = TerminateProcessHandle(handle, 23)
		})
		if handleErr != nil || terminateErr != nil {
			t.Fatalf("TerminateProcessHandle() handle_error=%v terminate_error=%v", handleErr, terminateErr)
		}
		waitErr := command.Wait()
		waited = true
		if _, ok := errors.AsType[*exec.ExitError](waitErr); !ok {
			t.Fatalf("Wait() error = %v, want exec.ExitError", waitErr)
		}
		if code := command.ProcessState.ExitCode(); code != 23 {
			t.Fatalf("ExitCode() = %d, want 23", code)
		}
	})
}
