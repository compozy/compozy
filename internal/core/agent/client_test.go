package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
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

func TestClientCreateSessionStartsAgentProcessInWorkingDirectory(t *testing.T) {
	t.Parallel()

	t.Run("Should start agent process in provided working directory", func(t *testing.T) {
		workingDir := t.TempDir()
		scenario := helperScenario{
			ExpectedProcessCWD: workingDir,
			ExpectedCWD:        workingDir,
			ExpectedPrompt:     "process cwd must match session workspace",
			StopReason:         string(acp.StopReasonEndTurn),
		}

		client := newTestClient(t, scenario)
		t.Cleanup(func() {
			if err := client.Close(); err != nil {
				t.Errorf("close client: %v", err)
			}
		})

		session, err := client.CreateSession(context.Background(), SessionRequest{
			WorkingDir: workingDir,
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
	})
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

func TestClientCreateSessionServesTerminalRequestsFromAgent(t *testing.T) {
	t.Parallel()

	wantExitCode := 0
	scenario := helperScenario{
		ExpectedCWD:          t.TempDir(),
		ExpectedPrompt:       "run a terminal command",
		StopReason:           string(acp.StopReasonEndTurn),
		TerminalCommand:      os.Args[0],
		TerminalArgs:         []string{"-test.run=TestTerminalCommandHelperProcess", "--"},
		TerminalEnv:          terminalHelperEnv("print-exit", "acp-terminal-ok", "0"),
		TerminalWantOutput:   "acp-terminal-ok",
		TerminalWantExitCode: &wantExitCode,
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
		t.Fatalf("updates length = %d, want 1", len(updates))
	}
	if updates[0].Status != model.StatusCompleted {
		t.Fatalf("final status = %q, want completed", updates[0].Status)
	}
	if err := session.Err(); err != nil {
		t.Fatalf("session error = %v, want nil", err)
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
		ExpectedCWD:                t.TempDir(),
		ExpectedPrompt:             "run with full access",
		ExpectedSessionModeID:      "bypassPermissions",
		ExpectedConfigurationOrder: []string{"mode:bypassPermissions"},
		StopReason:                 string(acp.StopReasonEndTurn),
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

func TestClientCreateSessionConfiguresAdvertisedRuntimeBeforePrompt(t *testing.T) {
	t.Parallel()

	modelOption := testSessionSelectOption(
		"model",
		acp.SessionConfigOptionCategoryModel,
		"default[]",
		[]acp.SessionConfigSelectOption{
			{Value: "default[]", Name: "Auto"},
			{Value: "grok-4.5[effort=high,fast=true]", Name: "grok-4.5"},
		},
	)
	effortOption := testSessionSelectOption(
		"effort",
		acp.SessionConfigOptionCategoryThoughtLevel,
		"medium",
		[]acp.SessionConfigSelectOption{
			{Value: "medium", Name: "Medium"},
			{Value: "max", Name: "Max"},
		},
	)
	scenario := helperScenario{
		ExpectedCWD:    t.TempDir(),
		ExpectedPrompt: "configure before prompting",
		NewSessionConfigOptions: []acp.SessionConfigOption{
			{Select: modelOption},
			{Select: effortOption},
		},
		SessionModes: &acp.SessionModeState{
			CurrentModeId: "agent",
			AvailableModes: []acp.SessionMode{
				{Id: "agent", Name: "Agent"},
				{Id: "agent-full-access", Name: "Agent (full access)"},
			},
		},
		ExpectedSessionConfig: []helperExpectedSessionConfig{
			{ConfigID: "model", Value: "grok-4.5[effort=high,fast=true]"},
			{ConfigID: "effort", Value: "max"},
		},
		ExpectedSessionModeID: "agent-full-access",
		ExpectedConfigurationOrder: []string{
			"config:model=grok-4.5[effort=high,fast=true]",
			"config:effort=max",
			"mode:agent-full-access",
		},
		StopReason: string(acp.StopReasonEndTurn),
	}

	client := newTestClientWithSpecConfig(
		t,
		scenario,
		func(spec *Spec) {
			spec.UsesBootstrapModel = false
			spec.FullAccessModeID = "agent-full-access"
		},
		func(cfg *ClientConfig) {
			cfg.Model = "grok-4.5"
			cfg.ReasoningEffort = "max"
			cfg.AccessMode = model.AccessModeFull
		},
	)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close client: %v", err)
		}
	})

	session, err := client.CreateSession(context.Background(), SessionRequest{
		WorkingDir: scenario.ExpectedCWD,
		Prompt:     []byte(scenario.ExpectedPrompt),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	updates := collectSessionUpdates(t, session)
	if len(updates) != 1 || updates[0].Status != model.StatusCompleted {
		t.Fatalf("unexpected updates: %#v", updates)
	}
}

func TestClientCreateSessionForcesFableAutoModeBeforePrompt(t *testing.T) {
	t.Parallel()

	modelOption := testSessionSelectOption(
		"model",
		acp.SessionConfigOptionCategoryModel,
		"opus",
		[]acp.SessionConfigSelectOption{
			{Value: "opus", Name: "Opus"},
			{Value: "claude-fable-5", Name: "Fable 5"},
		},
	)
	effortOption := testSessionSelectOption(
		"effort",
		acp.SessionConfigOptionCategoryThoughtLevel,
		"medium",
		[]acp.SessionConfigSelectOption{
			{Value: "medium", Name: "Medium"},
			{Value: "max", Name: "Max"},
		},
	)
	scenario := helperScenario{
		ExpectedCWD:    t.TempDir(),
		ExpectedPrompt: "run Fable safely",
		NewSessionConfigOptions: []acp.SessionConfigOption{
			{Select: modelOption},
			{Select: effortOption},
		},
		SessionModes: &acp.SessionModeState{
			CurrentModeId: "default",
			AvailableModes: []acp.SessionMode{
				{Id: "default", Name: "Default"},
				{Id: "bypassPermissions", Name: "Bypass permissions"},
				{Id: "auto", Name: "Auto"},
			},
		},
		ExpectedSessionConfig: []helperExpectedSessionConfig{
			{ConfigID: "model", Value: "claude-fable-5"},
			{ConfigID: "effort", Value: "max"},
		},
		ExpectedSessionModeID: "auto",
		ExpectedConfigurationOrder: []string{
			"config:model=claude-fable-5",
			"config:effort=max",
			"mode:auto",
		},
		StopReason: string(acp.StopReasonEndTurn),
	}

	client := newTestClientWithSpecConfig(
		t,
		scenario,
		func(spec *Spec) {
			spec.FullAccessModeID = "bypassPermissions"
		},
		func(cfg *ClientConfig) {
			cfg.Model = "fable"
			cfg.ReasoningEffort = "max"
			cfg.AccessMode = model.AccessModeFull
		},
	)
	client.(*clientImpl).spec.ID = model.IDEClaude
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close client: %v", err)
		}
	})

	session, err := client.CreateSession(context.Background(), SessionRequest{
		WorkingDir: scenario.ExpectedCWD,
		Prompt:     []byte(scenario.ExpectedPrompt),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	updates := collectSessionUpdates(t, session)
	if len(updates) != 1 || updates[0].Status != model.StatusCompleted {
		t.Fatalf("unexpected updates: %#v", updates)
	}
}

func TestClientCreateSessionRejectsUnadvertisedClaudeUltraBeforePrompt(t *testing.T) {
	t.Parallel()

	effortOption := testSessionSelectOption(
		"effort",
		acp.SessionConfigOptionCategoryThoughtLevel,
		"medium",
		[]acp.SessionConfigSelectOption{
			{Value: "medium", Name: "Medium"},
			{Value: "max", Name: "Max"},
		},
	)
	scenario := helperScenario{
		ExpectedCWD: t.TempDir(),
		NewSessionConfigOptions: []acp.SessionConfigOption{
			{Select: effortOption},
		},
	}
	client := newTestClientWithConfig(t, scenario, func(cfg *ClientConfig) {
		cfg.ReasoningEffort = "ultra"
	})
	client.(*clientImpl).spec.ID = model.IDEClaude
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close client: %v", err)
		}
	})

	_, err := client.CreateSession(context.Background(), SessionRequest{WorkingDir: scenario.ExpectedCWD})
	if err == nil {
		t.Fatal("expected unsupported Claude ultra error")
	}
	var setupErr *SessionSetupError
	if !errors.As(err, &setupErr) {
		t.Fatalf("expected SessionSetupError, got %T", err)
	}
	if setupErr.Stage != SessionSetupStageSetReasoning {
		t.Fatalf("setup stage = %q, want %q", setupErr.Stage, SessionSetupStageSetReasoning)
	}
	for _, want := range []string{
		`reasoning effort "ultra" is not available`,
		"Max (max)",
		"Medium (medium)",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("unsupported effort error = %q, want %q", err, want)
		}
	}
}

func TestClientCreateSessionRejectsCursorModelWithoutAdvertisedCatalog(t *testing.T) {
	t.Parallel()

	manager := &agentHookManager{}
	scenario := helperScenario{ExpectedCWD: t.TempDir()}
	client := newTestClientWithConfig(t, scenario, func(cfg *ClientConfig) {
		cfg.Model = "grok-4.5"
	})
	client.(*clientImpl).spec.ID = model.IDECursor
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close client: %v", err)
		}
	})

	_, err := client.CreateSession(context.Background(), SessionRequest{
		WorkingDir: scenario.ExpectedCWD,
		RuntimeMgr: manager,
	})
	if err == nil {
		t.Fatal("expected missing Cursor model catalog error")
	}
	var setupErr *SessionSetupError
	if !errors.As(err, &setupErr) {
		t.Fatalf("expected SessionSetupError, got %T", err)
	}
	if setupErr.Stage != SessionSetupStageSetModel {
		t.Fatalf("setup stage = %q, want %q", setupErr.Stage, SessionSetupStageSetModel)
	}
	if !strings.Contains(
		err.Error(),
		`did not advertise an ACP model option; cannot resolve requested model "grok-4.5"`,
	) {
		t.Fatalf("missing model catalog error = %q", err)
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if got, want := manager.mutableHooks, []string{"agent.pre_session_create"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mutable hooks = %#v, want %#v", got, want)
	}
	if len(manager.observerHooks) != 0 {
		t.Fatalf("configuration failure dispatched post-create hooks: %#v", manager.observerHooks)
	}
}

func TestClientCreateSessionExplainsCursorCLIVersusACPModelNames(t *testing.T) {
	t.Parallel()

	modelOption := testSessionSelectOption(
		"model",
		acp.SessionConfigOptionCategoryModel,
		"default[]",
		[]acp.SessionConfigSelectOption{
			{Value: "default[]", Name: "Auto"},
			{Value: "grok-4.5[effort=high,fast=true]", Name: "grok-4.5"},
		},
	)
	scenario := helperScenario{
		ExpectedCWD:             t.TempDir(),
		NewSessionConfigOptions: []acp.SessionConfigOption{{Select: modelOption}},
	}
	// A Cursor CLI model name reaches this path because the user typed --model, so
	// the request is explicit and must fail loudly instead of falling back.
	client := newTestClientWithConfig(t, scenario, func(cfg *ClientConfig) {
		cfg.Model = "grok-4.5-fast-high"
		cfg.ModelExplicit = true
	})
	client.(*clientImpl).spec.ID = model.IDECursor
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close client: %v", err)
		}
	})

	_, err := client.CreateSession(context.Background(), SessionRequest{WorkingDir: scenario.ExpectedCWD})
	if err == nil {
		t.Fatal("expected invalid Cursor ACP model error")
	}
	for _, want := range []string{
		`model "grok-4.5-fast-high" is not available`,
		"grok-4.5 (grok-4.5[effort=high,fast=true])",
		"Cursor CLI --list-models entries can differ from ACP model IDs",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Cursor model error = %q, want %q", err, want)
		}
	}
}

func TestClientCreateSessionFallsBackWhenInheritedModelIsFromAnotherRuntime(t *testing.T) {
	t.Parallel()

	// Mirrors `compozy exec --agent <id> --ide codex` in a workspace whose default
	// model targets Claude: the runtime never advertises "opus", and the user never
	// asked for it on this invocation.
	modelOption := testSessionSelectOption(
		"model",
		acp.SessionConfigOptionCategoryModel,
		"gpt-5.6-sol[reasoning=medium]",
		[]acp.SessionConfigSelectOption{
			{Value: "gpt-5.6-sol[reasoning=medium]", Name: "gpt-5.6-sol"},
			{Value: "gpt-5.6-terra[reasoning=medium]", Name: "gpt-5.6-terra"},
		},
	)
	scenario := helperScenario{
		ExpectedCWD:             t.TempDir(),
		ExpectedPrompt:          "run the council advisor",
		NewSessionConfigOptions: []acp.SessionConfigOption{{Select: modelOption}},
		ExpectedSessionConfig: []helperExpectedSessionConfig{
			{ConfigID: "model", Value: "gpt-5.6-sol[reasoning=medium]"},
		},
		StopReason: string(acp.StopReasonEndTurn),
	}

	// Codex sets UsesBootstrapModel but still advertises a model option, and the
	// modern codex-acp adapter gets no bootstrap model flag, so the session config
	// is the only place the model is applied.
	client := newTestClientWithConfig(t, scenario, func(cfg *ClientConfig) {
		cfg.Model = "opus"
		cfg.ModelExplicit = false
	})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close client: %v", err)
		}
	})

	session, err := client.CreateSession(context.Background(), SessionRequest{
		Prompt:     []byte(scenario.ExpectedPrompt),
		WorkingDir: scenario.ExpectedCWD,
	})
	if err != nil {
		t.Fatalf("create session with inherited cross-runtime model: %v", err)
	}
	collectSessionUpdates(t, session)
}

func TestInheritedModelFallback(t *testing.T) {
	t.Parallel()

	option := testSessionSelectOption(
		"model",
		acp.SessionConfigOptionCategoryModel,
		"gpt-5.6-sol[reasoning=medium]",
		[]acp.SessionConfigSelectOption{
			{Value: "gpt-5.6-sol[reasoning=medium]", Name: "gpt-5.6-sol"},
		},
	)

	tests := []struct {
		name      string
		option    *acp.SessionConfigOptionSelect
		explicit  bool
		requested string
		want      string
		wantOK    bool
	}{
		{
			name:      "Should fall back to the runtime default for an inherited model",
			option:    option,
			requested: "opus",
			want:      "gpt-5.6-sol[reasoning=medium]",
			wantOK:    true,
		},
		{
			name:      "Should refuse to fall back for an explicitly pinned model",
			option:    option,
			explicit:  true,
			requested: "opus",
		},
		{
			name:      "Should refuse to fall back without an advertised current value",
			option:    testSessionSelectOption("model", acp.SessionConfigOptionCategoryModel, "", nil),
			requested: "opus",
		},
		{
			name:      "Should refuse to fall back when the request already is the current value",
			option:    option,
			requested: "gpt-5.6-sol[reasoning=medium]",
		},
		{
			name:      "Should refuse to fall back without an advertised model option",
			requested: "opus",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &clientImpl{
				spec: Spec{ID: model.IDECodex},
				cfg:  ClientConfig{ModelExplicit: tt.explicit},
			}
			got, ok := client.inheritedModelFallback(tt.option, tt.requested)
			if ok != tt.wantOK {
				t.Fatalf("inheritedModelFallback() ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("inheritedModelFallback() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClientCreateSessionUsesCursorAdvertisedModelAsReasoningAuthority(t *testing.T) {
	t.Parallel()

	modelOption := testSessionSelectOption(
		"model",
		acp.SessionConfigOptionCategoryModel,
		"default[]",
		[]acp.SessionConfigSelectOption{
			{Value: "default[]", Name: "Auto"},
			{Value: "grok-4.5[effort=high,fast=true]", Name: "grok-4.5"},
		},
	)
	scenario := helperScenario{
		ExpectedCWD:             t.TempDir(),
		ExpectedPrompt:          "use Cursor ACP model",
		NewSessionConfigOptions: []acp.SessionConfigOption{{Select: modelOption}},
		ExpectedSessionConfig: []helperExpectedSessionConfig{
			{ConfigID: "model", Value: "grok-4.5[effort=high,fast=true]"},
		},
		ExpectedConfigurationOrder: []string{
			"config:model=grok-4.5[effort=high,fast=true]",
		},
		StopReason: string(acp.StopReasonEndTurn),
	}
	client := newTestClientWithSpecConfig(
		t,
		scenario,
		func(spec *Spec) {
			spec.UsesBootstrapModel = false
		},
		func(cfg *ClientConfig) {
			cfg.Model = "grok-4.5"
			cfg.ReasoningEffort = "ultra"
		},
	)
	client.(*clientImpl).spec.ID = model.IDECursor
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close client: %v", err)
		}
	})

	session, err := client.CreateSession(context.Background(), SessionRequest{
		WorkingDir: scenario.ExpectedCWD,
		Prompt:     []byte(scenario.ExpectedPrompt),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	updates := collectSessionUpdates(t, session)
	if len(updates) != 1 || updates[0].Status != model.StatusCompleted {
		t.Fatalf("unexpected updates: %#v", updates)
	}
}

func TestClientCreateSessionRejectsCodexReasoningWithoutAdvertisedOption(t *testing.T) {
	t.Parallel()

	modelOption := testSessionSelectOption(
		"model",
		acp.SessionConfigOptionCategoryModel,
		"test-model",
		[]acp.SessionConfigSelectOption{{Value: "test-model", Name: "Test model"}},
	)
	scenario := helperScenario{
		ExpectedCWD:             t.TempDir(),
		NewSessionConfigOptions: []acp.SessionConfigOption{{Select: modelOption}},
		ExpectedSessionConfig: []helperExpectedSessionConfig{
			{ConfigID: "model", Value: "test-model"},
		},
	}
	client := newTestClient(t, scenario)
	client.(*clientImpl).spec.ID = model.IDECodex
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close client: %v", err)
		}
	})

	_, err := client.CreateSession(context.Background(), SessionRequest{WorkingDir: scenario.ExpectedCWD})
	if err == nil {
		t.Fatal("expected missing Codex reasoning option error")
	}
	var setupErr *SessionSetupError
	if !errors.As(err, &setupErr) {
		t.Fatalf("expected SessionSetupError, got %T", err)
	}
	if setupErr.Stage != SessionSetupStageSetReasoning {
		t.Fatalf("setup stage = %q, want %q", setupErr.Stage, SessionSetupStageSetReasoning)
	}
	if !strings.Contains(
		err.Error(),
		`did not advertise an ACP reasoning option; cannot apply reasoning effort "medium"`,
	) {
		t.Fatalf("missing reasoning option error = %q", err)
	}
}

func TestClientCreateSessionTreatsAutoModelAsRuntimeDefaultForNonBootstrapACP(t *testing.T) {
	t.Parallel()

	scenario := helperScenario{
		ExpectedCWD:    t.TempDir(),
		ExpectedPrompt: "use runtime default model",
		StopReason:     string(acp.StopReasonEndTurn),
	}
	client := newTestClientWithSpecConfig(
		t,
		scenario,
		func(spec *Spec) {
			spec.DefaultModel = "runtime-default"
			spec.UsesBootstrapModel = false
		},
		func(cfg *ClientConfig) {
			cfg.Model = " AUTO "
		},
	)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close client: %v", err)
		}
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
	if session.Err() != nil {
		t.Fatalf("unexpected session error: %v", session.Err())
	}
}

func TestClientCreateSessionCompletesWithExplicitModelForNonBootstrapACP(t *testing.T) {
	t.Parallel()

	scenario := helperScenario{
		ExpectedCWD:    t.TempDir(),
		ExpectedPrompt: "use explicit model",
		ExpectedSessionConfig: []helperExpectedSessionConfig{
			{ConfigID: "model", Value: "composer-2"},
		},
		ExpectedConfigurationOrder: []string{"config:model=composer-2"},
		StopReason:                 string(acp.StopReasonEndTurn),
	}
	client := newTestClientWithSpecConfig(
		t,
		scenario,
		func(spec *Spec) {
			spec.DefaultModel = "runtime-default"
			spec.UsesBootstrapModel = false
		},
		func(cfg *ClientConfig) {
			cfg.Model = "composer-2"
		},
	)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close client: %v", err)
		}
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
	if session.Err() != nil {
		t.Fatalf("unexpected session error: %v", session.Err())
	}
}

func TestClientCreateSessionSkipsRuntimeModelUpdateForKiro(t *testing.T) {
	t.Parallel()

	t.Run("Should keep an explicit Kiro model pinned at bootstrap", func(t *testing.T) {
		t.Parallel()

		kiroSpec, err := lookupAgentSpec(model.IDEKiro)
		if err != nil {
			t.Fatalf("lookup Kiro spec: %v", err)
		}
		scenario := helperScenario{
			ExpectedCWD:    t.TempDir(),
			ExpectedPrompt: "use explicit Kiro model",
			StopReason:     string(acp.StopReasonEndTurn),
		}
		client := newTestClientWithSpecConfig(
			t,
			scenario,
			func(spec *Spec) {
				spec.DisplayName = kiroSpec.DisplayName
				spec.DefaultModel = kiroSpec.DefaultModel
				spec.UsesBootstrapModel = kiroSpec.UsesBootstrapModel
			},
			func(cfg *ClientConfig) {
				cfg.Model = "claude-sonnet-4.6"
			},
		)
		t.Cleanup(func() {
			if err := client.Close(); err != nil {
				t.Errorf("close client: %v", err)
			}
		})

		session, err := client.CreateSession(context.Background(), SessionRequest{
			WorkingDir: scenario.ExpectedCWD,
			Prompt:     []byte(scenario.ExpectedPrompt),
		})
		if err != nil {
			t.Fatalf("create Kiro session: %v", err)
		}
		updates := collectSessionUpdates(t, session)
		if len(updates) != 1 || updates[0].Status != model.StatusCompleted {
			t.Fatalf("Kiro session updates = %#v, want one completed update", updates)
		}
		if session.Err() != nil {
			t.Fatalf("Kiro session error: %v", session.Err())
		}
	})
}

func TestClientCreateSessionPassesExplicitModelThroughLaunchEnv(t *testing.T) {
	t.Parallel()

	scenario := helperScenario{
		ExpectedCWD:     t.TempDir(),
		ExpectedPrompt:  "use launch env model",
		ExpectedEnvVars: map[string]string{"COMPOZY_TEST_ACP_MODEL": "sonnet"},
		StopReason:      string(acp.StopReasonEndTurn),
	}
	client := newTestClientWithSpecConfig(
		t,
		scenario,
		func(spec *Spec) {
			spec.ModelEnvVar = "COMPOZY_TEST_ACP_MODEL"
		},
		func(cfg *ClientConfig) {
			cfg.Model = "sonnet"
		},
	)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close client: %v", err)
		}
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
	if session.Err() != nil {
		t.Fatalf("unexpected session error: %v", session.Err())
	}
}

func TestClientCreateSessionOmitsLaunchEnvModelForAutoModel(t *testing.T) {
	t.Parallel()

	scenario := helperScenario{
		ExpectedCWD:     t.TempDir(),
		ExpectedPrompt:  "use runtime default model",
		ExpectedEnvVars: map[string]string{"COMPOZY_TEST_ACP_MODEL": ""},
		StopReason:      string(acp.StopReasonEndTurn),
	}
	client := newTestClientWithSpecConfig(
		t,
		scenario,
		func(spec *Spec) {
			spec.ModelEnvVar = "COMPOZY_TEST_ACP_MODEL"
		},
		func(cfg *ClientConfig) {
			cfg.Model = " AUTO "
		},
	)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close client: %v", err)
		}
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
	if session.Err() != nil {
		t.Fatalf("unexpected session error: %v", session.Err())
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

func TestClientResumeSessionConfiguresAdvertisedRuntimeBeforePrompt(t *testing.T) {
	t.Parallel()

	modelOption := testSessionSelectOption(
		"model",
		acp.SessionConfigOptionCategoryModel,
		"gpt-5.5",
		[]acp.SessionConfigSelectOption{
			{Value: "gpt-5.5", Name: "GPT-5.5"},
			{Value: "gpt-5.6-sol", Name: "GPT-5.6 Sol"},
		},
	)
	reasoningOption := testSessionSelectOption(
		"reasoning_effort",
		acp.SessionConfigOptionCategoryThoughtLevel,
		"medium",
		[]acp.SessionConfigSelectOption{
			{Value: "medium", Name: "medium"},
			{Value: "ultra", Name: "ultra"},
		},
	)
	scenario := helperScenario{
		SessionID:             "sess-existing-config",
		ExpectedCWD:           t.TempDir(),
		ExpectedLoadSessionID: "sess-existing-config",
		ExpectedPrompt:        "resume after configuration",
		SupportsLoadSession:   true,
		LoadSessionConfigOptions: []acp.SessionConfigOption{
			{Select: modelOption},
			{Select: reasoningOption},
		},
		ExpectedSessionConfig: []helperExpectedSessionConfig{
			{ConfigID: "model", Value: "gpt-5.6-sol"},
			{ConfigID: "reasoning_effort", Value: "ultra"},
		},
		ExpectedConfigurationOrder: []string{
			"config:model=gpt-5.6-sol",
			"config:reasoning_effort=ultra",
		},
		Updates:    []acp.SessionUpdate{acp.UpdateAgentMessageText("configured")},
		StopReason: string(acp.StopReasonEndTurn),
	}

	client := newTestClientWithConfig(t, scenario, func(cfg *ClientConfig) {
		cfg.Model = "gpt-5.6-sol"
		cfg.ReasoningEffort = "ultra"
	})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close client: %v", err)
		}
	})

	session, err := client.ResumeSession(context.Background(), ResumeSessionRequest{
		SessionID:  scenario.SessionID,
		WorkingDir: scenario.ExpectedCWD,
		Prompt:     []byte(scenario.ExpectedPrompt),
		Model:      "gpt-5.6-sol",
	})
	if err != nil {
		t.Fatalf("resume session: %v", err)
	}
	updates := collectSessionUpdates(t, session)
	if len(updates) != 2 {
		t.Fatalf("unexpected update count: got %d want 2", len(updates))
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

func TestClientTerminalMethodsExecuteCommandAndRetainOutput(t *testing.T) {
	t.Parallel()

	client, sessionID := newTerminalTestClient(t)
	limit := 4
	resp, err := client.CreateTerminal(context.Background(), acp.CreateTerminalRequest{
		SessionId:       acp.SessionId(sessionID),
		Command:         os.Args[0],
		Args:            []string{"-test.run=TestTerminalCommandHelperProcess", "--"},
		Env:             terminalHelperEnv("print-exit", "alpha-beta", "7"),
		OutputByteLimit: &limit,
	})
	if err != nil {
		t.Fatalf("create terminal: %v", err)
	}

	waitResp, err := client.WaitForTerminalExit(context.Background(), acp.WaitForTerminalExitRequest{
		SessionId:  acp.SessionId(sessionID),
		TerminalId: resp.TerminalId,
	})
	if err != nil {
		t.Fatalf("wait for terminal: %v", err)
	}
	if waitResp.ExitCode == nil || *waitResp.ExitCode != 7 {
		t.Fatalf("terminal exit code = %#v, want 7", waitResp.ExitCode)
	}

	output, err := client.TerminalOutput(context.Background(), acp.TerminalOutputRequest{
		SessionId:  acp.SessionId(sessionID),
		TerminalId: resp.TerminalId,
	})
	if err != nil {
		t.Fatalf("terminal output: %v", err)
	}
	if output.Output != "beta" || !output.Truncated {
		t.Fatalf("terminal output = %#v truncated=%v, want beta truncated", output.Output, output.Truncated)
	}
	if output.ExitStatus == nil || output.ExitStatus.ExitCode == nil || *output.ExitStatus.ExitCode != 7 {
		t.Fatalf("terminal output exit status = %#v, want exit code 7", output.ExitStatus)
	}
	if _, err := client.ReleaseTerminal(context.Background(), acp.ReleaseTerminalRequest{
		SessionId:  acp.SessionId(sessionID),
		TerminalId: resp.TerminalId,
	}); err != nil {
		t.Fatalf("release terminal: %v", err)
	}
}

func TestClientTerminalRejectsCWDOutsideSessionRoots(t *testing.T) {
	t.Parallel()

	client, sessionID := newTerminalTestClient(t)
	outside := t.TempDir()
	_, err := client.CreateTerminal(context.Background(), acp.CreateTerminalRequest{
		SessionId: acp.SessionId(sessionID),
		Command:   os.Args[0],
		Args:      []string{"-test.run=TestTerminalCommandHelperProcess", "--"},
		Cwd:       &outside,
		Env:       terminalHelperEnv("print-exit", "outside", "0"),
	})
	if err == nil || !strings.Contains(err.Error(), "outside allowed session roots") {
		t.Fatalf("CreateTerminal() error = %v, want outside allowed roots", err)
	}
}

func TestClientTerminalKillTerminatesCommandAndKeepsOutput(t *testing.T) {
	t.Parallel()

	client, sessionID := newTerminalTestClient(t)
	resp, err := client.CreateTerminal(context.Background(), acp.CreateTerminalRequest{
		SessionId: acp.SessionId(sessionID),
		Command:   os.Args[0],
		Args:      []string{"-test.run=TestTerminalCommandHelperProcess", "--"},
		Env:       terminalHelperEnv("block", "ready", "0"),
	})
	if err != nil {
		t.Fatalf("create terminal: %v", err)
	}
	waitForTerminalOutput(t, client, sessionID, resp.TerminalId, "ready")

	if _, err := client.KillTerminal(context.Background(), acp.KillTerminalRequest{
		SessionId:  acp.SessionId(sessionID),
		TerminalId: resp.TerminalId,
	}); err != nil {
		t.Fatalf("kill terminal: %v", err)
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := client.WaitForTerminalExit(waitCtx, acp.WaitForTerminalExitRequest{
		SessionId:  acp.SessionId(sessionID),
		TerminalId: resp.TerminalId,
	}); err != nil {
		t.Fatalf("wait for killed terminal: %v", err)
	}
	output, err := client.TerminalOutput(context.Background(), acp.TerminalOutputRequest{
		SessionId:  acp.SessionId(sessionID),
		TerminalId: resp.TerminalId,
	})
	if err != nil {
		t.Fatalf("terminal output after kill: %v", err)
	}
	if !strings.Contains(output.Output, "ready") {
		t.Fatalf("terminal output after kill = %q, want ready", output.Output)
	}
	if _, err := client.ReleaseTerminal(context.Background(), acp.ReleaseTerminalRequest{
		SessionId:  acp.SessionId(sessionID),
		TerminalId: resp.TerminalId,
	}); err != nil {
		t.Fatalf("release killed terminal: %v", err)
	}
}

func TestClientTerminalKillTerminatesChildProcessTree(t *testing.T) {
	t.Parallel()

	t.Run("Should terminate child process tree when terminal is killed", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("process-group terminal cleanup is implemented differently on Windows")
		}

		client, sessionID := newTerminalTestClient(t)
		childPIDPath := filepath.Join(t.TempDir(), "child.pid")
		env := append(
			terminalHelperEnv("spawn-child", "tree-ready", "0"),
			acp.EnvVariable{Name: "GO_TERMINAL_CHILD_PID_FILE", Value: childPIDPath},
		)
		resp, err := client.CreateTerminal(context.Background(), acp.CreateTerminalRequest{
			SessionId: acp.SessionId(sessionID),
			Command:   os.Args[0],
			Args:      []string{"-test.run=TestTerminalCommandHelperProcess", "--"},
			Env:       env,
		})
		if err != nil {
			t.Fatalf("create terminal: %v", err)
		}
		waitForTerminalOutput(t, client, sessionID, resp.TerminalId, "tree-ready")
		childPID := readTerminalChildPID(t, childPIDPath)
		defer killProcessByPID(childPID)

		if _, err := client.KillTerminal(context.Background(), acp.KillTerminalRequest{
			SessionId:  acp.SessionId(sessionID),
			TerminalId: resp.TerminalId,
		}); err != nil {
			t.Fatalf("kill terminal: %v", err)
		}
		waitCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if _, err := client.WaitForTerminalExit(waitCtx, acp.WaitForTerminalExitRequest{
			SessionId:  acp.SessionId(sessionID),
			TerminalId: resp.TerminalId,
		}); err != nil {
			t.Fatalf("wait for killed terminal: %v", err)
		}
		if _, err := client.ReleaseTerminal(context.Background(), acp.ReleaseTerminalRequest{
			SessionId:  acp.SessionId(sessionID),
			TerminalId: resp.TerminalId,
		}); err != nil {
			t.Fatalf("release killed terminal: %v", err)
		}
		waitForProcessExit(t, childPID)
	})
}

func TestClientReleaseTerminalRetainsTrackingWhenWaitContextExpires(t *testing.T) {
	t.Parallel()

	client, sessionID := newTerminalTestClient(t)
	done := make(chan struct{})
	terminal := &terminalProcess{
		id:        "term-timeout",
		sessionID: sessionID,
		cancel:    func() {},
		done:      done,
		output:    newTerminalOutputBuffer(nil),
	}
	client.storeTerminal(terminal)

	releaseCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.ReleaseTerminal(releaseCtx, acp.ReleaseTerminalRequest{
		SessionId:  acp.SessionId(sessionID),
		TerminalId: terminal.id,
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("ReleaseTerminal(canceled) error = %v, want context.Canceled", err)
	}
	if _, err := client.lookupTerminal(acp.SessionId(sessionID), terminal.id); err != nil {
		t.Fatalf("lookup terminal after canceled release: %v", err)
	}

	close(done)
	if _, err := client.ReleaseTerminal(context.Background(), acp.ReleaseTerminalRequest{
		SessionId:  acp.SessionId(sessionID),
		TerminalId: terminal.id,
	}); err != nil {
		t.Fatalf("release terminal after wait: %v", err)
	}
	if _, err := client.lookupTerminal(acp.SessionId(sessionID), terminal.id); err == nil {
		t.Fatal("lookup terminal after successful release = nil error, want unknown terminal")
	}
}

func TestNewTerminalOutputBufferAppliesServerDefaultWhenLimitUnset(t *testing.T) {
	t.Parallel()

	zero := 0
	negative := -1
	tests := []struct {
		name      string
		limit     *int
		wantLimit int
	}{
		{
			name:      "Should use the server default when the limit is nil",
			wantLimit: defaultOutputByteLimit,
		},
		{
			name:      "Should use the server default when the limit is zero",
			limit:     &zero,
			wantLimit: defaultOutputByteLimit,
		},
		{
			name:      "Should use the server default when the limit is negative",
			limit:     &negative,
			wantLimit: defaultOutputByteLimit,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buffer := newTerminalOutputBuffer(tt.limit)
			if buffer.limit != tt.wantLimit {
				t.Fatalf("buffer.limit = %d, want %d", buffer.limit, tt.wantLimit)
			}
		})
	}
}

func TestClientCloseCleansUpActiveTerminals(t *testing.T) {
	t.Parallel()

	client, sessionID := newTerminalTestClient(t)
	resp, err := client.CreateTerminal(context.Background(), acp.CreateTerminalRequest{
		SessionId: acp.SessionId(sessionID),
		Command:   os.Args[0],
		Args:      []string{"-test.run=TestTerminalCommandHelperProcess", "--"},
		Env:       terminalHelperEnv("block", "ready", "0"),
	})
	if err != nil {
		t.Fatalf("create terminal: %v", err)
	}
	waitForTerminalOutput(t, client, sessionID, resp.TerminalId, "ready")
	terminal, err := client.lookupTerminal(acp.SessionId(sessionID), resp.TerminalId)
	if err != nil {
		t.Fatalf("lookup terminal: %v", err)
	}

	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
	select {
	case <-terminal.done:
	case <-time.After(2 * time.Second):
		t.Fatal("terminal still running after client close")
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

	authWrapped := wrapACPError(&acp.RequestError{
		Code:    -32000,
		Message: "Authentication required",
		Data:    map[string]any{"message": "Authentication required"},
	})
	var authErr *AuthenticationRequiredError
	if !errors.As(authWrapped, &authErr) {
		t.Fatalf("expected AuthenticationRequiredError, got %T", authWrapped)
	}
	var authSessionErr *SessionError
	if !errors.As(authWrapped, &authSessionErr) || authSessionErr.Code != -32000 {
		t.Fatalf("expected wrapped SessionError, got %#v", authWrapped)
	}
	if !IsAuthenticationRequired(authWrapped) {
		t.Fatal("expected IsAuthenticationRequired to match typed auth error")
	}

	noDataErr := (&SessionError{Code: 7, Message: "plain"}).Error()
	if !strings.Contains(noDataErr, "plain") {
		t.Fatalf("unexpected no-data session error string: %q", noDataErr)
	}
}

func TestClientSetupDiagnosticsPreserveContextCause(t *testing.T) {
	t.Parallel()

	cause := errors.New("activity timeout sentinel")
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(cause)
	client := &clientImpl{
		spec:         Spec{ID: "test-acp", DisplayName: "Test ACP"},
		startCommand: []string{"test-acp", "acp"},
	}

	err := client.wrapACPSetupErrorWithDiagnostics(
		ctx,
		SessionSetupStageNewSession,
		"create ACP session",
		&acp.RequestError{
			Code:    -32603,
			Message: "Internal error",
			Data:    map[string]any{"error": "activity timeout: no output received for 10m0s"},
		},
	)
	if !errors.Is(err, cause) {
		t.Fatalf("wrapped setup error does not preserve context cause: %v", err)
	}
	var setupErr *SessionSetupError
	if !errors.As(err, &setupErr) {
		t.Fatalf("expected SessionSetupError, got %T", err)
	}
	if setupErr.Stage != SessionSetupStageNewSession {
		t.Fatalf("setup stage = %q, want %q", setupErr.Stage, SessionSetupStageNewSession)
	}
	var sessionErr *SessionError
	if !errors.As(err, &sessionErr) || sessionErr.Code != -32603 {
		t.Fatalf("expected wrapped SessionError, got %#v", err)
	}
}

func TestACPLaunchErrorPreservesCauseAndFormatsDiagnostics(t *testing.T) {
	t.Parallel()

	wrapped := wrapACPLaunchError(
		Spec{
			ID:          "test-acp",
			DisplayName: "Test ACP",
			InstallHint: "Install the test adapter.",
			DocsURL:     "https://example.com/acp",
		},
		[]string{"test-acp", "acp"},
		"post-initialize stderr",
		"create ACP session",
		&SessionError{Code: -32603, Message: "Internal error"},
	)
	for _, want := range []string{
		"create ACP session while running test-acp acp",
		"post-initialize stderr",
	} {
		if !strings.Contains(wrapped.Error(), want) {
			t.Fatalf("launch error = %q, want %q", wrapped, want)
		}
	}
	for _, notWant := range []string{"Install the test adapter.", "https://example.com/acp"} {
		if strings.Contains(wrapped.Error(), notWant) {
			t.Fatalf("launch error = %q, should not include post-initialize install guidance %q", wrapped, notWant)
		}
	}
	var sessionErr *SessionError
	if !errors.As(wrapped, &sessionErr) || sessionErr.Code != -32603 {
		t.Fatalf("expected wrapped SessionError, got %#v", wrapped)
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

func TestClientCreateSessionSurfacesNewSessionCommandAndTypedError(t *testing.T) {
	workingDir := t.TempDir()
	scenario := helperScenario{
		ExpectedCWD: workingDir,
		NewSessionError: &helperRequestError{
			Code:    -32603,
			Message: "Internal error",
			Data:    json.RawMessage("{\"error\":\"activity timeout: no output received for 10m0s\"}"),
		},
	}
	client := newTestClient(t, scenario)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close client: %v", err)
		}
	})

	_, err := client.CreateSession(context.Background(), SessionRequest{
		WorkingDir: workingDir,
		Prompt:     []byte("hello"),
	})
	if err == nil {
		t.Fatal("expected create session error")
	}
	for _, want := range []string{
		"new_session",
		"create ACP session while running",
		"TestACPHelperProcess",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("new-session error = %q, want %q", err, want)
		}
	}
	var setupErr *SessionSetupError
	if !errors.As(err, &setupErr) || setupErr.Stage != SessionSetupStageNewSession {
		t.Fatalf("expected new-session setup error, got %#v", err)
	}
	var sessionErr *SessionError
	if !errors.As(err, &sessionErr) || sessionErr.Code != -32603 {
		t.Fatalf("expected wrapped SessionError, got %#v", err)
	}
}

func TestClientAgentProcessExitErrorDiagnosesCodexCodeModeOOM(t *testing.T) {
	t.Parallel()

	process, err := subprocess.Launch(context.Background(), subprocess.LaunchConfig{
		Command: []string{os.Args[0], "-test.run=TestProcessStderrHelperProcess", "--"},
		Env: subprocess.MergeEnvironment(nil, map[string]string{
			"GO_WANT_PROCESS_STDERR_HELPER": "1",
			"GO_PROCESS_STDERR_OUTPUT":      "Fatal process out of memory: Failed to reserve virtual memory for CodeRange",
			"GO_PROCESS_STDERR_EXIT_CODE":   "0",
		}),
		WorkingDir:      t.TempDir(),
		WaitErrorPrefix: "wait for test ACP process",
	})
	if err != nil {
		t.Fatalf("launch helper process: %v", err)
	}
	if err := process.Wait(); err != nil {
		t.Fatalf("wait helper process: %v", err)
	}

	client := &clientImpl{
		process: process,
		sessions: map[string]*sessionImpl{
			"sess-oom": newSession("sess-oom"),
		},
	}
	got := client.agentProcessExitError("ACP agent process exited before all sessions completed", nil).Error()
	for _, want := range []string{
		"open_sessions=1",
		"Failed to reserve virtual memory for CodeRange",
		"Codex Code Mode runtime crashed",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("agentProcessExitError() = %q, want %q", got, want)
		}
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
		"gpt-5.5 requires @agentclientprotocol/codex-acp >= 1.1.2",
		"legacy @zed-industries/codex-acp >= 0.12.0",
		"@zed-industries/codex-acp 0.11.1",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("compatibility error = %q, want %q", err, want)
		}
	}
}

func TestClientRunPromptPublishesUsageUpdateOnEndTurn(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	scenario := helperScenario{
		ExpectedCWD:    workingDir,
		ExpectedPrompt: "hello with usage",
		StopReason:     string(acp.StopReasonEndTurn),
		PromptUsage:    &acp.Usage{InputTokens: 100, OutputTokens: 50, TotalTokens: 150},
	}

	client := newTestClient(t, scenario)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close client: %v", err)
		}
	})

	session, err := client.CreateSession(context.Background(), SessionRequest{
		WorkingDir: workingDir,
		Prompt:     []byte(scenario.ExpectedPrompt),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	updates := collectSessionUpdates(t, session)
	if session.Err() != nil {
		t.Fatalf("unexpected session error: %v", session.Err())
	}

	wantUsage := model.Usage{InputTokens: 100, OutputTokens: 50, TotalTokens: 150}
	var gotUsageUpdate bool
	for _, u := range updates {
		if u.Usage == wantUsage {
			gotUsageUpdate = true
			break
		}
	}
	if !gotUsageUpdate {
		t.Fatalf("expected at least one session update with usage %+v, got updates: %+v", wantUsage, updates)
	}
}

func TestClientRunPromptDoesNotPublishUsageUpdateWhenNoneReturned(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	scenario := helperScenario{
		ExpectedCWD:    workingDir,
		ExpectedPrompt: "hello without usage",
		StopReason:     string(acp.StopReasonEndTurn),
		// PromptUsage intentionally nil
	}

	client := newTestClient(t, scenario)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close client: %v", err)
		}
	})

	session, err := client.CreateSession(context.Background(), SessionRequest{
		WorkingDir: workingDir,
		Prompt:     []byte(scenario.ExpectedPrompt),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	updates := collectSessionUpdates(t, session)
	if session.Err() != nil {
		t.Fatalf("unexpected session error: %v", session.Err())
	}

	for _, u := range updates {
		if u.Usage != (model.Usage{}) {
			t.Fatalf("expected no usage update, got update with usage %+v", u.Usage)
		}
	}
}

func TestClientRunPromptPublishesUsageWithThoughtTokensSummedIntoOutput(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	scenario := helperScenario{
		ExpectedCWD:    workingDir,
		ExpectedPrompt: "hello with thought tokens",
		StopReason:     string(acp.StopReasonEndTurn),
		PromptUsage: &acp.Usage{
			InputTokens:   200,
			OutputTokens:  40,
			TotalTokens:   250,
			ThoughtTokens: acp.Ptr(35),
		},
	}

	client := newTestClient(t, scenario)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close client: %v", err)
		}
	})

	session, err := client.CreateSession(context.Background(), SessionRequest{
		WorkingDir: workingDir,
		Prompt:     []byte(scenario.ExpectedPrompt),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	updates := collectSessionUpdates(t, session)
	if session.Err() != nil {
		t.Fatalf("unexpected session error: %v", session.Err())
	}

	// ThoughtTokens(35) must be summed into OutputTokens: 40 + 35 = 75.
	wantUsage := model.Usage{InputTokens: 200, OutputTokens: 75, TotalTokens: 250}
	var gotUsage bool
	for _, u := range updates {
		if u.Usage == wantUsage {
			gotUsage = true
			break
		}
	}
	if !gotUsage {
		t.Fatalf(
			"expected usage update with thought tokens summed into output, want %+v, got updates: %+v",
			wantUsage,
			updates,
		)
	}
}

func TestClientRunPromptDoesNotPublishUsageUpdateOnCancelledRun(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	scenario := helperScenario{
		ExpectedCWD: workingDir,
		StopReason:  string(acp.StopReasonCancelled),
		PromptUsage: &acp.Usage{InputTokens: 100, OutputTokens: 50, TotalTokens: 150},
	}

	client := newTestClient(t, scenario)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close client: %v", err)
		}
	})

	session, err := client.CreateSession(context.Background(), SessionRequest{
		WorkingDir: workingDir,
		Prompt:     []byte("hello canceled"),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	updates := collectSessionUpdates(t, session)
	// session.Err() is context.Canceled for canceled runs — not checked here

	for _, u := range updates {
		if u.Usage != (model.Usage{}) {
			t.Fatalf("expected no usage update for canceled run, got update with usage %+v", u.Usage)
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
	if scenario.ExpectedProcessCWD != "" {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "get helper cwd: %v\n", err)
			os.Exit(2)
		}
		if canonicalTestPath(cwd) != canonicalTestPath(scenario.ExpectedProcessCWD) {
			fmt.Fprintf(os.Stderr, "unexpected helper cwd %q, want %q\n", cwd, scenario.ExpectedProcessCWD)
			os.Exit(2)
		}
	}
	for key, want := range scenario.ExpectedEnvVars {
		if got := os.Getenv(key); got != want {
			fmt.Fprintf(os.Stderr, "unexpected helper env %s=%q, want %q\n", key, got, want)
			os.Exit(2)
		}
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

func TestTerminalCommandHelperProcess(_ *testing.T) {
	if os.Getenv("GO_WANT_TERMINAL_HELPER_PROCESS") != "1" {
		return
	}
	if os.Getenv("GO_TERMINAL_HELPER_MODE") == "spawn-child" {
		child := exec.CommandContext(context.Background(), os.Args[0], "-test.run=TestTerminalChildHelperProcess", "--")
		child.Env = append(os.Environ(), "GO_WANT_TERMINAL_CHILD_HELPER=1")
		if err := child.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "start child helper: %v\n", err)
			os.Exit(2)
		}
		pidPath := os.Getenv("GO_TERMINAL_CHILD_PID_FILE")
		if pidPath != "" {
			if err := os.WriteFile(pidPath, []byte(strconv.Itoa(child.Process.Pid)), 0o600); err != nil {
				fmt.Fprintf(os.Stderr, "write child pid: %v\n", err)
				os.Exit(2)
			}
		}
		fmt.Print(os.Getenv("GO_TERMINAL_HELPER_OUTPUT"))
		blockTerminalHelperUntilSignalled()
	}
	fmt.Print(os.Getenv("GO_TERMINAL_HELPER_OUTPUT"))
	if os.Getenv("GO_TERMINAL_HELPER_MODE") == "block" {
		blockTerminalHelperUntilSignalled()
	}
	code, err := strconv.Atoi(os.Getenv("GO_TERMINAL_HELPER_EXIT_CODE"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse terminal helper exit code: %v\n", err)
		os.Exit(2)
	}
	os.Exit(code)
}

func TestTerminalChildHelperProcess(_ *testing.T) {
	if os.Getenv("GO_WANT_TERMINAL_CHILD_HELPER") != "1" {
		return
	}
	blockTerminalHelperUntilSignalled()
}

// blockTerminalHelperUntilSignalled keeps a terminal helper process genuinely
// running (never emitting output, never exiting) so lifecycle tests must
// terminate it with a signal. It sleeps in a loop rather than using select{}
// because Go's deadlock detector ignores timer-parked goroutines but aborts a
// process whose only goroutines are blocked with no pending timers.
func blockTerminalHelperUntilSignalled() {
	for i := 0; i < 600; i++ {
		time.Sleep(time.Second)
	}
}

func TestProcessStderrHelperProcess(_ *testing.T) {
	if os.Getenv("GO_WANT_PROCESS_STDERR_HELPER") != "1" {
		return
	}
	fmt.Fprint(os.Stderr, os.Getenv("GO_PROCESS_STDERR_OUTPUT"))
	code, err := strconv.Atoi(firstNonEmpty(os.Getenv("GO_PROCESS_STDERR_EXIT_CODE"), "0"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse process stderr helper exit code: %v\n", err)
		os.Exit(2)
	}
	os.Exit(code)
}

type helperScenario struct {
	SessionID                     string                        `json:"session_id,omitempty"`
	ExpectedCWD                   string                        `json:"expected_cwd,omitempty"`
	ExpectedProcessCWD            string                        `json:"expected_process_cwd,omitempty"`
	ExpectedLoadSessionID         string                        `json:"expected_load_session_id,omitempty"`
	ExpectedNewSessionMCPServers  []acp.McpServer               `json:"expected_new_session_mcp_servers,omitempty"`
	ExpectedLoadSessionMCPServers []acp.McpServer               `json:"expected_load_session_mcp_servers,omitempty"`
	ExpectedPrompt                string                        `json:"expected_prompt,omitempty"`
	ExpectedSessionModeID         string                        `json:"expected_session_mode_id,omitempty"`
	NewSessionConfigOptions       []acp.SessionConfigOption     `json:"new_session_config_options,omitempty"`
	LoadSessionConfigOptions      []acp.SessionConfigOption     `json:"load_session_config_options,omitempty"`
	SessionModes                  *acp.SessionModeState         `json:"session_modes,omitempty"`
	ExpectedSessionConfig         []helperExpectedSessionConfig `json:"expected_session_config,omitempty"`
	ExpectedConfigurationOrder    []string                      `json:"expected_configuration_order,omitempty"`
	// ExpectedEnvVars asserts process environment values at helper startup.
	// An empty value means the variable must be unset or empty.
	ExpectedEnvVars         map[string]string   `json:"expected_env_vars,omitempty"`
	UpdateIntervalMillis    int                 `json:"update_interval_millis,omitempty"`
	SupportsLoadSession     bool                `json:"supports_load_session,omitempty"`
	SessionMeta             map[string]any      `json:"session_meta,omitempty"`
	NewSessionUpdates       []acp.SessionUpdate `json:"new_session_updates,omitempty"`
	ReplayUpdatesOnLoad     []acp.SessionUpdate `json:"replay_updates_on_load,omitempty"`
	Updates                 []acp.SessionUpdate `json:"updates,omitempty"`
	StopReason              string              `json:"stop_reason,omitempty"`
	BlockUntilCancel        bool                `json:"block_until_cancel,omitempty"`
	NewSessionError         *helperRequestError `json:"new_session_error,omitempty"`
	PromptError             *helperRequestError `json:"prompt_error,omitempty"`
	PromptErrorAfterUpdates bool                `json:"prompt_error_after_updates,omitempty"`
	TerminalCommand         string              `json:"terminal_command,omitempty"`
	TerminalArgs            []string            `json:"terminal_args,omitempty"`
	TerminalEnv             []acp.EnvVariable   `json:"terminal_env,omitempty"`
	TerminalWantOutput      string              `json:"terminal_want_output,omitempty"`
	TerminalWantExitCode    *int                `json:"terminal_want_exit_code,omitempty"`
	PromptUsage             *acp.Usage          `json:"prompt_usage,omitempty"`
}

type helperExpectedSessionConfig struct {
	ConfigID string `json:"config_id"`
	Value    string `json:"value"`
}

type helperRequestError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type helperAgent struct {
	conn               *acp.AgentSideConnection
	connReady          chan struct{}
	scenario           helperScenario
	sessionID          string
	configurationMu    sync.Mutex
	configurationOrder []string
	configuredValues   []helperExpectedSessionConfig
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
		SessionId:     acp.SessionId(a.sessionID),
		Meta:          a.scenario.SessionMeta,
		ConfigOptions: a.scenario.NewSessionConfigOptions,
		Modes:         a.scenario.SessionModes,
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
	return acp.LoadSessionResponse{
		Meta:          a.scenario.SessionMeta,
		ConfigOptions: a.scenario.LoadSessionConfigOptions,
		Modes:         a.scenario.SessionModes,
	}, nil
}

func (a *helperAgent) Authenticate(context.Context, acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, nil
}

func (a *helperAgent) Prompt(ctx context.Context, req acp.PromptRequest) (acp.PromptResponse, error) {
	a.configurationMu.Lock()
	configurationOrder := append([]string(nil), a.configurationOrder...)
	a.configurationMu.Unlock()
	if !reflect.DeepEqual(configurationOrder, a.scenario.ExpectedConfigurationOrder) {
		return acp.PromptResponse{}, &acp.RequestError{
			Code: 4005,
			Message: fmt.Sprintf(
				"configuration order = %#v, want %#v",
				configurationOrder,
				a.scenario.ExpectedConfigurationOrder,
			),
		}
	}
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

	if a.scenario.TerminalCommand != "" {
		if err := a.runTerminalRequest(ctx, req.SessionId); err != nil {
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
	return acp.PromptResponse{StopReason: stopReason, Usage: a.scenario.PromptUsage}, nil
}

func (a *helperAgent) runTerminalRequest(ctx context.Context, sessionID acp.SessionId) error {
	terminalResp, err := a.connection().CreateTerminal(ctx, acp.CreateTerminalRequest{
		SessionId: sessionID,
		Command:   a.scenario.TerminalCommand,
		Args:      append([]string(nil), a.scenario.TerminalArgs...),
		Env:       append([]acp.EnvVariable(nil), a.scenario.TerminalEnv...),
	})
	if err != nil {
		return fmt.Errorf("create helper terminal: %w", err)
	}
	defer func() {
		_, _ = a.connection().ReleaseTerminal(context.Background(), acp.ReleaseTerminalRequest{
			SessionId:  sessionID,
			TerminalId: terminalResp.TerminalId,
		})
	}()

	waitResp, err := a.connection().WaitForTerminalExit(ctx, acp.WaitForTerminalExitRequest{
		SessionId:  sessionID,
		TerminalId: terminalResp.TerminalId,
	})
	if err != nil {
		return fmt.Errorf("wait for helper terminal: %w", err)
	}
	if a.scenario.TerminalWantExitCode != nil &&
		(waitResp.ExitCode == nil || *waitResp.ExitCode != *a.scenario.TerminalWantExitCode) {
		return fmt.Errorf(
			"helper terminal exit code = %#v, want %d",
			waitResp.ExitCode,
			*a.scenario.TerminalWantExitCode,
		)
	}
	outputResp, err := a.connection().TerminalOutput(ctx, acp.TerminalOutputRequest{
		SessionId:  sessionID,
		TerminalId: terminalResp.TerminalId,
	})
	if err != nil {
		return fmt.Errorf("read helper terminal output: %w", err)
	}
	if a.scenario.TerminalWantOutput != "" && !strings.Contains(outputResp.Output, a.scenario.TerminalWantOutput) {
		return fmt.Errorf("helper terminal output = %q, want %q", outputResp.Output, a.scenario.TerminalWantOutput)
	}
	return nil
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

func (*helperAgent) Logout(context.Context, acp.LogoutRequest) (acp.LogoutResponse, error) {
	return acp.LogoutResponse{}, nil
}

func (*helperAgent) CloseSession(context.Context, acp.CloseSessionRequest) (acp.CloseSessionResponse, error) {
	return acp.CloseSessionResponse{}, nil
}

func (*helperAgent) ListSessions(context.Context, acp.ListSessionsRequest) (acp.ListSessionsResponse, error) {
	return acp.ListSessionsResponse{}, nil
}

func (*helperAgent) ResumeSession(context.Context, acp.ResumeSessionRequest) (acp.ResumeSessionResponse, error) {
	return acp.ResumeSessionResponse{}, nil
}

func (a *helperAgent) SetSessionConfigOption(
	_ context.Context,
	req acp.SetSessionConfigOptionRequest,
) (acp.SetSessionConfigOptionResponse, error) {
	if req.ValueId == nil {
		return acp.SetSessionConfigOptionResponse{}, &acp.RequestError{
			Code:    4006,
			Message: "expected string session config value",
		}
	}
	configured := helperExpectedSessionConfig{
		ConfigID: string(req.ValueId.ConfigId),
		Value:    string(req.ValueId.Value),
	}
	a.configurationMu.Lock()
	defer a.configurationMu.Unlock()
	index := len(a.configuredValues)
	if index >= len(a.scenario.ExpectedSessionConfig) || a.scenario.ExpectedSessionConfig[index] != configured {
		return acp.SetSessionConfigOptionResponse{}, &acp.RequestError{
			Code:    4007,
			Message: fmt.Sprintf("unexpected session config %#v at index %d", configured, index),
		}
	}
	a.configuredValues = append(a.configuredValues, configured)
	a.configurationOrder = append(
		a.configurationOrder,
		fmt.Sprintf("config:%s=%s", configured.ConfigID, configured.Value),
	)
	return acp.SetSessionConfigOptionResponse{ConfigOptions: []acp.SessionConfigOption{}}, nil
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
	a.configurationMu.Lock()
	a.configurationOrder = append(a.configurationOrder, "mode:"+string(req.ModeId))
	a.configurationMu.Unlock()
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

	return newTestClientWithSpecConfig(t, scenario, nil, configure)
}

func newTestClientWithSpecConfig(
	t *testing.T,
	scenario helperScenario,
	configureSpec func(*Spec),
	configure func(*ClientConfig),
) Client {
	t.Helper()

	scenarioJSON, err := json.Marshal(scenario)
	if err != nil {
		t.Fatalf("marshal helper scenario: %v", err)
	}

	ide := "test-acp-" + sanitizeTestName(t.Name())
	spec := Spec{
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
	}
	if configureSpec != nil {
		configureSpec(&spec)
	}
	registerTestSpec(t, spec)

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

func newTerminalTestClient(t *testing.T) (*clientImpl, string) {
	t.Helper()
	workingDir := t.TempDir()
	sessionID := "sess-terminal"
	return &clientImpl{
		shutdownTimeout: time.Second,
		sessions: map[string]*sessionImpl{
			sessionID: newSessionWithAccess(sessionID, workingDir, []string{workingDir}),
		},
	}, sessionID
}

func terminalHelperEnv(mode string, output string, exitCode string) []acp.EnvVariable {
	return []acp.EnvVariable{
		{Name: "GO_WANT_TERMINAL_HELPER_PROCESS", Value: "1"},
		{Name: "GO_TERMINAL_HELPER_MODE", Value: mode},
		{Name: "GO_TERMINAL_HELPER_OUTPUT", Value: output},
		{Name: "GO_TERMINAL_HELPER_EXIT_CODE", Value: exitCode},
	}
}

func waitForTerminalOutput(
	t *testing.T,
	client *clientImpl,
	sessionID string,
	terminalID string,
	want string,
) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for {
		output, err := client.TerminalOutput(context.Background(), acp.TerminalOutputRequest{
			SessionId:  acp.SessionId(sessionID),
			TerminalId: terminalID,
		})
		if err != nil {
			t.Fatalf("terminal output: %v", err)
		}
		if strings.Contains(output.Output, want) {
			return
		}
		select {
		case <-ticker.C:
		case <-timer.C:
			t.Fatalf("timed out waiting for terminal output %q, last output %q", want, output.Output)
		}
	}
}

func readTerminalChildPID(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read child pid file: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("parse child pid %q: %v", string(data), err)
	}
	return pid
}

func waitForProcessExit(t *testing.T, pid int) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for {
		if !processExists(pid) {
			return
		}
		select {
		case <-ticker.C:
		case <-timer.C:
			t.Fatalf("timed out waiting for child process %d to exit", pid)
		}
	}
}

func processExists(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		return false
	}
	return !processIsZombie(pid)
}

func processIsZombie(pid int) bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ps", "-p", strconv.Itoa(pid), "-o", "stat=").Output()
	if err != nil {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(string(out)), "Z")
}

func killProcessByPID(pid int) {
	process, err := os.FindProcess(pid)
	if err == nil {
		_ = process.Kill()
	}
}

func canonicalTestPath(path string) string {
	clean := filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(clean)
	if err != nil {
		return clean
	}
	return filepath.Clean(resolved)
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
	if valueType.Kind() == reflect.Pointer {
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
