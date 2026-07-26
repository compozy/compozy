package core_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/api/testutil"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/gin-gonic/gin"
)

func TestLoopHandlersExposeCatalogRunConfigAnnotationsAndEvents(t *testing.T) {
	t.Parallel()

	t.Run("Should assert status and body for every Loop endpoint", func(t *testing.T) {
		t.Parallel()

		service := happyLoopService(t)
		service.runLoopFn = func(
			_ context.Context,
			workspaceID string,
			name string,
			req contract.RunLoopRequest,
			startKind dsl.StartKind,
			actor taskpkg.ActorContext,
			dry bool,
		) (contract.RunLoopResponse, error) {
			if workspaceID != "ws-1" || name != "alpha" {
				t.Fatalf("RunLoop() target = %s/%s", workspaceID, name)
			}
			if startKind != dsl.StartHTTP {
				t.Fatalf("RunLoop() startKind = %q, want %q", startKind, dsl.StartHTTP)
			}
			if actor.Origin.Kind != taskpkg.OriginKindHTTP {
				t.Fatalf("RunLoop() actor origin = %q, want %q", actor.Origin.Kind, taskpkg.OriginKindHTTP)
			}
			if got := req.Inputs["ticket"]; got != "AGH-14" {
				t.Fatalf("RunLoop() input ticket = %#v, want AGH-14", got)
			}
			if req.NetworkParticipation == nil ||
				req.NetworkParticipation.Mode == nil ||
				*req.NetworkParticipation.Mode != participation.ModeLocal {
				t.Fatalf("RunLoop() network participation = %#v, want local request", req.NetworkParticipation)
			}
			if dry {
				return contract.RunLoopResponse{DryRun: &contract.LoopPlanPayload{
					LoopName:       "alpha",
					ResolvedInputs: map[string]any{"ticket": "AGH-14"},
					Generation:     1,
				}}, nil
			}
			return contract.RunLoopResponse{Run: loopRunPayload("run-1", looppkg.StatusRunning)}, nil
		}
		service.listLoopRunEventsFn = func(
			_ context.Context,
			workspaceID string,
			runID string,
			afterSeq int64,
		) ([]contract.LoopRunEventPayload, error) {
			if workspaceID != "ws-1" || runID != "run-1" || afterSeq != 3 {
				t.Fatalf("ListLoopRunEvents() = workspace=%s run=%s after=%d", workspaceID, runID, afterSeq)
			}
			return []contract.LoopRunEventPayload{{
				ID:          "evt-4",
				LoopRunID:   "run-1",
				WorkspaceID: "ws-1",
				Seq:         4,
				Kind:        "status_changed",
				Payload:     []byte(`{"status":"running"}`),
				At:          fixedLoopTime(),
			}}, nil
		}
		handlers, engine := newLoopHandlerFixture(t, "httpapi", service)
		done := make(chan struct{})
		close(done)
		handlers.SetStreamDone(done)

		listResp := performRequest(t, engine, http.MethodGet, "/workspaces/ws-1/loops", nil)
		assertLoopStatus(t, listResp.Code, http.StatusOK, listResp.Body.String())
		var listPayload contract.LoopsResponse
		testutil.DecodeJSONResponse(t, listResp, &listPayload)
		if got := listPayload.Loops[0].Aggregate30d.Runs; got != 2 {
			t.Fatalf("GET /loops aggregate runs = %d, want 2", got)
		}
		lastRun := listPayload.Loops[0].LastRun
		if lastRun == nil || lastRun.ID != "run-catalog-latest" ||
			lastRun.Status != contract.LoopRunStatusDone ||
			!lastRun.CreatedAt.Equal(time.Date(2026, 7, 10, 11, 0, 0, 0, time.UTC)) {
			t.Fatalf("GET /loops last_run = %#v, want lean run payload", lastRun)
		}

		createResp := performRequest(t, engine, http.MethodPost, "/workspaces/ws-1/loops", loopDefinitionRequestBody(t))
		assertLoopStatus(t, createResp.Code, http.StatusCreated, createResp.Body.String())
		var createPayload contract.LoopResponse
		testutil.DecodeJSONResponse(t, createResp, &createPayload)
		if createPayload.Loop.Name != "alpha" || createPayload.Loop.Version != 7 {
			t.Fatalf("POST /loops payload = %#v", createPayload.Loop)
		}

		getResp := performRequest(t, engine, http.MethodGet, "/workspaces/ws-1/loops/alpha", nil)
		assertLoopStatus(t, getResp.Code, http.StatusOK, getResp.Body.String())
		var getPayload contract.LoopResponse
		testutil.DecodeJSONResponse(t, getResp, &getPayload)
		if getPayload.Loop.Definition.Meta.Name != "alpha" {
			t.Fatalf("GET /loops/:name definition name = %q, want alpha", getPayload.Loop.Definition.Meta.Name)
		}

		patchResp := performRequest(
			t,
			engine,
			http.MethodPatch,
			"/workspaces/ws-1/loops/alpha",
			loopPatchRequestBody(t, 7),
		)
		assertLoopStatus(t, patchResp.Code, http.StatusOK, patchResp.Body.String())
		var patchPayload contract.LoopResponse
		testutil.DecodeJSONResponse(t, patchResp, &patchPayload)
		if patchPayload.Loop.Version != 7 {
			t.Fatalf("PATCH /loops/:name version = %d, want 7", patchPayload.Loop.Version)
		}

		validateResp := performRequest(
			t,
			engine,
			http.MethodPost,
			"/workspaces/ws-1/loops/alpha/validate",
			testutil.MustJSONBody(t, contract.ValidateLoopRequest{Definition: loopDefinitionDocument(t)}),
		)
		assertLoopStatus(t, validateResp.Code, http.StatusOK, validateResp.Body.String())
		var validationPayload contract.LoopValidationResponse
		testutil.DecodeJSONResponse(t, validateResp, &validationPayload)
		if !validationPayload.Valid || len(validationPayload.Errors) != 0 {
			t.Fatalf("POST /validate payload = %#v", validationPayload)
		}

		runResp := performRequest(
			t,
			engine,
			http.MethodPost,
			"/workspaces/ws-1/loops/alpha/run",
			[]byte(`{"inputs":{"ticket":"AGH-14"},"network_participation":{"mode":"local"}}`),
		)
		assertLoopStatus(t, runResp.Code, http.StatusCreated, runResp.Body.String())
		var runPayload contract.RunLoopResponse
		testutil.DecodeJSONResponse(t, runResp, &runPayload)
		if runPayload.Run == nil || runPayload.Run.ID != "run-1" {
			t.Fatalf("POST /run payload = %#v", runPayload)
		}

		dryResp := performRequest(
			t,
			engine,
			http.MethodPost,
			"/workspaces/ws-1/loops/alpha/run?dry=true",
			[]byte(`{"inputs":{"ticket":"AGH-14"},"network_participation":{"mode":"local"}}`),
		)
		assertLoopStatus(t, dryResp.Code, http.StatusOK, dryResp.Body.String())
		var dryPayload contract.RunLoopResponse
		testutil.DecodeJSONResponse(t, dryResp, &dryPayload)
		if dryPayload.DryRun == nil || dryPayload.DryRun.LoopName != "alpha" {
			t.Fatalf("POST /run?dry=true payload = %#v", dryPayload)
		}

		configResp := performRequest(t, engine, http.MethodGet, "/workspaces/ws-1/loops/alpha/config", nil)
		assertLoopStatus(t, configResp.Code, http.StatusOK, configResp.Body.String())
		var configPayload contract.LoopConfigResponse
		testutil.DecodeJSONResponse(t, configResp, &configPayload)
		if configPayload.Config == nil || configPayload.Config.IterationCap == nil ||
			*configPayload.Config.IterationCap != 4 {
			t.Fatalf("GET /config payload = %#v", configPayload)
		}
		if configPayload.EffectiveConfig.FanOutWidth != 4 ||
			configPayload.EffectiveConfig.GateMaxRevisions != 10 {
			t.Fatalf("GET /config effective config = %#v", configPayload.EffectiveConfig)
		}

		putConfigResp := performRequest(
			t,
			engine,
			http.MethodPut,
			"/workspaces/ws-1/loops/alpha/config",
			testutil.MustJSONBody(t, contract.PutLoopConfigRequest{Config: loopConfig()}),
		)
		assertLoopStatus(t, putConfigResp.Code, http.StatusOK, putConfigResp.Body.String())
		var putConfigPayload contract.LoopConfigResponse
		testutil.DecodeJSONResponse(t, putConfigResp, &putConfigPayload)
		if putConfigPayload.Config == nil || putConfigPayload.Config.ReattemptStrategy == nil {
			t.Fatalf("PUT /config payload = %#v", putConfigPayload)
		}

		annotationsResp := performRequest(t, engine, http.MethodGet, "/workspaces/ws-1/loops/alpha/annotations", nil)
		assertLoopStatus(t, annotationsResp.Code, http.StatusOK, annotationsResp.Body.String())
		var annotationsPayload contract.LoopAnnotationsResponse
		testutil.DecodeJSONResponse(t, annotationsResp, &annotationsPayload)
		if annotationsPayload.Annotations[0].NodeID != "draft" || annotationsPayload.Annotations[0].X != 12 {
			t.Fatalf("GET /annotations payload = %#v", annotationsPayload)
		}

		putAnnotationsResp := performRequest(
			t,
			engine,
			http.MethodPut,
			"/workspaces/ws-1/loops/alpha/annotations",
			testutil.MustJSONBody(t, contract.PutLoopAnnotationsRequest(annotationsPayload)),
		)
		assertLoopStatus(t, putAnnotationsResp.Code, http.StatusOK, putAnnotationsResp.Body.String())
		var putAnnotationsPayload contract.LoopAnnotationsResponse
		testutil.DecodeJSONResponse(t, putAnnotationsResp, &putAnnotationsPayload)
		if len(putAnnotationsPayload.Annotations) != 1 {
			t.Fatalf("PUT /annotations payload = %#v", putAnnotationsPayload)
		}

		runsResp := performRequest(
			t,
			engine,
			http.MethodGet,
			"/workspaces/ws-1/loop-runs?loop=alpha&status=running&origin=session&origin_session=session-1&live=true&limit=10",
			nil,
		)
		assertLoopStatus(t, runsResp.Code, http.StatusOK, runsResp.Body.String())
		var runsPayload contract.LoopRunsResponse
		testutil.DecodeJSONResponse(t, runsResp, &runsPayload)
		if runsPayload.Aggregates.Live != 1 || runsPayload.Runs[0].LoopName != "alpha" {
			t.Fatalf("GET /loop-runs payload = %#v", runsPayload)
		}

		runDetailResp := performRequest(t, engine, http.MethodGet, "/workspaces/ws-1/loop-runs/run-1", nil)
		assertLoopStatus(t, runDetailResp.Code, http.StatusOK, runDetailResp.Body.String())
		var runDetailPayload contract.LoopRunResponse
		testutil.DecodeJSONResponse(t, runDetailResp, &runDetailPayload)
		if runDetailPayload.Run.ID != "run-1" || len(runDetailPayload.Generations) != 1 {
			t.Fatalf("GET /loop-runs/:run_id payload = %#v", runDetailPayload)
		}

		assertLoopOKMutation(t, engine, http.MethodPost, "/workspaces/ws-1/loop-runs/run-1/stop", nil)
		assertLoopOKMutation(t, engine, http.MethodPost, "/workspaces/ws-1/loop-runs/run-1/pause", nil)
		assertLoopOKMutation(t, engine, http.MethodPost, "/workspaces/ws-1/loop-runs/run-1/resume", nil)
		assertLoopOKMutation(
			t,
			engine,
			http.MethodPost,
			"/workspaces/ws-1/loop-runs/run-1/approve",
			[]byte(`{"gate_id":"review","decision":"approve"}`),
		)

		streamResp := testutil.PerformRequestWithHeaders(
			t,
			engine,
			http.MethodGet,
			"/workspaces/ws-1/loop-runs/run-1/events?after_sequence=3",
			nil,
			map[string]string{"Last-Event-ID": "1"},
		)
		assertLoopStatus(t, streamResp.Code, http.StatusOK, streamResp.Body.String())
		if contentType := streamResp.Header().Get("Content-Type"); !strings.Contains(contentType, "text/event-stream") {
			t.Fatalf("GET /events content-type = %q, want text/event-stream", contentType)
		}
		records := testutil.ParseSSE(t, streamResp.Body.String())
		if len(records) != 1 || records[0].ID != "4" || records[0].Event != "status_changed" {
			t.Fatalf("GET /events SSE records = %#v", records)
		}
		var eventPayload contract.LoopRunEventPayload
		testutil.DecodeSSEData(t, records[0], &eventPayload)
		if eventPayload.Kind != "status_changed" || eventPayload.Seq != 4 || eventPayload.WorkspaceID != "ws-1" {
			t.Fatalf("GET /events payload = %#v", eventPayload)
		}

		deleteResp := performRequest(t, engine, http.MethodDelete, "/workspaces/ws-1/loops/alpha", nil)
		assertLoopStatus(t, deleteResp.Code, http.StatusNoContent, deleteResp.Body.String())
		if deleteResp.Body.Len() != 0 {
			t.Fatalf("DELETE /loops/:name body = %q, want empty", deleteResp.Body.String())
		}
	})

	t.Run("Should reject removed Loop run channel fields", func(t *testing.T) {
		t.Parallel()

		service := happyLoopService(t)
		service.runLoopFn = func(
			context.Context,
			string,
			string,
			contract.RunLoopRequest,
			dsl.StartKind,
			taskpkg.ActorContext,
			bool,
		) (contract.RunLoopResponse, error) {
			t.Fatal("RunLoop() should not be called for a removed channel field")
			return contract.RunLoopResponse{}, nil
		}
		_, engine := newLoopHandlerFixture(t, "httpapi", service)
		resp := performRequest(
			t,
			engine,
			http.MethodPost,
			"/workspaces/ws-1/loops/alpha/run",
			[]byte(`{"channel":"legacy"}`),
		)
		assertLoopStatus(t, resp.Code, http.StatusBadRequest, resp.Body.String())
		if body := resp.Body.String(); !strings.Contains(body, "unknown_field") || !strings.Contains(body, "channel") {
			t.Fatalf("POST /run legacy body = %s, want unknown_field with removed field name", body)
		}
	})
}

func TestLoopCatalogHandlerShouldForwardBoundedQuery(t *testing.T) {
	t.Parallel()

	t.Run("Should forward every catalog filter and preserve explicit empty page metadata", func(t *testing.T) {
		t.Parallel()

		service := happyLoopService(t)
		service.listLoopsFn = func(
			_ context.Context,
			workspaceID string,
			query looppkg.CatalogQuery,
		) (contract.LoopsResponse, error) {
			if workspaceID != "ws-1" || query.Search != "deploy safely" ||
				query.Kind != looppkg.CatalogKindWorkspace || query.Category != "delivery" ||
				query.Status != looppkg.StatusRunning || query.Sort != looppkg.CatalogSortName ||
				query.Cursor != "cursor-1" || query.Limit != 7 {
				t.Fatalf("ListLoops() query = workspace %q %#v", workspaceID, query)
			}
			return contract.LoopsResponse{
				Loops: []contract.LoopCatalogEntryPayload{},
				Facets: contract.LoopCatalogFacetsPayload{
					Kinds:      map[string]int{},
					Categories: map[string]int{},
					Statuses:   map[string]int{},
				},
				Page: contract.CountedCursorPagePayload{Total: 0, Limit: 7},
			}, nil
		}
		_, engine := newLoopHandlerFixture(t, "httpapi", service)
		resp := performRequest(
			t,
			engine,
			http.MethodGet,
			"/workspaces/ws-1/loops?q=deploy%20safely&kind=workspace&category=delivery&"+
				"status=running&sort=name&cursor=cursor-1&limit=7",
			nil,
		)
		assertLoopStatus(t, resp.Code, http.StatusOK, resp.Body.String())
		var payload contract.LoopsResponse
		testutil.DecodeJSONResponse(t, resp, &payload)
		if payload.Page.Total != 0 || payload.Page.Limit != 7 || payload.Loops == nil {
			t.Fatalf("GET /loops payload = %#v, want explicit empty counted page", payload)
		}
	})

	t.Run("Should reject a malformed limit before calling the service", func(t *testing.T) {
		t.Parallel()

		service := happyLoopService(t)
		service.listLoopsFn = func(
			context.Context,
			string,
			looppkg.CatalogQuery,
		) (contract.LoopsResponse, error) {
			t.Fatal("ListLoops() should not be called for malformed limit")
			return contract.LoopsResponse{}, nil
		}
		_, engine := newLoopHandlerFixture(t, "udsapi", service)
		resp := performRequest(t, engine, http.MethodGet, "/workspaces/ws-1/loops?limit=not-a-number", nil)
		assertLoopStatus(t, resp.Code, http.StatusBadRequest, resp.Body.String())
		var payload contract.ErrorPayload
		testutil.DecodeJSONResponse(t, resp, &payload)
		assertLoopErrorPayloadContains(t, payload, looppkg.ErrCatalogQueryInvalid.Error())
	})
}

func TestLoopHandlersExposeUDSStartKind(t *testing.T) {
	t.Parallel()

	t.Run("Should pass UDS start kind through the shared run handler", func(t *testing.T) {
		t.Parallel()

		service := happyLoopService(t)
		service.runLoopFn = func(
			_ context.Context,
			_ string,
			_ string,
			_ contract.RunLoopRequest,
			startKind dsl.StartKind,
			actor taskpkg.ActorContext,
			dry bool,
		) (contract.RunLoopResponse, error) {
			if startKind != dsl.StartUDS {
				t.Fatalf("RunLoop() startKind = %q, want %q", startKind, dsl.StartUDS)
			}
			if actor.Origin.Kind != taskpkg.OriginKindUDS {
				t.Fatalf("RunLoop() actor origin = %q, want %q", actor.Origin.Kind, taskpkg.OriginKindUDS)
			}
			if dry {
				t.Fatal("RunLoop() dry = true, want false")
			}
			return contract.RunLoopResponse{Run: loopRunPayload("run-uds", looppkg.StatusRunning)}, nil
		}
		_, engine := newLoopHandlerFixture(t, "udsapi", service)

		resp := performRequest(t, engine, http.MethodPost, "/workspaces/ws-1/loops/alpha/run", []byte(`{}`))
		assertLoopStatus(t, resp.Code, http.StatusCreated, resp.Body.String())
		var payload contract.RunLoopResponse
		testutil.DecodeJSONResponse(t, resp, &payload)
		if payload.Run == nil || payload.Run.ID != "run-uds" {
			t.Fatalf("POST /run UDS payload = %#v", payload)
		}
	})
}

func TestGoalReadHandlersExposeSnapshotAndTurnContracts(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve snapshot and open-turn nullability", func(t *testing.T) {
		t.Parallel()

		service := happyLoopService(t)
		service.getSessionGoalFn = func(
			_ context.Context,
			workspaceID string,
			sessionID string,
		) (*session.GoalSnapshot, error) {
			if workspaceID != "ws-1" || sessionID != "sess-1" {
				t.Fatalf("GetSessionGoal() target = %s/%s", workspaceID, sessionID)
			}
			return &session.GoalSnapshot{
				RunID: "run-1", NodeID: "converge", Objective: "Ship the change",
				OriginSessionID: "sess-1", BoundSessionID: "sess-bound",
				Status: "active", RunStatus: "running", TurnsUsed: 1, TurnLimit: 3, Live: true,
				ContractSummary: "All checks pass",
				Context:         session.GoalContextSnapshot{State: "pending", NudgeRatio: 0},
			}, nil
		}
		service.listGoalTurnsFn = func(
			_ context.Context,
			workspaceID string,
			runID string,
			query core.GoalTurnListQuery,
		) (session.GoalTurnPage, error) {
			if workspaceID != "ws-1" || runID != "run-1" || query.NodeID != "converge" ||
				query.ItemIndex == nil || *query.ItemIndex != 0 || query.AfterSeq != 4 || query.Limit != 2 {
				t.Fatalf("ListGoalTurns() query = %s/%s %#v", workspaceID, runID, query)
			}
			next := int64(5)
			return session.GoalTurnPage{
				Turns: []session.GoalTurn{{
					Seq: 5, Generation: 2, NodeID: "converge", ItemIndex: 0, Turn: 2,
					PromptAttempt: 1, SessionID: "sess-bound", BindingHandle: "goal:hash",
					BindingEpoch: 2, PromptID: "prompt-5", BlockingIssues: []session.GoalBlockingIssue{},
					ActorKind: "agent", ActorID: "goal:run-1", StartedAt: fixedLoopTime(),
				}},
				NextAfterSeq: &next,
			}, nil
		}
		_, engine := newLoopHandlerFixture(t, "httpapi", service)

		snapshotResponse := performRequest(
			t,
			engine,
			http.MethodGet,
			"/workspaces/ws-1/sessions/sess-1/goal",
			nil,
		)
		assertLoopStatus(t, snapshotResponse.Code, http.StatusOK, snapshotResponse.Body.String())
		var snapshot contract.SessionGoalResponse
		testutil.DecodeJSONResponse(t, snapshotResponse, &snapshot)
		if snapshot.Goal == nil || snapshot.Goal.RunID != "run-1" || snapshot.Goal.Context.NudgeRatio != 0 {
			t.Fatalf("GET session Goal = %#v", snapshot)
		}
		for _, field := range []string{`"used":null`, `"size":null`, `"ratio":null`, `"reported_at":null`} {
			if !strings.Contains(snapshotResponse.Body.String(), field) {
				t.Fatalf("GET session Goal body missing %s: %s", field, snapshotResponse.Body.String())
			}
		}

		turnsResponse := performRequest(
			t,
			engine,
			http.MethodGet,
			"/workspaces/ws-1/loop-runs/run-1/turns?node=converge&item=0&after_seq=4&limit=2",
			nil,
		)
		assertLoopStatus(t, turnsResponse.Code, http.StatusOK, turnsResponse.Body.String())
		var page contract.GoalTurnPage
		testutil.DecodeJSONResponse(t, turnsResponse, &page)
		if len(page.Turns) != 1 || page.Turns[0].ResultStatus != nil || page.NextAfterSeq == nil ||
			*page.NextAfterSeq != 5 || page.Turns[0].BlockingIssues == nil {
			t.Fatalf("GET Goal turns = %#v", page)
		}
	})

	t.Run("Should return null after clear and reject invalid turn filters", func(t *testing.T) {
		t.Parallel()

		service := happyLoopService(t)
		_, engine := newLoopHandlerFixture(t, "udsapi", service)
		nullResponse := performRequest(
			t,
			engine,
			http.MethodGet,
			"/workspaces/ws-1/sessions/sess-1/goal",
			nil,
		)
		assertLoopStatus(t, nullResponse.Code, http.StatusOK, nullResponse.Body.String())
		if strings.TrimSpace(nullResponse.Body.String()) != `{"goal":null}` {
			t.Fatalf("GET cleared Goal body = %q", nullResponse.Body.String())
		}

		for _, path := range []string{
			"/workspaces/ws-1/loop-runs/run-1/turns?limit=201",
			"/workspaces/ws-1/loop-runs/run-1/turns?after_seq=-1",
			"/workspaces/ws-1/loop-runs/run-1/turns?item=0",
		} {
			response := performRequest(t, engine, http.MethodGet, path, nil)
			assertLoopStatus(t, response.Code, http.StatusUnprocessableEntity, response.Body.String())
			var result contract.GoalCommandResult
			testutil.DecodeJSONResponse(t, response, &result)
			if result.Outcome != contract.GoalOutcomeError || result.ReasonCode == nil ||
				*result.ReasonCode != contract.GoalReasonTurnFilterInvalid {
				t.Fatalf("GET Goal turns invalid result = %#v", result)
			}
		}
	})
}

func TestLoopHandlersExposeValidationAndConflictBodies(t *testing.T) {
	t.Parallel()

	t.Run("Should return per-node 422 bodies from PATCH and validate", func(t *testing.T) {
		t.Parallel()

		diagnostic := contract.LoopLintErrorPayload{
			NodeID:   "draft",
			Code:     "unknown_reference",
			Message:  "nodes.missing.output is not declared",
			Severity: contract.LoopLintSeverityError,
		}
		service := happyLoopService(t)
		service.patchLoopFn = func(
			context.Context,
			string,
			string,
			contract.PatchLoopRequest,
		) (contract.LoopResponse, error) {
			return contract.LoopResponse{}, &core.LoopLintFailedError{
				Errors: []contract.LoopLintErrorPayload{diagnostic},
			}
		}
		service.validateLoopFn = func(
			context.Context,
			string,
			string,
			contract.ValidateLoopRequest,
		) (contract.LoopValidationResponse, error) {
			return contract.LoopValidationResponse{}, &core.LoopLintFailedError{
				Errors: []contract.LoopLintErrorPayload{diagnostic},
			}
		}
		_, engine := newLoopHandlerFixture(t, "httpapi", service)

		patchResp := performRequest(
			t,
			engine,
			http.MethodPatch,
			"/workspaces/ws-1/loops/alpha",
			loopPatchRequestBody(t, 7),
		)
		assertLoopStatus(t, patchResp.Code, http.StatusUnprocessableEntity, patchResp.Body.String())
		var patchPayload contract.LoopValidationResponse
		testutil.DecodeJSONResponse(t, patchResp, &patchPayload)
		assertLoopValidationDiagnostic(t, patchPayload, diagnostic)

		validateResp := performRequest(
			t,
			engine,
			http.MethodPost,
			"/workspaces/ws-1/loops/alpha/validate",
			testutil.MustJSONBody(t, contract.ValidateLoopRequest{Definition: loopDefinitionDocument(t)}),
		)
		assertLoopStatus(t, validateResp.Code, http.StatusUnprocessableEntity, validateResp.Body.String())
		var validatePayload contract.LoopValidationResponse
		testutil.DecodeJSONResponse(t, validateResp, &validatePayload)
		assertLoopValidationDiagnostic(t, validatePayload, diagnostic)
	})

	t.Run("Should return a bad request error payload for malformed Loop JSON", func(t *testing.T) {
		t.Parallel()

		_, engine := newLoopHandlerFixture(t, "httpapi", happyLoopService(t))

		resp := performRequest(t, engine, http.MethodPatch, "/workspaces/ws-1/loops/alpha", []byte(`{`))
		assertLoopStatus(t, resp.Code, http.StatusBadRequest, resp.Body.String())
		var payload contract.ErrorPayload
		testutil.DecodeJSONResponse(t, resp, &payload)
		if !strings.Contains(payload.Error, "decode patch loop request") {
			t.Fatalf("PATCH malformed payload error = %q, want decode context", payload.Error)
		}
	})

	t.Run("Should return a not found error payload for missing Loop definitions", func(t *testing.T) {
		t.Parallel()

		service := happyLoopService(t)
		service.getLoopFn = func(context.Context, string, string) (contract.LoopResponse, error) {
			return contract.LoopResponse{}, looppkg.ErrDefinitionNotFound
		}
		_, engine := newLoopHandlerFixture(t, "httpapi", service)

		resp := performRequest(t, engine, http.MethodGet, "/workspaces/ws-1/loops/missing", nil)
		assertLoopStatus(t, resp.Code, http.StatusNotFound, resp.Body.String())
		var payload contract.ErrorPayload
		testutil.DecodeJSONResponse(t, resp, &payload)
		assertLoopErrorPayloadContains(t, payload, looppkg.ErrDefinitionNotFound.Error())
	})

	t.Run("Should return a not found error payload for missing PATCH targets", func(t *testing.T) {
		t.Parallel()

		service := happyLoopService(t)
		service.patchLoopFn = func(
			context.Context,
			string,
			string,
			contract.PatchLoopRequest,
		) (contract.LoopResponse, error) {
			return contract.LoopResponse{}, looppkg.ErrDefinitionNotFound
		}
		_, engine := newLoopHandlerFixture(t, "httpapi", service)

		resp := performRequest(
			t,
			engine,
			http.MethodPatch,
			"/workspaces/ws-1/loops/missing",
			loopPatchRequestBody(t, 7),
		)
		assertLoopStatus(t, resp.Code, http.StatusNotFound, resp.Body.String())
		var payload contract.ErrorPayload
		testutil.DecodeJSONResponse(t, resp, &payload)
		assertLoopErrorPayloadContains(t, payload, looppkg.ErrDefinitionNotFound.Error())
	})

	t.Run("Should return a conflict error payload for duplicate Loop creates", func(t *testing.T) {
		t.Parallel()

		service := happyLoopService(t)
		service.createLoopFn = func(context.Context, string, contract.CreateLoopRequest) (contract.LoopResponse, error) {
			return contract.LoopResponse{}, looppkg.ErrDefinitionExists
		}
		_, engine := newLoopHandlerFixture(t, "httpapi", service)

		resp := performRequest(t, engine, http.MethodPost, "/workspaces/ws-1/loops", loopDefinitionRequestBody(t))
		assertLoopStatus(t, resp.Code, http.StatusConflict, resp.Body.String())
		var payload contract.ErrorPayload
		testutil.DecodeJSONResponse(t, resp, &payload)
		assertLoopErrorPayloadContains(t, payload, looppkg.ErrDefinitionExists.Error())
	})

	t.Run("Should return a not found error payload for missing fork sources", func(t *testing.T) {
		t.Parallel()

		service := happyLoopService(t)
		service.createLoopFn = func(context.Context, string, contract.CreateLoopRequest) (contract.LoopResponse, error) {
			return contract.LoopResponse{}, looppkg.ErrDefinitionNotFound
		}
		_, engine := newLoopHandlerFixture(t, "httpapi", service)

		resp := performRequest(
			t,
			engine,
			http.MethodPost,
			"/workspaces/ws-1/loops",
			testutil.MustJSONBody(t, contract.CreateLoopRequest{ForkFromName: "missing"}),
		)
		assertLoopStatus(t, resp.Code, http.StatusNotFound, resp.Body.String())
		var payload contract.ErrorPayload
		testutil.DecodeJSONResponse(t, resp, &payload)
		assertLoopErrorPayloadContains(t, payload, looppkg.ErrDefinitionNotFound.Error())
	})

	t.Run("Should return a bad request error payload for malformed Loop names", func(t *testing.T) {
		t.Parallel()

		service := happyLoopService(t)
		service.getLoopFn = func(context.Context, string, string) (contract.LoopResponse, error) {
			return contract.LoopResponse{}, looppkg.ErrValidation
		}
		_, engine := newLoopHandlerFixture(t, "httpapi", service)

		resp := performRequest(t, engine, http.MethodGet, "/workspaces/ws-1/loops/MyLoop", nil)
		assertLoopStatus(t, resp.Code, http.StatusBadRequest, resp.Body.String())
		var payload contract.ErrorPayload
		testutil.DecodeJSONResponse(t, resp, &payload)
		assertLoopErrorPayloadContains(t, payload, looppkg.ErrValidation.Error())
	})

	t.Run("Should return current version on publish CAS conflicts", func(t *testing.T) {
		t.Parallel()

		service := happyLoopService(t)
		service.patchLoopFn = func(
			context.Context,
			string,
			string,
			contract.PatchLoopRequest,
		) (contract.LoopResponse, error) {
			return contract.LoopResponse{}, &core.LoopVersionConflictError{CurrentVersion: 9}
		}
		_, engine := newLoopHandlerFixture(t, "httpapi", service)

		resp := performRequest(t, engine, http.MethodPatch, "/workspaces/ws-1/loops/alpha", loopPatchRequestBody(t, 7))
		assertLoopStatus(t, resp.Code, http.StatusConflict, resp.Body.String())
		var payload contract.LoopVersionConflictResponse
		testutil.DecodeJSONResponse(t, resp, &payload)
		if payload.CurrentVersion != 9 || payload.Error != core.ErrLoopVersionConflict.Error() {
			t.Fatalf("PATCH conflict payload = %#v", payload)
		}
	})

	t.Run("Should return error payloads for invalid Loop run transitions", func(t *testing.T) {
		t.Parallel()

		service := happyLoopService(t)
		service.pauseLoopRunFn = func(context.Context, string, string) error {
			return looppkg.ErrInvalidTransition
		}
		_, engine := newLoopHandlerFixture(t, "httpapi", service)

		resp := performRequest(t, engine, http.MethodPost, "/workspaces/ws-1/loop-runs/run-1/pause", nil)
		assertLoopStatus(t, resp.Code, http.StatusUnprocessableEntity, resp.Body.String())
		var payload contract.ErrorPayload
		testutil.DecodeJSONResponse(t, resp, &payload)
		assertLoopErrorPayloadContains(t, payload, looppkg.ErrInvalidTransition.Error())
	})

	t.Run("Should return conflict error payloads for Loop run CAS races", func(t *testing.T) {
		t.Parallel()

		service := happyLoopService(t)
		service.resumeLoopRunFn = func(context.Context, string, string) error {
			return looppkg.ErrTransitionConflict
		}
		_, engine := newLoopHandlerFixture(t, "httpapi", service)

		resp := performRequest(t, engine, http.MethodPost, "/workspaces/ws-1/loop-runs/run-1/resume", nil)
		assertLoopStatus(t, resp.Code, http.StatusConflict, resp.Body.String())
		var payload contract.ErrorPayload
		testutil.DecodeJSONResponse(t, resp, &payload)
		assertLoopErrorPayloadContains(t, payload, looppkg.ErrTransitionConflict.Error())
	})

	t.Run("Should return an error payload for run ancestry rejection", func(t *testing.T) {
		t.Parallel()

		service := happyLoopService(t)
		service.runLoopFn = func(
			context.Context,
			string,
			string,
			contract.RunLoopRequest,
			dsl.StartKind,
			taskpkg.ActorContext,
			bool,
		) (contract.RunLoopResponse, error) {
			return contract.RunLoopResponse{}, looppkg.ErrAncestryRejected
		}
		_, engine := newLoopHandlerFixture(t, "httpapi", service)

		resp := performRequest(t, engine, http.MethodPost, "/workspaces/ws-1/loops/alpha/run", []byte(`{}`))
		assertLoopStatus(t, resp.Code, http.StatusUnprocessableEntity, resp.Body.String())
		var payload contract.ErrorPayload
		testutil.DecodeJSONResponse(t, resp, &payload)
		assertLoopErrorPayloadContains(t, payload, looppkg.ErrAncestryRejected.Error())
	})

	t.Run("Should map RunLoop caller identity failures without masking them as Loop errors", func(t *testing.T) {
		t.Parallel()

		service := happyLoopService(t)
		service.runLoopFn = func(
			context.Context,
			string,
			string,
			contract.RunLoopRequest,
			dsl.StartKind,
			taskpkg.ActorContext,
			bool,
		) (contract.RunLoopResponse, error) {
			t.Fatal("RunLoop() should not be called when actor resolution fails")
			return contract.RunLoopResponse{}, nil
		}
		gin.SetMode(gin.TestMode)
		engine := gin.New()
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			TransportName: "udsapi",
			Loops:         service,
			TaskActorContextResolver: func(*gin.Context, string) (taskpkg.ActorContext, error) {
				return taskpkg.ActorContext{}, agentidentity.ErrIdentityStale
			},
		})
		registerLoopTestRoutes(engine, handlers)

		resp := performRequest(t, engine, http.MethodPost, "/workspaces/ws-1/loops/alpha/run", []byte(`{}`))
		assertLoopStatus(t, resp.Code, http.StatusUnauthorized, resp.Body.String())
		var payload contract.ErrorPayload
		testutil.DecodeJSONResponse(t, resp, &payload)
		assertLoopErrorPayloadContains(t, payload, agentidentity.ErrIdentityStale.Error())
	})

	t.Run("Should pass approve caller identity through the shared Loop handler", func(t *testing.T) {
		t.Parallel()

		service := happyLoopService(t)
		service.approveLoopRunFn = func(
			_ context.Context,
			workspaceID string,
			runID string,
			req contract.ApproveLoopRunRequest,
			actor taskpkg.ActorContext,
		) error {
			if workspaceID != "ws-1" || runID != "run-1" {
				t.Fatalf("ApproveLoopRun() target = %s/%s", workspaceID, runID)
			}
			if req.GateID != "human" || req.Decision != contract.LoopGateDecisionApprove {
				t.Fatalf("ApproveLoopRun() request = %#v", req)
			}
			if actor.Actor.Kind != taskpkg.ActorKindAgentSession || actor.Actor.Ref != "sess-author" {
				t.Fatalf("ApproveLoopRun() actor = %#v, want sess-author", actor.Actor)
			}
			return taskpkg.ErrPermissionDenied
		}
		gin.SetMode(gin.TestMode)
		engine := gin.New()
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			TransportName: "udsapi",
			Loops:         service,
			TaskActorContextResolver: func(*gin.Context, string) (taskpkg.ActorContext, error) {
				return taskpkg.DeriveAgentSessionActorContextForOrigin(
					"sess-author",
					"ws-1",
					taskpkg.OriginKindUDS,
					"loop_approve",
				)
			},
		})
		registerLoopTestRoutes(engine, handlers)

		body := []byte(`{"gate_id":"human","decision":"approve"}`)
		resp := performRequest(t, engine, http.MethodPost, "/workspaces/ws-1/loop-runs/run-1/approve", body)
		assertLoopStatus(t, resp.Code, http.StatusForbidden, resp.Body.String())
		var payload contract.ErrorPayload
		testutil.DecodeJSONResponse(t, resp, &payload)
		assertLoopErrorPayloadContains(t, payload, taskpkg.ErrPermissionDenied.Error())
	})
}

func assertLoopErrorPayloadContains(t *testing.T, payload contract.ErrorPayload, want string) {
	t.Helper()
	if !strings.Contains(payload.Error, want) {
		t.Fatalf("error payload = %#v, want error containing %q", payload, want)
	}
}

func newLoopHandlerFixture(
	t *testing.T,
	transport string,
	service *stubLoopService,
) (*core.BaseHandlers, http.Handler) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
		TransportName: transport,
		Loops:         service,
		PollInterval:  time.Millisecond,
	})
	registerLoopTestRoutes(engine, handlers)
	return handlers, engine
}

func registerLoopTestRoutes(engine *gin.Engine, handlers *core.BaseHandlers) {
	workspace := engine.Group("/workspaces/:workspace_id")
	workspace.GET("/loops", handlers.ListLoops)
	workspace.POST("/loops", handlers.CreateLoop)
	workspace.GET("/loops/:name", handlers.GetLoop)
	workspace.PATCH("/loops/:name", handlers.PatchLoop)
	workspace.DELETE("/loops/:name", handlers.DeleteLoop)
	workspace.POST("/loops/:name/validate", handlers.ValidateLoop)
	workspace.POST("/loops/:name/run", handlers.RunLoop)
	workspace.GET("/loops/:name/config", handlers.GetLoopConfig)
	workspace.PUT("/loops/:name/config", handlers.PutLoopConfig)
	workspace.GET("/loops/:name/annotations", handlers.GetLoopAnnotations)
	workspace.PUT("/loops/:name/annotations", handlers.PutLoopAnnotations)
	workspace.GET("/loop-runs", handlers.ListLoopRuns)
	workspace.GET("/loop-runs/:run_id", handlers.GetLoopRun)
	workspace.GET("/loop-runs/:run_id/turns", handlers.ListGoalTurns)
	workspace.POST("/loop-runs/:run_id/stop", handlers.StopLoopRun)
	workspace.POST("/loop-runs/:run_id/pause", handlers.PauseLoopRun)
	workspace.POST("/loop-runs/:run_id/resume", handlers.ResumeLoopRun)
	workspace.POST("/loop-runs/:run_id/approve", handlers.ApproveLoopRun)
	workspace.GET("/loop-runs/:run_id/events", handlers.StreamLoopRunEvents)
	workspace.GET("/sessions/:session_id/goal", handlers.GetSessionGoal)
}

type stubLoopService struct {
	listLoopsFn         func(context.Context, string, looppkg.CatalogQuery) (contract.LoopsResponse, error)
	createLoopFn        func(context.Context, string, contract.CreateLoopRequest) (contract.LoopResponse, error)
	getLoopFn           func(context.Context, string, string) (contract.LoopResponse, error)
	patchLoopFn         func(context.Context, string, string, contract.PatchLoopRequest) (contract.LoopResponse, error)
	validateLoopFn      func(context.Context, string, string, contract.ValidateLoopRequest) (contract.LoopValidationResponse, error)
	deleteLoopFn        func(context.Context, string, string) error
	runLoopFn           func(context.Context, string, string, contract.RunLoopRequest, dsl.StartKind, taskpkg.ActorContext, bool) (contract.RunLoopResponse, error)
	getLoopConfigFn     func(context.Context, string, string) (contract.LoopConfigResponse, error)
	putLoopConfigFn     func(context.Context, string, string, contract.PutLoopConfigRequest) (contract.LoopConfigResponse, error)
	getAnnotationsFn    func(context.Context, string, string) (contract.LoopAnnotationsResponse, error)
	putAnnotationsFn    func(context.Context, string, string, contract.PutLoopAnnotationsRequest) (contract.LoopAnnotationsResponse, error)
	listLoopRunsFn      func(context.Context, string, core.LoopRunListQuery) (contract.LoopRunsResponse, error)
	getLoopRunFn        func(context.Context, string, string) (contract.LoopRunResponse, error)
	getSessionGoalFn    func(context.Context, string, string) (*session.GoalSnapshot, error)
	listGoalTurnsFn     func(context.Context, string, string, core.GoalTurnListQuery) (session.GoalTurnPage, error)
	stopLoopRunFn       func(context.Context, string, string) error
	pauseLoopRunFn      func(context.Context, string, string) error
	resumeLoopRunFn     func(context.Context, string, string) error
	approveLoopRunFn    func(context.Context, string, string, contract.ApproveLoopRunRequest, taskpkg.ActorContext) error
	listLoopRunEventsFn func(context.Context, string, string, int64) ([]contract.LoopRunEventPayload, error)
}

func happyLoopService(t testing.TB) *stubLoopService {
	t.Helper()

	cfg := loopConfig()
	effectiveCfg := contract.LoopEffectiveConfig{FanOutWidth: 4, GateMaxRevisions: 10}
	annotations := []contract.LoopAnnotationPayload{{NodeID: "draft", X: 12, Y: 34}}
	return &stubLoopService{
		listLoopsFn: func(context.Context, string, looppkg.CatalogQuery) (contract.LoopsResponse, error) {
			return contract.LoopsResponse{Loops: []contract.LoopCatalogEntryPayload{{
				Name:    "alpha",
				Version: 7,
				Source:  contract.LoopSourceWorkspace,
				LastRun: &contract.LoopCatalogLastRunPayload{
					ID:        "run-catalog-latest",
					Status:    contract.LoopRunStatusDone,
					CreatedAt: time.Date(2026, 7, 10, 11, 0, 0, 0, time.UTC),
				},
				Inputs: map[string]contract.LoopInput{
					"ticket": {Type: string(dsl.InputTypeString), Required: true},
				},
				Start:         []contract.LoopStartBinding{{Kind: string(dsl.StartHTTP)}},
				Contract:      loopDefinitionDocument(t).Contract,
				Aggregate30d:  contract.LoopCatalogAggregatePayload{Runs: 2, Succeeded: 1, Failed: 1},
				SuccessRate30: 0.5,
			}}}, nil
		},
		createLoopFn: func(context.Context, string, contract.CreateLoopRequest) (contract.LoopResponse, error) {
			return loopResponse(t), nil
		},
		getLoopFn: func(context.Context, string, string) (contract.LoopResponse, error) {
			return loopResponse(t), nil
		},
		patchLoopFn: func(context.Context, string, string, contract.PatchLoopRequest) (contract.LoopResponse, error) {
			return loopResponse(t), nil
		},
		validateLoopFn: func(context.Context, string, string, contract.ValidateLoopRequest) (contract.LoopValidationResponse, error) {
			return contract.LoopValidationResponse{Valid: true}, nil
		},
		deleteLoopFn: func(context.Context, string, string) error {
			return nil
		},
		runLoopFn: func(context.Context, string, string, contract.RunLoopRequest, dsl.StartKind, taskpkg.ActorContext, bool) (contract.RunLoopResponse, error) {
			return contract.RunLoopResponse{Run: loopRunPayload("run-1", looppkg.StatusRunning)}, nil
		},
		getLoopConfigFn: func(context.Context, string, string) (contract.LoopConfigResponse, error) {
			return contract.LoopConfigResponse{Config: &cfg, EffectiveConfig: effectiveCfg}, nil
		},
		putLoopConfigFn: func(context.Context, string, string, contract.PutLoopConfigRequest) (contract.LoopConfigResponse, error) {
			return contract.LoopConfigResponse{Config: &cfg, EffectiveConfig: effectiveCfg}, nil
		},
		getAnnotationsFn: func(context.Context, string, string) (contract.LoopAnnotationsResponse, error) {
			return contract.LoopAnnotationsResponse{Annotations: annotations}, nil
		},
		putAnnotationsFn: func(context.Context, string, string, contract.PutLoopAnnotationsRequest) (contract.LoopAnnotationsResponse, error) {
			return contract.LoopAnnotationsResponse{Annotations: annotations}, nil
		},
		listLoopRunsFn: func(_ context.Context, workspaceID string, query core.LoopRunListQuery) (contract.LoopRunsResponse, error) {
			if workspaceID != "ws-1" || query.LoopName != "alpha" || query.Status != "running" ||
				query.Origin != "session" || query.OriginSession != "session-1" || query.Live == nil ||
				!*query.Live || query.Limit != 10 {
				return contract.LoopRunsResponse{}, errors.New("unexpected loop run query")
			}
			return contract.LoopRunsResponse{
				Runs:       []contract.LoopRunPayload{*loopRunPayload("run-1", looppkg.StatusRunning)},
				Aggregates: contract.LoopRunsAggregatePayload{Total: 1, Live: 1},
			}, nil
		},
		getLoopRunFn: func(context.Context, string, string) (contract.LoopRunResponse, error) {
			return contract.LoopRunResponse{
				Run:         *loopRunPayload("run-1", looppkg.StatusRunning),
				Generations: []contract.LoopGenerationPayload{{Generation: 1}},
			}, nil
		},
		getSessionGoalFn: func(context.Context, string, string) (*session.GoalSnapshot, error) {
			return nil, nil
		},
		listGoalTurnsFn: func(context.Context, string, string, core.GoalTurnListQuery) (session.GoalTurnPage, error) {
			return session.GoalTurnPage{Turns: []session.GoalTurn{}}, nil
		},
		stopLoopRunFn: func(context.Context, string, string) error {
			return nil
		},
		pauseLoopRunFn: func(context.Context, string, string) error {
			return nil
		},
		resumeLoopRunFn: func(context.Context, string, string) error {
			return nil
		},
		approveLoopRunFn: func(context.Context, string, string, contract.ApproveLoopRunRequest, taskpkg.ActorContext) error {
			return nil
		},
		listLoopRunEventsFn: func(context.Context, string, string, int64) ([]contract.LoopRunEventPayload, error) {
			return nil, nil
		},
	}
}

func (s *stubLoopService) ListLoops(
	ctx context.Context,
	workspaceID string,
	query looppkg.CatalogQuery,
) (contract.LoopsResponse, error) {
	return s.listLoopsFn(ctx, workspaceID, query)
}

func (s *stubLoopService) CreateLoop(
	ctx context.Context,
	workspaceID string,
	req contract.CreateLoopRequest,
) (contract.LoopResponse, error) {
	return s.createLoopFn(ctx, workspaceID, req)
}

func (s *stubLoopService) GetLoop(ctx context.Context, workspaceID string, name string) (contract.LoopResponse, error) {
	return s.getLoopFn(ctx, workspaceID, name)
}

func (s *stubLoopService) PatchLoop(
	ctx context.Context,
	workspaceID string,
	name string,
	req contract.PatchLoopRequest,
) (contract.LoopResponse, error) {
	return s.patchLoopFn(ctx, workspaceID, name, req)
}

func (s *stubLoopService) ValidateLoop(
	ctx context.Context,
	workspaceID string,
	name string,
	req contract.ValidateLoopRequest,
) (contract.LoopValidationResponse, error) {
	return s.validateLoopFn(ctx, workspaceID, name, req)
}

func (s *stubLoopService) DeleteLoop(ctx context.Context, workspaceID string, name string) error {
	return s.deleteLoopFn(ctx, workspaceID, name)
}

func (s *stubLoopService) RunLoop(
	ctx context.Context,
	workspaceID string,
	name string,
	req contract.RunLoopRequest,
	startKind dsl.StartKind,
	actor taskpkg.ActorContext,
	dry bool,
) (contract.RunLoopResponse, error) {
	return s.runLoopFn(ctx, workspaceID, name, req, startKind, actor, dry)
}

func (s *stubLoopService) GetLoopConfig(
	ctx context.Context,
	workspaceID string,
	name string,
) (contract.LoopConfigResponse, error) {
	return s.getLoopConfigFn(ctx, workspaceID, name)
}

func (s *stubLoopService) PutLoopConfig(
	ctx context.Context,
	workspaceID string,
	name string,
	req contract.PutLoopConfigRequest,
) (contract.LoopConfigResponse, error) {
	return s.putLoopConfigFn(ctx, workspaceID, name, req)
}

func (s *stubLoopService) GetLoopAnnotations(
	ctx context.Context,
	workspaceID string,
	name string,
) (contract.LoopAnnotationsResponse, error) {
	return s.getAnnotationsFn(ctx, workspaceID, name)
}

func (s *stubLoopService) PutLoopAnnotations(
	ctx context.Context,
	workspaceID string,
	name string,
	req contract.PutLoopAnnotationsRequest,
) (contract.LoopAnnotationsResponse, error) {
	return s.putAnnotationsFn(ctx, workspaceID, name, req)
}

func (s *stubLoopService) ListLoopRuns(
	ctx context.Context,
	workspaceID string,
	query core.LoopRunListQuery,
) (contract.LoopRunsResponse, error) {
	return s.listLoopRunsFn(ctx, workspaceID, query)
}

func (s *stubLoopService) GetLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
) (contract.LoopRunResponse, error) {
	return s.getLoopRunFn(ctx, workspaceID, runID)
}

func (s *stubLoopService) GetSessionGoal(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (*session.GoalSnapshot, error) {
	return s.getSessionGoalFn(ctx, workspaceID, sessionID)
}

func (s *stubLoopService) ListGoalTurns(
	ctx context.Context,
	workspaceID string,
	runID string,
	query core.GoalTurnListQuery,
) (session.GoalTurnPage, error) {
	return s.listGoalTurnsFn(ctx, workspaceID, runID, query)
}

func (s *stubLoopService) StopLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
	_ taskpkg.ActorContext,
) error {
	return s.stopLoopRunFn(ctx, workspaceID, runID)
}

func (s *stubLoopService) PauseLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
	_ taskpkg.ActorContext,
) error {
	return s.pauseLoopRunFn(ctx, workspaceID, runID)
}

func (s *stubLoopService) ResumeLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
	_ taskpkg.ActorContext,
) error {
	return s.resumeLoopRunFn(ctx, workspaceID, runID)
}

func (s *stubLoopService) ApproveLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
	req contract.ApproveLoopRunRequest,
	actor taskpkg.ActorContext,
) error {
	return s.approveLoopRunFn(ctx, workspaceID, runID, req, actor)
}

func (s *stubLoopService) ListLoopRunEvents(
	ctx context.Context,
	workspaceID string,
	runID string,
	afterSeq int64,
) ([]contract.LoopRunEventPayload, error) {
	return s.listLoopRunEventsFn(ctx, workspaceID, runID, afterSeq)
}

func loopDefinition() dsl.Definition {
	def := dsl.Definition{
		APIVersion: dsl.APIVersion,
		Kind:       dsl.KindLoop,
		Meta: dsl.Meta{
			Name:        "alpha",
			Version:     7,
			Description: "Test loop",
		},
		Inputs: map[string]dsl.Input{
			"ticket": {Type: dsl.InputTypeString, Required: true},
		},
		Contract: dsl.Contract{
			Goal:             "Handle the ticket",
			DefinitionOfDone: "Ticket is resolved",
			IterationCap:     3,
			TerminalStates:   []dsl.TerminalState{dsl.TerminalDone, dsl.TerminalFailed},
		},
		Graph: dsl.Graph{Nodes: []dsl.Node{{ID: "draft", Class: dsl.NodeClassAction, Kind: "run-agent"}}},
		DefinitionExtensionState: &dsl.DefinitionExtensionState{
			Start: []dsl.StartBinding{{Kind: dsl.StartHTTP}},
		},
	}
	def.Normalize()
	return def
}

func loopDefinitionDocument(t testing.TB) contract.LoopDefinitionDocument {
	t.Helper()

	document, err := contract.NewLoopDefinitionDocument(loopDefinition())
	if err != nil {
		t.Fatalf("NewLoopDefinitionDocument() error = %v", err)
	}
	return document
}

func loopResponse(t testing.TB) contract.LoopResponse {
	t.Helper()

	def := loopDefinitionDocument(t)
	return contract.LoopResponse{Loop: contract.LoopDefinitionPayload{
		Name:        "alpha",
		Version:     7,
		Description: "Test loop",
		Source:      contract.LoopSourceWorkspace,
		Definition:  def,
	}}
}

func loopDefinitionRequestBody(t *testing.T) []byte {
	t.Helper()

	definition := loopDefinitionDocument(t)
	return testutil.MustJSONBody(t, contract.CreateLoopRequest{Definition: &definition})
}

func loopPatchRequestBody(t *testing.T, expectedVersion int) []byte {
	t.Helper()

	return testutil.MustJSONBody(t, contract.PatchLoopRequest{
		ExpectedVersion: &expectedVersion,
		Definition:      loopDefinitionDocument(t),
	})
}

func loopRunPayload(id string, status looppkg.Status) *contract.LoopRunPayload {
	return &contract.LoopRunPayload{
		ID:                           id,
		WorkspaceID:                  "ws-1",
		LoopName:                     "alpha",
		Status:                       contract.LoopRunStatus(status),
		Generation:                   1,
		ReattemptStrategy:            contract.LoopReattemptStrategy(looppkg.ReattemptFailedOnly),
		CreatedAt:                    fixedLoopTime(),
		LastProgressAt:               fixedLoopTime(),
		IterationCap:                 3,
		Inputs:                       map[string]any{"ticket": "AGH-14"},
		ResolvedNetworkParticipation: participation.CloneSpec(participation.LocalSpec()),
	}
}

func loopConfig() contract.LoopConfig {
	iterationCap := 4
	reattempt := contract.LoopReattemptFullBody
	return contract.LoopConfig{
		IterationCap:      &iterationCap,
		ReattemptStrategy: &reattempt,
	}
}

func fixedLoopTime() time.Time {
	return time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
}

func assertLoopOKMutation(t *testing.T, engine http.Handler, method string, path string, body []byte) {
	t.Helper()

	resp := performRequest(t, engine, method, path, body)
	assertLoopStatus(t, resp.Code, http.StatusOK, resp.Body.String())
	var payload map[string]bool
	testutil.DecodeJSONResponse(t, resp, &payload)
	if !payload["ok"] {
		t.Fatalf("%s %s payload = %#v, want ok=true", method, path, payload)
	}
}

func assertLoopValidationDiagnostic(
	t *testing.T,
	payload contract.LoopValidationResponse,
	want contract.LoopLintErrorPayload,
) {
	t.Helper()

	if payload.Valid || len(payload.Errors) != 1 {
		t.Fatalf("validation payload = %#v", payload)
	}
	got := payload.Errors[0]
	if got.NodeID != want.NodeID || got.Code != want.Code || got.Message != want.Message ||
		got.Severity != want.Severity {
		t.Fatalf("validation diagnostic = %#v, want %#v", got, want)
	}
}

func assertLoopStatus(t *testing.T, got int, want int, body string) {
	t.Helper()

	if got != want {
		t.Fatalf("status = %d, want %d body=%s", got, want, body)
	}
}
