package run

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	acp "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
)

func TestExecuteJobWithTimeoutACPFullPipelineRoutesTypedBlocks(t *testing.T) {
	tmpDir := t.TempDir()
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
	uiCh := make(chan uiMsg, 16)
	var aggregate model.Usage
	var aggregateMu sync.Mutex
	result := executeJobWithTimeout(
		context.Background(),
		&config{
			ide:                    model.IDECodex,
			model:                  "",
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
		t.Fatalf("expected success status, got %s (%v)", got, result.failure)
	}

	var sawText bool
	var sawToolUse bool
	var sawToolResult bool
drain:
	for {
		select {
		case msg := <-uiCh:
			update, ok := msg.(jobUpdateMsg)
			if !ok {
				continue
			}
			for _, entry := range update.Snapshot.Entries {
				for _, block := range entry.Blocks {
					switch block.Type {
					case model.BlockText:
						sawText = true
					case model.BlockToolUse:
						sawToolUse = true
					case model.BlockToolResult:
						sawToolResult = true
					}
				}
			}
		default:
			break drain
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

	outLog, err := os.ReadFile(job.outLog)
	if err != nil {
		t.Fatalf("read out log: %v", err)
	}
	if !strings.Contains(string(outLog), "hello from ACP") || !strings.Contains(string(outLog), "README contents") {
		t.Fatalf("expected rendered ACP output in out log, got %q", string(outLog))
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
		cfg: &config{
			ide:                    model.IDECodex,
			model:                  "",
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
	errLog, err := os.ReadFile(job.errLog)
	if err != nil {
		t.Fatalf("read err log: %v", err)
	}
	if !strings.Contains(string(errLog), "retry me") {
		t.Fatalf("expected first ACP error in err log, got %q", string(errLog))
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
			ide:                    model.IDEOpenCode,
			model:                  "",
			reasoningEffort:        "medium",
			retryBackoffMultiplier: 2,
		},
		&job,
		tmpDir,
		false,
		nil,
		0,
		time.Second,
		nil,
		nil,
		nil,
	)

	if got := result.status; got != attemptStatusSuccess {
		t.Fatalf("expected success status, got %s (%v)", got, result.failure)
	}

	outLog, err := os.ReadFile(job.outLog)
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
		cfg: &config{
			ide:                    model.IDECodex,
			model:                  "",
			reasoningEffort:        "medium",
			concurrent:             2,
			retryBackoffMultiplier: 2,
			timeout:                time.Second,
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
		outLog, err := os.ReadFile(job.outLog)
		if err != nil {
			t.Fatalf("read out log %s: %v", job.outLog, err)
		}
		if !strings.Contains(string(outLog), "job completed") {
			t.Fatalf("expected success output in %s, got %q", job.outLog, string(outLog))
		}
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
		cfg: &config{
			ide:                    model.IDECodex,
			model:                  "test-model",
			reasoningEffort:        "medium",
			concurrent:             1,
			retryBackoffMultiplier: 2,
			timeout:                time.Second,
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
	SessionID              string                    `json:"session_id,omitempty"`
	ExpectedPromptContains string                    `json:"expected_prompt_contains,omitempty"`
	Updates                []acp.SessionUpdate       `json:"updates,omitempty"`
	StopReason             string                    `json:"stop_reason,omitempty"`
	BlockUntilCancel       bool                      `json:"block_until_cancel,omitempty"`
	NewSessionError        *runACPHelperRequestError `json:"new_session_error,omitempty"`
	PromptError            *runACPHelperRequestError `json:"prompt_error,omitempty"`
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
	return acp.InitializeResponse{ProtocolVersion: acp.ProtocolVersionNumber}, nil
}

func (a *runACPHelperAgent) NewSession(_ context.Context, _ acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	if a.scenario.NewSessionError != nil {
		return acp.NewSessionResponse{}, a.scenario.NewSessionError.toACPError()
	}
	return acp.NewSessionResponse{SessionId: acp.SessionId(a.sessionID)}, nil
}

func (a *runACPHelperAgent) LoadSession(context.Context, acp.LoadSessionRequest) (acp.LoadSessionResponse, error) {
	return acp.LoadSessionResponse{}, nil
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

	if a.scenario.PromptError != nil {
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
	job.codeFiles = []string{safeName}
	job.safeName = safeName
	job.outLog = filepath.Join(tmpDir, safeName+".out.log")
	job.errLog = filepath.Join(tmpDir, safeName+".err.log")
	return job
}
