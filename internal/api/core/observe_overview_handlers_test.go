package core_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/testutil"
	"github.com/compozy/agh/internal/observe"
	taskpkg "github.com/compozy/agh/internal/task"
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
							Kind:       "approval",
							Title:      "Deploy docs site",
							TaskID:     "task-1",
							OccurredAt: time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC),
							Actions:    []string{"approve", "reject", "open"},
						}},
					},
					Today:  observe.OverviewToday{RunsCompleted: 5, RunsFailed: 1, TasksClosed: 3},
					Usage:  observe.OverviewUsage{},
					System: observe.OverviewSystem{RetentionDays: 7},
				}, nil
			},
		}
		fixture := newHandlerFixture(
			t, testutil.StubSessionManager{}, observer, testutil.StubWorkspaceService{}, nil, nil,
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
		if decoded.Overview.Attention.Total != 2 || len(decoded.Overview.Attention.Items) != 1 {
			t.Fatalf("attention = %+v, want mapped items", decoded.Overview.Attention)
		}
		if decoded.Overview.Today.RunsCompleted != 5 || decoded.Overview.System.RetentionDays != 7 {
			t.Fatalf("payload = %+v, want mapped today and system blocks", decoded.Overview)
		}
	})

	t.Run("Should reject an invalid usage window with 422", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t, testutil.StubSessionManager{}, testutil.StubObserver{}, testutil.StubWorkspaceService{}, nil, nil,
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
