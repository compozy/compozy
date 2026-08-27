package core_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	apitestutil "github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/session"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

func TestBaseHandlersAgentSpawnMapsRequestAndDefaultsAutoStop(t *testing.T) {
	t.Parallel()

	var got session.SpawnOpts
	manager := &spawnStubSessionManager{}
	manager.StatusFn = func(_ context.Context, id string) (*session.Info, error) {
		if id != "sess-parent" {
			return nil, session.ErrSessionNotFound
		}
		return agentSpawnCallerInfo(), nil
	}
	manager.spawnFn = func(_ context.Context, opts session.SpawnOpts) (*session.Session, error) {
		got = opts
		notifyCreator := opts.NotifyCreator
		if !opts.NotifyCreatorSet {
			notifyCreator = true
		}
		ttl := time.Date(2026, 4, 26, 12, 1, 0, 0, time.UTC)
		return &session.Session{
			ID:                   "sess-child",
			Name:                 opts.Name,
			AgentName:            opts.AgentName,
			Provider:             opts.Provider,
			Speed:                opts.Speed,
			WorkspaceID:          "ws-1",
			Workspace:            "/workspace/project",
			NetworkParticipation: testLiveParticipation("ws-1", "builders"),
			Type:                 session.SessionTypeSpawned,
			State:                session.StateActive,
			Lineage: &store.SessionLineage{
				ParentSessionID:  opts.ParentSessionID,
				RootSessionID:    "sess-parent",
				SpawnDepth:       1,
				SpawnRole:        opts.SpawnRole,
				TTLExpiresAt:     &ttl,
				AutoStopOnParent: opts.AutoStopOnParent,
				NotifyCreator:    notifyCreator,
				SpawnBudget:      store.SessionSpawnBudget{MaxChildren: 5, MaxDepth: 1, TTLSeconds: 60},
				PermissionPolicy: opts.PermissionPolicy,
			},
		}, nil
	}

	router := agentSpawnRouter(manager)
	body := []byte(`{
		"agent_name":"coder",
		"provider":"codex",
		"model":"gpt-test",
		"reasoning_effort":" high ",
		"speed":"fast",
		"acp_options":[{"id":"context","value_id":"1m"},{"id":"thinking","bool_value":true}],
		"name":"child",
		"workspace":"ws-target",
		"prompt_overlay":"focus",
		"spawn_role":"worker",
		"ttl_seconds":60,
		"permissions":{"tools":["read"],"skills":["go"]},
		"idempotency_key":"spawn-1"
	}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/api/agent/spawn",
		bytes.NewReader(body),
	)
	req.Header.Set(agentidentity.HeaderSessionID, "sess-parent")
	req.Header.Set(agentidentity.HeaderAgent, "coder")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s, want 201", rec.Code, rec.Body.String())
	}
	if got.ParentSessionID != "sess-parent" ||
		got.AgentName != "coder" ||
		got.Provider != "codex" ||
		got.Model != "gpt-test" ||
		got.ReasoningEffort != "high" ||
		got.Speed != speedpkg.SpeedFast ||
		len(got.ACPOptions) != 2 ||
		got.Name != "child" ||
		got.Workspace != "ws-target" ||
		got.PromptOverlay != "focus" ||
		got.SpawnRole != "worker" ||
		got.TTL != time.Minute ||
		!got.AutoStopOnParent ||
		got.NotifyCreator ||
		got.NotifyCreatorSet ||
		got.IdempotencyKey != "spawn-1" {
		t.Fatalf("spawn opts = %#v, want mapped request with auto_stop default", got)
	}
	if got.ACPOptions[0].ID != "context" || got.ACPOptions[0].ValueID != "1m" ||
		got.ACPOptions[1].ID != "thinking" || got.ACPOptions[1].BoolValue == nil ||
		!*got.ACPOptions[1].BoolValue {
		t.Fatalf("spawn ACP options = %#v, want typed select and boolean", got.ACPOptions)
	}
	if len(got.PermissionPolicy.Tools) != 1 || got.PermissionPolicy.Tools[0] != "read" {
		t.Fatalf("permission policy = %#v, want narrowed read tool", got.PermissionPolicy)
	}

	var response contract.AgentSpawnResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal(response) error = %v", err)
	}
	if response.Spawn.Session.ID != "sess-child" ||
		response.Spawn.Lineage.ParentSessionID != "sess-parent" ||
		!response.Spawn.Lineage.NotifyCreator ||
		len(response.Spawn.Permissions.Tools) != 1 ||
		response.Spawn.Permissions.Tools[0] != "read" {
		t.Fatalf("spawn response = %#v, want child projection with permissions", response.Spawn)
	}

	body = []byte(`{
		"agent_name":"coder",
		"spawn_role":"worker",
		"ttl_seconds":60,
		"notify_creator":false,
		"permissions":{"tools":["read"],"skills":["go"]}
	}`)
	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/api/agent/spawn",
		bytes.NewReader(body),
	)
	req.Header.Set(agentidentity.HeaderSessionID, "sess-parent")
	req.Header.Set(agentidentity.HeaderAgent, "coder")
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("explicit opt-out status = %d body=%s, want 201", rec.Code, rec.Body.String())
	}
	if got.NotifyCreator || !got.NotifyCreatorSet {
		t.Fatalf("spawn opts = %#v, want explicit notify_creator=false", got)
	}
}

func TestBaseHandlersAgentSpawnStrictDecodeRejectsUnknownPermissionCategory(t *testing.T) {
	t.Parallel()

	spawnCalls := 0
	manager := &spawnStubSessionManager{}
	manager.StatusFn = func(_ context.Context, id string) (*session.Info, error) {
		if id != "sess-parent" {
			return nil, session.ErrSessionNotFound
		}
		return agentSpawnCallerInfo(), nil
	}
	manager.spawnFn = func(context.Context, session.SpawnOpts) (*session.Session, error) {
		spawnCalls++
		return nil, nil
	}

	router := agentSpawnRouter(manager)
	body := []byte(`{
		"agent_name":"coder",
		"spawn_role":"worker",
		"ttl_seconds":60,
		"permissions":{"filesystem":["/tmp"]}
	}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/api/agent/spawn",
		bytes.NewReader(body),
	)
	req.Header.Set(agentidentity.HeaderSessionID, "sess-parent")
	req.Header.Set(agentidentity.HeaderAgent, "coder")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d body=%s, want 422", rec.Code, rec.Body.String())
	}
	if spawnCalls != 0 {
		t.Fatalf("spawnCalls = %d, want 0 for decode failure", spawnCalls)
	}
}

type spawnStubSessionManager struct {
	apitestutil.StubSessionManager
	spawnFn func(context.Context, session.SpawnOpts) (*session.Session, error)
}

func (s *spawnStubSessionManager) Spawn(ctx context.Context, opts session.SpawnOpts) (*session.Session, error) {
	if s.spawnFn != nil {
		return s.spawnFn(ctx, opts)
	}
	return nil, nil
}

func agentSpawnRouter(manager core.SessionManager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
		TransportName: "uds-test",
		Sessions:      manager,
		StreamDone:    make(chan struct{}),
	})
	router.POST("/api/agent/spawn", handlers.AgentSpawn)
	return router
}

func agentSpawnCallerInfo() *session.Info {
	return &session.Info{
		ID:                   "sess-parent",
		ProfileID:            store.DefaultProfileID,
		Name:                 "parent",
		AgentName:            "coder",
		Provider:             "codex",
		WorkspaceID:          "ws-1",
		Workspace:            "/workspace/project",
		NetworkParticipation: testLiveParticipation("ws-1", "builders"),
		Type:                 session.SessionTypeUser,
		State:                session.StateActive,
		Lineage: &store.SessionLineage{
			RootSessionID: "sess-parent",
			PermissionPolicy: store.SessionPermissionPolicy{
				Tools:  []string{"read"},
				Skills: []string{"go"},
			},
		},
	}
}
