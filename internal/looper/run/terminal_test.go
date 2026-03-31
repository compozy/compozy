package run

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	xterm "github.com/charmbracelet/x/term"
	"github.com/charmbracelet/x/vt"
)

const (
	terminalHelperEnv     = "GO_WANT_TERMINAL_HELPER_PROCESS"
	terminalHelperModeEnv = "TERMINAL_HELPER_MODE"
	testTerminalTimeout   = 5 * time.Second
)

func TestTerminalHelperProcess(_ *testing.T) {
	if os.Getenv(terminalHelperEnv) != "1" {
		return
	}

	mode := os.Getenv(terminalHelperModeEnv)
	switch mode {
	case "echo":
		_, _ = os.Stdout.WriteString("hello")
	case "stdin-line":
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil {
			_, _ = fmt.Fprintf(os.Stdout, "read-error:%v", err)
			os.Exit(1)
		}
		_, _ = fmt.Fprintf(os.Stdout, "received:%s", line)
	case "hold":
		_, _ = os.Stdout.WriteString("holding\n")
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil {
			_, _ = fmt.Fprintf(os.Stdout, "hold-error:%v", err)
			os.Exit(1)
		}
		_, _ = fmt.Fprintf(os.Stdout, "release:%s", line)
	case "query-cpr":
		state, err := xterm.MakeRaw(os.Stdin.Fd())
		if err != nil {
			_, _ = fmt.Fprintf(os.Stdout, "raw-mode-error:%v", err)
			os.Exit(1)
		}
		_, _ = os.Stdout.WriteString("querying\n")
		_, _ = os.Stdout.WriteString("\x1b[6n")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('R')
		if err != nil {
			_ = xterm.Restore(os.Stdin.Fd(), state)
			_, _ = fmt.Fprintf(os.Stdout, "response-error:%v", err)
			os.Exit(1)
		}
		if err := xterm.Restore(os.Stdin.Fd(), state); err != nil {
			_, _ = fmt.Fprintf(os.Stdout, "restore-error:%v", err)
			os.Exit(1)
		}
		_, _ = fmt.Fprintf(os.Stdout, "response:%q", response)
	case "echo-loop":
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := scanner.Text()
			_, _ = fmt.Fprintf(os.Stdout, "echo:%s\n", line)
			if line == "quit" {
				os.Exit(0)
			}
		}
		if err := scanner.Err(); err != nil {
			_, _ = fmt.Fprintf(os.Stdout, "scan-error:%v", err)
			os.Exit(1)
		}
	default:
		_, _ = fmt.Fprintf(os.Stderr, "unknown helper mode: %s", mode)
		os.Exit(2)
	}

	os.Exit(0)
}

func TestTerminalStartAndRender(t *testing.T) {
	t.Parallel()

	term := newStartedTerminal(t, "echo")

	waitForRenderContains(t, term, "hello")
	waitForAliveState(t, term, false)
}

func TestTerminalWriteInput(t *testing.T) {
	t.Parallel()

	term := newStartedTerminal(t, "stdin-line")

	if err := term.WriteInput([]byte("ping\n")); err != nil {
		t.Fatalf("WriteInput() error = %v", err)
	}

	waitForRenderContains(t, term, "received:ping")
	waitForAliveState(t, term, false)
}

func TestTerminalResizeSyncsPTYAndEmulator(t *testing.T) {
	t.Parallel()

	term := newStartedTerminal(t, "hold")

	term.Resize(120, 40)

	width, height, err := term.pty.Size()
	if err != nil {
		t.Fatalf("pty.Size() error = %v", err)
	}
	if width != 120 || height != 40 {
		t.Fatalf("pty size = %dx%d, want 120x40", width, height)
	}
	if got := term.emu.Width(); got != 120 {
		t.Fatalf("emulator width = %d, want 120", got)
	}
	if got := term.emu.Height(); got != 40 {
		t.Fatalf("emulator height = %d, want 40", got)
	}
}

func TestTerminalIsAliveTracksProcessLifecycle(t *testing.T) {
	t.Parallel()

	term := newStartedTerminal(t, "hold")

	waitForAliveState(t, term, true)

	if err := term.WriteInput([]byte("release\n")); err != nil {
		t.Fatalf("WriteInput() error = %v", err)
	}

	waitForRenderContains(t, term, "release:release")
	waitForAliveState(t, term, false)
}

func TestTerminalCloseTerminatesProcessAndStopsPumps(t *testing.T) {
	t.Parallel()

	term := newStartedTerminal(t, "hold")

	if err := term.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	waitForAliveState(t, term, false)
	assertClosedChannel(t, term.outputDone, "outputDone")
	assertClosedChannel(t, term.responseDone, "responseDone")
	assertClosedChannel(t, term.processDone, "processDone")

	if err := term.WriteInput([]byte("after-close\n")); err == nil {
		t.Fatal("WriteInput() after Close() unexpectedly succeeded")
	}
}

func TestTerminalConcurrentRenderAndWriteInput(t *testing.T) {
	t.Parallel()

	term := newStartedTerminal(t, "echo-loop")

	var wg sync.WaitGroup
	errCh := make(chan error, 1)

	for worker := range 4 {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := range 10 {
				if _, err := io.Discard.Write([]byte(term.Render())); err != nil {
					select {
					case errCh <- fmt.Errorf("render discard: %w", err):
					default:
					}
					return
				}

				input := fmt.Sprintf("worker-%d-%d\n", worker, i)
				if err := term.WriteInput([]byte(input)); err != nil {
					select {
					case errCh <- err:
					default:
					}
					return
				}
			}
		}(worker)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent terminal operations failed: %v", err)
		}
	}

	waitForRenderContains(t, term, "echo:worker-")

	if err := term.WriteInput([]byte("quit\n")); err != nil {
		t.Fatalf("WriteInput() error = %v", err)
	}
	waitForRenderContains(t, term, "echo:quit")
	waitForAliveState(t, term, false)
}

func TestTerminalForwardsEmulatorResponsesToPTY(t *testing.T) {
	t.Parallel()

	term := newStartedTerminal(t, "query-cpr")

	waitForRenderContains(t, term, `response:"\x1b[`)
	waitForAliveState(t, term, false)
}

func TestTerminalLifecycleErrors(t *testing.T) {
	t.Parallel()

	t.Run("write before start", func(t *testing.T) {
		t.Parallel()

		term := NewTerminal(80, 24, t.Name())
		if err := term.WriteInput([]byte("ping")); err == nil {
			t.Fatal("WriteInput() before Start() unexpectedly succeeded")
		}
	})

	t.Run("start with nil command", func(t *testing.T) {
		t.Parallel()

		term := NewTerminal(80, 24, t.Name())
		if err := term.Start(nil); err == nil {
			t.Fatal("Start(nil) unexpectedly succeeded")
		}
	})

	t.Run("start command failure", func(t *testing.T) {
		t.Parallel()

		term := NewTerminal(80, 24, t.Name())
		cmd := exec.CommandContext(
			context.Background(),
			"this-command-does-not-exist-anywhere",
		)
		if err := term.Start(cmd); err == nil {
			t.Fatal("Start() with invalid command unexpectedly succeeded")
		}
	})

	t.Run("double start", func(t *testing.T) {
		t.Parallel()

		term := newStartedTerminal(t, "hold")
		if err := term.Start(helperCommand(t, "echo")); err == nil {
			t.Fatal("second Start() unexpectedly succeeded")
		}
	})

	t.Run("close before start", func(t *testing.T) {
		t.Parallel()

		term := NewTerminal(0, 0, t.Name())
		if err := term.Close(); err != nil {
			t.Fatalf("Close() before Start() error = %v", err)
		}
		if err := term.Start(helperCommand(t, "echo")); err == nil {
			t.Fatal("Start() after Close() unexpectedly succeeded")
		}
		if got := term.width; got != defaultTerminalWidth {
			t.Fatalf("terminal width = %d, want %d", got, defaultTerminalWidth)
		}
		if got := term.height; got != defaultTerminalHeight {
			t.Fatalf("terminal height = %d, want %d", got, defaultTerminalHeight)
		}
	})
}

func TestTerminalInternalHelpers(t *testing.T) {
	t.Parallel()

	t.Run("render without emulator", func(t *testing.T) {
		t.Parallel()

		term := &Terminal{}
		if got := term.Render(); got != "" {
			t.Fatalf("Render() = %q, want empty string", got)
		}
	})

	t.Run("close emulator input pipe nil", func(t *testing.T) {
		t.Parallel()

		if err := closeEmulatorInputPipe(nil); err != nil {
			t.Fatalf("closeEmulatorInputPipe(nil) error = %v", err)
		}
	})

	t.Run("close emulator input pipe already closed", func(t *testing.T) {
		t.Parallel()

		emu := vt.NewSafeEmulator(80, 24)
		closer := emu.InputPipe().(io.Closer)
		if err := closer.Close(); err != nil {
			t.Fatalf("closing emulator input pipe error = %v", err)
		}
		if err := closeEmulatorInputPipe(emu); err != nil {
			t.Fatalf("closeEmulatorInputPipe() error = %v", err)
		}
		if err := emu.Close(); err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("emu.Close() error = %v", err)
		}
	})

	t.Run("terminate process nil command", func(t *testing.T) {
		t.Parallel()

		if err := terminateProcess(nil, nil); err != nil {
			t.Fatalf("terminateProcess(nil, nil) error = %v", err)
		}
	})

	t.Run("terminate process done already closed", func(t *testing.T) {
		t.Parallel()

		cmd := helperCommand(t, "hold")
		if err := cmd.Start(); err != nil {
			t.Fatalf("cmd.Start() error = %v", err)
		}
		t.Cleanup(func() {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
				_, _ = cmd.Process.Wait()
			}
		})

		done := make(chan struct{})
		close(done)

		if err := terminateProcess(cmd, done); err != nil {
			t.Fatalf("terminateProcess() error = %v", err)
		}
		if cmd.ProcessState != nil {
			t.Fatal("terminateProcess() unexpectedly waited for the process")
		}
	})

	t.Run("wait for process exit without command", func(t *testing.T) {
		t.Parallel()

		term := &Terminal{
			processDone: make(chan struct{}),
		}
		term.setAlive(true)
		term.wg.Add(1)

		term.waitForProcessExit()

		if term.IsAlive() {
			t.Fatal("terminal remained alive without a command")
		}
		assertClosedChannel(t, term.processDone, "processDone")
	})

	t.Run("wait for terminal channel", func(t *testing.T) {
		t.Parallel()

		if !waitForTerminalChannel(nil, time.Millisecond) {
			t.Fatal("waitForTerminalChannel(nil) = false, want true")
		}

		closedCh := make(chan struct{})
		close(closedCh)
		if !waitForTerminalChannel(closedCh, 0) {
			t.Fatal("waitForTerminalChannel(closed, 0) = false, want true")
		}

		openCh := make(chan struct{})
		if waitForTerminalChannel(openCh, 0) {
			t.Fatal("waitForTerminalChannel(open, 0) = true, want false")
		}
		if waitForTerminalChannel(openCh, 5*time.Millisecond) {
			t.Fatal("waitForTerminalChannel(open, timeout) = true, want false")
		}
	})
}

func newStartedTerminal(t *testing.T, mode string) *Terminal {
	t.Helper()

	term := NewTerminal(80, 24, t.Name())
	if err := term.Start(helperCommand(t, mode)); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	t.Cleanup(func() {
		if err := term.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			t.Fatalf("Close() cleanup error = %v", err)
		}
	})

	return term
}

func helperCommand(t *testing.T, mode string) *exec.Cmd {
	t.Helper()

	cmd := exec.CommandContext(
		context.Background(),
		os.Args[0],
		"-test.run=^TestTerminalHelperProcess$",
	)
	cmd.Env = append(os.Environ(),
		terminalHelperEnv+"=1",
		terminalHelperModeEnv+"="+mode,
	)
	return cmd
}

func waitForRenderContains(t *testing.T, term *Terminal, want string) {
	t.Helper()

	waitForCondition(t, func() bool {
		return strings.Contains(term.Render(), want)
	}, "render to contain "+want)
}

func waitForAliveState(t *testing.T, term *Terminal, want bool) {
	t.Helper()

	waitForCondition(t, func() bool {
		return term.IsAlive() == want
	}, fmt.Sprintf("alive state %t", want))
}

func waitForCondition(t *testing.T, condition func() bool, description string) {
	t.Helper()

	deadline := time.Now().Add(testTerminalTimeout)
	for {
		if condition() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", description)
		}
		time.Sleep(terminalWaitPollInterval)
	}
}

func assertClosedChannel(t *testing.T, ch <-chan struct{}, name string) {
	t.Helper()

	select {
	case <-ch:
	default:
		t.Fatalf("%s was not closed", name)
	}
}
