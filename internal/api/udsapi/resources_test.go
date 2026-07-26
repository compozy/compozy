package udsapi

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/resources"
	"github.com/gin-gonic/gin"
)

func TestListResourcesHandlerPreservesFilterSemantics(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	var gotFilter resources.ResourceFilter
	engine := newTestRouter(
		t,
		newTestHandlersWithResources(
			t,
			stubSessionManager{},
			stubObserver{},
			stubResourceService{
				ListFn: func(_ context.Context, filter resources.ResourceFilter) ([]resources.RawRecord, error) {
					gotFilter = filter
					return []resources.RawRecord{
						{
							Kind:    resources.ResourceKind("bundle.activation"),
							ID:      "bundle-1",
							Version: 3,
							Scope: resources.ResourceScope{
								Kind: resources.ResourceScopeKindWorkspace,
								ID:   "ws-alpha",
							},
							Owner: resources.ResourceOwner{
								Kind: resources.ResourceOwnerKind("daemon"),
								ID:   "daemon-control",
							},
							Source: resources.ResourceSource{
								Kind: resources.ResourceSourceKind("daemon"),
								ID:   "system",
							},
							SpecJSON:  []byte(`{"enabled":true}`),
							CreatedAt: time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
							UpdatedAt: time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
						},
					}, nil
				},
			},
			homePaths,
		),
	)

	resp := performRequest(
		t,
		engine,
		http.MethodGet,
		"/api/resources/bundle.activation?scope_kind=workspace&scope_id=ws-alpha&owner_kind=daemon&owner_id=daemon-control&source_kind=daemon&source_id=system&limit=7",
		nil,
	)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}

	if gotFilter.Kind != resources.ResourceKind("bundle.activation") || gotFilter.Limit != 7 {
		t.Fatalf("gotFilter = %#v", gotFilter)
	}
	if gotFilter.Scope == nil || gotFilter.Scope.Kind != resources.ResourceScopeKindWorkspace ||
		gotFilter.Scope.ID != "ws-alpha" {
		t.Fatalf("gotFilter.Scope = %#v, want workspace ws-alpha", gotFilter.Scope)
	}
	if gotFilter.Owner == nil || gotFilter.Owner.Kind != resources.ResourceOwnerKind("daemon") ||
		gotFilter.Owner.ID != "daemon-control" {
		t.Fatalf("gotFilter.Owner = %#v", gotFilter.Owner)
	}
	if gotFilter.Source == nil || gotFilter.Source.Kind != resources.ResourceSourceKind("daemon") ||
		gotFilter.Source.ID != "system" {
		t.Fatalf("gotFilter.Source = %#v", gotFilter.Source)
	}

	var payload contract.ResourcesResponse
	decodeJSONResponse(t, resp, &payload)
	if len(payload.Records) != 1 || payload.Records[0].ID != "bundle-1" {
		t.Fatalf("payload.Records = %#v, want bundle-1", payload.Records)
	}
}

func TestGetResourceHandlerPreservesKindAndID(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	var gotKind resources.ResourceKind
	var gotID string
	engine := newTestRouter(
		t,
		newTestHandlersWithResources(
			t,
			stubSessionManager{},
			stubObserver{},
			stubResourceService{
				GetFn: func(_ context.Context, kind resources.ResourceKind, id string) (resources.RawRecord, error) {
					gotKind = kind
					gotID = id
					return resources.RawRecord{
						Kind:    kind,
						ID:      id,
						Version: 5,
						Scope:   resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
						Owner: resources.ResourceOwner{
							Kind: resources.ResourceOwnerKind("daemon"),
							ID:   "daemon-control",
						},
						Source:    resources.ResourceSource{Kind: resources.ResourceSourceKind("daemon"), ID: "system"},
						SpecJSON:  []byte(`{"enabled":true}`),
						CreatedAt: time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
						UpdatedAt: time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
					}, nil
				},
			},
			homePaths,
		),
	)

	resp := performRequest(t, engine, http.MethodGet, "/api/resources/bridge.instance/bridge-1", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	if gotKind != resources.ResourceKind("bridge.instance") || gotID != "bridge-1" {
		t.Fatalf("Get() arguments = kind:%q id:%q", gotKind, gotID)
	}
}

func TestPutResourceHandlerPreservesExpectedVersionAndStatusSemantics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		body            []byte
		wantStatus      int
		wantVersion     int64
		wantScopeKind   resources.ResourceScopeKind
		wantScopeID     string
		wantExpectedVer int64
	}{
		{
			name:            "create",
			body:            []byte(`{"scope":{"kind":"global"},"spec":{"enabled":true}}`),
			wantStatus:      http.StatusCreated,
			wantVersion:     1,
			wantScopeKind:   resources.ResourceScopeKindGlobal,
			wantExpectedVer: 0,
		},
		{
			name: "update",
			body: []byte(
				`{"scope":{"kind":"workspace","id":"ws-alpha"},"expected_version":4,"spec":{"enabled":false}}`,
			),
			wantStatus:      http.StatusOK,
			wantVersion:     5,
			wantScopeKind:   resources.ResourceScopeKindWorkspace,
			wantScopeID:     "ws-alpha",
			wantExpectedVer: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			homePaths := newTestHomePaths(t)
			var gotDraft resources.RawDraft
			engine := newTestRouter(
				t,
				newTestHandlersWithResources(
					t,
					stubSessionManager{},
					stubObserver{},
					stubResourceService{
						PutFn: func(_ context.Context, draft resources.RawDraft) (resources.RawRecord, error) {
							gotDraft = draft
							return resources.RawRecord{
								Kind:    draft.Kind,
								ID:      draft.ID,
								Version: tt.wantVersion,
								Scope:   draft.Scope,
								Owner: resources.ResourceOwner{
									Kind: resources.ResourceOwnerKind("daemon"),
									ID:   "daemon-control",
								},
								Source: resources.ResourceSource{
									Kind: resources.ResourceSourceKind("daemon"),
									ID:   "system",
								},
								SpecJSON:  append([]byte(nil), draft.SpecJSON...),
								CreatedAt: time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
								UpdatedAt: time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
							}, nil
						},
					},
					homePaths,
				),
			)

			resp := performRequest(t, engine, http.MethodPut, "/api/resources/bundle.activation/demo", tt.body)
			if resp.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, tt.wantStatus, resp.Body.String())
			}
			if gotDraft.Kind != resources.ResourceKind("bundle.activation") || gotDraft.ID != "demo" {
				t.Fatalf("gotDraft identity = %#v", gotDraft)
			}
			if gotDraft.Scope.Kind != tt.wantScopeKind || gotDraft.Scope.ID != tt.wantScopeID {
				t.Fatalf("gotDraft.Scope = %#v, want %q/%q", gotDraft.Scope, tt.wantScopeKind, tt.wantScopeID)
			}
			if gotDraft.ExpectedVersion != tt.wantExpectedVer {
				t.Fatalf("gotDraft.ExpectedVersion = %d, want %d", gotDraft.ExpectedVersion, tt.wantExpectedVer)
			}

			var payload contract.ResourceResponse
			decodeJSONResponse(t, resp, &payload)
			if payload.Record.Version != tt.wantVersion {
				t.Fatalf("payload.Record.Version = %d, want %d", payload.Record.Version, tt.wantVersion)
			}
		})
	}
}

func TestPutResourceHandlerMapsResourceErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "invalid spec",
			err:  fmt.Errorf("codec validation: %w", resources.ErrValidation),
			want: http.StatusUnprocessableEntity,
		},
		{name: "stale version", err: fmt.Errorf("cas conflict: %w", resources.ErrConflict), want: http.StatusConflict},
		{
			name: "payload too large",
			err:  fmt.Errorf("payload ceiling: %w", resources.ErrPayloadTooLarge),
			want: http.StatusRequestEntityTooLarge,
		},
		{
			name: "rate limited",
			err:  fmt.Errorf("limiter: %w", resources.ErrRateLimited),
			want: http.StatusTooManyRequests,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			homePaths := newTestHomePaths(t)
			engine := newTestRouter(
				t,
				newTestHandlersWithResources(
					t,
					stubSessionManager{},
					stubObserver{},
					stubResourceService{
						PutFn: func(context.Context, resources.RawDraft) (resources.RawRecord, error) {
							return resources.RawRecord{}, tt.err
						},
					},
					homePaths,
				),
			)

			resp := performRequest(
				t,
				engine,
				http.MethodPut,
				"/api/resources/bundle.activation/demo",
				[]byte(`{"scope":{"kind":"global"},"spec":{"enabled":true}}`),
			)
			if resp.Code != tt.want {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, tt.want, resp.Body.String())
			}
		})
	}
}

func TestDeleteResourceHandlerPreservesExpectedVersionAndMapsConflict(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	var gotKind resources.ResourceKind
	var gotID string
	var gotExpectedVersion int64
	engine := newTestRouter(
		t,
		newTestHandlersWithResources(
			t,
			stubSessionManager{},
			stubObserver{},
			stubResourceService{
				DeleteFn: func(_ context.Context, kind resources.ResourceKind, id string, expectedVersion int64) error {
					gotKind = kind
					gotID = id
					gotExpectedVersion = expectedVersion
					return fmt.Errorf("cas conflict: %w", resources.ErrConflict)
				},
			},
			homePaths,
		),
	)

	resp := performRequest(
		t,
		engine,
		http.MethodDelete,
		"/api/resources/bundle.activation/demo",
		[]byte(`{"expected_version":2}`),
	)
	if resp.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusConflict, resp.Body.String())
	}
	if gotKind != resources.ResourceKind("bundle.activation") || gotID != "demo" || gotExpectedVersion != 2 {
		t.Fatalf("Delete() arguments = kind:%q id:%q expected_version:%d", gotKind, gotID, gotExpectedVersion)
	}
}

func TestRegisterRoutesKeepsOperationalRuntimeEndpointsFamilySpecific(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	engine := newTestRouter(
		t,
		newTestHandlersWithResources(t, stubSessionManager{}, stubObserver{}, stubResourceService{}, homePaths),
	)

	routes := udsRouteSet(engine)
	for _, route := range []string{
		"GET /api/workspaces/:workspace_id/hooks/runs",
		"GET /api/bridges/health/stream",
		"POST /api/bridges/:id/test-delivery",
	} {
		if _, ok := routes[route]; !ok {
			t.Fatalf("expected family-specific runtime route %q to remain registered", route)
		}
	}
	for _, route := range []string{
		"GET /api/resources/:kind/:id/runs",
		"GET /api/resources/:kind/:id/health",
		"POST /api/resources/:kind/:id/test-delivery",
	} {
		if _, ok := routes[route]; ok {
			t.Fatalf("unexpected generic runtime route %q is registered", route)
		}
	}
}

func udsRouteSet(engine *gin.Engine) map[string]struct{} {
	routes := make(map[string]struct{}, len(engine.Routes()))
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	return routes
}
