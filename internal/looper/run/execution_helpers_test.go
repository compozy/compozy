package run

import (
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/looper/internal/looper/model"
)

func TestHandleLegacyCommandCancellationReturnsCanceledResult(t *testing.T) {
	cmd, cmdDone := startBlockingExecutionHelper(t)

	result := handleLegacyCommandCancellation(cmd, cmdDone, &job{
		codeFiles: []string{"legacy_cancel.go"},
		outLog:    "cancel.out",
		errLog:    "cancel.err",
	})
	if result.status != attemptStatusCanceled {
		t.Fatalf("status = %q, want %q", result.status, attemptStatusCanceled)
	}
	if result.exitCode != exitCodeCanceled {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, exitCodeCanceled)
	}
}

func TestHandleLegacyActivityTimeoutReturnsTimeoutResult(t *testing.T) {
	cmd, cmdDone := startBlockingExecutionHelper(t)

	result := handleLegacyActivityTimeout(cmd, cmdDone, &job{
		codeFiles: []string{"legacy_timeout.go"},
		outLog:    "timeout.out",
		errLog:    "timeout.err",
	}, 25*time.Millisecond)
	if result.status != attemptStatusTimeout {
		t.Fatalf("status = %q, want %q", result.status, attemptStatusTimeout)
	}
	if result.exitCode != exitCodeTimeout {
		t.Fatalf("exitCode = %d, want %d", result.exitCode, exitCodeTimeout)
	}
}

func TestHandleInteractiveCancellationClosesTerminalRuntime(t *testing.T) {
	execCtx := &jobExecutionContext{
		terminalRuntimes: make([]*terminalRuntime, 1),
	}
	term := newStartedTerminal(t, "hold")
	execCtx.terminalRuntimes[0] = &terminalRuntime{terminal: term}

	result := handleInteractiveCancellation(execCtx, 0, &job{
		codeFiles: []string{"interactive_cancel.go"},
		outLog:    "interactive.out",
		errLog:    "interactive.err",
	}, term, context.Canceled)
	if result.status != attemptStatusCanceled {
		t.Fatalf("status = %q, want %q", result.status, attemptStatusCanceled)
	}
	waitForAliveState(t, term, false)
}

func TestRecordFailureWithContextAppendsFailure(t *testing.T) {
	var failures []failInfo

	failure := recordFailureWithContext(nil, &job{
		codeFiles: []string{"failure.go"},
		outLog:    "failure.out",
		errLog:    "failure.err",
	}, &failures, io.EOF, 9)
	if len(failures) != 1 {
		t.Fatalf("len(failures) = %d, want 1", len(failures))
	}
	if failure.exitCode != 9 || failure.codeFile != "failure.go" {
		t.Fatalf("failure = %#v, want exitCode=9 codeFile=failure.go", failure)
	}
}

func TestExecutorControllerAwaitShutdownAfterCancel(t *testing.T) {
	done := make(chan struct{})
	execCtx := &jobExecutionContext{total: 1}
	atomic.StoreInt32(&execCtx.failed, 1)

	controller := &executorController{
		ctx:     context.Background(),
		execCtx: execCtx,
		done:    done,
	}

	go func() {
		time.Sleep(20 * time.Millisecond)
		close(done)
	}()

	failed, _, total, err := controller.awaitShutdownAfterCancel()
	if err != nil {
		t.Fatalf("awaitShutdownAfterCancel() error = %v", err)
	}
	if failed != 1 || total != 1 {
		t.Fatalf("result = failed:%d total:%d, want failed:1 total:1", failed, total)
	}
}

func TestNotifyJobStartWritesUIEventAndCLIMessage(t *testing.T) {
	t.Run("ui event", func(t *testing.T) {
		uiCh := make(chan uiMsg, 1)
		notifyJobStart(true, uiCh, 3, &job{}, &config{})

		msg := <-uiCh
		started, ok := msg.(jobStartedMsg)
		if !ok {
			t.Fatalf("message type = %T, want jobStartedMsg", msg)
		}
		if started.Index != 3 {
			t.Fatalf("Index = %d, want 3", started.Index)
		}
	})

	t.Run("cli output", func(t *testing.T) {
		previous := os.Stdout
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe() error = %v", err)
		}
		os.Stdout = w
		defer func() {
			os.Stdout = previous
		}()

		notifyJobStart(false, nil, 0, &job{
			codeFiles: []string{"file.go"},
			groups: map[string][]model.IssueEntry{
				"file.go": {{CodeFile: "file.go"}, {CodeFile: "file.go"}},
			},
		}, &config{
			ide:             model.IDEClaude,
			mode:            model.ExecutionModePRDTasks,
			signalPort:      4321,
			reasoningEffort: "high",
		})

		_ = w.Close()
		body, readErr := io.ReadAll(r)
		if readErr != nil {
			t.Fatalf("ReadAll() error = %v", readErr)
		}
		if !strings.Contains(string(body), "Running Claude") {
			t.Fatalf("stdout = %q, want command preview", string(body))
		}
	})
}

func startBlockingExecutionHelper(t *testing.T) (*exec.Cmd, <-chan error) {
	t.Helper()

	cmd := exec.CommandContext(
		context.Background(),
		os.Args[0],
		"-test.run=^TestExecutionHelperProcess$",
	)
	cmd.Env = append(os.Environ(),
		executionHelperEnv+"=1",
		executionHelperModeEnv+"=interactive-no-signal",
		executionHelperJobIDEnv+"=blocking-job",
	)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Stdin = strings.NewReader("blocking prompt\n")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	cmdDone := make(chan error, 1)
	go func() {
		cmdDone <- cmd.Wait()
	}()

	return cmd, cmdDone
}
