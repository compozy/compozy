package webhook

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_AddAndGet(t *testing.T) {
	t.Run("Should register unique slugs and retrieve entries", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
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
		t.Parallel()
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

func TestRegistry_Remove(t *testing.T) {
	t.Run("Should remove webhook by slug", func(t *testing.T) {
		t.Parallel()
		reg := NewRegistry()
		e := RegistryEntry{
			WorkflowID: "wf-1",
			Webhook: &Config{
				Slug:   "remove-me",
				Events: []EventConfig{{Name: "ev", Filter: "true", Input: map[string]string{"a": "b"}}},
			},
		}

		// Add entry
		require.NoError(t, reg.Add("remove-me", e))

		// Verify it exists
		_, ok := reg.Get("REMOVE-ME") // test case-insensitive
		require.True(t, ok)

		// Remove it
		reg.Remove(" REMOVE-ME ") // test trimming and case-insensitive

		// Verify it's gone
		_, ok = reg.Get("remove-me")
		assert.False(t, ok)
	})

	t.Run("Should handle removing non-existent slug gracefully", func(t *testing.T) {
		t.Parallel()
		reg := NewRegistry()

		// Should not panic or error when removing non-existent slug
		reg.Remove("non-existent")
		reg.Remove("") // empty slug should also be handled

		// Registry should remain empty
		slugs := reg.Slugs()
		assert.Empty(t, slugs)
	})
}

func TestRegistry_Slugs(t *testing.T) {
	t.Run("Should return sorted normalized slugs", func(t *testing.T) {
		t.Parallel()
		reg := NewRegistry()

		entries := []struct {
			slug  string
			entry RegistryEntry
		}{
			{" Zeta ", RegistryEntry{WorkflowID: "wf-3", Webhook: &Config{Slug: "zeta"}}},
			{"alpha", RegistryEntry{WorkflowID: "wf-1", Webhook: &Config{Slug: "ALPHA"}}},
			{" BETA ", RegistryEntry{WorkflowID: "wf-2", Webhook: &Config{Slug: "beta"}}},
		}

		// Add entries with mixed case and whitespace
		for _, entry := range entries {
			require.NoError(t, reg.Add(entry.slug, entry.entry))
		}

		// Get slugs - should be sorted and normalized
		slugs := reg.Slugs()

		// Should return 3 slugs in sorted order
		expected := []string{"alpha", "beta", "zeta"}
		assert.Equal(t, expected, slugs)
	})

	t.Run("Should return empty slice for empty registry", func(t *testing.T) {
		t.Parallel()
		reg := NewRegistry()

		slugs := reg.Slugs()
		assert.Empty(t, slugs)
		assert.IsType(t, []string{}, slugs)
	})

	t.Run("Should return slugs after remove operations", func(t *testing.T) {
		t.Parallel()
		reg := NewRegistry()

		// Add multiple entries
		require.NoError(t, reg.Add("alpha", RegistryEntry{WorkflowID: "wf-1", Webhook: &Config{Slug: "alpha"}}))
		require.NoError(t, reg.Add("beta", RegistryEntry{WorkflowID: "wf-2", Webhook: &Config{Slug: "beta"}}))
		require.NoError(t, reg.Add("gamma", RegistryEntry{WorkflowID: "wf-3", Webhook: &Config{Slug: "gamma"}}))

		// Verify all slugs present
		slugs := reg.Slugs()
		assert.Equal(t, []string{"alpha", "beta", "gamma"}, slugs)

		// Remove middle entry
		reg.Remove("BETA")

		// Verify remaining slugs
		slugs = reg.Slugs()
		assert.Equal(t, []string{"alpha", "gamma"}, slugs)
	})
}

func TestRegistry_ConcurrentReads(t *testing.T) {
	t.Run("Should handle concurrent Get calls safely", func(t *testing.T) {
		t.Parallel()
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
		for range 100 {
			wg.Go(func() {
				_, _ = reg.Get("ALPHA")
			})
		}
		wg.Wait()
	})
}
