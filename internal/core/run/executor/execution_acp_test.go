package executor

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
	"github.com/compozy/compozy/internal/core/run/internal/acpshared"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
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
	}, model.NewRunArtifacts(tmpDir, "dry-run-test"), nil, nil, &model.RuntimeConfig{
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
		ctx: context.Background(),
		cfg: &config{
			IDE:                    model.IDECodex,
			Model:                  "test-model",
			ReasoningEffort:        "medium",
			MaxRetries:             1,
			RetryBackoffMultiplier: 2,
			Timeout:                time.Second,
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

	outLog, err := os.ReadFile(job.OutLog)
	if err != nil {
		t.Fatalf("read out log: %v", err)
	}
	if !strings.Contains(string(outLog), "retry succeeded") {
		t.Fatalf("expected retry success output in out log, got %q", string(outLog))
	}
	errLog, err := os.ReadFile(job.ErrLog)
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

func TestJobRunnerRetriesRetryableACPSetupFailureThenSucceeds(t *testing.T) {
	tmpDir := t.TempDir()
	firstClient := newFakeACPClient(func(context.Context, agent.SessionRequest) (agent.Session, error) {
		return nil, &agent.SessionSetupError{
			Stage: agent.SessionSetupStageNewSession,
			Err:   errors.New("temporary session bootstrap failure"),
		}
	})
	secondClient := newFakeACPClient(func(_ context.Context, _ agent.SessionRequest) (agent.Session, error) {
		session := newFakeACPSession("sess-setup-retry")
		go session.finish(nil)
		return session, nil
	})
	installFakeACPClients(t, firstClient, secondClient)

	job := newTestACPJob(tmpDir)
	execCtx := &jobExecutionContext{
		ctx: context.Background(),
		cfg: &config{
			IDE:                    model.IDECodex,
			Model:                  "test-model",
			ReasoningEffort:        "medium",
			MaxRetries:             1,
			RetryBackoffMultiplier: 2,
			Timeout:                time.Second,
		},
		cwd: tmpDir,
	}

	runner := newJobRunner(0, &job, execCtx)
	runner.run(context.Background())

	if got := runner.lifecycle.state; got != jobPhaseSucceeded {
		t.Fatalf("expected succeeded lifecycle state, got %s", got)
	}
	if got := firstClient.createCalls.Load(); got != 1 {
		t.Fatalf("expected first setup-failure client to be attempted once, got %d", got)
	}
	if got := secondClient.createCalls.Load(); got != 1 {
		t.Fatalf("expected retry attempt to create one successful session, got %d", got)
	}
	if got := atomic.LoadInt32(&execCtx.failed); got != 0 {
		t.Fatalf("expected no failed jobs, got %d", got)
	}
}

func TestJobRunnerDoesNotRetryNonRetryableACPSetupFailure(t *testing.T) {
	tmpDir := t.TempDir()
	firstClient := newFakeACPClient(func(context.Context, agent.SessionRequest) (agent.Session, error) {
		return nil, &agent.SessionSetupError{
			Stage: agent.SessionSetupStageSetModel,
			Err:   errors.New("invalid session model override"),
		}
	})
	secondClient := newFakeACPClient(func(_ context.Context, _ agent.SessionRequest) (agent.Session, error) {
		session := newFakeACPSession("sess-should-not-run")
		go session.finish(nil)
		return session, nil
	})
	installFakeACPClients(t, firstClient, secondClient)

	job := newTestACPJob(tmpDir)
	execCtx := &jobExecutionContext{
		ctx: context.Background(),
		cfg: &config{
			IDE:                    model.IDECodex,
			Model:                  "test-model",
			ReasoningEffort:        "medium",
			MaxRetries:             3,
			RetryBackoffMultiplier: 2,
			Timeout:                time.Second,
		},
		cwd: tmpDir,
	}

	runner := newJobRunner(0, &job, execCtx)
	runner.run(context.Background())

	if got := runner.lifecycle.state; got != jobPhaseFailed {
		t.Fatalf("expected failed lifecycle state, got %s", got)
	}
	if got := firstClient.createCalls.Load(); got != 1 {
		t.Fatalf("expected non-retryable setup failure to run once, got %d", got)
	}
	if got := secondClient.createCalls.Load(); got != 0 {
		t.Fatalf("expected no retry after non-retryable setup failure, got %d", got)
	}
	if got := atomic.LoadInt32(&execCtx.failed); got != 1 {
		t.Fatalf("expected failed jobs counter to be 1, got %d", got)
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
	job.Groups = map[string][]model.IssueEntry{
		"task_01": {{
			Name:     "task_01.md",
			AbsPath:  taskPath,
			Content:  string(taskContent),
			CodeFile: "task_01",
		}},
	}
	execCtx := &jobExecutionContext{
		ctx: context.Background(),
		cfg: &config{
			IDE:                    model.IDECodex,
			Model:                  "test-model",
			ReasoningEffort:        "medium",
			Mode:                   model.ExecutionModePRDTasks,
			TasksDir:               tasksDir,
			RetryBackoffMultiplier: 2,
			Timeout:                time.Second,
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
		ctx: context.Background(),
		cfg: &config{
			IDE:                    model.IDECodex,
			Model:                  "test-model",
			ReasoningEffort:        "medium",
			MaxRetries:             3,
			RetryBackoffMultiplier: 2,
			Timeout:                time.Second,
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
			IDE:                    model.IDECodex,
			Model:                  "test-model",
			ReasoningEffort:        "medium",
			RetryBackoffMultiplier: 2,
		},
		&job,
		tmpDir,
		false,
		0,
		25*time.Millisecond,
		nil,
		&aggregate,
		&aggregateMu,
		nil,
	)

	if got := result.Status; got != attemptStatusTimeout {
		t.Fatalf("expected timeout status, got %s", got)
	}
	if result.Failure == nil || !strings.Contains(result.Failure.Err.Error(), "activity timeout") {
		t.Fatalf("expected activity-timeout failure, got %#v", result.Failure)
	}
	if got := timeoutClient.closeCalls.Load(); got != 1 {
		t.Fatalf("expected client close to run as timeout backstop, got %d closes", got)
	}
}

func TestExecuteJobWithTimeoutActiveACPUpdatesExtendTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	activeClient := newFakeACPClient(func(_ context.Context, _ agent.SessionRequest) (agent.Session, error) {
		session := newFakeACPSession("sess-active")
		go func() {
			for i := 0; i < 6; i++ {
				time.Sleep(20 * time.Millisecond)
				session.publish(model.SessionUpdate{
					Kind: model.UpdateKindPlanUpdated,
					PlanEntries: []model.SessionPlanEntry{{
						Content:  fmt.Sprintf("step-%d", i+1),
						Priority: "high",
						Status:   "in_progress",
					}},
					Status: model.StatusRunning,
				})
			}
			session.finish(nil)
		}()
		return session, nil
	})
	installFakeACPClients(t, activeClient)

	job := newTestACPJob(tmpDir)
	result := executeJobWithTimeout(
		context.Background(),
		&config{
			IDE:                    model.IDECodex,
			Model:                  "test-model",
			ReasoningEffort:        "medium",
			RetryBackoffMultiplier: 2,
		},
		&job,
		tmpDir,
		false,
		0,
		50*time.Millisecond,
		nil,
		nil,
		nil,
		nil,
	)

	if got := result.Status; got != attemptStatusSuccess {
		t.Fatalf("expected success status, got %s (%#v)", got, result.Failure)
	}
	errLog, err := os.ReadFile(job.ErrLog)
	if err != nil {
		t.Fatalf("read err log: %v", err)
	}
	if strings.Contains(string(errLog), "activity timeout") {
		t.Fatalf("expected no activity-timeout error, got %q", string(errLog))
	}
}

func TestExecuteJobWithTimeoutSetupHangUsesActivityTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	blockingSetupClient := newFakeACPClient(func(ctx context.Context, _ agent.SessionRequest) (agent.Session, error) {
		<-ctx.Done()
		return nil, context.Cause(ctx)
	})
	installFakeACPClients(t, blockingSetupClient)

	job := newTestACPJob(tmpDir)
	result := executeJobWithTimeout(
		context.Background(),
		&config{
			IDE:                    model.IDECodex,
			Model:                  "test-model",
			ReasoningEffort:        "medium",
			RetryBackoffMultiplier: 2,
		},
		&job,
		tmpDir,
		false,
		0,
		25*time.Millisecond,
		nil,
		nil,
		nil,
		nil,
	)

	if got := result.Status; got != attemptStatusTimeout {
		t.Fatalf("expected timeout status for blocked setup, got %s (%#v)", got, result.Failure)
	}
	if result.Failure == nil || !strings.Contains(result.Failure.Err.Error(), "activity timeout") {
		t.Fatalf("expected activity-timeout failure, got %#v", result.Failure)
	}
	if got := blockingSetupClient.closeCalls.Load(); got != 1 {
		t.Fatalf("expected blocked setup client to be closed, got %d closes", got)
	}
}

func TestExecuteJobWithTimeoutInteractiveSuppressesHumanFallbackOnTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	blockingSetupClient := newFakeACPClient(func(ctx context.Context, _ agent.SessionRequest) (agent.Session, error) {
		<-ctx.Done()
		return nil, context.Cause(ctx)
	})
	installFakeACPClients(t, blockingSetupClient)

	job := newTestACPJob(tmpDir)

	var result jobAttemptResult
	stdout, stderr, captureErr := captureExecuteStreams(t, func() error {
		result = executeJobWithTimeout(
			context.Background(),
			&config{
				IDE:                    model.IDECodex,
				Model:                  "test-model",
				ReasoningEffort:        "medium",
				RetryBackoffMultiplier: 2,
			},
			&job,
			tmpDir,
			true,
			0,
			25*time.Millisecond,
			nil,
			nil,
			nil,
			nil,
		)
		return nil
	})
	if captureErr != nil {
		t.Fatalf("capture execute streams: %v", captureErr)
	}
	if stdout != "" {
		t.Fatalf("expected no stdout fallback for interactive timeout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr fallback for interactive timeout, got %q", stderr)
	}
	if got := result.Status; got != attemptStatusTimeout {
		t.Fatalf("expected timeout status, got %s", got)
	}
	if result.Failure == nil || !strings.Contains(result.Failure.Err.Error(), "activity timeout") {
		t.Fatalf("expected activity-timeout failure, got %#v", result.Failure)
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
	result := executeJobWithTimeout(
		context.Background(),
		&config{
			IDE:                    model.IDECodex,
			Model:                  "test-model",
			ReasoningEffort:        "medium",
			RetryBackoffMultiplier: 2,
		},
		&job,
		tmpDir,
		true,
		0,
		time.Second,
		nil,
		&aggregate,
		&aggregateMu,
		nil,
	)

	if got := result.Status; got != attemptStatusSuccess {
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
	execCtx := &jobExecutionContext{ctx: context.Background(), ui: ui}

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
		ctx:   context.Background(),
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
		ctx:   context.Background(),
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
	execCtx := &jobExecutionContext{ctx: context.Background()}
	lifecycle := newJobLifecycle(0, &job{
		CodeFiles: []string{"task_01"},
		OutLog:    "task_01.out.log",
		ErrLog:    "task_01.err.log",
	}, execCtx)

	lifecycle.markGiveUp(failInfo{
		CodeFile: "task_01",
		ExitCode: 23,
		OutLog:   "task_01.out.log",
		ErrLog:   "task_01.err.log",
		Err:      errors.New("boom"),
	})

	if got := lifecycle.state; got != jobPhaseFailed {
		t.Fatalf("expected failed state, got %s", got)
	}
	if got := atomic.LoadInt32(&execCtx.failed); got != 1 {
		t.Fatalf("expected failed counter 1, got %d", got)
	}
	if len(execCtx.failures) != 1 || execCtx.failures[0].ExitCode != 23 {
		t.Fatalf("expected recorded failure, got %#v", execCtx.failures)
	}
}

func TestJobLifecycleEmitsStartedRetryAndCompletedEvents(t *testing.T) {
	runID, runJournal, eventsCh, cleanup := openRuntimeEventCapture(t)
	defer cleanup()

	execCtx := &jobExecutionContext{
		ctx: context.Background(),
		cfg: &config{
			MaxRetries: 1,
			RunArtifacts: model.RunArtifacts{
				RunID: runID,
			},
		},
		journal: runJournal,
	}
	lifecycle := newJobLifecycle(2, &job{
		CodeFiles: []string{"task_03"},
		OutLog:    "task_03.out.log",
		ErrLog:    "task_03.err.log",
	}, execCtx)

	lifecycle.startAttempt(1, 2, time.Second)
	lifecycle.markRetry(failInfo{
		CodeFile: "task_03",
		ExitCode: 75,
		OutLog:   "task_03.out.log",
		ErrLog:   "task_03.err.log",
		Err:      errors.New("retry me"),
	}, 2, 2)
	lifecycle.startAttempt(2, 2, time.Second)
	lifecycle.markSuccess()

	events := collectRuntimeEvents(t, eventsCh, 3)
	gotKinds := []eventspkg.EventKind{events[0].Kind, events[1].Kind, events[2].Kind}
	wantKinds := []eventspkg.EventKind{
		eventspkg.EventKindJobStarted,
		eventspkg.EventKindJobRetryScheduled,
		eventspkg.EventKindJobCompleted,
	}
	for i := range wantKinds {
		if gotKinds[i] != wantKinds[i] {
			t.Fatalf("unexpected job lifecycle event order: got %v want %v", gotKinds, wantKinds)
		}
	}
}

func TestJobLifecycleEmitsFailedEvent(t *testing.T) {
	runID, runJournal, eventsCh, cleanup := openRuntimeEventCapture(t)
	defer cleanup()

	execCtx := &jobExecutionContext{
		ctx: context.Background(),
		cfg: &config{
			MaxRetries: 2,
			RunArtifacts: model.RunArtifacts{
				RunID: runID,
			},
		},
		journal: runJournal,
	}
	lifecycle := newJobLifecycle(0, &job{
		CodeFiles: []string{"task_01"},
		OutLog:    "task_01.out.log",
		ErrLog:    "task_01.err.log",
	}, execCtx)

	lifecycle.startAttempt(1, 3, time.Second)
	lifecycle.markGiveUp(failInfo{
		CodeFile: "task_01",
		ExitCode: 23,
		OutLog:   "task_01.out.log",
		ErrLog:   "task_01.err.log",
		Err:      errors.New("boom"),
	})

	events := collectRuntimeEvents(t, eventsCh, 2)
	if got := events[0].Kind; got != eventspkg.EventKindJobStarted {
		t.Fatalf("expected job.started event, got %s", got)
	}
	if got := events[1].Kind; got != eventspkg.EventKindJobFailed {
		t.Fatalf("expected job.failed event, got %s", got)
	}
}

func TestHandleNilExecutionReturnsSetupFailure(t *testing.T) {
	result := handleNilExecution(&job{
		CodeFiles: []string{"task_01"},
		OutLog:    "task_01.out.log",
		ErrLog:    "task_01.err.log",
	}, 0, true)

	if got := result.Status; got != attemptStatusSetupFailed {
		t.Fatalf("expected setup failure status, got %s", got)
	}
	if result.Failure == nil ||
		!strings.Contains(result.Failure.Err.Error(), "failed to set up ACP session execution") {
		t.Fatalf("unexpected failure payload: %#v", result.Failure)
	}
}

func TestRetryableSetupFailureMatchesExpectedStages(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "retryable start process",
			err:  &agent.SessionSetupError{Stage: agent.SessionSetupStageStartProcess, Err: errors.New("boom")},
			want: true,
		},
		{
			name: "retryable initialize",
			err:  &agent.SessionSetupError{Stage: agent.SessionSetupStageInitialize, Err: errors.New("boom")},
			want: true,
		},
		{
			name: "retryable new session",
			err:  &agent.SessionSetupError{Stage: agent.SessionSetupStageNewSession, Err: errors.New("boom")},
			want: true,
		},
		{
			name: "non retryable set model",
			err:  &agent.SessionSetupError{Stage: agent.SessionSetupStageSetModel, Err: errors.New("boom")},
			want: false,
		},
		{
			name: "non retryable plain error",
			err:  errors.New("plain"),
			want: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := retryableSetupFailure(tc.err); got != tc.want {
				t.Fatalf("unexpected retryableSetupFailure result: got %v want %v", got, tc.want)
			}
		})
	}
}

func TestRecordFailureWithContextAddsFailure(t *testing.T) {
	var failures []failInfo
	job := &job{
		CodeFiles: []string{"task_01"},
		OutLog:    "task_01.out.log",
		ErrLog:    "task_01.err.log",
	}
	got := recordFailureWithContext(nil, job, &failures, errors.New("boom"), 77)
	if got.ExitCode != 77 || got.CodeFile != "task_01" {
		t.Fatalf("unexpected failure info: %#v", got)
	}
	if len(failures) != 1 || failures[0].ExitCode != 77 {
		t.Fatalf("expected failure to be recorded, got %#v", failures)
	}
}

type fakeACPClient struct {
	createSessionFn func(context.Context, agent.SessionRequest) (agent.Session, error)
	resumeSessionFn func(context.Context, agent.ResumeSessionRequest) (agent.Session, error)
	supportsLoad    bool
	createCalls     atomic.Int32
	resumeCalls     atomic.Int32
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

func (c *fakeACPClient) ResumeSession(ctx context.Context, req agent.ResumeSessionRequest) (agent.Session, error) {
	c.resumeCalls.Add(1)
	if c.resumeSessionFn == nil {
		return nil, errors.New("missing fake resume session factory")
	}
	return c.resumeSessionFn(ctx, req)
}

func (c *fakeACPClient) SupportsLoadSession() bool {
	return c.supportsLoad
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

func (f *fakeLifecycleUISession) Enqueue(msg any) {
	if f.eventsCh == nil {
		return
	}
	f.eventsCh <- msg
}

func (f *fakeLifecycleUISession) SetQuitHandler(func(uiQuitRequest)) {}

func (f *fakeLifecycleUISession) CloseEvents() {
	f.closeEventsCalls++
}

func (f *fakeLifecycleUISession) Shutdown() {
	f.shutdownCalls++
}

func (f *fakeLifecycleUISession) Wait() error {
	f.waitCalls++
	return nil
}

func (s *fakeACPSession) ID() string {
	return s.id
}

func (s *fakeACPSession) Identity() agent.SessionIdentity {
	return agent.SessionIdentity{ACPSessionID: s.id}
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

func (s *fakeACPSession) SlowPublishes() uint64 {
	return 0
}

func (s *fakeACPSession) DroppedUpdates() uint64 {
	return 0
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
	restore := acpshared.SwapNewAgentClientForTest(func(context.Context, agent.ClientConfig) (agent.Client, error) {
		mu.Lock()
		defer mu.Unlock()
		if index >= len(clients) {
			return nil, fmt.Errorf("no fake ACP client configured for attempt %d", index+1)
		}
		client := clients[index]
		index++
		return client, nil
	})
	t.Cleanup(func() {
		restore()
	})
}

func newTestACPJob(tmpDir string) job {
	return job{
		CodeFiles:    []string{"task_01"},
		Groups:       map[string][]model.IssueEntry{},
		SafeName:     "task_01",
		Prompt:       []byte("finish the task"),
		SystemPrompt: "workflow memory",
		OutLog:       filepath.Join(tmpDir, "task_01.out.log"),
		ErrLog:       filepath.Join(tmpDir, "task_01.err.log"),
		OutBuffer:    newLineBuffer(0),
		ErrBuffer:    newLineBuffer(0),
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
