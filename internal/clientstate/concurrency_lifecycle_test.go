package clientstate

// Invariant: concurrent writers lose no updates, and workspace generations fence deletion and shutdown.
// Owning layer: client-state service lifecycle. Canonical suite: concurrency_lifecycle_test.go.

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestEngineShouldSerializeConcurrentWriters(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve fifty revisions on each of eight concurrent keys (UT-019)", func(t *testing.T) {
		t.Parallel()
		engine, _, _ := newTestEngine(t, Limits{MaxValueBytes: 1024, MaxKeysPerWorkspace: 16})
		const writers = 8
		const writesPerKey = 50
		var waitGroup sync.WaitGroup
		for writer := range writers {
			waitGroup.Go(func() {
				key := fmt.Sprintf("writer:%d", writer)
				for write := range writesPerKey {
					_, err := engine.Apply(context.Background(), "w1", "os_shell", []Op{{
						Kind: OpPut, Key: key, Value: objectValue(fmt.Sprintf("%d", write)),
					}}, ApplyOptions{})
					if err != nil {
						t.Errorf("Apply(%s write=%d) error = %v", key, write, err)
						return
					}
				}
			})
		}
		waitGroup.Wait()
		for writer := range writers {
			key := fmt.Sprintf("writer:%d", writer)
			entry, err := engine.Get(context.Background(), "w1", "os_shell", key)
			if err != nil {
				t.Fatalf("Get(%s) error = %v", key, err)
			}
			if entry.Rev != writesPerKey {
				t.Fatalf("Get(%s).Rev = %d, want %d", key, entry.Rev, writesPerKey)
			}
		}
	})
}

func TestEngineShouldFenceWorkspaceLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should reject every operation outside its resolver gate (UT-076)", func(t *testing.T) {
		t.Parallel()
		engine, resolver, _ := newTestEngine(t, testLimits())
		ctx := context.Background()
		assertWorkspaceNotFound := func(name string, operation func() error) {
			t.Helper()
			if err := operation(); !errors.Is(err, ErrWorkspaceNotFound) {
				t.Fatalf("%s error = %v, want ErrWorkspaceNotFound", name, err)
			}
		}
		assertWorkspaceNotFound("Get unknown", func() error {
			_, err := engine.Get(ctx, "unknown", "os_shell", "desktop")
			return err
		})
		assertWorkspaceNotFound("List unknown", func() error {
			_, err := engine.List(ctx, "unknown", "os_shell")
			return err
		})
		assertWorkspaceNotFound("Apply unknown", func() error {
			_, err := engine.Apply(ctx, "unknown", "os_shell", []Op{{
				Kind: OpPut, Key: "desktop", Value: objectValue("unknown"),
			}}, ApplyOptions{})
			return err
		})
		assertWorkspaceNotFound("Watch unknown", func() error {
			_, err := engine.Watch(ctx, "unknown", []string{"os_shell"})
			return err
		})
		assertWorkspaceNotFound("Purge unknown", func() error {
			return engine.PurgeWorkspace(ctx, "unknown")
		})
		assertWorkspaceNotFound("Purge active", func() error {
			return engine.PurgeWorkspace(ctx, "w1")
		})
		engine.mu.Lock()
		_, unknownSequencerCreated := engine.sequencers["unknown"]
		engine.mu.Unlock()
		if unknownSequencerCreated {
			t.Fatal("unknown workspace operations created sequencer state")
		}
		resolver.markDeleting("w1")
		assertWorkspaceNotFound("Get deleting", func() error {
			_, err := engine.Get(ctx, "w1", "os_shell", "desktop")
			return err
		})
		assertWorkspaceNotFound("List deleting", func() error {
			_, err := engine.List(ctx, "w1", "os_shell")
			return err
		})
		assertWorkspaceNotFound("Apply deleting", func() error {
			_, err := engine.Apply(ctx, "w1", "os_shell", []Op{{
				Kind: OpPut, Key: "desktop", Value: objectValue("deleting"),
			}}, ApplyOptions{})
			return err
		})
		assertWorkspaceNotFound("Watch deleting", func() error {
			_, err := engine.Watch(ctx, "w1", []string{"os_shell"})
			return err
		})
	})

	t.Run("Should serialize an apply racing a deletion-gated purge (UT-077)", func(t *testing.T) {
		t.Parallel()
		base := newFakeWorkspaceResolver()
		base.register("w1", "generation-1")
		resolver := &armedBlockingResolver{
			base:    base,
			entered: make(chan struct{}),
			release: make(chan struct{}),
		}
		engine, err := Open(DatabasePath(t.TempDir()), resolver, testLimits())
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		t.Cleanup(func() {
			if err := engine.Close(); err != nil {
				t.Errorf("Close() error = %v", err)
			}
		})
		subscription, err := engine.Watch(context.Background(), "w1", []string{"os_shell"})
		if err != nil {
			t.Fatalf("Watch() error = %v", err)
		}
		resolver.armExecutionCheck()
		applyDone := make(chan error, 1)
		go func() {
			_, applyErr := engine.Apply(context.Background(), "w1", "os_shell", []Op{{
				Kind: OpPut, Key: "desktop", Value: objectValue("racing"),
			}}, ApplyOptions{})
			applyDone <- applyErr
		}()
		select {
		case <-resolver.entered:
		case <-time.After(2 * time.Second):
			t.Fatal("apply did not reach its execution-time deletion gate")
		}
		base.markDeleting("w1")
		purgeDone := make(chan error, 1)
		go func() {
			purgeDone <- engine.PurgeWorkspace(context.Background(), "w1")
		}()
		close(resolver.release)
		if err := <-applyDone; !errors.Is(err, ErrWorkspaceNotFound) {
			t.Fatalf("racing Apply() error = %v, want ErrWorkspaceNotFound", err)
		}
		if err := <-purgeDone; err != nil {
			t.Fatalf("PurgeWorkspace() error = %v", err)
		}
		select {
		case _, ok := <-subscription.Events():
			if ok {
				t.Fatal("subscription remained open after purge")
			}
		case <-time.After(2 * time.Second):
			t.Fatal("subscription did not close after purge")
		}
		if err := engine.PurgeWorkspace(context.Background(), "w1"); err != nil {
			t.Fatalf("second PurgeWorkspace() error = %v", err)
		}
		base.register("w1", "generation-2")
		if entries, err := engine.List(context.Background(), "w1", "os_shell"); err != nil || len(entries) != 0 {
			t.Fatalf("List(recreated) = %#v, %v; want empty", entries, err)
		}
	})

	t.Run("Should reject delayed old-generation writes and allow clean recreation (UT-077)", func(t *testing.T) {
		t.Parallel()
		base := newFakeWorkspaceResolver()
		base.register("w1", "generation-1")
		resolver := &blockingSecondResolve{
			base:    base,
			entered: make(chan struct{}),
			release: make(chan struct{}),
		}
		engine, err := Open(DatabasePath(t.TempDir()), resolver, testLimits())
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		t.Cleanup(func() {
			if err := engine.Close(); err != nil {
				t.Errorf("Close() error = %v", err)
			}
		})
		applyDone := make(chan error, 1)
		go func() {
			_, applyErr := engine.Apply(context.Background(), "w1", "os_shell", []Op{{
				Kind: OpPut, Key: "desktop", Value: objectValue("old-generation"),
			}}, ApplyOptions{})
			applyDone <- applyErr
		}()
		select {
		case <-resolver.entered:
		case <-time.After(2 * time.Second):
			t.Fatal("apply did not reach execution-time generation check")
		}
		base.register("w1", "generation-2")
		close(resolver.release)
		if err := <-applyDone; !errors.Is(err, ErrWorkspaceNotFound) {
			t.Fatalf("delayed Apply() error = %v, want ErrWorkspaceNotFound", err)
		}
		if _, err := engine.Get(context.Background(), "w1", "os_shell", "desktop"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("Get(recreated) error = %v, want empty ErrNotFound", err)
		}
		created := applyPut(t, engine, "w1", "os_shell", "desktop", objectValue("new-generation"), 0, ApplyOptions{})
		if created.Rev != 1 || created.Seq != 1 {
			t.Fatalf("created = %#v, want fresh rev=1 seq=1", created)
		}
		base.markDeleting("w1")
		if err := engine.PurgeWorkspace(context.Background(), "w1"); err != nil {
			t.Fatalf("PurgeWorkspace() error = %v", err)
		}
		if err := engine.PurgeWorkspace(context.Background(), "w1"); err != nil {
			t.Fatalf("second PurgeWorkspace() error = %v", err)
		}
		base.remove("w1")
		base.register("w1", "generation-3")
		if entries, err := engine.List(context.Background(), "w1", "os_shell"); err != nil || len(entries) != 0 {
			t.Fatalf("List(second recreation) = %#v, %v; want empty", entries, err)
		}
	})

	t.Run("Should close service and subscriptions idempotently (UT-089)", func(t *testing.T) {
		t.Parallel()
		engine, _, _ := newTestEngine(t, testLimits())
		subscription, err := engine.Watch(context.Background(), "w1", []string{"os_shell"})
		if err != nil {
			t.Fatalf("Watch() error = %v", err)
		}
		closeErrors := make(chan error, 8)
		var closeGroup sync.WaitGroup
		for range 8 {
			closeGroup.Go(func() {
				closeErrors <- engine.Close()
			})
		}
		closeGroup.Wait()
		close(closeErrors)
		for err := range closeErrors {
			if err != nil {
				t.Fatalf("concurrent Close() error = %v", err)
			}
		}
		if err := engine.Close(); err != nil {
			t.Fatalf("second Close() error = %v", err)
		}
		if err := subscription.Close(); err != nil {
			t.Fatalf("Subscription.Close() error = %v", err)
		}
		if err := subscription.Close(); err != nil {
			t.Fatalf("second Subscription.Close() error = %v", err)
		}
		operations := []struct {
			name string
			run  func() error
		}{
			{name: "Get", run: func() error {
				_, err := engine.Get(context.Background(), "w1", "os_shell", "desktop")
				return err
			}},
			{name: "List", run: func() error {
				_, err := engine.List(context.Background(), "w1", "os_shell")
				return err
			}},
			{name: "Apply", run: func() error {
				_, err := engine.Apply(
					context.Background(),
					"w1",
					"os_shell",
					[]Op{{Kind: OpPut, Key: "desktop", Value: objectValue("closed")}},
					ApplyOptions{},
				)
				return err
			}},
			{name: "Watch", run: func() error {
				_, err := engine.Watch(context.Background(), "w1", []string{"os_shell"})
				return err
			}},
			{name: "PurgeWorkspace", run: func() error { return engine.PurgeWorkspace(context.Background(), "w1") }},
		}
		for _, operation := range operations {
			if err := operation.run(); !errors.Is(err, ErrClosed) {
				t.Fatalf("%s after close error = %v, want ErrClosed", operation.name, err)
			}
		}
	})
}

func TestEngineShouldValidateConstructionAndTransportMetrics(t *testing.T) {
	t.Parallel()

	t.Run("Should expose the production default limits", func(t *testing.T) {
		t.Parallel()
		limits := DefaultLimits()
		if limits.MaxValueBytes != 256*1024 || limits.MaxKeysPerWorkspace != 512 {
			t.Fatalf("DefaultLimits() = %#v, want 256KiB/512", limits)
		}
	})

	t.Run("Should reject incomplete construction inputs before opening bbolt", func(t *testing.T) {
		t.Parallel()
		resolver := newFakeWorkspaceResolver()
		resolver.register("w1", "generation-1")
		path := DatabasePath(t.TempDir())
		tests := []struct {
			name string
			open func() (*Engine, error)
		}{
			{name: "Should reject a nil resolver", open: func() (*Engine, error) {
				return Open(path, nil, testLimits())
			}},
			{name: "Should reject a zero value limit", open: func() (*Engine, error) {
				return Open(path, resolver, Limits{MaxKeysPerWorkspace: 1})
			}},
			{name: "Should reject a zero key limit", open: func() (*Engine, error) {
				return Open(path, resolver, Limits{MaxValueBytes: 1})
			}},
			{name: "Should reject a nil logger", open: func() (*Engine, error) {
				return Open(path, resolver, testLimits(), WithLogger(nil))
			}},
			{name: "Should reject a nil clock", open: func() (*Engine, error) {
				return Open(path, resolver, testLimits(), WithClock(nil))
			}},
			{name: "Should reject a zero open timeout", open: func() (*Engine, error) {
				return Open(path, resolver, testLimits(), WithOpenTimeout(0))
			}},
			{name: "Should reject an empty database path", open: func() (*Engine, error) {
				return Open("", resolver, testLimits())
			}},
		}
		for _, test := range tests {
			if engine, err := test.open(); err == nil {
				if closeErr := engine.Close(); closeErr != nil {
					t.Errorf("Close(%s) error = %v", test.name, closeErr)
				}
				t.Fatalf("Open(%s) error = nil, want validation failure", test.name)
			}
		}
	})

	t.Run("Should bound connection gauges and count reconnects", func(t *testing.T) {
		t.Parallel()
		engine, _, _ := newTestEngine(t, testLimits())
		engine.ConnectionOpened()
		engine.ConnectionOpened()
		engine.ConnectionClosed()
		engine.ConnectionClosed()
		engine.ConnectionClosed()
		engine.RecordReconnect()
		metrics := engine.Metrics()
		if metrics.ActiveConnections != 0 || metrics.Reconnects != 1 {
			t.Fatalf("Metrics() = %#v, want active_connections=0 reconnects=1", metrics)
		}
	})

	t.Run("Should report zero latency percentiles before observing operations", func(t *testing.T) {
		t.Parallel()
		engine, _, _ := newTestEngine(t, testLimits())
		metrics := engine.Metrics()
		if metrics.ApplyLatencyP95 != 0 || metrics.SnapshotDurationP95 != 0 {
			t.Fatalf("Metrics() = %#v, want zero unobserved latency percentiles", metrics)
		}
	})

	t.Run("Should close a subscription when its context is canceled", func(t *testing.T) {
		t.Parallel()
		engine, _, _ := newTestEngine(t, testLimits())
		ctx, cancel := context.WithCancel(context.Background())
		subscription, err := engine.Watch(ctx, "w1", []string{"os_shell"})
		if err != nil {
			t.Fatalf("Watch() error = %v", err)
		}
		cancel()
		select {
		case _, ok := <-subscription.Events():
			if ok {
				t.Fatal("subscription emitted an event after context cancellation")
			}
		case <-time.After(2 * time.Second):
			t.Fatal("subscription remained open after context cancellation")
		}
		if metrics := engine.Metrics(); metrics.ActiveSubscriptions != 0 {
			t.Fatalf("ActiveSubscriptions = %d, want 0", metrics.ActiveSubscriptions)
		}
	})
}

type blockingSecondResolve struct {
	base    *fakeWorkspaceResolver
	calls   atomic.Int32
	entered chan struct{}
	release chan struct{}
}

type armedBlockingResolver struct {
	base    *fakeWorkspaceResolver
	calls   atomic.Int32
	blockAt atomic.Int32
	entered chan struct{}
	release chan struct{}
}

func (r *armedBlockingResolver) armExecutionCheck() {
	r.blockAt.Store(r.calls.Load() + 2)
}

func (r *armedBlockingResolver) ResolveWorkspace(
	ctx context.Context,
	ws WorkspaceID,
) (WorkspaceGeneration, error) {
	if call := r.calls.Add(1); call == r.blockAt.Load() {
		close(r.entered)
		select {
		case <-r.release:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return r.base.ResolveWorkspace(ctx, ws)
}

func (r *armedBlockingResolver) ResolveWorkspaceForPurge(
	ctx context.Context,
	ws WorkspaceID,
) (WorkspaceGeneration, error) {
	return r.base.ResolveWorkspaceForPurge(ctx, ws)
}

func (r *blockingSecondResolve) ResolveWorkspace(
	ctx context.Context,
	ws WorkspaceID,
) (WorkspaceGeneration, error) {
	if r.calls.Add(1) == 2 {
		close(r.entered)
		select {
		case <-r.release:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return r.base.ResolveWorkspace(ctx, ws)
}

func (r *blockingSecondResolve) ResolveWorkspaceForPurge(
	ctx context.Context,
	ws WorkspaceID,
) (WorkspaceGeneration, error) {
	return r.base.ResolveWorkspaceForPurge(ctx, ws)
}
