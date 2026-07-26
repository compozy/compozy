package daemon

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
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/httpapi"
	"github.com/compozy/agh/internal/api/testutil"
	"github.com/compozy/agh/internal/api/udsapi"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/soul"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

type authoredContextFixture struct {
	workspace workspacepkg.ResolvedWorkspace
	soul      *fakeSoulAuthoring
	heartbeat *fakeHeartbeatAuthoring
	wake      *fakeHeartbeatWake
	health    *fakeSessionHealth
}

func init() {
	gin.SetMode(gin.TestMode)
}

type fakeSoulAuthoring struct {
	validateFn func(context.Context, soul.ValidateRequest) (soul.ValidateResult, error)
	putFn      func(context.Context, soul.PutRequest) (soul.MutationResult, error)
}

func (f *fakeSoulAuthoring) Validate(
	ctx context.Context,
	req soul.ValidateRequest,
) (soul.ValidateResult, error) {
	if f.validateFn != nil {
		return f.validateFn(ctx, req)
	}
	return soul.ValidateResult{}, nil
}

func (f *fakeSoulAuthoring) Put(ctx context.Context, req soul.PutRequest) (soul.MutationResult, error) {
	if f.putFn != nil {
		return f.putFn(ctx, req)
	}
	return soul.MutationResult{}, nil
}

func (f *fakeSoulAuthoring) Delete(context.Context, soul.DeleteRequest) (soul.MutationResult, error) {
	return soul.MutationResult{}, nil
}

func (f *fakeSoulAuthoring) History(context.Context, soul.HistoryRequest) (soul.HistoryResult, error) {
	return soul.HistoryResult{}, nil
}

func (f *fakeSoulAuthoring) Rollback(context.Context, soul.RollbackRequest) (soul.MutationResult, error) {
	return soul.MutationResult{}, nil
}

type fakeHeartbeatAuthoring struct {
	validateFn func(context.Context, heartbeat.ValidateRequest) (heartbeat.ValidateResult, error)
	putFn      func(context.Context, heartbeat.PutRequest) (heartbeat.MutationResult, error)
	putCalls   atomic.Int64
}

func (f *fakeHeartbeatAuthoring) Validate(
	ctx context.Context,
	req heartbeat.ValidateRequest,
) (heartbeat.ValidateResult, error) {
	if f.validateFn != nil {
		return f.validateFn(ctx, req)
	}
	return heartbeat.ValidateResult{}, nil
}

func (f *fakeHeartbeatAuthoring) Put(
	ctx context.Context,
	req heartbeat.PutRequest,
) (heartbeat.MutationResult, error) {
	f.putCalls.Add(1)
	if f.putFn != nil {
		return f.putFn(ctx, req)
	}
	return heartbeat.MutationResult{}, nil
}

func (f *fakeHeartbeatAuthoring) Delete(
	context.Context,
	heartbeat.DeleteRequest,
) (heartbeat.MutationResult, error) {
	return heartbeat.MutationResult{}, nil
}

func (f *fakeHeartbeatAuthoring) History(
	context.Context,
	heartbeat.HistoryRequest,
) (heartbeat.HistoryResult, error) {
	return heartbeat.HistoryResult{}, nil
}

func (f *fakeHeartbeatAuthoring) Rollback(
	context.Context,
	heartbeat.RollbackRequest,
) (heartbeat.MutationResult, error) {
	return heartbeat.MutationResult{}, nil
}

type fakeHeartbeatWake struct {
	wakeFn func(context.Context, heartbeat.WakeRequest) (heartbeat.WakeDecision, error)
}

func (f *fakeHeartbeatWake) Wake(
	ctx context.Context,
	req heartbeat.WakeRequest,
) (heartbeat.WakeDecision, error) {
	if f.wakeFn != nil {
		return f.wakeFn(ctx, req)
	}
	return heartbeat.WakeDecision{}, nil
}

type fakeSessionHealth struct {
	getFn func(context.Context, string) (heartbeat.SessionHealth, error)
}

func (f *fakeSessionHealth) GetSessionHealth(
	ctx context.Context,
	sessionID string,
) (heartbeat.SessionHealth, error) {
	if f.getFn != nil {
		return f.getFn(ctx, sessionID)
	}
	return heartbeat.SessionHealth{}, heartbeat.ErrSessionHealthNotFound
}

func TestAuthoredContextTransportParity(t *testing.T) {
	t.Parallel()

	t.Run("Should return equivalent Soul read payloads over HTTP and UDS", func(t *testing.T) {
		t.Parallel()

		fixture := newAuthoredContextFixture(t)
		engines := map[string]*gin.Engine{
			"http": newHTTPAuthoredContextEngine(t, fixture),
			"uds":  newUDSAuthoredContextEngine(t, fixture),
		}
		responses := make(map[string]transportResponse, len(engines))
		for name, engine := range engines {
			responses[name] = performAuthoredRequest(
				engine,
				http.MethodGet,
				"/api/agents/coder/soul?workspace_id=ws-1",
				"",
				nil,
			)
		}
		assertTransportParity(t, responses, http.StatusOK)

		var payload contract.AgentSoulPayload
		if err := json.Unmarshal(responses["http"].body, &payload); err != nil {
			t.Fatalf("Unmarshal(AgentSoulPayload) error = %v", err)
		}
		if payload.AgentName != "coder" || payload.Body != "Stay precise." {
			t.Fatalf("Soul payload = %#v, want coder read model body", payload)
		}
		if bytes.Contains(responses["http"].body, []byte("raw_prompt_only")) {
			t.Fatalf("Soul response leaked forbidden prompt-only content: %s", responses["http"].body)
		}
	})

	t.Run("Should return equivalent stale expected digest errors over HTTP and UDS", func(t *testing.T) {
		t.Parallel()

		fixture := newAuthoredContextFixture(t)
		fixture.soul.putFn = func(context.Context, soul.PutRequest) (soul.MutationResult, error) {
			return soul.MutationResult{}, fmt.Errorf("wrapped: %w", soul.ErrAuthoringConflict)
		}
		body := `{"workspace_id":"ws-1","body":"---\nrole: Reviewer\n---\nUpdated.","expected_digest":"old"}`
		engines := map[string]*gin.Engine{
			"http": newHTTPAuthoredContextEngine(t, fixture),
			"uds":  newUDSAuthoredContextEngine(t, fixture),
		}
		responses := make(map[string]transportResponse, len(engines))
		for name, engine := range engines {
			responses[name] = performAuthoredRequest(
				engine,
				http.MethodPut,
				"/api/agents/coder/soul",
				body,
				nil,
			)
		}
		assertTransportParity(t, responses, http.StatusConflict)
	})
}

func TestAuthoredContextCoreRouteBehavior(t *testing.T) {
	t.Parallel()

	t.Run("Should reject Heartbeat If-Match headers before service mutation", func(t *testing.T) {
		t.Parallel()

		fixture := newAuthoredContextFixture(t)
		engine := newUDSAuthoredContextEngine(t, fixture)
		headers := map[string]string{"If-Match": `"stale"`}
		response := performAuthoredRequest(
			engine,
			http.MethodPut,
			"/api/agents/coder/heartbeat",
			`{"workspace_id":"ws-1","body":"---\nversion: 1\nsummary: ok\n---\nWake."}`,
			headers,
		)
		if response.status != http.StatusBadRequest {
			t.Fatalf("status = %d, body = %s, want %d", response.status, response.body, http.StatusBadRequest)
		}
		if fixture.heartbeat.putCalls.Load() != 0 {
			t.Fatalf("Heartbeat Put calls = %d, want 0", fixture.heartbeat.putCalls.Load())
		}
		if !bytes.Contains(response.body, []byte("expected_digest")) {
			t.Fatalf("error body = %s, want expected_digest guidance", response.body)
		}
	})

	t.Run("Should expose closed session health reason", func(t *testing.T) {
		t.Parallel()

		fixture := newAuthoredContextFixture(t)
		engine := newUDSAuthoredContextEngine(t, fixture)
		response := performAuthoredRequest(
			engine,
			http.MethodGet,
			"/api/workspaces/ws-1/sessions/sess-1/health",
			"",
			nil,
		)
		if response.status != http.StatusOK {
			t.Fatalf("status = %d, body = %s, want %d", response.status, response.body, http.StatusOK)
		}
		var payload contract.SessionHealthResponse
		if err := json.Unmarshal(response.body, &payload); err != nil {
			t.Fatalf("Unmarshal(SessionHealthResponse) error = %v", err)
		}
		if payload.Health.Health != contract.SessionHealthDead ||
			payload.Health.IneligibilityReason != contract.SessionHealthReasonDead {
			t.Fatalf("health payload = %#v, want dead closed reason", payload.Health)
		}
	})

	t.Run("Should return ineligible wake decisions as conflicts", func(t *testing.T) {
		t.Parallel()

		fixture := newAuthoredContextFixture(t)
		engine := newUDSAuthoredContextEngine(t, fixture)
		response := performAuthoredRequest(
			engine,
			http.MethodPost,
			"/api/agents/coder/heartbeat/wake",
			`{"workspace_id":"ws-1","session_id":"sess-1","source":"manual"}`,
			nil,
		)
		if response.status != http.StatusConflict {
			t.Fatalf("status = %d, body = %s, want %d", response.status, response.body, http.StatusConflict)
		}
		var payload contract.HeartbeatWakeResponse
		if err := json.Unmarshal(response.body, &payload); err != nil {
			t.Fatalf("Unmarshal(HeartbeatWakeResponse) error = %v", err)
		}
		if payload.Decision.Result != contract.HeartbeatWakeResultSkipped ||
			payload.Decision.Reason != contract.HeartbeatWakeReasonSessionUnhealthy {
			t.Fatalf("wake decision = %#v, want skipped/session_unhealthy", payload.Decision)
		}
	})
}

type transportResponse struct {
	status int
	body   []byte
}

func newAuthoredContextFixture(t *testing.T) *authoredContextFixture {
	t.Helper()

	root := t.TempDir()
	agentDir := filepath.Join(root, ".agh", "agents", "coder")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(agentDir) error = %v", err)
	}
	agentPath := filepath.Join(agentDir, "AGENT.md")
	if err := os.WriteFile(agentPath, []byte("name: coder\nprovider: test\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(AGENT.md) error = %v", err)
	}
	cfg := aghconfig.Config{}
	cfg.Agents.Soul = aghconfig.DefaultSoulConfig()
	cfg.Agents.Heartbeat = aghconfig.DefaultHeartbeatConfig()
	workspace := workspacepkg.ResolvedWorkspace{
		Workspace:   workspacepkg.Workspace{ID: "ws-1", RootDir: root, Name: "workspace"},
		WorkspaceID: "ws-1",
		Config:      cfg,
		Agents:      []aghconfig.AgentDef{{Name: "coder", Provider: "test", SourcePath: agentPath}},
	}
	resolvedSoul := parseTestSoul(t, root, filepath.Join(agentDir, soul.FileName), cfg.Agents.Soul)
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	return &authoredContextFixture{
		workspace: workspace,
		soul: &fakeSoulAuthoring{
			validateFn: func(context.Context, soul.ValidateRequest) (soul.ValidateResult, error) {
				return soul.ValidateResult{Soul: resolvedSoul}, nil
			},
		},
		heartbeat: &fakeHeartbeatAuthoring{},
		wake: &fakeHeartbeatWake{
			wakeFn: func(context.Context, heartbeat.WakeRequest) (heartbeat.WakeDecision, error) {
				return heartbeat.WakeDecision{
					WakeEventID: "wake-1",
					Result:      heartbeat.WakeResultSkipped,
					Reason:      heartbeat.WakeReasonSessionUnhealthy,
				}, nil
			},
		},
		health: &fakeSessionHealth{
			getFn: func(context.Context, string) (heartbeat.SessionHealth, error) {
				return heartbeat.SessionHealth{
					SessionID:           "sess-1",
					WorkspaceID:         "ws-1",
					AgentName:           "coder",
					State:               heartbeat.SessionHealthStateStopped,
					Health:              heartbeat.SessionHealthDead,
					Attachable:          false,
					EligibleForWake:     false,
					IneligibilityReason: string(heartbeat.SessionHealthReasonDead),
					UpdatedAt:           now,
				}, nil
			},
		},
	}
}

func parseTestSoul(
	t *testing.T,
	workspaceRoot string,
	sourcePath string,
	cfg aghconfig.SoulConfig,
) soul.ResolvedSoul {
	t.Helper()

	content := []byte(strings.Join([]string{
		"---",
		"version: 1",
		"role: Reviewer",
		"tone:",
		"  - direct",
		"principles:",
		"  - protect correctness",
		"---",
		"Stay precise.",
	}, "\n"))
	resolved, err := soul.Parse(context.Background(), soul.ParseRequest{
		SourcePath:    sourcePath,
		WorkspaceRoot: workspaceRoot,
		Content:       content,
		Config:        cfg,
	})
	if err != nil {
		t.Fatalf("soul.Parse() error = %v", err)
	}
	return resolved
}

func newHTTPAuthoredContextEngine(t *testing.T, fixture *authoredContextFixture) *gin.Engine {
	t.Helper()

	engine := gin.New()
	_, err := httpapi.New(
		httpapi.WithEngine(engine),
		httpapi.WithSessionManager(authoredContextSessionManager()),
		httpapi.WithTaskService(&testutil.StubTaskManager{}),
		httpapi.WithObserver(testutil.StubObserver{}),
		httpapi.WithWorkspaceResolver(workspaceResolverForFixture(fixture)),
		httpapi.WithSoulAuthoring(fixture.soul),
		httpapi.WithHeartbeatAuthoring(fixture.heartbeat),
		httpapi.WithHeartbeatWake(fixture.wake),
		httpapi.WithSessionHealthReader(fixture.health),
	)
	if err != nil {
		t.Fatalf("httpapi.New() error = %v", err)
	}
	return engine
}

func newUDSAuthoredContextEngine(t *testing.T, fixture *authoredContextFixture) *gin.Engine {
	t.Helper()

	engine := gin.New()
	socketPath := filepath.Join(
		os.TempDir(),
		fmt.Sprintf("agh-%d-%d.sock", os.Getpid(), time.Now().UnixNano()),
	)
	t.Cleanup(func() {
		if err := os.Remove(socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Remove(%q) error = %v", socketPath, err)
		}
	})
	_, err := udsapi.New(
		udsapi.WithEngine(engine),
		udsapi.WithSocketPath(socketPath),
		udsapi.WithSessionManager(authoredContextSessionManager()),
		udsapi.WithTaskService(&testutil.StubTaskManager{}),
		udsapi.WithObserver(testutil.StubObserver{}),
		udsapi.WithWorkspaceResolver(workspaceResolverForFixture(fixture)),
		udsapi.WithSoulAuthoring(fixture.soul),
		udsapi.WithHeartbeatAuthoring(fixture.heartbeat),
		udsapi.WithHeartbeatWake(fixture.wake),
		udsapi.WithSessionHealthReader(fixture.health),
	)
	if err != nil {
		t.Fatalf("udsapi.New() error = %v", err)
	}
	return engine
}

func workspaceResolverForFixture(fixture *authoredContextFixture) testutil.StubWorkspaceService {
	return testutil.StubWorkspaceService{
		ResolveFn: func(ctx context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
			if err := ctx.Err(); err != nil {
				return workspacepkg.ResolvedWorkspace{}, err
			}
			if strings.TrimSpace(ref) != "ws-1" {
				return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
			}
			return fixture.workspace, nil
		},
	}
}

func authoredContextSessionManager() testutil.StubSessionManager {
	return testutil.StubSessionManager{
		StatusFn: func(ctx context.Context, id string) (*session.Info, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if strings.TrimSpace(id) != "sess-1" {
				return nil, session.ErrSessionNotFound
			}
			return &session.Info{ID: "sess-1", WorkspaceID: "ws-1", AgentName: "coder"}, nil
		},
	}
}

func performAuthoredRequest(
	engine http.Handler,
	method string,
	path string,
	body string,
	headers map[string]string,
) transportResponse {
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	request := httptest.NewRequestWithContext(context.Background(), method, path, reader)
	request.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	return transportResponse{status: recorder.Code, body: recorder.Body.Bytes()}
}

func assertTransportParity(t *testing.T, responses map[string]transportResponse, expectedStatus int) {
	t.Helper()

	httpResponse, ok := responses["http"]
	if !ok {
		t.Fatal("missing http response")
	}
	udsResponse, ok := responses["uds"]
	if !ok {
		t.Fatal("missing uds response")
	}
	if httpResponse.status != expectedStatus || udsResponse.status != expectedStatus {
		t.Fatalf(
			"statuses = http:%d uds:%d, want %d; bodies http=%s uds=%s",
			httpResponse.status,
			udsResponse.status,
			expectedStatus,
			httpResponse.body,
			udsResponse.body,
		)
	}
	if !bytes.Equal(httpResponse.body, udsResponse.body) {
		t.Fatalf("HTTP/UDS bodies differ:\nhttp: %s\nuds:  %s", httpResponse.body, udsResponse.body)
	}
}
