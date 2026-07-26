package acp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/sandbox"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/toolruntime"
)

func TestLocalLauncherLaunchProvidesWorkingPipes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	launcher := newLocalLauncher(testDiscardLogger(), time.Second)
	handle, err := launcher.Launch(testutil.Context(t), sandbox.LaunchSpec{
		Command: "sh -c 'read line; printf \"%s\\n\" \"$line\"; sleep 0.1'",
		Cwd:     root,
		Env:     os.Environ(),
	})
	if err != nil {
		t.Fatalf("Launch() error = %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if stopErr := handle.Stop(cleanupCtx); stopErr != nil {
			t.Fatalf("handle.Stop() cleanup error = %v", stopErr)
		}
	})
	if handle.PID() <= 0 {
		t.Fatalf("handle.PID() = %d, want positive pid", handle.PID())
	}
	if handle.Cwd() != root {
		t.Fatalf("handle.Cwd() = %q, want %q", handle.Cwd(), root)
	}

	if _, err := handle.Stdin().Write([]byte("hello launcher\n")); err != nil {
		t.Fatalf("handle.Stdin().Write() error = %v", err)
	}
	if err := handle.Stdin().Close(); err != nil {
		t.Fatalf("handle.Stdin().Close() error = %v", err)
	}
	output := make([]byte, len("hello launcher\n"))
	if _, err := io.ReadFull(handle.Stdout(), output); err != nil {
		t.Fatalf("io.ReadFull(stdout) error = %v", err)
	}
	if got := string(output); got != "hello launcher\n" {
		t.Fatalf("stdout = %q, want %q", got, "hello launcher\n")
	}
	if err := handle.Wait(); err != nil {
		t.Fatalf("handle.Wait() error = %v", err)
	}
	select {
	case <-handle.Done():
	case <-time.After(time.Second):
		t.Fatal("handle.Done() did not close after process exit")
	}
}

func TestLocalConstructorsReturnInterfaceImplementations(t *testing.T) {
	t.Parallel()

	if launcher := NewLocalLauncher(nil, 0); launcher == nil {
		t.Fatal("NewLocalLauncher() = nil, want launcher")
	}

	host, err := NewLocalToolHost(context.Background(), t.TempDir(), "", nil)
	if err != nil {
		t.Fatalf("NewLocalToolHost() error = %v", err)
	}
	localHost, ok := host.(*localToolHost)
	if !ok {
		t.Fatalf("NewLocalToolHost() type = %T, want *localToolHost", host)
	}
	if localHost.terminals == nil {
		t.Fatal("NewLocalToolHost() terminals = nil, want terminal manager")
	}
	localHost.Close()
}

func TestLocalLauncherLaunchInvalidCommandReturnsError(t *testing.T) {
	t.Parallel()

	launcher := newLocalLauncher(testDiscardLogger(), time.Second)
	if _, err := launcher.Launch(testutil.Context(t), sandbox.LaunchSpec{
		Command: "definitely-not-an-agh-test-command",
		Cwd:     t.TempDir(),
	}); err == nil {
		t.Fatal("Launch(invalid command) error = nil, want non-nil")
	}
}

func TestLocalLauncherLaunchHonorsCanceledContext(t *testing.T) {
	t.Parallel()

	launcher := newLocalLauncher(testDiscardLogger(), time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := launcher.Launch(ctx, sandbox.LaunchSpec{
		Command: "sh -c 'sleep 1'",
		Cwd:     t.TempDir(),
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Launch(canceled context) error = %v, want context canceled", err)
	}
}

func TestLocalProcessHandleStopTerminatesProcess(t *testing.T) {
	t.Parallel()

	launcher := newLocalLauncher(testDiscardLogger(), 10*time.Millisecond)
	handle, err := launcher.Launch(testutil.Context(t), sandbox.LaunchSpec{
		Command: "sh -c 'while :; do sleep 1; done'",
		Cwd:     t.TempDir(),
		Env:     os.Environ(),
	})
	if err != nil {
		t.Fatalf("Launch(long-running) error = %v", err)
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := handle.Stop(stopCtx); err != nil {
		t.Fatalf("handle.Stop() error = %v", err)
	}
	select {
	case <-handle.Done():
	case <-time.After(time.Second):
		t.Fatal("handle.Done() did not close after Stop")
	}
	if err := handle.Wait(); err != nil {
		t.Fatalf("handle.Wait() after Stop error = %v", err)
	}
}

func TestLocalToolHostReadTextFile(t *testing.T) {
	t.Parallel()

	host, root := newTestLocalToolHost(t, aghconfig.PermissionModeApproveAll)
	target := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(target, []byte("from disk"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	content, err := host.ReadTextFile(testutil.Context(t), "notes.txt")
	if err != nil {
		t.Fatalf("ReadTextFile() error = %v", err)
	}
	if content != "from disk" {
		t.Fatalf("ReadTextFile() = %q, want %q", content, "from disk")
	}

	if _, err := host.ReadTextFile(testutil.Context(t), "missing.txt"); err == nil {
		t.Fatal("ReadTextFile(missing) error = nil, want non-nil")
	}
}

func TestLocalToolHostWriteTextFile(t *testing.T) {
	t.Parallel()

	host, root := newTestLocalToolHost(t, aghconfig.PermissionModeApproveAll)
	if err := host.WriteTextFile(testutil.Context(t), "nested/notes.txt", "saved"); err != nil {
		t.Fatalf("WriteTextFile() error = %v", err)
	}

	target := filepath.Join(root, "nested", "notes.txt")
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", target, err)
	}
	if string(content) != "saved" {
		t.Fatalf("written content = %q, want %q", content, "saved")
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v", target, err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("written mode = %v, want 0600", got)
	}
}

func TestLocalToolHostRejectsNullBytePaths(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		run  func(*testing.T, *localToolHost) error
	}{
		{
			name: "Should reject NUL bytes for read tool paths",
			run: func(t *testing.T, host *localToolHost) error {
				_, err := host.ReadTextFile(testutil.Context(t), "leak\x00.txt")
				return err
			},
		},
		{
			name: "Should reject NUL bytes for write tool paths",
			run: func(t *testing.T, host *localToolHost) error {
				return host.WriteTextFile(testutil.Context(t), "leak\x00.txt", "saved")
			},
		},
		{
			name: "Should reject NUL bytes for direct resolution",
			run: func(_ *testing.T, host *localToolHost) error {
				_, err := host.ResolvePath("leak\x00.txt")
				return err
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			host, root := newTestLocalToolHost(t, aghconfig.PermissionModeApproveAll)
			err := tc.run(t, host)
			if !errors.Is(err, ErrInvalidPath) {
				t.Fatalf("null-byte path error = %v, want ErrInvalidPath", err)
			}
			matches, globErr := filepath.Glob(filepath.Join(root, "leak*"))
			if globErr != nil {
				t.Fatalf("filepath.Glob(leak*) error = %v", globErr)
			}
			if len(matches) != 0 {
				t.Fatalf("leak artifacts = %#v, want none", matches)
			}
		})
	}
}

func TestLocalToolHostResolvePath(t *testing.T) {
	t.Parallel()

	host, root := newTestLocalToolHost(t, aghconfig.PermissionModeApproveAll)
	resolved, err := host.ResolvePath("inside.txt")
	if err != nil {
		t.Fatalf("ResolvePath(relative) error = %v", err)
	}
	if want := filepath.Join(mustCanonicalDir(t, root), "inside.txt"); resolved != want {
		t.Fatalf("ResolvePath(relative) = %q, want %q", resolved, want)
	}

	if _, err := host.ResolvePath(filepath.Join(root, "..", "escape.txt")); !errors.Is(err, ErrPathOutsideWorkspace) {
		t.Fatalf("ResolvePath(outside) error = %v, want ErrPathOutsideWorkspace", err)
	}
}

func TestLocalToolHostAuthorize(t *testing.T) {
	t.Parallel()

	approveAll, _ := newTestLocalToolHost(t, aghconfig.PermissionModeApproveAll)
	for _, op := range []sandbox.PermissionOperation{
		sandbox.PermissionOperationReadTextFile,
		sandbox.PermissionOperationWriteTextFile,
		sandbox.PermissionOperationCreateTerminal,
		sandbox.PermissionOperationRequestToolGrant,
	} {
		if err := approveAll.Authorize(op); err != nil {
			t.Fatalf("Authorize(%s) with approve-all error = %v", op, err)
		}
	}

	denyAll, _ := newTestLocalToolHost(t, aghconfig.PermissionModeDenyAll)
	for _, op := range []sandbox.PermissionOperation{
		sandbox.PermissionOperationReadTextFile,
		sandbox.PermissionOperationWriteTextFile,
		sandbox.PermissionOperationCreateTerminal,
		sandbox.PermissionOperationRequestToolGrant,
	} {
		if err := denyAll.Authorize(op); !errors.Is(err, ErrPermissionDenied) {
			t.Fatalf("Authorize(%s) with deny-all error = %v, want ErrPermissionDenied", op, err)
		}
	}
}

func TestWithLocalAdditionalRootsAccumulatesAcrossOptions(t *testing.T) {
	t.Parallel()

	t.Run("Should accumulate additional roots across repeated options", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		first := t.TempDir()
		second := t.TempDir()
		host, err := newLocalToolHost(
			testutil.Context(t),
			root,
			aghconfig.PermissionModeApproveAll,
			testDiscardLogger(),
			WithLocalAdditionalRoots(first),
			WithLocalAdditionalRoots(second),
		)
		if err != nil {
			t.Fatalf("newLocalToolHost() error = %v", err)
		}
		t.Cleanup(host.Close)

		if got, want := host.permissions.roots, []string{
			mustCanonicalDir(t, root),
			mustCanonicalDir(t, first),
			mustCanonicalDir(t, second),
		}; !slices.Equal(got, want) {
			t.Fatalf("permission roots = %#v, want %#v", got, want)
		}
	})
}

func TestLocalToolHostCreateTerminalUsesResolvedCwd(t *testing.T) {
	t.Parallel()

	host, root := newTestLocalToolHost(t, aghconfig.PermissionModeApproveAll)
	cwd := filepath.Join(root, "work")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", cwd, err)
	}

	response, err := host.CreateTerminal(testutil.Context(t), acpsdk.CreateTerminalRequest{
		SessionId: "sess-terminal",
		Command:   "pwd",
		Cwd:       new(cwd),
	})
	if err != nil {
		t.Fatalf("CreateTerminal() error = %v", err)
	}
	if _, err := host.WaitForTerminalExit(testutil.Context(t), response.TerminalId); err != nil {
		t.Fatalf("WaitForTerminalExit() error = %v", err)
	}
	output, err := host.TerminalOutput(response.TerminalId)
	if err != nil {
		t.Fatalf("TerminalOutput() error = %v", err)
	}
	if got, want := strings.TrimSpace(output), mustCanonicalDir(t, cwd); got != want {
		t.Fatalf("terminal cwd output = %q, want %q", got, want)
	}
}

func TestLocalToolHostCreateTerminalRegistersProcess(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	root := t.TempDir()
	store := toolruntime.NewMemoryStore()
	registry := toolruntime.NewRegistry(store)
	host, err := newLocalToolHost(
		ctx,
		root,
		aghconfig.PermissionModeApproveAll,
		testDiscardLogger(),
		WithLocalProcessRegistry(registry),
	)
	if err != nil {
		t.Fatalf("newLocalToolHost() error = %v", err)
	}
	t.Cleanup(host.Close)

	response, err := host.CreateTerminal(ctx, acpsdk.CreateTerminalRequest{
		SessionId: "sess-terminal",
		Command:   "pwd",
	})
	if err != nil {
		t.Fatalf("CreateTerminal() error = %v", err)
	}
	if _, err := host.WaitForTerminalExit(ctx, response.TerminalId); err != nil {
		t.Fatalf("WaitForTerminalExit() error = %v", err)
	}

	records, err := store.ListProcessRecords(ctx, toolruntime.ProcessQuery{
		Scope: toolruntime.InterruptScope{TerminalID: response.TerminalId},
	})
	if err != nil {
		t.Fatalf("ListProcessRecords() error = %v", err)
	}
	if got, want := len(records), 1; got != want {
		t.Fatalf("records = %d, want %d", got, want)
	}
	record := records[0]
	if record.Source != toolruntime.ProcessSourceACPTerminal {
		t.Fatalf("record.Source = %q, want acp_terminal", record.Source)
	}
	if record.Owner.TerminalID != response.TerminalId {
		t.Fatalf("record.Owner.TerminalID = %q, want %q", record.Owner.TerminalID, response.TerminalId)
	}
	if record.State != toolruntime.ProcessStateCompleted {
		t.Fatalf("record.State = %q, want completed", record.State)
	}
}

func TestLocalToolHostWaitForTerminalExitSignalOnlyFailure(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("POSIX signal behavior")
	}

	t.Run("Should return error for signal-only exit", func(t *testing.T) {
		host, _ := newTestLocalToolHost(t, aghconfig.PermissionModeApproveAll)
		response, err := host.CreateTerminal(testutil.Context(t), acpsdk.CreateTerminalRequest{
			SessionId: "sess-terminal",
			Command:   "/bin/sh",
			Args:      []string{"-c", "kill -TERM $$"},
		})
		if err != nil {
			t.Fatalf("CreateTerminal() error = %v", err)
		}

		exitCode, err := host.WaitForTerminalExit(testutil.Context(t), response.TerminalId)
		if err == nil {
			t.Fatal("WaitForTerminalExit() error = nil, want signal failure")
		}
		if exitCode == 0 {
			t.Fatalf("WaitForTerminalExit() exitCode = %d, want non-zero fallback", exitCode)
		}
		if !strings.Contains(err.Error(), "signal") {
			t.Fatalf("WaitForTerminalExit() error = %v, want signal detail", err)
		}
	})
}

func TestLocalToolHostScopedInterruptStopsOnlyRequestedTerminal(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell process-group interrupt test")
	}

	t.Run("Should stop only the requested terminal", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		root := t.TempDir()
		store := toolruntime.NewMemoryStore()
		registry := toolruntime.NewRegistry(store)
		host, err := newLocalToolHost(
			ctx,
			root,
			aghconfig.PermissionModeApproveAll,
			testDiscardLogger(),
			WithLocalProcessRegistry(registry),
		)
		if err != nil {
			t.Fatalf("newLocalToolHost() error = %v", err)
		}
		t.Cleanup(host.Close)

		first, err := host.CreateTerminal(ctx, acpsdk.CreateTerminalRequest{
			SessionId: "sess-terminal",
			Command:   "/bin/sh",
			Args:      []string{"-c", "sleep 5"},
		})
		if err != nil {
			t.Fatalf("CreateTerminal(first) error = %v", err)
		}
		second, err := host.CreateTerminal(ctx, acpsdk.CreateTerminalRequest{
			SessionId: "sess-terminal",
			Command:   "/bin/sh",
			Args:      []string{"-c", "sleep 5"},
		})
		if err != nil {
			t.Fatalf("CreateTerminal(second) error = %v", err)
		}
		t.Cleanup(func() {
			if err := host.ReleaseTerminal(first.TerminalId); err != nil {
				t.Fatalf("ReleaseTerminal(first) error = %v", err)
			}
		})

		report, err := registry.Interrupt(ctx, toolruntime.InterruptScope{TerminalID: second.TerminalId})
		if err != nil {
			t.Fatalf("Interrupt(second) error = %v", err)
		}
		if report.Matched != 1 || report.Signaled != 1 {
			t.Fatalf("Interrupt(second) = %#v, want one signaled terminal", report)
		}
		waitCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		exitCode, err := host.WaitForTerminalExit(waitCtx, second.TerminalId)
		if err != nil && !strings.Contains(err.Error(), "signal") {
			t.Fatalf("WaitForTerminalExit(second) error = %v, want signal detail", err)
		}
		if err == nil && exitCode == 0 {
			t.Fatalf("WaitForTerminalExit(second) exitCode = %d, want interrupted terminal to stop non-zero", exitCode)
		}

		stillRunningCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()
		if _, err := host.WaitForTerminalExit(
			stillRunningCtx,
			first.TerminalId,
		); !errors.Is(
			err,
			context.DeadlineExceeded,
		) {
			t.Fatalf("WaitForTerminalExit(first) error = %v, want context deadline", err)
		}
	})
}

func TestDriverUsesInjectedLauncherAndToolHostOptions(t *testing.T) {
	t.Parallel()

	launcher := &recordingLauncher{delegate: newLocalLauncher(testDiscardLogger(), time.Second)}
	toolHost, _ := newTestLocalToolHost(t, aghconfig.PermissionModeApproveAll)
	driver := New(WithLauncher(launcher), WithToolHost(toolHost))

	if driver.launcher != launcher {
		t.Fatal("WithLauncher() did not apply")
	}
	if driver.toolHost != toolHost {
		t.Fatal("WithToolHost() did not apply")
	}
}

func TestDriverStartUsesInjectedLauncher(t *testing.T) {
	t.Parallel()

	handle := newFakeHandle(t.TempDir())
	launcher := &recordingLauncher{handle: handle}
	driver := New(WithLogger(testDiscardLogger()), WithLauncher(launcher))
	proc, err := driver.launchAgentProcess(testutil.Context(t), StartOpts{
		AgentName:   "helper",
		Command:     "sh -c 'cat'",
		Cwd:         t.TempDir(),
		Permissions: aghconfig.PermissionModeApproveAll,
	})
	if err != nil {
		t.Fatalf("launchAgentProcess() error = %v", err)
	}
	if err := handle.finish(); err != nil {
		t.Fatalf("finish fake handle: %v", err)
	}
	select {
	case <-proc.Done():
	case <-time.After(time.Second):
		t.Fatal("process Done() did not close for fake handle")
	}

	spec, ok := launcher.lastSpec()
	if !ok {
		t.Fatal("launchAgentProcess() did not call injected launcher")
	}
	if spec.Command != "sh -c 'cat'" {
		t.Fatalf("launcher command = %q, want %q", spec.Command, "sh -c 'cat'")
	}
}

func TestDriverLaunchAgentProcessWrapsLauncherErrors(t *testing.T) {
	t.Parallel()

	launchErr := errors.New("launch failed")
	driver := New(WithLogger(testDiscardLogger()), WithLauncher(&recordingLauncher{err: launchErr}))
	_, err := driver.launchAgentProcess(testutil.Context(t), StartOpts{
		AgentName:   "helper",
		Command:     "sh -c 'cat'",
		Cwd:         t.TempDir(),
		Permissions: aghconfig.PermissionModeApproveAll,
	})
	if err == nil {
		t.Fatal("launchAgentProcess() error = nil, want non-nil")
	}
	if !errors.Is(err, launchErr) {
		t.Fatalf("launchAgentProcess() error = %v, want wrapped launch error", err)
	}
	if !strings.Contains(err.Error(), `helper`) || !strings.Contains(err.Error(), `sh -c 'cat'`) {
		t.Fatalf("launchAgentProcess() error = %v, want agent and command context", err)
	}
}

type recordingLauncher struct {
	delegate sandbox.Launcher
	handle   sandbox.Handle
	err      error

	mu     sync.Mutex
	called bool
	spec   sandbox.LaunchSpec
}

func (l *recordingLauncher) Launch(
	ctx context.Context,
	spec sandbox.LaunchSpec,
) (sandbox.Handle, error) {
	l.mu.Lock()
	l.called = true
	l.spec = spec
	l.mu.Unlock()
	if l.handle != nil {
		return l.handle, nil
	}
	if l.err != nil {
		return nil, l.err
	}
	return l.delegate.Launch(ctx, spec)
}

func (l *recordingLauncher) lastSpec() (sandbox.LaunchSpec, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.spec, l.called
}

type fakeHandle struct {
	cwd          string
	stdoutReader *io.PipeReader
	stdoutWriter *io.PipeWriter
	done         chan struct{}
	finishOnce   sync.Once
}

type noopWriteCloser struct{}

func newFakeHandle(cwd string) *fakeHandle {
	stdoutReader, stdoutWriter := io.Pipe()
	return &fakeHandle{
		cwd:          cwd,
		stdoutReader: stdoutReader,
		stdoutWriter: stdoutWriter,
		done:         make(chan struct{}),
	}
}

func (h *fakeHandle) PID() int {
	return 123
}

func (h *fakeHandle) Cwd() string {
	return h.cwd
}

func (h *fakeHandle) Stdin() io.WriteCloser {
	return noopWriteCloser{}
}

func (h *fakeHandle) Stdout() io.ReadCloser {
	return h.stdoutReader
}

func (h *fakeHandle) Stderr() string {
	return ""
}

func (h *fakeHandle) Done() <-chan struct{} {
	return h.done
}

func (h *fakeHandle) Wait() error {
	<-h.done
	return nil
}

func (h *fakeHandle) Stop(context.Context) error {
	return h.finish()
}

func (h *fakeHandle) finish() error {
	var closeErr error
	h.finishOnce.Do(func() {
		closeErr = h.stdoutWriter.Close()
		close(h.done)
	})
	return closeErr
}

func (noopWriteCloser) Write(p []byte) (int, error) {
	return len(p), nil
}

func (noopWriteCloser) Close() error {
	return nil
}

func newTestLocalToolHost(
	t *testing.T,
	mode aghconfig.PermissionMode,
) (*localToolHost, string) {
	t.Helper()

	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	host, err := newLocalToolHost(ctx, root, mode, testDiscardLogger())
	if err != nil {
		t.Fatalf("newLocalToolHost() error = %v", err)
	}
	t.Cleanup(host.Close)
	return host, root
}

func testDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
