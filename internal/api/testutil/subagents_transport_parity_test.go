package testutil_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/httpapi"
	"github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/api/udsapi"
	"github.com/compozy/compozy/internal/listcursor"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
)

// IT-018, UT-034: transport routing preserves scoped DTOs and cancellation semantics.
func TestSubagentHTTPUDSParity(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, method, path, body, contains string
		status                             int
	}{
		{"list", "GET", "/api/workspaces/ws-public/sessions/parent/subagents", "", `"next_cursor":null`, 200},
		{"show", "GET", "/api/workspaces/ws-public/subagents/sub-1", "", `"result_preview":"answer"`, 200},
		{"cancel", "POST", "/api/workspaces/ws-public/subagents/sub-1/cancel", `{"reason":"done"}`, `"status":"cancel_requested"`, 202},
		{"cancel terminal", "POST", "/api/workspaces/ws-public/subagents/completed/cancel", `{"reason":"done"}`, `"status":"completed"`, 202},
		{"valid filters", "GET", "/api/workspaces/ws-public/sessions/parent/subagents?origin=delegated&status=completed,running&limit=20&cursor=opaque", "", `"next_cursor":"next"`, 200},
		{"empty limit", "GET", "/api/workspaces/ws-public/sessions/parent/subagents?limit=", "", `"code":"invalid_request"`, 400},
		{"cancel native", "POST", "/api/workspaces/ws-public/subagents/native/cancel", `{}`, `"code":"subagent_not_cancelable"`, 409},
		{"unknown parent", "GET", "/api/workspaces/ws-public/sessions/missing/subagents", "", `"code":"session_not_found"`, 404},
		{"inaccessible parent", "GET", "/api/workspaces/ws-public/sessions/foreign/subagents", "", `"code":"session_not_found"`, 404},
		{"unknown row", "GET", "/api/workspaces/ws-public/subagents/missing", "", `"code":"subagent_not_found"`, 404},
		{"foreign row", "GET", "/api/workspaces/ws-public/subagents/foreign", "", `"code":"subagent_not_found"`, 404},
		{"foreign cancel", "POST", "/api/workspaces/ws-public/subagents/foreign/cancel", `{}`, `"code":"subagent_not_found"`, 404},
		{"profile isolation", "GET", "/api/workspaces/ws-public/subagents/private", "", `"code":"subagent_not_found"`, 404},
		{"bad origin", "GET", "/api/workspaces/ws-public/sessions/parent/subagents?origin=bad", "", `"code":"invalid_request"`, 400},
		{"bad status", "GET", "/api/workspaces/ws-public/sessions/parent/subagents?status=running,bad", "", `"code":"invalid_request"`, 400},
		{"bad limit", "GET", "/api/workspaces/ws-public/sessions/parent/subagents?limit=201", "", `"code":"invalid_request"`, 400},
		{"bad cursor", "GET", "/api/workspaces/ws-public/sessions/parent/subagents?cursor=bad", "", `"code":"invalid_request"`, 400},
		{"malformed body", "POST", "/api/workspaces/ws-public/subagents/sub-1/cancel", `{`, `"code":"invalid_request"`, 400},
		{"archive child", "POST", "/api/workspaces/ws-public/sessions/child/archive", `{}`, `"code":"subagent_archive_follows_parent"`, 409},
		{"unarchive child", "POST", "/api/workspaces/ws-public/sessions/child/unarchive", `{}`, `"code":"subagent_archive_follows_parent"`, 409},
	} {
		t.Run("Should match "+tc.name, func(t *testing.T) {
			t.Parallel()
			service := &subagentParityService{}
			httpRouter, udsRouter := subagentParityRouters(t, service)
			a := testutil.PerformRequest(t, httpRouter, tc.method, tc.path, []byte(tc.body))
			b := testutil.PerformRequest(t, udsRouter, tc.method, tc.path, []byte(tc.body))
			if a.Code != tc.status || b.Code != tc.status || a.Body.String() != b.Body.String() || !strings.Contains(a.Body.String(), tc.contains) {
				t.Fatalf("HTTP = %d %s; UDS = %d %s; want %d containing %s", a.Code, a.Body, b.Code, b.Body, tc.status, tc.contains)
			}
		})
	}
}

// IT-029 API half: filtering reaches the catalog and summaries are batched once per page.
func TestSubagentSessionCatalog(t *testing.T) {
	t.Parallel()
	for _, filter := range []string{"", "include", "exclude", "only", "bad"} {
		t.Run("Should pass and validate visibility "+filter, func(t *testing.T) {
			t.Parallel()
			service := &subagentParityService{}
			a, b := subagentParityRouters(t, service)
			path := "/api/workspaces/ws-public/sessions?limit=100"
			if filter != "" {
				path += "&subagents=" + filter
			}
			for _, router := range []http.Handler{a, b} {
				response := testutil.PerformRequest(t, router, "GET", path, nil)
				if filter == "bad" {
					if response.Code != 400 || !strings.Contains(response.Body.String(), `"code":"invalid_request"`) {
						t.Fatalf("invalid filter: %d %s", response.Code, response.Body)
					}
					continue
				}
				var page contract.SessionCatalogResponse
				if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
					t.Fatal(err)
				}
				if response.Code != 200 || len(page.Sessions) != 1 || (page.Sessions[0].SubagentSummary == nil || page.Sessions[0].SubagentSummary.Total != 30) {
					t.Fatalf("page = %d %s", response.Code, response.Body)
				}
				if service.filter != filter {
					t.Fatalf("filter = %q, want %q", service.filter, filter)
				}
			}
			if filter != "bad" && service.summaries != 2 {
				t.Fatalf("summary calls = %d, want one per page", service.summaries)
			}
		})
	}
	t.Run("Should expose unavailable service explicitly", func(t *testing.T) {
		t.Parallel()
		a, b := subagentParityRouters(t, nil)
		for _, router := range []http.Handler{a, b} {
			response := testutil.PerformRequest(t, router, "GET", "/api/workspaces/ws-public/subagents/sub-1", nil)
			if response.Code != 503 || !strings.Contains(response.Body.String(), "feature_unavailable") {
				t.Fatalf("response = %d %s", response.Code, response.Body)
			}
		}
	})
}

type subagentParityService struct {
	session.SubagentService
	summaries int
	filter    string
}

var _ session.SubagentService = (*subagentParityService)(nil)

func (s *subagentParityService) Get(_ context.Context, workspaceID, id string) (session.Subagent, error) {
	if id == "missing" {
		return session.Subagent{}, session.ErrSubagentNotFound
	}
	row := store.SessionSubagent{ID: id, WorkspaceID: workspaceID, ParentSessionID: "parent", Origin: store.SubagentOriginDelegated, Result: new("answer\nmore")}
	if id == "native" {
		row.Origin = store.SubagentOriginProviderNative
	}
	if id == "foreign" || id == "private" {
		row.ParentSessionID = id
	}
	return session.Subagent{SessionSubagent: row}, nil
}
func (s *subagentParityService) List(_ context.Context, q store.SubagentListQuery) (store.SubagentPage, error) {
	if q.Cursor == "bad" {
		return store.SubagentPage{}, listcursor.ErrInvalid
	}
	if q.Cursor == "opaque" {
		if q.Limit != 20 || len(q.Origins) != 1 || q.Origins[0] != "delegated" || strings.Join(q.Statuses, ",") != "completed,running" {
			return store.SubagentPage{}, errors.New("filters were not forwarded")
		}
		return store.SubagentPage{Items: []store.SessionSubagent{}, NextCursor: "next"}, nil
	}
	if q.ParentSessionID != "parent" || q.WorkspaceID != "ws-registry" || q.Limit != 50 {
		return store.SubagentPage{}, errors.New("unexpected query")
	}
	return store.SubagentPage{Items: []store.SessionSubagent{{ID: "sub-1", ParentSessionID: q.ParentSessionID, WorkspaceID: q.WorkspaceID}}}, nil
}
func (s *subagentParityService) Cancel(_ context.Context, actor session.SubagentActor, id, reason string) (session.SubagentCancelOutcome, error) {
	if actor.Kind != "operator" || reason != "done" {
		return session.SubagentCancelOutcome{}, errors.New("unexpected cancel")
	}
	if id == "completed" {
		return session.SubagentCancelOutcome{ID: id, Status: "completed"}, nil
	}
	return session.SubagentCancelOutcome{ID: id, Status: "cancel_requested"}, nil
}
func (s *subagentParityService) Summaries(_ context.Context, ids []string) (map[string]store.SubagentSummary, error) {
	s.summaries++
	if len(ids) != 1 || ids[0] != "parent" {
		return nil, errors.New("unexpected summary ids")
	}
	return map[string]store.SubagentSummary{"parent": {Total: 30, Live: 3}}, nil
}

func subagentParityRouters(t *testing.T, service *subagentParityService) (http.Handler, http.Handler) {
	t.Helper()
	manager := testutil.StubSessionManager{
		StatusFn: func(_ context.Context, id string) (*session.Info, error) {
			if id == "missing" {
				return nil, session.ErrSessionNotFound
			}
			info := &session.Info{ID: id, WorkspaceID: "ws-registry", ProfileID: store.DefaultProfileID, State: session.StateStopped}
			if id == "foreign" {
				info.WorkspaceID = "elsewhere"
			}
			if id == "private" {
				info.ProfileID = "other-profile"
			}
			if id == "child" {
				info.Lineage = &store.SessionLineage{SpawnRole: store.SubagentSpawnRole}
			}
			return info, nil
		},
		ListPageFn: func(_ context.Context, query session.ListQuery) (session.ListPage, error) {
			service.filter = query.Subagents
			return session.ListPage{Sessions: []*session.Info{{ID: "parent", WorkspaceID: "ws-registry", ProfileID: store.DefaultProfileID}}, Total: 120, Limit: 100}, nil
		},
	}
	var dependency session.SubagentService
	if service != nil {
		dependency = service
	}
	home := newShortParityHomePaths(t)
	cfg := testutil.ConfigForTest(home)
	cfg.HTTP.Host, cfg.HTTP.Port = "127.0.0.1", 2123
	a, b := gin.New(), gin.New()
	workspaceService := parityWorkspaceService()
	workspaceService.GetFn = func(context.Context, string) (workspace.Workspace, error) {
		return workspace.Workspace{ID: "ws-registry", Name: "Parity", RootDir: "/repo"}, nil
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if _, err := httpapi.New(httpapi.WithEngine(a), httpapi.WithHomePaths(home), httpapi.WithConfig(&cfg), httpapi.WithHost(cfg.HTTP.Host), httpapi.WithPort(cfg.HTTP.Port), httpapi.WithLogger(logger), httpapi.WithSessionManager(manager), httpapi.WithTaskService(&testutil.StubTaskManager{}), httpapi.WithObserver(testutil.StubObserver{}), httpapi.WithWorkspaceResolver(workspaceService), httpapi.WithSubagentService(dependency)); err != nil {
		t.Fatal(err)
	}
	if _, err := udsapi.New(udsapi.WithEngine(b), udsapi.WithHomePaths(home), udsapi.WithConfig(&cfg), udsapi.WithLogger(logger), udsapi.WithSessionManager(manager), udsapi.WithTaskService(&testutil.StubTaskManager{}), udsapi.WithObserver(testutil.StubObserver{}), udsapi.WithWorkspaceResolver(workspaceService), udsapi.WithSubagentService(dependency)); err != nil {
		t.Fatal(err)
	}
	return a, b
}
