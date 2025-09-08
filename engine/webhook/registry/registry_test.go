package registry

import (
	"sync"
	"testing"

	"github.com/compozy/compozy/engine/webhook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_AddAndGet(t *testing.T) {
	t.Run("Should register unique slugs and retrieve entries", func(t *testing.T) {
		reg := New()
		e1 := Entry{
			WorkflowID: "wf-1",
			Webhook: &webhook.Config{
				Slug:   "alpha",
				Events: []webhook.EventConfig{{Name: "ev", Filter: "true", Input: map[string]string{"a": "b"}}},
			},
		}
		e2 := Entry{
			WorkflowID: "wf-2",
			Webhook: &webhook.Config{
				Slug:   "beta",
				Events: []webhook.EventConfig{{Name: "ev2", Filter: "true", Input: map[string]string{"c": "d"}}},
			},
		}
		require.NoError(t, reg.Add(" alpha ", e1))
		require.NoError(t, reg.Add("BETA", e2))
		got1, ok1 := reg.Get(" Alpha ")
		require.True(t, ok1)
		assert.Equal(t, "wf-1", got1.WorkflowID)
		got2, ok2 := reg.Get("beta")
		require.True(t, ok2)
		assert.Equal(t, "wf-2", got2.WorkflowID)
		_, ok3 := reg.Get("missing")
		assert.False(t, ok3)
	})
}

func TestRegistry_DuplicateSlug(t *testing.T) {
	t.Run("Should fail on duplicate slug", func(t *testing.T) {
		reg := New()
		e := Entry{
			WorkflowID: "wf-1",
			Webhook: &webhook.Config{
				Slug:   "dup",
				Events: []webhook.EventConfig{{Name: "ev", Filter: "true", Input: map[string]string{"a": "b"}}},
			},
		}
		require.NoError(t, reg.Add("dup", e))
		err := reg.Add("DUP", e)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrDuplicateSlug)
	})
}

func TestRegistry_SlugValidationAndMismatch(t *testing.T) {
	t.Run("Should reject empty slug and mismatched payload slug", func(t *testing.T) {
		reg := New()
		badEntry := Entry{
			WorkflowID: "wf-1",
			Webhook: &webhook.Config{
				Slug:   "beta",
				Events: []webhook.EventConfig{{Name: "ev", Filter: "true", Input: map[string]string{"a": "b"}}},
			},
		}
		err1 := reg.Add(" ", badEntry)
		require.Error(t, err1)
		assert.ErrorContains(t, err1, "slug must not be empty")
		mismatch := Entry{
			WorkflowID: "wf-2",
			Webhook: &webhook.Config{
				Slug:   "gamma",
				Events: []webhook.EventConfig{{Name: "ev2", Filter: "true", Input: map[string]string{"c": "d"}}},
			},
		}
		err2 := reg.Add("delta", mismatch)
		require.Error(t, err2)
		assert.ErrorContains(t, err2, "slug mismatch")
	})
}

func TestRegistry_ConcurrentReads(t *testing.T) {
	t.Run("Should handle concurrent Get calls safely", func(t *testing.T) {
		reg := New()
		e := Entry{
			WorkflowID: "wf-1",
			Webhook: &webhook.Config{
				Slug:   "alpha",
				Events: []webhook.EventConfig{{Name: "ev", Filter: "true", Input: map[string]string{"a": "b"}}},
			},
		}
		require.NoError(t, reg.Add("alpha", e))
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _ = reg.Get("ALPHA")
			}()
		}
		wg.Wait()
	})
}
