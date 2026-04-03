package run

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
)

func TestComposeSessionPromptPrependsSystemPrompt(t *testing.T) {
	got := string(composeSessionPrompt([]byte("user prompt"), "system instructions"))
	want := "system instructions\n\nuser prompt"
	if got != want {
		t.Fatalf("expected composed prompt %q, got %q", want, got)
	}
}

func TestExecuteDryRunCompletesTopLevelFlow(t *testing.T) {
	tmpDir := t.TempDir()
	err := Execute(context.Background(), []model.Job{
		{
			CodeFiles: []string{"task_01"},
			Groups: map[string][]model.IssueEntry{
				"task_01": {{Name: "task_01.md", CodeFile: "task_01"}},
			},
			SafeName: "task_01",
			Prompt:   []byte("do the work"),
			OutLog:   filepath.Join(tmpDir, "task_01.out.log"),
			ErrLog:   filepath.Join(tmpDir, "task_01.err.log"),
		},
	}, &model.RuntimeConfig{
		DryRun:                 true,
		Concurrent:             1,
		IDE:                    model.IDECodex,
		Model:                  "test-model",
		ReasoningEffort:        "medium",
		RetryBackoffMultiplier: 2,
		Mode:                   model.ExecutionModePRReview,
	})
	if err != nil {
		t.Fatalf("execute dry run: %v", err)
	}
}

func TestJobRunnerRetriesACPErrorThenSucceeds(t *testing.T) {
	tmpDir := t.TempDir()
	firstClient := newFakeACPClient(func(_ context.Context, _ agent.SessionRequest) (agent.Session, error) {
		session := newFakeACPSession("sess-1")
		go session.finish(&agent.SessionError{Code: 4901, Message: "temporary failure"})
		return session, nil
	})
	secondClientErrCh := make(chan error, 1)
	secondClient := newFakeACPClient(func(_ context.Context, _ agent.SessionRequest) (agent.Session, error) {
		session := newFakeACPSession("sess-2")
		go func() {
			textBlock, err := model.NewContentBlock(model.TextBlock{Text: "retry succeeded"})
			if err != nil {
				secondClientErrCh <- err
				return
			}
			session.publish(model.SessionUpdate{
				Blocks: []model.ContentBlock{textBlock},
				Status: model.StatusRunning,
			})
			session.finish(nil)
			secondClientErrCh <- nil
		}()
		return session, nil
	})
	installFakeACPClients(t, firstClient, secondClient)

	job := newTestACPJob(tmpDir)
	execCtx := &jobExecutionContext{
		cfg: &config{
			ide:                    model.IDECodex,
			model:                  "test-model",
			reasoningEffort:        "medium",
			maxRetries:             1,
			retryBackoffMultiplier: 2,
			timeout:                time.Second,
		},
		cwd: tmpDir,
	}

	runner := newJobRunner(0, &job, execCtx)
	runner.run(context.Background())

	if got := runner.lifecycle.state; got != jobPhaseSucceeded {
		t.Fatalf("expected succeeded lifecycle state, got %s", got)
	}
	if got := atomic.LoadInt32(&execCtx.failed); got != 0 {
		t.Fatalf("expected no failed jobs, got %d", got)
	}
	if got := firstClient.closeCalls.Load() + secondClient.closeCalls.Load(); got != 2 {
		t.Fatalf("expected both clients to close, got %d", got)
	}

	outLog, err := os.ReadFile(job.outLog)
	if err != nil {
		t.Fatalf("read out log: %v", err)
	}
	if !strings.Contains(string(outLog), "retry succeeded") {
		t.Fatalf("expected retry success output in out log, got %q", string(outLog))
	}
	errLog, err := os.ReadFile(job.errLog)
	if err != nil {
		t.Fatalf("read err log: %v", err)
	}
	if !strings.Contains(string(errLog), "temporary failure") {
		t.Fatalf("expected first failure in err log, got %q", string(errLog))
	}
	if err := waitForAsyncTestError(t, secondClientErrCh); err != nil {
		t.Fatalf("new content block: %v", err)
	}
}

func TestJobRunnerSuccessRunsTaskPostSuccessHook(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, ".compozy", "tasks", "demo")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir: %v", err)
	}
	writeRunTaskFile(t, tasksDir, "task_01.md", "pending")

	taskPath := filepath.Join(tasksDir, "task_01.md")
	taskContent, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read task file: %v", err)
	}

	successClient := newFakeACPClient(func(_ context.Context, _ agent.SessionRequest) (agent.Session, error) {
		session := newFakeACPSession("sess-task")
		go session.finish(nil)
		return session, nil
	})
	installFakeACPClients(t, successClient)

	job := newTestACPJob(tmpDir)
	job.groups = map[string][]model.IssueEntry{
		"task_01": {{
			Name:     "task_01.md",
			AbsPath:  taskPath,
			Content:  string(taskContent),
			CodeFile: "task_01",
		}},
	}
	execCtx := &jobExecutionContext{
		cfg: &config{
			ide:                    model.IDECodex,
			model:                  "test-model",
			reasoningEffort:        "medium",
			mode:                   model.ExecutionModePRDTasks,
			tasksDir:               tasksDir,
			retryBackoffMultiplier: 2,
			timeout:                time.Second,
		},
		cwd: tmpDir,
	}

	runner := newJobRunner(0, &job, execCtx)
	runner.run(context.Background())

	if got := runner.lifecycle.state; got != jobPhaseSucceeded {
		t.Fatalf("expected succeeded lifecycle state, got %s", got)
	}
	updatedTask, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read updated task file: %v", err)
	}
	if !strings.Contains(string(updatedTask), "status: completed") {
		t.Fatalf("expected task hook to mark file completed, got:\n%s", string(updatedTask))
	}
}

func TestJobRunnerCancellationDoesNotRetry(t *testing.T) {
	tmpDir := t.TempDir()
	created := make(chan struct{}, 1)
	cancelClient := newFakeACPClient(func(ctx context.Context, _ agent.SessionRequest) (agent.Session, error) {
		session := newFakeACPSession("sess-cancel")
		created <- struct{}{}
		go func() {
			<-ctx.Done()
			session.finish(context.Cause(ctx))
		}()
		return session, nil
	})
	installFakeACPClients(t, cancelClient)

	job := newTestACPJob(tmpDir)
	execCtx := &jobExecutionContext{
		cfg: &config{
			ide:                    model.IDECodex,
			model:                  "test-model",
			reasoningEffort:        "medium",
			maxRetries:             3,
			retryBackoffMultiplier: 2,
			timeout:                time.Second,
		},
		cwd: tmpDir,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	runner := newJobRunner(0, &job, execCtx)
	go func() {
		defer close(done)
		runner.run(ctx)
	}()

	select {
	case <-created:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for session creation")
	}
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for canceled runner")
	}

	if got := runner.lifecycle.state; got != jobPhaseCanceled {
		t.Fatalf("expected canceled lifecycle state, got %s", got)
	}
	if got := cancelClient.createCalls.Load(); got != 1 {
		t.Fatalf("expected exactly one attempt before cancellation, got %d", got)
	}
}

func TestExecuteJobWithTimeoutUsesContextBackstop(t *testing.T) {
	tmpDir := t.TempDir()
	timeoutClient := newFakeACPClient(func(ctx context.Context, _ agent.SessionRequest) (agent.Session, error) {
		session := newFakeACPSession("sess-timeout")
		go func() {
			<-ctx.Done()
			session.finish(context.Cause(ctx))
		}()
		return session, nil
	})
	installFakeACPClients(t, timeoutClient)

	job := newTestACPJob(tmpDir)
	var aggregate model.Usage
	var aggregateMu sync.Mutex
	result := executeJobWithTimeout(
		context.Background(),
		&config{
			ide:                    model.IDECodex,
			model:                  "test-model",
			reasoningEffort:        "medium",
			retryBackoffMultiplier: 2,
		},
		&job,
		tmpDir,
		false,
		nil,
		0,
		25*time.Millisecond,
		&aggregate,
		&aggregateMu,
		nil,
	)

	if got := result.status; got != attemptStatusTimeout {
		t.Fatalf("expected timeout status, got %s", got)
	}
	if got := timeoutClient.closeCalls.Load(); got != 1 {
		t.Fatalf("expected client close to run as timeout backstop, got %d closes", got)
	}
}

func TestExecuteJobWithTimeoutInteractiveDoesNotLeakACPLogsToDefaultLogger(t *testing.T) {
	tmpDir := t.TempDir()

	var logBuf bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, nil)))
	t.Cleanup(func() {
		slog.SetDefault(previousLogger)
	})

	successClientErrCh := make(chan error, 1)
	successClient := newFakeACPClient(func(_ context.Context, _ agent.SessionRequest) (agent.Session, error) {
		session := newFakeACPSession("sess-ui")
		go func() {
			textBlock, err := model.NewContentBlock(model.TextBlock{Text: "hello from ACP"})
			if err != nil {
				successClientErrCh <- err
				return
			}
			session.publish(model.SessionUpdate{
				Kind:   model.UpdateKindAgentMessageChunk,
				Blocks: []model.ContentBlock{textBlock},
				Status: model.StatusRunning,
			})
			session.finish(nil)
			successClientErrCh <- nil
		}()
		return session, nil
	})
	installFakeACPClients(t, successClient)

	job := newTestACPJob(tmpDir)
	var aggregate model.Usage
	var aggregateMu sync.Mutex
	uiCh := make(chan uiMsg, 4)
	result := executeJobWithTimeout(
		context.Background(),
		&config{
			ide:                    model.IDECodex,
			model:                  "test-model",
			reasoningEffort:        "medium",
			retryBackoffMultiplier: 2,
		},
		&job,
		tmpDir,
		true,
		uiCh,
		0,
		time.Second,
		&aggregate,
		&aggregateMu,
		nil,
	)

	if got := result.status; got != attemptStatusSuccess {
		t.Fatalf("expected success status, got %s", got)
	}
	if err := waitForAsyncTestError(t, successClientErrCh); err != nil {
		t.Fatalf("new content block: %v", err)
	}
	if got := strings.TrimSpace(logBuf.String()); got != "" {
		t.Fatalf("expected interactive ACP execution to suppress default logger output, got %q", got)
	}
}

func TestJobExecutionContextUICleanupHelpers(t *testing.T) {
	ui := &fakeLifecycleUISession{eventsCh: make(chan uiMsg)}
	execCtx := &jobExecutionContext{ui: ui}

	if err := execCtx.awaitUIAfterCompletion(); err != nil {
		t.Fatalf("awaitUIAfterCompletion: %v", err)
	}
	if ui.closeEventsCalls != 1 || ui.waitCalls != 1 {
		t.Fatalf(
			"expected awaitUIAfterCompletion to close events and wait once, got close=%d wait=%d",
			ui.closeEventsCalls,
			ui.waitCalls,
		)
	}

	if err := execCtx.shutdownUI(); err != nil {
		t.Fatalf("shutdownUI: %v", err)
	}
	if ui.shutdownCalls != 1 {
		t.Fatalf("expected shutdownUI to invoke shutdown once, got %d", ui.shutdownCalls)
	}

	execCtx.cleanup()
	if ui.shutdownCalls != 2 {
		t.Fatalf("expected cleanup to call shutdownUI again, got %d", ui.shutdownCalls)
	}
}

func TestExecutorControllerAwaitCompletionAndCancelPaths(t *testing.T) {
	done := make(chan struct{})
	close(done)

	ui := &fakeLifecycleUISession{eventsCh: make(chan uiMsg)}
	execCtx := &jobExecutionContext{
		ui:    ui,
		total: 1,
	}
	controller := &executorController{
		ctx:     context.Background(),
		execCtx: execCtx,
		done:    done,
	}

	failed, _, total, err := controller.awaitCompletion()
	if err != nil {
		t.Fatalf("awaitCompletion: %v", err)
	}
	if failed != 0 || total != 1 {
		t.Fatalf("unexpected controller result failed=%d total=%d", failed, total)
	}

	cancelDone := make(chan struct{})
	close(cancelDone)
	cancelUI := &fakeLifecycleUISession{eventsCh: make(chan uiMsg)}
	cancelExecCtx := &jobExecutionContext{
		ui:    cancelUI,
		total: 2,
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cancelController := &executorController{
		ctx:        ctx,
		execCtx:    cancelExecCtx,
		cancelJobs: func(error) {},
		done:       cancelDone,
	}

	failed, _, total, err = cancelController.awaitCompletion()
	if err != nil {
		t.Fatalf("awaitCompletion after cancel: %v", err)
	}
	if failed != 0 || total != 2 {
		t.Fatalf("unexpected canceled controller result failed=%d total=%d", failed, total)
	}
}

func TestJobLifecycleMarkGiveUpRecordsFailure(t *testing.T) {
	execCtx := &jobExecutionContext{}
	lifecycle := newJobLifecycle(0, &job{
		codeFiles: []string{"task_01"},
		outLog:    "task_01.out.log",
		errLog:    "task_01.err.log",
	}, execCtx)

	lifecycle.markGiveUp(failInfo{
		codeFile: "task_01",
		exitCode: 23,
		outLog:   "task_01.out.log",
		errLog:   "task_01.err.log",
		err:      errors.New("boom"),
	})

	if got := lifecycle.state; got != jobPhaseFailed {
		t.Fatalf("expected failed state, got %s", got)
	}
	if got := atomic.LoadInt32(&execCtx.failed); got != 1 {
		t.Fatalf("expected failed counter 1, got %d", got)
	}
	if len(execCtx.failures) != 1 || execCtx.failures[0].exitCode != 23 {
		t.Fatalf("expected recorded failure, got %#v", execCtx.failures)
	}
}

func TestHandleNilExecutionReturnsSetupFailure(t *testing.T) {
	result := handleNilExecution(&job{
		codeFiles: []string{"task_01"},
		outLog:    "task_01.out.log",
		errLog:    "task_01.err.log",
	}, 0)

	if got := result.status; got != attemptStatusSetupFailed {
		t.Fatalf("expected setup failure status, got %s", got)
	}
	if result.failure == nil ||
		!strings.Contains(result.failure.err.Error(), "failed to set up ACP session execution") {
		t.Fatalf("unexpected failure payload: %#v", result.failure)
	}
}

func TestRecordFailureWithContextAddsFailure(t *testing.T) {
	var failures []failInfo
	job := &job{
		codeFiles: []string{"task_01"},
		outLog:    "task_01.out.log",
		errLog:    "task_01.err.log",
	}
	got := recordFailureWithContext(nil, job, &failures, errors.New("boom"), 77)
	if got.exitCode != 77 || got.codeFile != "task_01" {
		t.Fatalf("unexpected failure info: %#v", got)
	}
	if len(failures) != 1 || failures[0].exitCode != 77 {
		t.Fatalf("expected failure to be recorded, got %#v", failures)
	}
}

type fakeACPClient struct {
	createSessionFn func(context.Context, agent.SessionRequest) (agent.Session, error)
	createCalls     atomic.Int32
	closeCalls      atomic.Int32
	killCalls       atomic.Int32
}

func newFakeACPClient(
	createSessionFn func(context.Context, agent.SessionRequest) (agent.Session, error),
) *fakeACPClient {
	return &fakeACPClient{createSessionFn: createSessionFn}
}

func (c *fakeACPClient) CreateSession(ctx context.Context, req agent.SessionRequest) (agent.Session, error) {
	c.createCalls.Add(1)
	if c.createSessionFn == nil {
		return nil, errors.New("missing fake session factory")
	}
	return c.createSessionFn(ctx, req)
}

func (c *fakeACPClient) Close() error {
	c.closeCalls.Add(1)
	return nil
}

func (c *fakeACPClient) Kill() error {
	c.killCalls.Add(1)
	return nil
}

type fakeACPSession struct {
	id      string
	updates chan model.SessionUpdate
	done    chan struct{}

	mu       sync.RWMutex
	err      error
	finished bool
}

func newFakeACPSession(id string) *fakeACPSession {
	return &fakeACPSession{
		id:      id,
		updates: make(chan model.SessionUpdate, 8),
		done:    make(chan struct{}),
	}
}

type fakeLifecycleUISession struct {
	eventsCh         chan uiMsg
	closeEventsCalls int
	shutdownCalls    int
	waitCalls        int
}

func (f *fakeLifecycleUISession) events() chan uiMsg {
	return f.eventsCh
}

func (f *fakeLifecycleUISession) setQuitHandler(func(uiQuitRequest)) {}

func (f *fakeLifecycleUISession) closeEvents() {
	f.closeEventsCalls++
}

func (f *fakeLifecycleUISession) shutdown() {
	f.shutdownCalls++
}

func (f *fakeLifecycleUISession) wait() error {
	f.waitCalls++
	return nil
}

func (s *fakeACPSession) ID() string {
	return s.id
}

func (s *fakeACPSession) Updates() <-chan model.SessionUpdate {
	return s.updates
}

func (s *fakeACPSession) Done() <-chan struct{} {
	return s.done
}

func (s *fakeACPSession) Err() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.err
}

func (s *fakeACPSession) publish(update model.SessionUpdate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return
	}
	s.updates <- update
}

func (s *fakeACPSession) finish(err error) {
	s.mu.Lock()
	if s.finished {
		s.mu.Unlock()
		return
	}
	s.finished = true
	s.err = err
	close(s.updates)
	close(s.done)
	s.mu.Unlock()
}

func installFakeACPClients(t *testing.T, clients ...*fakeACPClient) {
	t.Helper()

	var mu sync.Mutex
	index := 0
	previous := newAgentClient
	newAgentClient = func(context.Context, agent.ClientConfig) (agent.Client, error) {
		mu.Lock()
		defer mu.Unlock()
		if index >= len(clients) {
			return nil, fmt.Errorf("no fake ACP client configured for attempt %d", index+1)
		}
		client := clients[index]
		index++
		return client, nil
	}
	t.Cleanup(func() {
		newAgentClient = previous
	})
}

func newTestACPJob(tmpDir string) job {
	return job{
		codeFiles:    []string{"task_01"},
		groups:       map[string][]model.IssueEntry{},
		safeName:     "task_01",
		prompt:       []byte("finish the task"),
		systemPrompt: "workflow memory",
		outLog:       filepath.Join(tmpDir, "task_01.out.log"),
		errLog:       filepath.Join(tmpDir, "task_01.err.log"),
		outBuffer:    newLineBuffer(0),
		errBuffer:    newLineBuffer(0),
	}
}

func waitForAsyncTestError(t *testing.T, errCh <-chan error) error {
	t.Helper()

	select {
	case err := <-errCh:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for async test result")
		return nil
	}
}
