package resources

import (
	"context"
	"sync"
	"testing"
	"time"
)

const shortWait = 500 * time.Millisecond

func TestMemoryStore_PutGetDeepCopy(t *testing.T) {
	ctx := context.TODO()
	st := NewMemoryResourceStore()
	key := ResourceKey{Project: "p1", Type: ResourceAgent, ID: "writer"}
	orig := map[string]any{"id": "writer", "cfg": map[string]any{"a": 1.0}}
	if _, err := st.Put(ctx, key, orig); err != nil {
		t.Fatalf("put failed: %v", err)
	}
	orig["id"] = "mutated"
	got, etag, err := st.Get(ctx, key)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if etag == "" {
		t.Fatalf("expected etag")
	}
	m := got.(map[string]any)
	if m["id"].(string) != "writer" {
		t.Fatalf("expected deep copy to preserve original, got: %v", m["id"])
	}
	m["id"] = "changed"
	got2, _, err := st.Get(ctx, key)
	if err != nil {
		t.Fatalf("get2 failed: %v", err)
	}
	m2 := got2.(map[string]any)
	if m2["id"].(string) != "writer" {
		t.Fatalf("store mutated by client copy: %v", m2["id"])
	}
}

func TestMemoryStore_ListAndDelete(t *testing.T) {
	ctx := context.TODO()
	st := NewMemoryResourceStore()
	a := ResourceKey{Project: "p1", Type: ResourceTool, ID: "browser"}
	b := ResourceKey{Project: "p1", Type: ResourceTool, ID: "search"}
	c := ResourceKey{Project: "p2", Type: ResourceTool, ID: "x"}
	_, _ = st.Put(ctx, a, map[string]any{"id": "browser"})
	_, _ = st.Put(ctx, b, map[string]any{"id": "search"})
	_, _ = st.Put(ctx, c, map[string]any{"id": "x"})
	keys, err := st.List(ctx, "p1", ResourceTool)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
	if err := st.Delete(ctx, a); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if err := st.Delete(ctx, a); err != nil {
		t.Fatalf("delete idempotent failed: %v", err)
	}
}

func TestMemoryStore_Watch_PrimeAndEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	st := NewMemoryResourceStore()
	key := ResourceKey{Project: "p1", Type: ResourceModel, ID: "gpt"}
	_, _ = st.Put(ctx, key, map[string]any{"id": "gpt"})
	ch, err := st.Watch(ctx, "p1", ResourceModel)
	if err != nil {
		t.Fatalf("watch failed: %v", err)
	}
	select {
	case e := <-ch:
		if e.Type != EventPut || e.Key != key {
			t.Fatalf("unexpected prime event: %#v", e)
		}
	case <-time.After(shortWait):
		t.Fatalf("timeout waiting prime event")
	}
	_, _ = st.Put(ctx, key, map[string]any{"id": "gpt", "v": 2.0})
	select {
	case e := <-ch:
		if e.Type != EventPut || e.Key != key {
			t.Fatalf("unexpected put event: %#v", e)
		}
	case <-time.After(shortWait):
		t.Fatalf("timeout waiting put event")
	}
	if err := st.Delete(ctx, key); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	select {
	case e := <-ch:
		if e.Type != EventDelete || e.Key != key {
			t.Fatalf("unexpected delete event: %#v", e)
		}
	case <-time.After(shortWait):
		t.Fatalf("timeout waiting delete event")
	}
}

func TestMemoryStore_Watch_StopOnCancel(t *testing.T) {
	base := context.TODO()
	st := NewMemoryResourceStore()
	ctx, cancel := context.WithCancel(base)
	ch, err := st.Watch(ctx, "p1", ResourceSchema)
	if err != nil {
		t.Fatalf("watch failed: %v", err)
	}
	cancel()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatalf("expected channel closed")
		}
	case <-time.After(shortWait):
		t.Fatalf("timeout waiting channel close")
	}
}

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	ctx := context.TODO()
	st := NewMemoryResourceStore()
	project := "p1"
	typ := ResourceTask
	var wg sync.WaitGroup
	ch, err := st.Watch(ctx, project, typ)
	if err != nil {
		t.Fatalf("watch failed: %v", err)
	}
	for range 4 {
		wg.Go(func() {
			for j := range 200 {
				k := ResourceKey{Project: project, Type: typ, ID: "k"}
				switch j % 3 {
				case 0:
					_, _ = st.Put(ctx, k, map[string]any{"n": j})
				case 1:
					_, _, _ = st.Get(ctx, k)
				default:
					_ = st.Delete(ctx, k)
				}
			}
		})
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		count := 0
		deadline := time.After(2 * time.Second)
		for {
			select {
			case <-ch:
				count++
				if count > 0 {
					return
				}
			case <-deadline:
				return
			}
		}
	}()
	wg.Wait()
	<-done
}
