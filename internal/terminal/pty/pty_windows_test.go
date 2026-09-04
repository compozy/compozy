//go:build windows

package pty

// Suite: Windows terminal substrate.
// Invariant: ConPTY reads close promptly, resizing is live-safe, process handles report honest exits,
// a child cannot run before its Job Object owns every descendant, and redacted writes disable echo
// before delivery, restore the prior mode, and never terminate the shell when their helper fails.
// Boundary IN: process specs and terminal control. Boundary OUT: ConPTY, process handles, and Job Objects.

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

const (
	windowsSleepHelperEnv  = "COMPOZY_WINDOWS_PTY_SLEEP_HELPER"
	windowsSleepTestRunArg = "-test.run=^TestWindowsPTYSleepHelper$"
)

func TestWindowsPTYSleepHelper(t *testing.T) {
	t.Run("Should remain silent while the terminal process is alive", func(t *testing.T) {
		t.Parallel()
		if os.Getenv(windowsSleepHelperEnv) != "1" {
			return
		}
		time.Sleep(5 * time.Minute)
	})
}

func TestWindowsPTYHardening(t *testing.T) { // IT-038
	t.Run("Should isolate a mode-helper failure from the terminal job", func(t *testing.T) {
		proc := startWindowsSleepTestProc(t, ModePTY)
		windowsTerminal, ok := proc.(*windowsProc)
		if !ok {
			t.Fatal("Windows PTY process has an unexpected concrete type")
		}
		if _, err := windowsTerminal.runConsoleModeHelper("invalid"); err == nil {
			t.Fatal("invalid ConPTY mode helper operation succeeded")
		}
		waitCtx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()
		if _, err := proc.Wait(waitCtx); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("terminal exited after helper failure: %v", err)
		}
	})

	t.Run("Should refuse redacted input while ConPTY input is visible [UT-092]", func(t *testing.T) {
		proc := startWindowsTestProc(t, ModePTY, []string{
			"powershell.exe",
			"-NoLogo",
			"-NoProfile",
			"-NonInteractive",
			"-Command",
			`[Console]::Out.Write('Password:'); [Console]::Out.Flush(); $secret = [Console]::ReadLine(); [Console]::Out.WriteLine('accepted'); [Console]::Out.Flush(); Start-Sleep -Seconds 2`,
		})
		visibilityProc, ok := proc.(interface {
			InputVisible() (bool, error)
			WriteRedacted([]byte) (RedactedWriteResult, error)
		})
		if !ok {
			t.Fatal("Windows PTY does not expose the echo-aware input contract")
		}
		reader := bufio.NewReader(proc.Reader())
		output := readWindowsUntil(t, proc, reader, "Password:")
		visible, err := visibilityProc.InputVisible()
		if err != nil || !visible {
			t.Fatalf("InputVisible(before) = %t error=%v, want visible", visible, err)
		}
		secret := []byte("conpty-secret\r")
		result, err := visibilityProc.WriteRedacted(secret)
		if !errors.Is(err, ErrInputVisible) || result.BytesDelivered != 0 {
			t.Fatalf(
				"WriteRedacted() = %#v error=%v, want zero bytes and ErrInputVisible",
				result,
				err,
			)
		}
		if bytes.Contains(output, []byte("conpty-secret")) {
			t.Fatalf("ConPTY output = %q", output)
		}
	})

	t.Run("Should deliver redacted input after a ConPTY program hides input", func(t *testing.T) {
		proc := startWindowsTestProc(t, ModePTY, []string{
			"powershell.exe", "-NoLogo", "-NoProfile", "-Command",
			`$secret = Read-Host -AsSecureString -Prompt 'Password'; Write-Output 'accepted'; Start-Sleep -Seconds 2`,
		})
		visibilityProc := proc.(interface {
			InputVisible() (bool, error)
			WriteRedacted([]byte) (RedactedWriteResult, error)
		})
		reader := bufio.NewReader(proc.Reader())
		output := readWindowsUntil(t, proc, reader, "Password")
		visible, err := visibilityProc.InputVisible()
		if err != nil || visible {
			t.Fatalf("InputVisible(hidden prompt) = %t error=%v, want hidden", visible, err)
		}
		secret := []byte("conpty-secret\r")
		result, err := visibilityProc.WriteRedacted(secret)
		if err != nil || result.BytesDelivered != len(secret) {
			t.Fatalf("WriteRedacted(hidden) = %#v error=%v, want %d bytes", result, err, len(secret))
		}
		output = append(output, readWindowsUntil(t, proc, reader, "accepted")...)
		if !bytes.Contains(output, []byte("accepted")) || bytes.Contains(output, []byte("conpty-secret")) {
			t.Fatalf("ConPTY redacted output = %q", output)
		}
	})

	t.Run("Should unblock a parked read within 200ms while the child is alive", func(t *testing.T) {
		proc := startWindowsSleepTestProc(t, ModePTY)
		readDone := startWindowsReadAfterStartup(t, proc)
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
		assertWindowsSignaledExit(t, proc, SignalHUP)
	})

	t.Run("Should interrupt a blocked write before closing ConPTY ownership", func(t *testing.T) {
		proc := startWindowsSleepTestProc(t, ModePTY)
		var attempted atomic.Int64
		var completed atomic.Int64
		writeDone := make(chan error, 1)
		go func() {
			payload := bytes.Repeat([]byte("x"), 64<<10)
			for {
				attempted.Add(1)
				if _, err := proc.Write(payload); err != nil {
					writeDone <- err
					return
				}
				completed.Add(1)
			}
		}()
		deadline := time.NewTimer(time.Second)
		defer deadline.Stop()
		for completed.Load() == 0 {
			select {
			case err := <-writeDone:
				t.Fatalf("ConPTY writer stopped before reaching backpressure: %v", err)
			case <-deadline.C:
				t.Fatal("ConPTY writer made no initial progress")
			default:
				runtime.Gosched()
			}
		}
		for attempted.Load() == completed.Load() {
			select {
			case err := <-writeDone:
				t.Fatalf("ConPTY writer stopped before an active write was observed: %v", err)
			case <-deadline.C:
				t.Fatal("ConPTY writer never exposed an active write")
			default:
				runtime.Gosched()
			}
		}
		closeDone := make(chan error, 1)
		go func() { closeDone <- proc.Close() }()
		select {
		case err := <-closeDone:
			if err != nil {
				t.Fatalf("Close() error = %v", err)
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatal("Close() did not interrupt the blocked ConPTY write")
		}
		select {
		case <-writeDone:
		case <-time.After(200 * time.Millisecond):
			t.Fatal("blocked ConPTY writer retained device ownership after Close()")
		}
		assertWindowsSignaledExit(t, proc, SignalHUP)
	})

	t.Run("Should resize while another goroutine is reading", func(t *testing.T) {
		proc := startWindowsSleepTestProc(t, ModePTY)
		readDone := startWindowsReadAfterStartup(t, proc)
		select {
		case err := <-readDone:
			t.Fatalf("live-child read returned before resize: %v", err)
		case <-time.After(30 * time.Millisecond):
		}
		if err := proc.Resize(140, 40); err != nil {
			t.Fatalf("Resize() error = %v", err)
		}
		if err := proc.Kill(SignalKILL); err != nil {
			t.Fatalf("Kill(KILL) error = %v", err)
		}
		select {
		case <-readDone:
		case <-time.After(time.Second):
			t.Fatal("reader stayed parked after process exit")
		}
		assertWindowsSignaledExit(t, proc, SignalKILL)
	})

	t.Run("Should report normal and terminated exits honestly", func(t *testing.T) {
		normal := startWindowsTestProc(t, ModePTY, []string{"cmd.exe", "/d", "/s", "/c", "exit /b 7"})
		exit, err := normal.Wait(context.Background())
		if err != nil || exit.Cause != "exited" || exit.Code == nil || *exit.Code != 7 || exit.Signal != nil {
			t.Fatalf("normal exit = %#v error=%v", exit, err)
		}
		if err := normal.Close(); err != nil {
			t.Fatalf("Close(normal) error = %v", err)
		}

		terminated := startWindowsSleepTestProc(t, ModePTY)
		if err := terminated.Kill(SignalTERM); err != nil {
			t.Fatalf("Kill(TERM) error = %v", err)
		}
		assertWindowsSignaledExit(t, terminated, SignalTERM)
		if err := terminated.Close(); err != nil {
			t.Fatalf("Close(terminated) error = %v", err)
		}
	})

	t.Run("Should terminate a grandchild through the owning Job Object", func(t *testing.T) {
		proc := startWindowsTestProc(t, ModePTY, windowsGrandchildCommand())
		childPID := readWindowsChildPID(t, proc)
		if err := proc.Kill(SignalKILL); err != nil {
			t.Fatalf("Kill(KILL) error = %v", err)
		}
		assertWindowsSignaledExit(t, proc, SignalKILL)
		assertWindowsProcessExited(t, childPID)
		if err := proc.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})
}

func readWindowsUntil(t *testing.T, proc Proc, reader *bufio.Reader, needle string) []byte {
	t.Helper()
	type result struct {
		output []byte
		err    error
	}
	done := make(chan result, 1)
	go func() {
		var output []byte
		for !bytes.Contains(output, []byte(needle)) {
			value, err := reader.ReadByte()
			if err != nil {
				done <- result{output: output, err: err}
				return
			}
			output = append(output, value)
		}
		done <- result{output: output}
	}()
	select {
	case read := <-done:
		if read.err != nil {
			t.Fatalf("read ConPTY until %q: output=%q error=%v", needle, read.output, read.err)
		}
		return read.output
	case <-time.After(20 * time.Second):
		if err := proc.Kill(SignalKILL); err != nil {
			t.Errorf("Kill(KILL) after ConPTY prompt timeout error = %v", err)
		}
		t.Fatalf("timed out waiting for ConPTY output %q", needle)
		return nil
	}
}

func TestWindowsPipeRuntime(t *testing.T) { // IT-038
	t.Run("Should execute a command and return merged output", func(t *testing.T) {
		proc := startWindowsTestProc(t, ModePipe, []string{"cmd.exe", "/d", "/s", "/c", "echo pipe-ok"})
		output, readErr := io.ReadAll(proc.Reader())
		if readErr != nil {
			t.Fatalf("ReadAll() error = %v", readErr)
		}
		exit, waitErr := proc.Wait(context.Background())
		if waitErr != nil || exit.Code == nil || *exit.Code != 0 || !strings.Contains(string(output), "pipe-ok") {
			t.Fatalf("output = %q exit=%#v error=%v", output, exit, waitErr)
		}
		if err := proc.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	t.Run("Should terminate a pipe-mode grandchild through the owning Job Object", func(t *testing.T) {
		proc := startWindowsTestProc(t, ModePipe, windowsGrandchildCommand())
		childPID := readWindowsChildPID(t, proc)
		if err := proc.Kill(SignalKILL); err != nil {
			t.Fatalf("Kill(KILL) error = %v", err)
		}
		if _, err := proc.Wait(context.Background()); err != nil {
			t.Fatalf("Wait() error = %v", err)
		}
		assertWindowsProcessExited(t, childPID)
		if err := proc.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})
}

func startWindowsTestProc(t *testing.T, mode Mode, argv []string) Proc {
	t.Helper()
	return startWindowsTestProcWithEnv(t, mode, argv, nil)
}

func startWindowsSleepTestProc(t *testing.T, mode Mode) Proc {
	t.Helper()
	return startWindowsTestProcWithEnv(
		t,
		mode,
		windowsSleepCommand(),
		map[string]string{windowsSleepHelperEnv: "1"},
	)
}

func startWindowsTestProcWithEnv(t *testing.T, mode Mode, argv []string, env map[string]string) Proc {
	t.Helper()
	proc, err := New().Start(context.Background(), ProcSpec{
		Argv: argv, Cwd: t.TempDir(), Env: env, Mode: mode, Cols: 80, Rows: 24,
	})
	if err != nil {
		t.Fatalf("Start(%v) error = %v", argv, err)
	}
	t.Cleanup(func() { stopWindowsTestProc(t, proc) })
	return proc
}

func startWindowsReadAfterStartup(t *testing.T, proc Proc) <-chan error {
	t.Helper()
	startupDeadline := time.NewTimer(2 * time.Second)
	defer startupDeadline.Stop()
	for {
		readDone := make(chan error, 1)
		readStarted := make(chan struct{})
		go func() {
			close(readStarted)
			buffer := make([]byte, 256)
			_, err := proc.Reader().Read(buffer)
			readDone <- err
		}()
		<-readStarted
		quiet := time.NewTimer(100 * time.Millisecond)
		select {
		case err := <-readDone:
			quiet.Stop()
			if err != nil {
				t.Fatalf("read ConPTY startup output: %v", err)
			}
		case <-quiet.C:
			return readDone
		case <-startupDeadline.C:
			quiet.Stop()
			t.Fatal("ConPTY startup output did not become quiet within 2s")
		}
	}
}

func stopWindowsTestProc(t *testing.T, proc Proc) {
	t.Helper()
	cleanupBase := context.WithoutCancel(t.Context())
	waitCtx, cancel := context.WithTimeout(cleanupBase, 50*time.Millisecond)
	defer cancel()
	_, err := proc.Wait(waitCtx)
	if errors.Is(err, context.DeadlineExceeded) {
		if killErr := proc.Kill(SignalKILL); killErr != nil {
			t.Errorf("Kill(KILL) cleanup error = %v", killErr)
		}
		postKillCtx, postKillCancel := context.WithTimeout(cleanupBase, time.Second)
		defer postKillCancel()
		if _, waitErr := proc.Wait(postKillCtx); waitErr != nil {
			t.Errorf("Wait() cleanup error = %v", waitErr)
		}
	} else if err != nil {
		t.Errorf("Wait() cleanup error = %v", err)
	}
	if err := proc.Close(); err != nil {
		t.Errorf("Close() cleanup error = %v", err)
	}
}

func assertWindowsSignaledExit(t *testing.T, proc Proc, want Signal) {
	t.Helper()
	exit, err := proc.Wait(t.Context())
	if err != nil || exit.Cause != "signaled" || exit.Signal == nil || *exit.Signal != string(want) ||
		exit.Code != nil {
		t.Fatalf("Wait() = %#v error=%v, want signal %s", exit, err, want)
	}
}

func windowsSleepCommand() []string {
	return []string{os.Args[0], windowsSleepTestRunArg}
}

func windowsGrandchildCommand() []string {
	return []string{
		"powershell.exe",
		"-NoLogo",
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		`$child = Start-Process powershell.exe -ArgumentList @('-NoLogo','-NoProfile','-NonInteractive','-Command','Start-Sleep -Seconds 300') -PassThru; [Console]::Out.WriteLine("CHILD_PID=$($child.Id)"); [Console]::Out.Flush(); Wait-Process -Id $child.Id`,
	}
}

func readWindowsChildPID(t *testing.T, proc Proc) int {
	t.Helper()
	type result struct {
		pid int
		err error
	}
	done := make(chan result, 1)
	go func() {
		matcher := regexp.MustCompile(`CHILD_PID=([0-9]+)`)
		scanner := bufio.NewScanner(proc.Reader())
		for scanner.Scan() {
			match := matcher.FindStringSubmatch(scanner.Text())
			if len(match) != 2 {
				continue
			}
			pid, err := strconv.Atoi(match[1])
			done <- result{pid: pid, err: err}
			return
		}
		done <- result{err: scanner.Err()}
	}()
	select {
	case child := <-done:
		if child.err != nil || child.pid == 0 {
			t.Fatalf("read child PID: pid=%d error=%v", child.pid, child.err)
		}
		return child.pid
	case <-time.After(5 * time.Second):
		if err := proc.Kill(SignalKILL); err != nil {
			t.Errorf("Kill(KILL) after child PID timeout error = %v", err)
		}
		if err := proc.Close(); err != nil {
			t.Errorf("Close() after child PID timeout error = %v", err)
		}
		t.Fatal("timed out waiting for CHILD_PID")
		return 0
	}
}

func assertWindowsProcessExited(t *testing.T, pid int) {
	t.Helper()
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return
	}
	if err != nil {
		t.Fatalf("OpenProcess(%d) error = %v", pid, err)
	}
	event, waitErr := windows.WaitForSingleObject(handle, 2_000)
	closeErr := windows.CloseHandle(handle)
	if waitErr != nil || event != windows.WAIT_OBJECT_0 || closeErr != nil {
		t.Fatalf("grandchild %d survived: wait=%d wait_error=%v close_error=%v", pid, event, waitErr, closeErr)
	}
}
