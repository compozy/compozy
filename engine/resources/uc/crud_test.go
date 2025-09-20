package uc

import (
	"context"
	"errors"
	"testing"

	"github.com/compozy/compozy/engine/resources"
	"github.com/stretchr/testify/require"
)

func TestCreateGetDelete_AndMetaErrors(t *testing.T) {
	t.Run("Should create, get and delete successfully", func(t *testing.T) {
		ctx := context.Background()
		store := resources.NewMemoryResourceStore()
		crt := NewCreateResource(store)
		out, err := crt.Execute(
			ctx,
			&CreateInput{
				Project: "p",
				Type:    resources.ResourceAgent,
				Body:    map[string]any{"id": "a", "type": "agent", "instructions": "x"},
			},
		)
		require.NoError(t, err)
		require.Equal(t, "a", out.ID)
		getUC := NewGetResource(store)
		g, err := getUC.Execute(ctx, &GetInput{Project: "p", Type: resources.ResourceAgent, ID: "a"})
		require.NoError(t, err)
		require.NotEmpty(t, g.ETag)
		delUC := NewDeleteResource(store)
		require.NoError(t, delUC.Execute(ctx, &DeleteInput{Project: "p", Type: resources.ResourceAgent, ID: "a"}))
		_, err = getUC.Execute(ctx, &GetInput{Project: "p", Type: resources.ResourceAgent, ID: "a"})
		require.ErrorIs(t, err, ErrNotFound)
	})
	t.Run("Should return meta write error from Create", func(t *testing.T) {
		ctx := context.Background()
		fs := &failingMetaStore{inner: resources.NewMemoryResourceStore()}
		crt := NewCreateResource(fs)
		_, err := crt.Execute(
			ctx,
			&CreateInput{
				Project: "p",
				Type:    resources.ResourceAgent,
				Body:    map[string]any{"id": "a", "type": "agent", "instructions": "x"},
			},
		)
		require.Error(t, err)
	})
}

type failingMetaStore struct{ inner resources.ResourceStore }

func (s *failingMetaStore) Put(ctx context.Context, key resources.ResourceKey, value any) (resources.ETag, error) {
	if key.Type == resources.ResourceMeta {
		return resources.ETag(""), errAssert
	}
	return s.inner.Put(ctx, key, value)
}
func (s *failingMetaStore) PutIfMatch(
	ctx context.Context,
	key resources.ResourceKey,
	value any,
	expectedETag resources.ETag,
) (resources.ETag, error) {
	if key.Type == resources.ResourceMeta {
		return resources.ETag(""), errAssert
	}
	return s.inner.PutIfMatch(ctx, key, value, expectedETag)
}
func (s *failingMetaStore) Get(ctx context.Context, key resources.ResourceKey) (any, resources.ETag, error) {
	return s.inner.Get(ctx, key)
}
func (s *failingMetaStore) Delete(ctx context.Context, key resources.ResourceKey) error {
	return s.inner.Delete(ctx, key)
}

func (s *failingMetaStore) List(
	ctx context.Context,
	project string,
	typ resources.ResourceType,
) ([]resources.ResourceKey, error) {
	return s.inner.List(ctx, project, typ)
}

func (s *failingMetaStore) Watch(
	ctx context.Context,
	project string,
	typ resources.ResourceType,
) (<-chan resources.Event, error) {
	return s.inner.Watch(ctx, project, typ)
}

func (s *failingMetaStore) ListWithValues(
	ctx context.Context,
	project string,
	typ resources.ResourceType,
) ([]resources.StoredItem, error) {
	return s.inner.ListWithValues(ctx, project, typ)
}

func (s *failingMetaStore) ListWithValuesPage(
	ctx context.Context,
	project string,
	typ resources.ResourceType,
	offset, limit int,
) ([]resources.StoredItem, int, error) {
	return s.inner.ListWithValuesPage(ctx, project, typ, offset, limit)
}
func (s *failingMetaStore) Close() error { return s.inner.Close() }

var errAssert = errors.New("assert-err")

func TestUpsert_IfMatchPaths(t *testing.T) {
	t.Run("Should return stale when If-Match and missing", func(t *testing.T) {
		ctx := context.Background()
		store := resources.NewMemoryResourceStore()
		put := NewUpsertResource(store)
		_, err := put.Execute(
			ctx,
			&UpsertInput{
				Project: "p",
				Type:    resources.ResourceTool,
				ID:      "t",
				Body:    map[string]any{"id": "t", "type": "tool"},
				IfMatch: "etag",
			},
		)
		require.ErrorIs(t, err, ErrIfMatchStaleOrMissing)
	})
	t.Run("Should return etag mismatch when different", func(t *testing.T) {
		ctx := context.Background()
		store := resources.NewMemoryResourceStore()
		crt := NewCreateResource(store)
		out, err := crt.Execute(
			ctx,
			&CreateInput{Project: "p", Type: resources.ResourceTool, Body: map[string]any{"id": "t", "type": "tool"}},
		)
		require.NoError(t, err)
		put := NewUpsertResource(store)
		_, err = put.Execute(
			ctx,
			&UpsertInput{
				Project: "p",
				Type:    resources.ResourceTool,
				ID:      "t",
				Body:    map[string]any{"id": "t", "type": "tool"},
				IfMatch: string(out.ETag) + "x",
			},
		)
		require.ErrorIs(t, err, ErrETagMismatch)
	})
	t.Run("Should update when If-Match matches and write meta", func(t *testing.T) {
		ctx := context.Background()
		store := resources.NewMemoryResourceStore()
		crt := NewCreateResource(store)
		out, err := crt.Execute(
			ctx,
			&CreateInput{Project: "p", Type: resources.ResourceTool, Body: map[string]any{"id": "t", "type": "tool"}},
		)
		require.NoError(t, err)
		put := NewUpsertResource(store)
		up, err := put.Execute(
			ctx,
			&UpsertInput{
				Project: "p",
				Type:    resources.ResourceTool,
				ID:      "t",
				Body:    map[string]any{"id": "t", "type": "tool", "v": 2.0},
				IfMatch: string(out.ETag),
			},
		)
		require.NoError(t, err)
		require.NotEmpty(t, up.ETag)
	})
}

func TestList_ErrorsAndFilter(t *testing.T) {
	t.Run("Should propagate store error", func(t *testing.T) {
		uc := NewListResources(&errListStore{})
		_, err := uc.Execute(context.Background(), &ListInput{Project: "p", Type: resources.ResourceAgent, Prefix: "a"})
		require.Error(t, err)
	})
	t.Run("Should filter by prefix", func(t *testing.T) {
		ctx := context.Background()
		s := resources.NewMemoryResourceStore()
		_, _ = s.Put(
			ctx,
			resources.ResourceKey{Project: "p", Type: resources.ResourceAgent, ID: "a1"},
			map[string]any{"id": "a1"},
		)
		_, _ = s.Put(
			ctx,
			resources.ResourceKey{Project: "p", Type: resources.ResourceAgent, ID: "a2"},
			map[string]any{"id": "a2"},
		)
		_, _ = s.Put(
			ctx,
			resources.ResourceKey{Project: "p", Type: resources.ResourceAgent, ID: "x"},
			map[string]any{"id": "x"},
		)
		uc := NewListResources(s)
		out, err := uc.Execute(ctx, &ListInput{Project: "p", Type: resources.ResourceAgent, Prefix: "a"})
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"a1", "a2"}, out.Keys)
	})
}

type errListStore struct{ resources.MemoryResourceStore }

func (e *errListStore) List(context.Context, string, resources.ResourceType) ([]resources.ResourceKey, error) {
	return nil, errAssert
}
