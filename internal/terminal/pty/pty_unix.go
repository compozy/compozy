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

const killGrace = time.Second

type unixProc struct {
	device    *xpty.UnixPty
	reader    *os.File
	command   *exec.Cmd
	waiter    processWaiter
	closeOnce sync.Once
	closeErr  error
}

func startInteractive(_ context.Context, spec ProcSpec) (Proc, error) {
	if len(spec.Argv) == 0 || spec.Argv[0] == "" {
		return nil, errors.New("terminal pty: command is required")
	}
	cols, rows := normalizedSize(spec.Cols, spec.Rows)
	device, err := xpty.NewUnixPty(int(cols), int(rows))
	if err != nil {
		return nil, fmt.Errorf("terminal pty: open: %w", err)
	}
	reader, err := duplicateNonblocking(device)
	if err != nil {
		closeErr := closeUnixDevice(device)
		return nil, errors.Join(fmt.Errorf("terminal pty: duplicate master: %w", err), closeErr)
	}
	command := exec.Command(spec.Argv[0], spec.Argv[1:]...)
	command.Dir = spec.Cwd
	command.Env = environment(spec.Env)
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}
	if err := device.Start(command); err != nil {
		return nil, errors.Join(fmt.Errorf("terminal pty: start %q: %w", spec.Argv[0], err), reader.Close(), closeUnixDevice(device))
	}
	if runtime.GOOS == "linux" {
		if err := device.Slave().Close(); err != nil {
			cleanupErr := terminateStartedCommand(command)
			return nil, errors.Join(fmt.Errorf("terminal pty: close parent slave: %w", err), cleanupErr, reader.Close(), device.Master().Close())
		}
	}
	proc := &unixProc{device: device, reader: reader, command: command, waiter: newProcessWaiter()}
	proc.waiter.start(func() waitResult {
		err := command.Wait()
		return waitResult{exit: classifyExit(err, command), err: waitError(err)}
	})
	return proc, nil
}

func duplicateNonblocking(device *xpty.UnixPty) (*os.File, error) {
	duplicate := -1
	var duplicateErr error
	if err := device.Control(func(fd uintptr) {
		duplicate, duplicateErr = unix.FcntlInt(fd, unix.F_DUPFD_CLOEXEC, 0)
		if duplicateErr == nil {
			duplicateErr = unix.SetNonblock(duplicate, true)
		}
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
	return os.NewFile(uintptr(duplicate), device.Name()+"-reader"), nil
}

func (p *unixProc) PID() int {
	if p == nil || p.command == nil || p.command.Process == nil {
		return 0
	}
	return p.command.Process.Pid
}

func (p *unixProc) ProcessGroupID() int { return p.PID() }

func (p *unixProc) Reader() io.Reader { return p.reader }

func (p *unixProc) Write(input []byte) (int, error) {
	written, err := p.device.Write(input)
	if err != nil {
		return written, fmt.Errorf("terminal pty: write: %w", err)
	}
	return written, nil
}

func (p *unixProc) Resize(cols, rows uint16) error {
	cols, rows = normalizedSize(cols, rows)
	if err := p.device.Resize(int(cols), int(rows)); err != nil {
		return fmt.Errorf("terminal pty: resize: %w", err)
	}
	return nil
}

func (p *unixProc) Wait(ctx context.Context) (Exit, error) {
	return p.waiter.wait(ctx)
}

func (p *unixProc) Kill(signal Signal) error {
	syscallSignal, err := unixSignal(signal)
	if err != nil {
		return err
	}
	if err := procutil.SignalCommandProcessGroup(p.command, syscallSignal); err != nil {
		return fmt.Errorf("terminal pty: signal process group: %w", err)
	}
	if signal != SignalHUP && signal != SignalTERM {
		return nil
	}
	if err := procutil.WaitForCommandProcessGroupExit(p.command, killGrace); err == nil {
		return nil
	}
	if err := procutil.KillCommandProcessGroupAndWait(p.command, killGrace); err != nil {
		return fmt.Errorf("terminal pty: escalate process group: %w", err)
	}
	return nil
}

func (p *unixProc) Close() error {
	p.closeOnce.Do(func() {
		p.closeErr = errors.Join(closeIgnoringClosed(p.reader), closeUnixDevice(p.device))
	})
	return p.closeErr
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
	killErr := procutil.KillCommandProcessGroupAndWait(command, killGrace)
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
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return nil
	}
	return err
}
