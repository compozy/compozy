//go:build darwin || linux

package pty

// Suite: Unix terminal substrate.
// Invariant: PTY reads are independently pollable/closable, process groups are isolated, and exits are reported honestly.
// Boundary IN: process specs and terminal control. Boundary OUT: OS PTY, process group, and merged pipe output.

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/charmbracelet/x/xpty"
	"golang.org/x/sys/unix"
)

func TestUnixPTYHardening(t *testing.T) {
	t.Run("Should keep the hardened reader pollable before terminal control [UT-002]", func(t *testing.T) {
		proc := startTestProc(t, ProcSpec{Argv: []string{"sh", "-c", "sleep 300"}, Mode: ModePTY, Cols: 80, Rows: 24})
		defer stopTestProc(t, proc)
		reader := proc.(*unixProc).reader
		if err := reader.SetReadDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
			t.Fatalf("SetReadDeadline(fresh hardened reader) error = %v", err)
		}
		buffer := make([]byte, 1)
		if _, err := reader.Read(buffer); !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatalf("Read(fresh hardened reader) error = %v, want deadline exceeded", err)
		}
		if err := reader.SetReadDeadline(time.Time{}); err != nil {
			t.Fatalf("clear read deadline error = %v", err)
		}
	})

	t.Run("Should demonstrate why a no-child close probe is insufficient [UT-002]", func(t *testing.T) {
		device, err := xpty.NewUnixPty(80, 24)
		if err != nil {
			t.Fatalf("xpty.NewUnixPty() error = %v", err)
		}
		readDone := make(chan error, 1)
		go func() {
			buffer := make([]byte, 1)
			_, readErr := device.Master().Read(buffer)
			readDone <- readErr
		}()
		select {
		case err := <-readDone:
			t.Fatalf("no-child read returned before close: %v", err)
		case <-time.After(30 * time.Millisecond):
		}
		if err := device.Master().Close(); err != nil {
			t.Fatalf("master.Close() error = %v", err)
		}
		select {
		case err := <-readDone:
			t.Fatalf("master-only close unexpectedly completed the no-child probe: %v", err)
		case <-time.After(30 * time.Millisecond):
		}
		if err := device.Slave().Close(); err != nil {
			t.Fatalf("slave.Close() error = %v", err)
		}
		select {
		case <-readDone:
		case <-time.After(200 * time.Millisecond):
			t.Fatal("closing both no-child PTY handles did not unblock the naive probe")
		}
	})

	t.Run("Should unblock a parked read within 200ms while the child is alive [UT-001][UT-002]", func(t *testing.T) {
		proc := startTestProc(t, ProcSpec{Argv: []string{"sh", "-c", "sleep 300"}, Mode: ModePTY, Cols: 80, Rows: 24})
		readDone := make(chan error, 1)
		go func() {
			buffer := make([]byte, 1)
			_, err := proc.Reader().Read(buffer)
			readDone <- err
		}()
		select {
		case err := <-readDone:
			t.Fatalf("live-child read returned before close: %v", err)
		case <-time.After(30 * time.Millisecond):
		}
		started := time.Now()
		if err := proc.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		select {
		case <-readDone:
			if elapsed := time.Since(started); elapsed > 200*time.Millisecond {
				t.Fatalf("read unblock took %s, want <= 200ms", elapsed)
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatal("Close() did not unblock live-child read")
		}
		stopTestProc(t, proc)
	})

	t.Run("Should isolate the child process group and controlling terminal [UT-003]", func(t *testing.T) {
		proc := startTestProc(t, ProcSpec{Argv: []string{"sh", "-c", "sleep 300"}, Mode: ModePTY, Cols: 80, Rows: 24})
		defer stopTestProc(t, proc)
		unixProcess, ok := proc.(*unixProc)
		if !ok {
			t.Fatalf("proc type = %T, want *unixProc", proc)
		}
		foregroundGroup := 0
		var ioctlErr error
		if err := unixProcess.device.Control(func(fd uintptr) {
			foregroundGroup, ioctlErr = unix.IoctlGetInt(int(fd), unix.TIOCGPGRP)
		}); err != nil {
			t.Fatalf("SyscallConn.Control() error = %v", err)
		}
		if ioctlErr != nil {
			t.Fatalf("TIOCGPGRP error = %v", ioctlErr)
		}
		if foregroundGroup != unixProcess.PID() || foregroundGroup == syscall.Getpgrp() {
			t.Fatalf("foreground pgid=%d child pid=%d daemon pgid=%d", foregroundGroup, unixProcess.PID(), syscall.Getpgrp())
		}
	})

	t.Run("Should propagate resize through the controlled master [UT-004]", func(t *testing.T) {
		executable, err := os.Executable()
		if err != nil {
			t.Fatalf("os.Executable() error = %v", err)
		}
		proc := startTestProc(t, ProcSpec{
			Argv: []string{executable, "-test.run=TestPTYWinsizeHelperProcess"},
			Env:  map[string]string{"GO_WANT_PTY_WINSIZE_HELPER": "1"}, Mode: ModePTY, Cols: 80, Rows: 24,
		})
		unixProcess := proc.(*unixProc)
		if err := unixProcess.reader.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			t.Fatalf("SetReadDeadline() error = %v", err)
		}
		reader := bufio.NewReader(unixProcess.reader)
		if line, err := readUntilContaining(reader, "ready"); err != nil {
			t.Fatalf("read readiness = %q error=%v", line, err)
		}
		if err := proc.Resize(140, 40); err != nil {
			t.Fatalf("Resize() error = %v", err)
		}
		line, err := readUntilContaining(reader, "40 140")
		if err != nil {
			t.Fatalf("read SIGWINCH size = %q error=%v", line, err)
		}
		exit, err := proc.Wait(context.Background())
		if err != nil || exit.Code == nil || *exit.Code != 0 {
			t.Fatalf("Wait() = %#v error=%v", exit, err)
		}
		if err := proc.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	t.Run("Should preserve read deadlines after a controlled resize [UT-009]", func(t *testing.T) {
		proc := startTestProc(t, ProcSpec{Argv: []string{"sh", "-c", "sleep 300"}, Mode: ModePTY, Cols: 80, Rows: 24})
		defer stopTestProc(t, proc)
		unixProcess := proc.(*unixProc)
		if err := proc.Resize(140, 40); err != nil {
			t.Fatalf("Resize() error = %v", err)
		}
		width, height, err := unixProcess.device.Size()
		if err != nil {
			t.Fatalf("Size() error = %v", err)
		}
		if width != 140 || height != 40 {
			t.Fatalf("size = %dx%d, want 140x40", width, height)
		}
		if err := unixProcess.reader.SetReadDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
			t.Fatalf("SetReadDeadline() error = %v", err)
		}
		buffer := make([]byte, 1)
		_, err = unixProcess.reader.Read(buffer)
		if !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatalf("Read() error = %v, want deadline exceeded", err)
		}
	})

	t.Run("Should kill an ignored-HUP process group after the grace period [UT-006][UT-007]", func(t *testing.T) {
		proc := startTestProc(t, ProcSpec{
			Argv: []string{"sh", "-c", `trap "" HUP TERM; echo ready; sleep 300 & wait`}, Mode: ModePTY, Cols: 80, Rows: 24,
		})
		unixProcess := proc.(*unixProc)
		if err := unixProcess.reader.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			t.Fatalf("SetReadDeadline() error = %v", err)
		}
		line, err := bufio.NewReader(unixProcess.reader).ReadString('\n')
		if err != nil || !strings.Contains(line, "ready") {
			t.Fatalf("read readiness = %q error=%v", line, err)
		}
		if err := unixProcess.reader.SetReadDeadline(time.Time{}); err != nil {
			t.Fatalf("clear read deadline error = %v", err)
		}
		started := time.Now()
		if err := proc.Kill(SignalHUP); err != nil {
			t.Fatalf("Kill(HUP) error = %v", err)
		}
		if elapsed := time.Since(started); elapsed > killGrace+500*time.Millisecond {
			t.Fatalf("kill escalation took %s", elapsed)
		}
		exit, err := proc.Wait(context.Background())
		if err != nil {
			t.Fatalf("Wait() error = %v", err)
		}
		if exit.Cause != "signaled" || exit.Signal == nil || *exit.Signal != string(SignalKILL) {
			t.Fatalf("exit = %#v, want SIGKILL", exit)
		}
		if err := unix.Kill(-unixProcess.ProcessGroupID(), 0); !errors.Is(err, unix.ESRCH) {
			t.Fatalf("process group %d still exists: %v", unixProcess.ProcessGroupID(), err)
		}
		if err := proc.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})
}

func TestPipeRunnerContract(t *testing.T) {
	t.Parallel()

	t.Run("Should merge output scrub host terminal identity and pin TERM [UT-008]", func(t *testing.T) {
		t.Parallel()
		proc := startTestProc(t, ProcSpec{
			Argv: []string{"sh", "-c", `printf 'a\n'; printf 'b\n' >&2; printf '%s|%s|%s\n' "$TERM" "$COLORTERM" "${KITTY_WINDOW_ID-unset}"`},
			Env:  map[string]string{"KITTY_WINDOW_ID": "secret", "TERM_PROGRAM": "host"}, Mode: ModePipe,
		})
		output, readErr := io.ReadAll(proc.Reader())
		if readErr != nil {
			t.Fatalf("ReadAll() error = %v", readErr)
		}
		exit, waitErr := proc.Wait(context.Background())
		if waitErr != nil || exit.Code == nil || *exit.Code != 0 {
			t.Fatalf("Wait() = %#v error=%v", exit, waitErr)
		}
		got := string(output)
		if !strings.Contains(got, "a\nb\n") || !strings.Contains(got, "xterm-256color|truecolor|unset") {
			t.Fatalf("pipe output = %q", got)
		}
		if err := proc.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	t.Run("Should report normal signal and unknown exit causes honestly [UT-010]", func(t *testing.T) {
		t.Parallel()
		normal := startTestProc(t, ProcSpec{Argv: []string{"sh", "-c", "exit 7"}, Mode: ModePipe})
		exit, err := normal.Wait(context.Background())
		if err != nil || exit.Cause != "exited" || exit.Code == nil || *exit.Code != 7 {
			t.Fatalf("normal exit = %#v error=%v", exit, err)
		}
		if closeErr := normal.Close(); closeErr != nil {
			t.Fatalf("Close(normal) error = %v", closeErr)
		}

		signaled := startTestProc(t, ProcSpec{Argv: []string{"sh", "-c", "sleep 300"}, Mode: ModePipe})
		if err := signaled.Kill(SignalKILL); err != nil {
			t.Fatalf("Kill(KILL) error = %v", err)
		}
		exit, err = signaled.Wait(context.Background())
		if err != nil || exit.Cause != "signaled" || exit.Signal == nil || *exit.Signal != string(SignalKILL) || exit.Code != nil {
			t.Fatalf("signaled exit = %#v error=%v", exit, err)
		}
		if closeErr := signaled.Close(); closeErr != nil {
			t.Fatalf("Close(signaled) error = %v", closeErr)
		}

		if unknown := classifyExit(errors.New("lost wait status"), nil); unknown.Cause != "unknown" || unknown.Code != nil {
			t.Fatalf("unknown exit = %#v", unknown)
		}
	})

	t.Run("Should escalate an ignored pipe HUP to a process-group kill [UT-007]", func(t *testing.T) {
		t.Parallel()
		proc := startTestProc(t, ProcSpec{
			Argv: []string{"sh", "-c", `trap "" HUP TERM; echo ready; sleep 300 & wait`}, Mode: ModePipe,
		})
		reader := bufio.NewReader(proc.Reader())
		if line, err := readUntilContaining(reader, "ready"); err != nil {
			t.Fatalf("read readiness = %q error=%v", line, err)
		}
		started := time.Now()
		if err := proc.Kill(SignalHUP); err != nil {
			t.Fatalf("Kill(HUP) error = %v", err)
		}
		if elapsed := time.Since(started); elapsed > killGrace+500*time.Millisecond {
			t.Fatalf("pipe escalation took %s", elapsed)
		}
		exit, err := proc.Wait(context.Background())
		if err != nil || exit.Cause != "signaled" || exit.Signal == nil || *exit.Signal != string(SignalKILL) {
			t.Fatalf("Wait() = %#v error=%v, want SIGKILL", exit, err)
		}
		if err := proc.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})
}

func startTestProc(t *testing.T, spec ProcSpec) Proc {
	t.Helper()
	if spec.Cwd == "" {
		spec.Cwd = t.TempDir()
	}
	proc, err := New().Start(context.Background(), spec)
	if err != nil {
		t.Fatalf("Start(%v) error = %v", spec.Argv, err)
	}
	return proc
}

func stopTestProc(t *testing.T, proc Proc) {
	t.Helper()
	waitCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := proc.Wait(waitCtx)
	if errors.Is(err, context.DeadlineExceeded) {
		killErr := proc.Kill(SignalKILL)
		_, waitErr := proc.Wait(context.Background())
		if killErr != nil || waitErr != nil {
			t.Errorf("stop process kill=%v wait=%v", killErr, waitErr)
		}
	} else if err != nil {
		t.Errorf("Wait() cleanup error = %v", err)
	}
	if err := proc.Close(); err != nil {
		t.Errorf("Close() cleanup error = %v", err)
	}
}

func readUntilContaining(reader *bufio.Reader, want string) (string, error) {
	for {
		line, err := reader.ReadString('\n')
		if strings.Contains(line, want) {
			return line, nil
		}
		if err != nil {
			return line, err
		}
	}
}

func TestPTYWinsizeHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_PTY_WINSIZE_HELPER") != "1" {
		return
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGWINCH)
	if _, err := fmt.Println("ready"); err != nil {
		os.Exit(2)
	}
	<-signals
	winsize, err := unix.IoctlGetWinsize(int(os.Stdin.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		os.Exit(3)
	}
	if _, err := fmt.Printf("%d %d\n", winsize.Row, winsize.Col); err != nil {
		os.Exit(4)
	}
	os.Exit(0)
}
