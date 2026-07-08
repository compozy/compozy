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
	"github.com/compozy/compozy/internal/core/run/journal"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
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
	}, nil)
	if err != nil {
		t.Fatalf("execute dry run: %v", err)
	}
}

func TestExecuteUsesWorkspaceRootForWorkflowSessionCWD(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceRoot := t.TempDir()
	tasksDir := filepath.Join(workspaceRoot, model.TasksBaseDir(), "demo")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir: %v", err)
	}
	writeRunTaskFile(t, tasksDir, "task_01.md", "pending")
	taskPath := filepath.Join(tasksDir, "task_01.md")
	taskContent, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read task file: %v", err)
	}
	processCWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get process cwd: %v", err)
	}
	if filepath.Clean(workspaceRoot) == filepath.Clean(processCWD) {
		t.Fatalf("test requires workspace root %q to differ from process cwd %q", workspaceRoot, processCWD)
	}

	workingDirCh := make(chan string, 1)
	client := newFakeACPClient(func(_ context.Context, req agent.SessionRequest) (agent.Session, error) {
		workingDirCh <- req.WorkingDir
		session := newFakeACPSession("sess-workspace-cwd")
		go session.finish(nil)
		return session, nil
	})
	installFakeACPClients(t, client)

	job := model.Job{
		CodeFiles: []string{"task_01"},
		Groups: map[string][]model.IssueEntry{
			"task_01": {{
				Name:     "task_01.md",
				AbsPath:  taskPath,
				Content:  string(taskContent),
				CodeFile: "task_01",
			}},
		},
		SafeName:      "task_01",
		Prompt:        []byte("finish the task"),
		OutLog:        filepath.Join(tmpDir, "task_01.out.log"),
		ErrLog:        filepath.Join(tmpDir, "task_01.err.log"),
		OutPromptPath: filepath.Join(tmpDir, "task_01.prompt.md"),
	}
	err = Execute(
		context.Background(),
		[]model.Job{job},
		model.NewRunArtifacts(tmpDir, "workspace-cwd"),
		nil,
		nil,
		&model.RuntimeConfig{
			WorkspaceRoot:          workspaceRoot,
			Name:                   "demo",
			TasksDir:               tasksDir,
			Mode:                   model.ExecutionModePRDTasks,
			IDE:                    model.IDECodex,
			Concurrent:             1,
			ReasoningEffort:        "medium",
			RetryBackoffMultiplier: 2,
			DaemonOwned:            true,
		},
		nil,
	)
	if err != nil {
		t.Fatalf("execute workflow: %v", err)
	}

	select {
	case got := <-workingDirCh:
		if filepath.Clean(got) != filepath.Clean(workspaceRoot) {
			t.Fatalf("session working dir = %q, want workspace root %q", got, workspaceRoot)
		}
		if filepath.Clean(got) == filepath.Clean(processCWD) {
			t.Fatalf("session working dir used daemon process cwd %q", got)
		}
	default:
		t.Fatal("expected fake ACP client to receive a session request")
	}
}

func TestResolveWorkflowSessionCWDFallsBackToProcessCWDWithoutWorkspaceRoot(t *testing.T) {
	got, err := resolveWorkflowSessionCWD(&config{})
	if err != nil {
		t.Fatalf("resolve workflow session cwd: %v", err)
	}
	want, err := os.Getwd()
	if err != nil {
		t.Fatalf("get process cwd: %v", err)
	}
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("workflow session cwd = %q, want process cwd %q", got, want)
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
	if got := firstClient.setModelCalls.Load(); got != 1 {
		t.Fatalf("expected first client SetSessionModel once, got %d", got)
	}
	if got := secondClient.setModelCalls.Load(); got != 1 {
		t.Fatalf("expected second client SetSessionModel once, got %d", got)
	}
	secondClient.setModelMu.Lock()
	gotModelID := secondClient.lastSetModelID
	secondClient.setModelMu.Unlock()
	if gotModelID != "test-model" {
		t.Fatalf("expected second client SetSessionModel model %q, got %q", "test-model", gotModelID)
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

func TestJobRunnerRetriesActivityTimeoutThenSucceeds(t *testing.T) {
	tmpDir := t.TempDir()
	firstClient := newFakeACPClient(func(ctx context.Context, _ agent.SessionRequest) (agent.Session, error) {
		session := newFakeACPSession("sess-timeout-first")
		go func() {
			<-ctx.Done()
			session.finish(context.Cause(ctx))
		}()
		return session, nil
	})
	secondClientErrCh := make(chan error, 1)
	secondClient := newFakeACPClient(func(_ context.Context, _ agent.SessionRequest) (agent.Session, error) {
		session := newFakeACPSession("sess-timeout-retry")
		go func() {
			textBlock, err := model.NewContentBlock(model.TextBlock{Text: "retry after timeout succeeded"})
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

	runID, runJournal, eventsCh, cleanup := openRuntimeEventCapture(t)
	defer cleanup()

	job := newTestACPJob(tmpDir)
	execCtx := &jobExecutionContext{
		ctx: context.Background(),
		cfg: &config{
			IDE:                    model.IDECodex,
			Model:                  "test-model",
			ReasoningEffort:        "medium",
			MaxRetries:             1,
			RetryBackoffMultiplier: 2,
			Timeout:                25 * time.Millisecond,
			// The activity watchdog keys off the resolved stall policy's idle
			// window, independent of Timeout; arm it with a tiny idle window so
			// the silent first attempt trips it.
			Stall: model.StallPolicy{Enabled: true, IdleTimeout: 25 * time.Millisecond},
			RunArtifacts: model.RunArtifacts{
				RunID: runID,
			},
		},
		cwd:     tmpDir,
		journal: runJournal,
	}

	runner := newJobRunner(0, &job, execCtx)
	runner.run(context.Background())

	if got := runner.lifecycle.state; got != jobPhaseSucceeded {
		t.Fatalf("expected succeeded lifecycle state after retry, got %s", got)
	}
	if got := firstClient.createCalls.Load(); got != 1 {
		t.Fatalf("expected first timeout attempt once, got %d", got)
	}
	if got := secondClient.createCalls.Load(); got != 1 {
		t.Fatalf("expected one successful retry attempt, got %d", got)
	}
	if got := firstClient.setModelCalls.Load(); got != 1 {
		t.Fatalf("expected first client SetSessionModel once, got %d", got)
	}
	if got := secondClient.setModelCalls.Load(); got != 1 {
		t.Fatalf("expected second client SetSessionModel once, got %d", got)
	}
	if got := atomic.LoadInt32(&execCtx.failed); got != 0 {
		t.Fatalf("expected no failed jobs after timeout retry, got %d", got)
	}

	var retryEvent eventspkg.Event
	var sawRetry bool
	var sawCompleted bool
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	for !sawRetry || !sawCompleted {
		select {
		case ev := <-eventsCh:
			switch ev.Kind {
			case eventspkg.EventKindJobRetryScheduled:
				retryEvent = ev
				sawRetry = true
			case eventspkg.EventKindJobCompleted:
				sawCompleted = true
			}
		case <-deadline.C:
			t.Fatalf(
				"timed out waiting for retry and completion events: retry=%t completed=%t",
				sawRetry,
				sawCompleted,
			)
		}
	}
	var retryPayload kinds.JobRetryScheduledPayload
	decodeRuntimeEventPayload(t, retryEvent, &retryPayload)
	if !strings.Contains(retryPayload.Reason, "activity timeout") {
		t.Fatalf("expected retry reason to mention activity timeout, got %#v", retryPayload)
	}
	if err := waitForAsyncTestError(t, secondClientErrCh); err != nil {
		t.Fatalf("new content block: %v", err)
	}
}

func TestACPSetupPhaseScenarios(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "Should retry a setup error that arrives after the init timeout and then succeed",
			run: func(t *testing.T) {
				tmpDir := t.TempDir()
				firstClient := newFakeACPClient(
					func(ctx context.Context, req agent.SessionRequest) (agent.Session, error) {
						setupCtx := fakeSetupContext(ctx, req)
						<-setupCtx.Done()
						return nil, &agent.SessionSetupError{
							Stage: agent.SessionSetupStageNewSession,
							Err:   &agent.SessionError{Code: -32603, Message: "Internal error"},
						}
					},
				)
				secondClient := newFakeACPClient(
					func(_ context.Context, _ agent.SessionRequest) (agent.Session, error) {
						session := newFakeACPSession("sess-setup-timeout-retry")
						go session.finish(nil)
						return session, nil
					},
				)
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
						Timeout:                25 * time.Millisecond,
					},
					cwd: tmpDir,
				}

				runner := newJobRunner(0, &job, execCtx)
				runner.run(context.Background())

				if got := runner.lifecycle.state; got != jobPhaseSucceeded {
					t.Fatalf("expected succeeded lifecycle state after setup timeout retry, got %s", got)
				}
				if got := firstClient.createCalls.Load(); got != 1 {
					t.Fatalf("expected first setup timeout attempt once, got %d", got)
				}
				if got := secondClient.createCalls.Load(); got != 1 {
					t.Fatalf("expected one successful retry attempt, got %d", got)
				}
				if got := atomic.LoadInt32(&execCtx.failed); got != 0 {
					t.Fatalf("expected no failed jobs after setup timeout retry, got %d", got)
				}
			},
		},
		{
			name: "Should classify a setup error after the init timeout as a retryable init timeout",
			run: func(t *testing.T) {
				tmpDir := t.TempDir()
				blockingSetupClient := newFakeACPClient(
					func(ctx context.Context, req agent.SessionRequest) (agent.Session, error) {
						setupCtx := fakeSetupContext(ctx, req)
						<-setupCtx.Done()
						return nil, &agent.SessionSetupError{
							Stage: agent.SessionSetupStageNewSession,
							Err:   &agent.SessionError{Code: -32603, Message: "Internal error"},
						}
					},
				)
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
					t.Fatalf("expected setup error after init timeout to be timeout, got %s", got)
				}
				if result.ExitCode != -2 {
					t.Fatalf("expected timeout exit code -2, got %d", result.ExitCode)
				}
				if !result.Retryable {
					t.Fatal("expected setup init timeout to be retryable")
				}
				if result.Failure == nil || !acpshared.IsInitTimeout(result.Failure.Err) {
					t.Fatalf("expected typed init-timeout failure, got %#v", result.Failure)
				}
			},
		},
		{
			name: "Should arm the activity watchdog only after the session is established",
			run: func(t *testing.T) {
				tmpDir := t.TempDir()
				activityTimeout := 100 * time.Millisecond
				setupDelay := activityTimeout + 20*time.Millisecond
				slowSetupClient := newFakeACPClient(
					func(ctx context.Context, req agent.SessionRequest) (agent.Session, error) {
						setupCtx := fakeSetupContext(ctx, req)
						timer := time.NewTimer(setupDelay)
						defer timer.Stop()
						select {
						case <-timer.C:
						case <-setupCtx.Done():
							return nil, context.Cause(setupCtx)
						}
						session := newFakeACPSession("sess-slow-setup")
						go func() {
							textBlock, err := model.NewContentBlock(model.TextBlock{Text: "slow setup completed"})
							if err != nil {
								session.finish(err)
								return
							}
							session.publish(model.SessionUpdate{
								Kind:   model.UpdateKindAgentMessageChunk,
								Blocks: []model.ContentBlock{textBlock},
								Status: model.StatusRunning,
							})
							session.finish(nil)
						}()
						return session, nil
					},
				)
				installFakeACPClients(t, slowSetupClient)

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
					activityTimeout,
					nil,
					nil,
					nil,
					nil,
				)

				if got := result.Status; got != attemptStatusSuccess {
					t.Fatalf("expected slow setup to succeed, got %s (%#v)", got, result.Failure)
				}
				if got := slowSetupClient.closeCalls.Load(); got != 1 {
					t.Fatalf("expected slow setup client to close once, got %d", got)
				}
			},
		},
		{
			name: "Should return a typed init timeout when setup never establishes the session",
			run: func(t *testing.T) {
				tmpDir := t.TempDir()
				blockingSetupClient := newFakeACPClient(
					func(ctx context.Context, req agent.SessionRequest) (agent.Session, error) {
						setupCtx := fakeSetupContext(ctx, req)
						<-setupCtx.Done()
						return nil, context.Cause(setupCtx)
					},
				)
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
					t.Fatalf("expected setup init timeout status, got %s (%#v)", got, result.Failure)
				}
				if result.ExitCode != -2 {
					t.Fatalf("expected timeout exit code -2, got %d", result.ExitCode)
				}
				if !result.Retryable {
					t.Fatal("expected setup init timeout to be retryable")
				}
				if result.Failure == nil || !acpshared.IsInitTimeout(result.Failure.Err) {
					t.Fatalf("expected typed init-timeout failure, got %#v", result.Failure)
				}
				if acpshared.IsActivityTimeout(result.Failure.Err) {
					t.Fatalf("expected init timeout not to masquerade as activity timeout: %#v", result.Failure.Err)
				}
				if got := blockingSetupClient.closeCalls.Load(); got != 1 {
					t.Fatalf("expected blocked setup client to be closed, got %d closes", got)
				}
			},
		},
		{
			name: "Should surface cancellation distinctly from setup timeouts",
			run: func(t *testing.T) {
				tmpDir := t.TempDir()
				started := make(chan struct{})
				blockingSetupClient := newFakeACPClient(
					func(ctx context.Context, req agent.SessionRequest) (agent.Session, error) {
						setupCtx := fakeSetupContext(ctx, req)
						close(started)
						<-setupCtx.Done()
						return nil, context.Cause(setupCtx)
					},
				)
				installFakeACPClients(t, blockingSetupClient)

				job := newTestACPJob(tmpDir)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				resultCh := make(chan jobAttemptResult, 1)
				go func() {
					resultCh <- executeJobWithTimeout(
						ctx,
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
						time.Second,
						nil,
						nil,
						nil,
						nil,
					)
				}()

				select {
				case <-started:
				case <-time.After(2 * time.Second):
					t.Fatal("timed out waiting for setup to start")
				}
				cancel()

				var result jobAttemptResult
				select {
				case result = <-resultCh:
				case <-time.After(2 * time.Second):
					t.Fatal("timed out waiting for setup cancellation")
				}
				if got := result.Status; got != attemptStatusCanceled {
					t.Fatalf("expected setup cancellation status, got %s (%#v)", got, result.Failure)
				}
				if result.ExitCode != -1 {
					t.Fatalf("expected canceled exit code -1, got %d", result.ExitCode)
				}
				if result.Failure == nil {
					t.Fatal("expected cancellation failure info")
				}
				if acpshared.IsInitTimeout(result.Failure.Err) || acpshared.IsActivityTimeout(result.Failure.Err) {
					t.Fatalf("expected cancellation not timeout, got %#v", result.Failure.Err)
				}
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t)
		})
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
	if got := firstClient.setModelCalls.Load(); got != 0 {
		t.Fatalf("expected no SetSessionModel on failed client, got %d", got)
	}
	if got := secondClient.setModelCalls.Load(); got != 1 {
		t.Fatalf("expected retry client SetSessionModel once, got %d", got)
	}
	if got := atomic.LoadInt32(&execCtx.failed); got != 0 {
		t.Fatalf("expected no failed jobs, got %d", got)
	}
}

func TestJobRunnerDoesNotRetryNonRetryableACPSetupFailure(t *testing.T) {
	tmpDir := t.TempDir()
	firstClient := newFakeACPClient(func(context.Context, agent.SessionRequest) (agent.Session, error) {
		return nil, &agent.SessionSetupError{
			Stage: agent.SessionSetupStageSetMode,
			Err:   errors.New("invalid session mode override"),
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
	if got := firstClient.setModelCalls.Load(); got != 0 {
		t.Fatalf("expected no SetSessionModel on non-retryable failure, got %d", got)
	}
	if got := atomic.LoadInt32(&execCtx.failed); got != 1 {
		t.Fatalf("expected failed jobs counter to be 1, got %d", got)
	}
}

func TestJobExecutionContextLaunchWorkersStopsAfterAuthenticationSetupFailure(t *testing.T) {
	t.Run("Should stop remaining jobs after authentication setup failure", func(t *testing.T) {
		tmpDir := t.TempDir()
		authErr := &agent.SessionSetupError{
			Stage: agent.SessionSetupStageNewSession,
			Err: &agent.AuthenticationRequiredError{
				Err: &agent.SessionError{Code: -32000, Message: "Authentication required"},
			},
		}
		firstClient := newFakeACPClient(func(context.Context, agent.SessionRequest) (agent.Session, error) {
			return nil, authErr
		})
		secondClient := newFakeACPClient(func(_ context.Context, _ agent.SessionRequest) (agent.Session, error) {
			session := newFakeACPSession("sess-should-not-run")
			go session.finish(nil)
			return session, nil
		})
		installFakeACPClients(t, firstClient, secondClient)

		jobs := []job{
			newTestACPJob(tmpDir),
			newTestACPJob(tmpDir),
		}
		jobs[1].SafeName = "task_02"
		jobs[1].CodeFiles = []string{"task_02"}
		jobs[1].OutLog = filepath.Join(tmpDir, "task_02.out.log")
		jobs[1].ErrLog = filepath.Join(tmpDir, "task_02.err.log")
		execCtx := &jobExecutionContext{
			ctx: context.Background(),
			cfg: &config{
				IDE:                    model.IDECursor,
				Model:                  "test-model",
				ReasoningEffort:        "medium",
				Concurrent:             1,
				MaxRetries:             3,
				RetryBackoffMultiplier: 2,
				Timeout:                time.Second,
			},
			jobs:  jobs,
			total: len(jobs),
			cwd:   tmpDir,
			sem:   make(chan struct{}, 1),
		}
		jobCtx, cancel := context.WithCancelCause(context.Background())
		defer cancel(nil)
		execCtx.cancelJobs = cancel

		execCtx.launchWorkers(jobCtx)
		select {
		case <-execCtx.waitChannel():
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for auth fail-fast execution")
		}

		if got := firstClient.createCalls.Load(); got != 1 {
			t.Fatalf("expected authentication setup attempt once, got %d", got)
		}
		if got := secondClient.createCalls.Load(); got != 0 {
			t.Fatalf("expected later job not to create an ACP session after auth failure, got %d", got)
		}
		if got := jobs[0].Status; got != runStatusFailed {
			t.Fatalf("expected first job failed status, got %q", got)
		}
		if got := jobs[1].Status; got != runStatusCanceled {
			t.Fatalf("expected second job canceled status after fail-fast, got %q", got)
		}
		for _, want := range []string{
			"cursor-agent is not authenticated",
			"Run 'cursor-agent login' and retry",
			"Authentication required",
		} {
			if !strings.Contains(jobs[0].Failure, want) {
				t.Fatalf("first job failure %q does not contain %q", jobs[0].Failure, want)
			}
		}
		if !agent.IsAuthenticationRequired(context.Cause(jobCtx)) {
			t.Fatalf("expected auth failure cancel cause, got %v", context.Cause(jobCtx))
		}
		errLog, err := os.ReadFile(jobs[0].ErrLog)
		if err != nil {
			t.Fatalf("read first job err log: %v", err)
		}
		if !strings.Contains(string(errLog), "cursor-agent is not authenticated") {
			t.Fatalf("expected actionable auth message in err log, got %q", string(errLog))
		}
	})
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
	if got := successClient.setModelCalls.Load(); got != 1 {
		t.Fatalf("expected SetSessionModel once on success, got %d", got)
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
	if got := cancelClient.setModelCalls.Load(); got != 1 {
		t.Fatalf("expected SetSessionModel once before cancellation, got %d", got)
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
			// Arm the activity watchdog via the stall policy idle window; the
			// 25ms timeout arg now only drives the init timeout.
			Stall: model.StallPolicy{Enabled: true, IdleTimeout: 25 * time.Millisecond},
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
	if !result.Retryable {
		t.Fatal("expected post-init activity timeout to be retryable")
	}
	if got := timeoutClient.closeCalls.Load(); got != 1 {
		t.Fatalf("expected client close to run as timeout backstop, got %d closes", got)
	}
	if got := timeoutClient.setModelCalls.Load(); got != 1 {
		t.Fatalf("expected SetSessionModel once before timeout, got %d", got)
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
	if got := activeClient.setModelCalls.Load(); got != 1 {
		t.Fatalf("expected SetSessionModel once on successful session, got %d", got)
	}
}

func TestExecuteJobWithTimeoutInteractiveSetupTimeoutScenarios(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "Should classify a blocked setup as an init timeout",
			run: func(t *testing.T) {
				tmpDir := t.TempDir()
				blockingSetupClient := newFakeACPClient(
					func(ctx context.Context, req agent.SessionRequest) (agent.Session, error) {
						setupCtx := fakeSetupContext(ctx, req)
						<-setupCtx.Done()
						return nil, context.Cause(setupCtx)
					},
				)
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
				if result.Failure == nil || !acpshared.IsInitTimeout(result.Failure.Err) {
					t.Fatalf("expected typed init-timeout failure, got %#v", result.Failure)
				}
				if got := blockingSetupClient.closeCalls.Load(); got != 1 {
					t.Fatalf("expected blocked setup client to be closed, got %d closes", got)
				}
				if got := blockingSetupClient.setModelCalls.Load(); got != 0 {
					t.Fatalf("expected no SetSessionModel when setup times out, got %d", got)
				}
			},
		},
		{
			name: "Should suppress human fallback output for interactive setup timeouts",
			run: func(t *testing.T) {
				tmpDir := t.TempDir()
				blockingSetupClient := newFakeACPClient(
					func(ctx context.Context, req agent.SessionRequest) (agent.Session, error) {
						setupCtx := fakeSetupContext(ctx, req)
						<-setupCtx.Done()
						return nil, context.Cause(setupCtx)
					},
				)
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
				if result.Failure == nil || !acpshared.IsInitTimeout(result.Failure.Err) {
					t.Fatalf("expected typed init-timeout failure, got %#v", result.Failure)
				}
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t)
		})
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
	if ui.closeEventsCalls != 0 || ui.waitCalls != 1 {
		t.Fatalf(
			"expected awaitUIAfterCompletion to keep events open and wait once, got close=%d wait=%d",
			ui.closeEventsCalls,
			ui.waitCalls,
		)
	}

	if err := execCtx.shutdownUI(); err != nil {
		t.Fatalf("shutdownUI: %v", err)
	}
	if ui.closeEventsCalls != 1 || ui.shutdownCalls != 1 || ui.waitCalls != 2 {
		t.Fatalf(
			"expected shutdownUI to close events, request shutdown, and wait again, got close=%d shutdown=%d wait=%d",
			ui.closeEventsCalls,
			ui.shutdownCalls,
			ui.waitCalls,
		)
	}

	execCtx.cleanup()
	if ui.closeEventsCalls != 2 || ui.shutdownCalls != 2 || ui.waitCalls != 3 {
		t.Fatalf(
			"expected cleanup to rerun the shutdown path, got close=%d shutdown=%d wait=%d",
			ui.closeEventsCalls,
			ui.shutdownCalls,
			ui.waitCalls,
		)
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
	var payload kinds.JobFailedPayload
	decodeRuntimeEventPayload(t, events[1], &payload)
	if payload.Error != "boom" {
		t.Fatalf("expected job.failed payload error to carry failure reason, got %#v", payload)
	}
}

func TestJobLifecycleTerminalEventsCarryDuration(t *testing.T) {
	newLifecycle := func(runID string, runJournal *journal.Journal) *jobLifecycle {
		execCtx := &jobExecutionContext{
			ctx:     context.Background(),
			cfg:     &config{MaxRetries: 2, RunArtifacts: model.RunArtifacts{RunID: runID}},
			journal: runJournal,
		}
		return newJobLifecycle(0, &job{CodeFiles: []string{"task_01"}}, execCtx)
	}

	t.Run("Should carry elapsed duration for a failed job", func(t *testing.T) {
		t.Parallel()
		runID, runJournal, eventsCh, cleanup := openRuntimeEventCapture(t)
		defer cleanup()
		lifecycle := newLifecycle(runID, runJournal)
		lifecycle.startAttempt(1, 3, time.Second)
		// Backdate the start so the reported elapsed is deterministically non-zero.
		lifecycle.startedAt = lifecycle.startedAt.Add(-2 * time.Second)
		lifecycle.markGiveUp(failInfo{CodeFile: "task_01", ExitCode: 23, Err: errors.New("boom")})

		events := collectRuntimeEvents(t, eventsCh, 2)
		var payload kinds.JobFailedPayload
		decodeRuntimeEventPayload(t, events[1], &payload)
		if payload.DurationMs <= 0 {
			t.Fatalf("expected failed payload to carry elapsed duration, got %d", payload.DurationMs)
		}
	})

	t.Run("Should carry elapsed duration for a canceled job", func(t *testing.T) {
		t.Parallel()
		runID, runJournal, eventsCh, cleanup := openRuntimeEventCapture(t)
		defer cleanup()
		lifecycle := newLifecycle(runID, runJournal)
		lifecycle.startAttempt(1, 1, time.Second)
		lifecycle.startedAt = lifecycle.startedAt.Add(-2 * time.Second)
		lifecycle.markCanceled(exitCodeCanceled)

		events := collectRuntimeEvents(t, eventsCh, 2)
		if got := events[1].Kind; got != eventspkg.EventKindJobCancelled {
			t.Fatalf("expected job.cancelled event, got %s", got)
		}
		var payload kinds.JobCancelledPayload
		decodeRuntimeEventPayload(t, events[1], &payload)
		if payload.DurationMs <= 0 {
			t.Fatalf("expected canceled payload to carry elapsed duration, got %d", payload.DurationMs)
		}
	})

	t.Run("Should report zero duration when terminated before any attempt", func(t *testing.T) {
		t.Parallel()
		runID, runJournal, eventsCh, cleanup := openRuntimeEventCapture(t)
		defer cleanup()
		lifecycle := newLifecycle(runID, runJournal)
		// No startAttempt: startedAt is the zero time, so elapsedMs must report 0
		// rather than a bogus duration measured from Go's year-1 zero time.
		lifecycle.markGiveUp(failInfo{CodeFile: "task_01", ExitCode: 1, Err: errors.New("early")})

		events := collectRuntimeEvents(t, eventsCh, 1)
		var payload kinds.JobFailedPayload
		decodeRuntimeEventPayload(t, events[0], &payload)
		if payload.DurationMs != 0 {
			t.Fatalf("expected zero duration when no attempt started, got %d", payload.DurationMs)
		}
	})
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
			name: "non retryable authentication required",
			err: &agent.SessionSetupError{
				Stage: agent.SessionSetupStageNewSession,
				Err: &agent.AuthenticationRequiredError{
					Err: &agent.SessionError{Code: -32000, Message: "Authentication required"},
				},
			},
			want: false,
		},
		{
			name: "non retryable set mode",
			err:  &agent.SessionSetupError{Stage: agent.SessionSetupStageSetMode, Err: errors.New("boom")},
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
	promptSessionFn func(context.Context, agent.PromptSessionRequest) (agent.Session, error)
	supportsLoad    bool
	createCalls     atomic.Int32
	resumeCalls     atomic.Int32
	promptCalls     atomic.Int32
	cancelCalls     atomic.Int32
	setModelCalls   atomic.Int32
	closeCalls      atomic.Int32
	killCalls       atomic.Int32

	setModelMu         sync.Mutex
	lastSetModelSessID string
	lastSetModelID     string
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

func (c *fakeACPClient) CancelSession(context.Context, string) error {
	c.cancelCalls.Add(1)
	return nil
}

func (c *fakeACPClient) SetSessionModel(_ context.Context, sessionID string, modelID string) error {
	c.setModelCalls.Add(1)
	c.setModelMu.Lock()
	c.lastSetModelSessID = sessionID
	c.lastSetModelID = modelID
	c.setModelMu.Unlock()
	return nil
}

func (c *fakeACPClient) PromptSession(ctx context.Context, req agent.PromptSessionRequest) (agent.Session, error) {
	c.promptCalls.Add(1)
	if c.promptSessionFn == nil {
		return nil, errors.New("missing fake prompt session factory")
	}
	return c.promptSessionFn(ctx, req)
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

func (f *fakeLifecycleUISession) SetJobControlHandler(
	func(context.Context, uiJobControlRequest) (jobControlResponse, error),
) {
}

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

func fakeSetupContext(ctx context.Context, req agent.SessionRequest) context.Context {
	if req.SetupContext != nil {
		return req.SetupContext
	}
	return ctx
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
