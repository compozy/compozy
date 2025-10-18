package pricing

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/stretchr/testify/require"
)

func TestLookup(t *testing.T) {
	price, ok := Lookup(core.ProviderOpenAI, "gpt-4o")
	require.True(t, ok)
	require.InDelta(t, 0.005, price.PromptPer1K, 1e-9)
	require.InDelta(t, 0.015, price.CompletionPer1K, 1e-9)
}

func TestEstimateCostUSD(t *testing.T) {
	usage := &llmadapter.Usage{
		PromptTokens:     2000,
		CompletionTokens: 1000,
	}
	cost, ok := EstimateCostUSD(core.ProviderOpenAI, "gpt-4o", usage)
	require.True(t, ok)
	require.InDelta(t, 0.025, cost, 1e-6)
}

func TestEstimateCostUSD_UnknownModel(t *testing.T) {
	cost, ok := EstimateCostUSD(core.ProviderOpenAI, "unknown-model", &llmadapter.Usage{})
	require.False(t, ok)
	require.Zero(t, cost)
}
