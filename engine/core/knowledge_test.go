package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestKnowledgeBindingClone tests all clone behaviors for KnowledgeBinding.
func TestKnowledgeBindingClone(t *testing.T) {
	t.Parallel()

	t.Run("Should return zero value for nil receiver", func(t *testing.T) {
		t.Parallel()
		var binding *KnowledgeBinding
		cloned := binding.Clone()
		assert.Equal(t, KnowledgeBinding{}, cloned)
	})

	t.Run("Should copy pointer fields independently", func(t *testing.T) {
		t.Parallel()
		topK := 10
		minScore := 0.75
		maxTokens := 1000
		original := &KnowledgeBinding{
			ID:        "kb-123",
			TopK:      &topK,
			MinScore:  &minScore,
			MaxTokens: &maxTokens,
			InjectAs:  "context",
			Fallback:  "default-text",
			Filters:   map[string]string{"type": "doc", "category": "tech"},
		}
		cloned := original.Clone()
		assert.Equal(t, original.ID, cloned.ID)
		require.NotNil(t, cloned.TopK)
		assert.Equal(t, *original.TopK, *cloned.TopK)
		require.NotNil(t, cloned.MinScore)
		assert.Equal(t, *original.MinScore, *cloned.MinScore)
		require.NotNil(t, cloned.MaxTokens)
		assert.Equal(t, *original.MaxTokens, *cloned.MaxTokens)
		assert.Equal(t, original.InjectAs, cloned.InjectAs)
		assert.Equal(t, original.Fallback, cloned.Fallback)
		assert.Equal(t, original.Filters, cloned.Filters)
	})

	t.Run("Should maintain independence after clone", func(t *testing.T) {
		t.Parallel()
		topK := 5
		minScore := 0.5
		maxTokens := 500
		original := &KnowledgeBinding{
			ID:        "kb-456",
			TopK:      &topK,
			MinScore:  &minScore,
			MaxTokens: &maxTokens,
			InjectAs:  "system",
			Fallback:  "fallback-text",
			Filters:   map[string]string{"lang": "en"},
		}
		cloned := original.Clone()
		*cloned.TopK = 20
		*cloned.MinScore = 0.9
		*cloned.MaxTokens = 2000
		cloned.Filters["lang"] = "es"
		cloned.Filters["new"] = "value"
		assert.Equal(t, 5, *original.TopK)
		assert.Equal(t, 0.5, *original.MinScore)
		assert.Equal(t, 500, *original.MaxTokens)
		assert.Equal(t, "system", original.InjectAs)
		assert.Equal(t, "fallback-text", original.Fallback)
		assert.Equal(t, "en", original.Filters["lang"])
		assert.NotContains(t, original.Filters, "new")
	})

	t.Run("Should handle bindings with only string fields", func(t *testing.T) {
		t.Parallel()
		original := &KnowledgeBinding{
			ID:       "kb-789",
			InjectAs: "assistant",
			Fallback: "default",
		}
		cloned := original.Clone()
		assert.Equal(t, original.ID, cloned.ID)
		assert.Equal(t, original.InjectAs, cloned.InjectAs)
		assert.Equal(t, original.Fallback, cloned.Fallback)
		assert.Nil(t, cloned.TopK)
		assert.Nil(t, cloned.MinScore)
		assert.Nil(t, cloned.MaxTokens)
		assert.Empty(t, cloned.Filters)
	})

	t.Run("Should initialize empty map for nil filters", func(t *testing.T) {
		t.Parallel()
		topK := 15
		original := &KnowledgeBinding{
			ID:      "kb-nil-filters",
			TopK:    &topK,
			Filters: nil,
		}
		cloned := original.Clone()
		assert.Equal(t, original.ID, cloned.ID)
		require.NotNil(t, cloned.TopK)
		assert.Equal(t, *original.TopK, *cloned.TopK)
		assert.Empty(t, cloned.Filters)
	})
}

// TestKnowledgeBindingMerge tests all merge behaviors for KnowledgeBinding.
func TestKnowledgeBindingMerge(t *testing.T) {
	t.Parallel()

	t.Run("Should safely handle nil receiver", func(t *testing.T) {
		t.Parallel()
		var binding *KnowledgeBinding
		topK := 10
		override := &KnowledgeBinding{TopK: &topK}
		assert.NotPanics(t, func() {
			binding.Merge(override)
		})
	})

	t.Run("Should leave binding unchanged when override is nil", func(t *testing.T) {
		t.Parallel()
		topK := 5
		binding := &KnowledgeBinding{ID: "kb-123", TopK: &topK}
		assert.NotPanics(t, func() {
			binding.Merge(nil)
		})
		assert.Equal(t, "kb-123", binding.ID)
		assert.Equal(t, 5, *binding.TopK)
	})

	t.Run("Should overwrite pointer field values", func(t *testing.T) {
		t.Parallel()
		originalTopK := 5
		originalMinScore := 0.5
		originalMaxTokens := 500
		binding := &KnowledgeBinding{
			ID:        "kb-123",
			TopK:      &originalTopK,
			MinScore:  &originalMinScore,
			MaxTokens: &originalMaxTokens,
		}
		overrideTopK := 20
		overrideMinScore := 0.9
		overrideMaxTokens := 2000
		override := &KnowledgeBinding{
			TopK:      &overrideTopK,
			MinScore:  &overrideMinScore,
			MaxTokens: &overrideMaxTokens,
		}
		binding.Merge(override)
		require.NotNil(t, binding.TopK)
		assert.Equal(t, 20, *binding.TopK)
		require.NotNil(t, binding.MinScore)
		assert.Equal(t, 0.9, *binding.MinScore)
		require.NotNil(t, binding.MaxTokens)
		assert.Equal(t, 2000, *binding.MaxTokens)
	})

	t.Run("Should override string fields with non-empty values", func(t *testing.T) {
		t.Parallel()
		binding := &KnowledgeBinding{
			ID:       "kb-123",
			InjectAs: "original-inject",
			Fallback: "original-fallback",
		}
		override := &KnowledgeBinding{InjectAs: "new-inject", Fallback: "new-fallback"}
		binding.Merge(override)
		assert.Equal(t, "new-inject", binding.InjectAs)
		assert.Equal(t, "new-fallback", binding.Fallback)
	})

	t.Run("Should preserve original pointer fields when override has nil pointers", func(t *testing.T) {
		t.Parallel()
		originalTopK := 5
		originalMinScore := 0.5
		binding := &KnowledgeBinding{
			ID:       "kb-123",
			TopK:     &originalTopK,
			MinScore: &originalMinScore,
			InjectAs: "original",
		}
		binding.Merge(&KnowledgeBinding{})
		require.NotNil(t, binding.TopK)
		assert.Equal(t, 5, *binding.TopK)
		require.NotNil(t, binding.MinScore)
		assert.Equal(t, 0.5, *binding.MinScore)
		assert.Equal(t, "original", binding.InjectAs)
	})

	t.Run("Should ignore empty string overrides", func(t *testing.T) {
		t.Parallel()
		binding := &KnowledgeBinding{
			ID:       "kb-123",
			InjectAs: "original-inject",
			Fallback: "original-fallback",
		}
		binding.Merge(&KnowledgeBinding{InjectAs: "", Fallback: ""})
		assert.Equal(t, "original-inject", binding.InjectAs)
		assert.Equal(t, "original-fallback", binding.Fallback)
	})

	t.Run("Should replace filters completely with override filters", func(t *testing.T) {
		t.Parallel()
		binding := &KnowledgeBinding{
			ID:      "kb-123",
			Filters: map[string]string{"old": "value", "keep": "this"},
		}
		override := &KnowledgeBinding{Filters: map[string]string{"new": "filter", "type": "doc"}}
		binding.Merge(override)
		require.NotNil(t, binding.Filters)
		assert.Equal(t, 2, len(binding.Filters))
		assert.Equal(t, "filter", binding.Filters["new"])
		assert.Equal(t, "doc", binding.Filters["type"])
		assert.NotContains(t, binding.Filters, "old")
		assert.NotContains(t, binding.Filters, "keep")
	})

	t.Run("Should preserve original filters when override filters are nil", func(t *testing.T) {
		t.Parallel()
		binding := &KnowledgeBinding{
			ID:      "kb-123",
			Filters: map[string]string{"original": "value"},
		}
		binding.Merge(&KnowledgeBinding{Filters: nil})
		require.NotNil(t, binding.Filters)
		assert.Equal(t, "value", binding.Filters["original"])
	})

	t.Run("Should maintain pointer independence after merge", func(t *testing.T) {
		t.Parallel()
		originalTopK := 5
		binding := &KnowledgeBinding{ID: "kb-123", TopK: &originalTopK}
		overrideTopK := 20
		override := &KnowledgeBinding{TopK: &overrideTopK}
		binding.Merge(override)
		*override.TopK = 99
		require.NotNil(t, binding.TopK)
		assert.Equal(t, 20, *binding.TopK)
	})

	t.Run("Should handle complete merge scenario correctly", func(t *testing.T) {
		t.Parallel()
		originalTopK := 5
		originalMinScore := 0.5
		binding := &KnowledgeBinding{
			ID:        "kb-complete",
			TopK:      &originalTopK,
			MinScore:  &originalMinScore,
			MaxTokens: nil,
			InjectAs:  "original",
			Fallback:  "original-fallback",
			Filters:   map[string]string{"lang": "en"},
		}
		overrideTopK := 10
		overrideMaxTokens := 1000
		override := &KnowledgeBinding{
			TopK:      &overrideTopK,
			MaxTokens: &overrideMaxTokens,
			InjectAs:  "updated",
			Filters:   map[string]string{"lang": "es", "type": "doc"},
		}
		binding.Merge(override)
		require.NotNil(t, binding.TopK)
		assert.Equal(t, 10, *binding.TopK)
		require.NotNil(t, binding.MinScore)
		assert.Equal(t, 0.5, *binding.MinScore)
		require.NotNil(t, binding.MaxTokens)
		assert.Equal(t, 1000, *binding.MaxTokens)
		assert.Equal(t, "updated", binding.InjectAs)
		assert.Equal(t, "original-fallback", binding.Fallback)
		assert.Equal(t, "es", binding.Filters["lang"])
		assert.Equal(t, "doc", binding.Filters["type"])
	})
}
