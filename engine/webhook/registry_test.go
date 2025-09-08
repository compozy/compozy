package webhook

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_AddAndGet(t *testing.T) {
	t.Run("Should register unique slugs and retrieve entries", func(t *testing.T) {
		reg := NewRegistry()
		e1 := RegistryEntry{
			WorkflowID: "wf-1",
			Webhook: &Config{
				Slug:   "alpha",
				Events: []EventConfig{{Name: "ev", Filter: "true", Input: map[string]string{"a": "b"}}},
			},
		}
		e2 := RegistryEntry{
			WorkflowID: "wf-2",
			Webhook: &Config{
				Slug:   "beta",
				Events: []EventConfig{{Name: "ev2", Filter: "true", Input: map[string]string{"c": "d"}}},
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
		reg := NewRegistry()
		e := RegistryEntry{
			WorkflowID: "wf-1",
			Webhook: &Config{
				Slug:   "dup",
				Events: []EventConfig{{Name: "ev", Filter: "true", Input: map[string]string{"a": "b"}}},
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
		reg := NewRegistry()
		badEntry := RegistryEntry{
			WorkflowID: "wf-1",
			Webhook: &Config{
				Slug:   "beta",
				Events: []EventConfig{{Name: "ev", Filter: "true", Input: map[string]string{"a": "b"}}},
			},
		}
		err1 := reg.Add(" ", badEntry)
		require.Error(t, err1)
		assert.ErrorContains(t, err1, "slug must not be empty")
		mismatch := RegistryEntry{
			WorkflowID: "wf-2",
			Webhook: &Config{
				Slug:   "gamma",
				Events: []EventConfig{{Name: "ev2", Filter: "true", Input: map[string]string{"c": "d"}}},
			},
		}
		err2 := reg.Add("delta", mismatch)
		require.Error(t, err2)
		assert.ErrorContains(t, err2, "slug mismatch")
	})
}

func TestRegistry_ConcurrentReads(t *testing.T) {
	t.Run("Should handle concurrent Get calls safely", func(t *testing.T) {
		reg := NewRegistry()
		e := RegistryEntry{
			WorkflowID: "wf-1",
			Webhook: &Config{
				Slug:   "alpha",
				Events: []EventConfig{{Name: "ev", Filter: "true", Input: map[string]string{"a": "b"}}},
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
