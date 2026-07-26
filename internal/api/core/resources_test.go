package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/resources"
	"github.com/gin-gonic/gin"
)

type stubRawStore struct {
	PutRawFn              func(context.Context, resources.MutationActor, resources.RawDraft) (resources.RawRecord, error)
	DeleteRawFn           func(context.Context, resources.MutationActor, resources.ResourceKind, string, int64) error
	ApplySourceSnapshotFn func(context.Context, resources.MutationActor, resources.SourceSnapshot) error
	GetRawFn              func(context.Context, resources.MutationActor, resources.ResourceKind, string) (resources.RawRecord, error)
	ListRawFn             func(context.Context, resources.MutationActor, resources.ResourceFilter) ([]resources.RawRecord, error)
}

func (s stubRawStore) PutRaw(
	ctx context.Context,
	actor resources.MutationActor,
	draft resources.RawDraft,
) (resources.RawRecord, error) {
	if s.PutRawFn != nil {
		return s.PutRawFn(ctx, actor, draft)
	}
	return resources.RawRecord{}, nil
}

func (s stubRawStore) DeleteRaw(
	ctx context.Context,
	actor resources.MutationActor,
	kind resources.ResourceKind,
	id string,
	expectedVersion int64,
) error {
	if s.DeleteRawFn != nil {
		return s.DeleteRawFn(ctx, actor, kind, id, expectedVersion)
	}
	return nil
}

func (s stubRawStore) ApplySourceSnapshotRaw(
	ctx context.Context,
	actor resources.MutationActor,
	snapshot resources.SourceSnapshot,
) error {
	if s.ApplySourceSnapshotFn != nil {
		return s.ApplySourceSnapshotFn(ctx, actor, snapshot)
	}
	return nil
}

func (s stubRawStore) GetRaw(
	ctx context.Context,
	actor resources.MutationActor,
	kind resources.ResourceKind,
	id string,
) (resources.RawRecord, error) {
	if s.GetRawFn != nil {
		return s.GetRawFn(ctx, actor, kind, id)
	}
	return resources.RawRecord{}, nil
}

func (s stubRawStore) ListRaw(
	ctx context.Context,
	actor resources.MutationActor,
	filter resources.ResourceFilter,
) ([]resources.RawRecord, error) {
	if s.ListRawFn != nil {
		return s.ListRawFn(ctx, actor, filter)
	}
	return nil, nil
}

type stubResourceService struct {
	ListFn   func(context.Context, resources.ResourceFilter) ([]resources.RawRecord, error)
	GetFn    func(context.Context, resources.ResourceKind, string) (resources.RawRecord, error)
	PutFn    func(context.Context, resources.RawDraft) (resources.RawRecord, error)
	DeleteFn func(context.Context, resources.ResourceKind, string, int64) error
}

func (s stubResourceService) List(
	ctx context.Context,
	filter resources.ResourceFilter,
) ([]resources.RawRecord, error) {
	if s.ListFn != nil {
		return s.ListFn(ctx, filter)
	}
	return nil, nil
}

func (s stubResourceService) Get(
	ctx context.Context,
	kind resources.ResourceKind,
	id string,
) (resources.RawRecord, error) {
	if s.GetFn != nil {
		return s.GetFn(ctx, kind, id)
	}
	return resources.RawRecord{}, nil
}

func (s stubResourceService) Put(
	ctx context.Context,
	draft resources.RawDraft,
) (resources.RawRecord, error) {
	if s.PutFn != nil {
		return s.PutFn(ctx, draft)
	}
	return resources.RawRecord{}, nil
}

func (s stubResourceService) Delete(
	ctx context.Context,
	kind resources.ResourceKind,
	id string,
	expectedVersion int64,
) error {
	if s.DeleteFn != nil {
		return s.DeleteFn(ctx, kind, id, expectedVersion)
	}
	return nil
}

func TestStatusForResourceError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "nil", err: nil, want: http.StatusOK},
		{name: "validation", err: resources.ErrValidation, want: http.StatusUnprocessableEntity},
		{name: "invalid scope", err: resources.ErrInvalidScopeBinding, want: http.StatusUnprocessableEntity},
		{name: "codec mismatch", err: resources.ErrCodecTypeMismatch, want: http.StatusUnprocessableEntity},
		{name: "permission denied", err: resources.ErrPermissionDenied, want: http.StatusForbidden},
		{name: "direct mutation denied", err: resources.ErrDirectMutationNotAllowed, want: http.StatusForbidden},
		{name: "conflict", err: resources.ErrConflict, want: http.StatusConflict},
		{name: "stale source version", err: resources.ErrStaleSourceVersion, want: http.StatusConflict},
		{name: "payload too large", err: resources.ErrPayloadTooLarge, want: http.StatusRequestEntityTooLarge},
		{name: "rate limited", err: resources.ErrRateLimited, want: http.StatusTooManyRequests},
		{name: "not found", err: resources.ErrNotFound, want: http.StatusNotFound},
		{
			name: "wrapped validation",
			err:  errors.Join(errors.New("boom"), resources.ErrValidation),
			want: http.StatusUnprocessableEntity,
		},
		{name: "default", err: errors.New("boom"), want: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := StatusForResourceError(tt.err); got != tt.want {
				t.Fatalf("StatusForResourceError(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

func TestParseResourceFilterPreservesListSemantics(t *testing.T) {
	t.Parallel()

	ctx := newResourceTestContext(
		t,
		http.MethodGet,
		"/api/resources/bundle.activation?scope_kind=workspace&scope_id=ws-alpha&owner_kind=daemon&owner_id=daemon-control&source_kind=daemon&source_id=system&limit=7",
	)
	ctx.Params = gin.Params{{Key: "kind", Value: "bundle.activation"}}

	filter, err := ParseResourceFilter(ctx)
	if err != nil {
		t.Fatalf("ParseResourceFilter() error = %v", err)
	}
	if filter.Kind != resources.ResourceKind("bundle.activation") {
		t.Fatalf("filter.Kind = %q, want %q", filter.Kind, resources.ResourceKind("bundle.activation"))
	}
	if filter.Limit != 7 {
		t.Fatalf("filter.Limit = %d, want 7", filter.Limit)
	}
	if filter.Scope == nil || filter.Scope.Kind != resources.ResourceScopeKindWorkspace ||
		filter.Scope.ID != "ws-alpha" {
		t.Fatalf("filter.Scope = %#v, want workspace ws-alpha", filter.Scope)
	}
	if filter.Owner == nil || filter.Owner.Kind != resources.ResourceOwnerKind("daemon") ||
		filter.Owner.ID != "daemon-control" {
		t.Fatalf("filter.Owner = %#v, want daemon/daemon-control", filter.Owner)
	}
	if filter.Source == nil || filter.Source.Kind != resources.ResourceSourceKind("daemon") ||
		filter.Source.ID != "system" {
		t.Fatalf("filter.Source = %#v, want daemon/system", filter.Source)
	}
}

func TestParseResourceFilterRejectsMismatchedPathAndQueryKinds(t *testing.T) {
	t.Parallel()

	ctx := newResourceTestContext(t, http.MethodGet, "/api/resources/bundle.activation?kind=bridge.instance")
	ctx.Params = gin.Params{{Key: "kind", Value: "bundle.activation"}}

	_, err := ParseResourceFilter(ctx)
	if err == nil {
		t.Fatal("ParseResourceFilter() error = nil, want non-nil")
	}
	if !errors.Is(err, resources.ErrValidation) {
		t.Fatalf("ParseResourceFilter() error = %v, want ErrValidation", err)
	}
	if got := statusForResourceRequestError(err); got != http.StatusUnprocessableEntity {
		t.Fatalf("statusForResourceRequestError() = %d, want %d", got, http.StatusUnprocessableEntity)
	}
}

func TestParseResourcePutDraftPreservesExpectedVersionAndScope(t *testing.T) {
	t.Parallel()

	wantSpec := []byte(`{"enabled":true}`)
	spec := append([]byte(nil), wantSpec...)

	draft, err := parseResourcePutDraft(
		resources.ResourceKind("bundle.activation"),
		"bundle-1",
		contract.PutResourceRequest{
			Scope:           resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: " ws-alpha "},
			ExpectedVersion: 7,
			Spec:            spec,
		},
	)
	if err != nil {
		t.Fatalf("parseResourcePutDraft() error = %v", err)
	}
	if draft.Kind != resources.ResourceKind("bundle.activation") || draft.ID != "bundle-1" {
		t.Fatalf("draft identity = %#v", draft)
	}
	if draft.Scope.Kind != resources.ResourceScopeKindWorkspace || draft.Scope.ID != "ws-alpha" {
		t.Fatalf("draft.Scope = %#v, want workspace ws-alpha", draft.Scope)
	}
	if draft.ExpectedVersion != 7 {
		t.Fatalf("draft.ExpectedVersion = %d, want 7", draft.ExpectedVersion)
	}
	spec[2] = 'X'
	if !bytes.Equal(draft.SpecJSON, wantSpec) {
		t.Fatalf("draft.SpecJSON = %s, want %s", string(draft.SpecJSON), string(wantSpec))
	}
}

func TestParseResourcePutDraftRejectsNegativeExpectedVersion(t *testing.T) {
	t.Parallel()

	_, err := parseResourcePutDraft(
		resources.ResourceKind("bundle.activation"),
		"bundle-1",
		contract.PutResourceRequest{
			Scope:           resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			ExpectedVersion: -1,
			Spec:            []byte(`{"enabled":true}`),
		},
	)
	if err == nil {
		t.Fatal("parseResourcePutDraft() error = nil, want non-nil")
	}
	if !errors.Is(err, resources.ErrValidation) {
		t.Fatalf("parseResourcePutDraft() error = %v, want ErrValidation", err)
	}
}

func TestResourceRecordPayloadFromRawCopiesSpec(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	record := resources.RawRecord{
		Kind:      resources.ResourceKind("bundle.activation"),
		ID:        "bundle-1",
		Version:   3,
		Scope:     resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
		Owner:     resources.ResourceOwner{Kind: resources.ResourceOwnerKind("daemon"), ID: "daemon-control"},
		Source:    resources.ResourceSource{Kind: resources.ResourceSourceKind("daemon"), ID: "system"},
		SpecJSON:  []byte(`{"enabled":true}`),
		CreatedAt: now,
		UpdatedAt: now,
	}

	payload := ResourceRecordPayloadFromRaw(record)
	record.SpecJSON[2] = 'X'
	if !bytes.Equal(payload.Spec, []byte(`{"enabled":true}`)) {
		t.Fatalf("payload.Spec = %s, want original JSON", string(payload.Spec))
	}

	payload.Spec[3] = 'Y'
	if bytes.Equal(record.SpecJSON, payload.Spec) {
		t.Fatal("payload.Spec shares backing storage with record.SpecJSON")
	}
}

func TestNewOperatorResourceServiceRequiresRawStore(t *testing.T) {
	t.Parallel()

	if _, err := NewOperatorResourceService(nil); err == nil {
		t.Fatal("NewOperatorResourceService(nil) error = nil, want non-nil")
	}
	if _, err := NewOperatorResourceService(&ResourceServiceConfig{}); err == nil {
		t.Fatal("NewOperatorResourceService(missing store) error = nil, want non-nil")
	}
}

func TestOperatorResourceServiceUsesDefaultControlActorAndCodecValidation(t *testing.T) {
	t.Parallel()

	type spec struct {
		Name string `json:"name"`
	}

	registry := resources.NewCodecRegistry()
	testKind := resources.ResourceKind("test.resource")
	codec, err := resources.NewJSONCodec[spec](
		testKind,
		1024,
		func(_ context.Context, scope resources.ResourceScope, value spec) (spec, error) {
			if scope.Kind != resources.ResourceScopeKindGlobal {
				t.Fatalf("validator scope = %#v, want global", scope)
			}
			value.Name = strings.TrimSpace(value.Name)
			return value, nil
		},
	)
	if err != nil {
		t.Fatalf("resources.NewJSONCodec() error = %v", err)
	}
	if err := resources.RegisterCodec(registry, codec); err != nil {
		t.Fatalf("resources.RegisterCodec() error = %v", err)
	}

	var listActor resources.MutationActor
	var getActor resources.MutationActor
	var putActor resources.MutationActor
	var deleteActor resources.MutationActor
	var gotFilter resources.ResourceFilter
	var gotDraft resources.RawDraft

	store := stubRawStore{
		ListRawFn: func(_ context.Context, actor resources.MutationActor, filter resources.ResourceFilter) ([]resources.RawRecord, error) {
			listActor = actor
			gotFilter = filter
			return []resources.RawRecord{{
				Kind:      testKind,
				ID:        "demo",
				Version:   2,
				Scope:     resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
				Owner:     resources.ResourceOwner{Kind: resources.ResourceOwnerKind("daemon"), ID: "daemon-control"},
				Source:    resources.ResourceSource{Kind: resources.ResourceSourceKind("daemon"), ID: "system"},
				SpecJSON:  []byte(`{"name":"demo"}`),
				CreatedAt: time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
			}}, nil
		},
		GetRawFn: func(_ context.Context, actor resources.MutationActor, kind resources.ResourceKind, id string) (resources.RawRecord, error) {
			getActor = actor
			return resources.RawRecord{
				Kind:      kind,
				ID:        id,
				Version:   2,
				Scope:     resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
				Owner:     resources.ResourceOwner{Kind: resources.ResourceOwnerKind("daemon"), ID: "daemon-control"},
				Source:    resources.ResourceSource{Kind: resources.ResourceSourceKind("daemon"), ID: "system"},
				SpecJSON:  []byte(`{"name":"demo"}`),
				CreatedAt: time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
			}, nil
		},
		PutRawFn: func(_ context.Context, actor resources.MutationActor, draft resources.RawDraft) (resources.RawRecord, error) {
			putActor = actor
			gotDraft = draft
			return resources.RawRecord{
				Kind:      draft.Kind,
				ID:        draft.ID,
				Version:   1,
				Scope:     draft.Scope,
				Owner:     resources.ResourceOwner{Kind: resources.ResourceOwnerKind("daemon"), ID: "daemon-control"},
				Source:    resources.ResourceSource{Kind: resources.ResourceSourceKind("daemon"), ID: "system"},
				SpecJSON:  append([]byte(nil), draft.SpecJSON...),
				CreatedAt: time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
			}, nil
		},
		DeleteRawFn: func(_ context.Context, actor resources.MutationActor, kind resources.ResourceKind, id string, expectedVersion int64) error {
			deleteActor = actor
			if kind != testKind || id != "demo" || expectedVersion != 2 {
				t.Fatalf("DeleteRaw() args = kind:%q id:%q expected_version:%d", kind, id, expectedVersion)
			}
			return nil
		},
	}

	service, err := NewOperatorResourceService(&ResourceServiceConfig{
		RawStore:      store,
		CodecRegistry: registry,
	})
	if err != nil {
		t.Fatalf("NewOperatorResourceService() error = %v", err)
	}

	records, err := service.List(
		context.Background(),
		resources.ResourceFilter{Kind: testKind, Limit: 5},
	)
	if err != nil {
		t.Fatalf("service.List() error = %v", err)
	}
	if len(records) != 1 || gotFilter.Kind != testKind || gotFilter.Limit != 5 {
		t.Fatalf("service.List() records=%#v filter=%#v", records, gotFilter)
	}

	if _, err := service.Get(context.Background(), testKind, "demo"); err != nil {
		t.Fatalf("service.Get() error = %v", err)
	}

	if _, err := service.Put(
		context.Background(),
		resources.RawDraft{
			Kind:     testKind,
			ID:       "demo",
			Scope:    resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			SpecJSON: []byte(`{"name":"  demo  "}`),
		},
	); err != nil {
		t.Fatalf("service.Put() error = %v", err)
	}
	if string(gotDraft.SpecJSON) != `{"name":"demo"}` {
		t.Fatalf("service.Put() canonical spec = %s, want %s", string(gotDraft.SpecJSON), `{"name":"demo"}`)
	}

	if err := service.Delete(context.Background(), testKind, "demo", 2); err != nil {
		t.Fatalf("service.Delete() error = %v", err)
	}

	for name, actor := range map[string]resources.MutationActor{
		"list":   listActor,
		"get":    getActor,
		"put":    putActor,
		"delete": deleteActor,
	} {
		if actor.Kind != resources.MutationActorKindDaemon || actor.ID != "daemon-control" {
			t.Fatalf("%s actor = %#v, want daemon-control", name, actor)
		}
		if actor.Source.Kind != resources.ResourceSourceKind("daemon") || actor.Source.ID != "system" {
			t.Fatalf("%s actor source = %#v, want daemon/system", name, actor.Source)
		}
		if actor.MaxScope.Kind != resources.ResourceScopeKindGlobal {
			t.Fatalf("%s actor max_scope = %#v, want global", name, actor.MaxScope)
		}
	}
}

func TestOperatorResourceServicePutReturnsCodecValidationError(t *testing.T) {
	t.Parallel()

	type spec struct {
		Name string `json:"name"`
	}

	registry := resources.NewCodecRegistry()
	testKind := resources.ResourceKind("test.resource")
	codec, err := resources.NewJSONCodec[spec](
		testKind,
		1024,
		func(context.Context, resources.ResourceScope, spec) (spec, error) {
			return spec{}, fmt.Errorf("%w: name is required", resources.ErrValidation)
		},
	)
	if err != nil {
		t.Fatalf("resources.NewJSONCodec() error = %v", err)
	}
	if err := resources.RegisterCodec(registry, codec); err != nil {
		t.Fatalf("resources.RegisterCodec() error = %v", err)
	}

	called := false
	service, err := NewOperatorResourceService(&ResourceServiceConfig{
		RawStore: stubRawStore{
			PutRawFn: func(context.Context, resources.MutationActor, resources.RawDraft) (resources.RawRecord, error) {
				called = true
				return resources.RawRecord{}, nil
			},
		},
		CodecRegistry: registry,
	})
	if err != nil {
		t.Fatalf("NewOperatorResourceService() error = %v", err)
	}

	_, err = service.Put(
		context.Background(),
		resources.RawDraft{
			Kind:     testKind,
			ID:       "demo",
			Scope:    resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			SpecJSON: []byte(`{"name":"demo"}`),
		},
	)
	if err == nil {
		t.Fatal("service.Put() error = nil, want non-nil")
	}
	if !errors.Is(err, resources.ErrValidation) {
		t.Fatalf("service.Put() error = %v, want ErrValidation", err)
	}
	if called {
		t.Fatal("raw store PutRaw() was called after codec validation failed")
	}
}

func TestOperatorResourceServiceRejectsGenericBundleActivationMutation(t *testing.T) {
	t.Parallel()

	putCalled := false
	deleteCalled := false
	service, err := NewOperatorResourceService(&ResourceServiceConfig{
		RawStore: stubRawStore{
			PutRawFn: func(context.Context, resources.MutationActor, resources.RawDraft) (resources.RawRecord, error) {
				putCalled = true
				return resources.RawRecord{}, nil
			},
			DeleteRawFn: func(
				context.Context,
				resources.MutationActor,
				resources.ResourceKind,
				string,
				int64,
			) error {
				deleteCalled = true
				return nil
			},
		},
	})
	if err != nil {
		t.Fatalf("NewOperatorResourceService() error = %v", err)
	}

	_, putErr := service.Put(context.Background(), resources.RawDraft{
		Kind:     resources.ResourceKind("bundle.activation"),
		ID:       "act-1",
		Scope:    resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
		SpecJSON: []byte(`{}`),
	})
	if !errors.Is(putErr, resources.ErrDirectMutationNotAllowed) {
		t.Fatalf("Put(bundle.activation) error = %v, want ErrDirectMutationNotAllowed", putErr)
	}
	deleteErr := service.Delete(
		context.Background(),
		resources.ResourceKind("bundle.activation"),
		"act-1",
		1,
	)
	if !errors.Is(deleteErr, resources.ErrDirectMutationNotAllowed) {
		t.Fatalf("Delete(bundle.activation) error = %v, want ErrDirectMutationNotAllowed", deleteErr)
	}
	if putCalled || deleteCalled {
		t.Fatalf("raw mutations called: put=%t delete=%t, want both false", putCalled, deleteCalled)
	}
}

func TestBaseHandlersResourceEndpointsUseSharedSemantics(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)

	t.Run("Should list", func(t *testing.T) {
		var gotFilter resources.ResourceFilter
		handlers := NewBaseHandlers(&BaseHandlerConfig{
			TransportName: "core-test",
			Resources: stubResourceService{
				ListFn: func(_ context.Context, filter resources.ResourceFilter) ([]resources.RawRecord, error) {
					gotFilter = filter
					return []resources.RawRecord{
						{
							Kind:    resources.ResourceKind("bundle.activation"),
							ID:      "demo",
							Version: 1,
							Scope:   resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
							Owner: resources.ResourceOwner{
								Kind: resources.ResourceOwnerKind("daemon"),
								ID:   "daemon-control",
							},
							Source: resources.ResourceSource{
								Kind: resources.ResourceSourceKind("daemon"),
								ID:   "system",
							},
							SpecJSON:  []byte(`{"enabled":true}`),
							CreatedAt: now,
							UpdatedAt: now,
						},
					}, nil
				},
			},
		})
		ctx, recorder := newResourceRequestContext(
			t,
			http.MethodGet,
			"/api/resources/bundle.activation?scope_kind=global&limit=2",
			nil,
			gin.Params{{Key: "kind", Value: "bundle.activation"}},
		)

		handlers.ListResources(ctx)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if gotFilter.Kind != resources.ResourceKind("bundle.activation") || gotFilter.Limit != 2 {
			t.Fatalf("ListResources() filter = %#v", gotFilter)
		}
	})

	t.Run("Should get", func(t *testing.T) {
		var gotKind resources.ResourceKind
		var gotID string
		handlers := NewBaseHandlers(&BaseHandlerConfig{
			TransportName: "core-test",
			Resources: stubResourceService{
				GetFn: func(_ context.Context, kind resources.ResourceKind, id string) (resources.RawRecord, error) {
					gotKind = kind
					gotID = id
					return resources.RawRecord{
						Kind:    kind,
						ID:      id,
						Version: 1,
						Scope:   resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
						Owner: resources.ResourceOwner{
							Kind: resources.ResourceOwnerKind("daemon"),
							ID:   "daemon-control",
						},
						Source:    resources.ResourceSource{Kind: resources.ResourceSourceKind("daemon"), ID: "system"},
						SpecJSON:  []byte(`{"enabled":true}`),
						CreatedAt: now,
						UpdatedAt: now,
					}, nil
				},
			},
		})
		ctx, recorder := newResourceRequestContext(
			t,
			http.MethodGet,
			"/api/resources/bundle.activation/demo",
			nil,
			gin.Params{{Key: "kind", Value: "bundle.activation"}, {Key: "id", Value: "demo"}},
		)

		handlers.GetResource(ctx)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if gotKind != resources.ResourceKind("bundle.activation") || gotID != "demo" {
			t.Fatalf("GetResource() args = kind:%q id:%q", gotKind, gotID)
		}
	})

	t.Run("Should put", func(t *testing.T) {
		var gotDraft resources.RawDraft
		handlers := NewBaseHandlers(&BaseHandlerConfig{
			TransportName: "core-test",
			Resources: stubResourceService{
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
						CreatedAt: now,
						UpdatedAt: now,
					}, nil
				},
			},
		})
		ctx, recorder := newResourceRequestContext(
			t,
			http.MethodPut,
			"/api/resources/bundle.activation/demo",
			[]byte(`{"scope":{"kind":"global"},"spec":{"enabled":true}}`),
			gin.Params{{Key: "kind", Value: "bundle.activation"}, {Key: "id", Value: "demo"}},
		)

		handlers.PutResource(ctx)

		if recorder.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusCreated, recorder.Body.String())
		}
		if gotDraft.Kind != resources.ResourceKind("bundle.activation") || gotDraft.ID != "demo" {
			t.Fatalf("PutResource() draft = %#v", gotDraft)
		}
	})

	t.Run("Should delete", func(t *testing.T) {
		var gotKind resources.ResourceKind
		var gotID string
		var gotVersion int64
		handlers := NewBaseHandlers(&BaseHandlerConfig{
			TransportName: "core-test",
			Resources: stubResourceService{
				DeleteFn: func(_ context.Context, kind resources.ResourceKind, id string, expectedVersion int64) error {
					gotKind = kind
					gotID = id
					gotVersion = expectedVersion
					return nil
				},
			},
		})
		ctx, recorder := newResourceRequestContext(
			t,
			http.MethodDelete,
			"/api/resources/bundle.activation/demo",
			[]byte(`{"expected_version":3}`),
			gin.Params{{Key: "kind", Value: "bundle.activation"}, {Key: "id", Value: "demo"}},
		)

		handlers.DeleteResource(ctx)

		if got := ctx.Writer.Status(); got != http.StatusNoContent {
			t.Fatalf(
				"status = %d, want %d; recorder=%d body=%s",
				got,
				http.StatusNoContent,
				recorder.Code,
				recorder.Body.String(),
			)
		}
		if gotKind != resources.ResourceKind("bundle.activation") || gotID != "demo" || gotVersion != 3 {
			t.Fatalf("DeleteResource() args = kind:%q id:%q expected_version:%d", gotKind, gotID, gotVersion)
		}
	})
}

func TestBaseHandlersResourceEndpointsHandleUnavailableServicesAndBadRequests(t *testing.T) {
	t.Parallel()

	t.Run("Should service unavailable", func(t *testing.T) {
		handlers := NewBaseHandlers(&BaseHandlerConfig{TransportName: "core-test"})

		tests := []struct {
			name   string
			method string
			target string
			body   []byte
			params gin.Params
			call   func(*BaseHandlers, *gin.Context)
		}{
			{
				name:   "list",
				method: http.MethodGet,
				target: "/api/resources",
				call:   (*BaseHandlers).ListResources,
			},
			{
				name:   "get",
				method: http.MethodGet,
				target: "/api/resources/bundle.activation/demo",
				params: gin.Params{{Key: "kind", Value: "bundle.activation"}, {Key: "id", Value: "demo"}},
				call:   (*BaseHandlers).GetResource,
			},
			{
				name:   "put",
				method: http.MethodPut,
				target: "/api/resources/bundle.activation/demo",
				body:   []byte(`{"scope":{"kind":"global"},"spec":{"enabled":true}}`),
				params: gin.Params{{Key: "kind", Value: "bundle.activation"}, {Key: "id", Value: "demo"}},
				call:   (*BaseHandlers).PutResource,
			},
			{
				name:   "delete",
				method: http.MethodDelete,
				target: "/api/resources/bundle.activation/demo",
				body:   []byte(`{"expected_version":1}`),
				params: gin.Params{{Key: "kind", Value: "bundle.activation"}, {Key: "id", Value: "demo"}},
				call:   (*BaseHandlers).DeleteResource,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				ctx, recorder := newResourceRequestContext(t, tt.method, tt.target, tt.body, tt.params)
				tt.call(handlers, ctx)
				if recorder.Code != http.StatusServiceUnavailable {
					t.Fatalf(
						"status = %d, want %d; body=%s",
						recorder.Code,
						http.StatusServiceUnavailable,
						recorder.Body.String(),
					)
				}
			})
		}
	})

	t.Run("Should bad requests", func(t *testing.T) {
		handlers := NewBaseHandlers(&BaseHandlerConfig{
			TransportName: "core-test",
			Resources:     stubResourceService{},
		})

		tests := []struct {
			name   string
			method string
			target string
			body   []byte
			params gin.Params
			call   func(*BaseHandlers, *gin.Context)
			want   int
		}{
			{
				name:   "list invalid scope",
				method: http.MethodGet,
				target: "/api/resources?scope_kind=workspace",
				call:   (*BaseHandlers).ListResources,
				want:   http.StatusUnprocessableEntity,
			},
			{
				name:   "get missing id",
				method: http.MethodGet,
				target: "/api/resources/bundle.activation",
				params: gin.Params{{Key: "kind", Value: "bundle.activation"}},
				call:   (*BaseHandlers).GetResource,
				want:   http.StatusUnprocessableEntity,
			},
			{
				name:   "put bad json",
				method: http.MethodPut,
				target: "/api/resources/bundle.activation/demo",
				body:   []byte(`{"scope":`),
				params: gin.Params{{Key: "kind", Value: "bundle.activation"}, {Key: "id", Value: "demo"}},
				call:   (*BaseHandlers).PutResource,
				want:   http.StatusBadRequest,
			},
			{
				name:   "put invalid scope",
				method: http.MethodPut,
				target: "/api/resources/bundle.activation/demo",
				body:   []byte(`{"scope":{"kind":"workspace"},"spec":{"enabled":true}}`),
				params: gin.Params{{Key: "kind", Value: "bundle.activation"}, {Key: "id", Value: "demo"}},
				call:   (*BaseHandlers).PutResource,
				want:   http.StatusUnprocessableEntity,
			},
			{
				name:   "delete bad json",
				method: http.MethodDelete,
				target: "/api/resources/bundle.activation/demo",
				body:   []byte(`{"expected_version":`),
				params: gin.Params{{Key: "kind", Value: "bundle.activation"}, {Key: "id", Value: "demo"}},
				call:   (*BaseHandlers).DeleteResource,
				want:   http.StatusBadRequest,
			},
			{
				name:   "delete missing expected version",
				method: http.MethodDelete,
				target: "/api/resources/bundle.activation/demo",
				body:   []byte(`{"expected_version":0}`),
				params: gin.Params{{Key: "kind", Value: "bundle.activation"}, {Key: "id", Value: "demo"}},
				call:   (*BaseHandlers).DeleteResource,
				want:   http.StatusUnprocessableEntity,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				ctx, recorder := newResourceRequestContext(t, tt.method, tt.target, tt.body, tt.params)
				tt.call(handlers, ctx)
				if recorder.Code != tt.want {
					t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tt.want, recorder.Body.String())
				}
			})
		}
	})

	t.Run("Should service errors", func(t *testing.T) {
		tests := []struct {
			name   string
			method string
			target string
			body   []byte
			params gin.Params
			call   func(*BaseHandlers, *gin.Context)
			want   int
			build  func(error) stubResourceService
			err    error
		}{
			{
				name:   "list forbidden",
				method: http.MethodGet,
				target: "/api/resources",
				call:   (*BaseHandlers).ListResources,
				want:   http.StatusForbidden,
				err:    resources.ErrPermissionDenied,
				build: func(err error) stubResourceService {
					return stubResourceService{
						ListFn: func(context.Context, resources.ResourceFilter) ([]resources.RawRecord, error) {
							return nil, err
						},
					}
				},
			},
			{
				name:   "get missing",
				method: http.MethodGet,
				target: "/api/resources/bundle.activation/demo",
				params: gin.Params{{Key: "kind", Value: "bundle.activation"}, {Key: "id", Value: "demo"}},
				call:   (*BaseHandlers).GetResource,
				want:   http.StatusNotFound,
				err:    resources.ErrNotFound,
				build: func(err error) stubResourceService {
					return stubResourceService{
						GetFn: func(context.Context, resources.ResourceKind, string) (resources.RawRecord, error) {
							return resources.RawRecord{}, err
						},
					}
				},
			},
			{
				name:   "put payload too large",
				method: http.MethodPut,
				target: "/api/resources/bundle.activation/demo",
				body:   []byte(`{"scope":{"kind":"global"},"spec":{"enabled":true}}`),
				params: gin.Params{{Key: "kind", Value: "bundle.activation"}, {Key: "id", Value: "demo"}},
				call:   (*BaseHandlers).PutResource,
				want:   http.StatusRequestEntityTooLarge,
				err:    resources.ErrPayloadTooLarge,
				build: func(err error) stubResourceService {
					return stubResourceService{
						PutFn: func(context.Context, resources.RawDraft) (resources.RawRecord, error) {
							return resources.RawRecord{}, err
						},
					}
				},
			},
			{
				name:   "delete rate limited",
				method: http.MethodDelete,
				target: "/api/resources/bundle.activation/demo",
				body:   []byte(`{"expected_version":3}`),
				params: gin.Params{{Key: "kind", Value: "bundle.activation"}, {Key: "id", Value: "demo"}},
				call:   (*BaseHandlers).DeleteResource,
				want:   http.StatusTooManyRequests,
				err:    resources.ErrRateLimited,
				build: func(err error) stubResourceService {
					return stubResourceService{
						DeleteFn: func(context.Context, resources.ResourceKind, string, int64) error {
							return err
						},
					}
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				handlers := NewBaseHandlers(&BaseHandlerConfig{
					TransportName: "core-test",
					Resources:     tt.build(tt.err),
				})
				ctx, recorder := newResourceRequestContext(t, tt.method, tt.target, tt.body, tt.params)
				tt.call(handlers, ctx)
				if recorder.Code != tt.want {
					t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tt.want, recorder.Body.String())
				}
			})
		}
	})
}

func TestStatusForResourceRequestErrorTreatsUnknownErrorsAsBadRequests(t *testing.T) {
	t.Parallel()

	if got := statusForResourceRequestError(errors.New("boom")); got != http.StatusBadRequest {
		t.Fatalf("statusForResourceRequestError(boom) = %d, want %d", got, http.StatusBadRequest)
	}
}

func TestParseResourceFilterRequiresContext(t *testing.T) {
	t.Parallel()

	_, err := ParseResourceFilter(nil)
	if err == nil {
		t.Fatal("ParseResourceFilter(nil) error = nil, want non-nil")
	}
	if !errors.Is(err, resources.ErrValidation) {
		t.Fatalf("ParseResourceFilter(nil) error = %v, want ErrValidation", err)
	}
}

func TestParseResourcePathRejectsMissingID(t *testing.T) {
	t.Parallel()

	ctx := newResourceTestContext(t, http.MethodGet, "/api/resources/bundle.activation")
	ctx.Params = gin.Params{{Key: "kind", Value: "bundle.activation"}}

	_, _, err := parseResourcePath(ctx)
	if err == nil {
		t.Fatal("parseResourcePath() error = nil, want non-nil")
	}
	if !errors.Is(err, resources.ErrValidation) {
		t.Fatalf("parseResourcePath() error = %v, want ErrValidation", err)
	}
}

func TestResourceRecordPayloadsFromRawReturnsEmptySlice(t *testing.T) {
	t.Parallel()

	payloads := ResourceRecordPayloadsFromRaw(nil)
	if payloads == nil {
		t.Fatal("ResourceRecordPayloadsFromRaw(nil) = nil, want empty slice")
	}
	if len(payloads) != 0 {
		t.Fatalf("len(payloads) = %d, want 0", len(payloads))
	}
}

func newResourceTestContext(t *testing.T, method string, target string) *gin.Context {
	t.Helper()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(context.Background(), method, target, http.NoBody)
	return ctx
}

func newResourceRequestContext(
	t *testing.T,
	method string,
	target string,
	body []byte,
	params gin.Params,
) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	var reader io.Reader = http.NoBody
	if body != nil {
		reader = bytes.NewReader(body)
	}
	ctx.Request = httptest.NewRequestWithContext(context.Background(), method, target, reader)
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = params
	return ctx, recorder
}
