package core_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/testutil"
	looppkg "github.com/compozy/compozy/internal/loop"
)

func TestLoopReadHandlersMapping(t *testing.T) {
	t.Parallel()
	service := happyLoopService(t)
	service.getLoopRunNodesFn = func(_ context.Context, workspaceID, runID string, query looppkg.RosterQuery) (contract.LoopRunNodesResponse, error) {
		if workspaceID != "ws-1" || runID != "run-1" || query.State != "all" {
			t.Fatalf("nodes query = %s/%s %#v", workspaceID, runID, query)
		}
		return looppkg.RosterPage{
			RunID: "run-1", LoopName: "alpha",
			Nodes: []looppkg.RosterNode{{
				Generation: 1, NodeID: "build", State: looppkg.NodeStateRunning,
			}},
		}, nil
	}
	service.getLoopBriefingFn = func(context.Context, string, string) (contract.LoopBriefingResponse, error) {
		return looppkg.Briefing{
			RunID: "run-1", Tone: looppkg.BriefingToneOK, Headline: "Running",
			Progress: looppkg.StepProgress{Round: 1, StepsTotal: 1},
		}, nil
	}
	service.getLoopTimelineFn = func(_ context.Context, _ string, runID string, query looppkg.TimelineQuery) (contract.LoopTimelineResponse, error) {
		if query.Cursor == "foreign" {
			return contract.LoopTimelineResponse{}, looppkg.ErrTimelineBranchChanged
		}
		if query.AfterSeq > 2 {
			return contract.LoopTimelineResponse{}, &looppkg.TimelinePositionError{
				Position: query.AfterSeq, Head: 2,
			}
		}
		return looppkg.TimelinePage{
			RunID: looppkg.RunID(runID), HeadSeq: 2,
			Entries: []looppkg.TimelineEntry{{
				Seq: 2, Kind: looppkg.RunEventNodeRunning, Title: "node running",
			}},
		}, nil
	}
	_, engine := newLoopHandlerFixture(t, "httpapi", service)
	t.Run("Should map served roster and briefing responses", func(t *testing.T) {
		t.Parallel()
		nodesResponse := performRequest(
			t,
			engine,
			http.MethodGet,
			"/workspaces/ws-1/loop-runs/run-1/nodes?state=all",
			nil,
		)
		assertLoopStatus(t, nodesResponse.Code, http.StatusOK, nodesResponse.Body.String())
		var nodes contract.LoopRunNodesResponse
		testutil.DecodeJSONResponse(t, nodesResponse, &nodes)
		if len(nodes.Nodes) != 1 || nodes.Nodes[0].State != looppkg.NodeStateRunning {
			t.Fatalf("nodes = %#v", nodes)
		}
		briefingResponse := performRequest(
			t,
			engine,
			http.MethodGet,
			"/workspaces/ws-1/loop-runs/run-1/briefing",
			nil,
		)
		assertLoopStatus(t, briefingResponse.Code, http.StatusOK, briefingResponse.Body.String())
		var briefing contract.LoopBriefingResponse
		testutil.DecodeJSONResponse(t, briefingResponse, &briefing)
		if briefing.RunID != nodes.RunID || briefing.Progress.StepsTotal != 1 {
			t.Fatalf("briefing = %#v", briefing)
		}
	})
	t.Run("Should map a durable timeline response", func(t *testing.T) {
		t.Parallel()
		response := performRequest(t, engine, http.MethodGet, "/workspaces/ws-1/loop-runs/run-1/timeline?view=all", nil)
		assertLoopStatus(t, response.Code, http.StatusOK, response.Body.String())
		var page contract.LoopTimelineResponse
		testutil.DecodeJSONResponse(t, response, &page)
		if page.HeadSeq != 2 || len(page.Entries) != 1 {
			t.Fatalf("timeline = %#v", page)
		}
	})
	t.Run("Should map a plain sequence resume error", func(t *testing.T) {
		t.Parallel()
		response := performRequest(
			t,
			engine,
			http.MethodGet,
			"/workspaces/ws-1/loop-runs/run-1/timeline?view=all&after_sequence=3",
			nil,
		)
		assertLoopStatus(t, response.Code, http.StatusBadRequest, response.Body.String())
		var body contract.ErrorPayload
		testutil.DecodeJSONResponse(t, response, &body)
		if body.Error != "position 3 is beyond this run's history (head: 2)" ||
			body.Code != "timeline_position_beyond_head" ||
			body.Details["position"] != "3" ||
			body.Details["head_seq"] != "2" {
			t.Fatalf("body = %#v", body)
		}
	})
	t.Run("Should map the branch-changed conflict body", func(t *testing.T) {
		t.Parallel()
		response := performRequest(
			t,
			engine,
			http.MethodGet,
			"/workspaces/ws-1/loop-runs/run-1/timeline?cursor=foreign",
			nil,
		)
		assertLoopStatus(t, response.Code, http.StatusConflict, response.Body.String())
		var body map[string]string
		testutil.DecodeJSONResponse(t, response, &body)
		if body["error"] != "timeline_branch_changed" || body["code"] != "timeline_branch_changed" {
			t.Fatalf("body = %#v", body)
		}
	})
}

func TestLoopReadHandlersShouldMapMalformedCursorsExactly(t *testing.T) {
	t.Parallel()

	t.Run("Should map a malformed roster cursor to invalid_cursor", func(t *testing.T) {
		t.Parallel()
		service := happyLoopService(t)
		service.getLoopRunNodesFn = func(
			context.Context,
			string,
			string,
			looppkg.RosterQuery,
		) (contract.LoopRunNodesResponse, error) {
			return contract.LoopRunNodesResponse{}, looppkg.ErrInvalidRosterCursor
		}
		_, engine := newLoopHandlerFixture(t, "httpapi", service)
		response := performRequest(
			t,
			engine,
			http.MethodGet,
			"/workspaces/ws-1/loop-runs/run-1/nodes?cursor=malformed",
			nil,
		)
		assertLoopStatus(t, response.Code, http.StatusBadRequest, response.Body.String())
		var body map[string]string
		testutil.DecodeJSONResponse(t, response, &body)
		if body["error"] != "invalid_cursor" || body["code"] != "invalid_cursor" {
			t.Fatalf("body = %#v", body)
		}
	})

	t.Run("Should map an invalid run-list cursor to invalid_cursor", func(t *testing.T) {
		t.Parallel()
		service := happyLoopService(t)
		service.listLoopRunsFn = func(
			context.Context,
			string,
			core.LoopRunListQuery,
		) (contract.LoopRunsResponse, error) {
			return contract.LoopRunsResponse{}, looppkg.ErrInvalidRunListCursor
		}
		_, engine := newLoopHandlerFixture(t, "httpapi", service)
		response := performRequest(
			t,
			engine,
			http.MethodGet,
			"/workspaces/ws-1/loop-runs?cursor=malformed",
			nil,
		)
		assertLoopStatus(t, response.Code, http.StatusBadRequest, response.Body.String())
		var body map[string]string
		testutil.DecodeJSONResponse(t, response, &body)
		if body["error"] != "invalid_cursor" {
			t.Fatalf("body = %#v", body)
		}
	})
}

func TestLoopReadHandlersShouldMapInvalidRosterStateDetails(t *testing.T) {
	t.Parallel()
	t.Run("Should return the shared code and details envelope", func(t *testing.T) {
		t.Parallel()
		service := happyLoopService(t)
		service.getLoopRunNodesFn = func(
			context.Context,
			string,
			string,
			looppkg.RosterQuery,
		) (contract.LoopRunNodesResponse, error) {
			return contract.LoopRunNodesResponse{}, &looppkg.InvalidNodeStateError{
				State: "unknown", Allowed: []string{"all", "running"},
			}
		}
		_, engine := newLoopHandlerFixture(t, "httpapi", service)
		response := performRequest(
			t, engine, http.MethodGet, "/workspaces/ws-1/loop-runs/run-1/nodes?state=unknown", nil,
		)
		assertLoopStatus(t, response.Code, http.StatusBadRequest, response.Body.String())
		var body contract.ErrorPayload
		testutil.DecodeJSONResponse(t, response, &body)
		if body.Error != "invalid_node_state" || body.Code != "invalid_node_state" ||
			body.Details["allowed"] != "all,running" {
			t.Fatalf("body = %#v", body)
		}
	})
}
