//go:build windows

package pty

// Suite: Windows terminal substrate.
// Invariant: ConPTY reads close promptly, resizing is live-safe, process handles report honest exits,
// and a child cannot run before its Job Object owns every descendant.
// Boundary IN: process specs and terminal control. Boundary OUT: ConPTY, process handles, and Job Objects.

import (
	"bufio"
	"context"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestWindowsPTYHardening(t *testing.T) { // IT-038
	t.Run("Should unblock a parked read within 200ms while the child is alive", func(t *testing.T) {
		proc := startWindowsTestProc(t, ModePTY, windowsSleepCommand())
		readDone := make(chan error, 1)
		readStarted := make(chan struct{})
		go func() {
			close(readStarted)
			buffer := make([]byte, 1)
			_, err := proc.Reader().Read(buffer)
			readDone <- err
		}()
		<-readStarted
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

	t.Run("Should resize while another goroutine is reading", func(t *testing.T) {
		proc := startWindowsTestProc(t, ModePTY, windowsSleepCommand())
		readDone := make(chan error, 1)
		readStarted := make(chan struct{})
		go func() {
			close(readStarted)
			buffer := make([]byte, 1)
			_, err := proc.Reader().Read(buffer)
			readDone <- err
		}()
		<-readStarted
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

		terminated := startWindowsTestProc(t, ModePTY, windowsSleepCommand())
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
	proc, err := New().Start(context.Background(), ProcSpec{
		Argv: argv, Cwd: t.TempDir(), Mode: mode, Cols: 80, Rows: 24,
	})
	if err != nil {
		t.Fatalf("Start(%v) error = %v", argv, err)
	}
	t.Cleanup(func() { stopWindowsTestProc(t, proc) })
	return proc
}

func stopWindowsTestProc(t *testing.T, proc Proc) {
	t.Helper()
	waitCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := proc.Wait(waitCtx)
	if errors.Is(err, context.DeadlineExceeded) {
		if killErr := proc.Kill(SignalKILL); killErr != nil {
			t.Errorf("Kill(KILL) cleanup error = %v", killErr)
		}
		postKillCtx, postKillCancel := context.WithTimeout(context.Background(), time.Second)
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
	exit, err := proc.Wait(context.Background())
	if err != nil || exit.Cause != "signaled" || exit.Signal == nil || *exit.Signal != string(want) || exit.Code != nil {
		t.Fatalf("Wait() = %#v error=%v, want signal %s", exit, err, want)
	}
}

func windowsSleepCommand() []string {
	return []string{"powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", "Start-Sleep -Seconds 300"}
}

func windowsGrandchildCommand() []string {
	return []string{
		"powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command",
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
