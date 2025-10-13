package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKnowledgeBinding_Clone(t *testing.T) {
	t.Run("Should return empty struct when receiver is nil", func(t *testing.T) {
		var binding *KnowledgeBinding
		cloned := binding.Clone()
		assert.Equal(t, KnowledgeBinding{}, cloned)
		assert.Equal(t, "", cloned.ID)
		assert.Nil(t, cloned.TopK)
		assert.Nil(t, cloned.MinScore)
		assert.Nil(t, cloned.MaxTokens)
	})

	t.Run("Should create deep copy with all fields set", func(t *testing.T) {
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

	t.Run("Should create independent copy that doesn't affect original", func(t *testing.T) {
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
		cloned.InjectAs = "user"
		cloned.Fallback = "new-fallback"
		cloned.Filters["lang"] = "es"
		cloned.Filters["new"] = "value"
		assert.Equal(t, 5, *original.TopK, "original TopK should not change")
		assert.Equal(t, 0.5, *original.MinScore, "original MinScore should not change")
		assert.Equal(t, 500, *original.MaxTokens, "original MaxTokens should not change")
		assert.Equal(t, "system", original.InjectAs, "original InjectAs should not change")
		assert.Equal(t, "fallback-text", original.Fallback, "original Fallback should not change")
		assert.Equal(t, "en", original.Filters["lang"], "original Filters should not change")
		assert.NotContains(t, original.Filters, "new", "original Filters should not have new keys")
	})

	t.Run("Should handle binding with only string fields set", func(t *testing.T) {
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
		assert.Nil(t, cloned.Filters)
	})

	t.Run("Should handle binding with nil Filters map", func(t *testing.T) {
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
		assert.Nil(t, cloned.Filters)
	})
}

func TestKnowledgeBinding_Merge(t *testing.T) {
	t.Run("Should not panic when receiver is nil", func(t *testing.T) {
		var binding *KnowledgeBinding
		topK := 10
		override := &KnowledgeBinding{TopK: &topK}
		assert.NotPanics(t, func() {
			binding.Merge(override)
		})
	})

	t.Run("Should not panic when override is nil", func(t *testing.T) {
		topK := 5
		binding := &KnowledgeBinding{
			ID:   "kb-123",
			TopK: &topK,
		}
		assert.NotPanics(t, func() {
			binding.Merge(nil)
		})
		assert.Equal(t, "kb-123", binding.ID)
		assert.Equal(t, 5, *binding.TopK)
	})

	t.Run("Should merge all pointer fields when set in override", func(t *testing.T) {
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

	t.Run("Should merge string fields when non-empty in override", func(t *testing.T) {
		binding := &KnowledgeBinding{
			ID:       "kb-123",
			InjectAs: "original-inject",
			Fallback: "original-fallback",
		}
		override := &KnowledgeBinding{
			InjectAs: "new-inject",
			Fallback: "new-fallback",
		}
		binding.Merge(override)
		assert.Equal(t, "new-inject", binding.InjectAs)
		assert.Equal(t, "new-fallback", binding.Fallback)
	})

	t.Run("Should preserve original values when override fields are nil", func(t *testing.T) {
		originalTopK := 5
		originalMinScore := 0.5
		binding := &KnowledgeBinding{
			ID:       "kb-123",
			TopK:     &originalTopK,
			MinScore: &originalMinScore,
			InjectAs: "original",
		}
		override := &KnowledgeBinding{
			MaxTokens: nil,
		}
		binding.Merge(override)
		require.NotNil(t, binding.TopK)
		assert.Equal(t, 5, *binding.TopK)
		require.NotNil(t, binding.MinScore)
		assert.Equal(t, 0.5, *binding.MinScore)
		assert.Equal(t, "original", binding.InjectAs)
	})

	t.Run("Should preserve original string values when override strings are empty", func(t *testing.T) {
		binding := &KnowledgeBinding{
			ID:       "kb-123",
			InjectAs: "original-inject",
			Fallback: "original-fallback",
		}
		override := &KnowledgeBinding{
			InjectAs: "",
			Fallback: "",
		}
		binding.Merge(override)
		assert.Equal(t, "original-inject", binding.InjectAs, "empty InjectAs should not override")
		assert.Equal(t, "original-fallback", binding.Fallback, "empty Fallback should not override")
	})

	t.Run("Should replace Filters map when set in override", func(t *testing.T) {
		binding := &KnowledgeBinding{
			ID:      "kb-123",
			Filters: map[string]string{"old": "value", "keep": "this"},
		}
		override := &KnowledgeBinding{
			Filters: map[string]string{"new": "filter", "type": "doc"},
		}
		binding.Merge(override)
		require.NotNil(t, binding.Filters)
		assert.Equal(t, 2, len(binding.Filters))
		assert.Equal(t, "filter", binding.Filters["new"])
		assert.Equal(t, "doc", binding.Filters["type"])
		assert.NotContains(t, binding.Filters, "old", "old filters should be replaced")
		assert.NotContains(t, binding.Filters, "keep", "old filters should be replaced")
	})

	t.Run("Should preserve original Filters when override Filters is nil", func(t *testing.T) {
		binding := &KnowledgeBinding{
			ID:      "kb-123",
			Filters: map[string]string{"original": "value"},
		}
		override := &KnowledgeBinding{
			Filters: nil,
		}
		binding.Merge(override)
		require.NotNil(t, binding.Filters)
		assert.Equal(t, "value", binding.Filters["original"])
	})

	t.Run("Should create independent copy of pointer values", func(t *testing.T) {
		originalTopK := 5
		binding := &KnowledgeBinding{
			ID:   "kb-123",
			TopK: &originalTopK,
		}
		overrideTopK := 20
		override := &KnowledgeBinding{
			TopK: &overrideTopK,
		}
		binding.Merge(override)
		*override.TopK = 99
		require.NotNil(t, binding.TopK)
		assert.Equal(t, 20, *binding.TopK, "changing override value should not affect merged value")
	})

	t.Run("Should handle complete merge scenario", func(t *testing.T) {
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
		assert.Equal(t, 10, *binding.TopK, "TopK should be updated")
		require.NotNil(t, binding.MinScore)
		assert.Equal(t, 0.5, *binding.MinScore, "MinScore should remain unchanged")
		require.NotNil(t, binding.MaxTokens)
		assert.Equal(t, 1000, *binding.MaxTokens, "MaxTokens should be set")
		assert.Equal(t, "updated", binding.InjectAs, "InjectAs should be updated")
		assert.Equal(t, "original-fallback", binding.Fallback, "Fallback should remain unchanged")
		assert.Equal(t, "es", binding.Filters["lang"], "Filters should be completely replaced")
		assert.Equal(t, "doc", binding.Filters["type"], "Filters should contain new values")
	})
}
