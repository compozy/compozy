package exec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/agent"
	reusableagents "github.com/compozy/compozy/internal/core/agents"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/internal/acpshared"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestExecutePreparedPromptValidatesInputs(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		cfg         *model.RuntimeConfig
		promptText  string
		expectedErr string
	}{
		{
			name:        "Should error when runtime config is nil",
			cfg:         nil,
			promptText:  "delegate",
			expectedErr: "missing runtime config",
		},
		{
			name:        "Should error when prompt is empty",
			cfg:         &model.RuntimeConfig{},
			promptText:  "   ",
			expectedErr: "prompt is empty",
		},
	}

	for _, tt := range testCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ExecutePreparedPrompt(context.Background(), tt.cfg, tt.promptText, nil, nil)
			if err == nil {
				t.Fatalf("expected error containing %q", tt.expectedErr)
			}
			if !strings.Contains(err.Error(), tt.expectedErr) {
				t.Fatalf("expected error containing %q, got %v", tt.expectedErr, err)
			}
		})
	}
}

func TestPersistedExecRunOmitsRuntimeOnlyRecoveryAttempt(t *testing.T) {
	t.Parallel()

	t.Run("Should omit runtime-only recovery attempt metadata from persisted exec runs", func(t *testing.T) {
		t.Parallel()

		record := newPersistedExecRunRecord(
			&model.RuntimeConfig{
				WorkspaceRoot:   t.TempDir(),
				IDE:             model.IDECodex,
				Model:           "gpt-5.5",
				ReasoningEffort: "medium",
				AccessMode:      model.AccessModeFull,
				RecoveryAttempt: 1,
			},
			model.NewRunArtifacts(t.TempDir(), "exec-run"),
			"exec-run",
			"gpt-5.5",
		)
		payload, err := json.Marshal(record)
		if err != nil {
			t.Fatalf("marshal persisted exec run: %v", err)
		}
		if strings.Contains(string(payload), "recovery") {
			t.Fatalf("persisted exec run should omit runtime-only recovery marker: %s", string(payload))
		}
		if !strings.Contains(string(payload), `"speed":"normal"`) {
			t.Fatalf("persisted exec run should retain default requested speed: %s", string(payload))
		}
	})
}

func TestExecRunStatePersistsCanonicalSpeedOutcomes(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		resolution kinds.SpeedResolution
		status     string
		resultErr  error
	}{
		{
			name: "applied",
			resolution: kinds.SpeedResolution{
				Requested: kinds.SpeedFast,
				Status:    kinds.SpeedResolutionStatusApplied,
			},
			status: runStatusSucceeded,
		},
		{
			name: "unsupported",
			resolution: kinds.SpeedResolution{
				Requested: kinds.SpeedFast,
				Status:    kinds.SpeedResolutionStatusUnsupported,
				Reason:    kinds.SpeedResolutionReasonCapabilityAbsent,
			},
			status: runStatusSucceeded,
		},
		{
			name: "rejected",
			resolution: kinds.SpeedResolution{
				Requested: kinds.SpeedFast,
				Status:    kinds.SpeedResolutionStatusRejected,
				Reason:    kinds.SpeedResolutionReasonProviderRejected,
			},
			status:    runStatusFailed,
			resultErr: errors.New("speed rejected"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			runArtifacts := model.NewRunArtifacts(root, "exec-speed-"+tc.name)
			if err := os.MkdirAll(runArtifacts.RunDir, 0o755); err != nil {
				t.Fatalf("mkdir run dir: %v", err)
			}
			turnDir := filepath.Join(runArtifacts.TurnsDir, "0001")
			if err := os.MkdirAll(turnDir, 0o755); err != nil {
				t.Fatalf("mkdir turn dir: %v", err)
			}

			state := &execRunState{
				record: PersistedExecRun{
					Version:   execRunSchemaVersion,
					Mode:      model.ModeExec,
					RunID:     runArtifacts.RunID,
					Speed:     kinds.SpeedFast,
					UpdatedAt: time.Now().UTC(),
				},
				runArtifacts: runArtifacts,
				turn:         1,
				turnPaths: execTurnPaths{
					promptPath: filepath.Join(turnDir, "prompt.md"),
					resultPath: filepath.Join(turnDir, "result.json"),
				},
			}
			result := execExecutionResult{
				status:          tc.status,
				speedResolution: tc.resolution,
				err:             tc.resultErr,
			}
			err := state.completeTurn(result)
			if tc.resultErr == nil && err != nil {
				t.Fatalf("complete turn: %v", err)
			}
			if tc.resultErr != nil && !errors.Is(err, tc.resultErr) {
				t.Fatalf("complete turn error = %v, want %v", err, tc.resultErr)
			}

			var turn persistedExecTurn
			payload, readErr := os.ReadFile(state.turnPaths.resultPath)
			if readErr != nil {
				t.Fatalf("read turn result: %v", readErr)
			}
			if unmarshalErr := json.Unmarshal(payload, &turn); unmarshalErr != nil {
				t.Fatalf("decode turn result: %v", unmarshalErr)
			}
			if turn.Speed != kinds.SpeedFast {
				t.Fatalf("turn speed = %q, want %q", turn.Speed, kinds.SpeedFast)
			}
			if turn.SpeedResolution == nil || *turn.SpeedResolution != tc.resolution {
				t.Fatalf("turn resolution = %#v, want %#v", turn.SpeedResolution, tc.resolution)
			}

			runPayload, readErr := os.ReadFile(runArtifacts.RunMetaPath)
			if readErr != nil {
				t.Fatalf("read run record: %v", readErr)
			}
			var runRecord PersistedExecRun
			if unmarshalErr := json.Unmarshal(runPayload, &runRecord); unmarshalErr != nil {
				t.Fatalf("decode run record: %v", unmarshalErr)
			}
			if runRecord.Speed != kinds.SpeedFast {
				t.Fatalf("run speed = %q, want %q", runRecord.Speed, kinds.SpeedFast)
			}
			if runRecord.SpeedResolution == nil || *runRecord.SpeedResolution != tc.resolution {
				t.Fatalf("run resolution = %#v, want %#v", runRecord.SpeedResolution, tc.resolution)
			}
		})
	}
}

func TestExecRunStateEmitsCanceledSpeedOutcome(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	resolution := kinds.SpeedResolution{
		Requested: kinds.SpeedFast,
		Status:    kinds.SpeedResolutionStatusUnsupported,
		Reason:    kinds.SpeedResolutionReasonCapabilityAbsent,
	}
	state := &execRunState{
		record: PersistedExecRun{Speed: kinds.SpeedFast},
		runArtifacts: model.RunArtifacts{
			RunID: "exec-canceled",
		},
		turn:   1,
		events: newExecEventEmitter(execJSONStreamRaw, &output),
	}
	startedAt := time.Now().UTC().Add(-time.Second)
	completedAt := time.Now().UTC()
	if err := state.emitTurnResult(execExecutionResult{
		status:          runStatusCanceled,
		speedResolution: resolution,
		err:             context.Canceled,
	}, startedAt, completedAt); err != nil {
		t.Fatalf("emit canceled turn: %v", err)
	}
	if err := state.events.Close(); err != nil {
		t.Fatalf("close event emitter: %v", err)
	}

	var event execEvent
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &event); err != nil {
		t.Fatalf("decode canceled event: %v", err)
	}
	if event.Type != "run.canceled" || event.Speed != kinds.SpeedFast {
		t.Fatalf("unexpected canceled event: %#v", event)
	}
	if event.SpeedResolution == nil || *event.SpeedResolution != resolution {
		t.Fatalf("canceled event resolution = %#v, want %#v", event.SpeedResolution, resolution)
	}
}

func TestExecutePreparedPromptReturnsEnsureAvailableError(t *testing.T) {
	t.Parallel()

	_, err := ExecutePreparedPrompt(
		context.Background(),
		&model.RuntimeConfig{IDE: "missing-runtime"},
		"delegate this",
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("expected runtime availability error")
	}
	if !strings.Contains(err.Error(), `unknown agent runtime "missing-runtime"`) {
		t.Fatalf("expected unknown runtime error, got %v", err)
	}
}

func TestExecutePreparedPromptReturnsBuilderError(t *testing.T) {
	workspaceRoot := workspaceRootForExecTest(t)
	cfg := &model.RuntimeConfig{
		WorkspaceRoot: workspaceRoot,
		IDE:           model.IDECodex,
		Model:         "gpt-5.5",
		AccessMode:    model.AccessModeDefault,
		OutputFormat:  model.OutputFormatText,
		Persist:       true,
	}

	result, err := ExecutePreparedPrompt(
		context.Background(),
		cfg,
		"delegate this",
		nil,
		func(runID string) ([]model.MCPServer, error) {
			if strings.TrimSpace(runID) == "" {
				t.Fatal("expected run id before MCP builder executes")
			}
			return nil, errors.New("mcp builder failed")
		},
	)
	if err == nil || !strings.Contains(err.Error(), "mcp builder failed") {
		t.Fatalf("expected MCP builder error, got %v", err)
	}
	if strings.TrimSpace(result.RunID) == "" {
		t.Fatalf("expected failed prepared prompt to retain run id, got %#v", result)
	}
	record, loadErr := LoadPersistedExecRun(workspaceRoot, result.RunID)
	if loadErr != nil {
		t.Fatalf("load persisted exec run: %v", loadErr)
	}
	if record.Status != runStatusFailed {
		t.Fatalf("expected failed exec record after MCP builder error, got %#v", record)
	}
}

func TestPrepareExecRunStateScopedFreshPersistedRunInitializesRecord(t *testing.T) {
	workspaceRoot := workspaceRootForExecTest(t)
	cfg := &model.RuntimeConfig{
		WorkspaceRoot: workspaceRoot,
		RunID:         "exec-daemon-fresh",
		IDE:           model.IDECodex,
		Model:         "gpt-5.5",
		AccessMode:    model.AccessModeDefault,
		Speed:         kinds.SpeedFast,
		OutputFormat:  model.OutputFormatText,
		Persist:       true,
		DaemonOwned:   true,
		Mode:          model.ExecutionModeExec,
	}
	cfg.ApplyDefaults()

	scope, err := model.OpenBaseRunScope(context.Background(), cfg)
	if err != nil {
		t.Fatalf("OpenBaseRunScope() error = %v", err)
	}
	defer func() {
		_ = scope.Close(context.Background())
	}()
	_, eventsCh, unsubscribe := scope.RunEventBus().Subscribe()
	defer unsubscribe()

	state, err := prepareExecRunState(context.Background(), cfg, scope)
	if err != nil {
		t.Fatalf("prepareExecRunState() error = %v", err)
	}
	defer state.close()

	if state.record.RunID != cfg.RunID {
		t.Fatalf("state.record.RunID = %q, want %q", state.record.RunID, cfg.RunID)
	}
	if state.turn != 1 {
		t.Fatalf("state.turn = %d, want 1", state.turn)
	}
	if err := state.writeStarted(cfg); err != nil {
		t.Fatalf("state.writeStarted() error = %v", err)
	}
	select {
	case event := <-eventsCh:
		if event.Kind != eventspkg.EventKindRunStarted {
			t.Fatalf("event kind = %q, want run.started", event.Kind)
		}
		var payload kinds.RunStartedPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("decode run.started payload: %v", err)
		}
		if payload.Speed != kinds.SpeedFast {
			t.Fatalf("run.started speed = %q, want fast", payload.Speed)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for run.started event")
	}
	if _, err := os.Stat(scope.RunArtifacts().RunMetaPath); err != nil {
		t.Fatalf("stat run meta path %q: %v", scope.RunArtifacts().RunMetaPath, err)
	}
}

func TestExecutePreparedPromptReturnsBuilderAndCompletionFailure(t *testing.T) {
	workspaceRoot := workspaceRootForExecTest(t)
	builderErr := errors.New("mcp builder failed")

	result, err := ExecutePreparedPrompt(
		context.Background(),
		&model.RuntimeConfig{
			WorkspaceRoot: workspaceRoot,
			IDE:           model.IDECodex,
			Model:         "gpt-5.5",
			AccessMode:    model.AccessModeDefault,
			OutputFormat:  model.OutputFormatText,
			Persist:       true,
		},
		"delegate this",
		nil,
		func(runID string) ([]model.MCPServer, error) {
			runArtifacts, err := model.ResolveHomeRunArtifacts(runID)
			if err != nil {
				t.Fatalf("resolve home run artifacts: %v", err)
			}
			responsePath := filepath.Join(runArtifacts.TurnsDir, "0001", "response.txt")
			if err := os.Mkdir(responsePath, 0o755); err != nil {
				t.Fatalf("make response path unwritable: %v", err)
			}
			return nil, builderErr
		},
	)
	if err == nil {
		t.Fatal("expected prepared prompt to retain builder and completion failures")
	}
	if strings.TrimSpace(result.RunID) == "" {
		t.Fatalf("expected failed prepared prompt to retain run id, got %#v", result)
	}
	if !errors.Is(err, builderErr) {
		t.Fatalf("expected returned error to retain builder failure, got %v", err)
	}
	if !strings.Contains(err.Error(), "write exec response") {
		t.Fatalf("expected returned error to retain completion failure, got %v", err)
	}
}

func TestExecutePreparedPromptSucceedsWithoutMCPBuilder(t *testing.T) {
	workspaceRoot := workspaceRootForExecTest(t)

	var gotReq agent.SessionRequest
	restore := acpshared.SwapNewAgentClientForTest(
		func(_ context.Context, _ agent.ClientConfig) (agent.Client, error) {
			return &capturingExecACPClient{
				createSessionFn: func(_ context.Context, req agent.SessionRequest) (agent.SessionStart, error) {
					gotReq = req
					session := newCapturingExecSession("sess-prepared")
					session.updates <- model.SessionUpdate{
						Kind:   model.UpdateKindAgentMessageChunk,
						Status: model.StatusRunning,
						Blocks: []model.ContentBlock{preparedPromptTextContentBlock(t, "nested reply")},
					}
					go session.finish(nil)
					return unsupportedExecSessionStart(req, session), nil
				},
			}, nil
		},
	)
	t.Cleanup(restore)

	result, err := ExecutePreparedPrompt(
		context.Background(),
		&model.RuntimeConfig{
			WorkspaceRoot: workspaceRoot,
			IDE:           model.IDECodex,
			Model:         "gpt-5.5",
			AccessMode:    model.AccessModeDefault,
			OutputFormat:  model.OutputFormatText,
		},
		"delegate this",
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("execute prepared prompt: %v", err)
	}
	if result.RunID == "" || result.Output != "nested reply" {
		t.Fatalf("unexpected prepared prompt result: %#v", result)
	}
	if len(gotReq.MCPServers) != 0 {
		t.Fatalf("expected nil MCP builder to skip MCP servers, got %#v", gotReq.MCPServers)
	}
}

func TestExecutePreparedPromptReturnsCompletionFailureWhenExecAlsoFails(t *testing.T) {
	workspaceRoot := workspaceRootForExecTest(t)
	execErr := errors.New("session failed")

	restore := acpshared.SwapNewAgentClientForTest(
		func(_ context.Context, _ agent.ClientConfig) (agent.Client, error) {
			return &capturingExecACPClient{
				createSessionFn: func(_ context.Context, req agent.SessionRequest) (agent.SessionStart, error) {
					session := newCapturingExecSession("sess-prepared-failure")
					go session.finish(execErr)
					return unsupportedExecSessionStart(req, session), nil
				},
			}, nil
		},
	)
	t.Cleanup(restore)

	result, err := ExecutePreparedPrompt(
		context.Background(),
		&model.RuntimeConfig{
			WorkspaceRoot: workspaceRoot,
			IDE:           model.IDECodex,
			Model:         "gpt-5.5",
			AccessMode:    model.AccessModeDefault,
			OutputFormat:  model.OutputFormatText,
			Persist:       true,
		},
		"delegate this",
		nil,
		func(runID string) ([]model.MCPServer, error) {
			runArtifacts, err := model.ResolveHomeRunArtifacts(runID)
			if err != nil {
				t.Fatalf("resolve home run artifacts: %v", err)
			}
			responsePath := filepath.Join(runArtifacts.TurnsDir, "0001", "response.txt")
			if err := os.Mkdir(responsePath, 0o755); err != nil {
				t.Fatalf("make response path unwritable: %v", err)
			}
			return nil, nil
		},
	)
	if err == nil {
		t.Fatal("expected prepared prompt to surface completion failure")
	}
	if strings.TrimSpace(result.RunID) == "" {
		t.Fatalf("expected failed prepared prompt to retain run id, got %#v", result)
	}
	if !errors.Is(err, execErr) {
		t.Fatalf("expected returned error to retain exec failure, got %v", err)
	}
	if !strings.Contains(err.Error(), "write exec response") {
		t.Fatalf("expected returned error to retain completion failure, got %v", err)
	}
}

func TestNewExecRuntimeJobAttachesReservedServerWithoutReusableAgent(t *testing.T) {
	jb, err := newExecRuntimeJob(
		"delegate this",
		nil,
		nil,
		&model.RuntimeConfig{
			WorkspaceRoot: workspaceRootForExecTest(t),
			IDE:           model.IDECodex,
			Model:         "gpt-5.5",
			AccessMode:    model.AccessModeDefault,
		},
	)
	if err != nil {
		t.Fatalf("newExecRuntimeJob: %v", err)
	}
	if len(jb.MCPServers) != 1 {
		t.Fatalf("expected reserved MCP server for plain exec job, got %#v", jb.MCPServers)
	}
	if jb.MCPServers[0].Stdio == nil || jb.MCPServers[0].Stdio.Name != reusableagents.ReservedMCPServerName {
		t.Fatalf("unexpected reserved MCP server wiring: %#v", jb.MCPServers)
	}
}

func TestNewExecRuntimeJobUsesRuntimeSystemPromptOverride(t *testing.T) {
	t.Run("Should prefer the runtime system prompt over the reusable agent prompt", func(t *testing.T) {
		agentExecution := &reusableagents.ExecutionContext{
			Agent: reusableagents.ResolvedAgent{
				Name: "planner",
				Metadata: reusableagents.Metadata{
					Title:       "Planner",
					Description: "Plans work",
				},
				Prompt: "agent prompt must not be used",
				Source: reusableagents.Source{Scope: reusableagents.ScopeWorkspace},
			},
			Catalog: reusableagents.Catalog{
				Agents: []reusableagents.ResolvedAgent{{
					Name: "planner",
					Metadata: reusableagents.Metadata{
						Title:       "Planner",
						Description: "Plans work",
					},
					Prompt: "agent prompt must not be used",
					Source: reusableagents.Source{Scope: reusableagents.ScopeWorkspace},
				}},
			},
		}
		jb, err := newExecRuntimeJob(
			"delegate this",
			nil,
			agentExecution,
			&model.RuntimeConfig{
				WorkspaceRoot: workspaceRootForExecTest(t),
				IDE:           model.IDECodex,
				Model:         "gpt-5.5",
				AccessMode:    model.AccessModeDefault,
				SystemPrompt:  "runtime system prompt",
			},
		)
		if err != nil {
			t.Fatalf("newExecRuntimeJob: %v", err)
		}
		if jb.SystemPrompt != "runtime system prompt" {
			t.Fatalf("SystemPrompt = %q, want runtime override", jb.SystemPrompt)
		}
		if strings.Contains(jb.SystemPrompt, "agent prompt must not be used") {
			t.Fatalf("runtime SystemPrompt should override agent prompt, got:\n%s", jb.SystemPrompt)
		}
	})
}

func TestWriteExecJSONFailureAndReportedErrorHelpers(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := WriteExecJSONFailure(&buf, "exec-123", errors.New("boom")); err != nil {
		t.Fatalf("WriteExecJSONFailure: %v", err)
	}

	var payload execSetupErrorPayload
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &payload); err != nil {
		t.Fatalf("decode exec failure payload: %v", err)
	}
	if payload.Type != "run.failed" || payload.RunID != "exec-123" || payload.Error != "boom" {
		t.Fatalf("unexpected exec failure payload: %#v", payload)
	}

	reported := &execReportedError{err: errors.New("reported")}
	if !IsExecErrorReported(reported) {
		t.Fatal("expected reported exec error to be detected")
	}
	if got := reported.Error(); got != "reported" {
		t.Fatalf("unexpected reported error text: %q", got)
	}
	if reported.Unwrap() == nil {
		t.Fatal("expected unwrap to expose the wrapped error")
	}
	if IsExecErrorReported(errors.New("plain")) {
		t.Fatal("expected plain error not to be reported")
	}
}

func TestExecRunStateCompleteDryRunWritesArtifacts(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	runArtifacts := model.NewRunArtifacts(tmpDir, "exec-dry-run")
	if err := os.MkdirAll(runArtifacts.RunDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	state := &execRunState{
		record: PersistedExecRun{
			Speed: kinds.SpeedFast,
			SpeedResolution: &kinds.SpeedResolution{
				Requested: kinds.SpeedFast,
				Status:    kinds.SpeedResolutionStatusApplied,
			},
			UpdatedAt: time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC),
		},
		runArtifacts: runArtifacts,
		turn:         1,
		turnPaths: execTurnPaths{
			promptPath:   filepath.Join(tmpDir, "prompt.md"),
			responsePath: filepath.Join(tmpDir, "response.txt"),
			resultPath:   filepath.Join(tmpDir, "result.json"),
		},
	}

	if err := state.completeDryRun("summarize the repository"); err != nil {
		t.Fatalf("completeDryRun: %v", err)
	}

	promptBytes, err := os.ReadFile(state.turnPaths.promptPath)
	if err != nil {
		t.Fatalf("read prompt artifact: %v", err)
	}
	if got := string(promptBytes); got != "summarize the repository" {
		t.Fatalf("unexpected prompt artifact: %q", got)
	}

	responseBytes, err := os.ReadFile(state.turnPaths.responsePath)
	if err != nil {
		t.Fatalf("read response artifact: %v", err)
	}
	if got := string(responseBytes); got != "summarize the repository" {
		t.Fatalf("unexpected response artifact: %q", got)
	}

	var turn persistedExecTurn
	resultBytes, err := os.ReadFile(state.turnPaths.resultPath)
	if err != nil {
		t.Fatalf("read result artifact: %v", err)
	}
	if err := json.Unmarshal(resultBytes, &turn); err != nil {
		t.Fatalf("decode turn result: %v", err)
	}
	if !turn.DryRun || turn.Status != runStatusSucceeded {
		t.Fatalf("unexpected dry-run turn result: %#v", turn)
	}
	if turn.Speed != kinds.SpeedFast {
		t.Fatalf("dry-run requested speed = %q, want fast", turn.Speed)
	}
	if turn.SpeedResolution != nil {
		t.Fatalf("dry-run turn resolution = %#v, want omitted", turn.SpeedResolution)
	}
	if state.record.SpeedResolution != nil {
		t.Fatalf("dry-run latest run resolution = %#v, want omitted", state.record.SpeedResolution)
	}
	if strings.Contains(string(resultBytes), "speed_resolution") {
		t.Fatalf("dry-run result should omit speed resolution: %s", string(resultBytes))
	}
}

func TestFinalizeExecResultWrapsCompletionErrorsAsReported(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	state := &execRunState{
		turnPaths: execTurnPaths{
			responsePath: filepath.Join(tmpDir, "missing", "response.txt"),
		},
	}

	err := finalizeExecResult(state, execExecutionResult{
		status: runStatusFailed,
		err:    errors.New("boom"),
	})
	if err == nil {
		t.Fatal("expected finalizeExecResult to fail")
	}
	if !IsExecErrorReported(err) {
		t.Fatalf("expected finalizeExecResult to return reported error, got %T", err)
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected wrapped error to retain original cause, got %v", err)
	}
}

func TestExecRetryHelpersCoverRetryableAndBoundedTimeouts(t *testing.T) {
	t.Parallel()

	if !isExecRetryableError(newActivityTimeoutError(time.Second)) {
		t.Fatal("expected activity timeout to be retryable")
	}
	if !isExecRetryableError(&agent.SessionSetupError{
		Stage: agent.SessionSetupStageNewSession,
		Err:   errors.New("retry"),
	}) {
		t.Fatal("expected session setup errors to be retryable")
	}
	if isExecRetryableError(&agent.SessionSetupError{
		Stage: agent.SessionSetupStageNewSession,
		Err: &agent.AuthenticationRequiredError{
			Err: &agent.SessionError{Code: -32000, Message: "Authentication required"},
		},
	}) {
		t.Fatal("expected authentication setup errors not to be retryable")
	}
	if isExecRetryableError(errors.New("plain")) {
		t.Fatal("expected plain errors not to be retryable")
	}

	if got := nextRetryTimeout(5*time.Second, 2); got != 10*time.Second {
		t.Fatalf("unexpected retry timeout growth: %v", got)
	}
	if got := nextRetryTimeout(40*time.Minute, 2); got != 30*time.Minute {
		t.Fatalf("expected retry timeout cap, got %v", got)
	}
	if !equalStringSlices([]string{"a", "b"}, []string{"a", "b"}) {
		t.Fatal("expected equalStringSlices to match identical slices")
	}
	if equalStringSlices([]string{"a"}, []string{"b"}) {
		t.Fatal("expected equalStringSlices to reject mismatched slices")
	}
}

func TestShouldRetryExecAttemptSkipsResumedSessions(t *testing.T) {
	t.Parallel()

	retryableErr := newActivityTimeoutError(time.Second)
	if !shouldRetryExecAttempt(retryableErr, 1, 2, &job{}) {
		t.Fatal("expected retryable exec attempt without resume session to retry")
	}
	if shouldRetryExecAttempt(retryableErr, 1, 2, &job{ResumeSession: "sess-existing"}) {
		t.Fatal("expected resumed exec attempt to skip retries")
	}
	if shouldRetryExecAttempt(retryableErr, 1, 2, &job{
		SpeedResolution: kinds.SpeedResolution{
			Requested: kinds.SpeedFast,
			Status:    kinds.SpeedResolutionStatusRejected,
			Reason:    kinds.SpeedResolutionReasonProviderRejected,
		},
	}) {
		t.Fatal("expected rejected speed setup to skip retries")
	}
}

func TestLoadPersistedExecRunDefaultsPathsAndResumeValidation(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	runArtifacts := model.NewRunArtifacts(tmpDir, "exec-123")
	if err := os.MkdirAll(runArtifacts.RunDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	record := PersistedExecRun{
		Version:         execRunSchemaVersion,
		Mode:            model.ModeExec,
		RunID:           "exec-123",
		Status:          "running",
		WorkspaceRoot:   tmpDir,
		IDE:             model.IDECodex,
		Model:           "gpt-5.5",
		ReasoningEffort: "medium",
		AccessMode:      "workspace-write",
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
		ACPSessionID:    "sess-123",
	}
	payload, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal run record: %v", err)
	}
	if err := os.WriteFile(runArtifacts.RunMetaPath, payload, 0o600); err != nil {
		t.Fatalf("write run record: %v", err)
	}

	loaded, err := LoadPersistedExecRun(tmpDir, "exec-123")
	if err != nil {
		t.Fatalf("LoadPersistedExecRun: %v", err)
	}
	if loaded.EventsPath != runArtifacts.EventsPath {
		t.Fatalf("expected default events path %q, got %q", runArtifacts.EventsPath, loaded.EventsPath)
	}
	if loaded.TurnsDir != runArtifacts.TurnsDir {
		t.Fatalf("expected default turns dir %q, got %q", runArtifacts.TurnsDir, loaded.TurnsDir)
	}
	if loaded.Speed != kinds.SpeedNormal {
		t.Fatalf("legacy persisted speed = %q, want %q", loaded.Speed, kinds.SpeedNormal)
	}
	if loaded.SpeedResolution != nil {
		t.Fatalf("legacy persisted resolution = %#v, want omitted", loaded.SpeedResolution)
	}

	var legacyTurn persistedExecTurn
	if err := json.Unmarshal([]byte(`{"turn":1,"status":"succeeded"}`), &legacyTurn); err != nil {
		t.Fatalf("decode legacy turn result: %v", err)
	}
	if legacyTurn.Speed != "" || legacyTurn.SpeedResolution != nil {
		t.Fatalf("legacy turn speed fields = %q / %#v, want omitted", legacyTurn.Speed, legacyTurn.SpeedResolution)
	}

	err = validateExecResumeCompatibility(&model.RuntimeConfig{
		RunID:           "exec-123",
		WorkspaceRoot:   tmpDir,
		IDE:             model.IDECodex,
		Model:           "gpt-5.5",
		ReasoningEffort: "medium",
		Speed:           kinds.SpeedNormal,
		AccessMode:      "workspace-write",
	}, loaded)
	if err != nil {
		t.Fatalf("validateExecResumeCompatibility: %v", err)
	}

	err = validateExecResumeCompatibility(&model.RuntimeConfig{
		RunID:         "exec-123",
		WorkspaceRoot: filepath.Join(tmpDir, "other"),
		IDE:           model.IDECodex,
		Model:         "gpt-5.5",
	}, loaded)
	if err == nil || !strings.Contains(err.Error(), "belongs to workspace") {
		t.Fatalf("expected workspace mismatch error, got %v", err)
	}

	err = validateExecResumeCompatibility(&model.RuntimeConfig{
		RunID:           "exec-123",
		WorkspaceRoot:   tmpDir,
		IDE:             model.IDECodex,
		Model:           "gpt-5.5",
		ReasoningEffort: "medium",
		Speed:           kinds.SpeedFast,
		AccessMode:      "workspace-write",
	}, loaded)
	if err == nil || !strings.Contains(err.Error(), "persisted exec runtime configuration") {
		t.Fatalf("expected core speed mismatch error, got %v", err)
	}
}

func TestRuntimeEventHelperUtilities(t *testing.T) {
	t.Parallel()

	if got := providerStatusCode(nil); got != 200 {
		t.Fatalf("expected synthetic success status 200, got %d", got)
	}
	if got := providerStatusCode(errors.New("plain")); got != 0 {
		t.Fatalf("expected status 0 for plain error, got %d", got)
	}
	if got := issueIDFromPath("/tmp/reviews/issue_001.md"); got != "issue_001.md" {
		t.Fatalf("unexpected issue id from path: %q", got)
	}
}

func TestApplyExecPromptPostBuildHookMutatesPrompt(t *testing.T) {
	t.Parallel()

	t.Run("Should mutate prompt via prompt.post_build hook", func(t *testing.T) {
		manager := &execHookManager{
			dispatchMutable: func(_ context.Context, hook string, payload any) (any, error) {
				if hook != "prompt.post_build" {
					t.Fatalf("unexpected mutable hook %q", hook)
				}

				current, ok := payload.(execPromptPostBuildPayload)
				if !ok {
					t.Fatalf("payload type = %T, want execPromptPostBuildPayload", payload)
				}
				current.PromptText += "\n\nDecorated by exec hook."
				return current, nil
			},
		}
		state := &execRunState{
			ctx:            context.Background(),
			runArtifacts:   model.NewRunArtifacts(t.TempDir(), "exec-hook-run"),
			runtimeManager: manager,
		}

		got, err := applyExecPromptPostBuildHook(context.Background(), state, "decorate me")
		if err != nil {
			t.Fatalf("applyExecPromptPostBuildHook: %v", err)
		}
		if got != "decorate me\n\nDecorated by exec hook." {
			t.Fatalf("unexpected prompt mutation: %q", got)
		}
	})
}

func TestExecRunStateDispatchesRunHooks(t *testing.T) {
	t.Run("Should dispatch run hooks and allow safe config mutation", func(t *testing.T) {
		manager := &execHookManager{
			dispatchMutable: func(_ context.Context, hook string, payload any) (any, error) {
				if hook != "run.pre_start" {
					return payload, nil
				}

				current, ok := payload.(execRunPreStartPayload)
				if !ok {
					t.Fatalf("payload type = %T, want execRunPreStartPayload", payload)
				}
				current.Config.Model = "codex-fast"
				return current, nil
			},
		}
		cfg := &model.RuntimeConfig{
			WorkspaceRoot: workspaceRootForExecTest(t),
			IDE:           model.IDECodex,
			Model:         "gpt-5.5",
			AccessMode:    model.AccessModeDefault,
		}
		state := &execRunState{
			ctx:            context.Background(),
			record:         PersistedExecRun{UpdatedAt: time.Now().UTC()},
			runArtifacts:   model.NewRunArtifacts(t.TempDir(), "exec-run-hooks"),
			runtimeManager: manager,
		}

		if err := applyExecRunPreStartHook(context.Background(), state, cfg); err != nil {
			t.Fatalf("applyExecRunPreStartHook: %v", err)
		}
		if cfg.Model != "codex-fast" {
			t.Fatalf("expected run.pre_start to mutate model, got %q", cfg.Model)
		}

		if err := state.writeStarted(cfg); err != nil {
			t.Fatalf("writeStarted: %v", err)
		}
		if err := state.completeTurn(execExecutionResult{
			status: runStatusSucceeded,
			output: "done",
		}); err != nil {
			t.Fatalf("completeTurn: %v", err)
		}

		if got := len(manager.observerPayloads["run.post_start"]); got != 1 {
			t.Fatalf("expected one run.post_start payload, got %d", got)
		}
		if got := len(manager.observerPayloads["run.pre_shutdown"]); got != 1 {
			t.Fatalf("expected one run.pre_shutdown payload, got %d", got)
		}
		payloads := manager.observerPayloads["run.post_shutdown"]
		if len(payloads) != 1 {
			t.Fatalf("expected one run.post_shutdown payload, got %d", len(payloads))
		}
		payload, ok := payloads[0].(execRunPostShutdownPayload)
		if !ok {
			t.Fatalf("payload type = %T, want execRunPostShutdownPayload", payloads[0])
		}
		if payload.Summary.Status != runStatusSucceeded || payload.Summary.JobsSucceeded != 1 {
			t.Fatalf("unexpected run.post_shutdown summary: %#v", payload.Summary)
		}
	})
}

func TestValidateExecPreparedStateMutationRejectsStateDefiningChanges(t *testing.T) {
	t.Parallel()

	baseCfg := &model.RuntimeConfig{
		WorkspaceRoot: "/tmp/workspace",
		DryRun:        true,
		OutputFormat:  model.OutputFormatText,
	}
	snapshot := snapshotExecPreparedStateConfig(baseCfg)

	testCases := []struct {
		name        string
		mutate      func(*model.RuntimeConfig)
		expectedErr string
	}{
		{
			name: "Should reject workspace root mutations",
			mutate: func(cfg *model.RuntimeConfig) {
				cfg.WorkspaceRoot = "/tmp/other"
			},
			expectedErr: "workspace_root",
		},
		{
			name: "Should reject run id mutations",
			mutate: func(cfg *model.RuntimeConfig) {
				cfg.RunID = "exec-123"
			},
			expectedErr: "run_id",
		},
		{
			name: "Should reject dry run mutations",
			mutate: func(cfg *model.RuntimeConfig) {
				cfg.DryRun = false
			},
			expectedErr: "dry_run",
		},
		{
			name: "Should reject persist mutations",
			mutate: func(cfg *model.RuntimeConfig) {
				cfg.Persist = true
			},
			expectedErr: "persist",
		},
		{
			name: "Should reject output format mutations",
			mutate: func(cfg *model.RuntimeConfig) {
				cfg.OutputFormat = model.OutputFormatJSON
			},
			expectedErr: "output_format",
		},
		{
			name: "Should reject tui mutations",
			mutate: func(cfg *model.RuntimeConfig) {
				cfg.TUI = true
			},
			expectedErr: "tui",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := *baseCfg
			tc.mutate(&cfg)

			err := validateExecPreparedStateMutation(snapshot, &cfg)
			if err == nil || !strings.Contains(err.Error(), tc.expectedErr) {
				t.Fatalf("expected error containing %q, got %v", tc.expectedErr, err)
			}
		})
	}
}

type execHookManager struct {
	dispatchMutable  func(context.Context, string, any) (any, error)
	observerPayloads map[string][]any
}

func (m *execHookManager) Start(context.Context) error { return nil }

func (m *execHookManager) DispatchMutableHook(ctx context.Context, hook string, payload any) (any, error) {
	if m == nil {
		return payload, nil
	}
	if hook == "run.post_shutdown" {
		if m.observerPayloads == nil {
			m.observerPayloads = make(map[string][]any)
		}
		m.observerPayloads[hook] = append(m.observerPayloads[hook], payload)
	}
	if m.dispatchMutable != nil {
		return m.dispatchMutable(ctx, hook, payload)
	}
	return payload, nil
}

func (m *execHookManager) DispatchObserverHook(_ context.Context, hook string, payload any) {
	if m == nil {
		return
	}
	if m.observerPayloads == nil {
		m.observerPayloads = make(map[string][]any)
	}
	m.observerPayloads[hook] = append(m.observerPayloads[hook], payload)
}

func (m *execHookManager) Shutdown(context.Context) error { return nil }

func workspaceRootForExecTest(t *testing.T) string {
	t.Helper()

	workspaceRoot := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Join(workspaceRoot, model.WorkflowRootDirName), 0o755); err != nil {
		t.Fatalf("mkdir workflow root: %v", err)
	}
	installRuntimeProbeStub(t, "codex-acp")
	return workspaceRoot
}

func installRuntimeProbeStub(t *testing.T, command string) {
	t.Helper()

	binDir := t.TempDir()
	scriptPath := filepath.Join(binDir, command)
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write runtime probe stub: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func preparedPromptTextContentBlock(t *testing.T, text string) model.ContentBlock {
	t.Helper()

	payload, err := json.Marshal(model.TextBlock{
		Type: model.BlockText,
		Text: text,
	})
	if err != nil {
		t.Fatalf("marshal text content block: %v", err)
	}
	return model.ContentBlock{
		Type: model.BlockText,
		Data: payload,
	}
}
