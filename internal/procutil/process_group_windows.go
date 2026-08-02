//go:build windows

package procutil

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type windowsProcessJob struct {
	mu      sync.Mutex
	command *exec.Cmd
	pid     int
	handle  windows.Handle
	closed  bool
}

var windowsProcessJobs sync.Map

// ConfigureCommandProcessGroup prepares the command for Windows process-tree management.
func ConfigureCommandProcessGroup(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_NEW_PROCESS_GROUP
}

// RegisterCommandProcessGroup attaches the started command to a kill-on-close job object.
func RegisterCommandProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil || cmd.Process.Pid <= 0 {
		return nil
	}

	pid := cmd.Process.Pid
	job, err := createKillOnCloseJob(pid)
	if err != nil {
		return err
	}
	process, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.SYNCHRONIZE,
		false,
		uint32(pid),
	)
	if err != nil {
		return errors.Join(
			fmt.Errorf("procutil: open process %d for job assignment: %w", pid, err),
			closeWindowsHandle(job, fmt.Sprintf("job for process %d", pid)),
		)
	}

	if err := windows.AssignProcessToJobObject(job, process); err != nil {
		return errors.Join(
			fmt.Errorf("procutil: assign process %d to job: %w", pid, err),
			closeWindowsHandle(process, fmt.Sprintf("process %d handle", pid)),
			closeWindowsHandle(job, fmt.Sprintf("job for process %d", pid)),
		)
	}
	if err := closeWindowsHandle(process, fmt.Sprintf("process %d handle", pid)); err != nil {
		return errors.Join(err, closeWindowsHandle(job, fmt.Sprintf("job for process %d", pid)))
	}
	registration := &windowsProcessJob{command: cmd, pid: pid, handle: job}
	if _, loaded := windowsProcessJobs.LoadOrStore(pid, registration); loaded {
		return errors.Join(
			fmt.Errorf("procutil: process group already registered for pid %d", pid),
			registration.close(),
		)
	}
	return nil
}

// SignalCommandProcessGroup terminates the command's Windows job object.
func SignalCommandProcessGroup(cmd *exec.Cmd, sig syscall.Signal) error {
	if cmd == nil || cmd.Process == nil || cmd.Process.Pid <= 0 {
		return nil
	}
	if sig == 0 {
		return signalZeroWindowsCommandProcess(cmd)
	}
	if job, ok := windowsJobForCommand(cmd); ok {
		return job.signal(sig)
	}
	return killWindowsCommandProcess(cmd)
}

// SignalProcessGroupID terminates the Windows job object identified by pgid.
func SignalProcessGroupID(pgid int, sig syscall.Signal) error {
	if pgid <= 0 {
		return fmt.Errorf("procutil: invalid process group id %d", pgid)
	}
	if sig == 0 {
		return Signal(pgid, sig)
	}
	if job, ok := windowsJobForPID(pgid); ok {
		return job.signal(sig)
	}
	return Signal(pgid, sig)
}

// WaitForCommandProcessGroupExit blocks until the command's Windows job exits.
func WaitForCommandProcessGroupExit(cmd *exec.Cmd, timeout time.Duration) error {
	if cmd == nil || cmd.Process == nil || cmd.Process.Pid <= 0 {
		return nil
	}
	if job, ok := windowsJobForCommand(cmd); ok {
		return job.wait(timeout)
	}
	return nil
}

// WaitForProcessGroupIDExit blocks until the Windows job object exits.
func WaitForProcessGroupIDExit(pgid int, timeout time.Duration) error {
	if pgid <= 0 {
		return fmt.Errorf("procutil: invalid process group id %d", pgid)
	}
	if job, ok := windowsJobForPID(pgid); ok {
		return job.wait(timeout)
	}
	return waitForWindowsRootProcessExit(pgid, timeout)
}

// KillCommandProcessGroupAndWait forcefully terminates the command's Windows job.
func KillCommandProcessGroupAndWait(cmd *exec.Cmd, timeout time.Duration) error {
	if cmd == nil || cmd.Process == nil || cmd.Process.Pid <= 0 {
		return nil
	}
	if job, ok := windowsJobForCommand(cmd); ok {
		return job.killAndWait(timeout)
	}
	return killWindowsCommandProcess(cmd)
}

// KillProcessGroupIDAndWait forcefully terminates the Windows job object and waits for exit.
func KillProcessGroupIDAndWait(pgid int, timeout time.Duration) error {
	if pgid <= 0 {
		return fmt.Errorf("procutil: invalid process group id %d", pgid)
	}
	var signalErr error
	if job, ok := windowsJobForPID(pgid); ok {
		return job.killAndWait(timeout)
	}
	if err := Signal(pgid, syscall.SIGKILL); err != nil {
		signalErr = err
	}
	waitErr := waitForWindowsRootProcessExit(pgid, timeout)
	return errors.Join(signalErr, waitErr)
}

func createKillOnCloseJob(pid int) (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, fmt.Errorf("procutil: create windows job for process %d: %w", pid, err)
	}
	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		return 0, errors.Join(
			fmt.Errorf("procutil: set kill-on-close for process %d job: %w", pid, err),
			closeWindowsHandle(job, fmt.Sprintf("job for process %d", pid)),
		)
	}
	return job, nil
}

func windowsJobForPID(pid int) (*windowsProcessJob, bool) {
	value, ok := windowsProcessJobs.Load(pid)
	if !ok {
		return nil, false
	}
	job, ok := value.(*windowsProcessJob)
	return job, ok && job != nil
}

func windowsJobForCommand(cmd *exec.Cmd) (*windowsProcessJob, bool) {
	if cmd == nil || cmd.Process == nil || cmd.Process.Pid <= 0 {
		return nil, false
	}
	job, ok := windowsJobForPID(cmd.Process.Pid)
	if !ok || job.command != cmd {
		return nil, false
	}
	return job, true
}

func killWindowsCommandProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}

func signalZeroWindowsCommandProcess(cmd *exec.Cmd) error {
	var probeErr error
	handleErr := cmd.Process.WithHandle(func(handle uintptr) {
		event, err := windows.WaitForSingleObject(windows.Handle(handle), 0)
		if err != nil {
			probeErr = fmt.Errorf("procutil: probe process %d: %w", cmd.Process.Pid, err)
			return
		}
		if event == uint32(syscall.WAIT_TIMEOUT) {
			return
		}
		probeErr = fmt.Errorf("procutil: probe process %d: %w", cmd.Process.Pid, os.ErrProcessDone)
	})
	return errors.Join(handleErr, probeErr)
}

func closeWindowsHandle(handle windows.Handle, name string) error {
	if handle == 0 {
		return nil
	}
	if err := windows.CloseHandle(handle); err != nil {
		return fmt.Errorf("procutil: close %s: %w", name, err)
	}
	return nil
}

func (j *windowsProcessJob) signal(sig syscall.Signal) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed || j.handle == 0 {
		return nil
	}
	if sig == 0 {
		return nil
	}
	if err := windows.TerminateJobObject(j.handle, 1); err != nil {
		return fmt.Errorf("terminate windows job (pid %d, sig %v): %w", j.pid, sig, err)
	}
	return nil
}

func (j *windowsProcessJob) wait(timeout time.Duration) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed || j.handle == 0 {
		return nil
	}
	return j.waitLocked(windowsWaitTimeout(timeout))
}

func (j *windowsProcessJob) killAndWait(timeout time.Duration) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed || j.handle == 0 {
		return nil
	}
	var signalErr error
	if err := windows.TerminateJobObject(j.handle, 1); err != nil {
		signalErr = fmt.Errorf("terminate windows job (pid %d): %w", j.pid, err)
	}
	return errors.Join(signalErr, j.waitLocked(windowsWaitTimeout(timeout)))
}

func (j *windowsProcessJob) waitLocked(timeout uint32) error {
	event, err := windows.WaitForSingleObject(j.handle, timeout)
	if err != nil {
		return fmt.Errorf("wait for windows job exit (pid %d): %w", j.pid, err)
	}
	switch event {
	case windows.WAIT_OBJECT_0:
		return j.closeLocked()
	case uint32(syscall.WAIT_TIMEOUT):
		return fmt.Errorf(
			"wait for process group exit (pid %d): deadline exceeded after %s",
			j.pid,
			windowsTimeoutText(timeout),
		)
	default:
		return fmt.Errorf("wait for windows job exit (pid %d): unexpected wait status %d", j.pid, event)
	}
}

func (j *windowsProcessJob) close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.closeLocked()
}

func (j *windowsProcessJob) closeLocked() error {
	if j.closed || j.handle == 0 {
		return nil
	}
	if err := closeWindowsHandle(j.handle, fmt.Sprintf("job for process %d", j.pid)); err != nil {
		return err
	}
	j.handle = 0
	j.closed = true
	windowsProcessJobs.CompareAndDelete(j.pid, j)
	return nil
}

func waitForWindowsRootProcessExit(pid int, timeout time.Duration) error {
	if timeout <= 0 {
		if Alive(pid) {
			return fmt.Errorf("wait for process group exit (pid %d): deadline exceeded after %s", pid, timeout)
		}
		return nil
	}
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for Alive(pid) {
		select {
		case <-ticker.C:
		case <-deadline.C:
			return fmt.Errorf("wait for process group exit (pid %d): deadline exceeded after %s", pid, timeout)
		}
	}
	return nil
}

func windowsWaitTimeout(timeout time.Duration) uint32 {
	if timeout <= 0 {
		return 0
	}
	millis := timeout.Milliseconds()
	if millis <= 0 {
		return 1
	}
	if millis >= int64(windows.INFINITE) {
		return windows.INFINITE - 1
	}
	return uint32(millis)
}

func windowsTimeoutText(timeout uint32) time.Duration {
	return time.Duration(timeout) * time.Millisecond
}
