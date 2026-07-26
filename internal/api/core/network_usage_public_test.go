package core_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/api/testutil"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func usageWorkspaceService() testutil.StubWorkspaceService {
	return testutil.StubWorkspaceService{
		GetFn: func(_ context.Context, ref string) (workspacepkg.Workspace, error) {
			return workspacepkg.Workspace{ID: "ws-alpha", Name: ref, RootDir: "/workspace"}, nil
		},
	}
}

type stubNetworkUsageStore struct {
	report    store.NetworkUsageReport
	err       error
	lastQuery store.NetworkUsageQuery
}

func (s *stubNetworkUsageStore) GetNetworkUsage(
	_ context.Context,
	query store.NetworkUsageQuery,
) (store.NetworkUsageReport, error) {
	s.lastQuery = query
	if s.err != nil {
		return store.NetworkUsageReport{}, s.err
	}
	return s.report, nil
}

func TestNetworkUsageHandlers(t *testing.T) {
	t.Parallel()

	t.Run("Should return workspace-scoped ledger totals and details", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			usageWorkspaceService(),
			nil,
			nil,
		)
		settled := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
		usage := &stubNetworkUsageStore{
			report: store.NetworkUsageReport{
				Details: []store.NetworkWakeUsageDetail{{
					WakeID:       "wake-1",
					TaskRunID:    "run-1",
					OwnerKey:     "session:sess-1",
					WorkspaceID:  "ws-alpha",
					Channel:      "builders",
					State:        "settled",
					UsageState:   "actual",
					InputTokens:  11,
					OutputTokens: 7,
					SettledAt:    &settled,
				}},
				Total: store.NetworkUsageSummary{
					WakeCount:       1,
					ActualWakeCount: 1,
					InputTokens:     11,
					OutputTokens:    7,
				},
				Budget: &store.NetworkBudgetUsage{
					OwnerKey:         "task_run:run-1",
					WakesUsed:        1,
					WallTimeUsed:     time.Second,
					InputTokensUsed:  11,
					OutputTokensUsed: 7,
					UpdatedAt:        settled,
				},
				NextCursor: "cursor-next",
			},
		}
		fixture.Handlers.NetworkUsage = usage

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/alpha/network/usage?channel=builders&run_id=run-1",
			nil,
		)
		if resp.Code != http.StatusOK {
			t.Fatalf("GET usage status = %d body=%s", resp.Code, resp.Body.String())
		}
		var body contract.NetworkUsageResponse
		if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode usage response: %v", err)
		}
		if body.WorkspaceID != "ws-alpha" {
			t.Fatalf("workspace_id = %q, want ws-alpha", body.WorkspaceID)
		}
		if body.Total.ActualWakeCount != 1 || body.Total.InputTokens != 11 {
			t.Fatalf("total = %#v, want actual wake + tokens", body.Total)
		}
		if len(body.Details) != 1 || body.Details[0].WakeID != "wake-1" {
			t.Fatalf("details = %#v, want wake-1", body.Details)
		}
		if body.Details[0].ParticipationStatus != (participation.Status{
			Owner: participation.OwnerRef{
				WorkspaceID: "ws-alpha",
				Kind:        participation.OwnerKindSession,
				ID:          "sess-1",
			},
			Available:     true,
			Participating: true,
		}) {
			t.Fatalf(
				"details[0].participation_status = %#v, want typed session owner",
				body.Details[0].ParticipationStatus,
			)
		}
		if body.Budget == nil || body.Budget.ParticipationStatus.Owner != (participation.OwnerRef{
			WorkspaceID: "ws-alpha",
			Kind:        participation.OwnerKindTaskRun,
			ID:          "run-1",
		}) || !body.Budget.ParticipationStatus.Available || !body.Budget.ParticipationStatus.Participating ||
			body.Budget.WallTimeUsed != "1s" {
			t.Fatalf("budget = %#v, want run owner consumption", body.Budget)
		}
		if strings.Contains(resp.Body.String(), "owner_key") {
			t.Fatalf("usage response exposed internal owner_key: %s", resp.Body.String())
		}
		if body.NextCursor != "cursor-next" {
			t.Fatalf("next_cursor = %q, want cursor-next", body.NextCursor)
		}
		if usage.lastQuery.WorkspaceID != "ws-alpha" ||
			usage.lastQuery.Channel != "builders" ||
			usage.lastQuery.RunID != "run-1" ||
			usage.lastQuery.Limit != store.DefaultNetworkUsageLimit {
			t.Fatalf("query = %#v, want workspace/channel/run filters", usage.lastQuery)
		}
	})

	t.Run("Should convert store report without inventing totals", func(t *testing.T) {
		t.Parallel()

		report := store.NetworkUsageReport{
			Total: store.NetworkUsageSummary{
				WakeCount:            2,
				ReservedWakeCount:    1,
				ActualWakeCount:      1,
				UnavailableWakeCount: 0,
				InputTokens:          3,
				OutputTokens:         4,
			},
		}
		got, err := core.NetworkUsageResponseFromReport("ws-beta", report)
		if err != nil {
			t.Fatalf("NetworkUsageResponseFromReport() error = %v", err)
		}
		if got.WorkspaceID != "ws-beta" {
			t.Fatalf("workspace_id = %q, want ws-beta", got.WorkspaceID)
		}
		if got.Total.WakeCount != 2 || got.Total.ActualWakeCount != 1 {
			t.Fatalf("total = %#v, want ledger totals", got.Total)
		}
		if len(got.Details) != 0 {
			t.Fatalf("details = %#v, want empty", got.Details)
		}
	})

	t.Run("Should parse owner filters and explicit page bounds", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			usageWorkspaceService(),
			nil,
			nil,
		)
		usage := &stubNetworkUsageStore{}
		fixture.Handlers.NetworkUsage = usage
		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/alpha/network/usage?owner_kind=task_run&owner_id=run-1&limit=200",
			nil,
		)
		if resp.Code != http.StatusOK {
			t.Fatalf("GET owner usage status = %d body=%s", resp.Code, resp.Body.String())
		}
		if usage.lastQuery.Owner == nil || usage.lastQuery.Owner.WorkspaceID != "ws-alpha" ||
			usage.lastQuery.Owner.Kind != "task_run" || usage.lastQuery.Owner.ID != "run-1" ||
			usage.lastQuery.Limit != 200 || !usage.lastQuery.LimitSet {
			t.Fatalf("query = %#v, want workspace-qualified owner and explicit limit", usage.lastQuery)
		}
	})

	for _, testCase := range []struct {
		name     string
		query    string
		wantBody string
	}{
		{name: "explicit zero limit", query: "limit=0", wantBody: "network_usage_limit_invalid"},
		{name: "limit above maximum", query: "limit=201", wantBody: "network_usage_limit_invalid"},
		{name: "malformed cursor", query: "cursor=broken", wantBody: "network_usage_cursor_invalid"},
		{name: "partial owner", query: "owner_kind=task_run", wantBody: "owner id"},
		{name: "owner and run", query: "owner_kind=task_run&owner_id=run-1&run_id=run-1", wantBody: "mutually exclusive"},
		{name: "unknown query field", query: "owner_key=task_run%3Arun-1", wantBody: "unknown_field"},
	} {
		t.Run("Should reject "+testCase.name+" before store execution", func(t *testing.T) {
			t.Parallel()

			fixture := newHandlerFixture(
				t,
				testutil.StubSessionManager{},
				testutil.StubObserver{},
				usageWorkspaceService(),
				nil,
				nil,
			)
			usage := &stubNetworkUsageStore{}
			fixture.Handlers.NetworkUsage = usage
			resp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/workspaces/alpha/network/usage?"+testCase.query,
				nil,
			)
			if resp.Code != http.StatusBadRequest || !strings.Contains(resp.Body.String(), testCase.wantBody) {
				t.Fatalf("GET invalid usage status = %d body=%s", resp.Code, resp.Body.String())
			}
			if usage.lastQuery != (store.NetworkUsageQuery{}) {
				t.Fatalf("store query = %#v, want no execution", usage.lastQuery)
			}
		})
	}

	t.Run("Should reject token counters above the JavaScript-safe ceiling", func(t *testing.T) {
		t.Parallel()

		report := store.NetworkUsageReport{
			Total: store.NetworkUsageSummary{InputTokens: store.MaxJavaScriptSafeInteger},
		}
		if _, err := core.NetworkUsageResponseFromReport("ws-alpha", report); err != nil {
			t.Fatalf("NetworkUsageResponseFromReport(max) error = %v", err)
		}
		report.Total.InputTokens++
		_, err := core.NetworkUsageResponseFromReport("ws-alpha", report)
		if !errors.Is(err, store.ErrNetworkUsageUnsafeInteger) {
			t.Fatalf("NetworkUsageResponseFromReport(max+1) error = %v", err)
		}
	})
}
