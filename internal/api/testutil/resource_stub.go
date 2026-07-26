package testutil

import (
	"context"
	"time"

	core "github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/resources"
)

const (
	resourceStubDaemonKey = "daemon"
)

type StubResourceService struct {
	ListFn   func(context.Context, resources.ResourceFilter) ([]resources.RawRecord, error)
	GetFn    func(context.Context, resources.ResourceKind, string) (resources.RawRecord, error)
	PutFn    func(context.Context, resources.RawDraft) (resources.RawRecord, error)
	DeleteFn func(context.Context, resources.ResourceKind, string, int64) error
}

func (s StubResourceService) List(
	ctx context.Context,
	filter resources.ResourceFilter,
) ([]resources.RawRecord, error) {
	if s.ListFn != nil {
		return s.ListFn(ctx, filter)
	}
	return nil, nil
}

func (s StubResourceService) Get(
	ctx context.Context,
	kind resources.ResourceKind,
	id string,
) (resources.RawRecord, error) {
	if s.GetFn != nil {
		return s.GetFn(ctx, kind, id)
	}
	return resources.RawRecord{}, resources.ErrNotFound
}

func (s StubResourceService) Put(
	ctx context.Context,
	draft resources.RawDraft,
) (resources.RawRecord, error) {
	if s.PutFn != nil {
		return s.PutFn(ctx, draft)
	}
	return resources.RawRecord{
		Kind:      draft.Kind,
		ID:        draft.ID,
		Version:   1,
		Scope:     draft.Scope,
		Owner:     resources.ResourceOwner{Kind: resourceStubDaemonKey, ID: "daemon-control"},
		Source:    resources.ResourceSource{Kind: resourceStubDaemonKey, ID: "system"},
		SpecJSON:  append([]byte(nil), draft.SpecJSON...),
		CreatedAt: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
	}, nil
}

func (s StubResourceService) Delete(
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

var _ core.ResourceService = (*StubResourceService)(nil)
