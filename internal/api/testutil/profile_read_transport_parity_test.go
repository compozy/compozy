package testutil_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/httpapi"
	"github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/api/udsapi"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/worktree"
	"github.com/gin-gonic/gin"
)

const aggregateReadProfileID = "mmmmmmmmmmmmmmmmmmmmmmmmmm"

// Canonical suite: IT-070 scoped and aggregate detail-read parity across HTTP and UDS.
func TestProfileDetailReadHTTPUDSTransportParityIT070(t *testing.T) {
	t.Parallel()

	taskService := &testutil.StubTaskManager{GetTaskFn: func(
		_ context.Context,
		id string,
		_ taskpkg.ActorContext,
	) (*taskpkg.View, error) {
		record := taskpkg.Task{
			ID: id, ProfileID: aggregateReadProfileID, Scope: taskpkg.ScopeGlobal,
			Title: "Foreign task", Status: taskpkg.TaskStatusReady,
		}
		return &taskpkg.View{
			Task: record,
			Summary: taskpkg.Summary{
				ID: id, ProfileID: aggregateReadProfileID, Scope: taskpkg.ScopeGlobal,
				Title: "Foreign task", Status: taskpkg.TaskStatusReady,
			},
		}, nil
	}}
	profiles := parityProfileService{profiles: []profilepkg.WithCounts{
		{
			ID: store.DefaultProfileID, Name: "default", Color: "#8E8EB5", State: profilepkg.StateActive},
		{
			ID: aggregateReadProfileID, Name: "marketing", Color: "#E8572A", State: profilepkg.StateActive},
	}}
	httpRouter := newProfileReadParityHTTPRouter(t, taskService, profiles, &testutil.StubSessionManager{})
	udsRouter := newProfileReadParityUDSRouter(t, taskService, profiles, &testutil.StubSessionManager{})

	for _, test := range []struct {
		name       string
		path       string
		wantStatus int
	}{
		{name: "scoped foreign item", path: "/api/tasks/task-foreign", wantStatus: http.StatusNotFound},
		{name: "aggregate owner-labeled item", path: "/api/tasks/task-foreign?all_profiles=true", wantStatus: http.StatusOK},
	} {
		t.Run("Should match "+test.name, func(t *testing.T) {
			httpResponse := performWorktreeParityRequest(t, httpRouter, http.MethodGet, test.path, "")
			udsResponse := performWorktreeParityRequest(t, udsRouter, http.MethodGet, test.path, "")
			assertWorktreeParityResponse(t, httpResponse, udsResponse, test.wantStatus)
			if test.wantStatus != http.StatusOK {
				var payload contract.ErrorPayload
				if err := json.Unmarshal(httpResponse.Body.Bytes(), &payload); err != nil {
					t.Fatalf("decode scoped foreign task error: %v; body=%s", err, httpResponse.Body.String())
				}
				if payload.Error != taskpkg.ErrTaskNotFound.Error() {
					t.Fatalf("scoped foreign task error = %q, want %q", payload.Error, taskpkg.ErrTaskNotFound.Error())
				}
				return
			}
			var response contract.TaskDetailResponse
			if err := json.Unmarshal(httpResponse.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode aggregate task response: %v", err)
			}
			if response.Task.Task.ProfileID != aggregateReadProfileID ||
				response.Task.Task.ProfileName != "marketing" {
				t.Fatalf("aggregate task owner = %#v, want marketing", response.Task.Task)
			}
		})
	}
}

// Canonical suite: the routed session's profile fences workspace session routes alike over HTTP
// and UDS (#1193), including the transport-owned prompt handlers.
func TestWorkspaceSessionProfileFenceHTTPUDSTransportParity(t *testing.T) {
	t.Parallel()

	sessions := &testutil.StubSessionManager{StatusFn: func(_ context.Context, id string) (*session.Info, error) {
		info := testutil.NewSessionInfo(id)
		info.WorkspaceID, info.ProfileID = "ws-registry", aggregateReadProfileID
		return info, nil
	}}
	profiles := parityProfileService{profiles: []profilepkg.WithCounts{
		{ID: store.DefaultProfileID, Name: "default", Color: "#8E8EB5", State: profilepkg.StateActive},
		{ID: aggregateReadProfileID, Name: "marketing", Color: "#E8572A", State: profilepkg.StateActive},
	}}
	httpRouter := newProfileReadParityHTTPRouter(t, &testutil.StubTaskManager{}, profiles, sessions)
	udsRouter := newProfileReadParityUDSRouter(t, &testutil.StubTaskManager{}, profiles, sessions)
	const route = "/api/workspaces/ws-public/sessions/sess-foreign"
	const prompt = `{"message":"hello","message_id":"msg-parity","idempotency_key":"idem-parity"}`
	for _, test := range []struct {
		name, method, path, body string
		wantStatus               int
	}{
		{"workspace file entries foreign read", http.MethodGet, "/api/workspaces/ws-public/files/entries?profile=default&worktree_id=foreign", "", http.StatusNotFound},
		{"workspace file content foreign read", http.MethodGet, "/api/workspaces/ws-public/files/content?profile=default&worktree_id=foreign&path=a", "", http.StatusNotFound},
		{"file entries foreign read", http.MethodGet, route + "/files/entries?profile=default", "", http.StatusNotFound},
		{"workspace find foreign read", http.MethodGet, "/api/workspaces/ws-public/files/find?q=aa&profile=default&worktree_id=foreign", "", http.StatusNotFound},
		{"workspace search foreign read", http.MethodGet, "/api/workspaces/ws-public/files/search?q=aa&profile=default&worktree_id=foreign", "", http.StatusNotFound},
		{"session find foreign read", http.MethodGet, route + "/files/find?q=aa&profile=default", "", http.StatusNotFound},
		{"session search foreign read", http.MethodGet, route + "/files/search?q=aa&profile=default", "", http.StatusNotFound},
		{"file content foreign read", http.MethodGet, route + "/files/content?path=a&profile=default", "", http.StatusNotFound},
		{"scoped foreign read", http.MethodGet, route + "/events?profile=default", "", http.StatusNotFound},
		{"scoped foreign write", http.MethodPost, route + "/prompt?profile=default", prompt, http.StatusNotFound},
		{"aggregate write", http.MethodPost, route + "/prompt?all_profiles=true", prompt, http.StatusBadRequest},
	} {
		t.Run("Should match "+test.name, func(t *testing.T) {
			t.Parallel()
			httpResponse := performWorktreeParityRequest(t, httpRouter, test.method, test.path, test.body)
			udsResponse := performWorktreeParityRequest(t, udsRouter, test.method, test.path, test.body)
			assertWorktreeParityResponse(t, httpResponse, udsResponse, test.wantStatus)
		})
	}
}

type parityProfileService struct {
	core.ProfileService
	profiles []profilepkg.WithCounts
}

func (s parityProfileService) Resolve(
	_ context.Context,
	input profilepkg.ResolveInput,
) (profilepkg.Resolution, error) {
	for _, item := range s.profiles {
		if item.Name == input.Flag {
			return profilepkg.Resolution{Profile: item.Profile, Source: profilepkg.ResolutionSourceFlag}, nil
		}
	}
	return profilepkg.Resolution{}, profilepkg.ErrNotFound
}

func (s parityProfileService) List(context.Context) ([]profilepkg.WithCounts, error) {
	return append([]profilepkg.WithCounts(nil), s.profiles...), nil
}

func newProfileReadParityHTTPRouter(
	t *testing.T,
	tasks core.TaskService,
	profiles core.ProfileService,
	sessions core.SessionManager,
) http.Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	homePaths := newShortParityHomePaths(t)
	cfg := testutil.ConfigForTest(homePaths)
	cfg.HTTP.Host, cfg.HTTP.Port = "127.0.0.1", 2123
	if _, err := httpapi.New(
		httpapi.WithEngine(engine), httpapi.WithHomePaths(homePaths), httpapi.WithConfig(&cfg),
		httpapi.WithHost(cfg.HTTP.Host), httpapi.WithPort(cfg.HTTP.Port),
		httpapi.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		httpapi.WithStartedAt(parityWorktreeTime()), httpapi.WithNow(parityWorktreeTime),
		httpapi.WithSessionManager(sessions),
		httpapi.WithTaskService(tasks), httpapi.WithProfileService(profiles),
		httpapi.WithObserver(testutil.StubObserver{}),
		httpapi.WithWorkspaceResolver(parityWorkspaceService()),
		httpapi.WithWorktreeService(foreignFileWorktreeService()),
	); err != nil {
		t.Fatalf("httpapi.New(profile read parity) error = %v", err)
	}
	return engine
}

func newProfileReadParityUDSRouter(
	t *testing.T,
	tasks core.TaskService,
	profiles core.ProfileService,
	sessions core.SessionManager,
) http.Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	homePaths := newShortParityHomePaths(t)
	cfg := testutil.ConfigForTest(homePaths)
	if _, err := udsapi.New(
		udsapi.WithEngine(engine), udsapi.WithHomePaths(homePaths), udsapi.WithConfig(&cfg),
		udsapi.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		udsapi.WithStartedAt(parityWorktreeTime()), udsapi.WithNow(parityWorktreeTime),
		udsapi.WithSessionManager(sessions),
		udsapi.WithTaskService(tasks), udsapi.WithProfileService(profiles),
		udsapi.WithObserver(testutil.StubObserver{}),
		udsapi.WithWorkspaceResolver(parityWorkspaceService()),
		udsapi.WithWorktreeService(foreignFileWorktreeService()),
	); err != nil {
		t.Fatalf("udsapi.New(profile read parity) error = %v", err)
	}
	return engine
}

func (s parityProfileService) ListIdentities(ctx context.Context) ([]profilepkg.Profile, error) {
	items, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	profiles := make([]profilepkg.Profile, 0, len(items))
	for _, item := range items {
		profiles = append(profiles, item.Profile)
	}
	return profiles, nil
}

type foreignProfileFileWorktrees struct{ core.WorktreeService }

func (foreignProfileFileWorktrees) Resolve(context.Context, string, string) (*worktree.Worktree, error) {
	return &worktree.Worktree{ID: "foreign", WorkspaceID: "ws-registry", ProfileID: aggregateReadProfileID}, nil
}
func foreignFileWorktreeService() core.WorktreeService { return foreignProfileFileWorktrees{} }
