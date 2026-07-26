package bridgesdk

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/subprocess"
)

type providerReconcileTestConfig struct {
	InstanceID string
	Path       string
	Generation int
	Conflict   bool
}

func TestManagedConfigReconcilerPreservesRouteTransitions(t *testing.T) {
	t.Parallel()

	t.Run("Should add remove detect path conflicts and atomically swap routes", func(t *testing.T) {
		t.Parallel()

		routes := NewRouteTable(func(config providerReconcileTestConfig) []string {
			return []string{config.Path}
		})
		removed := make([]string, 0)
		published := false
		reconciler := ManagedConfigReconciler[providerReconcileTestConfig]{
			Routes: routes,
			Resolve: func(_ *Session, managed subprocess.InitializeBridgeManagedInstance) providerReconcileTestConfig {
				return providerReconcileTestConfig{
					InstanceID: managed.Instance.ID,
					Path:       "/shared",
					Generation: 1,
				}
			},
			Prepare: func(
				_ context.Context,
				_ *Session,
				configs []providerReconcileTestConfig,
			) ([]providerReconcileTestConfig, error) {
				seen := make(map[string]int)
				for index := range configs {
					if previous, ok := seen[configs[index].Path]; ok {
						configs[previous].Conflict = true
						configs[index].Conflict = true
					}
					seen[configs[index].Path] = index
				}
				return configs, nil
			},
			Finalize: func(
				_ context.Context,
				_ *Session,
				configs []providerReconcileTestConfig,
			) ([]providerReconcileTestConfig, error) {
				for index := range configs {
					configs[index].Generation++
				}
				return configs, nil
			},
			Identity: func(config providerReconcileTestConfig) string { return config.InstanceID },
			Merge: func(previous providerReconcileTestConfig, next providerReconcileTestConfig) providerReconcileTestConfig {
				next.Generation = previous.Generation + 1
				return next
			},
			OnRemoved: func(config providerReconcileTestConfig) error {
				removed = append(removed, config.InstanceID)
				return nil
			},
			OnPublish: func() { published = true },
		}

		first, err := reconciler.Reconcile(t.Context(), nil, []subprocess.InitializeBridgeManagedInstance{
			{Instance: testBridgeInstance("brg-a")},
			{Instance: testBridgeInstance("brg-b")},
		})
		if err != nil {
			t.Fatalf("Reconcile(first) error = %v", err)
		}
		if len(first) != 2 || !first[0].Conflict || !first[1].Conflict {
			t.Fatalf("first configs = %#v, want symmetric path conflict", first)
		}
		if !published {
			t.Fatal("OnPublish() was not called")
		}
		if got := routes.ByPath("shared"); len(got) != 2 {
			t.Fatalf("ByPath(shared) len = %d, want 2", len(got))
		}

		second, err := reconciler.Reconcile(t.Context(), nil, []subprocess.InitializeBridgeManagedInstance{
			{Instance: testBridgeInstance("brg-b")},
			{Instance: testBridgeInstance("brg-c")},
		})
		if err != nil {
			t.Fatalf("Reconcile(second) error = %v", err)
		}
		if len(second) != 2 {
			t.Fatalf("second configs len = %d, want 2", len(second))
		}
		configB, ok := routes.Get("brg-b")
		if !ok || configB.Generation != 4 {
			t.Fatalf("brg-b route = %#v, %t, want retained generation 4", configB, ok)
		}
		if !slices.Equal(removed, []string{"brg-a"}) {
			t.Fatalf("removed = %#v, want [brg-a]", removed)
		}
	})

	t.Run("Should preserve the prior route snapshot when removed-resource cleanup fails", func(t *testing.T) {
		t.Parallel()

		routes := NewRouteTable[providerReconcileTestConfig](nil)
		routes.Replace(map[string]providerReconcileTestConfig{
			"brg-old": {InstanceID: "brg-old", Generation: 1},
		}, nil)
		cleanupErr := errors.New("close listener")
		reconciler := ManagedConfigReconciler[providerReconcileTestConfig]{
			Routes:    routes,
			Identity:  func(config providerReconcileTestConfig) string { return config.InstanceID },
			OnRemoved: func(providerReconcileTestConfig) error { return cleanupErr },
		}

		err := reconciler.Publish([]providerReconcileTestConfig{{InstanceID: "brg-new", Generation: 2}})
		if !errors.Is(err, cleanupErr) {
			t.Fatalf("Publish() error = %v, want cleanup error", err)
		}
		if _, ok := routes.Get("brg-old"); !ok {
			t.Fatal("Get(brg-old) found = false after failed cleanup")
		}
		if _, ok := routes.Get("brg-new"); ok {
			t.Fatal("Get(brg-new) found = true after failed cleanup")
		}
	})
}

func TestRouteTableWaitAndDeliveryStateStore(t *testing.T) {
	t.Parallel()

	t.Run("Should wake route waiters on swap without polling", func(t *testing.T) {
		t.Parallel()

		routes := NewRouteTable[providerReconcileTestConfig](nil)
		result := make(chan providerReconcileTestConfig, 1)
		errs := make(chan error, 1)
		go func() {
			config, ok, err := routes.Wait(t.Context(), "brg-a", time.Second, nil)
			if err != nil {
				errs <- err
				return
			}
			if !ok {
				errs <- context.DeadlineExceeded
				return
			}
			result <- config
		}()
		routes.Replace(map[string]providerReconcileTestConfig{
			"brg-a": {InstanceID: "brg-a"},
		}, nil)
		select {
		case err := <-errs:
			t.Fatalf("Wait() error = %v", err)
		case config := <-result:
			if config.InstanceID != "brg-a" {
				t.Fatalf("Wait() config = %#v", config)
			}
		case <-t.Context().Done():
			t.Fatal("Wait() did not wake after route swap")
		}
	})

	t.Run("Should leave terminal and progress cleanup policy to the provider", func(t *testing.T) {
		t.Parallel()

		store := NewDeliveryStateStore[providerReconcileTestConfig]()
		store.Store("dlv-1", providerReconcileTestConfig{Generation: 1})
		if !store.Update("dlv-1", func(state providerReconcileTestConfig) providerReconcileTestConfig {
			state.Generation++
			return state
		}) {
			t.Fatal("Update(dlv-1) = false")
		}
		state, ok := store.Delete("dlv-1")
		if !ok || state.Generation != 2 {
			t.Fatalf("Delete(dlv-1) = %#v, %t", state, ok)
		}
		store.Store("dlv-2", providerReconcileTestConfig{Generation: 2})
		store.Store("dlv-3", providerReconcileTestConfig{Generation: 3})
		previous := store.TransformAll(func(key string, state providerReconcileTestConfig) providerReconcileTestConfig {
			state.InstanceID = key
			state.Generation++
			return state
		})
		if len(previous) != 2 || previous["dlv-2"].Generation != 2 {
			t.Fatalf("TransformAll() previous = %#v", previous)
		}
		if state, ok := store.Load("dlv-3"); !ok || state.InstanceID != "dlv-3" || state.Generation != 4 {
			t.Fatalf("Load(dlv-3) = %#v, %t", state, ok)
		}
		drained := store.Drain()
		if len(drained) != 2 {
			t.Fatalf("Drain() len = %d, want 2", len(drained))
		}
		if _, ok := store.Load("dlv-2"); ok {
			t.Fatal("Load(dlv-2) remained present after Drain()")
		}
	})

	t.Run("Should create delivery state once under concurrent access", func(t *testing.T) {
		t.Parallel()

		store := NewDeliveryStateStore[providerReconcileTestConfig]()
		var creates int
		var createsMu sync.Mutex
		var wg sync.WaitGroup
		for range 32 {
			wg.Go(func() {
				state, _ := store.UpdateOrCreate(
					"dlv-shared",
					func(current providerReconcileTestConfig, exists bool) providerReconcileTestConfig {
						if !exists {
							createsMu.Lock()
							creates++
							createsMu.Unlock()
							current.Generation = 1
						}
						return current
					},
				)
				if state.Generation != 1 {
					t.Errorf("UpdateOrCreate() state = %#v, want generation 1", state)
				}
			})
		}
		wg.Wait()
		if creates != 1 {
			t.Fatalf("state creations = %d, want 1", creates)
		}
	})

	t.Run("Should evict delivery state by age and capacity", func(t *testing.T) {
		t.Parallel()

		now := time.Unix(1_000, 0)
		store := NewDeliveryStateStore(
			WithDeliveryStateRetention[providerReconcileTestConfig](2, time.Minute, func() time.Time { return now }),
		)
		store.Store("dlv-oldest", providerReconcileTestConfig{Generation: 1})
		now = now.Add(time.Second)
		store.Store("dlv-middle", providerReconcileTestConfig{Generation: 2})
		now = now.Add(time.Second)
		store.Store("dlv-newest", providerReconcileTestConfig{Generation: 3})
		if _, ok := store.Load("dlv-oldest"); ok {
			t.Fatal("Load(dlv-oldest) found = true after capacity eviction")
		}
		now = now.Add(time.Minute)
		if _, ok := store.Load("dlv-middle"); ok {
			t.Fatal("Load(dlv-middle) found = true after TTL expiry")
		}
		if _, ok := store.Load("dlv-newest"); ok {
			t.Fatal("Load(dlv-newest) found = true after TTL expiry")
		}
	})

	t.Run("Should expose snapshots and updates without leaking the route map", func(t *testing.T) {
		t.Parallel()

		routes := NewRouteTable[providerReconcileTestConfig](nil)
		routes.Replace(map[string]providerReconcileTestConfig{
			"brg-a": {InstanceID: "brg-a", Generation: 1},
		}, nil)
		snapshot := routes.Snapshot()
		snapshot["brg-a"] = providerReconcileTestConfig{Generation: 99}
		if !routes.Update("brg-a", func(config providerReconcileTestConfig) providerReconcileTestConfig {
			config.Generation++
			return config
		}) {
			t.Fatal("Update(brg-a) = false")
		}
		if config, ok := routes.Get("brg-a"); !ok || config.Generation != 2 {
			t.Fatalf("Get(brg-a) = %#v, %t, want generation 2", config, ok)
		}
	})
}
