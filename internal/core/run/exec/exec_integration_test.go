package exec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	acp "github.com/coder/acp-go-sdk"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/core/agent"
	reusableagents "github.com/compozy/compozy/internal/core/agents"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/internal/acpshared"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestExecuteExecTextModePrintsOnlyFinalAssistantResponse(t *testing.T) {
	tmpDir := t.TempDir()
	installACPHelperOnPath(t, []runACPHelperScenario{{
		ExpectedPromptContains: "finish the task",
		Updates: []acp.SessionUpdate{
			acp.UpdateAgentMessageText("final answer"),
		},
	}})

	stdout, stderr, execErr := captureExecuteStreams(t, func() error {
		return ExecuteExec(context.Background(), &model.RuntimeConfig{
			WorkspaceRoot:          tmpDir,
			IDE:                    model.IDECodex,
			Mode:                   model.ExecutionModeExec,
			OutputFormat:           model.OutputFormatText,
			PromptText:             "finish the task",
			ReasoningEffort:        "medium",
			RetryBackoffMultiplier: 1.5,
		}, nil)
	})
	if execErr != nil {
		t.Fatalf("execute exec text: %v\nstdout:\n%s\nstderr:\n%s", execErr, stdout, stderr)
	}
	if strings.TrimSpace(stdout) != "final answer" {
		t.Fatalf("expected final assistant response only, got %q", stdout)
	}
	if stderr != "Speed normal: unsupported (capability_absent)\n" {
		t.Fatalf("unexpected human speed status: %q", stderr)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, ".compozy", "runs")); !os.IsNotExist(err) {
		t.Fatalf("expected no persisted run artifacts, got stat err=%v", err)
	}
}

func TestExecuteExecHeadlessDefaultDoesNotEmitOperationalLogs(t *testing.T) {
	tmpDir := t.TempDir()
	installACPHelperOnPath(t, []runACPHelperScenario{{
		ExpectedPromptContains: "finish the task",
		Updates: []acp.SessionUpdate{
			acp.UpdateAgentMessageText("final answer"),
		},
	}})

	stdout, stderr, execErr := captureExecuteStreams(t, func() error {
		previousLogger := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
		defer slog.SetDefault(previousLogger)

		return ExecuteExec(context.Background(), &model.RuntimeConfig{
			WorkspaceRoot:          tmpDir,
			IDE:                    model.IDECodex,
			Mode:                   model.ExecutionModeExec,
			OutputFormat:           model.OutputFormatText,
			PromptText:             "finish the task",
			ReasoningEffort:        "medium",
			RetryBackoffMultiplier: 1.5,
		}, nil)
	})
	if execErr != nil {
		t.Fatalf("execute exec default logging: %v\nstdout:\n%s\nstderr:\n%s", execErr, stdout, stderr)
	}
	if strings.TrimSpace(stdout) != "final answer" {
		t.Fatalf("unexpected exec stdout: %q", stdout)
	}
	if stderr != "Speed normal: unsupported (capability_absent)\n" {
		t.Fatalf("expected only the human speed status on stderr, got %q", stderr)
	}
}

func TestExecuteExecVerboseEmitsOperationalLogsToStderr(t *testing.T) {
	tmpDir := t.TempDir()
	installACPHelperOnPath(t, []runACPHelperScenario{{
		ExpectedPromptContains: "finish the task",
		Updates: []acp.SessionUpdate{
			acp.UpdateAgentMessageText("final answer"),
		},
	}})

	stdout, stderr, execErr := captureExecuteStreams(t, func() error {
		previousLogger := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
		defer slog.SetDefault(previousLogger)

		return ExecuteExec(context.Background(), &model.RuntimeConfig{
			WorkspaceRoot:          tmpDir,
			IDE:                    model.IDECodex,
			Mode:                   model.ExecutionModeExec,
			OutputFormat:           model.OutputFormatText,
			Verbose:                true,
			PromptText:             "finish the task",
			ReasoningEffort:        "medium",
			RetryBackoffMultiplier: 1.5,
		}, nil)
	})
	if execErr != nil {
		t.Fatalf("execute exec verbose logging: %v\nstdout:\n%s\nstderr:\n%s", execErr, stdout, stderr)
	}
	if strings.TrimSpace(stdout) != "final answer" {
		t.Fatalf("unexpected verbose exec stdout: %q", stdout)
	}
	if !strings.Contains(stderr, "acp session created") {
		t.Fatalf("expected verbose stderr to include ACP lifecycle logs, got %q", stderr)
	}
	if !strings.Contains(stderr, "Speed normal: unsupported (capability_absent)") {
		t.Fatalf("expected verbose stderr to include speed status, got %q", stderr)
	}
}

func TestExecuteExecPersistsIndependentSpeedOutcomesAcrossResume(t *testing.T) {
	workspaceRoot := workspaceRootForExecTest(t)
	applied := kinds.SpeedResolution{
		Requested: kinds.SpeedFast,
		Status:    kinds.SpeedResolutionStatusApplied,
	}
	unsupported := kinds.SpeedResolution{
		Requested: kinds.SpeedFast,
		Status:    kinds.SpeedResolutionStatusUnsupported,
		Reason:    kinds.SpeedResolutionReasonCapabilityAbsent,
	}

	var clientCount int
	restore := acpshared.SwapNewAgentClientForTest(
		func(_ context.Context, _ agent.ClientConfig) (agent.Client, error) {
			clientCount++
			if clientCount == 1 {
				return &capturingExecACPClient{
					supportsLoadSession: true,
					createSessionFn: func(
						_ context.Context,
						req agent.SessionRequest,
					) (agent.SessionStart, error) {
						if req.Speed != kinds.SpeedFast {
							t.Fatalf("new-session speed = %q, want fast", req.Speed)
						}
						session := newCapturingExecSession("sess-speed")
						session.updates <- model.SessionUpdate{
							Kind:   model.UpdateKindAgentMessageChunk,
							Status: model.StatusRunning,
							Blocks: []model.ContentBlock{preparedPromptTextContentBlock(t, "first")},
						}
						go session.finish(nil)
						return agent.SessionStart{Session: session, Speed: applied}, nil
					},
				}, nil
			}
			return &capturingExecACPClient{
				supportsLoadSession: true,
				resumeSessionFn: func(
					_ context.Context,
					req agent.ResumeSessionRequest,
				) (agent.SessionStart, error) {
					if req.SessionID != "sess-speed" || req.Speed != kinds.SpeedFast {
						t.Fatalf("unexpected resume request: %#v", req)
					}
					session := newCapturingExecSession("sess-speed")
					session.identity.Resumed = true
					session.updates <- model.SessionUpdate{
						Kind:   model.UpdateKindAgentMessageChunk,
						Status: model.StatusRunning,
						Blocks: []model.ContentBlock{preparedPromptTextContentBlock(t, "second")},
					}
					go session.finish(nil)
					return agent.SessionStart{Session: session, Speed: unsupported}, nil
				},
			}, nil
		},
	)
	t.Cleanup(restore)

	firstCfg := &model.RuntimeConfig{
		WorkspaceRoot:          workspaceRoot,
		IDE:                    model.IDECodex,
		Model:                  "gpt-5.5",
		Speed:                  kinds.SpeedFast,
		AccessMode:             model.AccessModeDefault,
		Mode:                   model.ExecutionModeExec,
		OutputFormat:           model.OutputFormatJSON,
		PromptText:             "first turn",
		RetryBackoffMultiplier: 1.5,
		Persist:                true,
	}
	firstStdout, firstStderr, firstErr := captureExecuteStreams(t, func() error {
		return ExecuteExec(context.Background(), firstCfg, nil)
	})
	if firstErr != nil {
		t.Fatalf("execute first turn: %v", firstErr)
	}
	if firstStderr != "" {
		t.Fatalf("first machine stderr = %q, want empty", firstStderr)
	}
	assertExecSpeedEvents(t, decodeExecJSONLEventsForRunTest(t, firstStdout), applied)

	runRecord, err := LoadPersistedExecRun(workspaceRoot, firstCfg.RunID)
	if err != nil {
		t.Fatalf("load first run record: %v", err)
	}
	firstTurnPath := filepath.Join(runRecord.TurnsDir, "0001", "result.json")
	firstTurnBeforeResume, err := os.ReadFile(firstTurnPath)
	if err != nil {
		t.Fatalf("read first turn before resume: %v", err)
	}

	secondCfg := &model.RuntimeConfig{
		WorkspaceRoot:          workspaceRoot,
		RunID:                  firstCfg.RunID,
		IDE:                    model.IDECodex,
		Model:                  "gpt-5.5",
		Speed:                  kinds.SpeedFast,
		AccessMode:             model.AccessModeDefault,
		Mode:                   model.ExecutionModeExec,
		OutputFormat:           model.OutputFormatJSON,
		PromptText:             "second turn",
		RetryBackoffMultiplier: 1.5,
		Persist:                true,
	}
	secondStdout, secondStderr, secondErr := captureExecuteStreams(t, func() error {
		return ExecuteExec(context.Background(), secondCfg, nil)
	})
	if secondErr != nil {
		t.Fatalf("execute resumed turn: %v", secondErr)
	}
	if secondStderr != "" {
		t.Fatalf("resumed machine stderr = %q, want empty", secondStderr)
	}
	assertExecSpeedEvents(t, decodeExecJSONLEventsForRunTest(t, secondStdout), unsupported)

	runRecord, err = LoadPersistedExecRun(workspaceRoot, firstCfg.RunID)
	if err != nil {
		t.Fatalf("load resumed run record: %v", err)
	}
	if runRecord.SpeedResolution == nil || *runRecord.SpeedResolution != unsupported {
		t.Fatalf("latest run resolution = %#v, want %#v", runRecord.SpeedResolution, unsupported)
	}
	runtimeEvents := readRuntimeEventFile(t, runRecord.EventsPath)
	gotRuntimeResolutions := make([]kinds.SpeedResolution, 0, 2)
	for _, event := range runtimeEvents {
		if event.Kind != eventspkg.EventKindSessionStarted {
			continue
		}
		var payload kinds.SessionStartedPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("decode session.started payload: %v", err)
		}
		if payload.SpeedResolution != nil {
			gotRuntimeResolutions = append(gotRuntimeResolutions, *payload.SpeedResolution)
		}
	}
	if !slices.Equal(gotRuntimeResolutions, []kinds.SpeedResolution{applied, unsupported}) {
		t.Fatalf(
			"runtime session resolutions = %#v, want %#v",
			gotRuntimeResolutions,
			[]kinds.SpeedResolution{applied, unsupported},
		)
	}

	var firstTurn persistedExecTurn
	if err := json.Unmarshal(firstTurnBeforeResume, &firstTurn); err != nil {
		t.Fatalf("decode first turn: %v", err)
	}
	if firstTurn.SpeedResolution == nil || *firstTurn.SpeedResolution != applied {
		t.Fatalf("first turn resolution = %#v, want %#v", firstTurn.SpeedResolution, applied)
	}
	secondTurnPayload, err := os.ReadFile(filepath.Join(runRecord.TurnsDir, "0002", "result.json"))
	if err != nil {
		t.Fatalf("read second turn: %v", err)
	}
	var secondTurn persistedExecTurn
	if err := json.Unmarshal(secondTurnPayload, &secondTurn); err != nil {
		t.Fatalf("decode second turn: %v", err)
	}
	if secondTurn.SpeedResolution == nil || *secondTurn.SpeedResolution != unsupported {
		t.Fatalf("second turn resolution = %#v, want %#v", secondTurn.SpeedResolution, unsupported)
	}
	firstTurnAfterResume, err := os.ReadFile(firstTurnPath)
	if err != nil {
		t.Fatalf("read first turn after resume: %v", err)
	}
	if !bytes.Equal(firstTurnAfterResume, firstTurnBeforeResume) {
		t.Fatal("resumed execution rewrote the prior turn artifact")
	}
}

func TestExecuteExecRejectedSetupPersistsOutcomeWithoutPromptOutput(t *testing.T) {
	workspaceRoot := workspaceRootForExecTest(t)
	rejected := kinds.SpeedResolution{
		Requested: kinds.SpeedFast,
		Status:    kinds.SpeedResolutionStatusRejected,
		Reason:    kinds.SpeedResolutionReasonProviderRejected,
	}
	restore := acpshared.SwapNewAgentClientForTest(
		func(_ context.Context, _ agent.ClientConfig) (agent.Client, error) {
			return &capturingExecACPClient{
				createSessionFn: func(
					context.Context,
					agent.SessionRequest,
				) (agent.SessionStart, error) {
					return agent.SessionStart{Speed: rejected}, &agent.SessionSetupError{
						Stage: agent.SessionSetupStageSetSpeed,
						Err:   errors.New("provider rejected fast"),
					}
				},
			}, nil
		},
	)
	t.Cleanup(restore)

	cfg := &model.RuntimeConfig{
		WorkspaceRoot:          workspaceRoot,
		IDE:                    model.IDECodex,
		Model:                  "gpt-5.5",
		Speed:                  kinds.SpeedFast,
		AccessMode:             model.AccessModeDefault,
		Mode:                   model.ExecutionModeExec,
		OutputFormat:           model.OutputFormatText,
		PromptText:             "must not execute",
		RetryBackoffMultiplier: 1.5,
		Persist:                true,
	}
	stdout, stderr, execErr := captureExecuteStreams(t, func() error {
		return ExecuteExec(context.Background(), cfg, nil)
	})
	if execErr == nil || !strings.Contains(execErr.Error(), "provider rejected fast") {
		t.Fatalf("rejected setup error = %v", execErr)
	}
	if stdout != "" {
		t.Fatalf("rejected setup stdout = %q, want no prompt output", stdout)
	}
	if stderr != "Speed fast: rejected (provider_rejected)\n" {
		t.Fatalf("rejected setup status = %q", stderr)
	}

	runRecord, err := LoadPersistedExecRun(workspaceRoot, cfg.RunID)
	if err != nil {
		t.Fatalf("load rejected run record: %v", err)
	}
	if runRecord.SpeedResolution == nil || *runRecord.SpeedResolution != rejected {
		t.Fatalf("rejected run resolution = %#v, want %#v", runRecord.SpeedResolution, rejected)
	}
	resultPayload, err := os.ReadFile(filepath.Join(runRecord.TurnsDir, "0001", "result.json"))
	if err != nil {
		t.Fatalf("read rejected turn result: %v", err)
	}
	var turn persistedExecTurn
	if err := json.Unmarshal(resultPayload, &turn); err != nil {
		t.Fatalf("decode rejected turn result: %v", err)
	}
	if turn.SpeedResolution == nil || *turn.SpeedResolution != rejected {
		t.Fatalf("rejected turn resolution = %#v, want %#v", turn.SpeedResolution, rejected)
	}
}

func TestExecuteExecMachineAndDaemonModesSuppressRejectedHumanStatus(t *testing.T) {
	testCases := []struct {
		name         string
		outputFormat model.OutputFormat
		daemonOwned  bool
		wantEvents   bool
	}{
		{name: "json", outputFormat: model.OutputFormatJSON, wantEvents: true},
		{name: "raw json", outputFormat: model.OutputFormatRawJSON, wantEvents: true},
		{name: "daemon", outputFormat: model.OutputFormatText, daemonOwned: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			workspaceRoot := workspaceRootForExecTest(t)
			rejected := kinds.SpeedResolution{
				Requested: kinds.SpeedFast,
				Status:    kinds.SpeedResolutionStatusRejected,
				Reason:    kinds.SpeedResolutionReasonProviderRejected,
			}
			restore := acpshared.SwapNewAgentClientForTest(
				func(_ context.Context, _ agent.ClientConfig) (agent.Client, error) {
					return &capturingExecACPClient{
						createSessionFn: func(
							context.Context,
							agent.SessionRequest,
						) (agent.SessionStart, error) {
							return agent.SessionStart{Speed: rejected}, &agent.SessionSetupError{
								Stage: agent.SessionSetupStageSetSpeed,
								Err:   errors.New("provider rejected fast"),
							}
						},
					}, nil
				},
			)
			defer restore()

			stdout, stderr, execErr := captureExecuteStreams(t, func() error {
				return ExecuteExec(context.Background(), &model.RuntimeConfig{
					WorkspaceRoot:          workspaceRoot,
					IDE:                    model.IDECodex,
					Model:                  "gpt-5.5",
					Speed:                  kinds.SpeedFast,
					AccessMode:             model.AccessModeDefault,
					Mode:                   model.ExecutionModeExec,
					OutputFormat:           tc.outputFormat,
					DaemonOwned:            tc.daemonOwned,
					PromptText:             "must not execute",
					RetryBackoffMultiplier: 1.5,
				}, nil)
			})
			if execErr == nil || !strings.Contains(execErr.Error(), "provider rejected fast") {
				t.Fatalf("rejected setup error = %v", execErr)
			}
			if stderr != "" {
				t.Fatalf("machine/daemon stderr = %q, want empty", stderr)
			}
			if !tc.wantEvents {
				if stdout != "" {
					t.Fatalf("daemon stdout = %q, want empty", stdout)
				}
				return
			}

			events := decodeExecJSONLEventsForRunTest(t, stdout)
			foundFailed := false
			for _, event := range events {
				if event["type"] != "run.failed" {
					continue
				}
				foundFailed = true
				if event["speed"] != string(kinds.SpeedFast) {
					t.Fatalf("run.failed speed = %#v, want fast", event["speed"])
				}
				assertExecJSONResolution(t, event["speed_resolution"], rejected)
				if _, exists := event["output"]; exists {
					t.Fatalf("rejected run.failed event contains prompt output: %#v", event["output"])
				}
			}
			if !foundFailed {
				t.Fatal("missing run.failed machine event")
			}
		})
	}
}

func TestExecuteExecPersistedRunCanResumeSameSession(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	plannerDir := filepath.Join(tmpDir, model.WorkflowRootDirName, "agents", "planner")
	if err := os.MkdirAll(plannerDir, 0o755); err != nil {
		t.Fatalf("mkdir planner agent dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(plannerDir, "AGENT.md"),
		[]byte(strings.Join([]string{
			"---",
			"title: Planner",
			"description: Plans the work",
			"ide: codex",
			"---",
			"",
			"Plan the work carefully.",
			"",
		}, "\n")),
		0o600,
	); err != nil {
		t.Fatalf("write planner agent: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(plannerDir, "mcp.json"),
		[]byte(`{"mcpServers":{"filesystem":{"command":"/tmp/fs-mcp","args":["--serve"]}}}`),
		0o600,
	); err != nil {
		t.Fatalf("write planner mcp.json: %v", err)
	}
	installACPHelperOnPath(
		t,
		[]runACPHelperScenario{{
			SupportsLoadSession:              true,
			ExpectedNewSessionMCPServerNames: []string{"compozy", "filesystem"},
		}},
		[]runACPHelperScenario{{
			SessionID:                        "sess-1",
			ExpectedPromptContains:           "first turn",
			ExpectedNewSessionMCPServerNames: []string{"compozy", "filesystem"},
			SupportsLoadSession:              true,
			SessionMeta:                      map[string]any{"agentSessionId": "agent-1"},
			Updates: []acp.SessionUpdate{
				acp.UpdateAgentMessageText("first response"),
			},
		}},
		[]runACPHelperScenario{{SupportsLoadSession: true}},
		[]runACPHelperScenario{{
			SessionID:                         "sess-1",
			ExpectedLoadSessionID:             "sess-1",
			ExpectedPromptContains:            "second turn",
			ExpectedLoadSessionMCPServerNames: []string{"compozy", "filesystem"},
			SupportsLoadSession:               true,
			SessionMeta:                       map[string]any{"agentSessionId": "agent-1"},
			ReplayUpdatesOnLoad: []acp.SessionUpdate{
				acp.UpdateAgentMessageText("replayed response"),
			},
			Updates: []acp.SessionUpdate{
				acp.UpdateAgentMessageText("second response"),
			},
		}},
	)

	if err := ExecuteExec(context.Background(), &model.RuntimeConfig{
		WorkspaceRoot:          tmpDir,
		IDE:                    model.IDECodex,
		Mode:                   model.ExecutionModeExec,
		OutputFormat:           model.OutputFormatText,
		PromptText:             "first turn",
		ReasoningEffort:        "medium",
		RetryBackoffMultiplier: 1.5,
		Persist:                true,
		AgentName:              "planner",
	}, nil); err != nil {
		t.Fatalf("execute first persisted exec: %v", err)
	}

	runID := latestPersistedExecRunID(t, tmpDir)
	if err := ExecuteExec(context.Background(), &model.RuntimeConfig{
		WorkspaceRoot:          tmpDir,
		IDE:                    model.IDECodex,
		Mode:                   model.ExecutionModeExec,
		OutputFormat:           model.OutputFormatText,
		PromptText:             "second turn",
		ReasoningEffort:        "medium",
		RetryBackoffMultiplier: 1.5,
		Persist:                true,
		RunID:                  runID,
		AgentName:              "planner",
	}, nil); err != nil {
		t.Fatalf("execute resumed exec: %v", err)
	}

	runRecord, err := LoadPersistedExecRun(tmpDir, runID)
	if err != nil {
		t.Fatalf("load persisted exec run: %v", err)
	}
	if runRecord.TurnCount != 2 {
		t.Fatalf("expected two turns after resume, got %d", runRecord.TurnCount)
	}
	if runRecord.ACPSessionID != "sess-1" {
		t.Fatalf("unexpected persisted acp session id: %q", runRecord.ACPSessionID)
	}
	if runRecord.AgentSessionID != "agent-1" {
		t.Fatalf("unexpected persisted agent session id: %q", runRecord.AgentSessionID)
	}
	responseBytes, err := os.ReadFile(filepath.Join(runRecord.TurnsDir, "0002", "response.txt"))
	if err != nil {
		t.Fatalf("read resumed response: %v", err)
	}
	if strings.TrimSpace(string(responseBytes)) != "second response" {
		t.Fatalf("unexpected resumed response: %q", string(responseBytes))
	}
}

func TestExecuteExecJSONModeEmitsLeanJSONLAndPersistsRawEvents(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	installACPHelperOnPath(t, []runACPHelperScenario{{
		ExpectedPromptContains: "stream the session",
		Updates:                execJSONProjectionScenarioUpdates(),
	}})

	stdout, stderr, execErr := captureExecuteStreams(t, func() error {
		return ExecuteExec(context.Background(), &model.RuntimeConfig{
			WorkspaceRoot:          tmpDir,
			IDE:                    model.IDECodex,
			Mode:                   model.ExecutionModeExec,
			OutputFormat:           model.OutputFormatJSON,
			PromptText:             "stream the session",
			ReasoningEffort:        "medium",
			RetryBackoffMultiplier: 1.5,
			Persist:                true,
		}, nil)
	})
	if execErr != nil {
		t.Fatalf("execute exec json projection: %v\nstdout:\n%s\nstderr:\n%s", execErr, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr, got %q", stderr)
	}

	stdoutEvents := decodeExecJSONLEventsForRunTest(t, stdout)
	assertSessionUpdateKindsPresent(t, stdoutEvents,
		string(model.UpdateKindUserMessageChunk),
		string(model.UpdateKindAgentMessageChunk),
		string(model.UpdateKindToolCallStarted),
		string(model.UpdateKindToolCallUpdated),
	)
	assertSessionUpdateKindsAbsent(t, stdoutEvents,
		string(model.UpdateKindAgentThoughtChunk),
		string(model.UpdateKindPlanUpdated),
		string(model.UpdateKindAvailableCommandsUpdated),
		string(model.UpdateKindCurrentModeUpdated),
	)

	runID := latestPersistedExecRunID(t, tmpDir)
	runRecord, err := LoadPersistedExecRun(tmpDir, runID)
	if err != nil {
		t.Fatalf("load persisted exec run: %v", err)
	}
	rawEvents := readRuntimeEventFile(t, runRecord.EventsPath)
	assertRuntimeSessionUpdateKindsPresent(t, rawEvents,
		string(model.UpdateKindUserMessageChunk),
		string(model.UpdateKindAgentThoughtChunk),
		string(model.UpdateKindAgentMessageChunk),
		string(model.UpdateKindPlanUpdated),
		string(model.UpdateKindAvailableCommandsUpdated),
		string(model.UpdateKindCurrentModeUpdated),
		string(model.UpdateKindToolCallStarted),
		string(model.UpdateKindToolCallUpdated),
	)
}

func TestExecuteExecRawJSONModeEmitsFullJSONL(t *testing.T) {
	tmpDir := t.TempDir()
	installACPHelperOnPath(t, []runACPHelperScenario{{
		ExpectedPromptContains: "stream everything",
		Updates:                execJSONProjectionScenarioUpdates(),
	}})

	stdout, stderr, execErr := captureExecuteStreams(t, func() error {
		return ExecuteExec(context.Background(), &model.RuntimeConfig{
			WorkspaceRoot:          tmpDir,
			IDE:                    model.IDECodex,
			Mode:                   model.ExecutionModeExec,
			OutputFormat:           model.OutputFormatRawJSON,
			PromptText:             "stream everything",
			ReasoningEffort:        "medium",
			RetryBackoffMultiplier: 1.5,
		}, nil)
	})
	if execErr != nil {
		t.Fatalf("execute exec raw-json: %v\nstdout:\n%s\nstderr:\n%s", execErr, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr, got %q", stderr)
	}

	events := decodeExecJSONLEventsForRunTest(t, stdout)
	assertSessionUpdateKindsPresent(t, events,
		string(model.UpdateKindUserMessageChunk),
		string(model.UpdateKindAgentThoughtChunk),
		string(model.UpdateKindAgentMessageChunk),
		string(model.UpdateKindPlanUpdated),
		string(model.UpdateKindAvailableCommandsUpdated),
		string(model.UpdateKindCurrentModeUpdated),
		string(model.UpdateKindToolCallStarted),
		string(model.UpdateKindToolCallUpdated),
	)
}

func TestShouldEmitLeanSessionUpdateKeepsOnlyUserFacingAndTerminalUpdates(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		update model.SessionUpdate
		want   bool
	}{
		{
			name:   "Should keep agent message chunks",
			update: model.SessionUpdate{Kind: model.UpdateKindAgentMessageChunk, Status: model.StatusRunning},
			want:   true,
		},
		{
			name:   "Should drop plan updates",
			update: model.SessionUpdate{Kind: model.UpdateKindPlanUpdated, Status: model.StatusRunning},
			want:   false,
		},
		{
			name:   "Should keep unknown completed updates",
			update: model.SessionUpdate{Status: model.StatusCompleted},
			want:   true,
		},
		{
			name:   "Should drop unknown running updates",
			update: model.SessionUpdate{Status: model.StatusRunning},
			want:   false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := shouldEmitLeanSessionUpdate(&tc.update); got != tc.want {
				t.Fatalf("shouldEmitLeanSessionUpdate() = %v, want %v for %#v", got, tc.want, tc.update)
			}
		})
	}
}

func TestExecuteExecWithSelectedAgentResolvesRuntimeAndCanonicalSystemPrompt(t *testing.T) {
	workspaceRoot := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	plannerDir := filepath.Join(workspaceRoot, model.WorkflowRootDirName, "agents", "planner")
	if err := os.MkdirAll(plannerDir, 0o755); err != nil {
		t.Fatalf("mkdir planner agent dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(plannerDir, "AGENT.md"),
		[]byte(strings.Join([]string{
			"---",
			"title: Planner",
			"description: Plans the work",
			"ide: claude",
			"model: agent-model",
			"access_mode: default",
			"---",
			"",
			"Plan the work carefully.",
			"",
		}, "\n")),
		0o600,
	); err != nil {
		t.Fatalf("write planner agent: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(plannerDir, "mcp.json"),
		[]byte(`{"mcpServers":{"filesystem":{"command":"/tmp/fs-mcp","args":["--serve"]}}}`),
		0o600,
	); err != nil {
		t.Fatalf("write planner mcp.json: %v", err)
	}

	reviewerDir := filepath.Join(workspaceRoot, model.WorkflowRootDirName, "agents", "reviewer")
	if err := os.MkdirAll(reviewerDir, 0o755); err != nil {
		t.Fatalf("mkdir reviewer agent dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(reviewerDir, "AGENT.md"),
		[]byte(strings.Join([]string{
			"---",
			"title: Reviewer",
			"description: Reviews code",
			"ide: codex",
			"---",
			"",
			"Review the code.",
			"",
		}, "\n")),
		0o600,
	); err != nil {
		t.Fatalf("write reviewer agent: %v", err)
	}

	var (
		gotClientCfg agent.ClientConfig
		gotReq       agent.SessionRequest
	)
	restore := acpshared.SwapNewAgentClientForTest(
		func(_ context.Context, cfg agent.ClientConfig) (agent.Client, error) {
			gotClientCfg = cfg
			return &capturingExecACPClient{
				createSessionFn: func(_ context.Context, req agent.SessionRequest) (agent.SessionStart, error) {
					gotReq = req
					session := newCapturingExecSession("sess-agent")
					go session.finish(nil)
					return unsupportedExecSessionStart(req, session), nil
				},
			}, nil
		},
	)
	t.Cleanup(restore)

	err := ExecuteExec(context.Background(), &model.RuntimeConfig{
		WorkspaceRoot:          workspaceRoot,
		IDE:                    model.IDECodex,
		Model:                  "cli-model",
		ReasoningEffort:        "workspace-reasoning",
		AccessMode:             model.AccessModeFull,
		Mode:                   model.ExecutionModeExec,
		OutputFormat:           model.OutputFormatText,
		PromptText:             "finish the task",
		RetryBackoffMultiplier: 1.5,
		AgentName:              "planner",
		ExplicitRuntime: model.ExplicitRuntimeFlags{
			Model: true,
		},
	}, nil)
	if err != nil {
		t.Fatalf("execute exec with selected agent: %v", err)
	}

	if gotClientCfg.IDE != model.IDEClaude {
		t.Fatalf("expected agent ide to reach ACP client config, got %q", gotClientCfg.IDE)
	}
	if gotClientCfg.Model != "cli-model" {
		t.Fatalf("expected explicit model to reach ACP client config, got %q", gotClientCfg.Model)
	}
	if gotClientCfg.ReasoningEffort != "workspace-reasoning" {
		t.Fatalf("expected existing reasoning effort to be preserved, got %q", gotClientCfg.ReasoningEffort)
	}
	if gotClientCfg.AccessMode != model.AccessModeDefault {
		t.Fatalf("expected agent access mode to reach ACP client config, got %q", gotClientCfg.AccessMode)
	}
	if gotReq.Model != "cli-model" {
		t.Fatalf("expected session request model to use explicit override, got %q", gotReq.Model)
	}
	if len(gotReq.MCPServers) != 2 {
		t.Fatalf("expected reserved plus agent-local MCP servers, got %#v", gotReq.MCPServers)
	}
	if gotReq.MCPServers[0].Stdio == nil || gotReq.MCPServers[0].Stdio.Name != reusableagents.ReservedMCPServerName {
		t.Fatalf("unexpected reserved MCP server wiring: %#v", gotReq.MCPServers)
	}
	if gotReq.MCPServers[1].Stdio == nil || gotReq.MCPServers[1].Stdio.Name != "filesystem" {
		t.Fatalf("unexpected agent-local MCP server wiring: %#v", gotReq.MCPServers)
	}

	promptText := string(gotReq.Prompt)
	requiredSnippets := []string{
		"<agent_metadata>",
		"name: planner",
		"title: Planner",
		"description: Plans the work",
		"source: workspace",
		"<available_agents>",
		"- reviewer: Reviews code (workspace)",
		"Plan the work carefully.",
		"finish the task",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(promptText, snippet) {
			t.Fatalf("expected composed ACP prompt to include %q, got:\n%s", snippet, promptText)
		}
	}
	if strings.Count(promptText, "<agent_metadata>") != 1 ||
		strings.Count(promptText, "Plan the work carefully.") != 1 {
		t.Fatalf("expected selected agent metadata and prompt body exactly once, got:\n%s", promptText)
	}
	if strings.Contains(promptText, "- planner:") {
		t.Fatalf("expected discovery catalog to exclude selected agent, got:\n%s", promptText)
	}
}

func latestPersistedExecRunID(t *testing.T, _ string) string {
	t.Helper()

	homePaths, err := compozyconfig.ResolveHomePaths()
	if err != nil {
		t.Fatalf("resolve persisted exec runs dir: %v", err)
	}
	entries, err := os.ReadDir(homePaths.RunsDir)
	if err != nil {
		t.Fatalf("read persisted exec runs: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one persisted exec run, got %d", len(entries))
	}
	return entries[0].Name()
}

func execJSONProjectionScenarioUpdates() []acp.SessionUpdate {
	return []acp.SessionUpdate{
		acp.UpdateUserMessageText("user says hello"),
		acp.UpdateAgentThoughtText("thinking"),
		acp.UpdateAgentMessageText("visible answer"),
		acp.UpdatePlan(acp.PlanEntry{
			Content:  "Inspect repo",
			Status:   acp.PlanEntryStatusInProgress,
			Priority: acp.PlanEntryPriorityHigh,
		}),
		{
			AvailableCommandsUpdate: &acp.SessionAvailableCommandsUpdate{
				AvailableCommands: []acp.AvailableCommand{{
					Name:        "run",
					Description: "Run the task",
					Input: &acp.AvailableCommandInput{
						Unstructured: &acp.UnstructuredCommandInput{Hint: "--fast"},
					},
				}},
			},
		},
		{
			CurrentModeUpdate: &acp.SessionCurrentModeUpdate{CurrentModeId: "review"},
		},
		acp.StartReadToolCall(acp.ToolCallId("tool-1"), "Read README.md", "README.md"),
		acp.UpdateToolCall(
			acp.ToolCallId("tool-1"),
			acp.WithUpdateStatus(acp.ToolCallStatusCompleted),
			acp.WithUpdateContent([]acp.ToolCallContent{
				acp.ToolContent(acp.TextBlock("README contents")),
			}),
		),
	}
}

func readRuntimeEventFile(t *testing.T, path string) []eventspkg.Event {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read runtime event file %s: %v", path, err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	events := make([]eventspkg.Event, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event eventspkg.Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("decode runtime event: %v\nline:\n%s", err, line)
		}
		events = append(events, event)
	}
	return events
}

func decodeExecJSONLEventsForRunTest(t *testing.T, data string) []map[string]any {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(data), "\n")
	events := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			t.Fatalf("decode exec event: %v\nline:\n%s", err, line)
		}
		events = append(events, payload)
	}
	return events
}

func assertExecSpeedEvents(
	t *testing.T,
	events []map[string]any,
	want kinds.SpeedResolution,
) {
	t.Helper()

	var foundStarted, foundAttached, foundTerminal bool
	for _, event := range events {
		eventType, _ := event["type"].(string)
		switch eventType {
		case "run.started":
			foundStarted = true
			if event["speed"] != string(want.Requested) {
				t.Fatalf("run.started speed = %#v, want %q", event["speed"], want.Requested)
			}
		case "session.attached":
			foundAttached = true
			session, _ := event["session"].(map[string]any)
			assertExecJSONResolution(t, session["speed_resolution"], want)
		case "run.succeeded":
			foundTerminal = true
			if event["speed"] != string(want.Requested) {
				t.Fatalf("run.succeeded speed = %#v, want %q", event["speed"], want.Requested)
			}
			assertExecJSONResolution(t, event["speed_resolution"], want)
		}
	}
	if !foundStarted || !foundAttached || !foundTerminal {
		t.Fatalf(
			"missing speed lifecycle events: started=%v attached=%v terminal=%v",
			foundStarted,
			foundAttached,
			foundTerminal,
		)
	}
}

func assertExecJSONResolution(t *testing.T, raw any, want kinds.SpeedResolution) {
	t.Helper()

	payload, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal JSON resolution: %v", err)
	}
	var got kinds.SpeedResolution
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("decode JSON resolution: %v", err)
	}
	if got != want {
		t.Fatalf("JSON resolution = %#v, want %#v", got, want)
	}
}

func assertSessionUpdateKindsPresent(t *testing.T, events []map[string]any, want ...string) {
	t.Helper()

	kinds := collectedSessionUpdateKinds(events)
	for _, kind := range want {
		if !slices.Contains(kinds, kind) {
			t.Fatalf("expected session.update kind %q in %v", kind, kinds)
		}
	}
}

func assertSessionUpdateKindsAbsent(t *testing.T, events []map[string]any, want ...string) {
	t.Helper()

	kinds := collectedSessionUpdateKinds(events)
	for _, kind := range want {
		if slices.Contains(kinds, kind) {
			t.Fatalf("expected session.update kind %q to be absent from %v", kind, kinds)
		}
	}
}

func assertRuntimeSessionUpdateKindsPresent(t *testing.T, events []eventspkg.Event, want ...string) {
	t.Helper()

	kinds := collectedRuntimeSessionUpdateKinds(t, events)
	for _, kind := range want {
		if !slices.Contains(kinds, kind) {
			t.Fatalf("expected runtime session.update kind %q in %v", kind, kinds)
		}
	}
}

func collectedSessionUpdateKinds(events []map[string]any) []string {
	kinds := make([]string, 0, len(events))
	for _, event := range events {
		eventType, _ := event["type"].(string)
		if eventType != "session.update" {
			continue
		}
		update, _ := event["update"].(map[string]any)
		kind, _ := update["kind"].(string)
		kinds = append(kinds, kind)
	}
	return kinds
}

func collectedRuntimeSessionUpdateKinds(t *testing.T, events []eventspkg.Event) []string {
	t.Helper()

	updateKinds := make([]string, 0, len(events))
	for _, event := range events {
		if event.Kind != eventspkg.EventKindSessionUpdate {
			continue
		}
		var payload kinds.SessionUpdatePayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("decode runtime session.update payload: %v", err)
		}
		updateKinds = append(updateKinds, string(payload.Update.Kind))
	}
	return updateKinds
}

type capturingExecACPClient struct {
	createSessionFn     func(context.Context, agent.SessionRequest) (agent.SessionStart, error)
	resumeSessionFn     func(context.Context, agent.ResumeSessionRequest) (agent.SessionStart, error)
	supportsLoadSession bool
}

func (c *capturingExecACPClient) CreateSession(
	ctx context.Context,
	req agent.SessionRequest,
) (agent.SessionStart, error) {
	return c.createSessionFn(ctx, req)
}

func (c *capturingExecACPClient) ResumeSession(
	ctx context.Context,
	req agent.ResumeSessionRequest,
) (agent.SessionStart, error) {
	if c.resumeSessionFn != nil {
		return c.resumeSessionFn(ctx, req)
	}
	return agent.SessionStart{}, errors.New("resume not supported in capturing exec fake")
}

func (*capturingExecACPClient) CancelSession(context.Context, string) error {
	return nil
}

func (*capturingExecACPClient) PromptSession(context.Context, agent.PromptSessionRequest) (agent.Session, error) {
	return nil, nil
}

func (c *capturingExecACPClient) SupportsLoadSession() bool {
	return c.supportsLoadSession
}

func (*capturingExecACPClient) Close() error {
	return nil
}

func (*capturingExecACPClient) Kill() error {
	return nil
}

func unsupportedExecSessionStart(
	req agent.SessionRequest,
	session agent.Session,
) agent.SessionStart {
	return agent.SessionStart{
		Session: session,
		Speed: kinds.SpeedResolution{
			Requested: req.Speed,
			Status:    kinds.SpeedResolutionStatusUnsupported,
			Reason:    kinds.SpeedResolutionReasonCapabilityAbsent,
		},
	}
}

var _ agent.Client = (*capturingExecACPClient)(nil)

type capturingExecSession struct {
	id       string
	identity agent.SessionIdentity
	updates  chan model.SessionUpdate
	done     chan struct{}

	mu       sync.RWMutex
	err      error
	finished bool
}

func newCapturingExecSession(id string) *capturingExecSession {
	return &capturingExecSession{
		id:       id,
		identity: agent.SessionIdentity{ACPSessionID: id},
		updates:  make(chan model.SessionUpdate, 1),
		done:     make(chan struct{}),
	}
}

func (s *capturingExecSession) ID() string {
	return s.id
}

func (s *capturingExecSession) Identity() agent.SessionIdentity {
	return s.identity
}

func (s *capturingExecSession) Updates() <-chan model.SessionUpdate {
	return s.updates
}

func (s *capturingExecSession) Done() <-chan struct{} {
	return s.done
}

func (s *capturingExecSession) Err() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.err
}

func (*capturingExecSession) SlowPublishes() uint64 {
	return 0
}

func (*capturingExecSession) DroppedUpdates() uint64 {
	return 0
}

func (s *capturingExecSession) finish(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return
	}
	s.finished = true
	s.err = err
	close(s.updates)
	close(s.done)
}
