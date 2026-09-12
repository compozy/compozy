package core_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/testutil"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/soul"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
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
	artifacts   session.AgentArtifacts
	profileName string
}

func (c *packageOwnedAgentCatalog) ListAgents(context.Context) ([]core.AgentCatalogEntry, error) {
	return []core.AgentCatalogEntry{{
		Def:    compozyconfig.CloneAgentDef(c.artifacts.Agent),
		Origin: contract.AgentOriginGlobal,
	}}, nil
}

func (c *packageOwnedAgentCatalog) ListAgentsForWorkspace(
	ctx context.Context,
	_ *workspacepkg.ResolvedWorkspace,
) ([]core.AgentCatalogEntry, error) {
	return c.ListAgents(ctx)
}

func (c *packageOwnedAgentCatalog) GetAgent(context.Context, string) (core.AgentCatalogEntry, error) {
	return core.AgentCatalogEntry{
		Def:    compozyconfig.CloneAgentDef(c.artifacts.Agent),
		Origin: contract.AgentOriginGlobal,
	}, nil
}

func (c *packageOwnedAgentCatalog) ResolveAgentArtifacts(
	_ string,
	workspace *workspacepkg.ResolvedWorkspace,
) (session.AgentArtifacts, error) {
	if c.profileName != "" && workspace.ProfileName != c.profileName {
		return session.AgentArtifacts{}, fmt.Errorf("catalog profile %q, want %q", workspace.ProfileName, c.profileName)
	}
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
	agentDir := filepath.Join(workspaceRoot, compozyconfig.DirName, compozyconfig.AgentsDirName, "coder")
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
				Config: compozyconfig.Config{
					Agents: compozyconfig.AgentsConfig{
						Soul:      compozyconfig.DefaultSoulConfig(),
						Heartbeat: compozyconfig.DefaultHeartbeatConfig(),
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
				return &session.Info{
					ID:          id,
					WorkspaceID: "ws-stable",
					ProfileID:   store.DefaultProfileID,
					AgentName:   "coder",
				}, nil
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
					Config: compozyconfig.Config{
						Agents: compozyconfig.AgentsConfig{Heartbeat: compozyconfig.DefaultHeartbeatConfig()},
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
		name                   string
		stopVerificationFailed bool
		path                   string
		heartbeatErr           error
		wantStatus             int
		registerRoute          func(handlerFixture)
		assertPayload          func(*testing.T, map[string]any)
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
				if got, want := payload["badge"], string(session.BadgeWaitingForInput); got != want {
					t.Fatalf("badge = %#v, want %q", got, want)
				}
				if _, ok := payload["wake_state"]; ok {
					t.Fatalf("wake_state = %#v, want omitted", payload["wake_state"])
				}
			},
		},
		{
			name:                   "Should expose unverified stop attention without terminal success",
			path:                   "/workspaces/ws-stable/sessions/sess-deleted-agent/status",
			heartbeatErr:           heartbeat.ErrAuthoringAgentNotFound,
			stopVerificationFailed: true,
			wantStatus:             http.StatusOK,
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
				if got, want := payload["badge"], string(session.BadgeNeedsAttention); got != want {
					t.Fatalf("badge = %#v, want %q", got, want)
				}
				if payload["lifecycle_state"] != string(session.StateActive) || payload["verified"] != false ||
					payload["escalated"] != true ||
					payload["attention"] != session.StopVerificationFailedCode {
					t.Fatalf("stop attention = %#v", payload)
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
						ID:                     id,
						WorkspaceID:            "ws-registry",
						ProfileID:              store.DefaultProfileID,
						AgentName:              "deleted-agent",
						State:                  session.StateActive,
						PendingClarifyCount:    1,
						StopVerificationFailed: testCase.stopVerificationFailed,
						StopEscalated:          testCase.stopVerificationFailed,
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
						Config: compozyconfig.Config{
							Agents: compozyconfig.AgentsConfig{Heartbeat: compozyconfig.DefaultHeartbeatConfig()},
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
			return &session.Info{
				ID:          strings.TrimSpace(id),
				WorkspaceID: "ws-owned",
				ProfileID:   store.DefaultProfileID,
				AgentName:   "coder",
			}, nil
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
				Config: compozyconfig.Config{
					Agents: compozyconfig.AgentsConfig{Heartbeat: compozyconfig.DefaultHeartbeatConfig()},
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
			name:          "Should read package-owned Soul in the selected profile",
			method:        http.MethodGet,
			path:          "/agents/marketer/soul",
			registerRoute: func(fixture handlerFixture) { fixture.Engine.GET("/agents/:name/soul", fixture.Handlers.GetAgentSoul) },
		},
		{
			name:   "Should read package-owned Heartbeat in the selected profile",
			method: http.MethodGet,
			path:   "/agents/marketer/heartbeat",
			registerRoute: func(fixture handlerFixture) {
				fixture.Engine.GET("/agents/:name/heartbeat", fixture.Handlers.GetAgentHeartbeat)
			},
		},
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
					ResolveForProfileFn: func(_ context.Context, ref, profile string) (workspacepkg.ResolvedWorkspace, error) {
						if profile != "marketing" {
							t.Fatalf("workspace profile=%q, want marketing", profile)
						}
						if ref != "ws-1" {
							return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
						}
						return workspacepkg.ResolvedWorkspace{
							Workspace:   workspacepkg.Workspace{ID: "ws-1", RootDir: workspaceRoot},
							ProfileName: profile,
							Config: compozyconfig.Config{
								Agents: compozyconfig.AgentsConfig{
									Soul:      compozyconfig.DefaultSoulConfig(),
									Heartbeat: compozyconfig.DefaultHeartbeatConfig(),
								},
							},
						}, nil
					},
				},
				nil,
				nil,
			)
			fixture.Handlers.Profiles = sessionProfileServiceStub{}
			fixture.Handlers.SoulAuthoring = soulAuthoring
			fixture.Handlers.HeartbeatAuthoring = heartbeatAuthoring
			fixture.Handlers.AgentCatalog = &packageOwnedAgentCatalog{
				profileName: "marketing",
				artifacts: session.AgentArtifacts{
					Agent:               compozyconfig.AgentDef{Name: "marketer", Prompt: "Run marketing workflows."},
					PackageOwned:        true,
					SoulSourcePath:      ".compozy/bundles/act/agents/marketer/SOUL.md",
					SoulBody:            "---\nversion: \"1\"\nrole: marketer\n---\nLead with campaign context.",
					HeartbeatSourcePath: ".compozy/bundles/act/agents/marketer/HEARTBEAT.md",
					HeartbeatBody:       "---\nversion: \"1\"\nenabled: true\nsummary: Campaign context\n---\nInspect campaigns and use Compozy task APIs.",
				},
			}
			tc.registerRoute(fixture)

			req := httptest.NewRequestWithContext(
				context.Background(),
				tc.method,
				tc.path+"?workspace_id=ws-1&profile=marketing",
				bytes.NewReader(tc.body),
			)
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			fixture.Engine.ServeHTTP(recorder, req)

			if tc.method == http.MethodGet {
				if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"present":true`) ||
					!strings.Contains(recorder.Body.String(), "campaign") {
					t.Fatalf("package read: status=%d body=%s", recorder.Code, recorder.Body.String())
				}
				return
			}
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

type emptyHeartbeatStatusStore struct{}

// FindHeartbeatSnapshotByDigest models a Profile with no stored Heartbeat snapshots.
func (emptyHeartbeatStatusStore) FindHeartbeatSnapshotByDigest(
	context.Context, string, string, string,
) (heartbeat.Snapshot, bool, error) {
	return heartbeat.Snapshot{}, false, nil
}

// GetHeartbeatWakeState models a missing wake state in the Profile fixture.
func (emptyHeartbeatStatusStore) GetHeartbeatWakeState(
	context.Context, string, string, string,
) (heartbeat.WakeState, error) {
	return heartbeat.WakeState{}, heartbeat.ErrWakeStateNotFound
}

// ListHeartbeatWakeState models an empty wake-state catalog in the Profile fixture.
func (emptyHeartbeatStatusStore) ListHeartbeatWakeState(
	context.Context, heartbeat.WakeStateListQuery,
) ([]heartbeat.WakeState, error) {
	return nil, nil
}

// TestAuthoredContextResolvesProfileAgentSources verifies authored reads select the correct Profile for agent resources and Heartbeat policy.
func TestAuthoredContextResolvesProfileAgentSources(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	home := testutil.NewTestHomePaths(t)
	cfg := compozyconfig.DefaultWithHome(home)
	digests := make(map[string]string)
	for _, name := range []string{"default", "marketing"} {
		dir := filepath.Join(root, ".compozy", "profiles", name, "agents", "coder")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(
			filepath.Join(dir, "AGENT.md"),
			[]byte("---\nname: coder\nprovider: codex\n---\nMaintain notes.\n"),
			0o644,
		); err != nil {
			t.Fatal(err)
		}
		body := "---\nversion: 1\nenabled: true\n---\nCheck " + name + " notes.\n"
		path := filepath.Join(dir, "HEARTBEAT.md")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		policy, err := heartbeat.Parse(t.Context(), heartbeat.ParseRequest{
			SourcePath: path, WorkspaceRoot: root, Content: []byte(body), Config: cfg.Agents.Heartbeat,
		})
		if err != nil {
			t.Fatal(err)
		}
		digests[name] = policy.Digest
	}
	resolve := func(_ context.Context, ref, profileName string) (workspacepkg.ResolvedWorkspace, error) {
		if ref != "ws-profile" {
			return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
		}
		agents, err := compozyconfig.LoadWorkspaceAgentDefs(root, nil, home, profileName)
		return workspacepkg.ResolvedWorkspace{
			Workspace: workspacepkg.Workspace{ID: ref, RootDir: root}, WorkspaceID: ref,
			ProfileName: profileName, Config: cfg, Agents: agents,
		}, err
	}
	workspaces := testutil.StubWorkspaceService{
		ResolveFn: func(ctx context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
			return resolve(ctx, ref, "")
		},
		ResolveForProfileFn: resolve,
	}
	healthReader := sessionHealthReaderStub{health: heartbeat.SessionHealth{
		SessionID: "sess-profile", WorkspaceID: "ws-profile", AgentName: "coder",
		State: heartbeat.SessionHealthStateIdle, Health: heartbeat.SessionHealthHealthy,
		UpdatedAt: time.Date(2026, 9, 10, 22, 40, 0, 0, time.UTC),
	}}
	status, err := heartbeat.NewManagedHeartbeatStatusService(
		emptyHeartbeatStatusStore{},
		heartbeat.WithHeartbeatStatusSessionHealthReader(healthReader),
	)
	if err != nil {
		t.Fatal(err)
	}
	manager := testutil.StubSessionManager{
		StatusFn: func(context.Context, string) (*session.Info, error) {
			return &session.Info{
				ID: "sess-profile", WorkspaceID: "ws-profile", AgentName: "coder",
				ProfileID: "profile-marketing", State: session.StateActive,
			}, nil
		},
	}
	for _, tc := range []struct {
		name, path, profile, digestField string
	}{
		{"Should inspect the default profile agent", "/agents/coder/heartbeat?workspace_id=ws-profile", "default", "digest"},
		{"Should inspect the selected profile agent", "/agents/coder/heartbeat?workspace_id=ws-profile&profile=marketing", "marketing", "digest"},
		{"Should enrich a session using its owning profile", "/workspaces/ws-profile/sessions/sess-profile/inspect?profile=default", "marketing", "policy_digest"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
				HomePaths: home, Config: cfg, Workspaces: workspaces, Profiles: sessionProfileServiceStub{},
				Sessions: manager, HeartbeatStatus: status,
				SessionHealth: healthReader,
			})
			engine := gin.New()
			engine.GET("/agents/:name/heartbeat", handlers.GetAgentHeartbeat)
			engine.GET("/workspaces/:workspace_id/sessions/:session_id/inspect", handlers.InspectSession)
			response := performRequest(t, engine, http.MethodGet, tc.path, nil)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body)
			}
			var payload map[string]any
			decodeJSON(t, response.Body.Bytes(), &payload)
			if got := payload[tc.digestField]; got != digests[tc.profile] {
				t.Fatalf(
					"%s = %v, want policy from %s profile (%s)",
					tc.digestField,
					got,
					tc.profile,
					digests[tc.profile],
				)
			}
		})
	}
}

func TestAuthoredContextProfileScope(t *testing.T) {
	t.Parallel()
	for _, sidecar := range []string{"soul", "heartbeat"} {
		for _, sourceKind := range []string{"workspace", "personal", "dot-compozy-home"} {
			t.Run(
				"Should isolate "+sidecar+" reads writes and history by selected profile in "+sourceKind,
				func(t *testing.T) {
					t.Parallel()
					root := t.TempDir()
					agentRoot := filepath.Join(root, ".compozy")
					prefix := ".compozy/"
					if sourceKind == "personal" {
						agentRoot = filepath.Join(t.TempDir(), "compozy-home")
						prefix = ""
					}
					if sourceKind == "dot-compozy-home" {
						agentRoot = filepath.Join(t.TempDir(), ".compozy")
					}
					paths := map[string]string{}
					for _, profile := range []string{"default", "marketing"} {
						dir := filepath.Join(agentRoot, "agents", "coder")
						if profile != "default" {
							dir = filepath.Join(agentRoot, "profiles", profile, "agents", "coder")
						}
						if err := os.MkdirAll(dir, 0o755); err != nil {
							t.Fatal(err)
						}
						paths[profile] = filepath.Join(dir, "AGENT.md")
						if err := os.WriteFile(
							paths[profile],
							[]byte("---\nname: coder\nprovider: codex\n---\nCode carefully.\n"),
							0o600,
						); err != nil {
							t.Fatal(err)
						}
					}
					db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), store.GlobalDatabaseName))
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if err := db.Close(context.Background()); err != nil {
							t.Error(err)
						}
					})
					ws := workspacepkg.Workspace{ID: "ws-profile-authoring", RootDir: root, Name: "profile-authoring"}
					if err := db.InsertWorkspace(t.Context(), ws); err != nil {
						t.Fatal(err)
					}
					workspaces := testutil.StubWorkspaceService{
						ResolveForProfileFn: func(_ context.Context, ref, profile string) (workspacepkg.ResolvedWorkspace, error) {
							if ref != ws.ID {
								return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
							}
							return workspacepkg.ResolvedWorkspace{
								Workspace:   ws,
								WorkspaceID: ws.ID,
								ProfileName: profile,
								Agents:      []compozyconfig.AgentDef{{Name: "coder", SourcePath: paths[profile]}},
								Config: compozyconfig.Config{
									Agents: compozyconfig.AgentsConfig{
										Soul:      compozyconfig.DefaultSoulConfig(),
										Heartbeat: compozyconfig.DefaultHeartbeatConfig(),
									},
								},
							}, nil
						},
					}
					fixture := newHandlerFixture(
						t,
						testutil.StubSessionManager{},
						testutil.StubObserver{},
						workspaces,
						nil,
						nil,
					)
					fixture.Handlers.Profiles = sessionProfileServiceStub{}
					fixture.Handlers.SoulAuthoring, err = soul.NewManagedSoulAuthoringService(db)
					if err != nil {
						t.Fatal(err)
					}
					fixture.Handlers.HeartbeatAuthoring, err = heartbeat.NewManagedHeartbeatAuthoringService(db)
					if err != nil {
						t.Fatal(err)
					}
					fixture.Handlers.HeartbeatStatus, err = heartbeat.NewManagedHeartbeatStatusService(db)
					if err != nil {
						t.Fatal(err)
					}
					fixture.Engine.GET("/agents/:name/soul", fixture.Handlers.GetAgentSoul)
					fixture.Engine.POST("/agents/:name/soul/validate", fixture.Handlers.ValidateAgentSoulDefinition)
					fixture.Engine.PUT("/agents/:name/soul", fixture.Handlers.PutAgentSoul)
					fixture.Engine.DELETE("/agents/:name/soul", fixture.Handlers.DeleteAgentSoul)
					fixture.Engine.GET("/agents/:name/soul/history", fixture.Handlers.ListAgentSoulHistory)
					fixture.Engine.POST("/agents/:name/soul/rollback", fixture.Handlers.RollbackAgentSoul)
					fixture.Engine.GET("/agents/:name/heartbeat", fixture.Handlers.GetAgentHeartbeat)
					fixture.Engine.POST("/agents/:name/heartbeat/validate", fixture.Handlers.ValidateAgentHeartbeat)
					fixture.Engine.PUT("/agents/:name/heartbeat", fixture.Handlers.PutAgentHeartbeat)
					fixture.Engine.DELETE("/agents/:name/heartbeat", fixture.Handlers.DeleteAgentHeartbeat)
					fixture.Engine.GET("/agents/:name/heartbeat/history", fixture.Handlers.ListAgentHeartbeatHistory)
					fixture.Engine.POST("/agents/:name/heartbeat/rollback", fixture.Handlers.RollbackAgentHeartbeat)
					request := func(method, suffix, profile string, payload any, status int) *httptest.ResponseRecorder {
						t.Helper()
						query := "?workspace_id=" + ws.ID
						if profile != "default" {
							query += "&profile=" + profile
						}
						var body []byte
						if payload != nil {
							body = mustJSON(t, payload)
						}
						response := performRequest(
							t,
							fixture.Engine,
							method,
							"/agents/coder/"+sidecar+suffix+query,
							body,
						)
						if response.Code != status {
							t.Fatalf(
								"%s %s profile=%s: %d, want %d; body=%s",
								method,
								suffix,
								profile,
								response.Code,
								status,
								response.Body.String(),
							)
						}
						return response
					}
					digests, revisions, bodies := map[string]string{}, map[string]string{}, map[string]string{}
					for _, profile := range []string{"default", "marketing"} {
						body := "---\nversion: \"1\"\nrole: coder\n---\n" + profile + " guidance.\n"
						if sidecar == "heartbeat" {
							body = "---\nversion: \"1\"\nenabled: true\nsummary: Profile guidance\n---\n" + profile + " guidance.\n"
						}
						bodies[profile] = body
						response := request(
							http.MethodPut,
							"",
							profile,
							map[string]string{"body": body, "expected_digest": ""},
							http.StatusOK,
						)
						var created struct {
							Soul      contract.AgentSoulPayload
							Heartbeat contract.HeartbeatPolicyPayload
							Revision  struct {
								ID         string `json:"id"`
								SourcePath string `json:"source_path"`
							}
						}
						if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
							t.Fatal(err)
						}
						digests[profile] = created.Soul.Digest
						if sidecar == "heartbeat" {
							digests[profile] = created.Heartbeat.Digest
						}
						revisions[profile] = created.Revision.ID
						wantSource := prefix + "agents/coder/" + strings.ToUpper(sidecar) + ".md"
						if profile != "default" {
							wantSource = prefix + "profiles/" + profile + "/agents/coder/" + strings.ToUpper(
								sidecar,
							) + ".md"
						}
						if created.Revision.SourcePath != wantSource {
							t.Fatalf(
								"profile %s revision source=%q want historical representation %q",
								profile,
								created.Revision.SourcePath,
								wantSource,
							)
						}
					}
					for _, profile := range []string{"default", "marketing"} {
						for _, method := range []string{http.MethodGet, http.MethodPost} {
							suffix := ""
							var body any
							if method == http.MethodPost {
								suffix = "/validate"
								body = map[string]string{"body": bodies[profile]}
							}
							response := request(method, suffix, profile, body, http.StatusOK)
							if !strings.Contains(response.Body.String(), profile+" guidance.") {
								t.Fatalf("read/validate profile %s body=%s", profile, response.Body.String())
							}
						}
						response := request(http.MethodGet, "/history", profile, nil, http.StatusOK)
						var history struct{ Revisions []struct{ ID string } }
						if err := json.Unmarshal(response.Body.Bytes(), &history); err != nil {
							t.Fatal(err)
						}
						if len(history.Revisions) != 1 || history.Revisions[0].ID != revisions[profile] {
							t.Fatalf("profile %s history=%s", profile, response.Body.String())
						}
					}
					foreign := request(
						http.MethodPost,
						"/rollback",
						"marketing",
						map[string]string{"revision_id": revisions["default"], "expected_digest": digests["marketing"]},
						http.StatusNotFound,
					)
					if !strings.Contains(foreign.Body.String(), "revision") {
						t.Fatalf("foreign rollback error=%s", foreign.Body.String())
					}
					request(
						http.MethodPost,
						"/rollback",
						"marketing",
						map[string]string{
							"revision_id":     revisions["marketing"],
							"expected_digest": digests["marketing"],
						},
						http.StatusOK,
					)
					request(
						http.MethodDelete,
						"",
						"marketing",
						map[string]string{"expected_digest": digests["marketing"]},
						http.StatusOK,
					)
					defaultBody, err := os.ReadFile(
						filepath.Join(filepath.Dir(paths["default"]), strings.ToUpper(sidecar)+".md"),
					)
					if err != nil || string(defaultBody) != bodies["default"] {
						t.Fatalf("default sidecar changed: body=%s err=%v", defaultBody, err)
					}
					if _, err := os.Stat(
						filepath.Join(filepath.Dir(paths["marketing"]), strings.ToUpper(sidecar)+".md"),
					); !errors.Is(
						err,
						os.ErrNotExist,
					) {
						t.Fatalf("selected sidecar deletion: %v", err)
					}
					response := request(http.MethodGet, "", "marketing&all_profiles=true", nil, http.StatusBadRequest)
					if !strings.Contains(response.Body.String(), "profile") {
						t.Fatalf("aggregate error=%s", response.Body.String())
					}
				},
			)
		}
	}
}

func TestAuthoredContextHeartbeatSessionProfileScope(t *testing.T) {
	t.Parallel()
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		for _, sessionScope := range []string{"foreign-profile", "foreign-agent", "selected"} {
			t.Run(
				fmt.Sprintf("Should bind %s heartbeat session to profile and agent %s", method, sessionScope),
				func(t *testing.T) {
					t.Parallel()
					root := t.TempDir()
					manager := testutil.StubSessionManager{
						StatusFn: func(_ context.Context, id string) (*session.Info, error) {
							info := &session.Info{
								ID:          id,
								WorkspaceID: "ws-1",
								ProfileID:   "profile-marketing",
								AgentName:   "coder",
							}
							if sessionScope == "foreign-profile" {
								info.ProfileID = store.DefaultProfileID
							}
							if sessionScope == "foreign-agent" {
								info.AgentName = "other"
							}
							return info, nil
						},
					}
					workspaces := testutil.StubWorkspaceService{
						ResolveForProfileFn: func(_ context.Context, ref, profile string) (workspacepkg.ResolvedWorkspace, error) {
							if profile != "marketing" {
								t.Fatalf("workspace profile=%q, want marketing", profile)
							}
							return workspacepkg.ResolvedWorkspace{
								Workspace: workspacepkg.Workspace{ID: ref, RootDir: root},
								Config: compozyconfig.Config{
									Agents: compozyconfig.AgentsConfig{
										Heartbeat: compozyconfig.DefaultHeartbeatConfig(),
									},
								},
							}, nil
						},
					}
					fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, workspaces, nil, nil)
					fixture.Handlers.Profiles = sessionProfileServiceStub{}
					status, wake := &heartbeatStatusSpy{}, &heartbeatWakeSpy{}
					fixture.Handlers.HeartbeatStatus, fixture.Handlers.HeartbeatWake = status, wake
					fixture.Engine.GET("/agents/:name/heartbeat/status", fixture.Handlers.GetAgentHeartbeatStatus)
					fixture.Engine.POST("/agents/:name/heartbeat/wake", fixture.Handlers.WakeAgentHeartbeat)
					path := "/agents/coder/heartbeat/status?workspace_id=ws-1&profile=marketing&session_id=sess-owned"
					var body []byte
					if method == http.MethodPost {
						path = "/agents/coder/heartbeat/wake?profile=marketing"
						body = []byte(
							`{"workspace_id":"ws-1","session_id":"sess-owned","source":"manual","dry_run":true}`,
						)
					}
					response := performRequest(t, fixture.Engine, method, path, body)
					expected := http.StatusNotFound
					if sessionScope == "selected" {
						expected = http.StatusOK
						if method == http.MethodPost {
							expected = http.StatusConflict
						}
					}
					if response.Code != expected {
						t.Fatalf("status=%d want=%d body=%s", response.Code, expected, response.Body.String())
					}
					if sessionScope != "selected" {
						if !strings.Contains(response.Body.String(), "workspace-scoped resource not found") ||
							status.calls != 0 ||
							wake.calls != 0 {
							t.Fatalf(
								"foreign session reached service: status=%d wake=%d body=%s",
								status.calls,
								wake.calls,
								response.Body.String(),
							)
						}
					} else {
						marker := `"agent_name":"coder"`
						if method == http.MethodPost {
							marker = `"result":"skipped"`
						}
						if status.calls+wake.calls != 1 || !strings.Contains(response.Body.String(), marker) {
							t.Fatalf(
								"selected session calls=%d/%d body=%s",
								status.calls,
								wake.calls,
								response.Body.String(),
							)
						}
					}
				},
			)
		}
	}
}
