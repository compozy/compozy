package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

type workspaceGetterFunc func(context.Context, string) (workspacepkg.Workspace, error)

func (f workspaceGetterFunc) Get(ctx context.Context, ref string) (workspacepkg.Workspace, error) {
	return f(ctx, ref)
}

type workspaceServiceStub struct {
	get workspaceGetterFunc
}

func (s workspaceServiceStub) Register(context.Context, workspacepkg.RegisterOptions) (workspacepkg.Workspace, error) {
	return workspacepkg.Workspace{}, workspacepkg.ErrWorkspaceNotFound
}

func (s workspaceServiceStub) Unregister(context.Context, string) error {
	return workspacepkg.ErrWorkspaceNotFound
}

func (s workspaceServiceStub) Update(context.Context, string, workspacepkg.UpdateOptions) error {
	return workspacepkg.ErrWorkspaceNotFound
}

func (s workspaceServiceStub) List(context.Context) ([]workspacepkg.Workspace, error) {
	return nil, nil
}

func (s workspaceServiceStub) Get(ctx context.Context, ref string) (workspacepkg.Workspace, error) {
	return s.get(ctx, ref)
}

func (s workspaceServiceStub) Resolve(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
	return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
}

func (s workspaceServiceStub) ResolveOrRegister(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
	return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
}

func assertTaskValidationError(t *testing.T, err error, wantSubstring string) {
	t.Helper()
	if !errors.Is(err, taskpkg.ErrValidation) {
		t.Fatalf("error = %v, want task validation error", err)
	}
	if wantSubstring != "" && !strings.Contains(err.Error(), wantSubstring) {
		t.Fatalf("error = %q, want substring %q", err.Error(), wantSubstring)
	}
}

func TestStatusForTaskAgentIdentityErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should map agent identity failures to stable task statuses", func(t *testing.T) {
		t.Parallel()

		if got := StatusForTaskError(agentidentity.ErrIdentityRequired); got != http.StatusUnauthorized {
			t.Fatalf("StatusForTaskError(identity required) = %d, want %d", got, http.StatusUnauthorized)
		}
		if got := StatusForTaskError(agentidentity.ErrIdentityLookupUnavailable); got != http.StatusServiceUnavailable {
			t.Fatalf(
				"StatusForTaskError(identity lookup unavailable) = %d, want %d",
				got,
				http.StatusServiceUnavailable,
			)
		}
		if got := StatusForTaskError(agentidentity.ErrIdentityUnauthorized); got != http.StatusForbidden {
			t.Fatalf("StatusForTaskError(identity unauthorized) = %d, want %d", got, http.StatusForbidden)
		}
	})
}

func TestTaskActorContextAndTransportHelpers(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	testCases := []struct {
		name      string
		transport string
		wantKind  taskpkg.OriginKind
	}{
		{name: "uds", transport: "uds-api", wantKind: taskpkg.OriginKindUDS},
		{name: "web", transport: "web-ui", wantKind: taskpkg.OriginKindWeb},
		{name: "cli", transport: "agh-cli", wantKind: taskpkg.OriginKindCLI},
		{name: "default", transport: "api-core-test", wantKind: taskpkg.OriginKindHTTP},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			handlers := &BaseHandlers{TransportName: tc.transport}

			actor, err := handlers.taskActorContext(ctx, taskActionGet)
			if err != nil {
				t.Fatalf("taskActorContext() error = %v", err)
			}
			if actor.Actor.Ref != defaultTaskActorRef || actor.Origin.Kind != tc.wantKind ||
				actor.Origin.Ref != "tasks.get" {
				t.Fatalf("taskActorContext() = %#v", actor)
			}
		})
	}

	t.Run("Should custom resolver", func(t *testing.T) {
		t.Parallel()

		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		handlers := &BaseHandlers{
			TaskActorContextResolver: func(_ *gin.Context, action string) (taskpkg.ActorContext, error) {
				return taskpkg.DeriveHumanActorContext("user-2", taskpkg.OriginKindHTTP, "custom."+action)
			},
		}

		actor, err := handlers.taskActorContext(ctx, taskActionList)
		if err != nil {
			t.Fatalf("taskActorContext(custom) error = %v", err)
		}
		if actor.Actor.Ref != "user-2" || actor.Origin.Ref != "custom.list" {
			t.Fatalf("taskActorContext(custom) = %#v", actor)
		}
	})

	t.Run("Should use validated agent identity headers", func(t *testing.T) {
		t.Parallel()

		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/tasks", http.NoBody)
		ctx.Request.Header.Set(agentidentity.HeaderSessionID, "sess-agent")
		ctx.Request.Header.Set(agentidentity.HeaderAgent, "coder")
		handlers := &BaseHandlers{
			TransportName: "uds-api",
			Sessions: sessionManagerStub{status: func(_ context.Context, id string) (*session.Info, error) {
				if id != "sess-agent" {
					t.Fatalf("session id = %q, want sess-agent", id)
				}
				return &session.Info{
					ID:          "sess-agent",
					AgentName:   "coder",
					WorkspaceID: "ws-1",
					State:       session.StateActive,
				}, nil
			}},
		}

		actor, err := handlers.taskActorContextForWorkspace(ctx, taskActionCreate, "ws-1")
		if err != nil {
			t.Fatalf("taskActorContextForWorkspace(agent) error = %v", err)
		}
		if actor.Actor.Kind != taskpkg.ActorKindAgentSession ||
			actor.Actor.Ref != "sess-agent" ||
			actor.Origin.Kind != taskpkg.OriginKindUDS ||
			actor.Origin.Ref != "tasks.create" {
			t.Fatalf("taskActorContextForWorkspace(agent) = %#v", actor)
		}
	})
}

func TestTaskParsingAndValidationHelpers(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	handlers := &BaseHandlers{
		TransportName: "api-core-test",
		Workspaces: workspaceServiceStub{get: func(_ context.Context, ref string) (workspacepkg.Workspace, error) {
			if ref != "alpha" {
				t.Fatalf("workspace ref = %q, want %q", ref, "alpha")
			}
			return workspacepkg.Workspace{ID: "ws-alpha"}, nil
		}},
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/tasks?scope=workspace&workspace=alpha&status=ready&owner_kind=pool&owner_ref=reviewers&parent_task_id=task-root&participation_channel=builders&limit=3",
		http.NoBody,
	)

	transportQuery, err := ParseTaskListQuery(ctx)
	if err != nil {
		t.Fatalf("ParseTaskListQuery() error = %v", err)
	}
	query, err := handlers.taskListDomainQuery(context.Background(), transportQuery)
	if err != nil {
		t.Fatalf("taskListDomainQuery() error = %v", err)
	}
	if query.WorkspaceID != "ws-alpha" || query.Status != taskpkg.TaskStatusReady ||
		query.OwnerKind != taskpkg.OwnerKindPool {
		t.Fatalf("taskListDomainQuery() = %#v", query)
	}

	runRecorder := httptest.NewRecorder()
	runCtx, _ := gin.CreateTestContext(runRecorder)
	runCtx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/tasks/task-1/runs?status=running&session_id=sess-1&participation_channel=builders&limit=1",
		http.NoBody,
	)

	runQuery, err := parseTaskRunListQuery(runCtx)
	if err != nil {
		t.Fatalf("parseTaskRunListQuery() error = %v", err)
	}
	if runQuery.Status != taskpkg.TaskRunStatusRunning ||
		runQuery.SessionID != "sess-1" ||
		runQuery.ParticipationChannel != "builders" ||
		runQuery.Limit != 1 {
		t.Fatalf("parseTaskRunListQuery() = %#v", runQuery)
	}

	if _, err := addTaskDependencyFromRequest(
		"task-1",
		contract.AddTaskDependencyRequest{DependsOnTaskID: "task-2"},
	); err != nil {
		t.Fatalf("addTaskDependencyFromRequest() error = %v", err)
	}
	if _, err := startTaskRunFromRequest(contract.StartTaskRunRequest{IdempotencyKey: "start-1"}); err != nil {
		t.Fatalf("startTaskRunFromRequest() error = %v", err)
	}
	if _, err := completeTaskRunFromRequest(
		contract.CompleteTaskRunRequest{Result: json.RawMessage(`{"ok":true}`)},
	); err != nil {
		t.Fatalf("completeTaskRunFromRequest() error = %v", err)
	}
	if _, err := cancelTaskRunFromRequest(
		contract.CancelTaskRunRequest{Reason: "stop", Metadata: json.RawMessage(`{"source":"test"}`)},
	); err != nil {
		t.Fatalf("cancelTaskRunFromRequest() error = %v", err)
	}
	if _, err := cancelTaskFromRequest(
		contract.CancelTaskRequest{Reason: "stop", Metadata: json.RawMessage(`{"source":"test"}`)},
	); err != nil {
		t.Fatalf("cancelTaskFromRequest() error = %v", err)
	}

	if _, err := attachTaskRunSessionIDFromRequest(contract.AttachTaskRunSessionRequest{}); err == nil {
		t.Fatal("attachTaskRunSessionIDFromRequest() error = nil, want non-nil")
	} else {
		assertTaskValidationError(t, err, "session_id is required")
	}
	if _, err := failTaskRunFromRequest(contract.FailTaskRunRequest{}); err == nil {
		t.Fatal("failTaskRunFromRequest() error = nil, want non-nil")
	} else {
		assertTaskValidationError(t, err, "run_failure.error is required")
	}
	if err := validateParticipationChannel("task.participation_channel", "bad.channel"); err == nil {
		t.Fatal("validateParticipationChannel(invalid) error = nil, want non-nil")
	} else {
		assertTaskValidationError(t, err, `task.participation_channel: network: invalid field: channel="bad.channel"`)
	}
	if _, err := enqueueTaskRunFromRequest(
		"task-1",
		contract.EnqueueTaskRunRequest{},
	); err != nil {
		t.Fatalf("enqueueTaskRunFromRequest(empty) error = %v", err)
	}
	if _, err := requiredPathID("", "task id"); err == nil {
		t.Fatal("requiredPathID(empty) error = nil, want non-nil")
	} else {
		assertTaskValidationError(t, err, "task id is required")
	}

	invalidRecorder := httptest.NewRecorder()
	invalidCtx, _ := gin.CreateTestContext(invalidRecorder)
	invalidCtx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/tasks?limit=bad",
		http.NoBody,
	)
	if _, err := ParseTaskListQuery(invalidCtx); err == nil {
		t.Fatal("ParseTaskListQuery(invalid limit) error = nil, want non-nil")
	} else {
		assertTaskValidationError(t, err, `invalid integer "bad"`)
	}

	t.Run("Should reject a bounded query before workspace lookup", func(t *testing.T) {
		t.Parallel()

		boundedRecorder := httptest.NewRecorder()
		boundedCtx, _ := gin.CreateTestContext(boundedRecorder)
		boundedCtx.Request = httptest.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			"/tasks?scope=workspace&workspace=missing&limit=-1",
			http.NoBody,
		)
		if _, err := ParseTaskListQuery(boundedCtx); err == nil {
			t.Fatal("ParseTaskListQuery(invalid bounded query) error = nil, want non-nil")
		} else {
			assertTaskValidationError(t, err, "task_query.limit")
		}
	})

	invalidRunRecorder := httptest.NewRecorder()
	invalidRunCtx, _ := gin.CreateTestContext(invalidRunRecorder)
	invalidRunCtx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/tasks/task-1/runs?limit=bad",
		http.NoBody,
	)
	if _, err := parseTaskRunListQuery(invalidRunCtx); err == nil {
		t.Fatal("parseTaskRunListQuery(invalid limit) error = nil, want non-nil")
	} else {
		assertTaskValidationError(t, err, `invalid integer "bad"`)
	}

	invalidChannelRecorder := httptest.NewRecorder()
	invalidChannelCtx, _ := gin.CreateTestContext(invalidChannelRecorder)
	invalidChannelCtx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/tasks/task-1/runs?participation_channel=invalid%20channel",
		http.NoBody,
	)
	if _, err := parseTaskRunListQuery(invalidChannelCtx); err == nil {
		t.Fatal("parseTaskRunListQuery(invalid participation channel) error = nil, want non-nil")
	} else {
		assertTaskValidationError(t, err, "task_run_query.participation_channel")
	}

	decodeRecorder := httptest.NewRecorder()
	decodeCtx, _ := gin.CreateTestContext(decodeRecorder)
	decodeCtx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/tasks",
		bytes.NewBufferString(`{"broken":`),
	)
	decodeCtx.Request.Header.Set("Content-Type", "application/json")
	var payload contract.CancelTaskRequest
	if err := decodeOptionalJSON(decodeCtx, &payload); err == nil {
		t.Fatal("decodeOptionalJSON(invalid) error = nil, want non-nil")
	} else if !strings.Contains(err.Error(), "unexpected EOF") {
		t.Fatalf("decodeOptionalJSON(invalid) error = %q, want unexpected EOF", err.Error())
	}

	unknownRecorder := httptest.NewRecorder()
	unknownCtx, _ := gin.CreateTestContext(unknownRecorder)
	unknownCtx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/tasks/task-1/runs",
		bytes.NewBufferString(`{"network_channel":"builders"}`),
	)
	unknownCtx.Request.Header.Set("Content-Type", "application/json")
	var enqueuePayload contract.EnqueueTaskRunRequest
	if err := decodeOptionalJSON(unknownCtx, &enqueuePayload); err == nil {
		t.Fatal("decodeOptionalJSON(unknown field) error = nil, want non-nil")
	} else if !errors.Is(err, ErrUnknownJSONField) || !strings.Contains(err.Error(), "network_channel") {
		t.Fatalf("decodeOptionalJSON(unknown field) error = %q, want unknown_field with field named", err.Error())
	}
}

func TestTaskHandlerInfrastructureHelpers(t *testing.T) {
	t.Parallel()

	t.Run("Should expose nil-safe payload and required service helpers", func(t *testing.T) {
		t.Parallel()

		gin.SetMode(gin.TestMode)

		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		handlers := &BaseHandlers{TransportName: "api-core-test"}

		service, ok := handlers.requireTaskManager(ctx)
		if ok || service != nil {
			t.Fatalf("requireTaskManager() = (%v, %v), want (nil, false)", service, ok)
		}

		if !reflect.DeepEqual(TaskPayloadFromTask(nil), contract.TaskPayload{}) {
			t.Fatal("TaskPayloadFromTask(nil) should return zero payload")
		}
		if !reflect.DeepEqual(TaskRunPayloadFromRun(nil), contract.TaskRunPayload{}) {
			t.Fatal("TaskRunPayloadFromRun(nil) should return zero payload")
		}
		if !reflect.DeepEqual(TaskDetailPayloadFromView(nil), contract.TaskDetailPayload{}) {
			t.Fatal("TaskDetailPayloadFromView(nil) should return zero payload")
		}

		idOnly := json.RawMessage("{\"ok\":true}")
		ptr := cloneRawMessagePtr(&idOnly)
		if ptr == nil || string(*ptr) != "{\"ok\":true}" {
			t.Fatalf("cloneRawMessagePtr() = %v", ptr)
		}
	})
}

func TestTaskRunPayloadFromRunExposesLeaseStateWithoutRawClaimToken(t *testing.T) {
	t.Run("Should not expose raw claim tokens and should expose lease state", func(t *testing.T) {
		t.Parallel()

		claimedAt := time.Date(2026, 4, 26, 10, 0, 0, 0, time.UTC)
		run := taskpkg.Run{
			ID:                 "run-lease",
			TaskID:             "task-lease",
			Status:             taskpkg.TaskRunStatusRunning,
			Attempt:            1,
			RecoveryCount:      2,
			ClaimedBy:          &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "scheduler"},
			SessionID:          "sess-lease",
			Origin:             taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "scheduler"},
			ClaimTokenHash:     "sha256:" + strings.Repeat("c", 64),
			LeaseUntil:         claimedAt.Add(15 * time.Minute),
			HeartbeatAt:        claimedAt.Add(time.Minute),
			RunNetworkState:    &taskpkg.RunNetworkState{NetworkSpec: coreTestLiveParticipation("ws-1", "coord-lease")},
			QueuedAt:           claimedAt.Add(-time.Minute),
			ClaimedAt:          claimedAt,
			StartedAt:          claimedAt.Add(time.Minute),
			DesignationGroupID: "designation-group",
			Error:              "provider rejected agh_claim_error-secret",
			Metadata: json.RawMessage(
				`{"keep":"metadata","claim_token":"raw-secret-token","nested":{"claim_token":"nested-secret"},` +
					`"designation":{"index":1,"brief":"Review with agh_claim_designation-secret"}}`,
			),
			Result: json.RawMessage(`[{"ok":true},{"claim_token":"result-secret"}]`),
		}

		payload := TaskRunPayloadFromRun(&run)
		if payload.ClaimTokenHash != run.ClaimTokenHash {
			t.Fatalf("ClaimTokenHash = %q, want %q", payload.ClaimTokenHash, run.ClaimTokenHash)
		}
		if payload.LeaseUntil == nil || !payload.LeaseUntil.Equal(run.LeaseUntil) {
			t.Fatalf("LeaseUntil = %v, want %v", payload.LeaseUntil, run.LeaseUntil)
		}
		if payload.HeartbeatAt == nil || !payload.HeartbeatAt.Equal(run.HeartbeatAt) {
			t.Fatalf("HeartbeatAt = %v, want %v", payload.HeartbeatAt, run.HeartbeatAt)
		}
		if payload.RecoveryCount != int(run.RecoveryCount) {
			t.Fatalf("RecoveryCount = %d, want %d", payload.RecoveryCount, run.RecoveryCount)
		}
		if payload.ResolvedNetworkParticipation == nil ||
			payload.ResolvedNetworkParticipation.ChannelID != run.NetworkSpecSnapshot().ChannelID {
			t.Fatalf(
				"ResolvedNetworkParticipation = %#v, want channel %q",
				payload.ResolvedNetworkParticipation,
				run.NetworkSpecSnapshot().ChannelID,
			)
		}

		content, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal(TaskRunPayload) error = %v", err)
		}
		encoded := string(content)
		if strings.Contains(encoded, `"claim_token"`) {
			t.Fatalf("TaskRunPayload JSON exposed raw claim_token field: %s", encoded)
		}
		for _, rawValue := range []string{
			"raw-secret-token",
			"nested-secret",
			"result-secret",
			"agh_claim_error-secret",
			"agh_claim_designation-secret",
		} {
			if strings.Contains(encoded, rawValue) {
				t.Fatalf("TaskRunPayload JSON exposed raw token value %q: %s", rawValue, encoded)
			}
		}
		if payload.Designation == nil || payload.Designation.Index != 1 ||
			strings.Contains(payload.Designation.Brief, "agh_claim_designation-secret") ||
			!strings.Contains(payload.Designation.Brief, "agh_claim_[REDACTED]") {
			t.Fatalf("TaskRunPayload designation = %#v, want redacted index-1 projection", payload.Designation)
		}
		if !strings.Contains(encoded, `"claim_token_hash"`) || !strings.Contains(encoded, run.ClaimTokenHash) {
			t.Fatalf("TaskRunPayload JSON = %s, want claim_token_hash", encoded)
		}
		if !strings.Contains(encoded, `"keep":"metadata"`) {
			t.Fatalf("TaskRunPayload JSON = %s, want non-sensitive metadata preserved", encoded)
		}
	})
}

func TestTaskExecutionRequestFromRequestValidatesDomainRequest(t *testing.T) {
	t.Parallel()

	t.Run("Should reject oversized execution metadata", func(t *testing.T) {
		t.Parallel()

		oversizedMetadata := json.RawMessage(`"` + strings.Repeat("x", taskpkg.MaxMetadataBytes) + `"`)
		_, err := taskExecutionRequestFromRequest(contract.TaskExecutionRequest{Metadata: oversizedMetadata})
		if !errors.Is(err, taskpkg.ErrPayloadTooLarge) {
			t.Fatalf("taskExecutionRequestFromRequest() error = %v, want %v", err, taskpkg.ErrPayloadTooLarge)
		}
	})
}
