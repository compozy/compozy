package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	acp "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/plan"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
	"github.com/compozy/compozy/pkg/compozy/runs"
)

var captureExecuteStreamsMu sync.Mutex

func TestExecuteJobWithTimeoutACPFullPipelineRoutesTypedBlocks(t *testing.T) {
	tmpDir := t.TempDir()
	timeout := 3 * time.Second
	installACPHelperOnPath(t, []runACPHelperScenario{{
		ExpectedPromptContains: "finish the task",
		Updates: []acp.SessionUpdate{
			acp.UpdateAgentMessageText("hello from ACP"),
			acp.StartReadToolCall(acp.ToolCallId("tool-1"), "Reading README.md", "README.md"),
			acp.UpdateToolCall(
				acp.ToolCallId("tool-1"),
				acp.WithUpdateStatus(acp.ToolCallStatusCompleted),
				acp.WithUpdateContent([]acp.ToolCallContent{
					acp.ToolContent(acp.TextBlock("README contents")),
				}),
			),
		},
	}})

	job := newTestACPJob(tmpDir)
	runID, runJournal, eventsCh, cleanup := openRuntimeEventCapture(t)
	defer cleanup()

	var aggregate model.Usage
	var aggregateMu sync.Mutex
	result := executeJobWithTimeout(
		context.Background(),
		&config{
			IDE:                    model.IDECodex,
			Model:                  "",
			ReasoningEffort:        "medium",
			RetryBackoffMultiplier: 2,
			RunArtifacts:           model.RunArtifacts{RunID: runID},
		},
		&job,
		tmpDir,
		false,
		0,
		timeout,
		runJournal,
		&aggregate,
		&aggregateMu,
		nil,
	)

	if got := result.Status; got != attemptStatusSuccess {
		t.Fatalf("expected success status, got %s (%v)", got, result.Failure)
	}

	var sawText bool
	var sawToolUse bool
	var sawToolResult bool
	events := collectRuntimeEvents(t, eventsCh, 5)
	for _, event := range events {
		if event.Kind != eventspkg.EventKindSessionUpdate {
			continue
		}

		var payload kinds.SessionUpdatePayload
		decodeRuntimeEventPayload(t, event, &payload)
		for _, block := range payload.Update.Blocks {
			switch block.Type {
			case kinds.BlockText:
				sawText = true
			case kinds.BlockToolUse:
				sawToolUse = true
			case kinds.BlockToolResult:
				sawToolResult = true
			}
		}
	}
	if !sawText || !sawToolUse || !sawToolResult {
		t.Fatalf(
			"expected text/tool_use/tool_result blocks, got text=%v tool_use=%v tool_result=%v",
			sawText,
			sawToolUse,
			sawToolResult,
		)
	}

	outLog, err := os.ReadFile(job.OutLog)
	if err != nil {
		t.Fatalf("read out log: %v", err)
	}
	if !strings.Contains(string(outLog), "hello from ACP") || !strings.Contains(string(outLog), "README contents") {
		t.Fatalf("expected rendered ACP output in out log, got %q", string(outLog))
	}
}

func TestExecuteJobWithTimeoutACPCycleBlockKeepsParentSessionUsable(t *testing.T) {
	tmpDir := t.TempDir()
	timeout := 3 * time.Second
	failedStatus := acp.ToolCallStatusFailed
	installACPHelperOnPath(t, []runACPHelperScenario{{
		Updates: []acp.SessionUpdate{
			{
				ToolCall: &acp.SessionUpdateToolCall{
					ToolCallId: acp.ToolCallId("tool-1"),
					Title:      "run_agent",
					Status:     acp.ToolCallStatusPending,
					RawInput:   map[string]any{"name": "child", "input": "delegate this"},
					Meta:       map[string]any{"tool_name": "run_agent"},
				},
			},
			{
				ToolCallUpdate: &acp.SessionToolCallUpdate{
					ToolCallId: acp.ToolCallId("tool-1"),
					Status:     &failedStatus,
					Content: []acp.ToolCallContent{
						acp.ToolContent(
							acp.TextBlock(
								`{"name":"child","source":"workspace","success":false,"blocked":true,"blocked_reason":"cycle-detected","error":"nested execution blocked: cycle detected","depth":2,"max_depth":3}`,
							),
						),
					},
				},
			},
			acp.UpdateAgentMessageText("parent recovered"),
		},
	}})

	job := newTestACPJob(tmpDir)
	runID, runJournal, eventsCh, cleanup := openRuntimeEventCapture(t)
	defer cleanup()

	result := executeJobWithTimeout(
		context.Background(),
		&config{
			IDE:                    model.IDECodex,
			ReasoningEffort:        "medium",
			RetryBackoffMultiplier: 2,
			RunArtifacts:           model.RunArtifacts{RunID: runID},
		},
		&job,
		tmpDir,
		false,
		0,
		timeout,
		runJournal,
		nil,
		nil,
		nil,
	)
	if got := result.Status; got != attemptStatusSuccess {
		t.Fatalf("expected success despite blocked nested call, got %s (%#v)", got, result.Failure)
	}

	events := collectRuntimeEvents(t, eventsCh, 7)
	var lifecycle []kinds.ReusableAgentLifecyclePayload
	for _, event := range events {
		if event.Kind != eventspkg.EventKindReusableAgentLifecycle {
			continue
		}
		var payload kinds.ReusableAgentLifecyclePayload
		decodeRuntimeEventPayload(t, event, &payload)
		lifecycle = append(lifecycle, payload)
	}
	if len(lifecycle) != 2 {
		t.Fatalf("expected nested start and blocked lifecycle events, got %#v", lifecycle)
	}
	if lifecycle[0].Stage != kinds.ReusableAgentLifecycleStageNestedStarted {
		t.Fatalf("unexpected nested started payload: %#v", lifecycle[0])
	}
	if lifecycle[1].Stage != kinds.ReusableAgentLifecycleStageNestedBlocked ||
		lifecycle[1].BlockedReason != kinds.ReusableAgentBlockedReasonCycleDetected {
		t.Fatalf("unexpected nested blocked payload: %#v", lifecycle[1])
	}

	outLog, err := os.ReadFile(job.OutLog)
	if err != nil {
		t.Fatalf("read out log: %v", err)
	}
	if !strings.Contains(string(outLog), "parent recovered") {
		t.Fatalf("expected parent session to continue after blocked nested call, got %q", string(outLog))
	}
}

func TestJobRunnerACPErrorThenSuccessRetries(t *testing.T) {
	tmpDir := t.TempDir()
	installACPHelperOnPath(t,
		[]runACPHelperScenario{{
			PromptError: &runACPHelperRequestError{Code: 4901, Message: "retry me"},
		}},
		[]runACPHelperScenario{{
			Updates: []acp.SessionUpdate{acp.UpdateAgentMessageText("second try worked")},
		}},
	)

	job := newTestACPJob(tmpDir)
	execCtx := &jobExecutionContext{
		ctx: context.Background(),
		cfg: &config{
			IDE:                    model.IDECodex,
			Model:                  "",
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
	errLog, err := os.ReadFile(job.ErrLog)
	if err != nil {
		t.Fatalf("read err log: %v", err)
	}
	if !strings.Contains(string(errLog), "retry me") {
		t.Fatalf("expected first ACP error in err log, got %q", string(errLog))
	}
}

func TestExecuteJobWithTimeoutACPFailedToolCallDoesNotFailJob(t *testing.T) {
	tmpDir := t.TempDir()
	installACPHelperOnPath(t, []runACPHelperScenario{{
		Updates: []acp.SessionUpdate{
			acp.StartToolCall(
				acp.ToolCallId("tool-1"),
				"Read missing file",
				acp.WithStartRawInput(map[string]any{"path": "missing.txt"}),
			),
			acp.UpdateToolCall(
				acp.ToolCallId("tool-1"),
				acp.WithUpdateStatus(acp.ToolCallStatusFailed),
				acp.WithUpdateContent([]acp.ToolCallContent{
					acp.ToolContent(acp.TextBlock("open missing.txt: no such file")),
				}),
			),
		},
		PromptError: &runACPHelperRequestError{
			Code:    4242,
			Message: "tool call failed",
			Data:    json.RawMessage(`{"tool_call_id":"tool-1"}`),
		},
		PromptErrorAfterUpdates: true,
	}})

	job := newTestACPJob(tmpDir)
	result := executeJobWithTimeout(
		context.Background(),
		&config{
			IDE:                    model.IDECodex,
			Model:                  "",
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

	if got := result.Status; got != attemptStatusSuccess {
		t.Fatalf("expected success status, got %s (%v)", got, result.Failure)
	}

	errLog, err := os.ReadFile(job.ErrLog)
	if err != nil {
		t.Fatalf("read err log: %v", err)
	}
	if !strings.Contains(string(errLog), "open missing.txt: no such file") {
		t.Fatalf("expected tool failure details in err log, got %q", string(errLog))
	}
	if strings.Contains(string(errLog), "ACP session error:") {
		t.Fatalf("expected no terminal ACP session error in err log, got %q", string(errLog))
	}
}

func TestExecuteACPJSONModeWritesStructuredFailureResult(t *testing.T) {
	tmpDir := t.TempDir()
	installACPHelperOnPath(t, []runACPHelperScenario{{
		PromptError: &runACPHelperRequestError{Code: 4901, Message: "json failure"},
	}})

	runArtifacts := model.NewRunArtifacts(tmpDir, "exec-json-failure")
	if err := os.MkdirAll(runArtifacts.JobsDir, 0o755); err != nil {
		t.Fatalf("mkdir jobs dir: %v", err)
	}
	jobArtifacts := runArtifacts.JobArtifacts("exec")
	for _, path := range []string{jobArtifacts.PromptPath, jobArtifacts.OutLogPath, jobArtifacts.ErrLogPath} {
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatalf("seed artifact %s: %v", path, err)
		}
	}

	stdout, stderr, execErr := captureExecuteStreams(t, func() error {
		return Execute(context.Background(), []model.Job{{
			CodeFiles:     []string{"exec"},
			Groups:        map[string][]model.IssueEntry{"exec": {{Name: "exec", CodeFile: "exec"}}},
			SafeName:      "exec",
			Prompt:        []byte("finish the task"),
			SystemPrompt:  "workflow memory",
			OutPromptPath: jobArtifacts.PromptPath,
			OutLog:        jobArtifacts.OutLogPath,
			ErrLog:        jobArtifacts.ErrLogPath,
		}}, runArtifacts, nil, nil, &model.RuntimeConfig{
			IDE:                    model.IDECodex,
			Mode:                   model.ExecutionModeExec,
			OutputFormat:           model.OutputFormatJSON,
			ReasoningEffort:        "medium",
			RetryBackoffMultiplier: 2,
		}, nil)
	})
	if execErr == nil {
		t.Fatal("expected JSON execution failure")
	}
	if stderr != "" {
		t.Fatalf("expected JSON mode to suppress human stderr, got %q", stderr)
	}

	var payload executionResult
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("decode stdout json: %v\nstdout:\n%s", err, stdout)
	}
	if payload.Status != runStatusFailed {
		t.Fatalf("unexpected run status: %q", payload.Status)
	}
	if payload.Error == "" || !strings.Contains(payload.Error, "json failure") {
		t.Fatalf("unexpected run error: %q", payload.Error)
	}
	if payload.ArtifactsDir != runArtifacts.RunDir {
		t.Fatalf("unexpected artifacts dir: %q", payload.ArtifactsDir)
	}
	if payload.RunMetaPath != runArtifacts.RunMetaPath {
		t.Fatalf("unexpected run meta path: %q", payload.RunMetaPath)
	}
	if payload.ResultPath != runArtifacts.ResultPath {
		t.Fatalf("unexpected result path: %q", payload.ResultPath)
	}
	if len(payload.Jobs) != 1 || payload.Jobs[0].Status != runStatusFailed {
		t.Fatalf("unexpected job payload: %#v", payload.Jobs)
	}
	if _, err := os.Stat(payload.ResultPath); err != nil {
		t.Fatalf("expected result.json to exist: %v", err)
	}
	if _, err := os.Stat(payload.Jobs[0].StderrLogPath); err != nil {
		t.Fatalf("expected stderr log path to exist: %v", err)
	}
	if !strings.HasPrefix(payload.ResultPath, filepath.Join(tmpDir, ".compozy", "runs")) {
		t.Fatalf("expected result path under shared runs dir, got %q", payload.ResultPath)
	}
}

func TestExecuteACPJSONModeWritesStructuredSuccessResult(t *testing.T) {
	tmpDir := t.TempDir()
	installACPHelperOnPath(t, []runACPHelperScenario{{
		ExpectedPromptContains: "finish the task",
		Updates: []acp.SessionUpdate{
			acp.UpdateAgentMessageText("json success"),
		},
	}})

	runArtifacts := model.NewRunArtifacts(tmpDir, "exec-json-success")
	if err := os.MkdirAll(runArtifacts.JobsDir, 0o755); err != nil {
		t.Fatalf("mkdir jobs dir: %v", err)
	}
	jobArtifacts := runArtifacts.JobArtifacts("exec")
	for _, path := range []string{jobArtifacts.PromptPath, jobArtifacts.OutLogPath, jobArtifacts.ErrLogPath} {
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatalf("seed artifact %s: %v", path, err)
		}
	}

	stdout, stderr, execErr := captureExecuteStreams(t, func() error {
		return Execute(context.Background(), []model.Job{{
			CodeFiles:     []string{"exec"},
			Groups:        map[string][]model.IssueEntry{"exec": {{Name: "exec", CodeFile: "exec"}}},
			SafeName:      "exec",
			Prompt:        []byte("finish the task"),
			SystemPrompt:  "workflow memory",
			OutPromptPath: jobArtifacts.PromptPath,
			OutLog:        jobArtifacts.OutLogPath,
			ErrLog:        jobArtifacts.ErrLogPath,
		}}, runArtifacts, nil, nil, &model.RuntimeConfig{
			IDE:                    model.IDECodex,
			Mode:                   model.ExecutionModeExec,
			OutputFormat:           model.OutputFormatJSON,
			ReasoningEffort:        "medium",
			RetryBackoffMultiplier: 2,
		}, nil)
	})
	if execErr != nil {
		t.Fatalf("expected JSON execution success: %v\nstdout:\n%s\nstderr:\n%s", execErr, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected JSON mode to suppress human stderr, got %q", stderr)
	}

	var payload executionResult
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("decode stdout json: %v\nstdout:\n%s", err, stdout)
	}
	if payload.Status != runStatusSucceeded {
		t.Fatalf("unexpected run status: %q", payload.Status)
	}
	if payload.OutputFormat != string(model.OutputFormatJSON) {
		t.Fatalf("unexpected output format: %q", payload.OutputFormat)
	}
	if payload.ArtifactsDir != runArtifacts.RunDir {
		t.Fatalf("unexpected artifacts dir: %q", payload.ArtifactsDir)
	}
	if payload.RunMetaPath != runArtifacts.RunMetaPath {
		t.Fatalf("unexpected run meta path: %q", payload.RunMetaPath)
	}
	if payload.ResultPath != runArtifacts.ResultPath {
		t.Fatalf("unexpected result path: %q", payload.ResultPath)
	}
	if len(payload.Jobs) != 1 || payload.Jobs[0].Status != runStatusSucceeded {
		t.Fatalf("unexpected job payload: %#v", payload.Jobs)
	}
	if payload.Jobs[0].PromptPath != jobArtifacts.PromptPath {
		t.Fatalf("unexpected prompt path: %q", payload.Jobs[0].PromptPath)
	}
	for _, path := range []string{
		payload.ResultPath,
		payload.Jobs[0].PromptPath,
		payload.Jobs[0].StdoutLogPath,
		payload.Jobs[0].StderrLogPath,
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected exec artifact to exist at %s: %v", path, err)
		}
		if !strings.HasPrefix(path, filepath.Join(tmpDir, ".compozy", "runs")) {
			t.Fatalf("expected artifact path under shared runs dir, got %q", path)
		}
	}

	resultBytes, err := os.ReadFile(payload.ResultPath)
	if err != nil {
		t.Fatalf("read result.json: %v", err)
	}
	if strings.TrimSpace(stdout) != strings.TrimSpace(string(resultBytes)) {
		t.Fatalf("expected stdout JSON to match result.json\nstdout:\n%s\nresult:\n%s", stdout, string(resultBytes))
	}
}

func TestExecutePRDTasksPublishesCanonicalEventsToBusAndJournal(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, model.TasksBaseDir(), "demo")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir: %v", err)
	}
	writeRunTaskFile(t, tasksDir, "task_01.md", "pending")

	installACPHelperOnPath(t, []runACPHelperScenario{{
		Updates: []acp.SessionUpdate{
			acp.UpdateAgentMessageText("task completed"),
		},
	}})

	cfg := &model.RuntimeConfig{
		Name:                   "demo",
		WorkspaceRoot:          tmpDir,
		IDE:                    model.IDECodex,
		Mode:                   model.ExecutionModePRDTasks,
		OutputFormat:           model.OutputFormatRawJSON,
		ReasoningEffort:        "medium",
		RetryBackoffMultiplier: 2,
		Concurrent:             1,
	}
	scope, err := model.OpenRunScope(context.Background(), cfg, model.OpenRunScopeOptions{})
	if err != nil {
		t.Fatalf("OpenRunScope() error = %v", err)
	}
	prep, err := plan.Prepare(context.Background(), cfg, scope)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	defer func() {
		if err := prep.CloseJournal(context.Background()); err != nil {
			t.Fatalf("close preparation scope: %v", err)
		}
	}()
	if prep.Journal() == nil {
		t.Fatal("expected prepare to return a journal")
	}
	bus := prep.EventBus()
	if bus == nil {
		t.Fatal("expected prepare to return an event bus")
	}
	_, busCh, unsubscribe := bus.Subscribe()
	defer unsubscribe()

	if err := Execute(
		context.Background(),
		prep.Jobs,
		prep.RunArtifacts,
		prep.Journal(),
		prep.EventBus(),
		cfg,
		nil,
	); err != nil {
		t.Fatalf("execute: %v", err)
	}

	busEvents := collectRuntimeEvents(t, busCh, 10)
	wantKinds := []eventspkg.EventKind{
		eventspkg.EventKindRunStarted,
		eventspkg.EventKindJobStarted,
		eventspkg.EventKindSessionStarted,
		eventspkg.EventKindSessionUpdate,
		eventspkg.EventKindSessionUpdate,
		eventspkg.EventKindSessionCompleted,
		eventspkg.EventKindTaskFileUpdated,
		eventspkg.EventKindTaskMetadataRefreshed,
		eventspkg.EventKindJobCompleted,
		eventspkg.EventKindRunCompleted,
	}
	if got := runtimeEventKinds(busEvents); !slices.Equal(got, wantKinds) {
		t.Fatalf("unexpected bus event kinds: got %v want %v", got, wantKinds)
	}

	run, err := runs.Open(tmpDir, prep.RunArtifacts.RunID)
	if err != nil {
		t.Fatalf("open run: %v", err)
	}
	replayed := replayRuntimeEvents(t, run)
	if got := runtimeEventKinds(replayed); !slices.Equal(got, wantKinds) {
		t.Fatalf("unexpected replayed event kinds: got %v want %v", got, wantKinds)
	}
	for i := range busEvents {
		if busEvents[i].Seq != replayed[i].Seq || busEvents[i].Kind != replayed[i].Kind {
			t.Fatalf(
				"bus event %d = (%d,%s), replayed = (%d,%s)",
				i,
				busEvents[i].Seq,
				busEvents[i].Kind,
				replayed[i].Seq,
				replayed[i].Kind,
			)
		}
	}
}

func TestExecuteJobWithTimeoutACPSubcommandRuntimeUsesLaunchSpec(t *testing.T) {
	tmpDir := t.TempDir()
	installACPHelperCommandOnPath(t, "opencode", []runACPHelperScenario{{
		ExpectedPromptContains: "finish the task",
		Updates: []acp.SessionUpdate{
			acp.UpdateAgentMessageText("opencode subcommand path worked"),
		},
	}})

	job := newTestACPJob(tmpDir)
	result := executeJobWithTimeout(
		context.Background(),
		&config{
			IDE:                    model.IDEOpenCode,
			Model:                  "",
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

	if got := result.Status; got != attemptStatusSuccess {
		t.Fatalf("expected success status, got %s (%v)", got, result.Failure)
	}

	outLog, err := os.ReadFile(job.OutLog)
	if err != nil {
		t.Fatalf("read out log: %v", err)
	}
	if !strings.Contains(string(outLog), "opencode subcommand path worked") {
		t.Fatalf("expected subcommand ACP output in out log, got %q", string(outLog))
	}
}

func TestJobExecutionContextLaunchWorkersRunsMultipleACPJobs(t *testing.T) {
	tmpDir := t.TempDir()
	installACPHelperOnPath(t, []runACPHelperScenario{{
		Updates: []acp.SessionUpdate{acp.UpdateAgentMessageText("job completed")},
	}})

	jobs := []job{
		newNamedTestACPJob(tmpDir, "task_01"),
		newNamedTestACPJob(tmpDir, "task_02"),
	}
	execCtx := &jobExecutionContext{
		ctx: context.Background(),
		cfg: &config{
			IDE:                    model.IDECodex,
			Model:                  "",
			ReasoningEffort:        "medium",
			Concurrent:             2,
			RetryBackoffMultiplier: 2,
			Timeout:                time.Second,
		},
		jobs:  jobs,
		total: len(jobs),
		cwd:   tmpDir,
		sem:   make(chan struct{}, 2),
	}

	jobCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	execCtx.launchWorkers(jobCtx)
	select {
	case <-execCtx.waitChannel():
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for ACP worker execution")
	}

	if got := atomic.LoadInt32(&execCtx.completed); got != 2 {
		t.Fatalf("expected 2 completed jobs, got %d", got)
	}
	if got := atomic.LoadInt32(&execCtx.failed); got != 0 {
		t.Fatalf("expected 0 failed jobs, got %d", got)
	}
	for _, job := range jobs {
		outLog, err := os.ReadFile(job.OutLog)
		if err != nil {
			t.Fatalf("read out log %s: %v", job.OutLog, err)
		}
		if !strings.Contains(string(outLog), "job completed") {
			t.Fatalf("expected success output in %s, got %q", job.OutLog, string(outLog))
		}
	}
}

func TestJobExecutionContextLaunchWorkersRetriesRetryableSetupFailureForReviewBatch(t *testing.T) {
	tmpDir := t.TempDir()
	started := make(chan string, 1)
	finished := make(chan string, 1)

	firstClient := newFakeACPClient(func(context.Context, agent.SessionRequest) (agent.Session, error) {
		return nil, &agent.SessionSetupError{
			Stage: agent.SessionSetupStageNewSession,
			Err:   errors.New("temporary review batch setup failure"),
		}
	})
	secondClient := newPromptReportingACPClient(started, finished, nil)
	installFakeACPClients(t, firstClient, secondClient)

	jobs := []job{newNamedTestACPJob(tmpDir, "task_01")}
	execCtx := &jobExecutionContext{
		ctx: context.Background(),
		cfg: &config{
			IDE:                    model.IDECodex,
			Model:                  "test-model",
			ReasoningEffort:        "medium",
			Concurrent:             1,
			MaxRetries:             1,
			RetryBackoffMultiplier: 2,
			Timeout:                time.Second,
		},
		jobs:  jobs,
		total: len(jobs),
		cwd:   tmpDir,
		sem:   make(chan struct{}, 1),
	}

	jobCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	execCtx.launchWorkers(jobCtx)

	if got := waitForACPTaskEvent(t, started); !strings.Contains(got, "finish the task") {
		t.Fatalf("expected retry attempt prompt to include review batch instructions, got %q", got)
	}

	select {
	case <-execCtx.waitChannel():
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for retried review batch execution")
	}

	if got := firstClient.createCalls.Load(); got != 1 {
		t.Fatalf("expected one retryable setup failure attempt, got %d", got)
	}
	if got := secondClient.createCalls.Load(); got != 1 {
		t.Fatalf("expected one successful retry attempt, got %d", got)
	}
	if got := atomic.LoadInt32(&execCtx.failed); got != 0 {
		t.Fatalf("expected 0 failed jobs after retry, got %d", got)
	}
}

func TestJobExecutionContextLaunchWorkersRunsPRDTasksSequentially(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, ".compozy", "tasks", "demo")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir: %v", err)
	}

	for _, name := range []string{"task_01.md", "task_02.md", "task_03.md"} {
		writeRunTaskFile(t, tasksDir, name, "pending")
	}

	started := make(chan string, 3)
	finished := make(chan string, 3)
	releaseFirst := make(chan struct{})
	releaseSecond := make(chan struct{})

	installFakeACPClients(t,
		newPromptReportingACPClient(started, finished, releaseFirst),
		newPromptReportingACPClient(started, finished, releaseSecond),
		newPromptReportingACPClient(started, finished, nil),
	)

	jobs := []job{
		newPRDTaskACPJob(t, tmpDir, tasksDir, "task_01.md"),
		newPRDTaskACPJob(t, tmpDir, tasksDir, "task_02.md"),
		newPRDTaskACPJob(t, tmpDir, tasksDir, "task_03.md"),
	}
	execCtx := &jobExecutionContext{
		ctx: context.Background(),
		cfg: &config{
			IDE:                    model.IDECodex,
			Model:                  "test-model",
			ReasoningEffort:        "medium",
			Mode:                   model.ExecutionModePRDTasks,
			TasksDir:               tasksDir,
			Concurrent:             3,
			RetryBackoffMultiplier: 2,
			Timeout:                time.Second,
		},
		jobs:  jobs,
		total: len(jobs),
		cwd:   tmpDir,
		sem:   make(chan struct{}, 3),
	}

	jobCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	launchDone := make(chan struct{})
	go func() {
		execCtx.launchWorkers(jobCtx)
		close(launchDone)
	}()

	select {
	case <-launchDone:
	case <-time.After(time.Second):
		t.Fatal("launchWorkers blocked for sequential PRD execution")
	}

	if got := waitForACPTaskEvent(t, started); got != "task_01" {
		t.Fatalf("expected first PRD task to start task_01, got %q", got)
	}
	assertNoACPTaskEvent(
		t,
		started,
		150*time.Millisecond,
		"expected task_02 to remain pending while task_01 was blocked",
	)

	close(releaseFirst)
	if got := waitForACPTaskEvent(t, finished); got != "task_01" {
		t.Fatalf("expected task_01 to finish before task_02 starts, got %q", got)
	}
	if got := waitForACPTaskEvent(t, started); got != "task_02" {
		t.Fatalf("expected second PRD task to start task_02, got %q", got)
	}
	assertNoACPTaskEvent(
		t,
		started,
		150*time.Millisecond,
		"expected task_03 to remain pending while task_02 was blocked",
	)

	close(releaseSecond)
	if got := waitForACPTaskEvent(t, finished); got != "task_02" {
		t.Fatalf("expected task_02 to finish before task_03 starts, got %q", got)
	}
	if got := waitForACPTaskEvent(t, started); got != "task_03" {
		t.Fatalf("expected third PRD task to start task_03, got %q", got)
	}

	select {
	case <-execCtx.waitChannel():
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for sequential PRD worker execution")
	}

	if got := waitForACPTaskEvent(t, finished); got != "task_03" {
		t.Fatalf("expected task_03 to finish last, got %q", got)
	}
	assertNoACPTaskEvent(t, started, 50*time.Millisecond, "unexpected extra PRD task start recorded")
	assertNoACPTaskEvent(t, finished, 50*time.Millisecond, "unexpected extra PRD task finish recorded")

	if got := atomic.LoadInt32(&execCtx.completed); got != 3 {
		t.Fatalf("expected 3 completed PRD jobs, got %d", got)
	}
	if got := atomic.LoadInt32(&execCtx.failed); got != 0 {
		t.Fatalf("expected 0 failed PRD jobs, got %d", got)
	}
}

func TestJobExecutionContextLaunchWorkersRetriesPRDSetupFailureBeforeLaterTasks(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, ".compozy", "tasks", "demo")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir: %v", err)
	}

	for _, name := range []string{"task_01.md", "task_02.md"} {
		writeRunTaskFile(t, tasksDir, name, "pending")
	}

	started := make(chan string, 2)
	finished := make(chan string, 2)
	releaseFirst := make(chan struct{})

	firstClient := newFakeACPClient(func(context.Context, agent.SessionRequest) (agent.Session, error) {
		return nil, &agent.SessionSetupError{
			Stage: agent.SessionSetupStageNewSession,
			Err:   errors.New("temporary PRD setup failure"),
		}
	})
	secondClient := newPromptReportingACPClient(started, finished, releaseFirst)
	thirdClient := newPromptReportingACPClient(started, finished, nil)
	installFakeACPClients(t, firstClient, secondClient, thirdClient)

	jobs := []job{
		newPRDTaskACPJob(t, tmpDir, tasksDir, "task_01.md"),
		newPRDTaskACPJob(t, tmpDir, tasksDir, "task_02.md"),
	}
	execCtx := &jobExecutionContext{
		ctx: context.Background(),
		cfg: &config{
			IDE:                    model.IDECodex,
			Model:                  "test-model",
			ReasoningEffort:        "medium",
			Mode:                   model.ExecutionModePRDTasks,
			TasksDir:               tasksDir,
			Concurrent:             2,
			MaxRetries:             1,
			RetryBackoffMultiplier: 2,
			Timeout:                time.Second,
		},
		jobs:  jobs,
		total: len(jobs),
		cwd:   tmpDir,
		sem:   make(chan struct{}, 2),
	}

	jobCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	execCtx.launchWorkers(jobCtx)

	if got := waitForACPTaskEvent(t, started); got != "task_01" {
		t.Fatalf("expected retried PRD task_01 to start first, got %q", got)
	}
	assertNoACPTaskEvent(
		t,
		started,
		150*time.Millisecond,
		"expected task_02 to remain pending until task_01 retry succeeded",
	)

	close(releaseFirst)
	if got := waitForACPTaskEvent(t, finished); got != "task_01" {
		t.Fatalf("expected task_01 retry to finish before task_02 starts, got %q", got)
	}
	if got := waitForACPTaskEvent(t, started); got != "task_02" {
		t.Fatalf("expected task_02 to start after task_01 retry success, got %q", got)
	}

	select {
	case <-execCtx.waitChannel():
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for retried sequential PRD execution")
	}

	if got := firstClient.createCalls.Load(); got != 1 {
		t.Fatalf("expected one retryable PRD setup failure attempt, got %d", got)
	}
	if got := atomic.LoadInt32(&execCtx.failed); got != 0 {
		t.Fatalf("expected 0 failed PRD jobs after retry, got %d", got)
	}
}

func TestJobExecutionContextLaunchWorkersReturnsPromptlyWithPendingACPJobs(t *testing.T) {
	tmpDir := t.TempDir()
	firstCreated := make(chan struct{}, 1)

	firstClient := newFakeACPClient(func(ctx context.Context, _ agent.SessionRequest) (agent.Session, error) {
		session := newFakeACPSession("sess-blocking")
		firstCreated <- struct{}{}
		go func() {
			<-ctx.Done()
			session.finish(context.Cause(ctx))
		}()
		return session, nil
	})
	secondClient := newFakeACPClient(func(_ context.Context, _ agent.SessionRequest) (agent.Session, error) {
		session := newFakeACPSession("sess-pending")
		go session.finish(nil)
		return session, nil
	})
	installFakeACPClients(t, firstClient, secondClient)

	jobs := []job{
		newNamedTestACPJob(tmpDir, "task_01"),
		newNamedTestACPJob(tmpDir, "task_02"),
	}
	execCtx := &jobExecutionContext{
		ctx: context.Background(),
		cfg: &config{
			IDE:                    model.IDECodex,
			Model:                  "test-model",
			ReasoningEffort:        "medium",
			Concurrent:             1,
			RetryBackoffMultiplier: 2,
			Timeout:                time.Second,
		},
		jobs:  jobs,
		total: len(jobs),
		cwd:   tmpDir,
		sem:   make(chan struct{}, 1),
	}

	jobCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	launchDone := make(chan struct{})
	go func() {
		execCtx.launchWorkers(jobCtx)
		close(launchDone)
	}()

	select {
	case <-launchDone:
	case <-time.After(time.Second):
		t.Fatal("launchWorkers blocked on concurrency limits")
	}

	select {
	case <-firstCreated:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the first ACP session to start")
	}

	cancel()

	select {
	case <-execCtx.waitChannel():
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for workers to drain after cancellation")
	}

	if got := secondClient.createCalls.Load(); got != 0 {
		t.Fatalf("expected pending job to avoid ACP session creation after cancellation, got %d", got)
	}
}

func TestJobExecutionContextLaunchWorkersReturnsPromptlyWithPendingPRDTasks(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, ".compozy", "tasks", "demo")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir: %v", err)
	}

	for _, name := range []string{"task_01.md", "task_02.md"} {
		writeRunTaskFile(t, tasksDir, name, "pending")
	}

	firstCreated := make(chan struct{}, 1)
	firstClient := newFakeACPClient(func(ctx context.Context, _ agent.SessionRequest) (agent.Session, error) {
		session := newFakeACPSession("sess-prd-blocking")
		firstCreated <- struct{}{}
		go func() {
			<-ctx.Done()
			session.finish(context.Cause(ctx))
		}()
		return session, nil
	})
	secondClient := newFakeACPClient(func(_ context.Context, _ agent.SessionRequest) (agent.Session, error) {
		session := newFakeACPSession("sess-prd-pending")
		go session.finish(nil)
		return session, nil
	})
	installFakeACPClients(t, firstClient, secondClient)

	jobs := []job{
		newPRDTaskACPJob(t, tmpDir, tasksDir, "task_01.md"),
		newPRDTaskACPJob(t, tmpDir, tasksDir, "task_02.md"),
	}
	execCtx := &jobExecutionContext{
		ctx: context.Background(),
		cfg: &config{
			IDE:                    model.IDECodex,
			Model:                  "test-model",
			ReasoningEffort:        "medium",
			Mode:                   model.ExecutionModePRDTasks,
			TasksDir:               tasksDir,
			Concurrent:             1,
			RetryBackoffMultiplier: 2,
			Timeout:                time.Second,
		},
		jobs:  jobs,
		total: len(jobs),
		cwd:   tmpDir,
		sem:   make(chan struct{}, 1),
	}

	jobCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	launchDone := make(chan struct{})
	go func() {
		execCtx.launchWorkers(jobCtx)
		close(launchDone)
	}()

	select {
	case <-launchDone:
	case <-time.After(time.Second):
		t.Fatal("launchWorkers blocked for pending PRD tasks")
	}

	select {
	case <-firstCreated:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the first PRD ACP session to start")
	}

	cancel()

	select {
	case <-execCtx.waitChannel():
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for sequential PRD workers to drain after cancellation")
	}

	if got := secondClient.createCalls.Load(); got != 0 {
		t.Fatalf("expected later PRD task to avoid ACP session creation after cancellation, got %d", got)
	}
}

func TestRunACPHelperProcess(_ *testing.T) {
	if os.Getenv("GO_WANT_RUN_ACP_HELPER_PROCESS") != "1" {
		return
	}

	scenario, err := loadRunACPHelperScenario()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load helper scenario: %v\n", err)
		os.Exit(2)
	}

	agent := &runACPHelperAgent{
		scenario:  scenario,
		sessionID: helperFirstNonEmpty(scenario.SessionID, "sess-run-1"),
	}
	conn := acp.NewAgentSideConnection(agent, os.Stdout, os.Stdin)
	agent.conn = conn

	<-conn.Done()
	os.Exit(0)
}

type runACPHelperScenario struct {
	SessionID               string                    `json:"session_id,omitempty"`
	ExpectedLoadSessionID   string                    `json:"expected_load_session_id,omitempty"`
	ExpectedPromptContains  string                    `json:"expected_prompt_contains,omitempty"`
	SupportsLoadSession     bool                      `json:"supports_load_session,omitempty"`
	ReplayUpdatesOnLoad     []acp.SessionUpdate       `json:"replay_updates_on_load,omitempty"`
	SessionMeta             map[string]any            `json:"session_meta,omitempty"`
	Updates                 []acp.SessionUpdate       `json:"updates,omitempty"`
	StopReason              string                    `json:"stop_reason,omitempty"`
	BlockUntilCancel        bool                      `json:"block_until_cancel,omitempty"`
	NewSessionError         *runACPHelperRequestError `json:"new_session_error,omitempty"`
	PromptError             *runACPHelperRequestError `json:"prompt_error,omitempty"`
	PromptErrorAfterUpdates bool                      `json:"prompt_error_after_updates,omitempty"`
}

type runACPHelperRequestError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type runACPHelperAgent struct {
	conn      *acp.AgentSideConnection
	scenario  runACPHelperScenario
	sessionID string
}

func (a *runACPHelperAgent) Initialize(context.Context, acp.InitializeRequest) (acp.InitializeResponse, error) {
	return acp.InitializeResponse{
		ProtocolVersion: acp.ProtocolVersionNumber,
		AgentCapabilities: acp.AgentCapabilities{
			LoadSession: a.scenario.SupportsLoadSession,
		},
	}, nil
}

func (a *runACPHelperAgent) NewSession(_ context.Context, _ acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	if a.scenario.NewSessionError != nil {
		return acp.NewSessionResponse{}, a.scenario.NewSessionError.toACPError()
	}
	return acp.NewSessionResponse{
		SessionId: acp.SessionId(a.sessionID),
		Meta:      a.scenario.SessionMeta,
	}, nil
}

func (a *runACPHelperAgent) LoadSession(
	ctx context.Context,
	req acp.LoadSessionRequest,
) (acp.LoadSessionResponse, error) {
	if a.scenario.ExpectedLoadSessionID != "" && string(req.SessionId) != a.scenario.ExpectedLoadSessionID {
		return acp.LoadSessionResponse{}, &acp.RequestError{
			Code:    4002,
			Message: fmt.Sprintf("unexpected load session id %q", req.SessionId),
		}
	}
	for _, update := range a.scenario.ReplayUpdatesOnLoad {
		if err := a.conn.SessionUpdate(ctx, acp.SessionNotification{
			SessionId: acp.SessionId(a.sessionID),
			Update:    update,
		}); err != nil {
			return acp.LoadSessionResponse{}, err
		}
	}
	return acp.LoadSessionResponse{Meta: a.scenario.SessionMeta}, nil
}

func (a *runACPHelperAgent) Authenticate(context.Context, acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, nil
}

func (a *runACPHelperAgent) Prompt(ctx context.Context, req acp.PromptRequest) (acp.PromptResponse, error) {
	if want := strings.TrimSpace(a.scenario.ExpectedPromptContains); want != "" {
		gotPrompt := helperPromptText(req.Prompt)
		if !strings.Contains(gotPrompt, want) {
			return acp.PromptResponse{}, &acp.RequestError{
				Code:    4000,
				Message: fmt.Sprintf("prompt %q missing %q", gotPrompt, want),
			}
		}
	}

	if a.scenario.PromptError != nil && !a.scenario.PromptErrorAfterUpdates {
		return acp.PromptResponse{}, a.scenario.PromptError.toACPError()
	}

	for _, update := range a.scenario.Updates {
		if err := a.conn.SessionUpdate(ctx, acp.SessionNotification{
			SessionId: acp.SessionId(a.sessionID),
			Update:    update,
		}); err != nil {
			return acp.PromptResponse{}, err
		}
	}

	if a.scenario.PromptError != nil && a.scenario.PromptErrorAfterUpdates {
		return acp.PromptResponse{}, a.scenario.PromptError.toACPError()
	}

	if a.scenario.BlockUntilCancel {
		<-ctx.Done()
		return acp.PromptResponse{StopReason: acp.StopReasonCancelled}, nil
	}

	stopReason := acp.StopReasonEndTurn
	if a.scenario.StopReason != "" {
		stopReason = acp.StopReason(a.scenario.StopReason)
	}
	return acp.PromptResponse{StopReason: stopReason}, nil
}

func (a *runACPHelperAgent) Cancel(context.Context, acp.CancelNotification) error {
	return nil
}

func (a *runACPHelperAgent) SetSessionMode(
	context.Context,
	acp.SetSessionModeRequest,
) (acp.SetSessionModeResponse, error) {
	return acp.SetSessionModeResponse{}, nil
}

func (e *runACPHelperRequestError) toACPError() error {
	if e == nil {
		return nil
	}

	var data any
	if len(e.Data) > 0 {
		data = e.Data
	}
	return &acp.RequestError{
		Code:    e.Code,
		Message: e.Message,
		Data:    data,
	}
}

func installACPHelperOnPath(t *testing.T, sequences ...[]runACPHelperScenario) {
	t.Helper()
	installACPHelperCommandOnPath(t, "codex-acp", sequences...)
}

func installACPHelperCommandOnPath(t *testing.T, commandName string, sequences ...[]runACPHelperScenario) {
	t.Helper()

	if len(sequences) == 0 {
		t.Fatal("expected at least one helper scenario")
	}

	scenarioSets := sequences
	if len(scenarioSets) == 1 {
		scenarioSets = [][]runACPHelperScenario{sequences[0]}
	}

	payload, err := json.Marshal(scenarioSets)
	if err != nil {
		t.Fatalf("marshal helper scenarios: %v", err)
	}

	tmpDir := t.TempDir()
	counterFile := filepath.Join(tmpDir, "scenario-counter")
	if len(scenarioSets) > 1 {
		if err := os.WriteFile(counterFile, []byte("0"), 0o600); err != nil {
			t.Fatalf("write helper counter: %v", err)
		}
	}

	scriptPath := filepath.Join(tmpDir, commandName)
	script := fmt.Sprintf("#!/bin/sh\nexec %q -test.run=TestRunACPHelperProcess -- \"$@\"\n", os.Args[0])
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil {
		t.Fatalf("write helper script: %v", err)
	}

	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GO_WANT_RUN_ACP_HELPER_PROCESS", "1")
	t.Setenv("GO_RUN_ACP_HELPER_SCENARIOS", string(payload))
	if len(scenarioSets) > 1 {
		t.Setenv("GO_RUN_ACP_HELPER_COUNTER_FILE", counterFile)
	}
}

func loadRunACPHelperScenario() (runACPHelperScenario, error) {
	var scenarios [][]runACPHelperScenario
	if err := json.Unmarshal([]byte(os.Getenv("GO_RUN_ACP_HELPER_SCENARIOS")), &scenarios); err != nil {
		return runACPHelperScenario{}, err
	}
	if len(scenarios) == 0 {
		return runACPHelperScenario{}, fmt.Errorf("missing helper scenarios")
	}

	index := 0
	counterFile := os.Getenv("GO_RUN_ACP_HELPER_COUNTER_FILE")
	if counterFile != "" {
		content, err := os.ReadFile(counterFile)
		if err != nil {
			return runACPHelperScenario{}, err
		}
		index, err = strconv.Atoi(strings.TrimSpace(string(content)))
		if err != nil {
			return runACPHelperScenario{}, err
		}
		next := index + 1
		if next >= len(scenarios) {
			next = len(scenarios) - 1
		}
		if err := os.WriteFile(counterFile, []byte(strconv.Itoa(next)), 0o600); err != nil {
			return runACPHelperScenario{}, err
		}
		if index >= len(scenarios) {
			index = len(scenarios) - 1
		}
	}

	selected := scenarios[index]
	if len(selected) == 0 {
		return runACPHelperScenario{}, fmt.Errorf("empty helper scenario set %d", index)
	}
	return selected[0], nil
}

func helperPromptText(blocks []acp.ContentBlock) string {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Text != nil {
			parts = append(parts, block.Text.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func captureExecuteStreams(t *testing.T, fn func() error) (string, string, error) {
	t.Helper()

	// Process stdio is global state; parallel tests must serialize replacement.
	captureExecuteStreamsMu.Lock()
	defer captureExecuteStreamsMu.Unlock()

	originalStdout := os.Stdout
	originalStderr := os.Stderr
	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	stderrRead, stderrWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}

	os.Stdout = stdoutWrite
	os.Stderr = stderrWrite
	defer func() {
		os.Stdout = originalStdout
		os.Stderr = originalStderr
	}()

	type pipeReadResult struct {
		content string
		err     error
	}

	readPipe := func(file *os.File) <-chan pipeReadResult {
		resultCh := make(chan pipeReadResult, 1)
		go func() {
			bytes, err := io.ReadAll(file)
			resultCh <- pipeReadResult{content: string(bytes), err: err}
		}()
		return resultCh
	}

	stdoutResultCh := readPipe(stdoutRead)
	stderrResultCh := readPipe(stderrRead)
	runErr := fn()

	if err := stdoutWrite.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	if err := stderrWrite.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}

	stdoutResult := <-stdoutResultCh
	if stdoutResult.err != nil {
		t.Fatalf("read stdout: %v", stdoutResult.err)
	}
	stderrResult := <-stderrResultCh
	if stderrResult.err != nil {
		t.Fatalf("read stderr: %v", stderrResult.err)
	}
	if err := stdoutRead.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}
	if err := stderrRead.Close(); err != nil {
		t.Fatalf("close stderr reader: %v", err)
	}

	return stdoutResult.content, stderrResult.content, runErr
}

func TestCaptureExecuteStreamsDrainsPipesWhileFunctionRuns(t *testing.T) {
	type captureResult struct {
		stdout string
		stderr string
		err    error
	}

	resultCh := make(chan captureResult, 1)
	go func() {
		stdout, stderr, err := captureExecuteStreams(t, func() error {
			largeChunk := strings.Repeat("x", 256*1024)
			if _, writeErr := fmt.Fprint(os.Stdout, largeChunk); writeErr != nil {
				return writeErr
			}
			if _, writeErr := fmt.Fprint(os.Stderr, largeChunk); writeErr != nil {
				return writeErr
			}
			return nil
		})
		resultCh <- captureResult{stdout: stdout, stderr: stderr, err: err}
	}()

	select {
	case result := <-resultCh:
		if result.err != nil {
			t.Fatalf("captureExecuteStreams: %v", result.err)
		}
		if len(result.stdout) != 256*1024 {
			t.Fatalf("unexpected stdout size: got %d want %d", len(result.stdout), 256*1024)
		}
		if len(result.stderr) != 256*1024 {
			t.Fatalf("unexpected stderr size: got %d want %d", len(result.stderr), 256*1024)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("captureExecuteStreams blocked instead of draining the pipes")
	}
}

func helperFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func newNamedTestACPJob(tmpDir, safeName string) job {
	job := newTestACPJob(tmpDir)
	job.CodeFiles = []string{safeName}
	job.SafeName = safeName
	job.OutLog = filepath.Join(tmpDir, safeName+".out.log")
	job.ErrLog = filepath.Join(tmpDir, safeName+".err.log")
	return job
}

func newPromptReportingACPClient(
	started chan<- string,
	finished chan<- string,
	release <-chan struct{},
) *fakeACPClient {
	return newFakeACPClient(func(_ context.Context, req agent.SessionRequest) (agent.Session, error) {
		taskName := strings.TrimSpace(string(req.Prompt))
		started <- taskName
		session := newFakeACPSession("sess-" + taskName)
		go func() {
			if release != nil {
				<-release
			}
			finished <- taskName
			session.finish(nil)
		}()
		return session, nil
	})
}

func newPRDTaskACPJob(t *testing.T, tmpDir, tasksDir, fileName string) job {
	t.Helper()

	taskPath := filepath.Join(tasksDir, fileName)
	content, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read task file %s: %v", fileName, err)
	}

	codeFile := strings.TrimSuffix(fileName, ".md")
	job := newNamedTestACPJob(tmpDir, codeFile)
	job.Prompt = []byte(codeFile)
	job.SystemPrompt = ""
	job.Groups = map[string][]model.IssueEntry{
		codeFile: {{
			Name:     fileName,
			AbsPath:  taskPath,
			Content:  string(content),
			CodeFile: codeFile,
		}},
	}
	return job
}

func waitForACPTaskEvent(t *testing.T, ch <-chan string) string {
	t.Helper()

	select {
	case got := <-ch:
		return got
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ACP task event")
		return ""
	}
}

func assertNoACPTaskEvent(t *testing.T, ch <-chan string, window time.Duration, failure string) {
	t.Helper()

	select {
	case got := <-ch:
		t.Fatalf("%s: %q", failure, got)
	case <-time.After(window):
	}
}

func runtimeEventKinds(events []eventspkg.Event) []eventspkg.EventKind {
	kinds := make([]eventspkg.EventKind, 0, len(events))
	for _, event := range events {
		kinds = append(kinds, event.Kind)
	}
	return kinds
}

func replayRuntimeEvents(t *testing.T, run *runs.Run) []eventspkg.Event {
	t.Helper()

	var replayed []eventspkg.Event
	for event, err := range run.Replay(0) {
		if err != nil {
			t.Fatalf("replay runtime events: %v", err)
		}
		replayed = append(replayed, event)
	}
	return replayed
}
