package resources

import (
	"context"
	"errors"
	"testing"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/require"
)

func testCtx() context.Context {
	return logger.ContextWithLogger(context.Background(), logger.NewForTests())
}

func TestWriteMetaAndGetMetaSource(t *testing.T) {
	ctx := testCtx()
	st := NewMemoryResourceStore()
	require.NoError(t, WriteMeta(ctx, st, "p", ResourceAgent, "a", "api", "tester"))
	src := GetMetaSource(ctx, st, "p", ResourceAgent, "a")
	require.Equal(t, "api", src)
}

func TestWriteMetaForAutoload(t *testing.T) {
	ctx := testCtx()
	st := NewMemoryResourceStore()
	require.NoError(t, WriteMetaForAutoload(ctx, st, "p", ResourceTool, "t"))
	v, _, err := st.Get(ctx, ResourceKey{Project: "p", Type: ResourceMeta, ID: "p:tool:t"})
	require.NoError(t, err)
	m := v.(map[string]any)
	require.Equal(t, "autoload", m["source"].(string))
}

func TestIndexPutWithMeta_ConflictAndErrors(t *testing.T) {
	t.Run("Should log conflict when prior source differs and update meta", func(t *testing.T) {
		ctx := testCtx()
		st := NewMemoryResourceStore()
		require.NoError(t, WriteMeta(ctx, st, "p", ResourceSchema, "s", "yaml", "y"))
		err := IndexPutWithMeta(ctx, st, "p", ResourceSchema, "s", map[string]any{"id": "s"}, "autoload", "a")
		require.NoError(t, err)
		v, _, err := st.Get(ctx, ResourceKey{Project: "p", Type: ResourceMeta, ID: "p:schema:s"})
		require.NoError(t, err)
		m := v.(map[string]any)
		require.Equal(t, "autoload", m["source"].(string))
	})
	t.Run("Should return error when value put fails", func(t *testing.T) {
		ctx := testCtx()
		st := &failingPutStore{err: errors.New("put-fail")}
		err := IndexPutWithMeta(ctx, st, "p", ResourceAgent, "a", map[string]any{"id": "a"}, "api", "u")
		require.Error(t, err)
	})
	t.Run("Should return error when meta write fails", func(t *testing.T) {
		ctx := testCtx()
		st := &failingMetaOnlyStore{inner: NewMemoryResourceStore()}
		err := IndexPutWithMeta(ctx, st, "p", ResourceAgent, "a", map[string]any{"id": "a"}, "api", "u")
		require.Error(t, err)
	})
}

type failingPutStore struct{ err error }

func (f *failingPutStore) Put(context.Context, ResourceKey, any) (string, error) { return "", f.err }
func (f *failingPutStore) PutIfMatch(context.Context, ResourceKey, any, string) (string, error) {
	return "", f.err
}
func (f *failingPutStore) Get(context.Context, ResourceKey) (any, string, error) {
	return nil, "", ErrNotFound
}
func (f *failingPutStore) Delete(context.Context, ResourceKey) error { return nil }
func (f *failingPutStore) List(context.Context, string, ResourceType) ([]ResourceKey, error) {
	return nil, nil
}
func (f *failingPutStore) Watch(context.Context, string, ResourceType) (<-chan Event, error) {
	return nil, nil
}
func (f *failingPutStore) ListWithValues(context.Context, string, ResourceType) ([]StoredItem, error) {
	return nil, nil
}

func (f *failingPutStore) ListWithValuesPage(
	context.Context,
	string,
	ResourceType,
	int,
	int,
) ([]StoredItem, int, error) {
	return nil, 0, nil
}
func (f *failingPutStore) Close() error { return nil }

type failingMetaOnlyStore struct{ inner ResourceStore }

func (s *failingMetaOnlyStore) Put(ctx context.Context, key ResourceKey, value any) (string, error) {
	if key.Type == ResourceMeta {
		return "", errors.New("meta-fail")
	}
	return s.inner.Put(ctx, key, value)
}
func (s *failingMetaOnlyStore) PutIfMatch(
	ctx context.Context,
	key ResourceKey,
	value any,
	expectedETag string,
) (string, error) {
	if key.Type == ResourceMeta {
		return "", errors.New("meta-fail")
	}
	return s.inner.PutIfMatch(ctx, key, value, expectedETag)
}
func (s *failingMetaOnlyStore) Get(ctx context.Context, key ResourceKey) (any, string, error) {
	return s.inner.Get(ctx, key)
}
func (s *failingMetaOnlyStore) Delete(ctx context.Context, key ResourceKey) error {
	return s.inner.Delete(ctx, key)
}
func (s *failingMetaOnlyStore) List(ctx context.Context, project string, typ ResourceType) ([]ResourceKey, error) {
	return s.inner.List(ctx, project, typ)
}
func (s *failingMetaOnlyStore) Watch(ctx context.Context, project string, typ ResourceType) (<-chan Event, error) {
	return s.inner.Watch(ctx, project, typ)
}

func (s *failingMetaOnlyStore) ListWithValues(
	ctx context.Context,
	project string,
	typ ResourceType,
) ([]StoredItem, error) {
	return s.inner.ListWithValues(ctx, project, typ)
}

func (s *failingMetaOnlyStore) ListWithValuesPage(
	ctx context.Context,
	project string,
	typ ResourceType,
	offset, limit int,
) ([]StoredItem, int, error) {
	return s.inner.ListWithValuesPage(ctx, project, typ, offset, limit)
}
func (s *failingMetaOnlyStore) Close() error { return s.inner.Close() }
