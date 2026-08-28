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
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
)

const windowsJobExitCode = 1

const windowsProcessCleanupWaitMS = uint32(2_000)

type windowsProc struct {
	device    *conpty.ConPty
	job       *windowsJob
	waiter    processWaiter
	pid       int
	startedAt time.Time

	processMu  sync.Mutex
	inputMu    sync.Mutex
	readMu     sync.Mutex
	io         ioLifecycle
	process    windows.Handle
	killSignal string
	inputIO    *windowsSyncIO
	outputIO   *windowsSyncIO
	closeOnce  sync.Once
	closeErr   error
}

func startInteractive(ctx context.Context, spec ProcSpec) (Proc, error) {
	if len(spec.Argv) == 0 || spec.Argv[0] == "" {
		return nil, errors.New("terminal pty: command is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("terminal pty: start context: %w", err)
	}
	environmentList := environment(spec.Env)
	path, err := interp.LookPathDir(spec.Cwd, expand.ListEnviron(environmentList...), spec.Argv[0])
	if err != nil {
		return nil, fmt.Errorf("terminal pty: resolve %q: %w", spec.Argv[0], err)
	}
	argv := append([]string(nil), spec.Argv...)
	argv[0] = path
	cols, rows := normalizedSize(spec.Cols, spec.Rows)
	job, err := newWindowsJob()
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, errors.Join(fmt.Errorf("terminal pty: start context: %w", err), job.close())
	}
	device, err := conpty.New(int(cols), int(rows), 0)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("terminal pty: open ConPTY: %w", err), job.close())
	}
	if err := ctx.Err(); err != nil {
		return nil, errors.Join(fmt.Errorf("terminal pty: start context: %w", err), device.Close(), job.close())
	}
	pid, rawHandle, err := device.Spawn(path, argv, &syscall.ProcAttr{
		Dir: spec.Cwd,
		Env: environmentList,
		Sys: &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_SUSPENDED},
	})
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("terminal pty: start %q: %w", spec.Argv[0], err),
			device.Close(), job.close(),
		)
	}
	process := windows.Handle(rawHandle)
	if err := ctx.Err(); err != nil {
		cleanupErr := terminateUnassignedWindowsProcess(process)
		return nil, errors.Join(
			fmt.Errorf("terminal pty: start context: %w", err), cleanupErr, device.Close(), job.close(),
		)
	}
	if err := job.assign(process); err != nil {
		cleanupErr := terminateUnassignedWindowsProcess(process)
		return nil, errors.Join(
			fmt.Errorf("terminal pty: assign process %d to job: %w", pid, err),
			cleanupErr, device.Close(), job.close(),
		)
	}
	if err := ctx.Err(); err != nil {
		cleanupErr := errors.Join(job.terminate(windowsJobExitCode), waitAndCloseWindowsProcess(process, pid))
		return nil, errors.Join(
			fmt.Errorf("terminal pty: start context: %w", err), cleanupErr, device.Close(), job.close(),
		)
	}
	if err := resumeWindowsProcess(process); err != nil {
		cleanupErr := errors.Join(job.terminate(windowsJobExitCode), waitAndCloseWindowsProcess(process, pid))
		return nil, errors.Join(
			fmt.Errorf("terminal pty: resume process %d: %w", pid, err),
			cleanupErr, device.Close(), job.close(),
		)
	}
	if err := ctx.Err(); err != nil {
		cleanupErr := errors.Join(job.terminate(windowsJobExitCode), waitAndCloseWindowsProcess(process, pid))
		return nil, errors.Join(
			fmt.Errorf("terminal pty: start context: %w", err), cleanupErr, device.Close(), job.close(),
		)
	}
	startedAt, err := procutil.StartedAt(pid)
	if err != nil {
		cleanupErr := errors.Join(job.terminate(windowsJobExitCode), waitAndCloseWindowsProcess(process, pid))
		return nil, errors.Join(
			fmt.Errorf("terminal pty: read process %d start time: %w", pid, err),
			cleanupErr, device.Close(), job.close(),
		)
	}
	inputIO, err := newWindowsSyncIO("input write", device.Write)
	if err != nil {
		cleanupErr := errors.Join(job.terminate(windowsJobExitCode), waitAndCloseWindowsProcess(process, pid))
		return nil, errors.Join(
			fmt.Errorf("terminal pty: start input worker: %w", err),
			cleanupErr, device.Close(), job.close(),
		)
	}
	outputIO, err := newWindowsSyncIO("output read", device.Read)
	if err != nil {
		cleanupErr := errors.Join(
			inputIO.stop(),
			job.terminate(windowsJobExitCode),
			waitAndCloseWindowsProcess(process, pid),
		)
		return nil, errors.Join(
			fmt.Errorf("terminal pty: start output worker: %w", err),
			cleanupErr, device.Close(), job.close(),
		)
	}
	proc := &windowsProc{
		device: device, job: job, waiter: newProcessWaiter(), pid: pid,
		process: process, startedAt: startedAt, inputIO: inputIO, outputIO: outputIO,
	}
	proc.waiter.start(proc.waitForExit)
	return proc, nil
}

func (p *windowsProc) PID() int { return p.pid }

func (p *windowsProc) ProcessGroupID() int { return p.pid }

func (p *windowsProc) StartedAt() time.Time { return p.startedAt }

func (p *windowsProc) Reader() io.Reader { return windowsPTYReader{process: p} }

type windowsPTYReader struct {
	process *windowsProc
}

func (r windowsPTYReader) Read(buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return 0, nil
	}
	r.process.readMu.Lock()
	defer r.process.readMu.Unlock()
	if err := r.process.io.begin(); err != nil {
		return 0, err
	}
	defer r.process.io.end()
	for {
		read, err := r.process.outputIO.do(buffer)
		if read != 0 || err != nil {
			return read, err
		}
	}
}

func (p *windowsProc) Write(input []byte) (int, error) {
	p.inputMu.Lock()
	defer p.inputMu.Unlock()
	if err := p.io.begin(); err != nil {
		return 0, err
	}
	defer p.io.end()
	return p.write(input)
}

func (p *windowsProc) write(input []byte) (int, error) {
	written, err := p.inputIO.do(input)
	if err != nil {
		return written, fmt.Errorf("terminal pty: write: %w", err)
	}
	return written, nil
}

func (p *windowsProc) Resize(cols, rows uint16) error {
	if err := p.io.begin(); err != nil {
		return err
	}
	defer p.io.end()
	cols, rows = normalizedSize(cols, rows)
	if err := p.device.Resize(int(cols), int(rows)); err != nil {
		return fmt.Errorf("terminal pty: resize: %w", err)
	}
	return nil
}

func (p *windowsProc) Wait(ctx context.Context) (Exit, error) { return p.waiter.wait(ctx) }

func (p *windowsProc) Kill(signal Signal) error {
	return p.kill(signal)
}

func (p *windowsProc) kill(signal Signal) error {
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
		p.io.seal()
		killErr := p.kill(SignalHUP)
		cancelErr := errors.Join(p.outputIO.cancel(), p.inputIO.cancel())
		p.io.wait()
		workerErr := errors.Join(p.outputIO.stop(), p.inputIO.stop())
		p.closeErr = errors.Join(killErr, cancelErr, workerErr, p.job.close(), p.device.Close())
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

func terminateUnassignedWindowsProcess(process windows.Handle) error {
	terminateErr := procutil.TerminateProcessHandle(uintptr(process), windowsJobExitCode)
	return errors.Join(terminateErr, waitAndCloseWindowsProcess(process, 0))
}

func waitAndCloseWindowsProcess(process windows.Handle, pid int) error {
	event, waitErr := windows.WaitForSingleObject(process, windowsProcessCleanupWaitMS)
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
