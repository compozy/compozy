package acp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/compozy/internal/subprocess"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/toolruntime"
)

func TestACPHelperProcess(_ *testing.T) {
	if os.Getenv(testHelperEnvKey) != "1" {
		return
	}

	agent := &helperACPAgent{
		scenario: os.Getenv(testHelperScenarioKey),
		filePath: os.Getenv(testHelperFileKey),
	}
	input := io.Reader(os.Stdin)
	var captureFile *os.File
	capturePath := strings.TrimSpace(os.Getenv(testHelperCaptureKey))
	if capturePath != "" {
		var err error
		captureFile, err = os.Create(capturePath)
		if err != nil {
			if _, printErr := fmt.Fprintf(os.Stderr, "create capture file: %v\n", err); printErr != nil {
				os.Exit(1)
			}
			os.Exit(1)
		}
		input = io.TeeReader(os.Stdin, captureFile)
	}

	conn := acpsdk.NewAgentSideConnection(agent, os.Stdout, input)
	agent.conn = conn
	<-conn.Done()
	if captureFile != nil {
		if err := captureFile.Close(); err != nil {
			if _, printErr := fmt.Fprintf(os.Stderr, "close capture file: %v\n", err); printErr != nil {
				os.Exit(1)
			}
			os.Exit(1)
		}
	}
	os.Exit(0)
}

func TestACPWrapperProcess(_ *testing.T) {
	if os.Getenv(testWrapperEnvKey) != "1" {
		return
	}

	bin, err := os.Executable()
	if err != nil {
		if _, printErr := fmt.Fprintf(os.Stderr, "resolve test binary: %v\n", err); printErr != nil {
			os.Exit(1)
		}
		os.Exit(1)
	}

	cmd := exec.CommandContext(context.Background(), bin, "-test.run=TestACPHelperProcess")
	cmd.Env = append([]string(nil), os.Environ()...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		if _, printErr := fmt.Fprintf(os.Stderr, "start wrapped helper: %v\n", err); printErr != nil {
			os.Exit(1)
		}
		os.Exit(1)
	}

	if pidFile := strings.TrimSpace(os.Getenv(testWrapperPIDFileEnvKey)); pidFile != "" {
		if writeErr := os.WriteFile(pidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0o644); writeErr != nil {
			if _, printErr := fmt.Fprintf(os.Stderr, "write pid file: %v\n", writeErr); printErr != nil {
				os.Exit(1)
			}
			if killErr := cmd.Process.Kill(); killErr != nil {
				if _, printErr := fmt.Fprintf(os.Stderr, "kill wrapped helper: %v\n", killErr); printErr != nil {
					os.Exit(1)
				}
			}
			os.Exit(1)
		}
	}

	if err := cmd.Wait(); err != nil {
		if _, printErr := fmt.Fprintf(os.Stderr, "wrapped helper exited: %v\n", err); printErr != nil {
			os.Exit(1)
		}
		os.Exit(1)
	}

	os.Exit(0)
}

func TestDriverRejectsUninitializedProcessState(t *testing.T) {
	t.Parallel()

	driver := New()

	t.Run("Should prompt requires connection", func(t *testing.T) {
		t.Parallel()

		proc := &AgentProcess{SessionID: "session-1"}
		events, err := driver.Prompt(context.Background(), proc, PromptRequest{
			TurnID:  "turn-1",
			Message: "hello",
		})
		if err == nil {
			t.Fatalf("Prompt() error = nil, want %v", errProcessConnectionUninitialized)
		}
		if !errors.Is(err, errProcessConnectionUninitialized) {
			t.Fatalf("Prompt() error = %v, want %v", err, errProcessConnectionUninitialized)
		}
		if events != nil {
			t.Fatalf("Prompt() events = %v, want nil", events)
		}
	})

	t.Run("Should cancel requires connection and does not panic", func(t *testing.T) {
		t.Parallel()

		proc := &AgentProcess{SessionID: "session-1"}
		var (
			err    error
			panicV any
		)
		func() {
			defer func() {
				panicV = recover()
			}()
			err = driver.Cancel(context.Background(), proc)
		}()

		if panicV != nil {
			t.Fatalf("Cancel() panicked: %v", panicV)
		}
		if !errors.Is(err, errProcessConnectionUninitialized) {
			t.Fatalf("Cancel() error = %v, want %v", err, errProcessConnectionUninitialized)
		}
	})

	t.Run("Should stop requires lifecycle", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		err := driver.Stop(ctx, &AgentProcess{})
		if err == nil {
			t.Fatalf("Stop() error = nil, want %v", errProcessLifecycleUninitialized)
		}
		if !errors.Is(err, errProcessLifecycleUninitialized) {
			t.Fatalf("Stop() error = %v, want %v", err, errProcessLifecycleUninitialized)
		}
	})
}

func TestAgentProcessExitLifecycle(t *testing.T) {
	t.Run("Should surface a process crash", func(t *testing.T) {
		t.Parallel()

		driver := New()
		proc := startHelperProcess(t, driver, "crash_on_prompt", "", StartOpts{})

		eventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
			TurnID:  "turn-crash",
			Message: "trigger crash",
		})
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		events := collectEvents(t, eventsCh)
		if len(events) == 0 || events[len(events)-1].Type != EventTypeError {
			t.Fatalf("Prompt() last event = %#v, want error", events)
		}

		waitErr := waitForProcess(t, proc)
		if waitErr == nil {
			t.Fatal("Wait() error = nil, want process crash")
		}
	})

	t.Run("Should close Done only after accepted child work stops", func(t *testing.T) {
		t.Parallel()

		processCtx, cancelProcess := context.WithCancel(testutil.Context(t))
		proc := &AgentProcess{
			processCtx:    processCtx,
			cancelProcess: cancelProcess,
			done:          make(chan struct{}),
		}
		started := make(chan struct{})
		release := make(chan struct{})
		finished := make(chan struct{})
		run, ok := proc.beginChildTask()
		if !ok {
			t.Fatal("beginChildTask() rejected initial work")
		}
		proc.startReservedChildTask(run, func() {
			close(started)
			<-release
			close(finished)
		})
		<-started

		go proc.waitForExit(testutil.Context(t), defaultProcessRecordTimeout)
		select {
		case <-proc.Done():
			t.Fatal("Done() closed before child work stopped")
		default:
		}

		close(release)
		<-proc.Done()
		select {
		case <-finished:
		default:
			t.Fatal("Done() closed before child finalizer returned")
		}
		if _, admitted := proc.beginChildTask(); admitted {
			t.Fatal("beginChildTask() admitted work after Done()")
		}
	})
}

func TestStopManagedProcessRespectsContext(t *testing.T) {
	t.Run("ShouldReturnDeadlineExceededWhenManagedProcessShutdownExceedsStopContext", func(t *testing.T) {
		t.Parallel()

		driver := New(WithStopTimeout(5 * time.Second))
		managed, err := subprocess.Launch(context.Background(), subprocess.LaunchConfig{
			Command:          "sh",
			Args:             []string{"-c", "sleep 30"},
			DisableTransport: true,
			ShutdownTimeout:  time.Second,
		})
		if err != nil {
			t.Fatalf("Launch() error = %v", err)
		}

		proc := &AgentProcess{
			managed: managed,
			done:    make(chan struct{}),
		}
		go proc.waitForExit(context.Background(), defaultProcessRecordTimeout)
		t.Cleanup(func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if shutdownErr := managed.Shutdown(cleanupCtx); shutdownErr != nil {
				t.Fatalf("managed.Shutdown() error = %v", shutdownErr)
			}
			select {
			case <-proc.Done():
			case <-cleanupCtx.Done():
				t.Fatalf("process did not exit during cleanup: %v", cleanupCtx.Err())
			}
		})

		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		defer cancel()

		startedAt := time.Now()
		err = driver.Stop(stopCtx, proc)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Stop() error = %v, want context deadline exceeded", err)
		}
		if elapsed := time.Since(startedAt); elapsed > time.Second {
			t.Fatalf("Stop() elapsed = %v, want <= 1s", elapsed)
		}
	})
}

func TestRegisterAgentProcessRetainsRegistryForPIDLessSandboxAgents(t *testing.T) {
	t.Run("Should keep registry available for external sandbox terminal tracking", func(t *testing.T) {
		t.Parallel()

		registry := toolruntime.NewRegistry(nil)
		driver := &Driver{processRegistry: registry}
		process := &AgentProcess{PID: 0}

		if err := driver.registerAgentProcess(context.Background(), process); err != nil {
			t.Fatalf("registerAgentProcess(PID=0) error = %v", err)
		}
		if process.processRegistry != registry {
			t.Fatalf("process.processRegistry = %p, want %p", process.processRegistry, registry)
		}
		if process.processRecord != nil {
			t.Fatalf("process.processRecord = %#v, want nil for PID-less agent", process.processRecord)
		}
	})
}

func TestProcessRecordContext(t *testing.T) {
	t.Run("Should detach cancellation while preserving a bounded deadline", func(t *testing.T) {
		t.Parallel()

		parent, cancelParent := context.WithCancel(context.Background())
		cancelParent()

		ctx, cancel := processRecordContext(parent, 25*time.Millisecond)
		defer cancel()
		if err := ctx.Err(); err != nil {
			t.Fatalf("processRecordContext() err = %v, want detached from parent cancellation", err)
		}
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("processRecordContext() deadline missing")
		}
		if remaining := time.Until(deadline); remaining <= 0 || remaining > time.Second {
			t.Fatalf("processRecordContext() remaining deadline = %s, want bounded positive deadline", remaining)
		}
	})
}

func TestCheckpointProcessOwnerWrapsCheckpointErrors(t *testing.T) {
	t.Run("Should add ACP context while preserving checkpoint root error", func(t *testing.T) {
		t.Parallel()

		root := errors.New("checkpoint failed")
		registry := toolruntime.NewRegistry(&failingToolRuntimeStore{updateErr: root})
		handle, err := registry.Register(context.Background(), toolruntime.RegisterConfig{
			Source:  toolruntime.ProcessSourceACPAgent,
			Owner:   toolruntime.ProcessOwner{SessionID: "old-session"},
			Command: "agent",
		})
		if err != nil {
			t.Fatalf("Register() error = %v", err)
		}
		process := &AgentProcess{
			SessionID:     "new-session",
			processRecord: handle,
		}

		err = process.checkpointProcessOwner(context.Background())
		if !errors.Is(err, root) || !strings.Contains(err.Error(), "checkpoint process owner") {
			t.Fatalf("checkpointProcessOwner() error = %v, want ACP context wrapping root", err)
		}
	})
}
