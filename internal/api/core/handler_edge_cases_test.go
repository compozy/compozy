package core_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/api/testutil"
	automationpkg "github.com/compozy/agh/internal/automation"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/observe"
	"github.com/compozy/agh/internal/session"
	settingspkg "github.com/compozy/agh/internal/settings"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/transcript"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

type unpagedSessionManager struct {
	core.SessionManager
}

type sessionHealthReaderFunc func(context.Context, string) (heartbeat.SessionHealth, error)

func (f sessionHealthReaderFunc) GetSessionHealth(
	ctx context.Context,
	sessionID string,
) (heartbeat.SessionHealth, error) {
	return f(ctx, sessionID)
}

type sessionHealthPageReaderStub struct {
	getFn  func(context.Context, string) (heartbeat.SessionHealth, error)
	pageFn func(context.Context, []*session.Info) (map[string]heartbeat.SessionHealth, error)
}

func (s sessionHealthPageReaderStub) GetSessionHealth(
	ctx context.Context,
	sessionID string,
) (heartbeat.SessionHealth, error) {
	return s.getFn(ctx, sessionID)
}

func (s sessionHealthPageReaderStub) SessionHealthForPage(
	ctx context.Context,
	infos []*session.Info,
) (map[string]heartbeat.SessionHealth, error) {
	return s.pageFn(ctx, infos)
}

type bufferFlusher struct {
	bytes.Buffer
}

func (bufferFlusher) Flush() {}

type failingFlusher struct {
	writes int
}

func (f *failingFlusher) Write(p []byte) (int, error) {
	f.writes++
	if f.writes > 1 {
		return 0, io.ErrClosedPipe
	}
	return len(p), nil
}

func (*failingFlusher) Flush() {}

type failNthWriteFlusher struct {
	writes int
	failAt int
	err    error
}

func (f *failNthWriteFlusher) Write(p []byte) (int, error) {
	f.writes++
	if f.writes == f.failAt {
		if f.err != nil {
			return 0, f.err
		}
		return 0, io.ErrClosedPipe
	}
	return len(p), nil
}

func (*failNthWriteFlusher) Flush() {}

func TestObserveAndSSEHelpers(t *testing.T) {
	t.Parallel()

	timestamp := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	event := store.EventSummary{
		ID:        "ev-1",
		SessionID: "sess-1",
		Sequence:  7,
		Type:      "agent_message",
		AgentName: "coder",
		Timestamp: timestamp,
	}

	if !core.LogEventAfterCursor(event, core.LogsCursor{}) {
		t.Fatal("LogEventAfterCursor(empty cursor) = false, want true")
	}
	if core.LogEventAfterCursor(event, core.LogsCursor{Timestamp: timestamp.Add(time.Second), ID: "older"}) {
		t.Fatal("LogEventAfterCursor(newer cursor) = true, want false")
	}
	if core.LogEventAfterCursor(event, core.LogsCursor{Timestamp: timestamp, Sequence: 9}) {
		t.Fatal("LogEventAfterCursor(same timestamp higher sequence) = true, want false")
	}
	if got, want := core.LogEventID(event), "2026-04-03T12:00:00Z|00000000000000000007"; got != want {
		t.Fatalf("LogEventID() = %q, want %q", got, want)
	}

	writer := &bufferFlusher{}
	next := core.EmitLogs(writer, []store.EventSummary{event}, core.LogsCursor{})
	if next.Sequence != event.Sequence || next.Timestamp.IsZero() {
		t.Fatalf("EmitLogs() cursor = %#v", next)
	}
	if writer.Len() == 0 {
		t.Fatal("expected SSE output to be written")
	}

	failingWriter := &failingFlusher{}
	prior := core.LogsCursor{Timestamp: timestamp.Add(-time.Second), Sequence: 3, ID: "legacy"}
	if got := core.EmitLogs(failingWriter, []store.EventSummary{event}, prior); got != prior {
		t.Fatalf("EmitLogs(failing writer) cursor = %#v, want %#v", got, prior)
	}

	if err := core.WriteSSE(
		writer,
		core.SSEMessage{ID: "2", Name: "done", Data: map[string]string{"ok": "true"}},
	); err != nil {
		t.Fatalf("WriteSSE() error = %v", err)
	}
	if err := core.WriteSSERaw(writer, "3", `"raw"`, "raw"); err != nil {
		t.Fatalf("WriteSSERaw() error = %v", err)
	}
	if err := core.WriteSSE(nil, core.SSEMessage{}); err == nil {
		t.Fatal("WriteSSE(nil) error = nil, want non-nil")
	}
	if err := core.WriteSSERaw(nil, "", "null"); err == nil {
		t.Fatal("WriteSSERaw(nil) error = nil, want non-nil")
	}
}

func TestWriteSSERawWrapsStepFailuresWithContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		failAt int
		want   string
	}{
		{name: "ShouldWrapIDPrefixFailure", failAt: 1, want: "write sse id prefix"},
		{name: "ShouldWrapEventPrefixFailure", failAt: 4, want: "write sse event prefix"},
		{name: "ShouldWrapDataPrefixFailure", failAt: 7, want: "write sse data prefix"},
		{name: "ShouldWrapPayloadFailure", failAt: 8, want: "write sse data payload"},
		{name: "ShouldWrapTerminatorFailure", failAt: 9, want: "write sse message terminator"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			writer := &failNthWriteFlusher{failAt: tt.failAt, err: io.ErrClosedPipe}
			err := core.WriteSSERaw(writer, "msg-1", `"raw"`, "done")
			if !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("WriteSSERaw() error = %v, want %v", err, io.ErrClosedPipe)
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("WriteSSERaw() error = %v, want context %q", err, tt.want)
			}
		})
	}
}

func TestConversionAndStatusHelpers(t *testing.T) {
	t.Parallel()

	usageValue := int64(10)
	goalTurn := 2
	clientMessageID := "client-message-1"
	event := acp.AgentEvent{
		Type:      acp.EventTypePermission,
		SessionID: "sess-1",
		TurnID:    "turn-1",
		Timestamp: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
		Action:    "fs/read_text_file",
		Failure: &store.SessionFailure{
			Kind:    store.FailurePermission,
			Summary: "permission policy denied",
		},
		Usage: &acp.TokenUsage{
			InputTokens: &usageValue,
			Timestamp:   time.Date(2026, 4, 3, 12, 0, 1, 0, time.UTC),
		},
		Goal: &acp.GoalPromptMeta{
			Kind: "goal-continuation", RunID: "run-goal", NodeID: "converge", Generation: 3,
			ItemIndex: 1, Turn: &goalTurn, PromptAttempt: 2, PromptID: "goal-prompt-2",
		},
		Raw: []byte(`{"ok":true}`),
	}.WithClientMessageID(clientMessageID)
	agentEvent := core.AgentEventPayloadFromEvent(event)
	if agentEvent.Type != acp.EventTypePermission || agentEvent.Usage == nil || agentEvent.Usage.InputTokens == nil {
		t.Fatalf("agent event payload = %#v", agentEvent)
	}
	if got, want := agentEvent.ClientMessageID, "client-message-1"; got != want {
		t.Fatalf("agent event client_message_id = %q, want %q", got, want)
	}
	if agentEvent.Failure == nil || agentEvent.Failure.Kind != store.FailurePermission {
		t.Fatalf("agent event failure = %#v", agentEvent.Failure)
	}
	if agentEvent.Goal == nil || agentEvent.Goal.Kind != contract.GoalPromptContinuation ||
		agentEvent.Goal.RunID != "run-goal" || agentEvent.Goal.Turn == nil || *agentEvent.Goal.Turn != goalTurn {
		t.Fatalf("agent event Goal metadata = %#v", agentEvent.Goal)
	}
	storedGoalEvent, err := transcript.MarshalAgentEvent(acp.AgentEvent{Goal: &acp.GoalPromptMeta{
		Kind: "goal-continuation", RunID: "run-goal", NodeID: "converge", Generation: 3,
		ItemIndex: 1, Turn: &goalTurn, PromptAttempt: 2, PromptID: "goal-prompt-2",
	}})
	if err != nil {
		t.Fatalf("MarshalAgentEvent(Goal) error = %v", err)
	}
	sessionEvent := core.SessionEventPayloadFromEvent(store.SessionEvent{Content: storedGoalEvent}, nil)
	if sessionEvent.Goal == nil || sessionEvent.Goal.Kind != contract.GoalPromptContinuation ||
		sessionEvent.Goal.PromptID != "goal-prompt-2" {
		t.Fatalf("session event Goal metadata = %#v", sessionEvent.Goal)
	}
	if got := string(agentEvent.Raw); got != `{"ok":true}` {
		t.Fatalf("agent event raw payload = %s, want valid JSON passthrough", got)
	}
	plainRawEvent := core.AgentEventPayloadFromEvent(acp.AgentEvent{Raw: []byte("plain-text")})
	if got := string(plainRawEvent.Raw); got != `"plain-text"` {
		t.Fatalf("plain raw payload = %s, want quoted string", got)
	}
	if payload := core.PayloadJSON("plain-text"); string(payload) == "plain-text" {
		t.Fatalf("PayloadJSON() = %s, want quoted JSON", string(payload))
	}
	if status := core.StatusForWorkspaceError(workspacepkg.ErrWorkspacePathTaken); status != http.StatusConflict {
		t.Fatalf("StatusForWorkspaceError() = %d, want %d", status, http.StatusConflict)
	}
	if status := core.StatusForMemoryError(errors.New("boom")); status != http.StatusInternalServerError {
		t.Fatalf("StatusForMemoryError(default) = %d, want %d", status, http.StatusInternalServerError)
	}
	if status := core.StatusForMemoryError(nil); status != http.StatusOK {
		t.Fatalf("StatusForMemoryError(nil) = %d, want %d", status, http.StatusOK)
	}
	if got := core.NewMemoryValidationError(nil); got != nil {
		t.Fatalf("NewMemoryValidationError(nil) = %v, want nil", got)
	}

	sessions := core.SessionPayloadsForWorkspace([]*session.Info{
		{ID: "sess-1", WorkspaceID: "ws_alpha"},
		{ID: "sess-2", WorkspaceID: "ws_beta"},
		{ID: "sess-3", WorkspaceID: "ws_alpha", Type: session.SessionTypeDream},
		{
			ID:          "sess-4",
			WorkspaceID: "ws_alpha",
			Type:        session.SessionTypeSpawned,
			Lineage: &store.SessionLineage{
				ParentSessionID: "sess-1",
				RootSessionID:   "sess-1",
				SpawnDepth:      1,
				SpawnRole:       session.SpawnRoleMemoryExtractor,
			},
		},
		{
			ID:          "sess-5",
			WorkspaceID: "ws_alpha",
			Type:        session.SessionTypeSpawned,
			Lineage: &store.SessionLineage{
				ParentSessionID: "sess-1",
				RootSessionID:   "sess-1",
				SpawnDepth:      1,
				SpawnRole:       session.SpawnRoleAutoTitle,
			},
		},
	}, "ws_alpha")
	if len(sessions) != 1 || sessions[0].ID != "sess-1" {
		t.Fatalf("SessionPayloadsForWorkspace() = %#v", sessions)
	}
}

func TestPromptResponseFromSessionShouldEnforceClosedGoalOutcomeMatrix(t *testing.T) {
	t.Parallel()

	snapshot := &session.GoalSnapshot{
		RunID: "run-1", NodeID: "goal", Objective: "Ship the release",
		OriginSessionID: "session-1", BoundSessionID: "session-1",
		Status: "active", RunStatus: "running", TurnsUsed: 0, TurnLimit: 7, Live: true,
		ContractSummary: "Ship the release",
		Context:         session.GoalContextSnapshot{State: "unknown", NudgeRatio: 0},
	}
	replacedRunID := "run-old"
	blankRunID := "   "
	used, size, ratio := int64(10), int64(20), 1.1
	reportedAt := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	invalidContextSnapshot := *snapshot
	invalidContextSnapshot.Context = session.GoalContextSnapshot{
		State: "known", Used: &used, Size: &size, Ratio: &ratio, ReportedAt: &reportedAt,
		NudgeRatio: 0.8,
	}
	replaceRequired := session.GoalReasonReplaceRequired
	replaceStale := session.GoalReasonReplaceStale
	notActive := session.GoalReasonNotActive
	invalid := session.GoalReasonCommandInvalid
	tests := []struct {
		name       string
		result     *session.GoalCommandResult
		wantStatus int
		wantError  bool
		wantErrMsg string
	}{
		{
			name:       "Should map started to accepted",
			result:     &session.GoalCommandResult{Outcome: session.GoalOutcomeStarted, Snapshot: snapshot},
			wantStatus: http.StatusAccepted,
		},
		{
			name: "Should map replaced to accepted",
			result: &session.GoalCommandResult{
				Outcome:       session.GoalOutcomeReplaced,
				Snapshot:      snapshot,
				ReplacedRunID: &replacedRunID,
			},
			wantStatus: http.StatusAccepted,
		},
		{
			name:       "Should map status to OK",
			result:     &session.GoalCommandResult{Outcome: session.GoalOutcomeStatus, Snapshot: snapshot},
			wantStatus: http.StatusOK,
		},
		{
			name:       "Should map clear to OK",
			result:     &session.GoalCommandResult{Outcome: session.GoalOutcomeCleared},
			wantStatus: http.StatusOK,
		},
		{
			name:       "Should map not active to not found",
			result:     &session.GoalCommandResult{Outcome: session.GoalOutcomeError, ReasonCode: &notActive},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "Should map replace required to conflict",
			result: &session.GoalCommandResult{
				Outcome:    session.GoalOutcomeError,
				ReasonCode: &replaceRequired,
				Snapshot:   snapshot,
			},
			wantStatus: http.StatusConflict,
		},
		{
			name: "Should map replace stale to conflict",
			result: &session.GoalCommandResult{
				Outcome:    session.GoalOutcomeError,
				ReasonCode: &replaceStale,
				Snapshot:   snapshot,
			},
			wantStatus: http.StatusConflict,
		},
		{
			name:       "Should map invalid command to unprocessable content",
			result:     &session.GoalCommandResult{Outcome: session.GoalOutcomeError, ReasonCode: &invalid},
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "Should reject started without a snapshot",
			result:     &session.GoalCommandResult{Outcome: session.GoalOutcomeStarted},
			wantError:  true,
			wantErrMsg: "Goal started result requires a snapshot",
		},
		{
			name:       "Should reject stale replace without a current snapshot",
			result:     &session.GoalCommandResult{Outcome: session.GoalOutcomeError, ReasonCode: &replaceStale},
			wantError:  true,
			wantErrMsg: `Goal error reason "goal_replace_stale" requires current snapshot: true`,
		},
		{
			name: "Should reject a whitespace-only replacement run ID",
			result: &session.GoalCommandResult{
				Outcome:       session.GoalOutcomeReplaced,
				Snapshot:      snapshot,
				ReplacedRunID: &blankRunID,
			},
			wantError:  true,
			wantErrMsg: "Goal replacement run ID must not be blank",
		},
		{
			name: "Should reject a known context ratio above one",
			result: &session.GoalCommandResult{
				Outcome:  session.GoalOutcomeStatus,
				Snapshot: &invalidContextSnapshot,
			},
			wantError:  true,
			wantErrMsg: "known Goal context requires valid usage metrics",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			status, body, err := core.PromptResponseFromSession(
				session.SendPromptResult{Status: "goal", Goal: tt.result},
			)
			if tt.wantError {
				if err == nil {
					t.Fatal("PromptResponseFromSession() error = nil")
				}
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Fatalf("PromptResponseFromSession() error = %v, want substring %q", err, tt.wantErrMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("PromptResponseFromSession() error = %v", err)
			}
			if status != tt.wantStatus {
				t.Fatalf("status = %d, want %d", status, tt.wantStatus)
			}
			result, ok := body.(contract.GoalCommandResult)
			if !ok {
				t.Fatalf("body type = %T, want contract.GoalCommandResult", body)
			}
			if result.Outcome != contract.GoalCommandOutcome(tt.result.Outcome) {
				t.Fatalf("body outcome = %q, want %q", result.Outcome, tt.result.Outcome)
			}
			if (result.ReasonCode == nil) != (tt.result.ReasonCode == nil) ||
				(result.ReasonCode != nil && string(*result.ReasonCode) != string(*tt.result.ReasonCode)) {
				t.Fatalf("body reason = %v, want %v", result.ReasonCode, tt.result.ReasonCode)
			}
			if (result.ReplacedRunID == nil) != (tt.result.ReplacedRunID == nil) ||
				(result.ReplacedRunID != nil && *result.ReplacedRunID != strings.TrimSpace(*tt.result.ReplacedRunID)) {
				t.Fatalf("body replaced_run_id = %v, want %v", result.ReplacedRunID, tt.result.ReplacedRunID)
			}
		})
	}
}

func TestBaseHandlersWorkspaceFilteringAndDefaults(t *testing.T) {
	t.Parallel()

	manager := testutil.StubSessionManager{
		ListAllFn: func(context.Context) ([]*session.Info, error) {
			return []*session.Info{
				{ID: "sess-1", WorkspaceID: "ws_alpha"},
				{ID: "sess-2", WorkspaceID: "ws_beta"},
			}, nil
		},
	}
	workspaces := testutil.StubWorkspaceService{
		GetFn: func(_ context.Context, ref string) (workspacepkg.Workspace, error) {
			if ref != "alpha" {
				t.Fatalf("Get workspace ref = %q, want alpha", ref)
			}
			return workspacepkg.Workspace{ID: "ws_alpha", RootDir: "/workspace"}, nil
		},
	}
	fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, workspaces, nil, nil)

	resp := performRequest(t, fixture.Engine, http.MethodGet, "/sessions?workspace=alpha", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("filtered list status = %d, want %d", resp.Code, http.StatusOK)
	}

	fixture.Handlers.SetHTTPPort(4321)
	recorder := performRequest(t, fixture.Engine, http.MethodGet, "/status", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("daemon status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var payload struct {
		Daemon contract.DaemonStatusPayload `json:"daemon"`
	}
	testutil.DecodeJSONResponse(t, recorder, &payload)
	if payload.Daemon.HTTPPort != 4321 {
		t.Fatalf("daemon http port = %d, want 4321", payload.Daemon.HTTPPort)
	}
	resolvedUserHomeDir, err := aghconfig.ResolvePath(os.Getenv("HOME"))
	if err != nil {
		t.Fatalf("ResolvePath(HOME) error = %v", err)
	}
	if payload.Daemon.UserHomeDir != resolvedUserHomeDir {
		t.Fatalf("daemon user home dir = %q, want %q", payload.Daemon.UserHomeDir, resolvedUserHomeDir)
	}

	handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{})
	if handlers.TransportName != "" {
		t.Fatalf("TransportName default = %q, want empty", handlers.TransportName)
	}
}

func TestMemoryWrapperExports(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if _, err := workspacepkg.EnsureIdentity(context.Background(), workspace); err != nil {
		t.Fatalf("EnsureIdentity(%q) error = %v", workspace, err)
	}
	req := contract.MemoryWriteRequest{
		Scope:     "workspace",
		Workspace: workspace,
		Content:   "---\nname: Project\ndescription: desc\ntype: project\n---\n\nbody",
	}
	scope, resolvedWorkspace, err := core.ResolveMemoryWriteScope(req)
	if err != nil {
		t.Fatalf("ResolveMemoryWriteScope() error = %v", err)
	}
	if scope != memcontract.ScopeWorkspace || resolvedWorkspace == "" {
		t.Fatalf("scope=%q workspace=%q", scope, resolvedWorkspace)
	}
	if _, err := core.ParseOptionalMemoryScope("bogus"); err == nil {
		t.Fatal("ParseOptionalMemoryScope(bogus) error = nil, want non-nil")
	}
	if _, err := core.ResolveMemoryWorkspace(""); err == nil {
		t.Fatal("ResolveMemoryWorkspace(\"\") error = nil, want non-nil")
	}
	if scope, resolved, err := core.ResolveMemoryWriteScope(contract.MemoryWriteRequest{
		Content: "---\nname: Global\ndescription: desc\ntype: user\n---\n\nbody",
	}); err != nil || scope != memcontract.ScopeGlobal || resolved != "" {
		t.Fatalf("ResolveMemoryWriteScope(user default) = %q %q %v", scope, resolved, err)
	}

	store := memory.NewStore(t.TempDir())
	if err := store.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs() error = %v", err)
	}
	if err := store.ForWorkspace(workspace).
		Write(memcontract.ScopeWorkspace, "note.md", []byte("---\nname: note\ndescription: desc\ntype: project\n---\n\nbody")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	manager := testutil.StubSessionManager{
		ListAllFn: func(context.Context) ([]*session.Info, error) {
			info := testutil.NewSessionInfo("sess-a")
			info.Workspace = workspace
			return []*session.Info{info}, nil
		},
	}
	fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, testutil.StubWorkspaceService{}, store, nil)
	if _, err := fixture.Handlers.ResolveMemoryLocation("note.md", "workspace", workspace); err != nil {
		t.Fatalf("ResolveMemoryLocation() error = %v", err)
	}
	workspacesOut, err := fixture.Handlers.MemoryHealthWorkspaces(context.Background(), "")
	if err != nil || len(workspacesOut) != 1 {
		t.Fatalf("MemoryHealthWorkspaces() = %#v, %v", workspacesOut, err)
	}
}

func TestHealthHandlerReturnsRetentionAndPersistencePayload(t *testing.T) {
	t.Parallel()

	lastSweepAt := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	lastCutoffAt := lastSweepAt.AddDate(0, 0, -14)
	fixture := newHandlerFixture(
		t,
		testutil.StubSessionManager{},
		testutil.StubObserver{
			HealthFn: func(context.Context) (observe.Health, error) {
				return observe.Health{
					Status:             "degraded",
					ActiveSessions:     2,
					GlobalDBSizeBytes:  4096,
					SessionDBSizeBytes: 2048,
					Persistence: observe.PersistenceHealth{
						Status:             "degraded",
						GlobalDBSizeBytes:  4096,
						SessionDBSizeBytes: 2048,
					},
					Retention: observe.RetentionHealth{
						Enabled:                  true,
						RetentionDays:            14,
						SweepIntervalSeconds:     int64((24 * time.Hour).Seconds()),
						LastSweepStatus:          "error",
						LastSweepAt:              &lastSweepAt,
						LastCutoffAt:             &lastCutoffAt,
						LastSweepError:           "disk full",
						DeletedEventSummaries:    3,
						DeletedTokenStats:        2,
						DeletedPermissionLogRows: 1,
					},
					Failures: observe.FailureHealth{
						Status: "degraded",
						Total:  1,
						ByKind: map[store.FailureKind]int{store.FailureProcess: 1},
						Recent: []observe.SessionFailureHealth{{
							SessionID:       "sess-crash",
							AgentName:       "coder",
							Provider:        "claude",
							WorkspaceID:     "ws-1",
							State:           "stopped",
							FailureKind:     store.FailureProcess,
							Summary:         "provider crashed",
							CrashBundlePath: "/tmp/crash.json",
							UpdatedAt:       lastSweepAt,
						}},
					},
					AgentProbes: []acp.ProbeResult{{
						AgentName:  "coder",
						Provider:   "claude",
						Command:    "missing-agent",
						Status:     acp.ProbeStatusMissing,
						Error:      "not found",
						CheckedAt:  lastSweepAt,
						DurationMS: 7,
					}},
					Version: "dev",
				}, nil
			},
		},
		testutil.StubWorkspaceService{},
		nil,
		nil,
	)

	resp := performRequest(t, fixture.Engine, http.MethodGet, "/status", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("health status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}

	var payload contract.StatusPayload
	decodeJSON(t, resp.Body.Bytes(), &payload)
	if payload.Health.Persistence.Status != "degraded" ||
		payload.Health.Persistence.GlobalDBSizeBytes != 4096 ||
		payload.Health.Persistence.SessionDBSizeBytes != 2048 {
		t.Fatalf("health.persistence = %#v, want degraded persistence payload", payload.Health.Persistence)
	}
	if !payload.Health.Retention.Enabled ||
		payload.Health.Retention.RetentionDays != 14 ||
		payload.Health.Retention.LastSweepStatus != "error" ||
		payload.Health.Retention.LastSweepError != "disk full" ||
		payload.Health.Retention.DeletedEventSummaries != 3 ||
		payload.Health.Retention.DeletedTokenStats != 2 ||
		payload.Health.Retention.DeletedPermissionLogRows != 1 {
		t.Fatalf("health.retention = %#v, want typed retention payload", payload.Health.Retention)
	}
	if payload.Health.Retention.LastSweepAt == nil || !payload.Health.Retention.LastSweepAt.Equal(lastSweepAt) {
		t.Fatalf("health.retention.last_sweep_at = %#v, want %s", payload.Health.Retention.LastSweepAt, lastSweepAt)
	}
	if payload.Health.Retention.LastCutoffAt == nil || !payload.Health.Retention.LastCutoffAt.Equal(lastCutoffAt) {
		t.Fatalf("health.retention.last_cutoff_at = %#v, want %s", payload.Health.Retention.LastCutoffAt, lastCutoffAt)
	}
	if payload.Health.Failures.Status != "degraded" ||
		payload.Health.Failures.Total != 1 ||
		payload.Health.Failures.ByKind[store.FailureProcess] != 1 ||
		len(payload.Health.Failures.Recent) != 1 {
		t.Fatalf("health.failures = %#v, want lifecycle failure payload", payload.Health.Failures)
	}
	if payload.Health.AgentProbes == nil ||
		len(payload.Health.AgentProbes) != 1 ||
		payload.Health.AgentProbes[0].Status != acp.ProbeStatusMissing {
		t.Fatalf("health.agent_probes = %#v, want missing probe payload", payload.Health.AgentProbes)
	}
}

func TestBaseHandlersHealthAndDaemonStatusErrorBranches(t *testing.T) {
	t.Parallel()

	t.Run("Should health observer failure", func(t *testing.T) {
		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{
				HealthFn: func(context.Context) (observe.Health, error) {
					return observe.Health{}, errors.New("boom")
				},
			},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)

		resp := performRequest(t, fixture.Engine, http.MethodGet, "/status", nil)
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf(
				"health status = %d, want %d; body=%s",
				resp.Code,
				http.StatusInternalServerError,
				resp.Body.String(),
			)
		}
	})

	t.Run("Should health memory failure", func(t *testing.T) {
		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{
				HealthFn: func(context.Context) (observe.Health, error) {
					return observe.Health{Status: "ok", Version: "dev"}, nil
				},
			},
			testutil.StubWorkspaceService{},
			nil,
			&stubDreamTrigger{EnabledFn: true, LastErr: errors.New("dream status failed")},
		)

		resp := performRequest(t, fixture.Engine, http.MethodGet, "/status", nil)
		if resp.Code != http.StatusOK {
			t.Fatalf(
				"health status = %d, want %d; body=%s",
				resp.Code,
				http.StatusOK,
				resp.Body.String(),
			)
		}
		var payload contract.StatusPayload
		testutil.DecodeJSONResponse(t, resp, &payload)
		if payload.Memory.Status != "unavailable" || payload.Memory.Reason == "" {
			t.Fatalf("health memory = %#v, want structured unavailable memory payload", payload.Memory)
		}
	})

	t.Run("Should health automation failure", func(t *testing.T) {
		fixture := newHandlerFixtureWithAutomation(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{
				HealthFn: func(context.Context) (observe.Health, error) {
					return observe.Health{Status: "ok", Version: "dev"}, nil
				},
			},
			testutil.StubAutomationManager{
				StatusFn: func(context.Context) (automationpkg.ManagerStatus, error) {
					return automationpkg.ManagerStatus{}, errors.New("automation status failed")
				},
			},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)

		resp := performRequest(t, fixture.Engine, http.MethodGet, "/status", nil)
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf(
				"health status = %d, want %d; body=%s",
				resp.Code,
				http.StatusInternalServerError,
				resp.Body.String(),
			)
		}
	})

	t.Run("Should daemon status session list failure", func(t *testing.T) {
		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{
				ListAllFn: func(context.Context) ([]*session.Info, error) {
					return nil, errors.New("list failed")
				},
			},
			testutil.StubObserver{
				HealthFn: func(context.Context) (observe.Health, error) {
					return observe.Health{Status: "ok", Version: "dev"}, nil
				},
			},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)

		resp := performRequest(t, fixture.Engine, http.MethodGet, "/status", nil)
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf(
				"daemon status = %d, want %d; body=%s",
				resp.Code,
				http.StatusInternalServerError,
				resp.Body.String(),
			)
		}
	})

	t.Run("Should daemon status observer failure", func(t *testing.T) {
		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{
				HealthFn: func(context.Context) (observe.Health, error) {
					return observe.Health{}, errors.New("observer failed")
				},
			},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)

		resp := performRequest(t, fixture.Engine, http.MethodGet, "/status", nil)
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf(
				"daemon status = %d, want %d; body=%s",
				resp.Code,
				http.StatusInternalServerError,
				resp.Body.String(),
			)
		}
	})

	t.Run("Should daemon status network enabled without service", func(t *testing.T) {
		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{
				ListAllFn: func(context.Context) ([]*session.Info, error) {
					return []*session.Info{}, nil
				},
			},
			testutil.StubObserver{
				HealthFn: func(context.Context) (observe.Health, error) {
					return observe.Health{Status: "ok", Version: "dev"}, nil
				},
			},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = nil

		resp := performRequest(t, fixture.Engine, http.MethodGet, "/status", nil)
		if resp.Code != http.StatusOK {
			t.Fatalf(
				"daemon status = %d, want %d; body=%s",
				resp.Code,
				http.StatusOK,
				resp.Body.String(),
			)
		}
		var payload contract.StatusPayload
		testutil.DecodeJSONResponse(t, resp, &payload)
		if payload.Daemon.Network == nil || payload.Daemon.Network.Status != "unavailable" {
			t.Fatalf("daemon network = %#v, want unavailable status", payload.Daemon.Network)
		}
	})

	t.Run("Should daemon status network missing payload", func(t *testing.T) {
		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{
				ListAllFn: func(context.Context) ([]*session.Info, error) {
					return []*session.Info{}, nil
				},
			},
			testutil.StubObserver{
				HealthFn: func(context.Context) (observe.Health, error) {
					return observe.Health{Status: "ok", Version: "dev"}, nil
				},
			},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{
			StatusFn: func(context.Context) (*network.Status, error) {
				return nil, nil
			},
		}

		resp := performRequest(t, fixture.Engine, http.MethodGet, "/status", nil)
		if resp.Code != http.StatusOK {
			t.Fatalf(
				"daemon status = %d, want %d; body=%s",
				resp.Code,
				http.StatusOK,
				resp.Body.String(),
			)
		}
		var payload contract.StatusPayload
		testutil.DecodeJSONResponse(t, resp, &payload)
		if payload.Daemon.Network == nil || payload.Daemon.Network.Status != "unavailable" {
			t.Fatalf("daemon network = %#v, want unavailable status", payload.Daemon.Network)
		}
	})

	t.Run("Should daemon status network failure", func(t *testing.T) {
		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{
				ListAllFn: func(context.Context) ([]*session.Info, error) {
					return []*session.Info{}, nil
				},
			},
			testutil.StubObserver{
				HealthFn: func(context.Context) (observe.Health, error) {
					return observe.Health{Status: "ok", Version: "dev"}, nil
				},
			},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{
			StatusFn: func(context.Context) (*network.Status, error) {
				return nil, errors.New("network failed")
			},
		}

		resp := performRequest(t, fixture.Engine, http.MethodGet, "/status", nil)
		if resp.Code != http.StatusOK {
			t.Fatalf(
				"daemon status = %d, want %d; body=%s",
				resp.Code,
				http.StatusOK,
				resp.Body.String(),
			)
		}
		var payload contract.StatusPayload
		testutil.DecodeJSONResponse(t, resp, &payload)
		if payload.Daemon.Network == nil || payload.Daemon.Network.Status != "unavailable" {
			t.Fatalf("daemon network = %#v, want unavailable status", payload.Daemon.Network)
		}
	})
}

func TestBaseHandlersStatusProjectsWorkspaceMCPDeadState(t *testing.T) {
	t.Parallel()

	settingsService := &stubSettingsService{
		ListCollectionFn: func(
			_ context.Context,
			req settingspkg.CollectionRequest,
		) (settingspkg.CollectionEnvelope, error) {
			return settingspkg.CollectionEnvelope{
				Collection:  req.Collection,
				Scope:       req.Scope,
				WorkspaceID: req.WorkspaceID,
				MCPServers: []settingspkg.MCPServerItem{{
					Name:        "dead-docs",
					Transport:   aghconfig.MCPServerTransportStdio,
					Scope:       settingspkg.ScopeWorkspace,
					WorkspaceID: req.WorkspaceID,
					RuntimeStatus: &settingspkg.MCPServerRuntimeStatus{
						Configured: true,
						State:      settingspkg.MCPServerRuntimeStateDead,
						Probe:      settingspkg.MCPServerProbeSkipped,
						Reason:     "backend_dead",
						Diagnostic: "sidecar terminated",
					},
				}},
			}, nil
		},
	}
	fixture := newHandlerFixture(
		t,
		testutil.StubSessionManager{},
		testutil.StubObserver{
			HealthFn: func(context.Context) (observe.Health, error) {
				return observe.Health{Status: "ok", Version: "dev"}, nil
			},
		},
		testutil.StubWorkspaceService{},
		nil,
		nil,
	)
	fixture.Handlers.Settings = settingsService

	response := performRequest(t, fixture.Engine, http.MethodGet, "/status?workspace_id=ws-dead", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	if settingsService.LastListCollectionRequest.Scope != settingspkg.ScopeWorkspace ||
		settingsService.LastListCollectionRequest.WorkspaceID != "ws-dead" {
		t.Fatalf(
			"MCP collection request = %#v, want workspace ws-dead",
			settingsService.LastListCollectionRequest,
		)
	}
	var payload contract.StatusPayload
	decodeJSON(t, response.Body.Bytes(), &payload)
	if len(payload.MCPServers) != 1 {
		t.Fatalf("status.mcp_servers = %#v, want one dead server", payload.MCPServers)
	}
	server := payload.MCPServers[0]
	if server.Name != "dead-docs" || server.WorkspaceID != "ws-dead" ||
		server.State != "dead" || server.RuntimeStatus != "unavailable" ||
		server.Reason != "backend_dead" || server.Diagnostic != "sidecar terminated" {
		t.Fatalf("status.mcp_servers[0] = %#v, want dead unavailable server with reason", server)
	}
}

func TestBaseHandlersListSessionsPageContract(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Minute)
	workerLineage := &store.SessionLineage{
		ParentSessionID: "sess-user",
		RootSessionID:   "sess-user",
		SpawnDepth:      1,
		SpawnRole:       "worker",
	}

	t.Run("Should return the bounded visible session page", func(t *testing.T) {
		t.Parallel()

		manager := testutil.StubSessionManager{
			ListPageFn: func(_ context.Context, query session.ListQuery) (session.ListPage, error) {
				if query.Limit != 0 {
					t.Fatalf("session page limit = %d, want server default request", query.Limit)
				}
				return session.ListPage{Sessions: []*session.Info{
					{
						ID:          "sess-user",
						Name:        "Operator session",
						AgentName:   "coder",
						Provider:    "codex",
						WorkspaceID: "ws_alpha",
						Type:        session.SessionTypeUser,
						State:       session.StateActive,
						CreatedAt:   createdAt,
						UpdatedAt:   updatedAt,
					},
					{
						ID:          "sess-worker",
						Name:        "Worker session",
						AgentName:   "coder",
						Provider:    "codex",
						WorkspaceID: "ws_alpha",
						Type:        session.SessionTypeSpawned,
						Lineage:     workerLineage,
						State:       session.StateActive,
						CreatedAt:   createdAt,
						UpdatedAt:   updatedAt,
					},
				}, Total: 2, Limit: session.DefaultListLimit}, nil
			},
		}
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, testutil.StubWorkspaceService{}, nil, nil)

		resp := performRequest(t, fixture.Engine, http.MethodGet, "/sessions", nil)
		if resp.Code != http.StatusOK {
			t.Fatalf("list sessions status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
		}
		var payload contract.SessionCatalogResponse
		testutil.DecodeJSONResponse(t, resp, &payload)

		ids := make([]string, 0, len(payload.Sessions))
		for _, sessionPayload := range payload.Sessions {
			ids = append(ids, sessionPayload.ID)
		}
		if got, want := strings.Join(ids, ","), "sess-user,sess-worker"; got != want {
			t.Fatalf("session ids = %q, want %q", got, want)
		}
		if payload.Page.Total != 2 || payload.Page.Limit != session.DefaultListLimit {
			t.Fatalf("session page = %#v, want truthful total/default limit", payload.Page)
		}
	})

	t.Run("Should preserve the page contract for resumable lists", func(t *testing.T) {
		t.Parallel()

		manager := testutil.StubSessionManager{
			ListPageFn: func(_ context.Context, query session.ListQuery) (session.ListPage, error) {
				if !query.Resumable {
					t.Fatalf("resumable query = false, want true")
				}
				if query.AgentName != "coder" {
					t.Fatalf("agent query = %q, want coder", query.AgentName)
				}
				if query.SessionType != session.SessionTypeUser {
					t.Fatalf("type query = %q, want %q", query.SessionType, session.SessionTypeUser)
				}
				return session.ListPage{Sessions: []*session.Info{
					{
						ID:          "sess-user",
						Name:        "Operator session",
						AgentName:   "coder",
						Provider:    "codex",
						WorkspaceID: "ws_alpha",
						Type:        session.SessionTypeUser,
						State:       session.StateActive,
						CreatedAt:   createdAt,
						UpdatedAt:   updatedAt,
					},
					{
						ID:          "sess-worker",
						Name:        "Worker session",
						AgentName:   "coder",
						Provider:    "codex",
						WorkspaceID: "ws_alpha",
						Type:        session.SessionTypeSpawned,
						Lineage:     workerLineage,
						State:       session.StateActive,
						CreatedAt:   createdAt,
						UpdatedAt:   updatedAt,
					},
				}, Total: 2, Limit: session.DefaultListLimit}, nil
			},
		}
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, testutil.StubWorkspaceService{}, nil, nil)

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/sessions?resumable=true&agent=coder&type=user",
			nil,
		)
		if resp.Code != http.StatusOK {
			t.Fatalf(
				"list resumable sessions status = %d, want %d; body=%s",
				resp.Code,
				http.StatusOK,
				resp.Body.String(),
			)
		}
		var payload contract.SessionCatalogResponse
		testutil.DecodeJSONResponse(t, resp, &payload)

		ids := make([]string, 0, len(payload.Sessions))
		for _, sessionPayload := range payload.Sessions {
			ids = append(ids, sessionPayload.ID)
		}
		if got, want := strings.Join(ids, ","), "sess-user,sess-worker"; got != want {
			t.Fatalf("session ids = %q, want %q", got, want)
		}
	})

	t.Run("Should hydrate health only for sessions returned by the page", func(t *testing.T) {
		t.Parallel()

		manager := testutil.StubSessionManager{
			ListPageFn: func(_ context.Context, _ session.ListQuery) (session.ListPage, error) {
				return session.ListPage{
					Sessions: []*session.Info{{
						ID:          "sess-returned",
						AgentName:   "coder",
						WorkspaceID: "ws-alpha",
						Type:        session.SessionTypeUser,
						State:       session.StateActive,
						CreatedAt:   createdAt,
						UpdatedAt:   updatedAt,
					}},
					Total: 100,
					Limit: 1,
				}, nil
			},
		}
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, testutil.StubWorkspaceService{}, nil, nil)
		pageHealthCalls := 0
		singleHealthCalls := 0
		fixture.Handlers.SessionHealth = sessionHealthPageReaderStub{
			getFn: func(context.Context, string) (heartbeat.SessionHealth, error) {
				singleHealthCalls++
				return heartbeat.SessionHealth{}, errors.New("unexpected single health read")
			},
			pageFn: func(
				_ context.Context,
				infos []*session.Info,
			) (map[string]heartbeat.SessionHealth, error) {
				pageHealthCalls++
				if len(infos) != 1 || infos[0].ID != "sess-returned" {
					t.Fatalf("SessionHealthForPage() infos = %#v, want returned page only", infos)
				}
				return map[string]heartbeat.SessionHealth{
					"sess-returned": {
						SessionID:   "sess-returned",
						WorkspaceID: "ws-alpha",
						AgentName:   "coder",
						State:       heartbeat.SessionHealthStateIdle,
						Health:      heartbeat.SessionHealthHealthy,
						Attachable:  true,
						UpdatedAt:   updatedAt,
					},
				}, nil
			},
		}

		resp := performRequest(t, fixture.Engine, http.MethodGet, "/sessions?include_health=true&limit=1", nil)
		if resp.Code != http.StatusOK {
			t.Fatalf("list sessions status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
		}
		var payload contract.SessionCatalogResponse
		testutil.DecodeJSONResponse(t, resp, &payload)
		if pageHealthCalls != 1 || singleHealthCalls != 0 ||
			len(payload.Sessions) != 1 || payload.Sessions[0].Health == nil {
			t.Fatalf(
				"health calls = (page %d, single %d) payload = %#v, want one batched page hydration",
				pageHealthCalls,
				singleHealthCalls,
				payload,
			)
		}
	})
}

func TestBaseHandlersListSessionsErrorBranches(t *testing.T) {
	t.Parallel()

	t.Run("Should list all failure", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{
				ListPageFn: func(context.Context, session.ListQuery) (session.ListPage, error) {
					return session.ListPage{}, errors.New("list failed")
				},
			},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)

		resp := performRequest(t, fixture.Engine, http.MethodGet, "/sessions", nil)
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf(
				"list sessions status = %d, want %d; body=%s",
				resp.Code,
				http.StatusInternalServerError,
				resp.Body.String(),
			)
		}
	})

	t.Run("Should reject an invalid session type before paging", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{
				ListPageFn: func(context.Context, session.ListQuery) (session.ListPage, error) {
					t.Fatal("ListPage() called for invalid session type")
					return session.ListPage{}, nil
				},
			},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)

		resp := performRequest(t, fixture.Engine, http.MethodGet, "/sessions?type=temporary", nil)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf(
				"list sessions status = %d, want %d; body=%s",
				resp.Code,
				http.StatusBadRequest,
				resp.Body.String(),
			)
		}
	})

	t.Run("Should reject the internal dream type before paging", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{
				ListPageFn: func(context.Context, session.ListQuery) (session.ListPage, error) {
					t.Fatal("ListPage() called for internal dream session type")
					return session.ListPage{}, nil
				},
			},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)

		resp := performRequest(t, fixture.Engine, http.MethodGet, "/sessions?type=dream", nil)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf(
				"list sessions status = %d, want %d; body=%s",
				resp.Code,
				http.StatusBadRequest,
				resp.Body.String(),
			)
		}
	})

	t.Run("Should report an unavailable paged catalog", func(t *testing.T) {
		t.Parallel()

		manager := unpagedSessionManager{SessionManager: testutil.StubSessionManager{}}
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			TransportName:      "api-core-test",
			MaskInternalErrors: false,
			Sessions:           manager,
		})
		engine := gin.New()
		engine.GET("/sessions", handlers.ListSessions)
		resp := performRequest(t, engine, http.MethodGet, "/sessions", nil)
		if resp.Code != http.StatusServiceUnavailable {
			t.Fatalf(
				"list sessions status = %d, want %d; body=%s",
				resp.Code,
				http.StatusServiceUnavailable,
				resp.Body.String(),
			)
		}
	})

	t.Run("Should reject per-session health fallback for a catalog page", func(t *testing.T) {
		t.Parallel()

		manager := testutil.StubSessionManager{
			ListPageFn: func(context.Context, session.ListQuery) (session.ListPage, error) {
				return session.ListPage{Sessions: []*session.Info{{ID: "sess-returned"}}, Total: 1, Limit: 1}, nil
			},
		}
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, testutil.StubWorkspaceService{}, nil, nil)
		singleCalls := 0
		fixture.Handlers.SessionHealth = sessionHealthReaderFunc(func(
			context.Context,
			string,
		) (heartbeat.SessionHealth, error) {
			singleCalls++
			return heartbeat.SessionHealth{}, nil
		})
		resp := performRequest(t, fixture.Engine, http.MethodGet, "/sessions?include_health=true", nil)
		if resp.Code != http.StatusServiceUnavailable || singleCalls != 0 {
			t.Fatalf(
				"list sessions status = %d single calls = %d, want 503 with no per-session fallback; body=%s",
				resp.Code,
				singleCalls,
				resp.Body.String(),
			)
		}
	})

	t.Run("Should workspace lookup failure", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{
				ListAllFn: func(context.Context) ([]*session.Info, error) {
					return []*session.Info{{ID: "sess-1", WorkspaceID: "ws_alpha"}}, nil
				},
			},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{
				GetFn: func(context.Context, string) (workspacepkg.Workspace, error) {
					return workspacepkg.Workspace{}, workspacepkg.ErrWorkspaceNotFound
				},
			},
			nil,
			nil,
		)

		resp := performRequest(t, fixture.Engine, http.MethodGet, "/sessions?workspace=alpha", nil)
		if resp.Code != http.StatusNotFound {
			t.Fatalf("list sessions status = %d, want %d; body=%s", resp.Code, http.StatusNotFound, resp.Body.String())
		}
	})

	t.Run("Should preserve missing workspace root status", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{
				GetFn: func(context.Context, string) (workspacepkg.Workspace, error) {
					return workspacepkg.Workspace{}, workspacepkg.ErrWorkspaceRootMissing
				},
			},
			nil,
			nil,
		)
		resp := performRequest(t, fixture.Engine, http.MethodGet, "/sessions?workspace=missing-root", nil)
		if resp.Code != http.StatusGone {
			t.Fatalf("list sessions status = %d, want %d; body=%s", resp.Code, http.StatusGone, resp.Body.String())
		}
	})

	for _, path := range []string{
		"/sessions?resumable=not-a-bool",
		"/sessions?include_health=not-a-bool",
		"/sessions?limit=not-an-int",
	} {
		t.Run("Should reject invalid query "+path, func(t *testing.T) {
			t.Parallel()

			fixture := newHandlerFixture(
				t,
				testutil.StubSessionManager{},
				testutil.StubObserver{},
				testutil.StubWorkspaceService{},
				nil,
				nil,
			)
			resp := performRequest(t, fixture.Engine, http.MethodGet, path, nil)
			if resp.Code != http.StatusBadRequest {
				t.Fatalf(
					"list sessions status = %d, want %d; body=%s",
					resp.Code,
					http.StatusBadRequest,
					resp.Body.String(),
				)
			}
		})
	}
}

func TestBaseHandlersStreamSessionCatalog(t *testing.T) {
	t.Parallel()

	t.Run("Should stream workspace-identified catalog wakes through one global endpoint", func(t *testing.T) {
		t.Parallel()

		events := make(chan session.CatalogEvent, 2)
		events <- session.CatalogEvent{
			Kind:        session.CatalogEventUpserted,
			WorkspaceID: "ws_beta",
			SessionID:   "sess_beta",
		}
		events <- session.CatalogEvent{
			Kind:        session.CatalogEventDeleted,
			WorkspaceID: "ws_alpha",
			SessionID:   "sess_deleted",
		}
		close(events)
		canceled := false
		manager := testutil.StubSessionManager{
			SubscribeCatalogFn: func(
				_ context.Context,
			) (<-chan session.CatalogEvent, func(), error) {
				return events, func() { canceled = true }, nil
			},
		}
		fixture := newHandlerFixture(
			t,
			manager,
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/sessions/catalog-stream",
			nil,
		)
		if resp.Code != http.StatusOK {
			t.Fatalf(
				"session catalog stream status = %d, want %d; body=%s",
				resp.Code,
				http.StatusOK,
				resp.Body.String(),
			)
		}
		if contentType := resp.Header().Get("Content-Type"); contentType != "text/event-stream" {
			t.Fatalf("session catalog stream content type = %q, want text/event-stream", contentType)
		}
		if !canceled {
			t.Fatal("session catalog subscription was not canceled")
		}
		body := resp.Body.String()
		for _, want := range []string{
			"event: session_catalog_changed",
			`"kind":"upserted"`,
			`"workspace_id":"ws_beta"`,
			`"session_id":"sess_beta"`,
			`"kind":"deleted"`,
			`"workspace_id":"ws_alpha"`,
			`"session_id":"sess_deleted"`,
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("session catalog stream body = %q, want %q", body, want)
			}
		}
	})
}

func TestObserveStreamAndParseObserveQuery(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	callCount := 0
	observer := testutil.StubObserver{
		QueryEventsFn: func(_ context.Context, _ store.EventSummaryQuery) ([]store.EventSummary, error) {
			callCount++
			ts := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
			switch callCount {
			case 1:
				return []store.EventSummary{
					{ID: "sum-1", SessionID: "sess-1", Type: "agent_message", AgentName: "coder", Timestamp: ts},
				}, nil
			case 2:
				close(done)
				return []store.EventSummary{
					{
						ID:        "sum-2",
						SessionID: "sess-1",
						Type:      "done",
						AgentName: "coder",
						Timestamp: ts.Add(time.Second),
					},
				}, nil
			default:
				return nil, nil
			}
		},
	}
	fixture := newHandlerFixture(t, testutil.StubSessionManager{}, observer, testutil.StubWorkspaceService{}, nil, nil)
	fixture.Handlers.SetStreamDone(done)

	resp := performRequest(
		t,
		fixture.Engine,
		http.MethodGet,
		"/logs/stream?workspace_id=ws-workspace&agent_name=coder",
		nil,
	)
	if resp.Code != http.StatusOK {
		t.Fatalf("observe stream status = %d, want %d", resp.Code, http.StatusOK)
	}
	if records := testutil.ParseSSE(t, resp.Body.String()); len(records) < 2 {
		t.Fatalf("observe stream records = %d, want at least 2", len(records))
	}
}

func TestBaseHandlersGetAgentNotFound(t *testing.T) {
	t.Parallel()

	fixture := newHandlerFixture(
		t,
		testutil.StubSessionManager{},
		testutil.StubObserver{},
		testutil.StubWorkspaceService{},
		nil,
		nil,
	)
	fixture.Handlers.AgentLoader = func(string, aghconfig.HomePaths) (aghconfig.AgentDef, error) {
		return aghconfig.AgentDef{}, os.ErrNotExist
	}

	resp := performRequest(t, fixture.Engine, http.MethodGet, "/agents/missing", nil)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("get missing agent status = %d, want %d", resp.Code, http.StatusNotFound)
	}
}
