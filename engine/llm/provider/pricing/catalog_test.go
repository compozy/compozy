package pricing

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/stretchr/testify/require"
)

func TestLookup(t *testing.T) {
	t.Run("Should return pricing for known provider and model", func(t *testing.T) {
		price, ok := Lookup(core.ProviderOpenAI, "gpt-4o")
		require.True(t, ok)
		require.InDelta(t, 0.005, price.PromptPer1K, 1e-9)
		require.InDelta(t, 0.015, price.CompletionPer1K, 1e-9)
	})
}

func TestEstimateCostUSD(t *testing.T) {
	t.Run("Should calculate cost for valid usage", func(t *testing.T) {
		usage := &llmadapter.Usage{
			PromptTokens:     2000,
			CompletionTokens: 1000,
		}
		cost, ok := EstimateCostUSD(core.ProviderOpenAI, "gpt-4o", usage)
		require.True(t, ok)
		require.InDelta(t, 0.025, cost, 1e-6)
	})
}

func TestEstimateCostUSD_UnknownModel(t *testing.T) {
	t.Run("Should return false for unknown model", func(t *testing.T) {
		cost, ok := EstimateCostUSD(core.ProviderOpenAI, "unknown-model", &llmadapter.Usage{})
		require.False(t, ok)
		require.Zero(t, cost)
	})
}

func TestLookup_CaseInsensitive(t *testing.T) {
	t.Run("Should match provider case-insensitively", func(t *testing.T) {
		price1, ok1 := Lookup(core.ProviderOpenAI, "gpt-4o")
		price2, ok2 := Lookup(core.ProviderName("OpenAI"), "gpt-4o")
		require.True(t, ok1)
		require.True(t, ok2)
		require.Equal(t, price1, price2)
	})
}

func TestLookup_ModelNormalization(t *testing.T) {
	t.Run("Should normalize model with colon separator", func(t *testing.T) {
		price1, ok1 := Lookup(core.ProviderOpenAI, "gpt-4o")
		price2, ok2 := Lookup(core.ProviderOpenAI, "gpt:4o")
		require.True(t, ok1)
		require.True(t, ok2)
		require.Equal(t, price1, price2)
	})

	t.Run("Should normalize model with dot separator", func(t *testing.T) {
		price1, ok1 := Lookup(core.ProviderGoogle, "gemini-1-5-pro")
		price2, ok2 := Lookup(core.ProviderGoogle, "gemini.1.5.pro")
		require.True(t, ok1)
		require.True(t, ok2)
		require.Equal(t, price1, price2)
	})
}

func TestLookup_UnknownProvider(t *testing.T) {
	t.Run("Should return false for unknown provider", func(t *testing.T) {
		_, ok := Lookup(core.ProviderName("unknown-provider"), "some-model")
		require.False(t, ok)
	})
}

func TestEstimateCostUSD_ZeroTokenUsage(t *testing.T) {
	t.Run("Should return zero cost and false for zero tokens", func(t *testing.T) {
		usage := &llmadapter.Usage{
			PromptTokens:     0,
			CompletionTokens: 0,
		}
		cost, ok := EstimateCostUSD(core.ProviderOpenAI, "gpt-4o", usage)
		require.False(t, ok)
		require.Zero(t, cost)
	})
}

func TestEstimateCostUSD_NilUsage(t *testing.T) {
	t.Run("Should return false for nil usage", func(t *testing.T) {
		cost, ok := EstimateCostUSD(core.ProviderOpenAI, "gpt-4o", nil)
		require.False(t, ok)
		require.Zero(t, cost)
	})
}
