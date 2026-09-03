//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package pty

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/x/xpty"
	"github.com/compozy/compozy/internal/procutil"
	"golang.org/x/sys/unix"
)

type unixProc struct {
	device      *xpty.UnixPty
	reader      *unixPTYReader
	writer      *os.File
	command     *exec.Cmd
	waiter      processWaiter
	startedAt   time.Time
	mu          sync.RWMutex
	inputMu     sync.Mutex
	signalMu    sync.Mutex
	io          ioLifecycle
	killSignal  Signal
	closeOnce   sync.Once
	closeErr    error
	shimCleanup func() error
}

func startInteractive(ctx context.Context, spec ProcSpec) (Proc, error) {
	if len(spec.Argv) == 0 || spec.Argv[0] == "" {
		return nil, errors.New("terminal pty: command is required")
	}
	cols, rows := normalizedSize(spec.Cols, spec.Rows)
	device, err := xpty.NewUnixPty(int(cols), int(rows))
	if err != nil {
		return nil, fmt.Errorf("terminal pty: open: %w", err)
	}
	if err := makeUnixMasterNonblocking(device); err != nil {
		return nil, errors.Join(
			fmt.Errorf("terminal pty: make master pollable: %w", err), closeUnixDevice(device),
		)
	}
	readerFile, err := duplicateMaster(device, "reader")
	if err != nil {
		return nil, errors.Join(fmt.Errorf("terminal pty: duplicate reader: %w", err), closeUnixDevice(device))
	}
	reader := &unixPTYReader{File: readerFile}
	writer, err := duplicateMaster(device, "writer")
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("terminal pty: duplicate writer: %w", err),
			closeIgnoringClosed(reader.File), closeUnixDevice(device),
		)
	}
	setup, err := prepareShellIntegration(spec)
	if err != nil {
		return nil, errors.Join(err, closeUnixFiles(writer, reader.File, device))
	}
	// The context bounds startup only. The returned Proc owns the interactive
	// process lifetime after an HTTP create request has completed.
	// #nosec G204 -- argv is the resolved shell command from the validated terminal request.
	command := exec.CommandContext(context.WithoutCancel(ctx), setup.argv[0], setup.argv[1:]...)
	command.Dir = spec.Cwd
	command.Env = environment(setup.env)
	procutil.ConfigureCommandTerminalSession(command)
	if err := ctx.Err(); err != nil {
		return nil, errors.Join(
			fmt.Errorf("terminal pty: start context: %w", err),
			setup.cleanup(), closeUnixFiles(writer, reader.File, device),
		)
	}
	if err := device.Start(command); err != nil {
		return nil, errors.Join(
			fmt.Errorf("terminal pty: start %q: %w", spec.Argv[0], err),
			setup.cleanup(), closeUnixFiles(writer, reader.File, device),
		)
	}
	if runtime.GOOS == "linux" {
		if err := device.Slave().Close(); err != nil {
			cleanupErr := terminateStartedCommand(command)
			return nil, errors.Join(
				fmt.Errorf("terminal pty: close parent slave: %w", err), cleanupErr,
				setup.cleanup(), closeUnixFiles(writer, reader.File, device),
			)
		}
	}
	if err := ctx.Err(); err != nil {
		cleanupErr := terminateStartedCommand(command)
		return nil, errors.Join(
			fmt.Errorf("terminal pty: start context: %w", err),
			cleanupErr, setup.cleanup(), closeUnixFiles(writer, reader.File, device),
		)
	}
	proc := &unixProc{
		device: device, reader: reader, writer: writer, command: command, waiter: newProcessWaiter(),
		startedAt: time.Now().UTC(), shimCleanup: setup.cleanup,
	}
	proc.waiter.start(func() waitResult {
		err := command.Wait()
		return waitResult{exit: classifyExit(err, command), err: waitError(err)}
	})
	return proc, nil
}

func makeUnixMasterNonblocking(device *xpty.UnixPty) error {
	var nonblockingErr error
	if err := device.Control(func(fd uintptr) {
		nonblockingErr = unix.SetNonblock(int(fd), true)
	}); err != nil {
		return err
	}
	return nonblockingErr
}

func duplicateMaster(device *xpty.UnixPty, role string) (*os.File, error) {
	duplicate := -1
	var duplicateErr error
	if err := device.Control(func(fd uintptr) {
		duplicate, duplicateErr = unix.FcntlInt(fd, unix.F_DUPFD_CLOEXEC, 0)
	}); err != nil {
		return nil, err
	}
	if duplicateErr != nil {
		if duplicate >= 0 {
			if closeErr := unix.Close(duplicate); closeErr != nil {
				return nil, errors.Join(duplicateErr, closeErr)
			}
		}
		return nil, duplicateErr
	}
	file := os.NewFile(uintptr(duplicate), device.Name()+"-"+role)
	if file == nil {
		closeErr := unix.Close(duplicate)
		return nil, errors.Join(errors.New("terminal pty: create duplicate file"), closeErr)
	}
	return file, nil
}

func (p *unixProc) PID() int {
	if p == nil || p.command == nil || p.command.Process == nil {
		return 0
	}
	return p.command.Process.Pid
}

func (p *unixProc) ProcessGroupID() int { return p.PID() }

func (p *unixProc) StartedAt() time.Time { return p.startedAt }

func (p *unixProc) Reader() io.Reader { return p.reader }

func (p *unixProc) Write(input []byte) (int, error) {
	if err := p.io.begin(); err != nil {
		return 0, err
	}
	defer p.io.end()
	p.inputMu.Lock()
	defer p.inputMu.Unlock()
	return p.write(input)
}

func (p *unixProc) write(input []byte) (int, error) {
	written, err := p.writer.Write(input)
	if err != nil {
		return written, fmt.Errorf("terminal pty: write: %w", err)
	}
	return written, nil
}

func (p *unixProc) Resize(cols, rows uint16) error {
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

func (p *unixProc) Wait(ctx context.Context) (Exit, error) {
	exit, err := p.waiter.wait(ctx)
	if err != nil {
		return exit, err
	}
	p.mu.RLock()
	signal := p.killSignal
	p.mu.RUnlock()
	return reportedExitForSignal(signal, exit), nil
}

func (p *unixProc) Kill(signal Signal) error {
	p.signalMu.Lock()
	defer p.signalMu.Unlock()
	return p.kill(signal)
}

func (p *unixProc) kill(signal Signal) error {
	syscallSignal, err := unixSignal(signal)
	if err != nil {
		return err
	}
	p.mu.Lock()
	previousSignal := p.killSignal
	p.killSignal = signal
	p.mu.Unlock()
	if err := procutil.SignalCommandProcessGroup(p.command, syscallSignal); err != nil {
		p.mu.Lock()
		p.killSignal = previousSignal
		p.mu.Unlock()
		return fmt.Errorf("terminal pty: signal process group: %w", err)
	}
	if signal != SignalHUP && signal != SignalTERM {
		return nil
	}
	if err := procutil.WaitForCommandProcessGroupExit(p.command, processGroupGrace); err == nil {
		return nil
	}
	p.mu.Lock()
	p.killSignal = SignalKILL
	p.mu.Unlock()
	if err := procutil.KillCommandProcessGroupAndWait(p.command, processGroupGrace); err != nil {
		return fmt.Errorf("terminal pty: escalate process group: %w", err)
	}
	return nil
}

func (p *unixProc) Close() error {
	p.closeOnce.Do(func() {
		p.io.seal()
		writerErr := closeIgnoringClosed(p.writer)
		readerErr := closeIgnoringClosed(p.reader.File)
		p.io.wait()
		deviceErr := closeUnixDevice(p.device)
		var shimErr error
		if p.shimCleanup != nil {
			shimErr = p.shimCleanup()
		}
		p.closeErr = errors.Join(writerErr, readerErr, deviceErr, shimErr)
	})
	return p.closeErr
}

func closeUnixFiles(writer, reader *os.File, device *xpty.UnixPty) error {
	return errors.Join(closeIgnoringClosed(writer), closeIgnoringClosed(reader), closeUnixDevice(device))
}

func closeUnixDevice(device *xpty.UnixPty) error {
	if device == nil {
		return nil
	}
	return errors.Join(closeIgnoringClosed(device.Master()), closeIgnoringClosed(device.Slave()))
}

func closeIgnoringClosed(file *os.File) error {
	if file == nil {
		return nil
	}
	err := file.Close()
	if errors.Is(err, os.ErrClosed) {
		return nil
	}
	return err
}

func terminateStartedCommand(command *exec.Cmd) error {
	if command == nil || command.Process == nil {
		return nil
	}
	killErr := procutil.KillCommandProcessGroupAndWait(command, processGroupGrace)
	waitErr := command.Wait()
	if _, ok := waitErr.(*exec.ExitError); ok {
		waitErr = nil
	}
	return errors.Join(killErr, waitErr)
}

func unixSignal(signal Signal) (syscall.Signal, error) {
	switch signal {
	case SignalINT:
		return syscall.SIGINT, nil
	case SignalTERM:
		return syscall.SIGTERM, nil
	case SignalKILL:
		return syscall.SIGKILL, nil
	case SignalHUP:
		return syscall.SIGHUP, nil
	default:
		return 0, fmt.Errorf("terminal pty: unsupported signal %q", signal)
	}
}

func processSignal(state *os.ProcessState) string {
	status, ok := state.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() {
		return ""
	}
	return signalName(status.Signal())
}

func signalName(signal syscall.Signal) string {
	switch signal {
	case syscall.SIGINT:
		return string(SignalINT)
	case syscall.SIGTERM:
		return string(SignalTERM)
	case syscall.SIGKILL:
		return string(SignalKILL)
	case syscall.SIGHUP:
		return string(SignalHUP)
	default:
		return signal.String()
	}
}

func waitError(err error) error {
	if errors.As(err, new(*exec.ExitError)) {
		return nil
	}
	return err
}
