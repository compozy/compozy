package core_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/observe"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func TestObserveOverviewHandler(t *testing.T) {
	t.Parallel()

	t.Run("Should return the mapped overview payload", func(t *testing.T) {
		t.Parallel()

		var captured observe.OverviewQuery
		observer := testutil.StubObserver{
			QueryObserveOverviewFn: func(_ context.Context, query observe.OverviewQuery) (observe.OverviewView, error) {
				captured = query
				return observe.OverviewView{
					GeneratedAt: time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC),
					Attention: observe.OverviewAttention{
						Total:  2,
						ByKind: map[string]int{"approval": 1, "failure": 1},
						Items: []observe.OverviewAttentionItem{{
							Kind:        "approval",
							Title:       "Deploy docs site",
							TaskID:      "task-1",
							WorkspaceID: "ws-home",
							OccurredAt:  time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC),
							Actions:     []string{"approve", "reject", "open"},
						}},
					},
					Today:  observe.OverviewToday{RunsCompleted: 5, RunsFailed: 1, TasksClosed: 3},
					Usage:  observe.OverviewUsage{},
					System: observe.OverviewSystem{RetentionDays: 7},
				}, nil
			},
		}
		fixture := newHandlerFixture(
			t, testutil.StubSessionManager{}, observer, testutil.StubWorkspaceService{},
		)

		request := httptest.NewRequestWithContext(
			t.Context(), http.MethodGet, "/observe/overview?usage_window=7", http.NoBody,
		)
		response := httptest.NewRecorder()
		fixture.Engine.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body.String())
		}
		if captured.UsageWindowDays != 7 || captured.TaskScope != taskpkg.CatalogScopeGlobal {
			t.Fatalf("observer query = %+v, want window 7 with global scope", captured)
		}

		var decoded contract.ObserveOverviewResponse
		if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if decoded.Overview.SchemaVersion != contract.ObserveOverviewSchemaVersion {
			t.Fatalf(
				"schema_version = %q, want %q",
				decoded.Overview.SchemaVersion, contract.ObserveOverviewSchemaVersion,
			)
		}
		if decoded.Overview.Attention.Total != 2 || len(decoded.Overview.Attention.Items) != 1 ||
			decoded.Overview.Attention.Items[0].WorkspaceID != "ws-home" {
			t.Fatalf("attention = %+v, want mapped items with their owning workspace", decoded.Overview.Attention)
		}
		if decoded.Overview.Today.RunsCompleted != 5 || decoded.Overview.System.RetentionDays != 7 {
			t.Fatalf("payload = %+v, want mapped today and system blocks", decoded.Overview)
		}
	})

	t.Run("Should reject an invalid usage window with 422", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t, testutil.StubSessionManager{}, testutil.StubObserver{}, testutil.StubWorkspaceService{},
		)

		request := httptest.NewRequestWithContext(
			t.Context(), http.MethodGet, "/observe/overview?usage_window=13", http.NoBody,
		)
		response := httptest.NewRecorder()
		fixture.Engine.ServeHTTP(response, request)

		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422; body = %s", response.Code, response.Body.String())
		}
		var payload contract.ErrorPayload
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if !strings.Contains(payload.Error, "usage_window must be 7, 30, or 90") {
			t.Fatalf("error payload = %+v, want usage_window validation message", payload)
		}
	})
}

func TestObserveOverviewAllWorkspaces(t *testing.T) {
	t.Parallel()
	t.Run("Should select every task only with explicit all workspaces", func(t *testing.T) {
		t.Parallel()
		var captured observe.OverviewQuery
		observer := testutil.StubObserver{
			QueryObserveOverviewFn: func(_ context.Context, q observe.OverviewQuery) (observe.OverviewView, error) {
				captured = q
				return observe.OverviewView{}, nil
			},
		}
		fixture := newHandlerFixture(t, testutil.StubSessionManager{}, observer, testutil.StubWorkspaceService{})
		for _, tc := range []struct {
			suffix string
			want   taskpkg.CatalogScope
		}{{"", taskpkg.CatalogScopeGlobal}, {"?all_workspaces=false", taskpkg.CatalogScopeGlobal}, {"?all_workspaces=true", taskpkg.CatalogScopeAll}} {
			response := httptest.NewRecorder()
			fixture.Engine.ServeHTTP(
				response,
				httptest.NewRequestWithContext(t.Context(), "GET", "/observe/overview"+tc.suffix, nil),
			)
			if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "overview") ||
				captured.TaskScope != tc.want {
				t.Fatalf("response=%d %s query=%+v", response.Code, response.Body.String(), captured)
			}
		}
	})
}

func TestPermissionRollupHandler(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve profile and unattended filters in daily totals", func(t *testing.T) {
		t.Parallel()
		observer := testutil.StubObserver{QueryPermissionDailyRollupsFn: func(_ context.Context, q store.PermissionRollupQuery) ([]store.PermissionDailyRollup, error) {
			if !q.Unattended || q.AgentName != "coder" || q.ReadScope.ProfileID != store.DefaultProfileID || q.Since.IsZero() || !q.Until.After(q.Since) {
				t.Fatalf("query = %#v", q)
			}
			return []store.PermissionDailyRollup{{Day: "2026-01-01", AgentName: "coder", Decision: "deny", Count: 3}}, nil
		}}
		fixture := newHandlerFixture(t, testutil.StubSessionManager{}, observer, testutil.StubWorkspaceService{})
		response := httptest.NewRecorder()
		fixture.Engine.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), "GET", "/permissions/rollup?days=7&agent=coder&unattended=true", nil))
		var payload contract.PermissionRollupResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusOK || len(payload.Rollups) != 1 || payload.Rollups[0].Count != 3 {
			t.Fatalf("response = %d %s", response.Code, response.Body.String())
		}
	})
	t.Run("Should list entries under the same scope and filters", func(t *testing.T) {
		t.Parallel()
		observer := testutil.StubObserver{QueryPermissionEntriesFn: func(_ context.Context, q store.PermissionEntriesQuery) ([]store.PermissionLogEntry, error) {
			if !q.Unattended || q.ReadScope.ProfileID != store.DefaultProfileID || q.Limit != 1000 || q.Since.IsZero() {
				t.Fatalf("query = %#v", q)
			}
			return []store.PermissionLogEntry{{ID: "p1", AgentName: "coder", Decision: "allow", InitiatorKind: "daemon"}}, nil
		}}
		fixture := newHandlerFixture(t, testutil.StubSessionManager{}, observer, testutil.StubWorkspaceService{})
		response := httptest.NewRecorder()
		fixture.Engine.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), "GET", "/permissions?days=14&unattended=true", nil))
		var payload contract.PermissionEntriesResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusOK || len(payload.Entries) != 1 || payload.Entries[0].InitiatorKind != "daemon" {
			t.Fatalf("response = %d %s", response.Code, response.Body.String())
		}
	})
	for _, query := range []string{"days=0", "days=91", "days=bad", "unattended=bad"} {
		t.Run("Should reject invalid query "+query, func(t *testing.T) {
			t.Parallel()
			fixture := newHandlerFixture(t, testutil.StubSessionManager{}, testutil.StubObserver{}, testutil.StubWorkspaceService{})
			response := httptest.NewRecorder()
			fixture.Engine.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), "GET", "/permissions/rollup?"+query, nil))
			if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "error") {
				t.Fatalf("response = %d %s", response.Code, response.Body.String())
			}
		})
	}
}
