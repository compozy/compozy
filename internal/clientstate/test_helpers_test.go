package clientstate

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type resolverWorkspace struct {
	generation WorkspaceGeneration
	deleting   bool
}

type fakeWorkspaceResolver struct {
	mu         sync.RWMutex
	workspaces map[WorkspaceID]resolverWorkspace
}

func newFakeWorkspaceResolver() *fakeWorkspaceResolver {
	return &fakeWorkspaceResolver{workspaces: make(map[WorkspaceID]resolverWorkspace)}
}

func (r *fakeWorkspaceResolver) register(ws WorkspaceID, generation WorkspaceGeneration) {
	r.mu.Lock()
	r.workspaces[ws] = resolverWorkspace{generation: generation}
	r.mu.Unlock()
}

func (r *fakeWorkspaceResolver) markDeleting(ws WorkspaceID) {
	r.mu.Lock()
	workspace := r.workspaces[ws]
	workspace.deleting = true
	r.workspaces[ws] = workspace
	r.mu.Unlock()
}

func (r *fakeWorkspaceResolver) remove(ws WorkspaceID) {
	r.mu.Lock()
	delete(r.workspaces, ws)
	r.mu.Unlock()
}

func (r *fakeWorkspaceResolver) ResolveWorkspace(
	_ context.Context,
	ws WorkspaceID,
) (WorkspaceGeneration, error) {
	r.mu.RLock()
	workspace, exists := r.workspaces[ws]
	r.mu.RUnlock()
	if !exists || workspace.deleting {
		return "", ErrWorkspaceNotFound
	}
	return workspace.generation, nil
}

func (r *fakeWorkspaceResolver) ResolveWorkspaceForPurge(
	_ context.Context,
	ws WorkspaceID,
) (WorkspaceGeneration, error) {
	r.mu.RLock()
	workspace, exists := r.workspaces[ws]
	r.mu.RUnlock()
	if !exists || !workspace.deleting {
		return "", ErrWorkspaceNotFound
	}
	return workspace.generation, nil
}

func newTestEngine(
	t *testing.T,
	limits Limits,
) (*Engine, *fakeWorkspaceResolver, string) {
	t.Helper()
	resolver := newFakeWorkspaceResolver()
	resolver.register("w1", "w1-generation-1")
	resolver.register("w2", "w2-generation-1")
	path := filepath.Join(t.TempDir(), "state", DatabaseName)
	fixedNow := time.Date(2026, 7, 19, 12, 0, 0, 123, time.UTC)
	engine, err := Open(path, resolver, limits, WithClock(func() time.Time { return fixedNow }))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Errorf("Engine.Close() error = %v", err)
		}
	})
	return engine, resolver, path
}

func testLimits() Limits {
	return Limits{MaxValueBytes: 1024, MaxKeysPerWorkspace: 32}
}

func applyPut(
	t *testing.T,
	engine *Engine,
	ws WorkspaceID,
	domain string,
	key string,
	value []byte,
	ifRev uint64,
	options ApplyOptions,
) Entry {
	t.Helper()
	entries, err := engine.Apply(context.Background(), ws, domain, []Op{{
		Kind: OpPut, Key: key, Value: value, IfRev: ifRev,
	}}, options)
	if err != nil {
		t.Fatalf("Apply(put %s/%s) error = %v", domain, key, err)
	}
	if len(entries) != 1 {
		t.Fatalf("Apply(put %s/%s) returned %d entries, want 1", domain, key, len(entries))
	}
	return entries[0]
}

func applyDelete(
	t *testing.T,
	engine *Engine,
	ws WorkspaceID,
	domain string,
	key string,
	ifRev uint64,
) Entry {
	t.Helper()
	entries, err := engine.Apply(context.Background(), ws, domain, []Op{{
		Kind: OpDelete, Key: key, IfRev: ifRev,
	}}, ApplyOptions{})
	if err != nil {
		t.Fatalf("Apply(delete %s/%s) error = %v", domain, key, err)
	}
	if len(entries) != 1 {
		t.Fatalf("Apply(delete %s/%s) returned %d entries, want 1", domain, key, len(entries))
	}
	return entries[0]
}

func receiveEvent(t *testing.T, subscription Subscription) Event {
	t.Helper()
	select {
	case event, ok := <-subscription.Events():
		if !ok {
			t.Fatalf("subscription closed before event: %v", subscription.Err())
		}
		return event
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for subscription event")
		return Event{}
	}
}

func waitForSubscriptionError(t *testing.T, subscription Subscription, target error) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		if errors.Is(subscription.Err(), target) {
			return
		}
		select {
		case _, ok := <-subscription.Events():
			if !ok && !errors.Is(subscription.Err(), target) {
				t.Fatalf("subscription error = %v, want %v", subscription.Err(), target)
			}
		case <-deadline:
			t.Fatalf("subscription error = %v, want %v", subscription.Err(), target)
		}
	}
}

func objectValue(label string) []byte {
	return fmt.Appendf(nil, `{"value":%q}`, label)
}
