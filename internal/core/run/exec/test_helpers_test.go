package exec

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	acp "github.com/coder/acp-go-sdk"
)

var captureExecuteStreamsMu sync.Mutex

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

func helperFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
