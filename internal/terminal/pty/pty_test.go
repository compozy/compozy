//go:build darwin || linux

package pty

// Suite: Unix terminal substrate.
// Invariant: PTY reads are independently pollable/closable, process groups are isolated, and exits are reported honestly.
// Boundary IN: process specs and terminal control. Boundary OUT: OS PTY, process group, and merged pipe output.

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/charmbracelet/x/xpty"
	"golang.org/x/sys/unix"
)

func TestUnixPTYHardening(t *testing.T) {
	t.Run("Should keep a started interactive process alive after startup context cancellation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		proc, err := New().Start(ctx, ProcSpec{
			Argv: []string{"sh", "-c", "sleep 300"}, Cwd: t.TempDir(),
			Mode: ModePTY, Cols: 80, Rows: 24,
		})
		if err != nil {
			cancel()
			t.Fatalf("Start() error = %v", err)
		}
		defer stopTestProc(t, proc)

		cancel()
		waitCtx, stopWaiting := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer stopWaiting()
		if exit, waitErr := proc.Wait(waitCtx); !errors.Is(waitErr, context.DeadlineExceeded) {
			t.Fatalf("Wait() = %#v error=%v, want live process after startup context cancellation", exit, waitErr)
		}
	})

	t.Run("Should distinguish visible shell editing from hidden input [UT-092]", func(t *testing.T) {
		proc := startTestProc(
			t,
			ProcSpec{Argv: []string{"/bin/sh", "-c", "sleep 300"}, Mode: ModePTY, Cols: 80, Rows: 24},
		)
		defer stopTestProc(t, proc)
		unixProcess := proc.(*unixProc)

		inputVisible, err := unixProcess.InputVisible()
		if err != nil {
			t.Fatalf("InputVisible() error = %v", err)
		}
		if !inputVisible {
			t.Fatal("InputVisible() = false with terminal echo enabled")
		}

		prior, err := unixProcess.readTermios()
		if err != nil {
			t.Fatalf("readTermios() error = %v", err)
		}
		hidden := *prior
		hidden.Lflag &^= unix.ECHO
		if err := unixProcess.writeTermios(&hidden, "disable"); err != nil {
			t.Fatalf("writeTermios(hidden) error = %v", err)
		}
		inputVisible, err = unixProcess.InputVisible()
		if err != nil {
			t.Fatalf("InputVisible() with hidden canonical input error = %v", err)
		}
		if inputVisible {
			t.Fatal("InputVisible() = true with canonical echo disabled")
		}

		lineEditor := hidden
		lineEditor.Lflag &^= unix.ICANON
		if err := unixProcess.writeTermios(&lineEditor, "enter line editor mode"); err != nil {
			t.Fatalf("writeTermios(line editor) error = %v", err)
		}
		inputVisible, err = unixProcess.InputVisible()
		if err != nil {
			t.Fatalf("InputVisible() in line editor mode error = %v", err)
		}
		if !inputVisible {
			t.Fatal("InputVisible() = false for foreground shell line editing")
		}
		if err := unixProcess.restoreTermios(prior); err != nil {
			t.Fatalf("restoreTermios() error = %v", err)
		}
	})

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

	t.Run("Should interrupt a blocked write before closing PTY ownership", func(t *testing.T) {
		proc := startTestProc(t, ProcSpec{
			Argv: []string{"sh", "-c", "sleep 300"}, Mode: ModePTY, Cols: 80, Rows: 24,
		})
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
				t.Fatalf("PTY writer stopped before reaching backpressure: %v", err)
			case <-deadline.C:
				t.Fatal("PTY writer made no initial progress")
			default:
				runtime.Gosched()
			}
		}
		for attempted.Load() == completed.Load() {
			select {
			case err := <-writeDone:
				t.Fatalf("PTY writer stopped before an active write was observed: %v", err)
			case <-deadline.C:
				t.Fatal("PTY writer never exposed an active write")
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
			t.Fatal("Close() did not interrupt the blocked PTY write")
		}
		select {
		case err := <-writeDone:
			if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) {
				t.Fatalf("blocked PTY writer returned a readiness error: %v", err)
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatal("blocked PTY writer retained device ownership after Close()")
		}
		stopTestProc(t, proc)
	})

	t.Run("Should deliver redacted input and restore terminal echo", func(t *testing.T) {
		t.Parallel()

		proc := startTestProc(t, ProcSpec{
			Argv: []string{
				"sh",
				"-c",
				"stty -icanon; printf 'redacted-ready\\n'; head -c 65536 >/dev/null; printf 'redacted-accepted\\n'",
			},
			Mode: ModePTY,
			Cols: 80,
			Rows: 24,
		})
		defer stopTestProc(t, proc)
		unixProcess := proc.(*unixProc)
		reader := bufio.NewReader(unixProcess.reader)
		if line, err := readUntilContaining(reader, "redacted-ready"); err != nil {
			t.Fatalf("read readiness = %q error=%v", line, err)
		}
		secret := bytes.Repeat([]byte("x"), 64<<10)
		result, err := unixProcess.WriteRedacted(secret)
		if err != nil || result.RestoreError != nil || result.BytesDelivered != len(secret) {
			t.Fatalf(
				"WriteRedacted() = %#v error=%v, want %d delivered bytes and restored echo",
				result, err, len(secret),
			)
		}
		visible, err := unixProcess.InputVisible()
		if err != nil {
			t.Fatalf("InputVisible(after redacted write) error = %v", err)
		}
		if !visible {
			t.Fatal("InputVisible(after redacted write) = false, want restored echo")
		}
		line, err := readUntilContaining(reader, "redacted-accepted")
		if err != nil {
			t.Fatalf("read redacted completion = %q error=%v", line, err)
		}
		if bytes.Contains([]byte(line), secret) {
			t.Fatalf("redacted output = %q", line)
		}
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
			t.Fatalf(
				"foreground pgid=%d child pid=%d daemon pgid=%d",
				foregroundGroup,
				unixProcess.PID(),
				syscall.Getpgrp(),
			)
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
			Argv: []string{
				"sh",
				"-c",
				`trap "" HUP TERM; echo ready; sleep 300 & wait`,
			},
			Mode: ModePTY,
			Cols: 80,
			Rows: 24,
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
		if elapsed := time.Since(started); elapsed > processGroupGrace+500*time.Millisecond {
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
			Argv: []string{
				"sh",
				"-c",
				`printf 'a\n'; printf 'b\n' >&2; printf '%s|%s|%s\n' "$TERM" "$COLORTERM" "${KITTY_WINDOW_ID-unset}"`,
			},
			Env:  map[string]string{"KITTY_WINDOW_ID": "secret", "TERM_PROGRAM": "host"},
			Mode: ModePipe,
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
		if err != nil || exit.Cause != "signaled" || exit.Signal == nil || *exit.Signal != string(SignalKILL) ||
			exit.Code != nil {
			t.Fatalf("signaled exit = %#v error=%v", exit, err)
		}
		if closeErr := signaled.Close(); closeErr != nil {
			t.Fatalf("Close(signaled) error = %v", closeErr)
		}

		if unknown := classifyExit(
			errors.New("lost wait status"),
			nil,
		); unknown.Cause != "unknown" ||
			unknown.Code != nil {
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
		if elapsed := time.Since(started); elapsed > processGroupGrace+500*time.Millisecond {
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

func TestShellIntegrationContract(t *testing.T) {
	t.Run("Should leave the shell untouched when integration is disabled [UT-086]", func(t *testing.T) {
		spec := ProcSpec{
			Argv: []string{"/bin/bash"}, Env: map[string]string{"HOME": t.TempDir(), "KEEP": "value"},
			MarkerNonce: "nonce-disabled", ShellIntegration: false,
		}
		setup, err := prepareShellIntegration(spec)
		if err != nil {
			t.Fatalf("prepareShellIntegration() error = %v", err)
		}
		if strings.Join(setup.argv, "\x00") != strings.Join(spec.Argv, "\x00") {
			t.Fatalf("argv = %q, want %q", setup.argv, spec.Argv)
		}
		if setup.env["KEEP"] != "value" || setup.env["ENV"] != "" || setup.env["ZDOTDIR"] != "" {
			t.Fatalf("disabled environment = %#v", setup.env)
		}
		if err := setup.cleanup(); err != nil {
			t.Fatalf("cleanup() error = %v", err)
		}
	})

	t.Run("Should prepare private shims after each user rc without exporting the nonce [UT-087]", func(t *testing.T) {
		tests := []struct {
			name     string
			shell    string
			env      map[string]string
			shimPath func(shellSetup) string
		}{
			{
				name: "bash", shell: "/bin/bash", env: map[string]string{"HOME": t.TempDir()},
				shimPath: func(setup shellSetup) string { return setup.env["ENV"] },
			},
			{
				name: "zsh", shell: "/bin/zsh",
				env:      map[string]string{"HOME": t.TempDir(), "ZDOTDIR": t.TempDir()},
				shimPath: func(setup shellSetup) string { return filepath.Join(setup.env["ZDOTDIR"], ".zshrc") },
			},
			{
				name: "fish", shell: "/opt/homebrew/bin/fish",
				env: map[string]string{
					"HOME":            t.TempDir(),
					"XDG_CONFIG_HOME": t.TempDir(),
					"XDG_DATA_DIRS":   "/prior/share",
				},
				shimPath: func(setup shellSetup) string {
					root, _, _ := strings.Cut(setup.env["XDG_DATA_DIRS"], string(os.PathListSeparator))
					return filepath.Join(root, "fish", "vendor_conf.d", "compozy-terminal.fish")
				},
			},
		}
		for _, test := range tests {
			t.Run("Should inject "+test.name, func(t *testing.T) {
				nonce := "nonce-" + test.name
				setup, err := prepareShellIntegration(ProcSpec{
					Argv: []string{test.shell}, Env: test.env, MarkerNonce: nonce, ShellIntegration: true,
				})
				if err != nil {
					t.Fatalf("prepareShellIntegration(%s) error = %v", test.name, err)
				}
				shimPath := test.shimPath(setup)
				root := shellShimRoot(test.name, setup, shimPath)
				assertPrivatePath(t, root, 0o700)
				assertPrivatePath(t, shimPath, 0o600)
				content, err := os.ReadFile(shimPath)
				if err != nil {
					t.Fatalf("ReadFile(%s) error = %v", shimPath, err)
				}
				if !strings.Contains(string(content), nonce) || !strings.Contains(string(content), "7113;v1") {
					t.Fatalf("%s shim does not contain authenticated marker grammar", test.name)
				}
				for key, value := range setup.env {
					if strings.Contains(value, nonce) {
						t.Fatalf("nonce leaked through environment %s=%q", key, value)
					}
				}
				assertUserRCFirst(t, test.name, setup, string(content), test.env)
				if err := setup.cleanup(); err != nil {
					t.Fatalf("cleanup(%s) error = %v", test.name, err)
				}
				if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("shim root remains after cleanup: %v", err)
				}
			})
		}
	})

	t.Run("Should preserve caller and daemon data dirs for fish vendor markers [UT-120]", func(t *testing.T) {
		tests := []struct {
			name       string
			specDirs   string
			daemonDirs string
			wantBase   string
		}{
			{
				name:       "spec env wins",
				specDirs:   "/prior/a:/prior/b",
				daemonDirs: "/daemon/share",
				wantBase:   "/prior/a:/prior/b",
			},
			{name: "daemon env fallback", specDirs: "", daemonDirs: "/daemon/share", wantBase: "/daemon/share"},
			{name: "xdg spec default", specDirs: "", daemonDirs: "", wantBase: "/usr/local/share:/usr/share"},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Setenv("XDG_DATA_DIRS", test.daemonDirs)
				env := map[string]string{"HOME": t.TempDir()}
				if test.specDirs != "" {
					env["XDG_DATA_DIRS"] = test.specDirs
				}
				setup, err := prepareShellIntegration(ProcSpec{
					Argv: []string{"/usr/bin/fish"}, Env: env, MarkerNonce: "nonce-dirs", ShellIntegration: true,
				})
				if err != nil {
					t.Fatalf("prepareShellIntegration(fish) error = %v", err)
				}
				defer func() {
					if err := setup.cleanup(); err != nil {
						t.Errorf("cleanup() error = %v", err)
					}
				}()
				if key := "XDG_CONFIG_HOME"; setup.env[key] != env[key] {
					t.Fatalf("fish config home overridden: %q", setup.env[key])
				}
				root, rest, found := strings.Cut(setup.env["XDG_DATA_DIRS"], string(os.PathListSeparator))
				if !found || rest != test.wantBase {
					t.Fatalf("data dirs = %q, want shim root + %q", setup.env["XDG_DATA_DIRS"], test.wantBase)
				}
				vendor := filepath.Join(root, "fish", "vendor_conf.d", "compozy-terminal.fish")
				if _, err := os.Stat(vendor); err != nil {
					t.Fatalf("vendor marker missing: %v", err)
				}
			})
		}
	})

	t.Run("Should emit one authenticated start marker for one human bash command [UT-088]", func(t *testing.T) {
		home := t.TempDir()
		if err := os.WriteFile(filepath.Join(home, ".bashrc"), []byte("export USER_RC_LOADED=1\n"), 0o600); err != nil {
			t.Fatalf("write user bashrc error = %v", err)
		}
		proc := startTestProc(t, ProcSpec{
			Argv: []string{"/bin/bash"}, Env: map[string]string{"HOME": home}, MarkerNonce: "nonce-human",
			ShellIntegration: true, Mode: ModePTY, Cols: 80, Rows: 24,
		})
		if _, err := proc.Write([]byte("echo compozy-human-command\nexit\n")); err != nil {
			t.Fatalf("Write(command) error = %v", err)
		}
		output, readErr := io.ReadAll(proc.Reader())
		if readErr != nil {
			t.Fatalf("ReadAll() error = %v", readErr)
		}
		exit, waitErr := proc.Wait(context.Background())
		if waitErr != nil || exit.Code == nil || *exit.Code != 0 {
			t.Fatalf("Wait() = %#v error=%v", exit, waitErr)
		}
		if err := proc.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		marker := ";S;cmd=echo%20compozy-human-command;"
		if count := strings.Count(string(output), marker); count != 1 {
			t.Fatalf("human command marker count = %d, want 1; output=%q", count, output)
		}
		if strings.Contains(string(output), ";S;cmd=__compozy_precmd;") {
			t.Fatalf("prompt lifecycle leaked into command markers: output=%q", output)
		}
		if !strings.Contains(string(output), ";F;exit=0") {
			t.Fatalf("human command did not emit its finish marker: output=%q", output)
		}
	})

	t.Run("Should emit clean start and finish markers for one human zsh command [UT-123]", func(t *testing.T) {
		zshPath, err := exec.LookPath("zsh")
		if err != nil {
			t.Skipf("zsh is not available: %v", err)
		}
		home := t.TempDir()
		proc := startTestProc(t, ProcSpec{
			Argv: []string{zshPath}, Env: map[string]string{"HOME": home}, MarkerNonce: "nonce-zsh",
			ShellIntegration: true, Mode: ModePTY, Cols: 80, Rows: 24,
		})
		if _, err := proc.Write([]byte("echo compozy-zsh-command\nexit\n")); err != nil {
			t.Fatalf("Write(command) error = %v", err)
		}
		output, readErr := io.ReadAll(proc.Reader())
		if readErr != nil {
			t.Fatalf("ReadAll() error = %v", readErr)
		}
		exit, waitErr := proc.Wait(context.Background())
		if waitErr != nil || exit.Code == nil || *exit.Code != 0 {
			t.Fatalf("Wait() = %#v error=%v", exit, waitErr)
		}
		if err := proc.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		if count := strings.Count(string(output), ";S;cmd=echo%20compozy-zsh-command;"); count != 1 {
			t.Fatalf("zsh command marker count = %d, want 1; output=%q", count, output)
		}
		// The finish marker only exists when the precmd hook survives — zsh's
		// read-only `status` parameter must never be shadowed.
		if !strings.Contains(string(output), ";F;exit=0") {
			t.Fatalf("zsh command did not emit its finish marker: output=%q", output)
		}
		if strings.Contains(string(output), "read-only variable") {
			t.Fatalf("zsh hooks errored on a read-only variable: output=%q", output)
		}
	})
}

func shellShimRoot(shell string, setup shellSetup, shimPath string) string {
	switch shell {
	case "zsh":
		return setup.env["ZDOTDIR"]
	case "fish":
		root, _, _ := strings.Cut(setup.env["XDG_DATA_DIRS"], string(os.PathListSeparator))
		return root
	default:
		return filepath.Dir(shimPath)
	}
}

func assertPrivatePath(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%s) error = %v", path, err)
	}
	if got := info.Mode().Perm(); got != mode {
		t.Fatalf("mode(%s) = %o, want %o", path, got, mode)
	}
}

func assertUserRCFirst(t *testing.T, shell string, setup shellSetup, shim string, env map[string]string) {
	t.Helper()
	switch shell {
	case "bash":
		if source, marker := strings.Index(
			shim,
			"; then . ",
		), strings.Index(
			shim,
			"__compozy_nonce",
		); source < 0 ||
			source > marker {
			t.Fatalf("%s user rc is not sourced before integration: %q", shell, shim)
		}
	case "zsh":
		if source, marker := strings.Index(
			shim,
			"source ",
		), strings.Index(
			shim,
			"__compozy_nonce",
		); source < 0 ||
			source > marker {
			t.Fatalf("%s user rc is not sourced before integration: %q", shell, shim)
		}
		envShim, err := os.ReadFile(filepath.Join(setup.env["ZDOTDIR"], ".zshenv"))
		if err != nil {
			t.Fatalf("ReadFile(zsh .zshenv shim) error = %v", err)
		}
		if !strings.Contains(string(envShim), filepath.Join(env["ZDOTDIR"], ".zshenv")) {
			t.Fatalf("zsh shim does not source the user .zshenv: %q", envShim)
		}
	case "fish":
		if setup.env["XDG_CONFIG_HOME"] != env["XDG_CONFIG_HOME"] {
			t.Fatalf(
				"fish config home overridden: %q, want untouched %q",
				setup.env["XDG_CONFIG_HOME"], env["XDG_CONFIG_HOME"],
			)
		}
		root := shellShimRoot(shell, setup, "")
		if _, err := os.Stat(filepath.Join(root, "fish", "config.fish")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("fish shim writes a config.fish that shadows the user config: %v", err)
		}
		suffix := string(os.PathListSeparator) + env["XDG_DATA_DIRS"]
		if !strings.HasSuffix(setup.env["XDG_DATA_DIRS"], suffix) {
			t.Fatalf("fish data dirs dropped the prior value: %q", setup.env["XDG_DATA_DIRS"])
		}
	}
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

func TestPTYWinsizeHelperProcess(_ *testing.T) {
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
