package httpapi

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/resources"
	"github.com/gin-gonic/gin"
)

func TestRegisterRoutesLeavesResourceSurfaceDisabledWithoutOperatorAuth(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	engine := newTestRouter(
		t,
		newTestHandlersWithResources(t, stubSessionManager{}, stubObserver{}, stubResourceService{}, homePaths),
	)

	routes := routeSet(engine)
	for _, route := range []string{
		"GET /api/resources",
		"GET /api/resources/:kind",
		"GET /api/resources/:kind/:id",
		"PUT /api/resources/:kind/:id",
		"DELETE /api/resources/:kind/:id",
	} {
		if _, ok := routes[route]; ok {
			t.Fatalf("route %q is registered without operator auth", route)
		}
	}
}

func TestRegisterRoutesExposesResourceSurfaceWhenOperatorAuthPresent(t *testing.T) {
	t.Parallel()

	var authCalls int
	var gotDraft resources.RawDraft

	engine := newTestRouter(
		t,
		newTestHandlersWithResourcesAndAuth(
			t,
			stubSessionManager{},
			stubObserver{},
			stubResourceService{
				PutFn: func(_ context.Context, draft resources.RawDraft) (resources.RawRecord, error) {
					gotDraft = draft
					return resources.RawRecord{
						Kind:    draft.Kind,
						ID:      draft.ID,
						Version: 1,
						Scope:   draft.Scope,
						Owner: resources.ResourceOwner{
							Kind: resources.ResourceOwnerKind("daemon"),
							ID:   "daemon-control",
						},
						Source:    resources.ResourceSource{Kind: resources.ResourceSourceKind("daemon"), ID: "system"},
						SpecJSON:  append([]byte(nil), draft.SpecJSON...),
						CreatedAt: time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
						UpdatedAt: time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
					}, nil
				},
			},
			func(c *gin.Context) {
				authCalls++
				c.Next()
			},
		),
	)

	routes := routeSet(engine)
	for _, route := range []string{
		"GET /api/resources",
		"GET /api/resources/:kind",
		"GET /api/resources/:kind/:id",
		"PUT /api/resources/:kind/:id",
		"DELETE /api/resources/:kind/:id",
	} {
		if _, ok := routes[route]; !ok {
			t.Fatalf("route %q is missing with operator auth", route)
		}
	}

	resp := performRequest(
		t,
		engine,
		http.MethodPut,
		"/api/resources/bundle.activation/demo",
		[]byte(`{"scope":{"kind":"global"},"spec":{"enabled":true}}`),
	)
	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusCreated, resp.Body.String())
	}
	if authCalls != 1 {
		t.Fatalf("authCalls = %d, want 1", authCalls)
	}
	if gotDraft.Kind != resources.ResourceKind("bundle.activation") || gotDraft.ID != "demo" {
		t.Fatalf("gotDraft identity = %#v", gotDraft)
	}

	var payload contract.ResourceResponse
	decodeJSONResponse(t, resp, &payload)
	if payload.Record.Version != 1 {
		t.Fatalf("payload.Record.Version = %d, want 1", payload.Record.Version)
	}
}

func TestResourceMutationRoutesRemainUnavailableWithoutOperatorAuth(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	called := false
	engine := newTestRouter(
		t,
		newTestHandlersWithResources(
			t,
			stubSessionManager{},
			stubObserver{},
			stubResourceService{
				PutFn: func(context.Context, resources.RawDraft) (resources.RawRecord, error) {
					called = true
					return resources.RawRecord{}, nil
				},
				DeleteFn: func(context.Context, resources.ResourceKind, string, int64) error {
					called = true
					return nil
				},
			},
			homePaths,
		),
	)

	putResp := performRequest(
		t,
		engine,
		http.MethodPut,
		"/api/resources/bundle.activation/demo",
		[]byte(`{"scope":{"kind":"global"},"spec":{"enabled":true}}`),
	)
	if putResp.Code != http.StatusNotFound {
		t.Fatalf("PUT status = %d, want %d; body=%s", putResp.Code, http.StatusNotFound, putResp.Body.String())
	}

	deleteResp := performRequest(
		t,
		engine,
		http.MethodDelete,
		"/api/resources/bundle.activation/demo",
		[]byte(`{"expected_version":1}`),
	)
	if deleteResp.Code != http.StatusNotFound {
		t.Fatalf("DELETE status = %d, want %d; body=%s", deleteResp.Code, http.StatusNotFound, deleteResp.Body.String())
	}

	if called {
		t.Fatal("resource service was invoked even though resource routes are disabled")
	}
}

func routeSet(engine *gin.Engine) map[string]struct{} {
	routes := make(map[string]struct{}, len(engine.Routes()))
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	return routes
}
