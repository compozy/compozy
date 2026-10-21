package core_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	apitestutil "github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/session"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/worktree"
	"github.com/gin-gonic/gin"
)

func TestBaseHandlersAgentSpawnMapsRequestAndDefaultsAutoStop(t *testing.T) {
	t.Parallel()
	t.Run("Should map spawn ACP options and default auto-stop", func(t *testing.T) {
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
				ID:          "sess-child",
				Name:        opts.Name,
				AgentName:   opts.AgentName,
				Provider:    opts.Provider,
				Speed:       opts.Speed,
				WorkspaceID: "ws-1",
				Workspace:   "/workspace/project",

				Type:  session.SessionTypeSpawned,
				State: session.StateActive,
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
			t.Context(),
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
			t.Context(),
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
	})
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
		t.Context(),
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
	spawnFn       func(context.Context, session.SpawnOpts) (*session.Session, error)
	spawnPromptFn func(context.Context, session.SpawnPromptRequest) (session.SendPromptResult, error)
	spawnWsFn     func(context.Context, string, string) (string, error)
}

func (s *spawnStubSessionManager) ResolveSpawnWorkspace(
	ctx context.Context,
	parentSessionID string,
	workspaceRef string,
) (string, error) {
	if s.spawnWsFn == nil {
		return "ws-1", nil
	}
	return s.spawnWsFn(ctx, parentSessionID, workspaceRef)
}

func (s *spawnStubSessionManager) SendSpawnPrompt(
	ctx context.Context,
	req session.SpawnPromptRequest,
) (session.SendPromptResult, error) {
	if s.spawnPromptFn == nil {
		return session.SendPromptResult{}, errors.New("unexpected SendSpawnPrompt call")
	}
	return s.spawnPromptFn(ctx, req)
}

type spawnWorktreeServiceStub struct {
	core.WorktreeService
	created   worktree.CreateOptions
	workspace string
	removed   string
}

func (s *spawnWorktreeServiceStub) CreateReady(
	_ context.Context,
	workspaceID string,
	opts worktree.CreateOptions,
) (*worktree.Worktree, error) {
	s.created, s.workspace = opts, workspaceID
	return &worktree.Worktree{
		ID: "wt-new", WorkspaceID: workspaceID, Name: opts.Name, Branch: opts.Branch, Path: "/wt/" + opts.Name,
		CreatedHead: "abc123",
	}, nil
}

func (s *spawnWorktreeServiceStub) Remove(
	_ context.Context,
	_ string,
	id string,
	_ bool,
) (*worktree.RemovalRefusal, error) {
	s.removed = id
	return nil, nil
}

func (s *spawnStubSessionManager) Spawn(ctx context.Context, opts session.SpawnOpts) (*session.Session, error) {
	if s.spawnFn != nil {
		return s.spawnFn(ctx, opts)
	}
	return nil, nil
}

func agentSpawnRouter(manager core.SessionManager, worktrees ...core.WorktreeService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	config := &core.BaseHandlerConfig{
		TransportName: "uds-test",
		Sessions:      manager,
		StreamDone:    make(chan struct{}),
		Workspaces: apitestutil.StubWorkspaceService{
			GetFn: func(_ context.Context, ref string) (workspacepkg.Workspace, error) {
				return workspacepkg.Workspace{ID: ref}, nil
			},
		},
	}
	if len(worktrees) > 0 {
		config.Worktrees = worktrees[0]
	}
	handlers := core.NewBaseHandlers(config)
	router.POST("/api/agent/spawn", handlers.AgentSpawn)
	return router
}

func agentSpawnCallerInfo() *session.Info {
	return &session.Info{
		ID:          "sess-parent",
		ProfileID:   store.DefaultProfileID,
		Name:        "parent",
		AgentName:   "coder",
		Provider:    "codex",
		WorkspaceID: "ws-1",
		Workspace:   "/workspace/project",

		Type:  session.SessionTypeUser,
		State: session.StateActive,
		Lineage: &store.SessionLineage{
			RootSessionID: "sess-parent",
			PermissionPolicy: store.SessionPermissionPolicy{
				Tools:  []string{"read"},
				Skills: []string{"go"},
			},
		},
	}
}

func TestBaseHandlersAgentSpawnBindsWorktreeAndSendsFirstPrompt(t *testing.T) {
	t.Parallel()

	newManager := func(spawn func(session.SpawnOpts) (*session.Session, error)) *spawnStubSessionManager {
		manager := &spawnStubSessionManager{}
		manager.StatusFn = func(_ context.Context, id string) (*session.Info, error) {
			if id != "sess-parent" {
				return nil, session.ErrSessionNotFound
			}
			return agentSpawnCallerInfo(), nil
		}
		manager.spawnFn = func(_ context.Context, opts session.SpawnOpts) (*session.Session, error) {
			return spawn(opts)
		}
		return manager
	}
	post := func(t *testing.T, router *gin.Engine, body string) *httptest.ResponseRecorder {
		t.Helper()
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(
			t.Context(), http.MethodPost, "/api/agent/spawn", bytes.NewReader([]byte(body)),
		)
		req.Header.Set(agentidentity.HeaderSessionID, "sess-parent")
		req.Header.Set(agentidentity.HeaderAgent, "coder")
		router.ServeHTTP(rec, req)
		return rec
	}

	t.Run("Should create, bind, and report the worktree and first prompt", func(t *testing.T) {
		t.Parallel()

		var got session.SpawnOpts
		var prompt session.SpawnPromptRequest
		ttl := time.Date(2026, 4, 26, 12, 1, 0, 0, time.UTC)
		manager := newManager(func(opts session.SpawnOpts) (*session.Session, error) {
			got = opts
			return &session.Session{
				ID: "sess-child", AgentName: opts.AgentName, WorkspaceID: "ws-1", WorktreeID: opts.IsolatedWorktreeID,
				Type: session.SessionTypeSpawned, State: session.StateActive,
				Lineage: &store.SessionLineage{
					ParentSessionID: "sess-parent", RootSessionID: "sess-parent", SpawnDepth: 1,
					SpawnRole: "worker", TTLExpiresAt: &ttl,
				},
			}, nil
		})
		manager.spawnPromptFn = func(_ context.Context, req session.SpawnPromptRequest) (session.SendPromptResult, error) {
			prompt = req
			return session.SendPromptResult{
				MessageID: session.SpawnPromptMessageID(req.ChildSessionID), Status: "accepted",
				ReplyWatch: &session.ReplyWatchRef{ID: "rw-1", State: "armed"},
			}, nil
		}
		worktrees := &spawnWorktreeServiceStub{}
		rec := post(t, agentSpawnRouter(manager, worktrees), `{
			"agent_name":"coder","spawn_role":"worker","ttl_seconds":60,
			"inherit_permissions":true,"permissions":{"skills":[]},
			"new_worktree":{"name":"auth","base_ref":"main","branch":"feat/auth"},
			"prompt":"Implement auth."
		}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d body=%s, want 201", rec.Code, rec.Body.String())
		}
		if worktrees.created.Name != "auth" || worktrees.created.BaseRef != "main" ||
			worktrees.created.Branch != "feat/auth" || worktrees.created.ProfileID != store.DefaultProfileID {
			t.Fatalf("CreateReady() opts = %#v", worktrees.created)
		}
		if got.IsolatedWorktreeID != "wt-new" || !got.InheritPermissions || got.PermissionNarrowing == nil ||
			got.PermissionNarrowing.Tools != nil || got.PermissionNarrowing.Skills == nil {
			t.Fatalf("spawn opts = %#v, want bound worktree and nil-preserving narrowing", got)
		}
		if prompt.ParentSessionID != "sess-parent" || prompt.ChildSessionID != "sess-child" ||
			prompt.Message != "Implement auth." {
			t.Fatalf("SendSpawnPrompt() request = %#v", prompt)
		}
		var response contract.AgentSpawnResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if response.Spawn.Worktree == nil || response.Spawn.Worktree.ID != "wt-new" ||
			response.Spawn.Worktree.Branch != "feat/auth" || !response.Spawn.Worktree.Created ||
			response.Spawn.Prompt == nil || response.Spawn.Prompt.ReplyWatchID != "rw-1" {
			t.Fatalf("spawn response = %#v, want worktree and prompt projections", response.Spawn)
		}
	})

	t.Run("Should authorize a foreign workspace before creating its worktree", func(t *testing.T) {
		t.Parallel()

		var got session.SpawnOpts
		ttl := time.Date(2026, 4, 26, 12, 1, 0, 0, time.UTC)
		manager := newManager(func(opts session.SpawnOpts) (*session.Session, error) {
			got = opts
			return &session.Session{
				ID: "sess-child", WorkspaceID: "ws-foreign", Type: session.SessionTypeSpawned,
				State: session.StateActive,
				Lineage: &store.SessionLineage{
					ParentSessionID: "sess-parent", RootSessionID: "sess-parent", SpawnDepth: 1, TTLExpiresAt: &ttl,
				},
			}, nil
		})
		manager.spawnWsFn = func(_ context.Context, _ string, ref string) (string, error) {
			if ref == "ws-denied" {
				return "", fmt.Errorf("%w: child workspace is outside the parent workspace", session.ErrSpawnPermissionDenied)
			}
			return ref, nil
		}
		worktrees := &spawnWorktreeServiceStub{}
		denied := post(t, agentSpawnRouter(manager, worktrees), `{
			"agent_name":"coder","spawn_role":"worker","ttl_seconds":60,
			"workspace":"ws-denied","new_worktree":{"name":"auth"}
		}`)
		if denied.Code != http.StatusForbidden || worktrees.created.Name != "" {
			t.Fatalf("denied status = %d created = %#v, want 403 before any worktree", denied.Code, worktrees.created)
		}
		allowed := post(t, agentSpawnRouter(manager, worktrees), `{
			"agent_name":"coder","spawn_role":"worker","ttl_seconds":60,
			"workspace":"ws-foreign","new_worktree":{"name":"auth"}
		}`)
		if allowed.Code != http.StatusCreated {
			t.Fatalf("allowed status = %d body=%s, want 201", allowed.Code, allowed.Body.String())
		}
		if worktrees.workspace != "ws-foreign" || got.IsolatedWorktreeID != "wt-new" || got.Workspace != "ws-foreign" {
			t.Fatalf("worktree workspace = %q opts = %#v, want worktree created in ws-foreign", worktrees.workspace, got)
		}
	})

	t.Run("Should roll back the created worktree and name the refused field", func(t *testing.T) {
		t.Parallel()

		manager := newManager(func(session.SpawnOpts) (*session.Session, error) {
			return nil, &session.SpawnPermissionError{
				Field: "tools", Value: "shell", Rule: session.SpawnPermissionRuleSubset, Allowed: []string{"read"},
			}
		})
		worktrees := &spawnWorktreeServiceStub{}
		rec := post(t, agentSpawnRouter(manager, worktrees), `{
			"agent_name":"coder","spawn_role":"worker","ttl_seconds":60,
			"permissions":{"tools":["shell"]},"new_worktree":{}
		}`)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d body=%s, want 403", rec.Code, rec.Body.String())
		}
		if worktrees.removed != "wt-new" {
			t.Fatalf("rolled back worktree = %q, want wt-new", worktrees.removed)
		}
		if body := rec.Body.String(); !strings.Contains(body, `tools value \"shell\"`) ||
			!strings.Contains(body, session.SpawnPermissionRuleSubset) {
			t.Fatalf("body = %s, want refused field, value, and rule", body)
		}
	})
}
