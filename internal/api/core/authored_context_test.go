package core_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/api/testutil"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/soul"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

type soulIfMatchTestAuthoring struct {
	putCalls      int
	deleteCalls   int
	rollbackCalls int
}

func (s *soulIfMatchTestAuthoring) Validate(context.Context, soul.ValidateRequest) (soul.ValidateResult, error) {
	return soul.ValidateResult{}, nil
}

func (s *soulIfMatchTestAuthoring) Put(context.Context, soul.PutRequest) (soul.MutationResult, error) {
	s.putCalls++
	return soul.MutationResult{}, nil
}

func (s *soulIfMatchTestAuthoring) Delete(context.Context, soul.DeleteRequest) (soul.MutationResult, error) {
	s.deleteCalls++
	return soul.MutationResult{}, nil
}

func (s *soulIfMatchTestAuthoring) History(context.Context, soul.HistoryRequest) (soul.HistoryResult, error) {
	return soul.HistoryResult{}, nil
}

func (s *soulIfMatchTestAuthoring) Rollback(context.Context, soul.RollbackRequest) (soul.MutationResult, error) {
	s.rollbackCalls++
	return soul.MutationResult{}, nil
}

type soulIfMatchTestRefresher struct {
	calls int
}

func (s *soulIfMatchTestRefresher) RefreshSoulWithExpectedDigest(
	context.Context,
	string,
	string,
) (session.SoulRefreshResult, error) {
	s.calls++
	return session.SoulRefreshResult{}, nil
}

type packageOwnedHeartbeatAuthoring struct {
	putCalls      int
	deleteCalls   int
	rollbackCalls int
}

func (h *packageOwnedHeartbeatAuthoring) Validate(
	context.Context,
	heartbeat.ValidateRequest,
) (heartbeat.ValidateResult, error) {
	return heartbeat.ValidateResult{}, nil
}

func (h *packageOwnedHeartbeatAuthoring) Put(
	context.Context,
	heartbeat.PutRequest,
) (heartbeat.MutationResult, error) {
	h.putCalls++
	return heartbeat.MutationResult{}, nil
}

func (h *packageOwnedHeartbeatAuthoring) Delete(
	context.Context,
	heartbeat.DeleteRequest,
) (heartbeat.MutationResult, error) {
	h.deleteCalls++
	return heartbeat.MutationResult{}, nil
}

func (h *packageOwnedHeartbeatAuthoring) History(
	context.Context,
	heartbeat.HistoryRequest,
) (heartbeat.HistoryResult, error) {
	return heartbeat.HistoryResult{}, nil
}

func (h *packageOwnedHeartbeatAuthoring) Rollback(
	context.Context,
	heartbeat.RollbackRequest,
) (heartbeat.MutationResult, error) {
	h.rollbackCalls++
	return heartbeat.MutationResult{}, nil
}

type packageOwnedAgentCatalog struct {
	artifacts session.AgentArtifacts
}

func (c packageOwnedAgentCatalog) ListAgents(context.Context) ([]core.AgentCatalogEntry, error) {
	return []core.AgentCatalogEntry{{
		Def:    aghconfig.CloneAgentDef(c.artifacts.Agent),
		Origin: contract.AgentOriginGlobal,
	}}, nil
}

func (c packageOwnedAgentCatalog) GetAgent(context.Context, string) (core.AgentCatalogEntry, error) {
	return core.AgentCatalogEntry{
		Def:    aghconfig.CloneAgentDef(c.artifacts.Agent),
		Origin: contract.AgentOriginGlobal,
	}, nil
}

func (c packageOwnedAgentCatalog) ResolveAgentArtifacts(
	string,
	*workspacepkg.ResolvedWorkspace,
) (session.AgentArtifacts, error) {
	return c.artifacts, nil
}

type heartbeatStatusSpy struct {
	calls int
	last  heartbeat.StatusRequest
	err   error
}

func (s *heartbeatStatusSpy) Inspect(context.Context, heartbeat.InspectRequest) (heartbeat.InspectResult, error) {
	return heartbeat.InspectResult{}, nil
}

func (s *heartbeatStatusSpy) Status(
	_ context.Context,
	req heartbeat.StatusRequest,
) (heartbeat.StatusResult, error) {
	s.calls++
	s.last = req
	if s.err != nil {
		return heartbeat.StatusResult{}, s.err
	}
	return heartbeat.StatusResult{
		AgentName: req.Target.AgentName,
		Enabled:   true,
		Present:   true,
		Active:    true,
		Valid:     true,
	}, nil
}

type sessionHealthReaderStub struct {
	health heartbeat.SessionHealth
	err    error
}

func (s sessionHealthReaderStub) GetSessionHealth(
	_ context.Context,
	_ string,
) (heartbeat.SessionHealth, error) {
	return s.health, s.err
}

type heartbeatWakeSpy struct {
	calls int
	last  heartbeat.WakeRequest
}

func (s *heartbeatWakeSpy) Wake(
	_ context.Context,
	req heartbeat.WakeRequest,
) (heartbeat.WakeDecision, error) {
	s.calls++
	s.last = req
	return heartbeat.WakeDecision{
		Result: heartbeat.WakeResultSkipped,
		Reason: heartbeat.WakeReasonHeartbeatNoEligible,
	}, nil
}

type workspaceIDCaptureSoulAuthoring struct {
	putCalls int
	last     soul.PutRequest
}

func (s *workspaceIDCaptureSoulAuthoring) Validate(context.Context, soul.ValidateRequest) (soul.ValidateResult, error) {
	return soul.ValidateResult{}, nil
}

func (s *workspaceIDCaptureSoulAuthoring) Put(
	_ context.Context,
	req soul.PutRequest,
) (soul.MutationResult, error) {
	s.putCalls++
	s.last = req
	return soul.MutationResult{}, errors.New("captured soul put request")
}

func (s *workspaceIDCaptureSoulAuthoring) Delete(context.Context, soul.DeleteRequest) (soul.MutationResult, error) {
	return soul.MutationResult{}, nil
}

func (s *workspaceIDCaptureSoulAuthoring) History(context.Context, soul.HistoryRequest) (soul.HistoryResult, error) {
	return soul.HistoryResult{}, nil
}

func (s *workspaceIDCaptureSoulAuthoring) Rollback(context.Context, soul.RollbackRequest) (soul.MutationResult, error) {
	return soul.MutationResult{}, nil
}

func TestAuthoredContextUsesRegistryWorkspaceIDForStorageBackedOperations(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	agentDir := filepath.Join(workspaceRoot, aghconfig.DirName, aghconfig.AgentsDirName, "coder")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(agent dir) error = %v", err)
	}
	agentBody := []byte("---\nname: coder\nprovider: claude\n---\nReview startup launch work.\n")
	if err := os.WriteFile(filepath.Join(agentDir, "AGENT.md"), agentBody, 0o644); err != nil {
		t.Fatalf("WriteFile(AGENT.md) error = %v", err)
	}

	workspaces := testutil.StubWorkspaceService{
		ResolveFn: func(ctx context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
			if err := ctx.Err(); err != nil {
				return workspacepkg.ResolvedWorkspace{}, err
			}
			if strings.TrimSpace(ref) != "ws-stable" {
				return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
			}
			return workspacepkg.ResolvedWorkspace{
				Workspace:   workspacepkg.Workspace{ID: "ws-registry", RootDir: workspaceRoot, Name: "Ad8 QA"},
				WorkspaceID: "ws-stable",
				Config: aghconfig.Config{
					Agents: aghconfig.AgentsConfig{
						Soul:      aghconfig.DefaultSoulConfig(),
						Heartbeat: aghconfig.DefaultHeartbeatConfig(),
					},
				},
			}, nil
		},
	}
	fixture := newHandlerFixture(t, testutil.StubSessionManager{}, testutil.StubObserver{}, workspaces, nil, nil)
	soulAuthoring := &workspaceIDCaptureSoulAuthoring{}
	statusSpy := &heartbeatStatusSpy{}
	wakeSpy := &heartbeatWakeSpy{}
	fixture.Handlers.SoulAuthoring = soulAuthoring
	fixture.Handlers.HeartbeatStatus = statusSpy
	fixture.Handlers.HeartbeatWake = wakeSpy
	fixture.Engine.PUT("/agents/:name/soul", fixture.Handlers.PutAgentSoul)
	fixture.Engine.GET("/agents/:name/heartbeat/status", fixture.Handlers.GetAgentHeartbeatStatus)
	fixture.Engine.POST("/agents/:name/heartbeat/wake", fixture.Handlers.WakeAgentHeartbeat)

	t.Run("Should pass registry workspace id to Soul authoring", func(t *testing.T) {
		body := []byte("{\"workspace_id\":\"ws-stable\",\"agent_name\":\"coder\",\"body\":\"# Soul\"}")
		req := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodPut,
			"/agents/coder/soul",
			bytes.NewReader(body),
		)
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		fixture.Engine.ServeHTTP(recorder, req)

		if soulAuthoring.putCalls != 1 {
			t.Fatalf("soul put calls = %d, want 1", soulAuthoring.putCalls)
		}
		if got, want := soulAuthoring.last.Target.WorkspaceID, "ws-registry"; got != want {
			t.Fatalf("Soul target WorkspaceID = %q, want %q", got, want)
		}
	})

	t.Run("Should pass registry workspace id to Heartbeat status", func(t *testing.T) {
		req := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			"/agents/coder/heartbeat/status?workspace_id=ws-stable",
			nil,
		)
		recorder := httptest.NewRecorder()
		fixture.Engine.ServeHTTP(recorder, req)

		if statusSpy.calls != 1 {
			t.Fatalf("heartbeat status calls = %d, want 1", statusSpy.calls)
		}
		if got, want := statusSpy.last.Target.WorkspaceID, "ws-registry"; got != want {
			t.Fatalf("Heartbeat status target WorkspaceID = %q, want %q", got, want)
		}
	})

	t.Run("Should pass registry workspace id to Heartbeat wake", func(t *testing.T) {
		body := []byte(
			"{\"workspace_id\":\"ws-stable\",\"agent_name\":\"coder\",\"source\":\"manual\",\"dry_run\":true}",
		)
		req := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			"/agents/coder/heartbeat/wake",
			bytes.NewReader(body),
		)
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		fixture.Engine.ServeHTTP(recorder, req)

		if wakeSpy.calls != 1 {
			t.Fatalf("heartbeat wake calls = %d, want 1", wakeSpy.calls)
		}
		if got, want := wakeSpy.last.WorkspaceID, "ws-registry"; got != want {
			t.Fatalf("Heartbeat wake WorkspaceID = %q, want %q", got, want)
		}
	})

	t.Run("Should use the stable workspace id for session-gated heartbeat fallback checks", func(t *testing.T) {
		manager := testutil.StubSessionManager{
			StatusFn: func(ctx context.Context, id string) (*session.Info, error) {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				if id != "sess-owned" {
					t.Fatalf("Status() session id = %q, want sess-owned", id)
				}
				return &session.Info{ID: id, WorkspaceID: "ws-stable", AgentName: "coder"}, nil
			},
		}
		workspacesWithStableOnly := testutil.StubWorkspaceService{
			ResolveFn: func(ctx context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
				if err := ctx.Err(); err != nil {
					return workspacepkg.ResolvedWorkspace{}, err
				}
				if strings.TrimSpace(ref) != "ws-stable" {
					return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
				}
				return workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{
						RootDir: workspaceRoot,
						Name:    "Ad8 QA",
					},
					WorkspaceID: "ws-stable",
					Config: aghconfig.Config{
						Agents: aghconfig.AgentsConfig{Heartbeat: aghconfig.DefaultHeartbeatConfig()},
					},
				}, nil
			},
		}
		stableFixture := newHandlerFixture(t, manager, testutil.StubObserver{}, workspacesWithStableOnly, nil, nil)
		stableStatusSpy := &heartbeatStatusSpy{}
		stableWakeSpy := &heartbeatWakeSpy{}
		stableFixture.Handlers.HeartbeatStatus = stableStatusSpy
		stableFixture.Handlers.HeartbeatWake = stableWakeSpy
		stableFixture.Engine.GET("/agents/:name/heartbeat/status", stableFixture.Handlers.GetAgentHeartbeatStatus)
		stableFixture.Engine.POST("/agents/:name/heartbeat/wake", stableFixture.Handlers.WakeAgentHeartbeat)

		statusReq := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			"/agents/coder/heartbeat/status?workspace_id=ws-stable&session_id=sess-owned",
			nil,
		)
		statusRecorder := httptest.NewRecorder()
		stableFixture.Engine.ServeHTTP(statusRecorder, statusReq)
		if got, want := statusRecorder.Code, http.StatusOK; got != want {
			t.Fatalf("heartbeat status code = %d, want %d body=%s", got, want, statusRecorder.Body.String())
		}
		if stableStatusSpy.calls != 1 {
			t.Fatalf("heartbeat status calls = %d, want 1", stableStatusSpy.calls)
		}
		if got, want := stableStatusSpy.last.Target.WorkspaceID, "ws-stable"; got != want {
			t.Fatalf("heartbeat status target WorkspaceID = %q, want %q", got, want)
		}

		wakeBody := []byte(
			"{\"workspace_id\":\"ws-stable\",\"agent_name\":\"coder\",\"session_id\":\"sess-owned\",\"source\":\"manual\",\"dry_run\":true}",
		)
		wakeReq := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			"/agents/coder/heartbeat/wake",
			bytes.NewReader(wakeBody),
		)
		wakeReq.Header.Set("Content-Type", "application/json")
		wakeRecorder := httptest.NewRecorder()
		stableFixture.Engine.ServeHTTP(wakeRecorder, wakeReq)
		if got, want := wakeRecorder.Code, http.StatusConflict; got != want {
			t.Fatalf("heartbeat wake code = %d, want %d body=%s", got, want, wakeRecorder.Body.String())
		}
		if stableWakeSpy.calls != 1 {
			t.Fatalf("heartbeat wake calls = %d, want 1", stableWakeSpy.calls)
		}
		if got, want := stableWakeSpy.last.WorkspaceID, "ws-stable"; got != want {
			t.Fatalf("heartbeat wake WorkspaceID = %q, want %q", got, want)
		}
	})
}

func TestSessionReadsSurviveAgentDefinitionDeletion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		path          string
		heartbeatErr  error
		wantStatus    int
		registerRoute func(handlerFixture)
		assertPayload func(*testing.T, map[string]any)
	}{
		{
			name:         "Should return status without Heartbeat enrichment after agent deletion",
			path:         "/workspaces/ws-stable/sessions/sess-deleted-agent/status",
			heartbeatErr: heartbeat.ErrAuthoringAgentNotFound,
			wantStatus:   http.StatusOK,
			registerRoute: func(fixture handlerFixture) {
				fixture.Engine.GET(
					"/workspaces/:workspace_id/sessions/:session_id/status",
					fixture.Handlers.GetSessionStatus,
				)
			},
			assertPayload: func(t *testing.T, payload map[string]any) {
				t.Helper()
				if got, want := payload["session_id"], "sess-deleted-agent"; got != want {
					t.Fatalf("session_id = %#v, want %q", got, want)
				}
				if got, want := payload["agent_name"], "deleted-agent"; got != want {
					t.Fatalf("agent_name = %#v, want %q", got, want)
				}
				if _, ok := payload["wake_state"]; ok {
					t.Fatalf("wake_state = %#v, want omitted", payload["wake_state"])
				}
			},
		},
		{
			name:         "Should return inspection without Heartbeat enrichment after agent deletion",
			path:         "/workspaces/ws-stable/sessions/sess-deleted-agent/inspect",
			heartbeatErr: heartbeat.ErrAuthoringAgentNotFound,
			wantStatus:   http.StatusOK,
			registerRoute: func(fixture handlerFixture) {
				fixture.Engine.GET(
					"/workspaces/:workspace_id/sessions/:session_id/inspect",
					fixture.Handlers.InspectSession,
				)
			},
			assertPayload: func(t *testing.T, payload map[string]any) {
				t.Helper()
				health, ok := payload["health"].(map[string]any)
				if !ok {
					t.Fatalf("health = %#v, want object", payload["health"])
				}
				if got, want := health["session_id"], "sess-deleted-agent"; got != want {
					t.Fatalf("health.session_id = %#v, want %q", got, want)
				}
				if got, want := health["agent_name"], "deleted-agent"; got != want {
					t.Fatalf("health.agent_name = %#v, want %q", got, want)
				}
				if _, ok := payload["wake_state"]; ok {
					t.Fatalf("wake_state = %#v, want omitted", payload["wake_state"])
				}
			},
		},
		{
			name:         "Should preserve unrelated Heartbeat status failures",
			path:         "/workspaces/ws-stable/sessions/sess-deleted-agent/status",
			heartbeatErr: errors.New("heartbeat status unavailable"),
			wantStatus:   http.StatusInternalServerError,
			registerRoute: func(fixture handlerFixture) {
				fixture.Engine.GET(
					"/workspaces/:workspace_id/sessions/:session_id/status",
					fixture.Handlers.GetSessionStatus,
				)
			},
			assertPayload: func(t *testing.T, payload map[string]any) {
				t.Helper()
				if got, want := payload["error"], "read heartbeat status: heartbeat status unavailable"; got != want {
					t.Fatalf("error = %#v, want %q", got, want)
				}
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			workspaceRoot := t.TempDir()
			manager := testutil.StubSessionManager{
				StatusFn: func(ctx context.Context, id string) (*session.Info, error) {
					if err := ctx.Err(); err != nil {
						return nil, err
					}
					return &session.Info{
						ID:          id,
						WorkspaceID: "ws-registry",
						AgentName:   "deleted-agent",
					}, nil
				},
			}
			workspaces := testutil.StubWorkspaceService{
				ResolveFn: func(ctx context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
					if err := ctx.Err(); err != nil {
						return workspacepkg.ResolvedWorkspace{}, err
					}
					if ref := strings.TrimSpace(ref); ref != "ws-stable" && ref != "ws-registry" {
						return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
					}
					return workspacepkg.ResolvedWorkspace{
						Workspace: workspacepkg.Workspace{
							ID:      "ws-registry",
							RootDir: workspaceRoot,
							Name:    "Deleted agent session",
						},
						WorkspaceID: "ws-stable",
						Config: aghconfig.Config{
							Agents: aghconfig.AgentsConfig{Heartbeat: aghconfig.DefaultHeartbeatConfig()},
						},
					}, nil
				},
			}
			fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, workspaces, nil, nil)
			fixture.Handlers.SessionHealth = sessionHealthReaderStub{health: heartbeat.SessionHealth{
				SessionID:       "sess-deleted-agent",
				WorkspaceID:     "ws-registry",
				AgentName:       "deleted-agent",
				State:           heartbeat.SessionHealthStateIdle,
				Health:          heartbeat.SessionHealthHealthy,
				Attachable:      true,
				EligibleForWake: false,
				UpdatedAt:       time.Date(2026, 7, 11, 19, 30, 0, 0, time.UTC),
			}}
			statusSpy := &heartbeatStatusSpy{err: testCase.heartbeatErr}
			fixture.Handlers.HeartbeatStatus = statusSpy
			testCase.registerRoute(fixture)

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, testCase.path, nil)
			recorder := httptest.NewRecorder()
			fixture.Engine.ServeHTTP(recorder, req)

			if got, want := recorder.Code, testCase.wantStatus; got != want {
				t.Fatalf("response code = %d, want %d body=%s", got, want, recorder.Body.String())
			}
			var payload map[string]any
			if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
				t.Fatalf("Unmarshal(response) error = %v", err)
			}
			testCase.assertPayload(t, payload)
			if got, want := statusSpy.calls, 1; got != want {
				t.Fatalf("HeartbeatStatus.Status() calls = %d, want %d", got, want)
			}
		})
	}
}

func TestAuthoredContextHeartbeatStatusAndWakeRejectForeignSessionWorkspace(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	manager := testutil.StubSessionManager{
		StatusFn: func(ctx context.Context, id string) (*session.Info, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return &session.Info{ID: strings.TrimSpace(id), WorkspaceID: "ws-owned", AgentName: "coder"}, nil
		},
	}
	workspaces := testutil.StubWorkspaceService{
		ResolveFn: func(ctx context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
			if err := ctx.Err(); err != nil {
				return workspacepkg.ResolvedWorkspace{}, err
			}
			workspaceID := strings.TrimSpace(ref)
			if workspaceID == "" {
				return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
			}
			return workspacepkg.ResolvedWorkspace{
				Workspace:   workspacepkg.Workspace{ID: workspaceID, RootDir: workspaceRoot, Name: workspaceID},
				WorkspaceID: workspaceID,
				Config: aghconfig.Config{
					Agents: aghconfig.AgentsConfig{Heartbeat: aghconfig.DefaultHeartbeatConfig()},
				},
			}, nil
		},
	}
	fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, workspaces, nil, nil)
	statusSpy := &heartbeatStatusSpy{}
	wakeSpy := &heartbeatWakeSpy{}
	fixture.Handlers.HeartbeatStatus = statusSpy
	fixture.Handlers.HeartbeatWake = wakeSpy
	fixture.Engine.GET("/agents/:name/heartbeat/status", fixture.Handlers.GetAgentHeartbeatStatus)
	fixture.Engine.POST("/agents/:name/heartbeat/wake", fixture.Handlers.WakeAgentHeartbeat)

	t.Run("Should reject foreign workspace heartbeat status session", func(t *testing.T) {
		req := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			"/agents/coder/heartbeat/status?workspace_id=ws-foreign&session_id=sess-owned&include_session_health=true",
			nil,
		)
		recorder := httptest.NewRecorder()
		fixture.Engine.ServeHTTP(recorder, req)

		if got, want := recorder.Code, http.StatusNotFound; got != want {
			t.Fatalf("heartbeat status code = %d, want %d body=%s", got, want, recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), `"error":"api: workspace-scoped resource not found"`) {
			t.Fatalf("heartbeat status body = %s, want workspace-scoped resource error payload", recorder.Body.String())
		}
		if statusSpy.calls != 0 {
			t.Fatalf("heartbeat status calls = %d, want 0 before ownership validation", statusSpy.calls)
		}
	})

	t.Run("Should reject foreign workspace heartbeat wake session", func(t *testing.T) {
		body := []byte(
			"{\"workspace_id\":\"ws-foreign\",\"agent_name\":\"coder\",\"session_id\":\"sess-owned\",\"source\":\"manual\"}",
		)
		req := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			"/agents/coder/heartbeat/wake",
			bytes.NewReader(body),
		)
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		fixture.Engine.ServeHTTP(recorder, req)

		if got, want := recorder.Code, http.StatusNotFound; got != want {
			t.Fatalf("heartbeat wake code = %d, want %d body=%s", got, want, recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), `"error":"api: workspace-scoped resource not found"`) {
			t.Fatalf("heartbeat wake body = %s, want workspace-scoped resource error payload", recorder.Body.String())
		}
		if wakeSpy.calls != 0 {
			t.Fatalf("heartbeat wake calls = %d, want 0 before ownership validation", wakeSpy.calls)
		}
	})
}

func TestAuthoredContextRejectsPackageOwnedSidecarMutations(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		method        string
		path          string
		body          []byte
		registerRoute func(handlerFixture)
		assertCalls   func(*testing.T, *soulIfMatchTestAuthoring, *packageOwnedHeartbeatAuthoring)
	}{
		{
			name:   "Should reject package-owned Soul writes",
			method: http.MethodPut,
			path:   "/agents/marketer/soul",
			body:   []byte(`{"workspace_id":"ws-1","body":"new soul"}`),
			registerRoute: func(fixture handlerFixture) {
				fixture.Engine.PUT("/agents/:name/soul", fixture.Handlers.PutAgentSoul)
			},
			assertCalls: func(t *testing.T, soulAuthoring *soulIfMatchTestAuthoring, _ *packageOwnedHeartbeatAuthoring) {
				t.Helper()
				if soulAuthoring.putCalls != 0 {
					t.Fatalf("soul put calls = %d, want 0", soulAuthoring.putCalls)
				}
			},
		},
		{
			name:   "Should reject package-owned Soul deletes",
			method: http.MethodDelete,
			path:   "/agents/marketer/soul",
			body:   []byte(`{"workspace_id":"ws-1"}`),
			registerRoute: func(fixture handlerFixture) {
				fixture.Engine.DELETE("/agents/:name/soul", fixture.Handlers.DeleteAgentSoul)
			},
			assertCalls: func(t *testing.T, soulAuthoring *soulIfMatchTestAuthoring, _ *packageOwnedHeartbeatAuthoring) {
				t.Helper()
				if soulAuthoring.deleteCalls != 0 {
					t.Fatalf("soul delete calls = %d, want 0", soulAuthoring.deleteCalls)
				}
			},
		},
		{
			name:   "Should reject package-owned Heartbeat writes",
			method: http.MethodPut,
			path:   "/agents/marketer/heartbeat",
			body:   []byte(`{"workspace_id":"ws-1","body":"new heartbeat"}`),
			registerRoute: func(fixture handlerFixture) {
				fixture.Engine.PUT("/agents/:name/heartbeat", fixture.Handlers.PutAgentHeartbeat)
			},
			assertCalls: func(t *testing.T, _ *soulIfMatchTestAuthoring, heartbeatAuthoring *packageOwnedHeartbeatAuthoring) {
				t.Helper()
				if heartbeatAuthoring.putCalls != 0 {
					t.Fatalf("heartbeat put calls = %d, want 0", heartbeatAuthoring.putCalls)
				}
			},
		},
		{
			name:   "Should reject package-owned Heartbeat rollback",
			method: http.MethodPost,
			path:   "/agents/marketer/heartbeat/rollback",
			body:   []byte(`{"workspace_id":"ws-1","revision_id":"rev-hb-1"}`),
			registerRoute: func(fixture handlerFixture) {
				fixture.Engine.POST("/agents/:name/heartbeat/rollback", fixture.Handlers.RollbackAgentHeartbeat)
			},
			assertCalls: func(t *testing.T, _ *soulIfMatchTestAuthoring, heartbeatAuthoring *packageOwnedHeartbeatAuthoring) {
				t.Helper()
				if heartbeatAuthoring.rollbackCalls != 0 {
					t.Fatalf("heartbeat rollback calls = %d, want 0", heartbeatAuthoring.rollbackCalls)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			workspaceRoot := t.TempDir()
			soulAuthoring := &soulIfMatchTestAuthoring{}
			heartbeatAuthoring := &packageOwnedHeartbeatAuthoring{}
			fixture := newHandlerFixture(
				t,
				testutil.StubSessionManager{},
				testutil.StubObserver{},
				testutil.StubWorkspaceService{
					ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
						if ref != "ws-1" {
							return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
						}
						return workspacepkg.ResolvedWorkspace{
							Workspace: workspacepkg.Workspace{ID: "ws-1", RootDir: workspaceRoot},
							Config: aghconfig.Config{
								Agents: aghconfig.AgentsConfig{
									Soul:      aghconfig.DefaultSoulConfig(),
									Heartbeat: aghconfig.DefaultHeartbeatConfig(),
								},
							},
						}, nil
					},
				},
				nil,
				nil,
			)
			fixture.Handlers.SoulAuthoring = soulAuthoring
			fixture.Handlers.HeartbeatAuthoring = heartbeatAuthoring
			fixture.Handlers.AgentCatalog = packageOwnedAgentCatalog{
				artifacts: session.AgentArtifacts{
					Agent:               aghconfig.AgentDef{Name: "marketer", Prompt: "Run marketing workflows."},
					PackageOwned:        true,
					SoulSourcePath:      ".agh/bundles/act/agents/marketer/SOUL.md",
					SoulBody:            "Lead with campaign context.",
					HeartbeatSourcePath: ".agh/bundles/act/agents/marketer/HEARTBEAT.md",
					HeartbeatBody:       "Inspect campaigns and use AGH task APIs.",
				},
			}
			tc.registerRoute(fixture)

			req := httptest.NewRequestWithContext(context.Background(), tc.method, tc.path, bytes.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			fixture.Engine.ServeHTTP(recorder, req)

			if got, want := recorder.Code, http.StatusConflict; got != want {
				t.Fatalf(
					"%s %s status = %d, want %d body=%s",
					tc.method,
					tc.path,
					got,
					want,
					recorder.Body.String(),
				)
			}
			var payload contract.ErrorPayload
			if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
				t.Fatalf("json.Unmarshal(error payload) error = %v", err)
			}
			if !strings.Contains(payload.Error, "package-owned") {
				t.Fatalf("payload.Error = %q, want package-owned context", payload.Error)
			}
			tc.assertCalls(t, soulAuthoring, heartbeatAuthoring)
		})
	}
}

func TestSoulHandlersRejectIfMatchHeader(t *testing.T) {
	testCases := []struct {
		name          string
		method        string
		path          string
		body          []byte
		registerRoute func(fixture handlerFixture)
		wantError     string
		assertCalls   func(t *testing.T, authoring *soulIfMatchTestAuthoring, refresher *soulIfMatchTestRefresher)
	}{
		{
			name:   "Should reject If-Match on soul write",
			method: http.MethodPut,
			path:   "/agents/coder/soul",
			body:   []byte(`{"body":"# Soul"}`),
			registerRoute: func(fixture handlerFixture) {
				fixture.Engine.PUT("/agents/:name/soul", fixture.Handlers.PutAgentSoul)
			},
			wantError: "authored context validation error: soul_if_match_header_unsupported: use expected_digest in request body",
			assertCalls: func(t *testing.T, authoring *soulIfMatchTestAuthoring, refresher *soulIfMatchTestRefresher) {
				t.Helper()
				if authoring.putCalls != 0 || refresher.calls != 0 {
					t.Fatalf("write calls = put:%d refresh:%d, want zero", authoring.putCalls, refresher.calls)
				}
			},
		},
		{
			name:   "Should reject If-Match on soul delete",
			method: http.MethodDelete,
			path:   "/agents/coder/soul",
			body:   []byte(`{}`),
			registerRoute: func(fixture handlerFixture) {
				fixture.Engine.DELETE("/agents/:name/soul", fixture.Handlers.DeleteAgentSoul)
			},
			wantError: "authored context validation error: soul_if_match_header_unsupported: use expected_digest in request body",
			assertCalls: func(t *testing.T, authoring *soulIfMatchTestAuthoring, refresher *soulIfMatchTestRefresher) {
				t.Helper()
				if authoring.deleteCalls != 0 || refresher.calls != 0 {
					t.Fatalf("delete calls = delete:%d refresh:%d, want zero", authoring.deleteCalls, refresher.calls)
				}
			},
		},
		{
			name:   "Should reject If-Match on soul rollback",
			method: http.MethodPost,
			path:   "/agents/coder/soul/rollback",
			body:   []byte(`{"revision_id":"rev_1"}`),
			registerRoute: func(fixture handlerFixture) {
				fixture.Engine.POST("/agents/:name/soul/rollback", fixture.Handlers.RollbackAgentSoul)
			},
			wantError: "authored context validation error: soul_if_match_header_unsupported: use expected_digest in request body",
			assertCalls: func(t *testing.T, authoring *soulIfMatchTestAuthoring, refresher *soulIfMatchTestRefresher) {
				t.Helper()
				if authoring.rollbackCalls != 0 || refresher.calls != 0 {
					t.Fatalf(
						"rollback calls = rollback:%d refresh:%d, want zero",
						authoring.rollbackCalls,
						refresher.calls,
					)
				}
			},
		},
		{
			name:   "Should reject If-Match on session soul refresh",
			method: http.MethodPost,
			path:   "/workspaces/ws-workspace/sessions/sess_1/soul/refresh",
			body:   []byte(`{"expected_digest":"sha256:old"}`),
			registerRoute: func(fixture handlerFixture) {
				fixture.Engine.POST(
					"/workspaces/ws-workspace/sessions/:id/soul/refresh",
					fixture.Handlers.RefreshSessionSoul,
				)
			},
			wantError: "authored context validation error: soul_if_match_header_unsupported: use expected_digest in request body",
			assertCalls: func(t *testing.T, authoring *soulIfMatchTestAuthoring, refresher *soulIfMatchTestRefresher) {
				t.Helper()
				if authoring.putCalls != 0 || authoring.deleteCalls != 0 || authoring.rollbackCalls != 0 ||
					refresher.calls != 0 {
					t.Fatalf(
						"refresh calls = put:%d delete:%d rollback:%d refresh:%d, want zero",
						authoring.putCalls,
						authoring.deleteCalls,
						authoring.rollbackCalls,
						refresher.calls,
					)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			authoring := &soulIfMatchTestAuthoring{}
			refresher := &soulIfMatchTestRefresher{}
			fixture := newHandlerFixture(
				t,
				testutil.StubSessionManager{},
				testutil.StubObserver{},
				testutil.StubWorkspaceService{},
				nil,
				nil,
			)
			fixture.Handlers.SoulAuthoring = authoring
			fixture.Handlers.SoulRefresher = refresher
			tc.registerRoute(fixture)

			req := httptest.NewRequestWithContext(
				context.Background(),
				tc.method,
				tc.path,
				bytes.NewReader(tc.body),
			)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("If-Match", `"sha256:stale"`)
			recorder := httptest.NewRecorder()
			fixture.Engine.ServeHTTP(recorder, req)

			if got, want := recorder.Code, http.StatusBadRequest; got != want {
				t.Fatalf("%s %s status = %d, want %d body=%s", tc.method, tc.path, got, want, recorder.Body.String())
			}

			var payload contract.ErrorPayload
			if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
				t.Fatalf("json.Unmarshal(error payload) error = %v; body=%s", err, recorder.Body.String())
			}
			if got, want := payload.Error, tc.wantError; got != want {
				t.Fatalf("payload.Error = %q, want %q", got, want)
			}

			tc.assertCalls(t, authoring, refresher)
		})
	}
}
