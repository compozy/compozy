package resources

import (
	"context"
	"fmt"
	"maps"
	"sync"
	"testing"
)

type exampleStore struct {
	mu sync.RWMutex
	m  map[ResourceKey]any
}

func newExampleStore() *exampleStore {
	return &exampleStore{m: make(map[ResourceKey]any)}
}

func (s *exampleStore) Put(_ context.Context, key ResourceKey, value any) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m, ok := value.(map[string]any); ok {
		c := make(map[string]any, len(m))
		maps.Copy(c, m)
		s.m[key] = c
	} else {
		s.m[key] = value
	}
	return "", nil
}

func (s *exampleStore) Get(_ context.Context, key ResourceKey) (any, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.m[key]
	if !ok {
		return nil, "", ErrNotFound
	}
	if m, ok := v.(map[string]any); ok {
		c := make(map[string]any, len(m))
		maps.Copy(c, m)
		return c, "", nil
	}
	return v, "", nil
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

func ExampleResourceStore_basic() {
	ctx := context.Background()
	st := newExampleStore()
	key := ResourceKey{Project: "proj", Type: ResourceAgent, ID: "writer"}
	_, _ = st.Put(ctx, key, map[string]any{"resource": "agent", "id": "writer"})
	v, _, _ := st.Get(ctx, key)
	m, ok := v.(map[string]any)
	if !ok {
		fmt.Println("value is not a map[string]any")
		return
	}
	fmt.Println(m["id"])
	// Output: writer
}

func TestExampleStorePutGet(t *testing.T) {
	ctx := context.Background()
	st := newExampleStore()
	key := ResourceKey{Project: "p", Type: ResourceTool, ID: "browser"}
	if _, err := st.Put(ctx, key, map[string]any{"resource": "tool", "id": "browser"}); err != nil {
		t.Fatalf("put failed: %v", err)
	}
	v, _, err := st.Get(ctx, key)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	m := v.(map[string]any)
	if m["id"].(string) != "browser" {
		t.Fatalf("unexpected value: %v", m)
	}
}
