//go:build windows

package pty

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/compozy/compozy/internal/procutil"
	"golang.org/x/sys/windows"
)

const (
	windowsConsoleModeHelperArg = "--compozy-internal-conpty-mode-helper"
	windowsConsoleModeHelperEnv = "COMPOZY_INTERNAL_CONPTY_MODE_HELPER"
	windowsConsoleModeErrorMask = uint32(1 << 31)
	windowsConsoleModeWaitMS    = uint32(2_000)
)

func init() {
	if len(os.Args) != 2 || os.Args[1] != windowsConsoleModeHelperArg {
		return
	}
	operation := os.Getenv(windowsConsoleModeHelperEnv)
	if operation == "" {
		return
	}
	windowsConsoleModeHelper(operation)
}

func windowsConsoleModeHelper(operation string) {
	input, err := windows.GetStdHandle(windows.STD_INPUT_HANDLE)
	if err != nil {
		os.Exit(int(windowsConsoleModeErrorMask | windowsErrorCode(err)))
	}
	var prior uint32
	if err := windows.GetConsoleMode(input, &prior); err != nil {
		os.Exit(int(windowsConsoleModeErrorMask | windowsErrorCode(err)))
	}
	switch {
	case operation == "query":
	default:
		os.Exit(int(windowsConsoleModeErrorMask | uint32(windows.ERROR_INVALID_PARAMETER)))
	}
	os.Exit(int(prior))
}

func windowsErrorCode(err error) uint32 {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return uint32(errno) &^ windowsConsoleModeErrorMask
	}
	return uint32(windows.ERROR_GEN_FAILURE)
}

func (p *windowsProc) InputVisible() (bool, error) {
	p.inputMu.Lock()
	defer p.inputMu.Unlock()
	if err := p.io.begin(); err != nil {
		return false, err
	}
	defer p.io.end()
	mode, err := p.runConsoleModeHelper("query")
	if err != nil {
		return false, err
	}
	return mode&windows.ENABLE_ECHO_INPUT != 0, nil
}

func (p *windowsProc) WriteRedacted(input []byte) (RedactedWriteResult, error) {
	p.inputMu.Lock()
	defer p.inputMu.Unlock()
	if err := p.io.begin(); err != nil {
		return RedactedWriteResult{}, err
	}
	defer p.io.end()
	mode, err := p.runConsoleModeHelper("query")
	if err != nil {
		return RedactedWriteResult{}, err
	}
	if mode&windows.ENABLE_ECHO_INPUT != 0 {
		return RedactedWriteResult{}, ErrInputVisible
	}
	written, writeErr := writeAllRedactedBytes(input, p.write)
	return RedactedWriteResult{BytesDelivered: written}, writeErr
}

func (p *windowsProc) runConsoleModeHelper(operation string) (uint32, error) {
	executable, err := os.Executable()
	if err != nil {
		return 0, fmt.Errorf("terminal pty: resolve ConPTY mode helper: %w", err)
	}
	helperEnvironment := environment(map[string]string{windowsConsoleModeHelperEnv: operation})
	pid, rawHandle, err := p.device.Spawn(executable, []string{
		executable, windowsConsoleModeHelperArg,
	}, &syscall.ProcAttr{
		Env: helperEnvironment,
		Sys: &syscall.SysProcAttr{
			CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_SUSPENDED,
		},
	})
	if err != nil {
		return 0, fmt.Errorf("terminal pty: start ConPTY mode helper: %w", err)
	}
	process := windows.Handle(rawHandle)
	if err := p.job.assign(process); err != nil {
		cleanupErr := cleanupWindowsConsoleModeHelper(process, pid)
		return 0, errors.Join(fmt.Errorf("terminal pty: assign ConPTY mode helper: %w", err), cleanupErr)
	}
	if err := resumeWindowsProcess(process); err != nil {
		cleanupErr := cleanupWindowsConsoleModeHelper(process, pid)
		return 0, errors.Join(fmt.Errorf("terminal pty: resume ConPTY mode helper: %w", err), cleanupErr)
	}
	event, waitErr := windows.WaitForSingleObject(process, windowsConsoleModeWaitMS)
	if waitErr == nil && event != windows.WAIT_OBJECT_0 {
		waitErr = fmt.Errorf("unexpected wait status %d", event)
	}
	if waitErr != nil {
		cleanupErr := cleanupWindowsConsoleModeHelper(process, pid)
		return 0, errors.Join(fmt.Errorf("terminal pty: await ConPTY mode helper: %w", waitErr), cleanupErr)
	}
	var code uint32
	if err := windows.GetExitCodeProcess(process, &code); err != nil {
		closeErr := closeWindowsProcessHandle(process, pid)
		return 0, errors.Join(fmt.Errorf("terminal pty: read ConPTY mode helper exit: %w", err), closeErr)
	}
	closeErr := closeWindowsProcessHandle(process, pid)
	if closeErr != nil {
		return 0, closeErr
	}
	if code&windowsConsoleModeErrorMask != 0 {
		return 0, fmt.Errorf("terminal pty: ConPTY mode helper: %w", syscall.Errno(code&^windowsConsoleModeErrorMask))
	}
	return code, nil
}

func cleanupWindowsConsoleModeHelper(process windows.Handle, pid int) error {
	terminateErr := procutil.TerminateProcessHandle(uintptr(process), windowsJobExitCode)
	event, waitErr := windows.WaitForSingleObject(process, windowsConsoleModeWaitMS)
	if waitErr == nil && event != windows.WAIT_OBJECT_0 {
		waitErr = fmt.Errorf("unexpected cleanup wait status %d", event)
	}
	closeErr := closeWindowsProcessHandle(process, pid)
	return errors.Join(terminateErr, waitErr, closeErr)
}
