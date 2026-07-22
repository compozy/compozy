package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	acp "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/core/model"
)

func TestClientTerminalActivityCallbacksBracketCommandLifetime(t *testing.T) {
	t.Parallel()

	client, sessionID := newTerminalTestClient(t)
	var active atomic.Int32
	started := make(chan struct{}, 1)
	finished := make(chan struct{}, 1)
	client.terminalActivityStarted = func() {
		active.Add(1)
		started <- struct{}{}
	}
	client.terminalActivityFinished = func() {
		active.Add(-1)
		finished <- struct{}{}
	}

	resp, err := client.CreateTerminal(context.Background(), acp.CreateTerminalRequest{
		SessionId: acp.SessionId(sessionID),
		Command:   os.Args[0],
		Args:      []string{"-test.run=TestTerminalCommandHelperProcess", "--"},
		Env:       terminalHelperEnv("block", "ready", "0"),
	})
	if err != nil {
		t.Fatalf("create terminal: %v", err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("terminal activity did not start")
	}
	if got := active.Load(); got != 1 {
		t.Fatalf("active terminal count = %d, want 1", got)
	}

	if _, err := client.KillTerminal(context.Background(), acp.KillTerminalRequest{
		SessionId:  acp.SessionId(sessionID),
		TerminalId: resp.TerminalId,
	}); err != nil {
		t.Fatalf("kill terminal: %v", err)
	}
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("terminal activity did not finish")
	}
	if got := active.Load(); got != 0 {
		t.Fatalf("active terminal count after exit = %d, want 0", got)
	}
}

func newTerminalTestClientWithRunContext(ctx context.Context, t *testing.T) (*clientImpl, string) {
	t.Helper()
	client, sessionID := newTerminalTestClient(t)
	client.sessions[sessionID].setRunContext(ctx)
	return client, sessionID
}

func TestClientTerminalParentContextCancellationReapsProcessTree(t *testing.T) {
	t.Parallel()
	t.Run("Should reap the process group and unblock wait when the run context is canceled", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("process-group terminal cleanup is implemented differently on Windows")
		}

		runCtx, cancelRun := context.WithCancel(context.Background())
		defer cancelRun()
		client, sessionID := newTerminalTestClientWithRunContext(runCtx, t)

		childPIDPath := filepath.Join(t.TempDir(), "child.pid")
		env := append(
			terminalHelperEnv("spawn-child", "tree-ready", "0"),
			acp.EnvVariable{Name: "GO_TERMINAL_CHILD_PID_FILE", Value: childPIDPath},
		)
		resp, err := client.CreateTerminal(context.Background(), acp.CreateTerminalRequest{
			SessionId: acp.SessionId(sessionID),
			Command:   os.Args[0],
			Args:      []string{"-test.run=TestTerminalCommandHelperProcess", "--"},
			Env:       env,
		})
		if err != nil {
			t.Fatalf("create terminal: %v", err)
		}
		waitForTerminalOutput(t, client, sessionID, resp.TerminalId, "tree-ready")
		childPID := readTerminalChildPID(t, childPIDPath)
		defer killProcessByPID(childPID)

		// Canceling the run/attempt context must propagate to the subprocess.
		cancelRun()

		waitCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if _, err := client.WaitForTerminalExit(waitCtx, acp.WaitForTerminalExitRequest{
			SessionId:  acp.SessionId(sessionID),
			TerminalId: resp.TerminalId,
		}); err != nil {
			t.Fatalf("wait-for-exit did not unblock after run context cancel: %v", err)
		}
		waitForProcessExit(t, childPID)
	})
}

func TestClientTerminalCapExpiryResolvesAsFailure(t *testing.T) {
	t.Parallel()
	t.Run("Should terminate a command that exceeds the cap and report the cap as cause", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("process-group terminal cleanup is implemented differently on Windows")
		}

		client, sessionID := newTerminalTestClient(t)
		client.terminalCommandTimeout = 150 * time.Millisecond
		resp, err := client.CreateTerminal(context.Background(), acp.CreateTerminalRequest{
			SessionId: acp.SessionId(sessionID),
			Command:   os.Args[0],
			Args:      []string{"-test.run=TestTerminalCommandHelperProcess", "--"},
			Env:       terminalHelperEnv("block", "ready", "0"),
		})
		if err != nil {
			t.Fatalf("create terminal: %v", err)
		}

		waitCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		waitResp, err := client.WaitForTerminalExit(waitCtx, acp.WaitForTerminalExitRequest{
			SessionId:  acp.SessionId(sessionID),
			TerminalId: resp.TerminalId,
		})
		if err != nil {
			t.Fatalf("wait-for-exit hung past the cap: %v", err)
		}
		if waitResp.ExitCode != nil {
			t.Fatalf("expected no exit code for a capped command, got %d", *waitResp.ExitCode)
		}
		if waitResp.Signal == nil || !strings.Contains(*waitResp.Signal, "wall-clock cap") {
			t.Fatalf("expected wall-clock cap cause, got signal %#v", waitResp.Signal)
		}
	})
}

func TestClientTerminalCompletesNormallyWithinCap(t *testing.T) {
	t.Parallel()
	t.Run("Should return the real exit code when the command finishes within the cap", func(t *testing.T) {
		t.Parallel()

		client, sessionID := newTerminalTestClient(t)
		client.terminalCommandTimeout = 30 * time.Second
		resp, err := client.CreateTerminal(context.Background(), acp.CreateTerminalRequest{
			SessionId: acp.SessionId(sessionID),
			Command:   os.Args[0],
			Args:      []string{"-test.run=TestTerminalCommandHelperProcess", "--"},
			Env:       terminalHelperEnv("print-exit", "done", "7"),
		})
		if err != nil {
			t.Fatalf("create terminal: %v", err)
		}

		waitCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		waitResp, err := client.WaitForTerminalExit(waitCtx, acp.WaitForTerminalExitRequest{
			SessionId:  acp.SessionId(sessionID),
			TerminalId: resp.TerminalId,
		})
		if err != nil {
			t.Fatalf("wait for terminal: %v", err)
		}
		if waitResp.ExitCode == nil || *waitResp.ExitCode != 7 {
			t.Fatalf("terminal exit code = %#v, want 7", waitResp.ExitCode)
		}
		if waitResp.Signal != nil {
			t.Fatalf("expected no signal for a normal exit, got %q", *waitResp.Signal)
		}
	})
}

func TestResolveTerminalCapFallsBackToStallDefault(t *testing.T) {
	t.Parallel()

	t.Run("Should fall back to the stall default and honor a configured cap", func(t *testing.T) {
		client := &clientImpl{}
		if got := client.resolveTerminalCap(); got != model.DefaultStallTerminalCap {
			t.Fatalf("resolveTerminalCap() = %s, want default %s", got, model.DefaultStallTerminalCap)
		}
		client.terminalCommandTimeout = 5 * time.Minute
		if got := client.resolveTerminalCap(); got != 5*time.Minute {
			t.Fatalf("resolveTerminalCap() = %s, want 5m", got)
		}
	})
}

func TestTerminalBaseContextPrefersSessionRunContext(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should prefer the session run context and fall back to the request or background context",
		func(t *testing.T) {
			dir := t.TempDir()
			session := newSessionWithAccess("sess", dir, []string{dir})
			type ctxKey string
			const key ctxKey = "k"
			runCtx := context.WithValue(context.Background(), key, "run")
			session.setRunContext(runCtx)
			if got := terminalBaseContext(context.Background(), session); got.Value(key) != "run" {
				t.Fatalf("terminalBaseContext did not prefer the session run context")
			}

			bare := newSessionWithAccess("sess2", dir, []string{dir})
			fallback := context.WithValue(context.Background(), key, "fallback")
			if got := terminalBaseContext(fallback, bare); got.Value(key) != "fallback" {
				t.Fatalf("terminalBaseContext did not fall back to the request context")
			}
			var nilCtx context.Context
			if got := terminalBaseContext(nilCtx, nil); got == nil {
				t.Fatalf("terminalBaseContext with no contexts = nil, want background context")
			}
		},
	)
}

func TestRunWithDeadlineReturnsStructuredFailureWhenBlocked(t *testing.T) {
	t.Parallel()

	t.Run("Should return a structured failure when the operation blocks past the deadline", func(t *testing.T) {
		t.Parallel()
		release := make(chan struct{})
		t.Cleanup(func() { close(release) })
		_, err := runWithDeadline(
			context.Background(),
			50*time.Millisecond,
			"blocking op",
			func() (int, error) {
				<-release
				return 1, nil
			},
		)
		if err == nil {
			t.Fatal("expected a deadline failure, got nil error")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected context.DeadlineExceeded, got %v", err)
		}
		if !strings.Contains(err.Error(), "blocking op deadline exceeded") {
			t.Fatalf("expected structured deadline message, got %q", err.Error())
		}
	})

	t.Run("Should return the result when the operation finishes within the deadline", func(t *testing.T) {
		t.Parallel()
		got, err := runWithDeadline(
			context.Background(),
			time.Second,
			"fast op",
			func() (int, error) { return 42, nil },
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 42 {
			t.Fatalf("runWithDeadline result = %d, want 42", got)
		}
	})

	t.Run("Should run inline without a deadline when the timeout is not positive", func(t *testing.T) {
		t.Parallel()
		got, err := runWithDeadline(
			context.Background(),
			0,
			"inline op",
			func() (string, error) { return "ok", nil },
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "ok" {
			t.Fatalf("runWithDeadline result = %q, want ok", got)
		}
	})
}

func TestClientReadTextFileHonorsHandlerDeadline(t *testing.T) {
	t.Parallel()

	t.Run("Should return a structured failure when a filesystem read blocks past the deadline", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("named pipes behave differently on Windows")
		}

		dir := t.TempDir()
		fifoPath := filepath.Join(dir, "blocking.fifo")
		if err := syscall.Mkfifo(fifoPath, 0o600); err != nil {
			t.Fatalf("mkfifo: %v", err)
		}
		// Unblock the leaked reader goroutine when the test finishes by opening
		// the write end so the reader's os.ReadFile returns.
		t.Cleanup(func() {
			writer, err := os.OpenFile(fifoPath, os.O_WRONLY|syscall.O_NONBLOCK, 0)
			if err == nil {
				_ = writer.Close()
			}
		})

		client := &clientImpl{
			handlerDeadline: 150 * time.Millisecond,
			sessions: map[string]*sessionImpl{
				"sess-fifo": newSessionWithAccess("sess-fifo", dir, []string{dir}),
			},
		}
		_, err := client.ReadTextFile(context.Background(), acp.ReadTextFileRequest{
			SessionId: "sess-fifo",
			Path:      fifoPath,
		})
		if err == nil {
			t.Fatal("expected a deadline failure reading a blocking fifo, got nil error")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected context.DeadlineExceeded, got %v", err)
		}
		if !strings.Contains(err.Error(), "read text file deadline exceeded") {
			t.Fatalf("expected structured read deadline message, got %q", err.Error())
		}
	})
}

func TestClientRequestPermissionReturnsWithinDeadline(t *testing.T) {
	t.Parallel()

	t.Run("Should select the offered option and cancel within the deadline when none is offered", func(t *testing.T) {
		client := &clientImpl{handlerDeadline: time.Second}
		resp, err := client.RequestPermission(context.Background(), acp.RequestPermissionRequest{
			Options: []acp.PermissionOption{{OptionId: "allow"}},
		})
		if err != nil {
			t.Fatalf("request permission: %v", err)
		}
		if resp.Outcome.Selected == nil || resp.Outcome.Selected.OptionId != "allow" {
			t.Fatalf("unexpected permission selection: %#v", resp.Outcome)
		}

		empty, err := client.RequestPermission(context.Background(), acp.RequestPermissionRequest{})
		if err != nil {
			t.Fatalf("request permission without options: %v", err)
		}
		// Concatenate the variant name so misspell does not rewrite the acp field.
		if !outcomeHasVariant(empty.Outcome, "Cancel"+"led") {
			t.Fatalf("expected canceled outcome, got %#v", empty.Outcome)
		}
	})
}
