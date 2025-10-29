package standalone

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/test/helpers"
)

// rshort is a small timeout used in watch tests to keep them fast but stable.
const rshort = 1 * time.Second

func TestResourceStore_MiniredisCompatibility(t *testing.T) {
	t.Run("Should support TxPipeline atomic operations", func(t *testing.T) {
		ctx := helpers.NewTestContext(t)
		env := helpers.SetupStandaloneResourceStore(ctx, t)
		defer env.Cleanup()

		key := resources.ResourceKey{Project: "proj", Type: resources.ResourceAgent, ID: "writer"}
		value := map[string]any{"id": "writer", "cfg": map[string]any{"x": 1.0}}

		et1, err := env.Store.Put(ctx, key, value)
		require.NoError(t, err)
		require.NotEmpty(t, et1)

		got, et2, err := env.Store.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, et1, et2)
		m := got.(map[string]any)
		assert.Equal(t, "writer", m["id"]) // deep copy semantics

		// Update and verify ETag changes, implying value+etag were updated together.
		value2 := map[string]any{"id": "writer", "cfg": map[string]any{"x": 2.0}}
		et3, err := env.Store.Put(ctx, key, value2)
		require.NoError(t, err)
		assert.NotEqual(t, et1, et3)

		got2, et4, err := env.Store.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, et3, et4)
        m2 := got2.(map[string]any)
        x := m2["cfg"].(map[string]any)["x"]
        switch xv := x.(type) {
        case string:
            assert.Equal(t, "2", xv)
        case fmt.Stringer:
            assert.Equal(t, "2", xv.String())
        default:
            // As a fallback, accept numeric equality when decoded as float64
            assert.Equal(t, 2.0, xv)
        }
	})

	t.Run("Should support optimistic locking via PutIfMatch Lua script", func(t *testing.T) {
		ctx := helpers.NewTestContext(t)
		env := helpers.SetupStandaloneResourceStore(ctx, t)
		defer env.Cleanup()

		key := resources.ResourceKey{Project: "proj", Type: resources.ResourceModel, ID: "gpt"}
		initial := map[string]any{"id": "gpt", "ver": 1.0}
		et, err := env.Store.Put(ctx, key, initial)
		require.NoError(t, err)

		// Correct ETag → success
		updated := map[string]any{"id": "gpt", "ver": 2.0}
		newETag, err := env.Store.PutIfMatch(ctx, key, updated, et)
		require.NoError(t, err)
		assert.NotEmpty(t, newETag)
		assert.NotEqual(t, et, newETag)

		// Stale ETag → conflict error
		stale := map[string]any{"id": "gpt", "ver": 3.0}
		_, err = env.Store.PutIfMatch(ctx, key, stale, et)
		require.Error(t, err)
		assert.True(t, errors.Is(err, resources.ErrETagMismatch))
	})

	t.Run("Should maintain ETag consistency across operations", func(t *testing.T) {
		ctx := helpers.NewTestContext(t)
		env := helpers.SetupStandaloneResourceStore(ctx, t)
		defer env.Cleanup()

		key := resources.ResourceKey{Project: "proj", Type: resources.ResourceSchema, ID: "schema1"}
		et, err := env.Store.Put(ctx, key, map[string]any{"id": "schema1", "v": 1.0})
		require.NoError(t, err)
		require.NotEmpty(t, et)

		// ETag changes on each update
		et2, err := env.Store.Put(ctx, key, map[string]any{"id": "schema1", "v": 2.0})
		require.NoError(t, err)
		assert.NotEqual(t, et, et2)

		// Get returns last ETag
		_, et3, err := env.Store.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, et2, et3)
	})

	t.Run("Should handle concurrent resource updates correctly", func(t *testing.T) {
		ctx := helpers.NewTestContext(t)
		env := helpers.SetupStandaloneResourceStore(ctx, t)
		defer env.Cleanup()

		key := resources.ResourceKey{Project: "proj", Type: resources.ResourceTool, ID: "tool1"}
		et, err := env.Store.Put(ctx, key, map[string]any{"id": "tool1", "seq": 0.0})
		require.NoError(t, err)

		// Compete with PutIfMatch using the same starting ETag; exactly one should win.
		var wg sync.WaitGroup
		const n = 10
		errs := make([]error, n)
		var success int

		wg.Go(func() {
			// No-op to exercise WaitGroup.Go usage; actual updates below.
		})
		for i := 0; i < n; i++ {
			i := i
			wg.Go(func() {
				val := map[string]any{"id": "tool1", "seq": float64(i + 1)}
				_, err := env.Store.PutIfMatch(ctx, key, val, et)
				if err == nil {
					// track success in a data race-safe way by deferring read aggregation
					errs[i] = nil
				} else {
					errs[i] = err
				}
			})
		}
		wg.Wait()

		for _, e := range errs {
			if e == nil {
				success++
			} else {
				assert.True(t, errors.Is(e, resources.ErrETagMismatch) || errors.Is(e, resources.ErrNotFound))
			}
		}
		assert.Equal(t, 1, success, "only one concurrent update should succeed")
	})

	t.Run("Should publish watch notifications via Pub/Sub", func(t *testing.T) {
		ctx := helpers.NewTestContext(t)
		env := helpers.SetupStandaloneResourceStore(ctx, t)
		defer env.Cleanup()

		key := resources.ResourceKey{Project: "proj", Type: resources.ResourceAgent, ID: "watchme"}

		ch, err := env.Store.Watch(ctx, key.Project, key.Type)
		require.NoError(t, err)

		_, err = env.Store.Put(ctx, key, map[string]any{"id": "watchme"})
		require.NoError(t, err)

		select {
		case evt := <-ch:
			// First event could be a prime PUT if something already existed; ensure our key shows up quickly.
			if evt.Key != key {
				// Consume until our key arrives or timeout.
				timeout := time.After(rshort)
				for evt.Key != key {
					select {
					case evt = <-ch:
						if evt.Key == key {
							break
						}
					case <-timeout:
						t.Fatalf("did not receive expected event for key %v", key)
					}
				}
			}
			assert.Equal(t, resources.EventPut, evt.Type)
			assert.NotEmpty(t, evt.ETag)
		case <-time.After(2 * rshort):
			t.Fatalf("timeout waiting for watch event")
		}
	})

	t.Run("Should handle error cases gracefully", func(t *testing.T) {
		ctx := helpers.NewTestContext(t)
		env := helpers.SetupStandaloneResourceStore(ctx, t)
		defer env.Cleanup()

		missing := resources.ResourceKey{Project: "proj", Type: resources.ResourceSchema, ID: "missing"}
		_, _, err := env.Store.Get(ctx, missing)
		require.Error(t, err)
		assert.True(t, errors.Is(err, resources.ErrNotFound))

		// ETag mismatch path covered in optimistic locking test; add one more quick check here.
		key := resources.ResourceKey{Project: "proj", Type: resources.ResourceSchema, ID: "exists"}
		et, err := env.Store.Put(ctx, key, map[string]any{"id": "exists", "n": 1.0})
		require.NoError(t, err)
		_, err = env.Store.PutIfMatch(
			ctx,
			key,
			map[string]any{"id": "exists", "n": 2.0},
			resources.ETag(string(et)+"-stale"),
		)
		require.Error(t, err)
		assert.True(t, errors.Is(err, resources.ErrETagMismatch))
	})
}

func TestResourceStore_MultipleSubscribersReceiveNotifications(t *testing.T) {
	t.Run("Should deliver updates to multiple subscribers", func(t *testing.T) {
		ctx := helpers.NewTestContext(t)
		env := helpers.SetupStandaloneResourceStore(ctx, t)
		defer env.Cleanup()

		key := resources.ResourceKey{Project: "proj", Type: resources.ResourceTool, ID: "fanout"}
		c1, err := env.Store.Watch(ctx, key.Project, key.Type)
		require.NoError(t, err)
		c2, err := env.Store.Watch(ctx, key.Project, key.Type)
		require.NoError(t, err)

		_, err = env.Store.Put(ctx, key, map[string]any{"id": "fanout", "n": 1.0})
		require.NoError(t, err)

		waitOne := func(ch <-chan resources.Event) resources.Event {
			select {
			case e := <-ch:
				return e
			case <-time.After(2 * rshort):
				t.Fatalf("timeout waiting event")
			}
			return resources.Event{}
		}

		e1 := waitOne(c1)
		e2 := waitOne(c2)
		assert.Equal(t, key, e1.Key)
		assert.Equal(t, key, e2.Key)
		assert.Equal(t, resources.EventPut, e1.Type)
		assert.Equal(t, resources.EventPut, e2.Type)

		// Another update to confirm continuous delivery
		_, err = env.Store.Put(ctx, key, map[string]any{"id": "fanout", "n": 2.0})
		require.NoError(t, err)
		e1 = waitOne(c1)
		e2 = waitOne(c2)
		assert.Equal(t, resources.EventPut, e1.Type)
		assert.Equal(t, resources.EventPut, e2.Type)
	})
}

// generateTestResource is a tiny helper mirroring the PRD examples (kept local to this file).
func generateTestResource(id string, v int) map[string]any {
	return map[string]any{"id": id, "v": float64(v)}
}

func TestResourceStore_ListWithValuesConsistency(t *testing.T) {
	t.Run("Should return items with consistent ETags", func(t *testing.T) {
		ctx := helpers.NewTestContext(t)
		env := helpers.SetupStandaloneResourceStore(ctx, t)
		defer env.Cleanup()

		for i := 0; i < 5; i++ {
			id := fmt.Sprintf("r-%d", i)
			_, err := env.Store.Put(
				ctx,
				resources.ResourceKey{Project: "proj", Type: resources.ResourceAgent, ID: id},
				generateTestResource(id, i),
			)
			require.NoError(t, err)
		}
		items, err := env.Store.ListWithValues(ctx, "proj", resources.ResourceAgent)
		require.NoError(t, err)
		require.Len(t, items, 5)
		for _, it := range items {
			assert.NotEmpty(t, it.ETag)
			// Spot-check: immediate Get must return same ETag
			_, et, err := env.Store.Get(ctx, it.Key)
			require.NoError(t, err)
			assert.Equal(t, it.ETag, et)
		}
	})
}
