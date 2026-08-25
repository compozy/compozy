//go:build windows

package pty

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/x/conpty"
	"github.com/compozy/compozy/internal/procutil"
	"golang.org/x/sys/windows"
)

const windowsJobExitCode = 1

type windowsProc struct {
	device    *conpty.ConPty
	job       *windowsJob
	waiter    processWaiter
	pid       int
	startedAt time.Time

	processMu  sync.Mutex
	process    windows.Handle
	killSignal string
	closeOnce  sync.Once
	closeErr   error
}

func startInteractive(_ context.Context, spec ProcSpec) (Proc, error) {
	if len(spec.Argv) == 0 || spec.Argv[0] == "" {
		return nil, errors.New("terminal pty: command is required")
	}
	cols, rows := normalizedSize(spec.Cols, spec.Rows)
	job, err := newWindowsJob()
	if err != nil {
		return nil, err
	}
	device, err := conpty.New(int(cols), int(rows), 0)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("terminal pty: open ConPTY: %w", err), job.close())
	}
	pid, rawHandle, err := device.Spawn(spec.Argv[0], append([]string(nil), spec.Argv...), &syscall.ProcAttr{
		Dir: spec.Cwd,
		Env: environment(spec.Env),
		Sys: &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_SUSPENDED},
	})
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("terminal pty: start %q: %w", spec.Argv[0], err),
			device.Close(), job.close(),
		)
	}
	process := windows.Handle(rawHandle)
	if err := job.assign(process); err != nil {
		cleanupErr := terminateUnassignedWindowsProcess(process)
		return nil, errors.Join(
			fmt.Errorf("terminal pty: assign process %d to job: %w", pid, err),
			cleanupErr, device.Close(), job.close(),
		)
	}
	if err := resumeWindowsProcess(process); err != nil {
		cleanupErr := errors.Join(job.terminate(windowsJobExitCode), waitAndCloseWindowsProcess(process, pid))
		return nil, errors.Join(
			fmt.Errorf("terminal pty: resume process %d: %w", pid, err),
			cleanupErr, device.Close(), job.close(),
		)
	}
	proc := &windowsProc{
		device: device, job: job, waiter: newProcessWaiter(), pid: pid,
		process: process, startedAt: time.Now().UTC(),
	}
	proc.waiter.start(proc.waitForExit)
	return proc, nil
}

func (p *windowsProc) PID() int { return p.pid }

func (p *windowsProc) ProcessGroupID() int { return p.pid }

func (p *windowsProc) StartedAt() time.Time { return p.startedAt }

func (p *windowsProc) Reader() io.Reader { return p.device }

func (p *windowsProc) Write(input []byte) (int, error) {
	written, err := p.device.Write(input)
	if err != nil {
		return written, fmt.Errorf("terminal pty: write: %w", err)
	}
	return written, nil
}

func (p *windowsProc) Resize(cols, rows uint16) error {
	cols, rows = normalizedSize(cols, rows)
	if err := p.device.Resize(int(cols), int(rows)); err != nil {
		return fmt.Errorf("terminal pty: resize: %w", err)
	}
	return nil
}

func (p *windowsProc) Wait(ctx context.Context) (Exit, error) { return p.waiter.wait(ctx) }

func (p *windowsProc) Kill(signal Signal) error {
	if err := validateWindowsSignal(signal); err != nil {
		return err
	}
	p.processMu.Lock()
	defer p.processMu.Unlock()
	if p.process == 0 {
		return nil
	}
	event, err := windows.WaitForSingleObject(p.process, 0)
	if err != nil {
		return fmt.Errorf("terminal pty: probe process %d: %w", p.pid, err)
	}
	if event == windows.WAIT_OBJECT_0 {
		return nil
	}
	if event != uint32(syscall.WAIT_TIMEOUT) {
		return fmt.Errorf("terminal pty: probe process %d: unexpected wait status %d", p.pid, event)
	}
	if err := p.job.terminate(windowsJobExitCode); err != nil {
		return fmt.Errorf("terminal pty: terminate process tree %d: %w", p.pid, err)
	}
	p.killSignal = string(signal)
	return nil
}

func (p *windowsProc) Close() error {
	p.closeOnce.Do(func() {
		killErr := p.Kill(SignalHUP)
		cancelErr := cancelWindowsRead(p.device)
		p.closeErr = errors.Join(killErr, cancelErr, p.job.close(), p.device.Close())
	})
	return p.closeErr
}

func (p *windowsProc) waitForExit() waitResult {
	p.processMu.Lock()
	process := p.process
	p.processMu.Unlock()
	if process == 0 {
		return waitResult{exit: Exit{Cause: "unknown"}, err: errors.New("terminal pty: process handle is closed")}
	}
	event, waitErr := windows.WaitForSingleObject(process, windows.INFINITE)
	var exitCode uint32
	if waitErr == nil && event != windows.WAIT_OBJECT_0 {
		waitErr = fmt.Errorf("terminal pty: wait process %d: unexpected wait status %d", p.pid, event)
	}
	if waitErr == nil {
		waitErr = windows.GetExitCodeProcess(process, &exitCode)
		if waitErr != nil {
			waitErr = fmt.Errorf("terminal pty: read process %d exit code: %w", p.pid, waitErr)
		}
	}
	p.processMu.Lock()
	signal := p.killSignal
	p.process = 0
	p.processMu.Unlock()
	closeErr := closeWindowsProcessHandle(process, p.pid)
	if waitErr != nil {
		return waitResult{exit: Exit{Cause: "unknown"}, err: errors.Join(waitErr, closeErr)}
	}
	if signal != "" {
		return waitResult{exit: Exit{Cause: "signaled", Signal: &signal}, err: closeErr}
	}
	code := int(exitCode)
	return waitResult{exit: Exit{Cause: "exited", Code: &code}, err: closeErr}
}

func validateWindowsSignal(signal Signal) error {
	switch signal {
	case SignalINT, SignalTERM, SignalKILL, SignalHUP:
		return nil
	default:
		return fmt.Errorf("terminal pty: unsupported signal %q", signal)
	}
}

func cancelWindowsRead(device *conpty.ConPty) error {
	if device == nil || device.OutPipeReadFd() == 0 {
		return nil
	}
	err := windows.CancelIoEx(windows.Handle(device.OutPipeReadFd()), nil)
	if errors.Is(err, windows.ERROR_NOT_FOUND) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("terminal pty: cancel output read: %w", err)
	}
	return nil
}

func terminateUnassignedWindowsProcess(process windows.Handle) error {
	terminateErr := procutil.TerminateProcessHandle(uintptr(process), windowsJobExitCode)
	return errors.Join(terminateErr, waitAndCloseWindowsProcess(process, 0))
}

func waitAndCloseWindowsProcess(process windows.Handle, pid int) error {
	event, waitErr := windows.WaitForSingleObject(process, windows.INFINITE)
	if waitErr == nil && event != windows.WAIT_OBJECT_0 {
		waitErr = fmt.Errorf("unexpected wait status %d", event)
	}
	return errors.Join(waitErr, closeWindowsProcessHandle(process, pid))
}

func closeWindowsProcessHandle(handle windows.Handle, pid int) error {
	if handle == 0 {
		return nil
	}
	if err := windows.CloseHandle(handle); err != nil {
		return fmt.Errorf("terminal pty: close process %d handle: %w", pid, err)
	}
	return nil
}

func processSignal(*os.ProcessState) string { return "" }
