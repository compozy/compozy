package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	acp "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/subprocess"
)

func TestClientCreateSessionSendsWorkingDirectoryAndPromptOverACP(t *testing.T) {
	t.Parallel()

	scenario := helperScenario{
		ExpectedCWD:    t.TempDir(),
		ExpectedPrompt: "hello from compozy",
		StopReason:     string(acp.StopReasonEndTurn),
	}

	client := newTestClient(t, scenario)
	session, err := client.CreateSession(context.Background(), SessionRequest{
		WorkingDir: scenario.ExpectedCWD,
		Prompt:     []byte(scenario.ExpectedPrompt),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	updates := collectSessionUpdates(t, session)
	if len(updates) != 1 {
		t.Fatalf("unexpected updates length: %d", len(updates))
	}
	if updates[0].Status != model.StatusCompleted {
		t.Fatalf("unexpected final status: %q", updates[0].Status)
	}
	if session.Err() != nil {
		t.Fatalf("unexpected session error: %v", session.Err())
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
}

func TestClientCreateSessionBuffersUpdatesArrivingBeforeNewSessionReturns(t *testing.T) {
	t.Parallel()

	scenario := helperScenario{
		ExpectedCWD:       t.TempDir(),
		ExpectedPrompt:    "hello after early update",
		NewSessionUpdates: []acp.SessionUpdate{acp.UpdateAgentMessageText("early update")},
		StopReason:        string(acp.StopReasonEndTurn),
	}

	client := newTestClient(t, scenario)
	session, err := client.CreateSession(context.Background(), SessionRequest{
		WorkingDir: scenario.ExpectedCWD,
		Prompt:     []byte(scenario.ExpectedPrompt),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	updates := collectSessionUpdates(t, session)
	if len(updates) != 2 {
		t.Fatalf("unexpected updates length: got %d want 2", len(updates))
	}
	if updates[0].Status != model.StatusRunning {
		t.Fatalf("unexpected first update status: %q", updates[0].Status)
	}
	text := mustFirstTextBlock(t, updates[0].Blocks)
	if text.Text != "early update" {
		t.Fatalf("unexpected early update text: %q", text.Text)
	}
	if updates[1].Status != model.StatusCompleted {
		t.Fatalf("unexpected final status: %q", updates[1].Status)
	}
	if session.Err() != nil {
		t.Fatalf("unexpected session error: %v", session.Err())
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
}

func TestClientStoreSessionReplaysPendingUpdatesAfterRequestCancellation(t *testing.T) {
	t.Parallel()

	setSessionPublishBackpressureTimeout(t, 200*time.Millisecond)

	session := newTestSessionWithBuffer("session-1", 1)
	session.updates <- model.SessionUpdate{
		Kind:   model.UpdateKindAgentMessageChunk,
		Status: model.StatusRunning,
	}

	pending := model.SessionUpdate{
		Kind:          model.UpdateKindToolCallUpdated,
		ToolCallID:    "tool-1",
		ToolCallState: model.ToolCallStateInProgress,
	}
	client := &clientImpl{
		sessions: make(map[string]*sessionImpl),
		pendingUpdates: map[string][]model.SessionUpdate{
			session.id: {pending},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	drained := make(chan struct{})
	go func() {
		time.Sleep(20 * time.Millisecond)
		<-session.updates
		close(drained)
	}()

	client.storeSession(ctx, session)

	select {
	case <-drained:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting to drain the pre-filled session buffer")
	}

	got := mustReceiveSessionUpdate(t, session.updates)
	if got.Kind != pending.Kind ||
		got.ToolCallID != pending.ToolCallID ||
		got.ToolCallState != pending.ToolCallState {
		t.Fatalf("unexpected replayed update: %#v", got)
	}
	if got.Status != model.StatusRunning {
		t.Fatalf("unexpected replayed status: %q", got.Status)
	}
	if pending := client.pendingUpdates[session.id]; len(pending) != 0 {
		t.Fatalf("expected pending updates to be cleared, got %#v", pending)
	}
}

func TestClientCreateSessionAppliesPreSessionCreateMutationAndPostCreateObserver(t *testing.T) {
	t.Parallel()

	const promptSuffix = " ::mutated"
	manager := &agentHookManager{
		mutators: map[string]func(any) (any, error){
			"agent.pre_session_create": func(input any) (any, error) {
				payload := input.(sessionCreateHookPayload)
				payload.SessionRequest.Prompt = append(payload.SessionRequest.Prompt, []byte(promptSuffix)...)
				return payload, nil
			},
		},
	}

	scenario := helperScenario{
		ExpectedCWD:    t.TempDir(),
		ExpectedPrompt: "hook me" + promptSuffix,
		StopReason:     string(acp.StopReasonEndTurn),
	}

	client := newTestClient(t, scenario)
	session, err := client.CreateSession(context.Background(), SessionRequest{
		Prompt:     []byte("hook me"),
		WorkingDir: scenario.ExpectedCWD,
		RunID:      "run-123",
		JobID:      "job-123",
		RuntimeMgr: manager,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	updates := collectSessionUpdates(t, session)
	if len(updates) != 1 || updates[0].Status != model.StatusCompleted {
		t.Fatalf("unexpected updates: %#v", updates)
	}

	manager.mu.Lock()
	defer manager.mu.Unlock()
	if got, want := manager.mutableHooks, []string{"agent.pre_session_create"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected mutable hook order\nwant: %#v\ngot:  %#v", want, got)
	}
	if got, want := manager.observerHooks, []string{"agent.post_session_create"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected observer hooks\nwant: %#v\ngot:  %#v", want, got)
	}
	if len(manager.observerPayloads) != 1 {
		t.Fatalf("expected one post-create payload, got %d", len(manager.observerPayloads))
	}
	payload := manager.observerPayloads[0].(sessionCreatedHookPayload)
	if payload.RunID != "run-123" || payload.JobID != "job-123" {
		t.Fatalf("unexpected observer payload ids: %#v", payload)
	}
	if payload.SessionID == "" {
		t.Fatalf("expected observer payload to carry session id, got %#v", payload)
	}

	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
}

func TestSessionUpdatesStreamAndComplete(t *testing.T) {
	t.Parallel()

	scenario := helperScenario{
		ExpectedCWD:    t.TempDir(),
		ExpectedPrompt: "stream updates",
		Updates: []acp.SessionUpdate{
			acp.UpdateAgentMessageText("hello"),
			acp.StartToolCall(
				acp.ToolCallId("tool-1"),
				"Read README",
				acp.WithStartRawInput(map[string]any{"path": "README.md"}),
			),
			acp.UpdateToolCall(
				acp.ToolCallId("tool-1"),
				acp.WithUpdateStatus(acp.ToolCallStatusCompleted),
				acp.WithUpdateContent([]acp.ToolCallContent{
					acp.ToolContent(acp.TextBlock("done")),
					acp.ToolDiffContent("README.md", "new content", "old content"),
				}),
			),
			acp.UpdateAgentMessage(acp.ImageBlock("ZmFrZQ==", "image/png")),
		},
		StopReason: string(acp.StopReasonEndTurn),
	}

	client := newTestClient(t, scenario)
	session, err := client.CreateSession(context.Background(), SessionRequest{
		WorkingDir: scenario.ExpectedCWD,
		Prompt:     []byte(scenario.ExpectedPrompt),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	updates := collectSessionUpdates(t, session)
	if len(updates) != 5 {
		t.Fatalf("unexpected update count: got %d want 5", len(updates))
	}

	blocks := flattenBlocks(updates)
	assertBlockTypes(t, blocks,
		model.BlockText,
		model.BlockToolUse,
		model.BlockToolResult,
		model.BlockDiff,
		model.BlockImage,
	)

	if updates[len(updates)-1].Status != model.StatusCompleted {
		t.Fatalf("unexpected final status: %q", updates[len(updates)-1].Status)
	}
	if session.Err() != nil {
		t.Fatalf("unexpected session error: %v", session.Err())
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
}

func TestClientSessionUpdateRejectsUnknownSessionWhenNoCreateIsPending(t *testing.T) {
	t.Parallel()

	client := &clientImpl{
		spec:     Spec{ID: "test-acp"},
		sessions: make(map[string]*sessionImpl),
	}

	err := client.SessionUpdate(context.Background(), acp.SessionNotification{
		SessionId: acp.SessionId("missing"),
		Update:    acp.UpdateAgentMessageText("no session"),
	})
	if err == nil || !strings.Contains(err.Error(), "received update for unknown session") {
		t.Fatalf("expected unknown session error, got %v", err)
	}
}

func TestClientCreateSessionAppliesFullAccessSessionModeWhenSupported(t *testing.T) {
	t.Parallel()

	scenario := helperScenario{
		ExpectedCWD:           t.TempDir(),
		ExpectedPrompt:        "run with full access",
		ExpectedSessionModeID: "bypassPermissions",
		StopReason:            string(acp.StopReasonEndTurn),
	}

	client := newTestClientWithConfig(t, scenario, func(cfg *ClientConfig) {
		cfg.AccessMode = model.AccessModeFull
	})
	session, err := client.CreateSession(context.Background(), SessionRequest{
		WorkingDir: scenario.ExpectedCWD,
		Prompt:     []byte(scenario.ExpectedPrompt),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	updates := collectSessionUpdates(t, session)
	if len(updates) != 1 {
		t.Fatalf("unexpected updates length: %d", len(updates))
	}
	if updates[0].Status != model.StatusCompleted {
		t.Fatalf("unexpected final status: %q", updates[0].Status)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
}

func TestClientCreateSessionForwardsMCPServersIntoNewSessionRequest(t *testing.T) {
	t.Parallel()

	scenario := helperScenario{
		ExpectedCWD: t.TempDir(),
		ExpectedNewSessionMCPServers: []acp.McpServer{
			{
				Stdio: &acp.McpServerStdio{
					Name:    "compozy",
					Command: "/tmp/compozy-test",
					Args:    []string{"mcp-serve", "--server", "compozy"},
					Env: []acp.EnvVariable{
						{Name: "COMPOZY_RUN_AGENT_CONTEXT", Value: "{\"depth\":0}"},
						{Name: "FORCE_COLOR", Value: "1"},
					},
				},
			},
		},
	}

	client := newTestClient(t, scenario)
	session, err := client.CreateSession(context.Background(), SessionRequest{
		WorkingDir: scenario.ExpectedCWD,
		Prompt:     []byte("hello"),
		MCPServers: []model.MCPServer{{
			Stdio: &model.MCPServerStdio{
				Name:    "compozy",
				Command: "/tmp/compozy-test",
				Args:    []string{"mcp-serve", "--server", "compozy"},
				Env: map[string]string{
					"FORCE_COLOR":               "1",
					"COMPOZY_RUN_AGENT_CONTEXT": "{\"depth\":0}",
				},
			},
		}},
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	_ = collectSessionUpdates(t, session)
}

func TestClientCreateSessionRejectsUnsupportedMCPTransport(t *testing.T) {
	t.Parallel()

	scenario := helperScenario{
		ExpectedCWD: t.TempDir(),
	}

	client := newTestClient(t, scenario)
	defer client.Close()
	_, err := client.CreateSession(context.Background(), SessionRequest{
		WorkingDir: scenario.ExpectedCWD,
		Prompt:     []byte("hello"),
		MCPServers: []model.MCPServer{{}},
	})
	if err == nil {
		t.Fatal("expected unsupported MCP transport error")
	}
	if !strings.Contains(err.Error(), "unsupported ACP MCP server transport at index 0") {
		t.Fatalf("unexpected unsupported MCP transport error: %v", err)
	}
}

func TestClientResumeSessionLoadsExistingSessionAndSuppressesReplay(t *testing.T) {
	t.Parallel()

	scenario := helperScenario{
		SessionID:             "sess-existing",
		ExpectedCWD:           t.TempDir(),
		ExpectedLoadSessionID: "sess-existing",
		ExpectedPrompt:        "continue the conversation",
		SupportsLoadSession:   true,
		SessionMeta:           map[string]any{"agentSessionId": "agent-123"},
		ReplayUpdatesOnLoad:   []acp.SessionUpdate{acp.UpdateAgentMessageText("replayed")},
		Updates:               []acp.SessionUpdate{acp.UpdateAgentMessageText("fresh response")},
		StopReason:            string(acp.StopReasonEndTurn),
	}

	client := newTestClient(t, scenario)
	session, err := client.ResumeSession(context.Background(), ResumeSessionRequest{
		SessionID:  scenario.SessionID,
		WorkingDir: scenario.ExpectedCWD,
		Prompt:     []byte(scenario.ExpectedPrompt),
	})
	if err != nil {
		t.Fatalf("resume session: %v", err)
	}
	if !client.SupportsLoadSession() {
		t.Fatal("expected load session support after initialization")
	}

	updates := collectSessionUpdates(t, session)
	if len(updates) != 2 {
		t.Fatalf("unexpected update count: got %d want 2", len(updates))
	}
	if got := mustFirstTextBlock(t, updates[0].Blocks).Text; got != "fresh response" {
		t.Fatalf("expected replay updates to be suppressed, got %q", got)
	}
	identity := session.Identity()
	if identity.ACPSessionID != scenario.SessionID {
		t.Fatalf("unexpected acp session id: %q", identity.ACPSessionID)
	}
	if identity.AgentSessionID != "agent-123" {
		t.Fatalf("unexpected agent session id: %q", identity.AgentSessionID)
	}
	if !identity.Resumed {
		t.Fatal("expected resumed identity")
	}
}

func TestClientResumeSessionForwardsMCPServersIntoLoadSessionRequest(t *testing.T) {
	t.Parallel()

	scenario := helperScenario{
		SessionID:             "sess-existing",
		ExpectedCWD:           t.TempDir(),
		ExpectedLoadSessionID: "sess-existing",
		SupportsLoadSession:   true,
		ExpectedLoadSessionMCPServers: []acp.McpServer{
			{
				Stdio: &acp.McpServerStdio{
					Name:    "filesystem",
					Command: "/tmp/fs-mcp",
					Args:    []string{"--serve"},
					Env: []acp.EnvVariable{
						{Name: "ROOT", Value: "/tmp/workspace"},
					},
				},
			},
		},
	}

	client := newTestClient(t, scenario)
	session, err := client.ResumeSession(context.Background(), ResumeSessionRequest{
		SessionID:  scenario.SessionID,
		WorkingDir: scenario.ExpectedCWD,
		Prompt:     []byte("continue"),
		MCPServers: []model.MCPServer{{
			Stdio: &model.MCPServerStdio{
				Name:    "filesystem",
				Command: "/tmp/fs-mcp",
				Args:    []string{"--serve"},
				Env: map[string]string{
					"ROOT": "/tmp/workspace",
				},
			},
		}},
	})
	if err != nil {
		t.Fatalf("resume session: %v", err)
	}

	_ = collectSessionUpdates(t, session)
}

func TestClientResumeSessionRejectsUnsupportedMCPTransport(t *testing.T) {
	t.Parallel()

	scenario := helperScenario{
		SessionID:           "sess-existing",
		ExpectedCWD:         t.TempDir(),
		SupportsLoadSession: true,
	}

	client := newTestClient(t, scenario)
	defer client.Close()
	_, err := client.ResumeSession(context.Background(), ResumeSessionRequest{
		SessionID:  scenario.SessionID,
		WorkingDir: scenario.ExpectedCWD,
		Prompt:     []byte("continue"),
		MCPServers: []model.MCPServer{{}},
	})
	if err == nil {
		t.Fatal("expected unsupported MCP transport error")
	}
	if !strings.Contains(err.Error(), "unsupported ACP MCP server transport at index 0") {
		t.Fatalf("unexpected unsupported MCP transport error: %v", err)
	}
}

func TestClientResumeSessionAppliesPreSessionResumeMutation(t *testing.T) {
	t.Parallel()

	const promptSuffix = " ::resume-mutated"
	manager := &agentHookManager{
		mutators: map[string]func(any) (any, error){
			"agent.pre_session_resume": func(input any) (any, error) {
				payload := input.(sessionResumeHookPayload)
				payload.ResumeRequest.Prompt = append(payload.ResumeRequest.Prompt, []byte(promptSuffix)...)
				return payload, nil
			},
		},
	}

	scenario := helperScenario{
		SessionID:             "sess-existing",
		ExpectedCWD:           t.TempDir(),
		ExpectedLoadSessionID: "sess-existing",
		ExpectedPrompt:        "continue" + promptSuffix,
		SupportsLoadSession:   true,
		Updates:               []acp.SessionUpdate{acp.UpdateAgentMessageText("fresh response")},
		StopReason:            string(acp.StopReasonEndTurn),
	}

	client := newTestClient(t, scenario)
	session, err := client.ResumeSession(context.Background(), ResumeSessionRequest{
		SessionID:  scenario.SessionID,
		Prompt:     []byte("continue"),
		WorkingDir: scenario.ExpectedCWD,
		RunID:      "run-456",
		JobID:      "job-456",
		RuntimeMgr: manager,
	})
	if err != nil {
		t.Fatalf("resume session: %v", err)
	}

	updates := collectSessionUpdates(t, session)
	if len(updates) != 2 {
		t.Fatalf("unexpected update count: got %d want 2", len(updates))
	}

	manager.mu.Lock()
	defer manager.mu.Unlock()
	if got, want := manager.mutableHooks, []string{"agent.pre_session_resume"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected resume hook order\nwant: %#v\ngot:  %#v", want, got)
	}

	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
}

func TestSessionRequestDispatchPreCreateHookWithoutManagerReturnsOriginal(t *testing.T) {
	t.Parallel()

	request := SessionRequest{
		Prompt:     []byte("keep me"),
		WorkingDir: t.TempDir(),
	}

	got, err := request.dispatchPreCreateHook()
	if err != nil {
		t.Fatalf("dispatchPreCreateHook() error = %v", err)
	}
	if !reflect.DeepEqual(got, request) {
		t.Fatalf("expected original request, got %#v", got)
	}
}

func TestResumeSessionRequestDispatchPreResumeHookWithoutManagerReturnsOriginal(t *testing.T) {
	t.Parallel()

	request := ResumeSessionRequest{
		SessionID:  "sess-123",
		Prompt:     []byte("keep me"),
		WorkingDir: t.TempDir(),
	}

	got, err := request.dispatchPreResumeHook()
	if err != nil {
		t.Fatalf("dispatchPreResumeHook() error = %v", err)
	}
	if !reflect.DeepEqual(got, request) {
		t.Fatalf("expected original resume request, got %#v", got)
	}
}

func TestSessionRequestJSONUsesReadablePromptText(t *testing.T) {
	t.Parallel()

	request := SessionRequest{
		Prompt:     []byte("plain prompt"),
		WorkingDir: "/tmp/work",
		Model:      "gpt-5.5",
	}

	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal session request: %v", err)
	}
	if strings.Contains(string(raw), "cGxhaW4gcHJvbXB0") {
		t.Fatalf("expected prompt text instead of base64 JSON, got %s", string(raw))
	}
	if !strings.Contains(string(raw), `"prompt":"plain prompt"`) {
		t.Fatalf("expected readable prompt JSON, got %s", string(raw))
	}

	var roundTrip SessionRequest
	if err := json.Unmarshal(raw, &roundTrip); err != nil {
		t.Fatalf("unmarshal session request: %v", err)
	}
	if got := string(roundTrip.Prompt); got != "plain prompt" {
		t.Fatalf("unexpected round-trip prompt: %q", got)
	}
}

func TestResumeSessionRequestJSONUsesReadablePromptText(t *testing.T) {
	t.Parallel()

	request := ResumeSessionRequest{
		SessionID:  "sess-123",
		Prompt:     []byte("resume prompt"),
		WorkingDir: "/tmp/work",
		Model:      "gpt-5.5",
	}

	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal resume session request: %v", err)
	}
	if strings.Contains(string(raw), "cmVzdW1lIHByb21wdA==") {
		t.Fatalf("expected prompt text instead of base64 JSON, got %s", string(raw))
	}
	if !strings.Contains(string(raw), `"prompt":"resume prompt"`) {
		t.Fatalf("expected readable resume prompt JSON, got %s", string(raw))
	}

	var roundTrip ResumeSessionRequest
	if err := json.Unmarshal(raw, &roundTrip); err != nil {
		t.Fatalf("unmarshal resume session request: %v", err)
	}
	if got := string(roundTrip.Prompt); got != "resume prompt" {
		t.Fatalf("unexpected round-trip resume prompt: %q", got)
	}
}

func TestDetachedContextPreservesValuesWithoutCancellation(t *testing.T) {
	t.Parallel()

	type contextKey string

	parent := context.WithValue(context.Background(), contextKey("trace_id"), "trace-123")
	withDeadline, cancel := context.WithTimeout(parent, time.Minute)
	t.Cleanup(cancel)

	detached := detachedContext(withDeadline)
	if got := detached.Value(contextKey("trace_id")); got != "trace-123" {
		t.Fatalf("unexpected detached context value: %#v", got)
	}
	if _, ok := detached.Deadline(); ok {
		t.Fatal("expected detached context to drop the parent deadline")
	}
	if detached.Done() != nil {
		t.Fatal("expected detached context to ignore parent cancellation")
	}
	if detached.Err() != nil {
		t.Fatalf("expected detached context to stay uncancelled, got %v", detached.Err())
	}
}

func TestSessionRequestDispatchPreCreateHookPreservesContext(t *testing.T) {
	t.Parallel()

	type contextKey string

	ctx := context.WithValue(context.Background(), contextKey("request_id"), "req-123")
	manager := &agentHookManager{
		mutators: map[string]func(any) (any, error){
			"agent.pre_session_create": func(input any) (any, error) {
				payload := input.(sessionCreateHookPayload)
				payload.SessionRequest.Prompt = append(payload.SessionRequest.Prompt, []byte("!")...)
				return payload, nil
			},
		},
	}

	request := SessionRequest{
		Context:    ctx,
		Prompt:     []byte("keep context"),
		WorkingDir: t.TempDir(),
		RunID:      "run-123",
		JobID:      "job-123",
		RuntimeMgr: manager,
	}

	got, err := request.dispatchPreCreateHook()
	if err != nil {
		t.Fatalf("dispatchPreCreateHook() error = %v", err)
	}
	if got.Context == nil {
		t.Fatal("expected hook-dispatched request to retain context")
	}
	if got := got.Context.Value(contextKey("request_id")); got != "req-123" {
		t.Fatalf("unexpected preserved context value: %#v", got)
	}
}

func TestResumeSessionRequestDispatchPreResumeHookPreservesContext(t *testing.T) {
	t.Parallel()

	type contextKey string

	ctx := context.WithValue(context.Background(), contextKey("request_id"), "req-456")
	manager := &agentHookManager{
		mutators: map[string]func(any) (any, error){
			"agent.pre_session_resume": func(input any) (any, error) {
				payload := input.(sessionResumeHookPayload)
				payload.ResumeRequest.Prompt = append(payload.ResumeRequest.Prompt, []byte("!")...)
				return payload, nil
			},
		},
	}

	request := ResumeSessionRequest{
		Context:    ctx,
		SessionID:  "sess-123",
		Prompt:     []byte("keep context"),
		WorkingDir: t.TempDir(),
		RunID:      "run-123",
		JobID:      "job-123",
		RuntimeMgr: manager,
	}

	got, err := request.dispatchPreResumeHook()
	if err != nil {
		t.Fatalf("dispatchPreResumeHook() error = %v", err)
	}
	if got.Context == nil {
		t.Fatal("expected hook-dispatched resume request to retain context")
	}
	if got := got.Context.Value(contextKey("request_id")); got != "req-456" {
		t.Fatalf("unexpected preserved resume context value: %#v", got)
	}
}

func TestNewSessionOutcome(t *testing.T) {
	t.Parallel()

	success := newSessionOutcome(model.StatusCompleted, nil)
	if success.Status != model.StatusCompleted || success.Error != "" {
		t.Fatalf("unexpected success outcome: %#v", success)
	}

	failure := newSessionOutcome(model.StatusFailed, errors.New("boom"))
	if failure.Status != model.StatusFailed || failure.Error != "boom" {
		t.Fatalf("unexpected failure outcome: %#v", failure)
	}
}
func TestSessionErrReturnsStructuredPromptError(t *testing.T) {
	t.Parallel()

	scenario := helperScenario{
		ExpectedCWD:    t.TempDir(),
		ExpectedPrompt: "fail me",
		PromptError: &helperRequestError{
			Code:    4242,
			Message: "prompt failed",
			Data:    json.RawMessage(`{"reason":"mock"}`),
		},
	}

	client := newTestClient(t, scenario)
	session, err := client.CreateSession(context.Background(), SessionRequest{
		WorkingDir: scenario.ExpectedCWD,
		Prompt:     []byte(scenario.ExpectedPrompt),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	updates := collectSessionUpdates(t, session)
	if len(updates) != 1 {
		t.Fatalf("unexpected updates length: %d", len(updates))
	}
	if updates[0].Status != model.StatusFailed {
		t.Fatalf("unexpected final status: %q", updates[0].Status)
	}

	sessionErr := session.Err()
	if sessionErr == nil {
		t.Fatal("expected session error")
	}

	var structuredErr *SessionError
	if !errors.As(sessionErr, &structuredErr) {
		t.Fatalf("expected SessionError, got %T", sessionErr)
	}
	if structuredErr.Code != 4242 {
		t.Fatalf("unexpected error code: %d", structuredErr.Code)
	}
	if structuredErr.Message != "prompt failed" {
		t.Fatalf("unexpected error message: %q", structuredErr.Message)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
}

func TestFailedToolCallPromptErrorFinishesSessionSuccessfully(t *testing.T) {
	t.Parallel()

	scenario := helperScenario{
		ExpectedCWD:    t.TempDir(),
		ExpectedPrompt: "recover from tool failure",
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
		PromptError: &helperRequestError{
			Code:    4242,
			Message: "tool call failed",
			Data:    json.RawMessage(`{"tool_call_id":"tool-1"}`),
		},
		PromptErrorAfterUpdates: true,
	}

	client := newTestClient(t, scenario)
	session, err := client.CreateSession(context.Background(), SessionRequest{
		WorkingDir: scenario.ExpectedCWD,
		Prompt:     []byte(scenario.ExpectedPrompt),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	updates := collectSessionUpdates(t, session)
	if len(updates) != 3 {
		t.Fatalf("unexpected updates length: %d", len(updates))
	}
	if updates[len(updates)-1].Status != model.StatusCompleted {
		t.Fatalf("unexpected final status: %q", updates[len(updates)-1].Status)
	}
	if session.Err() != nil {
		t.Fatalf("expected nil session error, got %v", session.Err())
	}

	assertBlockTypes(t, flattenBlocks(updates),
		model.BlockToolUse,
		model.BlockToolResult,
	)

	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
}

func TestShouldDowngradePromptErrorAfterToolFailureUsesErrorToolCallID(t *testing.T) {
	t.Parallel()

	if !shouldDowngradePromptErrorAfterToolFailure(nil, &SessionError{
		Code:    4242,
		Message: "tool call failed",
		Data:    json.RawMessage(`{"tool_call_id":"tool-1"}`),
	}) {
		t.Fatal("expected tool_call_id in session error payload to trigger downgrade")
	}

	if shouldDowngradePromptErrorAfterToolFailure(nil, &SessionError{
		Code:    4242,
		Message: "plain prompt failure",
		Data:    json.RawMessage(`{"reason":"boom"}`),
	}) {
		t.Fatal("expected generic session error payload to remain terminal")
	}
}

func TestSessionDoneClosesOnContextCancellation(t *testing.T) {
	t.Parallel()

	scenario := helperScenario{
		ExpectedCWD:      t.TempDir(),
		ExpectedPrompt:   "cancel me",
		BlockUntilCancel: true,
	}

	client := newTestClient(t, scenario)
	ctx, cancel := context.WithCancel(context.Background())
	session, err := client.CreateSession(ctx, SessionRequest{
		WorkingDir: scenario.ExpectedCWD,
		Prompt:     []byte(scenario.ExpectedPrompt),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	cancel()

	select {
	case <-session.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for session cancellation")
	}

	if !errors.Is(session.Err(), context.Canceled) {
		t.Fatalf("expected context cancellation error, got %v", session.Err())
	}

	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
}

func TestClientCloseTerminatesSubprocessGracefully(t *testing.T) {
	t.Parallel()

	scenario := helperScenario{
		ExpectedCWD:    t.TempDir(),
		ExpectedPrompt: "close gracefully",
		StopReason:     string(acp.StopReasonEndTurn),
	}

	client := newTestClient(t, scenario)
	session, err := client.CreateSession(context.Background(), SessionRequest{
		WorkingDir: scenario.ExpectedCWD,
		Prompt:     []byte(scenario.ExpectedPrompt),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	_ = collectSessionUpdates(t, session)

	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
}

func TestClientKillForceTerminatesSubprocess(t *testing.T) {
	t.Parallel()

	scenario := helperScenario{
		ExpectedCWD:      t.TempDir(),
		ExpectedPrompt:   "kill immediately",
		BlockUntilCancel: true,
	}

	client := newTestClient(t, scenario)
	session, err := client.CreateSession(context.Background(), SessionRequest{
		WorkingDir: scenario.ExpectedCWD,
		Prompt:     []byte(scenario.ExpectedPrompt),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := client.Kill(); err != nil {
		t.Fatalf("kill client: %v", err)
	}

	select {
	case <-session.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for session shutdown after kill")
	}

	if !errors.Is(session.Err(), context.Canceled) {
		t.Fatalf("expected forced kill to finish sessions as canceled, got %v", session.Err())
	}
}

func TestWaitForProcessTreatsForcedExitAsCancellation(t *testing.T) {
	t.Parallel()

	scenarioJSON, err := json.Marshal(helperScenario{})
	if err != nil {
		t.Fatalf("marshal helper scenario: %v", err)
	}

	process, err := subprocess.Launch(context.Background(), subprocess.LaunchConfig{
		Command:         []string{os.Args[0], "-test.run=TestACPHelperProcess", "--"},
		Env:             append(os.Environ(), "GO_WANT_ACP_HELPER_PROCESS=1", "GO_ACP_SCENARIO="+string(scenarioJSON)),
		WaitErrorPrefix: "wait for process test helper",
	})
	if err != nil {
		t.Fatalf("launch helper process: %v", err)
	}
	t.Cleanup(func() {
		_ = process.Kill()
	})

	workingDir := t.TempDir()
	session := newSessionWithAccess("sess-wait", workingDir, []string{workingDir})
	client := &clientImpl{
		process:  process,
		sessions: map[string]*sessionImpl{session.id: session},
	}

	client.wg.Add(1)
	go client.waitForProcess()

	if err := process.Kill(); err != nil {
		t.Fatalf("kill helper process: %v", err)
	}

	select {
	case <-session.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for session shutdown after forced process exit")
	}

	if !errors.Is(session.Err(), context.Canceled) {
		t.Fatalf("session.Err() = %v, want context.Canceled", session.Err())
	}

	client.wg.Wait()
}

func TestClientHelperMethods(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	addDir := t.TempDir()
	filePath := filepath.Join(tempDir, "helper.txt")
	relativePath := filepath.Join("nested", "helper.txt")
	client := &clientImpl{
		sessions: map[string]*sessionImpl{
			"sess-1": newSessionWithAccess("sess-1", tempDir, []string{tempDir, addDir}),
		},
	}

	if _, err := client.WriteTextFile(context.Background(), acp.WriteTextFileRequest{
		SessionId: "sess-1",
		Path:      filePath,
		Content:   "hello",
	}); err != nil {
		t.Fatalf("write text file: %v", err)
	}

	readResp, err := client.ReadTextFile(context.Background(), acp.ReadTextFileRequest{
		SessionId: "sess-1",
		Path:      filePath,
	})
	if err != nil {
		t.Fatalf("read text file: %v", err)
	}
	if readResp.Content != "hello" {
		t.Fatalf("unexpected read content: %q", readResp.Content)
	}

	relativeFilePath := filepath.Join(tempDir, relativePath)
	if err := os.MkdirAll(filepath.Dir(relativeFilePath), 0o755); err != nil {
		t.Fatalf("mkdir relative helper dir: %v", err)
	}
	if err := os.WriteFile(relativeFilePath, []byte("nested"), 0o600); err != nil {
		t.Fatalf("write relative helper file: %v", err)
	}
	relativeResp, err := client.ReadTextFile(context.Background(), acp.ReadTextFileRequest{
		SessionId: "sess-1",
		Path:      relativePath,
	})
	if err != nil {
		t.Fatalf("read relative text file: %v", err)
	}
	if relativeResp.Content != "nested" {
		t.Fatalf("unexpected relative read content: %q", relativeResp.Content)
	}

	modeFilePath := filepath.Join(tempDir, "mode.txt")
	if err := os.WriteFile(modeFilePath, []byte("old"), 0o644); err != nil {
		t.Fatalf("write mode helper file: %v", err)
	}
	if _, err := client.WriteTextFile(context.Background(), acp.WriteTextFileRequest{
		SessionId: "sess-1",
		Path:      modeFilePath,
		Content:   "updated",
	}); err != nil {
		t.Fatalf("overwrite text file: %v", err)
	}
	modeInfo, err := os.Stat(modeFilePath)
	if err != nil {
		t.Fatalf("stat mode helper file: %v", err)
	}
	if got := modeInfo.Mode().Perm(); got != 0o644 {
		t.Fatalf("expected overwrite to preserve file mode 0644, got %04o", got)
	}

	addDirFile := filepath.Join(addDir, "shared.txt")
	if err := os.WriteFile(addDirFile, []byte("shared"), 0o600); err != nil {
		t.Fatalf("write add-dir helper file: %v", err)
	}
	addDirResp, err := client.ReadTextFile(context.Background(), acp.ReadTextFileRequest{
		SessionId: "sess-1",
		Path:      addDirFile,
	})
	if err != nil {
		t.Fatalf("read add-dir text file: %v", err)
	}
	if addDirResp.Content != "shared" {
		t.Fatalf("unexpected add-dir read content: %q", addDirResp.Content)
	}

	outsideFile := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outsideFile, []byte("outside"), 0o600); err != nil {
		t.Fatalf("write outside helper file: %v", err)
	}
	if _, err := client.ReadTextFile(context.Background(), acp.ReadTextFileRequest{
		SessionId: "sess-1",
		Path:      outsideFile,
	}); err == nil || !strings.Contains(err.Error(), "outside allowed session roots") {
		t.Fatalf("expected outside-root read failure, got %v", err)
	}
	if _, err := client.WriteTextFile(context.Background(), acp.WriteTextFileRequest{
		SessionId: "missing",
		Path:      filepath.Join(tempDir, "other.txt"),
		Content:   "nope",
	}); err == nil || !strings.Contains(err.Error(), "unknown session") {
		t.Fatalf("expected unknown session write failure, got %v", err)
	}

	permResp, err := client.RequestPermission(context.Background(), acp.RequestPermissionRequest{
		Options: []acp.PermissionOption{{OptionId: "allow"}},
	})
	if err != nil {
		t.Fatalf("request permission: %v", err)
	}
	if permResp.Outcome.Selected == nil || permResp.Outcome.Selected.OptionId != "allow" {
		t.Fatalf("unexpected permission selection: %#v", permResp.Outcome)
	}

	cancelledResp, err := client.RequestPermission(context.Background(), acp.RequestPermissionRequest{})
	if err != nil {
		t.Fatalf("request permission without options: %v", err)
	}
	if !outcomeHasVariant(cancelledResp.Outcome, "Cancel"+"led") {
		t.Fatalf("expected canceled permission outcome: %#v", cancelledResp.Outcome)
	}
}

func TestClientTerminalMethodsReturnUnsupported(t *testing.T) {
	t.Parallel()

	client := &clientImpl{}
	cases := []struct {
		name string
		call func() error
	}{
		{
			name: "create",
			call: func() error {
				_, err := client.CreateTerminal(context.Background(), acp.CreateTerminalRequest{})
				return err
			},
		},
		{
			name: "kill",
			call: func() error {
				_, err := client.KillTerminalCommand(context.Background(), acp.KillTerminalCommandRequest{})
				return err
			},
		},
		{
			name: "output",
			call: func() error {
				_, err := client.TerminalOutput(context.Background(), acp.TerminalOutputRequest{})
				return err
			},
		},
		{
			name: "release",
			call: func() error {
				_, err := client.ReleaseTerminal(context.Background(), acp.ReleaseTerminalRequest{})
				return err
			},
		},
		{
			name: "wait_for_exit",
			call: func() error {
				_, err := client.WaitForTerminalExit(context.Background(), acp.WaitForTerminalExitRequest{})
				return err
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.call(); err == nil {
				t.Fatal("expected unsupported terminal error")
			}
		})
	}
}

func TestClientUtilityHelpers(t *testing.T) {
	t.Parallel()

	if _, err := NewClient(context.Background(), ClientConfig{IDE: "unknown"}); err == nil {
		t.Fatal("expected unknown ide error")
	}

	relativeDir, err := resolveWorkingDir(".")
	if err == nil || relativeDir != "" {
		t.Fatalf("expected empty directory validation error, got %q err=%v", relativeDir, err)
	}

	absoluteDir, err := resolveWorkingDir("..")
	if err != nil {
		t.Fatalf("resolve working dir: %v", err)
	}
	if !filepath.IsAbs(absoluteDir) {
		t.Fatalf("expected absolute path, got %q", absoluteDir)
	}

	buffer := &subprocess.LockedBuffer{}
	if _, err := buffer.Write([]byte("stderr")); err != nil {
		t.Fatalf("write locked buffer: %v", err)
	}
	if got := buffer.String(); got != "stderr" {
		t.Fatalf("unexpected buffer contents: %q", got)
	}

	wrapped := wrapACPError(&acp.RequestError{
		Code:    42,
		Message: "boom",
		Data:    map[string]any{"status": "bad"},
	})
	var sessionErr *SessionError
	if !errors.As(wrapped, &sessionErr) {
		t.Fatalf("expected SessionError, got %T", wrapped)
	}
	if !strings.Contains(sessionErr.Error(), "boom") {
		t.Fatalf("unexpected session error string: %q", sessionErr.Error())
	}

	noDataErr := (&SessionError{Code: 7, Message: "plain"}).Error()
	if !strings.Contains(noDataErr, "plain") {
		t.Fatalf("unexpected no-data session error string: %q", noDataErr)
	}
}

func TestClientCreateSessionSurfacesStartupCommandAndStderr(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "broken-agent")
	script := "#!/bin/sh\nprintf 'adapter boot failed' >&2\nexit 23\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil {
		t.Fatalf("write broken helper: %v", err)
	}

	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	registerTestSpec(t, Spec{
		ID:           "broken-agent-test",
		DisplayName:  "Broken Agent",
		DefaultModel: "test-model",
		Command:      "broken-agent",
		FixedArgs:    []string{"acp"},
		ProbeArgs:    []string{"--help"},
		InstallHint:  "Install the working ACP adapter.",
		DocsURL:      "https://example.com/acp",
	})

	client, err := NewClient(context.Background(), ClientConfig{
		IDE:             "broken-agent-test",
		Model:           "test-model",
		ReasoningEffort: "medium",
		ShutdownTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	_, err = client.CreateSession(context.Background(), SessionRequest{
		WorkingDir: t.TempDir(),
		Prompt:     []byte("hello"),
	})
	if err == nil {
		t.Fatal("expected create session error")
	}
	if !strings.Contains(err.Error(), "broken-agent acp") {
		t.Fatalf("expected attempted command in error, got %q", err)
	}
	if !strings.Contains(err.Error(), "adapter boot failed") {
		t.Fatalf("expected adapter stderr in error, got %q", err)
	}
}

func TestClientCreateSessionChecksCodexModelCompatibilityBeforeLaunch(t *testing.T) {
	installCodexACPNPMPackage(t, "0.11.1")

	client, err := NewClient(context.Background(), ClientConfig{
		IDE:             model.IDECodex,
		Model:           "gpt-5.5",
		ReasoningEffort: "low",
		ShutdownTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	_, err = client.CreateSession(context.Background(), SessionRequest{
		WorkingDir: t.TempDir(),
		Prompt:     []byte("hello"),
	})
	if err == nil {
		t.Fatal("expected codex-acp compatibility error")
	}
	var setupErr *SessionSetupError
	if !errors.As(err, &setupErr) {
		t.Fatalf("expected SessionSetupError, got %T", err)
	}
	if setupErr.Stage != SessionSetupStageStartProcess {
		t.Fatalf("setup stage = %q, want %q", setupErr.Stage, SessionSetupStageStartProcess)
	}
	for _, want := range []string{
		"gpt-5.5 requires codex-acp >= 0.12.0",
		"found 0.11.1",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("compatibility error = %q, want %q", err, want)
		}
	}
}

func TestACPHelperProcess(_ *testing.T) {
	if os.Getenv("GO_WANT_ACP_HELPER_PROCESS") != "1" {
		return
	}

	var scenario helperScenario
	if err := json.Unmarshal([]byte(os.Getenv("GO_ACP_SCENARIO")), &scenario); err != nil {
		fmt.Fprintf(os.Stderr, "unmarshal helper scenario: %v\n", err)
		os.Exit(2)
	}

	agent := &helperAgent{
		scenario:  scenario,
		sessionID: firstNonEmpty(scenario.SessionID, "sess-1"),
		connReady: make(chan struct{}),
	}
	conn := acp.NewAgentSideConnection(agent, os.Stdout, os.Stdin)
	agent.setConn(conn)

	<-conn.Done()
	os.Exit(0)
}

type helperScenario struct {
	SessionID                     string              `json:"session_id,omitempty"`
	ExpectedCWD                   string              `json:"expected_cwd,omitempty"`
	ExpectedLoadSessionID         string              `json:"expected_load_session_id,omitempty"`
	ExpectedNewSessionMCPServers  []acp.McpServer     `json:"expected_new_session_mcp_servers,omitempty"`
	ExpectedLoadSessionMCPServers []acp.McpServer     `json:"expected_load_session_mcp_servers,omitempty"`
	ExpectedPrompt                string              `json:"expected_prompt,omitempty"`
	ExpectedSessionModeID         string              `json:"expected_session_mode_id,omitempty"`
	UpdateIntervalMillis          int                 `json:"update_interval_millis,omitempty"`
	SupportsLoadSession           bool                `json:"supports_load_session,omitempty"`
	SessionMeta                   map[string]any      `json:"session_meta,omitempty"`
	NewSessionUpdates             []acp.SessionUpdate `json:"new_session_updates,omitempty"`
	ReplayUpdatesOnLoad           []acp.SessionUpdate `json:"replay_updates_on_load,omitempty"`
	Updates                       []acp.SessionUpdate `json:"updates,omitempty"`
	StopReason                    string              `json:"stop_reason,omitempty"`
	BlockUntilCancel              bool                `json:"block_until_cancel,omitempty"`
	NewSessionError               *helperRequestError `json:"new_session_error,omitempty"`
	PromptError                   *helperRequestError `json:"prompt_error,omitempty"`
	PromptErrorAfterUpdates       bool                `json:"prompt_error_after_updates,omitempty"`
}

type helperRequestError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type helperAgent struct {
	conn      *acp.AgentSideConnection
	connReady chan struct{}
	scenario  helperScenario
	sessionID string
}

func (a *helperAgent) setConn(conn *acp.AgentSideConnection) {
	a.conn = conn
	close(a.connReady)
}

func (a *helperAgent) connection() *acp.AgentSideConnection {
	<-a.connReady
	return a.conn
}

func (a *helperAgent) Initialize(context.Context, acp.InitializeRequest) (acp.InitializeResponse, error) {
	return acp.InitializeResponse{
		ProtocolVersion: acp.ProtocolVersionNumber,
		AgentCapabilities: acp.AgentCapabilities{
			LoadSession: a.scenario.SupportsLoadSession,
		},
	}, nil
}

func (a *helperAgent) NewSession(_ context.Context, req acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	if a.scenario.NewSessionError != nil {
		return acp.NewSessionResponse{}, a.scenario.NewSessionError.toACPError()
	}
	if a.scenario.ExpectedCWD != "" && req.Cwd != a.scenario.ExpectedCWD {
		return acp.NewSessionResponse{}, &acp.RequestError{
			Code:    4001,
			Message: fmt.Sprintf("unexpected cwd %q", req.Cwd),
		}
	}
	if a.scenario.ExpectedNewSessionMCPServers != nil &&
		!reflect.DeepEqual(req.McpServers, a.scenario.ExpectedNewSessionMCPServers) {
		return acp.NewSessionResponse{}, &acp.RequestError{
			Code:    4004,
			Message: fmt.Sprintf("unexpected new-session MCP servers %#v", req.McpServers),
		}
	}
	if err := a.emitUpdates(context.Background(), a.scenario.NewSessionUpdates); err != nil {
		return acp.NewSessionResponse{}, err
	}
	return acp.NewSessionResponse{
		SessionId: acp.SessionId(a.sessionID),
		Meta:      a.scenario.SessionMeta,
	}, nil
}

func (a *helperAgent) LoadSession(ctx context.Context, req acp.LoadSessionRequest) (acp.LoadSessionResponse, error) {
	if a.scenario.ExpectedLoadSessionID != "" && string(req.SessionId) != a.scenario.ExpectedLoadSessionID {
		return acp.LoadSessionResponse{}, &acp.RequestError{
			Code:    4002,
			Message: fmt.Sprintf("unexpected load session id %q", req.SessionId),
		}
	}
	if a.scenario.ExpectedCWD != "" && req.Cwd != a.scenario.ExpectedCWD {
		return acp.LoadSessionResponse{}, &acp.RequestError{
			Code:    4003,
			Message: fmt.Sprintf("unexpected load cwd %q", req.Cwd),
		}
	}
	if a.scenario.ExpectedLoadSessionMCPServers != nil &&
		!reflect.DeepEqual(req.McpServers, a.scenario.ExpectedLoadSessionMCPServers) {
		return acp.LoadSessionResponse{}, &acp.RequestError{
			Code:    4004,
			Message: fmt.Sprintf("unexpected load-session MCP servers %#v", req.McpServers),
		}
	}
	if err := a.emitUpdates(ctx, a.scenario.ReplayUpdatesOnLoad); err != nil {
		return acp.LoadSessionResponse{}, err
	}
	return acp.LoadSessionResponse{Meta: a.scenario.SessionMeta}, nil
}

func (a *helperAgent) Authenticate(context.Context, acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, nil
}

func (a *helperAgent) Prompt(ctx context.Context, req acp.PromptRequest) (acp.PromptResponse, error) {
	if a.scenario.ExpectedPrompt != "" {
		gotPrompt := firstPromptText(req.Prompt)
		if gotPrompt != a.scenario.ExpectedPrompt {
			return acp.PromptResponse{}, &acp.RequestError{
				Code:    4000,
				Message: fmt.Sprintf("unexpected prompt %q", gotPrompt),
			}
		}
	}

	if a.scenario.PromptError != nil && !a.scenario.PromptErrorAfterUpdates {
		return acp.PromptResponse{}, a.scenario.PromptError.toACPError()
	}

	if err := a.emitUpdates(ctx, a.scenario.Updates); err != nil {
		return acp.PromptResponse{}, err
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

func (a *helperAgent) emitUpdates(ctx context.Context, updates []acp.SessionUpdate) error {
	for i, update := range updates {
		if i > 0 && a.scenario.UpdateIntervalMillis > 0 {
			timer := time.NewTimer(time.Duration(a.scenario.UpdateIntervalMillis) * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}

		if err := a.connection().SessionUpdate(ctx, acp.SessionNotification{
			SessionId: acp.SessionId(a.sessionID),
			Update:    update,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (a *helperAgent) Cancel(context.Context, acp.CancelNotification) error {
	return nil
}

func (a *helperAgent) SetSessionMode(
	_ context.Context,
	req acp.SetSessionModeRequest,
) (acp.SetSessionModeResponse, error) {
	if a.scenario.ExpectedSessionModeID != "" && string(req.ModeId) != a.scenario.ExpectedSessionModeID {
		return acp.SetSessionModeResponse{}, &acp.RequestError{
			Code:    4002,
			Message: fmt.Sprintf("unexpected session mode %q", req.ModeId),
		}
	}
	return acp.SetSessionModeResponse{}, nil
}

func (e *helperRequestError) toACPError() error {
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

func newTestClient(t *testing.T, scenario helperScenario) Client {
	t.Helper()

	return newTestClientWithConfig(t, scenario, nil)
}

func newTestClientWithConfig(t *testing.T, scenario helperScenario, configure func(*ClientConfig)) Client {
	t.Helper()

	scenarioJSON, err := json.Marshal(scenario)
	if err != nil {
		t.Fatalf("marshal helper scenario: %v", err)
	}

	ide := "test-acp-" + sanitizeTestName(t.Name())
	registerTestSpec(t, Spec{
		ID:           ide,
		DisplayName:  "Test ACP",
		DefaultModel: "test-model",
		Command:      os.Args[0],
		EnvVars: map[string]string{
			"GO_WANT_ACP_HELPER_PROCESS": "1",
			"GO_ACP_SCENARIO":            string(scenarioJSON),
		},
		FullAccessModeID:   scenario.ExpectedSessionModeID,
		UsesBootstrapModel: true,
		BootstrapArgs: func(_, _ string, _ []string, _ string) []string {
			return []string{"-test.run=TestACPHelperProcess", "--"}
		},
	})

	cfg := ClientConfig{
		IDE:             ide,
		Model:           "test-model",
		ReasoningEffort: "medium",
		ShutdownTimeout: 3 * time.Second,
	}
	if configure != nil {
		configure(&cfg)
	}

	client, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return client
}

func collectSessionUpdates(t *testing.T, session Session) []model.SessionUpdate {
	t.Helper()

	var updates []model.SessionUpdate
	for update := range session.Updates() {
		updates = append(updates, update)
	}

	select {
	case <-session.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for session.Done")
	}

	return updates
}

func flattenBlocks(updates []model.SessionUpdate) []model.ContentBlock {
	blocks := make([]model.ContentBlock, 0)
	for i := range updates {
		blocks = append(blocks, updates[i].Blocks...)
	}
	return blocks
}

func mustFirstTextBlock(t *testing.T, blocks []model.ContentBlock) model.TextBlock {
	t.Helper()

	if len(blocks) == 0 {
		t.Fatal("expected at least one content block")
	}
	textBlock, err := blocks[0].AsText()
	if err != nil {
		t.Fatalf("decode first text block: %v", err)
	}
	return textBlock
}

func assertBlockTypes(t *testing.T, blocks []model.ContentBlock, wantTypes ...model.ContentBlockType) {
	t.Helper()

	gotTypes := make([]model.ContentBlockType, 0, len(blocks))
	for _, block := range blocks {
		gotTypes = append(gotTypes, block.Type)
	}

	for _, want := range wantTypes {
		found := false
		for _, got := range gotTypes {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing block type %q in %v", want, gotTypes)
		}
	}
}

func sanitizeTestName(name string) string {
	replacer := strings.NewReplacer("/", "-", " ", "-", ":", "-", "_", "-")
	return replacer.Replace(strings.ToLower(name))
}

func firstPromptText(blocks []acp.ContentBlock) string {
	for _, block := range blocks {
		if block.Text != nil {
			return block.Text.Text
		}
	}
	return ""
}

func outcomeHasVariant(outcome acp.RequestPermissionOutcome, variant string) bool {
	field := reflect.ValueOf(outcome).FieldByName(variant)
	return field.IsValid() && field.Kind() == reflect.Pointer && !field.IsNil()
}

type agentHookManager struct {
	mu               sync.Mutex
	mutators         map[string]func(any) (any, error)
	mutableHooks     []string
	observerHooks    []string
	observerPayloads []any
}

func (*agentHookManager) Start(context.Context) error { return nil }

func (*agentHookManager) Shutdown(context.Context) error { return nil }

func (m *agentHookManager) DispatchMutableHook(
	_ context.Context,
	hook string,
	input any,
) (any, error) {
	m.mu.Lock()
	m.mutableHooks = append(m.mutableHooks, hook)
	mutate := m.mutators[hook]
	m.mu.Unlock()
	if mutate == nil {
		return input, nil
	}

	roundTripped, err := roundTripHookPayload(input)
	if err != nil {
		return nil, fmt.Errorf("round-trip hook input: %w", err)
	}

	mutated, err := mutate(roundTripped)
	if err != nil {
		return nil, err
	}

	roundTripped, err = roundTripHookPayload(mutated)
	if err != nil {
		return nil, fmt.Errorf("round-trip hook output: %w", err)
	}
	return roundTripped, nil
}

func (m *agentHookManager) DispatchObserverHook(_ context.Context, hook string, payload any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.observerHooks = append(m.observerHooks, hook)
	m.observerPayloads = append(m.observerPayloads, payload)
}

func roundTripHookPayload(value any) (any, error) {
	if value == nil {
		return nil, nil
	}

	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal hook payload: %w", err)
	}

	valueType := reflect.TypeOf(value)
	if valueType.Kind() == reflect.Ptr {
		target := reflect.New(valueType.Elem())
		if err := json.Unmarshal(raw, target.Interface()); err != nil {
			return nil, fmt.Errorf("unmarshal hook payload: %w", err)
		}
		return target.Interface(), nil
	}

	target := reflect.New(valueType)
	if err := json.Unmarshal(raw, target.Interface()); err != nil {
		return nil, fmt.Errorf("unmarshal hook payload: %w", err)
	}
	return target.Elem().Interface(), nil
}
