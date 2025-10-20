package resources

import (
	"context"
	"sync"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type exampleStore struct {
	mu sync.RWMutex
	m  map[ResourceKey]any
}

func newExampleStore() *exampleStore {
	return &exampleStore{m: make(map[ResourceKey]any)}
}

func (s *exampleStore) Put(_ context.Context, key ResourceKey, value any) (ETag, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m, ok := value.(map[string]any); ok {
		s.m[key] = core.CloneMap(m)
	} else {
		s.m[key] = value
	}
	return ETag(core.ETagFromAny(s.m[key])), nil
}

func (s *exampleStore) Get(_ context.Context, key ResourceKey) (any, ETag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.m[key]
	if !ok {
		return nil, ETag(""), ErrNotFound
	}
	if m, ok := v.(map[string]any); ok {
		c := core.CloneMap(m)
		return c, ETag(core.ETagFromAny(c)), nil
	}
	return v, ETag(core.ETagFromAny(v)), nil
}

func (s *exampleStore) PutIfMatch(_ context.Context, key ResourceKey, value any, expectedETag ETag) (ETag, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.m[key]
	if !ok {
		if expectedETag != "" {
			return ETag(""), ErrNotFound
		}
		if m, ok := value.(map[string]any); ok {
			c := core.CloneMap(m)
			s.m[key] = c
		} else {
			s.m[key] = value
		}
		return ETag(core.ETagFromAny(s.m[key])), nil
	}
	if core.ETagFromAny(cur) != string(expectedETag) {
		return ETag(""), ErrETagMismatch
	}
	if m, ok := value.(map[string]any); ok {
		c := core.CloneMap(m)
		s.m[key] = c
	} else {
		s.m[key] = value
	}
	return ETag(core.ETagFromAny(s.m[key])), nil
}

func (s *exampleStore) Delete(_ context.Context, key ResourceKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, key)
	return nil
}

func (s *exampleStore) List(_ context.Context, project string, typ ResourceType) ([]ResourceKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]ResourceKey, 0, len(s.m))
	for k := range s.m {
		if k.Project == project && k.Type == typ {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

func (s *exampleStore) Watch(ctx context.Context, _ string, _ ResourceType) (<-chan Event, error) {
	// NOTE: No-op watcher for example purposes; real stores should broadcast events.
	ch := make(chan Event)
	go func() { <-ctx.Done(); close(ch) }()
	return ch, nil
}

func (s *exampleStore) Close() error { return nil }

func TestResourceStore_basic(t *testing.T) {
	t.Run("Should demonstrate basic put and get operations", func(t *testing.T) {
		ctx := testCtx(t)
		st := newExampleStore()
		key := ResourceKey{Project: "proj", Type: ResourceAgent, ID: "writer"}
		_, err := st.Put(ctx, key, map[string]any{"resource": "agent", "id": "writer"})
		require.NoError(t, err)
		v, _, err := st.Get(ctx, key)
		require.NoError(t, err)
		m, ok := v.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "writer", m["id"])
	})
}

func TestExampleStorePutGet(t *testing.T) {
	t.Run("Should put and get a tool by key", func(t *testing.T) {
		ctx := testCtx(t)
		st := newExampleStore()
		key := ResourceKey{Project: "p", Type: ResourceTool, ID: "browser"}
		_, err := st.Put(ctx, key, map[string]any{"resource": "tool", "id": "browser"})
		require.NoError(t, err)
		v, _, err := st.Get(ctx, key)
		require.NoError(t, err)
		m, ok := v.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "browser", m["id"])
	})
}
