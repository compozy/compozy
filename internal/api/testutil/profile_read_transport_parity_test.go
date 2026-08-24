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
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
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
		{Profile: profilepkg.Profile{
			ID: store.DefaultProfileID, Name: "default", Color: "#8E8EB5", State: profilepkg.StateActive,
		}},
		{Profile: profilepkg.Profile{
			ID: aggregateReadProfileID, Name: "marketing", Color: "#E8572A", State: profilepkg.StateActive,
		}},
	}}
	httpRouter := newProfileReadParityHTTPRouter(t, taskService, profiles)
	udsRouter := newProfileReadParityUDSRouter(t, taskService, profiles)

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

// Canonical suite: IT-074 memory aggregate reads fail closed across HTTP and UDS.
func TestMemoryAggregateReadRefusalHTTPUDSTransportParityIT074(t *testing.T) {
	t.Parallel()
	t.Run("Should reject memory aggregate reads", func(t *testing.T) {
		t.Parallel()
		profiles := parityProfileService{profiles: []profilepkg.WithCounts{{Profile: profilepkg.Profile{
			ID: store.DefaultProfileID, Name: "default", Color: "#8E8EB5", State: profilepkg.StateActive,
		}}}}
		httpRouter := newProfileReadParityHTTPRouter(t, &testutil.StubTaskManager{}, profiles)
		udsRouter := newProfileReadParityUDSRouter(t, &testutil.StubTaskManager{}, profiles)
		httpResponse := performWorktreeParityRequest(t, httpRouter, http.MethodGet, "/api/memory?all_profiles=true", "")
		udsResponse := performWorktreeParityRequest(t, udsRouter, http.MethodGet, "/api/memory?all_profiles=true", "")
		assertWorktreeParityResponse(t, httpResponse, udsResponse, http.StatusBadRequest)

		var payload contract.ProfileErrorPayload
		if err := json.Unmarshal(httpResponse.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode memory aggregate refusal: %v", err)
		}
		if payload.Error.Code != "profile_selection_conflict" {
			t.Fatalf("memory aggregate error = %#v, want profile_selection_conflict", payload.Error)
		}
	})
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
) http.Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	homePaths := newShortParityHomePaths(t)
	cfg := testutil.ConfigWithDisabledNetwork(homePaths)
	cfg.HTTP.Host, cfg.HTTP.Port = "127.0.0.1", 2123
	if _, err := httpapi.New(
		httpapi.WithEngine(engine), httpapi.WithHomePaths(homePaths), httpapi.WithConfig(&cfg),
		httpapi.WithHost(cfg.HTTP.Host), httpapi.WithPort(cfg.HTTP.Port),
		httpapi.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		httpapi.WithStartedAt(parityWorktreeTime()), httpapi.WithNow(parityWorktreeTime),
		httpapi.WithSessionManager(testutil.StubSessionManager{}),
		httpapi.WithTaskService(tasks), httpapi.WithProfileService(profiles),
		httpapi.WithObserver(testutil.StubObserver{}),
		httpapi.WithWorkspaceResolver(parityWorkspaceService()),
	); err != nil {
		t.Fatalf("httpapi.New(profile read parity) error = %v", err)
	}
	return engine
}

func newProfileReadParityUDSRouter(
	t *testing.T,
	tasks core.TaskService,
	profiles core.ProfileService,
) http.Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	homePaths := newShortParityHomePaths(t)
	cfg := testutil.ConfigWithDisabledNetwork(homePaths)
	if _, err := udsapi.New(
		udsapi.WithEngine(engine), udsapi.WithHomePaths(homePaths), udsapi.WithConfig(&cfg),
		udsapi.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		udsapi.WithStartedAt(parityWorktreeTime()), udsapi.WithNow(parityWorktreeTime),
		udsapi.WithSessionManager(testutil.StubSessionManager{}),
		udsapi.WithTaskService(tasks), udsapi.WithProfileService(profiles),
		udsapi.WithObserver(testutil.StubObserver{}),
		udsapi.WithWorkspaceResolver(parityWorkspaceService()),
	); err != nil {
		t.Fatalf("udsapi.New(profile read parity) error = %v", err)
	}
	return engine
}
