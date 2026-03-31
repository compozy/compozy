package run

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

	"github.com/charmbracelet/x/vt"
	"github.com/charmbracelet/x/xpty"
)

const (
	defaultTerminalWidth     = 80
	defaultTerminalHeight    = 24
	terminalCloseGracePeriod = 250 * time.Millisecond
	terminalCloseTimeout     = 2 * time.Second
	terminalWaitPollInterval = 10 * time.Millisecond
)

// Terminal wraps a PTY-backed process and a VT emulator for a single job.
type Terminal struct {
	emu   *vt.SafeEmulator
	pty   xpty.Pty
	cmd   *exec.Cmd
	jobID string

	width  int
	height int

	aliveMu sync.RWMutex
	alive   bool

	stateMu sync.Mutex
	started bool
	closed  bool

	outputErr    error
	responseErr  error
	processErr   error
	outputMirror io.Writer

	closeOnce sync.Once
	wg        sync.WaitGroup

	outputDone   chan struct{}
	responseDone chan struct{}
	processDone  chan struct{}
}

type terminalCloseState struct {
	cmd          *exec.Cmd
	pty          xpty.Pty
	outputDone   chan struct{}
	responseDone chan struct{}
	processDone  chan struct{}
}

// NewTerminal constructs a terminal wrapper with the requested initial size.
func NewTerminal(width, height int, jobID string) *Terminal {
	width, height = normalizeTerminalSize(width, height)

	return &Terminal{
		emu:    vt.NewSafeEmulator(width, height),
		width:  width,
		height: height,
		jobID:  jobID,
	}
}

// Start creates a PTY, starts the command within it, and begins terminal I/O
// pumps plus process exit tracking.
func (t *Terminal) Start(cmd *exec.Cmd) error {
	if cmd == nil {
		return errors.New("command is nil")
	}

	t.stateMu.Lock()
	defer t.stateMu.Unlock()

	if t.closed {
		return errors.New("terminal is closed")
	}
	if t.started {
		return errors.New("terminal already started")
	}

	width, height := normalizeTerminalSize(t.width, t.height)
	pty, err := xpty.NewPty(width, height)
	if err != nil {
		return fmt.Errorf("create pty: %w", err)
	}
	if err := pty.Start(cmd); err != nil {
		closeErr := pty.Close()
		if closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close pty after start failure: %w", closeErr))
		}
		return fmt.Errorf("start command in pty: %w", err)
	}

	t.pty = pty
	t.cmd = cmd
	t.width = width
	t.height = height
	t.started = true
	t.outputDone = make(chan struct{})
	t.responseDone = make(chan struct{})
	t.processDone = make(chan struct{})
	t.setAlive(true)

	t.wg.Add(3)
	go t.copyPTYToEmulator()
	go t.forwardEmulatorResponses()
	go t.waitForProcessExit()

	return nil
}

func (t *Terminal) SetOutputMirror(dst io.Writer) {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	t.outputMirror = dst
}

// Render returns the emulator's rendered screen buffer.
func (t *Terminal) Render() string {
	if t.emu == nil {
		return ""
	}
	return t.emu.Render()
}

// WriteInput writes raw input bytes to the PTY.
func (t *Terminal) WriteInput(p []byte) error {
	pty, err := t.currentPTY()
	if err != nil {
		return err
	}

	if _, err := pty.Write(p); err != nil {
		return fmt.Errorf("write input to pty: %w", err)
	}
	return nil
}

// Resize updates both the PTY and emulator dimensions.
func (t *Terminal) Resize(width, height int) {
	width, height = normalizeTerminalSize(width, height)

	t.stateMu.Lock()
	defer t.stateMu.Unlock()

	if t.closed {
		return
	}

	if t.pty != nil {
		if err := t.pty.Resize(width, height); err != nil {
			return
		}
	}

	t.width = width
	t.height = height
	t.emu.Resize(width, height)
}

// IsAlive reports whether the underlying process is still running.
func (t *Terminal) IsAlive() bool {
	t.aliveMu.RLock()
	defer t.aliveMu.RUnlock()
	return t.alive
}

func (t *Terminal) ProcessError() error {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	return t.processErr
}

func (t *Terminal) ExitCode() int {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	if t.cmd == nil || t.cmd.ProcessState == nil {
		return -1
	}
	return t.cmd.ProcessState.ExitCode()
}

// Close terminates the process if needed and shuts down the PTY and emulator.
func (t *Terminal) Close() error {
	var closeErr error

	t.closeOnce.Do(func() {
		state, started := t.beginClose()
		if !started {
			closeErr = t.closeUnstarted()
			return
		}

		closeErr = t.closeStarted(state)
	})

	return closeErr
}

func (t *Terminal) copyPTYToEmulator() {
	defer t.wg.Done()
	defer close(t.outputDone)

	if t.pty == nil || t.emu == nil {
		return
	}

	buf := make([]byte, 32*1024)
	for {
		n, err := t.pty.Read(buf)
		if n > 0 {
			chunk := append([]byte(nil), buf[:n]...)
			if _, writeErr := t.emu.Write(chunk); writeErr != nil {
				t.recordOutputErr(fmt.Errorf("write PTY output to emulator: %w", writeErr))
				return
			}
			if mirrorErr := t.writeOutputMirror(chunk); mirrorErr != nil {
				t.recordOutputErr(fmt.Errorf("write PTY output mirror: %w", mirrorErr))
				return
			}
		}
		if err != nil {
			if !isTerminalPumpShutdownError(err) {
				t.recordOutputErr(fmt.Errorf("read PTY output: %w", err))
			}
			return
		}
	}
}

func (t *Terminal) forwardEmulatorResponses() {
	defer t.wg.Done()
	defer close(t.responseDone)

	if t.pty == nil || t.emu == nil {
		return
	}

	if _, err := io.Copy(t.pty, t.emu); err != nil && !isTerminalPumpShutdownError(err) {
		t.recordResponseErr(fmt.Errorf("forward emulator responses to PTY: %w", err))
	}
}

func (t *Terminal) waitForProcessExit() {
	defer t.wg.Done()
	defer close(t.processDone)

	if t.cmd == nil {
		t.setAlive(false)
		return
	}

	if err := xpty.WaitProcess(context.Background(), t.cmd); err != nil && !errors.Is(err, os.ErrProcessDone) {
		t.recordProcessErr(fmt.Errorf("wait for process exit: %w", err))
	}
	t.setAlive(false)
}

func (t *Terminal) currentPTY() (xpty.Pty, error) {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()

	if t.closed {
		return nil, errors.New("terminal is closed")
	}
	if !t.started || t.pty == nil {
		return nil, errors.New("terminal not started")
	}
	return t.pty, nil
}

func (t *Terminal) setAlive(alive bool) {
	t.aliveMu.Lock()
	defer t.aliveMu.Unlock()
	t.alive = alive
}

func (t *Terminal) beginClose() (terminalCloseState, bool) {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()

	t.closed = true
	return terminalCloseState{
		cmd:          t.cmd,
		pty:          t.pty,
		outputDone:   t.outputDone,
		responseDone: t.responseDone,
		processDone:  t.processDone,
	}, t.started
}

func (t *Terminal) closeUnstarted() error {
	defer t.setAlive(false)
	return t.closeEmulator()
}

func (t *Terminal) closeStarted(state terminalCloseState) error {
	defer t.setAlive(false)

	var closeErr error
	closeErr = errors.Join(closeErr, terminateProcess(state.cmd, state.processDone))
	closeErr = errors.Join(closeErr, closeTerminalPTY(state.pty))
	closeErr = errors.Join(closeErr, waitForPumpShutdown(state.outputDone, "PTY output pump"))
	closeErr = errors.Join(closeErr, closeEmulatorInputPipe(t.emu))
	closeErr = errors.Join(closeErr, waitForPumpShutdown(state.responseDone, "emulator response pump"))
	closeErr = errors.Join(closeErr, t.closeEmulator())
	closeErr = errors.Join(closeErr, waitForPumpShutdown(state.processDone, "process shutdown"))
	closeErr = errors.Join(closeErr, t.waitForGoroutines())
	return closeErr
}

func (t *Terminal) closeEmulator() error {
	if t.emu == nil {
		return nil
	}
	if err := t.emu.Close(); err != nil &&
		!errors.Is(err, io.EOF) &&
		!errors.Is(err, io.ErrClosedPipe) {
		return fmt.Errorf("close emulator: %w", err)
	}
	return nil
}

func (t *Terminal) waitForGoroutines() error {
	allDone := make(chan struct{})
	go func() {
		t.wg.Wait()
		close(allDone)
	}()

	return waitForPumpShutdown(allDone, "terminal goroutines")
}

func (t *Terminal) recordOutputErr(err error) {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	t.outputErr = err
}

func (t *Terminal) recordResponseErr(err error) {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	t.responseErr = err
}

func (t *Terminal) recordProcessErr(err error) {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	t.processErr = err
}

func (t *Terminal) writeOutputMirror(chunk []byte) error {
	t.stateMu.Lock()
	mirror := t.outputMirror
	t.stateMu.Unlock()
	if mirror == nil || len(chunk) == 0 {
		return nil
	}
	_, err := mirror.Write(chunk)
	return err
}

func normalizeTerminalSize(width, height int) (int, int) {
	if width <= 0 {
		width = defaultTerminalWidth
	}
	if height <= 0 {
		height = defaultTerminalHeight
	}
	return width, height
}

func closeEmulatorInputPipe(emu *vt.SafeEmulator) error {
	if emu == nil {
		return nil
	}

	closer, ok := emu.InputPipe().(io.Closer)
	if !ok {
		return errors.New("emulator input pipe is not closable")
	}

	if err := closer.Close(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
		return fmt.Errorf("close emulator input pipe: %w", err)
	}
	return nil
}

func closeTerminalPTY(pty xpty.Pty) error {
	if pty == nil {
		return nil
	}
	if err := pty.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
		return fmt.Errorf("close pty: %w", err)
	}
	return nil
}

func terminateProcess(cmd *exec.Cmd, done <-chan struct{}) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	if waitForTerminalChannel(done, 0) {
		return nil
	}

	if runtime.GOOS != "windows" {
		if err := cmd.Process.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return fmt.Errorf("interrupt process: %w", err)
		}
		if waitForTerminalChannel(done, terminalCloseGracePeriod) {
			return nil
		}
	}

	if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("kill process: %w", err)
	}
	if !waitForTerminalChannel(done, terminalCloseTimeout) {
		return errors.New("process did not exit after kill")
	}
	return nil
}

func waitForPumpShutdown(ch <-chan struct{}, name string) error {
	if ok := waitForTerminalChannel(ch, terminalCloseTimeout); !ok {
		return fmt.Errorf("timed out waiting for %s", name)
	}
	return nil
}

func waitForTerminalChannel(ch <-chan struct{}, timeout time.Duration) bool {
	if ch == nil {
		return true
	}
	if timeout <= 0 {
		select {
		case <-ch:
			return true
		default:
			return false
		}
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-ch:
		return true
	case <-timer.C:
		return false
	}
}

func isTerminalPumpShutdownError(err error) bool {
	return err == nil ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrClosedPipe) ||
		errors.Is(err, os.ErrClosed) ||
		errors.Is(err, syscall.EIO)
}
