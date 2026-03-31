package run

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/compozy/looper/internal/looper/model"
)

const (
	executionHelperEnv      = "GO_WANT_EXECUTION_HELPER_PROCESS"
	executionHelperModeEnv  = "EXECUTION_HELPER_MODE"
	executionHelperPortEnv  = "EXECUTION_SIGNAL_PORT"
	executionHelperJobIDEnv = "EXECUTION_JOB_ID"
	executionHelperTimeout  = 10 * time.Second
)

func TestExecutionHelperProcess(_ *testing.T) {
	if os.Getenv(executionHelperEnv) != "1" {
		return
	}

	mode := os.Getenv(executionHelperModeEnv)
	port := os.Getenv(executionHelperPortEnv)
	jobID := os.Getenv(executionHelperJobIDEnv)

	if port != "" {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)

		url := fmt.Sprintf("http://localhost:%s/health", port)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
		if err != nil {
			cancel()
			_, _ = fmt.Fprintf(os.Stderr, "health-check-request-error:%v", err)
			os.Exit(3)
		}
		resp, err := (&http.Client{}).Do(req)
		if err != nil {
			cancel()
			_, _ = fmt.Fprintf(os.Stderr, "health-check-error:%v", err)
			os.Exit(3)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			cancel()
			_, _ = fmt.Fprintf(os.Stderr, "health-check-status:%d", resp.StatusCode)
			os.Exit(4)
		}
		cancel()
	}

	reader := bufio.NewReader(os.Stdin)
	_, _ = os.Stdout.WriteString("Claude Code\n> ")

	switch mode {
	case "interactive-hang-no-signal":
		_, _ = os.Stdout.WriteString("prompt-received\n")
		select {}
	case "interactive-exit-after-ready":
		_, _ = os.Stdout.WriteString("prompt-received\n")
		os.Exit(0)
	}

	line, err := readExecutionHelperLine(reader)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stdout, "read-error:%v", err)
		os.Exit(5)
	}
	_, _ = fmt.Fprintf(os.Stdout, "received:%s\n", line)

	switch mode {
	case "interactive-done":
		if err := postJobDoneSignal(port, jobID); err != nil {
			_, _ = fmt.Fprintf(os.Stdout, "signal-error:%v\n", err)
			os.Exit(6)
		}
		_, _ = os.Stdout.WriteString("done-signaled\n")
		select {}
	case "interactive-no-signal":
		_, _ = os.Stdout.WriteString("prompt-received\n")
		select {}
	case "interactive-exit-no-signal":
		_, _ = os.Stdout.WriteString("prompt-received\n")
		os.Exit(0)
	case "legacy-echo-exit":
		_, _ = fmt.Fprintf(os.Stdout, "legacy:%s\n", line)
		os.Exit(0)
	case "legacy-exit-7":
		_, _ = os.Stdout.WriteString("legacy-fail\n")
		os.Exit(7)
	default:
		_, _ = fmt.Fprintf(os.Stderr, "unknown execution helper mode: %s", mode)
		os.Exit(2)
	}
}

func TestExecuteInteractiveFullLifecycleAndSignalServerCleanup(t *testing.T) {
	overrideHeadlessUIProgram(t)
	overrideExecutionCommandFactory(t, "interactive-done")

	port := freeSignalServerPort(t)
	cfg := &model.RuntimeConfig{
		IDE:                    model.IDEClaude,
		Mode:                   model.ExecutionModePRDTasks,
		SignalPort:             port,
		Concurrent:             1,
		Timeout:                time.Second,
		RetryBackoffMultiplier: 1.5,
		ReasoningEffort:        "medium",
	}

	jobs := []model.Job{
		newExecutionTestJob(t, "batch-001"),
		newExecutionTestJob(t, "batch-002"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- Execute(ctx, jobs, cfg)
	}()

	waitForFileContains(t, jobs[0].OutLog, "done-signaled")
	waitForFileContains(t, jobs[1].OutLog, "done-signaled")

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
	case <-time.After(executionHelperTimeout):
		t.Fatal("timed out waiting for Execute() to return")
	}

	assertSignalServerStopped(t, port)
}

func TestExecuteInteractiveActivityTimeoutWithoutDoneSignal(t *testing.T) {
	overrideHeadlessUIProgram(t)
	overrideExecutionCommandFactory(t, "interactive-no-signal")
	overrideActivityCheckInterval(t, 10*time.Millisecond)

	port := freeSignalServerPort(t)
	cfg := &model.RuntimeConfig{
		IDE:                    model.IDEClaude,
		Mode:                   model.ExecutionModePRDTasks,
		SignalPort:             port,
		Concurrent:             1,
		Timeout:                40 * time.Millisecond,
		RetryBackoffMultiplier: 1.5,
		ReasoningEffort:        "medium",
	}

	jobs := []model.Job{newExecutionTestJob(t, "batch-timeout")}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- Execute(ctx, jobs, cfg)
	}()

	waitForFileContains(t, jobs[0].OutLog, "prompt-received")
	time.Sleep(200 * time.Millisecond)
	cancel()

	var err error
	select {
	case err = <-errCh:
	case <-time.After(executionHelperTimeout):
		t.Fatal("timed out waiting for Execute() after timeout path")
	}
	if err == nil {
		t.Fatal("Execute() unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), "one or more groups failed") {
		t.Fatalf("unexpected Execute() error: %v", err)
	}

	assertSignalServerStopped(t, port)
}

func TestExecuteInteractiveGracefulShutdownStopsSignalServer(t *testing.T) {
	overrideHeadlessUIProgram(t)
	overrideExecutionCommandFactory(t, "interactive-no-signal")
	overrideActivityCheckInterval(t, 100*time.Millisecond)

	port := freeSignalServerPort(t)
	cfg := &model.RuntimeConfig{
		IDE:                    model.IDEClaude,
		Mode:                   model.ExecutionModePRDTasks,
		SignalPort:             port,
		Concurrent:             1,
		Timeout:                5 * time.Second,
		RetryBackoffMultiplier: 1.5,
		ReasoningEffort:        "medium",
	}

	jobs := []model.Job{newExecutionTestJob(t, "batch-shutdown")}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- Execute(ctx, jobs, cfg)
	}()

	waitForFileContains(t, jobs[0].OutLog, "prompt-received")
	cancel()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("Execute() unexpectedly succeeded after shutdown")
		}
	case <-time.After(executionHelperTimeout):
		t.Fatal("timed out waiting for Execute() to return after shutdown")
	}

	assertSignalServerStopped(t, port)
}

func TestExecuteInteractiveJobWithTimeoutReturnsTimeoutWithoutDoneSignal(t *testing.T) {
	overrideExecutionCommandFactory(t, "interactive-hang-no-signal")
	overrideActivityCheckInterval(t, 10*time.Millisecond)

	execCtx, jb := newDirectInteractiveExecutionContext(t, "batch-direct-timeout")

	result := executeInteractiveJobWithTimeout(context.Background(), execCtx, jb, 0, 40*time.Millisecond)
	if result.status != attemptStatusTimeout {
		t.Fatalf("status = %q, want %q", result.status, attemptStatusTimeout)
	}
	if result.failure == nil || !strings.Contains(result.failure.err.Error(), "activity timeout") {
		t.Fatalf("failure = %#v, want activity timeout", result.failure)
	}
}

func TestExecuteInteractiveJobWithTimeoutReturnsCanceledOnContextCancel(t *testing.T) {
	overrideExecutionCommandFactory(t, "interactive-hang-no-signal")
	overrideActivityCheckInterval(t, 100*time.Millisecond)

	execCtx, jb := newDirectInteractiveExecutionContext(t, "batch-direct-cancel")

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	result := executeInteractiveJobWithTimeout(ctx, execCtx, jb, 0, time.Second)
	if result.status != attemptStatusCanceled {
		t.Fatalf("status = %q, want %q", result.status, attemptStatusCanceled)
	}
	if result.failure == nil || result.failure.exitCode != exitCodeCanceled {
		t.Fatalf("failure = %#v, want canceled failure", result.failure)
	}
}

func TestExecuteInteractiveJobWithTimeoutDoesNotSucceedWhenProcessExitsWithoutDoneSignal(t *testing.T) {
	overrideExecutionCommandFactory(t, "interactive-exit-after-ready")
	overrideActivityCheckInterval(t, 10*time.Millisecond)

	execCtx, jb := newDirectInteractiveExecutionContext(t, "batch-direct-exit")

	result := executeInteractiveJobWithTimeout(context.Background(), execCtx, jb, 0, time.Second)
	if result.status == attemptStatusSuccess || result.status == attemptStatusCanceled {
		t.Fatalf("status = %q, want non-success result when done signal is missing", result.status)
	}
	if result.failure == nil {
		t.Fatal("failure = nil, want missing done signal failure details")
	}
	errText := result.failure.err.Error()
	if !strings.Contains(errText, "done signal") && !strings.Contains(errText, "activity timeout") {
		t.Fatalf("failure err = %q, want missing done signal or activity timeout", errText)
	}
}

func TestExecuteLegacyJobWithTimeoutSuccessAndFailurePaths(t *testing.T) {
	overrideActivityCheckInterval(t, 10*time.Millisecond)

	t.Run("success", func(t *testing.T) {
		overrideExecutionCommandFactory(t, "legacy-echo-exit")

		cfg := &config{ide: model.IDECodex}
		jobModel := newExecutionTestJob(t, "legacy-success")
		jobModel.Prompt = []byte("legacy prompt\n")
		jb := newJobs([]model.Job{jobModel})[0]

		result := executeLegacyJobWithTimeout(context.Background(), cfg, &jb, 0, t.TempDir(), executionHelperTimeout)
		if result.status != attemptStatusSuccess {
			t.Fatalf("status = %q, want %q", result.status, attemptStatusSuccess)
		}
		waitForFileContains(t, jobModel.OutLog, "legacy:legacy prompt")
	})

	t.Run("failure", func(t *testing.T) {
		overrideExecutionCommandFactory(t, "legacy-exit-7")

		cfg := &config{ide: model.IDECodex}
		jobModel := newExecutionTestJob(t, "legacy-failure")
		jobModel.Prompt = []byte("legacy prompt\n")
		jb := newJobs([]model.Job{jobModel})[0]

		result := executeLegacyJobWithTimeout(context.Background(), cfg, &jb, 0, t.TempDir(), executionHelperTimeout)
		if result.status != attemptStatusFailure {
			t.Fatalf("status = %q, want %q", result.status, attemptStatusFailure)
		}
		if result.exitCode != 7 {
			t.Fatalf("exitCode = %d, want 7", result.exitCode)
		}
	})
}

func overrideHeadlessUIProgram(t *testing.T) {
	t.Helper()

	previous := newUIProgramFunc
	newUIProgramFunc = func(model tea.Model) *tea.Program {
		return tea.NewProgram(model, tea.WithInput(nil), tea.WithOutput(io.Discard))
	}
	t.Cleanup(func() {
		newUIProgramFunc = previous
	})
}

func overrideExecutionCommandFactory(t *testing.T, mode string) {
	t.Helper()

	previous := buildJobCommandFunc
	buildJobCommandFunc = func(ctx context.Context, cfg *config, jb *job) *exec.Cmd {
		cmd := exec.CommandContext(
			ctx,
			os.Args[0],
			"-test.run=^TestExecutionHelperProcess$",
		)
		env := []string{
			executionHelperEnv + "=1",
			executionHelperModeEnv + "=" + mode,
			executionHelperJobIDEnv + "=" + jb.safeName,
		}
		if cfg.signalPort > 0 {
			env = append(env, executionHelperPortEnv+"="+strconv.Itoa(cfg.signalPort))
		}
		cmd.Env = append(os.Environ(), env...)
		return cmd
	}
	t.Cleanup(func() {
		buildJobCommandFunc = previous
	})
}

func overrideActivityCheckInterval(t *testing.T, interval time.Duration) {
	t.Helper()

	previous := activityCheckInterval
	activityCheckInterval = interval
	t.Cleanup(func() {
		activityCheckInterval = previous
	})
}

func newExecutionTestJob(t *testing.T, safeName string) model.Job {
	t.Helper()

	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, safeName+".md")
	if err := os.WriteFile(promptPath, []byte("# prompt\n"), 0o600); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}

	return model.Job{
		CodeFiles:     []string{safeName + ".go"},
		Groups:        map[string][]model.IssueEntry{},
		SafeName:      safeName,
		Prompt:        []byte("unused in interactive tests"),
		OutPromptPath: promptPath,
		OutLog:        filepath.Join(tmpDir, safeName+".out.log"),
		ErrLog:        filepath.Join(tmpDir, safeName+".err.log"),
	}
}

func newDirectInteractiveExecutionContext(t *testing.T, safeName string) (*jobExecutionContext, *job) {
	t.Helper()

	jobModel := newExecutionTestJob(t, safeName)
	jb := newJobs([]model.Job{jobModel})[0]
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	return &jobExecutionContext{
		cfg: &config{
			ide:                    model.IDEClaude,
			signalPort:             0,
			timeout:                time.Second,
			retryBackoffMultiplier: 1.5,
		},
		cwd:              cwd,
		interactive:      true,
		jobDoneSignals:   map[string]chan SignalEvent{safeName: make(chan SignalEvent, 1)},
		terminalRuntimes: make([]*terminalRuntime, 1),
	}, &jb
}

func waitForFileContains(t *testing.T, path string, want string) {
	t.Helper()

	deadline := time.Now().Add(executionHelperTimeout)
	for time.Now().Before(deadline) {
		body, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(body), want) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	body, _ := os.ReadFile(path)
	t.Fatalf("timed out waiting for %s to contain %q; got %q", path, want, string(body))
}

func assertSignalServerStopped(t *testing.T, port int) {
	t.Helper()

	client := &http.Client{Timeout: 200 * time.Millisecond}
	url := fmt.Sprintf("http://localhost:%d/health", port)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := client.Do(req)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatalf("expected signal server on port %d to be stopped", port)
	}
}

func readExecutionHelperLine(reader *bufio.Reader) (string, error) {
	var builder strings.Builder
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return builder.String(), err
		}
		if b == '\n' || b == '\r' {
			return builder.String(), nil
		}
		builder.WriteByte(b)
	}
}

func postJobDoneSignal(port, jobID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	body := strings.NewReader(fmt.Sprintf(`{"id":%q}`, jobID))
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("http://localhost:%s/job/done", port),
		body,
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}
